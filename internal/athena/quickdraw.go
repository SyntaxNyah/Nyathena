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
"crypto/rand"
"fmt"
mrand "math/rand"
"math/big"
"strconv"
"sync"
"sync/atomic"
"time"

"github.com/MangosArentLiterature/Athena/internal/area"
)

const (
quickdrawChallengeTimeout = 30 * time.Second // window to accept a challenge
quickdrawReactionTimeout  = 10 * time.Second // time to react after DRAW!
quickdrawPunishDuration   = 10 * time.Minute // how long the loser's punishment lasts
)

// randomQuickdrawPunishment picks a random punishment from the shared pool.
func randomQuickdrawPunishment() PunishmentType {
return hotPotatoPunishmentPool[mrand.Intn(len(hotPotatoPunishmentPool))]
}

// quickdrawWords is the large, varied pool of words players must type after "DRAW!".
// Words span multiple themes and difficulty levels to keep every duel unpredictable.
// quickdrawWordCount is the big.Int upper bound for quickdrawPickWord so that
// big.NewInt is not allocated on every duel start.
var quickdrawWordCount *big.Int

// quickdrawAnyActive is an atomic fast-path flag for quickdrawOnIC.
// It is true when len(activeDuels) > 0.  quickdrawOnIC reads this without
// the mutex so that the common case (no duel active) costs only one atomic
// load rather than a full mutex acquire/release on every IC message.
var quickdrawAnyActive atomic.Bool

func init() {
// Populated after quickdrawWords is initialised.
quickdrawWordCount = big.NewInt(int64(len(quickdrawWords)))
}

var quickdrawWords = []string{
// Duel / western theme
"draw", "fire", "shoot", "bang", "aim", "duel", "ready", "blaze",
"holster", "trigger", "gunshot", "bullet", "pistol", "revolver",
"cowboy", "outlaw", "sheriff", "marshal", "bounty", "saloon",
// Speed / reaction
"quick", "flash", "bolt", "spark", "swift", "rapid", "dash",
"sprint", "laser", "warp", "zoom", "rush", "burst", "snap",
// Animals / nature
"hawk", "eagle", "wolf", "tiger", "cobra", "viper", "falcon",
"panther", "jaguar", "lynx", "raven", "coyote", "puma", "mamba",
// Weapons / combat
"blade", "sword", "arrow", "lance", "shield", "spear", "fist",
"cannon", "dagger", "cutlass", "saber", "rapier", "mace", "axe",
// Elements / forces
"ember", "frost", "storm", "thunder", "gale", "quake", "flood",
"inferno", "glacier", "tempest", "cyclone", "typhoon", "volt",
// Attorney Online flavour
"objection", "holdit", "evidence", "witness", "verdict", "alibi",
"guilty", "innocent", "motive", "suspect", "courtroom", "gavel",
// Misc short punchy words
"chaos", "glory", "valor", "honor", "pride", "vengeance", "legend",
"crimson", "shadow", "phantom", "specter", "wraith", "spirit",
"gold", "silver", "iron", "steel", "copper", "diamond",
}

// quickdrawPickWord returns a cryptographically random word from the pool so that
// picks are genuinely unpredictable across server restarts and sequential duels.
func quickdrawPickWord() string {
n, err := rand.Int(rand.Reader, quickdrawWordCount)
if err != nil {
// crypto/rand failure is extraordinarily rare; fall back to math/rand.
return quickdrawWords[mrand.Intn(len(quickdrawWords))]
}
return quickdrawWords[n.Int64()]
}

// quickdrawDuel holds the two participants and the current duel phase.
type quickdrawDuel struct {
challengerUID int
challengedUID int
targetWord    string     // word players must type after DRAW! (lower-case); empty in bullet mode
bulletMode    bool       // true when the duel is a bullet duel (first ANY IC message wins)
drawSignaled  bool       // true after "DRAW!" is announced
resolved      bool       // true once the outcome has been determined
area          *area.Area // area where the duel takes place (for local-only broadcasts)
}

