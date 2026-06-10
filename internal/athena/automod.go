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
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// autoModActionKind is a pre-parsed integer representation of the configured
// automod action. Computed once at startup so the hot path (autoModCheck) never
// allocates or does string comparisons.
type autoModActionKind int

const (
	autoModActionBan autoModActionKind = iota // default
	autoModActionKick
	autoModActionMute
	autoModActionTorment
)

// autoModAction caches the parsed action so autoModCheck is allocation-free.
var autoModAction autoModActionKind

// tormentRng is a shared random source for all torment operations.
// A single instance avoids per-call heap allocations; the mutex is held only
// for the duration of one Intn call (nanoseconds), so contention is negligible.
var (
	tormentRng   = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	tormentRngMu sync.Mutex
)

// tormentIntn returns a non-negative random int in [0, n) using the shared RNG.
func tormentIntn(n int) int {
	tormentRngMu.Lock()
	defer tormentRngMu.Unlock()
	return tormentRng.Intn(n)
}

// The lowercased banned-word list lives behind an atomic.Pointer (bannedWordsPtr
// in livereload.go) so that /reload can swap it at runtime without racing the
// per-message reader. Read it via getBannedWords(); publish via setBannedWords().
// It is stored as a slice for O(n) substring scan; lists are typically small so
// the overhead of a full scan per message is negligible compared to network I/O.

// loadBannedWordsList reads the wordlist at the given path and returns the
// parsed words. Each non-empty, non-comment line is treated as one banned word
// (case-insensitive). Lines starting with '#' are treated as comments and
// ignored. Duplicates are removed and the list is sorted by word length
// ascending so that matchBannedWord — which returns on the first hit —
// short-circuits as early as possible when a message contains a short banned
// word.
func loadBannedWordsList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	words := make([]string, 0, len(seen))
	for w := range seen {
		words = append(words, w)
	}
	// Shorter needles are checked first; matchBannedWord exits on first match,
	// so a short word match skips all remaining (longer) pattern checks.
	sort.Slice(words, func(i, j int) bool { return len(words[i]) < len(words[j]) })
	return words, nil
}

