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

// Nyathena fork addition: tests for auto-unlocking an area when its last CM
// disconnects, while leaving it locked if another CM remains.

import (
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// TestAutoUnlockIfLastCMGoneUnlocksWhenNoCMsRemain verifies that a locked
// area with a single CM auto-unlocks (and clears its invite list) once that
// CM is gone.
func TestAutoUnlockIfLastCMGoneUnlocksWhenNoCMsRemain(t *testing.T) {
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	a.AddCM(1)
	a.SetLock(area.LockLocked)
	a.AddInvited(2)

	// Simulate the CM having already been removed (mirrors clientCleanup,
	// which calls RemoveCM before checking whether to auto-unlock).
	a.RemoveCM(1)

	if !autoUnlockIfLastCMGone(a) {
		t.Fatal("expected area to auto-unlock when its last CM is gone")
	}
	if a.Lock() != area.LockFree {
		t.Errorf("Lock() = %v, want LockFree", a.Lock())
	}
	if a.HasInvited(2) {
		t.Error("expected invite list to be cleared on auto-unlock")
	}
}

// TestAutoUnlockIfLastCMGoneStaysLockedWithAnotherCM verifies that an area
// stays locked if a second CM is still present after one CM leaves.
func TestAutoUnlockIfLastCMGoneStaysLockedWithAnotherCM(t *testing.T) {
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	a.AddCM(1)
	a.AddCM(2)
	a.SetLock(area.LockLocked)

	// Only one of the two CMs disconnects.
	a.RemoveCM(1)

	if autoUnlockIfLastCMGone(a) {
		t.Fatal("expected area to remain locked while another CM is present")
	}
	if a.Lock() != area.LockLocked {
		t.Errorf("Lock() = %v, want LockLocked", a.Lock())
	}
}

// TestAutoUnlockIfLastCMGoneNoOpWhenNotLocked verifies no state changes (and
// false is returned) when the area was never locked to begin with.
func TestAutoUnlockIfLastCMGoneNoOpWhenNotLocked(t *testing.T) {
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	a.AddCM(1)
	a.RemoveCM(1)

	if autoUnlockIfLastCMGone(a) {
		t.Fatal("expected no-op when area was never locked")
	}
	if a.Lock() != area.LockFree {
		t.Errorf("Lock() = %v, want LockFree", a.Lock())
	}
}

// TestAutoUnlockIfLastCMGoneRespectsAdminLock verifies that an /adminlock
// seal is left untouched — only an admin can lift that one.
func TestAutoUnlockIfLastCMGoneRespectsAdminLock(t *testing.T) {
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	a.AddCM(1)
	a.SetLock(area.LockLocked)
	a.SetAdminLocked(true)

	a.RemoveCM(1)

	if autoUnlockIfLastCMGone(a) {
		t.Fatal("expected admin-locked area to stay sealed")
	}
	if a.Lock() != area.LockLocked {
		t.Errorf("Lock() = %v, want LockLocked (admin seal preserved)", a.Lock())
	}
}
