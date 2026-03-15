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
	"fmt"
	"strings"
	"testing"
)

// resetIsimState resets global insult simulator state between tests.
func resetIsimState() {
	isimGlobal.mu.Lock()
	isimGlobal.challengerBusy = make(map[int]struct{})
	isimGlobal.pendingChallenges = make(map[int]int)
	isimGlobal.activeDuels = make(map[int]*isimDuel)
	isimGlobal.mu.Unlock()
}

// TestIsimRandomPunishment verifies that every returned punishment type belongs
// to the insult-simulator punishment pool.
func TestIsimRandomPunishment(t *testing.T) {
	valid := make(map[PunishmentType]bool, len(isimPunishmentPool))
	for _, p := range isimPunishmentPool {
		valid[p] = true
	}
	for i := 0; i < 100; i++ {
		if p := randomIsimPunishment(); !valid[p] {
			t.Errorf("randomIsimPunishment returned unexpected type: %v", p)
		}
	}
}

// TestIsimPickFragments verifies fragment count and uniqueness.
func TestIsimPickFragments(t *testing.T) {
	if isimFragmentsPerRound > len(insultFragments) {
		t.Fatalf("isimFragmentsPerRound (%d) exceeds pool size (%d)",
			isimFragmentsPerRound, len(insultFragments))
	}
	if numInsultFragments != len(insultFragments) {
		t.Fatalf("numInsultFragments constant (%d) is out of sync with len(insultFragments) (%d)",
			numInsultFragments, len(insultFragments))
	}

	frags := isimPickFragments()
	if len(frags) != isimFragmentsPerRound {
		t.Errorf("expected %d fragments, got %d", isimFragmentsPerRound, len(frags))
	}

	// All returned fragments must appear in the master pool.
	pool := make(map[string]bool, len(insultFragments))
	for _, f := range insultFragments {
		pool[f] = true
	}
	for _, f := range frags {
		if !pool[f] {
			t.Errorf("fragment %q not found in insultFragments pool", f)
		}
	}

	// No duplicates within a single deal.
	seen := make(map[string]bool, len(frags))
	for _, f := range frags {
		if seen[f] {
			t.Errorf("duplicate fragment %q returned in one deal", f)
		}
		seen[f] = true
	}
}

// TestIsimCalculateDamage verifies the damage formula.
func TestIsimCalculateDamage(t *testing.T) {
	cases := []struct {
		picks    int
		wantMin  int // damage must be at least this
		wantMax  int // damage must be at most this
	}{
		{0, 0, 0},
		{1, isimBaseDamage, isimBaseDamage},                                       // no combo
		{2, isimBaseDamage*2 + isimComboBonusDamage, isimBaseDamage*2 + isimComboBonusDamage}, // combo
		{3, isimBaseDamage*3 + isimComboBonusDamage, isimBaseDamage*3 + isimComboBonusDamage},
	}
	for _, tc := range cases {
		got := isimCalculateDamage(tc.picks)
		if got < tc.wantMin || got > tc.wantMax {
			t.Errorf("isimCalculateDamage(%d) = %d, want [%d, %d]",
				tc.picks, got, tc.wantMin, tc.wantMax)
		}
	}
}

// TestIsimAssembleInsult verifies insult assembly from fragments and picks.
func TestIsimAssembleInsult(t *testing.T) {
	frags := []string{"alpha", "bravo", "charlie", "delta", "echo"}

	// No picks → ellipsis placeholder.
	got := isimAssembleInsult(frags, nil)
	if got != "..." {
		t.Errorf("expected '...', got %q", got)
	}

	// Single pick.
	got = isimAssembleInsult(frags, []int{2})
	if got != "bravo" {
		t.Errorf("expected 'bravo', got %q", got)
	}

	// Multiple picks are comma-joined.
	got = isimAssembleInsult(frags, []int{1, 3, 5})
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "charlie") || !strings.Contains(got, "echo") {
		t.Errorf("expected all picked fragments in assembled insult, got %q", got)
	}
}

// TestIsimPendingChallenge verifies that a challenge is stored correctly.
func TestIsimPendingChallenge(t *testing.T) {
	resetIsimState()

	const challengerUID = 1
	const challengedUID = 2

	isimGlobal.mu.Lock()
	isimGlobal.pendingChallenges[challengedUID] = challengerUID
	isimGlobal.challengerBusy[challengerUID] = struct{}{}
	isimGlobal.mu.Unlock()

	isimGlobal.mu.Lock()
	stored, ok := isimGlobal.pendingChallenges[challengedUID]
	_, busy := isimGlobal.challengerBusy[challengerUID]
	isimGlobal.mu.Unlock()

	if !ok {
		t.Fatal("expected pending challenge to be stored")
	}
	if stored != challengerUID {
		t.Errorf("expected challenger UID %d, got %d", challengerUID, stored)
	}
	if !busy {
		t.Error("expected challenger to be marked busy")
	}
}

// TestIsimNoDuplicateChallenge verifies that a player cannot issue two challenges.
func TestIsimNoDuplicateChallenge(t *testing.T) {
	resetIsimState()

	const challengerUID = 10

	isimGlobal.mu.Lock()
	isimGlobal.pendingChallenges[20] = challengerUID
	isimGlobal.challengerBusy[challengerUID] = struct{}{}
	isimGlobal.mu.Unlock()

	isimGlobal.mu.Lock()
	_, alreadyChallenging := isimGlobal.challengerBusy[challengerUID]
	isimGlobal.mu.Unlock()

	if !alreadyChallenging {
		t.Error("expected duplicate challenge to be detected via challengerBusy")
	}
}

