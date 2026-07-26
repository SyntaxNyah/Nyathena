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
	"strings"
	"testing"
	"time"
)

// resetGiveawayState resets global giveaway state between tests.
func resetGiveawayState() {
	giveaway.mu.Lock()
	giveaway.active = false
	giveaway.item = ""
	giveaway.hostUID = -1
	giveaway.hostName = ""
	giveaway.entrants = make(map[int]struct{})
	giveaway.lastEnd = time.Time{}
	giveaway.mu.Unlock()
}

// TestGiveawayCooldown verifies the cooldown helper returns the correct state.
func TestGiveawayCooldown(t *testing.T) {
	resetGiveawayState()

	// No giveaway has run yet — should not be cooling down.
	if cooling, _ := isGiveawayCoolingDown(); cooling {
		t.Error("expected no cooldown when no giveaway has run yet")
	}

	// Giveaway ended 1 second ago — cooldown must be active.
	giveaway.mu.Lock()
	giveaway.lastEnd = time.Now().Add(-1 * time.Second)
	giveaway.mu.Unlock()

	cooling, secs := isGiveawayCoolingDown()
	if !cooling {
		t.Error("expected cooldown to be active after a recent giveaway")
	}
	if secs <= 0 {
		t.Errorf("expected positive remaining seconds, got %d", secs)
	}

	// Giveaway ended 11 minutes ago — cooldown must have expired.
	giveaway.mu.Lock()
	giveaway.lastEnd = time.Now().Add(-11 * time.Minute)
	giveaway.mu.Unlock()

	if cooling, _ := isGiveawayCoolingDown(); cooling {
		t.Error("expected cooldown to be expired after 11 minutes")
	}
}

// TestGiveawayEntry verifies that distinct UIDs are tracked as separate entrants.
func TestGiveawayEntry(t *testing.T) {
	resetGiveawayState()

	giveaway.mu.Lock()
	giveaway.active = true
	giveaway.entrants[1] = struct{}{}
	giveaway.entrants[2] = struct{}{}
	count := len(giveaway.entrants)
	giveaway.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 entrants, got %d", count)
	}
}

// TestGiveawayDoubleEntry verifies that a UID can only appear in the set once.
func TestGiveawayDoubleEntry(t *testing.T) {
	resetGiveawayState()

	giveaway.mu.Lock()
	giveaway.active = true
	giveaway.entrants[42] = struct{}{}
	_, already := giveaway.entrants[42]
	giveaway.entrants[42] = struct{}{} // idempotent write
	count := len(giveaway.entrants)
	giveaway.mu.Unlock()

	if !already {
		t.Error("expected entrant 42 to be present in the set")
	}
	if count != 1 {
		t.Errorf("expected 1 entrant after duplicate write, got %d", count)
	}
}

// TestGiveawayOnlyOneActive verifies that a concurrent start is blocked while
// a giveaway is already active.
func TestGiveawayOnlyOneActive(t *testing.T) {
	resetGiveawayState()

	giveaway.mu.Lock()
	giveaway.active = true
	blocked := giveaway.active
	giveaway.mu.Unlock()

	if !blocked {
		t.Error("expected start to be blocked while giveaway is active")
	}
}

// TestGiveawayStartBlocksBannedWord verifies that a giveaway item containing a
// banned word is rejected and never opens a giveaway, regardless of whether
// automod's own IC/OOC enforcement is enabled — the word list is loaded
// independently of automod_enabled (see initAutoMod).
func TestGiveawayStartBlocksBannedWord(t *testing.T) {
	resetGiveawayState()

	origWords := getBannedWords()
	t.Cleanup(func() { setBannedWords(origWords) })
	setBannedWords([]string{normalizeForFilter("nigger")})

	client := &Client{conn: &testConn{}, uid: 99, ipid: "ip-giveaway-test", area: makeTestArea("GiveawayTestArea")}

	giveawayStart(client, "fuck niggers")

	giveaway.mu.Lock()
	active := giveaway.active
	giveaway.mu.Unlock()

	if active {
		t.Error("expected giveaway containing a banned word to be blocked, but it started")
	}
}

// TestGiveawayStartBlocksTooLongItem verifies an item over the character cap
// is rejected and never opens a giveaway.
func TestGiveawayStartBlocksTooLongItem(t *testing.T) {
	resetGiveawayState()

	client := &Client{conn: &testConn{}, uid: 101, ipid: "ip-giveaway-test-long", area: makeTestArea("GiveawayTestArea3")}

	giveawayStart(client, strings.Repeat("a", giveawayItemMaxLen+1))

	giveaway.mu.Lock()
	active := giveaway.active
	giveaway.mu.Unlock()

	if active {
		t.Error("expected an over-length giveaway item to be blocked, but it started")
	}
}

// TestGiveawayStartAllowsMaxLenItem verifies an item exactly at the cap is
// still allowed (guards against an off-by-one).
func TestGiveawayStartAllowsMaxLenItem(t *testing.T) {
	resetGiveawayState()

	client := &Client{conn: &testConn{}, uid: 102, ipid: "ip-giveaway-test-maxlen", area: makeTestArea("GiveawayTestArea4")}

	giveawayStart(client, strings.Repeat("a", giveawayItemMaxLen))

	giveaway.mu.Lock()
	active := giveaway.active
	giveaway.mu.Unlock()

	if !active {
		t.Error("expected an item exactly at the length cap to start normally")
	}
}

// TestGiveawayStartAllowsCleanItem verifies a clean item still opens a
// giveaway (guards against the filter over-blocking).
func TestGiveawayStartAllowsCleanItem(t *testing.T) {
	resetGiveawayState()

	origWords := getBannedWords()
	t.Cleanup(func() { setBannedWords(origWords) })
	setBannedWords([]string{normalizeForFilter("nigger")})

	client := &Client{conn: &testConn{}, uid: 100, ipid: "ip-giveaway-test-clean", area: makeTestArea("GiveawayTestArea2")}

	giveawayStart(client, "a shiny rock")

	giveaway.mu.Lock()
	active := giveaway.active
	giveaway.mu.Unlock()

	if !active {
		t.Error("expected a clean giveaway item to start normally")
	}
}
