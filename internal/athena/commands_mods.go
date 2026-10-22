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
	"sort"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// cmdMods answers the one question /gas answers badly: which room is a
// moderator in right now?  It is public on purpose -- a player looking for
// help should be able to see whether any staff member is reachable at all
// without paging through the full player list (/players only reveals mod
// identities to viewers holding BAN_INFO).
//
// Visibility follows the rules /players and /gas already established:
//   - only clients holding actual moderator permissions are listed
//     (permissions.IsModerator); CM-only and DJ-only roles are not staff for
//     this purpose, and a console command grant (grantcmd) never alters
//     client.Perms(), so a player with one granted command is not listed.
//   - shadow mods are omitted for non-admin viewers, exactly as in /players
//     (see writeEntry in commands_moderation.go); admins see them tagged.
//   - admins who ran /admin hide are omitted for non-admin viewers and
//     tagged "(hidden)" for admins (see admin_hide.go).
//   - clients ghosted by shadow-disconnect (Hidden) are omitted for
//     non-admin viewers, matching /players.
func cmdMods(client *Client, _ []string, _ string) {
	viewerIsAdmin := permissions.IsAdmin(client.Perms())

	grouped := make(map[*area.Area][]*Client)
	clients.ForEach(func(c *Client) {
		if !permissions.IsModerator(c.Perms()) {
			return
		}
		if !viewerIsAdmin {
			if c.Hidden() || permissions.IsShadow(c.Perms()) {
				return
			}
			if permissions.IsAdmin(c.Perms()) && isAdminHidden(c.ModName()) {
				return
			}
		}
		a := c.Area()
		if a == nil {
			return
		}
		grouped[a] = append(grouped[a], c)
	})

	for _, list := range grouped {
		sort.Slice(list, func(i, j int) bool { return list[i].Uid() < list[j].Uid() })
	}

	var out strings.Builder
	out.WriteString("Moderators\n----------\n")

	total, shownAreas := 0, 0
	// Iterate the configured area order rather than the map, mirroring /gas,
	// so the output is stable and matches the room order players already know.
	for _, a := range areas {
		list := grouped[a]
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(&out, "%v:\n", a.Name())
		for _, c := range list {
			fmt.Fprintf(&out, "  [%d] %v%v\n", c.Uid(), modsDisplayName(c), modsRoleSuffix(c, viewerIsAdmin))
			total++
		}
		shownAreas++
	}

	if total == 0 {
		out.WriteString("No moderators are currently online.\n")
		client.SendServerMessage(out.String())
		return
	}

	modNoun, areaNoun := "moderators", "areas"
	if total == 1 {
		modNoun = "moderator"
	}
	if shownAreas == 1 {
		areaNoun = "area"
	}
	fmt.Fprintf(&out, "----------\n%d %s online in %d %s.\n", total, modNoun, shownAreas, areaNoun)
	client.SendServerMessage(out.String())
}

// modsDisplayName returns the name /mods should print for a moderator.  The
// logged-in account name is the identity staff actually use; the fallbacks
// only matter for a connection whose moderator name never got populated.
func modsDisplayName(c *Client) string {
	if name := strings.TrimSpace(c.ModName()); name != "" {
		return name
	}
	if name := strings.TrimSpace(c.DisplayModName()); name != "" {
		return name
	}
	if name := strings.TrimSpace(c.OOCName()); name != "" {
		return name
	}
	return "Moderator"
}

// modsRoleSuffix tags a shadow or hidden admin for an ADMIN viewer so staff
// oversight survives the hiding.  Everyone else gets a bare name: revealing
// that a listed moderator is specifically an admin (or a shadow mod) is not
// information a public command should leak.
func modsRoleSuffix(c *Client, viewerIsAdmin bool) string {
	if !viewerIsAdmin {
		return ""
	}
	switch {
	case permissions.IsShadow(c.Perms()):
		return " (shadow)"
	case permissions.IsAdmin(c.Perms()) && isAdminHidden(c.ModName()):
		return " (hidden)"
	case permissions.IsAdmin(c.Perms()):
		return " (admin)"
	}
	return ""
}
