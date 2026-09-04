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

// Where the content gate sits inside pktIC, asserted against the source.
//
// This is an unusual shape for a test and it is deliberate. The property is an
// ORDERING one: the censor must run before anything that can put the text or the
// showname in front of another player. It cannot be observed from the outside
// without standing up a full client, an area and a database, and even then a
// passing test would only prove the ordering for the paths the test happened to
// drive -- the leak this pins was in the showname path specifically, which no
// existing test drove.
//
// It is also a property that regresses silently. The bug it guards against was
// introduced by nothing more than a new feature being appended in the obvious
// place: the showname was broadcast to every client in a PU packet during the
// state-update block, and the showname censor ran forty lines later, so a slur
// worn as a showname rendered in every client in the room and the filter never
// saw it. Nothing failed. No error was logged. It simply did not work.
//
// So the invariant is checked where it actually lives: in the order of the
// statements. If a future change moves the gate back down, or adds a new
// broadcast above it, this fails and names the two landmarks that swapped.
package athena

import (
	"os"
	"strings"
	"testing"
)

// pktICLandmarks are the statements whose relative order matters, in the order
// they must appear.
type landmark struct {
	name string
	find string
}

func TestContentGatePrecedesEveryLeakInPktIC(t *testing.T) {
	src, err := os.ReadFile("netprotocol.go")
	if err != nil {
		t.Fatalf("read netprotocol.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "func pktIC(") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("pktIC not found -- this test's assumptions about the file are stale")
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "func ") {
			end = i
			break
		}
	}
	body := lines[start:end]

	// lineOf returns the first line in pktIC containing needle, or -1.
	lineOf := func(needle string) int {
		for i, l := range body {
			if strings.Contains(l, needle) {
				return i
			}
		}
		return -1
	}

	gate := lineOf(`checkCensored(msgText, "IC message")`)
	if gate < 0 {
		t.Fatal("the content gate call was not found in pktIC -- if it was renamed, update this test; " +
			"do not delete it, the ordering it pins is a real leak")
	}

	// Everything the gate must precede, and why it matters if it does not.
	mustFollow := []landmark{
		{"the PU showname broadcast (a slur worn as a showname would reach every client uncensored)",
			"packet.PU{ID: client.Uid(), Type: 2"},
		{"the torment branch (a censored message could escape via the delayed rebroadcast)",
			"handleTormentedIC(client, ms)"},
		{"the quickdraw hook (a censored line could win the minigame)",
			"quickdrawOnIC(client, msgText)"},
		{"the typing-race hook", "typingRaceOnIC(client, msgText)"},
		{"the unscramble hook (a censored line could claim the chip prize)",
			"unscrambleOnIC(client, msgText)"},
		{"the stealthmute silencing decision",
			"silenced := hasPunishmentType(punishments, PunishmentStealthMute)"},
		{"LastMsg being updated from the uncensored packet", "client.SetLastMsg(ms.Message)"},
	}

	for _, lm := range mustFollow {
		at := lineOf(lm.find)
		if at < 0 {
			t.Errorf("landmark %q (%s) no longer appears in pktIC -- this test is stale and must be updated, "+
				"not deleted", lm.find, lm.name)
			continue
		}
		if at < gate {
			t.Errorf("ORDERING REGRESSION: %q appears at pktIC line +%d, BEFORE the content gate at +%d.\n"+
				"It must run after: %s", lm.find, at, gate, lm.name)
		}
	}
}

// TestNukeIsCheckedBeforeEveryGentlerAction pins the other half: within the
// gate, a nuke must be decided before the shadow/kick handling, since the whole
// point of the tier is that it does not echo to the sender the way a shadow trip
// does.
func TestNukeIsCheckedBeforeEveryGentlerAction(t *testing.T) {
	src, err := os.ReadFile("netprotocol.go")
	if err != nil {
		t.Fatalf("read netprotocol.go: %v", err)
	}
	body := string(src)

	nuke := strings.Index(body, "SeverityNuke")
	if nuke < 0 {
		t.Fatal("no SeverityNuke check in netprotocol.go -- the nuke tier is not wired up")
	}
	// Inside the shared checkCensored closure the nuke branch must come before
	// the autoModShadow branch that sets censorShadow.
	closure := strings.Index(body, "checkCensored := func(")
	if closure < 0 {
		t.Skip("checkCensored closure not found; the gate was restructured")
	}
	seg := body[closure:]
	nukeAt := strings.Index(seg, "SeverityNuke")
	shadowAt := strings.Index(seg, "case autoModShadow:")
	if nukeAt < 0 || shadowAt < 0 {
		t.Fatalf("gate no longer contains both a nuke branch (%d) and a shadow branch (%d)", nukeAt, shadowAt)
	}
	if nukeAt > shadowAt {
		t.Error("the nuke branch is evaluated after the shadow branch; a nuke would be echoed back to its " +
			"sender, which is exactly what the tier exists to prevent")
	}
}
