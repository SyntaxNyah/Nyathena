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

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// slotSymbols are the reel symbols in ascending value order.
var slotSymbols = []string{"🍒", "🍋", "🍊", "🍇", "⭐", "💎", "🎰"}

// slotsRateLimit tracks recent spin timestamps per player (uid → []time.Time).
var slotsRateLimit sync.Map

// slotsCheckRate returns true if the player is within the allowed spin rate.
func slotsCheckRate(uid int) bool {
	now := time.Now()
	cutoff := now.Add(-10 * time.Second)

	val, _ := slotsRateLimit.LoadOrStore(uid, &[]time.Time{})
	ts := val.(*[]time.Time)

	// Prune old timestamps.
	valid := (*ts)[:0]
	for _, t := range *ts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	*ts = valid

	if len(*ts) >= 5 {
		return false
	}
	*ts = append(*ts, now)
	return true
}

// slotsEvaluate returns the payout multiplier for three given symbols.
func slotsEvaluate(s1, s2, s3 string, jackpotEnabled bool) (float64, string) {
	switch {
	case s1 == "🎰" && s2 == "🎰" && s3 == "🎰":
		if jackpotEnabled {
			return -1, "JACKPOT! 🎰🎰🎰" // -1 signals jackpot
		}
		return 500, "🎰🎰🎰 — 500x payout!"
	case s1 == "💎" && s2 == "💎" && s3 == "💎":
		return 100, "💎💎💎 — 100x payout!"
	case s1 == "⭐" && s2 == "⭐" && s3 == "⭐":
		return 50, "⭐⭐⭐ — 50x payout!"
	case s1 == "🍇" && s2 == "🍇" && s3 == "🍇":
		return 20, "🍇🍇🍇 — 20x payout!"
	case s1 == "🍊" && s2 == "🍊" && s3 == "🍊":
		return 10, "🍊🍊🍊 — 10x payout!"
	case s1 == "🍋" && s2 == "🍋" && s3 == "🍋":
		return 5, "🍋🍋🍋 — 5x payout!"
	case s1 == "🍒" && s2 == "🍒" && s3 == "🍒":
		return 3, "🍒🍒🍒 — 3x payout!"
	case s1 == "🍒" && s2 == "🍒":
		return 2, "🍒🍒 — 2x payout!"
	default:
		if s1 == s2 || s2 == s3 || s1 == s3 {
			return 1, "Any pair — push (bet returned)"
		}
		return 0, "No match"
	}
}

// slotsDoSpin executes a single slot spin for the client at the given bet.
func slotsDoSpin(client *Client, bet int64) {
	if !slotsCheckRate(client.Uid()) {
		client.SendServerMessage("Slow down! Max 5 spins per 10 seconds.")
		return
	}

	ok, reason := validateBet(client, bet)
	if !ok {
		client.SendServerMessage(reason)
		return
	}
	_, err := db.SpendChips(client.Ipid(), bet)
	if err != nil {
		client.SendServerMessage("Failed to place bet: " + err.Error())
		return
	}

	s1 := slotSymbols[rand.Intn(len(slotSymbols))]
	s2 := slotSymbols[rand.Intn(len(slotSymbols))]
	s3 := slotSymbols[rand.Intn(len(slotSymbols))]

	jackpotEnabled := client.Area().CasinoJackpot()
	mult, desc := slotsEvaluate(s1, s2, s3, jackpotEnabled)

	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	cs.slotsStats.TotalSpins++

	var payout int64
	var msg string

	if mult == -1 {
		// Jackpot!
		pool := client.Area().CasinoJackpotPool()
		payout = pool + bet // win the pool plus the bet back
		cs.slotsStats.TotalPayout += payout
		cs.slotsStats.Jackpots++
		cs.mu.Unlock()

		client.Area().ResetCasinoJackpotPool()
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		bal, _ := db.GetChipBalance(client.Ipid())
		msg = fmt.Sprintf("[ %s | %s | %s ] %s\nJACKPOT! You won the pool of %d chips! Balance: %d",
			s1, s2, s3, desc, payout, bal)
		sendAreaServerMessage(client.Area(),
			fmt.Sprintf("🎰 JACKPOT! %s hit the jackpot for %d chips!", client.OOCName(), payout))
	} else if mult > 0 {
		payout = int64(float64(bet) * mult)
		cs.slotsStats.TotalPayout += payout
		cs.mu.Unlock()

		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		bal, _ := db.GetChipBalance(client.Ipid())
		msg = fmt.Sprintf("[ %s | %s | %s ] %s\nPayout: %d chips. Balance: %d",
			s1, s2, s3, desc, payout, bal)
	} else {
		// Loss: 5% of bet contributes to jackpot pool (integer truncation; zero for tiny bets is acceptable).
		if jackpotEnabled {
			contrib := bet / 20 // 5%
			cs.mu.Unlock()
			if contrib > 0 {
				client.Area().AddCasinoJackpotPool(contrib)
			}
		} else {
			cs.mu.Unlock()
		}
		bal, _ := db.GetChipBalance(client.Ipid())
		msg = fmt.Sprintf("[ %s | %s | %s ] %s\nYou lost %d chips. Balance: %d",
			s1, s2, s3, desc, bet, bal)
	}

	client.SendServerMessage(msg)
}

// cmdSlots handles /slots subcommands.
func cmdSlots(client *Client, args []string, _ string) {
	if !casinoCheck(client) {
		return
	}

	if len(args) == 0 || args[0] == "spin" {
		bet := int64(10)
		if len(args) >= 2 {
			n, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil || n <= 0 {
				client.SendServerMessage("Invalid bet amount.")
				return
			}
			bet = n
		}
		slotsDoSpin(client, bet)
		return
	}

	switch args[0] {
	case "max":
		maxBet := int64(client.Area().CasinoMaxBet())
		if maxBet <= 0 {
			bal, _ := db.GetChipBalance(client.Ipid())
			maxBet = bal
		}
		if maxBet <= 0 {
			client.SendServerMessage("You have no chips.")
			return
		}
		slotsDoSpin(client, maxBet)

	case "jackpot":
		if !client.Area().CasinoJackpot() {
			client.SendServerMessage("Jackpot is not enabled in this area.")
			return
		}
		pool := client.Area().CasinoJackpotPool()
		client.SendServerMessage(fmt.Sprintf("Current jackpot pool: %d chips", pool))

	case "stats":
		cs := getCasinoState(client.Area())
		cs.mu.Lock()
		stats := cs.slotsStats
		cs.mu.Unlock()
		lines := []string{
			"=== Slots Statistics ===",
			fmt.Sprintf("Total spins:   %d", stats.TotalSpins),
			fmt.Sprintf("Total payout:  %d chips", stats.TotalPayout),
			fmt.Sprintf("Jackpots hit:  %d", stats.Jackpots),
		}
		if client.Area().CasinoJackpot() {
			lines = append(lines, fmt.Sprintf("Current pool:  %d chips", client.Area().CasinoJackpotPool()))
		}
		client.SendServerMessage(strings.Join(lines, "\n"))

	default:
		client.SendServerMessage("Usage: /slots [spin [amount]] | /slots max | /slots jackpot | /slots stats")
	}
}
