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

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
)

// PokerPlayer holds one player's state at the poker table.
type PokerPlayer struct {
	Client  *Client
	Hand    []Card  // 2 hole cards
	Chips   int64   // table session stack (starts at buy-in)
	Bet     int64   // amount committed this betting round
	Folded  bool
	AllIn   bool
	Ready   bool // pressed ready to start
	SitOut  bool
}

// PokerRound represents the current street of a Texas Hold'em hand.
type PokerRound int

const (
	PokerWaiting  PokerRound = iota
	PokerPreflop
	PokerFlop
	PokerTurn
	PokerRiver
	PokerShowdown
)

// pokerReshuffleThreshold is the minimum number of cards remaining in the deck
// before a fresh deck is shuffled in. Chosen to ensure enough cards for a full
// deal (up to 9 players × 2 hole cards + 5 community cards = 23 cards worst case).
const pokerReshuffleThreshold = 25

// PokerTable holds the complete state for one Texas Hold'em table.
type PokerTable struct {
	mu         sync.Mutex
	area       *area.Area
	state      PokerRound
	players    []*PokerPlayer
	deck       []Card
	community  []Card // up to 5 community cards
	pot        int64
	currentBet int64  // highest bet in current street
	dealerIdx  int
	turnIdx    int
	smallBlind int64
	bigBlind   int64
	timer      *time.Timer
	buyIn      int64
	lastRaiser int // index of last player who raised; used to detect round-end
}

// --- internal helpers; callers must hold table.mu ---

func (t *PokerTable) drawCard() Card {
	if len(t.deck) < pokerReshuffleThreshold {
		t.deck = newDeck(1)
	}
	c := t.deck[0]
	t.deck = t.deck[1:]
	return c
}

func pokerFindPlayer(table *PokerTable, client *Client) *PokerPlayer {
	for _, p := range table.players {
		if p.Client == client {
			return p
		}
	}
	return nil
}

func pokerActivePlayers(table *PokerTable) []*PokerPlayer {
	var active []*PokerPlayer
	for _, p := range table.players {
		if !p.Folded && !p.SitOut {
			active = append(active, p)
		}
	}
	return active
}

func pokerResetTimer(table *PokerTable) {
	if table.timer != nil {
		table.timer.Stop()
	}
	table.timer = time.AfterFunc(60*time.Second, func() {
		table.mu.Lock()
		defer table.mu.Unlock()
		if table.state == PokerWaiting || table.state == PokerShowdown {
			return
		}
		if table.turnIdx < len(table.players) {
			p := table.players[table.turnIdx]
			if !p.Folded && !p.AllIn {
				p.Folded = true
				sendAreaGamblingMessage(table.area,
					fmt.Sprintf("%s timed out and folds.", p.Client.OOCName()))
				pokerAdvanceTurn(table)
			}
		}
	})
}

// pokerAdvanceTurn moves to the next eligible player or advances the street.
// Assumes table.mu is held.
func pokerAdvanceTurn(table *PokerTable) {
	active := pokerActivePlayers(table)

	// If only one player remains, they win the pot.
	if len(active) == 1 {
		pokerAwardPot(table, active[0])
		return
	}

	// Check if all active (non-all-in) players have matched the current bet.
	allSettled := true
	for _, p := range active {
		if !p.AllIn && (p.Bet < table.currentBet) {
			allSettled = false
			break
		}
	}
	// Also check if we've gone all the way around to the last raiser.
	if allSettled {
		nonAllIn := 0
		for _, p := range active {
			if !p.AllIn {
				nonAllIn++
			}
		}
		if nonAllIn <= 1 {
			allSettled = true
		}
	}

	if allSettled {
		pokerNextStreet(table)
		return
	}

	// Advance turnIdx to next eligible player.
	n := len(table.players)
	for i := 1; i <= n; i++ {
		idx := (table.turnIdx + i) % n
		p := table.players[idx]
		if !p.Folded && !p.AllIn && !p.SitOut {
			table.turnIdx = idx
			p.Client.SendServerMessage(fmt.Sprintf(
				"Your turn! Stack: %d | Pot: %d | Current bet: %d | Your bet: %d",
				p.Chips, table.pot, table.currentBet, p.Bet))
			sendAreaGamblingMessage(table.area, fmt.Sprintf("It's %s's turn.", p.Client.OOCName()))
			pokerResetTimer(table)
			return
		}
	}

	// No eligible player found — advance street.
	pokerNextStreet(table)
}

