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

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// handlePM handles the /pm command.
func (b *Bot) handlePM(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	message := optionString(opts, "message")

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	if err := b.server.SendPrivateMessage(p.UID, message); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to send message: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Message Sent", fmt.Sprintf("Private message sent to **%s** [UID %d].", p.Character, p.UID)))
}

// handleAnnounce handles the /announce command.
func (b *Bot) handleAnnounce(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	message := i.ApplicationCommandData().Options[0].StringValue()
	if err := b.server.SendAnnouncement(message); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to send announcement: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Announcement Sent", fmt.Sprintf("Broadcast to all players:\n> %s", message)))
}

// handleAnnouncePlayer handles the /announce_player command.
func (b *Bot) handleAnnouncePlayer(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	message := optionString(opts, "message")

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	if err := b.server.SendAnnouncementToPlayer(p.UID, message); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to send announcement: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Announcement Sent", fmt.Sprintf("Announcement sent to **%s** [UID %d]:\n> %s", p.Character, p.UID, message)))
}
