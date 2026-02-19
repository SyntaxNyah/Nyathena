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

// commandHelp maps command names to their usage, description, permissions, and example.
var commandHelp = map[string]struct {
	usage    string
	desc     string
	perms    string
	example  string
	related  []string
}{
	"help":            {"/help [command]", "Display all commands or detailed info for a specific command.", "None", "/help ban", []string{}},
	"players":         {"/players", "List all currently connected players.", "Moderator", "/players", []string{"info", "find", "status"}},
	"info":            {"/info <player>", "Get detailed information about a specific player (UID, character, area, IPID).", "Moderator", "/info 5", []string{"find", "players"}},
	"find":            {"/find <player>", "Find which area a player is currently in.", "Moderator", "/find Phoenix", []string{"info", "players"}},
	"status":          {"/status", "Get server status, player count, and area statistics.", "Moderator", "/status", []string{"players"}},
	"mute":            {"/mute <player> [duration] [reason]", "Mute a player from IC and OOC chat.", "Moderator", "/mute 3 30m Spamming", []string{"unmute", "gag"}},
	"unmute":          {"/unmute <player>", "Remove a mute from a player.", "Moderator", "/unmute 3", []string{"mute"}},
	"ban":             {"/ban <player> [duration] <reason>", "Ban a player from the server.", "Moderator", "/ban 3 3d Rule violation", []string{"unban", "kick"}},
	"unban":           {"/unban <id>", "Unban a player by their ban ID.", "Moderator", "/unban 42", []string{"ban", "banlist"}},
	"kick":            {"/kick <player> [reason]", "Kick a player from the server.", "Moderator", "/kick 3 Disconnecting", []string{"ban", "mute"}},
	"gag":             {"/gag <player>", "Prevent a player from speaking in IC chat.", "Moderator", "/gag 3", []string{"ungag", "mute"}},
	"ungag":           {"/ungag <player>", "Remove a gag from a player.", "Moderator", "/ungag 3", []string{"gag"}},
	"warn":            {"/warn <player> <reason>", "Issue a formal warning to a player.", "Moderator", "/warn 3 Spamming OOC", []string{"warnings"}},
	"warnings":        {"/warnings <player>", "View all warnings issued to a player.", "Moderator", "/warnings 3", []string{"warn"}},
	"parrot":          {"/parrot <player> [duration]", "Make a player repeat random parrot messages.", "Moderator", "/parrot 3 10m", []string{"roulette"}},
	"drunk":           {"/drunk <player> [duration]", "Apply a drunk text effect to a player's messages.", "Moderator", "/drunk 3 1h", []string{"stutterstep"}},
	"slowpoke":        {"/slowpoke <player> [duration]", "Slow down a player's message rate.", "Moderator", "/slowpoke 3 30m", []string{"roulette"}},
	"roulette":        {"/roulette <player> [duration]", "Apply a random punishment to a player.", "Moderator", "/roulette 3 15m", []string{"parrot", "drunk"}},
	"spotlight":       {"/spotlight <player> [duration]", "Force a player's messages to appear with an announcement prefix.", "Moderator", "/spotlight 3 20m", []string{"whisper"}},
	"whisper":         {"/whisper <player> [duration]", "Force a player into whisper mode.", "Moderator", "/whisper 3 10m", []string{"spotlight"}},
	"stutterstep":     {"/stutterstep <player> [duration]", "Apply a stutter effect to a player's messages.", "Moderator", "/stutterstep 3 10m", []string{"drunk"}},
	"backward":        {"/backward <player> [duration]", "Reverse all of a player's messages.", "Moderator", "/backward 3 15m", []string{"drunk"}},
	"pm":              {"/pm <player> <message>", "Send a private server message to a player.", "Moderator", "/pm 3 Hello!", []string{"announce"}},
	"announce":        {"/announce <message>", "Send a server-wide announcement to all players.", "Moderator", "/announce Welcome everyone!", []string{"pm", "announce_player"}},
	"announce_player": {"/announce_player <player> <message>", "Send an announcement to a specific player.", "Moderator", "/announce_player 3 You're special!", []string{"announce", "pm"}},
	"forcemove":       {"/forcemove <player> <area>", "Force move a player to a specified area.", "Moderator", "/forcemove 3 Courtroom", []string{"cleararea"}},
	"cleararea":       {"/cleararea <area>", "Force move all players out of an area.", "Moderator", "/cleararea Lobby", []string{"forcemove", "lock"}},
	"lock":            {"/lock <area>", "Lock an area so only invited players can enter.", "Moderator", "/lock Courtroom", []string{"unlock"}},
	"unlock":          {"/unlock <area>", "Unlock a previously locked area.", "Moderator", "/unlock Courtroom", []string{"lock"}},
	"logs":            {"/logs <player>", "View recent activity logs for a player.", "Moderator", "/logs 3", []string{"auditlog"}},
	"auditlog":        {"/auditlog [filter]", "View the server audit log with an optional filter.", "Moderator", "/auditlog ban", []string{"logs"}},
	"banlist":         {"/banlist", "View the full list of currently banned players.", "Moderator", "/banlist", []string{"ban", "unban"}},
}