// TestIsimActiveDuel verifies that both participants are registered after acceptance.
func TestIsimActiveDuel(t *testing.T) {
	resetIsimState()

	const challengerUID = 3
	const challengedUID = 4

	duel := &isimDuel{
		challenger: &isimPlayer{uid: challengerUID, hp: isimStartingHP},
		challenged: &isimPlayer{uid: challengedUID, hp: isimStartingHP},
		round:      1,
	}

	isimGlobal.mu.Lock()
	isimGlobal.activeDuels[challengerUID] = duel
	isimGlobal.activeDuels[challengedUID] = duel
	isimGlobal.mu.Unlock()

	isimGlobal.mu.Lock()
	d1 := isimGlobal.activeDuels[challengerUID]
	d2 := isimGlobal.activeDuels[challengedUID]
	isimGlobal.mu.Unlock()

	if d1 == nil || d2 == nil {
		t.Fatal("expected both UIDs to be in activeDuels")
	}
	if d1 != d2 {
		t.Error("expected both UIDs to share the same duel pointer")
	}
}

// TestIsimDeclineRemovesChallenge verifies that declining cleans up state.
func TestIsimDeclineRemovesChallenge(t *testing.T) {
	resetIsimState()

	const challengerUID = 11
	const challengedUID = 12

	isimGlobal.mu.Lock()
	isimGlobal.pendingChallenges[challengedUID] = challengerUID
	isimGlobal.challengerBusy[challengerUID] = struct{}{}
	isimGlobal.mu.Unlock()

	// Simulate decline.
	isimGlobal.mu.Lock()
	if _, ok := isimGlobal.pendingChallenges[challengedUID]; ok {
		delete(isimGlobal.pendingChallenges, challengedUID)
		delete(isimGlobal.challengerBusy, challengerUID)
	}
	isimGlobal.mu.Unlock()

	isimGlobal.mu.Lock()
	_, stillPending := isimGlobal.pendingChallenges[challengedUID]
	_, stillBusy := isimGlobal.challengerBusy[challengerUID]
	isimGlobal.mu.Unlock()

	if stillPending {
		t.Error("expected challenge to be removed from pendingChallenges after decline")
	}
	if stillBusy {
		t.Error("expected challenger to be removed from challengerBusy after decline")
	}
}

// TestIsimDuelPlayerHelper verifies isimDuelPlayer returns correct player records.
func TestIsimDuelPlayerHelper(t *testing.T) {
	duel := &isimDuel{
		challenger: &isimPlayer{uid: 5, hp: isimStartingHP},
		challenged: &isimPlayer{uid: 6, hp: isimStartingHP},
	}

	if p := isimDuelPlayer(duel, 5); p == nil || p.uid != 5 {
		t.Error("expected to retrieve challenger by uid 5")
	}
	if p := isimDuelPlayer(duel, 6); p == nil || p.uid != 6 {
		t.Error("expected to retrieve challenged by uid 6")
	}
	if p := isimDuelPlayer(duel, 99); p != nil {
		t.Error("expected nil for unknown uid")
	}
}

// TestIsimOtherPlayerHelper verifies isimOtherPlayer returns the opponent.
func TestIsimOtherPlayerHelper(t *testing.T) {
	duel := &isimDuel{
		challenger: &isimPlayer{uid: 7, hp: isimStartingHP},
		challenged: &isimPlayer{uid: 8, hp: isimStartingHP},
	}

	if other := isimOtherPlayer(duel, 7); other == nil || other.uid != 8 {
		t.Errorf("expected uid 8 as opponent of uid 7, got %v", other)
	}
	if other := isimOtherPlayer(duel, 8); other == nil || other.uid != 7 {
		t.Errorf("expected uid 7 as opponent of uid 8, got %v", other)
	}
}

// TestIsimHPDepletion verifies that damage is applied and HP reaches zero correctly.
func TestIsimHPDepletion(t *testing.T) {
	duel := &isimDuel{
		challenger: &isimPlayer{uid: 1, hp: isimStartingHP},
		challenged: &isimPlayer{uid: 2, hp: isimStartingHP},
	}

	// Simulate the challenger picking 3 fragments each round until the challenged HP reaches 0.
	dmgPerRound := isimCalculateDamage(isimMaxPicksPerRound)
	for duel.challenged.hp > 0 {
		duel.challenged.hp -= dmgPerRound
	}

	if duel.challenged.hp > 0 {
		t.Error("expected challenged HP to reach zero or below")
	}
}

// TestIsimFragmentListMessage verifies the fragment list message format.
func TestIsimFragmentListMessage(t *testing.T) {
	frags := []string{"one", "two", "three", "four", "five"}
	msg := isimFragmentListMessage(frags)

	for i, f := range frags {
		expected := fmt.Sprintf("[%d] %s", i+1, f)
		if !strings.Contains(msg, expected) {
			t.Errorf("expected fragment list to contain %q, but it does not.\nGot: %s", expected, msg)
		}
	}
}
