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
	"fmt"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestSummonCommandRemoved pins that /summon is gone from the registry after
// initCommands, so it can no longer be dispatched or shown in help.
func TestSummonCommandRemoved(t *testing.T) {
	initCommands()
	if _, ok := Commands["summon"]; ok {
		t.Fatal("summon command should have been removed from the registry")
	}
}

// newMoveClient registers a fresh client with the given UID in the given area
// and returns its conn (for assertions) along with the client itself.
func newMoveClient(uid int, a *area.Area, perms uint64) (*captureConn, *Client) {
	conn := &captureConn{}
	c := &Client{conn: conn, uid: uid, ipid: fmt.Sprintf("ip-%d", uid), char: -1, area: a, perms: perms}
	clients.AddClient(c)
	clients.RegisterUID(c)
	return conn, c
}

func TestMoveRefusesSafeToDangerous(t *testing.T) {
	newTestClients(t)
	safe := makeTestArea("Safe Haven")
	safe.SetPunishmentSafe(true)
	dangerous := makeTestArea("Trashpit")
	t.Cleanup(setupTestAreas([]*area.Area{safe, dangerous}))

	modConn, mod := newMoveClient(1, dangerous, permissions.PermissionField["MOVE_USERS"])
	targetConn, target := newMoveClient(2, safe, 0)

	// /move -u 2 1 -> move target (uid 2) into area index 1 (Trashpit).
	cmdMove(mod, []string{"-u", "2", "1"}, "usage")

	if target.Area() != safe {
		t.Fatalf("target should have stayed in the safe area; got %v", target.Area().Name())
	}
	if !strings.Contains(modConn.String(), "punishment-free area") {
		t.Fatalf("mod should be told the move was blocked by a punishment-free area; got %q", modConn.String())
	}
	if strings.Contains(targetConn.String(), "moved") {
		t.Fatalf("target should not have received a move notification; got %q", targetConn.String())
	}
}

func TestMoveAllowsSafeToSafe(t *testing.T) {
	newTestClients(t)
	safe1 := makeTestArea("Safe Haven")
	safe1.SetPunishmentSafe(true)
	safe2 := makeTestArea("Safe Lounge")
	safe2.SetPunishmentSafe(true)
	t.Cleanup(setupTestAreas([]*area.Area{safe1, safe2}))

	_, mod := newMoveClient(1, safe1, permissions.PermissionField["MOVE_USERS"])
	_, target := newMoveClient(2, safe1, 0)

	cmdMove(mod, []string{"-u", "2", "1"}, "usage")

	if target.Area() != safe2 {
		t.Fatalf("target should have moved between two safe areas; got %v", target.Area().Name())
	}
}

func TestMoveAllowsDangerousToDangerous(t *testing.T) {
	newTestClients(t)
	d1 := makeTestArea("Trashpit")
	d2 := makeTestArea("Sewer")
	t.Cleanup(setupTestAreas([]*area.Area{d1, d2}))

	_, mod := newMoveClient(1, d1, permissions.PermissionField["MOVE_USERS"])
	_, target := newMoveClient(2, d1, 0)

	cmdMove(mod, []string{"-u", "2", "1"}, "usage")

	if target.Area() != d2 {
		t.Fatalf("target should have moved between two non-safe areas; got %v", target.Area().Name())
	}
}

func TestMoveAllowsDangerousToSafe(t *testing.T) {
	newTestClients(t)
	dangerous := makeTestArea("Trashpit")
	safe := makeTestArea("Safe Haven")
	safe.SetPunishmentSafe(true)
	t.Cleanup(setupTestAreas([]*area.Area{dangerous, safe}))

	_, mod := newMoveClient(1, dangerous, permissions.PermissionField["MOVE_USERS"])
	_, target := newMoveClient(2, dangerous, 0)

	cmdMove(mod, []string{"-u", "2", "1"}, "usage")

	if target.Area() != safe {
		t.Fatalf("target should have moved into the safe area; got %v", target.Area().Name())
	}
}

func TestMoveSelfLeavesSafeArea(t *testing.T) {
	newTestClients(t)
	safe := makeTestArea("Safe Haven")
	safe.SetPunishmentSafe(true)
	dangerous := makeTestArea("Trashpit")
	t.Cleanup(setupTestAreas([]*area.Area{safe, dangerous}))

	_, client := newMoveClient(1, safe, 0)

	// /move 1 -> move self to area index 1 (Trashpit). Self-move is voluntary,
	// so it is not blocked by the punishment-free-area gate.
	cmdMove(client, []string{"1"}, "usage")

	if client.Area() != dangerous {
		t.Fatalf("self-move should still be allowed out of a safe area; got %v", client.Area().Name())
	}
}
