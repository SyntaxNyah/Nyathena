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

// handlePunishment returns a handler for applying a named punishment to a player.
func (b *Bot) handlePunishment(name string) func(*discordgo.Session, *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if !b.requireMod(s, i) {
			return
		}
		opts := i.ApplicationCommandData().Options
		playerArg := optionString(opts, "player")
		durationStr := optionString(opts, "duration")

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

		if err := b.server.ApplyPunishment(p.UID, name, dur); err != nil {
			respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to apply punishment: %v", err)))
			return
		}

		durDesc := "permanently"
		if dur > 0 {
			durDesc = "for " + durationStr
		}
		respondEmbed(s, i, successEmbed(
			fmt.Sprintf("Punishment Applied — %s", name),
			fmt.Sprintf("**%s** [UID %d] has been given the `%s` punishment %s.", p.Character, p.UID, name, durDesc),
		))
	}
}
