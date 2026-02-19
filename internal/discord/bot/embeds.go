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

const (
	colorBlue   = 0x3498db
	colorRed    = 0xe74c3c
	colorGreen  = 0x2ecc71
	colorOrange = 0xe67e22
	colorPurple = 0x9b59b6
	colorGold   = 0xf1c40f
	colorGray   = 0x95a5a6
)

// newEmbed returns a new Discord embed with a given color.
func newEmbed(color int) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{Color: color}
}

// embedResponse builds an InteractionResponseData with a single embed.
func embedResponse(embed *discordgo.MessageEmbed) *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}}
}

// errorEmbed returns an error embed with the given message.
func errorEmbed(msg string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "❌ Error",
		Description: msg,
		Color:       colorRed,
	}
}

// successEmbed returns a success embed with the given message.
func successEmbed(title, msg string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "✅ " + title,
		Description: msg,
		Color:       colorGreen,
	}
}

// infoEmbed returns an informational embed.
func infoEmbed(title, msg string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: msg,
		Color:       colorBlue,
	}
}

// respondEmbed sends an embed as the interaction response.
func respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: embedResponse(embed),
	})
}

// respondEmbedEphemeral sends an embed only visible to the invoking user.
func respondEmbedEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
