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

// TestLinkIPIDToUserMergesPlaytime verifies that when a player re-logs from a
// new IP address, the playtime accumulated under their old IPID is transferred
// to the new IPID so the leaderboard continues to show the correct total.
func TestLinkIPIDToUserMergesPlaytime(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// Register from old IPID and simulate accumulated playtime.
	if err := RegisterPlayer("migrant", []byte("pass1234"), "migrate_old"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}
	if err := MarkIPKnown("migrate_old"); err != nil {
		t.Fatalf("MarkIPKnown (old) failed: %v", err)
	}
	if err := AddPlaytime("migrate_old", 3600); err != nil {
		t.Fatalf("AddPlaytime failed: %v", err)
	}

	// Simulate the new connection and re-login from a different IP.
	if err := MarkIPKnown("migrate_new"); err != nil {
		t.Fatalf("MarkIPKnown (new) failed: %v", err)
	}
	if err := LinkIPIDToUser("migrant", "migrate_new"); err != nil {
		t.Fatalf("LinkIPIDToUser failed: %v", err)
	}

	// Old IPID's playtime should have been zeroed out.
	oldPT, err := GetPlaytime("migrate_old")
	if err != nil {
		t.Fatalf("GetPlaytime (old) failed: %v", err)
	}
	if oldPT != 0 {
		t.Errorf("expected old IPID playtime=0 after merge, got %d", oldPT)
	}

	// New IPID should have inherited the old playtime.
	newPT, err := GetPlaytime("migrate_new")
	if err != nil {
		t.Fatalf("GetPlaytime (new) failed: %v", err)
	}
	if newPT != 3600 {
		t.Errorf("expected new IPID playtime=3600 after merge, got %d", newPT)
	}
}

// TestLinkIPIDToUserMergesPlaytimeAdditive verifies that when both the old and
// new IPID already have playtime, the amounts are summed.
func TestLinkIPIDToUserMergesPlaytimeAdditive(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("addplayer", []byte("pass1234"), "add_old"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}
	if err := MarkIPKnown("add_old"); err != nil {
		t.Fatalf("MarkIPKnown (old) failed: %v", err)
	}
	if err := AddPlaytime("add_old", 1000); err != nil {
		t.Fatalf("AddPlaytime (old) failed: %v", err)
	}

	if err := MarkIPKnown("add_new"); err != nil {
		t.Fatalf("MarkIPKnown (new) failed: %v", err)
	}
	if err := AddPlaytime("add_new", 500); err != nil {
		t.Fatalf("AddPlaytime (new) failed: %v", err)
	}

	if err := LinkIPIDToUser("addplayer", "add_new"); err != nil {
		t.Fatalf("LinkIPIDToUser failed: %v", err)
	}

	newPT, err := GetPlaytime("add_new")
	if err != nil {
		t.Fatalf("GetPlaytime (new) failed: %v", err)
	}
	if newPT != 1500 {
		t.Errorf("expected combined playtime=1500, got %d", newPT)
	}

	oldPT, err := GetPlaytime("add_old")
	if err != nil {
		t.Fatalf("GetPlaytime (old) failed: %v", err)
	}
	if oldPT != 0 {
		t.Errorf("expected old IPID playtime=0 after merge, got %d", oldPT)
	}
}

// TestLinkIPIDToUserSameIPIDNoOp verifies that re-logging with the same IPID
// (no IP change) does not modify any KNOWN_IPS playtime.
func TestLinkIPIDToUserSameIPIDNoOp(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := RegisterPlayer("stable", []byte("pass1234"), "stable_ip"); err != nil {
		t.Fatalf("RegisterPlayer failed: %v", err)
	}
	if err := MarkIPKnown("stable_ip"); err != nil {
		t.Fatalf("MarkIPKnown failed: %v", err)
	}
	if err := AddPlaytime("stable_ip", 7200); err != nil {
		t.Fatalf("AddPlaytime failed: %v", err)
	}

	// Re-link with the same IPID — should be a no-op.
	if err := LinkIPIDToUser("stable", "stable_ip"); err != nil {
		t.Fatalf("LinkIPIDToUser failed: %v", err)
	}

	pt, err := GetPlaytime("stable_ip")
	if err != nil {
		t.Fatalf("GetPlaytime failed: %v", err)
	}
	if pt != 7200 {
		t.Errorf("expected playtime unchanged at 7200, got %d", pt)
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

// TestOneAccountPerIPID verifies the one-account-per-IPID gate: once an IPID is
// linked to an account, GetUsernameByIPID returns a non-empty string, which the
// /register command uses to block a second registration attempt.
func TestOneAccountPerIPID(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "shared_ipid"

	// First registration succeeds.
	if err := RegisterPlayer("first", []byte("pass1234"), ipid); err != nil {
		t.Fatalf("first RegisterPlayer failed: %v", err)
	}

	// GetUsernameByIPID must return the first account — this is what cmdRegister
	// checks before allowing a second registration from the same IPID.
	username, err := GetUsernameByIPID(ipid)
	if err != nil {
		t.Fatalf("GetUsernameByIPID failed: %v", err)
	}
	if username != "first" {
		t.Errorf("expected 'first', got %q", username)
	}

	// Attempting to register a second account with the same IPID should be
	// blocked by cmdRegister because GetUsernameByIPID returns a non-empty value.
	// At the DB layer the INSERT succeeds (different username), so we verify the
	// gate works at the command layer by confirming the lookup returns non-empty.
	if username == "" {
		t.Error("IPID should already be linked; cmdRegister would wrongly allow a second account")
	}
}
