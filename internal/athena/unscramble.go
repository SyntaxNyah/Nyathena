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
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	unscrambleReward      = 10              // chips awarded for a correct guess
	unscrambleMinInterval = 30 * time.Minute // minimum time between events
	unscrambleMaxInterval = 3 * time.Hour    // maximum time between events (30 min-3 hr random window)
	unscrambleTimeout     = 5 * time.Minute  // window to answer before the puzzle expires
)

// unscrambleWordList is the pool of words used for unscramble events.
// Words are chosen to be reasonably familiar but not trivially short.
var unscrambleWordList = []string{
	// Law / courtroom
	"attorney", "witness", "verdict", "courtroom", "justice",
	"evidence", "objection", "testimony", "suspect", "alibi",
	"motive", "defense", "prosecution", "argument", "statement",
	"penalty", "innocent", "guilty", "hearing", "ruling",
	"motion", "appeal", "statute", "warrant", "custody",
	"bailiff", "chamber", "counsel", "docket", "exonerate",
	"felony", "granted", "habeas", "inquest", "juror",
	"litigant", "mandate", "notary", "offense", "plaintiff",
	"rebuttal", "sidebar", "tribunal", "deponent", "subpoena",
	// Games / fun
	"puzzle", "scramble", "mystery", "clue", "cipher",
	"riddle", "challenge", "trophy", "victory", "triumph",
	// Nature
	"mountain", "volcano", "glacier", "canyon", "prairie",
	"tornado", "thunder", "lightning", "rainbow", "horizon",
	"waterfall", "cavern", "forest", "desert", "island",
	"ocean", "river", "meadow", "tundra", "swamp",
	// Animals
	"elephant", "penguin", "dolphin", "panther", "leopard",
	"crocodile", "flamingo", "vulture", "antelope", "gorilla",
	"cheetah", "lobster", "sparrow", "hamster", "porcupine",
	"platypus", "salamander", "chameleon", "scorpion", "falcon",
	// Science / tech
	"chemistry", "biology", "physics", "quantum", "molecule",
	"electron", "gravity", "velocity", "frequency", "spectrum",
	"telescope", "microscope", "algorithm", "database", "network",
	"satellite", "asteroid", "nebula", "polymer", "catalyst",
	// Food
	"spaghetti", "chocolate", "avocado", "broccoli", "cinnamon",
	"pineapple", "blueberry", "strawberry", "raspberry", "cantaloupe",
	"asparagus", "artichoke", "mushroom", "eggplant", "cucumber",
	// Adjectives / misc
	"brilliant", "enormous", "fantastic", "gorgeous", "horrible",
	"incredible", "jealous", "knowledge", "luminous", "majestic",
	"nervous", "obvious", "peaceful", "radiant", "splendid",
	"terrible", "ultimate", "vibrant", "whimsical", "zealous",
	// Sports / activity
	"basketball", "volleyball", "football", "baseball", "swimming",
	"marathon", "gymnast", "archery", "fencing", "wrestling",
	"snowboard", "skateboard", "surfboard", "climbing", "cycling",
}

// ── State ────────────────────────────────────────────────────────────────────

type unscrambleState struct {
	mu       sync.RWMutex
	active   bool
	answer   string    // the correct (unscrambled) word in lowercase
	scramble string    // the scrambled version shown to players
	postedAt time.Time // when the puzzle was first announced
	lastWord string    // the previous answer, to avoid immediate repeats
}

var unscramble = unscrambleState{}

// ── Helpers ──────────────────────────────────────────────────────────────────

// scrambleWord returns a shuffled version of word that differs from the original.
// A single Fisher-Yates shuffle is used; if it happens to match the original
// (only possible for words with near-identical letters), the first two distinct
// characters are swapped to guarantee the result is always different.
func scrambleWord(word string) string {
	r := []rune(word)
	n := len(r)
	if n <= 1 { // defensive safeguard; no words in the pool are this short
		return word
	}
	rand.Shuffle(n, func(i, j int) { r[i], r[j] = r[j], r[i] })
	if string(r) == word {
		for i := 1; i < n; i++ {
			if r[i] != r[0] {
				r[0], r[i] = r[i], r[0]
				break
			}
		}
	}
	return string(r)
}

// randomInterval returns a random duration in [min, max].
func randomInterval(min, max time.Duration) time.Duration {
	if d := int64(max - min); d > 0 {
		return min + time.Duration(rand.Int63n(d))
	}
	return min
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
		// Keep re-rolling until a different word from the previous round is chosen.
		for len(unscrambleWordList) > 1 && word == unscramble.lastWord {
			word = unscrambleWordList[rand.Intn(len(unscrambleWordList))]
		}
		shuffled := scrambleWord(word)
		unscramble.active = true
		unscramble.answer = strings.ToLower(word)
		unscramble.scramble = shuffled
		unscramble.postedAt = time.Now()
		unscramble.mu.Unlock()

		sendGlobalServerMessage(fmt.Sprintf(
			"🔤 UNSCRAMBLE EVENT! Unscramble this word in IC chat to win %d chips!\n"+
				"   Scrambled: %s\n"+
				"   You have %d minutes. First correct answer wins!",
			unscrambleReward, strings.ToUpper(shuffled), int(unscrambleTimeout.Minutes()),
		))
		logger.LogInfof("Unscramble: new puzzle posted — scramble=%q answer=%q", shuffled, word)

		// Expire the puzzle after the timeout window.
		time.AfterFunc(unscrambleTimeout, func() {
			unscramble.mu.Lock()
			if !unscramble.active || unscramble.answer != word {
				unscramble.mu.Unlock()
				return
			}
			unscramble.active = false
			unscramble.mu.Unlock()
			sendGlobalServerMessage("⌛ UNSCRAMBLE EXPIRED! Nobody got it in time. The answer was: " + word)
		})
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

	// Fast read-only check — avoids write-lock contention on every IC message.
	unscramble.mu.RLock()
	active, answer := unscramble.active, unscramble.answer
	unscramble.mu.RUnlock()
	if !active || guess != answer {
		return
	}

	// Correct guess — claim the round under a write lock and re-check to
	// prevent a double-award if two players answer at the same instant.
	unscramble.mu.Lock()
	if !unscramble.active || unscramble.answer != answer {
		unscramble.mu.Unlock()
		return
	}
	elapsed := time.Since(unscramble.postedAt)
	unscramble.active = false
	unscramble.lastWord = answer
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
		"🎉 UNSCRAMBLE SOLVED! %v typed \"%s\" in %.2fs — +%d chips awarded!",
		displayName, answer, elapsed.Seconds(), unscrambleReward,
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
		unscramble.mu.RLock()
		active := unscramble.active
		scrambled := unscramble.scramble
		unscramble.mu.RUnlock()

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
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 && v <= 50 {
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
