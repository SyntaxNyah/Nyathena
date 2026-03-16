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

// ============================================================
// /chips — balance command
// ============================================================

func cmdChips(client *Client, _ []string, _ string) {
	bal, err := db.GetChipBalance(client.Ipid())
	if err != nil {
		client.SendServerMessage("Could not retrieve your chip balance.")
		return
	}
	client.SendServerMessage(fmt.Sprintf("Your chip balance: %d chips", bal))
}

// ============================================================
// /croulette — European Roulette (37 pockets, 0-36)
// ============================================================

// rouletteRedNumbers is a fixed-size array marking the 18 red pockets (indices 0-36).
// Array indexing is faster than a map lookup and avoids heap allocation.
var rouletteRedNumbers = [37]bool{
	1: true, 3: true, 5: true, 7: true, 9: true, 12: true,
	14: true, 16: true, 18: true, 19: true, 21: true, 23: true,
	25: true, 27: true, 30: true, 32: true, 34: true, 36: true,
}

func cmdCasinoRoulette(client *Client, args []string, _ string) {
	if len(args) < 2 || strings.ToLower(args[0]) != "bet" {
		client.SendServerMessage("Usage: /croulette bet <red|black|even|odd|low|high|number <n>> <amount>")
		return
	}

	// Parse bet type and amount from args[1:].
	// Formats: bet red 100 | bet number 7 100
	betArgs := args[1:]
	var betType string
	var betNum int
	var amountStr string

	switch strings.ToLower(betArgs[0]) {
	case "number":
		if len(betArgs) < 3 {
			client.SendServerMessage("Usage: /croulette bet number <n> <amount>")
			return
		}
		n, err := strconv.Atoi(betArgs[1])
		if err != nil || n < 0 || n > 36 {
			client.SendServerMessage("Number must be 0-36.")
			return
		}
		betType = "number"
		betNum = n
		amountStr = betArgs[2]
	case "red", "black", "even", "odd", "low", "high":
		if len(betArgs) < 2 {
			client.SendServerMessage("Usage: /croulette bet <type> <amount>")
			return
		}
		betType = strings.ToLower(betArgs[0])
		amountStr = betArgs[1]
	default:
		client.SendServerMessage("Invalid bet type. Use: red|black|even|odd|low|high|number <n>")
		return
	}

	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Invalid bet amount.")
		return
	}
	ok, reason := validateBet(client, amount)
	if !ok {
		client.SendServerMessage(reason)
		return
	}
	balAfterBet, err := db.SpendChips(client.Ipid(), amount)
	if err != nil {
		client.SendServerMessage("Failed to place bet: " + err.Error())
		return
	}

	spin := rand.Intn(37) // 0-36
	isRed := rouletteRedNumbers[spin]
	isEven := spin != 0 && spin%2 == 0

	var win bool
	var payoutMult int64
	switch betType {
	case "red":
		win = isRed
		payoutMult = 2
	case "black":
		win = !isRed && spin != 0
		payoutMult = 2
	case "even":
		win = isEven
		payoutMult = 2
	case "odd":
		win = !isEven && spin != 0
		payoutMult = 2
	case "low":
		win = spin >= 1 && spin <= 18
		payoutMult = 2
	case "high":
		win = spin >= 19 && spin <= 36
		payoutMult = 2
	case "number":
		win = spin == betNum
		payoutMult = 36 // 35:1 pays 36x total
	}

	spinColour := "green (0)"
	if spin != 0 {
		if isRed {
			spinColour = fmt.Sprintf("red (%d)", spin)
		} else {
			spinColour = fmt.Sprintf("black (%d)", spin)
		}
	}

	var bal int64
	var result string
	if win {
		payout := amount * payoutMult
		bal, _ = db.AddChips(client.Ipid(), payout)
		result = fmt.Sprintf("WIN! +%d chips", payout-amount)
	} else {
		bal = balAfterBet
		result = fmt.Sprintf("LOSE. -%d chips", amount)
	}

	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🎡 Roulette: %s spun the wheel — %s! %s", client.OOCName(), spinColour, result))
	client.SendServerMessage(fmt.Sprintf(
		"Roulette result: %s | Your bet: %s | %s | Balance: %d",
		spinColour, betType, result, bal))
}

// ============================================================
// /baccarat — Baccarat
// ============================================================

// baccaratCardValue returns a baccarat card value (10/J/Q/K = 0, A = 1).
func baccaratCardValue(v int) int {
	if v >= 10 {
		return 0
	}
	return v
}

// baccaratHandValue returns the baccarat hand total (mod 10).
func baccaratHandValue(hand []Card) int {
	total := 0
	for _, c := range hand {
		total += baccaratCardValue(c.Value)
	}
	return total % 10
}

func cmdBaccarat(client *Client, args []string, _ string) {
	if len(args) < 2 {
		client.SendServerMessage("Usage: /baccarat <player|banker|tie> <amount>")
		return
	}

	betSide := strings.ToLower(args[0])
	if betSide != "player" && betSide != "banker" && betSide != "tie" {
		client.SendServerMessage("Bet side must be: player, banker, or tie.")
		return
	}

	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Invalid bet amount.")
		return
	}
	ok, reason := validateBet(client, amount)
	if !ok {
		client.SendServerMessage(reason)
		return
	}
	balAfterBet, err := db.SpendChips(client.Ipid(), amount)
	if err != nil {
		client.SendServerMessage("Failed to place bet: " + err.Error())
		return
	}

	deck := newDeck(6)

	// Initial deal: player and banker each get 2 cards.
	pHand := []Card{deck[0], deck[2]}
	bHand := []Card{deck[1], deck[3]}
	deck = deck[4:]

	pVal := baccaratHandValue(pHand)
	bVal := baccaratHandValue(bHand)

	// Natural: 8 or 9 on initial deal — no more cards.
	natural := pVal >= 8 || bVal >= 8

	if !natural {
		// Player draws if total 0-5.
		var pThird *Card
		if pVal <= 5 {
			c := deck[0]
			deck = deck[1:]
			pHand = append(pHand, c)
			pThird = &c
			pVal = baccaratHandValue(pHand)
		}

		// Banker draws based on banker total and player's third card.
		bankerDraws := false
		if pThird == nil {
			bankerDraws = bVal <= 5
		} else {
			pt := baccaratCardValue(pThird.Value)
			switch {
			case bVal <= 2:
				bankerDraws = true
			case bVal == 3:
				bankerDraws = pt != 8
			case bVal == 4:
				bankerDraws = pt >= 2 && pt <= 7
			case bVal == 5:
				bankerDraws = pt >= 4 && pt <= 7
			case bVal == 6:
				bankerDraws = pt == 6 || pt == 7
			}
		}
		if bankerDraws {
			c := deck[0]
			bHand = append(bHand, c)
			bVal = baccaratHandValue(bHand)
		}
	}

	// Determine winner.
	var winner string
	switch {
	case pVal > bVal:
		winner = "player"
	case bVal > pVal:
		winner = "banker"
	default:
		winner = "tie"
	}

	handDesc := func(hand []Card) string {
		parts := make([]string, len(hand))
		for i, c := range hand {
			parts[i] = c.String()
		}
		return strings.Join(parts, " ")
	}

	var bal int64
	var payout int64
	var result string
	switch {
	case winner == betSide:
		switch betSide {
		case "player":
			payout = amount * 2
		case "banker":
			// Banker wins pay 0.95:1 (5% commission). Total return = bet + 95% of bet = 1.95×.
			// Use integer arithmetic to avoid floating-point precision issues.
			payout = (amount * 195) / 100
		case "tie":
			payout = amount * 9 // 8:1
		}
		bal, _ = db.AddChips(client.Ipid(), payout)
		result = fmt.Sprintf("WIN! +%d chips", payout-amount)
	default:
		bal = balAfterBet
		result = fmt.Sprintf("LOSE. -%d chips", amount)
	}
	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🃏 Baccarat: Player %d vs Banker %d — %s wins! %s bet %s and %s.",
			pVal, bVal, winner, client.OOCName(), betSide, result))
	client.SendServerMessage(fmt.Sprintf(
		"Player: %s (%d) | Banker: %s (%d) | Winner: %s | %s | Balance: %d",
		handDesc(pHand), pVal, handDesc(bHand), bVal, winner, result, bal))
}

// ============================================================
// /craps — Simplified Pass/Don't-Pass Craps
// ============================================================

