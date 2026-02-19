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

// handleForceMove handles the /forcemove command.
func (b *Bot) handleForceMove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	opts := i.ApplicationCommandData().Options
	playerArg := optionString(opts, "player")
	areaArg := optionString(opts, "area")

	p := b.resolvePlayer(playerArg)
	if p == nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Player not found: `%s`", playerArg)))
		return
	}

	if err := b.server.ForceMove(p.UID, areaArg); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to move player: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Player Moved", fmt.Sprintf("**%s** [UID %d] has been moved to **%s**.", p.Character, p.UID, areaArg)))
}

// handleClearArea handles the /cleararea command.
func (b *Bot) handleClearArea(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	areaArg := i.ApplicationCommandData().Options[0].StringValue()
	if err := b.server.ClearArea(areaArg); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to clear area: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Area Cleared", fmt.Sprintf("All players have been moved out of **%s**.", areaArg)))
}

// handleLock handles the /lock command.
func (b *Bot) handleLock(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	areaArg := i.ApplicationCommandData().Options[0].StringValue()
	if err := b.server.LockArea(areaArg); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to lock area: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Area Locked", fmt.Sprintf("**%s** has been locked.", areaArg)))
}

// handleUnlock handles the /unlock command.
func (b *Bot) handleUnlock(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	areaArg := i.ApplicationCommandData().Options[0].StringValue()
	if err := b.server.UnlockArea(areaArg); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to unlock area: %v", err)))
		return
	}
	respondEmbed(s, i, successEmbed("Area Unlocked", fmt.Sprintf("**%s** has been unlocked.", areaArg)))
}
