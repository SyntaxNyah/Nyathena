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

// TestSerializeDeserialize verifies a round-trip through Serialize/Deserialize.
// Because leaf values are intentionally omitted from the wire format, only key
// presence is checked after deserialization (values will be 0).
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
			t.Errorf("after round-trip: Get(%#x) = not found", p[0])
		} else if got != 0 {
			// Values are not transmitted in the wire format; they should be 0
			// after deserialization.
			t.Errorf("after round-trip: Get(%#x) = %#x; want 0 (values not in wire)", p[0], got)
		}
	}
}

// TestXORAllValues verifies that XORAllValues returns the XOR of every stored
// value (in-memory only; values are not transmitted in the wire format).
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

// TestCollectKeysAfterRoundTrip verifies that every key survives
// Serialize/Deserialize and is returned by CollectKeys.
func TestCollectKeysAfterRoundTrip(t *testing.T) {
	h := New()
	wantKeys := map[uint32]struct{}{
		0x00000000: {},
		0xFFFFFFFF: {},
		0x12345678: {},
		0x0F0F0F0F: {},
		0xF0F0F0F0: {},
		0xDEADBEEF: {},
	}
	for k := range wantKeys {
		h.Insert(k, 0)
	}

	data := Serialize(h.Root())
	root, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}

	got := CollectKeys(root)
	if len(got) != len(wantKeys) {
		t.Fatalf("CollectKeys returned %d keys; want %d", len(got), len(wantKeys))
	}
	for _, k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Errorf("CollectKeys returned unexpected key %#x", k)
		}
	}
}

// TestComputeHMACAnswer verifies that ComputeHMACAnswer produces a consistent,
// non-trivial answer and that the answer changes when the nonce changes.
func TestComputeHMACAnswer(t *testing.T) {
	h := New()
	keys := []uint32{0x00000001, 0x00000002, 0x00000003, 0x00000004}
	for _, k := range keys {
		h.Insert(k, 0)
	}

	nonce1 := []byte("nonce-number-one")
	nonce2 := []byte("nonce-number-two")

	ans1a := ComputeHMACAnswer(h.Root(), nonce1)
	ans1b := ComputeHMACAnswer(h.Root(), nonce1)
	ans2 := ComputeHMACAnswer(h.Root(), nonce2)

	if ans1a != ans1b {
		t.Errorf("ComputeHMACAnswer is not deterministic: %#x != %#x", ans1a, ans1b)
	}
	if ans1a == ans2 {
		t.Errorf("different nonces produced the same answer %#x", ans1a)
	}
}

// TestComputeHMACAnswerRoundTrip verifies that the expected answer can be
// reproduced from a deserialized HAMT plus the original nonce.  This is the
// exact flow a compliant client must implement.
func TestComputeHMACAnswerRoundTrip(t *testing.T) {
	h := New()
	keys := []uint32{0xDEADBEEF, 0xCAFEBABE, 0x12345678, 0xABCDEF01}
	for _, k := range keys {
		h.Insert(k, 0)
	}

	nonce := []byte("test-nonce-16byt")
	want := ComputeHMACAnswer(h.Root(), nonce)

	// Simulate what the client receives and computes.
	data := Serialize(h.Root())
	root, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}
	got := ComputeHMACAnswer(root, nonce)

	if got != want {
		t.Errorf("answer after round-trip = %#x; want %#x", got, want)
	}
}

// TestLargeRandomRoundTrip inserts many random keys, serializes, deserializes
// and checks that every key is still accessible.  It also verifies that
// ComputeHMACAnswer is stable across a serialize/deserialize round-trip.
func TestLargeRandomRoundTrip(t *testing.T) {
	const n = 256
	rng := rand.New(rand.NewSource(42))

	h := New()
	keys := make([]uint32, 0, n)
	keySet := make(map[uint32]struct{}, n)

	for len(keys) < n {
		k := rng.Uint32()
		if _, exists := keySet[k]; exists {
			continue
		}
		h.Insert(k, 0)
		keys = append(keys, k)
		keySet[k] = struct{}{}
	}

	nonce := []byte("large-test-nonce")
	wantAnswer := ComputeHMACAnswer(h.Root(), nonce)

	data := Serialize(h.Root())
	root, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}
	h2 := &HAMT{root: root}
	for _, k := range keys {
		if _, ok := h2.Get(k); !ok {
			t.Errorf("large round-trip: Get(%#x) not found", k)
		}
	}

	gotAnswer := ComputeHMACAnswer(root, nonce)
	if gotAnswer != wantAnswer {
		t.Errorf("large round-trip: HMAC answer = %#x; want %#x", gotAnswer, wantAnswer)
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
		{"leaf truncated", []byte{0x48, 0x4D, 0x4C, 0x46, 0x00}}, // "HMLF" + 1 byte (need 4 more)
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

