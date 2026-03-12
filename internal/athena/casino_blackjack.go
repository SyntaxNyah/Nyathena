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

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
)

// CardSuit represents the suit of a playing card.
type CardSuit int

const (
	SuitClubs    CardSuit = iota
	SuitDiamonds          // ♦
	SuitHearts            // ♥
	SuitSpades            // ♠
)

// Card is a standard playing card. Value 1=Ace, 2-10=face value, 11=Jack, 12=Queen, 13=King.
type Card struct {
	Value int
	Suit  CardSuit
}

func (c Card) String() string {
	var v string
	switch c.Value {
	case 1:
		v = "A"
	case 11:
		v = "J"
	case 12:
		v = "Q"
	case 13:
		v = "K"
	default:
		v = strconv.Itoa(c.Value)
	}
	suits := [4]string{"♣", "♦", "♥", "♠"}
	return v + suits[c.Suit]
}

// cardValue returns the blackjack point value of a card (Ace=11, face cards=10).
func cardValue(c Card) int {
	if c.Value == 1 {
		return 11
	}
	if c.Value >= 10 {
		return 10
	}
	return c.Value
}

// newDeck creates n shuffled standard 52-card decks combined.
func newDeck(decks int) []Card {
	d := make([]Card, 0, decks*52)
	for i := 0; i < decks; i++ {
		for suit := SuitClubs; suit <= SuitSpades; suit++ {
			for value := 1; value <= 13; value++ {
				d = append(d, Card{Value: value, Suit: suit})
			}
		}
	}
	rand.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
	return d
}

// BJHand represents a single player hand in blackjack.
type BJHand struct {
	Cards    []Card
	Bet      int64
	Doubled  bool
	Standing bool
	Busted   bool
	Split    bool // was this hand created by a split?
}

// handTotal calculates the best blackjack total for a hand, handling soft aces.
func handTotal(hand BJHand) int {
	total := 0
	aces := 0
	for _, c := range hand.Cards {
		if c.Value == 1 {
			aces++
			total += 11
		} else if c.Value >= 10 {
			total += 10
		} else {
			total += c.Value
		}
	}
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	return total
}

// handString returns a human-readable representation of a blackjack hand.
func handString(hand BJHand) string {
	parts := make([]string, len(hand.Cards))
	for i, c := range hand.Cards {
		parts[i] = c.String()
	}
	return fmt.Sprintf("%s (%d)", strings.Join(parts, " "), handTotal(hand))
}

// BJPlayer holds one player's state at the blackjack table.
type BJPlayer struct {
	Client       *Client
	Hands        []BJHand
	ActiveHand   int
	InsuranceBet int64
}

// BJTableState represents the current phase of a blackjack round.
type BJTableState int

const (
	BJWaiting    BJTableState = iota // waiting for players and bets
	BJDealing                        // players taking turns
	BJDealerTurn                     // dealer playing
	BJDone                           // round resolved
)

// bjReshuffleThreshold is the minimum number of cards remaining in the 6-deck shoe
// before it is reshuffled. Set to ~17% penetration (52 of 312 cards) which is a
// standard casino cut-card placement to prevent card counting.
const bjReshuffleThreshold = 52

// BJTable holds the complete state for one blackjack table.
type BJTable struct {
	mu      sync.Mutex
	area    *area.Area
	state   BJTableState
	players []*BJPlayer
	deck    []Card
	dealer  BJHand
	turnIdx int
	timer   *time.Timer
}

// --- internal helpers; callers must hold table.mu ---

func (t *BJTable) drawCard() Card {
	if len(t.deck) < bjReshuffleThreshold {
		t.deck = newDeck(6)
	}
	c := t.deck[0]
	t.deck = t.deck[1:]
	return c
}

func bjFindPlayer(table *BJTable, client *Client) *BJPlayer {
	for _, p := range table.players {
		if p.Client == client {
			return p
		}
	}
	return nil
}

// bjResetTimer stops any running timer and starts a fresh 60-second turn timer.
// The callback re-acquires table.mu independently.
func bjResetTimer(table *BJTable) {
	if table.timer != nil {
		table.timer.Stop()
	}
	table.timer = time.AfterFunc(60*time.Second, func() {
		table.mu.Lock()
		defer table.mu.Unlock()
		if table.state != BJDealing || table.turnIdx >= len(table.players) {
			return
		}
		p := table.players[table.turnIdx]
		if p.ActiveHand < len(p.Hands) {
			h := &p.Hands[p.ActiveHand]
			if !h.Standing && !h.Busted {
				h.Standing = true
				sendAreaServerMessage(table.area, fmt.Sprintf("%s timed out and stands.", p.Client.OOCName()))
				bjAdvanceTurn(table)
			}
		}
	})
}

