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

// applicationCommands returns all slash command definitions to register with Discord.
func applicationCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		// Help
		{
			Name:        "help",
			Description: "Display help information for bot commands.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "command",
					Description: "Get detailed help for a specific command.",
					Required:    false,
				},
			},
		},
		// Player information
		{
			Name:        "players",
			Description: "List all connected players.",
		},
		{
			Name:        "info",
			Description: "Get detailed information about a specific player.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "player",
					Description: "UID or OOC name of the player.",
					Required:    true,
				},
			},
		},
		{
			Name:        "find",
			Description: "Find which area a player is in.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "player",
					Description: "UID or OOC name of the player.",
					Required:    true,
				},
			},
		},
		{
			Name:        "status",
			Description: "Get server status and statistics.",
		},
		// Moderation
		{
			Name:        "mute",
			Description: "Mute a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration (e.g. 1h, 30m). Leave blank for permanent.", Required: false},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for mute.", Required: false},
			},
		},
		{
			Name:        "unmute",
			Description: "Unmute a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
			},
		},
		{
			Name:        "ban",
			Description: "Ban a player from the server.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration (e.g. 3d, 1w). Leave blank for permanent.", Required: false},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for ban.", Required: true},
			},
		},
		{
			Name:        "unban",
			Description: "Unban a player by ban ID.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "id", Description: "Ban ID.", Required: true},
			},
		},
		{
			Name:        "kick",
			Description: "Kick a player from the server.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for kick.", Required: false},
			},
		},
		{
			Name:        "gag",
			Description: "Prevent a player from speaking in IC.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
			},
		},
		{
			Name:        "ungag",
			Description: "Remove a gag from a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
			},
		},
		{
			Name:        "warn",
			Description: "Issue a warning to a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for warning.", Required: true},
			},
		},
		{
			Name:        "warnings",
			Description: "View warnings for a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
			},
		},
		// Custom punishments
		{
			Name:        "parrot",
			Description: "Make a player repeat random messages.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration (e.g. 10m, 1h).", Required: false},
			},
		},
		{
			Name:        "drunk",
			Description: "Apply drunk text effect to a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		{
			Name:        "slowpoke",
			Description: "Slow down a player's messages.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		{
			Name:        "roulette",
			Description: "Apply a random punishment to a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		{
			Name:        "spotlight",
			Description: "Force a player into spotlight mode.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		{
			Name:        "whisper",
			Description: "Force a player into whisper mode.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		{
			Name:        "stutterstep",
			Description: "Apply stutter effect to a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		{
			Name:        "backward",
			Description: "Reverse a player's text.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration.", Required: false},
			},
		},
		// Communication
		{
			Name:        "pm",
			Description: "Send a private message to a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "Message to send.", Required: true},
			},
		},
		{
			Name:        "announce",
			Description: "Send a server-wide announcement.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "Announcement text.", Required: true},
			},
		},
		{
			Name:        "announce_player",
			Description: "Send an announcement to a specific player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "Message to send.", Required: true},
			},
		},
		// Area control
		{
			Name:        "forcemove",
			Description: "Force move a player to an area.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "area", Description: "Target area name.", Required: true},
			},
		},
		{
			Name:        "cleararea",
			Description: "Clear all players from an area.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "area", Description: "Area name.", Required: true},
			},
		},
		{
			Name:        "lock",
			Description: "Lock an area.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "area", Description: "Area name.", Required: true},
			},
		},
		{
			Name:        "unlock",
			Description: "Unlock an area.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "area", Description: "Area name.", Required: true},
			},
		},
		// Audit & Logs
		{
			Name:        "logs",
			Description: "View activity logs for a player.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "player", Description: "UID or OOC name.", Required: true},
			},
		},
		{
			Name:        "auditlog",
			Description: "View the server audit log.",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "filter", Description: "Optional filter string.", Required: false},
			},
		},
		{
			Name:        "banlist",
			Description: "View the list of banned players.",
		},
	}
}

// registerCommands registers all slash commands with Discord.
func (b *Bot) registerCommands() error {
	cmds := applicationCommands()
	registered := make([]*discordgo.ApplicationCommand, 0, len(cmds))
	for _, cmd := range cmds {
		created, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.guildID, cmd)
		if err != nil {
			return fmt.Errorf("failed to register command %q: %w", cmd.Name, err)
		}
		registered = append(registered, created)
	}
	b.commands = registered
	return nil
}

// commandHandlers returns the mapping of command names to handler functions.
func (b *Bot) commandHandlers() map[string]func(*discordgo.Session, *discordgo.InteractionCreate) {
	return map[string]func(*discordgo.Session, *discordgo.InteractionCreate){
		// Help
		"help": b.handleHelp,
		// Player information
		"players": b.handlePlayers,
		"info":    b.handleInfo,
		"find":    b.handleFind,
		"status":  b.handleStatus,
		// Moderation
		"mute":     b.handleMute,
		"unmute":   b.handleUnmute,
		"ban":      b.handleBan,
		"unban":    b.handleUnban,
		"kick":     b.handleKick,
		"gag":      b.handleGag,
		"ungag":    b.handleUngag,
		"warn":     b.handleWarn,
		"warnings": b.handleWarnings,
		// Custom punishments
		"parrot":      b.handlePunishment("parrot"),
		"drunk":       b.handlePunishment("drunk"),
		"slowpoke":    b.handlePunishment("slowpoke"),
		"roulette":    b.handlePunishment("roulette"),
		"spotlight":   b.handlePunishment("spotlight"),
		"whisper":     b.handlePunishment("whisper"),
		"stutterstep": b.handlePunishment("stutterstep"),
		"backward":    b.handlePunishment("backward"),
		// Communication
		"pm":              b.handlePM,
		"announce":        b.handleAnnounce,
		"announce_player": b.handleAnnouncePlayer,
		// Area control
		"forcemove": b.handleForceMove,
		"cleararea": b.handleClearArea,
		"lock":      b.handleLock,
		"unlock":    b.handleUnlock,
		// Audit & Logs
		"logs":     b.handleLogs,
		"auditlog": b.handleAuditLog,
		"banlist":  b.handleBanList,
	}
}