// quickdrawState is the mutex-protected global state for all quickdraw duels.
// challengerBusy provides an O(1) "already challenging?" lookup, eliminating
// the need to scan pendingChallenges for the challenger side.
type quickdrawState struct {
mu                sync.Mutex
challengerBusy    map[int]struct{}       // set of UIDs with an outgoing pending challenge
pendingChallenges map[int]int            // challenged UID → challenger UID
pendingBulletMode map[int]bool           // challenged UID → bullet mode flag
activeDuels       map[int]*quickdrawDuel // UID → duel (both parties share the same pointer)
}

var qdState = quickdrawState{
challengerBusy:    make(map[int]struct{}),
pendingChallenges: make(map[int]int),
pendingBulletMode: make(map[int]bool),
activeDuels:       make(map[int]*quickdrawDuel),
}

// cmdQuickdraw handles /quickdraw <uid|bullet <uid>|accept|decline>.
func cmdQuickdraw(client *Client, args []string, usage string) {
switch args[0] {
case "accept":
quickdrawAccept(client)
case "decline":
quickdrawDecline(client)
case "bullet":
if len(args) < 2 {
client.SendServerMessage("Usage: /quickdraw bullet <uid>")
return
}
uid, err := strconv.Atoi(args[1])
if err != nil || uid < 0 {
client.SendServerMessage("Invalid UID. Usage: /quickdraw bullet <uid>")
return
}
quickdrawChallenge(client, uid, true)
default:
uid, err := strconv.Atoi(args[0])
if err != nil || uid < 0 {
client.SendServerMessage("Invalid UID. " + usage)
return
}
quickdrawChallenge(client, uid, false)
}
}

