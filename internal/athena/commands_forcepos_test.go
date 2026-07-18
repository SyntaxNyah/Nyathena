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

func TestForcePosCommandRegistered(t *testing.T) {
	initCommands()
	cmd, ok := Commands["forcepos"]
	if !ok {
		t.Fatal("forcepos command is not registered in Commands map")
	}
	if cmd.handler == nil {
		t.Error("forcepos command has a nil handler")
	}
	if cmd.minArgs != 2 {
		t.Errorf("forcepos minArgs = %d, want 2", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["CM"] {
		t.Errorf("forcepos reqPerms = %v, want CM (%v)", cmd.reqPerms, permissions.PermissionField["CM"])
	}
}

func setupForcePosTestClients(t *testing.T) (a, other *area.Area, caller, target, bystander, elsewhere *Client) {
	t.Helper()
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a = area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	other = area.NewArea(area.AreaData{Name: "Basement"}, 5, 10, area.EviAny)

	caller = &Client{conn: &testConn{}, uid: 1, ipid: "ip-caller", area: a, pos: "def"}
	a.AddCM(caller.Uid())

	target = &Client{conn: &testConn{}, uid: 2, ipid: "ip-target", area: a, pos: "def"}
	bystander = &Client{conn: &testConn{}, uid: 3, ipid: "ip-bystander", area: a, pos: "def"}
	elsewhere = &Client{conn: &testConn{}, uid: 4, ipid: "ip-else", area: other, pos: "def"}

	for _, c := range []*Client{caller, target, bystander, elsewhere} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	return
}

func TestForcePosChangesTargetPosition(t *testing.T) {
	_, _, caller, target, bystander, _ := setupForcePosTestClients(t)

	cmdForcePos(caller, []string{"2", "wit"}, "")

	if target.Pos() != "wit" {
		t.Errorf("expected target position to be 'wit', got %q", target.Pos())
	}
	if bystander.Pos() != "def" {
		t.Errorf("expected bystander position to remain 'def', got %q", bystander.Pos())
	}
}

func TestForcePosIgnoresPlayersOutsideArea(t *testing.T) {
	_, _, caller, _, _, elsewhere := setupForcePosTestClients(t)

	cmdForcePos(caller, []string{"4", "wit"}, "")

	if elsewhere.Pos() != "def" {
		t.Errorf("expected player in a different area to be unaffected, got %q", elsewhere.Pos())
	}
}

func TestForcePosRejectsInvalidPosition(t *testing.T) {
	_, _, caller, target, _, _ := setupForcePosTestClients(t)

	cmdForcePos(caller, []string{"2", "notaposition"}, "")

	if target.Pos() != "def" {
		t.Errorf("expected target position to remain unchanged after invalid position, got %q", target.Pos())
	}
}

func TestForcePosAllTargetsEveryoneInArea(t *testing.T) {
	_, _, caller, target, bystander, elsewhere := setupForcePosTestClients(t)

	cmdForcePos(caller, []string{"all", "hld"}, "")

	if target.Pos() != "hld" {
		t.Errorf("expected target position to be 'hld', got %q", target.Pos())
	}
	if bystander.Pos() != "hld" {
		t.Errorf("expected bystander position to be 'hld', got %q", bystander.Pos())
	}
	if caller.Pos() != "hld" {
		t.Errorf("expected caller position to be 'hld', got %q", caller.Pos())
	}
	if elsewhere.Pos() != "def" {
		t.Errorf("expected player in a different area to be unaffected by 'all', got %q", elsewhere.Pos())
	}
}

func TestForcePosCommaSeparatedUIDs(t *testing.T) {
	_, _, caller, target, bystander, _ := setupForcePosTestClients(t)

	cmdForcePos(caller, []string{"2,3", "jud"}, "")

	if target.Pos() != "jud" {
		t.Errorf("expected target position to be 'jud', got %q", target.Pos())
	}
	if bystander.Pos() != "jud" {
		t.Errorf("expected bystander position to be 'jud', got %q", bystander.Pos())
	}
}
