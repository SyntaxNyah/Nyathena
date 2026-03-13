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

package db

import (
	"testing"
)

// TestRegisterPlayerCreatesAccount verifies that RegisterPlayer creates an account
// with zero permissions and the correct IPID linkage.
func TestRegisterPlayerCreatesAccount(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("testplayer", []byte("secret123"), "ipid_abc"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}

	if !UserExists("testplayer") {
		t.Fatal("account should exist after RegisterPlayer")
	}

	// Permissions must be zero — no extra powers.
	ok, perms := AuthenticateUser("testplayer", []byte("secret123"))
	if !ok {
		t.Fatal("should be able to authenticate with registered credentials")
	}
	if perms != 0 {
		t.Errorf("expected permissions=0, got %d", perms)
	}
}

// TestRegisterPlayerLinksIPID verifies that the IPID is immediately resolvable
// to the new account name after RegisterPlayer.
func TestRegisterPlayerLinksIPID(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("linktest", []byte("password1"), "ipid_xyz"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}

	username, err := GetUsernameByIPID("ipid_xyz")
	if err != nil {
		t.Fatalf("GetUsernameByIPID failed: %v", err)
	}
	if username != "linktest" {
		t.Errorf("expected username 'linktest', got %q", username)
	}
}

// TestGetUsernameByIPIDNotFound verifies that an unknown IPID returns ("", nil).
func TestGetUsernameByIPIDNotFound(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	username, err := GetUsernameByIPID("unknown_ipid")
	if err != nil {
		t.Fatalf("GetUsernameByIPID returned unexpected error: %v", err)
	}
	if username != "" {
		t.Errorf("expected empty string for unknown IPID, got %q", username)
	}
}

// TestLinkIPIDToUser verifies that LinkIPIDToUser updates the stored IPID.
func TestLinkIPIDToUser(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// Create a mod account via the standard path (no IPID set yet).
	if err := CreateUser("moduser", []byte("modpass"), 0xFFFF); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Simulate login linking the IPID.
	if err := LinkIPIDToUser("moduser", "ipid_mod1"); err != nil {
		t.Fatalf("LinkIPIDToUser failed: %v", err)
	}

	username, err := GetUsernameByIPID("ipid_mod1")
	if err != nil {
		t.Fatalf("GetUsernameByIPID failed: %v", err)
	}
	if username != "moduser" {
		t.Errorf("expected 'moduser', got %q", username)
	}
}

// TestLinkIPIDToUserUpdatesOnRelogin verifies that logging in from a new
// connection updates the IPID association.
func TestLinkIPIDToUserUpdatesOnRelogin(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("roamer", []byte("pass12345"), "ipid_old"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}

	// Simulate a new login from a different connection IPID.
	if err := LinkIPIDToUser("roamer", "ipid_new"); err != nil {
		t.Fatalf("LinkIPIDToUser failed: %v", err)
	}

	// Old IPID should now return nothing.
	u, _ := GetUsernameByIPID("ipid_old")
	if u != "" {
		t.Errorf("old IPID should no longer be linked, but got %q", u)
	}

	// New IPID should resolve to the account.
	u, _ = GetUsernameByIPID("ipid_new")
	if u != "roamer" {
		t.Errorf("expected 'roamer' for new IPID, got %q", u)
	}
}

// TestRegisterPlayerDuplicateUsername verifies that registering the same
// username twice returns an error.
func TestRegisterPlayerDuplicateUsername(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("taken", []byte("pass1234"), "ipid_1"); err != nil {
		t.Fatalf("first RegisterPlayer failed: %v", err)
	}
	if err := RegisterPlayer("taken", []byte("pass5678"), "ipid_2"); err == nil {
		t.Fatal("expected error for duplicate username, got nil")
	}
}

// TestGetUsernamesByIPIDsEmpty verifies that an empty input returns an empty map.
func TestGetUsernamesByIPIDsEmpty(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	m, err := GetUsernamesByIPIDs([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

// TestGetUsernamesByIPIDsSingle verifies that a single known IPID is resolved.
func TestGetUsernamesByIPIDsSingle(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("batchuser1", []byte("pass1234"), "batch_ipid_1"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}

	m, err := GetUsernamesByIPIDs([]string{"batch_ipid_1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["batch_ipid_1"] != "batchuser1" {
		t.Errorf("expected 'batchuser1', got %q", m["batch_ipid_1"])
	}
}

// TestGetUsernamesByIPIDsMultiple verifies that multiple IPIDs are resolved in one call.
func TestGetUsernamesByIPIDsMultiple(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("bulka", []byte("pass1234"), "bulk_1"); err != nil {
		t.Fatalf("RegisterPlayer bulka: %v", err)
	}
	if err := RegisterPlayer("bulkb", []byte("pass5678"), "bulk_2"); err != nil {
		t.Fatalf("RegisterPlayer bulkb: %v", err)
	}

	m, err := GetUsernamesByIPIDs([]string{"bulk_1", "bulk_2", "bulk_unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["bulk_1"] != "bulka" {
		t.Errorf("expected 'bulka', got %q", m["bulk_1"])
	}
	if m["bulk_2"] != "bulkb" {
		t.Errorf("expected 'bulkb', got %q", m["bulk_2"])
	}
	if _, ok := m["bulk_unknown"]; ok {
		t.Error("unknown IPID should be absent from the result map")
	}
}

// TestExistingModAccountsCompatible verifies that accounts created via
// CreateUser (the original admin path) still authenticate correctly after
// the IPID column migration.
func TestExistingModAccountsCompatible(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := CreateUser("admin", []byte("adminpass"), 0xFFFF); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	ok, perms := AuthenticateUser("admin", []byte("adminpass"))
	if !ok {
		t.Fatal("existing mod account should still authenticate")
	}
	if perms == 0 {
		t.Error("mod account should have non-zero permissions")
	}
}
