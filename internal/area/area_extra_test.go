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

package area

import (
	"testing"
)

// TestHP verifies the HP getter and setter, including boundary and invalid values.
func TestHP(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// Initial values should be 10/10.
	def, pro := a.HP()
	if def != 10 || pro != 10 {
		t.Errorf("initial HP: got def=%d pro=%d, want 10/10", def, pro)
	}

	// Set def bar (1) to 5.
	if !a.SetHP(1, 5) {
		t.Error("SetHP(1, 5) returned false, want true")
	}
	def, _ = a.HP()
	if def != 5 {
		t.Errorf("after SetHP(1,5) defhp = %d, want 5", def)
	}

	// Set pro bar (2) to 3.
	if !a.SetHP(2, 3) {
		t.Error("SetHP(2, 3) returned false, want true")
	}
	_, pro = a.HP()
	if pro != 3 {
		t.Errorf("after SetHP(2,3) prohp = %d, want 3", pro)
	}

	// Boundary: value 0 is valid.
	if !a.SetHP(1, 0) {
		t.Error("SetHP(1, 0) returned false, want true")
	}
	// Boundary: value 10 is valid.
	if !a.SetHP(2, 10) {
		t.Error("SetHP(2, 10) returned false, want true")
	}

	// Out-of-range values should fail.
	if a.SetHP(1, 11) {
		t.Error("SetHP(1, 11) returned true, want false")
	}
	if a.SetHP(2, -1) {
		t.Error("SetHP(2, -1) returned true, want false")
	}

	// Invalid bar number.
	if a.SetHP(0, 5) {
		t.Error("SetHP(0, 5) returned true, want false")
	}
	if a.SetHP(3, 5) {
		t.Error("SetHP(3, 5) returned true, want false")
	}
}

// TestStatus verifies the status getter and setter.
func TestStatus(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	if a.Status() != StatusIdle {
		t.Errorf("initial Status = %d, want StatusIdle", a.Status())
	}
	a.SetStatus(StatusCasing)
	if a.Status() != StatusCasing {
		t.Errorf("after SetStatus(Casing), Status = %d, want StatusCasing", a.Status())
	}
}

// TestLock verifies the lock getter and setter.
func TestLock(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	if a.Lock() != LockFree {
		t.Errorf("initial Lock = %d, want LockFree", a.Lock())
	}
	a.SetLock(LockLocked)
	if a.Lock() != LockLocked {
		t.Errorf("after SetLock(Locked), Lock = %d, want LockLocked", a.Lock())
	}
	a.SetLock(LockSpectatable)
	if a.Lock() != LockSpectatable {
		t.Errorf("after SetLock(Spectatable), Lock = %d, want LockSpectatable", a.Lock())
	}
}

// TestBackground verifies the background getter and setter.
func TestBackground(t *testing.T) {
	a := NewArea(AreaData{Bg: "courtroom"}, 10, 0, EviAny)
	if a.Background() != "courtroom" {
		t.Errorf("Background() = %q, want \"courtroom\"", a.Background())
	}
	a.SetBackground("prison")
	if a.Background() != "prison" {
		t.Errorf("after SetBackground, Background() = %q, want \"prison\"", a.Background())
	}
}

// TestIsTaken verifies the IsTaken method for occupied and free slots.
func TestIsTaken(t *testing.T) {
	a := NewArea(AreaData{}, 10, 0, EviAny)
	a.AddChar(3)

	if !a.IsTaken(3) {
		t.Error("IsTaken(3) = false, want true after AddChar(3)")
	}
	if a.IsTaken(4) {
		t.Error("IsTaken(4) = true, want false for untaken slot")
	}
	// Spectator slot (-1) is never "taken".
	if a.IsTaken(-1) {
		t.Error("IsTaken(-1) = true, want false")
	}
}

