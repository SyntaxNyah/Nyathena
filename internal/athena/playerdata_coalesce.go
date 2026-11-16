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

// Player-data (PU) coalescing.
//
// ChangeCharacter sends two full-server PU broadcasts per character change
// (char name and showname). A 319-CC minute at ~40 online clients is roughly
// 25,000 PU sends -- and the final value is the only one a client needs. Keep
// the latest value per (client, PU type) and publish at most five times per
// second. The client pointer is part of the key and is validated on flush, so
// a packet queued for a connection that left cannot be applied to a reused
// UID.
package athena

import (
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

const playerDataDebounce = 200 * time.Millisecond

type playerDataKey struct {
	client *Client
	uid    int
	typ    aolib.PlayerDataType
}

var (
	playerDataDirty       sync.Map // map[playerDataKey]string
	playerDataFlusherOnce sync.Once
)

// requestPlayerData records the latest value for one PU field and ensures the
// flusher is running. Replaces per-change broadcastToAll(PU) calls.
func requestPlayerData(c *Client, typ aolib.PlayerDataType, data string) {
	if c == nil {
		return
	}
	uid := c.Uid()
	if uid == -1 {
		return
	}
	playerDataDirty.Store(playerDataKey{client: c, uid: uid, typ: typ}, data)
	playerDataFlusherOnce.Do(func() {
		go playerDataFlusher()
	})
}

func playerDataFlusher() {
	ticker := time.NewTicker(playerDataDebounce)
	defer ticker.Stop()
	for range ticker.C {
		flushPlayerData()
	}
}

// flushPlayerData broadcasts one PU per dirty (client, type) pair. Split out
// so tests can run it deterministically without sleeping.
func flushPlayerData() {
	var packets []aolib.Outgoing
	playerDataDirty.Range(func(key, value any) bool {
		playerDataDirty.Delete(key)
		k, ok := key.(playerDataKey)
		if !ok || k.client == nil || k.client.closed.Load() || k.client.Uid() != k.uid {
			return true
		}
		data, ok := value.(string)
		if !ok {
			return true
		}
		// A moderator command (e.g. /showname) may have published a newer
		// value while this update was queued. Re-read char name/showname at
		// flush time so the trailing batch can never overwrite it with the
		// snapshot captured at request time. Other PU types keep the value
		// supplied by the caller.
		switch k.typ {
		case aolib.PlayerDataTypeCharName:
			data = k.client.CurrentCharacter()
		case aolib.PlayerDataTypeShowname:
			data = decode(k.client.Showname())
		}
		packets = append(packets, &aolib.PU{ID: k.uid, Type: k.typ, Data: data})
		return true
	})
	if len(packets) > 0 {
		broadcastToAllBatch(packets)
	}
}
