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
	"database/sql"
	"errors"
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

	if err := UpsertJail(ipid, future, "test jail", -1); err != nil {
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

	// Delete all punishments via DeleteAllPunishments.
	if err := DeleteAllPunishments(ipid); err != nil {
		t.Fatalf("DeleteAllPunishments failed: %v", err)
	}
	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after full delete failed: %v", err)
	}
	if len(punishments) != 0 {
		t.Errorf("expected 0 punishments after DeleteAllPunishments, got %d", len(punishments))
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

func TestDeleteAllPunishments(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid6"

	// Add one of each kind.
	if err := UpsertMute(ipid, 1, 0); err != nil {
		t.Fatalf("UpsertMute failed: %v", err)
	}
	if err := UpsertJail(ipid, time.Now().Add(1*time.Hour).Unix(), "", -1); err != nil {
		t.Fatalf("UpsertJail failed: %v", err)
	}
	if err := UpsertTextPunishment(ipid, 5, 0, ""); err != nil {
		t.Fatalf("UpsertTextPunishment failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments failed: %v", err)
	}
	if len(punishments) != 3 {
		t.Fatalf("expected 3 punishments before delete, got %d", len(punishments))
	}

	if err := DeleteAllPunishments(ipid); err != nil {
		t.Fatalf("DeleteAllPunishments failed: %v", err)
	}

	punishments, err = GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after DeleteAllPunishments failed: %v", err)
	}
	if len(punishments) != 0 {
		t.Errorf("expected 0 punishments after DeleteAllPunishments, got %d", len(punishments))
	}
}

func TestPurgeExpired(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "testipid7"

	past := time.Now().Add(-1 * time.Hour).Unix()
	future := time.Now().Add(1 * time.Hour).Unix()

	// One expired, one active, one permanent.
	if err := UpsertTextPunishment(ipid, 1, past, "expired"); err != nil {
		t.Fatalf("UpsertTextPunishment (expired) failed: %v", err)
	}
	if err := UpsertTextPunishment(ipid, 2, future, "active"); err != nil {
		t.Fatalf("UpsertTextPunishment (active) failed: %v", err)
	}
	if err := UpsertTextPunishment(ipid, 3, 0, "permanent"); err != nil {
		t.Fatalf("UpsertTextPunishment (permanent) failed: %v", err)
	}

	if err := PurgeExpired(); err != nil {
		t.Fatalf("PurgeExpired failed: %v", err)
	}

	punishments, err := GetPunishments(ipid)
	if err != nil {
		t.Fatalf("GetPunishments after PurgeExpired failed: %v", err)
	}
	if len(punishments) != 2 {
		t.Fatalf("expected 2 punishments after purge (active + permanent), got %d", len(punishments))
	}
	for _, p := range punishments {
		if p.Subtype == 1 {
			t.Errorf("expired punishment (Subtype=1) was not purged")
		}
	}
}

func TestMarkIPKnownAndLoadKnownIPs(t *testing.T) {
teardown := setupTestDB(t)
defer teardown()

// Initially there should be no known IPs.
ipids, err := LoadKnownIPs()
if err != nil {
t.Fatalf("LoadKnownIPs (empty) failed: %v", err)
}
if len(ipids) != 0 {
t.Fatalf("expected 0 known IPs initially, got %d", len(ipids))
}

// Mark two IPs as known.
if err := MarkIPKnown("1.2.3.4"); err != nil {
t.Fatalf("MarkIPKnown failed: %v", err)
}
if err := MarkIPKnown("5.6.7.8"); err != nil {
t.Fatalf("MarkIPKnown failed: %v", err)
}

ipids, err = LoadKnownIPs()
if err != nil {
t.Fatalf("LoadKnownIPs failed: %v", err)
}
if len(ipids) != 2 {
t.Fatalf("expected 2 known IPs, got %d", len(ipids))
}
}

