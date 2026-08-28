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

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
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

	if got, kick := autoModCheck(client, "totally clean message", "IC message"); got != autoModPass || kick {
		t.Fatalf("clean message: expected (autoModPass, false), got (%v, %v)", got, kick)
	}
	if isIPIDTormented(client.Ipid()) {
		t.Fatal("clean message must not torment the speaker")
	}

	if got, kick := autoModCheck(client, "well zqvexo to you too", "IC message"); got != autoModShadow || !kick {
		t.Fatalf("banned word: expected (autoModShadow, true), got (%v, %v)", got, kick)
	}
	if !isIPIDTormented(client.Ipid()) {
		t.Error("expected the censor trip to add the speaker's IPID to the torment list")
	}

	// A second trip while already tormented still shadows, without error.
	if got, kick := autoModCheck(client, "zqvexo again", "OOC message"); got != autoModShadow || !kick {
		t.Fatalf("repeat trip: expected (autoModShadow, true), got (%v, %v)", got, kick)
	}

	// Let the fire-and-forget DB persist goroutine finish before teardown,
	// then clean the in-memory entry directly (see showname_censor_test.go).
	time.Sleep(20 * time.Millisecond)
	tormentedIPIDs.mu.Lock()
	delete(tormentedIPIDs.set, client.Ipid())
	tormentedIPIDs.mu.Unlock()
}

// The word filter itself is independent of automod_enabled — only the
// full punitive action set (kick/mute/ban) is gated behind the toggle, and
// even then it defaults to the safe shadow-drop action (autoModActionShadow
// is the zero value) rather than doing nothing. This covers OOC messages, so
// a server that never turned on automod_enabled still can't be used to blast
// slurs into OOC once a wordlist is loaded.
func TestAutoModCheckFiresWithAutomodDisabled(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	origConfig := config
	origAction := autoModAction
	origWords := getBannedWords()
	t.Cleanup(func() {
		config = origConfig
		autoModAction = origAction
		setBannedWords(origWords)
	})
	config = &settings.Config{ServerConfig: settings.ServerConfig{AutoModEnabled: false}}
	autoModAction = autoModActionShadow
	setBannedWords([]string{"zqvexo"})

	client := &Client{conn: &testConn{}, uid: 78, ipid: "ip-disabled-automod-test"}

	if got, kick := autoModCheck(client, "totally clean message", "OOC message"); got != autoModPass || kick {
		t.Fatalf("clean message: expected (autoModPass, false), got (%v, %v)", got, kick)
	}

	if got, kick := autoModCheck(client, "well zqvexo to you too", "OOC message"); got != autoModShadow || !kick {
		t.Fatalf("banned word with automod disabled: expected (autoModShadow, true), got (%v, %v)", got, kick)
	}
	if !isIPIDTormented(client.Ipid()) {
		t.Error("expected the censor trip to add the speaker's IPID to the torment list even with automod disabled")
	}

	time.Sleep(20 * time.Millisecond)
	tormentedIPIDs.mu.Lock()
	delete(tormentedIPIDs.set, client.Ipid())
	tormentedIPIDs.mu.Unlock()
}

// The torment automod action leaves the connection open with no immediate
// consequence on its own, so autoModCheck must ask the caller to follow up
// with an escalating kick (kickAfter=true) -- this is the fix for a censor
// trip otherwise letting a determined troll keep hammering slurs forever.
func TestAutoModCheckTormentActionRequestsKick(t *testing.T) {
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
	autoModAction = autoModActionTorment
	setBannedWords([]string{"zqvexo"})

	client := &Client{conn: &testConn{}, uid: 79, ipid: "ip-torment-kick-test"}

	if got, kick := autoModCheck(client, "well zqvexo to you too", "IC message"); got != autoModBlocked || !kick {
		t.Fatalf("torment action: expected (autoModBlocked, true), got (%v, %v)", got, kick)
	}
	if !isIPIDTormented(client.Ipid()) {
		t.Error("expected the censor trip to add the speaker's IPID to the torment list")
	}

	time.Sleep(20 * time.Millisecond)
	tormentedIPIDs.mu.Lock()
	delete(tormentedIPIDs.set, client.Ipid())
	tormentedIPIDs.mu.Unlock()
}

