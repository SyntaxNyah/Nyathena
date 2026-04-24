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

// Voice moderation state: IPID-scoped voice mutes and bans, per-UID rate
// limiters for VC_JOIN and VC_SIG, and the new-IPID voice cooldown tracker.
//
// All state is in-memory.  A zero expiry means "permanent until lifted".
// Mutes, bans, and cooldown entries persist until the server restarts; when
// operators need persistence across restarts they can re-issue from a logged
// audit trail.  This mirrors how the existing connFloodAutoban tracker works.

import (
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

type voiceRestriction struct {
	Until  time.Time // zero = permanent
	Reason string
}

var (
	voiceModMu sync.RWMutex

	// Keyed by IPID.
	voiceMutes = map[string]voiceRestriction{}
	voiceBans  = map[string]voiceRestriction{}

	// Per-UID rate-limit windows.  Keyed by UID.
	voiceJoinEvents = map[int][]time.Time{}
	voiceSigEvents  = map[int][]time.Time{}

	// Tracks the first time we've seen each IPID in any voice-related packet
	// so the new-IPID cooldown applies even if an operator lowers it at
	// runtime.  Separate from the connection tracker so early joiners don't
	// carry over the cooldown from before voice was enabled.
	voiceFirstSeen = map[string]time.Time{}
)

// voiceConfigRateLimit returns the configured per-UID rate limit for a given
// event type, or 0 if rate limiting is disabled.
func voiceConfigJoinLimit() (int, time.Duration) {
	if config == nil || config.JoinRateLimit <= 0 || config.JoinRateLimitWindow <= 0 {
		return 0, 0
	}
	return config.JoinRateLimit, time.Duration(config.JoinRateLimitWindow) * time.Second
}

func voiceConfigSigLimit() (int, time.Duration) {
	if config == nil || config.SigRateLimit <= 0 || config.SigRateLimitWindow <= 0 {
		return 0, 0
	}
	return config.SigRateLimit, time.Duration(config.SigRateLimitWindow) * time.Second
}

// checkVoiceRestriction returns (blocked, remainingSeconds, reason) for the
// given IPID against the given map.  Expired entries are reaped lazily.
func checkVoiceRestriction(m map[string]voiceRestriction, ipid string) (bool, int, string) {
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	r, ok := m[ipid]
	if !ok {
		return false, 0, ""
	}
	if !r.Until.IsZero() && time.Now().UTC().After(r.Until) {
		delete(m, ipid)
		return false, 0, ""
	}
	if r.Until.IsZero() {
		return true, 0, r.Reason
	}
	return true, int(time.Until(r.Until).Seconds()) + 1, r.Reason
}

// IsVoiceMuted reports whether the IPID is currently voice-muted and, if so,
// how many seconds remain (0 = permanent) and the stored reason.
func IsVoiceMuted(ipid string) (bool, int, string) {
	return checkVoiceRestriction(voiceMutes, ipid)
}

// IsVoiceBanned reports whether the IPID is currently voice-banned.
func IsVoiceBanned(ipid string) (bool, int, string) {
	return checkVoiceRestriction(voiceBans, ipid)
}

// SetVoiceMute installs a voice mute against the IPID.  A zero duration means
// permanent; a positive duration sets an expiry relative to now.
func SetVoiceMute(ipid string, duration time.Duration, reason string) {
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	r := voiceRestriction{Reason: reason}
	if duration > 0 {
		r.Until = time.Now().UTC().Add(duration)
	}
	voiceMutes[ipid] = r
}

// ClearVoiceMute lifts a voice mute.  Returns true if an entry was removed.
func ClearVoiceMute(ipid string) bool {
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	if _, ok := voiceMutes[ipid]; ok {
		delete(voiceMutes, ipid)
		return true
	}
	return false
}

// SetVoiceBan installs a voice ban against the IPID.
func SetVoiceBan(ipid string, duration time.Duration, reason string) {
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	r := voiceRestriction{Reason: reason}
	if duration > 0 {
		r.Until = time.Now().UTC().Add(duration)
	}
	voiceBans[ipid] = r
}

// ClearVoiceBan lifts a voice ban.  Returns true if an entry was removed.
func ClearVoiceBan(ipid string) bool {
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	if _, ok := voiceBans[ipid]; ok {
		delete(voiceBans, ipid)
		return true
	}
	return false
}

// listVoiceRestrictions returns a copy of the given restriction map, pruning
// expired entries along the way.
func listVoiceRestrictions(m map[string]voiceRestriction) map[string]voiceRestriction {
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	out := make(map[string]voiceRestriction, len(m))
	now := time.Now().UTC()
	for ipid, r := range m {
		if !r.Until.IsZero() && now.After(r.Until) {
			delete(m, ipid)
			continue
		}
		out[ipid] = r
	}
	return out
}

// ListVoiceMutes returns a snapshot of current voice mutes.
func ListVoiceMutes() map[string]voiceRestriction {
	return listVoiceRestrictions(voiceMutes)
}

// ListVoiceBans returns a snapshot of current voice bans.
func ListVoiceBans() map[string]voiceRestriction {
	return listVoiceRestrictions(voiceBans)
}

// allowRate checks whether a rate-limited event should be admitted.  It
// returns (allowed, retryAfterSeconds).  When rate limiting is disabled
// (limit == 0) it always returns true.
func allowRate(m map[int][]time.Time, uid int, limit int, window time.Duration) (bool, int) {
	if limit <= 0 {
		return true, 0
	}
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	now := time.Now().UTC()
	cutoff := now.Add(-window)
	events := m[uid]
	// Trim entries outside the window.
	trimmed := events[:0]
	for _, t := range events {
		if t.After(cutoff) {
			trimmed = append(trimmed, t)
		}
	}
	if len(trimmed) >= limit {
		// Earliest within-window event determines the wait.
		retry := int(window-now.Sub(trimmed[0])) + 1
		if retry < 1 {
			retry = 1
		}
		m[uid] = trimmed
		return false, retry
	}
	trimmed = append(trimmed, now)
	m[uid] = trimmed
	return true, 0
}

// allowVoiceJoin admits or rejects a VC_JOIN under the join rate limit.
func allowVoiceJoin(uid int) (bool, int) {
	limit, window := voiceConfigJoinLimit()
	return allowRate(voiceJoinEvents, uid, limit, window)
}

// allowVoiceSig admits or rejects a VC_SIG under the signalling rate limit.
func allowVoiceSig(uid int) (bool, int) {
	limit, window := voiceConfigSigLimit()
	return allowRate(voiceSigEvents, uid, limit, window)
}

// touchVoiceFirstSeen records the first time we've observed this IPID in a
// voice context and returns the remaining cooldown in seconds (0 if passed).
func touchVoiceFirstSeen(ipid string) int {
	if config == nil || config.NewIPIDVoiceCooldown <= 0 {
		return 0
	}
	voiceModMu.Lock()
	defer voiceModMu.Unlock()
	first, ok := voiceFirstSeen[ipid]
	now := time.Now().UTC()
	if !ok {
		voiceFirstSeen[ipid] = now
		return config.NewIPIDVoiceCooldown
	}
	elapsed := int(now.Sub(first).Seconds())
	if elapsed >= config.NewIPIDVoiceCooldown {
		return 0
	}
	return config.NewIPIDVoiceCooldown - elapsed
}

// clearVoiceRateStateForUID drops rate-limit state on UID release.  Called
// during cleanup to avoid the tracker growing unboundedly.
func clearVoiceRateStateForUID(uid int) {
	voiceModMu.Lock()
	delete(voiceJoinEvents, uid)
	delete(voiceSigEvents, uid)
	voiceModMu.Unlock()
}

// kickAllVoiceFromArea ejects every joined voice peer in the given area and
// broadcasts VC_LEAVE for each.  Used by /voicearea off and /vmute/vban when
// a currently-joined client is targeted.
func kickAllVoiceFromArea(a *area.Area) {
	if a == nil {
		return
	}
	peers := currentVoicePeers(a)
	for _, uid := range peers {
		c := clients.GetClientByUID(uid)
		if c != nil {
			leaveVoiceForClient(c)
		}
	}
}

// kickVoiceByIPID ejects every joined voice client whose IPID matches.
// Returns the number of clients removed.
func kickVoiceByIPID(ipid string) int {
	matches := clients.GetByIPID(ipid)
	n := 0
	for _, c := range matches {
		if c.Area() != nil && inVoiceRoom(c.Area(), c.Uid()) {
			leaveVoiceForClient(c)
			n++
		}
	}
	return n
}
