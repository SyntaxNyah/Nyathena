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
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// newRoomClient registers a client standing in a with the given UID/OOC name and
// returns it alongside its captureConn for output assertions.
func newRoomClient(a *area.Area, uid int, oocName string) (*Client, *captureConn) {
	conn := &captureConn{}
	c := &Client{
		conn:    conn,
		uid:     uid,
		ipid:    fmt.Sprintf("ip-room-%d", uid),
		area:    a,
		char:    -1,
		oocName: oocName,
		pair:    ClientPairInfo{wanted_id: -1},
	}
	clients.AddClient(c)
	clients.RegisterUID(c)
	return c, conn
}

// TestRoomInviteSendsOOCRequest verifies /room invite <uid> sends the target an
// OOC request naming the inviter by their showname/OOC name.
func TestRoomInviteSendsOOCRequest(t *testing.T) {
	newTestClients(t)
	lobby := makeTestArea("Lobby")
	locked := makeTestArea("Locked Room")
	locked.SetLock(area.LockLocked)
	t.Cleanup(setupTestAreas([]*area.Area{lobby, locked}))

	inviter, _ := newRoomClient(locked, 1, "Alice")
	_, inviteeConn := newRoomClient(lobby, 2, "")

	cmdRoom(inviter, []string{"invite", "2"}, "usage")

	if !strings.Contains(inviteeConn.String(), "Alice has invited you to their area") {
		t.Fatalf("invitee should receive the invite request, got %q", inviteeConn.String())
	}
}

// TestRoomInviteCooldown verifies a scripted flood of invites is throttled: a
// second invite inside the cooldown window is refused.
func TestRoomInviteCooldown(t *testing.T) {
	newTestClients(t)
	lobby := makeTestArea("Lobby")
	t.Cleanup(setupTestAreas([]*area.Area{lobby}))

	inviter, inviterConn := newRoomClient(lobby, 1, "Alice")
	invitee, _ := newRoomClient(lobby, 2, "")

	cmdRoom(inviter, []string{"invite", "2"}, "usage")
	cmdRoom(inviter, []string{"invite", "2"}, "usage")

	if !strings.Contains(inviterConn.String(), "inviting too quickly") {
		t.Fatalf("second invite within cooldown should be refused, got %q", inviterConn.String())
	}
	_ = invitee
}

// TestRoomInviteAcceptMovesIntoLockedArea verifies the accept handshake moves the
// invitee straight into a locked area without requiring them to be a CM.
func TestRoomInviteAcceptMovesIntoLockedArea(t *testing.T) {
	newTestClients(t)
	lobby := makeTestArea("Lobby")
	locked := makeTestArea("Locked Room")
	locked.SetLock(area.LockLocked)
	t.Cleanup(setupTestAreas([]*area.Area{lobby, locked}))

	inviter, _ := newRoomClient(locked, 1, "Alice")
	invitee, _ := newRoomClient(lobby, 2, "Bob")

	cmdRoom(inviter, []string{"invite", "2"}, "usage")
	cmdRoom(invitee, []string{"invite", "accept"}, "usage")

	if invitee.Area() != locked {
		t.Fatalf("invitee should have moved into the locked area, got %v", invitee.Area().Name())
	}
}

// TestRoomInviteAcceptNoPendingInvite verifies accept without a pending invite is
// rejected with a clear message.
func TestRoomInviteAcceptNoPendingInvite(t *testing.T) {
	newTestClients(t)
	lobby := makeTestArea("Lobby")
	t.Cleanup(setupTestAreas([]*area.Area{lobby}))

	invitee, inviteeConn := newRoomClient(lobby, 2, "Bob")

	cmdRoom(invitee, []string{"invite", "accept"}, "usage")

	if !strings.Contains(inviteeConn.String(), "no pending area invite") {
		t.Fatalf("accept with no pending invite should be rejected, got %q", inviteeConn.String())
	}
}

// TestRoomInviteAcceptRefusedWhenInviterDisconnected verifies an invite dies with
// its inviter: once they disconnect, the invite can no longer be accepted.
func TestRoomInviteAcceptRefusedWhenInviterDisconnected(t *testing.T) {
	newTestClients(t)
	lobby := makeTestArea("Lobby")
	locked := makeTestArea("Locked Room")
	locked.SetLock(area.LockLocked)
	t.Cleanup(setupTestAreas([]*area.Area{lobby, locked}))

	inviter, _ := newRoomClient(locked, 1, "Alice")
	invitee, inviteeConn := newRoomClient(lobby, 2, "Bob")

	cmdRoom(inviter, []string{"invite", "2"}, "usage")
	clients.RemoveClient(inviter)
	cmdRoom(invitee, []string{"invite", "accept"}, "usage")

	if !strings.Contains(inviteeConn.String(), "no longer online") {
		t.Fatalf("accept after the inviter disconnected should be rejected, got %q", inviteeConn.String())
	}
	if invitee.Area() != lobby {
		t.Fatalf("invitee should have stayed in the lobby, got %v", invitee.Area().Name())
	}
}

// TestRoomInviteAcceptRefusedWhenInviterLeftArea verifies the "person has to be
// in that initial room" rule: if the inviter left the area they invited from, the
// invite is void.
func TestRoomInviteAcceptRefusedWhenInviterLeftArea(t *testing.T) {
	newTestClients(t)
	lobby := makeTestArea("Lobby")
	locked := makeTestArea("Locked Room")
	locked.SetLock(area.LockLocked)
	t.Cleanup(setupTestAreas([]*area.Area{lobby, locked}))

	inviter, _ := newRoomClient(locked, 1, "Alice")
	invitee, inviteeConn := newRoomClient(lobby, 2, "Bob")

	cmdRoom(inviter, []string{"invite", "2"}, "usage")
	inviter.ChangeArea(lobby)
	cmdRoom(invitee, []string{"invite", "accept"}, "usage")

	if !strings.Contains(inviteeConn.String(), "has left their area") {
		t.Fatalf("accept after the inviter left should be rejected, got %q", inviteeConn.String())
	}
	if invitee.Area() != lobby {
		t.Fatalf("invitee should have stayed in the lobby, got %v", invitee.Area().Name())
	}
}
