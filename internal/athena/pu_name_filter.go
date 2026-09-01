// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Last-line filtering of names on the way out, at the PU packet itself.
//
// pktIC and pktOOC censor a showname and an OOC name when the player sets them,
// which catches the case that matters most and does not catch several others.
// PU is the packet that actually puts a name in front of other people, and it is
// sent from a dozen places, only two of which are those input paths:
//
//   - server.go replays EVERY connected player's stored OOC name and showname to
//     each newly joining client. A name set before a word was added to the list
//     is therefore re-broadcast, in full, to every person who joins from then on
//     -- the input check has long since happened and will never run again.
//   - /forcename sets a showname on someone else, and a moderator typing it is
//     not checked by the input path at all.
//   - /reversename, /nameshuffle and its restore re-emit stored names.
//   - ChangeArea re-broadcasts the showname on every area change.
//
// So this is defence in depth at the point of exposure rather than the point of
// entry, which is the one place a filter cannot be bypassed by a code path
// nobody thought about -- including code paths added later.
//
// A match DROPS THE PACKET ENTIRELY rather than substituting a placeholder: a
// name nobody can see is the point, and a placeholder still tells the room that
// somebody tried, which is a small reward for trying.
//
// A nuke-tier match also bans -- and bans the OWNER of the name, resolved from
// the PU's own ID field, never the client the packet was about to be sent to.
// That distinction is the whole reason this can punish safely. The join replay
// carries other people's names to whoever is connecting, so acting on the
// recipient would ban an arbitrary bystander and would re-fire on every single
// join for as long as the name existed. Acting on the owner is correct from
// every one of the call sites above, and self-limiting: the first hit closes
// that connection, so the replay stops happening.
//
// Two carve-outs. Moderators are never banned by this path, matching every
// other automatic ban on the server. And /forcename is checked at the command
// itself (see cmdForceName) rather than here, because a moderator typing a
// slur into somebody else's showname is the moderator's doing and banning the
// target for it would be indefensible -- the packet is still dropped here if it
// somehow reaches this point.
//
// Cost: PU carries a name on a name change, an area change and a join -- never
// per message. broadcastToAll is not on the IC path (that is broadcastToArea),
// so nothing here is reachable from the per-message hot path at all.
package athena

import (
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// PU packet types that carry a player-chosen name. Type 1 is the character
// name (drawn from characters.txt, which the operator controls) and type 3 is
// an area index, so neither is player-supplied text and neither is filtered.
const (
	puTypeOOCName  = 0
	puTypeShowname = 2
)

// nameAllowedInPU reports whether a PU may be sent, and bans the name's owner
// when the entry that matched is nuke-tier.
//
// Returns true for everything that is not a name-carrying PU, so the check is
// two comparisons for the ARUP/character/area traffic that shares this path.
func nameAllowedInPU(pu *packet.PU) bool {
	if pu == nil || (pu.Type != puTypeOOCName && pu.Type != puTypeShowname) || pu.Data == "" {
		return true
	}
	entries := effectiveWordEntries()
	if len(entries) == 0 {
		return true
	}
	m := matchWordEntries(entries, pu.Data)
	if !m.Matched {
		return true
	}

	field := "showname"
	if pu.Type == puTypeOOCName {
		field = "OOC name"
	}
	logger.LogInfof("name filter: dropped a %s broadcast for uid %d — matched %s", field, pu.ID, m.Entry.String())

	if m.Entry.Severity == SeverityNuke {
		// Resolved from the packet's own ID: the owner of the name, not the
		// client this packet was headed for. Fired on its own goroutine so a
		// broadcast fan-out never waits on a database write and a socket close,
		// and so this cannot re-enter the client list from inside a caller that
		// is already walking it.
		go banPUNameOwner(pu.ID, m, field)
	}
	return false
}

// banPUNameOwner nukes the player a filtered name belongs to.
func banPUNameOwner(uid int, m WordListMatch, field string) {
	owner, err := getClientByUid(uid)
	if err != nil || owner == nil {
		// The name outlived its owner's connection -- the drop above already did
		// the part that matters.
		return
	}
	if permissions.IsModerator(owner.Perms()) {
		logger.LogWarningf("name filter: uid %d matched %s in their %s but is a moderator; not banned", uid, m.Entry.String(), field)
		return
	}
	applyAutoModNuke(owner, m, "PU "+field)
}

// puAllowed is the choke point callers use: it reports whether an outgoing
// packet may be sent, filtering name-carrying PUs and passing everything else
// through.
//
// One type assertion on a path that is already fanning a packet out to every
// connected client; it is not measurable against the serialization and socket
// writes that follow it, and broadcastToArea -- the per-message IC path -- does
// not go through here at all.
func puAllowed(p packet.Outgoing) bool {
	if pu, ok := p.(*packet.PU); ok {
		return nameAllowedInPU(pu)
	}
	return true
}
