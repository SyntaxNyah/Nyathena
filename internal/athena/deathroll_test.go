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
	"time"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// deathrollTestSetup installs a fresh client list and clears the global lobby.
func deathrollTestSetup(t *testing.T) {
	t.Helper()
	newTestClients(t)
	deathrollMu.Lock()
	deathrollCurrent = nil
	deathrollMu.Unlock()
	t.Cleanup(func() {
		deathrollMu.Lock()
		deathrollCurrent = nil
		deathrollMu.Unlock()
	})
}

// deathrollTestClient creates a registered client with a capturing connection,
// an OOC name, and a non-nil area (addToBuffer dereferences it).
func deathrollTestClient(t *testing.T, uid int, name string) (*Client, *captureConn) {
	t.Helper()
	conn := &captureConn{}
	c := &Client{
		conn:     conn,
		uid:      uid,
		ipid:     fmt.Sprintf("ip-deathroll-%d", uid),
		oocName:  name,
		showname: name,
		char:     -1,
		area:     makeTestArea("Deathroll Test"),
	}
	clients.AddClient(c)
	clients.RegisterUID(c)
	t.Cleanup(func() { clients.RemoveClient(c) })
	return c, conn
}

// deathrollTestGame opens a lobby, joins guest, and starts the game. It returns
// the current player and the other player.
func deathrollTestGame(t *testing.T, host, guest *Client) (current, other *Client) {
	t.Helper()
	cmdDeathroll(host, []string{"host"}, deathrollUsage)
	cmdDeathroll(guest, []string{"join"}, deathrollUsage)
	cmdDeathroll(host, []string{"start"}, deathrollUsage)

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil || st.phase != deathrollInProgress || len(st.order) != 2 {
		deathrollMu.Unlock()
		t.Fatalf("game did not start correctly: %+v", st)
	}
	curUID := st.order[st.turn].uid
	deathrollMu.Unlock()

	current, other = host, guest
	if curUID != host.Uid() {
		current, other = guest, host
	}
	return current, other
}

