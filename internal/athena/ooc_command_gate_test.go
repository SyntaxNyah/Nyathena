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

package athena

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// Every command handler that broadcasts free player text must consult
// oocCommandAllowed before it does, and pktOOC's command branch must keep
// returning before the content gate so the two cannot both run.
//
// Asserted against the source for the same reason TestContentGatePrecedes-
// EveryLeakInPktIC is: the property is an ordering one, it regresses silently,
// and the bug it guards was created by nothing worse than a handler being
// written in the obvious way. /global built a packet and broadcast it, with no
// hint anywhere in the function that a whole suppression pipeline was being
// skipped -- the word filter looked like it was working because it was, on the
// path /global does not take.
func TestBroadcastingOOCCommandsConsultTheGate(t *testing.T) {
	src, err := os.ReadFile("commands_moderation.go")
	if err != nil {
		t.Fatalf("read commands_moderation.go: %v", err)
	}
	text := string(src)

	for _, fn := range []string{"cmdGlobal", "cmdPM"} {
		t.Run(fn, func(t *testing.T) {
			start := strings.Index(text, "func "+fn+"(")
			if start < 0 {
				t.Fatalf("%s not found", fn)
			}
			end := strings.Index(text[start+1:], "\nfunc ")
			body := text[start:]
			if end > 0 {
				body = text[start : start+1+end]
			}

			gate := strings.Index(body, "oocCommandAllowed(")
			if gate < 0 {
				t.Fatalf("%s does not consult oocCommandAllowed; free player text "+
					"reaches other players unexamined by the word filter, the torment "+
					"list, stealthmute and the captcha restriction", fn)
			}

			// Every send of the message must come after the gate.
			sendRe := regexp.MustCompile(`broadcastToAll\(|c\.Send\(|client\.Send\(`)
			for _, loc := range sendRe.FindAllStringIndex(body, -1) {
				if loc[0] < gate {
					t.Errorf("%s sends at offset %d, before the gate at %d: %q",
						fn, loc[0], gate, strings.TrimSpace(body[loc[0]:min(loc[0]+60, len(body))]))
				}
			}
		})
	}
}

// The command branch must keep returning before pktOOC's own gate. If it ever
// stopped, /global would be filtered twice (two censor alerts, two kicks) --
// the opposite failure, and just as wrong.
func TestCommandBranchStillReturnsBeforePktOOCGate(t *testing.T) {
	src, err := os.ReadFile("netprotocol.go")
	if err != nil {
		t.Fatalf("read netprotocol.go: %v", err)
	}
	text := string(src)
	start := strings.Index(text, "func pktOOC(")
	if start < 0 {
		t.Fatal("pktOOC not found")
	}
	body := text[start:]

	dispatch := strings.Index(body, "ParseCommand(client, command, args)")
	gate := strings.Index(body, `autoModCheckTiered(client, decode(msg), "OOC message")`)
	if dispatch < 0 || gate < 0 {
		t.Fatal("landmarks not found; pktOOC has been restructured")
	}
	if dispatch > gate {
		t.Fatal("command dispatch now runs after the OOC content gate; a /global " +
			"would be filtered twice")
	}
	// And the dispatch must be immediately followed by a return.
	after := body[dispatch : dispatch+120]
	if !strings.Contains(after, "return") {
		t.Error("command dispatch no longer returns; commands would fall through " +
			"into the plain-OOC path")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// The gate's actual verdicts. Uses the same real-client-plus-real-word-list
// scaffolding as automod_tier_integrity_test.go, so this exercises the
// production matcher rather than a stand-in.
func TestOOCCommandGateVerdicts(t *testing.T) {
	// The shadow branch ends in KickForCensorTrip, which reads the autoban
	// config -- the same path production takes.
	withRaidConfig(t)
	oldAction := autoModAction
	t.Cleanup(func() { autoModAction = oldAction })
	autoModAction = autoModActionShadow

	writeWordList(t, "slurword | severe\nalertonly | watch\n")

	echo := &packet.CTToClient{Name: "[GLOBAL] [UID 1] tester", Message: "x", IsFromServer: "1"}

	// A real area, because the suppressing branches write to its report buffer.
	room := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	// A distinct IPID per client: a censor trip feeds the repeat-offender
	// autoban, so subtests sharing one IPID would cross its threshold and drag
	// a nil test database into the picture. That the kick path is reachable
	// from here at all is the point -- it means the gate is wired to the same
	// escalation the plain OOC path uses.
	n := 0
	newClient := func() *Client {
		n++
		return &Client{char: -1, conn: &testConn{}, area: room, ipid: fmt.Sprintf("test-%d", n)}
	}

	t.Run("clean text passes", func(t *testing.T) {
		c := newClient()
		if !oocCommandAllowed(c, "hello everyone", "global message", echo) {
			t.Error("a clean global was blocked")
		}
	})

	t.Run("banned word is blocked", func(t *testing.T) {
		c := newClient()
		if oocCommandAllowed(c, "you are a slurword", "global message", echo) {
			t.Error("a global carrying a severe-tier word was allowed to broadcast; " +
				"this is the bug -- it reached every client on the server")
		}
	})

	t.Run("evasion is normalized like anywhere else", func(t *testing.T) {
		c := newClient()
		if oocCommandAllowed(c, "s l u r w o r d", "global message", echo) {
			t.Error("spaced-out evasion passed the global gate")
		}
	})

	// The property PR #455 restored: watch means tell staff, never punish. It
	// has to hold on this path too, or the same false positives come back
	// through a different door.
	t.Run("watch tier still passes", func(t *testing.T) {
		for _, action := range []autoModActionKind{
			autoModActionShadow, autoModActionTorment, autoModActionKick,
			autoModActionMute, autoModActionBan,
		} {
			autoModAction = action
			c := newClient()
			if !oocCommandAllowed(c, "that is alertonly", "global message", echo) {
				t.Errorf("action %v: a watch-tier global was blocked; watch must only alert", action)
			}
		}
		autoModAction = autoModActionShadow
	})

	t.Run("stealthmute is no longer bypassable", func(t *testing.T) {
		c := newClient()
		c.AddPunishment(PunishmentStealthMute, time.Hour, "")
		if oocCommandAllowed(c, "perfectly clean text", "global message", echo) {
			t.Error("a stealthmuted player reached the whole server through /global")
		}
	})
}
