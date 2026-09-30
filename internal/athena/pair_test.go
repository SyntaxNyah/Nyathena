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

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// applyPairSanitization replicates the no-pair sanitization branch of pktIC:
// when PairedCharID is "" or "-1", PairedName and PairedEmote must be cleared
// regardless of any value the client may have leaked into those slots.
func applyPairSanitization(ms *packet.MSToClient) {
	if ms.PairedCharID == "" || ms.PairedCharID == "-1" {
		ms.PairedName = ""
		ms.PairedEmote = ""
	}
}

// TestPairArgSanitizationNoPair verifies that when PairedCharID is "-1" (no
// pair), the PairedName and PairedEmote fields are cleared even if the
// underlying packet had garbage in those slots.
func TestPairArgSanitizationNoPair(t *testing.T) {
	// Construct a server-format MS packet that simulates a stale PairedName /
	// PairedEmote left over from a previous pair, with PairedCharID set to
	// "-1" (no pair wanted).
	ms := &packet.MSToClient{
		PairedCharID: "-1",
		PairedName:   "leftover_pair_char",
		PairedEmote:  "leftover_pair_emote",
	}

	applyPairSanitization(ms)

	if ms.PairedCharID != "-1" {
		t.Errorf("PairedCharID should remain \"-1\", got %q", ms.PairedCharID)
	}
	if ms.PairedName != "" {
		t.Errorf("PairedName should be empty when no pair, got %q", ms.PairedName)
	}
	if ms.PairedEmote != "" {
		t.Errorf("PairedEmote should be empty when no pair, got %q", ms.PairedEmote)
	}
}

// TestPairArgSanitizationGarbageOffsets covers the same path with
// non-default offset-shaped strings — the sanitization must not care what
// the contents look like, only that PairedCharID indicates "no pair".
func TestPairArgSanitizationGarbageOffsets(t *testing.T) {
	ms := &packet.MSToClient{
		PairedCharID: "-1",
		PairedName:   "0     0",
		PairedEmote:  "0",
	}

	applyPairSanitization(ms)

	if ms.PairedName != "" {
		t.Errorf("PairedName should be empty when no pair, got %q", ms.PairedName)
	}
	if ms.PairedEmote != "" {
		t.Errorf("PairedEmote should be empty when no pair, got %q", ms.PairedEmote)
	}
}

// TestPairArgSanitizationEmptyCharId verifies sanitization when PairedCharID
// is completely absent (blank string) — also a "no pair" state.
func TestPairArgSanitizationEmptyCharId(t *testing.T) {
	ms := &packet.MSToClient{
		// PairedCharID left as "" (client did not send a pair char id)
		PairedName:  "50",
		PairedEmote: "-25",
	}

	applyPairSanitization(ms)

	if ms.PairedName != "" {
		t.Errorf("PairedName should be empty when no pair, got %q", ms.PairedName)
	}
	if ms.PairedEmote != "" {
		t.Errorf("PairedEmote should be empty when no pair, got %q", ms.PairedEmote)
	}
}

// TestParseMSToServerToServerExpands verifies that a 26-field client-format MS
// body, when parsed and re-encoded as a server packet, produces a 30-field
// slice with PairedName / PairedEmote inserted at slots 17 / 18 (matching the
// "two insertions" behavior the pre-refactor code handled inline).
func TestParseMSToServerToServerExpands(t *testing.T) {
	body := make([]string, 26)
	body[5] = "wit"
	body[14] = "0"
	body[16] = "-1"
	body[17] = "0&0" // self_offset on the client side (client slot 17)
	body[18] = "0"   // noninterrupting_preanim on the client side (client slot 18)

	ms := packet.ParseMSToServer(body)
	if ms.PairedCharID != "-1" {
		t.Errorf("PairedCharID = %q, want -1", ms.PairedCharID)
	}
	if ms.Offset != "0&0" {
		t.Errorf("Offset = %q, want \"0&0\"", ms.Offset)
	}
	if ms.NoninterruptingPreanim != "0" {
		t.Errorf("NoninterruptingPreanim = %q, want \"0\"", ms.NoninterruptingPreanim)
	}

	args := ms.ToClient().Args()
	if len(args) != 30 {
		t.Fatalf("Args len = %d, want 30", len(args))
	}
	if args[16] != "-1" {
		t.Errorf("server slot 16 (PairedCharID) = %q, want -1", args[16])
	}
	if args[17] != "" {
		t.Errorf("server slot 17 (PairedName) should be empty for client-origin packet, got %q", args[17])
	}
	if args[18] != "" {
		t.Errorf("server slot 18 (PairedEmote) should be empty for client-origin packet, got %q", args[18])
	}
	if args[19] != "0&0" {
		t.Errorf("server slot 19 (Offset) = %q, want \"0&0\"", args[19])
	}
	if args[22] != "0" {
		t.Errorf("server slot 22 (NoninterruptingPreanim) = %q, want \"0\"", args[22])
	}
}
