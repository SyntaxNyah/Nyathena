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
	"strconv"
	"sync/atomic"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// Possession lets a moderator speak through another player's character. There
// are three flavours, all sharing the same sprite-spoof and pair-spoof plumbing:
//
//   - /possess <uid> <msg>  one-shot: a single message rendered as the target.
//   - /fullpossess <uid>    persistent: every one of the possessor's IC messages
//                           is rendered as the target until /unpossess (ADMIN).
//   - /truepossess <uid>    persistent AND silences the target: their own IC and
//                           OOC are suppressed (echoed only back to them) and
//                           their showname / OOC name are frozen, so they cannot
//                           contest or expose the possession (SHADOW/ADMIN).
//
// The possessor's appearance transform lives in pktIC; this file holds the
// shared pair-spoof helper plus all of the /truepossess silencing state.

// activeTruePossess counts how many currently-connected clients are being
// silenced by an in-memory /truepossess. It is a cheap hot-path gate: pktIC and
// pktOOC only consult a client's per-client TruePossessed() flag when this is
// > 0, so a server that never uses /truepossess pays a single atomic load per
// IC/OOC packet and nothing more. It is kept exact — incremented on the
// false→true transition in SetTruePossessed and decremented on true→false
// (which covers /unpossess, switching target, and either party disconnecting).
var activeTruePossess atomic.Int32

// TruePossessed reports whether this client's own IC/OOC is currently being
// silenced by an active /truepossess.
func (client *Client) TruePossessed() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.trueMuted
}

// TruePossessedBy returns the UID of the possessor who silenced this client via
// /truepossess, or -1 if the client is not currently true-possessed. (The raw
// backing field is only meaningful while trueMuted is set, so the zero value can
// never be mistaken for "possessed by UID 0".)
func (client *Client) TruePossessedBy() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	if !client.trueMuted {
		return -1
	}
	return client.truePossessedBy
}

// SetTruePossessed marks (muted=true) or clears (muted=false) the silent IC/OOC
// mute that /truepossess applies to a target. byUID is the possessor's UID and
// is recorded only when muting. The activeTruePossess gate is adjusted on the
// actual state transition so it stays exact and balanced regardless of how many
// times this is called (idempotent re-sets do not move the counter).
func (client *Client) SetTruePossessed(muted bool, byUID int) {
	client.mu.Lock()
	was := client.trueMuted
	client.trueMuted = muted
	if muted {
		client.truePossessedBy = byUID
	} else {
		client.truePossessedBy = -1
	}
	client.mu.Unlock()

	switch {
	case muted && !was:
		activeTruePossess.Add(1)
	case !muted && was:
		activeTruePossess.Add(-1)
	}
}

// endTruePossession lifts the /truepossess silence from whatever player the
// given possessor is currently possessing, but only if this possessor is the one
// who set it. It is safe to call for a plain /fullpossess (the target won't be
// true-muted, so it no-ops) and when the possessor isn't possessing anyone. Used
// whenever a possession ends, switches target, or the possessor disconnects, so
// a target can never be left permanently muted.
func endTruePossession(possessor *Client) {
	pid := possessor.Possessing()
	if pid < 0 {
		return
	}
	target, err := getClientByUid(pid)
	if err != nil {
		return
	}
	if target.TruePossessedBy() == possessor.Uid() {
		target.SetTruePossessed(false, -1)
	}
}

// applyPossessedPairFields rewrites the pair-related fields of a possessed IC
// packet so the spoofed message shows the *target's* partner exactly as one of
// the target's own messages would. Possession replaces the speaker's whole
// appearance with the target's, but the pair fields are otherwise resolved from
// the possessor's state — so possessing a paired player would drop their
// partner's sprite from the viewport, an obvious "this is a possess" tell. This
// mirrors the in-line pairing resolution in pktIC, keyed off the target instead
// of the possessor. Used by both /fullpossess and /truepossess.
func applyPossessedPairFields(ms *packet.MSPacket, target *Client) {
	// A UID-locked force pair is resolved directly from the partner's UID. We do
	// NOT require the partner's PairWantedID to match the target's char here: the
	// in-line pairing code keeps those wanted-ids in sync inside the speaker's own
	// pktIC pass (which we skip during possession, since the *possessor* is the
	// one speaking), so relying on that sync would drop the pair. The mutual
	// ForcePairUID link is sufficient and authoritative.
	if forceUID := target.ForcePairUID(); forceUID >= 0 {
		partner, err := getClientByUid(forceUID)
		if err == nil && partner.ForcePairUID() == target.Uid() &&
			partner.CharID() >= 0 && partner.CharID() < len(getCharacters()) {
			ms.OtherCharID = strconv.Itoa(partner.CharID())
			info := partner.PairInfo()
			ms.OtherName = info.name
			ms.OtherEmote = info.emote
			ms.OtherOffset = info.offset
			ms.OtherFlip = info.flip
			return
		}
		// Force-pair partner missing or not mutual: render the target solo.
		clearPairFields(ms)
		return
	}

	// Normal /pair: the partner is whoever is mutually paired (each wants the
	// other's char) and standing at the target's position — exactly the in-line
	// rule, keyed off the target instead of the possessor.
	wantedID := target.PairWantedID()
	if wantedID < 0 || wantedID >= len(getCharacters()) || wantedID == target.CharID() {
		// No (valid) pair: clear every pair field so neither the target's stale
		// pair nor the possessor's leaks through.
		clearPairFields(ms)
		return
	}

	ms.OtherCharID = strconv.Itoa(wantedID)
	matched := false
	clients.ForEach(func(c *Client) {
		if matched {
			return
		}
		// A candidate UID-committed to someone else is never a /pair match.
		if c.ForcePairUID() >= 0 && c.ForcePairUID() != target.Uid() {
			return
		}
		if c.CharID() == wantedID && c.PairWantedID() == target.CharID() && c.Pos() == target.Pos() {
			info := c.PairInfo()
			ms.OtherName = info.name
			ms.OtherEmote = info.emote
			ms.OtherOffset = info.offset
			ms.OtherFlip = info.flip
			matched = true
		}
	})
	if !matched {
		// The partner isn't present / not mutually paired right now: show the
		// target solo, exactly as their own message would. Plain "-1" (never the
		// "^"-suffixed form, which some clients drop or parse as NaN).
		clearPairFields(ms)
	}
}

// clearPairFields blanks every pair slot on an outgoing IC packet, leaving the
// speaker rendered solo. Uses plain "-1" for OtherCharID (the "^" pair-order
// suffix breaks clients that can't parse it).
func clearPairFields(ms *packet.MSPacket) {
	ms.OtherCharID = "-1"
	ms.OtherName = ""
	ms.OtherEmote = ""
	ms.OtherOffset = ""
	ms.OtherFlip = ""
}
