/* Athena - A server for Attorney Online 2 written in Go
Copyright (C) 2022 MangosArentLiterature <mango@transmenace.dev>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>. */

package athena

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// firewallActive controls whether new connections must pass the IPHub VPN/proxy
// screening gate. When false (default), IPHub is never consulted.
var firewallActive atomic.Bool

// iphubCache stores the result of past IPHub API lookups so that each unique IP
// is checked at most once per server session, conserving the free-tier daily
// allowance of 1 000 requests.  true = VPN/proxy (block=1); false = clean.
var iphubCache = struct {
	mu    sync.RWMutex
	cache map[string]bool // raw IP → isVPN
}{cache: make(map[string]bool)}

// iphubInflight deduplicates concurrent API lookups for the same IP address.
// When a goroutine begins a lookup it inserts a channel here; subsequent
// goroutines that arrive for the same IP wait on that channel and then read the
// now-populated cache entry instead of making a second API call.
var iphubInflight = struct {
	mu sync.Mutex
	m  map[string]chan struct{} // raw IP → completion signal
}{m: make(map[string]chan struct{})}

// iphubResponse is the only field of the IPHub v2 JSON body that Athena needs.
type iphubResponse struct {
	Block int `json:"block"`
}

// iphubHTTPClient is shared across all lookups so TCP connections to the IPHub
// endpoint are reused (HTTP keep-alive), minimising per-lookup overhead.
// DisableCompression is set because IPHub responses are tiny (~50 bytes) and
// compression negotiation would cost more than it saves.
var iphubHTTPClient = &http.Client{
	Timeout: 4 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        2,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		ForceAttemptHTTP2:   false, // IPHub v2 API works fine on HTTP/1.1
	},
}

// queryIPHub calls the IPHub v2 API for the given raw IP address and returns
// true when IPHub classifies it as a VPN or non-residential proxy (block == 1).
// The API key must be non-empty; callers verify this beforehand.
func queryIPHub(apiKey, ip string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, "https://v2.api.iphub.info/ip/"+ip, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Key", apiKey)

	resp, err := iphubHTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("IPHub returned status %d", resp.StatusCode)
	}

	// Cap to 512 bytes — a valid IPHub response is ~50 bytes.
	var result iphubResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512)).Decode(&result); err != nil {
		return false, err
	}
	return result.Block == 1, nil
}

// checkFirewallForIP returns true (connection should be rejected) when:
//  1. The firewall is active (/firewall on).
//  2. An iphub_api_key is configured.
//  3. The IP is not already known to the server (ipFirstSeenTracker).
//  4. IPHub classifies the IP as a VPN/proxy.
//
// Each unique IP is queried at most once per session; the result is cached.
// Concurrent lookups for the same new IP are collapsed into a single API call
// via the inflight tracker — no duplicate requests are ever sent.
// On any API error the function fails open (allows the connection) and logs a
// warning so that a transient IPHub outage never locks out legitimate players.
func checkFirewallForIP(rawIP, ipid string) bool {
	if !firewallActive.Load() {
		return false
	}
	apiKey := config.IPHubAPIKey
	if apiKey == "" {
		return false
	}

	// Already-known IPs are exempt — no API call needed.
	ipFirstSeenTracker.mu.Lock()
	_, known := ipFirstSeenTracker.times[ipid]
	ipFirstSeenTracker.mu.Unlock()
	if known {
		return false
	}

	// Fast path: result already in cache (read lock, non-blocking for readers).
	iphubCache.mu.RLock()
	isVPN, cached := iphubCache.cache[rawIP]
	iphubCache.mu.RUnlock()
	if cached {
		return isVPN
	}

	// Slow path: ensure at most one goroutine calls the API per IP address.
	iphubInflight.mu.Lock()
	if ch, pending := iphubInflight.m[rawIP]; pending {
		// Another goroutine is already looking this IP up — wait for it, then
		// read from the cache (guaranteed non-empty once the channel closes).
		iphubInflight.mu.Unlock()
		<-ch
		iphubCache.mu.RLock()
		isVPN = iphubCache.cache[rawIP]
		iphubCache.mu.RUnlock()
		return isVPN
	}
	// Register this goroutine as the owner of the lookup.
	ch := make(chan struct{})
	iphubInflight.m[rawIP] = ch
	iphubInflight.mu.Unlock()

	// Wake all waiters regardless of how this function exits (including panics
	// from the HTTP stack). Writing false to the cache before closing ensures
	// waiters fail open rather than reading an uninitialised cache entry.
	defer func() {
		iphubCache.mu.Lock()
		if _, already := iphubCache.cache[rawIP]; !already {
			iphubCache.cache[rawIP] = false // safety net: fail open
		}
		iphubCache.mu.Unlock()
		// Delete and close under the same lock so there is no window between
		// the delete and the close during which a new goroutine could register
		// a fresh inflight entry for the same IP.
		iphubInflight.mu.Lock()
		delete(iphubInflight.m, rawIP)
		close(ch)
		iphubInflight.mu.Unlock()
	}()

	isVPN, err := queryIPHub(apiKey, rawIP)
	if err != nil {
		logger.LogErrorf("IPHub API error for %v: %v (allowing connection)", rawIP, err)
		isVPN = false // fail open
	}

	// Write result to cache; the defer above will then wake all waiters.
	iphubCache.mu.Lock()
	iphubCache.cache[rawIP] = isVPN
	iphubCache.mu.Unlock()

	return isVPN
}