// bjAdvanceTurn moves to the next un-finished hand/player or starts the dealer turn.
// Assumes table.mu is held.
func bjAdvanceTurn(table *BJTable) {
	// Try to move to the next hand for the current player first.
	if table.turnIdx >= 0 && table.turnIdx < len(table.players) {
		p := table.players[table.turnIdx]
		for next := p.ActiveHand + 1; next < len(p.Hands); next++ {
			if !p.Hands[next].Standing && !p.Hands[next].Busted {
				p.ActiveHand = next
				p.Client.SendServerMessage(fmt.Sprintf("Now playing hand %d: %s. Dealer shows: %s",
					next+1, handString(p.Hands[next]), table.dealer.Cards[0].String()))
				bjResetTimer(table)
				return
			}
		}
	}

	// Advance to next player.
	table.turnIdx++
	for table.turnIdx < len(table.players) {
		p := table.players[table.turnIdx]
		for i := range p.Hands {
			if !p.Hands[i].Standing && !p.Hands[i].Busted {
				p.ActiveHand = i
				p.Client.SendServerMessage(fmt.Sprintf("It's your turn! Your hand: %s. Dealer shows: %s",
					handString(p.Hands[i]), table.dealer.Cards[0].String()))
				sendAreaServerMessage(table.area, fmt.Sprintf("It's %s's turn.", p.Client.OOCName()))
				bjResetTimer(table)
				return
			}
		}
		table.turnIdx++
	}

	// All players done — start dealer turn.
	bjStartDealerTurn(table)
}

// bjStartDealerTurn reveals the dealer hole card, plays out dealer hand, then resolves.
// Assumes table.mu is held.
func bjStartDealerTurn(table *BJTable) {
	table.state = BJDealerTurn
	if table.timer != nil {
		table.timer.Stop()
		table.timer = nil
	}

	sendAreaServerMessage(table.area, fmt.Sprintf("Dealer reveals: %s", handString(table.dealer)))

	// Only draw if at least one player hand is still alive.
	anyActive := false
	for _, p := range table.players {
		for _, h := range p.Hands {
			if !h.Busted && h.Bet > 0 {
				anyActive = true
				break
			}
		}
	}
	if anyActive {
		for handTotal(table.dealer) < 17 {
			c := table.drawCard()
			table.dealer.Cards = append(table.dealer.Cards, c)
			sendAreaServerMessage(table.area,
				fmt.Sprintf("Dealer draws %s → %s", c.String(), handString(table.dealer)))
		}
	}

	bjResolveRound(table)
}

// bjResolveRound pays out winnings and schedules table cleanup.
// Assumes table.mu is held.
func bjResolveRound(table *BJTable) {
	table.state = BJDone
	dealerTotal := handTotal(table.dealer)
	dealerBJ := len(table.dealer.Cards) == 2 && dealerTotal == 21

	for _, p := range table.players {
		for hi, h := range p.Hands {
			if h.Bet == 0 {
				continue
			}
			playerTotal := handTotal(h)
			playerBJ := len(h.Cards) == 2 && playerTotal == 21 && !h.Split

			var result string
			var payout int64
			switch {
			case h.Busted:
				result = "bust — you lose"
			case playerBJ && dealerBJ:
				result = "push (both blackjack)"
				payout = h.Bet
				db.AddChips(p.Client.Ipid(), h.Bet) //nolint:errcheck
			case playerBJ:
				result = "BLACKJACK! (3:2)"
				payout = h.Bet + h.Bet*3/2
				db.AddChips(p.Client.Ipid(), payout) //nolint:errcheck
			case dealerBJ:
				result = "dealer blackjack — you lose"
			case dealerTotal > 21:
				result = "dealer busts — you win!"
				payout = h.Bet * 2
				db.AddChips(p.Client.Ipid(), payout) //nolint:errcheck
			case playerTotal > dealerTotal:
				result = "you win!"
				payout = h.Bet * 2
				db.AddChips(p.Client.Ipid(), payout) //nolint:errcheck
			case playerTotal == dealerTotal:
				result = "push"
				payout = h.Bet
				db.AddChips(p.Client.Ipid(), payout) //nolint:errcheck
			default:
				result = "dealer wins"
			}

			handLabel := ""
			if len(p.Hands) > 1 {
				handLabel = fmt.Sprintf(" (hand %d)", hi+1)
			}
			bal, _ := db.GetChipBalance(p.Client.Ipid())
			p.Client.SendServerMessage(fmt.Sprintf(
				"Your hand%s: %s vs dealer %s → %s. Payout: %d. Balance: %d",
				handLabel, handString(h), handString(table.dealer), result, payout, bal))
		}

		if p.InsuranceBet > 0 {
			if dealerBJ {
				win := p.InsuranceBet * 3
				db.AddChips(p.Client.Ipid(), win) //nolint:errcheck
				p.Client.SendServerMessage(fmt.Sprintf("Insurance wins! +%d chips.", win))
			} else {
				p.Client.SendServerMessage("Insurance bet lost.")
			}
		}
	}

	sendAreaServerMessage(table.area, fmt.Sprintf("Round over! Dealer: %s", handString(table.dealer)))

	a := table.area
	go func() {
		time.Sleep(5 * time.Second)
		bjCleanupTable(a, table)
	}()
}

