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
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// pktOOC dispatches a slash command and returns long before it reaches the
// content gate, the torment branch, the stealthmute branch, the captcha
// restriction or the raid guard. That is correct for the overwhelming majority
// of commands, which either take no free text or address only the caller.
//
// It is wrong for the handful that carry arbitrary player text to other people.
// /global reaches EVERY client on the server and /pm reaches any UID list the
// sender names, and neither was examined by anything: a slur went out in full,
// a stealthmuted player was audible again, a tormented one escaped the ghosting,
// and a connection the captcha had already stopped could still talk. The word
// filter appeared to be working because it was -- on the path those two commands
// do not take.
//
// oocCommandAllowed puts that path back, in the same order pktOOC applies it, so
// the two cannot drift on what a censored, tormented or silenced player sees.
//
// text must be the DECODED message (pktOOC splits args off an already-decoded
// string, so a command handler's args are plain text and must not be decoded
// again). echo is the packet the caller was about to send; every suppressing
// branch sends exactly that packet back to the sender alone, so their own client
// looks normal and the suppression stays undetectable from the inside -- the
// property shadow-sending exists for.
//
// Returns false when the caller must return without broadcasting.
func oocCommandAllowed(client *Client, text, source string, echo *packet.CTToClient) bool {
	// 1. Content. A nuke-tier hit is destroyed without even an echo and bans the
	//    IPID; everything else follows the configured automod_action, with watch
	//    tier passing through untouched.
	match, result, kick := autoModCheckTiered(client, text, source)
	if match.Matched && match.Entry.Severity == SeverityNuke {
		applyAutoModNuke(client, match, source)
		return false
	}
	raidGuardOnWordHit(client, match)

	switch result {
	case autoModBlocked:
		if kick {
			client.KickForCensorTrip()
		}
		return false
	case autoModShadow:
		client.Send(echo)
		addToBuffer(client, "OOC", "\""+text+"\" (censored)", false)
		if kick {
			client.KickForCensorTrip()
		}
		return false
	}

	// 2. Torment. Deliberately ghosted rather than routed through
	//    handleTormentedOOC: that helper re-broadcasts to the sender's AREA after
	//    a delay, which for a server-wide global would resurrect the message
	//    outside this gate and deliver it to the wrong audience. Ghosting is one
	//    of the two things torment already does, so this is a narrowing of its
	//    behaviour on this path, never an escape from it.
	if isIPIDTormented(client.Ipid()) {
		client.Send(echo)
		addToBuffer(client, "OOC", "\""+text+"\" (tormented)", false)
		return false
	}

	// 3. Stealthmute. Without this a stealthmuted player was audible to the whole
	//    server through /global -- the one place the punishment most needed to hold.
	if client.HasActivePunishment(PunishmentStealthMute) {
		client.Send(echo)
		addToBuffer(client, "OOC", "\""+text+"\" (stealthmuted)", false)
		return false
	}

	// 4. Captcha restriction. The command branch gates on awaitingCaptcha, but a
	//    connection that has already run out of attempts carries captchaRestricted
	//    instead (issueJoinCaptcha hands the flag over), and nothing was consulting
	//    it here -- so striking out silenced a player everywhere except the command
	//    that reaches the most people.
	if activeCaptchaRestricted.Load() > 0 && client.captchaRestricted.Load() {
		client.Send(echo)
		addToBuffer(client, "OOC", "\""+text+"\" (captcha-muted)", false)
		return false
	}

	return true
}
