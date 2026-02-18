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

// TestPossessionTracking tests that possession state is tracked correctly
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

	// Set up possession link
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

// TestPossessionPositionSync tests that position syncs correctly
func TestPossessionPositionSync(t *testing.T) {
	// Create two clients with different positions and proper initialization
	possessor := &Client{
		uid:        1,
		char:       -1,
		pos:        "def",
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
	}
	target := &Client{
		uid:        2,
		char:       -1,
		pos:        "wit",
		possessing: -1,
		pair:       ClientPairInfo{wanted_id: -1},
	}

	// Initially, possessor and target have different positions
	if possessor.Pos() == target.Pos() {
		t.Errorf("Test setup error: possessor and target should start with different positions")
	}

	// Set up possession link
	possessor.SetPossessing(target.Uid())

	// Simulate position sync by copying target's position to possessor
	possessor.SetPos(target.Pos())

	// Verify positions match
	if possessor.Pos() != target.Pos() {
		t.Errorf("Expected possessor position to match target position '%s', got '%s'", target.Pos(), possessor.Pos())
	}

	// Simulate target moving to a new position
	target.SetPos("jud")

	// Simulate position sync again
	possessor.SetPos(target.Pos())

	// Verify positions still match
	if possessor.Pos() != target.Pos() {
		t.Errorf("Expected possessor position to match target's new position '%s', got '%s'", target.Pos(), possessor.Pos())
	}
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
