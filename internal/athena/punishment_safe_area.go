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

package athena

import (
	"fmt"
	"strings"
)

// punishmentSafeBlocked reports whether target currently stands in an area
// marked punishment-safe, meaning moderators, shadow mods, and admins cannot
// land any punishment-system effect on it. Real moderation enforcement
// (/ban, /mute, /kick) never consults this — it only gates the
// punishment-system commands (text effects, dere archetypes, protocol/voice
// curses, traps, /stack, /charcurse, and the rest of the punishment list).
func punishmentSafeBlocked(target *Client) bool {
	return target.Area().PunishmentSafe()
}

// notePunishmentSafeSkip records that a target was shielded by a
// punishment-safe area so the issuing moderator's summary can report it.
// Call sites accumulate skipped/skippedReport and pass them to
// appendPunishmentSafeNotice when building their final summary message.
func notePunishmentSafeSkip(skipped *int, skippedReport *string, target *Client) {
	*skipped++
	*skippedReport += fmt.Sprintf("%v, ", target.Uid())
}

// partitionPunishmentSafe splits targets into ones that may be punished and
// ones shielded by a punishment-safe area. Use when a target list is already
// resolved to a single slice ahead of the apply loop (global and UID-list
// forms already merged).
func partitionPunishmentSafe(targets []*Client) (allowed []*Client, skipped int, skippedReport string) {
	for _, c := range targets {
		if punishmentSafeBlocked(c) {
			notePunishmentSafeSkip(&skipped, &skippedReport, c)
			continue
		}
		allowed = append(allowed, c)
	}
	return
}

// appendPunishmentSafeNotice appends a note to a moderator's summary message
// naming any targets that were shielded by a punishment-safe area and
// therefore not punished.
func appendPunishmentSafeNotice(summary string, skipped int, skippedReport string) string {
	if skipped == 0 {
		return summary
	}
	return summary + fmt.Sprintf(" %d client(s) could not be punished (punishment-safe area): %v.", skipped, strings.TrimSuffix(skippedReport, ", "))
}
