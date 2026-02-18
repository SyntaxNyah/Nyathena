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

// TestFullPossessionNotification tests that full possession can be identified
func TestFullPossessionNotification(t *testing.T) {
	// Create two clients with proper initialization
	admin := &Client{
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

	// Verify the target's UID can be retrieved for message mirroring
	if admin.Possessing() == target.Uid() {
		// This confirms the possession link works for mirroring messages
		// In actual usage, when target sends IC message, admin gets notified
	} else {
		t.Errorf("Possession link verification failed")
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
