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
	"sync"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// PairGroup is a multi-pair group of any size (2+). It starts pending and grows
// incrementally as each member accepts (1 accept → pair, 2 → triple, …).
// Members reference it via Client.pairGroup; a nil pointer means "not in a
// group". members[0] is pre-accepted and front-most: for a consent group that
// is the initiator, for a forced group it is simply the first target. members
// order is the render z-order (members[0] front-most).
type PairGroup struct {
	mu       sync.Mutex
	members  []*Client
	accepted map[int]bool // by UID
	groupID  string       // stable id carried in the GP roster packet
	forced   bool         // true when created by /forcegrouppair (members had no say)
}

// roster returns a human-readable, comma-joined member name list.
func (g *PairGroup) roster() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	names := make([]string, len(g.members))
	for i, m := range g.members {
		names[i] = oocDisplayName(m)
	}
	return strings.Join(names, ", ")
}

// status returns a per-member accept status line.
func (g *PairGroup) status() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.statusLocked()
}

// statusLocked is status() without acquiring g.mu (caller must hold it).
func (g *PairGroup) statusLocked() string {
	parts := make([]string, len(g.members))
	for i, m := range g.members {
		if g.accepted[m.Uid()] {
			parts[i] = oocDisplayName(m) + " (accepted)"
		} else {
			parts[i] = oocDisplayName(m) + " (pending)"
		}
	}
	return strings.Join(parts, ", ")
}

// dissolve tears the group down, clearing every member's reference, notifying
// them with reason (empty reason = silent), and pushing an empty GP roster so
// JSON clients drop their on-screen group state.
func (g *PairGroup) dissolve(reason string) {
	g.mu.Lock()
	members := append([]*Client(nil), g.members...)
	groupID := g.groupID
	g.mu.Unlock()
	for _, m := range members {
		m.SetPairGroup(nil)
		if reason != "" {
			m.SendServerMessage(reason)
		}
		if m.supportsGroupPair() {
			m.Send(&GP{GroupID: groupID, Members: []GPMember{}})
		}
	}
}

// acceptedMembers returns the accepted members in group (z-)order.
func (g *PairGroup) acceptedMembers() []*Client {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]*Client, 0, len(g.members))
	for _, m := range g.members {
		if g.accepted[m.Uid()] {
			out = append(out, m)
		}
	}
	return out
}

// IsForced reports whether this group was created by a moderator via
// /forcegrouppair (members had no say), as opposed to a consent-based
// /grouppair.
func (g *PairGroup) IsForced() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.forced
}

// buildGP snapshots the accepted roster as a GP packet (ordered, speaker
// included) for the JSON-only group-pair extension.
func (g *PairGroup) buildGP() *GP {
	accepted := g.acceptedMembers()
	members := make([]GPMember, 0, len(accepted))
	for i, m := range accepted {
		pi := m.PairInfo()
		members = append(members, GPMember{
			UID:    m.Uid(),
			CharID: m.CharID(),
			Name:   pi.name,
			Emote:  pi.emote,
			Side:   m.Pos(),
			Offset: parseOffset(pi.offset),
			Flip:   parsePairFlip(pi.flip),
			Order:  i,
		})
	}
	return &GP{GroupID: g.groupID, Members: members}
}

// remove gracefully removes one member from the group. If fewer than two
// members remain the group dissolves; otherwise it persists and the remaining
// members receive an updated roster.
func (g *PairGroup) remove(member *Client, reason string) {
	g.mu.Lock()
	remaining := make([]*Client, 0, len(g.members)-1)
	for _, m := range g.members {
		if m != member {
			remaining = append(remaining, m)
		}
	}
	g.members = remaining
	delete(g.accepted, member.Uid())
	dissolve := len(remaining) < 2
	members := append([]*Client(nil), remaining...)
	g.mu.Unlock()

	member.SetPairGroup(nil)
	if reason != "" {
		member.SendServerMessage(reason)
	}

	if dissolve {
		for _, m := range members {
			m.SetPairGroup(nil)
			if reason != "" {
				m.SendServerMessage("The pairing group was dissolved.")
			}
			if m.supportsGroupPair() {
				m.Send(&GP{GroupID: g.groupID, Members: []GPMember{}})
			}
		}
	} else {
		for _, m := range members {
			if reason != "" {
				m.SendServerMessage(reason)
			}
		}
		sendGroupState(g)
	}

	if member.supportsGroupPair() {
		member.Send(&GP{GroupID: g.groupID, Members: []GPMember{}})
	}
}

// reorder moves an accepted member within the roster (front→back ordering,
// members[0] front-most). op is "front", "back", "up" (one step toward the
// front) or "down" (one step toward the back). It returns the previous and new
// indices within members and whether the order actually changed.
func (g *PairGroup) reorder(member *Client, op string) (from, to int, changed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	idx := -1
	for i, m := range g.members {
		if m == member {
			idx = i
			break
		}
	}
	if idx < 0 || !g.accepted[member.Uid()] || len(g.members) < 2 {
		return idx, idx, false
	}

	n := len(g.members)
	newIdx := idx
	switch op {
	case "front":
		newIdx = 0
	case "back":
		newIdx = n - 1
	case "up":
		if idx > 0 {
			newIdx = idx - 1
		}
	case "down":
		if idx < n-1 {
			newIdx = idx + 1
		}
	}
	if newIdx == idx {
		return idx, idx, false
	}

	m := g.members[idx]
	if newIdx < idx {
		copy(g.members[newIdx+1:idx+1], g.members[newIdx:idx])
	} else {
		copy(g.members[idx:newIdx], g.members[idx+1:newIdx+1])
	}
	g.members[newIdx] = m

	return idx, newIdx, true
}

