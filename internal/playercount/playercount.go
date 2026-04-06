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

package playercount

import "sync/atomic"

type PlayerCount struct {
	players int64
}

// GetPlayerCount returns the current player count.
func (pc *PlayerCount) GetPlayerCount() int {
	return int(atomic.LoadInt64(&pc.players))
}

// AddPlayer increments the player count by one.
func (pc *PlayerCount) AddPlayer() {
	atomic.AddInt64(&pc.players, 1)
}

// RemovePlayer decrements the player count by one.
func (pc *PlayerCount) RemovePlayer() {
	atomic.AddInt64(&pc.players, -1)
}

// TryAddPlayer atomically increments the player count only if it is strictly
// below max. It returns true when the slot was reserved, false when the server
// is already full. Using compare-and-swap closes the TOCTOU race that would
// otherwise let concurrent handshakes push the count past MaxPlayers.
func (pc *PlayerCount) TryAddPlayer(max int) bool {
	for {
		current := atomic.LoadInt64(&pc.players)
		if int(current) >= max {
			return false
		}
		if atomic.CompareAndSwapInt64(&pc.players, current, current+1) {
			return true
		}
	}
}
