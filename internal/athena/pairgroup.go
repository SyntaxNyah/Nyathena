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
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// PairGroup is a multi-pair group (3-5 characters). It starts pending and
// becomes active once every member has accepted. Members reference it via
// Client.pairGroup; a nil pointer means "not in a group". members[0] is the
// initiator and is pre-accepted.
type PairGroup struct {
	mu       sync.Mutex
	members  []*Client
	accepted map[int]bool // by UID
}

// groupName returns the human command/type name for a target group size.
func groupName(size int) string {
	switch size {
	case 3:
		return "triple"
	case 4:
		return "quad"
	case 5:
		return "quint"
	default:
		return "pair"
	}
}

func (g *PairGroup) active() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, m := range g.members {
		if !g.accepted[m.Uid()] {
			return false
		}
	}
	return true
}

// others returns the members other than the speaker, in group order.
func (g *PairGroup) others(speaker *Client) []*Client {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]*Client, 0, len(g.members)-1)
	for _, m := range g.members {
		if m != speaker {
			out = append(out, m)
		}
	}
	return out
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

// dissolve tears the group down, clearing every member's reference and
// notifying them with reason (empty reason = silent).
func (g *PairGroup) dissolve(reason string) {
	g.mu.Lock()
	members := append([]*Client(nil), g.members...)
	g.mu.Unlock()
	for _, m := range members {
		m.SetPairGroup(nil)
		if reason != "" {
			m.SendServerMessage(reason)
		}
	}
}

func (g *PairGroup) accept(member *Client) {
	g.mu.Lock()
	if g.accepted[member.Uid()] {
		g.mu.Unlock()
		member.SendServerMessage("You have already accepted. Waiting for the other members.")
		return
	}
	g.accepted[member.Uid()] = true
	done := true
	for _, m := range g.members {
		if !g.accepted[m.Uid()] {
			done = false
		}
	}
	members := append([]*Client(nil), g.members...)
	status := g.statusLocked()
	g.mu.Unlock()

	if done {
		roster := g.roster()
		for _, m := range members {
			m.SendServerMessage("Multi-pair formed! Members: " + roster + ".")
		}
		return
	}
	for _, m := range members {
		m.SendServerMessage(oocDisplayName(member) + " accepted. Status: " + status)
	}
}

// cmdStartGroup implements /triple, /quad and /quint: the initiator invites
// size-1 other players into a pending group.
func cmdStartGroup(client *Client, args []string, size int) {
	if client.CharID() < 0 {
		client.SendServerMessage("You have not selected a character.")
		return
	}
	if client.PairGroup() != nil {
		client.SendServerMessage("You are already in a pairing group.")
		return
	}
	if len(args) < size-1 {
		client.SendServerMessage(fmt.Sprintf("Usage: /%s <uid> ... (%d players total).", groupName(size), size))
		return
	}

	invitees := make([]*Client, 0, size-1)
	for i := 0; i < size-1; i++ {
		uid, err := strconv.Atoi(args[i])
		if err != nil {
			client.SendServerMessage("Invalid UID: " + args[i])
			return
		}
		t, err := getClientByUid(uid)
		if err != nil {
			client.SendServerMessage(fmt.Sprintf("Client with UID %d does not exist.", uid))
			return
		}
		if t == client {
			client.SendServerMessage("You cannot pair with yourself.")
			return
		}
		if t.Area() != client.Area() {
			client.SendServerMessage("That player is not in your area.")
			return
		}
		if t.CharID() < 0 {
			client.SendServerMessage("That player has not selected a character.")
			return
		}
		if t.PairGroup() != nil {
			client.SendServerMessage(oocDisplayName(t) + " is already in a pairing group.")
			return
		}
		for _, e := range invitees {
			if e == t {
				client.SendServerMessage("Duplicate player in request.")
				return
			}
		}
		invitees = append(invitees, t)
	}

	g := &PairGroup{
		members:  append([]*Client{client}, invitees...),
		accepted: map[int]bool{client.Uid(): true},
	}
	for _, m := range g.members {
		m.SetPairGroup(g)
	}

	client.SendServerMessage(fmt.Sprintf("Sent a %d-way pairing request. Waiting for acceptance.", size))
	for _, t := range invitees {
		t.SendServerMessage(fmt.Sprintf("%v wants to form a %d-way pairing with you. Members: %s. Type /accept to accept or /deny to decline.", oocDisplayName(client), size, g.roster()))
	}
}

func cmdTriple(client *Client, args []string, usage string) { cmdStartGroup(client, args, 3) }
func cmdQuad(client *Client, args []string, usage string)   { cmdStartGroup(client, args, 4) }
func cmdQuint(client *Client, args []string, usage string)  { cmdStartGroup(client, args, 5) }

func cmdAccept(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You have no pending pairing group request.")
		return
	}
	g.accept(client)
}

func cmdDeny(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You have no pending pairing group request.")
		return
	}
	g.dissolve(oocDisplayName(client) + " declined — the pairing group was dissolved.")
}

func cmdPairRequests(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You are not in any pairing group.")
		return
	}
	client.SendServerMessage("Pairing group status: " + g.status())
}

// applyPairGroupInjection fills the MS pair + multi-pair slots for a speaker in
// an active group: the standard paired_* fields from the first partner (so
// FantaCode/legacy clients render a pair) and the JSON-only additional_chars
// list from the remaining partners.
func applyPairGroupInjection(client *Client, ms *packet.MSPacket) {
	g := client.PairGroup()
	if g == nil || !g.active() {
		return
	}
	others := g.others(client)
	if len(others) == 0 {
		return
	}

	first := others[0]
	info := first.PairInfo()
	ms.OtherCharID = first.CharIDStr()
	ms.OtherName = info.name
	ms.OtherEmote = info.emote
	ms.OtherOffset = info.offset
	ms.OtherFlip = info.flip

	for _, p := range others[1:] {
		pi := p.PairInfo()
		ms.AdditionalChars = append(ms.AdditionalChars, packet.AdditionalChar{
			CharID: p.CharID(),
			Name:   pi.name,
			Emote:  pi.emote,
			Offset: parsePairOffset(pi.offset),
			Flip:   parsePairFlip(pi.flip),
		})
	}
}

// parsePairOffset converts an "x" or "x&y" offset string into a PairOffset.
func parsePairOffset(s string) packet.PairOffset {
	if s == "" {
		return packet.PairOffset{}
	}
	parts := strings.SplitN(s, "&", 2)
	x, _ := strconv.Atoi(parts[0])
	y := 0
	if len(parts) > 1 {
		y, _ = strconv.Atoi(parts[1])
	}
	return packet.PairOffset{X: x, Y: y}
}

// parsePairFlip converts a flip string ("0".."3") into an int.
func parsePairFlip(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// dissolvePairGroupOnDisconnect tears down any group a disconnecting client
// belongs to. Called from clearPairLinksOnDisconnect while the leaver's UID is
// still valid.
func dissolvePairGroupOnDisconnect(client *Client) {
	if g := client.PairGroup(); g != nil {
		g.dissolve(oocDisplayName(client) + " disconnected — the pairing group was dissolved.")
	}
}

// supportsMultiPair reports whether this JSON client may receive the
// additional_chars extension. Gated on the client's own FL advertisement: the
// client sends its supported features (client→server FL), and the server
// honors "multi_pair". Symmetric capability negotiation — no hardcoded client
// list.
func (client *Client) supportsMultiPair() bool {
	if !client.jsonMode.Load() {
		return false
	}
	return client.SupportsFeature("multi_pair")
}
