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

// Tests for /modcall <enable|disable|status>, the server-wide modcall switch.
//
// Two properties matter operationally and neither is visible from the handler
// alone: everybody who receives modcalls must be able to READ the state (the
// registry gate is MOD_CHAT), while flipping it must stay with BAN -- and the
// switch must be in-memory only, so a restart always restores modcalls.

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// withModcallSwitch restores the global switch and counter after a test, so one
// case can never decide another's outcome.
func withModcallSwitch(t *testing.T) {
	t.Helper()
	oldDisabled := modcallsDisabled.Load()
	oldSuppressed := modcallsSuppressed.Load()
	t.Cleanup(func() {
		modcallsDisabled.Store(oldDisabled)
		modcallsSuppressed.Store(oldSuppressed)
	})
	modcallsDisabled.Store(false)
	modcallsSuppressed.Store(0)
}

// modcallTestClient builds a client with a capturing conn, a real area (the
// handler writes a CMD line to the area buffer) and the given perms.
func modcallTestClient(perms uint64) (*Client, *capturingConn) {
	conn := &capturingConn{}
	c := &Client{conn: conn, uid: 42, char: -1, perms: perms}
	c.SetArea(area.NewArea(area.AreaData{Name: "Courtroom"}, 1, 10, area.EviAny))
	return c, conn
}

func TestModcallCommandRegistered(t *testing.T) {
	initCommands()
	cmd, ok := Commands["modcall"]
	if !ok {
		t.Fatal("modcall command is not registered in Commands map")
	}
	if cmd.handler == nil {
		t.Error("modcall command has a nil handler")
	}
	if cmd.minArgs != 0 {
		t.Errorf("modcall minArgs = %d, want 0 (bare form reports the state)", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["MOD_CHAT"] {
		t.Errorf("modcall reqPerms = %v, want MOD_CHAT (%v) so every alert recipient can read the state",
			cmd.reqPerms, permissions.PermissionField["MOD_CHAT"])
	}
	if cmd.category != "moderation" {
		t.Errorf("modcall category = %q, want moderation", cmd.category)
	}
}

// TestModcallCommandTogglesAndReports walks the operational path: disable,
// status, enable, and the counter reset on each flip.
func TestModcallCommandTogglesAndReports(t *testing.T) {
	withEmptyClientList(t)
	withModcallSwitch(t)

	c, conn := modcallTestClient(permissions.PermissionField["BAN"])

	cmdModcall(c, nil, "usage")
	if modcallsDisabled.Load() {
		t.Fatal("a bare /modcall changed the switch; it must only report")
	}

	cmdModcall(c, []string{"disable"}, "usage")
	if !modcallsDisabled.Load() {
		t.Fatal("/modcall disable did not disable modcalls")
	}
	modcallsSuppressed.Store(17)

	conn.written = nil
	cmdModcall(c, []string{"status"}, "usage")
	got := strings.Join(conn.snapshot(), "")
	if !strings.Contains(got, "DISABLED") {
		t.Errorf("status after disable does not say DISABLED: %q", got)
	}
	if !strings.Contains(got, "17") {
		t.Errorf("status does not report the suppressed count: %q", got)
	}
	if !strings.Contains(got, "restart") {
		t.Errorf("status does not say the switch resets on restart: %q", got)
	}

	cmdModcall(c, []string{"enable"}, "usage")
	if modcallsDisabled.Load() {
		t.Fatal("/modcall enable did not re-enable modcalls")
	}
	if modcallsSuppressed.Load() != 0 {
		t.Errorf("suppressed counter = %d after enable, want it reported and reset", modcallsSuppressed.Load())
	}
}

// TestModcallCommandRequiresBan pins the permission split: MOD_CHAT can read the
// state, only BAN can change it.
func TestModcallCommandRequiresBan(t *testing.T) {
	withEmptyClientList(t)
	withModcallSwitch(t)

	c, conn := modcallTestClient(permissions.PermissionField["MOD_CHAT"])

	cmdModcall(c, []string{"disable"}, "usage")
	if modcallsDisabled.Load() {
		t.Fatal("a MOD_CHAT-only holder changed the switch; that must need BAN")
	}
	got := strings.Join(conn.snapshot(), "")
	if !strings.Contains(strings.ToLower(got), "permission") {
		t.Errorf("refusal did not mention permission: %q", got)
	}

	// ... but reading it is allowed.
	conn.written = nil
	cmdModcall(c, []string{"status"}, "usage")
	if got := strings.Join(conn.snapshot(), ""); !strings.Contains(got, "ENABLED") {
		t.Errorf("MOD_CHAT holder cannot read the state: %q", got)
	}
}

// TestModcallCommandRejectsUnknownSubcommand keeps a typo from being a no-op
// that looks like it worked.
func TestModcallCommandRejectsUnknownSubcommand(t *testing.T) {
	withEmptyClientList(t)
	withModcallSwitch(t)

	c, conn := modcallTestClient(permissions.PermissionField["BAN"])
	cmdModcall(c, []string{"offf"}, "Usage: /modcall <enable|disable|status>")

	if modcallsDisabled.Load() {
		t.Fatal("a typo changed the switch")
	}
	if got := strings.Join(conn.snapshot(), ""); !strings.Contains(got, "Usage:") {
		t.Errorf("typo did not report usage: %q", got)
	}
}
