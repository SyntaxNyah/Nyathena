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

import (
	"sync"
	"testing"
)

func TestInitialCount(t *testing.T) {
	var pc PlayerCount
	if got := pc.GetPlayerCount(); got != 0 {
		t.Errorf("initial GetPlayerCount() = %d, want 0", got)
	}
}

func TestAddPlayer(t *testing.T) {
	var pc PlayerCount
	pc.AddPlayer()
	if got := pc.GetPlayerCount(); got != 1 {
		t.Errorf("after AddPlayer, GetPlayerCount() = %d, want 1", got)
	}
	pc.AddPlayer()
	if got := pc.GetPlayerCount(); got != 2 {
		t.Errorf("after two AddPlayer calls, GetPlayerCount() = %d, want 2", got)
	}
}

func TestRemovePlayer(t *testing.T) {
	var pc PlayerCount
	pc.AddPlayer()
	pc.AddPlayer()
	pc.RemovePlayer()
	if got := pc.GetPlayerCount(); got != 1 {
		t.Errorf("after two adds and one remove, GetPlayerCount() = %d, want 1", got)
	}
}

func TestAddRemoveSymmetric(t *testing.T) {
	var pc PlayerCount
	for i := 0; i < 10; i++ {
		pc.AddPlayer()
	}
	for i := 0; i < 10; i++ {
		pc.RemovePlayer()
	}
	if got := pc.GetPlayerCount(); got != 0 {
		t.Errorf("after 10 adds and 10 removes, GetPlayerCount() = %d, want 0", got)
	}
}

func TestConcurrentAddRemove(t *testing.T) {
	var pc PlayerCount
	var wg sync.WaitGroup
	const n = 1000
	wg.Add(2 * n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			pc.AddPlayer()
		}()
		go func() {
			defer wg.Done()
			pc.RemovePlayer()
		}()
	}
	wg.Wait()
	if got := pc.GetPlayerCount(); got != 0 {
		t.Errorf("after concurrent %d adds and %d removes, GetPlayerCount() = %d, want 0", n, n, got)
	}
}
