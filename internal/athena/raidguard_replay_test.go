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

// Raid guard validation harness.
//
// This file replays two real packet captures -- a genuine raid and a clean
// evening of ordinary play -- through the raid guard's scoring engine
// (raidguard.go / raidguard_corr.go) exactly as pktIC/pktOOC would drive it,
// and checks the one property the whole system exists for: it must catch the
// raid, and it must NEVER act on a legitimate player. Of the two, the second
// is load-bearing -- a raid guard that occasionally bans a real player is
// worse than no raid guard at all, so a false positive anywhere in the normal
// capture fails this test loudly rather than being weakened away.
//
// The harness is hermetic: no network, no database, no *Client. It parses the
// RECV lines of a log ("[ISO8601] RECV|SEND | IPID:<b64> | HDID:<b64> | <raw
// AO2 packet>") into one ordered event stream per IPID, in true wall-clock
// order *across* IPIDs (not IPID-by-IPID), because the layer-2 correlation
// window's notion of "recent" is driven by the timestamps it's fed and would
// be corrupted by replaying one connection's whole history before another's.
//
// Only RECV lines carry authorship -- SEND lines log the recipient, not the
// sender, and are ignored entirely.
package athena

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// ---------------------------------------------------------------------------
// Log parsing
// ---------------------------------------------------------------------------

// recvLineRE splits one log line into its timestamp, direction, IPID, HDID
// and raw AO2 packet body. Only lines matching RECV are of interest; SEND
// lines (and any other line, such as a warning banner) simply fail to match
// and are skipped.
var recvLineRE = regexp.MustCompile(`^\[([^\]]+)\] RECV \| IPID:([^ ]*) \| HDID:([^ ]*) \| (.*)$`)

// recvEvent is one client->server packet, attributed to the IPID that sent it.
type recvEvent struct {
	ts   time.Time
	ipid string
	raw  string // raw AO2 packet, '#'-delimited, still AO2-escaped
}

// parseRecvLog reads a capture file and returns every RECV event in
// chronological order. Lines with a blank IPID (e.g. a pre-auth HI# seen in
// the raid capture before the server had assigned one) are dropped -- there
// is no identity to attribute them to, and they carry no scoring signal
// anyway.
func parseRecvLog(t *testing.T, path string) []recvEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var events []recvEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		m := recvLineRE.FindStringSubmatch(line)
		if m == nil {
			continue // SEND line, or something else entirely -- not authorship
		}
		ipid := m[2]
		if ipid == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, m[1])
		if err != nil {
			t.Fatalf("%s:%d: unparseable timestamp %q: %v", path, lineNo, m[1], err)
		}
		events = append(events, recvEvent{ts: ts, ipid: ipid, raw: m[4]})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning %s: %v", path, err)
	}
	if len(events) == 0 {
		t.Fatalf("%s: parsed zero RECV events -- the log format assumption is wrong", path)
	}
	// The capture should already be chronological, but sort defensively (and
	// stably, so same-timestamp lines keep file order) rather than assume it.
	sort.SliceStable(events, func(i, j int) bool { return events[i].ts.Before(events[j].ts) })
	return events
}

// parseObjection reads the MS ShoutModifier field. Newer clients can suffix it
// with a custom shout name ("4&mycustomsound"), so the numeric part is taken
// before any '&' rather than handed to Atoi whole.
func parseObjection(shoutModifier string) int {
	main := shoutModifier
	if i := strings.IndexByte(shoutModifier, '&'); i >= 0 {
		main = shoutModifier[:i]
	}
	n, err := strconv.Atoi(main)
	if err != nil {
		return 0
	}
	return n
}