func cmdCraps(client *Client, args []string, _ string) {
	if len(args) < 3 || strings.ToLower(args[0]) != "bet" {
		client.SendServerMessage("Usage: /craps bet <pass|nopass> <amount>")
		return
	}

	betType := strings.ToLower(args[1])
	if betType != "pass" && betType != "nopass" {
		client.SendServerMessage("Bet type must be pass or nopass.")
		return
	}

	amount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Invalid bet amount.")
		return
	}
	ok, reason := validateBet(client, amount)
	if !ok {
		client.SendServerMessage(reason)
		return
	}
	balAfterBet, err := db.SpendChips(client.Ipid(), amount)
	if err != nil {
		client.SendServerMessage("Failed to place bet: " + err.Error())
		return
	}

	rollDice := func() (int, int) {
		return rand.Intn(6) + 1, rand.Intn(6) + 1
	}

	d1, d2 := rollDice()
	comeOut := d1 + d2
	rolls := []string{fmt.Sprintf("%d+%d=%d", d1, d2, comeOut)}

	var passWin bool
	switch comeOut {
	case 7, 11:
		passWin = true
	case 2, 3, 12:
		passWin = false
	default:
		// Establish point, keep rolling.
		point := comeOut
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🎲 Craps: %s rolls %d+%d=%d — point is %d!", client.OOCName(), d1, d2, comeOut, point))
		for {
			d1, d2 = rollDice()
			sum := d1 + d2
			rolls = append(rolls, fmt.Sprintf("%d+%d=%d", d1, d2, sum))
			if sum == point {
				passWin = true
				break
			} else if sum == 7 {
				passWin = false
				break
			}
		}
	}

	win := (betType == "pass") == passWin
	var bal int64
	var result string
	if win {
		payout := amount * 2
		bal, _ = db.AddChips(client.Ipid(), payout)
		result = fmt.Sprintf("WIN! +%d chips", amount)
	} else {
		bal = balAfterBet
		result = fmt.Sprintf("LOSE. -%d chips", amount)
	}

	outcome := "pass"
	if !passWin {
		outcome = "don't-pass"
	}
	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🎲 Craps: %s — %s wins! %s bet %s and %s.",
			strings.Join(rolls, " → "), outcome, client.OOCName(), betType, result))
	client.SendServerMessage(fmt.Sprintf(
		"Craps rolls: %s | Outcome: %s | %s | Balance: %d",
		strings.Join(rolls, " → "), outcome, result, bal))
}

// ============================================================
// /crash — Crash
// ============================================================

// crashMinMultiplier and crashMaxMultiplier define the range of possible crash points.
// Min 1.05× means the game almost always ends in a near-instant loss for instant cashouts;
// max 6× provides a large but rare upside. The 20% house edge (crashHouseEdge = 0.80) is
// applied multiplicatively to the payout — players receive 80 cents per dollar of expected
// value, matching a typical real-money crash game RTP of ~80%. crashGrowthPerSec controls
// how fast the displayed multiplier rises; the actual crash point is determined at bet time.
const (
	crashMinMultiplier = 1.05
	crashMaxMultiplier = 6.0
	crashGrowthPerSec  = 0.1  // multiplier increase per second
	crashHouseEdge     = 0.80 // 20% house edge: payout = bet × current_mult × 0.80
	crashMinHoldSec    = 5.0  // minimum seconds before cashout is allowed
	crashBetCooldown   = 45 * time.Second // per-player cooldown between rounds
)

// CrashState holds per-player crash game state.
type CrashState struct {
	Bet       int64
	StartTime time.Time
	CrashAt   float64 // multiplier at which the game crashes
	Active    bool
}

// playerCrashStates maps uid → *CrashState.
var playerCrashStates sync.Map

// playerCrashCooldown maps uid → Unix nanoseconds of last bet start for cooldown enforcement.
// Storing int64 (UnixNano) instead of time.Time avoids boxing a larger struct into the sync.Map.
var playerCrashCooldown sync.Map

func cmdCrash(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.SendServerMessage("Usage: /crash bet <amount> | /crash cashout")
		return
	}

	switch strings.ToLower(args[0]) {
	case "bet":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /crash bet <amount>")
			return
		}
		if val, ok := playerCrashStates.Load(client.Uid()); ok {
			if val.(*CrashState).Active {
				client.SendServerMessage("You already have an active crash game. Use /crash cashout or wait for it to crash.")
				return
			}
		}

		// Enforce per-player cooldown between crash bets to prevent spam.
		if rawNano, ok := playerCrashCooldown.Load(client.Uid()); ok {
			if elapsed := time.Duration(time.Now().UnixNano() - rawNano.(int64)); elapsed < crashBetCooldown {
				remaining := (crashBetCooldown - elapsed).Truncate(time.Second)
				client.SendServerMessage(fmt.Sprintf(
					"🕐 Crash cooldown: the rocket needs %v to refuel before you can launch again.", remaining))
				return
			}
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || amount <= 0 {
			client.SendServerMessage("Invalid bet amount.")
			return
		}
		ok, reason := validateBet(client, amount)
		if !ok {
			client.SendServerMessage(reason)
			return
		}
		if _, err = db.SpendChips(client.Ipid(), amount); err != nil {
			client.SendServerMessage("Failed to place bet: " + err.Error())
			return
		}

		// Record bet time (as UnixNano) for cooldown tracking.
		playerCrashCooldown.Store(client.Uid(), time.Now().UnixNano())

		// Crash point: single RNG call raised to the 4th power (r*r*r*r).
		// This gives a Beta(1,4)-like distribution heavily skewed toward 0, so most
		// games crash near the minimum multiplier. The expected value of r^4 is 0.2,
		// meaning the average crash point is about crashMin + 0.2*(crashMax-crashMin).
		// One RNG call instead of the previous four calls keeps the hot path fast.
		r := rand.Float64()
		crashAt := crashMinMultiplier + r*r*r*r*(crashMaxMultiplier-crashMinMultiplier)

		state := &CrashState{
			Bet:       amount,
			StartTime: time.Now(),
			CrashAt:   crashAt,
			Active:    true,
		}
		playerCrashStates.Store(client.Uid(), state)

		client.SendServerMessage(fmt.Sprintf(
			"🚀 Crash started! Bet: %d chips. Multiplier grows at 0.1x/sec. Use /crash cashout to cash out!",
			amount))

	case "cashout":
		val, ok := playerCrashStates.Load(client.Uid())
		if !ok || !val.(*CrashState).Active {
			client.SendServerMessage("You have no active crash game.")
			return
		}
		state := val.(*CrashState)
		state.Active = false
		playerCrashStates.Delete(client.Uid())

		elapsed := time.Since(state.StartTime).Seconds()

		// Enforce minimum hold time — prevents instant-cashout spamming.
		if elapsed < crashMinHoldSec {
			// Count as a loss: the rocket explodes on the launchpad.
			sendAreaGamblingMessage(client.Area(),
				fmt.Sprintf("💥 Crash! %s tried to eject before the rocket cleared the launchpad — BOOM!",
					client.OOCName()))
			client.SendServerMessage(fmt.Sprintf(
				"💥 Too early! You must hold for at least %.0f seconds (only %.1fs elapsed).\n"+
					"The rocket exploded on the launchpad. You lost %d chips.",
				crashMinHoldSec, elapsed, state.Bet))
			return
		}

		current := 1.0 + elapsed*crashGrowthPerSec

		if current >= state.CrashAt {
			// Already crashed.
			sendAreaGamblingMessage(client.Area(),
				fmt.Sprintf("💥 Crash! %s's game already crashed at %.2fx (too late to cash out).",
					client.OOCName(), state.CrashAt))
			client.SendServerMessage(fmt.Sprintf(
				"💥 Too late! The game crashed at %.2fx. You lost %d chips.",
				state.CrashAt, state.Bet))
			return
		}

		payout := int64(float64(state.Bet) * current * crashHouseEdge)
		bal, _ := db.AddChips(client.Ipid(), payout)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🚀 Crash: %s cashed out at %.2fx for %d chips!",
				client.OOCName(), current, payout))
		client.SendServerMessage(fmt.Sprintf(
			"Cashed out at %.2fx! Payout: %d chips (crash was at %.2fx). Balance: %d",
			current, payout, state.CrashAt, bal))

	default:
		client.SendServerMessage("Usage: /crash bet <amount> | /crash cashout")
	}
}

// ============================================================
// /mines — Mines
// ============================================================

// MinesState holds per-player mines game state.
type MinesState struct {
	Grid      [25]bool // true = mine
	Revealed  [25]bool
	Bet       int64
	SafePicks int
	MineCount int
	Active    bool
}

// playerMinesStates maps uid → *MinesState.
var playerMinesStates sync.Map

// minesMultiplier returns the payout multiplier for n safe picks with m mines on a 5x5 grid.
func minesMultiplier(safePicks, mineCount int) float64 {
	if safePicks == 0 {
		return 1.0
	}
	// Use a simplified cumulative multiplier:
	// Each safe pick multiplies winnings by (total_cells / remaining_safe_cells).
	total := 25
	safe := total - mineCount
	mult := 1.0
	for i := 0; i < safePicks; i++ {
		remaining := total - i
		safeCellsLeft := safe - i
		if safeCellsLeft <= 0 {
			break
		}
		mult *= float64(remaining) / float64(safeCellsLeft)
	}
	return mult * 0.97 // 3% house edge
}

