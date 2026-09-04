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
	"time"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// cmdArea dispatches the /area sub-commands. It is registered under the "area"
// command name, so a player typing "/area mute" arrives here with
// args = ["mute"]. Both CMs (area CMs or CM-permission holders) and moderators
// can run it — the registry gates entry on the CM permission, which every mod
// role carries, and clientCanUseCommand also lets area CMs through.
func cmdArea(client *Client, args []string, usage string) {
	switch strings.ToLower(args[0]) {
	case "mute":
		areaMuteAll(client, false)
	case "unmute":
		areaMuteAll(client, true)
	case "rename":
		cmdAreaRename(client, args[1:], usage)
	case "unrename":
		cmdAreaUnrename(client, args[1:], usage)
	default:
		client.SendServerMessage("Unknown /area sub-command.\n" + usage)
	}
}

// areaMuteAll mutes (or, when unmute is true, unmutes) every player in the
// caller's area except CMs and moderators. Muting applies both IC and OOC
// (ICOOCMuted) but is scoped to the room and lives purely in memory — unlike a
// real /mute it is NEVER persisted to the DB. Leaving the area
// (client.liftAreaMuteOnDeparture, hooked into ChangeArea/forceChangeArea) or
// disconnecting entirely lifts it, since /area mute is meant to silence someone
// only while they're actually in that room. Persisting it was the cause of the
// "forever muted by a CM" bug: a reconnect restored the mute via SetMuted (which
// clears the in-memory area origin), leaving an origin-less ICOOCMuted that
// neither /area unmute nor leaving the area could clear, and that a CM (who has
// no MUTE permission) could not /unmute. The caller is never affected, nor is
// any CM or moderator in the room.
func areaMuteAll(client *Client, unmute bool) {
	a := client.Area()

	var targets []*Client
	clients.ForEach(func(c *Client) {
		if c.Area() != a || c == client {
			return
		}
		// Never silence staff: area CMs, CM-permission holders, and moderators
		// are all exempt.
		if c.HasCMPermission() || permissions.IsModerator(c.Perms()) {
			return
		}
		targets = append(targets, c)
	})

	var count int
	for _, c := range targets {
		if unmute {
			// Only cancel out an /area mute that *this* area applied: lift the
			// ICOOCMuted state it set, and leave a separate individual mute
			// (IC-only, OOC-only, music, etc.) — or one from another area —
			// untouched so /area unmute is a clean inverse of /area mute.
			if c.Muted() != ICOOCMuted || c.AreaMuteOrigin() != a {
				continue
			}
			c.SetMuted(Unmuted)
			c.SetUnmuteTime(time.Time{})
			c.SendServerMessage("The area mute has been lifted; you can speak again.")
		} else {
			// Don't clobber a player who already carries a separate individual
			// mute — otherwise /area unmute couldn't restore it. Only silence
			// players who are currently unmuted.
			if c.Muted() != Unmuted {
				continue
			}
			c.SetAreaMuted(a)
			c.SetUnmuteTime(time.Time{})
			c.SendServerMessage("This area has been muted by staff; you cannot speak IC or OOC until you leave, or until the mute is lifted.")
		}
		count++
	}

	if unmute {
		sendAreaServerMessage(a, fmt.Sprintf("%v lifted the area mute — %v player(s) can speak again.", oocDisplayName(client), count))
		client.SendServerMessage(fmt.Sprintf("Unmuted %v player(s) in this area.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Lifted area mute (%v players).", count), false)
	} else {
		sendAreaServerMessage(a, fmt.Sprintf("%v muted the area — everyone except CMs and moderators has been silenced (%v player(s)).", oocDisplayName(client), count))
		client.SendServerMessage(fmt.Sprintf("Muted %v player(s) in this area. CMs and moderators are exempt. Use /area unmute to lift it.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Muted the area (%v players).", count), false)
	}
}