// isFrontmost reports whether member is the front-most accepted member of the
// group (the one rendered in front of all others).
func (g *PairGroup) isFrontmost(member *Client) bool {
	accepted := g.acceptedMembers()
	return len(accepted) > 0 && accepted[0] == member
}

// accept marks a member as accepted. The group grows incrementally: each accept
// adds one more renderable member (1 accept → pair, 2 → triple, …).
func (g *PairGroup) accept(member *Client) {
	g.mu.Lock()
	if g.accepted[member.Uid()] {
		g.mu.Unlock()
		member.SendServerMessage("You have already accepted. Waiting for the other members.")
		return
	}
	g.accepted[member.Uid()] = true
	members := append([]*Client(nil), g.members...)
	g.mu.Unlock()

	for _, m := range members {
		m.SendServerMessage(oocDisplayName(member) + " accepted. Members: " + g.roster())
	}
	sendGroupState(g)
}

// cmdAccept handles /accept.
func cmdAccept(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You have no pending pairing group request.")
		return
	}
	g.accept(client)
}

// cmdDeny handles /deny.
func cmdDeny(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You have no pending pairing group request.")
		return
	}
	g.remove(client, oocDisplayName(client)+" declined — the pairing group has been updated.")
}

// cmdPairRequests handles /pair-requests.
func cmdPairRequests(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You are not in any pairing group.")
		return
	}
	client.SendServerMessage("Pairing group status: " + g.status())
}

// applyPairGroupInjection fills the MS pair slots for a speaker in a group. The
// standard paired_* fields carry the first accepted partner (FantaCode/legacy
// clients), and the JSON-only additional_chars list carries the FULL accepted
// roster in z-order — speaker included — so the ordering lives in one place.
func applyPairGroupInjection(client *Client, ms *MSToClient) {
	g := client.PairGroup()
	if g == nil {
		return
	}
	accepted := g.acceptedMembers()
	if len(accepted) < 2 {
		return
	}
	// The speaker must be an accepted member for their message to render the
	// group (a pending invitee has not joined yet).
	speakerAccepted := false
	for _, m := range accepted {
		if m == client {
			speakerAccepted = true
			break
		}
	}
	if !speakerAccepted {
		return
	}

	// First accepted partner other than the speaker fills the legacy pair slots.
	var first *Client
	for _, m := range accepted {
		if m != client {
			first = m
			break
		}
	}
	info := first.PairInfo()
	ms.PairedCharID = first.CharIDStr()
	ms.PairedName = info.name
	ms.PairedEmote = info.emote
	ms.PairedOffset = info.offset
	otherFlip, _ := strconv.Atoi(info.flip)
	ms.PairedFlip = packetutil.FlipFromWire[otherFlip]

	// JSON-only: the ordered roster (speaker included) is the z-order.
	for i, m := range accepted {
		pi := m.PairInfo()
		ms.AdditionalChars = append(ms.AdditionalChars, AdditionalChar{
			CharID: m.CharID(),
			Name:   pi.name,
			Emote:  pi.emote,
			Side:   m.Pos(),
			Offset: parseOffset(pi.offset),
			Flip:   parsePairFlip(pi.flip),
			Order:  i,
		})
	}
}

// parseOffset converts an "x" or "x&y" offset string into a Offset. The wire
// escapes "&" as "<and>", so unescape before splitting (the JSON-MS path does
// the same in parseMSOffset).
func parseOffset(s string) aolib.Offset {
	s = strings.ReplaceAll(s, "<and>", "&")
	if s == "" {
		return aolib.Offset{}
	}
	parts := strings.SplitN(s, "&", 2)
	x, _ := strconv.Atoi(parts[0])
	y := 0
	if len(parts) > 1 {
		y, _ = strconv.Atoi(parts[1])
	}
	return aolib.Offset{X: x, Y: y}
}

// parsePairFlip converts a flip string ("0".."3") into the Flip enum.
func parsePairFlip(s string) aolib.Flip {
	n, _ := strconv.Atoi(s)
	return packetutil.FlipFromWire[n]
}

// removeGroupMemberOnDisconnect gracefully removes a disconnecting client from
// their group (the group shrinks; it only dissolves when fewer than two members
// remain). Called from clearPairLinksOnDisconnect while the leaver's UID is
// still valid.
func removeGroupMemberOnDisconnect(client *Client) {
	if g := client.PairGroup(); g != nil {
		g.remove(client, oocDisplayName(client)+" disconnected — the pairing group has been updated.")
	}
}

// supportsGroupPair reports whether this JSON client may receive the group-pair
// extension (GP roster + additional_chars). GP is JSON-only, so JSON mode alone
// is sufficient: a client that never registers GP ignores the unknown header,
// and additional_chars lands in Extras. No client FL handshake is required —
// aolib models FL as server→client only, so there is no typed C2S FL for a
// client to send.
func (client *Client) supportsGroupPair() bool {
	return client.jsonMode.Load()
}
