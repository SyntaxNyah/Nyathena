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

// oocGuardVerdictSuppresses re-reads the raid guard's effect on this connection
// immediately after it scored the message, and reports whether the message that
// earned the verdict must now be withheld.
//
// This is the ordering property pktIC already has and this path did not. There,
// raidGuardOnIC runs BEFORE the delivery switch reads captchaRestricted, so a
// connection the guard silences on message N has message N suppressed. Scoring
// after the suppression checks -- which is deliberate, so a message the room
// never heard is never fed to the correlation window -- means the flag is read
// before the guard has had its say, and the message that earned a ban goes out
// to the whole server first. On a global that is every client on the server,
// which is precisely the audience the verdict exists to protect.
//
// Two outcomes are checked. A silence sets captchaRestricted (raidGuardSilence);
// a kick or ban closes the connection (markClosed), and the offending message
// must not be delivered to everyone else on the way out.
func oocGuardVerdictSuppresses(client *Client, text string, echo *packet.CTToClient) bool {
	if client == nil {
		return false
	}
	if client.closed.Load() {
		// Kicked or banned by the verdict this message just earned. Nothing is
		// echoed -- the connection is already gone.
		addToBuffer(client, "OOC", "\""+text+"\" (raid guard)", false)
		return true
	}
	if activeCaptchaRestricted.Load() > 0 && client.captchaRestricted.Load() {
		client.Send(echo)
		addToBuffer(client, "OOC", "\""+text+"\" (raid guard)", false)
		return true
	}

	// While the server is actually under attack, a connection carrying ANY raid
	// evidence does not get to broadcast to every client on the server, even if
	// its score has not yet reached a rung that acts.
	//
	// A global is the largest audience a player can reach and it is opt-in, so
	// holding one back costs an ordinary player nothing while a raid is running.
	// The same is not true of area chat, which is why this applies here and not
	// there.
	//
	// It borrows the safety argument the under-attack threshold scaling already
	// rests on, and needs no new one:
	//
	//   - It never creates evidence. A score of zero -- every ordinary player who
	//     is merely talking while a raid happens around them -- is untouched, so
	//     the check is inert for them at any setting.
	//   - It requires the under-attack state, which needs cross-IPID correlation
	//     or an arrival burst and is not reachable by one person acting alone, so
	//     a quiet evening never enters this mode at all.
	//   - It respects the playtime tiers: raidGuardTier reports punishable=false
	//     for moderators and for anyone at or above raid_guard_min_playtime, and
	//     they are excluded here exactly as they are everywhere else.
	//
	// And it is a hold on one message, not a punishment: nothing is recorded
	// against the sender, nothing persists, and it lapses on its own when the
	// attack state does. The message is echoed back so their client looks normal.
	if rs := client.raidGuard(); rs != nil && raidGuardUnderAttack() {
		if score, _, _ := rs.snapshot(); score > 0 {
			if _, punishable := raidGuardTier(client); punishable {
				client.Send(echo)
				addToBuffer(client, "OOC", "\""+text+"\" (raid guard: held during attack)", false)
				return true
			}
		}
	}
	return false
}
