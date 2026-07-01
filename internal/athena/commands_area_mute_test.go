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

	for _, c := range []*Client{caller, regular, mod, cmByPerm, areaCM, elsewhere} {
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

	// Unmute lifts the mute on the regular player only.
	areaMuteAll(caller, true)
	if regular.Muted() != Unmuted {
		t.Fatalf("expected regular player to be unmuted after /area unmute, got %v", regular.Muted())
	}
}