// bjCleanupTable removes the table from the area casino state.
// Must NOT be called while holding table.mu.
func bjCleanupTable(a *area.Area, table *BJTable) {
	cs := getCasinoState(a)
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.bjTable == table {
		cs.bjTable = nil
		if cs.activeTables > 0 {
			cs.activeTables--
		}
	}
}

// bjHandleDisconnect is called when a client disconnects mid-game.
func bjHandleDisconnect(table *BJTable, client *Client) {
	table.mu.Lock()
	defer table.mu.Unlock()

	p := bjFindPlayer(table, client)
	if p == nil {
		return
	}

	// Mark all active hands as standing.
	for i := range p.Hands {
		if !p.Hands[i].Standing && !p.Hands[i].Busted {
			p.Hands[i].Standing = true
		}
	}

	isTurn := table.state == BJDealing &&
		table.turnIdx < len(table.players) &&
		table.players[table.turnIdx] == p

	// Remove player from slice.
	for i, pp := range table.players {
		if pp == p {
			table.players = append(table.players[:i], table.players[i+1:]...)
			if table.turnIdx > i {
				table.turnIdx--
			} else if table.turnIdx == i {
				table.turnIdx = i - 1 // bjAdvanceTurn will increment
			}
			break
		}
	}

	if isTurn {
		sendAreaServerMessage(table.area, fmt.Sprintf("%s disconnected and auto-stands.", client.OOCName()))
		bjAdvanceTurn(table)
	}

	if len(table.players) == 0 && table.state != BJDone {
		sendAreaServerMessage(table.area, "All players left; ending blackjack round.")
		bjStartDealerTurn(table)
	}
}

// --- Command handlers ---

func bjJoin(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()

	if cs.bjTable == nil {
		maxT := client.Area().CasinoMaxTables()
		if maxT > 0 && cs.activeTables >= maxT {
			cs.mu.Unlock()
			client.SendServerMessage("Maximum number of tables has been reached for this area.")
			return
		}
		cs.bjTable = &BJTable{
			area:    client.Area(),
			deck:    newDeck(6),
			turnIdx: -1,
		}
		cs.activeTables++
	}
	table := cs.bjTable
	cs.mu.Unlock()

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJWaiting {
		client.SendServerMessage("A round is already in progress. Wait for the next round.")
		return
	}
	if bjFindPlayer(table, client) != nil {
		client.SendServerMessage("You are already at the blackjack table.")
		return
	}
	if len(table.players) >= 6 {
		client.SendServerMessage("The blackjack table is full (max 6 players).")
		return
	}

	table.players = append(table.players, &BJPlayer{
		Client: client,
		Hands:  []BJHand{{}},
	})
	sendAreaServerMessage(table.area,
		fmt.Sprintf("%s joined the blackjack table. (%d/6 players)", client.OOCName(), len(table.players)))
	client.SendServerMessage("Joined the blackjack table. Use /bj bet <amount> to place your bet, then /bj deal to start.")
}

