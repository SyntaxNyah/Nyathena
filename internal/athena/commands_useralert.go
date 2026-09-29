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
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// userAlertHelp is the guide shown when /useralert is run bare. The \n escapes
// in the Go source become real newlines in the rendered message; the \\n
// sequences are shown to the user literally so they learn the formatting trick
// (mirrors roomMotdHelp).
const userAlertHelp = "Usage: /useralert <uid> <message>\n" +
	"       /useralert global <message>\n" +
	"\n" +
	"Sends a popup (BB packet) to a specific player by UID, or to every\n" +
	"connected player with the 'global' keyword. Moderators only.\n" +
	"\n" +
	"Formatting:\n" +
	"  \\n    new line (type a backslash, then n)\n" +
	"  \\n\\n  blank line between paragraphs\n" +
	"  Unicode symbols (• ━━━ → and emoji) also render\n" +
	"\n" +
	"Examples:\n" +
	"  /useralert 5 Please stop spamming OOC.\n" +
	"  /useralert global Server maintenance in 10 minutes."

// cmdUserAlert sends a BB popup to a specific player (/useralert <uid>
// <message>) or to every connected player (/useralert global <message>).
// Permission is enforced by the registry (reqPerms: MUTE) so only moderators
// can reach it. The text is sent verbatim like /mod and /announce — it is not
// censor-filtered, because a moderator issuing a targeted or global popup is
// deliberate staff speech, not player chat.
func cmdUserAlert(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage(userAlertHelp)
		return
	}
	if len(args) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + userAlertHelp)
		return
	}

	msg := strings.Join(args[1:], " ")
	// Formatting: a literal \n (backslash + n) becomes a newline; pasted CRLF is
	// normalized to LF so the popup renders one clean line break (same as the
	// /roommotd popup).
	msg = strings.ReplaceAll(msg, "\\n", "\n")
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	msg = strings.ReplaceAll(msg, "\r", "\n")
	if strings.TrimSpace(msg) == "" {
		client.SendServerMessage(userAlertHelp)
		return
	}

	if args[0] == "global" {
		broadcastToAll(&packet.BB{Message: encode(msg)})
		client.SendServerMessage(fmt.Sprintf("📢 Popup alert sent to every connected player:\n> %v", msg))
		addToBuffer(client, "CMD", fmt.Sprintf("Sent global popup alert: %v", msg), true)
		return
	}

	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID. Usage: /useralert <uid> <message> | /useralert global <message>")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("No connected player has UID %d.", uid))
		return
	}
	target.Send(&packet.BB{Message: encode(msg)})
	client.SendServerMessage(fmt.Sprintf("Popup alert sent to %v [UID %d]:\n> %v", target.OOCName(), uid, msg))
	addToBuffer(client, "CMD", fmt.Sprintf("Sent popup alert to UID %d: %v", uid, msg), true)
}
