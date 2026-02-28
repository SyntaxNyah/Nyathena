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
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	tmp, err := os.CreateTemp("", "athena-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmp.Close()
	DBPath = tmp.Name()
	if err := Open(); err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return func() {
		Close()
		os.Remove(tmp.Name())
	}
}

func TestUpsertAndDeleteMute(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid1"

	// Store a permanent mute.
	if err := UpsertMute(ipid, 1 /* ICMuted */, 0); err != nil {
		t.Fatalf("UpsertMute failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 1 {
		t.Fatalf("expected 1 punishment, got %d", len(punishments))
	}
	p := punishments[0]
	if p.Kind != PunishKindMute {
		t.Errorf("expected Kind=%d, got %d", PunishKindMute, p.Kind)
	}
	if p.Value != 1 {
		t.Errorf("expected Value=1 (ICMuted), got %d", p.Value)
	}
	if p.Expires != 0 {
		t.Errorf("expected Expires=0 (permanent), got %d", p.Expires)
	}

	// Overwrite with a different mute type.
	if err := UpsertMute(ipid, 2 /* OOCMuted */, 0); err != nil {
		t.Fatalf("UpsertMute (overwrite) failed: %v", err)
	}
	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 1 {
		t.Fatalf("expected 1 punishment after overwrite, got %d", len(punishments))
	}
	if punishments[0].Value != 2 {
		t.Errorf("expected Value=2 (OOCMuted) after overwrite, got %d", punishments[0].Value)
	}

	// Delete mute.
	if err := DeleteMute(ipid); err != nil {
		t.Fatalf("DeleteMute failed: %v", err)
	}
	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after delete failed: %v", err)
	}
	if len(punishments) != 0 {
		t.Errorf("expected 0 punishments after DeleteMute, got %d", len(punishments))
	}
}

func TestUpsertAndDeleteJail(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid2"
	future := time.Now().Add(1 * time.Hour).Unix()

	if err := UpsertJail(ipid, future, "test jail"); err != nil {
		t.Fatalf("UpsertJail failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 1 {
		t.Fatalf("expected 1 punishment, got %d", len(punishments))
	}
	p := punishments[0]
	if p.Kind != PunishKindJail {
		t.Errorf("expected Kind=%d, got %d", PunishKindJail, p.Kind)
	}
	if p.Expires != future {
		t.Errorf("expected Expires=%d, got %d", future, p.Expires)
	}
	if p.Reason != "test jail" {
		t.Errorf("expected Reason='test jail', got %q", p.Reason)
	}

	if err := DeleteJail(ipid); err != nil {
		t.Fatalf("DeleteJail failed: %v", err)
	}
	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after delete failed: %v", err)
	}
	if len(punishments) != 0 {
		t.Errorf("expected 0 punishments after DeleteJail, got %d", len(punishments))
	}
}

func TestUpsertAndDeleteTextPunishment(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid3"
	future := time.Now().Add(10 * time.Minute).Unix()

	// Add two different text punishments.
	if err := UpsertTextPunishment(ipid, 5 /* PunishmentUppercase */, future, "test reason"); err != nil {
		t.Fatalf("UpsertTextPunishment failed: %v", err)
	}
	if err := UpsertTextPunishment(ipid, 6 /* PunishmentLowercase */, future, "another reason"); err != nil {
		t.Fatalf("UpsertTextPunishment (second) failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 2 {
		t.Fatalf("expected 2 text punishments, got %d", len(punishments))
	}
	for _, p := range punishments {
		if p.Kind != PunishKindText {
			t.Errorf("expected Kind=%d, got %d", PunishKindText, p.Kind)
		}
	}

	// Delete one specific punishment.
	if err := DeleteTextPunishment(ipid, 5); err != nil {
		t.Fatalf("DeleteTextPunishment failed: %v", err)
	}
	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after partial delete failed: %v", err)
	}
	if len(punishments) != 1 {
		t.Fatalf("expected 1 punishment after partial delete, got %d", len(punishments))
	}
	if punishments[0].Subtype != 6 {
		t.Errorf("expected remaining Subtype=6, got %d", punishments[0].Subtype)
	}

	// Delete all text punishments.
	if err := DeleteAllTextPunishments(ipid); err != nil {
		t.Fatalf("DeleteAllTextPunishments failed: %v", err)
	}
	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after full delete failed: %v", err)
	}
	if len(punishments) != 0 {
		t.Errorf("expected 0 punishments after DeleteAllTextPunishments, got %d", len(punishments))
	}
}

func TestGetPunishmentsFiltersExpired(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid4"

	// Store a punishment that already expired.
	past := time.Now().Add(-1 * time.Hour).Unix()
	if err := UpsertTextPunishment(ipid, 5, past, "expired"); err != nil {
		t.Fatalf("UpsertTextPunishment failed: %v", err)
	}
	// Store a punishment that is still active.
	future := time.Now().Add(1 * time.Hour).Unix()
	if err := UpsertTextPunishment(ipid, 6, future, "active"); err != nil {
		t.Fatalf("UpsertTextPunishment (active) failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 1 {
		t.Fatalf("expected 1 active punishment (expired one filtered), got %d", len(punishments))
	}
	if punishments[0].Subtype != 6 {
		t.Errorf("expected active punishment Subtype=6, got %d", punishments[0].Subtype)
	}
}

func TestGetPunishmentsReturnsPermanent(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid5"

	// A permanent punishment has Expires=0 and should never be filtered.
	if err := UpsertTextPunishment(ipid, 7, 0, "permanent"); err != nil {
		t.Fatalf("UpsertTextPunishment failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 1 {
		t.Fatalf("expected 1 permanent punishment, got %d", len(punishments))
	}
	if punishments[0].Expires != 0 {
		t.Errorf("expected Expires=0 for permanent punishment, got %d", punishments[0].Expires)
	}
}