// handleHelp handles the /help command.
func (b *Bot) handleHelp(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	// /help <command> – detailed help for a specific command
	if len(options) > 0 {
		cmd := strings.ToLower(strings.TrimSpace(options[0].StringValue()))
		info, ok := commandHelp[cmd]
		if !ok {
			respondEmbed(s, i, errorEmbed(fmt.Sprintf("Unknown command: `%s`. Use `/help` to see all commands.", cmd)))
			return
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("📖 Command: /%s", cmd),
			Description: info.desc,
			Color:       colorBlue,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Usage", Value: fmt.Sprintf("`%s`", info.usage), Inline: false},
				{Name: "Example", Value: fmt.Sprintf("`%s`", info.example), Inline: false},
				{Name: "Required Permissions", Value: info.perms, Inline: true},
			},
		}
		if len(info.related) > 0 {
			related := make([]string, len(info.related))
			for idx, r := range info.related {
				related[idx] = fmt.Sprintf("`/%s`", r)
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:  "Related Commands",
				Value: strings.Join(related, ", "),
			})
		}
		respondEmbed(s, i, embed)
		return
	}

	// /help – categorized overview of all commands
	embed := &discordgo.MessageEmbed{
		Title:       "📋 Nyathena Moderation Bot — Help",
		Description: "Use `/help <command>` for detailed information about a specific command.\nAll commands require the **Moderator** role unless stated otherwise.",
		Color:       colorBlue,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "📊 Player Information",
				Value: "`/players` — List connected players\n" +
					"`/info` — Get player details\n" +
					"`/find` — Find a player's area\n" +
					"`/status` — Server status & stats",
				Inline: false,
			},
			{
				Name: "🛡️ Moderation",
				Value: "`/mute` `/unmute` — Mute/unmute a player\n" +
					"`/ban` `/unban` — Ban/unban a player\n" +
					"`/kick` — Kick a player\n" +
					"`/gag` `/ungag` — Prevent/allow IC speech\n" +
					"`/warn` `/warnings` — Warnings system",
				Inline: false,
			},
			{
				Name: "🎭 Custom Punishments",
				Value: "`/parrot` `/drunk` `/slowpoke`\n" +
					"`/roulette` `/spotlight` `/whisper`\n" +
					"`/stutterstep` `/backward`",
				Inline: false,
			},
			{
				Name: "💬 Communication",
				Value: "`/pm` — Private message a player\n" +
					"`/announce` — Server-wide announcement\n" +
					"`/announce_player` — Announcement to one player",
				Inline: false,
			},
			{
				Name: "🏛️ Area Control",
				Value: "`/forcemove` — Move player to area\n" +
					"`/cleararea` — Clear an area\n" +
					"`/lock` `/unlock` — Lock/unlock an area",
				Inline: false,
			},
			{
				Name: "📝 Audit & Logs",
				Value: "`/logs` — Player activity logs\n" +
					"`/auditlog` — Server audit log\n" +
					"`/banlist` — List of banned players",
				Inline: false,
			},
		},
	}
	respondEmbed(s, i, embed)
}
