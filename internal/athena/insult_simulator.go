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
	"strings"
	"sync"
	"time"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	isimChallengeTimeout  = 30 * time.Second // window to accept a challenge
	isimPickTimeout       = 45 * time.Second // time to pick fragments each round
	isimStartingHP        = 100              // HP each player starts with
	isimFragmentsPerRound = 5                // number of fragment choices offered each round
	isimMaxPicksPerRound  = 3                // maximum fragments a player may pick per round
	isimBaseDamage        = 15               // base damage per fragment selected
	isimComboBonusDamage  = 10               // extra damage when ≥2 fragments are picked together
	isimPunishDuration    = 10 * time.Minute // loser's punishment duration
)

// isimRules is the explanation sent to both players when a duel begins.
const isimRules = `⚔️ INSULT DUEL HAS BEGUN! ⚔️

📋 HOW TO PLAY:
• Each round you will receive %d insult fragments to choose from.
• Use /insultsim pick <numbers> (e.g. /insultsim pick 1 3) to select up to %d fragments.
• Combining 2+ fragments grants a combo damage bonus!
• You have %d seconds per round to make your picks.
• Each player starts with %d HP. First to reach 0 HP loses.
• The loser receives a random punishment. Choose your words carefully! 😈`

// isimRulesMsg is isimRules pre-formatted once at startup; all substitution
// values are compile-time constants so the string never changes.
var isimRulesMsg = fmt.Sprintf(isimRules,
	isimFragmentsPerRound, isimMaxPicksPerRound,
	int(isimPickTimeout.Seconds()), isimStartingHP)

// numInsultFragments is the compile-time length of insultFragments.
// It is used by isimPickFragments to avoid a heap allocation for the index
// permutation.
const numInsultFragments = 30 // keep in sync with len(insultFragments)

// ── Fragment pool ────────────────────────────────────────────────────────────

// insultFragments is the pool of insult fragment phrases used to build insults.
// Fragments are kept family-friendly-ish and AO/courtroom themed.
var insultFragments = []string{
	// Courtroom insults
	"Your objections are weaker than wet parchment",
	"You couldn't win a case against a sleeping judge",
	"Your logic has more holes than Swiss cheese",
	"Even the bailiff pities your argument",
	"Your evidence would embarrass a first-year law student",
	"The jury fell asleep during your testimony",
	"You call that a cross-examination?",
	"Your case collapsed faster than a house of cards",
	"The judge yawned through your entire opening statement",
	"Your closing argument was a masterpiece of incompetence",
	// Personal insults
	"you absolute turnip",
	"you magnificent disaster",
	"you spectacular buffoon",
	"you gloriously confused individual",
	"you walking contradiction",
	"your brilliance is truly inversely proportional to your confidence",
	"your wit is as sharp as a spoon",
	"you couldn't find a clue with both hands and a map",
	// Combination fodder
	"and furthermore",
	"not to mention",
	"on top of all that",
	"as if that weren't enough",
	"to crown it all",
	"I might add that",
	// Finishers
	"good day to you, sir!",
	"I bid you good riddance!",
	"kindly see yourself out!",
	"you absolute waste of courtroom air!",
	"the defence rests its case against your existence!",
	"OBJECTION to your entire personality!",
}

// ── State ────────────────────────────────────────────────────────────────────

// isimPlayer holds per-player round state.
type isimPlayer struct {
	uid       int
	hp        int
	fragments []string // the fragments offered this round
	picks     []int    // 1-based indices of chosen fragments
	picked    bool     // true once the player has submitted picks for this round
}

// isimDuel holds the full state of one active insult duel.
type isimDuel struct {
	challenger *isimPlayer
	challenged *isimPlayer
	round      int
	resolved   bool
	pickDone   chan struct{} // buffered(1); a value is sent when both players have picked
}

// isimState is the mutex-protected global state for all insult duels.
type isimState struct {
	mu                sync.Mutex
	challengerBusy    map[int]struct{}   // set of UIDs with an outgoing pending challenge
	pendingChallenges map[int]int        // challenged UID → challenger UID
	activeDuels       map[int]*isimDuel  // UID → duel (both parties share the same pointer)
}

