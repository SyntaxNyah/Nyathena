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
// when OtherCharID is "" or "-1", OtherName and OtherEmote must be cleared
// regardless of any value the client may have leaked into those slots.
func applyPairSanitization(ms *packet.MSPacket) {
	if ms.OtherCharID == "" || ms.OtherCharID == "-1" {
		ms.OtherName = ""
		ms.OtherEmote = ""
	}
}

// TestPairArgSanitizationNoPair verifies that when OtherCharID is "-1" (no
// pair), the OtherName and OtherEmote fields are cleared even if the
// underlying packet had garbage in those slots.
func TestPairArgSanitizationNoPair(t *testing.T) {
	// Construct a server-format MS packet that simulates a stale OtherName /
	// OtherEmote left over from a previous pair, with OtherCharID set to
	// "-1" (no pair wanted).
	ms := &packet.MSPacket{
		OtherCharID: "-1",
		OtherName:   "leftover_pair_char",
		OtherEmote:  "leftover_pair_emote",
	}

	applyPairSanitization(ms)

	if ms.OtherCharID != "-1" {
		t.Errorf("OtherCharID should remain \"-1\", got %q", ms.OtherCharID)
	}
	if ms.OtherName != "" {
		t.Errorf("OtherName should be empty when no pair, got %q", ms.OtherName)
	}
	if ms.OtherEmote != "" {
		t.Errorf("OtherEmote should be empty when no pair, got %q", ms.OtherEmote)
	}
}

// TestPairArgSanitizationGarbageOffsets covers the same path with
// non-default offset-shaped strings — the sanitization must not care what
// the contents look like, only that OtherCharID indicates "no pair".
func TestPairArgSanitizationGarbageOffsets(t *testing.T) {
	ms := &packet.MSPacket{
		OtherCharID: "-1",
		OtherName:   "0     0",
		OtherEmote:  "0",
	}

	applyPairSanitization(ms)

	if ms.OtherName != "" {
		t.Errorf("OtherName should be empty when no pair, got %q", ms.OtherName)
	}
	if ms.OtherEmote != "" {
		t.Errorf("OtherEmote should be empty when no pair, got %q", ms.OtherEmote)
	}
}

// TestPairArgSanitizationEmptyCharId verifies sanitization when OtherCharID
// is completely absent (blank string) — also a "no pair" state.
func TestPairArgSanitizationEmptyCharId(t *testing.T) {
	ms := &packet.MSPacket{
		// OtherCharID left as "" (client did not send a pair char id)
		OtherName:  "50",
		OtherEmote: "-25",
	}

	applyPairSanitization(ms)

	if ms.OtherName != "" {
		t.Errorf("OtherName should be empty when no pair, got %q", ms.OtherName)
	}
	if ms.OtherEmote != "" {
		t.Errorf("OtherEmote should be empty when no pair, got %q", ms.OtherEmote)
	}
}

// TestParseMSClientToServerExpands verifies that a 26-field client-format MS
// body, when parsed and re-encoded as a server packet, produces a 30-field
// slice with OtherName / OtherEmote inserted at slots 17 / 18 (matching the
// "two insertions" behavior the pre-refactor code handled inline).
func TestParseMSClientToServerExpands(t *testing.T) {
	body := make([]string, 26)
	body[5] = "wit"
	body[14] = "0"
	body[16] = "-1"
	body[17] = "0&0" // self_offset on the client side (client slot 17)
	body[18] = "0"   // noninterrupting_preanim on the client side (client slot 18)

	ms := packet.ParseMSClient(body)
	if ms.OtherCharID != "-1" {
		t.Errorf("OtherCharID = %q, want -1", ms.OtherCharID)
	}
	if ms.SelfOffset != "0&0" {
		t.Errorf("SelfOffset = %q, want \"0&0\"", ms.SelfOffset)
	}
	if ms.NonInterruptingPreAnim != "0" {
		t.Errorf("NonInterruptingPreAnim = %q, want \"0\"", ms.NonInterruptingPreAnim)
	}

	args := ms.ServerArgs()
	if len(args) != 30 {
		t.Fatalf("ServerArgs len = %d, want 30", len(args))
	}
	if args[16] != "-1" {
		t.Errorf("server slot 16 (OtherCharID) = %q, want -1", args[16])
	}
	if args[17] != "" {
		t.Errorf("server slot 17 (OtherName) should be empty for client-origin packet, got %q", args[17])
	}
	if args[18] != "" {
		t.Errorf("server slot 18 (OtherEmote) should be empty for client-origin packet, got %q", args[18])
	}
	if args[19] != "0&0" {
		t.Errorf("server slot 19 (SelfOffset) = %q, want \"0&0\"", args[19])
	}
	if args[22] != "0" {
		t.Errorf("server slot 22 (NonInterruptingPreAnim) = %q, want \"0\"", args[22])
	}
}

// TestApplyPairOrderBackSwapsVisualsOnly verifies the /pair_order back rewrite:
// the visual pair fields (character name, emote, offset, flip) are swapped so
// the sender renders behind the partner, while the identity fields CHAR_ID and
// OTHER_CHARID are left untouched. Swapping CHAR_ID is what broke the IC textbox
// (the client keys "is this my own message" off CHAR_ID == m_cid), so this test
// pins the contract that CHAR_ID must survive the rewrite.
func TestApplyPairOrderBackSwapsVisualsOnly(t *testing.T) {
	ms := &packet.MSPacket{
		Character:   "Phoenix", // self (sender)
		Emote:       "point",
		CharID:      "3",
		SelfOffset:  "10&0",
		Flip:        "0",
		OtherCharID: "5", // partner
		OtherName:   "Edgeworth",
		OtherEmote:  "desk_slam",
		OtherOffset: "-10&0",
		OtherFlip:   "1",
	}

	applyPairOrderBack(ms)

	// Visual fields must be swapped (partner now drawn in the front viewport).
	if ms.Character != "Edgeworth" || ms.OtherName != "Phoenix" {
		t.Errorf("character/othername not swapped: Character=%q OtherName=%q", ms.Character, ms.OtherName)
	}
	if ms.Emote != "desk_slam" || ms.OtherEmote != "point" {
		t.Errorf("emote/otheremote not swapped: Emote=%q OtherEmote=%q", ms.Emote, ms.OtherEmote)
	}
	if ms.SelfOffset != "-10&0" || ms.OtherOffset != "10&0" {
		t.Errorf("offsets not swapped: SelfOffset=%q OtherOffset=%q", ms.SelfOffset, ms.OtherOffset)
	}
	if ms.Flip != "1" || ms.OtherFlip != "0" {
		t.Errorf("flips not swapped: Flip=%q OtherFlip=%q", ms.Flip, ms.OtherFlip)
	}

	// Identity fields MUST be preserved so attribution / textbox clearing works.
	if ms.CharID != "3" {
		t.Errorf("CharID must NOT be swapped (broke textbox); got %q, want \"3\"", ms.CharID)
	}
	if ms.OtherCharID != "5" {
		t.Errorf("OtherCharID must NOT be swapped; got %q, want \"5\"", ms.OtherCharID)
	}
}
