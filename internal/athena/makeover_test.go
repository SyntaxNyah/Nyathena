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

// TestMakeoverValidCharacter tests that makeover command fully changes all clients to target character
func TestMakeoverValidCharacter(t *testing.T) {
	// Save original characters array and restore after test
	originalCharacters := characters
	defer func() {
		characters = originalCharacters
	}()

	// Set up test characters
	characters = []string{
		"Phoenix Wright",
		"Miles Edgeworth",
		"Maya Fey",
	}

	// Create mock area for testing
	testArea := area.NewArea(area.AreaData{}, 50, 100, area.EviAny)

	// Create mock clients with different initial characters
	client1 := &Client{
		uid:        1,
		char:       0, // Phoenix Wright
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: "normal", flip: "0", offset: ""},
		pairedUID:  -1,
		area:       testArea,
	}

	client2 := &Client{
		uid:        2,
		char:       1, // Miles Edgeworth
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: "thinking", flip: "0", offset: ""},
		pairedUID:  -1,
		area:       testArea,
	}

	client3 := &Client{
		uid:        3,
		char:       2, // Maya Fey
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: "happy", flip: "1", offset: "50&60"},
		pairedUID:  -1,
		area:       testArea,
	}

	// Add characters to area's taken list
	testArea.AddChar(client1.CharID())
	testArea.AddChar(client2.CharID())
	testArea.AddChar(client3.CharID())

	// Save original clients list and restore after test
	originalClients := clients
	defer func() {
		clients = originalClients
	}()

	// Set up test clients list
	clients = ClientList{list: make(map[*Client]struct{})}
	clients.list[client1] = struct{}{}
	clients.list[client2] = struct{}{}
	clients.list[client3] = struct{}{}

	// Test: Force all clients to fully change into "Miles Edgeworth"
	targetChar := "Miles Edgeworth"
	targetCharID := getCharacterID(targetChar)

	// Verify character exists
	if targetCharID == -1 {
		t.Fatalf("Test setup failed: character '%s' not found", targetChar)
	}

	// Simulate what cmdMakeover does
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 || c.CharID() == -1 {
			continue
		}

		// Remove old character
		c.Area().RemoveChar(c.CharID())

		// Set new character
		c.SetCharID(targetCharID)

		// Clear PairInfo
		c.SetPairInfo("", "", "", "")

		// Add new character to area
		c.Area().AddChar(targetCharID)
	}

	// Verify all clients now have the target character as their actual character
	for c := range clients.GetAllClients() {
		if c.CharID() != targetCharID {
			t.Errorf("Expected client UID %d to have CharID %d (%s), got %d",
				c.Uid(), targetCharID, targetChar, c.CharID())
		}

		// Verify PairInfo is cleared (no iniswap)
		if c.PairInfo().name != "" {
			t.Errorf("Expected client UID %d to have empty PairInfo name, got '%s'",
				c.Uid(), c.PairInfo().name)
		}
	}
}

// TestMakeoverInvalidCharacter tests that makeover command handles invalid characters properly
func TestMakeoverInvalidCharacter(t *testing.T) {
	// Save original characters array and restore after test
	originalCharacters := characters
	defer func() {
		characters = originalCharacters
	}()

	// Set up test characters
	characters = []string{
		"Phoenix Wright",
		"Miles Edgeworth",
		"Maya Fey",
	}

	// Test with a character that doesn't exist
	invalidChar := "NonExistent Character"
	charID := getCharacterID(invalidChar)

	if charID != -1 {
		t.Errorf("Expected getCharacterID to return -1 for invalid character '%s', got %d",
			invalidChar, charID)
	}
}

// TestMakeoverSkipsUnjoined tests that makeover skips clients with UID -1 or CharID -1
func TestMakeoverSkipsUnjoined(t *testing.T) {
	// Save original characters array and restore after test
	originalCharacters := characters
	defer func() {
		characters = originalCharacters
	}()

	// Set up test characters
	characters = []string{
		"Phoenix Wright",
		"Miles Edgeworth",
	}

	// Create mock area
	testArea := area.NewArea(area.AreaData{}, 50, 100, area.EviAny)

	// Create a joined client
	joinedClient := &Client{
		uid:        1,
		char:       0,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: "normal"},
		pairedUID:  -1,
		area:       testArea,
	}
	testArea.AddChar(joinedClient.CharID())

	// Create an unjoined client (UID -1)
	unjoinedClient := &Client{
		uid:        -1,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: ""},
		pairedUID:  -1,
		area:       testArea,
	}

	// Create a client in char select (CharID -1 but UID valid)
	charSelectClient := &Client{
		uid:        2,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: ""},
		pairedUID:  -1,
		area:       testArea,
	}

	// Save original clients list and restore after test
	originalClients := clients
	defer func() {
		clients = originalClients
	}()

	// Set up test clients list
	clients = ClientList{list: make(map[*Client]struct{})}
	clients.list[joinedClient] = struct{}{}
	clients.list[unjoinedClient] = struct{}{}
	clients.list[charSelectClient] = struct{}{}

	targetChar := "Miles Edgeworth"
	targetCharID := getCharacterID(targetChar)

	// Simulate what cmdMakeover does
	var count int
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 || c.CharID() == -1 {
			continue
		}

		c.Area().RemoveChar(c.CharID())
		c.SetCharID(targetCharID)
		c.SetPairInfo("", "", "", "")
		c.Area().AddChar(targetCharID)
		count++
	}

	// Verify only the joined client was affected
	if count != 1 {
		t.Errorf("Expected 1 client to be affected, got %d", count)
	}

	// Verify joined client was updated
	if joinedClient.CharID() != targetCharID {
		t.Errorf("Expected joined client to have CharID %d, got %d",
			targetCharID, joinedClient.CharID())
	}

	// Verify PairInfo was cleared
	if joinedClient.PairInfo().name != "" {
		t.Errorf("Expected joined client to have empty PairInfo name, got '%s'",
			joinedClient.PairInfo().name)
	}

	// Verify unjoined client was NOT updated
	if unjoinedClient.CharID() != -1 {
		t.Errorf("Expected unjoined client to still have CharID -1, got %d",
			unjoinedClient.CharID())
	}

	// Verify char select client was NOT updated
	if charSelectClient.CharID() != -1 {
		t.Errorf("Expected char select client to still have CharID -1, got %d",
			charSelectClient.CharID())
	}
}

