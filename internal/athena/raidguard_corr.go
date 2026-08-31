// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Raid guard, layer 2: what the server as a whole is seeing.
//
// Layer 1 scores one connection against itself, which is necessary but cannot
// see the shape of a raid. A raid is not one connection behaving badly, it is
// many connections behaving *identically*, and identical-ness is only visible
// from above. Two things are tracked here:
//
//	Content correlation -- the same text arriving from several distinct IPIDs
//	inside a few seconds. In the capture this reached 34% of raid messages and
//	0% of baseline messages once trivially short lines were excluded. It is the
//	single signal a per-IPID limiter is structurally incapable of producing, and
//	it is the precondition the guard requires before it will ever ban anybody.
//
//	Arrival bursts -- new IPIDs per second. The raid put 29 new IPIDs on the box
//	in its first second against a baseline of 1.31/s.
//
// Matching is by token shingle rather than whole-message equality. Raiders vary
// a line slightly between sends -- prepending "/g ", appending another slur --
// and exact matching missed a measured 12 such pairs that shingling catches. A
// shingle is a window of consecutive normalised tokens; two messages sharing
// one share a real phrase, which survives edits at either end.
package athena

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Shingle parameters. shingleSize is the number of consecutive tokens in a
// window; maxShinglesPerMsg bounds the work a single long message can cause,
// which matters because the hot path runs under a flood by definition.
const (
	shingleSize       = 4
	maxShinglesPerMsg = 8
)

// normalizeRaidText folds text to a comparison form: lowercased, with anything
// that is not a letter or digit dropped, and runs of 3+ identical characters
// collapsed to 2.
//
// This is deliberately NOT normalizeForFilter (text_filter_normalize.go), which
// exists to defeat slur obfuscation and throws digits away to do it. Here the
// digits carry meaning -- two raiders posting the same URL or the same number
// is exactly the correlation we want -- so this keeps them.
func normalizeRaidText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var last rune
	var run int
	for _, r := range strings.ToLower(s) {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			last, run = 0, 0
			b.WriteRune(' ')
			continue
		}
		if r == last {
			run++
			if run >= 2 {
				continue
			}
		} else {
			run = 0
		}
		last = r
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// raidFingerprints returns the set of fingerprints identifying a message. A
// message long enough to shingle yields its shingles; a shorter one yields a
// single whole-text fingerprint so short lines still correlate exactly.
//
// Returns nil for anything below minLen normalised characters. That floor is
// load-bearing: without it the baseline capture showed a 57% "duplicate" rate
// that was entirely two players independently typing /gas and /cm. Short
// identical lines are what ordinary players legitimately have in common.
func raidFingerprints(text string, minLen int) []uint64 {
	norm := normalizeRaidText(text)
	if len(norm) < minLen {
		return nil
	}
	tokens := strings.Fields(norm)
	if len(tokens) < shingleSize {
		return []uint64{hashString(norm)}
	}
	n := len(tokens) - shingleSize + 1
	if n > maxShinglesPerMsg {
		n = maxShinglesPerMsg
	}
	out := make([]uint64, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, hashString(strings.Join(tokens[i:i+shingleSize], " ")))
	}
	return out
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// corrEntry is the set of IPIDs seen uttering one fingerprint, and when it was
// last touched so it can be evicted.
type corrEntry struct {
	ipids map[string]struct{}
	last  time.Time
}

// CorrelationWindow maps a content fingerprint to the distinct IPIDs that have
// uttered it recently.
//
// Bounded on purpose: this structure is fed by exactly the traffic it exists to
// detect, so an unbounded map here would turn a raid into an OOM. Entries are
// pruned by age and, if that is not enough, the map is cleared wholesale when
// it exceeds maxEntries -- losing detection state under extreme load is
// acceptable, running the server out of memory is not.
type CorrelationWindow struct {
	mu         sync.Mutex
	entries    map[uint64]*corrEntry
	window     time.Duration
	maxEntries int
	maxIPIDs   int
	lastPrune  time.Time
}

