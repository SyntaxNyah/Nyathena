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

	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Censor-trip staff alerts: every time a player trips the word censor
// (AutoMod banned words in IC/OOC text, shownames, or OOC usernames, or a
// censored_names.txt showname), everyone holding MOD_CHAT gets an OOC alert.
// The alert always carries the opt-out hint, and each staff member can mute
// the alerts for their own session with /censoralerts off. Manual torment
// additions (/lag) deliberately never alert — only censor trips do.

// censorAlertHint is appended to every alert so staff always know how to
// silence the notifications.
const censorAlertHint = "(Disable these alerts for yourself with /censoralerts off)"

// maxCensorAlertTextLen caps how much of the offending text is quoted in the
// alert so a max-length slur wall doesn't flood staff OOC.
const maxCensorAlertTextLen = 120

// CensorAlertsDisabled reports whether this client has muted censor-trip
// alerts for their current session.
func (c *Client) CensorAlertsDisabled() bool {
	return c.censorAlertsOff.Load()
}

// SetCensorAlertsDisabled mutes or unmutes censor-trip alerts for this
// client's current session.
func (c *Client) SetCensorAlertsDisabled(off bool) {
	c.censorAlertsOff.Store(off)
}

// alertCensorTrip notifies every staff member holding MOD_CHAT (minus those
// who ran /censoralerts off) that offender tripped the censor. source labels
// the field that matched ("IC message", "showname", ...), matched is the
// normalized wordlist entry that fired, text is the offending decoded text
// (truncated for the alert), and outcome describes what the server did about
// it ("The message was shadow-dropped...", "They were kicked.", ...).
func alertCensorTrip(offender *Client, source, matched, text, outcome string) {
	if runes := []rune(text); len(runes) > maxCensorAlertTextLen {
		text = string(runes[:maxCensorAlertTextLen]) + "…"
	}
	areaName := "unknown area"
	if a := offender.Area(); a != nil {
		areaName = a.Name()
	}
	msg := fmt.Sprintf("%s (UID %d, IPID %s) tripped the %s censor in %s — matched %q. %s\nText: %q\n%s",
		oocDisplayName(offender), offender.Uid(), offender.Ipid(), source, areaName, matched, outcome, text, censorAlertHint)
	out := &packet.CTToClient{Name: "[CENSOR]", Message: encode(msg), IsFromServer: "1"}
	clients.ForEach(func(c *Client) {
		if !permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			return
		}
		if c.CensorAlertsDisabled() {
			return
		}
		c.Send(out)
	})
}

// cmdCensorAlerts handles /censoralerts <on|off>. With no argument it reports
// the caller's current setting. The toggle is per-session: alerts default
// back to on for every fresh connection.
func cmdCensorAlerts(client *Client, args []string, usage string) {
	if len(args) == 0 {
		state := "ENABLED"
		if client.CensorAlertsDisabled() {
			state = "DISABLED"
		}
		client.SendServerMessage(fmt.Sprintf("Censor-trip alerts are currently %s for you.\n%s", state, usage))
		return
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "on":
		client.SetCensorAlertsDisabled(false)
		client.SendServerMessage("Censor-trip alerts are now ON for you.")
	case "off":
		client.SetCensorAlertsDisabled(true)
		client.SendServerMessage("Censor-trip alerts are now OFF for you (this session only — they reset to on when you reconnect).")
	default:
		client.SendServerMessage("Invalid argument:\n" + usage)
	}
}
