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

package athena

import (
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// Playtime is a security decision input on several hot paths: the raid-guard
// playtime tiers, the lockdown silence gate, the join-captcha exemption and the
// rate-limit autoban tiers. Every one of those used to call db.GetPlaytime
// directly; while lockdown was active that meant a SQLite query on EVERY IC and
// OOC message.
//
// A short TTL cache collapses those reads to at most one query per IPID per
// five minutes. Playtime only increases while somebody is connected, so a
// five-minute stale value can delay a tier crossing by minutes -- never grant
// or remove a permanent exemption. The cache is invalidated explicitly when a
// write path (session cleanup, /addplaytime, /setplaytime) changes the value.
const (
	playtimeCacheTTL        = 5 * time.Minute
	playtimeCacheMaxEntries = 4096
)

type playtimeCacheEntry struct {
	seconds int64
	expires time.Time
}

type playtimeStore struct {
	sync.RWMutex
	entries map[string]playtimeCacheEntry
}

func newPlaytimeStore() *playtimeStore {
	return &playtimeStore{entries: make(map[string]playtimeCacheEntry)}
}

// playtimeLookup is the DB read behind cachedPlaytime. A variable so tests can
// exercise security decisions without a live SQLite handle; production always
// points at db.GetPlaytime.
var playtimeLookup = db.GetPlaytime

// cachedPlaytime returns an IPID's all-time playtime in seconds, using a
// five-minute in-memory cache. A DB error is returned to the caller rather than
// cached: security callers have deliberately different fail-open/fail-closed
// rules, and swallowing the error here would erase that distinction.
func cachedPlaytime(ipid string) (int64, error) {
	if ipid == "" {
		return 0, nil
	}

	now := time.Now()
	raidSystem.playtime.RLock()
	entry, ok := raidSystem.playtime.entries[ipid]
	raidSystem.playtime.RUnlock()
	if ok && now.Before(entry.expires) {
		return entry.seconds, nil
	}

	seconds, err := playtimeLookup(ipid)
	if err != nil {
		return 0, err
	}

	raidSystem.playtime.Lock()
	if len(raidSystem.playtime.entries) >= playtimeCacheMaxEntries {
		for key, cached := range raidSystem.playtime.entries {
			if !now.Before(cached.expires) {
				delete(raidSystem.playtime.entries, key)
			}
		}
		if len(raidSystem.playtime.entries) >= playtimeCacheMaxEntries {
			raidSystem.playtime.entries = make(map[string]playtimeCacheEntry)
		}
	}
	raidSystem.playtime.entries[ipid] = playtimeCacheEntry{
		seconds: seconds,
		expires: now.Add(playtimeCacheTTL),
	}
	raidSystem.playtime.Unlock()
	return seconds, nil
}

// invalidatePlaytime drops one IPID's cached playtime after a write path has
// changed it. Safe to call for an IPID that was never cached.
func invalidatePlaytime(ipid string) {
	if ipid == "" {
		return
	}
	raidSystem.playtime.Lock()
	delete(raidSystem.playtime.entries, ipid)
	raidSystem.playtime.Unlock()
}