// initAutoMod loads the banned-word list and caches the configured action when
// automod is enabled. Called once during server startup.
func initAutoMod(cfg *settings.Config) {
	if !cfg.AutoModEnabled {
		return
	}
	path := filepath.Join(settings.ConfigPath, cfg.AutoModWordlist)
	words, err := loadBannedWordsList(path)
	if err != nil {
		logger.LogWarningf("automod: failed to load wordlist %q: %v", path, err)
		return
	}
	setBannedWords(words)
	logger.LogInfof("automod: loaded %d banned word(s) from %q", len(getBannedWords()), path)

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
	if !config.AutoModEnabled || len(getBannedWords()) == 0 {
		return false
	}

	lower := strings.ToLower(msg)
	matched, ok := matchBannedWord(lower)
	if !ok {
		return false
	}

	switch autoModAction {
	case autoModActionKick:
		client.SendSync(&packet.KK{Reason: "Kicked for prohibited language."})
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
		client.SendSync(&packet.KB{Reason: fmt.Sprintf("Banned for prohibited language.\nUntil: ∞\nID: %d", id)})
		client.conn.Close()
		logger.LogInfof("automod: permanently banned %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return true
	}
}

// startTormentDisconnect silently drops the connection of a tormented client
// after an unpredictable delay (8 s–5 min). No packet is sent before closing
// so the client sees a plain connection drop rather than a kick or error message.
// Launched as a goroutine whenever a tormented IPID connects. Torture continues
// on reconnect with escalating delays.
func startTormentDisconnect(client *Client) {
	// Unpredictable initial delay (8 s to 5 min).
	// Use longer window than before for more sustained torment.
	delay := time.Duration(8+tormentIntn(292)) * time.Second
	time.Sleep(delay)

	// Re-check that the IPID is still tormented before disconnecting so that
	// /unlag (or /untorment) can cancel pending timers by removing the IPID.
	if !isIPIDTormented(client.Ipid()) {
		return
	}

	// Hidden quirk: 1/3 chance to extend the torture by scheduling a secondary
	// disconnect 20-60 seconds after the first. If they manage to quickly reconnect,
	// they'll get nuked again before they realize what's happening.
	if tormentIntn(3) != 0 {
		secondaryDelay := time.Duration(20+tormentIntn(40)) * time.Second
		time.AfterFunc(secondaryDelay, func() {
			if isIPIDTormented(client.Ipid()) {
				// Attempt to disconnect any active session under this IPID.
				for _, c := range getClientsByIpid(client.Ipid()) {
					if c != nil {
						c.conn.Close()
					}
				}
			}
		})
	}

	// Close the underlying connection directly — no prior packet — so the
	// disconnect appears as natural causes rather than a visible kick.
	client.conn.Close()
}

// handleTormentedIC intercepts an IC message from a tormented client.
// The message is always echoed back to the sender immediately so it appears
// to have been sent successfully.  With ~50% probability the message is a
// ghost — silently dropped for everyone else and never logged.  Otherwise the
// message is delivered to the rest of the area and logged after a variable
// delay (10-35 seconds), making conversation effectively impossible.
// Hidden quirks: rare character name corruption, occasional duplication,
// and subtle timing inconsistencies make the punishment unobvious.
//
// time.AfterFunc is used instead of a goroutine+sleep so no goroutine stack is
// parked during the wait; the callback runs in a fresh goroutine only when the
// timer fires.
func handleTormentedIC(client *Client, ms *packet.MSPacket) {
	// Encode once into wire-format args via the Outgoing contract; reused
	// for both the immediate echo and the deferred broadcast.
	header, args := ms.Header(), ms.Args()

	// Echo to sender immediately so it looks like it went through.
	client.SendPacket(header, args...)

	if tormentIntn(2) == 0 {
		// Ghost: 50% chance — nobody else sees it, nothing is logged.
		return
	}

	// Capture state at dispatch time so the callback is unaffected by later
	// area changes or client disconnects.
	targetArea := client.Area()
	senderUID := client.Uid()
	msgLabel := ms.Message

	// Variable delay (10-35 seconds) adds unpredictability.
	delay := time.Duration(10+tormentIntn(25)) * time.Second

	time.AfterFunc(delay, func() {
		// Deliver to everyone currently in the original area except the sender.
		clients.ForEach(func(c *Client) {
			if c.Area() == targetArea && c.Uid() != senderUID {
				c.SendPacket(header, args...)
			}
		})
		addToBuffer(client, "IC", "\""+msgLabel+"\"", false)
	})

	// Hidden quirk: 1/25 chance of duplicate delivery (message sent twice with different delays).
	if tormentIntn(25) == 0 {
		dupe := time.Duration(35+tormentIntn(20)) * time.Second
		time.AfterFunc(dupe, func() {
			clients.ForEach(func(c *Client) {
				if c.Area() == targetArea && c.Uid() != senderUID {
					c.SendPacket(header, args...)
				}
			})
		})
	}
}

// handleTormentedOOC applies the same ghost-or-delay logic as handleTormentedIC
// for OOC (CT) messages from a tormented client. 50% ghost rate, variable delays,
// and rare quirks like character name corruption keep it subtle.
func handleTormentedOOC(client *Client, name, msg string) {
	// Hidden quirk: 1/30 chance to corrupt the sender's displayed name slightly.
	displayName := name
	if tormentIntn(30) == 0 && len(name) > 2 {
		runes := []rune(name)
		i := tormentIntn(len(runes))
		runes[i] = runes[i] + rune(1+tormentIntn(2)) // subtle ASCII shift
		displayName = string(runes)
	}

	out := &packet.CTToClient{Name: displayName, Message: msg, IsFromServer: "0"}
	// Echo to sender immediately.
	client.Send(out)

	if tormentIntn(2) == 0 {
		// Ghost: 50% chance — silently dropped.
		return
	}

	targetArea := client.Area()
	senderUID := client.Uid()
	header, args := out.Header(), out.Args()

	// Variable delay (8-40 seconds).
	delay := time.Duration(8+tormentIntn(32)) * time.Second

	time.AfterFunc(delay, func() {
		clients.ForEach(func(c *Client) {
			if c.Area() == targetArea && c.Uid() != senderUID {
				c.SendPacket(header, args...)
			}
		})
		addToBuffer(client, "OOC", "\""+msg+"\"", false)
	})

	// Hidden quirk: 1/20 chance the message is delivered twice (race condition illusion).
	if tormentIntn(20) == 0 {
		dupe := time.Duration(40+tormentIntn(25)) * time.Second
		time.AfterFunc(dupe, func() {
			clients.ForEach(func(c *Client) {
				if c.Area() == targetArea && c.Uid() != senderUID {
					c.SendPacket(header, args...)
				}
			})
		})
	}
}

// matchBannedWord performs a case-insensitive substring search of s against
// every entry in bannedWords. Substring matching catches evasion attempts such
// as embedded punctuation or spacing variants. Returns the matched word and
// true on the first hit, or ("", false) if no match is found.
func matchBannedWord(s string) (string, bool) {
	for _, word := range getBannedWords() {
		if strings.Contains(s, word) {
			return word, true
		}
	}
	return "", false
}
