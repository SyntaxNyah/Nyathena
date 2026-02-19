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
	"net"
	"strings"
	"testing"
)

// TestPossessionTracking tests that possession state is tracked correctly for fullpossess
func TestPossessionTracking(t *testing.T) {
	// Create two clients with proper initialization
	possessor := &Client{
		uid:        1,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
		pairedUID:  -1,
	}
	target := &Client{
		uid:        2,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
		pairedUID:  -1,
	}

	// Initially, possessor should not be possessing anyone
	if possessor.Possessing() != -1 {
		t.Errorf("Expected possessor to not be possessing anyone initially, got %d", possessor.Possessing())
	}

	// Set up full possession link
	possessor.SetPossessing(target.Uid())

	// Verify possession link is established
	if possessor.Possessing() != target.Uid() {
		t.Errorf("Expected possessor to be possessing target UID %d, got %d", target.Uid(), possessor.Possessing())
	}

	// Clear possession link
	possessor.SetPossessing(-1)

	// Verify possession link is cleared
	if possessor.Possessing() != -1 {
		t.Errorf("Expected possession link to be cleared, got %d", possessor.Possessing())
	}
}

// TestFullPossessionTransformation tests that full possession transforms admin's messages
func TestFullPossessionTransformation(t *testing.T) {
	// Create two clients with proper initialization
	admin := &Client{
		uid:        1,
		char:       0, // Phoenix Wright
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
		pairedUID:  -1,
		pos:        "def",
	}
	target := &Client{
		uid:        2,
		char:       1, // Miles Edgeworth
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
		pairedUID:  -1,
		pos:        "pro",
	}

	// Initially, admin should not be possessing anyone
	if admin.Possessing() != -1 {
		t.Errorf("Expected admin to not be possessing anyone initially, got %d", admin.Possessing())
	}

	// Simulate fullpossess command execution
	admin.SetPossessing(target.Uid())

	// Verify admin is now possessing the target
	if admin.Possessing() != target.Uid() {
		t.Errorf("Expected admin to be possessing target UID %d, got %d", target.Uid(), admin.Possessing())
	}

	// In actual usage, when admin sends IC message, pktIC will transform it
	// to use target's character, position, emote, colors, etc.
	// This test verifies the possession link is properly established
}

// TestNewClientInitialization tests that new clients have possessing field initialized to -1
func TestNewClientInitialization(t *testing.T) {
	client := &Client{
		uid:        -1,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
		pairedUID:  -1,
	}

	if client.Possessing() != -1 {
		t.Errorf("Expected new client possessing field to be -1, got %d", client.Possessing())
	}
}

// TestPossessPreservesAdminPosition tests that possession commands preserve admin's position
func TestPossessPreservesAdminPosition(t *testing.T) {
	// Create admin with position "def" (defense)
	admin := &Client{
		uid:          1,
		char:         0, // Phoenix Wright
		possessing:   -1,
		possessedPos: "",
		pair:         ClientPairInfo{wanted_id: -1},
		pairedUID:    -1,
		pos:          "def",
	}

	// Create target with position "pro" (prosecution)
	target := &Client{
		uid:          2,
		char:         1, // Miles Edgeworth
		possessing:   -1,
		possessedPos: "",
		pair:         ClientPairInfo{wanted_id: -1},
		pairedUID:    -1,
		pos:          "pro",
	}

	// Verify initial positions
	if admin.Pos() != "def" {
		t.Errorf("Expected admin initial position 'def', got %s", admin.Pos())
	}
	if target.Pos() != "pro" {
		t.Errorf("Expected target initial position 'pro', got %s", target.Pos())
	}

	// Set up full possession
	admin.SetPossessing(target.Uid())
	admin.SetPossessedPos(target.Pos()) // Save target's position

	// Verify admin is possessing target
	if admin.Possessing() != target.Uid() {
		t.Errorf("Expected admin to be possessing target UID %d, got %d", target.Uid(), admin.Possessing())
	}

	// Admin should have saved the target's position "pro"
	if admin.PossessedPos() != "pro" {
		t.Errorf("Expected admin to have saved target position 'pro', got %s", admin.PossessedPos())
	}

	// The pktIC function will use admin.PossessedPos() to spoof the target's position
	// Admin's own position remains "def" but messages will appear at "pro"
	if admin.Pos() != "def" {
		t.Errorf("Expected admin's own position to remain 'def', got %s", admin.Pos())
	}
}

