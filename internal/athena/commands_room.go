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

// /room: a peer-to-peer area invite any player can use, unlike the CM-only /invite
// (which only grants lock-entry without moving anyone). /room invite <uid> sends
// the target an OOC request; /room invite accept moves the acceptor straight to
// the inviter's area — even a locked one, as long as the inviter is still there.
package athena

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// roomInviteTTL is how long a pending /room invite stays valid. After this the
// invite is treated as expired and cannot be accepted.
const roomInviteTTL = 60 * time.Second

// roomInviterName labels the inviting player for the OOC invite message:
// showname first, then OOC name, then character name — the "showname/oocname"
// identity players already see in OOC-facing notices.
func roomInviterName(c *Client) string {
	if name := strings.TrimSpace(c.EffectiveShowname()); name != "" {
		return name
	}
	if name := strings.TrimSpace(c.OOCName()); name != "" {
		return name
	}
	return clientDisplayName(c)
}

// cmdRoom dispatches the /room sub-commands.
func cmdRoom(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	switch strings.ToLower(args[0]) {
	case "invite":
		if len(args) < 2 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		if strings.EqualFold(args[1], "accept") {
			roomInviteAccept(client)
			return
		}
		roomInviteSend(client, args[1])
	default:
		client.SendServerMessage("Unknown /room sub-command.\n" + usage)
	}
}

// roomInviteSend records a pending invite on the target and tells them about it.
func roomInviteSend(client *Client, uidArg string) {
	uid, err := strconv.Atoi(strings.TrimSpace(uidArg))
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("No client found with that UID.")
		return
	}
	if target == client {
		client.SendServerMessage("You can't invite yourself.")
		return
	}
	if ok, remaining := client.CheckAndUpdateRoomInviteCooldown(); !ok {
		client.SendServerMessage(fmt.Sprintf("You're inviting too quickly. Please wait %v.", remaining.Truncate(time.Second)))
		return
	}

	// Read the inviter's identity before locking the target so the two mutexes
	// are never held in a nested order.
	inviterUID := client.Uid()
	inviterArea := client.Area()
	target.mu.Lock()
	target.pendingRoomInviteFrom = inviterUID
	target.pendingRoomInviteArea = inviterArea
	target.pendingRoomInviteAt = time.Now()
	target.mu.Unlock()

	target.SendServerMessage(fmt.Sprintf("%v has invited you to their area. Type /room invite accept to join.", roomInviterName(client)))
	client.SendServerMessage(fmt.Sprintf("You invited %v to your area.", clientDisplayName(target)))
}

// roomInviteAccept moves the accepting player to the inviter's area when the
// invite is still valid: the inviter must still be in the area they invited from.
func roomInviteAccept(client *Client) {
	client.mu.Lock()
	from := client.pendingRoomInviteFrom
	inviteArea := client.pendingRoomInviteArea
	inviteAt := client.pendingRoomInviteAt
	client.mu.Unlock()

	if inviteArea == nil {
		client.SendServerMessage("You have no pending area invite.")
		return
	}
	if time.Since(inviteAt) > roomInviteTTL {
		client.clearRoomInvite()
		client.SendServerMessage("That invite has expired.")
		return
	}
	inviter, err := getClientByUid(from)
	if err != nil {
		client.clearRoomInvite()
		client.SendServerMessage("The player who invited you is no longer online.")
		return
	}
	if inviter.Area() != inviteArea {
		client.clearRoomInvite()
		client.SendServerMessage("The player who invited you has left their area.")
		return
	}
	if client.Area() == inviteArea {
		client.clearRoomInvite()
		client.SendServerMessage("You are already in that area.")
		return
	}

	// Grant lock entry when the area is locked, then move. Admin-locked areas are
	// still refused by ChangeArea (an absolute seal).
	if inviteArea.Lock() == area.LockLocked {
		inviteArea.AddInvited(client.Uid())
	}
	client.clearRoomInvite()
	if client.ChangeArea(inviteArea) {
		client.SendServerMessage(fmt.Sprintf("You accepted %v's invite.", roomInviterName(inviter)))
		inviter.SendServerMessage(fmt.Sprintf("%v accepted your area invite.", clientDisplayName(client)))
	}
}

// clearRoomInvite clears any pending /room invite on this client.
func (c *Client) clearRoomInvite() {
	c.mu.Lock()
	c.pendingRoomInviteFrom = -1
	c.pendingRoomInviteArea = nil
	c.pendingRoomInviteAt = time.Time{}
	c.mu.Unlock()
}
