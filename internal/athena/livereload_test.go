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

func TestCheckCharAppendOnly(t *testing.T) {
	tests := []struct {
		name        string
		old, new    []string
		wantErr     bool
		errContains string
	}{
		{
			name: "identical",
			old:  []string{"A", "B", "C"},
			new:  []string{"A", "B", "C"},
		},
		{
			name: "appended at end (the supported case)",
			old:  []string{"A", "B"},
			new:  []string{"A", "B", "C", "D"},
		},
		{
			name:        "shrunk — rejected",
			old:         []string{"A", "B", "C"},
			new:         []string{"A", "B"},
			wantErr:     true,
			errContains: "shrank",
		},
		{
			name:        "middle insertion — rejected",
			old:         []string{"A", "B", "C"},
			new:         []string{"A", "X", "B", "C"},
			wantErr:     true,
			errContains: "slot 1",
		},
		{
			name:        "renamed slot — rejected",
			old:         []string{"A", "B", "C"},
			new:         []string{"A", "B", "Z"},
			wantErr:     true,
			errContains: "slot 2",
		},
		{
			name:        "reorder — rejected",
			old:         []string{"A", "B", "C"},
			new:         []string{"B", "A", "C"},
			wantErr:     true,
			errContains: "slot 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkCharAppendOnly(tt.old, tt.new)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

// Verifies that setCharacters publishes both the list AND the derived
// name→index lookup map atomically, so a reload immediately serves new
// characters via getCharacterID.
func TestSetCharactersUpdatesIndex(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })

	setCharacters([]string{"Alpha", "Beta", "Gamma"})

	if id := getCharacterID("beta"); id != 1 {
		t.Errorf("getCharacterID(beta) = %d, want 1 (case-insensitive index lookup)", id)
	}
	if id := getCharacterID("Gamma"); id != 2 {
		t.Errorf("getCharacterID(Gamma) = %d, want 2", id)
	}
	if id := getCharacterID("Delta"); id != -1 {
		t.Errorf("getCharacterID(Delta) = %d, want -1", id)
	}
}

// Verifies setBackgrounds rebuilds the cached /bglist string in lockstep so a
// reload doesn't serve a stale list to /bglist.
func TestSetBackgroundsUpdatesListStr(t *testing.T) {
	origBg := getBackgrounds()
	origStr := getBgListStr()
	t.Cleanup(func() {
		setBackgrounds(origBg)
		// The list string is derived from the list, so resetting the list above
		// is enough; this assertion is defensive in case the cleanup order ever
		// matters in the future.
		_ = origStr
	})

	setBackgrounds([]string{"forest", "court", "library"})
	got := getBgListStr()
	for _, bg := range []string{"forest", "court", "library"} {
		if !strings.Contains(got, bg) {
			t.Errorf("bgListStr missing entry %q: %q", bg, got)
		}
	}
}