func cmdMines(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.SendServerMessage("Usage: /mines start <mines> <bet> | /mines pick <n> | /mines cashout | /mines quit")
		return
	}

	switch strings.ToLower(args[0]) {
	case "start":
		if len(args) < 3 {
			client.SendServerMessage("Usage: /mines start <mines 1-24> <bet>")
			return
		}
		if val, ok := playerMinesStates.Load(client.Uid()); ok {
			if val.(*MinesState).Active {
				client.SendServerMessage("You already have an active mines game. Use /mines cashout or /mines quit.")
				return
			}
		}

		mineCount, err := strconv.Atoi(args[1])
		if err != nil || mineCount < 1 || mineCount > 24 {
			client.SendServerMessage("Mine count must be between 1 and 24.")
			return
		}
		bet, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil || bet <= 0 {
			client.SendServerMessage("Invalid bet amount.")
			return
		}
		ok, reason := validateBet(client, bet)
		if !ok {
			client.SendServerMessage(reason)
			return
		}
		if _, err = db.SpendChips(client.Ipid(), bet); err != nil {
			client.SendServerMessage("Failed to place bet: " + err.Error())
			return
		}

		// Place mines randomly.
		positions := rand.Perm(25)
		var state MinesState
		state.Bet = bet
		state.MineCount = mineCount
		state.Active = true
		for i := 0; i < mineCount; i++ {
			state.Grid[positions[i]] = true
		}
		playerMinesStates.Store(client.Uid(), &state)

		mult := minesMultiplier(1, mineCount)
		client.SendServerMessage(fmt.Sprintf(
			"💣 Mines started! %d mines hidden in a 5×5 grid. Bet: %d chips.\n"+
				"Use /mines pick <1-25> to reveal a cell. Next pick multiplier: %.2fx",
			mineCount, bet, mult))

	case "pick":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /mines pick <1-25>")
			return
		}
		val, ok := playerMinesStates.Load(client.Uid())
		if !ok || !val.(*MinesState).Active {
			client.SendServerMessage("No active mines game. Use /mines start <mines> <bet>.")
			return
		}
		state := val.(*MinesState)

		cell, err := strconv.Atoi(args[1])
		if err != nil || cell < 1 || cell > 25 {
			client.SendServerMessage("Cell must be 1-25.")
			return
		}
		idx := cell - 1
		if state.Revealed[idx] {
			client.SendServerMessage("That cell is already revealed.")
			return
		}

		state.Revealed[idx] = true
		if state.Grid[idx] {
			// Hit a mine!
			state.Active = false
			playerMinesStates.Delete(client.Uid())
			client.SendServerMessage(fmt.Sprintf(
				"💥 BOOM! Cell %d was a mine! You lose %d chips.",
				cell, state.Bet))
			sendAreaGamblingMessage(client.Area(),
				fmt.Sprintf("💣 %s hit a mine in Mines!", client.OOCName()))
			return
		}

		state.SafePicks++
		mult := minesMultiplier(state.SafePicks, state.MineCount)
		currentWin := int64(float64(state.Bet) * mult)
		safe := 25 - state.MineCount - state.SafePicks
		client.SendServerMessage(fmt.Sprintf(
			"✅ Safe! Cell %d cleared. Safe picks: %d | Current win: %d chips (%.2fx) | Remaining safe cells: %d\n"+
				"Use /mines pick <n> to continue or /mines cashout to take your winnings.",
			cell, state.SafePicks, currentWin, mult, safe))

	case "cashout":
		val, ok := playerMinesStates.Load(client.Uid())
		if !ok || !val.(*MinesState).Active {
			client.SendServerMessage("No active mines game.")
			return
		}
		state := val.(*MinesState)
		if state.SafePicks == 0 {
			client.SendServerMessage("Pick at least one safe cell before cashing out.")
			return
		}
		state.Active = false
		playerMinesStates.Delete(client.Uid())

		mult := minesMultiplier(state.SafePicks, state.MineCount)
		payout := int64(float64(state.Bet) * mult)
		bal, _ := db.AddChips(client.Ipid(), payout)
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("💣 %s cashed out Mines for %d chips (%.2fx)!", client.OOCName(), payout, mult))
		client.SendServerMessage(fmt.Sprintf(
			"Cashed out! %d safe picks × %.2fx = %d chips. Balance: %d",
			state.SafePicks, mult, payout, bal))

	case "quit":
		val, ok := playerMinesStates.Load(client.Uid())
		if !ok || !val.(*MinesState).Active {
			client.SendServerMessage("No active mines game.")
			return
		}
		state := val.(*MinesState)
		state.Active = false
		playerMinesStates.Delete(client.Uid())
		client.SendServerMessage(fmt.Sprintf(
			"Quit mines. You forfeited your bet of %d chips.", state.Bet))

	default:
		client.SendServerMessage("Usage: /mines start <mines> <bet> | /mines pick <n> | /mines cashout | /mines quit")
	}
}

// ============================================================
// /keno — Keno
// ============================================================

// kenoPayouts maps [picks][matches] → multiplier (0 = no payout).
// Based on a standard keno pay table scaled by number of picks.
var kenoPayouts = map[int]map[int]int{
	1:  {1: 2},
	2:  {2: 5},
	3:  {2: 1, 3: 15},
	4:  {2: 1, 3: 5, 4: 50},
	5:  {3: 2, 4: 15, 5: 200},
	6:  {3: 2, 4: 8, 5: 50, 6: 500},
	7:  {4: 5, 5: 20, 6: 100, 7: 1000},
	8:  {4: 3, 5: 10, 6: 50, 7: 500, 8: 2000},
	9:  {4: 2, 5: 6, 6: 25, 7: 100, 8: 1000, 9: 5000},
	10: {5: 3, 6: 15, 7: 50, 8: 200, 9: 2000, 10: 10000},
}

func cmdKeno(client *Client, args []string, _ string) {
	// Usage: /keno pick <numbers...> <bet>
	// e.g.  /keno pick 1 5 13 27 100
	if len(args) < 3 || strings.ToLower(args[0]) != "pick" {
		client.SendServerMessage("Usage: /keno pick <1-10 numbers between 1-80> <bet>")
		return
	}

	numArgs := args[1:]
	// Last arg is the bet.
	betStr := numArgs[len(numArgs)-1]
	numStrs := numArgs[:len(numArgs)-1]

	if len(numStrs) < 1 || len(numStrs) > 10 {
		client.SendServerMessage("Pick between 1 and 10 numbers.")
		return
	}

	bet, err := strconv.ParseInt(betStr, 10, 64)
	if err != nil || bet <= 0 {
		client.SendServerMessage("Invalid bet amount.")
		return
	}

	picked := make([]int, 0, len(numStrs))
	var seen [81]bool // indices 1-80; stack-allocated, avoids map overhead
	for _, s := range numStrs {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 80 {
			client.SendServerMessage(fmt.Sprintf("Invalid number %q — must be 1-80.", s))
			return
		}
		if seen[n] {
			client.SendServerMessage(fmt.Sprintf("Duplicate number: %d", n))
			return
		}
		seen[n] = true
		picked = append(picked, n)
	}

	ok, reason := validateBet(client, bet)
	if !ok {
		client.SendServerMessage(reason)
		return
	}
	balAfterBet, err := db.SpendChips(client.Ipid(), bet)
	if err != nil {
		client.SendServerMessage("Failed to place bet: " + err.Error())
		return
	}

	// Draw 20 unique numbers from 1-80.
	pool := rand.Perm(80)
	drawn := make([]int, 20)
	for i := 0; i < 20; i++ {
		drawn[i] = pool[i] + 1
	}
	// Use a fixed-size array instead of a map to mark drawn numbers.
	var drawnSet [81]bool
	for _, n := range drawn {
		drawnSet[n] = true
	}

	// Count matches.
	matches := 0
	for _, n := range picked {
		if drawnSet[n] {
			matches++
		}
	}

	payTable := kenoPayouts[len(picked)]
	mult := 0
	if payTable != nil {
		mult = payTable[matches]
	}

	var bal int64
	var result string
	if mult > 0 {
		payout := bet * int64(mult)
		bal, _ = db.AddChips(client.Ipid(), payout)
		result = fmt.Sprintf("WIN! %dx = +%d chips", mult, payout-bet)
	} else {
		bal = balAfterBet
		result = fmt.Sprintf("LOSE. -%d chips", bet)
	}

	drawnStrs := make([]string, len(drawn))
	for i, n := range drawn {
		drawnStrs[i] = strconv.Itoa(n)
	}
	pickedStrs := make([]string, len(picked))
	for i, n := range picked {
		pickedStrs[i] = strconv.Itoa(n)
	}

	client.SendServerMessage(fmt.Sprintf(
		"🎱 Keno | Picked: %s\nDrawn: %s\nMatches: %d/%d | %s | Balance: %d",
		strings.Join(pickedStrs, " "), strings.Join(drawnStrs, " "), matches, len(picked), result, bal))
	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🎱 Keno: %s matched %d/%d numbers — %s",
			client.OOCName(), matches, len(picked), result))
}