var isimGlobal = isimState{
	challengerBusy:    make(map[int]struct{}),
	pendingChallenges: make(map[int]int),
	activeDuels:       make(map[int]*isimDuel),
}

// ── Punishment pool ──────────────────────────────────────────────────────────

var isimPunishmentPool = []PunishmentType{
	PunishmentBackward,
	PunishmentStutterstep,
	PunishmentElongate,
	PunishmentUppercase,
	PunishmentLowercase,
	PunishmentRobotic,
	PunishmentAlternating,
	PunishmentUwu,
	PunishmentPirate,
	PunishmentCaveman,
	PunishmentDrunk,
	PunishmentHiccup,
	PunishmentConfused,
	PunishmentParanoid,
	PunishmentMumble,
	PunishmentDegrade,
}

func randomIsimPunishment() PunishmentType {
	return isimPunishmentPool[rand.Intn(len(isimPunishmentPool))]
}

// ── Helper ───────────────────────────────────────────────────────────────────

// isimPickFragments returns a slice of isimFragmentsPerRound randomly sampled
// (without replacement) fragment strings. It performs a partial Fisher-Yates
// shuffle over a stack-allocated index array to avoid the heap allocation that
// rand.Perm would require.
func isimPickFragments() []string {
	var perm [numInsultFragments]int
	for i := range perm {
		perm[i] = i
	}
	out := make([]string, isimFragmentsPerRound)
	for i := range out {
		j := i + rand.Intn(numInsultFragments-i)
		perm[i], perm[j] = perm[j], perm[i]
		out[i] = insultFragments[perm[i]]
	}
	return out
}

// isimCalculateDamage computes how much damage a set of picks deals.
// base = isimBaseDamage × numPicks; combo bonus if numPicks ≥ 2.
func isimCalculateDamage(numPicks int) int {
	if numPicks <= 0 {
		return 0
	}
	dmg := isimBaseDamage * numPicks
	if numPicks >= 2 {
		dmg += isimComboBonusDamage
	}
	return dmg
}

// isimFragmentListMessage formats the per-player fragment list for display.
func isimFragmentListMessage(frags []string) string {
	var sb strings.Builder
	sb.WriteString("Your insult fragments for this round:\n")
	for i, f := range frags {
		sb.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, f))
	}
	sb.WriteString(fmt.Sprintf(
		"Use /insultsim pick <numbers> (e.g. /insultsim pick 1 3) to pick up to %d. "+
			"You have %d seconds!",
		isimMaxPicksPerRound, int(isimPickTimeout.Seconds())))
	return sb.String()
}

// ── Command entry point ──────────────────────────────────────────────────────

// cmdInsultSim is the entry point for /insultsim.
func cmdInsultSim(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage(usage)
		return
	}
	switch args[0] {
	case "accept":
		isimAccept(client)
	case "decline":
		isimDecline(client)
	case "pick":
		isimPick(client, args[1:])
	default:
		uid, err := strconv.Atoi(args[0])
		if err != nil || uid < 0 {
			client.SendServerMessage("Invalid UID. " + usage)
			return
		}
		isimChallenge(client, uid)
	}
}

// ── Challenge flow ───────────────────────────────────────────────────────────

