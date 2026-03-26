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

// plinkoRows is the number of peg rows on the board.
// With N rows there are N+1 possible final slots (0..N).
const plinkoRows = 8

// plinkoMaxBet is the per-drop bet ceiling for Plinko specifically.
const plinkoMaxBet = 10_000

// plinkoMultipliers maps risk level → per-slot payout multipliers.
// Slot distribution is binomial(8, 0.5); centre slot (4) is the MOST LIKELY.
// Highest multipliers are placed on the edge slots (hardest to hit), matching
// standard plinko design — this ensures wins occur at varied multipliers rather
// than a single centre value.
//
// House edge (approx): low ~5.4%, med ~4.5%, high ~26.9%
var plinkoMultipliers = map[string][]float64{
	// Low  : edges 2.0×, gentle bell, centre 0.5× loss  (~5.4% house edge)
	"low": {2.0, 1.5, 1.2, 1.0, 0.5, 1.0, 1.2, 1.5, 2.0},
	// Med  : edges 5.0×, centre wipes bet               (~4.5% house edge)
	"med": {5.0, 2.5, 1.5, 0.8, 0.3, 0.8, 1.5, 2.5, 5.0},
	// High : edges 20×, centre zero — feast or famine   (~26.9% house edge)
	"high": {20.0, 5.0, 1.0, 0.1, 0.0, 0.1, 1.0, 5.0, 20.0},
}

// plinkoRateLimit tracks recent drop timestamps per player (uid → *[]time.Time).
var plinkoRateLimit sync.Map

// plinkoCheckRate returns true if the player is within the allowed drop rate.
// Limit: 3 drops per 10 seconds.
func plinkoCheckRate(uid int) bool {
	now := time.Now()
	cutoff := now.Add(-10 * time.Second)

	val, _ := plinkoRateLimit.LoadOrStore(uid, &[]time.Time{})
	ts := val.(*[]time.Time)

	valid := (*ts)[:0]
	for _, t := range *ts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	*ts = valid

	if len(*ts) >= 3 {
		return false
	}
	if len(*ts) == 0 {
		plinkoRateLimit.Delete(uid)
		fresh := &[]time.Time{now}
		plinkoRateLimit.Store(uid, fresh)
		return true
	}
	*ts = append(*ts, now)
	return true
}

// plinkoSimulate drops a ball through the board and returns the per-row path.
//
// path[r] = ball column at peg row r (0-indexed), range [0..r].
// path[plinkoRows] = final slot index (range [0..plinkoRows]).
//
// At each row the ball bounces either left (stay) or right (+1), giving a
// natural binomial distribution over final slots.
func plinkoSimulate() []int {
	path := make([]int, plinkoRows+1)
	pos := 0
	for r := 0; r <= plinkoRows; r++ {
		path[r] = pos
		if r < plinkoRows {
			pos += rand.Intn(2)
		}
	}
	return path
}

// plinkoBoard renders an ASCII Plinko board with the ball path marked (●) and
// the winning slot highlighted with brackets.
func plinkoBoard(path []int, risk string, multipliers []float64) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("═══════ PLINKO  [%s RISK] ═══════\n", strings.ToUpper(risk)))

	// Peg rows: row r has (r+1) pegs, indented so the board forms a triangle.
	for r := 0; r < plinkoRows; r++ {
		indent := plinkoRows - 1 - r
		sb.WriteString(strings.Repeat(" ", indent))
		ballAt := path[r]
		pegs := r + 1
		for i := 0; i < pegs; i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			if i == ballAt {
				sb.WriteString("●")
			} else {
				sb.WriteString("·")
			}
		}
		sb.WriteByte('\n')
	}

	// Slot row below the final peg row.
	finalSlot := path[plinkoRows]
	for i, m := range multipliers {
		var cell string
		if m == 0 {
			cell = "0x"
		} else if m == float64(int(m)) {
			cell = fmt.Sprintf("%.0fx", m)
		} else {
			cell = fmt.Sprintf("%.1fx", m)
		}
		if i == finalSlot {
			sb.WriteString("[" + cell + "]")
		} else {
			sb.WriteString(" " + cell + " ")
		}
		if i < len(multipliers)-1 {
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

// cmdPlinko handles /plinko subcommands.
// Usage: /plinko drop <low|med|high> <bet>
func cmdPlinko(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.SendServerMessage(
			"Drop a chip down the peg board and win based on where it lands!\n" +
				"Usage: /plinko drop <risk> <bet>\n" +
				"  risk: low  (0.5×–2.0×, gentle swings, edges pay best)\n" +
				"        med  (0.3×–5.0×, moderate risk, edges pay best)\n" +
				"        high (0×–20×, feast or famine, edges pay best)\n" +
				fmt.Sprintf("Max bet per drop: %d chips\n", plinkoMaxBet) +
				"Example: /plinko drop med 500",
		)
		return
	}

	switch strings.ToLower(args[0]) {
	case "drop":
		if len(args) < 3 {
			client.SendServerMessage("Usage: /plinko drop <low|med|high> <bet>")
			return
		}
		risk := strings.ToLower(args[1])
		multipliers, ok := plinkoMultipliers[risk]
		if !ok {
			client.SendServerMessage("Invalid risk level. Choose: low | med | high")
			return
		}
		bet, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil || bet <= 0 {
			client.SendServerMessage("Invalid bet amount.")
			return
		}
		if bet > plinkoMaxBet {
			client.SendServerMessage(fmt.Sprintf("Plinko bets are capped at %d chips per drop.", plinkoMaxBet))
			return
		}

		if !plinkoCheckRate(client.Uid()) {
			client.SendServerMessage("Slow down! Max 3 drops per 10 seconds.")
			return
		}
		if valid, reason := validateBet(client, bet); !valid {
			client.SendServerMessage(reason)
			return
		}
		balAfterBet, ok := spendBet(client, bet)
		if !ok {
			return
		}

		path := plinkoSimulate()
		finalSlot := path[plinkoRows]
		mult := multipliers[finalSlot]
		board := plinkoBoard(path, risk, multipliers)
		payout := int64(float64(bet) * mult)

		var bal int64
		var resultLine string
		switch {
		case payout > bet:
			bal, _ = db.AddChips(client.Ipid(), payout)
			net := payout - bet
			resultLine = fmt.Sprintf("Slot %d (%.1fx) — Win! +%d chips. Balance: %d", finalSlot, mult, net, bal)
			if mult >= 5.0 {
				sendAreaGamblingMessage(client.Area(),
					fmt.Sprintf("🎯 PLINKO! %s hit %.0fx for +%d chips!", client.OOCName(), mult, net))
			}
		case payout == bet:
			bal = balAfterBet
			resultLine = fmt.Sprintf("Slot %d (%.1fx) — Push! Bet returned. Balance: %d", finalSlot, mult, bal)
		case payout > 0:
			// Partial return: add payout back (less than bet was taken).
			bal, _ = db.AddChips(client.Ipid(), payout)
			resultLine = fmt.Sprintf("Slot %d (%.1fx) — Lost %d chips. Balance: %d", finalSlot, mult, bet-payout, bal)
		default:
			// mult == 0: full loss, balance already deducted by spendBet.
			bal = balAfterBet
			resultLine = fmt.Sprintf("Slot %d (0×) — Lost %d chips. Balance: %d", finalSlot, bet, bal)
		}

		client.SendServerMessage(board + "\n" + resultLine)

	default:
		client.SendServerMessage("Usage: /plinko drop <low|med|high> <bet>")
	}
}