// ============================================================
// /wheel — Prize Wheel
// ============================================================

// wheelSegments defines the prize wheel: each entry is (multiplier, cumulative probability).
// Probabilities: 0x=60%, 1.5x=17%, 2x=13%, 3x=7%, 5x=2%, 10x=1%  (total=100%)
// RTP ≈ 92.5% (house edge ~7.5%), similar to a real casino prize wheel.
type wheelSegment struct {
	Label   string
	Mult    float64
	CumProb float64
}

var wheelSegments = []wheelSegment{
	{"0x (miss)", 0.0, 0.60},
	{"1.5x", 1.5, 0.77},
	{"2x", 2.0, 0.90},
	{"3x", 3.0, 0.97},
	{"5x", 5.0, 0.99},
	{"10x", 10.0, 1.00},
}

func cmdWheel(client *Client, args []string, _ string) {
	if len(args) < 2 || strings.ToLower(args[0]) != "spin" {
		client.SendServerMessage("Usage: /wheel spin <bet>")
		return
	}

	bet, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || bet <= 0 {
		client.SendServerMessage("Invalid bet amount.")
		return
	}
	ok, reason := validateBet(client, bet)
	if !ok {
		client.SendServerMessage(reason)
		return
	}
	balAfterBet, err := db.SpendChips(client.Ipid(), bet)
	if err != nil {
		client.SendServerMessage("Failed to place bet: " + err.Error())
		return
	}

	spin := rand.Float64()
	var seg wheelSegment
	for _, s := range wheelSegments {
		if spin <= s.CumProb {
			seg = s
			break
		}
	}

	payout := int64(float64(bet) * seg.Mult)
	var bal int64
	var result string
	if payout > 0 {
		bal, _ = db.AddChips(client.Ipid(), payout)
		result = fmt.Sprintf("WIN %s = +%d chips", seg.Label, payout-bet)
	} else {
		bal = balAfterBet
		result = fmt.Sprintf("Miss (0x) — lost %d chips", bet)
	}

	sendAreaGamblingMessage(client.Area(),
		fmt.Sprintf("🎡 Wheel: %s spun and got %s! %s", client.OOCName(), seg.Label, result))
	client.SendServerMessage(fmt.Sprintf(
		"🎡 Prize Wheel | Landed on: %s | %s | Balance: %d", seg.Label, result, bal))
}

// ============================================================
// /bar — Bar
// ============================================================

// barDrinkEffect holds the outcome of buying a bar drink.
type barDrinkEffect struct {
	chipDelta int64  // positive = gain, negative = loss (applied on top of the drink cost)
	msg       string // private message to the buyer
	areaMsg   string // public message broadcast to the area (empty = no broadcast)
}

const (
	barHighRollerThreshold = int64(100000) // chip balance above which the high-roller tax can apply
	barHighRollerTaxChance = 50            // percent chance the tax fires on a loss (50 = 50%)
	barHighRollerMinPct    = 5             // minimum percent of total balance lost when tax fires
	barHighRollerMaxPct    = 35            // maximum percent of total balance lost when tax fires
)

// barDrink describes a single drink available at the bar.
type barDrink struct {
	id    string
	emoji string
	cost  int64
	desc  string
	roll  func() barDrinkEffect
}

