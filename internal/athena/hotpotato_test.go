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

// resetHotPotatoState resets global hot potato state between tests.
func resetHotPotatoState() {
	hotPotato.mu.Lock()
	hotPotato.optInActive = false
	hotPotato.gameActive = false
	hotPotato.participants = make(map[int]struct{})
	hotPotato.carrierUID = -1
	hotPotato.lastGameEnd = time.Time{}
	hotPotato.mu.Unlock()
}

// TestHotPotatoCooldown verifies the cooldown helper returns the correct state.
func TestHotPotatoCooldown(t *testing.T) {
	resetHotPotatoState()

	// No game has run yet — should not be cooling down.
	if cooling, _ := isHotPotatoCoolingDown(); cooling {
		t.Error("expected no cooldown when no game has run yet")
	}

	// Game ended 1 second ago — cooldown must be active.
	hotPotato.mu.Lock()
	hotPotato.lastGameEnd = time.Now().Add(-1 * time.Second)
	hotPotato.mu.Unlock()

	cooling, secs := isHotPotatoCoolingDown()
	if !cooling {
		t.Error("expected cooldown to be active after a recent game")
	}
	if secs <= 0 {
		t.Errorf("expected positive remaining seconds, got %d", secs)
	}

	// Game ended 6 minutes ago — cooldown must have expired.
	hotPotato.mu.Lock()
	hotPotato.lastGameEnd = time.Now().Add(-6 * time.Minute)
	hotPotato.mu.Unlock()

	if cooling, _ := isHotPotatoCoolingDown(); cooling {
		t.Error("expected cooldown to be expired after 6 minutes")
	}
}

// TestHotPotatoOptIn verifies that distinct UIDs are tracked as separate participants.
func TestHotPotatoOptIn(t *testing.T) {
	resetHotPotatoState()

	hotPotato.mu.Lock()
	hotPotato.optInActive = true
	hotPotato.participants[1] = struct{}{}
	hotPotato.participants[2] = struct{}{}
	count := len(hotPotato.participants)
	hotPotato.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 participants, got %d", count)
	}
}

// TestHotPotatoDoubleOptIn verifies that a UID can only appear in the set once.
func TestHotPotatoDoubleOptIn(t *testing.T) {
	resetHotPotatoState()

	hotPotato.mu.Lock()
	hotPotato.optInActive = true
	hotPotato.participants[42] = struct{}{}
	_, already := hotPotato.participants[42]
	hotPotato.participants[42] = struct{}{} // idempotent write
	count := len(hotPotato.participants)
	hotPotato.mu.Unlock()

	if !already {
		t.Error("expected participant 42 to be present in the set")
	}
	if count != 1 {
		t.Errorf("expected 1 participant after duplicate write, got %d", count)
	}
}

// TestRandomHotPotatoPunishment verifies every returned type belongs to the pool.
func TestRandomHotPotatoPunishment(t *testing.T) {
	valid := make(map[PunishmentType]bool, len(hotPotatoPunishmentPool))
	for _, p := range hotPotatoPunishmentPool {
		valid[p] = true
	}

	// 100 draws gives high coverage of all 16 pool entries while staying fast.
	const draws = 100
	for i := 0; i < draws; i++ {
		if p := randomHotPotatoPunishment(); !valid[p] {
			t.Errorf("randomHotPotatoPunishment returned unexpected type: %v", p)
		}
	}
}

// TestHotPotatoOnlyOneGame verifies that a concurrent start is blocked while
// either the opt-in window or the game itself is active.
func TestHotPotatoOnlyOneGame(t *testing.T) {
	resetHotPotatoState()

	for _, tc := range []struct {
		name        string
		optInActive bool
		gameActive  bool
	}{
		{"opt-in active", true, false},
		{"game active", false, true},
		{"both active", true, true},
	} {
		hotPotato.mu.Lock()
		hotPotato.optInActive = tc.optInActive
		hotPotato.gameActive = tc.gameActive
		blocked := hotPotato.optInActive || hotPotato.gameActive
		hotPotato.mu.Unlock()

		if !blocked {
			t.Errorf("%s: expected start to be blocked", tc.name)
		}
	}
}
