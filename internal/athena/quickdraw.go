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
"math/rand"
"strconv"
"sync"
"time"
)

const (
quickdrawChallengeTimeout = 30 * time.Second // window to accept a challenge
quickdrawReactionTimeout  = 10 * time.Second // time to react after DRAW!
quickdrawPunishDuration   = 10 * time.Minute // how long the loser's punishment lasts
)

// randomQuickdrawPunishment picks a random punishment from the shared pool.
func randomQuickdrawPunishment() PunishmentType {
return hotPotatoPunishmentPool[rand.Intn(len(hotPotatoPunishmentPool))]
}

// quickdrawWords is the pool of short words players must type after "DRAW!".
var quickdrawWords = []string{
"draw", "fire", "shoot", "bang", "duel", "ready", "aim",
"blaze", "flash", "quick", "swift", "sharp", "steel", "iron",
"hawk", "eagle", "wolf", "tiger", "cobra", "viper",
"bolt", "blast", "spark", "flare", "strike",
}

// quickdrawPickWord returns a random word from the pool.
func quickdrawPickWord() string {
return quickdrawWords[rand.Intn(len(quickdrawWords))]
}

// quickdrawDuel holds the two participants and the current duel phase.
type quickdrawDuel struct {
challengerUID int
challengedUID int
targetWord    string // word players must type after DRAW! (lower-case)
drawSignaled  bool   // true after "DRAW!" is announced
resolved      bool   // true once the outcome has been determined
}

// quickdrawState is the mutex-protected global state for all quickdraw duels.
// challengerBusy provides an O(1) "already challenging?" lookup, eliminating
// the need to scan pendingChallenges for the challenger side.
type quickdrawState struct {
mu                sync.Mutex
challengerBusy    map[int]struct{}       // set of UIDs with an outgoing pending challenge
pendingChallenges map[int]int            // challenged UID → challenger UID
activeDuels       map[int]*quickdrawDuel // UID → duel (both parties share the same pointer)
}

var qdState = quickdrawState{
challengerBusy:    make(map[int]struct{}),
pendingChallenges: make(map[int]int),
activeDuels:       make(map[int]*quickdrawDuel),
}

// cmdQuickdraw handles /quickdraw <uid|accept|decline>.
func cmdQuickdraw(client *Client, args []string, usage string) {
switch args[0] {
case "accept":
quickdrawAccept(client)
case "decline":
quickdrawDecline(client)
default:
uid, err := strconv.Atoi(args[0])
if err != nil || uid < 0 {
client.SendServerMessage("Invalid UID. " + usage)
return
}
quickdrawChallenge(client, uid)
}
}

// quickdrawChallenge sends a duel challenge from client to the player with targetUID.
func quickdrawChallenge(client *Client, targetUID int) {
challengerUID := client.Uid()

if challengerUID == targetUID {
client.SendServerMessage("You cannot challenge yourself to a duel.")
return
}

target, err := getClientByUid(targetUID)
if err != nil {
client.SendServerMessage(fmt.Sprintf("No connected player with UID %d.", targetUID))
return
}

qdState.mu.Lock()
if _, inDuel := qdState.activeDuels[challengerUID]; inDuel {
qdState.mu.Unlock()
client.SendServerMessage("You are already in a quickdraw duel.")
return
}
if _, busy := qdState.challengerBusy[challengerUID]; busy {
qdState.mu.Unlock()
client.SendServerMessage("You already have a pending quickdraw challenge.")
return
}
if _, inDuel := qdState.activeDuels[targetUID]; inDuel {
qdState.mu.Unlock()
client.SendServerMessage(fmt.Sprintf("UID %d is already in a quickdraw duel.", targetUID))
return
}
if _, pending := qdState.pendingChallenges[targetUID]; pending {
qdState.mu.Unlock()
client.SendServerMessage(fmt.Sprintf("UID %d already has a pending quickdraw challenge.", targetUID))
return
}
qdState.pendingChallenges[targetUID] = challengerUID
qdState.challengerBusy[challengerUID] = struct{}{}
qdState.mu.Unlock()

challengerName := client.OOCName()
targetName := target.OOCName()

target.SendServerMessage(fmt.Sprintf(
"🔫 %v (UID %d) challenges you to a QUICKDRAW DUEL! "+
"Type /quickdraw accept to accept or /quickdraw decline to decline. "+
"You have 30 seconds.",
challengerName, challengerUID,
))
client.SendServerMessage(fmt.Sprintf(
"🔫 Challenge sent to %v (UID %d). Waiting for their response...",
targetName, targetUID,
))
addToBuffer(client, "QUICKDRAW",
fmt.Sprintf("Challenged UID %d (%v) to a quickdraw duel", targetUID, targetName), false)

go quickdrawExpireChallenge(challengerUID, targetUID, challengerName, targetName)
}

