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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// ── /chips give anti-abuse tracking ──────────────────────────────────────────

const (
	chipsGiveCooldown        = 10 * time.Minute // per-player cooldown between transfers
	chipsGiveMax             = 500              // maximum chips transferable per transaction
	chipsGiveMinPlaytime     = 24 * time.Hour   // minimum total playtime to use /chips give
)

var (
	chipsGiveMu        sync.Mutex
	chipsGiveLastTime  = make(map[string]time.Time) // ipid → last transfer time
)

// ── /casino ───────────────────────────────────────────────────────────────────

// cmdCasino is the top-level casino dashboard command.
// Subcommands: status
func cmdCasino(client *Client, args []string, _ string) {
	if len(args) == 0 {
		printCasinoDashboard(client)
		return
	}
	switch strings.ToLower(args[0]) {
	case "status":
		printCasinoStatus(client)
	default:
		client.SendServerMessage("Usage: /casino [status]")
	}
}

func printCasinoDashboard(client *Client) {
	a := client.Area()
	cs := getCasinoState(a)
	cs.mu.Lock()
	bj := cs.bjTable
	poker := cs.pokerTable
	stats := cs.slotsStats
	cs.mu.Unlock()

	enabled := a.CasinoEnabled()
	minBet := a.CasinoMinBet()
	maxBet := a.CasinoMaxBet()
	jackpot := a.CasinoJackpot()
	jackpotPool := a.CasinoJackpotPool()

	bal, _ := db.GetChipBalance(client.Ipid())

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🎰 Casino Dashboard — %v\n", a.Name()))
	sb.WriteString(fmt.Sprintf("  Status:  %v\n", boolStr(enabled, "OPEN", "CLOSED")))
	if minBet > 0 || maxBet > 0 {
		sb.WriteString(fmt.Sprintf("  Bets:    min=%d  max=%d\n", minBet, maxBet))
	}
	if jackpot {
		sb.WriteString(fmt.Sprintf("  Jackpot: enabled (pool: %d chips)\n", jackpotPool))
	}
	sb.WriteString(fmt.Sprintf("\n  Your balance: %d chips\n", bal))
	sb.WriteString("\n── Active Tables ──\n")
	if bj == nil {
		sb.WriteString("  Blackjack: no active table\n")
	} else {
		bj.mu.Lock()
		sb.WriteString(fmt.Sprintf("  Blackjack: %d player(s), state=%v\n", len(bj.players), bj.state))
		bj.mu.Unlock()
	}
	if poker == nil {
		sb.WriteString("  Poker:     no active table\n")
	} else {
		poker.mu.Lock()
		sb.WriteString(fmt.Sprintf("  Poker:     %d player(s), round=%v\n", len(poker.players), poker.state))
		poker.mu.Unlock()
	}
	sb.WriteString("\n── Slots Stats ──\n")
	sb.WriteString(fmt.Sprintf("  Spins: %d  Payout: %d  Jackpots: %d\n", stats.TotalSpins, stats.TotalPayout, stats.Jackpots))
	sb.WriteString("\n── Commands ──\n")
	sb.WriteString("  /bj join|bet|deal|hit|stand|double|split|insurance|status|leave\n")
	sb.WriteString("  /poker join|ready|hand|check|call|bet|raise|fold|allin|status|leave\n")
	sb.WriteString("  /slots spin [amount]|max|jackpot|stats\n")
	sb.WriteString("  /croulette bet <type> <amount>\n")
	sb.WriteString("  /baccarat <player|banker|tie> <amount>\n")
	sb.WriteString("  /craps bet <pass|nopass> <amount>\n")
	sb.WriteString("  /crash bet <amount> | /crash cashout\n")
	sb.WriteString("  /mines start <mines> <bet> | /mines pick <n> | /mines cashout | /mines quit\n")
	sb.WriteString("  /keno pick <numbers...> <bet>\n")
	sb.WriteString("  /wheel spin <bet>\n")

	client.SendServerMessage(sb.String())
}

func printCasinoStatus(client *Client) {
	a := client.Area()
	cs := getCasinoState(a)
	cs.mu.Lock()
	bj := cs.bjTable
	poker := cs.pokerTable
	stats := cs.slotsStats
	cs.mu.Unlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🎰 Casino Status — %v\n", a.Name()))
	sb.WriteString(fmt.Sprintf("  Casino: %v\n", boolStr(a.CasinoEnabled(), "OPEN", "CLOSED")))

	// Blackjack
	if bj == nil {
		sb.WriteString("  Blackjack: no active table\n")
	} else {
		bj.mu.Lock()
		sb.WriteString(fmt.Sprintf("  Blackjack: state=%v, players=%d\n", bj.state, len(bj.players)))
		for _, p := range bj.players {
			sb.WriteString(fmt.Sprintf("    • %v\n", p.Client.OOCName()))
		}
		bj.mu.Unlock()
	}

	// Poker
	if poker == nil {
		sb.WriteString("  Poker: no active table\n")
	} else {
		poker.mu.Lock()
		sb.WriteString(fmt.Sprintf("  Poker: round=%v, pot=%d chips, players=%d\n", poker.state, poker.pot, len(poker.players)))
		for _, p := range poker.players {
			if !p.Folded {
				sb.WriteString(fmt.Sprintf("    • %v\n", p.Client.OOCName()))
			}
		}
		poker.mu.Unlock()
	}

	// Slots
	sb.WriteString(fmt.Sprintf("  Slots: spins=%d, payout=%d, jackpots=%d\n", stats.TotalSpins, stats.TotalPayout, stats.Jackpots))
	if a.CasinoJackpot() {
		sb.WriteString(fmt.Sprintf("  Jackpot pool: %d chips\n", a.CasinoJackpotPool()))
	}

	client.SendServerMessage(sb.String())
}

