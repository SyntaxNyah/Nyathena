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

func TestTryAddPlayer(t *testing.T) {
	var pc PlayerCount

	// Fill to max=3
	for i := 0; i < 3; i++ {
		if !pc.TryAddPlayer(3) {
			t.Fatalf("expected TryAddPlayer to succeed on call %d", i+1)
		}
	}
	if pc.GetPlayerCount() != 3 {
		t.Fatalf("expected count 3, got %d", pc.GetPlayerCount())
	}

	// At capacity: must fail
	if pc.TryAddPlayer(3) {
		t.Fatal("expected TryAddPlayer to fail when at max")
	}
	if pc.GetPlayerCount() != 3 {
		t.Fatalf("count must not change after failed TryAddPlayer, got %d", pc.GetPlayerCount())
	}
}

// TestTryAddPlayerRace verifies there is no data race and that concurrent
// calls never exceed the configured maximum.
func TestTryAddPlayerRace(t *testing.T) {
	const goroutines = 200
	const max = 50

	var pc PlayerCount
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			pc.TryAddPlayer(max)
		}()
	}
	wg.Wait()

	if got := pc.GetPlayerCount(); got > max {
		t.Fatalf("player count %d exceeded maximum %d", got, max)
	}
}
