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
	"strings"
	"sync"
	"time"
)

// ── Timing constants ─────────────────────────────────────────────────────────

const (
	hotPotatoOptInDuration      = 60 * time.Second // window for /hotpotato accept
	hotPotatoGameDuration       = 5 * time.Minute  // how long the carrier holds the potato
	hotPotatoCooldown           = 5 * time.Minute  // global delay between games
	hotPotatoMinParticipants    = 2                 // minimum opt-ins required to start
	hotPotatoPunishmentDuration = 10 * time.Minute // how long punishments last
	hotPotatoPassCooldown       = 10 * time.Second // minimum delay between passes
)

// hotPotatoRules is broadcast in OOC when a game is announced.
const hotPotatoRules = `🥔 HOT POTATO EVENT STARTING! 🥔
Type /hotpotato accept within 60 seconds to join.

📋 HOW TO PLAY:
• One random participant is secretly given the "Hot Potato".
• The carrier has a 5-minute virtual timer — find other participants!
• AVOID being in the same area as the carrier when the timer runs out!
• When time's up, opted-in players sharing the carrier's area get a random punishment.
• If the carrier is a MODERATOR, those players are KICKED from the server instead.
• If the carrier ends up alone, THEY receive the punishment themselves.
• The carrier can type /hotpotato pass to pass the potato to a random participant (10s cooldown).
• Players who did not opt in are completely safe and unaffected.
• Only one game can run at a time (5-minute cooldown between games).

Good luck — and watch who you hang around with! 🔥`

// ── Punishment pool ──────────────────────────────────────────────────────────