// TestBuffer verifies that UpdateBuffer rotates the ring buffer and
// that Buffer returns only non-empty entries.
func TestBuffer(t *testing.T) {
	a := NewArea(AreaData{}, 10, 3, EviAny)

	// Buffer is initially all empty strings → Buffer() returns nothing.
	if len(a.Buffer()) != 0 {
		t.Errorf("initial Buffer() len = %d, want 0", len(a.Buffer()))
	}

	a.UpdateBuffer("line1")
	a.UpdateBuffer("line2")

	buf := a.Buffer()
	if len(buf) != 2 {
		t.Fatalf("Buffer() len = %d, want 2", len(buf))
	}
	if buf[0] != "line1" || buf[1] != "line2" {
		t.Errorf("Buffer() = %v, want [line1 line2]", buf)
	}

	// Overflow: adding a fourth entry drops the oldest.
	a.UpdateBuffer("line3")
	a.UpdateBuffer("line4")
	buf = a.Buffer()
	// Buffer capacity is 3, so after 4 pushes we have [line2 line3 line4].
	if len(buf) != 3 {
		t.Fatalf("Buffer() len after overflow = %d, want 3", len(buf))
	}
	if buf[0] != "line2" {
		t.Errorf("buf[0] = %q, want \"line2\"", buf[0])
	}
	if buf[2] != "line4" {
		t.Errorf("buf[2] = %q, want \"line4\"", buf[2])
	}
}

// TestReset verifies that Reset restores all area settings to their defaults.
func TestReset(t *testing.T) {
	data := AreaData{
		Bg:           "default_bg",
		Allow_iniswap: true,
		Allow_cms:    true,
	}
	a := NewArea(data, 50, 0, EviAny)

	// Mutate several fields.
	a.SetBackground("custom_bg")
	a.SetStatus(StatusGaming)
	a.SetLock(LockLocked)
	a.AddCM(1)
	a.AddInvited(2)
	a.AddEvidence("evi1")
	a.SetHP(1, 3)
	a.SetHP(2, 4)

	a.Reset()

	if a.Status() != StatusIdle {
		t.Errorf("after Reset, Status = %d, want StatusIdle", a.Status())
	}
	if a.Lock() != LockFree {
		t.Errorf("after Reset, Lock = %d, want LockFree", a.Lock())
	}
	if a.Background() != "default_bg" {
		t.Errorf("after Reset, Background = %q, want \"default_bg\"", a.Background())
	}
	if len(a.Evidence()) != 0 {
		t.Errorf("after Reset, evidence len = %d, want 0", len(a.Evidence()))
	}
	if len(a.CMs()) != 0 {
		t.Errorf("after Reset, CMs len = %d, want 0", len(a.CMs()))
	}
	if len(a.Invited()) != 0 {
		t.Errorf("after Reset, Invited len = %d, want 0", len(a.Invited()))
	}
	def, pro := a.HP()
	if def != 10 || pro != 10 {
		t.Errorf("after Reset, HP = %d/%d, want 10/10", def, pro)
	}
}

// TestPlayerCount verifies PlayerCount and VisiblePlayerCount.
func TestPlayerCount(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	a.AddChar(0)
	a.AddChar(1)
	if a.PlayerCount() != 2 {
		t.Errorf("PlayerCount() = %d, want 2", a.PlayerCount())
	}

	a.AddVisiblePlayer()
	a.AddVisiblePlayer()
	if a.VisiblePlayerCount() != 2 {
		t.Errorf("VisiblePlayerCount() = %d, want 2", a.VisiblePlayerCount())
	}
	a.RemoveVisiblePlayer()
	if a.VisiblePlayerCount() != 1 {
		t.Errorf("VisiblePlayerCount() after remove = %d, want 1", a.VisiblePlayerCount())
	}
}

// TestLastSpeaker verifies the last-speaker getter and setter.
func TestLastSpeaker(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	if a.LastSpeaker() != -1 {
		t.Errorf("initial LastSpeaker = %d, want -1", a.LastSpeaker())
	}
	a.SetLastSpeaker(7)
	if a.LastSpeaker() != 7 {
		t.Errorf("after SetLastSpeaker(7), LastSpeaker = %d, want 7", a.LastSpeaker())
	}
}

