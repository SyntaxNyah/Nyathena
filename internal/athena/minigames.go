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
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Casual mini-games under Pebble Mode. These are deliberately stripped of all
// stakes: they never read or write CHIPS or PEBBLES. RNG only decides the fun
// outcome and the high score, never a reward.

const (
	miniGameSlots     = "slots_streak"
	miniGameBlackjack = "blackjack_streak"
	miniGameCrash     = "crash"
)

type miniBlackjackSession struct {
	deck   []Card
	player []Card
	dealer []Card
	active bool
	ended  bool
}

type miniCrashSession struct {
	start   time.Time
	crashAt float64
	active  bool
	timer   *time.Timer
}

type miniGameSession struct {
	mu           sync.Mutex
	blackjack    miniBlackjackSession
	crash        miniCrashSession
	crashGen     int64
	slotsStreak  int
	blackjackWin int
}

var miniGameSessions sync.Map // key: *Client

func getMiniGameSession(client *Client) *miniGameSession {
	v, _ := miniGameSessions.LoadOrStore(client, &miniGameSession{})
	return v.(*miniGameSession)
}

// handleMinigameDisconnect stops timers and drops per-client state.
func handleMinigameDisconnect(client *Client) {
	if v, ok := miniGameSessions.Load(client); ok {
		s := v.(*miniGameSession)
		s.mu.Lock()
		if s.crash.timer != nil {
			s.crash.timer.Stop()
		}
		s.mu.Unlock()
		miniGameSessions.Delete(client)
	}
}

// miniGamesAvailable checks Pebble Mode and the area toggle.
func miniGamesAvailable(client *Client) bool {
	if pebbleModeGuard(client) {
		return false
	}
	if a := client.Area(); a != nil && !a.PebbleMinigames() {
		client.SendServerMessage("Mini-games are disabled in this area. A moderator can enable them with /pebbleset minigames true.")
		return false
	}
	return true
}

func cmdMiniGame(client *Client, args []string, _ string) {
	if len(args) == 0 {
		printMiniGameDashboard(client)
		return
	}
	switch strings.ToLower(args[0]) {
	case "slots":
		miniSlots(client)
	case "blackjack", "bj":
		miniBlackjack(client, args[1:])
	case "crash":
		miniCrash(client, args[1:])
	case "top", "scores":
		miniGameTop(client, args[1:])
	case "status":
		printMiniGameStatus(client)
	case "help":
		printMiniGameDashboard(client)
	default:
		client.SendServerMessage("Usage: /minigame [slots | blackjack [hit|stand] | crash [start|cashout] | top [slots|blackjack|crash]]")
	}
}

func printMiniGameDashboard(client *Client) {
	client.SendServerMessage(
		"\n🎲 Casual Mini-Games — no stakes, no Pebble rewards\n" +
			"  /minigame slots              — spin the reels\n" +
			"  /minigame blackjack          — deal a hand\n" +
			"  /minigame blackjack hit      — take another card\n" +
			"  /minigame blackjack stand    — finish the hand\n" +
			"  /minigame crash start        — start a round\n" +
			"  /minigame crash cashout      — lock in your multiplier\n" +
			"  /minigame top [game]         — high scores and streaks\n\n" +
			"These games are just for fun. Pebbles only come from activities and world events.")
}

func printMiniGameStatus(client *Client) {
	s := getMiniGameSession(client)
	s.mu.Lock()
	defer s.mu.Unlock()
	client.SendServerMessage(fmt.Sprintf(
		"🎲 Mini-game status\n  Slots streak: %d\n  Blackjack win streak: %d\n  Crash round: %v",
		s.slotsStreak, s.blackjackWin, boolStr(s.crash.active, "active", "idle")))
}

// ── Slots ─────────────────────────────────────────────────────────────────────

func miniSlots(client *Client) {
	if !miniGamesAvailable(client) {
		return
	}
	if !slotsCheckRate(client.Uid()) {
		client.SendServerMessage("Slow down! Max 5 spins per 10 seconds.")
		return
	}
	s1 := slotSymbols[rand.Intn(len(slotSymbols))]
	s2 := slotSymbols[rand.Intn(len(slotSymbols))]
	s3 := slotSymbols[rand.Intn(len(slotSymbols))]
	win := s1 == s2 && s2 == s3
	pair := s1 == s2 || s2 == s3 || s1 == s3

	s := getMiniGameSession(client)
	s.mu.Lock()
	if win {
		s.slotsStreak++
	} else {
		s.slotsStreak = 0
	}
	streak := s.slotsStreak
	s.mu.Unlock()

	var outcome string
	switch {
	case win:
		outcome = "Three of a kind! Lovely spin."
	case pair:
		outcome = "So close — a pair."
	default:
		outcome = "No match this time."
	}
	client.SendServerMessage(fmt.Sprintf(
		"🎰 [ %s | %s | %s ]\n%s\nStreak: %d (no Pebbles, no chips — just fun)",
		s1, s2, s3, outcome, streak))

	if win {
		recordMiniScore(client, miniGameSlots, int64(streak), func(v int64) string {
			return fmt.Sprintf("🎰 %s is on a %d-spin hot streak!", client.OOCName(), v)
		})
	}
}

