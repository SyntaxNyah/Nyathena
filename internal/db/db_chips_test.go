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

// TestEnsureChipBalanceCreatesRow verifies that a new IPID starts with 100 chips.
func TestEnsureChipBalanceCreatesRow(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testchip1"
	if err := EnsureChipBalance(ipid); err != nil {
		t.Fatalf("EnsureChipBalance failed: %v", err)
	}

	bal, err := GetChipBalance(ipid)
	if err != nil {
		t.Fatalf("GetChipBalance failed: %v", err)
	}
	if bal != 100 {
		t.Errorf("expected starting balance of 100, got %d", bal)
	}
}

// TestEnsureChipBalanceIdempotent verifies that calling EnsureChipBalance twice
// does not reset an existing balance.
func TestEnsureChipBalanceIdempotent(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testchip2"
	if err := EnsureChipBalance(ipid); err != nil {
		t.Fatalf("EnsureChipBalance (1st) failed: %v", err)
	}

	// Win some chips.
	if _, err := AddChips(ipid, 50); err != nil {
		t.Fatalf("AddChips failed: %v", err)
	}

	// Calling Ensure again must not clobber the balance.
	if err := EnsureChipBalance(ipid); err != nil {
		t.Fatalf("EnsureChipBalance (2nd) failed: %v", err)
	}

	bal, err := GetChipBalance(ipid)
	if err != nil {
		t.Fatalf("GetChipBalance failed: %v", err)
	}
	if bal != 150 {
		t.Errorf("expected balance 150 after AddChips(50), got %d", bal)
	}
}

// TestGetChipBalanceUnknownIPID verifies that an unknown IPID returns 0 without error.
func TestGetChipBalanceUnknownIPID(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	bal, err := GetChipBalance("unknown_ipid")
	if err != nil {
		t.Fatalf("GetChipBalance for unknown IPID returned error: %v", err)
	}
	if bal != 0 {
		t.Errorf("expected 0 for unknown IPID, got %d", bal)
	}
}

// TestAddChips verifies that AddChips increases the balance correctly.
func TestAddChips(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testchip3"
	EnsureChipBalance(ipid)

	newBal, err := AddChips(ipid, 200)
	if err != nil {
		t.Fatalf("AddChips failed: %v", err)
	}
	if newBal != 300 {
		t.Errorf("expected 300 after AddChips(200) on 100, got %d", newBal)
	}

	stored, _ := GetChipBalance(ipid)
	if stored != 300 {
		t.Errorf("stored balance mismatch: expected 300, got %d", stored)
	}
}

// TestSpendChips verifies that SpendChips decreases the balance correctly.
func TestSpendChips(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testchip4"
	EnsureChipBalance(ipid)

	newBal, err := SpendChips(ipid, 40)
	if err != nil {
		t.Fatalf("SpendChips failed: %v", err)
	}
	if newBal != 60 {
		t.Errorf("expected 60 after SpendChips(40) on 100, got %d", newBal)
	}
}

// TestSpendChipsInsufficientFunds verifies that SpendChips returns an error
// when the balance is too low (no negative balances allowed).
func TestSpendChipsInsufficientFunds(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testchip5"
	EnsureChipBalance(ipid)

	_, err := SpendChips(ipid, 500)
	if err == nil {
		t.Fatal("expected error for insufficient chips, got nil")
	}

	// Balance must remain unchanged.
	bal, _ := GetChipBalance(ipid)
	if bal != 100 {
		t.Errorf("balance should still be 100 after failed spend, got %d", bal)
	}
}

// TestGetTopChipBalances verifies the leaderboard ordering for registered players.
func TestGetTopChipBalances(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// Register accounts so the INNER JOIN can resolve names.
	players := []struct {
		username string
		ipid     string
		balance  int64
	}{
		{"lb2user", "lb2", 1000},
		{"lb1user", "lb1", 500},
		{"lb3user", "lb3", 250},
	}
	for _, p := range players {
		if err := RegisterPlayer(p.username, []byte("pass1234"), p.ipid); err != nil {
			t.Fatalf("RegisterPlayer %v: %v", p.username, err)
		}
		EnsureChipBalance(p.ipid)
		if extra := p.balance - 100; extra > 0 {
			if _, err := AddChips(p.ipid, extra); err != nil {
				t.Fatalf("AddChips failed for %v: %v", p.ipid, err)
			}
		}
	}

	entries, err := GetTopChipBalances(3)
	if err != nil {
		t.Fatalf("GetTopChipBalances failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be ordered descending: lb2user=1000, lb1user=500, lb3user=250
	expected := []struct {
		username string
		balance  int64
	}{{"lb2user", 1000}, {"lb1user", 500}, {"lb3user", 250}}
	for i, e := range expected {
		if entries[i].Username != e.username {
			t.Errorf("entry %d: expected username %v, got %v", i, e.username, entries[i].Username)
		}
		if entries[i].Balance != e.balance {
			t.Errorf("entry %d: expected balance %d, got %d", i, e.balance, entries[i].Balance)
		}
	}
}

// TestGetTopChipBalancesLimit verifies the limit parameter is respected.
func TestGetTopChipBalancesLimit(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	for i := 0; i < 5; i++ {
		ipid := "limitip" + string(rune('0'+i))
		username := "limituser" + string(rune('0'+i))
		if err := RegisterPlayer(username, []byte("pass1234"), ipid); err != nil {
			t.Fatalf("RegisterPlayer %v: %v", username, err)
		}
		EnsureChipBalance(ipid)
		AddChips(ipid, int64(i*100))
	}

	entries, err := GetTopChipBalances(3)
	if err != nil {
		t.Fatalf("GetTopChipBalances failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries with limit=3, got %d", len(entries))
	}
}

// TestAddChipsCapAtMaxBalance verifies that AddChips never exceeds MaxChipBalance.
func TestAddChipsCapAtMaxBalance(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "capcip1"
	EnsureChipBalance(ipid)

	// Add a huge amount that would far exceed the cap.
	newBal, err := AddChips(ipid, MaxChipBalance*10)
	if err != nil {
		t.Fatalf("AddChips failed: %v", err)
	}
	if newBal != MaxChipBalance {
		t.Errorf("expected balance to be capped at %d, got %d", MaxChipBalance, newBal)
	}

	stored, _ := GetChipBalance(ipid)
	if stored != MaxChipBalance {
		t.Errorf("stored balance mismatch: expected %d, got %d", MaxChipBalance, stored)
	}
}

// TestAddChipsNearCapStaysAtCap verifies repeated AddChips near the ceiling stays clamped.
func TestAddChipsNearCapStaysAtCap(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "capcip2"
	EnsureChipBalance(ipid)

	// Bring the balance to exactly MaxChipBalance.
	if _, err := AddChips(ipid, MaxChipBalance-100); err != nil {
		t.Fatalf("AddChips to near-cap failed: %v", err)
	}

	bal, _ := GetChipBalance(ipid)
	if bal != MaxChipBalance {
		t.Fatalf("setup: expected %d, got %d", MaxChipBalance, bal)
	}

	// Adding more chips at the ceiling should leave the balance unchanged.
	newBal, err := AddChips(ipid, 500)
	if err != nil {
		t.Fatalf("AddChips at cap failed: %v", err)
	}
	if newBal != MaxChipBalance {
		t.Errorf("expected balance to remain at cap %d, got %d", MaxChipBalance, newBal)
	}
}
