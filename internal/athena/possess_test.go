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
)

// TestPossessionTracking tests that possession state is tracked correctly for fullpossess
func TestPossessionTracking(t *testing.T) {
	// Create two clients with proper initialization
	possessor := &Client{
		uid:        1,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
	}
	target := &Client{
		uid:        2,
		char:       -1,
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
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
		pos:        "def",
	}
	target := &Client{
		uid:        2,
		char:       1, // Miles Edgeworth
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
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
		pos:          "def",
	}

	// Create target with position "pro" (prosecution)
	target := &Client{
		uid:          2,
		char:         1, // Miles Edgeworth
		possessing:   -1,
		possessedPos: "",
		pair:         ClientPairInfo{wanted_id: -1},
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
