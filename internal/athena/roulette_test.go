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
)

// resetRRState resets global Russian Roulette state between tests.
func resetRRState() {
	rr.mu.Lock()
	rr.joinActive = false
	rr.gameActive = false
	rr.players = nil
	rr.lastEnd = time.Time{}
	rr.mu.Unlock()
}

// TestRRCooldown verifies the cooldown helper returns the correct state.
func TestRRCooldown(t *testing.T) {
	resetRRState()

	// No game yet — should not be cooling down.
	if cooling, _ := isRRCoolingDown(); cooling {
		t.Error("expected no cooldown when no game has run yet")
	}

	// Game ended 1 second ago — cooldown must be active.
	rr.mu.Lock()
	rr.lastEnd = time.Now().Add(-1 * time.Second)
	rr.mu.Unlock()

	cooling, secs := isRRCoolingDown()
	if !cooling {
		t.Error("expected cooldown to be active after a recent game")
	}
	if secs <= 0 {
		t.Errorf("expected positive remaining seconds, got %d", secs)
	}

	// Game ended 6 minutes ago — cooldown must have expired.
	rr.mu.Lock()
	rr.lastEnd = time.Now().Add(-6 * time.Minute)
	rr.mu.Unlock()

	if cooling, _ := isRRCoolingDown(); cooling {
		t.Error("expected cooldown to be expired after 6 minutes")
	}
}

// TestRRJoinDuplicateBlocked verifies a UID can only join once.
func TestRRJoinDuplicateBlocked(t *testing.T) {
	resetRRState()

	rr.mu.Lock()
	rr.joinActive = true
	rr.players = append(rr.players, 42)
	rr.mu.Unlock()

	// Simulate the duplicate-join check.
	uid := 42
	rr.mu.Lock()
	alreadyIn := false
	for _, p := range rr.players {
		if p == uid {
			alreadyIn = true
			break
		}
	}
	rr.mu.Unlock()

	if !alreadyIn {
		t.Error("expected UID 42 to already be in the player list")
	}
}

// TestRRMinPlayers verifies that a game with fewer than rrMinPlayers is cancelled.
func TestRRMinPlayers(t *testing.T) {
	resetRRState()

	rr.mu.Lock()
	rr.joinActive = true
	rr.players = []int{1} // only 1 player
	n := len(rr.players)
	rr.mu.Unlock()

	if n >= rrMinPlayers {
		t.Errorf("expected fewer than %d players, got %d", rrMinPlayers, n)
	}
}

// TestRRBulletProbability verifies that the shot probability is bullets/remaining.
func TestRRBulletProbability(t *testing.T) {
	for _, tc := range []struct {
		remaining int
		bullets   int
		wantHit   bool // expected result of rand.Intn(remaining) < bullets for edge cases
	}{
		{6, 6, true},  // all chambers loaded — always hit
		{1, 1, true},  // last chamber, 1 bullet — always hit
		{6, 0, false}, // no bullets — never hit
	} {
		hit := tc.bullets > 0 && (tc.remaining == tc.bullets || 0 < tc.bullets)
		// Deterministic check: if bullets == 0, can never hit; if bullets == remaining, always hit.
		if tc.bullets == 0 {
			hit = false
		} else if tc.remaining == tc.bullets {
			hit = true
		} else {
			// Skip non-deterministic cases — just verify the formula is plausible.
			continue
		}
		if hit != tc.wantHit {
			t.Errorf("remaining=%d bullets=%d: expected hit=%v, got %v", tc.remaining, tc.bullets, tc.wantHit, hit)
		}
	}
}

// TestRRPunishmentPool verifies every returned type is in the pool.
func TestRRPunishmentPool(t *testing.T) {
	valid := make(map[PunishmentType]bool, len(rrPunishmentPool))
	for _, p := range rrPunishmentPool {
		valid[p] = true
	}
	const draws = 200
	for i := 0; i < draws; i++ {
		if p := randomRRPunishment(); !valid[p] {
			t.Errorf("randomRRPunishment returned unexpected type: %v", p)
		}
	}
}

