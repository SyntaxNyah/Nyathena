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

package area

import "testing"

func TestTestimony(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// Append a new statement
	a.TstAppend("foo")
	if a.tr.Testimony[0] != "foo" {
		t.Errorf("unexpected value for Testimony[0], got %s, want %s", a.tr.Testimony[0], "foo")
	}
	if a.TstLen() != 1 {
		t.Errorf("unexpected value for testimony length, got %d, want %d", a.TstLen(), 1)
	}

	// Insert a new statement at posistion 1
	a.TstInsert("bar")
	if a.tr.Testimony[1] != "bar" {
		t.Errorf("unexpected value for Testimony[1], got %s, want %s", a.tr.Testimony[1], "bar")
	}

	// Advance index
	a.TstAdvance()
	if a.CurrentTstIndex() != 1 {
		t.Errorf("unexpected value for CurrentTstIndex(), got %d, want %d", a.CurrentTstIndex(), 1)
	}
	if a.CurrentTstStatement() != "bar" {
		t.Errorf("unexpected value for CurrentTstStatement(), got %s, want %s", a.CurrentTstStatement(), "bar")
	}

	// Advance beyond index, should remain at 1
	a.TstAdvance()
	if a.CurrentTstIndex() != 1 {
		t.Errorf("unexpected value for CurrentTstIndex(), got %d, want %d", a.CurrentTstIndex(), 1)
	}

	// Rewind index, should remain at 1
	a.TstRewind()
	if a.CurrentTstIndex() != 1 {
		t.Errorf("unexpected value for CurrentTstIndex(), got %d, want %d", a.CurrentTstIndex(), 1)
	}
}

// TestTestimonyRemoveLast verifies that removing the current (last) statement
// clamps the index so subsequent navigation/access does not panic.
func TestTestimonyRemoveLast(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	a.TstAppend("title")
	a.TstAppend("a")
	a.TstAppend("b")
	a.TstAppend("c")

	a.TstJump(3)
	if a.CurrentTstIndex() != 3 {
		t.Fatalf("expected index 3, got %d", a.CurrentTstIndex())
	}

	if err := a.TstRemove(); err != nil {
		t.Fatalf("TstRemove returned error: %v", err)
	}
	if a.TstLen() != 3 {
		t.Fatalf("expected len 3, got %d", a.TstLen())
	}
	if a.CurrentTstIndex() >= a.TstLen() {
		t.Fatalf("index %d out of range for len %d", a.CurrentTstIndex(), a.TstLen())
	}

	// Navigation after deletion must not panic.
	a.TstAdvance()
	_ = a.CurrentTstStatement()
	a.TstRewind()
	_ = a.CurrentTstStatement()
}

// TestTestimonyJumpClamp verifies TstJump clamps out-of-range indices.
func TestTestimonyJumpClamp(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.TstAppend("title")
	a.TstAppend("a")

	a.TstJump(99)
	if a.CurrentTstIndex() >= a.TstLen() {
		t.Fatalf("index %d out of range for len %d", a.CurrentTstIndex(), a.TstLen())
	}
	_ = a.CurrentTstStatement()

	a.TstJump(-5)
	if a.CurrentTstIndex() < 0 {
		t.Fatalf("index %d should not be negative", a.CurrentTstIndex())
	}
}