func TestMarkIPKnownIdempotent(t *testing.T) {
teardown := setupTestDB(t)
defer teardown()

// Calling MarkIPKnown multiple times for the same IPID must not error
// and must not create duplicate rows.
for i := 0; i < 5; i++ {
if err := MarkIPKnown("dup.ip"); err != nil {
t.Fatalf("MarkIPKnown attempt %d failed: %v", i, err)
}
}

ipids, err := LoadKnownIPs()
if err != nil {
t.Fatalf("LoadKnownIPs failed: %v", err)
}
if len(ipids) != 1 {
t.Fatalf("expected exactly 1 entry for dup.ip (upsert), got %d", len(ipids))
}
}

func TestMarkIPKnownUpdatesLastSeen(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "lastseen.ip"
	if err := MarkIPKnown(ipid); err != nil {
		t.Fatalf("MarkIPKnown (insert) failed: %v", err)
	}

	// Read FIRST_SEEN and LAST_SEEN after first insert.
	var firstSeen1, lastSeen1 int64
	row := db.QueryRow("SELECT FIRST_SEEN, LAST_SEEN FROM KNOWN_IPS WHERE IPID = ?", ipid)
	if err := row.Scan(&firstSeen1, &lastSeen1); err != nil {
		t.Fatalf("scan after insert failed: %v", err)
	}

	// Wait at least 1 second so the LAST_SEEN Unix timestamp increments.
	time.Sleep(1100 * time.Millisecond)

	if err := MarkIPKnown(ipid); err != nil {
		t.Fatalf("MarkIPKnown (update) failed: %v", err)
	}

	var firstSeen2, lastSeen2 int64
	row = db.QueryRow("SELECT FIRST_SEEN, LAST_SEEN FROM KNOWN_IPS WHERE IPID = ?", ipid)
	if err := row.Scan(&firstSeen2, &lastSeen2); err != nil {
		t.Fatalf("scan after update failed: %v", err)
	}

	if firstSeen2 != firstSeen1 {
		t.Errorf("FIRST_SEEN changed after second MarkIPKnown: was %d, now %d", firstSeen1, firstSeen2)
	}
	if lastSeen2 <= lastSeen1 {
		t.Errorf("LAST_SEEN not updated: was %d, still %d", lastSeen1, lastSeen2)
	}
}

func TestRemoveKnownIP(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	if err := MarkIPKnown("remove.me"); err != nil {
		t.Fatalf("MarkIPKnown failed: %v", err)
	}

	ipids, _ := LoadKnownIPs()
	if len(ipids) != 1 {
		t.Fatalf("expected 1 IP before removal, got %d", len(ipids))
	}

	if err := RemoveKnownIP("remove.me"); err != nil {
		t.Fatalf("RemoveKnownIP failed: %v", err)
	}

	ipids, _ = LoadKnownIPs()
	if len(ipids) != 0 {
		t.Fatalf("expected 0 IPs after removal, got %d", len(ipids))
	}
}

func TestRemoveKnownIPNonExistent(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// Removing an IP that does not exist must not return an error.
	if err := RemoveKnownIP("no.such.ip"); err != nil {
		t.Fatalf("RemoveKnownIP on non-existent IP returned error: %v", err)
	}
}

func TestRemoveKnownIPPreservesPlaytime(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "banned.with.playtime"
	if err := MarkIPKnown(ipid); err != nil {
		t.Fatalf("MarkIPKnown failed: %v", err)
	}
	if err := AddPlaytime(ipid, 7200); err != nil {
		t.Fatalf("AddPlaytime failed: %v", err)
	}

	// Banning forgets the known-IP status...
	if err := RemoveKnownIP(ipid); err != nil {
		t.Fatalf("RemoveKnownIP failed: %v", err)
	}
	ipids, _ := LoadKnownIPs()
	for _, ip := range ipids {
		if ip == ipid {
			t.Fatal("banned IP still returned by LoadKnownIPs")
		}
	}

	// ...but the playtime must survive the ban.
	playtime, err := GetPlaytime(ipid)
	if err != nil {
		t.Fatalf("GetPlaytime failed: %v", err)
	}
	if playtime != 7200 {
		t.Fatalf("expected playtime 7200 after ban, got %d", playtime)
	}
}

