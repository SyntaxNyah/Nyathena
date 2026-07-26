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
	"fmt"
	"testing"
	"time"
)

func TestJoin(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// Add a new client with CharID 0.
	// They should be the only one there.
	if !a.AddChar(0) {
		t.Errorf("adding new player to area: got %t, want %t", false, true)
	}
	if a.players != 1 {
		t.Errorf("unexpected value for area player count, got %d, want %d", a.players, 1)
	}

	// A second client with CharID 1 joins.
	// There are now two clients.
	if !a.AddChar(1) {
		t.Errorf("adding new player to area: got %t, want %t", false, true)
	}
	if a.players != 2 {
		t.Errorf("unexpected value for area player count, got %d, want %d", a.players, 2)
	}

	// A third client attempts to join with CharID 0.
	// Since a player has already taken that, it should fail.
	if a.AddChar(0) {
		t.Errorf("adding invalid player to area: got %t, want %t", true, false)
	}
	if a.players != 2 {
		t.Errorf("unexpected value for area player count, got %d, want %d", a.players, 2)
	}

	// The client with CharID 0 leaves.
	// Another client can now join with CharID 0.
	a.RemoveChar(0)
	if !a.AddChar(0) {
		t.Errorf("adding new player to area: got %t, want %t", false, true)
	}

	// A spectator joins the area.
	if !a.AddChar(-1) {
		t.Errorf("adding spectator to area: got %t, want %t", false, true)
	}
	if a.players != 3 {
		t.Errorf("unexpected value for area player count, got %d, want %d", a.players, 3)
	}
}

func TestSwitch(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// A client with CharID 0 joins, and switches to CharID 1.
	a.AddChar(0)
	if !a.SwitchChar(0, 1) {
		t.Errorf("switching empty character, got %t, want %t", false, true)
	}
	if a.AddChar(1) {
		t.Errorf("adding invalid player to area: got %t, want %t", true, false)
	}

	// A client with CharID 0 joins.
	if !a.AddChar(0) {
		t.Errorf("adding new player to area: got %t, want %t", false, true)
	}
	if a.SwitchChar(0, 1) {
		t.Errorf("switching taken character, got %t, want %t", true, false)
	}

	// A spectator joins
	a.AddChar(-1)
	if a.SwitchChar(-1, 0) {
		t.Errorf("switching taken character, got %t, want %t", true, false)
	}
}

func TestEvidence(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// Two pieces of evidence are added.
	evi1 := "foo&foo&foo"
	evi2 := "bar&bar&bar"
	a.AddEvidence(evi1)
	a.AddEvidence(evi2)
	if len(a.evidence) != 2 {
		t.Errorf("unexpected value for evidence length, got %d, want %d", len(a.evidence), 2)
	}

	// Evidence at indexes 0 and 1 are swapped.
	a.SwapEvidence(0, 1)
	if a.evidence[0] != evi2 {
		t.Errorf("unexpected value for evidence[0], got %s, want %s", a.evidence[0], evi2)
	}
	if a.evidence[1] != evi1 {
		t.Errorf("unexpected value for evidence[1], got %s, want %s", a.evidence[1], evi1)
	}

	// Evidence at index 0 is edited
	evi3 := "foobar&foobar&foobar"
	a.EditEvidence(0, evi3)
	if a.evidence[0] != evi3 {
		t.Errorf("unexpected value for evidence[0], got %s, want %s", a.evidence[0], evi3)
	}

	// Evidence at index 0 is removed.
	a.RemoveEvidence(0)
	if len(a.evidence) != 1 {
		t.Errorf("unexpected value for evidence length, got %d, want %d", len(a.evidence), 1)
	}
	if a.evidence[0] != evi1 {
		t.Errorf("unexpected value for evidence[0], got %s, want %s", a.evidence[0], evi1)
	}
}

// Out-of-range indexes (negative, or one past the last valid index) must be
// rejected rather than panicking on the underlying slice.
func TestEvidenceOutOfRangeIndexes(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.AddEvidence("foo&foo&foo")
	a.AddEvidence("bar&bar&bar")

	if ok := a.SwapEvidence(-1, 0); ok {
		t.Errorf("SwapEvidence(-1, 0) = true, want false")
	}
	if ok := a.SwapEvidence(0, -1); ok {
		t.Errorf("SwapEvidence(0, -1) = true, want false")
	}
	if ok := a.SwapEvidence(len(a.evidence), 0); ok {
		t.Errorf("SwapEvidence(len, 0) = true, want false")
	}

	a.EditEvidence(-1, "should not panic")
	a.EditEvidence(len(a.evidence), "should not panic")

	a.RemoveEvidence(-1)
	a.RemoveEvidence(len(a.evidence))
	if len(a.evidence) != 2 {
		t.Errorf("out-of-range RemoveEvidence calls mutated evidence, got length %d, want %d", len(a.evidence), 2)
	}
}

