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
	"sync"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
)

// SlotsStats tracks cumulative slot machine statistics for an area.
type SlotsStats struct {
	TotalSpins  int64
	TotalPayout int64
	Jackpots    int64
}

// AreaCasinoState holds the per-area casino state.
type AreaCasinoState struct {
	mu           sync.Mutex
	bjTable      *BJTable
	pokerTable   *PokerTable
	slotsStats   SlotsStats
	activeTables int
}

var casinoStates sync.Map // key: *area.Area, value: *AreaCasinoState

// getCasinoState returns or creates the casino state for the given area.
func getCasinoState(a *area.Area) *AreaCasinoState {
	v, _ := casinoStates.LoadOrStore(a, &AreaCasinoState{})
	return v.(*AreaCasinoState)
}

// validateBet checks the bet against area min/max limits and the player's chip balance.
// Returns (valid bool, reason string).
func validateBet(client *Client, amount int64) (bool, string) {
	if amount <= 0 {
		return false, "Bet must be greater than 0."
	}
	minBet := int64(client.Area().CasinoMinBet())
	maxBet := int64(client.Area().CasinoMaxBet())
	if minBet > 0 && amount < minBet {
		return false, fmt.Sprintf("Minimum bet is %d chips.", minBet)
	}
	if maxBet > 0 && amount > maxBet {
		return false, fmt.Sprintf("Maximum bet is %d chips.", maxBet)
	}
	balance, err := db.GetChipBalance(client.Ipid())
	if err != nil || balance < amount {
		return false, fmt.Sprintf("Insufficient chips. Your balance: %d", balance)
	}
	return true, ""
}

// handleCasinoDisconnect cleans up casino state when a client disconnects.
// Wire this into clientCleanup in client.go.
func handleCasinoDisconnect(client *Client) {
	casinoStates.Range(func(_, value interface{}) bool {
		cs := value.(*AreaCasinoState)
		cs.mu.Lock()
		bj := cs.bjTable
		poker := cs.pokerTable
		cs.mu.Unlock()
		if bj != nil {
			bjHandleDisconnect(bj, client)
		}
		if poker != nil {
			pokerHandleDisconnect(poker, client)
		}
		return true
	})
}