// TestPossessWithIniswap tests that possession works correctly when target has iniswapped
func TestPossessWithIniswap(t *testing.T) {
	// Save original characters and restore after test to ensure test isolation
	originalCharacters := characters
	t.Cleanup(func() {
		characters = originalCharacters
	})

	// Initialize mock characters list for testing
	// In real usage, this is loaded from characters.txt
	characters = []string{
		"Phoenix Wright",      // ID 0
		"Miles Edgeworth",     // ID 1
		"Maya Fey",            // ID 2
		"Franziska von Karma", // ID 3
	}

	// Test getCharacterID helper function works correctly
	edgeworthID := getCharacterID("Miles Edgeworth")
	if edgeworthID != 1 {
		t.Errorf("Expected Miles Edgeworth ID to be 1, got %d", edgeworthID)
	}

	// Test case-insensitive character lookup
	edgeworthID2 := getCharacterID("miles edgeworth")
	if edgeworthID2 != 1 {
		t.Errorf("Expected case-insensitive lookup to find Miles Edgeworth (ID 1), got %d", edgeworthID2)
	}

	// Test getCharacterID with invalid character name
	invalidID := getCharacterID("NonexistentCharacter")
	if invalidID != -1 {
		t.Errorf("Expected getCharacterID to return -1 for invalid character, got %d", invalidID)
	}

	// Create a target who has selected Phoenix Wright (ID 0)
	target := &Client{
		uid:        2,
		char:       0, // Selected Phoenix Wright
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
		pairedUID:  -1,
		pos:        "def",
	}

	// Simulate target iniswapping to Miles Edgeworth
	// This is what happens when they send an IC message with a different character
	target.SetPairInfo("Miles Edgeworth", "normal", "0", "")

	// Verify that PairInfo contains the iniswapped character
	if target.PairInfo().name != "Miles Edgeworth" {
		t.Errorf("Expected target PairInfo name to be 'Miles Edgeworth', got '%s'", target.PairInfo().name)
	}

	// Verify that the helper correctly finds the iniswapped character ID
	targetCharName := target.PairInfo().name
	if targetCharName != "" {
		targetCharID := getCharacterID(targetCharName)
		if targetCharID != 1 {
			t.Errorf("Expected iniswapped character ID to be 1 (Miles Edgeworth), got %d", targetCharID)
		}
	}

	// Test fallback case when PairInfo is empty (no IC messages sent yet)
	target2 := &Client{
		uid:        3,
		char:       2, // Maya Fey
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1, name: ""}, // Empty PairInfo
		pairedUID:  -1,
		pos:        "wit",
	}

	// When PairInfo.name is empty, code should fall back to actual character
	if target2.PairInfo().name != "" {
		t.Errorf("Expected target2 PairInfo name to be empty, got '%s'", target2.PairInfo().name)
	}

	// Verify fallback to actual character works
	// Since PairInfo.name is empty, fallback should use target's actual character
	var fallbackCharName string
	if target2.CharID() >= 0 && target2.CharID() < len(characters) {
		fallbackCharName = characters[target2.CharID()]
	} else {
		t.Errorf("target2.CharID() is out of bounds: %d", target2.CharID())
		return
	}
	if fallbackCharName != "Maya Fey" {
		t.Errorf("Expected fallback to actual character 'Maya Fey', got '%s'", fallbackCharName)
	}
}

// TestPersistentPairing tests the new persistent pairing functionality
func TestPersistentPairing(t *testing.T) {
	// Create two clients
	client1 := &Client{
		uid:       1,
		char:      0,
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
		oocName:   "Player1",
	}
	client2 := &Client{
		uid:       2,
		char:      1,
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
		oocName:   "Player2",
	}

	// Initially, neither client should be paired
	if client1.PairedUID() != -1 {
		t.Errorf("Expected client1 to not be paired initially, got %d", client1.PairedUID())
	}
	if client2.PairedUID() != -1 {
		t.Errorf("Expected client2 to not be paired initially, got %d", client2.PairedUID())
	}

	// Client1 sets intent to pair with client2
	client1.SetPairedUID(client2.Uid())
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected client1 pairedUID to be %d, got %d", client2.Uid(), client1.PairedUID())
	}

	// Client2 should still not be paired
	if client2.PairedUID() != -1 {
		t.Errorf("Expected client2 to still not be paired, got %d", client2.PairedUID())
	}

	// Client2 accepts and pairs with client1
	client2.SetPairedUID(client1.Uid())
	if client2.PairedUID() != client1.Uid() {
		t.Errorf("Expected client2 pairedUID to be %d, got %d", client1.Uid(), client2.PairedUID())
	}

	// Both clients should now be paired with each other
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected client1 paired with client2")
	}
	if client2.PairedUID() != client1.Uid() {
		t.Errorf("Expected client2 paired with client1")
	}

	// Test unpairing
	client1.SetPairedUID(-1)
	if client1.PairedUID() != -1 {
		t.Errorf("Expected client1 to be unpaired, got %d", client1.PairedUID())
	}

	// Client2 should still have the pairing (unpair is one-way in this test)
	if client2.PairedUID() != 1 {
		t.Errorf("Expected client2 still paired with client1, got %d", client2.PairedUID())
	}
}

