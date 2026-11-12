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
	"time"
)

// raidGuardSystem is the single owner of the anti-raid state that is not
// per-connection. Packet hooks, admission checks and staff status all consult
// this value rather than a scattering of package globals.
//
// The per-connection evidence remains on raidState because it must die with the
// socket. This value owns the cross-connection evidence: correlation, arrival
// bursts, the modcall window, and the under-attack timer. The repeat ledger and
// playtime cache move behind the same value in the next step.
type raidGuardSystem struct {
	corr     *raidCorrelation
	ledger   *raidLedgerStore
	playtime *playtimeStore
}

// raidCorrelation groups the layer-2 windows. It is replaced wholesale on a
// test reset; production initialises it once at startup.
type raidCorrelation struct {
	once        sync.Once
	window      *CorrelationWindow
	echo        *echoWindow
	modcalls    *modcallWindow
	arrivals    *arrivalWindow
	attackUntil atomic.Int64 // unix nanos; server is "under attack" until then
}

func newRaidCorrelation() *raidCorrelation {
	return &raidCorrelation{
		modcalls: newModcallWindow(4096),
		arrivals: newArrivalWindow(time.Second),
	}
}

func newRaidGuardSystem() *raidGuardSystem {
	return &raidGuardSystem{
		corr:     newRaidCorrelation(),
		ledger:   newRaidLedgerStore(),
		playtime: newPlaytimeStore(),
	}
}

var raidSystem = newRaidGuardSystem()
