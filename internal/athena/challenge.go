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
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"

	"github.com/MangosArentLiterature/Athena/internal/hamt"
)

// cryptoRandUint32 reads a uniformly random uint32 from the OS CSPRNG.
// It panics if crypto/rand is unavailable, which would indicate a severe
// system fault and should never happen in practice.
func cryptoRandUint32() uint32 {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("hamt challenge: crypto/rand.Read failed: " + err.Error())
	}
	return binary.BigEndian.Uint32(buf[:])
}

// generateChallenge builds a HAMT containing n unique random keys, serializes
// it to the wire format (keys only — values are NOT in the wire), and produces
// a base64-encoded payload of the form:
//
//	base64( nonce(16 bytes) || serialized_HAMT )
//
// The expected answer is the XOR of HMAC-SHA256(nonce, big-endian(key)) for
// every leaf key, i.e. hamt.ComputeHMACAnswer(root, nonce).
//
// The client must:
//  1. Base64-decode the payload.
//  2. Extract the first 16 bytes as the nonce.
//  3. Deserialize the remaining bytes as a HAMT (leaf nodes carry keys only).
//  4. For every leaf key k compute first-4-bytes-of HMAC-SHA256(nonce, BE(k)).
//  5. XOR all those uint32 values together.
//  6. Reply with that uint32 formatted as a lowercase hex string.
//
// Because leaf values are absent from the wire, the answer cannot be recovered
// by a raw byte-scan of the payload; trie traversal and HMAC computation are
// both required.
func generateChallenge(n int) (payload string, answer uint32) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic("hamt challenge: crypto/rand.Read failed (nonce): " + err.Error())
	}

	h := hamt.New()
	seen := make(map[uint32]struct{}, n)

	for len(seen) < n {
		k := cryptoRandUint32()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		h.Insert(k, 0) // values are derived via HMAC; not stored in wire
	}

	answer = hamt.ComputeHMACAnswer(h.Root(), nonce[:])

	hamtBytes := hamt.Serialize(h.Root())
	blob := make([]byte, 16+len(hamtBytes))
	copy(blob[:16], nonce[:])
	copy(blob[16:], hamtBytes)
	payload = base64.StdEncoding.EncodeToString(blob)
	return
}