func boolStr(v bool, t, f string) string {
	if v {
		return t
	}
	return f
}

// ── /chips (enhanced) ─────────────────────────────────────────────────────────

// cmdChipsEnhanced handles /chips with optional subcommands: top, area, give.
func cmdChipsEnhanced(client *Client, args []string, _ string) {
	if len(args) == 0 {
		bal, err := db.GetChipBalance(client.Ipid())
		if err != nil {
			client.SendServerMessage("Could not retrieve your chip balance.")
			return
		}
		client.SendServerMessage(fmt.Sprintf("💰 Your balance: %d Nyathena Chips", bal))
		return
	}
	switch strings.ToLower(args[0]) {
	case "top":
		chipsTopGlobal(client, args[1:])
	case "area":
		chipsTopArea(client, args[1:])
	case "give":
		chipsGive(client, args[1:])
	default:
		client.SendServerMessage("Usage: /chips [top [n]] | [area [n]] | [give <uid> <amount>]")
	}
}

func chipsTopGlobal(client *Client, args []string) {
	n := 10
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil && v > 0 && v <= 50 {
			n = v
		}
	}
	entries, err := db.GetTopChipBalances(n)
	if err != nil || len(entries) == 0 {
		client.SendServerMessage("No chip data available.")
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🏆 Global Chip Leaderboard (Top %d)\n", n))
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %2d. %v — %d chips\n", i+1, e.Username, e.Balance))
	}
	client.SendServerMessage(sb.String())
}

// cmdRichest is a convenience alias for /chips top — shows the global chip leaderboard.
func cmdRichest(client *Client, args []string, _ string) {
	chipsTopGlobal(client, args)
}

func chipsTopArea(client *Client, args []string) {
	n := 10
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil && v > 0 && v <= 50 {
			n = v
		}
	}

	// Collect all clients in this area and their IPIDs in one pass.
	type entry struct {
		name    string
		balance int64
	}
	myArea := client.Area()
	var ipids []string
	var clientsInArea []*Client
	for c := range clients.GetAllClients() {
		if c.Area() != myArea || c.Uid() == -1 {
			continue
		}
		ipids = append(ipids, c.Ipid())
		clientsInArea = append(clientsInArea, c)
	}
	if len(clientsInArea) == 0 {
		client.SendServerMessage("No players in this area.")
		return
	}

	// Batch-resolve account names and chip balances in two queries instead of N+1.
	names, _ := db.GetUsernamesByIPIDs(ipids)
	balances, _ := db.GetChipBalancesByIPIDs(ipids)

	entries := make([]entry, 0, len(clientsInArea))
	for _, c := range clientsInArea {
		displayName := c.OOCName()
		if u := names[c.Ipid()]; u != "" {
			displayName = u
		}
		entries = append(entries, entry{name: displayName, balance: balances[c.Ipid()]})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].balance > entries[j].balance })
	if len(entries) > n {
		entries = entries[:n]
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🏆 Area Chip Leaderboard — %v (Top %d)\n", myArea.Name(), n))
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %2d. %v — %d chips\n", i+1, e.name, e.balance))
	}
	client.SendServerMessage(sb.String())
}