// isimChallenge sends an insult duel challenge from client to the player with targetUID.
func isimChallenge(client *Client, targetUID int) {
	challengerUID := client.Uid()

	if challengerUID == targetUID {
		client.SendServerMessage("You cannot challenge yourself to an insult duel.")
		return
	}

	target, err := getClientByUid(targetUID)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("No connected player with UID %d.", targetUID))
		return
	}

	isimGlobal.mu.Lock()
	if _, inDuel := isimGlobal.activeDuels[challengerUID]; inDuel {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You are already in an insult duel.")
		return
	}
	if _, busy := isimGlobal.challengerBusy[challengerUID]; busy {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You already have a pending insult duel challenge.")
		return
	}
	if _, inDuel := isimGlobal.activeDuels[targetUID]; inDuel {
		isimGlobal.mu.Unlock()
		client.SendServerMessage(fmt.Sprintf("UID %d is already in an insult duel.", targetUID))
		return
	}
	if _, pending := isimGlobal.pendingChallenges[targetUID]; pending {
		isimGlobal.mu.Unlock()
		client.SendServerMessage(fmt.Sprintf("UID %d already has a pending insult duel challenge.", targetUID))
		return
	}
	isimGlobal.pendingChallenges[targetUID] = challengerUID
	isimGlobal.challengerBusy[challengerUID] = struct{}{}
	isimGlobal.mu.Unlock()

	challengerName := client.OOCName()
	targetName := target.OOCName()

	target.SendServerMessage(fmt.Sprintf(
		"⚔️ %v (UID %d) challenges you to an INSULT DUEL! "+
			"Type /insultsim accept to accept or /insultsim decline to decline. "+
			"You have 30 seconds.",
		challengerName, challengerUID,
	))
	client.SendServerMessage(fmt.Sprintf(
		"⚔️ Challenge sent to %v (UID %d). Waiting for their response...",
		targetName, targetUID,
	))
	addToBuffer(client, "INSULTSIM",
		fmt.Sprintf("Challenged UID %d (%v) to an insult duel", targetUID, targetName), false)

	// time.AfterFunc fires the expiry callback without holding a goroutine
	// open for the full 30-second window.
	time.AfterFunc(isimChallengeTimeout, func() {
		isimExpireChallenge(challengerUID, targetUID, challengerName, targetName)
	})
}

// isimExpireChallenge is scheduled by time.AfterFunc; it runs in its own
// short-lived goroutine only after the timeout fires (no goroutine is held
// open during the wait).
func isimExpireChallenge(challengerUID, targetUID int, challengerName, targetName string) {
	isimGlobal.mu.Lock()
	if cUID, ok := isimGlobal.pendingChallenges[targetUID]; !ok || cUID != challengerUID {
		isimGlobal.mu.Unlock()
		return
	}
	delete(isimGlobal.pendingChallenges, targetUID)
	delete(isimGlobal.challengerBusy, challengerUID)
	isimGlobal.mu.Unlock()

	if challenger, err := getClientByUid(challengerUID); err == nil {
		challenger.SendServerMessage(fmt.Sprintf(
			"⌛ Your insult duel challenge to %v (UID %d) expired.", targetName, targetUID,
		))
	}
	if target, err := getClientByUid(targetUID); err == nil {
		target.SendServerMessage(fmt.Sprintf(
			"⌛ The insult duel challenge from %v (UID %d) expired.", challengerName, challengerUID,
		))
	}
}

// isimAccept is called when a challenged player accepts.
func isimAccept(client *Client) {
	challengedUID := client.Uid()

	isimGlobal.mu.Lock()
	challengerUID, ok := isimGlobal.pendingChallenges[challengedUID]
	if !ok {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You have no pending insult duel challenge.")
		return
	}
	challenger, err := getClientByUid(challengerUID)
	if err != nil {
		delete(isimGlobal.pendingChallenges, challengedUID)
		delete(isimGlobal.challengerBusy, challengerUID)
		isimGlobal.mu.Unlock()
		client.SendServerMessage("The challenger has disconnected. Challenge cancelled.")
		return
	}
	delete(isimGlobal.pendingChallenges, challengedUID)
	delete(isimGlobal.challengerBusy, challengerUID)

	duel := &isimDuel{
		challenger: &isimPlayer{uid: challengerUID, hp: isimStartingHP},
		challenged: &isimPlayer{uid: challengedUID, hp: isimStartingHP},
		round:      1,
	}
	isimGlobal.activeDuels[challengerUID] = duel
	isimGlobal.activeDuels[challengedUID] = duel
	isimGlobal.mu.Unlock()

	challengerName := challenger.OOCName()
	challengedName := client.OOCName()

	sendGlobalServerMessage(fmt.Sprintf(
		"⚔️ INSULT DUEL: %v (UID %d) vs %v (UID %d) — LET THE VERBAL BATTLE BEGIN!",
		challengerName, challengerUID, challengedName, challengedUID,
	))
	addToBuffer(client, "INSULTSIM",
		fmt.Sprintf("Accepted insult duel from UID %d (%v)", challengerUID, challengerName), false)

	// Send rules to both players.
	challenger.SendServerMessage(isimRulesMsg)
	client.SendServerMessage(isimRulesMsg)

	go isimRunRound(duel, challengerName, challengedName)
}

