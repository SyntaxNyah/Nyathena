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

// rouletteRedNumbers contains the 18 red pockets on a European wheel.
var rouletteRedNumbers = map[int]bool{
	1: true, 3: true, 5: true, 7: true, 9: true, 12: true,
	14: true, 16: true, 18: true, 19: true, 21: true, 23: true,
	25: true, 27: true, 30: true, 32: true, 34: true, 36: true,
}

func cmdCasinoRoulette(client *Client, args []string, _ string) {
	if !casinoCheck(client) {
		return
	}
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
	_, err = db.SpendChips(client.Ipid(), amount)
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

	var payout int64
	var result string
	if win {
		payout = amount * payoutMult
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		result = fmt.Sprintf("WIN! +%d chips", payout-amount)
	} else {
		result = fmt.Sprintf("LOSE. -%d chips", amount)
	}

	bal, _ := db.GetChipBalance(client.Ipid())
	sendAreaServerMessage(client.Area(),
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
	if !casinoCheck(client) {
		return
	}
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
	_, err = db.SpendChips(client.Ipid(), amount)
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
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		result = fmt.Sprintf("WIN! +%d chips", payout-amount)
	default:
		result = fmt.Sprintf("LOSE. -%d chips", amount)
	}

	bal, _ := db.GetChipBalance(client.Ipid())
	sendAreaServerMessage(client.Area(),
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
	if !casinoCheck(client) {
		return
	}
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
	_, err = db.SpendChips(client.Ipid(), amount)
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
		sendAreaServerMessage(client.Area(),
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
	var payout int64
	var result string
	if win {
		payout = amount * 2
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		result = fmt.Sprintf("WIN! +%d chips", amount)
	} else {
		result = fmt.Sprintf("LOSE. -%d chips", amount)
	}

	bal, _ := db.GetChipBalance(client.Ipid())
	outcome := "pass"
	if !passWin {
		outcome = "don't-pass"
	}
	sendAreaServerMessage(client.Area(),
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
const (
	crashMinMultiplier = 1.2
	crashMaxMultiplier = 20.0
	crashGrowthPerSec  = 0.1 // multiplier increase per second
)
type CrashState struct {
	Bet       int64
	StartTime time.Time
	CrashAt   float64 // multiplier at which the game crashes
	Active    bool
}

// playerCrashStates maps uid → *CrashState.
var playerCrashStates sync.Map

func cmdCrash(client *Client, args []string, _ string) {
	if !casinoCheck(client) {
		return
	}
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
		_, err = db.SpendChips(client.Ipid(), amount)
		if err != nil {
			client.SendServerMessage("Failed to place bet: " + err.Error())
			return
		}

		// Crash point: random between crashMinMultiplier and crashMaxMultiplier.
		crashAt := crashMinMultiplier + rand.Float64()*(crashMaxMultiplier-crashMinMultiplier)

		state := &CrashState{
			Bet:       amount,
			StartTime: time.Now(),
			CrashAt:   crashAt,
			Active:    true,
		}
		playerCrashStates.Store(client.Uid(), state)

		client.SendServerMessage(fmt.Sprintf(
			"🚀 Crash started! Bet: %d chips. Multiplier growing at 0.1x/sec. Use /crash cashout to cash out!",
			amount))

	case "cashout":
		val, ok := playerCrashStates.Load(client.Uid())
		if !ok || !val.(*CrashState).Active {
			client.SendServerMessage("You have no active crash game.")
			return
		}
		state := val.(*CrashState)
		state.Active = false

		elapsed := time.Since(state.StartTime).Seconds()
		current := 1.0 + elapsed*crashGrowthPerSec

		bal, _ := db.GetChipBalance(client.Ipid())

		if current >= state.CrashAt {
			// Already crashed.
			sendAreaServerMessage(client.Area(),
				fmt.Sprintf("💥 Crash! %s's game already crashed at %.2fx (too late to cash out).",
					client.OOCName(), state.CrashAt))
			client.SendServerMessage(fmt.Sprintf(
				"💥 Too late! The game crashed at %.2fx. You lost %d chips. Balance: %d",
				state.CrashAt, state.Bet, bal))
			return
		}

		payout := int64(float64(state.Bet) * current)
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		bal, _ = db.GetChipBalance(client.Ipid())
		sendAreaServerMessage(client.Area(),
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
	if !casinoCheck(client) {
		return
	}
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
		_, err = db.SpendChips(client.Ipid(), bet)
		if err != nil {
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
			bal, _ := db.GetChipBalance(client.Ipid())
			client.SendServerMessage(fmt.Sprintf(
				"💥 BOOM! Cell %d was a mine! You lose %d chips. Balance: %d",
				cell, state.Bet, bal))
			sendAreaServerMessage(client.Area(),
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

		mult := minesMultiplier(state.SafePicks, state.MineCount)
		payout := int64(float64(state.Bet) * mult)
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		bal, _ := db.GetChipBalance(client.Ipid())
		sendAreaServerMessage(client.Area(),
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
		bal, _ := db.GetChipBalance(client.Ipid())
		client.SendServerMessage(fmt.Sprintf(
			"Quit mines. You forfeited your bet of %d chips. Balance: %d", state.Bet, bal))

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
	if !casinoCheck(client) {
		return
	}
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
	seen := map[int]bool{}
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
	_, err = db.SpendChips(client.Ipid(), bet)
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
	drawnSet := map[int]bool{}
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

	var payout int64
	var result string
	if mult > 0 {
		payout = bet * int64(mult)
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		result = fmt.Sprintf("WIN! %dx = +%d chips", mult, payout-bet)
	} else {
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

	bal, _ := db.GetChipBalance(client.Ipid())
	client.SendServerMessage(fmt.Sprintf(
		"🎱 Keno | Picked: %s\nDrawn: %s\nMatches: %d/%d | %s | Balance: %d",
		strings.Join(pickedStrs, " "), strings.Join(drawnStrs, " "), matches, len(picked), result, bal))
	sendAreaServerMessage(client.Area(),
		fmt.Sprintf("🎱 Keno: %s matched %d/%d numbers — %s",
			client.OOCName(), matches, len(picked), result))
}

// ============================================================
// /wheel — Prize Wheel
// ============================================================

// wheelSegments defines the prize wheel: each entry is (multiplier, cumulative probability).
// Probabilities: 2x=30%, 1.5x=25%, 0x=20%, 3x=15%, 5x=8%, 10x=2%  (total=100%)
type wheelSegment struct {
	Label   string
	Mult    float64
	CumProb float64
}

var wheelSegments = []wheelSegment{
	{"2x", 2.0, 0.30},
	{"1.5x", 1.5, 0.55},
	{"0x (miss)", 0.0, 0.75},
	{"3x", 3.0, 0.90},
	{"5x", 5.0, 0.98},
	{"10x", 10.0, 1.00},
}

func cmdWheel(client *Client, args []string, _ string) {
	if !casinoCheck(client) {
		return
	}
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
	_, err = db.SpendChips(client.Ipid(), bet)
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
	var result string
	if payout > 0 {
		db.AddChips(client.Ipid(), payout) //nolint:errcheck
		result = fmt.Sprintf("WIN %s = +%d chips", seg.Label, payout-bet)
	} else {
		result = fmt.Sprintf("Miss (0x) — lost %d chips", bet)
	}

	bal, _ := db.GetChipBalance(client.Ipid())
	sendAreaServerMessage(client.Area(),
		fmt.Sprintf("🎡 Wheel: %s spun and got %s! %s", client.OOCName(), seg.Label, result))
	client.SendServerMessage(fmt.Sprintf(
		"🎡 Prize Wheel | Landed on: %s | %s | Balance: %d", seg.Label, result, bal))
}
