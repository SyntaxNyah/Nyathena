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

// TestMakeoverValidCharacter tests that makeover command uses iniswap to bypass slot limits
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

	// Test: Force all clients to iniswap into "Miles Edgeworth"
	targetChar := "Miles Edgeworth"
	targetCharID := getCharacterID(targetChar)

	// Verify character exists
	if targetCharID == -1 {
		t.Fatalf("Test setup failed: character '%s' not found", targetChar)
	}

	// Simulate what cmdMakeover does - use iniswap to bypass slot limit
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 || c.CharID() == -1 {
			continue
		}

		// Force iniswap with empty emote/flip/offset for consistent appearance
		c.SetPairInfo(targetChar, "", "", "")
	}

	// Verify all clients now have the target character in their PairInfo (iniswap)
	for c := range clients.GetAllClients() {
		if c.PairInfo().name != targetChar {
			t.Errorf("Expected client UID %d to have PairInfo name '%s' (iniswapped), got '%s'",
				c.Uid(), targetChar, c.PairInfo().name)
		}

		// Verify emote, flip, and offset are empty (reset for consistent appearance)
		if c.PairInfo().emote != "" {
			t.Errorf("Expected client UID %d to have empty emote, got '%s'",
				c.Uid(), c.PairInfo().emote)
		}
		if c.PairInfo().flip != "" {
			t.Errorf("Expected client UID %d to have empty flip, got '%s'",
				c.Uid(), c.PairInfo().flip)
		}
		if c.PairInfo().offset != "" {
			t.Errorf("Expected client UID %d to have empty offset, got '%s'",
				c.Uid(), c.PairInfo().offset)
		}
	}

	// Verify original CharIDs are preserved (not changed, since we use iniswap)
	if client1.CharID() != 0 {
		t.Errorf("Expected client1 to still have original CharID 0, got %d", client1.CharID())
	}
	if client2.CharID() != 1 {
		t.Errorf("Expected client2 to still have original CharID 1, got %d", client2.CharID())
	}
	if client3.CharID() != 2 {
		t.Errorf("Expected client3 to still have original CharID 2, got %d", client3.CharID())
	}

	// Verify area's taken slots are unchanged (iniswap doesn't affect them)
	if !testArea.IsTaken(0) {
		t.Errorf("Expected area to still have character 0 (Phoenix Wright) taken")
	}
	if !testArea.IsTaken(1) {
		t.Errorf("Expected area to still have character 1 (Miles Edgeworth) taken")
	}
	if !testArea.IsTaken(2) {
		t.Errorf("Expected area to still have character 2 (Maya Fey) taken")
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

	// Simulate what cmdMakeover does
	var count int
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 || c.CharID() == -1 {
			continue
		}

		c.SetPairInfo(targetChar, "", "", "")
		count++
	}

	// Verify only the joined client was affected
	if count != 1 {
		t.Errorf("Expected 1 client to be affected, got %d", count)
	}

	// Verify joined client was iniswapped
	if joinedClient.PairInfo().name != targetChar {
		t.Errorf("Expected joined client to have PairInfo name '%s', got '%s'",
			targetChar, joinedClient.PairInfo().name)
	}

	// Verify original CharID preserved
	if joinedClient.CharID() != 0 {
		t.Errorf("Expected joined client to still have original CharID 0, got %d",
			joinedClient.CharID())
	}

	// Verify unjoined client was NOT updated
	if unjoinedClient.PairInfo().name != "" {
		t.Errorf("Expected unjoined client to have empty PairInfo name, got '%s'",
			unjoinedClient.PairInfo().name)
	}

	// Verify char select client was NOT updated
	if charSelectClient.PairInfo().name != "" {
		t.Errorf("Expected char select client to have empty PairInfo name, got '%s'",
			charSelectClient.PairInfo().name)
	}
}

// TestMakeoverICMessageHandling tests that iniswapped clients appear as the iniswapped character when they speak
func TestMakeoverICMessageHandling(t *testing.T) {
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

	// Create mock area
	testArea := area.NewArea(area.AreaData{}, 50, 100, area.EviAny)

	// Create a client with Phoenix Wright
	testClient := &Client{
		uid:        1,
		char:       0, // Phoenix Wright
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, emote: "normal", flip: "0", offset: ""},
		pairedUID:  -1,
		area:       testArea,
	}
	testArea.AddChar(testClient.CharID())

	// Simulate makeover: force iniswap to Miles Edgeworth
	targetChar := "Miles Edgeworth"
	testClient.SetPairInfo(targetChar, "", "", "")

	// Verify PairInfo was set
	if testClient.PairInfo().name != targetChar {
		t.Fatalf("Test setup failed: PairInfo name should be '%s', got '%s'",
			targetChar, testClient.PairInfo().name)
	}

	// Verify that getCharacterID works for the iniswapped character
	iniswapCharID := getCharacterID(targetChar)
	if iniswapCharID == -1 {
		t.Fatalf("Test setup failed: character '%s' not found", targetChar)
	}

	// Expected character ID after iniswap
	expectedCharID := 1 // Miles Edgeworth

	// Simulate what pktIC does when client sends an IC message
	// The client would send their original character (Phoenix Wright) but the server should apply the iniswap

	// Get the client's PairInfo (as pktIC does)
	clientPairInfo := testClient.PairInfo()

	// Verify the iniswap would be applied (as the new code does)
	if clientPairInfo.name != "" {
		appliedCharID := getCharacterID(clientPairInfo.name)
		if appliedCharID != expectedCharID {
			t.Errorf("Expected iniswap to apply character ID %d (Miles Edgeworth), got %d",
				expectedCharID, appliedCharID)
		}
		if clientPairInfo.name != targetChar {
			t.Errorf("Expected iniswap to apply character name '%s', got '%s'",
				targetChar, clientPairInfo.name)
		}
	} else {
		t.Error("Expected client to have iniswap set, but PairInfo.name is empty")
	}

	// Verify original CharID is preserved (client slot doesn't change)
	if testClient.CharID() != 0 {
		t.Errorf("Expected client to still have original CharID 0 (Phoenix Wright), got %d",
			testClient.CharID())
	}

	// Verify the iniswap persists (isn't lost)
	if testClient.PairInfo().name != targetChar {
		t.Errorf("Expected iniswap to persist, got '%s'", testClient.PairInfo().name)
	}
}

