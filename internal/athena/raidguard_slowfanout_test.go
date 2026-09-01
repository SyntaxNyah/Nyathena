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

// Validation against the SLOW fan-out -- the raid shape the shipped layer-2
// thresholds could not see at all.
//
// raid_capture.log is a raid at its loudest: 65 IPIDs in three seconds, one
// line repeated far more than four times, every threshold crossed immediately.
// The 2026-08-31 incident is the other shape, and it is the more common one:
// ten IPIDs, ten different slurs, twenty-six seconds, lines re-used two or
// three times but never four. At the thresholds this codebase shipped, layer 2
// produced nothing whatsoever for it -- no signal, no under-attack flag, and so
// no ban ever reachable -- which is what "a few messages got in" looked like
// from the inside.
//
// These tests replay that incident from the server's own log buffer (testdata/
// raid_20260831_iclog.txt) and pin three things: the guard now corroborates the
// fan-out early, it still does not corroborate any of the real players talking
// in the same room at the same time, and it still cannot reach the ban gate on
// this evidence.
package athena

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// icLogLine is one line of the per-area log format the fixture is written in.
type icLogLine struct {
	ts   time.Time
	ipid string
	char string
	text string
}

// parseICLog reads the per-area log fixture. Only IC lines carry text; the
// timestamps are wall-clock times of day, so they are anchored onto the
// capture's date to give the correlation window real durations to work with.
func parseICLog(t *testing.T, path string) []icLogLine {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	day := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	var out []icLogLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Split on the bare pipe rather than " | ": the HDID field is empty in
		// this log, which renders as "| |" and would swallow a field if the
		// separator included its surrounding spaces.
		parts := strings.Split(line, "|")
		if len(parts) < 6 || strings.TrimSpace(parts[1]) != "IC" {
			continue
		}
		stamp := strings.TrimSpace(parts[0])
		clock, err := time.Parse("15:04:05", stamp)
		if err != nil {
			t.Fatalf("%s: unparseable timestamp %q: %v", path, stamp, err)
		}
		out = append(out, icLogLine{
			ts:   day.Add(time.Duration(clock.Hour())*time.Hour + time.Duration(clock.Minute())*time.Minute + time.Duration(clock.Second())*time.Second),
			ipid: strings.TrimSpace(parts[3]),
			char: strings.TrimSpace(parts[2]),
			// The message is the whole remainder, so a line containing a pipe
			// survives instead of being truncated at the first one.
			text: strings.TrimSpace(strings.Join(parts[5:], "|")),
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning %s: %v", path, err)
	}
	if len(out) == 0 {
		t.Fatalf("%s: parsed zero IC lines -- the fixture format assumption is wrong", path)
	}
	return out
}

// raiders20260831 is every IPID that sent raid content in the fixture, in the
// order it first spoke. Ten distinct IPIDs, none of which repeats a line often
// enough on its own to reach the strong corroboration threshold.
var raiders20260831 = []string{
	"UXFI2TY42Y41u1OXQNhJCQ", // COA NOW IS WHITELIST ONLY GET FUCKED
	"xiDtVwc75LbsMpnItQp8Dw", // Faggots faggots.
	"KJvOo1Uf2R5oi9a171Zn2A", // "...vibe coders go to hell." (1st)
	"zGGldMiqe+oGrO1TFQHQ7w", // "...vibe coders go to hell." (2nd, +1s)
	"p1X8IuAwVXuQkzWh7xbd7w", // CLOVERRR GET RAPED GET RAPED (1st)
	"OwIbZ6LlObZsjtLlR+QMSQ", // CLOVER YOU LOLICON RAPE RAPE RAPE
	"8ySOTcS1w0uF+b5x65qUzw", // 4CHAN GET FUCKED ON BY RAVEN (1st)
	"zcW3ge7pW+KXBmtyQJlTAA", // CLOVERRR LIKES LOLIS
	"p8+1fwBhAJ6iKgvZPg+dKw", // "...vibe coders go to hell." (3rd, +19s)
	"q0v47hVabxEfjtjEG9YSpQ", // 4CHAN GET FUCKED ON BY RAVEN (2nd, +16s)
	"NTnvZ2US5nAvtRFfwRbLfQ", // CLOVERRR GET RAPED GET RAPED (2nd, +35s)
}

// bystanders20260831 is every IPID in the fixture that is an ordinary player,
// talking in the room while the raid happened and immediately afterwards. Two
// of them are discussing the raid itself, which is the hardest case in the
// file: they use the raid's own vocabulary without being part of it.
var bystanders20260831 = map[string]string{
	"vdIBxXbiLtRk8JdgPwPS2g": "sans_atf, mid-conversation about a video game throughout",
	"7zcqZQv11MxtWQ3Gz0IKeg": "alice pc98, asking what she missed, then discussing the bans",
	"xT2NLl+OqAGvzW48zEhljw": "marshall aa, reacting to the raid",
	"Wbjbg4aWjNcbzdOKxs9X1A": "uzukamaru taizo, saying out loud that the anti-raid worked",
}

// replaySlowFanout drives the fixture through the same correlation helper the
// production IC hook uses and returns each IPID's fired signals plus the index
// of the raid message on which the guard first corroborated anything.
func replaySlowFanout(t *testing.T) (states map[string]*raidState, firstCorroboratedAt int, order []string) {
	t.Helper()
	lines := parseICLog(t, "testdata/raid_20260831_iclog.txt")
	raider := make(map[string]bool, len(raiders20260831))
	for _, r := range raiders20260831 {
		raider[r] = true
	}

	states = make(map[string]*raidState)
	firstCorroboratedAt = -1
	raidMsgSeen := 0
	for _, l := range lines {
		rs, ok := states[l.ipid]
		if !ok {
			rs = newRaidState()
			states[l.ipid] = rs
			order = append(order, l.ipid)
		}
		if raider[l.ipid] {
			raidMsgSeen++
		}
		fired := raidGuardCorrelate(rs, l.ipid, l.text, l.ts)
		if len(fired) > 0 && firstCorroboratedAt < 0 {
			firstCorroboratedAt = raidMsgSeen
		}
	}
	return states, firstCorroboratedAt, order
}

// TestSlowFanOutIsCorroboratedEarly is the regression this whole change exists
// for. Before it, replaying this fixture produced zero signals: the strong
// threshold (4 distinct IPIDs on one line inside 10 seconds) was never met, so
// every raid message landed with layer 2 silent.
//
// The assertion is on the count of raid messages that reach the room before the
// guard has anything to say, because that count is the user-visible defect --
// "i saw a few messages get in".
func TestSlowFanOutIsCorroboratedEarly(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()
	config = settings.DefaultConfig()

	resetRaidGuardState()
	t.Cleanup(resetRaidGuardState)

	states, firstAt, _ := replaySlowFanout(t)

	if firstAt < 0 {
		t.Fatalf("the 2026-08-31 raid produced NO layer-2 signal at all -- this is the exact regression " +
			"this test exists to catch: ten IPIDs fanning out over twenty-six seconds must not be invisible")
	}
	t.Logf("2026-08-31 raid: first corroborated on raid message #%d of %d", firstAt, len(raiders20260831))

	// The first cross-IPID repeat in the capture is the 4th raid message (the
	// second utterance of "...vibe coders go to hell.", one second after the
	// first). Nothing can detect a fan-out before a second party has fanned it
	// out, so 4 is the floor a perfect detector would hit, and the guard is
	// required to hit it rather than merely to beat "never".
	const perfect = 4
	if firstAt > perfect {
		t.Errorf("first corroboration on raid message #%d; the earliest cross-IPID repeat in the capture is "+
			"message #%d, so %d raid message(s) leaked that did not have to", firstAt, perfect, firstAt-perfect)
	}

	// Which raiders end up carrying the signal, and why it is not all of them.
	//
	// Corroboration is necessarily retrospective: the FIRST connection to say a
	// line has not been echoed by anyone at the moment it speaks, so it is never
	// marked for that line -- only the second and later sayers are. Of the four
	// cross-IPID re-uses in this fixture, three land inside the correlation
	// window ("...vibe coders go to hell." at +1s and again at +19s, "4CHAN GET
	// FUCKED ON BY RAVEN" at +16s) and one does not (the second "CLOVERRR GET
	// RAPED GET RAPED" is 35 seconds after the first, past the 30-second
	// window). Three marked connections is therefore the correct answer for this
	// capture, not a shortfall.
	//
	// Retroactively marking the first sayer once a line turns out to be echoed
	// is a real remaining improvement, deliberately not made here: it would need
	// the correlation window to hold connection state rather than bare IPIDs,
	// and it would not have prevented a single leaked message in this incident,
	// because by the time the echo arrives the first message is already in the
	// room. It changes who gets actioned afterwards, not how fast.
	var echoed []string
	for _, ipid := range raiders20260831 {
		if states[ipid] != nil && states[ipid].hasFired(SigEchoedAcrossIPIDs) {
			echoed = append(echoed, ipid)
		}
	}
	t.Logf("2026-08-31 raid: %d/%d raider IPIDs carry the echo signal: %v",
		len(echoed), len(raiders20260831), echoed)
	if len(echoed) < 3 {
		t.Errorf("only %d raider IPIDs were corroborated across the capture; three of the four cross-IPID "+
			"line re-uses fall inside the correlation window, so at least 3 should be", len(echoed))
	}
}

// TestSlowFanOutSparesTheRealPlayers is the other half, and the one that
// matters more. Four ordinary players are talking in this room while the raid
// runs -- two of them ABOUT the raid, using its vocabulary. Loosening the
// correlation thresholds is only worth anything if it leaves them alone.
func TestSlowFanOutSparesTheRealPlayers(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()
	config = settings.DefaultConfig()

	resetRaidGuardState()
	t.Cleanup(resetRaidGuardState)

	states, _, _ := replaySlowFanout(t)

	for ipid, who := range bystanders20260831 {
		rs := states[ipid]
		if rs == nil {
			t.Errorf("bystander %s (%s) never appeared in the replay -- the fixture or the raider list is wrong", ipid, who)
			continue
		}
		score, signals, _ := rs.snapshot()
		if len(signals) > 0 {
			t.Errorf("FALSE POSITIVE: bystander %s (%s) fired %v (score=%d) -- these are real players talking "+
				"in the room during the raid and the guard must have nothing to say about them",
				ipid, who, signals, score)
			continue
		}
		t.Logf("bystander %s (%s): clean (score=%d)", ipid, who, score)
	}
}

// TestSlowFanOutCannotReachTheBanGate pins the safety half of the graded
// corroboration: the weak echo signal is evidence, but it is not the evidence a
// ban requires. This raid never put four IPIDs on one line, so however many
// connections it lights up, raidBanAllowed must still refuse.
func TestSlowFanOutCannotReachTheBanGate(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()
	config = settings.DefaultConfig()

	resetRaidGuardState()
	t.Cleanup(resetRaidGuardState)

	states, _, order := replaySlowFanout(t)

	for _, ipid := range order {
		rs := states[ipid]
		if rs.hasFired(SigDupeAcrossIPIDs) {
			t.Errorf("IPID %s reached SigDupeAcrossIPIDs on the 2026-08-31 capture, which contains no line "+
				"said by four distinct IPIDs inside one window -- the strong threshold has been weakened, "+
				"and with it the ban gate", ipid)
		}
		// The echo signal must never be mistaken for the strong one at the gate.
		if raidBanAllowed(rs.hasFired(SigDupeAcrossIPIDs), true) && !rs.hasFired(SigDupeAcrossIPIDs) {
			t.Errorf("IPID %s: raidBanAllowed returned true without the strong signal", ipid)
		}
	}
}

// TestEchoSignalAloneIsBelowEveryThreshold is the property that makes a weak
// threshold of two IPIDs safe to ship. Somebody quoting you is not an
// accusation: on its own the echo signal must not reach even the watch rung, at
// any playtime tier, so it always takes a second independent signal before the
// guard does anything at all.
func TestEchoSignalAloneIsBelowEveryThreshold(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()
	config = settings.DefaultConfig()

	alone := raidSignalWeight[SigEchoedAcrossIPIDs]
	for _, scale := range []int{70, raidGuardScaleBase, 200} {
		if v := verdictForTier(alone, scale); v != VerdictClean {
			t.Errorf("the echo signal alone (%d points) yields %v at scalePct=%d; it must be clean at every "+
				"tier, or two players quoting each other becomes an accusation", alone, v, scale)
		}
	}

	// And paired with the single heaviest other signal it must still stop short
	// of anything the player cannot undo themselves.
	pair := alone + raidSignalWeight[SigHandshakeAnomaly]
	if v := clampDisconnect(verdictForTier(pair, 70), 2); v > VerdictSilence {
		t.Errorf("echo + the heaviest single signal (%d points) yields %v at the strict tier; two signals must "+
			"never reach a disconnect", pair, v)
	}
}
