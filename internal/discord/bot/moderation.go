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
	"time"

	"github.com/bwmarrin/discordgo"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// resolvePlayer looks up a player by UID (numeric string) or OOC name.
func (b *Bot) resolvePlayer(arg string) *PlayerInfo {
	p := b.server.FindPlayer(arg)
	return p
}

// parseDuration parses a human-readable duration string. Returns 0 for empty/permanent.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	d, err := str2duration.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: use values like 30m, 1h, 3d", s)
	}
	return d, nil
}

// optionString returns a named option value or "" if not present.
func optionString(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, o := range options {
		if o.Name == name {
			return strings.TrimSpace(o.StringValue())
		}
	}
	return ""
}

// handleMute handles the /mute command.
func (b *Bot) handleMute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	durationStr := optionString(opts, "duration")
	reason := optionString(opts, "reason")
	if reason == "" {
		reason = "No reason provided."
	}

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	dur, err := parseDuration(durationStr)
	if err != nil {
		respondEmbed(s, i, errorEmbed(err.Error()))
		return
	}

	if err := b.server.MutePlayer(p.UID, dur, reason); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to mute player: %v", err)))
		return
	}

	durDesc := "permanently"
	if dur > 0 {
		durDesc = "for " + durationStr
	}
	respondEmbed(s, i, successEmbed("Player Muted", fmt.Sprintf("**%s** [UID %d] has been muted %s.\nReason: %s", p.Character, p.UID, durDesc, reason)))
}

// handleUnmute handles the /unmute command.
func (b *Bot) handleUnmute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}
	if err := b.server.UnmutePlayer(p.UID); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to unmute player: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Player Unmuted", fmt.Sprintf("**%s** [UID %d] has been unmuted.", p.Character, p.UID)))
}

// handleBan handles the /ban command.
func (b *Bot) handleBan(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	durationStr := optionString(opts, "duration")
	reason := optionString(opts, "reason")
	if reason == "" {
		reason = "No reason provided."
	}

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	dur, err := parseDuration(durationStr)
	if err != nil {
		respondEmbed(s, i, errorEmbed(err.Error()))
		return
	}

	moderator := "Discord"
	if i.Member != nil && i.Member.User != nil {
		moderator = i.Member.User.Username
	}

	if err := b.server.BanPlayer(p.IPID, dur, reason, moderator); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to ban player: %v", err)))
		return
	}

	durDesc := "permanently"
	if dur > 0 {
		durDesc = "for " + durationStr
	}
	respondEmbed(s, i, successEmbed("Player Banned", fmt.Sprintf("**%s** [UID %d] has been banned %s.\nReason: %s", p.Character, p.UID, durDesc, reason)))
}

// handleUnban handles the /unban command.
func (b *Bot) handleUnban(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	id := int(i.ApplicationCommandData().Options[0].IntValue())
	if err := b.server.UnbanByID(id); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to unban ID %d: %v", id, err)))
		return
	}
	respondEmbed(s, i, successEmbed("Player Unbanned", fmt.Sprintf("Ban ID **%d** has been removed.", id)))
}

// handleKick handles the /kick command.
func (b *Bot) handleKick(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	reason := optionString(opts, "reason")
	if reason == "" {
		reason = "No reason provided."
	}

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	if err := b.server.KickPlayer(p.UID, reason); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to kick player: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Player Kicked", fmt.Sprintf("**%s** [UID %d] has been kicked.\nReason: %s", p.Character, p.UID, reason)))
}

// handleGag handles the /gag command.
func (b *Bot) handleGag(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}
	if err := b.server.GagPlayer(p.UID); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to gag player: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Player Gagged", fmt.Sprintf("**%s** [UID %d] has been gagged from IC chat.", p.Character, p.UID)))
}

// handleUngag handles the /ungag command.
func (b *Bot) handleUngag(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}
	if err := b.server.UngagPlayer(p.UID); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to ungag player: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Player Ungagged", fmt.Sprintf("**%s** [UID %d] can now speak in IC chat.", p.Character, p.UID)))
}

// handleWarn handles the /warn command.
func (b *Bot) handleWarn(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	reason := optionString(opts, "reason")

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	moderator := "Discord"
	if i.Member != nil && i.Member.User != nil {
		moderator = i.Member.User.Username
	}

	if err := b.server.WarnPlayer(p.UID, reason, moderator); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to warn player: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Warning Issued", fmt.Sprintf("**%s** [UID %d] has been warned.\nReason: %s", p.Character, p.UID, reason)))
}

// handleWarnings handles the /warnings command.
func (b *Bot) handleWarnings(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	playerArg := i.ApplicationCommandData().Options[0].StringValue()
	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	warnings := b.server.GetWarnings(p.IPID)
	if len(warnings) == 0 {
		respondEmbed(s, i, infoEmbed(fmt.Sprintf("⚠️ Warnings — %s", p.Character), "No warnings on record."))
		return
	}

	var lines []string
	for idx, w := range warnings {
		lines = append(lines, fmt.Sprintf("**%d.** %s — by %s", idx+1, w.Reason, w.Moderator))
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("⚠️ Warnings — %s [UID %d] (%d total)", p.Character, p.UID, len(warnings)),
		Description: strings.Join(lines, "\n"),
		Color:       colorOrange,
	}
	respondEmbed(s, i, embed)
}

// handleBanList handles the /banlist command.
func (b *Bot) handleBanList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	bans := b.server.GetBanList()
	if len(bans) == 0 {
		respondEmbed(s, i, infoEmbed("🚫 Ban List", "No active bans."))
		return
	}

	var lines []string
	for _, ban := range bans {
		durStr := "Permanent"
		if ban.Duration != -1 {
			durStr = "Until " + time.Unix(ban.Duration, 0).UTC().Format("02 Jan 2006 15:04 UTC")
		}
		lines = append(lines, fmt.Sprintf("**ID %d** — IPID: `%s` | %s | Reason: %s | By: %s",
			ban.ID, ban.IPID, durStr, ban.Reason, ban.Moderator))
	}

	// Discord embed descriptions have a 4096 char limit; truncate if needed.
	desc := strings.Join(lines, "\n")
	if len(desc) > 4000 {
		desc = desc[:4000] + "\n…(truncated)"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🚫 Ban List (%d entries)", len(bans)),
		Description: desc,
		Color:       colorRed,
	}
	respondEmbed(s, i, embed)
}