// quickdrawExpireChallenge expires a challenge that was never accepted or declined.
func quickdrawExpireChallenge(challengerUID, targetUID int, challengerName, targetName string) {
time.Sleep(quickdrawChallengeTimeout)

qdState.mu.Lock()
if cUID, ok := qdState.pendingChallenges[targetUID]; !ok || cUID != challengerUID {
qdState.mu.Unlock()
return
}
delete(qdState.pendingChallenges, targetUID)
delete(qdState.challengerBusy, challengerUID)
qdState.mu.Unlock()

if challenger, err := getClientByUid(challengerUID); err == nil {
challenger.SendServerMessage(fmt.Sprintf(
"⌛ Your quickdraw challenge to %v (UID %d) expired.", targetName, targetUID,
))
}
if target, err := getClientByUid(targetUID); err == nil {
target.SendServerMessage(fmt.Sprintf(
"⌛ The quickdraw challenge from %v (UID %d) expired.", challengerName, challengerUID,
))
}
}

// quickdrawAccept is called when a challenged player accepts the duel.
func quickdrawAccept(client *Client) {
challengedUID := client.Uid()

qdState.mu.Lock()
challengerUID, ok := qdState.pendingChallenges[challengedUID]
if !ok {
qdState.mu.Unlock()
client.SendServerMessage("You have no pending quickdraw challenge.")
return
}
challenger, err := getClientByUid(challengerUID)
if err != nil {
delete(qdState.pendingChallenges, challengedUID)
delete(qdState.challengerBusy, challengerUID)
qdState.mu.Unlock()
client.SendServerMessage("The challenger has disconnected. Challenge cancelled.")
return
}
delete(qdState.pendingChallenges, challengedUID)
delete(qdState.challengerBusy, challengerUID)
duel := &quickdrawDuel{challengerUID: challengerUID, challengedUID: challengedUID}
qdState.activeDuels[challengerUID] = duel
qdState.activeDuels[challengedUID] = duel
qdState.mu.Unlock()

challengerName := challenger.OOCName()
challengedName := client.OOCName()

sendGlobalServerMessage(fmt.Sprintf(
"🔫 QUICKDRAW DUEL: %v (UID %d) vs %v (UID %d)! Countdown starting...",
challengerName, challengerUID, challengedName, challengedUID,
))
addToBuffer(client, "QUICKDRAW",
fmt.Sprintf("Accepted quickdraw challenge from UID %d (%v)", challengerUID, challengerName), false)

go quickdrawRun(duel, challengerName, challengedName)
}

// quickdrawDecline is called when a challenged player declines the duel.
func quickdrawDecline(client *Client) {
challengedUID := client.Uid()

qdState.mu.Lock()
challengerUID, ok := qdState.pendingChallenges[challengedUID]
if !ok {
qdState.mu.Unlock()
client.SendServerMessage("You have no pending quickdraw challenge.")
return
}
delete(qdState.pendingChallenges, challengedUID)
delete(qdState.challengerBusy, challengerUID)
qdState.mu.Unlock()

challengedName := client.OOCName()
if challenger, err := getClientByUid(challengerUID); err == nil {
challenger.SendServerMessage(fmt.Sprintf(
"😤 %v (UID %d) declined your quickdraw challenge. Coward!",
challengedName, challengedUID,
))
}
client.SendServerMessage("You declined the quickdraw challenge.")
addToBuffer(client, "QUICKDRAW",
fmt.Sprintf("Declined quickdraw challenge from UID %d", challengerUID), false)
}

