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
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// /musicban — persistent per-IPID block on playing music. Persists across
// sessions via the MUSIC_BANS table. The MC handler consults an in-memory set
// (musicBans) seeded from the DB at startup so the hot path is a single map
// lookup under an RWMutex (no DB hit per IC tick).
//
// The "less than 3 people in the area" carve-out lets a banned player still set
// the room's mood when nobody else is around to hear it: if the area has fewer
// than musicBanQuietAreaThreshold players in it, the ban is bypassed and the
// music change is allowed. The threshold is exclusive — exactly 3 players means
// the ban applies again.
const musicBanQuietAreaThreshold = 3

var (
	musicBansMu sync.RWMutex
	musicBans   = map[string]struct{}{}
)

// initMusicBans seeds the in-memory music-ban set from the database. Called
// once during server startup after the DB is opened. A DB error is logged but
// non-fatal — the in-memory set just stays empty and bans will repopulate as
// staff re-issues them.
func initMusicBans() {
	rows, err := db.ListMusicBans()
	if err != nil {
		logger.LogErrorf("musicban: failed to load existing bans from DB: %v", err)
		return
	}
	musicBansMu.Lock()
	for _, mb := range rows {
		musicBans[mb.Ipid] = struct{}{}
	}
	n := len(musicBans)
	musicBansMu.Unlock()
	if n > 0 {
		logger.LogInfof("musicban: loaded %d music-ban(s) from database.", n)
	}
}

// isMusicBanned reports whether the given IPID currently carries a music-ban.
// Hot path: one RLock + map lookup, no DB query.
func isMusicBanned(ipid string) bool {
	musicBansMu.RLock()
	_, ok := musicBans[ipid]
	musicBansMu.RUnlock()
	return ok
}

// addMusicBanMemory inserts an IPID into the in-memory ban set. Pair with
// db.AddMusicBan for persistence; both are upserts so re-banning is idempotent.
func addMusicBanMemory(ipid string) {
	musicBansMu.Lock()
	musicBans[ipid] = struct{}{}
	musicBansMu.Unlock()
}

// removeMusicBanMemory drops an IPID from the in-memory ban set. Pair with
// db.RemoveMusicBan for persistence.
func removeMusicBanMemory(ipid string) {
	musicBansMu.Lock()
	delete(musicBans, ipid)
	musicBansMu.Unlock()
}

// cmdMusicBan (/musicban <uid> [-r reason]) bans the target's IPID from playing
// music across sessions. Requires MUTE permission. A banned player is still
// allowed to play music when the area has fewer than 3 people in it (see
// musicBanQuietAreaThreshold), so empty/quiet rooms aren't sterile.
//
// Idempotent: re-banning an already-banned IPID updates the reason and issuer
// rather than erroring.
func cmdMusicBan(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	reason := ""
	rest := args
	// Tiny ad-hoc flag parse for "-r <reason words…>". Avoids dragging in
	// flag.NewFlagSet just to pull out one optional positional reason.
	for i := 0; i < len(rest); i++ {
		if rest[i] == "-r" && i+1 < len(rest) {
			reason = strings.Join(rest[i+1:], " ")
			rest = rest[:i]
			break
		}
	}
	if len(rest) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	uid, err := strconv.Atoi(strings.TrimSpace(rest[0]))
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}
	if permissions.IsModerator(target.Perms()) {
		client.SendServerMessage("You cannot music-ban a moderator.")
		return
	}

	ipid := target.Ipid()
	if err := db.AddMusicBan(ipid, reason, client.ModName(), time.Now().UTC().Unix()); err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to persist music ban: %v", err))
		return
	}
	addMusicBanMemory(ipid)

	targetName := target.OOCName()
	notice := "You have been banned from playing music."
	if reason != "" {
		notice += " Reason: " + reason
	}
	notice += fmt.Sprintf("\n(In areas with fewer than %d people you may still play music.)", musicBanQuietAreaThreshold)
	target.SendServerMessage(notice)

	summary := fmt.Sprintf("Music-banned %v (UID %d, IPID %v).", targetName, uid, ipid)
	if reason != "" {
		summary += " Reason: " + reason
	}
	client.SendServerMessage(summary)
	addToBuffer(client, "CMD", summary, true)
}

// cmdMusicUnban (/musicunban <uid|ipid>) lifts a music-ban. Accepts either a
// connected target's UID or a raw IPID, so an offline player can still be
// unbanned.
func cmdMusicUnban(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	arg := strings.TrimSpace(args[0])
	ipid := arg
	// Numeric arg → UID lookup; non-numeric is treated as a raw IPID.
	if uid, err := strconv.Atoi(arg); err == nil {
		target, terr := getClientByUid(uid)
		if terr != nil {
			client.SendServerMessage(fmt.Sprintf("Client with UID %d not found; pass an IPID directly to unban an offline player.", uid))
			return
		}
		ipid = target.Ipid()
	}

	if err := db.RemoveMusicBan(ipid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			client.SendServerMessage(fmt.Sprintf("No music ban found for IPID %v.", ipid))
			return
		}
		client.SendServerMessage(fmt.Sprintf("Failed to remove music ban: %v", err))
		return
	}
	removeMusicBanMemory(ipid)

	summary := fmt.Sprintf("Music-unbanned IPID %v.", ipid)
	client.SendServerMessage(summary)
	addToBuffer(client, "CMD", summary, true)
}

// cmdMusicBans (/musicbans) lists every active music-ban with its reason and
// issuer, newest first. Visible to moderators only via the registry permission.
func cmdMusicBans(client *Client, _ []string, _ string) {
	rows, err := db.ListMusicBans()
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to read music bans: %v", err))
		return
	}
	if len(rows) == 0 {
		client.SendServerMessage("No active music bans.")
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Active music bans (%d):\n", len(rows))
	for _, mb := range rows {
		when := time.Unix(mb.BannedAt, 0).UTC().Format("2006-01-02 15:04 MST")
		reason := mb.Reason
		if reason == "" {
			reason = "(no reason given)"
		}
		fmt.Fprintf(&sb, "  • %v — banned %v by %v — %v\n", mb.Ipid, when, mb.BannedBy, reason)
	}
	client.SendServerMessage(strings.TrimRight(sb.String(), "\n"))
}