// hotPotatoPunishmentPool is a package-level slice so it is allocated exactly once.
var hotPotatoPunishmentPool = []PunishmentType{
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

// randomHotPotatoPunishment returns a random punishment from the pool.
func randomHotPotatoPunishment() PunishmentType {
	return hotPotatoPunishmentPool[rand.Intn(len(hotPotatoPunishmentPool))]
}

// ── State ────────────────────────────────────────────────────────────────────

// hotPotatoState is the complete, mutex-protected lifecycle state of the game.
// Only state mutation happens under the mutex; all I/O is performed after the
// lock has been released.
type hotPotatoState struct {
	mu           sync.Mutex
	optInActive  bool            // true during the 60-second opt-in window
	gameActive   bool            // true while the 5-minute game is running
	participants map[int]struct{} // set of opted-in UIDs
	carrierUID   int             // UID of the carrier (-1 when no game is active)
	lastGameEnd  time.Time       // when the last game ended (drives the cooldown)
	passLastUsed map[int]time.Time // when each UID last used /hotpotato pass
}

var hotPotato = hotPotatoState{
	participants: make(map[int]struct{}),
	carrierUID:   -1,
	passLastUsed: make(map[int]time.Time),
}

// ── Cooldown helper ──────────────────────────────────────────────────────────

// isHotPotatoCoolingDown reports whether the global cooldown is in effect and
// how many whole seconds remain (0 when not cooling down).
// The lock is held only long enough to read a single value.
func isHotPotatoCoolingDown() (bool, int) {
	hotPotato.mu.Lock()
	end := hotPotato.lastGameEnd
	hotPotato.mu.Unlock()

	if end.IsZero() {
		return false, 0
	}
	if remaining := hotPotatoCooldown - time.Since(end); remaining > 0 {
		return true, int((remaining+time.Second-1)/time.Second)
	}
	return false, 0
}

// ── Command entry point ──────────────────────────────────────────────────────

// cmdHotPotato is the entry point for both /hotpotato (start),
// /hotpotato accept (opt-in), and /hotpotato pass (pass the potato).
func cmdHotPotato(client *Client, args []string, _ string) {
	if len(args) > 0 {
		switch args[0] {
		case "accept":
			hotPotatoAccept(client)
			return
		case "pass":
			hotPotatoPass(client)
			return
		}
	}
	hotPotatoStart(client)
}

// ── Opt-in phase ─────────────────────────────────────────────────────────────

// hotPotatoStart validates preconditions and opens the opt-in window.
// State is mutated under the lock; all I/O follows after the lock is released.
func hotPotatoStart(client *Client) {
	hotPotato.mu.Lock()

	if hotPotato.optInActive || hotPotato.gameActive {
		hotPotato.mu.Unlock()
		client.SendServerMessage("A Hot Potato game is already in progress.")
		return
	}

	if !hotPotato.lastGameEnd.IsZero() {
		if remaining := hotPotatoCooldown - time.Since(hotPotato.lastGameEnd); remaining > 0 {
			hotPotato.mu.Unlock()
			client.SendServerMessage(fmt.Sprintf("Hot Potato is on cooldown. Please wait %d seconds.", int((remaining+time.Second-1)/time.Second)))
			return
		}
	}

	hotPotato.optInActive = true
	hotPotato.participants = make(map[int]struct{})
	hotPotato.carrierUID = -1
	hotPotato.passLastUsed = make(map[int]time.Time)
	hotPotato.mu.Unlock()

	// All I/O after the lock is released.
	sendGlobalServerMessage(hotPotatoRules)
	addToBuffer(client, "CMD", "Started Hot Potato opt-in", false)
	go hotPotatoOptInTimer()
}

// hotPotatoAccept records a player's opt-in during the active window.
// The lock is held only for state mutation; messages are sent after release.
func hotPotatoAccept(client *Client) {
	hotPotato.mu.Lock()

	if !hotPotato.optInActive {
		hotPotato.mu.Unlock()
		client.SendServerMessage("There is no active Hot Potato game to join right now.")
		return
	}

	uid := client.Uid()
	if _, already := hotPotato.participants[uid]; already {
		hotPotato.mu.Unlock()
		client.SendServerMessage("You have already joined the Hot Potato game.")
		return
	}

	hotPotato.participants[uid] = struct{}{}
	count := len(hotPotato.participants)
	hotPotato.mu.Unlock()

	// I/O after the lock is released.
	client.SendServerMessage(fmt.Sprintf("🥔 You have joined the Hot Potato game! (%d participant(s) so far)", count))
	sendGlobalServerMessage(fmt.Sprintf("🥔 %v joined Hot Potato! (%d participant(s))", client.OOCName(), count))
}

// ── Pass ─────────────────────────────────────────────────────────────────────

// hotPotatoPass allows the current carrier to pass the potato to a random
// other participant. The carrier must wait hotPotatoPassCooldown (10 s) between
// consecutive passes. The new carrier is chosen at random from the set of
// opted-in UIDs that are still connected.
func hotPotatoPass(client *Client) {
	uid := client.Uid()

	hotPotato.mu.Lock()

	if !hotPotato.gameActive {
		hotPotato.mu.Unlock()
		client.SendServerMessage("There is no active Hot Potato game right now.")
		return
	}

	if hotPotato.carrierUID != uid {
		hotPotato.mu.Unlock()
		client.SendServerMessage("You are not holding the Hot Potato.")
		return
	}

	// Enforce per-carrier pass cooldown.
	if last, ok := hotPotato.passLastUsed[uid]; ok {
		if elapsed := time.Since(last); elapsed < hotPotatoPassCooldown {
			remaining := hotPotatoPassCooldown - elapsed
			hotPotato.mu.Unlock()
			client.SendServerMessage(fmt.Sprintf("You must wait %d more second(s) before passing again.", int((remaining+time.Second-1)/time.Second)))
			return
		}
	}

	// Snapshot other participants under the lock; filter connectivity outside it.
	others := make([]int, 0, len(hotPotato.participants)-1)
	for p := range hotPotato.participants {
		if p != uid {
			others = append(others, p)
		}
	}
	hotPotato.mu.Unlock()

	// Filter in-place to still-connected participants.
	n := 0
	for _, p := range others {
		if _, err := getClientByUid(p); err == nil {
			others[n] = p
			n++
		}
	}
	if n == 0 {
		client.SendServerMessage("There are no other connected participants to pass to.")
		return
	}

	newCarrierUID := others[rand.Intn(n)]

	// Record the pass and update the carrier — under the lock.
	hotPotato.mu.Lock()
	hotPotato.passLastUsed[uid] = time.Now()
	hotPotato.carrierUID = newCarrierUID
	hotPotato.mu.Unlock()

	// Notify the new carrier and announce globally.
	if newCarrier, err := getClientByUid(newCarrierUID); err == nil {
		newCarrier.SendServerMessage("🥔🔥 The Hot Potato has been passed to YOU! You have it now — run!")
	}
	sendGlobalServerMessage("🥔 The Hot Potato has been passed to a new carrier! Who has it now…?")
	addToBuffer(client, "HOTPOTATO",
		fmt.Sprintf("Passed potato from UID %d to UID %d", uid, newCarrierUID), false)
}

// ── Background timers ────────────────────────────────────────────────────────

// hotPotatoOptInTimer sleeps for the opt-in window, then either launches the
// game or cancels it with an informative OOC message.
func hotPotatoOptInTimer() {
	time.Sleep(hotPotatoOptInDuration)

	// Snapshot participant UIDs and close the opt-in window — under the lock.
	hotPotato.mu.Lock()
	if !hotPotato.optInActive {
		hotPotato.mu.Unlock() // cancelled externally
		return
	}
	hotPotato.optInActive = false
	uids := make([]int, 0, len(hotPotato.participants))
	for uid := range hotPotato.participants {
		uids = append(uids, uid)
	}
	hotPotato.mu.Unlock()

	// Filter in-place to still-connected players — outside the lock so
	// getClientByUid does not run while hotPotato.mu is held.
	n := 0
	for _, uid := range uids {
		if _, err := getClientByUid(uid); err == nil {
			uids[n] = uid
			n++
		}
	}
	validUIDs := uids[:n]

	if len(validUIDs) < hotPotatoMinParticipants {
		hotPotato.mu.Lock()
		hotPotato.lastGameEnd = time.Now().UTC()
		hotPotato.mu.Unlock()
		sendGlobalServerMessage(fmt.Sprintf(
			"🥔 Hot Potato cancelled — not enough participants (%d/%d required).",
			len(validUIDs), hotPotatoMinParticipants,
		))
		return
	}

	// Pick the carrier and arm the game — under the lock.
	carrierUID := validUIDs[rand.Intn(len(validUIDs))]
	hotPotato.mu.Lock()
	hotPotato.carrierUID = carrierUID
	hotPotato.gameActive = true
	hotPotato.mu.Unlock()

	// Announce start and DM the carrier — no lock held.
	sendGlobalServerMessage(fmt.Sprintf(
		"🔥 THE HOT POTATO GAME HAS BEGUN! %d players are in. "+
			"One of them is carrying the Hot Potato… "+
			"Avoid anyone suspicious for the next 5 minutes!",
		len(validUIDs),
	))
	if carrier, err := getClientByUid(carrierUID); err == nil {
		carrier.SendServerMessage(
			"🥔🔥 YOU have the Hot Potato! " +
				"Be in the same area as other participants when the timer expires. " +
				"You have 5 minutes!",
		)
	}

	go hotPotatoGameTimer()
}

// hotPotatoGameTimer sleeps for the game duration, then hands off to
// hotPotatoResolve for outcome resolution. The carrier is read from state at
// resolution time so any passes made during the game are honoured.
func hotPotatoGameTimer() {
	time.Sleep(hotPotatoGameDuration)

	// Atomically close the game and snapshot the current carrier and participant UIDs.
	hotPotato.mu.Lock()
	if !hotPotato.gameActive {
		hotPotato.mu.Unlock() // already resolved
		return
	}
	hotPotato.gameActive = false
	hotPotato.optInActive = false
	hotPotato.lastGameEnd = time.Now().UTC()
	currentCarrierUID := hotPotato.carrierUID
	participantUIDs := make([]int, 0, len(hotPotato.participants))
	for uid := range hotPotato.participants {
		participantUIDs = append(participantUIDs, uid)
	}
	hotPotato.mu.Unlock()

	hotPotatoResolve(currentCarrierUID, participantUIDs)
}

// ── Resolution ───────────────────────────────────────────────────────────────

// hotPotatoResolve determines who was caught and applies consequences.
// It is always called with no locks held so all network I/O is safe.
func hotPotatoResolve(carrierUID int, participantUIDs []int) {
	carrier, err := getClientByUid(carrierUID)
	if err != nil {
		// Carrier disconnected before the timer fired — nothing to resolve.
		sendGlobalServerMessage("⏰ HOT POTATO TIMER EXPIRED! The carrier left the server — no outcome this round.")
		return
	}

	// Find opted-in players who share the carrier's current area.
	carrierArea := carrier.Area()
	var affected []*Client
	for _, uid := range participantUIDs {
		if uid == carrierUID {
			continue
		}
		if c, err := getClientByUid(uid); err == nil && c.Area() == carrierArea {
			affected = append(affected, c)
		}
	}

	if len(affected) == 0 {
		// Carrier was alone — they bear the punishment themselves.
		pType := randomHotPotatoPunishment()
		carrier.AddPunishment(pType, hotPotatoPunishmentDuration, "Hot Potato: solo carrier penalty")
		carrier.SendServerMessage(fmt.Sprintf(
			"💀 You had the Hot Potato and nobody was nearby — punished with '%v'!", pType))
		sendGlobalServerMessage("⏰ HOT POTATO TIMER EXPIRED! The carrier was alone — they get punished! 🥔💀")
		addToBuffer(carrier, "HOTPOTATO",
			fmt.Sprintf("Carrier self-punished with %v (no victims)", pType), false)
		return
	}

	if carrier.Authenticated() {
		// Mod carrier — kick every caught participant.
		uids := make([]string, len(affected))
		for i, c := range affected {
			uids[i] = fmt.Sprintf("%d", c.Uid())
			c.SendPacketSync("KK", "Hot Potato: caught in the same area as a moderator carrying the Hot Potato!")
			c.conn.Close()
		}
		sendGlobalServerMessage(fmt.Sprintf(
			"⏰ HOT POTATO TIMER EXPIRED! The carrier was a MODERATOR — %d participant(s) are being KICKED! 🔨",
			len(affected),
		))
		addToBuffer(carrier, "HOTPOTATO",
			fmt.Sprintf("Mod carrier kicked UIDs: %s", strings.Join(uids, ", ")), false)
		return
	}

	// Normal carrier — random punishment for every caught participant.
	victims := make([]string, len(affected))
	for i, c := range affected {
		pType := randomHotPotatoPunishment()
		c.AddPunishment(pType, hotPotatoPunishmentDuration, "Hot Potato punishment")
		c.SendServerMessage(fmt.Sprintf(
			"💥 Caught with the Hot Potato carrier! Punished with '%v' for 10 minutes.", pType))
		victims[i] = fmt.Sprintf("%d(%v)", c.Uid(), pType)
	}
	sendGlobalServerMessage(fmt.Sprintf(
		"⏰ HOT POTATO TIMER EXPIRED! %d participant(s) were caught and received random punishments! 🥔💥",
		len(affected),
	))
	addToBuffer(carrier, "HOTPOTATO",
		fmt.Sprintf("Punished UIDs: %s", strings.Join(victims, ", ")), false)
}
