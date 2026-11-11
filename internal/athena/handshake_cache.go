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
	"sync/atomic"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// Handshake list caches.
//
// SC carries one entry per character (several thousand on a real server) and
// SM carries every area plus every music track. Building either per connection
// means thousands of strings.SplitN/allocations on a path that a connection
// flood drives simultaneously for every attacker. The lists only change on
// /reload (and on an area rename for SM), so serialize them once and share the
// immutable buffers.
var (
	scPacketFanta atomic.Pointer[[]byte]
	scPacketJSON  atomic.Pointer[[]byte]
	smPacketJSON  atomic.Pointer[[]byte]
	smJSONMu      sync.Mutex
)

// rebuildSCPacketCache rebuilds the character-list packets for both wire modes.
// Called from setCharacters, i.e. startup and /reload, under the reload lock.
func rebuildSCPacketCache(chars []string) {
	packet := &aolib.SC{CharData: scCharDataItems(chars)}
	fanta := encodeFanta(packet)
	scPacketFanta.Store(&fanta)
	if encoded, err := packetutil.Encode(packet, aolib.WireJSON); err == nil {
		scPacketJSON.Store(&encoded)
	} else {
		scPacketJSON.Store(nil)
	}
}

// getSCPacketBytes returns the cached SC payload for one wire mode, or nil when
// the cache has not been populated so the caller can fall back to the typed
// send path.
func getSCPacketBytes(jsonMode bool) []byte {
	var cached *[]byte
	if jsonMode {
		cached = scPacketJSON.Load()
	} else {
		cached = scPacketFanta.Load()
	}
	if cached == nil {
		return nil
	}
	return *cached
}

// invalidateSMJSONCache drops the JSON music/area list after a reload or an
// area rename. The next JSON join rebuilds it once.
func invalidateSMJSONCache() {
	smPacketJSON.Store(nil)
}

// getSMJSONBytes returns the cached JSON SM payload, building it on first use.
// The Fanta SM blob already had a cache (getSMPacketBytes); this is the JSON
// equivalent that pktReqAM used to rebuild for every connection.
func getSMJSONBytes() []byte {
	if cached := smPacketJSON.Load(); cached != nil {
		return *cached
	}
	smJSONMu.Lock()
	defer smJSONMu.Unlock()
	if cached := smPacketJSON.Load(); cached != nil {
		return *cached
	}

	items := make([]string, 0, len(areas)+len(getMusicList()))
	for _, area := range areas {
		items = append(items, area.Name())
	}
	items = append(items, getMusicList()...)

	encoded, err := packetutil.Encode(&aolib.SM{MusicList: smMusicListItems(items)}, aolib.WireJSON)
	if err != nil {
		return nil
	}
	smPacketJSON.Store(&encoded)
	return encoded
}
