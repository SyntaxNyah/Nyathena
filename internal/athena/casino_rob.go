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
const robCooldown = 1 * time.Hour

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
		fmt.Sprintf("🤝 INSIDE JOB! Your contact on the inside propped open the back door. Walked out of %s with %d chips like you owned the place.", location, steal),
		fmt.Sprintf("🪄 MAGIC HANDS! The lock at %s clicked open on the first try. You don't even know how. Scooped up %d chips and vanished.", location, steal),
		fmt.Sprintf("🔥 FIRE DRILL SPECIAL! The entire staff of %s evacuated just as you arrived. %d chips, zero witnesses.", location, steal),
		fmt.Sprintf("📦 BRILLIANT DISGUISE! You mailed yourself inside %s in a cardboard box. Grabbed %d chips and mailed yourself back out. Nobody questioned it.", location, steal),
		fmt.Sprintf("😴 NIGHT SHIFT! The only guard at %s was asleep at his desk. You stole %d chips, tucked him in, and left a thank-you note.", location, steal),
	}
	msg := messages[rand.Intn(len(messages))]
	client.SendServerMessage(fmt.Sprintf("%s\n💰 New balance: %d chips", msg, newBal))
	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🔓 ROB: %s successfully robbed %s for %d chips!", client.OOCName(), location, steal))
}

// ── Failure (80% chance) ──────────────────────────────────────────────────────

