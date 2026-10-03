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

// startGroupPair implements /grouppair and /forcegrouppair. For /grouppair the
// initiator invites any number of other players and joins their own group;
// force skips the accept flow and pairs ONLY the invitees (the issuing
// moderator is not a member), marking everyone accepted immediately.
func startGroupPair(client *Client, args []string, force bool) {
	if len(args) == 0 {
		client.SendServerMessage("Usage: /grouppair <uid> [uid ...]")
		return
	}

	// /grouppair joins the caller to the group, so they need a character and
	// must not already be grouped. /forcegrouppair only groups the targets, so
	// the moderator is exempt from both checks.
	if !force {
		if client.CharID() < 0 {
			client.SendServerMessage("You have not selected a character.")
			return
		}
		if client.PairGroup() != nil {
			client.SendServerMessage("You are already in a pairing group.")
			return
		}
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

	// Force mode pairs only the invitees; consent mode adds the initiator first.
	members := invitees
	accepted := make(map[int]bool, len(members))
	if force {
		for _, m := range members {
			accepted[m.Uid()] = true
		}
	} else {
		members = append([]*Client{client}, invitees...)
		accepted[client.Uid()] = true
	}

	g := &PairGroup{
		members:  members,
		accepted: accepted,
		groupID:  strconv.Itoa(client.Uid()),
		forced:   force,
	}
	for _, m := range g.members {
		m.SetPairGroup(g)
	}

	if force {
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

// cmdUnforceGroupPair handles /unforcegrouppair (mod-only): it disbands every
// force-paired group in the caller's area, undoing /forcegrouppair.
func cmdUnforceGroupPair(client *Client, _ []string, _ string) {
	area := client.Area()
	seen := make(map[*PairGroup]struct{})
	var groups []*PairGroup
	clients.ForEach(func(c *Client) {
		if c.Area() != area {
			return
		}
		g := c.PairGroup()
		if g == nil || !g.IsForced() {
			return
		}
		if _, ok := seen[g]; ok {
			return
		}
		seen[g] = struct{}{}
		groups = append(groups, g)
	})
	if len(groups) == 0 {
		client.SendServerMessage("There are no force-paired groups in this area.")
		return
	}
	for _, g := range groups {
		g.dissolve("A moderator disbanded your force-paired group.")
	}
	client.SendServerMessage(fmt.Sprintf("Disbanded %d force-paired group(s) in this area.", len(groups)))
	addToBuffer(client, "CMD", fmt.Sprintf("Disbanded %d force-paired group(s).", len(groups)), false)
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
