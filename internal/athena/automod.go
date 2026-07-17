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
	"unicode/utf8"

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
	autoModActionShadow autoModActionKind = iota // default
	autoModActionBan
	autoModActionKick
	autoModActionMute
	autoModActionTorment
)

// autoModResult is what autoModCheck reports back to the packet handlers.
type autoModResult int

const (
	autoModPass    autoModResult = iota // no banned word — continue normally
	autoModBlocked                      // matched; handled destructively (ban/kick/mute/torment) — abort processing
	autoModShadow                       // matched; shadow-send — echo the message to the sender only, drop it for everyone else
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

// The normalized banned-word list lives behind an atomic.Pointer (bannedWordsPtr
// in livereload.go) so that /reload can swap it at runtime without racing the
// per-message reader. Read it via getBannedWords(); publish via setBannedWords().
// It is stored as a slice for O(n) substring scan; lists are typically small so
// the overhead of a full scan per message is negligible compared to network I/O.

// minNormalizedEntryLen is the shortest normalizeForFilter output
// loadWordListFile will accept into bannedWords/censoredNames. Below this,
// a substring match is either unconditional (an empty needle) or broad
// enough to fire on huge swaths of ordinary chat, and it's never what an
// admin actually meant to block (see loadWordListFile). 4 is a floor, not a
// guarantee: even a 4-letter entry can collide with common words (see
// commonWordCollisions), which is checked separately.
const minNormalizedEntryLen = 4

// loadWordListFile reads a plain wordlist file at the given path and returns
// the parsed entries. Each non-empty, non-comment line is treated as one
// entry, run through normalizeForFilter so it matches on the same terms as
// the text being checked (case-insensitive, Unicode-confusable-insensitive).
// Lines starting with '#' are treated as comments and ignored. Duplicates are
// removed and the list is sorted by entry length ascending so that a
// substring scanner that returns on the first hit (e.g. matchBannedWord,
// matchCensoredName) short-circuits as early as possible. Shared by the
// automod banned-word list and the censored-showname list.
func loadWordListFile(path string) ([]string, error) {
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
		normalized := normalizeForFilter(line)
		if n := utf8.RuneCountInString(normalized); n < minNormalizedEntryLen {
			// A word list entry that is mostly digits/punctuation can collapse
			// to something far shorter than it looks: e.g. "l36" normalizes to
			// "le" once the digit-drop and leetspeak substitution both apply.
			// A 0-2 character needle is either a substring of literally every
			// message (empty) or of a huge fraction of ordinary chat ("le"
			// alone matches "hello", "please", "level", ...), so entries this
			// short are always a filter-evasion own-goal, never intentional —
			// skip them and tell the admin so a dead/dangerous entry doesn't
			// silently do nothing (or too much).
			if n == 0 {
				logger.LogWarningf("%s: entry %q has no letters after normalization and was skipped (use '#' to comment out dividers)", path, line)
			} else {
				logger.LogWarningf("%s: entry %q normalized to %q (too short to use safely, min %d letters) and was skipped", path, line, normalized, minNormalizedEntryLen)
			}
			continue
		}
		if hits := collidesWithCommonWords(normalized); len(hits) > 0 {
			// Length alone doesn't guarantee safety: e.g. "tron" (4 letters,
			// clears minNormalizedEntryLen) still matches "electronic",
			// "strong", "astronomy", ... Reject rather than warn-and-load,
			// since letting an entry like this through is exactly the kind
			// of automod-fires-on-everyone incident this whole check exists
			// to prevent, and the admin can always rephrase the entry to be
			// more specific (e.g. keep more of the original spelling).
			logger.LogWarningf("%s: entry %q normalized to %q, which also matches common word(s) %v — skipped to avoid false positives on ordinary chat", path, line, normalized, hits)
			continue
		}
		seen[normalized] = struct{}{}
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
	words, err := loadWordListFile(path)
	if err != nil {
		logger.LogWarningf("automod: failed to load wordlist %q: %v", path, err)
		return
	}
	setBannedWords(words)
	logger.LogInfof("automod: loaded %d banned word(s) from %q", len(getBannedWords()), path)

	// Parse the action once so the hot path never allocates.
	switch strings.ToLower(strings.TrimSpace(cfg.AutoModAction)) {
	case "ban":
		autoModAction = autoModActionBan
	case "kick":
		autoModAction = autoModActionKick
	case "mute":
		autoModAction = autoModActionMute
	case "torment":
		autoModAction = autoModActionTorment
	default:
		// "shadow" and anything unset/unrecognized: shadow-send the censored
		// message (sender sees it, room doesn't) and torment-list the speaker.
		autoModAction = autoModActionShadow
	}
}

// autoModCheck tests msg for banned words. If one is found the configured
// action is applied and staff are alerted in OOC. source labels which field
// tripped (e.g. "IC message", "OOC username") for the staff alert and logs.
// The caller acts on the result: autoModBlocked aborts packet processing
// outright, while autoModShadow means the message must be echoed back to the
// sender only — it looks sent on their side but never reaches another client.
func autoModCheck(client *Client, msg string, source string) autoModResult {
	if !config.AutoModEnabled || len(getBannedWords()) == 0 {
		return autoModPass
	}

	normalized := normalizeForFilter(msg)
	matched, ok := matchBannedWord(normalized)
	if !ok {
		return autoModPass
	}

	switch autoModAction {
	case autoModActionKick:
		client.SendSync(&packet.KK{Reason: "Kicked for prohibited language."})
		client.conn.Close()
		alertCensorTrip(client, source, matched, msg, "They were kicked.")
		logger.LogInfof("automod: kicked %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return autoModBlocked

	case autoModActionMute:
		// expires = 0 means permanent in the PUNISHMENTS table.
		if err := db.UpsertMute(client.Ipid(), int(ICOOCMuted), 0); err != nil {
			logger.LogErrorf("automod: failed to mute %v: %v", client.Ipid(), err)
			return autoModPass
		}
		client.SetMuted(ICOOCMuted)
		client.SetUnmuteTime(time.Time{}) // zero = permanent
		client.SendServerMessage("You have been muted for prohibited language.")
		alertCensorTrip(client, source, matched, msg, "They were permanently muted.")
		logger.LogInfof("automod: permanently muted %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return autoModBlocked

	case autoModActionTorment:
		addCensorTripToTormentList(client)
		alertCensorTrip(client, source, matched, msg, "The message was dropped and they were added to the torment list.")
		logger.LogInfof("automod: added %v (uid %d) to torment list — matched word %q", client.Ipid(), client.Uid(), matched)
		return autoModBlocked

	case autoModActionBan:
		banTime := time.Now().UTC().Unix()
		id, err := db.AddBan(client.Ipid(), client.Hdid(), banTime, -1, "Automatic ban: prohibited language", "Server")
		if err != nil {
			logger.LogErrorf("automod: failed to ban %v: %v", client.Ipid(), err)
			return autoModPass
		}
		forgetIP(client.Ipid())
		client.SendSync(&packet.KB{Reason: fmt.Sprintf("Banned for prohibited language.\nUntil: ∞\nID: %d", id)})
		client.conn.Close()
		alertCensorTrip(client, source, matched, msg, "They were permanently banned.")
		logger.LogInfof("automod: permanently banned %v (uid %d) — matched word %q", client.Ipid(), client.Uid(), matched)
		return autoModBlocked

	default: // autoModActionShadow
		addCensorTripToTormentList(client)
		alertCensorTrip(client, source, matched, msg, "The message was shadow-dropped (only they can see it) and they were added to the torment list.")
		logger.LogInfof("automod: shadow-dropped %s from %v (uid %d) — matched word %q", source, client.Ipid(), client.Uid(), matched)
		return autoModShadow
	}
}

// addCensorTripToTormentList puts the offender's IPID on the torment list (if
// it isn't already there) and arms a disconnect timer for every session open
// under it, exactly like /lag. Censor trips are the only torment-list
// additions that alert staff — a moderator adding someone by hand with /lag
// stays silent (see alertCensorTrip's call sites).
func addCensorTripToTormentList(client *Client) {
	if isIPIDTormented(client.Ipid()) {
		return
	}
	addTormentedIP(client.Ipid())
	for _, c := range getClientsByIpid(client.Ipid()) {
		go startTormentDisconnect(c)
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

// matchBannedWord performs a substring search of s (expected to already be
// normalizeForFilter'd) against every entry in bannedWords. Returns the
// matched word and true on the first hit, or ("", false) if no match is found.
//
// An empty entry is skipped rather than matched: strings.Contains treats ""
// as a substring of everything, so a stray empty entry would match every
// message unconditionally. loadWordListFile already keeps empty entries out
// of the list, but this is the actual point of use, so it stays safe even if
// an empty string reaches getBannedWords() through some other path (e.g. a
// test or future caller of setBannedWords).
func matchBannedWord(s string) (string, bool) {
	for _, word := range getBannedWords() {
		if word == "" {
			continue
		}
		if strings.Contains(s, word) {
			return word, true
		}
	}
	return "", false
}
