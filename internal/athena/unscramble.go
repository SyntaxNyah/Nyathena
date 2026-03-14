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
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	unscrambleReward      = 10             // chips awarded for a correct guess
	unscrambleMinInterval = 30 * time.Minute // minimum time between events
	unscrambleMaxInterval = 60 * time.Minute // maximum time between events
	unscrambleTimeout     = 5 * time.Minute  // window to answer before the puzzle expires
)

// unscrambleWordList is the pool of words used for unscramble events.
// Words are chosen to be reasonably familiar but not trivially short.
var unscrambleWordList = []string{
	"attorney", "witness", "verdict", "courtroom", "justice",
	"evidence", "objection", "testimony", "suspect", "alibi",
	"motive", "defense", "prosecution", "argument", "statement",
	"penalty", "innocent", "guilty", "hearing", "ruling",
	"motion", "appeal", "statute", "warrant", "custody",
	"bailiff", "chamber", "counsel", "docket", "exonerate",
	"felony", "granted", "habeas", "inquest", "juror",
	"litigant", "mandate", "notary", "offense", "plaintiff",
	"rebuttal", "sidebar", "tribunal", "deponent", "subpoena",
	"puzzle", "scramble", "mystery", "clue", "cipher",
	"riddle", "challenge", "trophy", "victory", "triumph",
}

// ── State ────────────────────────────────────────────────────────────────────

type unscrambleState struct {
	mu       sync.Mutex
	active   bool
	answer   string // the correct (unscrambled) word in lowercase
	scramble string // the scrambled version shown to players
	expireAt time.Time
}

var unscramble = unscrambleState{}

// ── Helpers ──────────────────────────────────────────────────────────────────

