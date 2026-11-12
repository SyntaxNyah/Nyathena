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
// one. The first change after an idle period is sent immediately, so normal
// play and opening char select stay instant; changes that arrive within the
// 250 ms window are coalesced into one trailing update. A burst of 319 CCs in
// one minute therefore costs at most one immediate plus a handful of trailing
// broadcasts per area, instead of 319 x area population.
package athena

import (
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/area"
)

const charsCheckDebounce = 250 * time.Millisecond

type charsCheckState struct {
	mu    sync.Mutex
	last  time.Time
	dirty bool
}

var (
	charsCheckStates      sync.Map // map[*area.Area]*charsCheckState
	charsCheckFlusherOnce sync.Once
)

// requestCharsCheck broadcasts immediately when the area has not sent a
// CharsCheck recently, otherwise marks it for the trailing flush. Callers that
// previously broadcast CharsCheck directly should use this instead.
func requestCharsCheck(a *area.Area) {
	if a == nil {
		return
	}
	st := charsCheckStateFor(a)
	st.mu.Lock()
	now := time.Now()
	if st.last.IsZero() || now.Sub(st.last) >= charsCheckDebounce {
		st.last = now
		st.dirty = false
		st.mu.Unlock()
		broadcastToAreaOnce(a, charsCheckPacket(a))
		return
	}
	st.dirty = true
	st.mu.Unlock()

	charsCheckFlusherOnce.Do(func() {
		go charsCheckFlusher()
	})
}

func charsCheckStateFor(a *area.Area) *charsCheckState {
	if v, ok := charsCheckStates.Load(a); ok {
		return v.(*charsCheckState)
	}
	v, _ := charsCheckStates.LoadOrStore(a, &charsCheckState{})
	return v.(*charsCheckState)
}

func charsCheckFlusher() {
	ticker := time.NewTicker(charsCheckDebounce)
	defer ticker.Stop()
	for range ticker.C {
		flushCharsCheck()
	}
}

// flushCharsCheck sends one trailing CharsCheck per dirty area. Split out so
// tests can run it deterministically without sleeping.
func flushCharsCheck() {
	charsCheckStates.Range(func(key, value any) bool {
		a, ok := key.(*area.Area)
		st, ok2 := value.(*charsCheckState)
		if !ok || !ok2 {
			return true
		}
		st.mu.Lock()
		if !st.dirty {
			st.mu.Unlock()
			return true
		}
		st.dirty = false
		st.last = time.Now()
		st.mu.Unlock()
		broadcastToAreaOnce(a, charsCheckPacket(a))
		return true
	})
}

// charsCheckPacket builds the full availability payload for one area.
func charsCheckPacket(a *area.Area) aolib.Outgoing {
	return &aolib.CharsCheck{Taken: charAvailabilities(a.Taken())}
}
