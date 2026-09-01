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
	"sync/atomic"
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

// wordEntriesPtr holds the tiered banned-word list (loaded from
// automod_wordlist via loadWordListEntries) behind its own atomic.Pointer, on
// the same lock-free swap pattern livereload.go uses for bannedWordsPtr and
// friends -- but kept as a separate variable here rather than added to
// livereload.go, since that file's atomic-pointer set is owned elsewhere in
// this change. Read it via getWordEntries(); publish via setWordEntries().
var wordEntriesPtr atomic.Pointer[[]WordEntry]

func getWordEntries() []WordEntry {
	if v := wordEntriesPtr.Load(); v != nil {
		return *v
	}
	return nil
}

func setWordEntries(e []WordEntry) { wordEntriesPtr.Store(&e) }

// minNormalizedEntryLen is the shortest normalizeForFilter output
// loadWordListFile will accept into bannedWords/censoredNames. Below this,
// a substring match is either unconditional (an empty needle) or broad
// enough to fire on huge swaths of ordinary chat, and it's never what an
// admin actually meant to block (see loadWordListFile). 4 is a floor, not a
// guarantee: even a 4-letter entry can collide with common words (see
// commonWordCollisions), which is checked separately.
const minNormalizedEntryLen = 4

// readWordListLines opens path and returns every non-blank, non-comment
// line, trimmed, in file order ('#'-prefixed lines are comments). Shared by
// loadWordListFile and loadWordListEntries so the two parsers can never
// disagree about which lines in a word-list file count as content -- each
// applies its own per-line interpretation on top of the same raw lines.
func readWordListLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// loadWordListFile reads a plain wordlist file at the given path and returns
// the parsed entries. Each non-empty, non-comment line is treated as one
// entry, run through normalizeForFilter so it matches on the same terms as
// the text being checked (case-insensitive, Unicode-confusable-insensitive).
// Lines starting with '#' are treated as comments and ignored. Duplicates are
// removed and the list is sorted by entry length ascending so that a
// substring scanner that returns on the first hit (e.g. matchBannedWord,
// matchCensoredName) short-circuits as early as possible. Shared by the
// automod banned-word list and the censored-showname list.
//
// This is the flat, whole-line, no-severity reading of a word-list file --
// unaware of the '|'-separated tier/mode syntax buildWordEntries understands.
// It is kept exactly as it always behaved (a line is one literal entry,
// pipe characters and all) because censored_names.txt and
// punishment_names.txt still load through it and have no reason to grow
// tier syntax; only automod_wordlist has moved to the tiered reader
// (loadWordListEntries) in initAutoMod below. See getWordEntries/
// autoModCheckTiered for how the two lists now combine for banned_words.txt
// specifically.
func loadWordListFile(path string) ([]string, error) {
	lines, err := readWordListLines(path)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, line := range lines {
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

	words := make([]string, 0, len(seen))
	for w := range seen {
		words = append(words, w)
	}
	// Shorter needles are checked first; matchBannedWord exits on first match,
	// so a short word match skips all remaining (longer) pattern checks.
	sort.Slice(words, func(i, j int) bool { return len(words[i]) < len(words[j]) })
	return words, nil
}

// buildWordEntries parses word-list lines (as returned by readWordListLines --
// already trimmed, with blanks and '#' comments removed) into tiered
// WordEntry values. path is used only for log messages.
//
// Line syntax, fully backward compatible with the flat list
// loadWordListFile has always read -- an unmarked line behaves exactly as it
// always did, becoming a SeverityDefault/MatchSubstring entry:
//
//	badword                    -> SeverityDefault, MatchSubstring
//	tranny | nuke              -> SeverityNuke,    MatchSubstring
//	rape | severe | word       -> SeveritySevere,  MatchWord
//	mildthing | watch          -> SeverityWatch,   MatchSubstring
//
// Fields are '|'-separated and trimmed; a line may have 1, 2 or 3 fields.
// More than 3 is a warning and the whole line is skipped, same as any other
// malformed entry below.
//
// An unparseable tier or match mode is a warning naming the offending entry,
// and the ENTIRE entry is skipped -- never silently downgraded to a weaker
// tier or the default mode. Getting e.g. "nuek" treated as "default" would
// leave an operator believing a word is banned outright when in fact it is
// only mildly acted on (or, if automod is disabled, not acted on at all) --
// that is a worse failure than the entry simply not loading, because a typo
// that fails loudly gets fixed and a typo that fails quietly does not.
func buildWordEntries(lines []string, path string) []WordEntry {
	type key struct {
		pattern string
		mode    MatchMode
	}
	// best holds the highest-severity entry seen so far for each (pattern,
	// mode) pair; order preserves first-sight order so the output is stable
	// (aside from the final severity sort) across repeated loads of an
	// unchanged file.
	best := make(map[key]WordEntry)
	order := make([]key, 0, len(lines))

	for _, line := range lines {
		fields := strings.Split(line, "|")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if len(fields) > 3 {
			logger.LogWarningf("%s: entry %q has more than 3 '|'-separated fields (word | tier | mode) and was skipped", path, line)
			continue
		}
		raw := fields[0]

		severity := SeverityDefault
		if len(fields) >= 2 {
			sev, ok := parseWordSeverity(fields[1])
			if !ok {
				logger.LogWarningf("%s: entry %q has an unrecognized severity tier %q and was skipped -- a mistyped tier is never guessed at, since silently falling back to a weaker tier could leave an operator believing a word is blocked harder than it actually is", path, line, fields[1])
				continue
			}
			severity = sev
		}

		mode := MatchSubstring
		if len(fields) >= 3 {
			m, ok := parseMatchMode(fields[2])
			if !ok {
				logger.LogWarningf("%s: entry %q has an unrecognized match mode %q and was skipped", path, line, fields[2])
				continue
			}
			mode = m
		}

		var pattern string
		switch mode {
		case MatchWord:
			// normalizeForFilterBoundaries keeps word boundaries (as single
			// spaces) instead of collapsing them away, which is what lets
			// matchesWordBoundary tell "rape" (the whole word, and its
			// stem-preserving inflections) apart from "rape" as a substring
			// of "therapeutic". That is the entire reason this mode exists,
			// and it is also why it deliberately skips
			// collidesWithCommonWords below: that gate protects substring
			// matching from firing INSIDE an unrelated word, and a boundary
			// match structurally cannot do that -- "rape" in word mode
			// matches "rape"/"rapes"/"raped", never the middle of
			// "therapeutic". Today, without this mode, "rape" is silently
			// rejected by the substring gate for exactly that collision.
			pattern = normalizeForFilterBoundaries(raw)
			if strings.Contains(pattern, " ") {
				// A multi-word pattern can never match via matchesWordBoundary,
				// which only ever compares a pattern against ONE token at a
				// time (prefix-of-a-word) -- so an entry that normalizes to
				// more than one token would silently never fire. Reject it
				// loudly instead of loading a dead entry.
				logger.LogWarningf("%s: entry %q normalized to %q, which is more than one word -- word-mode matching can only prefix-match a single token and a multi-word pattern would never match anything; skipped", path, line, pattern)
				continue
			}
			if n := utf8.RuneCountInString(pattern); n < minWordModeEntryLen {
				if n == 0 {
					logger.LogWarningf("%s: entry %q has no letters after normalization and was skipped (use '#' to comment out dividers)", path, line)
				} else {
					logger.LogWarningf("%s: entry %q normalized to %q (too short for word mode, min %d letters) and was skipped", path, line, pattern, minWordModeEntryLen)
				}
				continue
			}
		default:
			pattern = normalizeForFilter(raw)
			if n := utf8.RuneCountInString(pattern); n < minNormalizedEntryLen {
				if n == 0 {
					logger.LogWarningf("%s: entry %q has no letters after normalization and was skipped (use '#' to comment out dividers)", path, line)
				} else {
					logger.LogWarningf("%s: entry %q normalized to %q (too short to use safely, min %d letters) and was skipped", path, line, pattern, minNormalizedEntryLen)
				}
				continue
			}
			if hits := collidesWithCommonWords(pattern); len(hits) > 0 {
				logger.LogWarningf("%s: entry %q normalized to %q, which also matches common word(s) %v — skipped to avoid false positives on ordinary chat; use 'word' mode instead if this entry needs to stay this generic", path, line, pattern, hits)
				continue
			}
		}

		k := key{pattern: pattern, mode: mode}
		entry := WordEntry{Raw: raw, Pattern: pattern, Severity: severity, Mode: mode}
		if existing, ok := best[k]; !ok {
			best[k] = entry
			order = append(order, k)
		} else if severity > existing.Severity {
			// Same pattern listed twice at different tiers: the worse tier
			// wins, since matchWordEntries already resolves a message's
			// verdict by worst-match -- a duplicate can only ever make that
			// verdict more accurate, never less, by keeping the harsher one.
			best[k] = entry
		}
	}

	entries := make([]WordEntry, 0, len(order))
	for _, k := range order {
		entries = append(entries, best[k])
	}

	// Severity descending, then pattern length ascending within a tier.
	// matchWordEntries relies on this ordering to short-circuit the instant
	// it finds a nuke -- see its doc comment.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Severity != entries[j].Severity {
			return entries[i].Severity > entries[j].Severity
		}
		return len(entries[i].Pattern) < len(entries[j].Pattern)
	})
	return entries
}

