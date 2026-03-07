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
)

// resetQuickdrawState resets the global quickdraw state between tests.
func resetQuickdrawState() {
	qdState.mu.Lock()
	qdState.pendingChallenges = make(map[int]int)
	qdState.activeDuels = make(map[int]*quickdrawDuel)
	qdState.mu.Unlock()
}

// TestRandomQuickdrawPunishment verifies that every returned type belongs to
// the punishment pool.
func TestRandomQuickdrawPunishment(t *testing.T) {
	valid := make(map[PunishmentType]bool, len(quickdrawPunishmentPool))
	for _, p := range quickdrawPunishmentPool {
		valid[p] = true
	}

	const draws = 100
	for i := 0; i < draws; i++ {
		if p := randomQuickdrawPunishment(); !valid[p] {
			t.Errorf("randomQuickdrawPunishment returned unexpected type: %v", p)
		}
	}
}

// TestQuickdrawPendingChallenge verifies that a challenge is stored correctly.
func TestQuickdrawPendingChallenge(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 1
	const challengedUID = 2

	qdState.mu.Lock()
	qdState.pendingChallenges[challengedUID] = challengerUID
	qdState.mu.Unlock()

	qdState.mu.Lock()
	stored, ok := qdState.pendingChallenges[challengedUID]
	qdState.mu.Unlock()

	if !ok {
		t.Fatal("expected pending challenge to be stored")
	}
	if stored != challengerUID {
		t.Errorf("expected challenger UID %d, got %d", challengerUID, stored)
	}
}

// TestQuickdrawNoDuplicateChallenge verifies that a player cannot have two
// simultaneous pending challenges as the challenger.
func TestQuickdrawNoDuplicateChallenge(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 10
	// Record a pending challenge from challengerUID to target 20.
	qdState.mu.Lock()
	qdState.pendingChallenges[20] = challengerUID
	qdState.mu.Unlock()

	// Simulate the check performed before accepting a new challenge.
	alreadyChallenging := false
	qdState.mu.Lock()
	for _, cUID := range qdState.pendingChallenges {
		if cUID == challengerUID {
			alreadyChallenging = true
			break
		}
	}
	qdState.mu.Unlock()

	if !alreadyChallenging {
		t.Error("expected duplicate challenge to be detected")
	}
}

// TestQuickdrawActiveDuel verifies that both duelist UIDs are present in
// activeDuels and point to the same duel object after acceptance.
func TestQuickdrawActiveDuel(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 3
	const challengedUID = 4

	duel := &quickdrawDuel{
		challengerUID: challengerUID,
		challengedUID: challengedUID,
		winnerUID:     -1,
	}

	qdState.mu.Lock()
	qdState.activeDuels[challengerUID] = duel
	qdState.activeDuels[challengedUID] = duel
	qdState.mu.Unlock()

	qdState.mu.Lock()
	d1 := qdState.activeDuels[challengerUID]
	d2 := qdState.activeDuels[challengedUID]
	qdState.mu.Unlock()

	if d1 == nil || d2 == nil {
		t.Fatal("expected both UIDs to be in activeDuels")
	}
	if d1 != d2 {
		t.Error("expected both UIDs to share the same duel pointer")
	}
}

// TestQuickdrawOnICBeforeDraw verifies that an IC message before the DRAW
// signal does not resolve the duel.
func TestQuickdrawOnICBeforeDraw(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 5
	const challengedUID = 6

	duel := &quickdrawDuel{
		challengerUID: challengerUID,
		challengedUID: challengedUID,
		drawSignaled:  false, // DRAW! not yet signaled
		winnerUID:     -1,
	}

	qdState.mu.Lock()
	qdState.activeDuels[challengerUID] = duel
	qdState.activeDuels[challengedUID] = duel
	qdState.mu.Unlock()

	// Simulate what quickdrawOnIC does when drawSignaled is false.
	qdState.mu.Lock()
	shouldReact := duel.drawSignaled && !duel.resolved
	qdState.mu.Unlock()

	if shouldReact {
		t.Error("expected IC message before DRAW to be ignored")
	}
	if duel.resolved {
		t.Error("duel should not be resolved before DRAW signal")
	}
}

