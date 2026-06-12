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

// applyHideDisplay pushes the speaker's own sprite off-screen via SelfOffset.
func TestApplyHideDisplay_SetsOffscreenOffset(t *testing.T) {
	ms := &packet.MSPacket{SelfOffset: ""}
	punishments := []PunishmentState{
		{punishmentType: PunishmentHideDisplay},
	}
	applyHideDisplay(ms, punishments)
	want := encode(hideDisplayOffset)
	if ms.SelfOffset != want {
		t.Fatalf("expected SelfOffset=%q, got %q", want, ms.SelfOffset)
	}
}

// Without a HideDisplay punishment in the active set, the offset is untouched —
// so /hidedisplay never interferes with /shrink / /grow / /wide or normal play.
func TestApplyHideDisplay_NoOpWithoutPunishment(t *testing.T) {
	ms := &packet.MSPacket{SelfOffset: "X&Y"}
	applyHideDisplay(ms, []PunishmentState{
		{punishmentType: PunishmentWhisper},
		{punishmentType: PunishmentFancy},
	})
	if ms.SelfOffset != "X&Y" {
		t.Fatalf("expected SelfOffset to be untouched, got %q", ms.SelfOffset)
	}
}

// applyForceDisplaySprite overwrites the IC packet's character/sprite fields
// with the pinned target's, and clears the pair fields so only one character
// renders.
func TestApplyForceDisplaySprite_OverwritesAndClearsPair(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})

	target := &Client{
		uid:  9,
		char: 1, // Miles Edgeworth
		pair: ClientPairInfo{wanted_id: -1, emote: "smug", flip: "1", offset: "10&-5"},
		pos:  "pro",
	}

	ms := &packet.MSPacket{
		Character:   "Phoenix Wright",
		CharID:      "0",
		Emote:       "normal",
		Side:        "def",
		Flip:        "0",
		OtherCharID: "0",
		OtherName:   "somebody",
		OtherEmote:  "smile",
		OtherOffset: "0&0",
		OtherFlip:   "0",
	}
	applyForceDisplaySprite(ms, target)

	if ms.Character != "Miles Edgeworth" {
		t.Errorf("Character = %q, want Miles Edgeworth", ms.Character)
	}
	if ms.CharID != "1" {
		t.Errorf("CharID = %q, want 1", ms.CharID)
	}
	if ms.Emote != "smug" {
		t.Errorf("Emote = %q, want smug (from target PairInfo)", ms.Emote)
	}
	if ms.Side != "pro" {
		t.Errorf("Side = %q, want pro (from target Pos)", ms.Side)
	}
	if ms.Flip != "1" {
		t.Errorf("Flip = %q, want 1", ms.Flip)
	}
	if ms.OtherCharID != "-1" {
		t.Errorf("OtherCharID = %q, want -1 (pair must be cleared, no \"^\" suffix)", ms.OtherCharID)
	}
	if ms.OtherName != "" || ms.OtherEmote != "" || ms.OtherOffset != "" || ms.OtherFlip != "" {
		t.Errorf("expected all Other* fields cleared, got name=%q emote=%q offset=%q flip=%q",
			ms.OtherName, ms.OtherEmote, ms.OtherOffset, ms.OtherFlip)
	}
}

// A target with no character selected (CharID == -1) is silently ignored —
// applyForceDisplaySprite must not write garbage when bounds-checking fails.
func TestApplyForceDisplaySprite_SkipsSpectatorTarget(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright"})

	target := &Client{uid: 9, char: -1, pair: ClientPairInfo{wanted_id: -1}}
	ms := &packet.MSPacket{Character: "Original", CharID: "0", Emote: "kept"}
	applyForceDisplaySprite(ms, target)

	if ms.Character != "Original" || ms.CharID != "0" || ms.Emote != "kept" {
		t.Fatalf("packet should be untouched for spectator target, got %+v", ms)
	}
}