func chipsGive(client *Client, args []string) {
	if len(args) < 2 {
		client.SendServerMessage("Usage: /chips give <uid> <amount>")
		return
	}
	targetUID, err := strconv.Atoi(args[0])
	if err != nil || targetUID < 0 {
		client.SendServerMessage("Invalid UID.")
		return
	}
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Amount must be a positive integer.")
		return
	}
	if amount > chipsGiveMax {
		client.SendServerMessage(fmt.Sprintf("You can transfer at most %d chips at a time.", chipsGiveMax))
		return
	}

	// Require 24 hours of total playtime before a player can transfer chips.
	// This prevents newly-created accounts from being used to funnel chips.
	storedPlaytime, ptErr := db.GetPlaytime(client.Ipid())
	if ptErr != nil {
		client.SendServerMessage("Could not verify playtime. Please try again.")
		return
	}
	totalPlaytime := time.Duration(storedPlaytime) * time.Second
	if connAt := client.ConnectedAt(); !connAt.IsZero() {
		totalPlaytime += time.Since(connAt)
	}
	if totalPlaytime < chipsGiveMinPlaytime {
		remaining := (chipsGiveMinPlaytime - totalPlaytime).Truncate(time.Second)
		client.SendServerMessage(fmt.Sprintf(
			"You need at least 24 hours of playtime to transfer chips. You still need %v.",
			remaining,
		))
		return
	}

	// Check and record the cooldown atomically.  Lazily delete entries whose
	// cooldown has already expired to keep the map bounded.
	chipsGiveMu.Lock()
	if last, ok := chipsGiveLastTime[client.Ipid()]; ok {
		if elapsed := time.Since(last); elapsed < chipsGiveCooldown {
			remaining := (chipsGiveCooldown - elapsed).Truncate(time.Second)
			chipsGiveMu.Unlock()
			client.SendServerMessage(fmt.Sprintf("You must wait %v before giving chips again.", remaining))
			return
		}
		// Cooldown has passed — remove the stale entry now.
		delete(chipsGiveLastTime, client.Ipid())
	}
	chipsGiveLastTime[client.Ipid()] = time.Now()
	chipsGiveMu.Unlock()

	target := clients.GetClientByUID(targetUID)
	if target == nil {
		client.SendServerMessage("Player not found.")
		return
	}
	if target.Ipid() == client.Ipid() {
		client.SendServerMessage("You cannot give chips to yourself.")
		return
	}

	senderBal, err := db.SpendChips(client.Ipid(), amount)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Transfer failed: %v", err))
		return
	}
	if _, err = db.AddChips(target.Ipid(), amount); err != nil {
		// Attempt to refund the sender; log any failure so admins can investigate.
		if _, refundErr := db.AddChips(client.Ipid(), amount); refundErr != nil {
			logger.LogErrorf("chips give: deducted %d chips from %v but credit to %v failed AND refund failed: %v",
				amount, client.Ipid(), target.Ipid(), refundErr)
		}
		client.SendServerMessage("Transfer failed: could not credit recipient.")
		return
	}

	client.SendServerMessage(fmt.Sprintf("Sent %d chips to %v. Your balance: %d chips.", amount, target.OOCName(), senderBal))
	target.SendServerMessage(fmt.Sprintf("You received %d Nyathena Chips from %v!", amount, client.OOCName()))
}

// ── /casinoenable ─────────────────────────────────────────────────────────────

func cmdCasinoEnable(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.SendServerMessage("Usage: /casinoenable <true|false>")
		return
	}
	val, err := strconv.ParseBool(args[0])
	if err != nil {
		client.SendServerMessage("Usage: /casinoenable <true|false>")
		return
	}
	client.Area().SetCasinoEnabled(val)
	state := "disabled"
	if val {
		state = "enabled"
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v %v the casino in this area.", client.OOCName(), state))
}

// ── /gamble ───────────────────────────────────────────────────────────────────

// cmdGamble handles the /gamble command.
// Subcommand: hide — toggles whether the client receives gambling broadcast messages.
func cmdGamble(client *Client, args []string, _ string) {
	if len(args) == 0 || strings.ToLower(args[0]) != "hide" {
		client.SendServerMessage("Usage: /gamble hide  — toggle gambling broadcast messages on/off.")
		return
	}
	if client.GambleHide() {
		client.SetGambleHide(false)
		client.SendServerMessage("🎰 Gambling messages are now visible again.")
	} else {
		client.SetGambleHide(true)
		client.SendServerMessage("🔇 Gambling broadcast messages are now hidden. Use /gamble hide again to show them.")
	}
}

// ── /casinoset ────────────────────────────────────────────────────────────────

func cmdCasinoSet(client *Client, args []string, _ string) {
	const usage = "Usage: /casinoset <minbet|maxbet|maxtables|jackpot> <value>"
	if len(args) < 2 {
		client.SendServerMessage(usage)
		return
	}
	switch strings.ToLower(args[0]) {
	case "minbet":
		v, err := strconv.Atoi(args[1])
		if err != nil || v < 0 {
			client.SendServerMessage("minbet must be a non-negative integer (0 = no limit).")
			return
		}
		client.Area().SetCasinoMinBet(v)
		client.SendServerMessage(fmt.Sprintf("Casino minimum bet set to %d.", v))
	case "maxbet":
		v, err := strconv.Atoi(args[1])
		if err != nil || v < 0 {
			client.SendServerMessage("maxbet must be a non-negative integer (0 = no limit).")
			return
		}
		client.Area().SetCasinoMaxBet(v)
		client.SendServerMessage(fmt.Sprintf("Casino maximum bet set to %d.", v))
	case "maxtables":
		v, err := strconv.Atoi(args[1])
		if err != nil || v < 0 {
			client.SendServerMessage("maxtables must be a non-negative integer (0 = no limit).")
			return
		}
		client.Area().SetCasinoMaxTables(v)
		client.SendServerMessage(fmt.Sprintf("Casino max tables set to %d.", v))
	case "jackpot":
		v, err := strconv.ParseBool(args[1])
		if err != nil {
			client.SendServerMessage("jackpot must be true or false.")
			return
		}
		client.Area().SetCasinoJackpot(v)
		client.SendServerMessage(fmt.Sprintf("Casino slots jackpot set to %v.", v))
	default:
		client.SendServerMessage(usage)
	}
}