func TestCMs(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// New CM is added to the area.
	if !a.AddCM(0) {
		t.Errorf("adding new cm to area: got %t, want %t", false, true)
	}
	if !a.HasCM(0) {
		t.Errorf("checking HasCM for joined cm: got %t, want %t", false, true)
	}

	// Adding CM with same UID should fail.
	if a.AddCM(0) {
		t.Errorf("adding invalid cm to area: got %t, want %t", true, false)
	}

	// CM is removed
	if !a.RemoveCM(0) {
		t.Errorf("removing cm from area: got %t, want %t", false, true)
	}
	if a.HasCM(0) {
		t.Errorf("checking HasCM for invalid cm: got %t, want %t", true, false)
	}
}

func TestInvited(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)

	// New user is added to invite list
	if !a.AddInvited(1) {
		t.Errorf("adding new user to area invited: got %t, want %t", false, true)
	}
	if !a.HasInvited(1) {
		t.Errorf("checking HasInvited for joined user: got %t, want %t", false, true)
	}

	// Adding invite with same UID should fail
	if a.AddInvited(1) {
		t.Errorf("adding invalid user to area invited: got %t, want %t", true, false)
	}

	// Remove invited user
	if !a.RemoveInvited(1) {
		t.Errorf("removing user to area invited: got %t, want %t", false, true)
	}
	if len(a.invited) != 0 {
		t.Errorf("unexpected value for invited length, got %d, want %d", len(a.invited), 0)
	}
}

// TestPunishmentSafe verifies the antipunish TOML field seeds PunishmentSafe()
// and that SetPunishmentSafe toggles it at runtime, mirroring DokiArea/
// SetDokiArea.
func TestPunishmentSafe(t *testing.T) {
	a := NewArea(AreaData{Name: "Safe Zone", Antipunish: true}, 5, 0, EviAny)
	if !a.PunishmentSafe() {
		t.Errorf("expected antipunish = true in AreaData to seed PunishmentSafe() true")
	}

	b := NewArea(AreaData{Name: "Normal Zone"}, 5, 0, EviAny)
	if b.PunishmentSafe() {
		t.Errorf("expected PunishmentSafe() to default false when antipunish is omitted")
	}

	b.SetPunishmentSafe(true)
	if !b.PunishmentSafe() {
		t.Errorf("SetPunishmentSafe(true) did not take effect")
	}
	b.SetPunishmentSafe(false)
	if b.PunishmentSafe() {
		t.Errorf("SetPunishmentSafe(false) did not take effect")
	}
}

// TestReportBufferRetainsBeyondFixedCount verifies the /modcall report
// buffer no longer rotates out on a fixed entry count: with no cap (0), an
// area busy enough to log more lines than the old default (150) in quick
// succession must still retain all of them, since none are older than the
// 24-hour window.
func TestReportBufferRetainsBeyondFixedCount(t *testing.T) {
	a := NewArea(AreaData{Name: "Busy Courtroom"}, 5, 0, EviAny)
	const lines = 300
	for i := 0; i < lines; i++ {
		a.UpdateBuffer(fmt.Sprintf("line %d", i))
	}
	got := a.Buffer()
	if len(got) != lines {
		t.Fatalf("Buffer() returned %d lines, want %d (fixed-size rotation should no longer apply)", len(got), lines)
	}
	if got[0] != "line 0" || got[lines-1] != fmt.Sprintf("line %d", lines-1) {
		t.Errorf("Buffer() did not preserve chronological order: first=%q last=%q", got[0], got[lines-1])
	}
}

// TestReportBufferPrunesOldEntries verifies entries older than
// reportBufferWindow (24h) are dropped on the next write, while recent
// entries survive -- the core fix for mod call reports not actually dating
// back 24 hours.
func TestReportBufferPrunesOldEntries(t *testing.T) {
	a := NewArea(AreaData{Name: "Stale Courtroom"}, 5, 0, EviAny)
	a.buffer = append(a.buffer,
		reportLine{text: "ancient", at: time.Now().Add(-25 * time.Hour)},
		reportLine{text: "recent", at: time.Now().Add(-1 * time.Hour)},
	)

	// A fresh write triggers the prune pass.
	a.UpdateBuffer("newest")

	got := a.Buffer()
	for _, line := range got {
		if line == "ancient" {
			t.Errorf("Buffer() still contains an entry older than reportBufferWindow: %v", got)
		}
	}
	if len(got) != 2 || got[0] != "recent" || got[1] != "newest" {
		t.Errorf("Buffer() = %v, want [recent newest]", got)
	}
}

// TestReportBufferSafetyCap verifies bufCap (log_buffer_size) still bounds
// memory when explicitly set, even for entries within the 24-hour window.
func TestReportBufferSafetyCap(t *testing.T) {
	a := NewArea(AreaData{Name: "Capped Courtroom"}, 5, 2, EviAny)
	a.UpdateBuffer("first")
	a.UpdateBuffer("second")
	a.UpdateBuffer("third")

	got := a.Buffer()
	if len(got) != 2 {
		t.Fatalf("Buffer() returned %d lines, want 2 (bufCap=2 should still apply)", len(got))
	}
	if got[0] != "second" || got[1] != "third" {
		t.Errorf("Buffer() = %v, want [second third]", got)
	}
}
