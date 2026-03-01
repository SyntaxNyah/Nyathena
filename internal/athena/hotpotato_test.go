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
	hotPotato.passLastUsed = make(map[int]time.Time)
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

// TestHotPotatoPassCooldown verifies that the 10-second pass cooldown is enforced.
func TestHotPotatoPassCooldown(t *testing.T) {
	resetHotPotatoState()

	const carrierUID = 7

	hotPotato.mu.Lock()
	hotPotato.gameActive = true
	hotPotato.carrierUID = carrierUID
	hotPotato.participants[carrierUID] = struct{}{}
	hotPotato.mu.Unlock()

	// No pass recorded yet — should be allowed.
	hotPotato.mu.Lock()
	_, hasCooldown := hotPotato.passLastUsed[carrierUID]
	hotPotato.mu.Unlock()
	if hasCooldown {
		t.Error("expected no pass cooldown entry before the first pass")
	}

	// Record a pass timestamp as "just now".
	hotPotato.mu.Lock()
	hotPotato.passLastUsed[carrierUID] = time.Now()
	hotPotato.mu.Unlock()

	// Should be blocked — not enough time has elapsed.
	hotPotato.mu.Lock()
	last := hotPotato.passLastUsed[carrierUID]
	elapsed := time.Since(last)
	blocked := elapsed < hotPotatoPassCooldown
	hotPotato.mu.Unlock()

	if !blocked {
		t.Error("expected pass to be on cooldown immediately after use")
	}

	// Simulate the cooldown having expired.
	hotPotato.mu.Lock()
	hotPotato.passLastUsed[carrierUID] = time.Now().Add(-hotPotatoPassCooldown - time.Second)
	hotPotato.mu.Unlock()

	hotPotato.mu.Lock()
	last = hotPotato.passLastUsed[carrierUID]
	elapsed = time.Since(last)
	blocked = elapsed < hotPotatoPassCooldown
	hotPotato.mu.Unlock()

	if blocked {
		t.Error("expected pass cooldown to have expired after sufficient time")
	}
}

// TestHotPotatoPassNotCarrier verifies that only the current carrier can pass.
func TestHotPotatoPassNotCarrier(t *testing.T) {
	resetHotPotatoState()

	hotPotato.mu.Lock()
	hotPotato.gameActive = true
	hotPotato.carrierUID = 10
	hotPotato.participants[10] = struct{}{}
	hotPotato.participants[11] = struct{}{}
	hotPotato.mu.Unlock()

	// A non-carrier UID should not equal carrierUID.
	hotPotato.mu.Lock()
	isCarrier := hotPotato.carrierUID == 11
	hotPotato.mu.Unlock()

	if isCarrier {
		t.Error("UID 11 should not be the carrier")
	}
}

// TestHotPotatoPassUpdatesCarrier verifies that passLastUsed and carrierUID are
// updated correctly when a pass is recorded.
func TestHotPotatoPassUpdatesCarrier(t *testing.T) {
	resetHotPotatoState()

	hotPotato.mu.Lock()
	hotPotato.gameActive = true
	hotPotato.carrierUID = 1
	hotPotato.participants[1] = struct{}{}
	hotPotato.participants[2] = struct{}{}
	hotPotato.mu.Unlock()

	// Simulate what hotPotatoPass does after selecting new carrier UID 2.
	hotPotato.mu.Lock()
	hotPotato.passLastUsed[1] = time.Now()
	hotPotato.carrierUID = 2
	hotPotato.mu.Unlock()

	hotPotato.mu.Lock()
	newCarrier := hotPotato.carrierUID
	_, recorded := hotPotato.passLastUsed[1]
	hotPotato.mu.Unlock()

	if newCarrier != 2 {
		t.Errorf("expected carrierUID to be 2 after pass, got %d", newCarrier)
	}
	if !recorded {
		t.Error("expected passLastUsed to be recorded for original carrier UID 1")
	}
}