func bjBet(client *Client, args []string) {
	if len(args) == 0 {
		client.SendServerMessage("Usage: /bj bet <amount>")
		return
	}
	amount, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Invalid bet amount.")
		return
	}
	ok, reason := validateBet(client, amount)
	if !ok {
		client.SendServerMessage(reason)
		return
	}

	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack table exists. Use /bj join first.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJWaiting {
		client.SendServerMessage("Cannot change bets during a round.")
		return
	}
	p := bjFindPlayer(table, client)
	if p == nil {
		client.SendServerMessage("You are not at the blackjack table. Use /bj join first.")
		return
	}
	p.Hands[0].Bet = amount
	client.SendServerMessage(fmt.Sprintf("Bet set to %d chips.", amount))
	sendAreaServerMessage(table.area, fmt.Sprintf("%s placed a bet.", client.OOCName()))
}

func bjDeal(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack table exists. Use /bj join first.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if bjFindPlayer(table, client) == nil {
		client.SendServerMessage("You are not at the blackjack table.")
		return
	}
	if table.state != BJWaiting {
		client.SendServerMessage("The round has already started.")
		return
	}

	// Collect players who have bets and can pay.
	active := make([]*BJPlayer, 0, len(table.players))
	for _, p := range table.players {
		if p.Hands[0].Bet <= 0 {
			continue
		}
		_, err := db.SpendChips(p.Client.Ipid(), p.Hands[0].Bet)
		if err != nil {
			p.Client.SendServerMessage(fmt.Sprintf("Could not place bet: %v", err))
			p.Hands[0].Bet = 0
			continue
		}
		p.Hands = []BJHand{{Bet: p.Hands[0].Bet}}
		active = append(active, p)
	}
	if len(active) == 0 {
		client.SendServerMessage("No players with valid bets. Use /bj bet <amount> first.")
		return
	}

	// Deal 2 cards to each active player then 2 to dealer.
	for _, p := range active {
		p.Hands[0].Cards = []Card{table.drawCard(), table.drawCard()}
	}
	table.dealer = BJHand{Cards: []Card{table.drawCard(), table.drawCard()}}
	table.state = BJDealing
	table.turnIdx = -1

	sendAreaServerMessage(table.area,
		fmt.Sprintf("Cards dealt! Dealer shows: %s", table.dealer.Cards[0].String()))
	for _, p := range active {
		p.Client.SendServerMessage(fmt.Sprintf("Your hand: %s", handString(p.Hands[0])))
	}
	if table.dealer.Cards[0].Value == 1 {
		sendAreaServerMessage(table.area,
			"Dealer shows an Ace — use /bj insurance to place an insurance side-bet before your turn ends.")
	}

	bjAdvanceTurn(table)
}

func bjHit(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack round in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJDealing {
		client.SendServerMessage("It is not the dealing phase.")
		return
	}
	if table.turnIdx >= len(table.players) || table.players[table.turnIdx].Client != client {
		client.SendServerMessage("It is not your turn.")
		return
	}

	p := table.players[table.turnIdx]
	h := &p.Hands[p.ActiveHand]
	if h.Standing || h.Busted {
		client.SendServerMessage("Your hand is already finished.")
		return
	}

	card := table.drawCard()
	h.Cards = append(h.Cards, card)
	total := handTotal(*h)
	client.SendServerMessage(fmt.Sprintf("You drew %s. Hand: %s", card.String(), handString(*h)))
	sendAreaServerMessage(table.area, fmt.Sprintf("%s hits.", client.OOCName()))

	switch {
	case total > 21:
		h.Busted = true
		client.SendServerMessage("Bust!")
		sendAreaServerMessage(table.area, fmt.Sprintf("%s busts!", client.OOCName()))
		bjAdvanceTurn(table)
	case total == 21:
		h.Standing = true
		client.SendServerMessage("21! Standing automatically.")
		bjAdvanceTurn(table)
	case h.Doubled:
		h.Standing = true
		bjAdvanceTurn(table)
	}
}

func bjStand(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack round in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJDealing {
		client.SendServerMessage("It is not the dealing phase.")
		return
	}
	if table.turnIdx >= len(table.players) || table.players[table.turnIdx].Client != client {
		client.SendServerMessage("It is not your turn.")
		return
	}

	p := table.players[table.turnIdx]
	h := &p.Hands[p.ActiveHand]
	if h.Standing || h.Busted {
		client.SendServerMessage("Your hand is already finished.")
		return
	}

	h.Standing = true
	client.SendServerMessage(fmt.Sprintf("You stand on %s.", handString(*h)))
	sendAreaServerMessage(table.area, fmt.Sprintf("%s stands.", client.OOCName()))
	bjAdvanceTurn(table)
}

