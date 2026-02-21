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
	giveawayDuration = 10 * time.Minute // how long the giveaway runs
	giveawayCooldown = 10 * time.Minute // global delay between giveaways
	giveawayReminder = 9 * time.Minute  // send reminder when 1 minute remains
)

// ── State ────────────────────────────────────────────────────────────────────

// giveawayState holds the mutex-protected lifecycle state of an active giveaway.
// State mutation happens under the mutex; all I/O is performed after the lock
// has been released.
type giveawayState struct {
	mu       sync.Mutex
	active   bool
	item     string
	hostUID  int
	hostName string          // showname or OOC name of the host
	entrants map[int]struct{} // set of opted-in UIDs
	lastEnd  time.Time        // when the last giveaway ended (drives the cooldown)
}

var giveaway = giveawayState{
	entrants: make(map[int]struct{}),
	hostUID:  -1,
}

// ── Cooldown helper ──────────────────────────────────────────────────────────

// isGiveawayCoolingDown reports whether the global cooldown is in effect and
// how many whole seconds remain (0 when not cooling down).
func isGiveawayCoolingDown() (bool, int) {
	giveaway.mu.Lock()
	end := giveaway.lastEnd
	giveaway.mu.Unlock()

	if end.IsZero() {
		return false, 0
	}
	if remaining := giveawayCooldown - time.Since(end); remaining > 0 {
		return true, int(remaining.Seconds()) + 1
	}
	return false, 0
}

// ── Command entry point ──────────────────────────────────────────────────────

// cmdGiveaway is the entry point for /giveaway start <item> and /giveaway enter.
func cmdGiveaway(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage(usage)
		return
	}
	switch args[0] {
	case "start":
		if len(args) < 2 {
			client.SendServerMessage(usage)
			return
		}
		giveawayStart(client, strings.Join(args[1:], " "))
	case "enter":
		giveawayEnter(client)
	default:
		client.SendServerMessage(usage)
	}
}

// ── Start ────────────────────────────────────────────────────────────────────

// giveawayStart validates preconditions and opens a new giveaway.
// Client fields are read before acquiring giveaway.mu to minimise lock duration
// and avoid holding two locks (client.mu + giveaway.mu) simultaneously.
// State is mutated under the lock; all I/O follows after the lock is released.
func giveawayStart(client *Client, item string) {
	// Read client fields outside giveaway.mu to keep the critical section short.
	uid := client.Uid()
	hostName := client.Showname()
	if hostName == "" {
		hostName = client.OOCName()
	}

	giveaway.mu.Lock()

	if giveaway.active {
		giveaway.mu.Unlock()
		client.SendServerMessage("A giveaway is already in progress.")
		return
	}

	if !giveaway.lastEnd.IsZero() {
		if remaining := giveawayCooldown - time.Since(giveaway.lastEnd); remaining > 0 {
			giveaway.mu.Unlock()
			client.SendServerMessage(fmt.Sprintf("Giveaway is on cooldown. Please wait %d seconds.", int(remaining.Seconds())+1))
			return
		}
	}

	giveaway.active = true
	giveaway.item = item
	giveaway.hostUID = uid
	giveaway.hostName = hostName
	giveaway.entrants = make(map[int]struct{})
	giveaway.mu.Unlock()

	// All I/O after the lock is released.
	sendGlobalServerMessage(fmt.Sprintf(
		"🎁 GIVEAWAY STARTED by %v! They are giving away: %v\n"+
			"Type /giveaway enter to join! You have 10 minutes. Good luck!",
		hostName, item,
	))
	addToBuffer(client, "CMD", fmt.Sprintf("Started giveaway for: %v", item), false)
	go giveawayTimer(item, hostName)
}

// ── Enter ────────────────────────────────────────────────────────────────────

// giveawayEnter records a player's entry in the active giveaway.
// The client UID is read before acquiring giveaway.mu to avoid holding two
// locks simultaneously. The lock is held only for state mutation; messages
// are sent after release.
func giveawayEnter(client *Client) {
	uid := client.Uid() // read before acquiring giveaway.mu

	giveaway.mu.Lock()

	if !giveaway.active {
		giveaway.mu.Unlock()
		client.SendServerMessage("There is no active giveaway to enter right now.")
		return
	}

	if _, already := giveaway.entrants[uid]; already {
		giveaway.mu.Unlock()
		client.SendServerMessage("You have already entered the giveaway.")
		return
	}

	giveaway.entrants[uid] = struct{}{}
	count := len(giveaway.entrants)
	giveaway.mu.Unlock()

	// I/O after the lock is released.
	client.SendServerMessage(fmt.Sprintf("🎁 You have entered the giveaway! (%d entrant(s) so far)", count))
	sendGlobalServerMessage(fmt.Sprintf("🎁 %v entered the giveaway! (%d entrant(s))", client.OOCName(), count))
}

// ── Background timer ─────────────────────────────────────────────────────────

// giveawayTimer manages the giveaway lifecycle using two independent timers
// started at the same instant, so the giveaway always ends exactly
// giveawayDuration after it starts regardless of reminder-processing time.
// defer end.Stop() releases the end timer's resources on any early return.
func giveawayTimer(item, hostName string) {
	reminder := time.NewTimer(giveawayReminder)
	end := time.NewTimer(giveawayDuration)
	defer end.Stop()

	// ── Reminder ──────────────────────────────────────────────────────────────
	<-reminder.C

	giveaway.mu.Lock()
	if !giveaway.active {
		giveaway.mu.Unlock()
		return
	}
	count := len(giveaway.entrants)
	giveaway.mu.Unlock()

	sendGlobalServerMessage(fmt.Sprintf(
		"🎁 GIVEAWAY REMINDER: 1 minute left to enter! %v is giving away: %v (%d entrant(s) so far)\n"+
			"Type /giveaway enter to join!",
		hostName, item, count,
	))

	// ── End ───────────────────────────────────────────────────────────────────
	<-end.C

	// Atomically close the giveaway and snapshot entrant UIDs.
	giveaway.mu.Lock()
	if !giveaway.active {
		giveaway.mu.Unlock()
		return
	}
	giveaway.active = false
	giveaway.lastEnd = time.Now().UTC()
	uids := make([]int, 0, len(giveaway.entrants))
	for uid := range giveaway.entrants {
		uids = append(uids, uid)
	}
	giveaway.mu.Unlock()

	// Filter disconnected players in-place — avoids a second heap allocation.
	n := 0
	for _, uid := range uids {
		if _, err := getClientByUid(uid); err == nil {
			uids[n] = uid
			n++
		}
	}
	uids = uids[:n]

	if n == 0 {
		sendGlobalServerMessage(fmt.Sprintf(
			"🎁 GIVEAWAY ENDED! Nobody entered %v's giveaway for: %v. No winner this time!",
			hostName, item,
		))
		return
	}

	winnerUID := uids[rand.Intn(n)]
	winner, err := getClientByUid(winnerUID)
	if err != nil {
		sendGlobalServerMessage("🎁 GIVEAWAY ENDED! The winner disconnected before they could be announced.")
		return
	}

	winnerName := winner.Showname()
	if winnerName == "" {
		winnerName = winner.OOCName()
	}

	sendGlobalServerMessage(fmt.Sprintf(
		"🎉 GIVEAWAY WINNER! Congratulations to %v (UID: %d)! They won: %v (hosted by %v)",
		winnerName, winnerUID, item, hostName,
	))
	winner.SendServerMessage(fmt.Sprintf("🎉 You won the giveaway for: %v! Congratulations!", item))
}
