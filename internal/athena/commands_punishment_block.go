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
	"sync"
)

// blockedPunishmentIPIDs is a server-level set of IPIDs that are blocked from
// using self-applied chaos commands (/maso, /megamaso, /potion, /coinflip).
// Key: IPID string, value: struct{}. Moderator-issued punishments are unaffected.
// Does not persist across server restarts.
var blockedPunishmentIPIDs sync.Map

// isPunishmentBlocked reports whether the given IPID is blocked from using
// self-applied chaos commands.
func isPunishmentBlocked(ipid string) bool {
	_, ok := blockedPunishmentIPIDs.Load(ipid)
	return ok
}

// cmdBlockPunishment blocks a player from using self-applied chaos commands
// (/maso, /megamaso, /potion, /coinflip). Moderator-issued punishments are unaffected.
//
//	/blockpunishment <uid1>,<uid2>,...
func cmdBlockPunishment(client *Client, args []string, usage string) {
	targets := getUidList(strings.Split(args[0], ","))
	if len(targets) == 0 {
		client.SendServerMessage("No valid UID(s) provided.\n" + usage)
		return
	}
	var count int
	var report string
	for _, c := range targets {
		ipid := c.Ipid()
		if isPunishmentBlocked(ipid) {
			client.SendServerMessage(fmt.Sprintf("UID %v is already blocked from self-applied commands.", c.Uid()))
			continue
		}
		blockedPunishmentIPIDs.Store(ipid, struct{}{})
		c.SendServerMessage("A moderator has disabled self-applied punishment commands for you (/maso, /megamaso, /potion, /coinflip).")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	if count == 0 {
		return
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Self-applied chaos blocked for %v client(s).", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Blocked self-punishment for UIDs: %v.", report), false)
}

// cmdUnblockPunishment restores a player's access to self-applied chaos commands.
//
//	/unblockpunishment <uid1>,<uid2>,...
func cmdUnblockPunishment(client *Client, args []string, usage string) {
	targets := getUidList(strings.Split(args[0], ","))
	if len(targets) == 0 {
		client.SendServerMessage("No valid UID(s) provided.\n" + usage)
		return
	}
	var count int
	var report string
	for _, c := range targets {
		ipid := c.Ipid()
		if !isPunishmentBlocked(ipid) {
			client.SendServerMessage(fmt.Sprintf("UID %v is not blocked from self-applied commands.", c.Uid()))
			continue
		}
		blockedPunishmentIPIDs.Delete(ipid)
		c.SendServerMessage("A moderator has restored your access to self-applied chaos commands.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	if count == 0 {
		return
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Self-applied chaos restored for %v client(s).", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Unblocked self-punishment for UIDs: %v.", report), false)
}