// fieldAt returns body[i], or "" if body is too short -- CT bodies from a
// malformed/legacy sender are not guaranteed to carry every field.
func fieldAt(body []string, i int) string {
	if i >= 0 && i < len(body) {
		return body[i]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Driving the engine
// ---------------------------------------------------------------------------

// ipidReplay is one IPID's accumulated connection state while replaying a
// capture -- the pieces of *Client the engine needs that a log line alone
// doesn't carry (when the connection was first seen, and when/whether it has
// picked a character).
type ipidReplay struct {
	ipid        string
	rs          *raidState
	connectedAt time.Time // proxy: the timestamp of this IPID's first RECV event
	charPicked  bool
	charPickAt  time.Time
	msgCount    int // CT + MS messages actually observed, i.e. "spoke"
}

func (ir *ipidReplay) sinceCharPick(now time.Time) time.Duration {
	if !ir.charPicked {
		return -1
	}
	return now.Sub(ir.charPickAt)
}

// replayResult is the final, read-only outcome for one IPID after a capture
// has been fully replayed -- everything the assertions and the report need.
type replayResult struct {
	ipid     string
	score    int
	signals  []string
	msgCount int
}

// applyEvent folds one RECV packet into the connection's evidence, exactly as
// the corresponding hot-path hook (pktIC/pktOOC/pktChangeChar/handshake
// handling) would: handshake ordering, character picks, IC/OOC messages
// through Observation, and cross-IPID content correlation.
func applyEvent(ir *ipidReplay, ev recvEvent) {
	fields := strings.Split(ev.raw, "#")
	kind := fields[0]
	body := fields[1:]

	switch kind {
	case "askchaa":
		ir.rs.noteAskchaa()

	case "RC", "RM", "RD":
		// Handshake list requests. Arriving before this connection's own
		// askchaa is the protocol anomaly the guard watches for.
		ir.rs.noteHandshakeStep()

	case "CC":
		// Character pick. sinceConnect uses the first-RECV-event proxy for
		// connection accept time (the capture has no lower-level accept
		// timestamp to draw on).
		ir.rs.noteCharPick(ev.ts.Sub(ir.connectedAt), ev.ts)
		ir.charPicked = true
		ir.charPickAt = ev.ts

	case "CT":
		// CT#<ooc_name>#<message>#
		oocName := decode(fieldAt(body, 0))
		msg := decode(fieldAt(body, 1))
		ir.msgCount++
		ir.rs.observe(Observation{
			IPID:          ir.ipid,
			IsIC:          false,
			Text:          msg,
			OOCName:       oocName,
			SinceConnect:  ev.ts.Sub(ir.connectedAt),
			SinceCharPick: ir.sinceCharPick(ev.ts),
			Now:           ev.ts,
		})
		// Same helper the production IC/OOC hooks use, so the harness cannot
		// drift from them on which half of the graded corroboration it records.
		raidGuardCorrelate(ir.rs, ir.ipid, msg, ev.ts)

	case "MS":
		// Client-format MS: up to 26 fields, no OtherName/OtherEmote. Decoded
		// once as a whole so packet.ParseMSClient sees plain text, then parsed
		// through the same production decoder every other MS consumer in this
		// codebase uses -- no packet field is ever indexed by hand here.
		//
		// Empirically verified against both wire shapes actually present in
		// the raid capture: a "modern" 26-field body
		// (MS#1#-#char#emote#msg#side#0#0#<id>#0#0#0#0#0#0##-1#0&0#...) and a
		// shorter "legacy" 20-field body
		// (MS#0##colin##text##1#0#3702#0#1#0#0#0#0#showname#-1#0#0#). Indexing
		// both bodies by hand and comparing against packet.ParseMSClient
		// confirms ShoutModifier is body-index 10 in *both* shapes -- the
		// legacy sender omits Emote/Side's usual content and reorders nothing,
		// it just leaves fields blank and stops early. So there is exactly one
		// client-format layout, of variable length, and ParseMSClient (which
		// is itself length-tolerant: "if len(body) > N") is the correct and
		// only parser needed for either shape.
		decoded := make([]string, len(body))
		for i, f := range body {
			decoded[i] = decode(f)
		}
		ms := packet.ParseMSClient(decoded)
		ir.msgCount++
		ir.rs.observe(Observation{
			IPID:          ir.ipid,
			IsIC:          true,
			Text:          ms.Message,
			Showname:      ms.Showname,
			Objection:     parseObjection(ms.ShoutModifier),
			SinceConnect:  ev.ts.Sub(ir.connectedAt),
			SinceCharPick: ir.sinceCharPick(ev.ts),
			Now:           ev.ts,
		})
		raidGuardCorrelate(ir.rs, ir.ipid, ms.Message, ev.ts)

	default:
		// askchaa's siblings (ID, HI, CH, VS_JOIN, MC, ...) carry no signal
		// this engine models; ignored, same as the production hot path
		// ignores them for raid-guard purposes.
	}
}

// replayCaptureFile parses path and drives a fresh raidState per IPID through
// every RECV event, in true chronological order across IPIDs (required for
// the correlation window's timing to mean anything), and returns one
// replayResult per IPID in first-seen order.
func replayCaptureFile(t *testing.T, path string) []replayResult {
	t.Helper()
	events := parseRecvLog(t, path)

	states := make(map[string]*ipidReplay)
	var order []string
	for _, ev := range events {
		ir, ok := states[ev.ipid]
		if !ok {
			ir = &ipidReplay{ipid: ev.ipid, rs: newRaidState(), connectedAt: ev.ts}
			states[ev.ipid] = ir
			order = append(order, ev.ipid)
		}
		applyEvent(ir, ev)
	}

	results := make([]replayResult, 0, len(order))
	for _, ipid := range order {
		ir := states[ipid]
		score, signals, _ := ir.rs.snapshot()
		results = append(results, replayResult{ipid: ipid, score: score, signals: signals, msgCount: ir.msgCount})
	}
	return results
}

// ---------------------------------------------------------------------------
// Reporting helpers
// ---------------------------------------------------------------------------

func scoreStats(results []replayResult) (min, max int, mean float64) {
	if len(results) == 0 {
		return 0, 0, 0
	}
	min, max = results[0].score, results[0].score
	sum := 0
	for _, r := range results {
		if r.score < min {
			min = r.score
		}
		if r.score > max {
			max = r.score
		}
		sum += r.score
	}
	return min, max, float64(sum) / float64(len(results))
}

var verdictOrder = []Verdict{VerdictClean, VerdictWatch, VerdictChallenge, VerdictSilence, VerdictKick, VerdictBan}

func verdictHistogram(results []replayResult, scalePct int) map[Verdict]int {
	h := make(map[Verdict]int, len(verdictOrder))
	for _, r := range results {
		h[verdictForTier(r.score, scalePct)]++
	}
	return h
}

func verdictHistLine(h map[Verdict]int) string {
	parts := make([]string, 0, len(verdictOrder))
	for _, v := range verdictOrder {
		parts = append(parts, fmt.Sprintf("%s=%d", v, h[v]))
	}
	return strings.Join(parts, " ")
}

func signalFireCounts(results []replayResult) map[string]int {
	counts := make(map[string]int)
	for _, r := range results {
		for _, s := range r.signals {
			counts[s]++
		}
	}
	return counts
}

func signalCountsLine(results []replayResult) string {
	counts := signalFireCounts(results)
	parts := make([]string, 0, numRaidSignals)
	for k := SignalKind(0); k < numRaidSignals; k++ {
		name := raidSignalName[k]
		parts = append(parts, fmt.Sprintf("%s=%d", name, counts[name]))
	}
	return strings.Join(parts, " | ")
}

// logReplayReport prints the tuning evidence for one capture: how many IPIDs
// were seen, the score distribution, per-signal fire counts, and the verdict
// histogram at the production baseline tier.
func logReplayReport(t *testing.T, label string, results []replayResult) {
	t.Helper()
	min, max, mean := scoreStats(results)
	speaking := 0
	for _, r := range results {
		if r.msgCount > 0 {
			speaking++
		}
	}
	t.Logf("[%s] %d IPIDs observed (%d spoke at least once) -- score min=%d max=%d mean=%.1f",
		label, len(results), speaking, min, max, mean)
	t.Logf("[%s] signal fire counts: %s", label, signalCountsLine(results))
	t.Logf("[%s] verdict histogram @ scalePct=%d (baseline): %s", label, raidGuardScaleBase, verdictHistLine(verdictHistogram(results, raidGuardScaleBase)))
}

// ---------------------------------------------------------------------------
// The validation test
// ---------------------------------------------------------------------------

// TestRaidGuardReplayCapturesAgainstRealTraffic is the validation harness: it
// replays a genuine raid capture and a genuine clean-evening capture through
// the exact same scoring engine production traffic drives, and checks the
// property this whole system exists for.
//
// scalePct=100 (raidGuardScaleBase) is the tier used for every primary
// pass/fail assertion below: it is the "no adjustment, judge the score as
// configured" baseline, and it is a strictly more conservative bar than the
// scalePct=70 "strict" tier a brand-new, no-history connection is actually
// judged at in production (a lower scalePct loosens every threshold, so
// anything that clears the scalePct=100 bar clears scalePct=70 at least as
// easily). Both captures are also reported (and, for the normal capture,
// *asserted*) at scalePct=70, since that is the harshest tier any real
// player -- who always starts with zero playtime -- could ever actually be
// judged at.
func TestRaidGuardReplayCapturesAgainstRealTraffic(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()
	config = settings.DefaultConfig() // RaidGuard* fields at their shipped defaults

	resetRaidGuardState()
	t.Cleanup(resetRaidGuardState)

	// --- Replay the raid capture ---
	raidResults := replayCaptureFile(t, "testdata/raid_capture.log")

	// Correlation state (content fingerprints, the "under attack" flag) must
	// never leak from one capture's replay into the next -- a raid's
	// fingerprints polluting a clean replay would be a self-inflicted false
	// positive that has nothing to do with the engine's real behaviour. Reset
	// before each replay below, not once here, since there is now more than one
	// clean capture and they must not pollute each other either.

	// --- Replay the clean captures ---
	//
	// Two of them, from different evenings and different populations:
	//
	//	normal_capture.log     a quiet evening, nothing happening.
	//	aftermath_capture.log  the same server two minutes after the 2026-08-31
	//	                       raid: 24 players, agitated, several of them
	//	                       shouting in caps and using the objection button
	//	                       while they talk about what just happened.
	//
	// The second is the harder and more useful one. A quiet evening does not
	// test much -- the interesting question is whether the guard stays silent
	// through a room that is loud, upset and talking in the raid's own
	// vocabulary, which is exactly the traffic a loosened threshold would eat.
	cleanCaptures := []struct {
		label string
		path  string
	}{
		{"NORMAL", "testdata/normal_capture.log"},
		{"AFTERMATH", "testdata/aftermath_capture.log"},
	}
	cleanResults := make(map[string][]replayResult, len(cleanCaptures))
	for _, c := range cleanCaptures {
		resetRaidGuardState()
		cleanResults[c.label] = replayCaptureFile(t, c.path)
	}

	logReplayReport(t, "RAID", raidResults)
	for _, c := range cleanCaptures {
		logReplayReport(t, c.label, cleanResults[c.label])
	}

	raidHist70 := verdictHistogram(raidResults, 70)
	t.Logf("[RAID] verdict histogram @ scalePct=70 (strict/brand-new tier -- what these connections would actually be judged at in production, informational only): %s",
		verdictHistLine(raidHist70))
	{
		speaking70, detected70 := 0, 0
		for _, r := range raidResults {
			if r.msgCount == 0 {
				continue
			}
			speaking70++
			if verdictForTier(r.score, 70) >= VerdictChallenge {
				detected70++
			}
		}
		t.Logf("[RAID] detection rate @ scalePct=70 (production tier for these connections), speaking IPIDs only: %d/%d (%.1f%%), informational only",
			detected70, speaking70, 100*float64(detected70)/float64(speaking70))
	}

	// -----------------------------------------------------------------
	// THE LOAD-BEARING ASSERTION: zero punitive verdicts on real traffic.
	// -----------------------------------------------------------------
	// Checked at both scalePct=100 (baseline) and scalePct=70 (the strict,
	// brand-new-connection tier -- the harshest a legitimate first-time
	// player could ever land at). If the baseline capture is clean at 70 it
	// is clean everywhere, since every other tier is only more forgiving.
	for _, c := range cleanCaptures {
		results := cleanResults[c.label]
		for _, scalePct := range []int{raidGuardScaleBase, 70} {
			var offenders []string
			for _, r := range results {
				if v := verdictForTier(r.score, scalePct); v > VerdictWatch {
					offenders = append(offenders, fmt.Sprintf("  IPID %s -> %v (score=%d, signals=[%s])",
						r.ipid, v, r.score, strings.Join(r.signals, ", ")))
				}
			}
			if len(offenders) > 0 {
				t.Errorf("FALSE POSITIVE: %s capture produced %d punitive verdict(s) at scalePct=%d -- "+
					"the guard must NEVER act on legitimate traffic:\n%s",
					c.label, len(offenders), scalePct, strings.Join(offenders, "\n"))
			} else {
				t.Logf("[%s] scalePct=%d: 0/%d IPIDs reached a punitive verdict (clean)", c.label, scalePct, len(results))
			}
		}
	}

	// -----------------------------------------------------------------
	// Detection: a substantial majority of the speaking raid IPIDs must
	// reach at least VerdictChallenge, judged at the conservative baseline
	// tier (scalePct=100).
	// -----------------------------------------------------------------
	// The primary floor below is measured, and asserted, against *every*
	// speaking IPID in the capture -- the exact "speaking IPIDs" population
	// this test's brief specifies -- deliberately without narrowing that set
	// by hand first. A validation harness for a false-positive-averse safety
	// system should not get to improve its own headline number by silently
	// excluding the cases it finds inconvenient, however well-reasoned the
	// exclusion; that is the same discipline the false-positive assertion
	// above is held to.
	//
	// That said, the raid capture is a live production log, so it plausibly
	// contains a handful of ordinary players who simply happened to be in the
	// room while the raid was happening -- and those are worth knowing about
	// on their own terms, because a legitimate player caught inside an actual
	// raid (not just a quiet evening) is the hardest version of the
	// never-false-positive requirement this system has to meet. Three
	// speaking IPIDs in this capture have packet content with no raid
	// signature at all, verified directly against testdata/raid_capture.log:
	//
	//	7BECOHJw8BF8+NhU4bHdXg  one packet total: modern-shape MS with an empty
	//	                        message and objection 0 -- an idle pose change,
	//	                        not speech.
	//	WdrGhwP5be+ByUw2ltS5UQ  CT "you can also do 1d25-[number]" -- someone
	//	                        explaining dice syntax to another player.
	//	6xUFHYCzSTOMcXtPmRNDhA  modern-shape MS, showname "The guy from
	//	                        Fortnite", message "It's ez", objection 0.
	//
	// These are reported and asserted on separately, below, in addition to
	// (never instead of) the full-population floor: if the guard ever landed
	// a punitive verdict on one of them that would be a false positive on a
	// real player inside the very capture meant to demonstrate detection,
	// which is worse than a missed raider.
	bystanders := map[string]string{
		"7BECOHJw8BF8+NhU4bHdXg": "one packet, empty message -- idle pose change, not speech",
		"WdrGhwP5be+ByUw2ltS5UQ": "explaining dice syntax to another player",
		"6xUFHYCzSTOMcXtPmRNDhA": "ordinary chat, benign showname",
	}

	var speaking, detected int
	var missed []string
	for _, r := range raidResults {
		if r.msgCount == 0 {
			continue // never actually spoke; nothing for content signals to see
		}
		speaking++
		v := verdictForTier(r.score, raidGuardScaleBase)
		if v >= VerdictChallenge {
			detected++
		} else {
			missed = append(missed, fmt.Sprintf("  IPID %s -> %v (score=%d, signals=[%s])",
				r.ipid, v, r.score, strings.Join(r.signals, ", ")))
		}
		if why, ok := bystanders[r.ipid]; ok {
			t.Logf("[RAID] bystander %s (%s) -> %v (score=%d)", r.ipid, why, v, r.score)
			if v > VerdictWatch {
				t.Errorf("FALSE POSITIVE on a real player caught inside the raid capture: "+
					"IPID %s (%s) reached %v (score=%d, signals=[%s])",
					r.ipid, why, v, r.score, strings.Join(r.signals, ", "))
			}
		}
	}
	if speaking == 0 {
		t.Fatalf("RAID capture yielded zero speaking IPIDs -- the parser is almost certainly broken")
	}

	rate := float64(detected) / float64(speaking)
	raiders, raidersDetected := speaking-len(bystanders), 0
	for _, r := range raidResults {
		if r.msgCount == 0 {
			continue
		}
		if _, isBystander := bystanders[r.ipid]; isBystander {
			continue
		}
		if verdictForTier(r.score, raidGuardScaleBase) >= VerdictChallenge {
			raidersDetected++
		}
	}
	t.Logf("[RAID] detection rate @ scalePct=%d: %d/%d speaking IPIDs (%.1f%%) reached >= VerdictChallenge "+
		"(%d/%d excluding the %d identified bystanders, %.1f%%)",
		raidGuardScaleBase, detected, speaking, rate*100,
		raidersDetected, raiders, len(bystanders), 100*float64(raidersDetected)/float64(raiders))
	if len(missed) > 0 {
		t.Logf("[RAID] speaking IPIDs that did NOT reach VerdictChallenge @ scalePct=%d:\n%s",
			raidGuardScaleBase, strings.Join(missed, "\n"))
	}

	// The floor is set well below the measured rate (currently 61.1% across
	// all 18 speaking IPIDs; see the logged line above for the number on the
	// checked-out capture). Margin is deliberate for two independent reasons:
	// the capture is a three-second slice, so several missed raiders are
	// OOC-only and never got the chance to accumulate the timing/content
	// signals a longer raid would keep feeding them; and the signal weights
	// and score thresholds this test reads from settings.DefaultConfig() are
	// still being tuned elsewhere in this codebase, so a floor set tight
	// against today's exact number would go flaky under routine retuning
	// rather than catching a genuine detection collapse, which is the only
	// thing this floor exists to catch.
	const detectionFloor = 0.50
	if rate < detectionFloor {
		t.Errorf("RAID detection rate %.1f%% is below the required floor of %.0f%% (%d/%d speaking IPIDs reached >= VerdictChallenge)",
			rate*100, detectionFloor*100, detected, speaking)
	}
}

// ---------------------------------------------------------------------------
// Memory-safety: the correlation window must stay bounded under flood
// ---------------------------------------------------------------------------

// TestRaidGuardCorrelationWindowBoundedUnderFlood hammers CorrelationWindow
// with far more unique fingerprints than maxEntries -- exactly the shape of
// load a real raid produces, since every raider varies their line slightly --
// and checks the tracked entry count never grows past a small, fixed bound
// regardless of how long the flood continues. This is the property that
// keeps a raid from turning into a server OOM: pruneLocked wipes the map
// wholesale once it is caught strictly over maxEntries, so the *worst* any
// caller can ever observe is one entry over the cap, never proportional to
// how much traffic has been thrown at it.
func TestRaidGuardCorrelationWindowBoundedUnderFlood(t *testing.T) {
	const maxEntries = 64
	const floodFactor = 25 // far more unique fingerprints than maxEntries
	w := NewCorrelationWindow(10*time.Second, maxEntries)

	now := time.Now()
	peak := 0
	for i := 0; i < maxEntries*floodFactor; i++ {
		// A unique fingerprint and a unique IPID per message: the worst case
		// for this structure, since nothing collides and every message grows
		// the map by one entry until pruning kicks in.
		w.Observe(uint64(i), fmt.Sprintf("flood-ipid-%d", i), now)
		now = now.Add(time.Millisecond)
		if l := w.Len(); l > peak {
			peak = l
		}
	}

	t.Logf("CorrelationWindow: fed %d unique fingerprints against maxEntries=%d; peak tracked length=%d",
		maxEntries*floodFactor, maxEntries, peak)

	// pruneLocked only wipes once the map is caught *strictly over*
	// maxEntries, so a length of maxEntries+1 can transiently be observed
	// right before the next call clears it -- that is the true, intended
	// bound, not maxEntries itself.
	if want := maxEntries + 1; peak > want {
		t.Errorf("CorrelationWindow.Len() peaked at %d while flooded with %dx maxEntries unique fingerprints; "+
			"want <= %d -- an unbounded correlation window would let a raid OOM the server",
			peak, floodFactor, want)
	}
	if final := w.Len(); final > maxEntries+1 {
		t.Errorf("CorrelationWindow.Len() = %d after the flood, want <= %d", final, maxEntries+1)
	}
}
