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

// makeTestArea is a helper that creates a minimal *area.Area for testing.
func makeTestArea(name string) *area.Area {
	return area.NewArea(
		area.AreaData{Name: name, Bg: "default"},
		1,   // charlen
		10,  // bufsize
		area.EviCMs,
	)
}

// setupTestAreas sets the package-level globals used by getAreaIndex.
// It returns a cleanup function that restores the original values.
func setupTestAreas(testAreas []*area.Area) func() {
	origAreas := areas
	origMap := areaIndexMap

	areas = testAreas
	areaIndexMap = make(map[*area.Area]int, len(testAreas))
	for i, a := range testAreas {
		areaIndexMap[a] = i
	}

	return func() {
		areas = origAreas
		areaIndexMap = origMap
	}
}

// TestGetAreaIndex verifies that getAreaIndex returns the correct 0-based index
// for each area in the global areas slice.
func TestGetAreaIndex(t *testing.T) {
	a0 := makeTestArea("Lobby")
	a1 := makeTestArea("Courtroom 1")
	a2 := makeTestArea("Courtroom 2")

	cleanup := setupTestAreas([]*area.Area{a0, a1, a2})
	defer cleanup()

	tests := []struct {
		name     string
		area     *area.Area
		wantIdx  int
	}{
		{"first area", a0, 0},
		{"middle area", a1, 1},
		{"last area", a2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAreaIndex(tt.area)
			if got != tt.wantIdx {
				t.Errorf("getAreaIndex() = %d, want %d", got, tt.wantIdx)
			}
		})
	}
}

// TestGetAreaIndexSingleArea verifies correct behaviour with only one area.
func TestGetAreaIndexSingleArea(t *testing.T) {
	a := makeTestArea("Lobby")
	cleanup := setupTestAreas([]*area.Area{a})
	defer cleanup()

	if idx := getAreaIndex(a); idx != 0 {
		t.Errorf("getAreaIndex() = %d, want 0", idx)
	}
}

// TestGetAreaIndexUnknown verifies that an area pointer not in the map
// returns the zero-value (0), which is the documented fallback.
func TestGetAreaIndexUnknown(t *testing.T) {
	a0 := makeTestArea("Lobby")
	unknown := makeTestArea("Ghost Area")

	cleanup := setupTestAreas([]*area.Area{a0})
	defer cleanup()

	if idx := getAreaIndex(unknown); idx != 0 {
		t.Errorf("getAreaIndex(unknown) = %d, want 0 (fallback)", idx)
	}
}

// TestAreaIndexMapConsistency verifies that the areaIndexMap mirrors the
// areas slice (every index round-trips correctly).
func TestAreaIndexMapConsistency(t *testing.T) {
	names := []string{"Alpha", "Beta", "Gamma", "Delta"}
	testAreas := make([]*area.Area, len(names))
	for i, n := range names {
		testAreas[i] = makeTestArea(n)
	}

	cleanup := setupTestAreas(testAreas)
	defer cleanup()

	for wantIdx, a := range testAreas {
		gotIdx := getAreaIndex(a)
		if gotIdx != wantIdx {
			t.Errorf("area %q: getAreaIndex() = %d, want %d", names[wantIdx], gotIdx, wantIdx)
		}
	}
}

// TestPlayerlistCurrentCharacterSpectator verifies that a client with no
// character selected reports "Spectator" as their character — the value that
// would be included in a PU CHARACTER packet.
func TestPlayerlistCurrentCharacterSpectator(t *testing.T) {
	origChars := characters
	defer func() { characters = origChars }()

	characters = []string{"Phoenix Wright", "Miles Edgeworth"}

	client := &Client{char: -1}
	if got := client.CurrentCharacter(); got != "Spectator" {
		t.Errorf("CurrentCharacter() with char=-1 = %q, want %q", got, "Spectator")
	}
}

// TestPlayerlistCurrentCharacterNamed verifies that a client with a character
// chosen returns the correct name from the characters slice.
func TestPlayerlistCurrentCharacterNamed(t *testing.T) {
	origChars := characters
	defer func() { characters = origChars }()

	characters = []string{"Phoenix Wright", "Miles Edgeworth"}

	tests := []struct {
		charID int
		want   string
	}{
		{0, "Phoenix Wright"},
		{1, "Miles Edgeworth"},
	}

	for _, tt := range tests {
		client := &Client{char: tt.charID}
		if got := client.CurrentCharacter(); got != tt.want {
			t.Errorf("CurrentCharacter() with char=%d = %q, want %q", tt.charID, got, tt.want)
		}
	}
}
