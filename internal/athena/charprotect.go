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
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// /charprotect — MOD only. Lets a moderator claim their current character as
// protected: if they later change into an area where another player is
// already using that same character (a normal situation, since each area
// tracks its taken character slots independently), the moderator keeps the
// character and the OTHER player is bumped to a random free character
// instead. Without this, ChangeArea's normal behavior demotes the
// *incoming* client to spectator whenever the slot they held is already
// taken in the destination area — which is the moderator, not the
// interloper.
//
// Session-only (an atomic.Bool on *Client, see client.go) — there's no
// evasion concern to guard against here (nobody is trying to escape a
// punishment), so unlike /curserandomchar this deliberately isn't persisted
// to the database. A reconnect simply resets it and the moderator can
// re-enable it with /charprotect on.

// CharProtectEnabled reports whether this client currently has character
// protection armed.
func (c *Client) CharProtectEnabled() bool {
	return c.charProtectOn.Load()
}

// SetCharProtectEnabled arms or disarms character protection for this
// client's current session.
func (c *Client) SetCharProtectEnabled(on bool) {
	c.charProtectOn.Store(on)
}

// findCharHolder returns the client currently holding charID in area a, or
// nil if nobody does.
func findCharHolder(a *area.Area, charID int) *Client {
	var found *Client
	clients.ForEach(func(c *Client) {
		if found != nil || c.Area() != a || c.CharID() != charID {
			return
		}
		found = c
	})
	return found
}

// resolveCharProtectOnJoin is called from ChangeArea/forceChangeArea right
// before a client would otherwise be demoted to spectator because their
// held character is already taken in the destination area a. If client has
// protection armed and someone else in a is holding that character, that
// other player is bumped to a random free character and the slot is freed
// for client to keep. Returns true when the slot was freed (caller should
// NOT demote client to spectator); false when protection doesn't apply and
// normal behavior should proceed.
func resolveCharProtectOnJoin(client *Client, a *area.Area) bool {
	if client.CharID() == -1 || !client.CharProtectEnabled() {
		return false
	}
	holder := findCharHolder(a, client.CharID())
	if holder == nil || holder == client {
		return false
	}
	newid := getRandomFreeChar(holder)
	if newid == -1 {
		return false // No free character to bump the holder to; fall back to normal demotion.
	}
	heldName := getCharacters()[client.CharID()]
	holder.ChangeCharacter(newid)
	holder.SendServerMessage(fmt.Sprintf("A moderator's protected claim on %v bumped you to a random character.", heldName))
	return true
}

// cmdCharProtect handles /charprotect <on|off>. MOD only (enforced by the
// command registry). With no argument it reports the caller's current
// setting.
func cmdCharProtect(client *Client, args []string, usage string) {
	if len(args) == 0 {
		state := "OFF"
		if client.CharProtectEnabled() {
			state = "ON"
		}
		client.SendServerMessage(fmt.Sprintf("Character protection is currently %s for you.\n%s", state, usage))
		return
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "on":
		if client.CharID() == -1 {
			client.SendServerMessage("You must be using a character to enable character protection.")
			return
		}
		client.SetCharProtectEnabled(true)
		client.SendServerMessage(fmt.Sprintf(
			"Character protection is now ON. If you change into an area where someone else is using %v, they will be bumped to a random character instead of you.",
			getCharacters()[client.CharID()]))
		addToBuffer(client, "CMD", "Enabled character protection.", false)
	case "off":
		client.SetCharProtectEnabled(false)
		client.SendServerMessage("Character protection is now OFF.")
		addToBuffer(client, "CMD", "Disabled character protection.", false)
	default:
		client.SendServerMessage("Invalid argument:\n" + usage)
	}
}
