/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the /lockdown whitelist passkey exemption persistence layer. */

package db

import (
	"database/sql"
	"errors"
	"testing"
)

func TestAddLockdownExemptIsUpsert(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := AddLockdownExempt("ipid_a", "admin1", 1000); err != nil {
		t.Fatalf("AddLockdownExempt failed: %v", err)
	}
	if err := AddLockdownExempt("ipid_a", "admin2", 2000); err != nil {
		t.Fatalf("re-issuing AddLockdownExempt failed: %v", err)
	}

	rows, err := ListLockdownExempts()
	if err != nil {
		t.Fatalf("ListLockdownExempts failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row after re-issuing, got %d", len(rows))
	}
	if rows[0].IssuedBy != "admin2" || rows[0].IssuedAt != 2000 {
		t.Fatalf("expected the second issue to overwrite the first, got %+v", rows[0])
	}
}

func TestRemoveLockdownExempt(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := AddLockdownExempt("ipid_b", "admin1", 1000); err != nil {
		t.Fatalf("AddLockdownExempt failed: %v", err)
	}
	if err := RemoveLockdownExempt("ipid_b"); err != nil {
		t.Fatalf("RemoveLockdownExempt failed: %v", err)
	}

	rows, err := ListLockdownExempts()
	if err != nil {
		t.Fatalf("ListLockdownExempts failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected ipid_b to no longer be listed, got %+v", rows)
	}
}

func TestRemoveLockdownExemptNoRows(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	err := RemoveLockdownExempt("nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for a non-existent entry, got %v", err)
	}
}

func TestListLockdownExemptsNewestFirst(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := AddLockdownExempt("ipid_old", "admin1", 1000); err != nil {
		t.Fatalf("AddLockdownExempt failed: %v", err)
	}
	if err := AddLockdownExempt("ipid_new", "admin1", 2000); err != nil {
		t.Fatalf("AddLockdownExempt failed: %v", err)
	}

	rows, err := ListLockdownExempts()
	if err != nil {
		t.Fatalf("ListLockdownExempts failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Ipid != "ipid_new" || rows[1].Ipid != "ipid_old" {
		t.Fatalf("expected newest-first ordering, got %+v", rows)
	}
}