// pokerNextStreet deals the next community cards and starts the next betting round.
// Assumes table.mu is held.
func pokerNextStreet(table *PokerTable) {
	// Reset bets for the new street.
	for _, p := range table.players {
		p.Bet = 0
	}
	table.currentBet = 0
	table.lastRaiser = -1

	switch table.state {
	case PokerPreflop:
		table.state = PokerFlop
		c1, c2, c3 := table.drawCard(), table.drawCard(), table.drawCard()
		table.community = append(table.community, c1, c2, c3)
		sendAreaGamblingMessage(table.area,
			fmt.Sprintf("*** FLOP: %s %s %s | Pot: %d ***",
				c1.String(), c2.String(), c3.String(), table.pot))
	case PokerFlop:
		table.state = PokerTurn
		c := table.drawCard()
		table.community = append(table.community, c)
		sendAreaGamblingMessage(table.area,
			fmt.Sprintf("*** TURN: %s | Community: %s | Pot: %d ***",
				c.String(), pokerCommunityStr(table), table.pot))
	case PokerTurn:
		table.state = PokerRiver
		c := table.drawCard()
		table.community = append(table.community, c)
		sendAreaGamblingMessage(table.area,
			fmt.Sprintf("*** RIVER: %s | Community: %s | Pot: %d ***",
				c.String(), pokerCommunityStr(table), table.pot))
	case PokerRiver:
		pokerShowdown(table)
		return
	default:
		return
	}

	// Find first player left of dealer to act.
	n := len(table.players)
	for i := 1; i <= n; i++ {
		idx := (table.dealerIdx + i) % n
		p := table.players[idx]
		if !p.Folded && !p.AllIn && !p.SitOut {
			table.turnIdx = idx
			p.Client.SendServerMessage(fmt.Sprintf(
				"Your turn! Stack: %d | Pot: %d | Check or bet.",
				p.Chips, table.pot))
			sendAreaGamblingMessage(table.area, fmt.Sprintf("It's %s's turn.", p.Client.OOCName()))
			pokerResetTimer(table)
			return
		}
	}
	// Everyone is all-in; run out the board.
	pokerNextStreet(table)
}

// pokerShowdown compares all remaining hands and awards the pot.
// Assumes table.mu is held.
func pokerShowdown(table *PokerTable) {
	table.state = PokerShowdown
	if table.timer != nil {
		table.timer.Stop()
		table.timer = nil
	}

	active := pokerActivePlayers(table)

	type result struct {
		player *PokerPlayer
		rank   int
		tb     []int
		desc   string
	}

	results := make([]result, 0, len(active))
	for _, p := range active {
		allCards := append(append([]Card{}, p.Hand...), table.community...)
		rank, tb, desc := pokerBestFiveCard(allCards)
		results = append(results, result{p, rank, tb, desc})
		p.Client.SendServerMessage(fmt.Sprintf("Your hand: %s %s — %s",
			p.Hand[0].String(), p.Hand[1].String(), desc))
	}

	// Reveal all hands.
	var reveal []string
	for _, r := range results {
		reveal = append(reveal, fmt.Sprintf("%s: %s %s (%s)",
			r.player.Client.OOCName(),
			r.player.Hand[0].String(), r.player.Hand[1].String(), r.desc))
	}
	sendAreaGamblingMessage(table.area, "=== SHOWDOWN ===\n"+strings.Join(reveal, "\n"))

	// Find winner(s).
	best := results[0]
	for _, r := range results[1:] {
		if r.rank > best.rank {
			best = r
		} else if r.rank == best.rank && pokerCompareTB(r.tb, best.tb) > 0 {
			best = r
		}
	}

	// Collect all tied winners.
	var winners []*PokerPlayer
	for _, r := range results {
		if r.rank == best.rank && pokerCompareTB(r.tb, best.tb) == 0 {
			winners = append(winners, r.player)
		}
	}

	split := table.pot / int64(len(winners))
	for _, w := range winners {
		w.Chips += split
		db.AddChips(w.Client.Ipid(), split) //nolint:errcheck
	}
	if len(winners) == 1 {
		sendAreaGamblingMessage(table.area,
			fmt.Sprintf("%s wins the pot of %d chips! (%s)", winners[0].Client.OOCName(), table.pot, best.desc))
	} else {
		names := make([]string, len(winners))
		for i, w := range winners {
			names[i] = w.Client.OOCName()
		}
		sendAreaGamblingMessage(table.area,
			fmt.Sprintf("Split pot (%d each): %s", split, strings.Join(names, ", ")))
	}

	a := table.area
	go func() {
		time.Sleep(8 * time.Second)
		pokerCleanupTable(a, table)
	}()
}