// barMenu lists all available drinks in order.
var barMenu = []barDrink{
	{
		id: "beer", emoji: "🍺", cost: 50,
		desc: "A cold pint of beer. Mostly fine — but one bad batch ruins lives.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			if r == 0 {
				loss := int64(80 + rand.Intn(121))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The beer was SKUNKED. You spit it across the bar, knock over someone's chips. -%d chips, nightmare.", loss),
					areaMsg:   "just spat skunked beer all over the poker table. 🍺🤮",
				}
			}
			gain := 60 + rand.Int63n(201)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You crack open a cold one. Ahhh, refreshing! Found some loose change in the coaster. +%d chips!", gain),
			}
		},
	},
	{
		id: "wine", emoji: "🍷", cost: 100,
		desc: "A glass of fine red wine. The sommelier has opinions. Dangerous opinions.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(150 + rand.Intn(201))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The sommelier is OFFENDED by how you hold the glass. An argument breaks out. -%d chips in damages.", loss),
					areaMsg:   "caused a wine-related incident at the bar. The sommelier is furious. 🍷😤",
				}
			}
			gain := 120 + rand.Int63n(381)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You swirl, sniff, and sip. Exquisite. The sommelier slips you a tip. +%d chips!", gain),
			}
		},
	},
	{
		id: "whiskey", emoji: "🥃", cost: 250,
		desc: "Whiskey on the rocks. Smooth and steady — until it isn't.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(300 + rand.Intn(401))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The whiskey hits different tonight. The room tilts. You bet someone you could stand up straight. You lost. -%d chips.", loss),
					areaMsg:   "bet they could stand up straight after whiskey. They could not. 🥃💀",
				}
			}
			gain := 350 + rand.Int63n(501)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You nurse the whiskey slowly. The ice clinks. Steady gains. +%d chips!", gain),
			}
		},
	},
	{
		id: "tequila", emoji: "🥃", cost: 150,
		desc: "A shot of tequila. Salt, lime, regret — or glory.",
		roll: func() barDrinkEffect {
			if rand.Intn(2) == 0 {
				gain := int64(300 + rand.Intn(701))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("YOLO! You slam the shot. Lime in the eye, but WHO CARES — you feel INVINCIBLE! +%d chips!", gain),
					areaMsg:   "is doing tequila shots and screaming victory! 🥃🍋",
				}
			}
			loss := int64(150 + rand.Intn(251))
			return barDrinkEffect{
				chipDelta: -loss,
				msg:       fmt.Sprintf("You lick salt, down the shot, and immediately regret it. The room spins. -%d chips (oops).", loss),
				areaMsg:   "just did a tequila shot and immediately fell off their stool. 😵",
			}
		},
	},
	{
		id: "vodka", emoji: "🍸", cost: 200,
		desc: "A straight shot of vodka. No chaser. No mercy.",
		roll: func() barDrinkEffect {
			r := rand.Intn(3)
			if r == 0 {
				gain := int64(700 + rand.Intn(901))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("You down it without blinking. RESPECT. Someone buys you a round back. +%d chips!", gain),
					areaMsg:   "slammed a vodka shot without even flinching. Absolute legend. 🍸",
				}
			} else if r == 1 {
				loss := int64(200 + rand.Intn(251))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("You cough. You splutter. You drop your chips. -%d chips. Should've ordered a mixer.", loss),
					areaMsg:   "coughed violently after a straight vodka shot. 😬",
				}
			}
			gain := int64(80 + rand.Intn(221))
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("Smooth. You barely feel it. A nearby gambler flips you a chip. +%d chips.", gain),
			}
		},
	},
	{
		id: "rum", emoji: "🍹", cost: 200,
		desc: "Dark rum, straight from the barrel. Arr, ye feel lucky? Pirates die a lot.",
		roll: func() barDrinkEffect {
			r := rand.Intn(3)
			if r == 0 {
				loss := int64(250 + rand.Intn(351))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The rum was cursed by a REAL pirate ghost. Your gold is gone, matey. -%d chips.", loss),
					areaMsg:   "drank cursed rum and is now haunted by a pirate ghost. 🏴‍☠️👻",
				}
			}
			gain := int64(200 + rand.Intn(701))
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("Ye raise yer glass to the sea! The pirate gods smile upon ye! +%d chips, ye scallywag!", gain),
				areaMsg:   "is channeling their inner pirate with a glass of dark rum. 🏴‍☠️",
			}
		},
	},
	{
		id: "gin", emoji: "🍸", cost: 300,
		desc: "Gin and tonic, garnished with lime. Classy — but botanicals are unpredictable.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(300 + rand.Intn(401))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The gin's juniper overtones awaken a mysterious allergy. Your face puffs up. -%d chips in medical fees.", loss),
					areaMsg:   "is having a dramatic botanical reaction to gin. Their face is doing things. 🍸🤧",
				}
			}
			gain := 450 + rand.Int63n(601)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You sip elegantly. The botanical notes dance on your tongue. Quite civilised! +%d chips!", gain),
			}
		},
	},
	{
		id: "mojito", emoji: "🍹", cost: 350,
		desc: "Fresh mint mojito. Cool, crisp, summer vibes — but the mint is sentient.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(200 + rand.Intn(351))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The mint revolts. It's everywhere. Your chips are minty and lost. -%d chips.", loss),
					areaMsg:   "was attacked by the mint in their mojito. It has achieved sentience. 🍹🌿",
				}
			}
			gain := 550 + rand.Int63n(701)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You slurp the mojito through a tiny straw. Instant paradise! +%d chips!", gain),
			}
		},
	},
	{
		id: "mead", emoji: "🍯", cost: 200,
		desc: "Ancient honey mead, brewed by monks. The monks were also brewers of chaos.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(200 + rand.Intn(301))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The monks who brewed this mead cursed it. You feel their disappointment. -%d chips, ye sinner.", loss),
					areaMsg:   "was cursed by monk-brewed mead. The monastery is displeased. ⚔️🍯😰",
				}
			}
			gain := 250 + rand.Int63n(651)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You lift the tankard and drink deep! 'TIS GOOD MEAD! +%d chips, brave warrior!", gain),
				areaMsg:   "is drinking mead like a medieval champion. ⚔️🍯",
			}
		},
	},
	{
		id: "sake", emoji: "🍶", cost: 400,
		desc: "Hot sake served in a tiny cup. Anime approved. Anime consequences included.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			if r == 0 {
				gain := int64(1500 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("*cherry blossoms fall* You close your eyes. The sake reveals your true power. NANI?! +%d chips!!", gain),
					areaMsg:   "just had a dramatic anime moment with sake and unlocked their true potential!! ✨🍶",
				}
			} else if r == 1 {
				loss := int64(400 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The sake triggers your character arc — the tragic kind. You lose chips for dramatic effect. -%d chips.", loss),
					areaMsg:   "is experiencing a dramatic anime backstory episode thanks to sake. 🍶😭",
				}
			}
			gain := int64(500 + rand.Intn(601))
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("Itadakimasu! The warm sake fills you with calm confidence. +%d chips.", gain),
			}
		},
	},
	{
		id: "champagne", emoji: "🥂", cost: 800,
		desc: "Premium champagne. Celebrate prematurely at your own risk.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(600 + rand.Intn(801))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The cork launches and shatters the casino's prized trophy. You owe restitution. -%d chips.", loss),
					areaMsg:   "popped champagne directly into the casino's antique trophy case. Staff are NOT pleased. 🥂💥",
				}
			}
			gain := 900 + rand.Int63n(1801)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("The cork POPS and flies across the room! Bubbles everywhere! Time to celebrate! +%d chips! 🥂", gain),
				areaMsg:   "is popping champagne and celebrating like they already won! 🥂✨",
			}
		},
	},
	{
		id: "margarita", emoji: "🍹", cost: 300,
		desc: "Frozen margarita. Brain freeze risk? That's the BEST case scenario.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			if r == 0 {
				loss := int64(200 + rand.Intn(301))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("BRAIN FREEZE + SPILL + CHAOS! The margarita achieves sentience and ruins your night. -%d chips.", loss),
					areaMsg:   "got a cataclysmic brain freeze from their margarita and wiped out half the bar. 🧠❄️💥",
				}
			}
			gain := 400 + rand.Int63n(601)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("Salt on the rim, perfect sip. Olé! +%d chips!", gain),
			}
		},
	},
	{
		id: "moonshine", emoji: "🫙", cost: 100,
		desc: "Illegal backwoods moonshine. Equal chance of enlightenment or oblivion.",
		roll: func() barDrinkEffect {
			if rand.Intn(2) == 0 {
				gain := int64(600 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("You take a swig. Nothing happens. Then EVERYTHING happens. You see the future! +%d chips!!!", gain),
					areaMsg:   "just drank moonshine and is now vibrating at a frequency only dogs can hear. 🫙⚡",
				}
			}
			loss := int64(300 + rand.Intn(501))
			return barDrinkEffect{
				chipDelta: -loss,
				msg:       fmt.Sprintf("That was... NOT water. You wake up three hours later with no eyebrows. -%d chips.", loss),
				areaMsg:   "drank the moonshine and is now questioning all of their life choices. 🫙💀",
			}
		},
	},
	{
		id: "absinthe", emoji: "💚", cost: 500,
		desc: "The Green Fairy. You will see things. Wonderful, terrible things.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			switch r {
			case 0:
				gain := int64(2500 + rand.Intn(3001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("The Green Fairy appears and hands you a SACK OF CHIPS! +%d chips!!! 🧚", gain),
					areaMsg:   "drank absinthe and is now having a full conversation with a fairy who is apparently VERY generous. 💚🧚",
				}
			case 1:
				loss := int64(500 + rand.Intn(801))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("The Green Fairy STEALS your chips and vanishes. '✨ Bye! ✨' -%d chips. You've been robbed by a hallucination.", loss),
					areaMsg:   "was robbed by their own absinthe hallucination. The Green Fairy strikes again. 💚😱",
				}
			case 2:
				gain := int64(600 + rand.Intn(901))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("Reality flickers. A phantom roulette table appears and you WIN. Was it real? Does it matter? +%d chips!", gain),
					areaMsg:   "just won at a ghost casino that may or may not exist. 💚🎰",
				}
			case 3:
				return barDrinkEffect{
					msg:     "You drink. Time stops. You stare at your hand for 47 minutes. Nothing happens chip-wise, but you've achieved enlightenment.",
					areaMsg: "has achieved enlightenment via absinthe and is now transcending material concerns like chips. 💚🧘",
				}
			default:
				gain := int64(900 + rand.Intn(1801))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("Somewhere between the third vision and the talking wall, you find a stash of chips. +%d chips! 💚", gain),
				}
			}
		},
	},
	{
		id: "fireball", emoji: "🔥", cost: 300,
		desc: "Fireball cinnamon whiskey. HOT HOT HOT. Can result in actual fire.",
		roll: func() barDrinkEffect {
			r := rand.Intn(3)
			if r == 0 {
				loss := int64(250 + rand.Intn(451))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🔥 IT BURNS! YOUR MOUTH IS ON FIRE! You breathe out like a dragon and accidentally singe your chips. -%d chips.", loss),
					areaMsg:   "just drank Fireball and is currently breathing fire at the bar. 🔥🐉",
				}
			}
			gain := int64(450 + rand.Intn(801))
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("🔥 YOU'RE ON FIRE! HOT STREAK ACTIVATED! The heat surges through your veins and manifests as chips! +%d chips!", gain),
				areaMsg:   "drank Fireball and is now on an absolute hot streak! 🔥💰",
			}
		},
	},
	{
		id: "jagerbomb", emoji: "💣", cost: 250,
		desc: "A Jägerbomb. Energy drink + Jäger = unpredictable consequences.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(300 + rand.Intn(501))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("💥 The energy drink and Jäger react badly. You vibrate off the barstool. -%d chips in property damage.", loss),
					areaMsg:   "vibrated off the barstool after a Jägerbomb. Everything is fine. Nothing is fine. 💣💀",
				}
			}
			gain := 300 + rand.Int63n(701)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("💥 BOOM! You feel the energy course through you! HYPERACTIVE GAMBLING ACTIVATED! +%d chips!", gain),
				areaMsg:   "just slammed a Jägerbomb and has way too much energy right now. 💣⚡",
			}
		},
	},
	{
		id: "longisland", emoji: "🧋", cost: 600,
		desc: "Long Island Iced Tea. Looks like tea. IS NOT TEA. Hits like a freight train.",
		roll: func() barDrinkEffect {
			total := int64(0)
			var parts []string
			for i := 0; i < 4+rand.Intn(4); i++ {
				delta := int64(rand.Intn(900)) - 300 // range: -300 to +599
				total += delta
				if delta >= 0 {
					parts = append(parts, fmt.Sprintf("+%d", delta))
				} else {
					parts = append(parts, fmt.Sprintf("%d", delta))
				}
			}
			summary := strings.Join(parts, ", ")
			return barDrinkEffect{
				chipDelta: total,
				msg:       fmt.Sprintf("It tastes EXACTLY like iced tea... until it doesn't. The cocktail makes several decisions for you: [%s] = %+d chips net.", summary, total),
				areaMsg:   "ordered what they THOUGHT was iced tea and is now regretting every choice that led here. 🧋😵",
			}
		},
	},
	{
		id: "cosmo", emoji: "🍸", cost: 350,
		desc: "Cosmopolitan. Pink, fabulous, and deceptively strong. Main character energy, villain arc risk.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(300 + rand.Intn(401))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("Your main character energy attracted a villain. They stole your chips while you posed. -%d chips.", loss),
					areaMsg:   "got robbed while posing dramatically with their Cosmopolitan. 🍸😒",
				}
			}
			gain := 500 + rand.Int63n(801)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("Fabulous! You sip the cosmo and feel absolutely iconic. The bar applauds. +%d chips! 🩷", gain),
				areaMsg:   "is sipping a Cosmopolitan and radiating main character energy. 🍸🩷",
			}
		},
	},
	{
		id: "pina", emoji: "🍍", cost: 400,
		desc: "Piña Colada. Tropical vibes but beware: the beach can also have sharks.",
		roll: func() barDrinkEffect {
			r := rand.Intn(4)
			if r == 0 {
				loss := int64(300 + rand.Intn(451))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("You imagined a beach so vividly you forgot to hold onto your chips. -%d chips, gone with the tide.", loss),
					areaMsg:   "is now SO relaxed from the Piña Colada that their chips slipped into the imaginary ocean. 🍍🌊",
				}
			}
			gain := 450 + rand.Int63n(751)
			return barDrinkEffect{
				chipDelta: gain,
				msg:       fmt.Sprintf("You close your eyes and imagine a beach. The bartender snaps you out of it but slides you some chips. +%d chips! 🏖️", gain),
			}
		},
	},
	{
		id: "mystery", emoji: "❓", cost: 1000,
		desc: "The Mystery Brew. Nobody knows what's in it. Not even the bartender. Extreme variance.",
		roll: func() barDrinkEffect {
			r := rand.Intn(10)
			switch {
			case r <= 1: // 20%: big jackpot
				gain := int64(6000 + rand.Intn(14001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("❓ The brew GLOWS. Your eyes go white. You levitate slightly. When you land, there are %d chips in your pocket. WHAT WAS IN THAT THING?!", gain),
					areaMsg:   "just drank the Mystery Brew and ascended to a higher plane of chip ownership. ❓✨💰",
				}
			case r <= 3: // 20%: big loss
				loss := int64(700 + rand.Intn(1501))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("❓ The brew tastes like despair, old copper coins, and something that might have been alive. -%d chips just... disappear. Gone. Into the void.", loss),
					areaMsg:   "drank the Mystery Brew and something unspeakable happened. ❓💀",
				}
			case r <= 5: // 20%: nothing
				return barDrinkEffect{
					msg:     "❓ The brew looks ominous. You sip it cautiously. It tastes like... tap water? Nothing happens. You've been cheated by the universe.",
					areaMsg: "drank the Mystery Brew. Nothing happened. They seem deeply unsatisfied. ❓🤷",
				}
			case r <= 7: // 20%: moderate gain
				gain := int64(1500 + rand.Intn(3001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("❓ The brew shimmers. You hear distant chanting. Chips materialize from thin air. +%d chips. Don't question it.", gain),
				}
			default: // 20%: small gain + wacky message
				gain := int64(600 + rand.Intn(801))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("❓ You taste elderflower, lightning, three kinds of cheese, and existential dread. Somehow, +%d chips. How. WHY.", gain),
					areaMsg:   "just experienced something profoundly weird via the Mystery Brew. ❓🧪",
				}
			}
		},
	},
	// ── New high-variance drinks ──────────────────────────────────────────────
	{
		id: "poison", emoji: "☠️", cost: 50,
		desc: "A suspiciously colored cocktail. 85% chance you lose big. 15% chance you hit jackpot.",
		roll: func() barDrinkEffect {
			if rand.Intn(100) < 15 {
				gain := int64(3000 + rand.Intn(7001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("Against ALL odds, the poison HEALS you! The bartender is in shock. +%d chips!!! ☠️🎉", gain),
					areaMsg:   "somehow SURVIVED the poison cocktail and is looking suspiciously healthy and rich. ☠️💪",
				}
			}
			loss := int64(200 + rand.Intn(600))
			return barDrinkEffect{
				chipDelta: -loss,
				msg:       fmt.Sprintf("You knew the risks. The poison did its job. -%d chips as your chips slowly drain away with your dignity.", loss),
				areaMsg:   "ordered the Poison cocktail and is currently paying the consequences. ☠️😵",
			}
		},
	},
	{
		id: "doubletrouble", emoji: "🃏", cost: 500,
		desc: "Pure coin-flip energy. Win 3x or lose 60% of the cost on top. No in-between.",
		roll: func() barDrinkEffect {
			if rand.Intn(2) == 0 {
				gain := int64(1500 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("DOUBLE TROUBLE PAYS OFF! You slam both glasses and victory is YOURS! +%d chips! 🃏🔥", gain),
					areaMsg:   "ordered Double Trouble and DOUBLED UP spectacularly! 🃏🔥",
				}
			}
			loss := int64(400 + rand.Intn(601))
			return barDrinkEffect{
				chipDelta: -loss,
				msg:       fmt.Sprintf("Double trouble means double the consequences. You pay extra for the privilege of losing. -%d chips.", loss),
				areaMsg:   "tried Double Trouble and received exactly double the trouble. 🃏😢",
			}
		},
	},
	{
		id: "dragonblood", emoji: "🐉", cost: 750,
		desc: "Infused with something ancient and angry. Scorching outcomes at both ends.",
		roll: func() barDrinkEffect {
			r := rand.Intn(6)
			switch r {
			case 0:
				gain := int64(4000 + rand.Intn(6001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🐉 THE DRAGON BLESSES YOU! You breathe fire and chips rain from the sky! +%d chips!!", gain),
					areaMsg:   "drank Dragon Blood and is now literally breathing fire and chips. 🐉🔥💰",
				}
			case 1:
				loss := int64(700 + rand.Intn(1001))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🐉 The dragon SCORCHES your chip pile. You watch them burn. -%d chips. This is fine.", loss),
					areaMsg:   "drank Dragon Blood and the dragon ate their chips. 🐉💀",
				}
			case 2, 3:
				gain := int64(1000 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🐉 The fire fills your belly and your wallet. +%d chips!", gain),
					areaMsg:   "is glowing suspiciously after Dragon Blood. 🐉✨",
				}
			default:
				loss := int64(300 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🐉 The dragon disagrees with you personally. -%d chips.", loss),
					areaMsg:   "had a disagreement with the Dragon Blood. The dragon won. 🐉😤",
				}
			}
		},
	},
	{
		id: "cursedwine", emoji: "🍾", cost: 600,
		desc: "A vintage from a haunted vineyard. The curse is the whole point.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			switch r {
			case 0:
				gain := int64(5000 + rand.Intn(5001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🍾 The curse REVERSES! The haunted vineyard blesses you in return for your bravery! +%d chips!", gain),
					areaMsg:   "broke the curse of the Haunted Vineyard and received a divine blessing of chips. 🍾✨",
				}
			case 1:
				loss := int64(800 + rand.Intn(1201))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🍾 The curse activates. Your chips vanish, your drink floats away, and a ghost laughs at you. -%d chips.", loss),
					areaMsg:   "was fully cursed by the Haunted Vineyard wine. The ghost is gleeful. 🍾👻",
				}
			case 2:
				loss := int64(200 + rand.Intn(401))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🍾 The curse is mild today. Only partial haunting. -%d chips.", loss),
				}
			case 3:
				gain := int64(800 + rand.Intn(1601))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🍾 The ghost decides to be generous this evening. +%d chips from the spirit realm!", gain),
					areaMsg:   "received chips from a ghost via Cursed Wine. Haunted economy is booming. 🍾👻💰",
				}
			default:
				return barDrinkEffect{
					msg:     "🍾 The curse is... confused. Nothing happens. You exist in an uncomfortable liminal space. The chips remain where they are.",
					areaMsg: "drank Cursed Wine and ended up in a liminal cursed space. Everything is fine. 🍾😶",
				}
			}
		},
	},
	{
		id: "goldenelixir", emoji: "✨", cost: 2000,
		desc: "A legendary brew made from pure luck. Huge cost. Catastrophic or godlike payout.",
		roll: func() barDrinkEffect {
			r := rand.Intn(10)
			switch {
			case r == 0: // 10%: legendary jackpot
				gain := int64(20000 + rand.Intn(30001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("✨ THE ELIXIR WORKS! GOLDEN LIGHT EVERYWHERE! You are CHOSEN! +%d chips!!! This is the best day of your life!", gain),
					areaMsg:   "drank the Golden Elixir and ASCENDED. Chips are raining from the ceiling. ✨💰🌟",
				}
			case r <= 3: // 30%: big loss
				loss := int64(2000 + rand.Intn(3001))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("✨ The elixir demands a tribute. Your chips are sacrificed to the golden gods. -%d chips. An expensive lesson.", loss),
					areaMsg:   "sacrificed a fortune to the Golden Elixir and received nothing in return. ✨💀",
				}
			case r <= 6: // 30%: good gain
				gain := int64(4000 + rand.Intn(6001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("✨ The elixir shines. Fortune favors the bold (and the wealthy). +%d chips! ✨", gain),
					areaMsg:   "had a golden moment with the Golden Elixir. ✨💰",
				}
			default: // 30%: moderate gain
				gain := int64(2000 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("✨ The elixir is... okay. Slightly magical. A modest glow and +%d chips.", gain),
				}
			}
		},
	},
	{
		id: "roulettebrew", emoji: "🔴", cost: 400,
		desc: "Each sip is a different roulette outcome. Literally. Pure roulette in a glass.",
		roll: func() barDrinkEffect {
			pocket := rand.Intn(37) // 0-36 European roulette
			if pocket == 0 {
				loss := int64(400 + rand.Intn(401))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🔴 Zero! The house wins. Again. -%d chips. You stare into the void.", loss),
					areaMsg:   "spun the Roulette Brew and hit zero. The house always wins. 🔴😩",
				}
			}
			if rouletteRedNumbers[pocket] {
				gain := int64(600 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🔴 Red %d! Hot winning streak! +%d chips!", pocket, gain),
					areaMsg:   fmt.Sprintf("hit Red %d on the Roulette Brew! 🔴🎉", pocket),
				}
			}
			loss := int64(200 + rand.Intn(401))
			return barDrinkEffect{
				chipDelta: -loss,
				msg:       fmt.Sprintf("⚫ Black %d. Cold. -%d chips fall into the dark.", pocket, loss),
				areaMsg:   fmt.Sprintf("hit Black %d on the Roulette Brew. Dark times. ⚫😶", pocket),
			}
		},
	},
	{
		id: "blackout", emoji: "🌑", cost: 300,
		desc: "You will not remember ordering this. You will not remember the outcome. Results vary WILDLY.",
		roll: func() barDrinkEffect {
			r := rand.Intn(8)
			switch r {
			case 0:
				gain := int64(3000 + rand.Intn(7001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🌑 You wake up. You don't know what happened. But there are %d extra chips in your pocket. You'll never know why.", gain),
					areaMsg:   "woke up from the Blackout drink with a suspiciously large chip stack. No questions. 🌑💰",
				}
			case 1:
				loss := int64(400 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🌑 You wake up. You don't know what happened. But -%d chips are missing. Probably for the best that you don't remember.", loss),
					areaMsg:   "woke up from the Blackout drink significantly poorer. No memory of what happened. 🌑💸",
				}
			case 2, 3:
				gain := int64(500 + rand.Intn(1001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🌑 Fragments. Lights. Cheering. +%d chips. You accept this.", gain),
				}
			case 4, 5:
				loss := int64(100 + rand.Intn(401))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🌑 Static. Darkness. A receipt for -%d chips. Unclear.", loss),
				}
			case 6:
				return barDrinkEffect{
					msg:     "🌑 You blink. You're still at the bar. Nothing happened. The bartender refuses to make eye contact.",
					areaMsg: "drank Blackout and absolutely nothing happened. The bartender seems relieved. 🌑🤫",
				}
			default:
				gain := int64(2000 + rand.Intn(3001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🌑 You have no memory of this. But your balance is +%d chips. Dream logic.", gain),
					areaMsg:   "gained chips via Blackout that defy rational explanation. 🌑🎆",
				}
			}
		},
	},
	{
		id: "thundermead", emoji: "⚡", cost: 450,
		desc: "Electrified mead. 5x the voltage of regular mead. May cause literal sparks.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			switch r {
			case 0:
				gain := int64(4000 + rand.Intn(4001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("⚡ THUNDER! LIGHTNING! The gods of Asgard ROAR with approval! +%d chips, WARRIOR! ⚡🍯", gain),
					areaMsg:   "drank Thunder Mead and Thor himself showed up to hand them chips. ⚡🍯👑",
				}
			case 1:
				loss := int64(500 + rand.Intn(801))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("⚡ The electricity surges THROUGH you. Your chips arc across the table. -%d chips, scattered to the winds.", loss),
					areaMsg:   "got struck by lightning FROM the Thunder Mead. Chips everywhere. ⚡💀",
				}
			case 2:
				gain := int64(1000 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("⚡ A moderate jolt. Exhilarating! You feel powerful. +%d chips!", gain),
					areaMsg:   "is vibrating gently after Thunder Mead. In a good way. ⚡🍯",
				}
			case 3:
				loss := int64(200 + rand.Intn(351))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("⚡ The mead sparks unexpectedly. A small surge singes your wallet. -%d chips.", loss),
				}
			default:
				gain := int64(600 + rand.Intn(801))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("⚡ The thunder rumbles distantly but cooperates. +%d chips, shaken but stirred.", gain),
				}
			}
		},
	},
	{
		id: "devilswhiskey", emoji: "😈", cost: 350,
		desc: "Brewed in hellfire. The devil gets his cut. Usually a big cut.",
		roll: func() barDrinkEffect {
			r := rand.Intn(6)
			switch r {
			case 0:
				gain := int64(5000 + rand.Intn(10001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("😈 The devil is impressed. He makes you a DEAL and gives you %d chips. This probably has consequences later.", gain),
					areaMsg:   "made a deal with the Devil via Devil's Whiskey. Short-term chip gain, long-term... unclear. 😈💰",
				}
			case 1, 2, 3:
				loss := int64(400 + rand.Intn(701))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("😈 The devil collects his tithe. -%d chips go directly to Hell's treasury. Unavoidable.", loss),
					areaMsg:   "paid the devil's tithe via Devil's Whiskey. He thanks you. 😈💀",
				}
			case 4:
				loss := int64(1000 + rand.Intn(2001))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("😈 The devil is GREEDY today. Major tithe. -%d chips. You have been taxed by Hell.", loss),
					areaMsg:   "was aggressively taxed by the devil through Devil's Whiskey. A LOT of chips gone. 😈💸",
				}
			default:
				gain := int64(500 + rand.Intn(1001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("😈 The devil is in a surprisingly good mood. +%d chips. Don't read into it.", gain),
				}
			}
		},
	},
	{
		id: "angelwine", emoji: "👼", cost: 800,
		desc: "Blessed by a celestial being. Mostly positive but angels are STRICT about worthiness.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			switch r {
			case 0:
				gain := int64(6000 + rand.Intn(9001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("👼 THE ANGEL DEEMS YOU WORTHY! Celestial chips rain from on high! +%d chips! Hallelujah!", gain),
					areaMsg:   "was deemed WORTHY by the Angel Wine and blessed with a divine chip shower. 👼✨💰",
				}
			case 1:
				loss := int64(1000 + rand.Intn(1501))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("👼 The angel is DISAPPOINTED in your gambling habits. -%d chips confiscated as penance.", loss),
					areaMsg:   "was judged by the Angel Wine and found lacking. Penance chips extracted. 👼😔",
				}
			case 2:
				gain := int64(2000 + rand.Intn(3001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("👼 The angel smiles warmly. You are forgiven for all previous bad bets. +%d chips.", gain),
					areaMsg:   "received celestial forgiveness via Angel Wine. And chips. Lots of chips. 👼💫",
				}
			case 3:
				gain := int64(800 + rand.Intn(1201))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("👼 The angel offers modest celestial guidance. +%d chips.", gain),
				}
			default:
				loss := int64(400 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("👼 The angel frowns. 'Really? Gambling again?' -%d chips taken as a lesson.", loss),
					areaMsg:   "got lectured by the Angel Wine. Chips docked. 👼😤",
				}
			}
		},
	},
	{
		id: "ghostshot", emoji: "👻", cost: 200,
		desc: "A spectral shot. The ghost decides your fate. Ghosts are unpredictable.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			switch r {
			case 0:
				gain := int64(2000 + rand.Intn(4001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("👻 The ghost is GENEROUS today! It leads you to buried chip treasure! +%d chips, haunted windfall!", gain),
					areaMsg:   "was guided to buried chip treasure by the Ghost Shot spirit. 👻💰",
				}
			case 1:
				loss := int64(300 + rand.Intn(501))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("👻 The ghost is mischievous. It flings your chips through the wall. Gone. -%d chips.", loss),
					areaMsg:   "had their chips flung through a wall by the Ghost Shot spirit. 👻💀",
				}
			case 2:
				return barDrinkEffect{
					msg:     "👻 The ghost stares at you for an unsettling amount of time. Then it leaves. Nothing happens. You feel watched.",
					areaMsg: "is being stared at by the Ghost Shot spirit. It's just... standing there. 👻🤔",
				}
			case 3:
				gain := int64(400 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("👻 The ghost rattles some chip machines loose. +%d chips fall out for you! 👻🎰", gain),
				}
			default:
				loss := int64(100 + rand.Intn(301))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("👻 Boo! You flinch and drop chips. -%d chips. The ghost laughs (or whatever ghosts do).", loss),
					areaMsg:   "got spooked by Ghost Shot and dropped their chips. 👻😱",
				}
			}
		},
	},
	{
		id: "electriclemonade", emoji: "⚡🍋", cost: 350,
		desc: "When life gives you lemons, they electrocute you. Massive variance.",
		roll: func() barDrinkEffect {
			r := rand.Intn(5)
			switch r {
			case 0:
				gain := int64(3500 + rand.Intn(4001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("⚡🍋 THE VOLTAGE IS UNREAL! You are FULLY CHARGED! +%d chips! BZZT BZZT!", gain),
					areaMsg:   "is fully electrically charged from the Electric Lemonade. Chips sparking everywhere. ⚡🍋💰",
				}
			case 1:
				loss := int64(400 + rand.Intn(601))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("⚡🍋 The lemon zaps you directly in the chips. -%d chips discharged involuntarily.", loss),
					areaMsg:   "was directly zapped by Electric Lemonade. Chips discharged. ⚡🍋💸",
				}
			case 2, 3:
				gain := int64(700 + rand.Intn(1201))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("⚡🍋 A pleasant tingle and a chip surge. +%d chips!", gain),
				}
			default:
				loss := int64(200 + rand.Intn(351))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("⚡🍋 Unexpected shock. Dropped some chips. -%d chips. The lemonade doesn't apologize.", loss),
				}
			}
		},
	},
	{
		id: "voiddrink", emoji: "🌀", cost: 1500,
		desc: "It contains nothing. It IS nothing. The void stares back. High risk, reality-bending reward.",
		roll: func() barDrinkEffect {
			r := rand.Intn(10)
			switch {
			case r == 0: // 10%: void jackpot
				gain := int64(15000 + rand.Intn(20001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🌀 The void gives back. You reach into the nothing and pull out EVERYTHING. +%d chips. The void is generous today.", gain),
					areaMsg:   "reached into the Void Drink and pulled out an incomprehensible amount of chips. 🌀💰🌌",
				}
			case r <= 3: // 30%: void consumes
				loss := int64(1500 + rand.Intn(2501))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🌀 The void consumes. Your chips spiral inward and vanish. -%d chips. This is expected.", loss),
					areaMsg:   "lost chips to the Void Drink. They went somewhere beyond space and time. 🌀💀",
				}
			case r <= 6: // 30%: moderate void reward
				gain := int64(2000 + rand.Intn(5001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🌀 The void offers a fragment of its infinite wealth. +%d chips, pulled from the nothing.", gain),
					areaMsg:   "extracted chips from the void. The laws of physics are disappointed. 🌀✨",
				}
			default: // 30%: nothing
				return barDrinkEffect{
					msg:     "🌀 The void gives nothing. The void takes nothing. You stare into the void. The void stares back. Nothing changes.",
					areaMsg: "drank the Void Drink and received exactly what the void promises. Nothing. 🌀😶",
				}
			}
		},
	},
	{
		id: "luckybrew", emoji: "🍀", cost: 250,
		desc: "Three-leaf or four-leaf? Every sip is a luck roll. Massive swings.",
		roll: func() barDrinkEffect {
			r := rand.Intn(6)
			switch r {
			case 0: // four-leaf clover
				gain := int64(3000 + rand.Intn(5001))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🍀 FOUR-LEAF CLOVER! The universe bends in your favor! +%d chips! Today is YOUR day!", gain),
					areaMsg:   "found a four-leaf clover in the Lucky Brew. Fortune EXPLODES. 🍀💰💰",
				}
			case 1: // three-leaf (bad luck)
				loss := int64(300 + rand.Intn(501))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🍀 Three leaves. Classic three-leaf bad luck. The universe shrugs. -%d chips.", loss),
					areaMsg:   "drew a three-leaf clover from the Lucky Brew. The luck is very bad. 🍀😬",
				}
			case 2, 3:
				gain := int64(400 + rand.Intn(701))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🍀 Decent luck today! A modest bloom. +%d chips!", gain),
				}
			case 4:
				loss := int64(150 + rand.Intn(351))
				return barDrinkEffect{
					chipDelta: -loss,
					msg:       fmt.Sprintf("🍀 The luck is... backwards today. -%d chips.", loss),
				}
			default:
				gain := int64(1200 + rand.Intn(1801))
				return barDrinkEffect{
					chipDelta: gain,
					msg:       fmt.Sprintf("🍀 A surprisingly strong four-leaf bloom! +%d chips! Fortune favors!", gain),
					areaMsg:   "hit the lucky streak with Lucky Brew! 🍀🌟",
				}
			}
		},
	},
}