// scrambleWord returns a shuffled version of the input word that differs from
// the original. Multiple attempts are made to ensure it is visibly scrambled.
func scrambleWord(word string) string {
	runes := []rune(word)
	for attempt := 0; attempt < 10; attempt++ {
		rand.Shuffle(len(runes), func(i, j int) { runes[i], runes[j] = runes[j], runes[i] })
		if string(runes) != word {
			return string(runes)
		}
	}
	// Fallback: reverse the word (guaranteed to differ for words length > 1).
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// randomInterval returns a random duration in [min, max].
func randomInterval(min, max time.Duration) time.Duration {
	delta := int64(max - min)
	if delta <= 0 {
		return min
	}
	return min + time.Duration(rand.Int63n(delta))
}

// ── Background loop ──────────────────────────────────────────────────────────

// startUnscrambleLoop runs in the background and periodically posts a word
// unscramble challenge to all connected players via OOC broadcast.
// The loop picks a random delay in [unscrambleMinInterval, unscrambleMaxInterval]
// between events so the schedule is unpredictable.
func startUnscrambleLoop() {
	for {
		delay := randomInterval(unscrambleMinInterval, unscrambleMaxInterval)
		time.Sleep(delay)

		if config == nil || !config.EnableCasino {
			continue
		}
		// Only start a new round when no round is currently active.
		unscramble.mu.Lock()
		if unscramble.active {
			unscramble.mu.Unlock()
			continue
		}

		word := unscrambleWordList[rand.Intn(len(unscrambleWordList))]
		shuffled := scrambleWord(word)
		unscramble.active = true
		unscramble.answer = strings.ToLower(word)
		unscramble.scramble = shuffled
		unscramble.expireAt = time.Now().Add(unscrambleTimeout)
		unscramble.mu.Unlock()

		sendGlobalServerMessage(fmt.Sprintf(
			"🔤 UNSCRAMBLE EVENT! Unscramble this word in IC chat to win %d chips!\n"+
				"   Scrambled: %s\n"+
				"   You have %d minutes. First correct answer wins!",
			unscrambleReward, strings.ToUpper(shuffled), int(unscrambleTimeout.Minutes()),
		))
		logger.LogInfof("Unscramble: new puzzle posted — scramble=%q answer=%q", shuffled, word)

		// Schedule the timeout expiry.
		go func(answer, shuffled string) {
			time.Sleep(unscrambleTimeout)
			unscramble.mu.Lock()
			if !unscramble.active || unscramble.answer != answer {
				unscramble.mu.Unlock()
				return
			}
			unscramble.active = false
			unscramble.mu.Unlock()
			sendGlobalServerMessage(fmt.Sprintf(
				"⌛ UNSCRAMBLE EXPIRED! Nobody got it in time. The answer was: %s", answer,
			))
		}(word, shuffled)
	}
}

// ── IC hook ──────────────────────────────────────────────────────────────────

// unscrambleOnIC is called from pktIC for every in-character message.
// If an unscramble round is active and the decoded message matches the answer,
// the sender wins and receives the chip reward.
func unscrambleOnIC(client *Client, msgText string) {
	guess := strings.ToLower(strings.TrimSpace(msgText))
	if guess == "" {
		return
	}

	unscramble.mu.Lock()
	if !unscramble.active {
		unscramble.mu.Unlock()
		return
	}
	if guess != unscramble.answer {
		unscramble.mu.Unlock()
		return
	}
	// Correct answer — close the round atomically.
	answer := unscramble.answer
	unscramble.active = false
	unscramble.mu.Unlock()

	ipid := client.Ipid()
	displayName := client.OOCName()

	// Award chips.
	newBal, chipErr := db.AddChips(ipid, unscrambleReward)
	if chipErr != nil {
		logger.LogErrorf("unscramble: AddChips failed for %v: %v", ipid, chipErr)
	}

	// Record win for leaderboard.
	if winErr := db.AddUnscrambleWin(ipid); winErr != nil {
		logger.LogErrorf("unscramble: AddUnscrambleWin failed for %v: %v", ipid, winErr)
	}

	sendGlobalServerMessage(fmt.Sprintf(
		"🎉 UNSCRAMBLE SOLVED! %v got it right — the answer was \"%s\"! +%d chips awarded.",
		displayName, answer, unscrambleReward,
	))
	if chipErr == nil {
		client.SendServerMessage(fmt.Sprintf(
			"🔤 Correct! You won the unscramble challenge. +%d chips | Balance: %d chips",
			unscrambleReward, newBal,
		))
	}
	logger.LogInfof("Unscramble: solved by %v (ipid=%v) answer=%q", displayName, ipid, answer)
}

// ── /unscramble command ──────────────────────────────────────────────────────

// cmdUnscramble handles /unscramble [top [n]].
func cmdUnscramble(client *Client, args []string, _ string) {
	if len(args) == 0 {
		// Show the player their own win count and the current puzzle status.
		wins, err := db.GetUnscrambleWins(client.Ipid())
		if err != nil {
			client.SendServerMessage("Could not retrieve your unscramble stats.")
			return
		}
		unscramble.mu.Lock()
		active := unscramble.active
		scrambled := unscramble.scramble
		unscramble.mu.Unlock()

		msg := fmt.Sprintf("🔤 Your unscramble wins: %d", wins)
		if active {
			msg += fmt.Sprintf("\n   Active puzzle: %s — type the answer in IC chat to win %d chips!", strings.ToUpper(scrambled), unscrambleReward)
		} else {
			msg += "\n   No active unscramble puzzle right now."
		}
		client.SendServerMessage(msg)
		return
	}

	if strings.ToLower(args[0]) == "top" {
		n := 10
		if len(args) > 1 {
			if v := parseInt(args[1]); v > 0 && v <= 50 {
				n = v
			}
		}
		entries, err := db.GetTopUnscrambleWins(n)
		if err != nil || len(entries) == 0 {
			client.SendServerMessage("No unscramble data available.")
			return
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\n🔤 Unscramble Leaderboard (Top %d)\n", n))
		for i, e := range entries {
			name := e.Username
			if name == "" {
				if len(e.IPID) > 8 {
					name = e.IPID[:8] + "…"
				} else if len(e.IPID) > 0 {
					name = e.IPID
				} else {
					name = "Anonymous"
				}
			}
			sb.WriteString(fmt.Sprintf("  %2d. %v — %d wins\n", i+1, name, e.Wins))
		}
		client.SendServerMessage(sb.String())
		return
	}

	client.SendServerMessage("Usage: /unscramble [top [n]]")
}

// parseInt parses a string into an int, returning 0 on failure.
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}