// isimDecline is called when a challenged player declines.
func isimDecline(client *Client) {
	challengedUID := client.Uid()

	isimGlobal.mu.Lock()
	challengerUID, ok := isimGlobal.pendingChallenges[challengedUID]
	if !ok {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You have no pending insult duel challenge.")
		return
	}
	delete(isimGlobal.pendingChallenges, challengedUID)
	delete(isimGlobal.challengerBusy, challengerUID)
	isimGlobal.mu.Unlock()

	challengedName := client.OOCName()
	if challenger, err := getClientByUid(challengerUID); err == nil {
		challenger.SendServerMessage(fmt.Sprintf(
			"😤 %v (UID %d) declined your insult duel challenge. Coward!",
			challengedName, challengedUID,
		))
	}
	client.SendServerMessage("You declined the insult duel challenge.")
	addToBuffer(client, "INSULTSIM",
		fmt.Sprintf("Declined insult duel from UID %d", challengerUID), false)
}

// ── Pick ─────────────────────────────────────────────────────────────────────

// isimPick records a player's fragment selections for the current round.
func isimPick(client *Client, args []string) {
	uid := client.Uid()

	isimGlobal.mu.Lock()
	duel, ok := isimGlobal.activeDuels[uid]
	if !ok {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You are not in an insult duel right now.")
		return
	}

	player := isimDuelPlayer(duel, uid)
	if player == nil {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You are not a participant in this duel.")
		return
	}

	if player.picked {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("You have already submitted your picks for this round.")
		return
	}

	if len(player.fragments) == 0 {
		isimGlobal.mu.Unlock()
		client.SendServerMessage("No fragments have been dealt yet. Please wait for the round to start.")
		return
	}

	if len(args) == 0 {
		isimGlobal.mu.Unlock()
		client.SendServerMessage(fmt.Sprintf(
			"Please provide at least one fragment number (1-%d). E.g. /insultsim pick 1 3",
			isimFragmentsPerRound))
		return
	}

	// Parse and validate picks.
	var picks []int
	for _, a := range args {
		n, err := strconv.Atoi(a)
		if err != nil || n < 1 || n > isimFragmentsPerRound {
			isimGlobal.mu.Unlock()
			client.SendServerMessage(fmt.Sprintf(
				"Invalid pick %q. Numbers must be between 1 and %d.", a, isimFragmentsPerRound))
			return
		}
		picks = append(picks, n)
	}
	if len(picks) > isimMaxPicksPerRound {
		picks = picks[:isimMaxPicksPerRound]
	}

	// Deduplicate picks using a tiny stack-allocated bool array; picks are
	// bounded to [1, isimFragmentsPerRound] so no heap allocation is needed.
	var seen [isimFragmentsPerRound + 1]bool
	deduped := picks[:0]
	for _, p := range picks {
		if !seen[p] {
			seen[p] = true
			deduped = append(deduped, p)
		}
	}
	picks = deduped

	player.picks = picks
	player.picked = true

	// Check if both players have picked.
	other := isimOtherPlayer(duel, uid)
	bothPicked := other.picked
	pickDone := duel.pickDone
	isimGlobal.mu.Unlock()

	client.SendServerMessage(fmt.Sprintf("⚔️ Picks recorded: %v. Waiting for your opponent...", picks))

	if bothPicked {
		// Signal isimRunRound to resolve immediately rather than waiting for
		// the timeout goroutine.
		select {
		case pickDone <- struct{}{}:
		default:
		}
	}
}

// ── Round lifecycle ──────────────────────────────────────────────────────────