func bjDouble(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack round in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJDealing {
		client.SendServerMessage("It is not the dealing phase.")
		return
	}
	if table.turnIdx >= len(table.players) || table.players[table.turnIdx].Client != client {
		client.SendServerMessage("It is not your turn.")
		return
	}

	p := table.players[table.turnIdx]
	h := &p.Hands[p.ActiveHand]
	if len(h.Cards) != 2 {
		client.SendServerMessage("You can only double down on your first two cards.")
		return
	}
	if h.Doubled {
		client.SendServerMessage("Already doubled down.")
		return
	}

	ok, reason := validateBet(client, h.Bet)
	if !ok {
		client.SendServerMessage("Cannot double: " + reason)
		return
	}
	_, err := db.SpendChips(client.Ipid(), h.Bet)
	if err != nil {
		client.SendServerMessage("Failed to double down: " + err.Error())
		return
	}

	h.Bet *= 2
	h.Doubled = true
	card := table.drawCard()
	h.Cards = append(h.Cards, card)
	client.SendServerMessage(fmt.Sprintf("Doubled down! Drew %s. Hand: %s", card.String(), handString(*h)))
	sendAreaServerMessage(table.area, fmt.Sprintf("%s doubles down.", client.OOCName()))

	if handTotal(*h) > 21 {
		h.Busted = true
		client.SendServerMessage("Bust!")
	} else {
		h.Standing = true
	}
	bjAdvanceTurn(table)
}

func bjSplit(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack round in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJDealing {
		client.SendServerMessage("It is not the dealing phase.")
		return
	}
	if table.turnIdx >= len(table.players) || table.players[table.turnIdx].Client != client {
		client.SendServerMessage("It is not your turn.")
		return
	}

	p := table.players[table.turnIdx]
	h := &p.Hands[p.ActiveHand]
	if len(h.Cards) != 2 {
		client.SendServerMessage("You can only split on your first two cards.")
		return
	}
	if len(p.Hands) >= 2 {
		client.SendServerMessage("You can only split once.")
		return
	}

	v1, v2 := h.Cards[0].Value, h.Cards[1].Value
	if v1 >= 10 {
		v1 = 10
	}
	if v2 >= 10 {
		v2 = 10
	}
	if v1 != v2 {
		client.SendServerMessage("You can only split if both cards have the same value.")
		return
	}

	ok, reason := validateBet(client, h.Bet)
	if !ok {
		client.SendServerMessage("Cannot split: " + reason)
		return
	}
	_, err := db.SpendChips(client.Ipid(), h.Bet)
	if err != nil {
		client.SendServerMessage("Failed to split: " + err.Error())
		return
	}

	bet := h.Bet
	hand1 := BJHand{Cards: []Card{h.Cards[0], table.drawCard()}, Bet: bet, Split: true}
	hand2 := BJHand{Cards: []Card{h.Cards[1], table.drawCard()}, Bet: bet, Split: true}
	p.Hands = []BJHand{hand1, hand2}
	p.ActiveHand = 0

	client.SendServerMessage(fmt.Sprintf("Split! Hand 1: %s | Hand 2: %s",
		handString(p.Hands[0]), handString(p.Hands[1])))
	sendAreaServerMessage(table.area, fmt.Sprintf("%s splits their hand.", client.OOCName()))
	bjResetTimer(table)
}

func bjInsurance(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack round in progress.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	if table.state != BJDealing {
		client.SendServerMessage("Insurance is only available during the dealing phase.")
		return
	}
	if len(table.dealer.Cards) == 0 || table.dealer.Cards[0].Value != 1 {
		client.SendServerMessage("Insurance is only available when the dealer shows an Ace.")
		return
	}

	p := bjFindPlayer(table, client)
	if p == nil {
		client.SendServerMessage("You are not at the blackjack table.")
		return
	}
	if p.InsuranceBet > 0 {
		client.SendServerMessage("You have already placed an insurance bet.")
		return
	}
	if len(p.Hands) == 0 || p.Hands[0].Bet == 0 {
		client.SendServerMessage("You need an active bet to place insurance.")
		return
	}

	insAmt := p.Hands[0].Bet / 2
	if insAmt == 0 {
		insAmt = 1
	}
	ok, reason := validateBet(client, insAmt)
	if !ok {
		client.SendServerMessage("Cannot place insurance: " + reason)
		return
	}
	_, err := db.SpendChips(client.Ipid(), insAmt)
	if err != nil {
		client.SendServerMessage("Failed to place insurance: " + err.Error())
		return
	}
	p.InsuranceBet = insAmt
	client.SendServerMessage(fmt.Sprintf("Insurance bet placed: %d chips.", insAmt))
}

