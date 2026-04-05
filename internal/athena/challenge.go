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

// generateChallenge builds a HAMT containing n unique random uint32 key→value
// pairs, serializes it to the wire format, base64-encodes it for transport
// over the AO2 text protocol, and returns both the encoded payload and the
// expected answer (XOR of all inserted values).
//
// The client must:
//  1. Base64-decode the payload.
//  2. Deserialize the binary HAMT (see internal/hamt for the wire format).
//  3. Traverse every LeafNode and XOR all values together.
//  4. Reply with that uint32 formatted as a lowercase hex string.
func generateChallenge(n int) (payload string, answer uint32) {
	h := hamt.New()
	seen := make(map[uint32]struct{}, n)
	var xorAcc uint32

	for len(seen) < n {
		k := cryptoRandUint32()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		v := cryptoRandUint32()
		h.Insert(k, v)
		xorAcc ^= v
	}

	raw := hamt.Serialize(h.Root())
	payload = base64.StdEncoding.EncodeToString(raw)
	answer = xorAcc
	return
}