// isimRunRound deals fragments and starts the pick timer for the given round.
func isimRunRound(duel *isimDuel, challengerName, challengedName string) {
	isimGlobal.mu.Lock()
	if duel.resolved {
		isimGlobal.mu.Unlock()
		return
	}
	round := duel.round
	// Reset picks for this round.
	duel.challenger.fragments = isimPickFragments()
	duel.challenger.picks = nil
	duel.challenger.picked = false
	duel.challenged.fragments = isimPickFragments()
	duel.challenged.picks = nil
	duel.challenged.picked = false
	// Fresh buffered channel for this round's early-completion signal.
	duel.pickDone = make(chan struct{}, 1)
	challengerUID := duel.challenger.uid
	challengedUID := duel.challenged.uid
	challengerFrags := duel.challenger.fragments
	challengedFrags := duel.challenged.fragments
	pickDone := duel.pickDone
	isimGlobal.mu.Unlock()

	// Send fragments to each player privately.
	if c, err := getClientByUid(challengerUID); err == nil {
		c.SendServerMessage(fmt.Sprintf("⚔️ ROUND %d — %s\n%s", round, challengerName, isimFragmentListMessage(challengerFrags)))
	}
	if c, err := getClientByUid(challengedUID); err == nil {
		c.SendServerMessage(fmt.Sprintf("⚔️ ROUND %d — %s\n%s", round, challengedName, isimFragmentListMessage(challengedFrags)))
	}

	// Wait for both players to pick or for the per-round timeout.
	// Using time.NewTimer lets us stop the timer immediately when both pick,
	// so the round resolves without waiting the full 45 seconds.
	timer := time.NewTimer(isimPickTimeout)
	select {
	case <-pickDone:
		// Stop the timer and drain its channel in case it fired concurrently.
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	case <-timer.C:
		// Force any player who has not yet picked.
		isimGlobal.mu.Lock()
		if !duel.challenger.picked {
			duel.challenger.picked = true
		}
		if !duel.challenged.picked {
			duel.challenged.picked = true
		}
		isimGlobal.mu.Unlock()
	}

	isimResolveRound(duel)
}

// isimResolveRound calculates damage for the current round and advances the game.
// Called exclusively from isimRunRound after the pick window closes.
func isimResolveRound(duel *isimDuel) {
	isimGlobal.mu.Lock()
	if duel.resolved {
		isimGlobal.mu.Unlock()
		return
	}
	// Safety: should never be false given the call site, but guard anyway.
	if !duel.challenger.picked || !duel.challenged.picked {
		isimGlobal.mu.Unlock()
		return
	}

	challengerPicks := duel.challenger.picks
	challengedPicks := duel.challenged.picks
	challengerFrags := duel.challenger.fragments
	challengedFrags := duel.challenged.fragments
	challengerUID := duel.challenger.uid
	challengedUID := duel.challenged.uid
	round := duel.round

	challengerDmg := isimCalculateDamage(len(challengerPicks))
	challengedDmg := isimCalculateDamage(len(challengedPicks))

	duel.challenged.hp -= challengerDmg
	duel.challenger.hp -= challengedDmg

	challengerHP := duel.challenger.hp
	challengedHP := duel.challenged.hp

	// Assemble insult strings.
	challengerInsult := isimAssembleInsult(challengerFrags, challengerPicks)
	challengedInsult := isimAssembleInsult(challengedFrags, challengedPicks)

	// Check for end-of-game.
	gameOver := challengerHP <= 0 || challengedHP <= 0
	if gameOver {
		duel.resolved = true
		delete(isimGlobal.activeDuels, challengerUID)
		delete(isimGlobal.activeDuels, challengedUID)
	} else {
		duel.round++
	}
	isimGlobal.mu.Unlock()

	// Announce round results globally.
	sendGlobalServerMessage(fmt.Sprintf(
		"⚔️ ROUND %d RESULTS:\n"+
			"💬 [UID %d]: \"%s\" → %d damage! (HP: %d)\n"+
			"💬 [UID %d]: \"%s\" → %d damage! (HP: %d)",
		round,
		challengerUID, challengerInsult, challengerDmg, challengedHP,
		challengedUID, challengedInsult, challengedDmg, challengerHP,
	))

	if gameOver {
		isimEndGame(duel, challengerUID, challengedUID, challengerHP, challengedHP)
		return
	}

	// Retrieve updated names for next round.
	challengerName := fmt.Sprintf("UID %d", challengerUID)
	if c, err := getClientByUid(challengerUID); err == nil {
		challengerName = c.OOCName()
	}
	challengedName := fmt.Sprintf("UID %d", challengedUID)
	if c, err := getClientByUid(challengedUID); err == nil {
		challengedName = c.OOCName()
	}

	go isimRunRound(duel, challengerName, challengedName)
}