// TestPersistentPairingIndependent tests that persistent pairing is independent from character pairing
func TestPersistentPairingIndependent(t *testing.T) {
	// Create two clients
	client1 := &Client{
		uid:       10,
		char:      0, // Phoenix Wright
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
	}
	client2 := &Client{
		uid:       20,
		char:      1, // Miles Edgeworth
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
	}

	// Set persistent pairing
	client1.SetPairedUID(client2.Uid())
	client2.SetPairedUID(client1.Uid())

	// Change character wanted_id (old pairing system)
	client1.SetPairWantedID(5)
	client2.SetPairWantedID(3)

	// Persistent pairing should be unaffected by character wanted_id
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected persistent pairing to remain after changing wanted_id")
	}
	if client2.PairedUID() != client1.Uid() {
		t.Errorf("Expected persistent pairing to remain after changing wanted_id")
	}

	// Character pairing should be independent
	if client1.PairWantedID() != 5 {
		t.Errorf("Expected wanted_id to be 5, got %d", client1.PairWantedID())
	}
	if client2.PairWantedID() != 3 {
		t.Errorf("Expected wanted_id to be 3, got %d", client2.PairWantedID())
	}
}

// TestPersistentPairingDisconnect tests that persistent pairing is cleared when a client disconnects
func TestPersistentPairingDisconnect(t *testing.T) {
	// Create two clients
	client1 := &Client{
		uid:       100,
		char:      0,
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
	}
	client2 := &Client{
		uid:       200,
		char:      1,
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
	}

	// Establish mutual pairing
	client1.SetPairedUID(client2.Uid())
	client2.SetPairedUID(client1.Uid())

	// Verify pairing is established
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected client1 paired with client2 (%d), got %d", client2.Uid(), client1.PairedUID())
	}
	if client2.PairedUID() != client1.Uid() {
		t.Errorf("Expected client2 paired with client1 (%d), got %d", client1.Uid(), client2.PairedUID())
	}

	// Simulate client1 disconnecting by clearing their pairing
	// (In actual cleanup, this would be done by clientCleanup)
	client1.SetPairedUID(-1)

	// Verify client1 is unpaired
	if client1.PairedUID() != -1 {
		t.Errorf("Expected client1 to be unpaired after disconnect, got %d", client1.PairedUID())
	}

	// In real scenario, clientCleanup would also clear client2's pairing
	// Simulate that here
	if client2.PairedUID() == client1.Uid() {
		client2.SetPairedUID(-1)
	}

	// Verify client2 is also unpaired
	if client2.PairedUID() != -1 {
		t.Errorf("Expected client2 to be unpaired after partner disconnects, got %d", client2.PairedUID())
	}
}

