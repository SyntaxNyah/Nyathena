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

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestRemoveRoleCommandRegistered verifies that /removerole is wired into the
// registry as an ADMIN-only command taking a single argument (the username),
// matching /setrole and /rmusr.
func TestRemoveRoleCommandRegistered(t *testing.T) {
	initCommands()

	cmd, ok := Commands["removerole"]
	if !ok {
		t.Fatal("removerole command is not registered in Commands map")
	}
	if cmd.handler == nil {
		t.Error("removerole command has a nil handler")
	}
	if cmd.minArgs != 1 {
		t.Errorf("removerole minArgs = %d, want 1", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["ADMIN"] {
		t.Errorf("removerole reqPerms = %v, want ADMIN (%v)", cmd.reqPerms, permissions.PermissionField["ADMIN"])
	}
	if cmd.category != "admin" {
		t.Errorf("removerole category = %q, want \"admin\"", cmd.category)
	}
}
