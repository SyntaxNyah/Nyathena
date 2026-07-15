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
	"os"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
)

func TestLoadWordListFile_ParsesCommentsBlanksAndDedup(t *testing.T) {
	f, err := os.CreateTemp("", "athena-wordlist-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("# comment\n\nAdmin\nADMIN\n  Moderator  \n# another comment\nServer\n")
	f.Close()

	words, err := loadWordListFile(f.Name())
	if err != nil {
		t.Fatalf("loadWordListFile returned error: %v", err)
	}

	want := map[string]bool{"admin": false, "moderator": false, "server": false}
	if len(words) != len(want) {
		t.Fatalf("expected %d entries, got %d: %v", len(want), len(words), words)
	}
	for _, w := range words {
		if _, ok := want[w]; !ok {
			t.Errorf("unexpected entry %q", w)
		}
		want[w] = true
	}
	for w, seen := range want {
		if !seen {
			t.Errorf("expected entry %q was not loaded", w)
		}
	}
}

func TestMatchCensoredName(t *testing.T) {
	orig := getCensoredNames()
	t.Cleanup(func() { setCensoredNames(orig) })
	setCensoredNames([]string{"admin", "moderator"})

	if _, ok := matchCensoredName("fake_admin_99"); !ok {
		t.Error("expected substring match against 'admin' to fire")
	}
	if _, ok := matchCensoredName("phoenix wright"); ok {
		t.Error("expected no match for an unrelated showname")
	}
}

// Regression test: same class of bug as TestMatchBannedWordIgnoresEmptyEntry
// (text_filter_normalize_test.go) — an empty entry must never match, since
// strings.Contains treats "" as a substring of every showname.
func TestMatchCensoredName_IgnoresEmptyEntry(t *testing.T) {
	orig := getCensoredNames()
	t.Cleanup(func() { setCensoredNames(orig) })
	setCensoredNames([]string{"", "admin"})

	if matched, ok := matchCensoredName("phoenix wright"); ok {
		t.Errorf("matchCensoredName(%q) unexpectedly matched empty entry (matched=%q)", "phoenix wright", matched)
	}
	if _, ok := matchCensoredName("admin"); !ok {
		t.Error("matchCensoredName failed to catch the real entry once an empty entry was also present")
	}
}

// A fullwidth-Unicode rendering of "admin" (e.g. typed by a user trying to
// dodge censored_names.txt) must still match once normalizeForFilter'd,
// exactly like the plain-ASCII form.
func TestMatchCensoredName_UnicodeBypass(t *testing.T) {
	orig := getCensoredNames()
	t.Cleanup(func() { setCensoredNames(orig) })
	setCensoredNames([]string{normalizeForFilter("admin")})

	const fullwidthAdmin = "ａｄｍｉｎ" // fullwidth "admin"
	if _, ok := matchCensoredName(normalizeForFilter(fullwidthAdmin)); !ok {
		t.Errorf("expected fullwidth-Unicode %q to still match 'admin'", fullwidthAdmin)
	}
}

func setupShownameCensorTestDB(t *testing.T) func() {
	t.Helper()
	tmp, err := os.CreateTemp("", "athena-showname-censor-*.db")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmp.Close()
	db.DBPath = tmp.Name()
	if err := db.Open(); err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return func() {
		db.Close()
		os.Remove(tmp.Name())
	}
}

// A showname matching censored_names.txt shadow-mutes the speaker
// (PunishmentStealthMute) and adds their IPID to the lag/torment list, exactly
// as /stealthmute + /lag would.
func TestCheckCensoredShowname_MatchAppliesStealthMuteAndLag(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	origNames := getCensoredNames()
	t.Cleanup(func() { setCensoredNames(origNames) })
	setCensoredNames([]string{"admin"})

	client := &Client{conn: &testConn{}, uid: 42, ipid: "ip-censor-test"}

	if got := checkCensoredShowname(client, "Fake_Admin_99"); !got {
		t.Fatal("expected checkCensoredShowname to report a match")
	}
	if !client.HasActivePunishment(PunishmentStealthMute) {
		t.Error("expected client to carry an active PunishmentStealthMute")
	}
	if !isIPIDTormented(client.Ipid()) {
		t.Error("expected client's IPID to be added to the torment/lag list")
	}

	// addTormentedIP persists to the DB via a fire-and-forget goroutine; give
	// it a moment to finish before the deferred DB teardown runs, and clean up
	// the in-memory torment entry the same way (not via removeTormentedIP,
	// which would spawn another unawaited goroutine racing the DB close).
	time.Sleep(20 * time.Millisecond)
	tormentedIPIDs.mu.Lock()
	delete(tormentedIPIDs.set, client.Ipid())
	tormentedIPIDs.mu.Unlock()
}

// A showname that doesn't match any entry is left alone entirely.
func TestCheckCensoredShowname_NoMatchIsNoOp(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	origNames := getCensoredNames()
	t.Cleanup(func() { setCensoredNames(origNames) })
	setCensoredNames([]string{"admin"})

	client := &Client{conn: &testConn{}, uid: 43, ipid: "ip-censor-clean"}

	if got := checkCensoredShowname(client, "Phoenix Wright"); got {
		t.Fatal("expected checkCensoredShowname to report no match")
	}
	if client.HasActivePunishment(PunishmentStealthMute) {
		t.Error("expected no punishment to be applied")
	}
	if isIPIDTormented(client.Ipid()) {
		t.Error("expected client's IPID not to be tormented")
	}
}

// With no censored_names.txt entries loaded, the check is a cheap no-op.
func TestCheckCensoredShowname_EmptyListIsNoOp(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	origNames := getCensoredNames()
	t.Cleanup(func() { setCensoredNames(origNames) })
	setCensoredNames(nil)

	client := &Client{conn: &testConn{}, uid: 44, ipid: "ip-censor-empty"}
	if got := checkCensoredShowname(client, "Admin"); got {
		t.Fatal("expected no match when the censored name list is empty")
	}
}