// isimAssembleInsult builds a readable insult string from the player's fragment picks.
func isimAssembleInsult(fragments []string, picks []int) string {
	if len(picks) == 0 {
		return "..."
	}
	parts := make([]string, 0, len(picks))
	for _, p := range picks {
		if p >= 1 && p <= len(fragments) {
			parts = append(parts, fragments[p-1])
		}
	}
	if len(parts) == 0 {
		return "..."
	}
	return strings.Join(parts, ", ")
}

// isimEndGame announces the winner/loser and applies the loser's punishment.
func isimEndGame(duel *isimDuel, challengerUID, challengedUID, challengerHP, challengedHP int) {
	challenger, cErr := getClientByUid(challengerUID)
	challenged, dErr := getClientByUid(challengedUID)

	challengerName := fmt.Sprintf("UID %d", challengerUID)
	if cErr == nil {
		challengerName = challenger.OOCName()
	}
	challengedName := fmt.Sprintf("UID %d", challengedUID)
	if dErr == nil {
		challengedName = challenged.OOCName()
	}

	switch {
	case challengerHP <= 0 && challengedHP <= 0:
		// Both reached 0 — mutual KO.
		sendGlobalServerMessage(fmt.Sprintf(
			"⚔️ INSULT DUEL RESULT: MUTUAL KO! Both %v and %v destroyed each other! Both receive a punishment!",
			challengerName, challengedName,
		))
		for _, pair := range []struct {
			client *Client
			err    error
		}{{challenger, cErr}, {challenged, dErr}} {
			if pair.err == nil {
				pType := randomIsimPunishment()
				pair.client.AddPunishment(pType, isimPunishDuration, "Insult Duel: mutual KO")
				pair.client.SendServerMessage(fmt.Sprintf(
					"💀 MUTUAL KO! You both lost! Punished with '%v' for %v.", pType, isimPunishDuration,
				))
			}
		}
	case challengerHP <= 0:
		// Challenger lost.
		applyIsimOutcome(challengerName, challengedName, challengerUID, challengedUID,
			challenger, challenged, cErr)
	default:
		// Challenged lost.
		applyIsimOutcome(challengedName, challengerName, challengedUID, challengerUID,
			challenged, challenger, dErr)
	}
}

// applyIsimOutcome announces the winner and punishes the loser.
// winnerUID is not directly used here because the winner is identified by
// the winner *Client pointer; it is kept in the signature for call-site clarity.
func applyIsimOutcome(
	loserName, winnerName string,
	loserUID, winnerUID int,
	loser, winner *Client,
	loserErr error,
) {
	_ = winnerUID // winner is accessed via the *Client pointer, not the UID
	pType := randomIsimPunishment()
	sendGlobalServerMessage(fmt.Sprintf(
		"🏆 INSULT DUEL WINNER: %v destroyed %v with their razor-sharp wit! %v receives a punishment!",
		winnerName, loserName, loserName,
	))
	if winner != nil {
		winner.SendServerMessage("🏆 You won the Insult Duel! Your tongue is truly a weapon.")
		addToBuffer(winner, "INSULTSIM",
			fmt.Sprintf("Won insult duel vs UID %d (%v), loser punished with %v",
				loserUID, loserName, pType), false)
	}
	if loserErr == nil {
		loser.AddPunishment(pType, isimPunishDuration, "Insult Duel: loser")
		loser.SendServerMessage(fmt.Sprintf(
			"💀 You lost the Insult Duel! Punished with '%v' for %v.", pType, isimPunishDuration,
		))
	}
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// isimDuelPlayer returns the isimPlayer record for the given UID within a duel,
// or nil if that UID is not a participant.
func isimDuelPlayer(duel *isimDuel, uid int) *isimPlayer {
	if duel.challenger.uid == uid {
		return duel.challenger
	}
	if duel.challenged.uid == uid {
		return duel.challenged
	}
	return nil
}

// isimOtherPlayer returns the opponent's isimPlayer record for the given UID.
func isimOtherPlayer(duel *isimDuel, uid int) *isimPlayer {
	if duel.challenger.uid == uid {
		return duel.challenged
	}
	return duel.challenger
}
