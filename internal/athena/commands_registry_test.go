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

// TestValidateCommandsCurrentRegistry runs the validator against the actual
// shipping registry. Any future command added without a handler/usage/desc/
// category trips this test and blocks the merge before a user ever sees it.
func TestValidateCommandsCurrentRegistry(t *testing.T) {
	initCommands()
	// Deferring a recover lets the test report which field is missing instead
	// of the vanilla panic trace.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("validateCommands panicked: %v", r)
		}
	}()
	validateCommands()
	if len(Commands) == 0 {
		t.Fatal("registry is empty after initCommands")
	}
}

// TestRegisterCommandInstalls verifies the additive registration path.
func TestRegisterCommandInstalls(t *testing.T) {
	initCommands()
	before := len(Commands)
	RegisterCommand("__testonly_register__", Command{
		handler:  func(client *Client, args []string, usage string) {},
		minArgs:  0,
		usage:    "Usage: /__testonly_register__",
		desc:     "unit test only",
		reqPerms: permissions.PermissionField["NONE"],
		category: "general",
	})
	if len(Commands) != before+1 {
		t.Fatalf("expected map to grow by 1, got %d -> %d", before, len(Commands))
	}
	if _, ok := Commands["__testonly_register__"]; !ok {
		t.Fatal("registered command not found in map")
	}
	delete(Commands, "__testonly_register__")
}

// TestRegisterCommandDuplicatePanics guards against silent shadowing when
// two feature files try to register the same name.
func TestRegisterCommandDuplicatePanics(t *testing.T) {
	initCommands()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
		delete(Commands, "__testonly_dup__")
	}()
	RegisterCommand("__testonly_dup__", Command{
		handler: func(c *Client, a []string, u string) {}, usage: "u", desc: "d", category: "general",
	})
	RegisterCommand("__testonly_dup__", Command{
		handler: func(c *Client, a []string, u string) {}, usage: "u", desc: "d", category: "general",
	})
}

// TestRegisterCommandBeforeInitPanics ensures operators catch the ordering
// mistake at startup rather than when the first client runs a command.
func TestRegisterCommandBeforeInitPanics(t *testing.T) {
	save := Commands
	Commands = nil
	defer func() {
		Commands = save
		if r := recover(); r == nil {
			t.Fatal("expected panic when Commands is nil")
		}
	}()
	RegisterCommand("x", Command{handler: func(c *Client, a []string, u string) {}, usage: "u", desc: "d", category: "general"})
}