// barDrinkIndex maps drink ID to *barDrink for O(1) lookup.
var barDrinkIndex = func() map[string]*barDrink {
	m := make(map[string]*barDrink, len(barMenu))
	for i := range barMenu {
		m[barMenu[i].id] = &barMenu[i]
	}
	return m
}()

// barMenuBody is the static portion of the /bar menu output, pre-computed once at package
// init to avoid rebuilding the string on every /bar menu invocation.
var barMenuBody string

func init() {
	var sb strings.Builder
	sb.Grow(4096)
	sb.WriteString("  ⚠️  ALL drinks carry RISK — big variance, big potential gains AND losses!\n\n")
	sb.WriteString(fmt.Sprintf("  %-16s %-6s  %s\n", "DRINK", "COST", "DESCRIPTION"))
	sb.WriteString("  ──────────────────────────────────────────────────────────────────\n")
	for _, d := range barMenu {
		sb.WriteString(fmt.Sprintf("  %s %-14s %-6d  %s\n", d.emoji, d.id, d.cost, d.desc))
	}
	sb.WriteString(fmt.Sprintf("\n  %d drinks total — Use /bar buy <drink> to order!\n", len(barMenu)))
	sb.WriteString("  Tip: higher cost drinks have wilder swings!\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════════\n")
	barMenuBody = sb.String()
}