func bjStatus(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack table is active in this area.")
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()

	stateStr := map[BJTableState]string{
		BJWaiting:    "Waiting for bets",
		BJDealing:    "Round in progress",
		BJDealerTurn: "Dealer's turn",
		BJDone:       "Round complete",
	}[table.state]

	lines := []string{fmt.Sprintf("=== Blackjack Table — %s ===", stateStr)}

	if table.state != BJWaiting && len(table.dealer.Cards) > 0 {
		lines = append(lines, fmt.Sprintf("Dealer: %s [hole card hidden]", table.dealer.Cards[0].String()))
	}

	for i, p := range table.players {
		for hi, h := range p.Hands {
			turnMark := ""
			if table.state == BJDealing && i == table.turnIdx && hi == p.ActiveHand {
				turnMark = " ← YOUR TURN"
			}
			handLabel := ""
			if len(p.Hands) > 1 {
				handLabel = fmt.Sprintf(" H%d", hi+1)
			}
			lines = append(lines, fmt.Sprintf("%s%s%s: bet=%d %s",
				p.Client.OOCName(), handLabel, turnMark, h.Bet, handString(h)))
		}
	}

	client.SendServerMessage(strings.Join(lines, "\n"))
}

func bjLeave(client *Client) {
	cs := getCasinoState(client.Area())
	cs.mu.Lock()
	table := cs.bjTable
	cs.mu.Unlock()

	if table == nil {
		client.SendServerMessage("No blackjack table to leave.")
		return
	}

	table.mu.Lock()

	p := bjFindPlayer(table, client)
	if p == nil {
		table.mu.Unlock()
		client.SendServerMessage("You are not at the blackjack table.")
		return
	}

	if table.state == BJDealing {
		for i := range p.Hands {
			if !p.Hands[i].Standing && !p.Hands[i].Busted {
				p.Hands[i].Standing = true
			}
		}
	}

	isTurn := table.state == BJDealing &&
		table.turnIdx < len(table.players) &&
		table.players[table.turnIdx] == p

	for i, pp := range table.players {
		if pp == p {
			table.players = append(table.players[:i], table.players[i+1:]...)
			if table.turnIdx > i {
				table.turnIdx--
			} else if table.turnIdx == i {
				table.turnIdx = i - 1
			}
			break
		}
	}

	isEmpty := len(table.players) == 0
	sendAreaServerMessage(table.area, fmt.Sprintf("%s left the blackjack table.", client.OOCName()))
	client.SendServerMessage("You left the blackjack table.")

	if isTurn {
		bjAdvanceTurn(table)
		table.mu.Unlock()
		return
	}

	if isEmpty {
		if table.state == BJWaiting {
			table.mu.Unlock()
			bjCleanupTable(table.area, table)
			return
		} else if table.state == BJDealing {
			bjStartDealerTurn(table)
			table.mu.Unlock()
			return
		}
	}

	table.mu.Unlock()
}

// cmdBlackjack is the dispatcher for /bj subcommands.
func cmdBlackjack(client *Client, args []string, _ string) {
	if !casinoCheck(client) {
		return
	}
	if len(args) == 0 {
		client.SendServerMessage("Usage: /bj join|bet <amount>|deal|hit|stand|double|split|insurance|status|leave")
		return
	}
	switch args[0] {
	case "join":
		bjJoin(client)
	case "bet":
		bjBet(client, args[1:])
	case "deal":
		bjDeal(client)
	case "hit":
		bjHit(client)
	case "stand":
		bjStand(client)
	case "double":
		bjDouble(client)
	case "split":
		bjSplit(client)
	case "insurance":
		bjInsurance(client)
	case "status":
		bjStatus(client)
	case "leave":
		bjLeave(client)
	default:
		client.SendServerMessage("Unknown subcommand. Usage: /bj join|bet|deal|hit|stand|double|split|insurance|status|leave")
	}
}