// pokerAwardPot hands the entire pot to the sole remaining player.
// Assumes table.mu is held.
func pokerAwardPot(table *PokerTable, winner *PokerPlayer) {
	table.state = PokerShowdown
	if table.timer != nil {
		table.timer.Stop()
		table.timer = nil
	}
	winner.Chips += table.pot
	db.AddChips(winner.Client.Ipid(), table.pot) //nolint:errcheck
	sendAreaGamblingMessage(table.area,
		fmt.Sprintf("%s wins the pot of %d chips (all others folded).", winner.Client.OOCName(), table.pot))

	a := table.area
	go func() {
		time.Sleep(5 * time.Second)
		pokerCleanupTable(a, table)
	}()
}

// pokerCleanupTable removes the table from the casino state.
func pokerCleanupTable(a *area.Area, table *PokerTable) {
	cs := getCasinoState(a)
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.pokerTable == table {
		cs.pokerTable = nil
		if cs.activeTables > 0 {
			cs.activeTables--
		}
	}
}

func pokerCommunityStr(table *PokerTable) string {
	parts := make([]string, len(table.community))
	for i, c := range table.community {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

// pokerHandleDisconnect removes a disconnected client from the poker table.
func pokerHandleDisconnect(table *PokerTable, client *Client) {
	table.mu.Lock()
	defer table.mu.Unlock()

	p := pokerFindPlayer(table, client)
	if p == nil {
		return
	}

	p.Folded = true
	p.SitOut = true

	isTurn := table.state != PokerWaiting && table.state != PokerShowdown &&
		table.turnIdx < len(table.players) && table.players[table.turnIdx] == p

	if isTurn {
		sendAreaGamblingMessage(table.area,
			fmt.Sprintf("%s disconnected and auto-folds.", client.OOCName()))
		pokerAdvanceTurn(table)
	}
}

// --- Hand evaluation ---

// pokerCardRank converts a card Value to a poker rank (Ace = 14).
func pokerCardRank(v int) int {
	if v == 1 {
		return 14
	}
	return v
}

// pokerCompareTB returns positive if a > b, negative if a < b, 0 if equal.
func pokerCompareTB(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

// pokerEvalFiveCard evaluates a 5-card hand and returns (rank 0-8, tiebreakers).
func pokerEvalFiveCard(cards []Card) (int, []int) {
	values := make([]int, 5)
	suitCount := make(map[CardSuit]int)
	valueCount := make(map[int]int)

	for i, c := range cards {
		v := pokerCardRank(c.Value)
		values[i] = v
		suitCount[c.Suit]++
		valueCount[v]++
	}
	sort.Sort(sort.Reverse(sort.IntSlice(values)))

	isFlush := false
	for _, cnt := range suitCount {
		if cnt == 5 {
			isFlush = true
			break
		}
	}

	isStraight := false
	if len(valueCount) == 5 && values[0]-values[4] == 4 {
		isStraight = true
	}
	// Wheel: A-2-3-4-5
	if values[0] == 14 && values[1] == 5 && values[2] == 4 && values[3] == 3 && values[4] == 2 {
		isStraight = true
		values = []int{5, 4, 3, 2, 1}
	}

	var quads, trips, pairs []int
	for v, cnt := range valueCount {
		switch cnt {
		case 4:
			quads = append(quads, v)
		case 3:
			trips = append(trips, v)
		case 2:
			pairs = append(pairs, v)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(quads)))
	sort.Sort(sort.Reverse(sort.IntSlice(trips)))
	sort.Sort(sort.Reverse(sort.IntSlice(pairs)))

	switch {
	case isFlush && isStraight:
		return 8, values
	case len(quads) > 0:
		var kicker int
		for _, v := range values {
			if v != quads[0] {
				kicker = v
				break
			}
		}
		return 7, []int{quads[0], kicker}
	case len(trips) > 0 && len(pairs) > 0:
		return 6, append([]int{trips[0]}, pairs...)
	case isFlush:
		return 5, values
	case isStraight:
		return 4, values
	case len(trips) > 0:
		var kickers []int
		for _, v := range values {
			if v != trips[0] {
				kickers = append(kickers, v)
			}
		}
		return 3, append([]int{trips[0]}, kickers...)
	case len(pairs) >= 2:
		var kicker int
		for _, v := range values {
			if v != pairs[0] && v != pairs[1] {
				kicker = v
				break
			}
		}
		return 2, []int{pairs[0], pairs[1], kicker}
	case len(pairs) == 1:
		var kickers []int
		for _, v := range values {
			if v != pairs[0] {
				kickers = append(kickers, v)
			}
		}
		return 1, append([]int{pairs[0]}, kickers...)
	default:
		return 0, values
	}
}

// pokerBestFiveCard selects the best 5-card hand from up to 7 cards.
func pokerBestFiveCard(cards []Card) (int, []int, string) {
	rankNames := []string{
		"High Card", "One Pair", "Two Pair", "Three of a Kind",
		"Straight", "Flush", "Full House", "Four of a Kind", "Straight Flush",
	}

	bestRank := -1
	var bestTB []int

	var combo func(start int, cur []Card)
	combo = func(start int, cur []Card) {
		if len(cur) == 5 {
			r, tb := pokerEvalFiveCard(cur)
			if r > bestRank || (r == bestRank && pokerCompareTB(tb, bestTB) > 0) {
				bestRank = r
				bestTB = tb
			}
			return
		}
		need := 5 - len(cur)
		for i := start; i <= len(cards)-need; i++ {
			combo(i+1, append(cur, cards[i]))
		}
	}
	combo(0, nil)

	if bestRank < 0 {
		bestRank = 0
	}
	return bestRank, bestTB, rankNames[bestRank]
}

// --- Command handlers ---

func pokerJoin(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()

	if cs.pokerTable == nil {
		maxT := client.Area().CasinoMaxTables()
		if maxT > 0 && cs.activeTables >= maxT {
			cs.mu.Unlock()
			client.SendServerMessage("Maximum number of tables has been reached.")
			return
		}
		cs.pokerTable = &PokerTable{
			area:       client.Area(),
			deck:       newDeck(1),
			smallBlind: 25,
			bigBlind:   50,
			buyIn:      500,
			dealerIdx:  0,
			turnIdx:    0,
			lastRaiser: -1,
		}
		cs.activeTables++
	}
	table := cs.pokerTable
	cs.mu.Unlock()

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != PokerWaiting {
		client.SendServerMessage("A hand is in progress. Wait for the next hand.")
		return
	}
	if pokerFindPlayer(table, client) != nil {
		client.SendServerMessage("You are already at the poker table.")
		return
	}
	if len(table.players) >= 9 {
		client.SendServerMessage("The poker table is full (max 9 players).")
		return
	}

	bal, err := db.GetChipBalance(client.Ipid())
	if err != nil || bal < table.buyIn {
		client.SendServerMessage(fmt.Sprintf("You need %d chips to buy in. Your balance: %d", table.buyIn, bal))
		return
	}
	_, err = db.SpendChips(client.Ipid(), table.buyIn)
	if err != nil {
		client.SendServerMessage("Failed to buy in: " + err.Error())
		return
	}

	table.players = append(table.players, &PokerPlayer{
		Client: client,
		Chips:  table.buyIn,
	})
	sendAreaGamblingMessage(table.area,
		fmt.Sprintf("%s joined the poker table (buy-in: %d chips). (%d players)",
			client.OOCName(), table.buyIn, len(table.players)))
	client.SendServerMessage(fmt.Sprintf(
		"Joined. Stack: %d chips. Use /poker ready when you want to start.", table.buyIn))
}

func pokerReady(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker table. Use /poker join first.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != PokerWaiting {
		client.SendServerMessage("A hand is already in progress.")
		return
	}

	p := pokerFindPlayer(table, client)
	if p == nil {
		client.SendServerMessage("You are not at the poker table.")
		return
	}

	p.Ready = true
	sendAreaGamblingMessage(table.area, fmt.Sprintf("%s is ready.", client.OOCName()))

	// Start if all seated players are ready and we have at least 2.
	eligible := 0
	allReady := true
	for _, pp := range table.players {
		if !pp.SitOut {
			eligible++
			if !pp.Ready {
				allReady = false
			}
		}
	}
	if eligible >= 2 && allReady {
		pokerStartHand(table)
	}
}

// pokerStartHand deals hole cards and posts blinds.
// Assumes table.mu is held.
func pokerStartHand(table *PokerTable) {
	table.state = PokerPreflop
	table.community = nil
	table.pot = 0
	table.currentBet = table.bigBlind
	table.lastRaiser = -1
	table.deck = newDeck(1)

	for _, p := range table.players {
		p.Hand = nil
		p.Bet = 0
		p.Folded = false
		p.AllIn = false
		p.Ready = false
	}

	// Deal 2 hole cards to each player.
	for _, p := range table.players {
		if p.SitOut {
			continue
		}
		p.Hand = []Card{table.drawCard(), table.drawCard()}
		p.Client.SendServerMessage(fmt.Sprintf("Your hole cards: %s %s",
			p.Hand[0].String(), p.Hand[1].String()))
	}

	n := len(table.players)

	// Heads-up rule: dealer = small blind and acts first pre-flop.
	// 3+ players: normal SB/BB/UTG rotation.
	var sbIdx, bbIdx, preflopFirstIdx int
	if n == 2 {
		sbIdx = table.dealerIdx
		bbIdx = (table.dealerIdx + 1) % n
		preflopFirstIdx = table.dealerIdx // dealer (SB) acts first heads-up
	} else {
		sbIdx = (table.dealerIdx + 1) % n
		bbIdx = (table.dealerIdx + 2) % n
		preflopFirstIdx = (table.dealerIdx + 3) % n
	}

	// Post small blind.
	sb := table.players[sbIdx]
	sbAmt := table.smallBlind
	if sb.Chips < sbAmt {
		sbAmt = sb.Chips
	}
	sb.Chips -= sbAmt
	sb.Bet = sbAmt
	table.pot += sbAmt

	// Post big blind.
	bb := table.players[bbIdx]
	bbAmt := table.bigBlind
	if bb.Chips < bbAmt {
		bbAmt = bb.Chips
	}
	bb.Chips -= bbAmt
	bb.Bet = bbAmt
	table.pot += bbAmt

	sendAreaGamblingMessage(table.area,
		fmt.Sprintf("New hand! Dealer: %s | SB: %s (%d) | BB: %s (%d) | Pot: %d",
			table.players[table.dealerIdx].Client.OOCName(),
			sb.Client.OOCName(), sbAmt,
			bb.Client.OOCName(), bbAmt,
			table.pot))

	// First to act pre-flop.
	for i := 0; i < n; i++ {
		idx := (preflopFirstIdx + i) % n
		p := table.players[idx]
		if !p.Folded && !p.AllIn && !p.SitOut {
			table.turnIdx = idx
			p.Client.SendServerMessage(fmt.Sprintf(
				"Your turn (pre-flop)! Stack: %d | Pot: %d | BB: %d | Your bet: %d",
				p.Chips, table.pot, table.bigBlind, p.Bet))
			sendAreaGamblingMessage(table.area, fmt.Sprintf("It's %s's turn.", p.Client.OOCName()))
			pokerResetTimer(table)
			return
		}
	}
}

func pokerShowHand(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker table active.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	p := pokerFindPlayer(table, client)
	if p == nil {
		client.SendServerMessage("You are not at the poker table.")
		return
	}
	if len(p.Hand) < 2 {
		client.SendServerMessage("No hole cards dealt yet.")
		return
	}
	community := pokerCommunityStr(table)
	if community == "" {
		community = "(none yet)"
	}
	client.SendServerMessage(fmt.Sprintf("Hole cards: %s %s | Community: %s",
		p.Hand[0].String(), p.Hand[1].String(), community))
}

func pokerCheck(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker hand in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if !pokerIsPlayerTurn(table, client) {
		return
	}
	p := table.players[table.turnIdx]
	if table.currentBet > p.Bet {
		client.SendServerMessage(fmt.Sprintf(
			"Cannot check — there is a bet of %d to call. Use /poker call.", table.currentBet))
		return
	}
	sendAreaGamblingMessage(table.area, fmt.Sprintf("%s checks.", client.OOCName()))
	pokerAdvanceTurn(table)
}

func pokerCall(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker hand in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if !pokerIsPlayerTurn(table, client) {
		return
	}
	p := table.players[table.turnIdx]
	callAmt := table.currentBet - p.Bet
	if callAmt <= 0 {
		client.SendServerMessage("Nothing to call — use /poker check.")
		return
	}
	if p.Chips <= callAmt {
		// All in.
		callAmt = p.Chips
		p.AllIn = true
	}
	p.Chips -= callAmt
	p.Bet += callAmt
	table.pot += callAmt
	sendAreaGamblingMessage(table.area, fmt.Sprintf("%s calls %d. Pot: %d", client.OOCName(), callAmt, table.pot))
	pokerAdvanceTurn(table)
}

func pokerBetOrRaise(client *Client, args []string, isRaise bool) {
	if len(args) == 0 {
		if isRaise {
			client.SendServerMessage("Usage: /poker raise <amount>")
		} else {
			client.SendServerMessage("Usage: /poker bet <amount>")
		}
		return
	}
	amount, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Invalid amount.")
		return
	}

	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker hand in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if !pokerIsPlayerTurn(table, client) {
		return
	}
	p := table.players[table.turnIdx]

	totalBet := p.Bet + amount
	if isRaise && totalBet <= table.currentBet {
		client.SendServerMessage(fmt.Sprintf(
			"Raise must be above current bet of %d. Need to raise by at least %d more.",
			table.currentBet, table.currentBet-p.Bet+1))
		return
	}

	spend := amount
	if spend >= p.Chips {
		spend = p.Chips
		p.AllIn = true
	}
	p.Chips -= spend
	p.Bet += spend
	table.pot += spend
	if p.Bet > table.currentBet {
		table.currentBet = p.Bet
		table.lastRaiser = table.turnIdx
	}

	action := "bets"
	if isRaise {
		action = "raises to"
	}
	sendAreaGamblingMessage(table.area,
		fmt.Sprintf("%s %s %d. Pot: %d", client.OOCName(), action, p.Bet, table.pot))
	pokerAdvanceTurn(table)
}

func pokerFold(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker hand in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if !pokerIsPlayerTurn(table, client) {
		return
	}
	p := table.players[table.turnIdx]
	p.Folded = true
	sendAreaGamblingMessage(table.area, fmt.Sprintf("%s folds.", client.OOCName()))
	pokerAdvanceTurn(table)
}

func pokerAllIn(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker hand in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if !pokerIsPlayerTurn(table, client) {
		return
	}
	p := table.players[table.turnIdx]
	if p.Chips == 0 {
		client.SendServerMessage("You are already all in.")
		return
	}

	spend := p.Chips
	p.Bet += spend
	p.Chips = 0
	p.AllIn = true
	table.pot += spend
	if p.Bet > table.currentBet {
		table.currentBet = p.Bet
		table.lastRaiser = table.turnIdx
	}
	sendAreaGamblingMessage(table.area,
		fmt.Sprintf("%s goes ALL IN for %d! Pot: %d", client.OOCName(), p.Bet, table.pot))
	pokerAdvanceTurn(table)
}

func pokerStatus(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker table active.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	streetNames := map[PokerRound]string{
		PokerWaiting:  "Waiting",
		PokerPreflop:  "Pre-flop",
		PokerFlop:     "Flop",
		PokerTurn:     "Turn",
		PokerRiver:    "River",
		PokerShowdown: "Showdown",
	}

	lines := []string{
		fmt.Sprintf("=== Poker Table — %s | Pot: %d | Bet: %d ===",
			streetNames[table.state], table.pot, table.currentBet),
	}
	if len(table.community) > 0 {
		lines = append(lines, "Community: "+pokerCommunityStr(table))
	}
	for i, p := range table.players {
		status := ""
		if p.Folded {
			status = " [folded]"
		} else if p.AllIn {
			status = " [all-in]"
		}
		turnMark := ""
		if table.state != PokerWaiting && i == table.turnIdx {
			turnMark = " ←"
		}
		lines = append(lines, fmt.Sprintf("%s%s%s — stack: %d, bet: %d",
			p.Client.OOCName(), status, turnMark, p.Chips, p.Bet))
	}
	client.SendServerMessage(strings.Join(lines, "\n"))
}

func pokerLeave(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.pokerTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No poker table to leave.")
		return
	}

	table.mu.Lock()

	p := pokerFindPlayer(table, client)
	if p == nil {
		table.mu.Unlock()
		client.SendServerMessage("You are not at the poker table.")
		return
	}

	// Cash out remaining stack.
	if p.Chips > 0 {
		db.AddChips(client.Ipid(), p.Chips) //nolint:errcheck
		client.SendServerMessage(fmt.Sprintf("Cashed out %d chips.", p.Chips))
	}

	p.Folded = true
	p.SitOut = true

	isTurn := table.state != PokerWaiting && table.state != PokerShowdown &&
		table.turnIdx < len(table.players) && table.players[table.turnIdx] == p

	sendAreaGamblingMessage(table.area, fmt.Sprintf("%s left the poker table.", client.OOCName()))

	if isTurn {
		pokerAdvanceTurn(table)
	}

	table.mu.Unlock()
}

// pokerIsPlayerTurn validates that it's the client's turn and the state is correct.
// Must be called with table.mu held.
func pokerIsPlayerTurn(table *PokerTable, client *Client) bool {
	if table.state == PokerWaiting || table.state == PokerShowdown {
		client.SendServerMessage("No betting in progress.")
		return false
	}
	if table.turnIdx >= len(table.players) || table.players[table.turnIdx].Client != client {
		client.SendServerMessage("It is not your turn.")
		return false
	}
	p := table.players[table.turnIdx]
	if p.Folded || p.AllIn {
		client.SendServerMessage("You have already folded or are all in.")
		return false
	}
	return true
}

// cmdPoker is the dispatcher for /poker subcommands.
func cmdPoker(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.SendServerMessage("Usage: /poker join|ready|hand|check|call|bet <n>|raise <n>|fold|allin|status|leave")
		return
	}
	switch args[0] {
	case "join":
		pokerJoin(client)
	case "ready":
		pokerReady(client)
	case "hand":
		pokerShowHand(client)
	case "check":
		pokerCheck(client)
	case "call":
		pokerCall(client)
	case "bet":
		pokerBetOrRaise(client, args[1:], false)
	case "raise":
		pokerBetOrRaise(client, args[1:], true)
	case "fold":
		pokerFold(client)
	case "allin":
		pokerAllIn(client)
	case "status":
		pokerStatus(client)
	case "leave":
		pokerLeave(client)
	default:
		client.SendServerMessage("Unknown subcommand. Usage: /poker join|ready|hand|check|call|bet|raise|fold|allin|status|leave")
	}
}
