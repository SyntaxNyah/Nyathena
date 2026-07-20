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
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

func TestCharStealCommandRegistered(t *testing.T) {
	initCommands()
	cmd, ok := Commands["charsteal"]
	if !ok {
		t.Fatal("charsteal command is not registered in Commands map")
	}
	if cmd.handler == nil {
		t.Error("charsteal command has a nil handler")
	}
	if cmd.minArgs != 1 {
		t.Errorf("charsteal minArgs = %d, want 1", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["MUTE"] {
		t.Errorf("charsteal reqPerms = %v, want MUTE (%v)", cmd.reqPerms, permissions.PermissionField["MUTE"])
	}
}

func setupCharStealTestClients(t *testing.T) (a, other *area.Area, caller, target, elsewhere *Client) {
	t.Helper()
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey", "Franziska von Karma"})

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a = area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)
	other = area.NewArea(area.AreaData{Name: "Basement"}, len(getCharacters()), 10, area.EviAny)

	caller = &Client{conn: &testConn{}, uid: 1, ipid: "ip-caller", char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	caller.SetArea(a)
	a.AddChar(0)

	target = &Client{conn: &testConn{}, uid: 2, ipid: "ip-target", char: 1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	target.SetArea(a)
	a.AddChar(1)

	elsewhere = &Client{conn: &testConn{}, uid: 3, ipid: "ip-else", char: 2, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	elsewhere.SetArea(other)
	other.AddChar(2)

	for _, c := range []*Client{caller, target, elsewhere} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	return
}

func TestCharStealTakesTargetsCharacter(t *testing.T) {
	a, _, caller, target, _ := setupCharStealTestClients(t)

	cmdCharSteal(caller, []string{"2"}, "")

	if caller.CharID() != 1 {
		t.Errorf("expected caller to end up on character 1 (stolen), got %d", caller.CharID())
	}
	if target.CharID() == 1 {
		t.Error("expected target to no longer hold character 1")
	}
	if target.CharID() == -1 {
		t.Error("expected target to be forced onto a random character, not spectator")
	}
	if !a.IsTaken(1) {
		t.Error("expected character 1 to remain taken (now by caller)")
	}
	if a.IsTaken(0) {
		t.Error("expected character 0 (caller's old character) to be freed")
	}
}

func TestCharStealRejectsSelfTarget(t *testing.T) {
	_, _, caller, _, _ := setupCharStealTestClients(t)

	cmdCharSteal(caller, []string{"1"}, "")

	if caller.CharID() != 0 {
		t.Errorf("expected caller's character to be unchanged when self-targeting, got %d", caller.CharID())
	}
}

func TestCharStealRejectsCrossAreaTarget(t *testing.T) {
	_, _, caller, _, elsewhere := setupCharStealTestClients(t)

	cmdCharSteal(caller, []string{"3"}, "")

	if caller.CharID() != 0 {
		t.Errorf("expected caller's character to be unchanged when target is in another area, got %d", caller.CharID())
	}
	if elsewhere.CharID() != 2 {
		t.Errorf("expected out-of-area target to be unaffected, got %d", elsewhere.CharID())
	}
}

func TestCharStealRejectsSpectatingTarget(t *testing.T) {
	a, _, caller, target, _ := setupCharStealTestClients(t)
	target.ChangeCharacter(-1)
	if a.IsTaken(1) {
		t.Fatal("test setup error: character 1 should be free after target moved to spectator")
	}

	cmdCharSteal(caller, []string{"2"}, "")

	if caller.CharID() != 0 {
		t.Errorf("expected caller's character to be unchanged when target is spectating, got %d", caller.CharID())
	}
}

func TestCharStealRejectsUnknownUID(t *testing.T) {
	_, _, caller, _, _ := setupCharStealTestClients(t)

	cmdCharSteal(caller, []string{"99"}, "")

	if caller.CharID() != 0 {
		t.Errorf("expected caller's character to be unchanged for an unknown UID, got %d", caller.CharID())
	}
}
