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

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// ============================================================
// /rob — Heist mini-game
// ============================================================

// robCooldown is the mandatory wait between /rob attempts per player.
const robCooldown = 20 * time.Minute

// robMinBalance is the minimum chip balance required to attempt a rob.
// Prevents players with nothing to lose from spamming the command.
const robMinBalance = 100

var (
	robMu       sync.Mutex
	robLastTime = make(map[string]time.Time) // ipid → last successful attempt time
)

// robTargetName converts a user-supplied keyword into a flavour location name.
func robTargetName(target string) string {
	switch strings.ToLower(target) {
	case "bank":
		return "First National Bank"
	case "casino":
		return "casino vault"
	case "vault":
		return "high-security vault"
	case "atm":
		return "the ATM"
	case "store", "shop":
		return "corner store"
	case "mint":
		return "national mint"
	case "armored", "truck", "armored truck":
		return "armored truck"
	case "museum":
		return "museum exhibit"
	default:
		targets := []string{
			"First National Bank", "casino vault", "high-security vault",
			"the ATM", "corner store", "national mint", "armored truck",
		}
		return targets[rand.Intn(len(targets))]
	}
}

// cmdRob handles the /rob command.
// Usage: /rob [bank|casino|vault|atm|store|mint|armored|museum]
func cmdRob(client *Client, args []string, _ string) {
	// Pick the target; default to a random location if none provided.
	targetKey := ""
	if len(args) > 0 {
		targetKey = args[0]
	}
	location := robTargetName(targetKey)

	ipid := client.Ipid()

	// ── Minimum balance check ─────────────────────────────────────────────────
	bal, err := db.GetChipBalance(ipid)
	if err != nil {
		client.SendServerMessage("Could not retrieve your chip balance. Try again later.")
		return
	}
	if bal < robMinBalance {
		client.SendServerMessage(fmt.Sprintf(
			"💸 You need at least %d chips to attempt a rob (you only have %d). Save up first!",
			robMinBalance, bal))
		return
	}

	// ── Cooldown check ────────────────────────────────────────────────────────
	robMu.Lock()
	if last, ok := robLastTime[ipid]; ok {
		if elapsed := time.Since(last); elapsed < robCooldown {
			remaining := (robCooldown - elapsed).Truncate(time.Second)
			robMu.Unlock()
			client.SendServerMessage(fmt.Sprintf(
				"🚔 You're still on parole. Wait %v before attempting another rob.", remaining))
			return
		}
		delete(robLastTime, ipid)
	}
	robLastTime[ipid] = time.Now()
	robMu.Unlock()

	// ── Re-fetch balance after cooldown check (balance can change) ─────────────
	bal, err = db.GetChipBalance(ipid)
	if err != nil || bal < robMinBalance {
		client.SendServerMessage("Could not verify your balance. Try again later.")
		return
	}

	// ── Outcome roll ──────────────────────────────────────────────────────────
	// 20% chance of success; 80% chance of a catastrophic failure.
	roll := rand.Float64()
	if roll < 0.20 {
		robSuccess(client, location, bal)
	} else {
		robFailure(client, location, bal)
	}
}

// ── Success (20% chance) ──────────────────────────────────────────────────────

// robSuccess handles a successful heist: award a random chip haul.
func robSuccess(client *Client, location string, bal int64) {
	// Steal between 5% and 20% of current balance, clamped to [50, 5000].
	minSteal := int64(50)
	maxSteal := int64(5000)
	steal := int64(float64(bal) * (0.05 + rand.Float64()*0.15))
	if steal < minSteal {
		steal = minSteal
	}
	if steal > maxSteal {
		steal = maxSteal
	}

	newBal, _ := db.AddChips(client.Ipid(), steal)

	// Pick a random flavour message.
	messages := []string{
		fmt.Sprintf("🎉 CLEAN GETAWAY! You slipped past every guard at %s and made off with %d chips! The crew is celebrating.", location, steal),
		fmt.Sprintf("💰 BIG SCORE! You cracked the safe at %s and scooped up %d chips before the silent alarm even triggered.", location, steal),
		fmt.Sprintf("🏎️ VROOM! You grabbed %d chips from %s and the getaway driver was waiting right outside. Perfect heist.", steal, location),
		fmt.Sprintf("🕵️ MASTER THIEF! Disguised as a maintenance worker, you walked out of %s with %d chips in a toolbox.", location, steal),
		fmt.Sprintf("😎 IN AND OUT! %d chips lifted from %s without a single camera catching your face. Legendary.", steal, location),
	}
	msg := messages[rand.Intn(len(messages))]
	client.SendServerMessage(fmt.Sprintf("%s\n💰 New balance: %d chips", msg, newBal))
	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🔓 ROB: %s successfully robbed %s for %d chips!", client.OOCName(), location, steal))
}

// ── Failure (80% chance) ──────────────────────────────────────────────────────

