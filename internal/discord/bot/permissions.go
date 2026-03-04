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

package bot

import "github.com/bwmarrin/discordgo"

// isModerator returns true if the invoking Discord member has the configured moderator role.
// If no mod role ID is configured, all interactions are allowed (open access).
func (b *Bot) isModerator(i *discordgo.InteractionCreate) bool {
	if b.modRoleID == "" {
		return true
	}
	if i.Member == nil {
		return false
	}
	for _, roleID := range i.Member.Roles {
		if roleID == b.modRoleID {
			return true
		}
	}
	return false
}

// requireMod checks whether the invoking user is a moderator and sends an error response if not.
// Returns true if the user is authorized, false otherwise.
func (b *Bot) requireMod(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	if !b.isModerator(i) {
		respondEmbedEphemeral(s, i, errorEmbed("You do not have permission to use this command."))
		return false
	}
	return true
}

// isAdmin returns true if the invoking Discord member has the server Administrator permission,
// or if the interaction was initiated by the bot's own user account.
func (b *Bot) isAdmin(i *discordgo.InteractionCreate) bool {
	// Allow the bot's own user (e.g. internal triggers surfaced as interactions).
	if b.session.State.User != nil && i.Member != nil && i.Member.User != nil &&
		i.Member.User.ID == b.session.State.User.ID {
		return true
	}
	if i.Member == nil {
		return false
	}
	return i.Member.Permissions&discordgo.PermissionAdministrator != 0
}

// requireAdmin checks whether the invoking user has Administrator permissions and sends an
// ephemeral error response if not. Returns true if the user is authorized, false otherwise.
func (b *Bot) requireAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	if !b.isAdmin(i) {
		respondEmbedEphemeral(s, i, errorEmbed("This command requires server Administrator permissions."))
		return false
	}
	return true
}