// maxIPIDsPerEntry bounds one fingerprint's IPID set. Once a line has been seen
// from far more IPIDs than any threshold requires, counting further is wasted.
const maxIPIDsPerEntry = 128

func NewCorrelationWindow(window time.Duration, maxEntries int) *CorrelationWindow {
	if window <= 0 {
		window = 10 * time.Second
	}
	if maxEntries <= 0 {
		maxEntries = 4096
	}
	return &CorrelationWindow{
		entries:    make(map[uint64]*corrEntry),
		window:     window,
		maxEntries: maxEntries,
		maxIPIDs:   maxIPIDsPerEntry,
	}
}

// Observe records that ipid uttered fp and returns how many distinct IPIDs have
// uttered it inside the window.
func (w *CorrelationWindow) Observe(fp uint64, ipid string, now time.Time) int {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pruneLocked(now)

	e, ok := w.entries[fp]
	if !ok || now.Sub(e.last) > w.window {
		e = &corrEntry{ipids: make(map[string]struct{}, 4)}
		w.entries[fp] = e
	}
	e.last = now
	if len(e.ipids) < w.maxIPIDs {
		e.ipids[ipid] = struct{}{}
	}
	return len(e.ipids)
}

// pruneLocked drops expired entries. Rate-limited to once per window/4 so a
// flood does not pay a full map scan on every single message.
func (w *CorrelationWindow) pruneLocked(now time.Time) {
	if len(w.entries) > w.maxEntries {
		w.entries = make(map[uint64]*corrEntry)
		w.lastPrune = now
		return
	}
	if now.Sub(w.lastPrune) < w.window/4 {
		return
	}
	w.lastPrune = now
	for k, e := range w.entries {
		if now.Sub(e.last) > w.window {
			delete(w.entries, k)
		}
	}
}

// Prune drops expired entries unconditionally. Exposed for tests.
func (w *CorrelationWindow) Prune(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastPrune = time.Time{}
	w.pruneLocked(now)
}

// Len reports the number of tracked fingerprints. Exposed for tests.
func (w *CorrelationWindow) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.entries)
}

// arrivalWindow counts distinct new IPIDs over a sliding window.
type arrivalWindow struct {
	mu      sync.Mutex
	stamps  []time.Time
	window  time.Duration
	maxKeep int
}

func newArrivalWindow(window time.Duration) *arrivalWindow {
	if window <= 0 {
		window = time.Second
	}
	return &arrivalWindow{window: window, maxKeep: 4096}
}

// Observe records an arrival and returns how many happened inside the window.
func (a *arrivalWindow) Observe(now time.Time) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	cut := now.Add(-a.window)
	i := 0
	for ; i < len(a.stamps); i++ {
		if a.stamps[i].After(cut) {
			break
		}
	}
	a.stamps = a.stamps[i:]
	if len(a.stamps) < a.maxKeep {
		a.stamps = append(a.stamps, now)
	}
	return len(a.stamps)
}

// Package-level layer-2 state.
var (
	raidCorrWindow  *CorrelationWindow
	raidArrivals    = newArrivalWindow(time.Second)
	raidCorrOnce    sync.Once
	raidAttackUntil atomic.Int64 // unix nanos; server is "under attack" until then
)

// raidAttackHold is how long a detected raid keeps the server flagged as under
// attack after the last piece of evidence. Long enough that the flag does not
// flicker between bursts, short enough that a finished raid stops mattering.
const raidAttackHold = 2 * time.Minute

func raidCorr() *CorrelationWindow {
	raidCorrOnce.Do(func() {
		w := 10 * time.Second
		max := 4096
		if config != nil {
			if config.RaidGuardCorrWindow > 0 {
				w = time.Duration(config.RaidGuardCorrWindow) * time.Second
			}
			if config.RaidGuardCorrMaxEntries > 0 {
				max = config.RaidGuardCorrMaxEntries
			}
		}
		raidCorrWindow = NewCorrelationWindow(w, max)
	})
	return raidCorrWindow
}

