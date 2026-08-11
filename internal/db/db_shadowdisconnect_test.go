/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the /shadowdisconnect persistence layer. */

package db

import (
	"database/sql"
	"errors"
	"testing"
)

func TestAddAndIsShadowDisconnected(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ok, err := IsShadowDisconnected("ipid_a")
	if err != nil {
		t.Fatalf("IsShadowDisconnected failed: %v", err)
	}
	if ok {
		t.Fatal("expected ipid_a to not be shadow-disconnected yet")
	}

	if err := AddShadowDisconnect("ipid_a", "test reason", "admin1", 1000); err != nil {
		t.Fatalf("AddShadowDisconnect failed: %v", err)
	}

	ok, err = IsShadowDisconnected("ipid_a")
	if err != nil {
		t.Fatalf("IsShadowDisconnected failed: %v", err)
	}
	if !ok {
		t.Fatal("expected ipid_a to be shadow-disconnected")
	}
}

func TestAddShadowDisconnectIsUpsert(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := AddShadowDisconnect("ipid_b", "first reason", "admin1", 1000); err != nil {
		t.Fatalf("AddShadowDisconnect failed: %v", err)
	}
	if err := AddShadowDisconnect("ipid_b", "second reason", "admin2", 2000); err != nil {
		t.Fatalf("re-issuing AddShadowDisconnect failed: %v", err)
	}

	rows, err := ListShadowDisconnects()
	if err != nil {
		t.Fatalf("ListShadowDisconnects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row after re-issuing, got %d", len(rows))
	}
	if rows[0].Reason != "second reason" || rows[0].IssuedBy != "admin2" {
		t.Fatalf("expected the second issue to overwrite the first, got %+v", rows[0])
	}
}

func TestRemoveShadowDisconnect(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := AddShadowDisconnect("ipid_c", "", "admin1", 1000); err != nil {
		t.Fatalf("AddShadowDisconnect failed: %v", err)
	}
	if err := RemoveShadowDisconnect("ipid_c"); err != nil {
		t.Fatalf("RemoveShadowDisconnect failed: %v", err)
	}

	ok, err := IsShadowDisconnected("ipid_c")
	if err != nil {
		t.Fatalf("IsShadowDisconnected failed: %v", err)
	}
	if ok {
		t.Fatal("expected ipid_c to no longer be shadow-disconnected")
	}
}

func TestRemoveShadowDisconnectNoRows(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	err := RemoveShadowDisconnect("nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for a non-existent entry, got %v", err)
	}
}

func TestClearAllShadowDisconnects(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	for _, ipid := range []string{"ipid_d", "ipid_e", "ipid_f"} {
		if err := AddShadowDisconnect(ipid, "", "admin1", 1000); err != nil {
			t.Fatalf("AddShadowDisconnect(%v) failed: %v", ipid, err)
		}
	}

	n, err := ClearAllShadowDisconnects()
	if err != nil {
		t.Fatalf("ClearAllShadowDisconnects failed: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 entries cleared, got %d", n)
	}

	rows, err := ListShadowDisconnects()
	if err != nil {
		t.Fatalf("ListShadowDisconnects failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected an empty list after clearing, got %d entries", len(rows))
	}
}

func TestListShadowDisconnectsNewestFirst(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := AddShadowDisconnect("ipid_old", "", "admin1", 1000); err != nil {
		t.Fatalf("AddShadowDisconnect failed: %v", err)
	}
	if err := AddShadowDisconnect("ipid_new", "", "admin1", 2000); err != nil {
		t.Fatalf("AddShadowDisconnect failed: %v", err)
	}

	rows, err := ListShadowDisconnects()
	if err != nil {
		t.Fatalf("ListShadowDisconnects failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Ipid != "ipid_new" || rows[1].Ipid != "ipid_old" {
		t.Fatalf("expected newest-first ordering, got %+v", rows)
	}
}