// ── Blackjack ─────────────────────────────────────────────────────────────────

func miniBlackjack(client *Client, args []string) {
	if !miniGamesAvailable(client) {
		return
	}
	action := "deal"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}
	s := getMiniGameSession(client)
	s.mu.Lock()
	defer s.mu.Unlock()

	switch action {
	case "deal", "new", "start":
		if s.blackjack.active {
			client.SendServerMessage("Finish the current hand first: /minigame blackjack hit or stand.")
			return
		}
		deck := newDeck(1)
		bj := miniBlackjackSession{
			deck:   deck,
			player: []Card{deck[0], deck[1]},
			dealer: []Card{deck[2], deck[3]},
			active: true,
		}
		s.blackjack = bj
		client.SendServerMessage(fmt.Sprintf(
			"🃏 Your hand: %s\nDealer shows: %s\n/minigame blackjack hit or stand. Play for fun — no stakes.",
			handString(BJHand{Cards: bj.player}), bj.dealer[0].String()))
	case "hit":
		if !s.blackjack.active {
			client.SendServerMessage("No hand in progress. Use /minigame blackjack to deal one.")
			return
		}
		s.blackjack.player, s.blackjack.deck = drawMiniCard(s.blackjack.player, s.blackjack.deck)
		total := handTotal(BJHand{Cards: s.blackjack.player})
		if total > 21 {
			client.SendServerMessage(fmt.Sprintf("🃏 You draw %s. Your hand: %s — bust!\nThe dealer wins this one.",
				s.blackjack.player[len(s.blackjack.player)-1].String(),
				handString(BJHand{Cards: s.blackjack.player})))
			s.blackjack.active = false
			s.blackjackWin = 0
			return
		}
		client.SendServerMessage(fmt.Sprintf("🃏 You draw %s. Your hand: %s",
			s.blackjack.player[len(s.blackjack.player)-1].String(),
			handString(BJHand{Cards: s.blackjack.player})))
		if total == 21 {
			miniBlackjackResolve(client, s)
		}
	case "stand":
		if !s.blackjack.active {
			client.SendServerMessage("No hand in progress. Use /minigame blackjack to deal one.")
			return
		}
		for handTotal(BJHand{Cards: s.blackjack.dealer}) < 17 {
			s.blackjack.dealer, s.blackjack.deck = drawMiniCard(s.blackjack.dealer, s.blackjack.deck)
		}
		miniBlackjackResolve(client, s)
	default:
		client.SendServerMessage("Usage: /minigame blackjack [deal|hit|stand]")
	}
}

func drawMiniCard(hand []Card, deck []Card) ([]Card, []Card) {
	if len(deck) == 0 {
		deck = newDeck(1)
	}
	hand = append(hand, deck[0])
	return hand, deck[1:]
}

// miniBlackjackResolve settles a hand and updates the casual win streak. It
// never awards currency.
func miniBlackjackResolve(client *Client, s *miniGameSession) {
	playerTotal := handTotal(BJHand{Cards: s.blackjack.player})
	dealerTotal := handTotal(BJHand{Cards: s.blackjack.dealer})
	s.blackjack.active = false

	result := ""
	win := false
	switch {
	case dealerTotal > 21:
		result = "The dealer busts!"
		win = true
	case playerTotal > dealerTotal:
		result = "You win the hand!"
		win = true
	case playerTotal == dealerTotal:
		result = "Push — a tie."
	default:
		result = "The dealer wins this one."
	}
	if win {
		s.blackjackWin++
	} else if playerTotal != dealerTotal {
		s.blackjackWin = 0
	}
	client.SendServerMessage(fmt.Sprintf(
		"🃏 Dealer: %s\nYou: %s\n%s\nWin streak: %d",
		handString(BJHand{Cards: s.blackjack.dealer}),
		handString(BJHand{Cards: s.blackjack.player}), result, s.blackjackWin))

	if win {
		streak := s.blackjackWin
		recordMiniScore(client, miniGameBlackjack, int64(streak), func(v int64) string {
			return fmt.Sprintf("🃏 %s just won %d blackjack hands in a row!", client.OOCName(), v)
		})
	}
}

// ── Crash ─────────────────────────────────────────────────────────────────────

const (
	miniCrashMin    = 1.05
	miniCrashMax    = 26.0
	miniCrashGrowth = 0.35 // multiplier per second
)

