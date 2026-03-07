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

// ── Timing constants ─────────────────────────────────────────────────────────

const (
	quickdrawChallengeTimeout = 30 * time.Second  // window to accept a challenge
	quickdrawReactionTimeout  = 10 * time.Second  // time to react after DRAW!
	quickdrawPunishDuration   = 10 * time.Minute  // how long the loser's punishment lasts
)

// ── Punishment pool ───────────────────────────────────────────────────────────

// quickdrawPunishmentPool is the set of punishments that may be applied to the
// loser of a quickdraw duel.
var quickdrawPunishmentPool = []PunishmentType{
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
	PunishmentSubtitles,
}

// randomQuickdrawPunishment returns a random punishment from the pool.
func randomQuickdrawPunishment() PunishmentType {
	return quickdrawPunishmentPool[rand.Intn(len(quickdrawPunishmentPool))]
}

// ── Duel state ────────────────────────────────────────────────────────────────

// quickdrawDuel represents an active duel between two players.
type quickdrawDuel struct {
	challengerUID int
	challengedUID int
	drawSignaled  bool // true after "DRAW!" is announced
	resolved      bool // true once the outcome has been determined
	winnerUID     int  // UID of the first responder; -1 until set
}

// quickdrawState holds the mutex-protected global state for all quickdraw duels.
type quickdrawState struct {
	mu                sync.Mutex
	pendingChallenges map[int]int           // challenged UID → challenger UID
	activeDuels       map[int]*quickdrawDuel // UID → duel (both parties share the same pointer)
}

var qdState = quickdrawState{
	pendingChallenges: make(map[int]int),
	activeDuels:       make(map[int]*quickdrawDuel),
}

// ── Command entry point ───────────────────────────────────────────────────────

// cmdQuickdraw is the entry point for /quickdraw.
// Usage:
//
//	/quickdraw <uid>    – challenge a player to a duel
//	/quickdraw accept   – accept a pending challenge
//	/quickdraw decline  – decline a pending challenge
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

// ── Challenge ─────────────────────────────────────────────────────────────────

// quickdrawChallenge sends a challenge from client to the player with targetUID.
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

	// Reject if the challenger is already busy.
	if _, inDuel := qdState.activeDuels[challengerUID]; inDuel {
		qdState.mu.Unlock()
		client.SendServerMessage("You are already in a quickdraw duel.")
		return
	}
	for _, cUID := range qdState.pendingChallenges {
		if cUID == challengerUID {
			qdState.mu.Unlock()
			client.SendServerMessage("You already have a pending quickdraw challenge.")
			return
		}
	}

	// Reject if the target is already busy.
	if _, inDuel := qdState.activeDuels[targetUID]; inDuel {
		qdState.mu.Unlock()
		client.SendServerMessage(fmt.Sprintf("UID %d is already in a quickdraw duel.", targetUID))
		return
	}
	if _, challenged := qdState.pendingChallenges[targetUID]; challenged {
		qdState.mu.Unlock()
		client.SendServerMessage(fmt.Sprintf("UID %d already has a pending quickdraw challenge.", targetUID))
		return
	}

	qdState.pendingChallenges[targetUID] = challengerUID
	qdState.mu.Unlock()

	// Notify both parties — no lock held.
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
		// Challenge was already accepted or declined.
		qdState.mu.Unlock()
		return
	}
	delete(qdState.pendingChallenges, targetUID)
	qdState.mu.Unlock()

	// Notify both parties that the challenge expired.
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

// ── Accept ────────────────────────────────────────────────────────────────────

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

	// Validate that the challenger is still online before creating the duel.
	challenger, err := getClientByUid(challengerUID)
	if err != nil {
		delete(qdState.pendingChallenges, challengedUID)
		qdState.mu.Unlock()
		client.SendServerMessage("The challenger has disconnected. Challenge cancelled.")
		return
	}

	// Remove the pending challenge and create the active duel.
	delete(qdState.pendingChallenges, challengedUID)
	duel := &quickdrawDuel{
		challengerUID: challengerUID,
		challengedUID: challengedUID,
		winnerUID:     -1,
	}
	qdState.activeDuels[challengerUID] = duel
	qdState.activeDuels[challengedUID] = duel
	qdState.mu.Unlock()

	// Announce the duel globally — no lock held.
	challengerName := challenger.OOCName()
	challengedName := client.OOCName()

	sendGlobalServerMessage(fmt.Sprintf(
		"🔫 QUICKDRAW DUEL: %v (UID %d) vs %v (UID %d)! Countdown starting...",
		challengerName, challengerUID, challengedName, challengedUID,
	))
	addToBuffer(client, "QUICKDRAW",
		fmt.Sprintf("Accepted quickdraw challenge from UID %d (%v)", challengerUID, challengerName), false)

	go quickdrawCountdown(duel, challengerName, challengedName)
}

