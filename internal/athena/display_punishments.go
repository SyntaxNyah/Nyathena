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
	"strings"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Display punishments operate on the IC packet's sprite fields rather than its
// message text. They are applied in pktIC and persist across sessions exactly
// like the text punishments (DB-backed, /unpunish-able, support -d/-r/-h, comma
// UID lists, the `global` keyword and /stack).
//
//   - /hidedisplay <uid>  pushes the punished speaker's OWN sprite off-screen so
//     their text still appears but their character does not. Applied to the
//     speaker's own outgoing IC packet (see applyHideDisplay).
//   - /forcedisplay <uid> pins the punished player's character onto every
//     non-moderator IC message in their area, so while it's active the whole
//     room renders as that one character and no other sprite can show.

// hideDisplayOffset is the self-offset ("x&y", AO2 2.8+ percentage form) used by
// /hidedisplay. The viewport is at most one screen tall/wide, so shifting a
// sprite a full +100% on both axes moves it entirely off the bottom-right
// corner. The dialogue box and showname are separate UI and stay visible. Both
// components are within the [-100,100] range the IC offset validator enforces.
const hideDisplayOffset = "100&100"

// activeForceDisplay counts how many currently-connected clients carry an
// in-memory /forcedisplay punishment. It is a cheap gate: the per-IC resolver
// only scans the area for a pinned target when this is > 0, so servers that
// never use /forcedisplay pay nothing on the hot path. It is kept exact —
// incremented when a forcedisplay is newly added (fresh application or DB
// restore on reconnect) and decremented on the single path that removes it
// (/unpunish, /unpunish all, expiry sweep, or disconnect). Correctness never
// depends on the count being precise: the resolver re-verifies every candidate
// with HasActivePunishment, so a stale-high value only costs a wasted scan.
var activeForceDisplay atomic.Int32

// HasActivePunishment reports whether the client currently carries a punishment
// of the given type that has not expired. Unlike HasPunishment it honours the
// expiry time, so an effect whose timer elapsed — but whose slice entry hasn't
// been lazily swept yet — is correctly treated as inactive. Used by the
// /forcedisplay resolver, which is driven by other players' IC messages and so
// can't rely on the punished player's own lazy expiry sweep having run.
func (client *Client) HasActivePunishment(pType PunishmentType) bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	now := time.Now().UTC()
	for i := range client.punishments {
		p := &client.punishments[i]
		if p.punishmentType == pType && (p.expiresAt.IsZero() || now.Before(p.expiresAt)) {
			return true
		}
	}
	return false
}

// releaseForceDisplayGate decrements the activeForceDisplay gate if this client
// still holds a forcedisplay punishment in memory. Called from the disconnect
// cleanup so a pinned player leaving the server lowers the gate. It is balanced
// against the increment in AddPunishmentBy: whichever of /unpunish, expiry or
// disconnect removes the instance first does the single decrement (the others
// then find it absent and do nothing).
func (client *Client) releaseForceDisplayGate() {
	client.mu.Lock()
	has := false
	for i := range client.punishments {
		if client.punishments[i].punishmentType == PunishmentForceDisplay {
			has = true
			break
		}
	}
	client.mu.Unlock()
	if has {
		activeForceDisplay.Add(-1)
	}
}

// applyHideDisplay pushes the speaker's own sprite off-screen when they carry an
// active /hidedisplay punishment. It is applied before the IC pair-info snapshot
// is taken, so the off-screen offset also propagates to anyone paired with the
// hidden player — they stay hidden in the partner's viewport too. punishments is
// the speaker's already-filtered active set from pktIC, so no extra lock is
// taken here.
func applyHideDisplay(ms *packet.MSPacket, punishments []PunishmentState) {
	for i := range punishments {
		if punishments[i].punishmentType == PunishmentHideDisplay {
			ms.SelfOffset = encode(hideDisplayOffset)
			return
		}
	}
}

// maybeApplyForceDisplay rewrites a non-moderator speaker's outgoing IC sprite
// to the pinned forcedisplay target's character, suppressing any pairing so only
// that one character renders for the whole room. Moderators are exempt (their
// own sprite still shows), matching how `global` punishments spare staff. The
// activeForceDisplay gate keeps this free when the feature is unused.
func maybeApplyForceDisplay(client *Client, ms *packet.MSPacket) {
	if activeForceDisplay.Load() <= 0 {
		return
	}
	if permissions.IsModerator(client.Perms()) {
		return
	}
	target := findActiveForceDisplayTarget(client.Area())
	if target == nil {
		return
	}
	applyForceDisplaySprite(ms, target)
}

// findActiveForceDisplayTarget returns a client in the given area that carries an
// active /forcedisplay punishment, or nil if none. When several players are
// pinned in one area (e.g. via `global`) the first one found wins — the mental
// model is a single character hogging the viewport.
func findActiveForceDisplayTarget(a *area.Area) *Client {
	var found *Client
	clients.ForEach(func(c *Client) {
		if found != nil {
			return
		}
		if c.Area() == a && c.HasActivePunishment(PunishmentForceDisplay) {
			found = c
		}
	})
	return found
}

// applyForceDisplaySprite stamps the target's character/emote/position onto the
// outgoing IC packet and clears the pair fields. The speaker's message text,
// showname and colour are left untouched, so the room sees the pinned character
// "speaking" everyone's lines. Mirrors the fullpossess sprite-spoof logic.
func applyForceDisplaySprite(ms *packet.MSPacket, target *Client) {
	chars := getCharacters()
	id := target.CharID()
	if id < 0 || id >= len(chars) {
		return // target is spectating or has an out-of-range char; nothing to pin
	}
	info := target.PairInfo()
	charName := info.name
	if strings.TrimSpace(charName) == "" {
		charName = chars[id]
	}
	// Resolve the slot for the displayed character name (handles iniswap names),
	// falling back to the target's actual slot when the name isn't in the list.
	cid := getCharacterID(charName)
	if cid < 0 {
		cid = id
		charName = chars[id]
	}
	emote := info.emote
	if emote == "" {
		emote = "normal"
	}
	ms.Character = charName
	ms.CharID = strconv.Itoa(cid)
	ms.Emote = emote
	if info.flip == "0" || info.flip == "1" {
		ms.Flip = info.flip
	}
	if pos := target.Pos(); pos != "" {
		ms.Side = pos
	}
	if info.offset != "" {
		ms.SelfOffset = info.offset
	}
	// Suppress any pairing so only the pinned character renders in the viewport.
	// Plain "-1": the "^" suffix breaks clients that can't parse pair order
	// (message dropped on some desktop forks, NaN pair id on webAO).
	ms.OtherCharID = "-1"
	ms.OtherName = ""
	ms.OtherEmote = ""
	ms.OtherOffset = ""
	ms.OtherFlip = ""
}

// cmdHideDisplay (/hidedisplay) hides the target's own sprite from the IC
// viewport while leaving their text visible. Standard punishment command:
// requires MUTE, supports -d/-r/-h, comma-separated UIDs and `global`.
func cmdHideDisplay(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHideDisplay)
}

// cmdForceDisplay (/forcedisplay) pins the target's character onto every
// non-moderator IC message in their area, suppressing all other sprites.
// Standard punishment command: requires MUTE, supports -d/-r/-h, comma-separated
// UIDs and `global`.
func cmdForceDisplay(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentForceDisplay)
}
