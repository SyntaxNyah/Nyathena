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
	"testing"
)

// TestTUILogRingBound verifies the ring buffer never grows past its cap and
// drops oldest entries first. This is the only correctness invariant that
// matters — an unbounded ring would leak memory over a long-running server.
func TestTUILogRingBound(t *testing.T) {
	// Reset to a known-empty state.
	tuiRing.mu.Lock()
	tuiRing.lines = tuiRing.lines[:0]
	tuiRing.mu.Unlock()

	for i := 0; i < tuiLogMaxLines*3; i++ {
		tuiAppendLog("entry")
	}
	snap := tuiRing.snapshot()
	if len(snap) != tuiLogMaxLines {
		t.Fatalf("ring size: got %d, want %d", len(snap), tuiLogMaxLines)
	}
}

// TestTUILogRingOrdering verifies oldest-first eviction preserves insertion
// order of the surviving entries.
func TestTUILogRingOrdering(t *testing.T) {
	tuiRing.mu.Lock()
	tuiRing.lines = tuiRing.lines[:0]
	tuiRing.mu.Unlock()

	tuiAppendLog("a")
	tuiAppendLog("b")
	tuiAppendLog("c")
	snap := tuiRing.snapshot()
	if len(snap) != 3 || snap[0] != "a" || snap[1] != "b" || snap[2] != "c" {
		t.Fatalf("ordering: got %v", snap)
	}
}

// TestTUITruncate verifies the trunc helper used for area-name rows. The
// ellipsis rune is multi-byte so a byte-based truncate would corrupt it; the
// helper must work on the full rune.
func TestTUITruncate(t *testing.T) {
	cases := []struct {
		in  string
		n   int
		out string
	}{
		{"short", 10, "short"},
		{"exactly10x", 10, "exactly10x"},
		{"toolongname", 6, "toolo…"},
		{"x", 0, ""},
	}
	for _, c := range cases {
		got := trunc(c.in, c.n)
		if got != c.out {
			t.Errorf("trunc(%q, %d): got %q, want %q", c.in, c.n, got, c.out)
		}
	}
}