// quickdrawRun runs the full duel lifecycle in a single goroutine:
// 3-2-1 countdown, DRAW signal, reaction window, and outcome resolution.
func quickdrawRun(duel *quickdrawDuel, challengerName, challengedName string) {
for i := 3; i > 0; i-- {
sendGlobalServerMessage(fmt.Sprintf("%d...", i))
time.Sleep(time.Second)
}

qdState.mu.Lock()
if duel.resolved {
qdState.mu.Unlock()
return
}
duel.targetWord = quickdrawPickWord()
duel.drawSignaled = true
qdState.mu.Unlock()

sendGlobalServerMessage(fmt.Sprintf("🔫 DRAW! Type this word in IC: \"%s\" — the first to type it wins!", duel.targetWord))
time.Sleep(quickdrawReactionTimeout)

qdState.mu.Lock()
if duel.resolved {
qdState.mu.Unlock()
return
}
duel.resolved = true
delete(qdState.activeDuels, duel.challengerUID)
delete(qdState.activeDuels, duel.challengedUID)
qdState.mu.Unlock()

// Both were too slow — punish both.
for _, uid := range [2]int{duel.challengerUID, duel.challengedUID} {
if c, err := getClientByUid(uid); err == nil {
pType := randomQuickdrawPunishment()
c.AddPunishment(pType, quickdrawPunishDuration, "Quickdraw: too slow")
c.SendServerMessage(fmt.Sprintf("🐢 You were too slow! Punished with '%v' for %v.", pType, quickdrawPunishDuration))
}
}
sendGlobalServerMessage(fmt.Sprintf(
"😴 QUICKDRAW RESULT: Both %v and %v were too slow! Both receive a punishment!",
challengerName, challengedName,
))
}

// quickdrawOnIC is called from pktIC whenever a client sends an IC message.
// If the client is in an active duel after the DRAW signal and typed the correct
// word, they win. Wrong words are silently ignored so the duel continues.
func quickdrawOnIC(client *Client, msgText string) {
uid := client.Uid()

qdState.mu.Lock()
duel, ok := qdState.activeDuels[uid]
if !ok || !duel.drawSignaled || duel.resolved {
qdState.mu.Unlock()
return
}
if normaliseTypingPhrase(msgText) != duel.targetWord {
qdState.mu.Unlock()
client.SendServerMessage(fmt.Sprintf("🔫 Wrong word! Type: \"%s\"", duel.targetWord))
return
}
duel.resolved = true
delete(qdState.activeDuels, duel.challengerUID)
delete(qdState.activeDuels, duel.challengedUID)
loserUID := duel.challengedUID
if uid == duel.challengedUID {
loserUID = duel.challengerUID
}
qdState.mu.Unlock()

quickdrawResolve(uid, loserUID)
}

// quickdrawResolve applies the punishment to the loser and announces the outcome.
func quickdrawResolve(winnerUID, loserUID int) {
winner, _ := getClientByUid(winnerUID)
loser, loserErr := getClientByUid(loserUID)

winnerName := fmt.Sprintf("UID %d", winnerUID)
if winner != nil {
winnerName = winner.OOCName()
}
loserName := fmt.Sprintf("UID %d", loserUID)
if loser != nil {
loserName = loser.OOCName()
}

if loserErr == nil {
pType := randomQuickdrawPunishment()
loser.AddPunishment(pType, quickdrawPunishDuration, "Quickdraw: loser")
loser.SendServerMessage(fmt.Sprintf(
"💀 You lost the quickdraw duel! Punished with '%v' for %v.", pType, quickdrawPunishDuration,
))
sendGlobalServerMessage(fmt.Sprintf(
"🏆 QUICKDRAW RESULT: %v was faster! %v loses and receives '%v'!", winnerName, loserName, pType,
))
if winner != nil {
winner.SendServerMessage("🏆 You won the quickdraw duel! Nice shot!")
addToBuffer(winner, "QUICKDRAW",
fmt.Sprintf("Won duel vs UID %d (%v), loser punished with %v", loserUID, loserName, pType), false)
}
} else {
sendGlobalServerMessage(fmt.Sprintf(
"🏆 QUICKDRAW RESULT: %v wins! Their opponent disconnected.", winnerName,
))
if winner != nil {
winner.SendServerMessage("🏆 You won the quickdraw duel — your opponent left the server!")
}
}
}