// robFailure picks and applies one of eight catastrophic failure outcomes.
func robFailure(client *Client, location string, bal int64) {
	ipid := client.Ipid()
	failRoll := rand.Float64()

	switch {
	// ── Outcome 1 (30%) ── Lose 50%, OOC mute 5 min ──────────────────────────
	case failRoll < 0.30:
		lose := bal / 2
		newBal := drainChips(ipid, lose)
		applyMute(client, OOCMuted, 5*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🚨 ROB FAIL: %s was tackled outside %s and lost %d chips!", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🚨 BUSTED! Security tackled you as you fled %s.\n"+
				"You dropped the bag. Legal fees wiped half your chips (--%d chips).\n"+
				"The cops also confiscated your phone — you're OOC muted for 5 minutes.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 2 (20%) ── Lose 75% ──────────────────────────────────────────
	case failRoll < 0.50:
		lose := (bal * 3) / 4
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("💥 ROB FAIL: A dye pack exploded on %s during the %s heist! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"💥 DYE PACK! The money bag at %s was rigged.\n"+
				"Bright purple dye coated every chip you grabbed — completely worthless.\n"+
				"Emergency cleaning plus evidence disposal set you back %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 3 (15%) ── Lose 90% ──────────────────────────────────────────
	case failRoll < 0.65:
		lose := (bal * 9) / 10
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🔒 ROB FAIL: SWAT swarmed %s while %s was inside! -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"🔒 SWAT TEAM! The whole block around %s lit up with flashing lights.\n"+
				"You surrendered immediately. Twenty-two lawyers later, you still owe %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 4 (10%) ── Lose EVERYTHING (down to 1 chip) ─────────────────
	case failRoll < 0.75:
		lose := bal - 1
		if lose < 0 {
			lose = 0
		}
		drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("☠️ ROB FAIL: %s got absolutely COOKED trying to rob %s. Lost EVERYTHING!", client.OOCName(), location))
		client.SendServerMessage(fmt.Sprintf(
			"☠️ YOU GOT ABSOLUTELY COOKED.\n"+
				"Caught on 47 cameras at %s, ID'd by three witnesses, arrested, tried, convicted.\n"+
				"Every single chip was seized as criminal proceeds. Every. Single. One.\n"+
				"You have 1 chip left. That's mercy.\n"+
				"💀 New balance: 1 chip",
			location))

	// ── Outcome 5 (10%) ── Lose 60%, IC mute 3 min ───────────────────────────
	case failRoll < 0.85:
		lose := (bal * 6) / 10
		newBal := drainChips(ipid, lose)
		applyMute(client, ICMuted, 3*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("😤 ROB FAIL: Guards at %s hospitalised %s! -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"😤 BEAT DOWN! The security guards at %s do NOT get paid enough to be nice.\n"+
				"Hospital bills plus bail wiped %d chips from your account.\n"+
				"Your jaw is wired shut — IC muted for 3 minutes.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 6 (5%) ── Lose 95%, OOC mute 10 min ──────────────────────────
	case failRoll < 0.90:
		lose := (bal * 19) / 20
		newBal := drainChips(ipid, lose)
		applyMute(client, OOCMuted, 10*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🚔 ROB FAIL: FEDERAL charges against %s after the %s incident! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🚔 FBI RAID! Federal agents swept %s the moment you walked in.\n"+
				"Charged with federal bank robbery. Your lawyer took 95%% of your chips as a retainer.\n"+
				"You're remanded and OOC-gagged for 10 minutes while they process paperwork.\n"+
				"💀 New balance: %d chips",
			location, newBal))

	// ── Outcome 7 (5%) ── Lose 40% ───────────────────────────────────────────
	case failRoll < 0.95:
		lose := (bal * 2) / 5
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🤦 ROB FAIL: %s dropped their wallet inside %s. Classic. -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🤦 ROOKIE MISTAKE. You dropped your own wallet on the floor of %s.\n"+
				"They knew exactly who you were before you even left the building.\n"+
				"Settlement cost you %d chips. Could've been worse.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 8 (5%) ── Lose 70%, IC mute 2 min ────────────────────────────
	default:
		lose := (bal * 7) / 10
		newBal := drainChips(ipid, lose)
		applyMute(client, ICMuted, 2*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🐕 ROB FAIL: The guard dog at %s mauled %s. -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"🐕 THE DOG. Nobody warned you about the dog at %s.\n"+
				"Vet bills for the dog (yes, really), damages, and hush money totalled %d chips.\n"+
				"You can't speak for the next 2 minutes — IC muted.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// drainChips removes amount chips from ipid's balance and returns the new balance.
// If amount ≤ 0 or the player has too few chips, it drains as much as possible
// without going below 1, leaving the player with at least 1 chip.
func drainChips(ipid string, amount int64) int64 {
	if amount <= 0 {
		bal, _ := db.GetChipBalance(ipid)
		return bal
	}
	newBal, err := db.SpendChips(ipid, amount)
	if err != nil {
		// Balance was less than requested — drain all but 1.
		bal, _ := db.GetChipBalance(ipid)
		if bal > 1 {
			newBal, _ = db.SpendChips(ipid, bal-1)
		} else {
			newBal = bal
		}
	}
	return newBal
}

// applyMute applies a timed mute to the client and persists it to the database.
// The mute is stored in the DB so it survives reconnects.
func applyMute(client *Client, m MuteState, duration time.Duration) {
	expires := time.Now().UTC().Add(duration)
	client.SetMuted(m)
	client.SetUnmuteTime(expires)
	db.UpsertMute(client.Ipid(), int(m), expires.Unix()) //nolint:errcheck
}
