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

// Spectate mode as the area list sees it.
//
// /spectate restricts IC speech without touching the area's stored lock, so
// until this the lock ARUP reported the raw LockFree and every client's area
// list showed a watch-only room as FREE. The fix is a display derivation
// (areaLockDisplay) plus a broadcast on the toggle, and both halves are pinned
// here: the derivation for what it advertises and for what it must not change,
// the broadcast against the source, since nothing observes it without a server.
package athena

import (
	"os"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

func newSpectateTestArea() *area.Area {
	return area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
}

// TestAreaLockDisplayAdvertisesSpectateMode covers what the area list shows for
// each combination of stored lock and spectate mode.
func TestAreaLockDisplayAdvertisesSpectateMode(t *testing.T) {
	tests := []struct {
		name     string
		lock     area.Lock
		spectate bool
		want     area.Lock
	}{
		{"open area", area.LockFree, false, area.LockFree},
		// The bug: this used to report FREE, so nothing on the area list said
		// the room was watch-only.
		{"/spectate on an open area", area.LockFree, true, area.LockSpectatable},
		// LOCKED is a claim about entry, which spectate mode makes none about,
		// and it is the stronger of the two — so it wins.
		{"/spectate inside a locked area", area.LockLocked, true, area.LockLocked},
		{"locked area", area.LockLocked, false, area.LockLocked},
		{"/lock -s", area.LockSpectatable, false, area.LockSpectatable},
		{"/lock -s plus /spectate", area.LockSpectatable, true, area.LockSpectatable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newSpectateTestArea()
			a.SetLock(tt.lock)
			a.SetSpectateMode(tt.spectate)

			if got := areaLockDisplay(a); got != tt.want {
				t.Errorf("areaLockDisplay() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAreaLockDisplayLeavesTheStoredLockAlone is the safety half of the fix.
// The lock the area actually holds is what /unlock, the auto-unlock when the
// last CM leaves, and the LockSpectatable branches in CanSpeakIC /
// CanChangeMusic / CanJud all reason about — those gate on the /lock invite
// list, not the separate spectate-invite list, so advertising SPECTATABLE must
// stay a display derivation and never write the state back.
func TestAreaLockDisplayLeavesTheStoredLockAlone(t *testing.T) {
	a := newSpectateTestArea()
	a.SetSpectateMode(true)

	if got := areaLockDisplay(a); got != area.LockSpectatable {
		t.Fatalf("areaLockDisplay() = %v, want LockSpectatable", got)
	}
	if got := a.Lock(); got != area.LockFree {
		t.Errorf("Lock() = %v after areaLockDisplay, want LockFree (display only)", got)
	}
}

// TestSpectateToggleBroadcastsTheAreaList pins the half of the fix that has no
// observable surface without a full server: the derivation is worthless if the
// toggle never tells anyone, which is exactly how the bug shipped — /spectate
// changed the area's state and broadcast nothing, so every client's area list,
// the toggling CM's included, kept rendering the pre-toggle state until some
// unrelated event happened to trigger a lock ARUP.
func TestSpectateToggleBroadcastsTheAreaList(t *testing.T) {
	body := funcBodyFromSource(t, "commands_area_admin.go", "func cmdSpectate(")
	if !strings.Contains(body, "sendLockArup()") {
		t.Error("cmdSpectate does not call sendLockArup(): toggling spectate mode " +
			"changes what the area list should show (see areaLockDisplay), so the " +
			"toggle has to broadcast it or the list goes stale for everyone")
	}
}

// funcBodyFromSource returns the source text of the named function, from its
// declaration to the next top-level closing brace.
func funcBodyFromSource(t *testing.T, file, decl string) string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %v: %v", file, err)
	}
	lines := strings.Split(string(src), "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, decl) {
			start = i
			break
		}
	}
	if start == -1 {
		t.Fatalf("%v: %q not found", file, decl)
	}
	for i := start + 1; i < len(lines); i++ {
		if lines[i] == "}" {
			return strings.Join(lines[start:i+1], "\n")
		}
	}
	t.Fatalf("%v: no closing brace for %q", file, decl)
	return ""
}

// TestAutoUnlockIfLastCMGoneClearsSpectateMode covers the state the visible
// status would otherwise get stuck in. /spectate leaves the lock free, which is
// precisely what the old LockFree early-out treated as "nothing to unlock" —
// so a CM could enable spectate mode, disconnect, and leave everyone still in
// the room IC-silent with nobody holding the CM/mod permission /spectate needs.
func TestAutoUnlockIfLastCMGoneClearsSpectateMode(t *testing.T) {
	a := newSpectateTestArea()
	a.AddCM(1)
	a.SetSpectateMode(true)
	a.AddSpectateInvited(2)

	// Mirrors clientCleanup, which removes the CM before checking.
	a.RemoveCM(1)

	if !autoUnlockIfLastCMGone(a) {
		t.Fatal("expected spectate mode to be lifted when its last CM disconnected")
	}
	if a.SpectateMode() {
		t.Error("SpectateMode() = true, want false")
	}
	if a.HasSpectateInvited(2) {
		t.Error("expected the spectate-invite list to be cleared")
	}
	if got := a.Lock(); got != area.LockFree {
		t.Errorf("Lock() = %v, want LockFree", got)
	}
	if got := areaLockDisplay(a); got != area.LockFree {
		t.Errorf("areaLockDisplay() = %v, want LockFree — the area list must stop "+
			"showing SPECTATABLE once the mode is lifted", got)
	}
}

// TestAutoUnlockIfLastCMGoneKeepsSpectateModeWithAnotherCM verifies the lift is
// scoped to the last CM leaving, exactly as the lock half already is: a second
// CM is still there to manage the room.
func TestAutoUnlockIfLastCMGoneKeepsSpectateModeWithAnotherCM(t *testing.T) {
	a := newSpectateTestArea()
	a.AddCM(1)
	a.AddCM(2)
	a.SetSpectateMode(true)

	a.RemoveCM(1)

	if autoUnlockIfLastCMGone(a) {
		t.Fatal("expected spectate mode to survive while another CM is present")
	}
	if !a.SpectateMode() {
		t.Error("SpectateMode() = false, want true")
	}
}

// TestAutoUnlockIfLastCMGoneStillNoOpsOnAnOrdinaryOpenArea guards the widened
// guard: an open area with no spectate mode has nothing to lift, and must not
// start reporting that it did.
func TestAutoUnlockIfLastCMGoneStillNoOpsOnAnOrdinaryOpenArea(t *testing.T) {
	a := newSpectateTestArea()
	a.AddCM(1)
	a.RemoveCM(1)

	if autoUnlockIfLastCMGone(a) {
		t.Fatal("expected no-op on an area that was never locked or spectated")
	}
}
