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
	"strings"
	"testing"
)

// TestValidPositions verifies that every entry in validPositions is accepted.
func TestValidPositions(t *testing.T) {
	for _, pos := range validPositions {
		client := &Client{pos: "def"}
		client.SetPos(pos)
		if client.Pos() != pos {
			t.Errorf("Expected position %q after SetPos, got %q", pos, client.Pos())
		}
	}
}

// TestInvalidPosition verifies that an unknown position string is not in validPositions.
func TestInvalidPosition(t *testing.T) {
	invalid := "invalid_pos"
	for _, v := range validPositions {
		if invalid == v {
			t.Errorf("Expected %q to not be a valid position, but it was found", invalid)
		}
	}
}

// TestPosCommandChangesPosition verifies the position-change branch of cmdPos logic.
func TestPosCommandChangesPosition(t *testing.T) {
	tests := []struct {
		input   string
		wantPos string
		wantOK  bool
	}{
		{"pro", "pro", true},
		{"def", "def", true},
		{"wit", "wit", true},
		{"jud", "jud", true},
		{"hld", "hld", true},
		{"hlp", "hlp", true},
		{"jur", "jur", true},
		{"sea", "sea", true},
		{"PRO", "pro", true},  // case-insensitive
		{"DEF", "def", true},  // case-insensitive
		{"bad", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		client := &Client{pos: "def"}
		pos := strings.ToLower(tt.input)
		changed := false
		for _, v := range validPositions {
			if pos == v {
				client.SetPos(pos)
				changed = true
				break
			}
		}
		if changed != tt.wantOK {
			t.Errorf("input=%q: expected changed=%v, got %v", tt.input, tt.wantOK, changed)
		}
		if tt.wantOK && client.Pos() != tt.wantPos {
			t.Errorf("input=%q: expected pos=%q, got %q", tt.input, tt.wantPos, client.Pos())
		}
	}
}

// TestCommandRegexCaseInsensitive verifies that commandRegex matches /pos in any case,
// and that the extracted command name is always lowercased for Commands map lookup.
func TestCommandRegexCaseInsensitive(t *testing.T) {
	tests := []struct {
		input       string
		wantCommand string
		wantMatch   bool
	}{
		{"/pos", "pos", true},
		{"/Pos", "pos", true},
		{"/POS", "pos", true},
		{"/pOs", "pos", true},
		{"/join-tournament", "join-tournament", true},
		{"/JOIN-TOURNAMENT", "join-tournament", true},
		{"notacommand", "", false},
		{"/123", "123", true},
	}

	for _, tt := range tests {
		match := commandRegex.FindString(tt.input)
		command := strings.ToLower(strings.TrimPrefix(match, "/"))
		gotMatch := match != ""
		if gotMatch != tt.wantMatch {
			t.Errorf("input=%q: expected match=%v, got match=%v", tt.input, tt.wantMatch, gotMatch)
		}
		if tt.wantMatch && command != tt.wantCommand {
			t.Errorf("input=%q: expected command=%q, got %q", tt.input, tt.wantCommand, command)
		}
	}
}