// TestSpectateInvited verifies the spectate-invite list operations.
func TestSpectateInvited(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	if !a.AddSpectateInvited(10) {
		t.Error("AddSpectateInvited(10) = false, want true")
	}
	if a.AddSpectateInvited(10) {
		t.Error("duplicate AddSpectateInvited(10) = true, want false")
	}
	if !a.HasSpectateInvited(10) {
		t.Error("HasSpectateInvited(10) = false after add")
	}
	if !a.RemoveSpectateInvited(10) {
		t.Error("RemoveSpectateInvited(10) = false, want true")
	}
	if a.HasSpectateInvited(10) {
		t.Error("HasSpectateInvited(10) = true after removal")
	}
	if a.RemoveSpectateInvited(99) {
		t.Error("RemoveSpectateInvited(99) = true, want false for non-existent UID")
	}
}

// TestSpectateMode verifies SetSpectateMode clears the invite list on disable.
func TestSpectateMode(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.SetSpectateMode(true)
	if !a.SpectateMode() {
		t.Error("SpectateMode() = false after SetSpectateMode(true)")
	}
	a.AddSpectateInvited(1)
	a.SetSpectateMode(false)
	if a.SpectateMode() {
		t.Error("SpectateMode() = true after SetSpectateMode(false)")
	}
	// The spectate invite list should be cleared when mode is turned off.
	if a.HasSpectateInvited(1) {
		t.Error("spectate invited list not cleared when mode disabled")
	}
}

// TestClearInvited verifies that ClearInvited removes all entries.
func TestClearInvited(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.AddInvited(1)
	a.AddInvited(2)
	a.ClearInvited()
	if len(a.Invited()) != 0 {
		t.Errorf("after ClearInvited, Invited len = %d, want 0", len(a.Invited()))
	}
}

// TestSwapEvidenceOutOfBounds verifies that SwapEvidence returns false when an
// index is out of range.
func TestSwapEvidenceOutOfBounds(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.AddEvidence("only")
	// Valid swap of a single-element list with itself.
	if !a.SwapEvidence(0, 0) {
		t.Error("SwapEvidence(0,0) on 1-element slice returned false")
	}
	// Out-of-bounds swap should return false.
	if a.SwapEvidence(0, 1) {
		t.Error("SwapEvidence(0,1) on 1-element slice returned true, want false")
	}
}

// TestTaken verifies the Taken helper returns "-1" for taken and "0" for free.
func TestTaken(t *testing.T) {
	a := NewArea(AreaData{}, 3, 0, EviAny)
	a.AddChar(1)

	taken := a.Taken()
	if len(taken) != 3 {
		t.Fatalf("Taken() len = %d, want 3", len(taken))
	}
	if taken[0] != "0" {
		t.Errorf("taken[0] = %q, want \"0\"", taken[0])
	}
	if taken[1] != "-1" {
		t.Errorf("taken[1] = %q, want \"-1\"", taken[1])
	}
	if taken[2] != "0" {
		t.Errorf("taken[2] = %q, want \"0\"", taken[2])
	}
}

// TestSwitchCharToSpectator verifies switching from a real character to -1 (spectator).
func TestSwitchCharToSpectator(t *testing.T) {
	a := NewArea(AreaData{}, 10, 0, EviAny)
	a.AddChar(2)
	if !a.SwitchChar(2, -1) {
		t.Error("SwitchChar(2,-1) returned false, want true")
	}
	// Slot 2 should now be free.
	if a.IsTaken(2) {
		t.Error("slot 2 still taken after switching to spectator")
	}
}

// TestEvidenceModeGetSet verifies the evidence-mode accessors.
func TestEvidenceModeGetSet(t *testing.T) {
	a := NewArea(AreaData{}, 10, 0, EviAny)
	if a.EvidenceMode() != EviAny {
		t.Errorf("initial EvidenceMode = %d, want EviAny", a.EvidenceMode())
	}
	a.SetEvidenceMode(EviMods)
	if a.EvidenceMode() != EviMods {
		t.Errorf("after SetEvidenceMode(EviMods), EvidenceMode = %d, want EviMods", a.EvidenceMode())
	}
}
