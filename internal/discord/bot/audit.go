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

// handleLogs handles the /logs command.
func (b *Bot) handleLogs(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	logs := b.server.GetPlayerLogs(p.IPID)
	if len(logs) == 0 {
		respondEmbed(s, i, infoEmbed(fmt.Sprintf("📜 Logs — %s", p.Character), "No log entries found."))
		return
	}

	desc := strings.Join(logs, "\n")
	if len(desc) > 4000 {
		desc = desc[:4000] + "\n…(truncated)"
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📜 Logs — %s [UID %d]", p.Character, p.UID),
		Description: fmt.Sprintf("```\n%s\n```", desc),
		Color:       colorPurple,
	}
	respondEmbed(s, i, embed)
}

// handleAuditLog handles the /auditlog command.
func (b *Bot) handleAuditLog(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	filter := optionString(opts, "filter")

	entries := b.server.GetAuditLog(filter)
	if len(entries) == 0 {
		respondEmbed(s, i, infoEmbed("📋 Audit Log", "No audit log entries found."))
		return
	}

	desc := strings.Join(entries, "\n")
	if len(desc) > 4000 {
		desc = desc[:4000] + "\n…(truncated)"
	}
	title := "📋 Audit Log"
	if filter != "" {
		title += fmt.Sprintf(" (filter: %s)", filter)
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: fmt.Sprintf("```\n%s\n```", desc),
		Color:       colorGold,
	}
	respondEmbed(s, i, embed)
}