func miniCrash(client *Client, args []string) {
	if !miniGamesAvailable(client) {
		return
	}
	action := "start"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}
	s := getMiniGameSession(client)
	s.mu.Lock()
	defer s.mu.Unlock()

	switch action {
	case "start", "play":
		if s.crash.active {
			client.SendServerMessage("A round is already running. Use /minigame crash cashout.")
			return
		}
		r := rand.Float64()
		crashAt := miniCrashMin + r*r*r*(miniCrashMax-miniCrashMin)
		s.crashGen++
		gen := s.crashGen
		s.crash = miniCrashSession{start: time.Now(), crashAt: crashAt, active: true}
		client.SendServerMessage(
			"🚀 Round started! The multiplier is rising. Use /minigame crash cashout to lock in your score.\n" +
				"No stakes — the worst that can happen is a funny explosion.")
		// Auto-bust. The generation counter ensures a new round is never
		// ended by an old timer.
		s.crash.timer = time.AfterFunc(time.Duration((crashAt-miniCrashMin)/miniCrashGrowth)*time.Second, func() {
			s.mu.Lock()
			if s.crashGen == gen && s.crash.active {
				s.crash.active = false
				s.mu.Unlock()
				client.SendServerMessage(fmt.Sprintf("💥 The round busted at %.2fx. No loss — just try again!", crashAt))
				return
			}
			s.mu.Unlock()
		})
	case "cashout", "bank":
		if !s.crash.active {
			client.SendServerMessage("No round in progress. Use /minigame crash start.")
			return
		}
		s.crash.active = false
		if s.crash.timer != nil {
			s.crash.timer.Stop()
		}
		current := miniCrashMin + time.Since(s.crash.start).Seconds()*miniCrashGrowth
		if current >= s.crash.crashAt {
			client.SendServerMessage(fmt.Sprintf("💥 Too late! The round busted at %.2fx. No loss — just fun.", s.crash.crashAt))
			return
		}
		score := int64(current * 100)
		client.SendServerMessage(fmt.Sprintf(
			"🚀 Locked in at %.2fx! No Pebbles, no chips — just a high score.", current))
		recordMiniScore(client, miniGameCrash, score, func(v int64) string {
			return fmt.Sprintf("🚀 %s set a new Crash record: %.2fx!", client.OOCName(), float64(v)/100)
		})
	default:
		client.SendServerMessage("Usage: /minigame crash [start|cashout]")
	}
}

// ── High scores ───────────────────────────────────────────────────────────────

// recordMiniScore stores an improved score and broadcasts a new personal best
// to the area. `message` builds the announcement for the new record value.
func recordMiniScore(client *Client, game string, score int64, message func(int64) string) {
	if score <= 0 {
		return
	}
	newBest, best, err := db.RecordPebbleHighScore(game, client.Ipid(), score)
	if err != nil || !newBest {
		return
	}
	// Streak games only announce meaningful runs; crash announces any new
	// personal best because the multiplier itself is the score.
	if game != miniGameCrash && best < 3 {
		return
	}
	if a := client.Area(); a != nil {
		sendAreaServerMessage(a, message(best))
	}
}

func miniGameTop(client *Client, args []string) {
	if pebbleModeGuard(client) {
		return
	}
	games := []string{}
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "slots", "slot":
			games = []string{miniGameSlots}
		case "blackjack", "bj":
			games = []string{miniGameBlackjack}
		case "crash":
			games = []string{miniGameCrash}
		default:
			client.SendServerMessage("Usage: /minigame top [slots|blackjack|crash]")
			return
		}
	} else {
		games = []string{miniGameSlots, miniGameBlackjack, miniGameCrash}
	}
	for _, game := range games {
		entries, err := db.GetTopPebbleScores(game, 10)
		if err != nil || len(entries) == 0 {
			client.SendServerMessage(fmt.Sprintf("\n🎲 %s — no scores yet.", miniGameTitle(game)))
			continue
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\n🎲 %s (Top %d)\n", miniGameTitle(game), len(entries)))
		for i, e := range entries {
			name := pebbleDisplayName(e.Username, e.IPID)
			value := fmt.Sprintf("%d", e.Score)
			if game == miniGameCrash {
				value = fmt.Sprintf("%.2fx", float64(e.Score)/100)
			} else {
				value += " in a row"
			}
			sb.WriteString(fmt.Sprintf("  %2d. %v — %v\n", i+1, name, value))
		}
		client.SendServerMessage(sb.String())
	}
}

func miniGameTitle(game string) string {
	switch game {
	case miniGameSlots:
		return "Slots — longest win streak"
	case miniGameBlackjack:
		return "Blackjack — longest win streak"
	case miniGameCrash:
		return "Crash — best multiplier"
	}
	return game
}

// registerMiniGameCommands installs /minigame. Called from
// registerPebbleCommands.
func registerMiniGameCommands() {
	RegisterCommand("minigame", Command{
		handler:   cmdMiniGame,
		minArgs:   0,
		usage:     "Usage: /minigame [slots | blackjack [deal|hit|stand] | crash [start|cashout] | top [game] | status]",
		desc:      "Casual no-stakes mini-games: /minigame slots, blackjack, crash, or top for high scores. No Pebble or chip rewards.",
		reqPerms:  permissions.PermissionField["NONE"],
		pebbleCmd: true,
		category:  "pebble",
	})
}
