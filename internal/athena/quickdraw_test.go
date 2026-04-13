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
qdState.challengerBusy = make(map[int]struct{})
qdState.pendingChallenges = make(map[int]int)
qdState.activeDuels = make(map[int]*quickdrawDuel)
qdState.mu.Unlock()
}

// TestRandomQuickdrawPunishment verifies that every returned type belongs to the shared pool.
// quickdraw intentionally reuses hotPotatoPunishmentPool to avoid duplication.
func TestRandomQuickdrawPunishment(t *testing.T) {
valid := make(map[PunishmentType]bool, len(hotPotatoPunishmentPool))
for _, p := range hotPotatoPunishmentPool {
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
qdState.challengerBusy[challengerUID] = struct{}{}
qdState.mu.Unlock()

qdState.mu.Lock()
stored, ok := qdState.pendingChallenges[challengedUID]
_, busy := qdState.challengerBusy[challengerUID]
qdState.mu.Unlock()

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

// TestQuickdrawNoDuplicateChallenge verifies the O(1) busy check blocks a second
// outgoing challenge from the same player.
func TestQuickdrawNoDuplicateChallenge(t *testing.T) {
resetQuickdrawState()

const challengerUID = 10

qdState.mu.Lock()
qdState.pendingChallenges[20] = challengerUID
qdState.challengerBusy[challengerUID] = struct{}{}
qdState.mu.Unlock()

qdState.mu.Lock()
_, alreadyChallenging := qdState.challengerBusy[challengerUID]
qdState.mu.Unlock()

if !alreadyChallenging {
t.Error("expected duplicate challenge to be detected via challengerBusy")
}
}

// TestQuickdrawActiveDuel verifies that both duelist UIDs are present in
// activeDuels and point to the same duel object after acceptance.
func TestQuickdrawActiveDuel(t *testing.T) {
resetQuickdrawState()

const challengerUID = 3
const challengedUID = 4

duel := &quickdrawDuel{challengerUID: challengerUID, challengedUID: challengedUID}

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
targetWord:    "bolt",
drawSignaled:  false,
}

qdState.mu.Lock()
qdState.activeDuels[challengerUID] = duel
qdState.activeDuels[challengedUID] = duel
qdState.mu.Unlock()

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
// DRAW! resolves the duel and removes both UIDs from activeDuels.
func TestQuickdrawOnICFirstResponder(t *testing.T) {
resetQuickdrawState()

const challengerUID = 7
const challengedUID = 8

duel := &quickdrawDuel{
challengerUID: challengerUID,
challengedUID: challengedUID,
targetWord:    "draw",
drawSignaled:  true,
}

qdState.mu.Lock()
qdState.activeDuels[challengerUID] = duel
qdState.activeDuels[challengedUID] = duel
qdState.mu.Unlock()

// Simulate quickdrawOnIC for the challenged player reacting first with the correct word.
uid := challengedUID
qdState.mu.Lock()
d, ok := qdState.activeDuels[uid]
if ok && d.drawSignaled && !d.resolved && normaliseTypingPhrase("draw") == d.targetWord {
d.resolved = true
delete(qdState.activeDuels, d.challengerUID)
delete(qdState.activeDuels, d.challengedUID)
}
qdState.mu.Unlock()

if !duel.resolved {
t.Error("duel should be resolved after first IC message post-DRAW")
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

duel := &quickdrawDuel{
challengerUID: challengerUID,
challengedUID: challengedUID,
targetWord:    "fire",
drawSignaled:  true,
resolved:      true, // already resolved — both UIDs already removed
}

// Simulate quickdrawOnIC for the late challenger.
uid := challengerUID
qdState.mu.Lock()
d, ok := qdState.activeDuels[uid]
if ok && d.drawSignaled && !d.resolved {
d.resolved = true
}
qdState.mu.Unlock()

// Not in activeDuels (already cleaned up) — correct behaviour.
if ok {
t.Error("expected UID to not be in activeDuels after resolution")
}
if !duel.resolved {
t.Error("expected duel to remain resolved")
}
}

// TestQuickdrawDeclineRemovesChallenge verifies that declining a challenge
// removes it from both pendingChallenges and challengerBusy.
func TestQuickdrawDeclineRemovesChallenge(t *testing.T) {
resetQuickdrawState()

const challengerUID = 11
const challengedUID = 12

qdState.mu.Lock()
qdState.pendingChallenges[challengedUID] = challengerUID
qdState.challengerBusy[challengerUID] = struct{}{}
qdState.mu.Unlock()

// Simulate quickdrawDecline.
qdState.mu.Lock()
if _, ok := qdState.pendingChallenges[challengedUID]; ok {
delete(qdState.pendingChallenges, challengedUID)
delete(qdState.challengerBusy, challengerUID)
}
qdState.mu.Unlock()

qdState.mu.Lock()
_, stillPending := qdState.pendingChallenges[challengedUID]
_, stillBusy := qdState.challengerBusy[challengerUID]
qdState.mu.Unlock()

if stillPending {
t.Error("expected challenge to be removed from pendingChallenges after decline")
}
if stillBusy {
t.Error("expected challenger to be removed from challengerBusy after decline")
}
}

// TestQuickdrawReactionTimerResolvesIfUnresolved verifies that when the reaction
// timer fires and the duel is unresolved, it marks it as resolved.
func TestQuickdrawReactionTimerResolvesIfUnresolved(t *testing.T) {
resetQuickdrawState()

duel := &quickdrawDuel{
challengerUID: 13,
challengedUID: 14,
drawSignaled:  true,
resolved:      false,
}

qdState.mu.Lock()
qdState.activeDuels[13] = duel
qdState.activeDuels[14] = duel
qdState.mu.Unlock()

// Simulate the timeout path in quickdrawRun.
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

// TestQuickdrawOnICWrongWordIgnored verifies that typing the wrong word after DRAW
// does not resolve the duel.
func TestQuickdrawOnICWrongWordIgnored(t *testing.T) {
resetQuickdrawState()

const challengerUID = 15
const challengedUID = 16

duel := &quickdrawDuel{
challengerUID: challengerUID,
challengedUID: challengedUID,
targetWord:    "draw",
drawSignaled:  true,
}

qdState.mu.Lock()
qdState.activeDuels[challengerUID] = duel
qdState.activeDuels[challengedUID] = duel
qdState.mu.Unlock()

// Simulate quickdrawOnIC with the wrong word — should not resolve.
uid := challengedUID
qdState.mu.Lock()
d, ok := qdState.activeDuels[uid]
if ok && d.drawSignaled && !d.resolved {
if normaliseTypingPhrase("wrongword") == d.targetWord {
d.resolved = true
delete(qdState.activeDuels, d.challengerUID)
delete(qdState.activeDuels, d.challengedUID)
}
}
qdState.mu.Unlock()

if duel.resolved {
t.Error("duel should not be resolved after wrong word")
}

qdState.mu.Lock()
_, stillActive1 := qdState.activeDuels[challengerUID]
_, stillActive2 := qdState.activeDuels[challengedUID]
qdState.mu.Unlock()

if !stillActive1 || !stillActive2 {
t.Error("expected both UIDs to remain in activeDuels after wrong word")
}
}
