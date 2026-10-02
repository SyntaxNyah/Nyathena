package athena

import (
	"encoding/json"
	"fmt"
	"strconv"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// GPMember is one ordered member of a group roster. Order is the z-order:
// 0 front-most, increasing toward the back. Every accepted member (speaker
// included) appears here, so the list position is the single source of truth
// for on-screen ordering.
type GPMember struct {
	UID    int          `json:"uid"`
	CharID int          `json:"char_id"`
	Name   string       `json:"name"`
	Emote  string       `json:"emote"`
	Side   string       `json:"side"`
	Offset aolib.Offset `json:"offset"`
	Flip   aolib.Flip   `json:"flip"`
	Order  int          `json:"order"`
}

// GP is the server→client group-pair roster announcement. JSON-only custom
// packet (header "GP", registered via packetutil.RegisterLocal); it carries the
// full ordered roster as an idempotent snapshot so clients just replace their
// group state on every change.
type GP struct {
	GroupID string     `json:"group_id"`
	Members []GPMember `json:"members"`
}

// Header returns the wire header "GP".
func (p *GP) Header() string { return "GP" }

// Args is unused: GP is JSON-only and never encoded as FantaCode.
func (p *GP) Args() []string { return nil }

// gpCodec registers GP's JSON-only outbound form. There is no FantaCode form —
// group pairing requires JSON mode (supportsGroupPair gates on jsonMode), so
// the EncodeFanta closure is intentionally left nil.
func gpCodec() packetutil.Codec {
	return packetutil.Codec{
		EncodeJSON: func(p any) ([]byte, error) {
			return json.Marshal(p.(*GP))
		},
	}
}

// sendGroupState pushes the group's current roster to every accepted member
// that negotiated the grouppair feature.
func sendGroupState(g *PairGroup) {
	p := g.buildGP()
	for _, m := range g.acceptedMembers() {
		if m.supportsGroupPair() {
			m.Send(p)
		}
	}
}

// startGroupPair implements /grouppair and /forcegrouppair: the initiator
// invites any number of other players (player UIDs) into a group. force skips
// the accept flow and marks everyone accepted immediately.
func startGroupPair(client *Client, args []string, force bool) {
	if client.CharID() < 0 {
		client.SendServerMessage("You have not selected a character.")
		return
	}
	if client.PairGroup() != nil {
		client.SendServerMessage("You are already in a pairing group.")
		return
	}
	if len(args) == 0 {
		client.SendServerMessage("Usage: /grouppair <uid> [uid ...]")
		return
	}

	invitees := make([]*Client, 0, len(args))
	for _, arg := range args {
		uid, err := strconv.Atoi(arg)
		if err != nil {
			client.SendServerMessage("Invalid UID: " + arg)
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
		groupID:  strconv.Itoa(client.Uid()),
	}
	for _, m := range g.members {
		m.SetPairGroup(g)
	}

	if force {
		g.mu.Lock()
		for _, t := range invitees {
			g.accepted[t.Uid()] = true
		}
		g.mu.Unlock()
		client.SendServerMessage(fmt.Sprintf("Force-formed a %d-player pairing group.", len(g.members)))
		for _, t := range invitees {
			t.SendServerMessage(fmt.Sprintf("%v force-paired you into a group. Members: %s.", oocDisplayName(client), g.roster()))
		}
		sendGroupState(g)
		return
	}

	client.SendServerMessage(fmt.Sprintf("Sent a group pairing request to %d player(s).", len(invitees)))
	for _, t := range invitees {
		t.SendServerMessage(fmt.Sprintf("%v wants to form a pairing group with you. Members: %s. Type /accept to accept or /deny to decline.", oocDisplayName(client), g.roster()))
	}
}

// cmdGroupPair handles /grouppair.
func cmdGroupPair(client *Client, args []string, _ string) {
	startGroupPair(client, args, false)
}

// cmdForceGroupPair handles /forcegrouppair (mod-only).
func cmdForceGroupPair(client *Client, args []string, _ string) {
	startGroupPair(client, args, true)
}

// cmdLeaveGroup handles /leavegroup: the caller gracefully leaves their group.
func cmdLeaveGroup(client *Client, _ []string, _ string) {
	g := client.PairGroup()
	if g == nil {
		client.SendServerMessage("You are not in a pairing group.")
		return
	}
	g.remove(client, oocDisplayName(client)+" left — the pairing group has been updated.")
}
