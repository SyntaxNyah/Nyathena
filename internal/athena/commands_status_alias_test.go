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
)

// TestGCommandIsGlobalAlias verifies /g is registered as an alias of /global,
// sharing the same handler and permission requirements.
func TestGCommandIsGlobalAlias(t *testing.T) {
	initCommands()

	global, ok := Commands["global"]
	if !ok {
		t.Fatal("global command is not registered in Commands map")
	}
	g, ok := Commands["g"]
	if !ok {
		t.Fatal("g command is not registered in Commands map")
	}
	if g.minArgs != global.minArgs {
		t.Errorf("g minArgs = %d, want %d (matching /global)", g.minArgs, global.minArgs)
	}
	if g.reqPerms != global.reqPerms {
		t.Errorf("g reqPerms = %v, want %v (matching /global)", g.reqPerms, global.reqPerms)
	}
	if g.category != global.category {
		t.Errorf("g category = %q, want %q (matching /global)", g.category, global.category)
	}
}

// TestStatusLfpAliasesLookingForPlayers verifies /status lfp sets the same
// area status as /status looking-for-players.
func TestStatusLfpAliasesLookingForPlayers(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	caller := &Client{conn: &testConn{}, uid: 1, ipid: "ip-caller", area: a}
	clients.AddClient(caller)
	clients.RegisterUID(caller)

	cmdStatus(caller, []string{"lfp"}, "")
	if a.Status() != area.StatusPlayers {
		t.Errorf("expected /status lfp to set StatusPlayers, got %v", a.Status())
	}

	a.SetStatus(area.StatusIdle)
	cmdStatus(caller, []string{"looking-for-players"}, "")
	if a.Status() != area.StatusPlayers {
		t.Errorf("expected /status looking-for-players to set StatusPlayers, got %v", a.Status())
	}
}

// TestStatusInvalidStillRejected verifies an unrecognized status string is
// still rejected after adding the lfp shorthand.
func TestStatusInvalidStillRejected(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	a.SetStatus(area.StatusIdle)
	caller := &Client{conn: &testConn{}, uid: 1, ipid: "ip-caller", area: a}
	clients.AddClient(caller)
	clients.RegisterUID(caller)

	cmdStatus(caller, []string{"not-a-status"}, "")
	if a.Status() != area.StatusIdle {
		t.Errorf("expected invalid status to leave area status unchanged, got %v", a.Status())
	}
}
