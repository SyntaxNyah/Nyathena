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

package uidheap

import (
	"container/heap"
	"testing"
)

func TestPushPop(t *testing.T) {
	h := &UidHeap{}
	heap.Init(h)

	heap.Push(h, 3)
	heap.Push(h, 1)
	heap.Push(h, 2)

	if h.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", h.Len())
	}

	// heap.Pop should return the minimum element first.
	got := heap.Pop(h).(int)
	if got != 1 {
		t.Errorf("first Pop = %d, want 1", got)
	}
	got = heap.Pop(h).(int)
	if got != 2 {
		t.Errorf("second Pop = %d, want 2", got)
	}
	got = heap.Pop(h).(int)
	if got != 3 {
		t.Errorf("third Pop = %d, want 3", got)
	}
	if h.Len() != 0 {
		t.Errorf("Len() after all pops = %d, want 0", h.Len())
	}
}

func TestLess(t *testing.T) {
	h := UidHeap{5, 3, 7}
	if !h.Less(1, 0) {
		t.Errorf("Less(1,0): 3 < 5 should be true")
	}
	if h.Less(0, 1) {
		t.Errorf("Less(0,1): 5 < 3 should be false")
	}
}

func TestSwap(t *testing.T) {
	h := UidHeap{10, 20}
	h.Swap(0, 1)
	if h[0] != 20 || h[1] != 10 {
		t.Errorf("after Swap: h = %v, want [20 10]", []int(h))
	}
}

func TestLen(t *testing.T) {
	h := UidHeap{}
	if h.Len() != 0 {
		t.Errorf("empty Len() = %d, want 0", h.Len())
	}
	h = UidHeap{1, 2, 3, 4}
	if h.Len() != 4 {
		t.Errorf("Len() = %d, want 4", h.Len())
	}
}

func TestMinHeapOrder(t *testing.T) {
	h := &UidHeap{}
	heap.Init(h)
	values := []int{10, 3, 7, 1, 5, 9}
	for _, v := range values {
		heap.Push(h, v)
	}
	prev := heap.Pop(h).(int)
	for h.Len() > 0 {
		next := heap.Pop(h).(int)
		if next < prev {
			t.Errorf("heap order violated: %d came after %d", next, prev)
		}
		prev = next
	}
}
