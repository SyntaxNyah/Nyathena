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
	"strings"

	"github.com/bwmarrin/discordgo"
)

// handlePlayers handles the /players command.
func (b *Bot) handlePlayers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	players := b.server.GetPlayers()
	if len(players) == 0 {
		respondEmbed(s, i, infoEmbed("👥 Connected Players", "No players are currently connected."))
		return
	}

	var lines []string
	for _, p := range players {
		lines = append(lines, fmt.Sprintf("**[%d]** %s (`%s`) — %s", p.UID, p.Character, p.OOCName, p.Area))
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("👥 Connected Players (%d)", len(players)),
		Description: strings.Join(lines, "\n"),
		Color:       colorBlue,
	}
	respondEmbed(s, i, embed)
}

// handleInfo handles the /info command.
func (b *Bot) handleInfo(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.server.FindPlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("ℹ️ Player Info — %s", p.Character),
		Color: colorBlue,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "UID", Value: fmt.Sprintf("%d", p.UID), Inline: true},
			{Name: "Character", Value: p.Character, Inline: true},
			{Name: "OOC Name", Value: p.OOCName, Inline: true},
			{Name: "Area", Value: p.Area, Inline: true},
			{Name: "IPID", Value: p.IPID, Inline: true},
		},
	}
	respondEmbed(s, i, embed)
}

// handleFind handles the /find command.
func (b *Bot) handleFind(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.server.FindPlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}
	respondEmbed(s, i, infoEmbed(
		fmt.Sprintf("🔍 Player Found — %s", p.Character),
		fmt.Sprintf("**[%d]** %s is currently in **%s**.", p.UID, p.Character, p.Area),
	))
}

// handleStatus handles the /status command.
func (b *Bot) handleStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	areas := b.server.GetAreas()
	count := b.server.GetPlayerCount()
	max := b.server.GetMaxPlayers()
	name := b.server.GetServerName()

	var areaLines []string
	for _, a := range areas {
		if a.PlayerCount > 0 {
			areaLines = append(areaLines, fmt.Sprintf("**%s** — %d player(s) [%s/%s]", a.Name, a.PlayerCount, a.Status, a.Lock))
		}
	}
	desc := fmt.Sprintf("**Players:** %d / %d\n**Areas:** %d total", count, max, len(areas))
	if len(areaLines) > 0 {
		desc += "\n\n**Active Areas:**\n" + strings.Join(areaLines, "\n")
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📡 Server Status — %s", name),
		Description: desc,
		Color:       colorGreen,
	}
	respondEmbed(s, i, embed)
}
