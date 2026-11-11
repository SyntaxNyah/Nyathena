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

// Per-identity repeat counters for the raid guard.
//
// raidState is per CONNECTION and is deliberately thrown away when the socket
// closes. That is correct for message-shape evidence, but it made two script
// tells trivially evadable: a bot that reconnects got a fresh score, and a bot
// that re-rolled characters got one warning and then nothing. This ledger is
// keyed by the connection's IPID + hashed HDID and outlives the socket, so a
// reconnect loop is visible as a reconnect loop rather than as N unrelated
// first connections.
//
// The key includes the HDID because an IPID is an IP hash: several real players
// behind one NAT share it, and a household of three must not look like three
// rerolls. A script that rotates its HDID falls back to IPID-scoped scoring in
// raidguard.go, which is where correlation and heat live.
package athena

import (
	"fmt"
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

const (
	raidLedgerMaxEntries = 4096
	raidLedgerEntryTTL   = 30 * time.Minute
	raidLedgerMaxJoins   = 64

	// Three rapid CCs inside raidGuardCCWindow() make one churn flag, and
	// three flags inside raidGuardCCFlagWindow() are the repeat offence.
	raidGuardCCRapidLimit = 3
)

type raidLedgerEntry struct {
	mu sync.Mutex

	active   int         // joined connections currently alive for this key
	joins    []time.Time // completed handshakes that started a session
	lastCC   time.Time
	rapidCC  []time.Time // CCs closer together than raid_guard_char_churn_ms
	ccFlags  []time.Time // rapid-CC clusters inside the flag window
	lastSeen time.Time   // guarded by raidSystem.ledger.mu
}

type raidLedgerStore struct {
	mu      sync.Mutex
	entries map[string]*raidLedgerEntry
}

func newRaidLedgerStore() *raidLedgerStore {
	return &raidLedgerStore{entries: make(map[string]*raidLedgerEntry)}
}

// raidLedgerKey identifies one client across reconnects. The HDID is already
// stored as a hash, never as the raw hardware identifier.
func raidLedgerKey(client *Client) string {
	if client == nil {
		return ""
	}
	hdid := client.Hdid()
	if hdid == "" {
		return client.Ipid()
	}
	return client.Ipid() + "|" + hdid
}

func raidLedgerEntryFor(client *Client, now time.Time) *raidLedgerEntry {
	key := raidLedgerKey(client)
	if key == "" {
		return nil
	}

	raidSystem.ledger.mu.Lock()
	defer raidSystem.ledger.mu.Unlock()
	if entry, ok := raidSystem.ledger.entries[key]; ok {
		entry.lastSeen = now
		return entry
	}
	if len(raidSystem.ledger.entries) >= raidLedgerMaxEntries {
		for oldKey, entry := range raidSystem.ledger.entries {
			if now.Sub(entry.lastSeen) > raidLedgerEntryTTL {
				delete(raidSystem.ledger.entries, oldKey)
			}
		}
		if len(raidSystem.ledger.entries) >= raidLedgerMaxEntries {
			raidSystem.ledger.entries = make(map[string]*raidLedgerEntry)
		}
	}
	entry := &raidLedgerEntry{lastSeen: now}
	raidSystem.ledger.entries[key] = entry
	return entry
}

// pruneBefore trims a time-ordered slice to entries after cutoff. Timestamps
// are appended in order, so expired entries are always at the front.
func pruneBefore(times []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	if i == len(times) {
		return nil
	}
	if i > 0 {
		return times[i:]
	}
	return times
}

// raidLedgerNoteJoin records a completed handshake and returns how many
// rerolls this identity has made inside the window. The first join is zero.
func raidLedgerNoteJoin(client *Client, now time.Time) int {
	entry := raidLedgerEntryFor(client, now)
	if entry == nil {
		return 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// A second live connection with the same IPID+HDID is a second tab, not a
	// reroll. Only a join that starts with no other live connection for this
	// identity counts toward the reroll window.
	if entry.active > 0 {
		entry.active++
		return len(entry.joins) - 1
	}
	entry.active = 1
	entry.joins = pruneBefore(entry.joins, now.Add(-raidGuardRerollWindow()))
	entry.joins = append(entry.joins, now)
	if len(entry.joins) > raidLedgerMaxJoins {
		entry.joins = entry.joins[len(entry.joins)-raidLedgerMaxJoins:]
	}
	return len(entry.joins) - 1
}

// raidLedgerNoteDisconnect releases the active-connection slot recorded by
// raidLedgerNoteJoin. Called from clientCleanup while the UID is still valid.
func raidLedgerNoteDisconnect(client *Client, now time.Time) {
	entry := raidLedgerEntryFor(client, now)
	if entry == nil {
		return
	}
	entry.mu.Lock()
	if entry.active > 0 {
		entry.active--
	}
	entry.mu.Unlock()
}

// raidLedgerNoteReplay records a post-join askchaa. The packet is impossible
// for a client driven by the server's own responses, so it counts as two
// rerolls rather than one: one connection replaying its handshake is stronger
// evidence than an ordinary reconnect.
func raidLedgerNoteReplay(client *Client, now time.Time) int {
	entry := raidLedgerEntryFor(client, now)
	if entry == nil {
		return 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.joins = pruneBefore(entry.joins, now.Add(-raidGuardRerollWindow()))
	entry.joins = append(entry.joins, now, now)
	if len(entry.joins) > raidLedgerMaxJoins {
		entry.joins = entry.joins[len(entry.joins)-raidLedgerMaxJoins:]
	}
	return len(entry.joins) - 1
}

// raidLedgerNoteCharChange records one CC and returns how many rapid-churn
// flags this identity has accumulated inside the flag window.
func raidLedgerNoteCharChange(client *Client, now time.Time) int {
	entry := raidLedgerEntryFor(client, now)
	if entry == nil {
		return 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()

	churnMs := 1000
	if config != nil {
		churnMs = raidGuardInt(config.RaidGuardCharChurnMs, churnMs)
	}
	churn := time.Duration(churnMs) * time.Millisecond
	if churn <= 0 {
		churn = time.Second
	}

	gap := time.Duration(0)
	if !entry.lastCC.IsZero() {
		gap = now.Sub(entry.lastCC)
	}
	entry.lastCC = now

	if gap > 0 && gap < churn {
		entry.rapidCC = pruneBefore(entry.rapidCC, now.Add(-raidGuardCCWindow()))
		entry.rapidCC = append(entry.rapidCC, now)
		if len(entry.rapidCC) >= raidGuardCCRapidLimit {
			entry.rapidCC = nil
			entry.ccFlags = pruneBefore(entry.ccFlags, now.Add(-raidGuardCCFlagWindow()))
			entry.ccFlags = append(entry.ccFlags, now)
		}
	}

	entry.ccFlags = pruneBefore(entry.ccFlags, now.Add(-raidGuardCCFlagWindow()))
	return len(entry.ccFlags)
}

// raidLedgerCounts reports the current repeat counters for one identity, for
// /raidguard status. It prunes expired entries but never mutates the tally.
func raidLedgerCounts(client *Client, now time.Time) (rerolls, ccFlags int) {
	entry := raidLedgerEntryFor(client, now)
	if entry == nil {
		return 0, 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()

	joins := pruneBefore(entry.joins, now.Add(-raidGuardRerollWindow()))
	if len(joins) > 0 {
		// The first join is a normal session start, not a reroll.
		rerolls = len(joins) - 1
	}
	ccFlags = len(pruneBefore(entry.ccFlags, now.Add(-raidGuardCCFlagWindow())))
	return rerolls, ccFlags
}

// raidGuardRerollWindow is how far back completed joins count toward a reroll.
func raidGuardRerollWindow() time.Duration {
	seconds := 600
	if config != nil {
		seconds = raidGuardInt(config.RaidGuardRerollWindow, seconds)
	}
	return time.Duration(seconds) * time.Second
}

// raidGuardRerollLimit is the reroll count that triggers the repeat action.
func raidGuardRerollLimit() int {
	if config == nil {
		return 3
	}
	return raidGuardInt(config.RaidGuardRerollLimit, 3)
}

// raidGuardCCWindow is the span in which three fast CCs make one churn flag.
func raidGuardCCWindow() time.Duration {
	seconds := 60
	if config != nil {
		seconds = raidGuardInt(config.RaidGuardCCWindow, seconds)
	}
	return time.Duration(seconds) * time.Second
}

// raidGuardCCFlagWindow is how long a churn flag stays counted.
func raidGuardCCFlagWindow() time.Duration {
	seconds := 1800
	if config != nil {
		seconds = raidGuardInt(config.RaidGuardCCFlagWindow, seconds)
	}
	return time.Duration(seconds) * time.Second
}

// raidGuardCCFlagLimit is the flag count that triggers the repeat action.
func raidGuardCCFlagLimit() int {
	if config == nil {
		return 3
	}
	return raidGuardInt(config.RaidGuardCCFlagLimit, 3)
}

// raidGuardChallengeStrikes is the number of blocked messages a guard-issued
// challenge tolerates. Deliberately smaller than the join-captcha budget: the
// guard already has behavioural evidence, and the point is to stop a script
// before it can keep filling the room.
func raidGuardChallengeStrikes() int32 {
	if config == nil {
		return 2
	}
	return int32(raidGuardInt(config.RaidGuardChallengeStrikes, 2))
}

// raidGuardChallengeTimeout is how long a guard-issued challenge may sit
// unanswered. Unlike join_captcha_timeout, 0 does not disable it: a challenge
// with no deadline is exactly the zombie connection this exists to prevent.
func raidGuardChallengeTimeout() int {
	if config == nil {
		return 60
	}
	return raidGuardInt(config.RaidGuardChallengeTimeout, 60)
}

// raidGuardRepeatOffence applies the action for a repeated handshake-reroll or
// rapid-character-change pattern. It is behaviour-based, not score-based: the
// threshold is the number of times the behaviour was observed, and playtime
// decides how much of it an identity is allowed before it loses the session.
//
// Brand-new identities are banned for raid_guard_ban_duration. Established
// identities (15 minutes and up) are kicked, not banned, and anyone at the
// full-exemption playtime is only alerted. A DB error fails open: a hiccup must
// never ban a real player.
func (g *raidGuardSystem) repeatOffence(client *Client, reason string, count int) {
	if client == nil || client.closed.Load() || permissions.IsModerator(client.Perms()) {
		return
	}

	seconds, err := cachedPlaytime(client.Ipid())
	if err != nil {
		return
	}

	strictMinutes := 15
	minMinutes := 1200
	if config != nil {
		strictMinutes = raidGuardInt(config.RaidGuardStrictPlaytime, strictMinutes)
		minMinutes = raidGuardInt(config.RaidGuardMinPlaytime, minMinutes)
	}

	if minMinutes > 0 && seconds >= int64(minMinutes)*60 {
		sendRaidGuardAlert(fmt.Sprintf(
			"%s (UID %d) matched repeated %s (%d times) but is playtime-exempt; no action taken.",
			oocDisplayName(client), client.Uid(), reason, count))
		return
	}

	if strictMinutes > 0 && seconds < int64(strictMinutes)*60 {
		if duration, ok := raidGuardBanDuration(); ok {
			autobanFlooderFor(client.Ipid(), "raid guard (repeated "+reason+")", duration)
		}
		client.SendSync(&aolib.KK{Reason: "Banned by the raid guard: repeated " + reason + "."})
		client.markClosed()
	} else {
		client.SendSync(&aolib.KK{Reason: "Disconnected by the raid guard: repeated " + reason + "."})
		client.markClosed()
	}

	logger.LogInfof("Raid guard: repeated %s (%d) on IPID:%v UID:%v playtime=%ds",
		reason, count, client.Ipid(), client.Uid(), seconds)
	logger.WriteAudit(fmt.Sprintf("%v | RAID_GUARD_REPEAT | IPID:%v | UID:%v | reason=%v | count=%d | playtime=%ds",
		time.Now().UTC().Format("15:04:05"), client.Ipid(), client.Uid(), reason, count, seconds))
	alertRaidGuard(client, VerdictKick, 0, []string{reason})
}
