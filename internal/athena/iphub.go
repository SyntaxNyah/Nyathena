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
// is checked at most once per server session, conserving the free-tier daily limit
// of 1 000 requests.  true = VPN/proxy (block=1), false = residential/clean.
var iphubCache = struct {
	mu    sync.RWMutex
	cache map[string]bool // raw IP -> isVPN
}{
	cache: make(map[string]bool),
}

// iphubResponse is the subset of the IPHub v2 API response that Athena needs.
type iphubResponse struct {
	Block int `json:"block"`
}

// iphubHTTPClient is the HTTP client used for IPHub requests.  A custom client
// with a short timeout is used so that a slow/unreachable IPHub API does not
// stall the connection-accept loop.
var iphubHTTPClient = &http.Client{Timeout: 5 * time.Second}

// queryIPHub calls the IPHub v2 API for the given raw IP address and returns
// true if IPHub classifies it as a VPN or non-residential proxy (block == 1).
// The API key must be non-empty; callers should verify this beforehand.
func queryIPHub(apiKey, ip string) (bool, error) {
	url := fmt.Sprintf("https://v2.api.iphub.info/ip/%s", ip)
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	var result iphubResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.Block == 1, nil
}

// checkFirewallForIP returns true (connection should be rejected) when all of
// the following conditions hold:
//  1. The firewall is currently active (/firewall on).
//  2. An IPHub API key is configured.
//  3. The IP is not already known to the server (not in ipFirstSeenTracker).
//  4. The IP is classified as a VPN/proxy by the IPHub API.
//
// Results are cached per IP so the API is called at most once per session.
// On any API error the connection is allowed through (fail-open), and a warning
// is logged.
func checkFirewallForIP(rawIP, ipid string) bool {
	if !firewallActive.Load() {
		return false
	}
	if config == nil || config.IPHubAPIKey == "" {
		return false
	}

	// Known IPs are exempt from the firewall check.
	ipFirstSeenTracker.mu.Lock()
	_, known := ipFirstSeenTracker.times[ipid]
	ipFirstSeenTracker.mu.Unlock()
	if known {
		return false
	}

	// Consult the cache before hitting the API.
	iphubCache.mu.RLock()
	isVPN, cached := iphubCache.cache[rawIP]
	iphubCache.mu.RUnlock()
	if cached {
		return isVPN
	}

	// Not cached yet – query the API.
	isVPN, err := queryIPHub(config.IPHubAPIKey, rawIP)
	if err != nil {
		logger.LogErrorf("IPHub API error for %v: %v (allowing connection)", rawIP, err)
		// Fail open: do not block the connection when the API is unreachable.
		isVPN = false
	}

	// Cache the result regardless of whether it came from the API or a fallback.
	iphubCache.mu.Lock()
	iphubCache.cache[rawIP] = isVPN
	iphubCache.mu.Unlock()

	return isVPN
}
