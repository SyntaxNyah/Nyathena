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
	"time"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// The shadow automod action reports autoModShadow (so the caller echoes the
// message to the sender only) and puts the speaker's IPID on the torment
// list. It must never report autoModBlocked — blocked would drop the sender's
// own echo and give the censor away.
func TestAutoModCheckShadowAction(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	origConfig := config
	origAction := autoModAction
	origWords := getBannedWords()
	t.Cleanup(func() {
		config = origConfig
		autoModAction = origAction
		setBannedWords(origWords)
	})
	config = &settings.Config{ServerConfig: settings.ServerConfig{AutoModEnabled: true}}
	autoModAction = autoModActionShadow
	setBannedWords([]string{"zqvexo"})

	client := &Client{conn: &testConn{}, uid: 77, ipid: "ip-shadow-test"}

	if got := autoModCheck(client, "totally clean message", "IC message"); got != autoModPass {
		t.Fatalf("clean message: expected autoModPass, got %v", got)
	}
	if isIPIDTormented(client.Ipid()) {
		t.Fatal("clean message must not torment the speaker")
	}

	if got := autoModCheck(client, "well zqvexo to you too", "IC message"); got != autoModShadow {
		t.Fatalf("banned word: expected autoModShadow, got %v", got)
	}
	if !isIPIDTormented(client.Ipid()) {
		t.Error("expected the censor trip to add the speaker's IPID to the torment list")
	}

	// A second trip while already tormented still shadows, without error.
	if got := autoModCheck(client, "zqvexo again", "OOC message"); got != autoModShadow {
		t.Fatalf("repeat trip: expected autoModShadow, got %v", got)
	}

	// Let the fire-and-forget DB persist goroutine finish before teardown,
	// then clean the in-memory entry directly (see showname_censor_test.go).
	time.Sleep(20 * time.Millisecond)
	tormentedIPIDs.mu.Lock()
	delete(tormentedIPIDs.set, client.Ipid())
	tormentedIPIDs.mu.Unlock()
}

// /untorment all: clearAllTormentedIPs wipes the whole in-memory set and
// reports how many entries it removed.
func TestClearAllTormentedIPs(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	tormentedIPIDs.mu.Lock()
	tormentedIPIDs.set["ip-purge-a"] = struct{}{}
	tormentedIPIDs.set["ip-purge-b"] = struct{}{}
	tormentedIPIDs.set["ip-purge-c"] = struct{}{}
	before := len(tormentedIPIDs.set)
	tormentedIPIDs.mu.Unlock()

	if n := clearAllTormentedIPs(); n != before {
		t.Errorf("expected clearAllTormentedIPs to report %d removed, got %d", before, n)
	}
	if got := snapshotTormentedIPs(); len(got) != 0 {
		t.Errorf("expected an empty torment list after purge, got %v", got)
	}
	if n := clearAllTormentedIPs(); n != 0 {
		t.Errorf("expected a second purge to remove 0, got %d", n)
	}

	// Let the fire-and-forget DB clear goroutines finish before the deferred
	// DB teardown closes the database out from under them.
	time.Sleep(20 * time.Millisecond)
}