// quickdrawChallenge sends a duel challenge from client to the player with targetUID.
// bulletMode=true starts a bullet duel where the first player to send ANY IC message wins.
func quickdrawChallenge(client *Client, targetUID int, bulletMode bool) {
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
if bulletMode {
qdState.pendingBulletMode[targetUID] = true
}
qdState.mu.Unlock()

challengerName := client.OOCName()
targetName := target.OOCName()

modeDesc := "standard (type a word)"
if bulletMode {
modeDesc = "bullet (first to send ANY message)"
}
target.SendServerMessage(fmt.Sprintf(
"🔫 %v (UID %d) challenges you to a QUICKDRAW DUEL [%s]! "+
"Type /quickdraw accept to accept or /quickdraw decline to decline. "+
"You have 30 seconds.",
challengerName, challengerUID, modeDesc,
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
delete(qdState.pendingBulletMode, targetUID)
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
delete(qdState.pendingBulletMode, challengedUID)
qdState.mu.Unlock()
client.SendServerMessage("The challenger has disconnected. Challenge cancelled.")
return
}
bulletMode := qdState.pendingBulletMode[challengedUID]
delete(qdState.pendingChallenges, challengedUID)
delete(qdState.challengerBusy, challengerUID)
delete(qdState.pendingBulletMode, challengedUID)
duel := &quickdrawDuel{challengerUID: challengerUID, challengedUID: challengedUID, bulletMode: bulletMode, area: challenger.Area()}
qdState.activeDuels[challengerUID] = duel
qdState.activeDuels[challengedUID] = duel
quickdrawAnyActive.Store(true)
qdState.mu.Unlock()

challengerName := challenger.OOCName()
challengedName := client.OOCName()

sendAreaServerMessage(duel.area, fmt.Sprintf(
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
delete(qdState.pendingBulletMode, challengedUID)
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
sendAreaServerMessage(duel.area, fmt.Sprintf("%d...", i))
time.Sleep(time.Second)
}

qdState.mu.Lock()
if duel.resolved {
qdState.mu.Unlock()
return
}
duel.drawSignaled = true
bullet := duel.bulletMode
if !bullet {
duel.targetWord = quickdrawPickWord()
}
word := duel.targetWord // capture before unlock to avoid data race
qdState.mu.Unlock()

if bullet {
sendAreaServerMessage(duel.area, "🔫 DRAW! — Send ANY IC message first to win!")
} else {
sendAreaServerMessage(duel.area, fmt.Sprintf("🔫 DRAW! Type this word in IC: \"%s\" — the first to type it wins!", word))
}
time.Sleep(quickdrawReactionTimeout)

qdState.mu.Lock()
if duel.resolved {
qdState.mu.Unlock()
return
}
duel.resolved = true
delete(qdState.activeDuels, duel.challengerUID)
delete(qdState.activeDuels, duel.challengedUID)
if len(qdState.activeDuels) == 0 {
quickdrawAnyActive.Store(false)
}
qdState.mu.Unlock()

// Both were too slow — punish both.
for _, uid := range [2]int{duel.challengerUID, duel.challengedUID} {
if c, err := getClientByUid(uid); err == nil {
pType := randomQuickdrawPunishment()
c.AddPunishment(pType, quickdrawPunishDuration, "Quickdraw: too slow")
c.SendServerMessage(fmt.Sprintf("🐢 You were too slow! Punished with '%v' for %v.", pType, quickdrawPunishDuration))
}
}
sendAreaServerMessage(duel.area, fmt.Sprintf(
"😴 QUICKDRAW RESULT: Both %v and %v were too slow! Both receive a punishment!",
challengerName, challengedName,
))
}

// quickdrawOnIC is called from pktIC whenever a client sends an IC message.
// In standard mode the client must type the target word to win; wrong words are
// silently ignored. In bullet mode any IC message after DRAW wins immediately.
func quickdrawOnIC(client *Client, msgText string) {
// Atomic fast-path: skip the mutex entirely when no duel is active.
// This is the common case — most IC messages arrive outside a duel.
if !quickdrawAnyActive.Load() {
return
}

uid := client.Uid()

qdState.mu.Lock()
duel, ok := qdState.activeDuels[uid]
if !ok || !duel.drawSignaled || duel.resolved {
qdState.mu.Unlock()
return
}
if !duel.bulletMode && normaliseTypingPhrase(msgText) != duel.targetWord {
qdState.mu.Unlock()
client.SendServerMessage(fmt.Sprintf("🔫 Wrong word! Type: \"%s\"", duel.targetWord))
return
}
duel.resolved = true
delete(qdState.activeDuels, duel.challengerUID)
delete(qdState.activeDuels, duel.challengedUID)
if len(qdState.activeDuels) == 0 {
quickdrawAnyActive.Store(false)
}
loserUID := duel.challengedUID
if uid == duel.challengedUID {
loserUID = duel.challengerUID
}
qdState.mu.Unlock()

quickdrawResolve(uid, loserUID, duel.area)
}

// quickdrawResolve applies the punishment to the loser and announces the outcome.
func quickdrawResolve(winnerUID, loserUID int, a *area.Area) {
winner, _ := getClientByUid(winnerUID)
loser, loserErr := getClientByUid(loserUID)

winnerName := "UID " + strconv.Itoa(winnerUID)
if winner != nil {
winnerName = winner.OOCName()
}
loserName := "UID " + strconv.Itoa(loserUID)
if loser != nil {
loserName = loser.OOCName()
}

if loserErr == nil {
pType := randomQuickdrawPunishment()
loser.AddPunishment(pType, quickdrawPunishDuration, "Quickdraw: loser")
loser.SendServerMessage(fmt.Sprintf(
"💀 You lost the quickdraw duel! Punished with '%v' for %v.", pType, quickdrawPunishDuration,
))
sendAreaServerMessage(a, fmt.Sprintf(
"🏆 QUICKDRAW RESULT: %v was faster! %v loses and receives '%v'!", winnerName, loserName, pType,
))
if winner != nil {
winner.SendServerMessage("🏆 You won the quickdraw duel! Nice shot!")
addToBuffer(winner, "QUICKDRAW",
fmt.Sprintf("Won duel vs UID %d (%v), loser punished with %v", loserUID, loserName, pType), false)
}
} else {
sendAreaServerMessage(a, fmt.Sprintf(
"🏆 QUICKDRAW RESULT: %v wins! Their opponent disconnected.", winnerName,
))
if winner != nil {
winner.SendServerMessage("🏆 You won the quickdraw duel — your opponent left the server!")
}
}
}