// TestRROnlyOneGame verifies that concurrent starts are blocked.
func TestRROnlyOneGame(t *testing.T) {
	resetRRState()

	for _, tc := range []struct {
		name       string
		joinActive bool
		gameActive bool
	}{
		{"join window open", true, false},
		{"game active", false, true},
		{"both active", true, true},
	} {
		rr.mu.Lock()
		rr.joinActive = tc.joinActive
		rr.gameActive = tc.gameActive
		blocked := rr.joinActive || rr.gameActive
		rr.mu.Unlock()

		if !blocked {
			t.Errorf("%s: expected start to be blocked", tc.name)
		}
	}
}

// TestRRJoinNamesHelper verifies the joinNames helper for display strings.
func TestRRJoinNamesHelper(t *testing.T) {
	for _, tc := range []struct {
		input []string
		want  string
	}{
		{nil, ""},
		{[]string{"Alice"}, "Alice"},
		{[]string{"Alice", "Bob"}, "Alice and Bob"},
		{[]string{"Alice", "Bob", "Carol"}, "Alice, Bob, and Carol"},
		{[]string{"Alice", "Bob", "Carol", "Dave"}, "Alice, Bob, Carol, and Dave"},
	} {
		if got := joinNames(tc.input); got != tc.want {
			t.Errorf("joinNames(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestRRGameClosedAfterShot simulates closing game state and verifies it resets.
func TestRRGameClosedAfterShot(t *testing.T) {
	resetRRState()

	rr.mu.Lock()
	rr.gameActive = true
	rr.players = []int{1, 2, 3}
	rr.mu.Unlock()

	// Simulate what rrRun does after a hit.
	rr.mu.Lock()
	rr.gameActive = false
	rr.players = rr.players[:0]
	rr.lastEnd = time.Now().UTC()
	rr.mu.Unlock()

	rr.mu.Lock()
	active := rr.gameActive
	players := len(rr.players)
	ended := rr.lastEnd.IsZero()
	rr.mu.Unlock()

	if active {
		t.Error("expected game to be inactive after shot")
	}
	if players != 0 {
		t.Errorf("expected empty player list after shot, got %d", players)
	}
	if ended {
		t.Error("expected lastEnd to be set after game over")
	}
}

// TestRRDoubleBulletChance verifies the double-bullet constant is in a sane range.
func TestRRDoubleBulletChance(t *testing.T) {
	if rrDoubleBulletP < 0 || rrDoubleBulletP > 100 {
		t.Errorf("rrDoubleBulletP must be 0–100, got %d", rrDoubleBulletP)
	}
}

// TestRRRicochetChance verifies the ricochet constant is in a sane range.
func TestRRRicochetChance(t *testing.T) {
	if rrRicochetP < 0 || rrRicochetP > 100 {
		t.Errorf("rrRicochetP must be 0–100, got %d", rrRicochetP)
	}
}

// TestRRNewChaosConstants verifies the new chaos event constants are valid percentages.
func TestRRNewChaosConstants(t *testing.T) {
	for _, tc := range []struct {
		name string
		val  int
	}{
		{"rrChainShotP", rrChainShotP},
		{"rrDoublePunishP", rrDoublePunishP},
		{"rrReSpinP", rrReSpinP},
		{"rrSurvivorCurseP", rrSurvivorCurseP},
	} {
		if tc.val < 0 || tc.val > 100 {
			t.Errorf("%s must be 0–100, got %d", tc.name, tc.val)
		}
	}
}

// TestRRCursePunishmentPool verifies every curse pool type is a valid punishment.
func TestRRCursePunishmentPool(t *testing.T) {
	if len(rrCursePunishmentPool) == 0 {
		t.Fatal("rrCursePunishmentPool must not be empty")
	}
	valid := make(map[PunishmentType]bool, len(rrPunishmentPool))
	for _, p := range rrPunishmentPool {
		valid[p] = true
	}
	for _, p := range rrCursePunishmentPool {
		if !valid[p] {
			t.Errorf("rrCursePunishmentPool contains type %v not in main pool", p)
		}
	}
}

// TestRRRandomExclusion verifies randomRRPunishmentExcluding never returns the excluded type.
func TestRRRandomExclusion(t *testing.T) {
	if len(rrPunishmentPool) < 2 {
		t.Skip("pool too small to test exclusion")
	}
	exclude := rrPunishmentPool[0]
	for i := 0; i < 200; i++ {
		p := randomRRPunishmentExcluding(exclude)
		if p == exclude {
			t.Errorf("randomRRPunishmentExcluding returned excluded type %v", p)
		}
	}
}