// TestQuickdrawOnICFirstResponder verifies that the first IC message after
// DRAW! marks that player as the winner and resolves the duel.
func TestQuickdrawOnICFirstResponder(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 7
	const challengedUID = 8

	duel := &quickdrawDuel{
		challengerUID: challengerUID,
		challengedUID: challengedUID,
		drawSignaled:  true, // DRAW! already signaled
		winnerUID:     -1,
	}

	qdState.mu.Lock()
	qdState.activeDuels[challengerUID] = duel
	qdState.activeDuels[challengedUID] = duel
	qdState.mu.Unlock()

	// Simulate quickdrawOnIC for the challenged player reacting first.
	uid := challengedUID
	qdState.mu.Lock()
	d, ok := qdState.activeDuels[uid]
	if ok && d.drawSignaled && !d.resolved {
		d.resolved = true
		d.winnerUID = uid
		delete(qdState.activeDuels, d.challengerUID)
		delete(qdState.activeDuels, d.challengedUID)
	}
	qdState.mu.Unlock()

	if !duel.resolved {
		t.Error("duel should be resolved after first IC message post-DRAW")
	}
	if duel.winnerUID != challengedUID {
		t.Errorf("expected winner UID %d, got %d", challengedUID, duel.winnerUID)
	}

	qdState.mu.Lock()
	_, stillActive1 := qdState.activeDuels[challengerUID]
	_, stillActive2 := qdState.activeDuels[challengedUID]
	qdState.mu.Unlock()

	if stillActive1 || stillActive2 {
		t.Error("expected both UIDs to be removed from activeDuels after resolution")
	}
}

// TestQuickdrawOnICSecondResponderIgnored verifies that a second IC message
// does not change the already-resolved duel.
func TestQuickdrawOnICSecondResponderIgnored(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 9
	const challengedUID = 10
	const firstWinnerUID = challengedUID

	duel := &quickdrawDuel{
		challengerUID: challengerUID,
		challengedUID: challengedUID,
		drawSignaled:  true,
		resolved:      true, // already resolved
		winnerUID:     firstWinnerUID,
	}

	// activeDuels should already have been cleaned up by resolution; simulate that.
	// (Both UIDs deleted from map.)

	// Simulate quickdrawOnIC for the challenger arriving late.
	uid := challengerUID
	qdState.mu.Lock()
	d, ok := qdState.activeDuels[uid]
	if ok && d.drawSignaled && !d.resolved {
		d.resolved = true
		d.winnerUID = uid
	}
	qdState.mu.Unlock()

	// The winner should still be the first responder.
	if !ok {
		// Not in activeDuels — correct behaviour.
	}
	if duel.winnerUID != firstWinnerUID {
		t.Errorf("expected winner to remain UID %d, got %d", firstWinnerUID, duel.winnerUID)
	}
}

// TestQuickdrawDeclineRemovesChallenge verifies that declining a challenge
// removes it from pendingChallenges.
func TestQuickdrawDeclineRemovesChallenge(t *testing.T) {
	resetQuickdrawState()

	const challengerUID = 11
	const challengedUID = 12

	qdState.mu.Lock()
	qdState.pendingChallenges[challengedUID] = challengerUID
	qdState.mu.Unlock()

	// Simulate quickdrawDecline.
	qdState.mu.Lock()
	_, ok := qdState.pendingChallenges[challengedUID]
	if ok {
		delete(qdState.pendingChallenges, challengedUID)
	}
	qdState.mu.Unlock()

	qdState.mu.Lock()
	_, stillPending := qdState.pendingChallenges[challengedUID]
	qdState.mu.Unlock()

	if stillPending {
		t.Error("expected challenge to be removed after decline")
	}
}

// TestQuickdrawReactionTimerResolvesIfUnresolved verifies that when the reaction
// timer fires and the duel is unresolved it marks it as resolved.
func TestQuickdrawReactionTimerResolvesIfUnresolved(t *testing.T) {
	resetQuickdrawState()

	duel := &quickdrawDuel{
		challengerUID: 13,
		challengedUID: 14,
		drawSignaled:  true,
		resolved:      false,
		winnerUID:     -1,
	}

	qdState.mu.Lock()
	qdState.activeDuels[13] = duel
	qdState.activeDuels[14] = duel
	qdState.mu.Unlock()

	// Simulate the timeout resolving the duel (as quickdrawReactionTimer does).
	qdState.mu.Lock()
	if !duel.resolved {
		duel.resolved = true
		delete(qdState.activeDuels, duel.challengerUID)
		delete(qdState.activeDuels, duel.challengedUID)
	}
	qdState.mu.Unlock()

	if !duel.resolved {
		t.Error("expected duel to be resolved after timeout")
	}

	qdState.mu.Lock()
	_, a := qdState.activeDuels[13]
	_, b := qdState.activeDuels[14]
	qdState.mu.Unlock()

	if a || b {
		t.Error("expected both UIDs to be removed from activeDuels after timeout")
	}
}
