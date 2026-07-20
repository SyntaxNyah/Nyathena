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
)

// cmdCharSteal handles /charsteal <uid>. MOD only (enforced by the command
// registry). Takes over the target's current character for the issuing
// moderator, then forces the target onto a random free character in the
// same area — the "kick them off" half mirrors /forcerandomchar's
// target-uid branch.
//
// The target is moved off their character first so the slot is free before
// the moderator claims it: Area.SwitchChar refuses to hand out a slot that's
// still marked taken, so claiming out of order would silently no-op.
func cmdCharSteal(client *Client, args []string, _ string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %d does not exist.", uid))
		return
	}
	if target == client {
		client.SendServerMessage("You cannot steal your own character.")
		return
	}
	if target.Area() != client.Area() {
		client.SendServerMessage("The target must be in your area to steal their character.")
		return
	}
	stolenID := target.CharID()
	if stolenID == -1 {
		client.SendServerMessage("The target is not using a character.")
		return
	}
	stolenName := getCharacters()[stolenID]

	newid := getRandomFreeChar(target)
	if newid == -1 {
		client.SendServerMessage("No free character is available to move the target to; steal aborted.")
		return
	}
	target.ChangeCharacter(newid)
	client.ChangeCharacter(stolenID)

	target.SendServerMessage(fmt.Sprintf("A moderator stole your character (%v) and forced you to a random character.", stolenName))
	client.SendServerMessage(fmt.Sprintf("Stole UID %d's character (%v) and forced them to a random character.", uid, stolenName))
	addToBuffer(client, "CMD", fmt.Sprintf("Stole UID %d's character (%v) and forced them to a random character.", uid, stolenName), true)
}