func printBarMenu(client *Client) {
	bal, _ := db.GetChipBalance(client.Ipid())
	client.SendServerMessage(
		"\n🍻 ═══════════ THE NYATHENA BAR ═══════════ 🍻\n" +
			fmt.Sprintf("  Your balance: %d chips\n", bal) +
			barMenuBody)
}

func cmdBar(client *Client, args []string, _ string) {
	if len(args) == 0 || strings.ToLower(args[0]) == "menu" {
		printBarMenu(client)
		return
	}
	if strings.ToLower(args[0]) != "buy" || len(args) < 2 {
		client.SendServerMessage("Usage: /bar menu | /bar buy <drink>")
		return
	}

	drinkID := strings.ToLower(args[1])
	drink, ok := barDrinkIndex[drinkID]
	if !ok {
		client.SendServerMessage(fmt.Sprintf("Unknown drink '%s'. Use /bar menu to see what's available.", drinkID))
		return
	}

	// Enforce the 20-second cooldown between drink purchases.
	if limited, remaining := client.CheckBarDrinkCooldown(); limited {
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("You're still feeling the last drink. Wait %d %s before ordering another.", remaining, unit))
		return
	}

	// Deduct the drink cost first.
	bal, err := db.SpendChips(client.Ipid(), drink.cost)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("You can't afford that drink! Your balance: %d chips (cost: %d).", bal, drink.cost))
		return
	}

	// Start the cooldown now that the purchase is confirmed.
	client.SetLastBarDrinkTime()

	// Roll the effect.
	effect := drink.roll()

	// Apply net chip delta (could be positive gain or additional loss).
	finalBal := bal
	if effect.chipDelta > 0 {
		finalBal, _ = db.AddChips(client.Ipid(), effect.chipDelta)
	} else if effect.chipDelta < 0 {
		spent := -effect.chipDelta
		// High rollers (pre-drink balance >100k) have a 50% chance of losing 5–35% of
		// their total balance instead of the normal drink penalty, to prevent farming.
		preDrinkBal := bal + drink.cost
		if preDrinkBal > barHighRollerThreshold && rand.Intn(100) < barHighRollerTaxChance {
			pct := int64(barHighRollerMinPct + rand.Intn(barHighRollerMaxPct-barHighRollerMinPct+1))
			spent = preDrinkBal * pct / 100
			effect.msg += fmt.Sprintf(" 💸 High roller reckoning: you lose %d%% of your pre-drink balance — %d chips gone!", pct, spent)
		}
		newBal, spendErr := db.SpendChips(client.Ipid(), spent)
		if spendErr != nil {
			// Not enough chips for the penalty — drain to zero instead.
			finalBal = 0
			db.SpendChips(client.Ipid(), bal) //nolint:errcheck
		} else {
			finalBal = newBal
		}
	}

	// Broadcast to area if the drink has a public message.
	if effect.areaMsg != "" {
		sendAreaGamblingMessage(client.Area(),
			fmt.Sprintf("🍻 %s %s", client.OOCName(), effect.areaMsg))
	}

	client.SendServerMessage(fmt.Sprintf(
		"%s %s\n%s\nBalance: %d chips", drink.emoji, drink.id, effect.msg, finalBal))
}