// The kick/mute/ban automod actions already give the offender an immediate,
// severe consequence on their own (an actual kick, a permanent mute, or a
// permanent ban), so autoModCheck must never ask for an additional escalating
// kick on top of them (kickAfter=false) -- that would be redundant at best.
func TestAutoModCheckKickMuteBanActionsDoNotRequestKick(t *testing.T) {
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
	setBannedWords([]string{"zqvexo"})

	autoModAction = autoModActionKick
	kickClient := &Client{conn: &testConn{}, uid: 80, ipid: "ip-kick-action-test"}
	if got, kick := autoModCheck(kickClient, "zqvexo", "IC message"); got != autoModBlocked || kick {
		t.Errorf("kick action: expected (autoModBlocked, false), got (%v, %v)", got, kick)
	}

	autoModAction = autoModActionMute
	muteClient := &Client{conn: &testConn{}, uid: 81, ipid: "ip-mute-action-test"}
	if got, kick := autoModCheck(muteClient, "zqvexo", "IC message"); got != autoModBlocked || kick {
		t.Errorf("mute action: expected (autoModBlocked, false), got (%v, %v)", got, kick)
	}

	autoModAction = autoModActionBan
	banClient := &Client{conn: &testConn{}, uid: 82, ipid: "ip-ban-action-test"}
	if got, kick := autoModCheck(banClient, "zqvexo", "IC message"); got != autoModBlocked || kick {
		t.Errorf("ban action: expected (autoModBlocked, false), got (%v, %v)", got, kick)
	}
}

// A rate-limit kick and a censor-trip kick on the same IPID must count toward the
// SAME repeat-offender counter -- not two separate, easier-to-dodge counters -- so an
// IPID that racks up one of each within a spree is banned exactly like one that
// trips the same check twice. Uses a flat (playtime-tier-disabled) threshold so the
// ban decision never needs a live playtime lookup.
func TestKickForCensorTripSharesCounterWithRateLimit(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	oldConfig := config
	defer func() { config = oldConfig }()
	config = &settings.Config{}
	config.RateLimitKickAutoban = true
	config.RateLimitKickAutobanThreshold = 2
	config.RateLimitKickAutobanWindow = 600
	config.RateLimitKickAutobanDuration = "15m"
	config.RateLimitKickAutobanMinPlaytime = 0
	config.RateLimitKickAutobanLenientPlaytime = 0
	config.BanLen = "3d"

	resetRateLimitKickTracker()

	ipid := "testSharedCounterIP"

	// First violation: a plain rate-limit kick (spree count reaches 1, below the
	// threshold of 2) -- must not ban yet.
	rateLimitClient := &Client{conn: &testConn{}, uid: 90, ipid: ipid, perms: permissions.PermissionField["NONE"]}
	rateLimitClient.KickForRateLimit()

	if banned, _, err := db.IsBanned(db.IPID, ipid); err != nil {
		t.Fatalf("IsBanned: %v", err)
	} else if banned {
		t.Fatal("a single rate-limit kick must not trigger the autoban (threshold is 2)")
	}

	// Second violation, of a DIFFERENT kind: a censor-trip kick on a new connection
	// under the same IPID (simulating a reconnect). This is the 2nd kick in the same
	// spree, reaching the threshold -- proving the counter is shared across violation
	// kinds, not tracked separately per kind.
	censorClient := &Client{conn: &testConn{}, uid: 91, ipid: ipid, perms: permissions.PermissionField["NONE"]}
	censorClient.KickForCensorTrip()

	time.Sleep(20 * time.Millisecond) // let the fire-and-forget account-link alert settle
	banned, _, err := db.IsBanned(db.IPID, ipid)
	if err != nil {
		t.Fatalf("IsBanned: %v", err)
	}
	if !banned {
		t.Error("expected the IPID to be auto-banned once a rate-limit kick and a censor-trip kick together reached the shared threshold")
	}
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