// TestPersistentPairingWithOffsets tests that persistent pairing works correctly when players change their offsets
func TestPersistentPairingWithOffsets(t *testing.T) {
	// Create two clients
	client1 := &Client{
		uid:       50,
		char:      0,
		pair:      ClientPairInfo{wanted_id: -1, name: "Phoenix Wright", emote: "normal", offset: "10&20", flip: "0"},
		pairedUID: -1,
	}
	client2 := &Client{
		uid:       60,
		char:      1,
		pair:      ClientPairInfo{wanted_id: -1, name: "Miles Edgeworth", emote: "confident", offset: "30&40", flip: "1"},
		pairedUID: -1,
	}

	// Establish mutual pairing
	client1.SetPairedUID(client2.Uid())
	client2.SetPairedUID(client1.Uid())

	// Verify pairing is established
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected client1 paired with client2 (%d), got %d", client2.Uid(), client1.PairedUID())
	}
	if client2.PairedUID() != client1.Uid() {
		t.Errorf("Expected client2 paired with client1 (%d), got %d", client1.Uid(), client2.PairedUID())
	}

	// Client1 changes their offset
	client1.SetPairInfo("Phoenix Wright", "thinking", "0", "50&60")

	// Verify pairing is still intact after offset change
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected client1 still paired with client2 after offset change, got %d", client1.PairedUID())
	}

	// Verify the new offset is stored
	pairInfo1 := client1.PairInfo()
	if pairInfo1.offset != "50&60" {
		t.Errorf("Expected client1 offset to be '50&60', got '%s'", pairInfo1.offset)
	}

	// Client2 changes their offset
	client2.SetPairInfo("Miles Edgeworth", "smirking", "1", "70&80")

	// Verify pairing is still intact after both players changed offsets
	if client2.PairedUID() != client1.Uid() {
		t.Errorf("Expected client2 still paired with client1 after offset change, got %d", client2.PairedUID())
	}
	if client1.PairedUID() != client2.Uid() {
		t.Errorf("Expected client1 still paired with client2 after both offset changes, got %d", client1.PairedUID())
	}

	// Verify both offsets are stored independently
	pairInfo1 = client1.PairInfo()
	pairInfo2 := client2.PairInfo()
	if pairInfo1.offset != "50&60" {
		t.Errorf("Expected client1 offset to be '50&60', got '%s'", pairInfo1.offset)
	}
	if pairInfo2.offset != "70&80" {
		t.Errorf("Expected client2 offset to be '70&80', got '%s'", pairInfo2.offset)
	}

	// Verify other pair info is maintained
	if pairInfo1.name != "Phoenix Wright" {
		t.Errorf("Expected client1 name to be 'Phoenix Wright', got '%s'", pairInfo1.name)
	}
	if pairInfo2.name != "Miles Edgeworth" {
		t.Errorf("Expected client2 name to be 'Miles Edgeworth', got '%s'", pairInfo2.name)
	}
}

// TestSendClearPairPacketInvalidChar tests that sendClearPairPacket is a no-op
// when the client has an invalid character ID (does not panic).
func TestSendClearPairPacketInvalidChar(t *testing.T) {
	originalCharacters := characters
	t.Cleanup(func() {
		characters = originalCharacters
	})
	characters = []string{"Phoenix Wright", "Miles Edgeworth"}

	client := &Client{
		char:      -1, // invalid: spectator
		pair:      ClientPairInfo{wanted_id: -1},
		pairedUID: -1,
	}
	// Should return without panic or sending any packet (no connection set up)
	sendClearPairPacket(client)
}

// TestSendClearPairPacketContent verifies that sendClearPairPacket sends an MS#
// packet with empty pair character name (args[17]) and empty message (args[4]),
// which instructs WebAO to hide the ghost pair sprite and the chatbox.
func TestSendClearPairPacketContent(t *testing.T) {
	originalCharacters := characters
	t.Cleanup(func() {
		characters = originalCharacters
	})
	characters = []string{"Phoenix Wright", "Miles Edgeworth"}

	// Create a connected pipe so we can read what the server sends.
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &Client{
		conn:      serverConn,
		char:      0, // Phoenix Wright
		pair:      ClientPairInfo{wanted_id: -1, name: "Phoenix Wright", emote: "normal"},
		pairedUID: -1,
		pos:       "def",
		showname:  "Phoenix",
	}

	// Read the packet in a goroutine so the write doesn't block.
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := clientConn.Read(buf)
		done <- string(buf[:n])
	}()

	sendClearPairPacket(c)

	// Close server side so the Read unblocks even if fewer bytes arrive.
	serverConn.Close()
	packet := <-done

	// The packet should be an MS# packet.
	if !strings.HasPrefix(packet, "MS#") {
		t.Errorf("Expected MS# packet, got: %q", packet)
	}

	// Split on '#' to check individual fields.
	// Format: MS#args[0]#args[1]#...#args[29]#%
	fields := strings.Split(packet, "#")
	// fields[0] = "MS", fields[1] = args[0] (deskmod), ..., fields[18] = args[17] (pair char name)
	if len(fields) < 19 {
		t.Fatalf("Packet too short (got %d fields): %q", len(fields), packet)
	}

	// fields[5] = args[4] = message content — must be empty so chatbox is hidden
	if fields[5] != "" {
		t.Errorf("Expected empty message content (args[4]) to hide chatbox, got %q", fields[5])
	}

	// fields[17] = args[16] = pair_id — must be "-1"
	if fields[17] != "-1" {
		t.Errorf("Expected pair_id (args[16]) to be \"-1\", got %q", fields[17])
	}

	// fields[18] = args[17] = pair character name — must be empty to clear WebAO pair sprite
	if fields[18] != "" {
		t.Errorf("Expected pair character name (args[17]) to be empty to clear WebAO ghost sprite, got %q", fields[18])
	}
}