// robFailure picks and applies one of twenty catastrophic failure outcomes.
func robFailure(client *Client, location string, bal int64) {
	ipid := client.Ipid()
	failRoll := rand.Float64()

	switch {
	// ── Outcome 1 (8%) ── Lose 50%, OOC mute 5 min ───────────────────────────
	case failRoll < 0.08:
		lose := bal / 2
		newBal := drainChips(ipid, lose)
		applyMute(client, OOCMuted, 5*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🚨 ROB FAIL: %s was tackled outside %s and lost %d chips!", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🚨 BUSTED! Security tackled you as you fled %s.\n"+
				"You dropped the bag. Legal fees wiped half your chips (-%d chips).\n"+
				"The cops also confiscated your phone — you're OOC muted for 5 minutes.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 2 (8%) ── Lose 75% ───────────────────────────────────────────
	case failRoll < 0.16:
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

	// ── Outcome 3 (7%) ── Lose 90% ───────────────────────────────────────────
	case failRoll < 0.23:
		lose := (bal * 9) / 10
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🔒 ROB FAIL: SWAT swarmed %s while %s was inside! -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"🔒 SWAT TEAM! The whole block around %s lit up with flashing lights.\n"+
				"You surrendered immediately. Twenty-two lawyers later, you still owe %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 4 (5%) ── Lose EVERYTHING (down to 1 chip) ──────────────────
	case failRoll < 0.28:
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

	// ── Outcome 5 (7%) ── Lose 60%, IC mute 3 min ────────────────────────────
	case failRoll < 0.35:
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

	// ── Outcome 6 (4%) ── Lose 95%, OOC mute 10 min ─────────────────────────
	case failRoll < 0.39:
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
	case failRoll < 0.44:
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
	case failRoll < 0.49:
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

	// ── Outcome 9 (7%) ── Lose 55% ───────────────────────────────────────────
	case failRoll < 0.56:
		lose := (bal * 55) / 100
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("👗 ROB FAIL: %s robbed the wrong building — it was a costume shop! -%d chips.", client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"👗 WRONG BUILDING! You confidently walked into what you thought was %s.\n"+
				"It was a costume shop. You stole a rack of Halloween wigs.\n"+
				"By the time you realised, the actual security from next door had surrounded you.\n"+
				"Legal settlement and embarrassment tax: %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 10 (6%) ── Lose 65% ──────────────────────────────────────────
	case failRoll < 0.62:
		lose := (bal * 65) / 100
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🔔 ROB FAIL: %s tripped the fire alarm at %s and got caught in the evacuation! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🔔 FIRE ALARM! You bumped a sensor at %s and set off the building-wide alarm.\n"+
				"Everyone evacuated — including you — and the fire department found you still clutching a half-open vault door.\n"+
				"Fines, damages, and the cost of the emergency response: %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 11 (5%) ── Lose 45%, IC mute 5 min ───────────────────────────
	case failRoll < 0.67:
		lose := (bal * 45) / 100
		newBal := drainChips(ipid, lose)
		applyMute(client, ICMuted, 5*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🤧 ROB FAIL: %s sneezed at exactly the wrong moment inside %s! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🤧 ACHOO! You were crouched silently in the vents above %s when your nose decided NOW was the time.\n"+
				"Every guard in the building heard it. You were caught, mid-sneeze, dangling from a ceiling tile.\n"+
				"Medical bills (you broke your nose on the floor) plus fines: %d chips.\n"+
				"Your sinuses are still recovering — IC muted for 5 minutes.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 12 (5%) ── Lose 80% ──────────────────────────────────────────
	case failRoll < 0.72:
		lose := (bal * 4) / 5
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("📞 ROB FAIL: %s accidentally called 911 instead of the getaway driver from %s! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"📞 WRONG NUMBER! While fleeing %s you tried to call your getaway driver.\n"+
				"You called 911 instead. The dispatcher was very helpful and dispatched six units immediately.\n"+
				"You stayed on the line for 40 seconds before you realised.\n"+
				"Legal fees and the dispatcher's therapy bill: %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 13 (4%) ── Lose 35% ──────────────────────────────────────────
	case failRoll < 0.76:
		lose := (bal * 35) / 100
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🚗 ROB FAIL: The getaway car outside %s had a flat tyre. %s had to take the bus. -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"🚗 FLAT TYRE! You sprinted out of %s with the loot and found your getaway car listing sideways on a flat.\n"+
				"You had to take the bus home. The bus driver recognised you from the news.\n"+
				"He didn't say anything, but the look he gave you cost you %d chips in sheer spiritual damage.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 14 (4%) ── Lose 85%, OOC mute 7 min ─────────────────────────
	case failRoll < 0.80:
		lose := (bal * 85) / 100
		newBal := drainChips(ipid, lose)
		applyMute(client, OOCMuted, 7*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("📺 ROB FAIL: A news crew was filming outside %s when %s came sprinting out! -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"📺 LIVE ON AIR! A local news crew was doing a feel-good piece outside %s.\n"+
				"You burst through the doors, cash bag over your shoulder, directly into the shot.\n"+
				"The anchor said \"...and there you have it.\" The clip went global.\n"+
				"Lawyers, PR crisis management, and the anchor's book deal cut: %d chips.\n"+
				"You're OOC-gagged for 7 minutes — your publicist says no interviews.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 15 (4%) ── Lose 20% ──────────────────────────────────────────
	case failRoll < 0.84:
		lose := bal / 5
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🏜️ ROB FAIL: The vault at %s was already empty when %s arrived. -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"🏜️ ALREADY ROBBED! You got all the way into the vault at %s and found... nothing.\n"+
				"Some other crew beat you by about four minutes. You saw their tyre tracks leaving.\n"+
				"You still had to pay the locksmith, and you broke your favourite crowbar.\n"+
				"Wasted expenses: %d chips.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 16 (3%) ── Lose EVERYTHING (IRS), OOC mute 5 min ─────────────
	case failRoll < 0.87:
		lose := bal - 1
		if lose < 0 {
			lose = 0
		}
		drainChips(ipid, lose)
		applyMute(client, OOCMuted, 5*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🧾 ROB FAIL: A time-travelling IRS agent audited %s ON THE SPOT at %s. Lost everything!", client.OOCName(), location))
		client.SendServerMessage(fmt.Sprintf(
			"🧾 TIME-TRAVELLING IRS AGENT! You had just cracked the vault at %s.\n"+
				"A figure in a grey suit materialised from thin air, showed you a badge dated 2047, and said:\n"+
				"\"We've been watching your chip transactions across multiple timelines.\"\n"+
				"Every chip, seized retroactively. You have 1 chip left (pre-1985 earnings, non-taxable).\n"+
				"You're OOC-gagged for 5 minutes while they finish the audit.\n"+
				"💀 New balance: 1 chip",
			location))

	// ── Outcome 17 (3%) ── Lose 25% ──────────────────────────────────────────
	case failRoll < 0.90:
		lose := bal / 4
		newBal := drainChips(ipid, lose)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("💬 ROB FAIL: The security guard at %s just wanted to talk to %s. It went poorly. -%d chips.", location, client.OOCName(), lose))
		client.SendServerMessage(fmt.Sprintf(
			"💬 THE GUARD WANTED TO CHAT! The security guard at %s didn't tackle you — he just wanted to talk.\n"+
				"He asked about your feelings, your goals, and whether you'd considered a career in hospitality.\n"+
				"You were so disarmed by his sincerity that you ended up in a two-hour heart-to-heart.\n"+
				"By the end you'd voluntarily donated %d chips to his retirement fund. You feel weird about it.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 18 (3%) ── Lose 60%, IC mute 4 min ───────────────────────────
	case failRoll < 0.93:
		lose := (bal * 3) / 5
		newBal := drainChips(ipid, lose)
		applyMute(client, ICMuted, 4*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🧼 ROB FAIL: %s slipped on a wet floor sign at %s and got concussed. -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🧼 WET FLOOR! You were this close to the exit at %s when you hit a freshly mopped patch.\n"+
				"The wet floor sign was right there. In fact, you landed on it.\n"+
				"Concussion, three cracked ribs, and the janitor's legal claim: %d chips.\n"+
				"Your brain is rattled — IC muted for 4 minutes.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 19 (3%) ── Lose 70%, OOC mute 6 min ─────────────────────────
	case failRoll < 0.96:
		lose := (bal * 7) / 10
		newBal := drainChips(ipid, lose)
		applyMute(client, OOCMuted, 6*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("📱 ROB FAIL: %s went viral on a livestream mid-heist at %s! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"📱 LIVESTREAMED! Some influencer was doing a vlog outside %s and caught your entire heist on camera.\n"+
				"Peak concurrent viewers: 340,000. Comments were mostly laughing emojis.\n"+
				"The clip was used as evidence, memed into oblivion, and sold as an NFT.\n"+
				"Lawyers and reputation repair: %d chips.\n"+
				"You're OOC-gagged for 6 minutes on your publicist's orders.\n"+
				"💀 New balance: %d chips",
			location, lose, newBal))

	// ── Outcome 20 (4%) ── Lose 80%, OOC mute 8 min ─────────────────────────
	default:
		lose := (bal * 4) / 5
		newBal := drainChips(ipid, lose)
		applyMute(client, OOCMuted, 8*time.Minute)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🕵️ ROB FAIL: %s's getaway driver at %s turned out to be an undercover cop! -%d chips.", client.OOCName(), location, lose))
		client.SendServerMessage(fmt.Sprintf(
			"🕵️ UNDERCOVER COP! Your getaway driver outside %s was very chatty on the drive over.\n"+
				"Suspiciously chatty. Also he was wearing a wire. Also he arrested you at the lights.\n"+
				"He gave you a 10/10 for audacity on his incident report.\n"+
				"Legal fees, impound fees, and the fine for littering (you dropped a chip): %d chips.\n"+
				"OOC-gagged for 8 minutes — your lawyer says say nothing.\n"+
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