// loadWordListEntries reads a word list file and returns tiered entries. See
// buildWordEntries for the line syntax and every validation rule applied.
func loadWordListEntries(path string) ([]WordEntry, error) {
	lines, err := readWordListLines(path)
	if err != nil {
		return nil, err
	}
	return buildWordEntries(lines, path), nil
}

// initAutoMod loads the banned-word list and caches the configured action.
// Called once during server startup. The word list itself is loaded
// regardless of automod_enabled — like censored_names.txt and
// punishment_names.txt, other features (e.g. the giveaway item filter) match
// against it independently of whether automod's IC/OOC enforcement is on.
// autoModCheck still gates its own enforcement on cfg.AutoModEnabled, so
// leaving automod disabled continues to leave IC/OOC/showname checks off.
func initAutoMod(cfg *settings.Config) {
	path := filepath.Join(settings.ConfigPath, cfg.AutoModWordlist)
	words, err := loadWordListFile(path)
	if err != nil {
		logger.LogWarningf("automod: failed to load wordlist %q: %v", path, err)
		return
	}
	setBannedWords(words)
	logger.LogInfof("automod: loaded %d banned word(s) from %q", len(getBannedWords()), path)

	// The tiered read of the same file, in parallel with the flat one above.
	// Loaded independently (a failure here doesn't affect the flat list, and
	// vice versa) and, like the flat list, regardless of automod_enabled --
	// the raid guard's word-hit scoring (raidGuardOnWordHit) and the nuke
	// tier's deterministic ban both want severity information whether or not
	// automod's own configured action is toggled on. See
	// autoModCheckTiered/effectiveWordEntries for how the two lists combine.
	if entries, eerr := loadWordListEntries(path); eerr == nil {
		setWordEntries(entries)
		logger.LogInfof("automod: loaded %d tiered banned-word entry/entries from %q", len(entries), path)
	} else {
		logger.LogWarningf("automod: failed to load tiered wordlist %q: %v", path, eerr)
	}

	if !cfg.AutoModEnabled {
		return
	}

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
//
// The second return value, kickAfter, is true only for the shadow and torment
// actions, which (unlike kick/mute/ban) don't otherwise give the offender any
// immediate consequence -- without a kick, nothing tells them their message
// failed, so a determined troll can keep hammering slurs indefinitely. The
// caller MUST call client.KickForCensorTrip() when kickAfter is true, but only
// once it is done using the connection for this packet (e.g. after echoing a
// shadow trip back to the sender) -- KickForCensorTrip closes the connection.
//
// This check runs whenever a banned-word list is loaded, regardless of
// automod_enabled — like censored_names.txt and the giveaway item filter, the
// word filter itself is not gated behind the automod toggle, only the
// wordlist-driven action is (autoModAction defaults to the safe shadow-drop
// behavior, autoModActionShadow, when automod is disabled — see initAutoMod).
//
// A thin wrapper around autoModCheckTiered, kept at this signature (no
// severity in the return) for callers that only ever need the plain
// pass/blocked/shadow verdict and have no use for the matched entry itself.
// The two can never drift because this one no longer has a matching
// implementation of its own -- see autoModCheckTiered for the actual logic.
func autoModCheck(client *Client, msg string, source string) (result autoModResult, kickAfter bool) {
	_, result, kickAfter = autoModCheckTiered(client, msg, source)
	return result, kickAfter
}

// effectiveWordEntries returns the entries autoModCheckTiered matches
// against: the tiered list built by loadWordListEntries (wordEntriesPtr,
// set in initAutoMod) UNION the legacy flat bannedWords list (bannedWordsPtr
// in livereload.go), with each legacy entry wrapped as an implicit
// SeverityDefault/MatchSubstring WordEntry.
//
// The union, rather than a straight swap over to the tiered list alone, is
// what keeps this backward compatible with every existing caller and test
// that only ever populates bannedWords via setBannedWords (giveaway's item
// filter reads it directly too, independently of this function) -- and in
// production the two lists are loaded from the very same file by initAutoMod,
// so the only entries the wrap adds beyond what loadWordListEntries already
// covers are harmless: an unmarked line produces the identical
// SeverityDefault/MatchSubstring entry either way, and a tiered line (e.g.
// "tranny | nuke") produces an inert, gibberish whole-line-normalized string
// via the flat loader ("trannynuke") that no real message will ever contain,
// sitting alongside the real "tranny" nuke entry the tiered loader produced
// for the same line.
//
// Built fresh on every call rather than cached, since either source list can
// be swapped out from under it at any time (by /reload, or by a test); the
// allocation is one small slice for what is typically a short list, dwarfed
// by the normalization passes matchWordEntries itself already performs.
func effectiveWordEntries() []WordEntry {
	tiered := getWordEntries()
	legacy := getBannedWords()
	if len(legacy) == 0 {
		return tiered
	}
	entries := make([]WordEntry, 0, len(tiered)+len(legacy))
	entries = append(entries, tiered...)
	for _, w := range legacy {
		if w == "" {
			continue
		}
		entries = append(entries, WordEntry{Raw: w, Pattern: w, Severity: SeverityDefault, Mode: MatchSubstring})
	}
	return entries
}

// autoModCheckTiered is autoModCheck plus the severity of what matched (the
// zero WordListMatch, Matched == false, when nothing did). source labels
// which field tripped (e.g. "IC message", "OOC username") for the staff
// alert and logs, exactly as in autoModCheck.
//
// For SeverityWatch, SeverityDefault and SeveritySevere this behaves EXACTLY
// as autoModCheck always has -- same result, same kickAfter, same staff
// [CENSOR] alert, same torment-listing -- with one deliberate exception:
// SeverityWatch always returns autoModPass (delivered normally) rather than
// whatever autoModAction says. Watch-tier words exist purely to score a
// connection for the raid guard and to keep staff informed; they are never,
// by themselves, supposed to be punished, which is the whole difference
// between "watch" and "default".
//
// For SeverityNuke this returns the match with autoModBlocked and
// kickAfter=false, and takes NO other action of its own -- no shadow-send,
// no torment-listing, no ban, no staff alert from this function. The caller
// (checkCensored in netprotocol.go, via applyAutoModNuke in
// automod_nuke.go) does everything for that tier: destroy the message before
// anyone including the sender sees it, and ban the IPID. A nuke's
// consequence is heavier than anything the configured automod_action switch
// below produces and deliberately doesn't route through it.
func autoModCheckTiered(client *Client, msg, source string) (WordListMatch, autoModResult, bool) {
	entries := effectiveWordEntries()
	if len(entries) == 0 {
		return WordListMatch{}, autoModPass, false
	}

	m := matchWordEntries(entries, msg)
	if !m.Matched {
		return WordListMatch{}, autoModPass, false
	}

	if m.Entry.Severity == SeverityNuke {
		return m, autoModBlocked, false
	}

	// The matched entry as the operator wrote it, plus its tier and mode, for
	// the staff [CENSOR] alert and the logs below -- e.g. `tranny (nuke/substring)`
	// (WordEntry.String()). This replaces the flat `matched` string the old
	// autoModCheck logged, which never carried tier/mode information.
	matched := m.Entry.String()

	if m.Entry.Severity == SeverityWatch {
		alertCensorTrip(client, source, matched, msg, "Watch tier: delivered normally, staff notified. Not punished on its own.")
		logger.LogInfof("automod: watch-tier match for %v (uid %d) — matched %s", client.Ipid(), client.Uid(), matched)
		return m, autoModPass, false
	}

	switch autoModAction {
	case autoModActionKick:
		client.SendSync(&packet.KK{Reason: "Kicked for prohibited language."})
		client.conn.Close()
		alertCensorTrip(client, source, matched, msg, "They were kicked.")
		logger.LogInfof("automod: kicked %v (uid %d) — matched %s", client.Ipid(), client.Uid(), matched)
		return m, autoModBlocked, false

	case autoModActionMute:
		// expires = 0 means permanent in the PUNISHMENTS table.
		if err := db.UpsertMute(client.Ipid(), int(ICOOCMuted), 0); err != nil {
			logger.LogErrorf("automod: failed to mute %v: %v", client.Ipid(), err)
			return m, autoModPass, false
		}
		client.SetMuted(ICOOCMuted)
		client.SetUnmuteTime(time.Time{}) // zero = permanent
		client.SendServerMessage("You have been muted for prohibited language.")
		alertCensorTrip(client, source, matched, msg, "They were permanently muted.")
		logger.LogInfof("automod: permanently muted %v (uid %d) — matched %s", client.Ipid(), client.Uid(), matched)
		return m, autoModBlocked, false

	case autoModActionTorment:
		addCensorTripToTormentList(client)
		alertCensorTrip(client, source, matched, msg, "The message was dropped, they were added to the torment list, and they were kicked.")
		logger.LogInfof("automod: added %v (uid %d) to torment list and kicked — matched %s", client.Ipid(), client.Uid(), matched)
		return m, autoModBlocked, true

	case autoModActionBan:
		banTime := time.Now().UTC().Unix()
		id, err := db.AddBan(client.Ipid(), client.Hdid(), banTime, -1, "Automatic ban: prohibited language", "Server")
		if err != nil {
			logger.LogErrorf("automod: failed to ban %v: %v", client.Ipid(), err)
			return m, autoModPass, false
		}
		forgetIP(client.Ipid())
		client.SendSync(&packet.KB{Reason: fmt.Sprintf("Banned for prohibited language.\nUntil: ∞\nID: %d", id)})
		client.conn.Close()
		alertCensorTrip(client, source, matched, msg, "They were permanently banned.")
		logger.LogInfof("automod: permanently banned %v (uid %d) — matched %s", client.Ipid(), client.Uid(), matched)
		return m, autoModBlocked, false

	default: // autoModActionShadow
		addCensorTripToTormentList(client)
		alertCensorTrip(client, source, matched, msg, "The message was shadow-dropped (only they can see it), they were added to the torment list, and they were kicked.")
		logger.LogInfof("automod: shadow-dropped %s from %v (uid %d) and kicked — matched %s", source, client.Ipid(), client.Uid(), matched)
		return m, autoModShadow, true
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
