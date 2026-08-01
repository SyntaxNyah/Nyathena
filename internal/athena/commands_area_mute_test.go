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
	"os"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// areaMuteAll persists mutes through the db package, so a temp DB is required.
func setupAreaMuteTestDB(t *testing.T) func() {
	t.Helper()
	tmp, err := os.CreateTemp("", "athena-areamute-*.db")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmp.Close()
	db.DBPath = tmp.Name()
	if err := db.Open(); err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return func() {
		db.Close()
		os.Remove(tmp.Name())
	}
}

func TestAreaMuteExemptsStaffAndOtherAreas(t *testing.T) {
	defer setupAreaMuteTestDB(t)()

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	other := area.NewArea(area.AreaData{Name: "Basement"}, 5, 10, area.EviAny)

	mutePerm := permissions.PermissionField["MUTE"]
	cmPerm := permissions.PermissionField["CM"]

	// The caller is an area CM (no global perms).
	caller := &Client{conn: &testConn{}, uid: 1, ipid: "ip-caller", area: a}
	a.AddCM(caller.Uid())

	regular := &Client{conn: &testConn{}, uid: 2, ipid: "ip-reg", area: a}
	mod := &Client{conn: &testConn{}, uid: 3, ipid: "ip-mod", area: a, perms: mutePerm}
	cmByPerm := &Client{conn: &testConn{}, uid: 4, ipid: "ip-cm", area: a, perms: cmPerm}
	areaCM := &Client{conn: &testConn{}, uid: 5, ipid: "ip-acm", area: a}
	a.AddCM(areaCM.Uid())
	elsewhere := &Client{conn: &testConn{}, uid: 6, ipid: "ip-else", area: other}
	// A non-staff player carrying a separate individual mute — /area unmute
	// must leave this one alone (it only reverses its own ICOOCMuted state).
	individuallyMuted := &Client{conn: &testConn{}, uid: 7, ipid: "ip-ind", area: a, muted: ICMuted}

	for _, c := range []*Client{caller, regular, mod, cmByPerm, areaCM, elsewhere, individuallyMuted} {
		clients.AddClient(c)
	}

	areaMuteAll(caller, false)

	if regular.Muted() != ICOOCMuted {
		t.Fatalf("expected regular player to be ICOOCMuted, got %v", regular.Muted())
	}
	for name, c := range map[string]*Client{"caller": caller, "mod": mod, "cmByPerm": cmByPerm, "areaCM": areaCM, "elsewhere": elsewhere} {
		if c.Muted() != Unmuted {
			t.Fatalf("expected %v to remain unmuted, got %v", name, c.Muted())
		}
	}

	// Unmute reverses /area mute: the ICOOCMuted regular player can speak
	// again, but a separate individual mute is left intact.
	areaMuteAll(caller, true)
	if regular.Muted() != Unmuted {
		t.Fatalf("expected regular player to be unmuted after /area unmute, got %v", regular.Muted())
	}
	if individuallyMuted.Muted() != ICMuted {
		t.Fatalf("expected individually-muted player to keep their ICMuted state after /area unmute, got %v", individuallyMuted.Muted())
	}
}

// TestAreaMuteLiftedOnDeparture verifies that /area mute is scoped to the
// room it was applied in: a muted player who moves to a different area is
// automatically unmuted (both in memory and in the persisted DB row),
// instead of the mute silently following them around the rest of the server
// until a moderator manually /unmutes them.
func TestAreaMuteLiftedOnDeparture(t *testing.T) {
	defer setupAreaMuteTestDB(t)()

	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	origAreas := areas
	t.Cleanup(func() { areas = origAreas })

	courtroom := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)
	lobby := area.NewArea(area.AreaData{Name: "Lobby"}, len(getCharacters()), 10, area.EviAny)
	areas = []*area.Area{courtroom, lobby}

	caller := &Client{conn: &testConn{}, uid: 1, ipid: "ip-caller", area: courtroom, char: -1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	courtroom.AddCM(caller.Uid())
	target := &Client{conn: &testConn{}, uid: 2, ipid: "ip-target", area: courtroom, char: -1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	for _, c := range []*Client{caller, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	areaMuteAll(caller, false)
	if target.Muted() != ICOOCMuted {
		t.Fatalf("expected target to be ICOOCMuted after /area mute, got %v", target.Muted())
	}
	if target.AreaMuteOrigin() != courtroom {
		t.Fatal("expected target's area mute origin to be the courtroom")
	}

	if !target.ChangeArea(lobby) {
		t.Fatal("target should be able to move to the lobby")
	}

	if target.Muted() != Unmuted {
		t.Fatalf("expected target to be auto-unmuted after leaving the muting area, got %v", target.Muted())
	}
	if target.AreaMuteOrigin() != nil {
		t.Fatal("expected area mute origin to be cleared after departure")
	}

	punishments, err := db.GetPunishments(target.Ipid())
	if err != nil {
		t.Fatalf("GetPunishments: %v", err)
	}
	for _, p := range punishments {
		if p.Kind == db.PunishKindMute {
			t.Fatal("expected the persisted mute to be removed after the target left the area")
		}
	}
}