func TestMarkIPKnownAfterBanRestoresKnownStatus(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "returning.after.ban"
	if err := MarkIPKnown(ipid); err != nil {
		t.Fatalf("MarkIPKnown failed: %v", err)
	}
	if err := AddPlaytime(ipid, 9000); err != nil {
		t.Fatalf("AddPlaytime failed: %v", err)
	}
	if err := RemoveKnownIP(ipid); err != nil {
		t.Fatalf("RemoveKnownIP failed: %v", err)
	}

	// The player reconnects after the ban expires.
	if err := MarkIPKnown(ipid); err != nil {
		t.Fatalf("MarkIPKnown after ban failed: %v", err)
	}

	ipids, _ := LoadKnownIPs()
	found := false
	for _, ip := range ipids {
		if ip == ipid {
			found = true
		}
	}
	if !found {
		t.Fatal("reconnected IP not returned by LoadKnownIPs")
	}

	var firstSeen, lastSeen int64
	row := db.QueryRow("SELECT FIRST_SEEN, LAST_SEEN FROM KNOWN_IPS WHERE IPID = ?", ipid)
	if err := row.Scan(&firstSeen, &lastSeen); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if firstSeen == 0 || lastSeen == 0 {
		t.Fatalf("expected seen-timestamps re-stamped after reconnect, got FIRST_SEEN=%d LAST_SEEN=%d", firstSeen, lastSeen)
	}

	playtime, err := GetPlaytime(ipid)
	if err != nil {
		t.Fatalf("GetPlaytime failed: %v", err)
	}
	if playtime != 9000 {
		t.Fatalf("expected playtime 9000 after ban and reconnect, got %d", playtime)
	}
}

func TestPruneInactiveIPs(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// Insert two IPs with different LAST_SEEN values using raw SQL so we can
	// control the timestamps precisely.
	past := time.Now().Add(-48 * time.Hour).Unix()
	recent := time.Now().Unix()
	if _, err := db.Exec(
		"INSERT INTO KNOWN_IPS(IPID, FIRST_SEEN, LAST_SEEN) VALUES(?, ?, ?)",
		"old.ip", past, past); err != nil {
		t.Fatalf("insert old.ip failed: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO KNOWN_IPS(IPID, FIRST_SEEN, LAST_SEEN) VALUES(?, ?, ?)",
		"new.ip", recent, recent); err != nil {
		t.Fatalf("insert new.ip failed: %v", err)
	}

	// Prune IPs not seen in the last 24 hours.
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	n, err := PruneInactiveIPs(cutoff)
	if err != nil {
		t.Fatalf("PruneInactiveIPs failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row pruned, got %d", n)
	}

	ipids, _ := LoadKnownIPs()
	if len(ipids) != 1 || ipids[0] != "new.ip" {
		t.Errorf("expected only new.ip to remain, got %v", ipids)
	}
}

func TestPruneInactiveIPsSkipsZeroLastSeen(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// An IP with LAST_SEEN = 0 (legacy / just migrated) should not be pruned.
	if _, err := db.Exec(
		"INSERT INTO KNOWN_IPS(IPID, FIRST_SEEN, LAST_SEEN) VALUES(?, ?, ?)",
		"legacy.ip", 0, 0); err != nil {
		t.Fatalf("insert legacy.ip failed: %v", err)
	}

	cutoff := time.Now().Unix() // everything before now
	n, err := PruneInactiveIPs(cutoff)
	if err != nil {
		t.Fatalf("PruneInactiveIPs failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows pruned (LAST_SEEN=0 is protected), got %d", n)
	}

	// Verify with a direct query: a fully zeroed row is hidden from LoadKnownIPs
	// (that's the banned-with-playtime marker) but must still exist in the table.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM KNOWN_IPS WHERE IPID = ?", "legacy.ip").Scan(&count); err != nil {
		t.Fatalf("count legacy.ip failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected legacy.ip to remain, got count %d", count)
	}
}