func TestDeathrollParseLimit(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    int
		wantErr bool
	}{
		{"default", nil, deathrollDefaultLimit, false},
		{"plain", []string{"5000"}, 5000, false},
		{"limit word", []string{"limit", "5000"}, 5000, false},
		{"short flag", []string{"-l", "5000"}, 5000, false},
		{"long flag", []string{"-limit", "5000"}, 5000, false},
		{"too small", []string{"1"}, 0, true},
		{"too large", []string{"1000001"}, 0, true},
		{"not a number", []string{"abc"}, 0, true},
		{"missing value", []string{"limit"}, 0, true},
		{"extra args", []string{"100", "200"}, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := deathrollParseLimit(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got limit %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("limit = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDeathrollLobbyLifecycle(t *testing.T) {
	deathrollTestSetup(t)
	host, hostConn := deathrollTestClient(t, 1, "Host")
	guest, guestConn := deathrollTestClient(t, 2, "Guest")
	late, lateConn := deathrollTestClient(t, 3, "Late")
	conns := map[int]*captureConn{host.Uid(): hostConn, guest.Uid(): guestConn, late.Uid(): lateConn}

	cmdDeathroll(host, []string{"host", "5000"}, deathrollUsage)

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		t.Fatal("host did not create a lobby")
	}
	if st.hostUID != host.Uid() || st.startingLimit != 5000 || st.upperBound != 5000 {
		deathrollMu.Unlock()
		t.Fatalf("unexpected lobby state: %+v", st)
	}
	if len(st.players) != 1 || st.players[0].uid != host.Uid() {
		deathrollMu.Unlock()
		t.Fatalf("host should be auto-joined: %+v", st.players)
	}
	deathrollMu.Unlock()

	for _, want := range []string{"HOST GUIDE", "1d5000", "one minute", "/deathroll join"} {
		if !strings.Contains(hostConn.String(), want) {
			t.Errorf("host guide/announcement missing %q", want)
		}
	}

	// A second host is rejected while the first lobby is open.
	cmdDeathroll(guest, []string{"host"}, deathrollUsage)
	if !strings.Contains(guestConn.String(), "already open") {
		t.Errorf("second host was not rejected; got %q", guestConn.String())
	}

	cmdDeathroll(guest, []string{"join"}, deathrollUsage)
	deathrollMu.Lock()
	if len(st.players) != 2 {
		deathrollMu.Unlock()
		t.Fatalf("join did not add the guest: %+v", st.players)
	}
	deathrollMu.Unlock()
	if !strings.Contains(guestConn.String(), "joined the Deathroll lobby") {
		t.Errorf("join confirmation missing; got %q", guestConn.String())
	}

	cmdDeathroll(guest, []string{"join"}, deathrollUsage)
	if !strings.Contains(guestConn.String(), "already joined") {
		t.Errorf("duplicate join was not rejected; got %q", guestConn.String())
	}

	cmdDeathroll(guest, []string{"start"}, deathrollUsage)
	if !strings.Contains(guestConn.String(), "Only the host") {
		t.Errorf("non-host start was not rejected; got %q", guestConn.String())
	}

	cmdDeathroll(host, []string{"start"}, deathrollUsage)
	deathrollMu.Lock()
	if st.phase != deathrollInProgress || len(st.order) != 2 {
		deathrollMu.Unlock()
		t.Fatalf("start did not enter progress: %+v", st)
	}
	firstUID := st.order[st.turn].uid
	deathrollMu.Unlock()

	cmdDeathroll(late, []string{"join"}, deathrollUsage)
	if !strings.Contains(lateConn.String(), "already started") {
		t.Errorf("late join was not rejected; got %q", lateConn.String())
	}

	nonCurrent := host
	if firstUID == host.Uid() {
		nonCurrent = guest
	}
	cmdDeathroll(nonCurrent, []string{"roll"}, deathrollUsage)
	if !strings.Contains(conns[nonCurrent.Uid()].String(), "not your turn") {
		t.Errorf("out-of-turn roll was not rejected; got %q", conns[nonCurrent.Uid()].String())
	}
}

func TestDeathrollRollOfOneEndsGameAndLocksPunishment(t *testing.T) {
	deathrollTestSetup(t)
	oldRoll := deathrollRollDie
	deathrollRollDie = func(int) int { return 1 }
	t.Cleanup(func() { deathrollRollDie = oldRoll })

	host, hostConn := deathrollTestClient(t, 10, "Host")
	guest, guestConn := deathrollTestClient(t, 11, "Guest")
	conns := map[int]*captureConn{host.Uid(): hostConn, guest.Uid(): guestConn}

	current, _ := deathrollTestGame(t, host, guest)
	cmdDeathroll(current, []string{"roll"}, deathrollUsage)

	deathrollMu.Lock()
	stillOpen := deathrollCurrent != nil
	deathrollMu.Unlock()
	if stillOpen {
		t.Fatal("a roll of 1 did not end the game")
	}

	var got PunishmentType
	for _, p := range current.Punishments() {
		if p.reason == "Deathroll: rolled a 1" {
			got = p.punishmentType
		}
	}
	if got == PunishmentNone {
		t.Fatal("loser did not receive the Deathroll maso punishment")
	}
	if until := current.PunishmentLockedUntil(got); !until.After(time.Now()) {
		t.Errorf("Deathroll punishment is not locked: until=%v", until)
	}
	if out := conns[current.Uid()].String(); !strings.Contains(out, "locked") {
		t.Errorf("loser/end message does not mention the lock: %q", out)
	}
	if out := conns[current.Uid()].String(); !strings.Contains(out, "Winner") {
		t.Errorf("end message does not announce a winner: %q", out)
	}
}

func TestDeathrollRollAdvancesUpperBoundAndTurn(t *testing.T) {
	deathrollTestSetup(t)
	oldRoll := deathrollRollDie
	deathrollRollDie = func(int) int { return 7 }
	t.Cleanup(func() { deathrollRollDie = oldRoll })

	host, hostConn := deathrollTestClient(t, 20, "Host")
	guest, guestConn := deathrollTestClient(t, 21, "Guest")
	conns := map[int]*captureConn{host.Uid(): hostConn, guest.Uid(): guestConn}

	current, other := deathrollTestGame(t, host, guest)
	cmdDeathroll(current, []string{"roll"}, deathrollUsage)

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		t.Fatal("game ended after a non-1 roll")
	}
	if st.upperBound != 7 {
		deathrollMu.Unlock()
		t.Errorf("upper bound = %d, want 7", st.upperBound)
	}
	if st.lastBound != deathrollDefaultLimit || st.lastRoll != 7 {
		deathrollMu.Unlock()
		t.Errorf("last roll metadata = %d/%d, want 7/%d", st.lastRoll, st.lastBound, deathrollDefaultLimit)
	}
	if st.order[st.turn].uid != other.Uid() {
		deathrollMu.Unlock()
		t.Errorf("turn did not advance to the other player")
	}
	deathrollMu.Unlock()

	if out := conns[current.Uid()].String(); !strings.Contains(out, "Next up") {
		t.Errorf("roll announcement missing the next player: %q", out)
	}
}

func TestDeathrollStartRequiresTwoPlayers(t *testing.T) {
	deathrollTestSetup(t)
	host, hostConn := deathrollTestClient(t, 30, "Host")

	cmdDeathroll(host, []string{"host"}, deathrollUsage)
	cmdDeathroll(host, []string{"start"}, deathrollUsage)

	if !strings.Contains(hostConn.String(), "at least 2") {
		t.Errorf("solo start was not rejected; got %q", hostConn.String())
	}
	deathrollMu.Lock()
	phase := deathrollOpen
	if deathrollCurrent != nil {
		phase = deathrollCurrent.phase
	}
	deathrollMu.Unlock()
	if phase != deathrollOpen {
		t.Errorf("solo start changed the phase to %v", phase)
	}
}

func TestDeathrollCancelHostOnly(t *testing.T) {
	deathrollTestSetup(t)
	host, hostConn := deathrollTestClient(t, 40, "Host")
	guest, guestConn := deathrollTestClient(t, 41, "Guest")

	cmdDeathroll(host, []string{"host"}, deathrollUsage)
	cmdDeathroll(guest, []string{"join"}, deathrollUsage)

	cmdDeathroll(guest, []string{"cancel"}, deathrollUsage)
	if !strings.Contains(guestConn.String(), "Only the host") {
		t.Errorf("non-host cancel was not rejected; got %q", guestConn.String())
	}
	deathrollMu.Lock()
	if deathrollCurrent == nil {
		deathrollMu.Unlock()
		t.Fatal("non-host cancel removed the lobby")
	}
	deathrollMu.Unlock()

	cmdDeathroll(host, []string{"cancel"}, deathrollUsage)
	deathrollMu.Lock()
	stillOpen := deathrollCurrent != nil
	deathrollMu.Unlock()
	if stillOpen {
		t.Fatal("host cancel did not clear the lobby")
	}
	if !strings.Contains(hostConn.String(), "cancelled") {
		t.Errorf("cancel announcement missing; got %q", hostConn.String())
	}
}

func TestDeathrollDisconnectForfeitsAndAwardsWin(t *testing.T) {
	deathrollTestSetup(t)
	host, hostConn := deathrollTestClient(t, 50, "Host")
	guest, guestConn := deathrollTestClient(t, 51, "Guest")
	conns := map[int]*captureConn{host.Uid(): hostConn, guest.Uid(): guestConn}

	current, other := deathrollTestGame(t, host, guest)
	handleDeathrollDisconnect(current)

	deathrollMu.Lock()
	ended := deathrollCurrent == nil
	deathrollMu.Unlock()
	if !ended {
		t.Fatal("forfeiting the current player with only one active player left should end the game")
	}
	if out := conns[other.Uid()].String(); !strings.Contains(out, "wins Deathroll by default") {
		t.Errorf("survivor win announcement missing; got %q", out)
	}
}

func TestDeathrollHostDisconnectCancelsOpenLobby(t *testing.T) {
	deathrollTestSetup(t)
	host, _ := deathrollTestClient(t, 60, "Host")
	guest, guestConn := deathrollTestClient(t, 61, "Guest")

	cmdDeathroll(host, []string{"host"}, deathrollUsage)
	cmdDeathroll(guest, []string{"join"}, deathrollUsage)

	handleDeathrollDisconnect(host)

	deathrollMu.Lock()
	stillOpen := deathrollCurrent != nil
	deathrollMu.Unlock()
	if stillOpen {
		t.Fatal("host disconnect did not cancel the open lobby")
	}
	if !strings.Contains(guestConn.String(), "cancelled") {
		t.Errorf("host-disconnect cancellation announcement missing; got %q", guestConn.String())
	}
}

func TestDeathrollRegisteredUnderMinigames(t *testing.T) {
	initCommands()
	cmd, ok := Commands["deathroll"]
	if !ok {
		t.Fatal("/deathroll is not registered")
	}
	if cmd.category != "minigames" {
		t.Errorf("category = %q, want minigames", cmd.category)
	}
	if cmd.reqPerms != permissions.PermissionField["NONE"] {
		t.Error("/deathroll should be usable by everyone (reqPerms NONE)")
	}
}
