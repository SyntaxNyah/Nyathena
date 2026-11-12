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

// CharsCheck coalescing.
//
// CharsCheck carries one entry per character -- about 5,000 on this server --
// and is broadcast to every member of the area on each character change. A
// script or a reconnect storm can produce hundreds of CCs in a minute; at ~14
// area members and ~5,000 entries that is tens of thousands of full-list sends
// and a large amount of serialization/queue pressure.
//
// The client only needs the latest availability list, not every intermediate
// one. Mark the area dirty and flush at most four times per second. A burst of
// 319 CCs in one minute becomes at most 240 broadcasts per area instead of
// 319 x area population.
package athena

import (
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/area"
)

const charsCheckDebounce = 250 * time.Millisecond

var (
	charsCheckDirty       sync.Map // map[*area.Area]struct{}
	charsCheckFlusherOnce sync.Once
)

// requestCharsCheck marks an area's character availability list dirty and
// ensures the flusher is running. Callers that previously broadcast CharsCheck
// directly should use this instead.
func requestCharsCheck(a *area.Area) {
	if a == nil {
		return
	}
	charsCheckDirty.Store(a, struct{}{})
	charsCheckFlusherOnce.Do(func() {
		go charsCheckFlusher()
	})
}

func charsCheckFlusher() {
	ticker := time.NewTicker(charsCheckDebounce)
	defer ticker.Stop()
	for range ticker.C {
		flushCharsCheck()
	}
}

// flushCharsCheck sends one CharsCheck per dirty area. Split out so tests can
// run it deterministically without sleeping.
func flushCharsCheck() {
	var dirty []*area.Area
	charsCheckDirty.Range(func(key, _ any) bool {
		if a, ok := key.(*area.Area); ok {
			dirty = append(dirty, a)
		}
		charsCheckDirty.Delete(key)
		return true
	})
	for _, a := range dirty {
		broadcastToAreaOnce(a, charsCheckPacket(a))
	}
}

// charsCheckPacket builds the full availability payload for one area.
func charsCheckPacket(a *area.Area) aolib.Outgoing {
	return &aolib.CharsCheck{Taken: charAvailabilities(a.Taken())}
}