func TestAddPlaytime(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "playtime.ip"
	if err := MarkIPKnown(ipid); err != nil {
		t.Fatalf("MarkIPKnown failed: %v", err)
	}

	// Add 100 seconds and verify.
	if err := AddPlaytime(ipid, 100); err != nil {
		t.Fatalf("AddPlaytime (100s) failed: %v", err)
	}
	var pt int64
	if err := db.QueryRow("SELECT PLAYTIME FROM KNOWN_IPS WHERE IPID = ?", ipid).Scan(&pt); err != nil {
		t.Fatalf("scan playtime after 100s failed: %v", err)
	}
	if pt != 100 {
		t.Errorf("expected PLAYTIME=100, got %d", pt)
	}

	// Add another 200 seconds; total should be 300.
	if err := AddPlaytime(ipid, 200); err != nil {
		t.Fatalf("AddPlaytime (200s) failed: %v", err)
	}
	if err := db.QueryRow("SELECT PLAYTIME FROM KNOWN_IPS WHERE IPID = ?", ipid).Scan(&pt); err != nil {
		t.Fatalf("scan playtime after 300s failed: %v", err)
	}
	if pt != 300 {
		t.Errorf("expected PLAYTIME=300, got %d", pt)
	}
}

func TestPruneShortPlaytimeIPs(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// Insert two IPs: one with >= 1h playtime, one without. Use non-zero seen
	// timestamps as MarkIPKnown would — zeroed timestamps mark a banned row,
	// which LoadKnownIPs deliberately skips.
	now := time.Now().Unix()
	if _, err := db.Exec("INSERT INTO KNOWN_IPS(IPID, FIRST_SEEN, LAST_SEEN, PLAYTIME) VALUES(?, ?, ?, ?)",
		"veteran.ip", now, now, 3600); err != nil {
		t.Fatalf("insert veteran.ip failed: %v", err)
	}
	if _, err := db.Exec("INSERT INTO KNOWN_IPS(IPID, FIRST_SEEN, LAST_SEEN, PLAYTIME) VALUES(?, ?, ?, ?)",
		"newbie.ip", now, now, 0); err != nil {
		t.Fatalf("insert newbie.ip failed: %v", err)
	}

	n, err := PruneShortPlaytimeIPs(3600)
	if err != nil {
		t.Fatalf("PruneShortPlaytimeIPs failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row pruned, got %d", n)
	}

	ipids, _ := LoadKnownIPs()
	if len(ipids) != 1 || ipids[0] != "veteran.ip" {
		t.Errorf("expected only veteran.ip to remain, got %v", ipids)
	}
}

func TestRandomCharCurse(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	ipid := "cursed.ip"

	if cursed, err := IsRandomCharCursed(ipid); err != nil {
		t.Fatalf("IsRandomCharCursed failed: %v", err)
	} else if cursed {
		t.Error("expected ipid to not be cursed before AddRandomCharCurse")
	}

	if err := AddRandomCharCurse(ipid, "TestAdmin", 12345); err != nil {
		t.Fatalf("AddRandomCharCurse failed: %v", err)
	}
	if cursed, err := IsRandomCharCursed(ipid); err != nil {
		t.Fatalf("IsRandomCharCursed failed: %v", err)
	} else if !cursed {
		t.Error("expected ipid to be cursed after AddRandomCharCurse")
	}

	// Re-cursing (upsert) must not error and must not create a duplicate row.
	if err := AddRandomCharCurse(ipid, "TestAdmin2", 67890); err != nil {
		t.Fatalf("re-AddRandomCharCurse failed: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM RANDOMCHAR_CURSES WHERE IPID = ?", ipid).Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row after re-cursing, got %d", count)
	}

	if err := RemoveRandomCharCurse(ipid); err != nil {
		t.Fatalf("RemoveRandomCharCurse failed: %v", err)
	}
	if cursed, err := IsRandomCharCursed(ipid); err != nil {
		t.Fatalf("IsRandomCharCursed failed: %v", err)
	} else if cursed {
		t.Error("expected ipid to not be cursed after RemoveRandomCharCurse")
	}

	// Removing again should report sql.ErrNoRows, not silently succeed.
	if err := RemoveRandomCharCurse(ipid); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows removing an already-removed curse, got %v", err)
	}
}
