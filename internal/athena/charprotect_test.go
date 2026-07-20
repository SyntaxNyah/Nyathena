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

func TestCharProtectCommandRegistered(t *testing.T) {
	initCommands()
	cmd, ok := Commands["charprotect"]
	if !ok {
		t.Fatal("charprotect command is not registered in Commands map")
	}
	if cmd.handler == nil {
		t.Error("charprotect command has a nil handler")
	}
	if cmd.minArgs != 0 {
		t.Errorf("charprotect minArgs = %d, want 0", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["MUTE"] {
		t.Errorf("charprotect reqPerms = %v, want MUTE (%v)", cmd.reqPerms, permissions.PermissionField["MUTE"])
	}
}

func TestCmdCharProtectToggle(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})

	a := area.NewArea(area.AreaData{}, len(getCharacters()), 10, area.EviAny)
	client := &Client{conn: &testConn{}, uid: 1, char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	client.SetArea(a)
	a.AddChar(0)

	if client.CharProtectEnabled() {
		t.Fatal("expected character protection to default to off")
	}

	cmdCharProtect(client, []string{"on"}, "usage")
	if !client.CharProtectEnabled() {
		t.Error("expected character protection to be enabled after /charprotect on")
	}

	cmdCharProtect(client, []string{"off"}, "usage")
	if client.CharProtectEnabled() {
		t.Error("expected character protection to be disabled after /charprotect off")
	}
}

func TestCmdCharProtectRejectsWhileSpectating(t *testing.T) {
	client := &Client{conn: &testConn{}, uid: 1, char: -1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}

	cmdCharProtect(client, []string{"on"}, "usage")

	if client.CharProtectEnabled() {
		t.Error("expected character protection to stay off when the caller is spectating")
	}
}

func setupCharProtectTestArea(t *testing.T) (a *area.Area, protector, holder *Client) {
	t.Helper()
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey"})

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a = area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	// protector is mid-transition: still "holding" character 0 (their claim)
	// but not yet counted in area a's taken map — mirrors the moment
	// resolveCharProtectOnJoin is invoked from ChangeArea, after the client
	// has left their old area but before JoinArea adds them to the new one.
	protector = &Client{conn: &testConn{}, uid: 1, char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}

	holder = &Client{conn: &testConn{}, uid: 2, char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	holder.SetArea(a)
	a.AddChar(0)

	clients.AddClient(protector)
	clients.RegisterUID(protector)
	clients.AddClient(holder)
	clients.RegisterUID(holder)
	return
}

func TestResolveCharProtectBumpsHolder(t *testing.T) {
	a, protector, holder := setupCharProtectTestArea(t)
	protector.SetCharProtectEnabled(true)

	ok := resolveCharProtectOnJoin(protector, a)

	if !ok {
		t.Fatal("expected resolveCharProtectOnJoin to report the slot was freed")
	}
	if holder.CharID() == 0 {
		t.Error("expected holder to be bumped off character 0")
	}
	if holder.CharID() == -1 {
		t.Error("expected holder to be bumped to a random character, not spectator")
	}
	if a.IsTaken(0) {
		t.Error("expected character 0 to be freed for the protector to claim")
	}
}

func TestResolveCharProtectNoopWhenDisabled(t *testing.T) {
	a, protector, holder := setupCharProtectTestArea(t)
	// protection left off (default)

	ok := resolveCharProtectOnJoin(protector, a)

	if ok {
		t.Error("expected resolveCharProtectOnJoin to no-op when protection is disabled")
	}
	if holder.CharID() != 0 {
		t.Error("expected holder to be unaffected when protection is disabled")
	}
}

func TestResolveCharProtectNoopWhenNoHolder(t *testing.T) {
	a, protector, holder := setupCharProtectTestArea(t)
	protector.SetCharProtectEnabled(true)
	// Free the slot so nobody actually holds character 0.
	holder.ChangeCharacter(1)

	ok := resolveCharProtectOnJoin(protector, a)

	if ok {
		t.Error("expected resolveCharProtectOnJoin to no-op when nobody holds the character")
	}
}

func TestResolveCharProtectNoopWhenSpectating(t *testing.T) {
	a, protector, holder := setupCharProtectTestArea(t)
	protector.SetCharProtectEnabled(true)
	protector.SetCharID(-1)

	ok := resolveCharProtectOnJoin(protector, a)

	if ok {
		t.Error("expected resolveCharProtectOnJoin to no-op when the protector is spectating")
	}
	if holder.CharID() != 0 {
		t.Error("expected holder to be unaffected when the protector is spectating")
	}
}

func TestResolveCharProtectNoopWhenNoFreeCharForHolder(t *testing.T) {
	a, protector, holder := setupCharProtectTestArea(t)
	protector.SetCharProtectEnabled(true)
	// Take every other character so the holder has nowhere free to go.
	a.AddChar(1)
	a.AddChar(2)

	ok := resolveCharProtectOnJoin(protector, a)

	if ok {
		t.Error("expected resolveCharProtectOnJoin to no-op when the holder has no free character to move to")
	}
	if holder.CharID() != 0 {
		t.Error("expected holder to remain on character 0 when there's nowhere to bump them")
	}
	if !a.IsTaken(0) {
		t.Error("expected character 0 to remain taken when the bump could not happen")
	}
}
