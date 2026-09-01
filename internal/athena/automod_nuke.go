// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// The nuke tier: the operator naming a specific word and saying "not this one,
// ever".
//
// Every other automod action leaves something behind. shadow echoes the message
// to its sender so their client looks normal; torment lets them keep talking
// into a void; kick and mute both leave the message to be dealt with separately.
// Those are the right defaults for a word list that mostly holds borderline
// calls, and they are the wrong behaviour for a small number of words where
// there is nothing to weigh up.
//
// So a nuke does the two things none of the others do together: the message is
// destroyed before it can reach anybody, INCLUDING the sender, and the IPID is
// banned. No echo is deliberate -- a shadow echo is a kindness to somebody who
// might be innocent, and it is also a tell: a sender who sees their own message
// land learns nothing, while a sender who sees nothing at all learns the same
// thing a network drop would tell them.
//
// This is a DETERMINISTIC rule, not a heuristic, and that distinction carries
// the whole safety argument. The raid guard is careful about banning because it
// is guessing from behaviour, so it demands three independent signals plus
// cross-IPID corroboration plus a concurrent server-wide raid before it will
// ever ban. None of that reasoning applies here: nobody is guessing. An operator
// typed an exact word into a file and said what should happen when somebody says
// it. The two mechanisms are kept separate on purpose, and a nuke deliberately
// contributes NO raid-guard score -- scoring a connection that is already banned
// would be pointless, and letting content rules feed the behavioural ban gate
// would quietly weaken the invariant that a lone player cannot be banned by it.
package athena

import (
	"fmt"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/xhit/go-str2duration/v2"
)

// autoModNukeBanDuration reports how long a nuke-tier ban lasts. The bool is
// false for a permanent ban, which db.AddBan represents as an expiry of -1
// rather than as a duration -- the same convention the automod "ban" action
// already uses.
func autoModNukeBanDuration() (time.Duration, bool) {
	if config == nil {
		return 0, false
	}
	raw := config.AutoModNukeBanDuration
	switch raw {
	case "", "permanent", "forever", "perma":
		return 0, false
	}
	d, err := str2duration.ParseDuration(raw)
	if err != nil || d <= 0 {
		// A misconfigured duration falls back to permanent rather than to some
		// arbitrary short ban: the operator put this word in the nuke tier, so
		// erring toward the harsher reading is what they asked for, and the
		// warning tells them to fix it.
		logger.LogWarningf("automod_nuke_ban_duration %q is not a valid duration — falling back to a permanent ban", raw)
		return 0, false
	}
	return d, true
}

// nukeFieldsOrNothing checks every field of one packet for a nuke-tier entry and,
// on the first hit, destroys the packet and bans the sender.
//
// This exists because the per-field checks that follow it are allowed to
// short-circuit each other. Once the message text trips a shadow, pktIC skips
// the showname entirely -- an optimisation that was harmless while every trip
// did the same thing, and stopped being harmless the moment tiers arrived: a
// merely "default" word in the message would shield a nuke word in the
// showname, and the offender would get a shadow-drop instead of a ban. The same
// shape exists in pktOOC, where a shadowed username returns before the message
// is ever looked at.
//
// Rather than unpick every short-circuit, the worst tier is decided across ALL
// fields up front, matching what matchWordEntries already does within a single
// field: the worst thing present wins, no matter which field it is hiding in or
// what any other field did. Everything gentler is left to the existing
// per-field logic below, whose short-circuiting is still correct for it.
//
// Matching only -- no action is taken for anything below the nuke tier here, so
// this cannot double-alert or double-apply an action.
func nukeFieldsOrNothing(client *Client, fields ...[2]string) bool {
	if client == nil {
		return false
	}
	entries := effectiveWordEntries()
	if len(entries) == 0 {
		return false
	}
	for _, f := range fields {
		text, source := f[0], f[1]
		if strings.TrimSpace(text) == "" {
			continue
		}
		if m := matchWordEntries(entries, text); m.Matched && m.Entry.Severity == SeverityNuke {
			return applyAutoModNuke(client, m, source)
		}
	}
	return false
}

// applyAutoModNuke destroys the message and bans the sender.
//
// Returns true when the caller must abandon the packet immediately and return
// without echoing, broadcasting, logging to the area buffer, or touching any
// client state. It returns true even when the ban itself fails: a nuke-tier word
// must never reach the room, and a database error is not a reason to deliver it.
func applyAutoModNuke(client *Client, m WordListMatch, source string) bool {
	if client == nil || !m.Matched || m.Entry.Severity != SeverityNuke {
		return false
	}

	banTime := time.Now().UTC()
	until := int64(-1)
	untilText := "∞"
	if d, temporary := autoModNukeBanDuration(); temporary {
		until = banTime.Add(d).Unix()
		untilText = banTime.Add(d).Format("02 Jan 2006 15:04 MST")
	}

	// A connection that is somehow already banned still gets its message
	// destroyed and dropped; only the ban record is skipped.
	if banned, _, err := db.IsBanned(db.IPID, client.Ipid()); err == nil && banned {
		logger.LogInfof("automod nuke: %v (uid %d) matched %q in %s; already banned, message destroyed",
			client.Ipid(), client.Uid(), m.Entry.Raw, source)
		client.conn.Close()
		return true
	}

	id, err := db.AddBan(client.Ipid(), client.Hdid(), banTime.Unix(), until,
		"Automatic ban: prohibited language ("+m.Entry.Raw+")", "Server")
	if err != nil {
		logger.LogErrorf("automod nuke: failed to ban %v after matching %q: %v — message destroyed anyway",
			client.Ipid(), m.Entry.Raw, err)
		client.conn.Close()
		alertCensorTrip(client, source, m.Entry.Raw, "",
			"NUKE tier: the message was destroyed, but the ban FAILED — ban them by hand.")
		return true
	}

	forgetIP(client.Ipid())
	client.SendSync(&packet.KB{Reason: fmt.Sprintf("Banned for prohibited language.\nUntil: %s\nID: %d", untilText, id)})
	client.conn.Close()

	alertCensorTrip(client, source, m.Entry.Raw, "",
		fmt.Sprintf("NUKE tier: the message was destroyed before anyone saw it (not even them) and they were banned until %s (ID %d).", untilText, id))
	logger.LogInfof("automod nuke: banned %v (uid %d) until %s — matched %q in %s",
		client.Ipid(), client.Uid(), untilText, m.Entry.Raw, source)
	return true
}
