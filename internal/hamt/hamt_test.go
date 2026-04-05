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

package hamt

import (
	"math/rand"
	"testing"
)

// TestInsertGet verifies that every inserted key can be retrieved and returns
// the correct value.
func TestInsertGet(t *testing.T) {
	h := New()
	pairs := map[uint32]uint32{
		0x00000000: 0xDEADBEEF,
		0xFFFFFFFF: 0xCAFEBABE,
		0x12345678: 0xABCDEF01,
		0x0000001F: 0x00000001,
		0x000003E0: 0x00000002,
		0x00007C00: 0x00000004,
	}
	for k, v := range pairs {
		h.Insert(k, v)
	}
	for k, want := range pairs {
		got, ok := h.Get(k)
		if !ok {
			t.Errorf("Get(%#x) = not found; want %#x", k, want)
		} else if got != want {
			t.Errorf("Get(%#x) = %#x; want %#x", k, got, want)
		}
	}
}

// TestGetMissing verifies that looking up an absent key returns false.
func TestGetMissing(t *testing.T) {
	h := New()
	h.Insert(0xAAAAAAAA, 42)
	if _, ok := h.Get(0xBBBBBBBB); ok {
		t.Error("Get(absent key) returned true; want false")
	}
}

// TestUpdateValue verifies that inserting a key a second time replaces its value.
func TestUpdateValue(t *testing.T) {
	h := New()
	h.Insert(0x1, 100)
	h.Insert(0x1, 200)
	got, ok := h.Get(0x1)
	if !ok || got != 200 {
		t.Errorf("Get after update = (%v, %v); want (200, true)", got, ok)
	}
}

// TestSerializeDeserialize verifies a round-trip through Serialize/Deserialize
// and that all values survive intact.
func TestSerializeDeserialize(t *testing.T) {
	h := New()
	pairs := [][2]uint32{
		{0x00000000, 1},
		{0xFFFFFFFF, 2},
		{0xDEADBEEF, 3},
		{0x12345678, 4},
		{0x0F0F0F0F, 5},
		{0xF0F0F0F0, 6},
	}
	for _, p := range pairs {
		h.Insert(p[0], p[1])
	}

	data := Serialize(h.Root())
	root, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}

	h2 := &HAMT{root: root}
	for _, p := range pairs {
		got, ok := h2.Get(p[0])
		if !ok {
			t.Errorf("after round-trip: Get(%#x) = not found; want %#x", p[0], p[1])
		} else if got != p[1] {
			t.Errorf("after round-trip: Get(%#x) = %#x; want %#x", p[0], got, p[1])
		}
	}
}

// TestXORAllValues verifies that XORAllValues returns the XOR of every stored
// value.
func TestXORAllValues(t *testing.T) {
	h := New()
	want := uint32(0)
	pairs := [][2]uint32{
		{1, 0xAAAA0001},
		{2, 0xBBBB0002},
		{3, 0xCCCC0003},
		{4, 0xDDDD0004},
	}
	for _, p := range pairs {
		h.Insert(p[0], p[1])
		want ^= p[1]
	}
	got := XORAllValues(h.Root())
	if got != want {
		t.Errorf("XORAllValues = %#x; want %#x", got, want)
	}
}

// TestXORAllValuesAfterRoundTrip verifies the XOR answer survives serialization.
func TestXORAllValuesAfterRoundTrip(t *testing.T) {
	h := New()
	want := uint32(0)
	for i := uint32(0); i < 32; i++ {
		v := i*0x1111 + 0xFF
		h.Insert(i, v)
		want ^= v
	}
	data := Serialize(h.Root())
	root, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}
	got := XORAllValues(root)
	if got != want {
		t.Errorf("XORAllValues after round-trip = %#x; want %#x", got, want)
	}
}

// TestLargeRandomRoundTrip inserts many random key/value pairs, serializes,
// deserializes and checks that every key/value pair is still accessible and
// the XOR answer is unchanged.
func TestLargeRandomRoundTrip(t *testing.T) {
	const n = 256
	rng := rand.New(rand.NewSource(42))

	h := New()
	keys := make([]uint32, 0, n)
	vals := make(map[uint32]uint32, n)
	wantXOR := uint32(0)

	for len(keys) < n {
		k := rng.Uint32()
		if _, exists := vals[k]; exists {
			continue
		}
		v := rng.Uint32()
		h.Insert(k, v)
		keys = append(keys, k)
		vals[k] = v
		wantXOR ^= v
	}

	data := Serialize(h.Root())
	root, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}
	h2 := &HAMT{root: root}
	for _, k := range keys {
		got, ok := h2.Get(k)
		if !ok {
			t.Errorf("large round-trip: Get(%#x) not found", k)
		} else if got != vals[k] {
			t.Errorf("large round-trip: Get(%#x) = %#x; want %#x", k, got, vals[k])
		}
	}

	gotXOR := XORAllValues(root)
	if gotXOR != wantXOR {
		t.Errorf("large round-trip: XOR = %#x; want %#x", gotXOR, wantXOR)
	}
}

// TestDeserializeErrors verifies that truncated or corrupt data returns errors.
func TestDeserializeErrors(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated tag", []byte{0x48, 0x4D}},
		{"unknown tag", []byte{0x00, 0x00, 0x00, 0x00}},
		{"array truncated bitmap", []byte{0x48, 0x4D, 0x41, 0x49}}, // "HMAI" only
		{"leaf truncated", []byte{0x48, 0x4D, 0x4C, 0x46, 0x00, 0x00}}, // "HMLF" + 2 bytes (need 8 more)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Deserialize(tc.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
