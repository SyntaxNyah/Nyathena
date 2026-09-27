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
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// newRoomMotdClient builds a client standing in a, optionally as an area CM.
func newRoomMotdClient(a *area.Area, uid int, isCM bool) (*Client, *captureConn) {
	conn := &captureConn{}
	c := &Client{conn: conn, uid: uid, ipid: "ip-roommotd", area: a, char: -1, pair: ClientPairInfo{wanted_id: -1}}
	if isCM {
		a.AddCM(uid)
	}
	clients.AddClient(c)
	clients.RegisterUID(c)
	return c, conn
}

// TestRoomMotdPermissionGate verifies a non-CM cannot set the room motd.
func TestRoomMotdPermissionGate(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	nonCM, _ := newRoomMotdClient(a, 1, false)
	t.Cleanup(func() { clients.RemoveClient(nonCM) })

	cmdRoomMotd(nonCM, []string{"hello"}, "")

	if a.Motd() != "" {
		t.Errorf("non-CM set the motd: got %q, want empty", a.Motd())
	}
}

// TestRoomMotdSetOverwriteClear verifies a CM can set, overwrite and clear the
// room motd.
func TestRoomMotdSetOverwriteClear(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	cm, _ := newRoomMotdClient(a, 2, true)
	t.Cleanup(func() { clients.RemoveClient(cm) })

	cmdRoomMotd(cm, []string{"welcome", "to", "the", "room"}, "")
	if a.Motd() != "welcome to the room" {
		t.Fatalf("motd after set = %q, want %q", a.Motd(), "welcome to the room")
	}

	cmdRoomMotd(cm, []string{"read", "the", "rules"}, "")
	if a.Motd() != "read the rules" {
		t.Fatalf("motd after overwrite = %q, want %q", a.Motd(), "read the rules")
	}

	cmdRoomMotd(cm, []string{"-c"}, "")
	if a.Motd() != "" {
		t.Fatalf("motd after clear = %q, want empty", a.Motd())
	}
}

// TestRoomMotdCensorBlocksBannedWord verifies a banned word is not stored as
// the room motd; the censor path routes it through the same automod as chat.
func TestRoomMotdCensorBlocksBannedWord(t *testing.T) {
	defer setupShownameCensorTestDB(t)()
	newTestClients(t)

	origConfig := config
	origAction := autoModAction
	origWords := getBannedWords()
	t.Cleanup(func() {
		config = origConfig
		autoModAction = origAction
		setBannedWords(origWords)
	})
	config = &settings.Config{ServerConfig: settings.ServerConfig{AutoModEnabled: true}}
	autoModAction = autoModActionShadow
	setBannedWords([]string{"zqvexo"})
	resetRateLimitKickTracker()

	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)
	cm, _ := newRoomMotdClient(a, 3, true)
	t.Cleanup(func() { clients.RemoveClient(cm) })

	cmdRoomMotd(cm, []string{"say", "zqvexo", "now"}, "")

	if a.Motd() != "" {
		t.Errorf("banned word set the motd: got %q, want empty", a.Motd())
	}
}