// markRaidAttack flags the server as under coordinated attack, and optionally
// closes the door behind it.
//
// raid_guard_auto_lockdown (off by default) engages the existing server
// lockdown on the first detection, so a fan-out stops growing while staff are
// still reading the alert. It deliberately does NOT run
// purgeLockdownFloodClients the way /lockdown on does: the purge disconnects
// every connected player under the playtime threshold, which during a raid
// includes any genuine newcomer who happened to join this evening. The join
// gate alone costs a real player a delayed connection; the purge costs them
// their session, and nothing here is confident enough to spend that
// automatically. A moderator who wants the purge still runs /lockdown on.
func markRaidAttack(now time.Time) {
	raidAttackUntil.Store(now.Add(raidAttackHold).UnixNano())

	if config == nil || !config.RaidGuardAutoLockdown {
		return
	}
	if !serverLockdown.CompareAndSwap(false, true) {
		return // already locked down; nothing to announce
	}
	logger.LogInfof("Raid guard: coordinated raid detected -- lockdown engaged automatically")
	logger.WriteAudit(fmt.Sprintf("%v | RAID_GUARD_LOCKDOWN | automatic lockdown on raid detection",
		now.UTC().Format("15:04:05")))
	alertRaidLockdown()
}

// alertRaidLockdown tells staff the guard locked the server down on its own,
// and how to undo it.
func alertRaidLockdown() {
	msg := "Raid guard detected a coordinated raid and engaged lockdown automatically: new IPIDs cannot " +
		"join until it is lifted. Connected players were NOT purged. Run /lockdown off to lift it, or " +
		"/lockdown on if you also want the playtime purge."
	out := &packet.CTToClient{Name: "[RAIDGUARD]", Message: encode(msg), IsFromServer: "1"}
	clients.ForEach(func(c *Client) {
		if !permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			return
		}
		c.Send(out)
	})
}

// raidGuardUnderAttack reports whether the server is currently seeing evidence
// of a coordinated raid rather than one odd connection.
//
// This is the precondition on banning. See raidGuardEnforce: no accumulation of
// single-connection signals can ban anybody while this is false, so a lone
// unusual player is structurally unbannable by this system no matter how they
// behave.
func raidGuardUnderAttack() bool {
	return time.Now().UnixNano() < raidAttackUntil.Load()
}

// raidObserveContent folds one message into the correlation window and reports
// whether it is corroborated across enough distinct IPIDs to count as
// coordinated. Any single fingerprint reaching the threshold is enough.
func raidObserveContent(ipid, text string, now time.Time) bool {
	minLen := 15
	need := 4
	if config != nil {
		if config.RaidGuardCorrMinLen > 0 {
			minLen = config.RaidGuardCorrMinLen
		}
		if config.RaidGuardCorrIPIDs > 0 {
			need = config.RaidGuardCorrIPIDs
		}
	}
	fps := raidFingerprints(text, minLen)
	if len(fps) == 0 {
		return false
	}
	w := raidCorr()
	hit := false
	for _, fp := range fps {
		if w.Observe(fp, ipid, now) >= need {
			hit = true
		}
	}
	if hit {
		markRaidAttack(now)
	}
	return hit
}

// raidObserveArrival records a newly-seen IPID and reports whether arrivals are
// bursting hard enough to look like a raid rather than a busy evening.
func raidObserveArrival(now time.Time) bool {
	burst := 12
	if config != nil && config.RaidGuardArrivalBurst > 0 {
		burst = config.RaidGuardArrivalBurst
	}
	if raidArrivals.Observe(now) >= burst {
		markRaidAttack(now)
		return true
	}
	return false
}

// resetRaidGuardState clears all layer-2 state. Tests only.
func resetRaidGuardState() {
	raidCorrOnce = sync.Once{}
	raidCorrWindow = nil
	raidArrivals = newArrivalWindow(time.Second)
	raidAttackUntil.Store(0)
}