// ── Decline ───────────────────────────────────────────────────────────────────

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
	qdState.mu.Unlock()

	// Notify both parties — no lock held.
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

// ── Countdown ─────────────────────────────────────────────────────────────────

// quickdrawCountdown runs the pre-draw countdown and then signals DRAW!
// It is always called in its own goroutine.
func quickdrawCountdown(duel *quickdrawDuel, challengerName, challengedName string) {
	sendGlobalServerMessage("3...")
	time.Sleep(1 * time.Second)
	sendGlobalServerMessage("2...")
	time.Sleep(1 * time.Second)
	sendGlobalServerMessage("1...")
	time.Sleep(1 * time.Second)

	qdState.mu.Lock()
	// Abort if the duel was already resolved (e.g. a participant disconnected).
	if duel.resolved {
		qdState.mu.Unlock()
		return
	}
	duel.drawSignaled = true
	qdState.mu.Unlock()

	sendGlobalServerMessage("🔫 DRAW! Send an IC message — the first to react wins!")

	go quickdrawReactionTimer(duel, challengerName, challengedName)
}

// ── Reaction timer ────────────────────────────────────────────────────────────

// quickdrawReactionTimer waits for the reaction window; if neither duelist has
// responded, it resolves the duel with both players losing.
func quickdrawReactionTimer(duel *quickdrawDuel, challengerName, challengedName string) {
	time.Sleep(quickdrawReactionTimeout)

	qdState.mu.Lock()
	if duel.resolved {
		qdState.mu.Unlock()
		return
	}
	duel.resolved = true
	// Remove both UIDs from activeDuels.
	delete(qdState.activeDuels, duel.challengerUID)
	delete(qdState.activeDuels, duel.challengedUID)
	qdState.mu.Unlock()

	// Both were too slow — punish both.
	punishBoth := func() {
		for _, uid := range []int{duel.challengerUID, duel.challengedUID} {
			if c, err := getClientByUid(uid); err == nil {
				pType := randomQuickdrawPunishment()
				c.AddPunishment(pType, quickdrawPunishDuration, "Quickdraw: too slow")
				c.SendServerMessage(fmt.Sprintf(
					"🐢 You were too slow! Punished with '%v' for %v.", pType, quickdrawPunishDuration,
				))
			}
		}
	}
	punishBoth()
	sendGlobalServerMessage(fmt.Sprintf(
		"😴 QUICKDRAW RESULT: Both %v and %v were too slow! Both receive a punishment!",
		challengerName, challengedName,
	))
}

// ── IC hook ───────────────────────────────────────────────────────────────────

// quickdrawOnIC is called from pktIC whenever a client sends an IC message.
// It checks whether this client is in an active duel that has reached the DRAW
// signal, and if so, records them as the winner and resolves the duel.
func quickdrawOnIC(client *Client) {
	uid := client.Uid()

	qdState.mu.Lock()
	duel, ok := qdState.activeDuels[uid]
	if !ok || !duel.drawSignaled || duel.resolved {
		qdState.mu.Unlock()
		return
	}

	// This is the first responder — they win.
	duel.resolved = true
	duel.winnerUID = uid
	delete(qdState.activeDuels, duel.challengerUID)
	delete(qdState.activeDuels, duel.challengedUID)

	loserUID := duel.challengedUID
	if uid == duel.challengedUID {
		loserUID = duel.challengerUID
	}
	qdState.mu.Unlock()

	// Resolve outside the lock.
	quickdrawResolve(uid, loserUID)
}

// quickdrawResolve applies the punishment to the loser and announces the outcome.
// It is always called with no locks held.
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
			"💀 You lost the quickdraw duel! Punished with '%v' for %v.",
			pType, quickdrawPunishDuration,
		))
		sendGlobalServerMessage(fmt.Sprintf(
			"🏆 QUICKDRAW RESULT: %v was faster! %v loses and receives '%v'!",
			winnerName, loserName, pType,
		))
		if winner != nil {
			winner.SendServerMessage("🏆 You won the quickdraw duel! Nice shot!")
			addToBuffer(winner, "QUICKDRAW",
				fmt.Sprintf("Won duel vs UID %d (%v), loser punished with %v", loserUID, loserName, pType), false)
		}
	} else {
		// Loser disconnected.
		sendGlobalServerMessage(fmt.Sprintf(
			"🏆 QUICKDRAW RESULT: %v wins! Their opponent disconnected.",
			winnerName,
		))
		if winner != nil {
			winner.SendServerMessage("🏆 You won the quickdraw duel — your opponent left the server!")
		}
	}
}
