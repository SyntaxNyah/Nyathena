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
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// autoModActionKind is a pre-parsed integer representation of the configured
// automod action. Computed once at startup so the hot path (autoModCheck) never
// allocates or does string comparisons.
type autoModActionKind int

const (
	autoModActionBan     autoModActionKind = iota // default
	autoModActionKick
	autoModActionMute
	autoModActionTorment
)

// autoModAction caches the parsed action so autoModCheck is allocation-free.
var autoModAction autoModActionKind

// tormentMessages is allocated once and reused by every startTormentDisconnect call.
var tormentMessages = []string{
	"Connection timed out.",
	"Server encountered an error.",
	"Network instability detected.",
	"Session expired.",
	"Ping timeout.",
}

// tormentWarnings are plausible-sounding server messages sent to a tormented
// client before the final disconnect.  They mimic real network-health alerts so
// the user wastes time blaming their ISP rather than the server.
var tormentWarnings = []string{
	"Warning: unstable connection detected.",
	"Your connection is experiencing high latency.",
	"Server is experiencing network instability.",
	"Warning: packet loss detected on your connection.",
	"Connection quality degraded. Please check your network.",
	"Server load is high. You may experience interruptions.",
	"Your session may be affected by current network conditions.",
}

// tormentSeedSeq is incremented atomically for each new torment goroutine so
// that two goroutines started within the same nanosecond get distinct RNG seeds.
var tormentSeedSeq int64

// bannedWords holds the lowercased banned words loaded from the wordlist file.
// Stored as a slice for O(n) substring scan; lists are typically small so the
// overhead of a full scan per message is negligible compared to network I/O.
var bannedWords []string

// loadBannedWords reads the wordlist at the given path and populates bannedWords.
// Each non-empty, non-comment line is treated as one banned word (case-insensitive).
// Lines starting with '#' are treated as comments and ignored.
// Duplicates are removed and the list is sorted by word length ascending so that
// matchBannedWord — which returns on the first hit — short-circuits as early as
// possible when a message contains a short banned word.
func loadBannedWords(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		seen[strings.ToLower(line)] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	words := make([]string, 0, len(seen))
	for w := range seen {
		words = append(words, w)
	}
	// Shorter needles are checked first; matchBannedWord exits on first match,
	// so a short word match skips all remaining (longer) pattern checks.
	sort.Slice(words, func(i, j int) bool { return len(words[i]) < len(words[j]) })
	bannedWords = words
	return nil
}

// initAutoMod loads the banned-word list and caches the configured action when
// automod is enabled. Called once during server startup.
func initAutoMod(cfg *settings.Config) {
	if !cfg.AutoModEnabled {
		return
	}
	path := filepath.Join(settings.ConfigPath, cfg.AutoModWordlist)
	if err := loadBannedWords(path); err != nil {
		logger.LogWarningf("automod: failed to load wordlist %q: %v", path, err)
		return
	}
	logger.LogInfof("automod: loaded %d banned word(s) from %q", len(bannedWords), path)

	// Parse the action once so the hot path never allocates.
	switch strings.ToLower(strings.TrimSpace(cfg.AutoModAction)) {
	case "kick":
		autoModAction = autoModActionKick
	case "mute":
		autoModAction = autoModActionMute
	case "torment":
		autoModAction = autoModActionTorment
	default:
		autoModAction = autoModActionBan
	}
}

// autoModCheck tests msg for banned words. If one is found the configured action
// (ban/kick/mute/torment) is applied and the function returns true so the caller
// can abort further packet processing.
func autoModCheck(client *Client, msg string) bool {
	if !config.AutoModEnabled || len(bannedWords) == 0 {
		return false
	}

	lower := strings.ToLower(msg)
	matched, ok := matchBannedWord(lower)
	if !ok {
		return false
	}

	switch autoModAction {
	case autoModActionKick:
		client.SendPacket("KK", "Kicked for prohibited language.")
		client.conn.Close()
		logger.LogInfof("automod: kicked %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return true

	case autoModActionMute:
		// expires = 0 means permanent in the PUNISHMENTS table.
		if err := db.UpsertMute(client.Ipid(), int(ICOOCMuted), 0); err != nil {
			logger.LogErrorf("automod: failed to mute %v: %v", client.Ipid(), err)
			return false
		}
		client.SetMuted(ICOOCMuted)
		client.SetUnmuteTime(time.Time{}) // zero = permanent
		client.SendServerMessage("You have been muted for prohibited language.")
		logger.LogInfof("automod: permanently muted %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return true

	case autoModActionTorment:
		addTormentedIP(client.Ipid())
		go startTormentDisconnect(client)
		logger.LogInfof("automod: added %v (uid %d) to torment list — matched word %q", client.Ipid(), client.Uid(), matched)
		return true

	default: // autoModActionBan
		banTime := time.Now().UTC().Unix()
		id, err := db.AddBan(client.Ipid(), client.Hdid(), banTime, -1, "Automatic ban: prohibited language", "Server")
		if err != nil {
			logger.LogErrorf("automod: failed to ban %v: %v", client.Ipid(), err)
			return false
		}
		forgetIP(client.Ipid())
		client.SendPacket("KB", fmt.Sprintf("Banned for prohibited language.\nUntil: ∞\nID: %d", id))
		client.conn.Close()
		logger.LogInfof("automod: permanently banned %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return true
	}
}

// startTormentDisconnect harasses and disconnects a tormented client in three phases.
//
//  1. Initial wait: a random delay in the 5 s–4 min range so the user never
//     learns the pattern (30–60 s was too predictable).
//  2. Optional warnings: 0–3 plausible-sounding network-health messages, each
//     separated by a short pause, to make them blame their ISP.
//  3. Final kick: generic error message + connection close.
//
// A goroutine-local *rand.Rand is used instead of the global source so this
// goroutine never contends on the global rand mutex.
// Launched as a goroutine whenever a tormented IPID connects.
func startTormentDisconnect(client *Client) {
	// XOR with an atomically incrementing counter so goroutines spawned within
	// the same nanosecond still receive distinct seeds.
	seed := time.Now().UnixNano() ^ atomic.AddInt64(&tormentSeedSeq, 1)
	rng := rand.New(rand.NewSource(seed))

	// Phase 1 — unpredictable initial delay (5 s to 4 min).
	time.Sleep(time.Duration(5+rng.Intn(235)) * time.Second)

	// Phase 2 — zero to three pre-kick "network warning" messages.
	for i, n := 0, rng.Intn(4); i < n; i++ {
		client.SendServerMessage(tormentWarnings[rng.Intn(len(tormentWarnings))])
		time.Sleep(time.Duration(3+rng.Intn(8)) * time.Second)
	}

	// Phase 3 — final disconnect with a plausible-sounding error.
	client.SendPacket("KK", tormentMessages[rng.Intn(len(tormentMessages))])
	client.conn.Close()
}

// matchBannedWord performs a case-insensitive substring search of s against
// every entry in bannedWords. Substring matching catches evasion attempts such
// as embedded punctuation or spacing variants. Returns the matched word and
// true on the first hit, or ("", false) if no match is found.
func matchBannedWord(s string) (string, bool) {
	for _, word := range bannedWords {
		if strings.Contains(s, word) {
			return word, true
		}
	}
	return "", false
}
