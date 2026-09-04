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

// Tests for the per-session [RAIDGUARD] alert mute (/raidguard alert off).
//
// Two properties matter and neither is observable without standing the
// fan-out up: an alert must always carry the hint that says how to turn it
// off (a mute nobody can find is the same as no mute), and the toggle must be
// reachable by everyone who receives the alerts — MOD_CHAT — rather than only
// by the BAN holders the rest of /raidguard is gated on.

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// newAlertTestClient registers a struct-literal client with the given perms
// against a fresh global client list, returning the conn it writes to.
func newAlertTestClient(uid int, perms uint64) (*Client, *capturingConn) {
	conn := &capturingConn{}
	c := &Client{conn: conn, uid: uid, char: -1, perms: perms}
	clients.AddClient(c)
	clients.RegisterUID(c)
	return c, conn
}

func withEmptyClientList(t *testing.T) {
	t.Helper()
	orig := clients
	t.Cleanup(func() { clients = orig })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}
}

func TestRaidGuardAlertRespectsPerSessionMute(t *testing.T) {
	withEmptyClientList(t)

	_, listeningConn := newAlertTestClient(1, permissions.PermissionField["MOD_CHAT"])
	muted, mutedConn := newAlertTestClient(2, permissions.PermissionField["MOD_CHAT"])
	_, playerConn := newAlertTestClient(3, permissions.PermissionField["NONE"])

	muted.SetRaidAlertsDisabled(true)

	sendRaidGuardAlert("something raid-shaped happened")

	got := listeningConn.snapshot()
	if len(got) != 1 {
		t.Fatalf("listening mod got %v, want exactly one alert", got)
	}
	if !strings.Contains(got[0], "something raid-shaped happened") {
		t.Errorf("alert %q does not carry the message body", got[0])
	}
	// The hint has to ride along on every alert: a mod being buried in these
	// mid-raid is exactly who needs to learn the toggle exists.
	if !strings.Contains(got[0], "/raidguard alert off") {
		t.Errorf("alert %q is missing the opt-out hint", got[0])
	}
	if n := len(mutedConn.snapshot()); n != 0 {
		t.Errorf("mod with alerts off received %d alert(s), want 0", n)
	}
	if n := len(playerConn.snapshot()); n != 0 {
		t.Errorf("non-staff client received %d alert(s), want 0 — these are MOD_CHAT only", n)
	}
}

func TestRaidGuardAlertCommandTogglesAndReports(t *testing.T) {
	withEmptyClientList(t)
	mod, conn := newAlertTestClient(1, permissions.PermissionField["MOD_CHAT"])

	const usage = "Usage: /raidguard status | clear <uid|all> | test <text> | alert <on|off>"

	cmdRaidGuard(mod, []string{"alert", "off"}, usage)
	if !mod.RaidAlertsDisabled() {
		t.Fatal("/raidguard alert off did not mute the alerts")
	}
	cmdRaidGuard(mod, []string{"alert", "on"}, usage)
	if mod.RaidAlertsDisabled() {
		t.Fatal("/raidguard alert on did not unmute the alerts")
	}

	// The bare form reports rather than changing anything, and the plural
	// spelling is accepted since both read naturally.
	conn.written = nil
	cmdRaidGuard(mod, []string{"alerts"}, usage)
	if mod.RaidAlertsDisabled() {
		t.Error("bare /raidguard alert changed the setting; it must only report")
	}
	if out := strings.Join(conn.snapshot(), ""); !strings.Contains(out, "ENABLED") {
		t.Errorf("status reply %q does not report the current state", out)
	}
}

// The toggle is why the command's registry gate dropped from BAN to MOD_CHAT.
// That is only safe while the other subcommands re-check BAN in the handler,
// so pin both halves.
func TestRaidGuardModChatOnlyGetsTheToggleAndNothingElse(t *testing.T) {
	withEmptyClientList(t)
	mod, conn := newAlertTestClient(1, permissions.PermissionField["MOD_CHAT"])

	const usage = "Usage: /raidguard status | clear <uid|all> | test <text> | alert <on|off>"

	cmdRaidGuard(mod, []string{"alert", "off"}, usage)
	if !mod.RaidAlertsDisabled() {
		t.Fatal("a MOD_CHAT holder could not mute the alerts they receive")
	}

	for _, sub := range [][]string{{"status"}, {"clear", "all"}, {"test", "hello there"}} {
		conn.written = nil
		cmdRaidGuard(mod, sub, usage)
		out := strings.Join(conn.snapshot(), "")
		if !strings.Contains(out, "permission") {
			t.Errorf("/raidguard %v without BAN returned %q, want a permission refusal", sub, out)
		}
	}
}
