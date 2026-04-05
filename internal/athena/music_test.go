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
	"strings"
	"testing"
)

// TestBuildSMPacketEncodesMusicNames verifies that music names with AO2-special
// characters are properly encoded in the SM packet so they don't break the
// packet's '#'-delimited structure.
func TestBuildSMPacketEncodesMusicNames(t *testing.T) {
	tests := []struct {
		name        string
		musicList   []string
		wantEncoded []string // expected encoded form inside the packet
	}{
		{
			name:        "ampersand in music name",
			musicList:   []string{"[T&T] Trial.opus"},
			wantEncoded: []string{"[T<and>T] Trial.opus"},
		},
		{
			name:        "hash in music name",
			musicList:   []string{"song#remix.opus"},
			wantEncoded: []string{"song<num>remix.opus"},
		},
		{
			name:        "percent in music name",
			musicList:   []string{"100% Pure.opus"},
			wantEncoded: []string{"100<percent> Pure.opus"},
		},
		{
			name:        "dollar in music name",
			musicList:   []string{"$money.opus"},
			wantEncoded: []string{"<dollar>money.opus"},
		},
		{
			name:        "no special characters unchanged",
			musicList:   []string{"[aatnt] godot ~ the fragrance of dark coffee.opus", "[s;g] explanation.opus"},
			wantEncoded: []string{"[aatnt] godot ~ the fragrance of dark coffee.opus", "[s;g] explanation.opus"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := buildSMPacket("Lobby", tt.musicList)
			for _, want := range tt.wantEncoded {
				if !strings.Contains(pkt, "#"+want+"#") && !strings.Contains(pkt, "#"+want+"%") {
					t.Errorf("buildSMPacket() packet = %q, want to contain %q", pkt, want)
				}
			}
		})
	}
}

// TestMusicEncodingRoundtrip verifies that encoding a music name and then
// decoding it returns the original string. This mirrors what happens when the
// server encodes names into the SM packet and the client sends them back in MC.
func TestMusicEncodingRoundtrip(t *testing.T) {
	names := []string{
		"[T&T] Trial.opus",
		"100% Pure.opus",
		"song#remix.opus",
		"$money.opus",
		"[s;g] explanation.opus",
		"[aatnt] godot ~ the fragrance of dark coffee.opus",
		"[ddlc] okay, everyone!.opus",
	}

	for _, name := range names {
		encoded := encode(name)
		decoded := decode(encoded)
		if decoded != name {
			t.Errorf("roundtrip failed for %q: encode→decode = %q", name, decoded)
		}
	}
}

// TestMusicSpecialCharsInSMPacket verifies that a music name with special
// characters does not introduce extra '#' fields or early '%' terminators in
// the SM packet.
func TestMusicSpecialCharsInSMPacket(t *testing.T) {
	musicList := []string{"Songs", "[T&T] Trial.opus", "100% Pure.opus", "normal.opus"}
	pkt := buildSMPacket("Lobby", musicList)

	// Packet must start with "SM#" and end with "#%"
	if !strings.HasPrefix(pkt, "SM#") {
		t.Errorf("packet does not start with SM#: %q", pkt)
	}
	if !strings.HasSuffix(pkt, "#%") {
		t.Errorf("packet does not end with #%%: %q", pkt)
	}

	// Strip "SM#" prefix and "#%" suffix, then split on "#" to count fields.
	inner := pkt[3 : len(pkt)-2]
	fields := strings.Split(inner, "#")
	// Expected: 1 area name + len(musicList) music fields = 5
	want := 1 + len(musicList)
	if len(fields) != want {
		t.Errorf("expected %d fields in SM packet, got %d; packet = %q", want, len(fields), pkt)
	}
}
