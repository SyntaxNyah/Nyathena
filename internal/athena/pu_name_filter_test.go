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
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// withWordEntries installs a tiered word list for the duration of a test and
// restores whatever was there before.
func withWordEntries(t *testing.T, entries []WordEntry) {
	t.Helper()
	prevTiered := getWordEntries()
	prevFlat := getBannedWords()
	setWordEntries(entries)
	setBannedWords(nil)
	t.Cleanup(func() {
		setWordEntries(prevTiered)
		setBannedWords(prevFlat)
	})
}

func TestPUNameFilterDropsMatchingNames(t *testing.T) {
	withWordEntries(t, []WordEntry{
		{Raw: "slurword", Pattern: "slurword", Severity: SeverityDefault, Mode: MatchSubstring},
	})

	for _, tc := range []struct {
		name  string
		pu    *packet.PU
		allow bool
	}{
		{"clean showname", &packet.PU{ID: 1, Type: puTypeShowname, Data: "Phoenix Wright"}, true},
		{"clean OOC name", &packet.PU{ID: 1, Type: puTypeOOCName, Data: "someone"}, true},
		{"dirty showname", &packet.PU{ID: 1, Type: puTypeShowname, Data: "a slurword here"}, false},
		{"dirty OOC name", &packet.PU{ID: 1, Type: puTypeOOCName, Data: "slurword"}, false},
		// Type 1 is the character name, drawn from characters.txt, and type 3 is
		// an area index. Neither is player-supplied text, so neither is filtered
		// -- a character legitimately named something awkward must not vanish.
		{"character name is never filtered", &packet.PU{ID: 1, Type: 1, Data: "slurword"}, true},
		{"area index is never filtered", &packet.PU{ID: 1, Type: 3, Data: "4"}, true},
		{"empty name", &packet.PU{ID: 1, Type: puTypeShowname, Data: ""}, true},
	} {
		if got := nameAllowedInPU(tc.pu); got != tc.allow {
			t.Errorf("%s: nameAllowedInPU = %v, want %v", tc.name, got, tc.allow)
		}
	}
}

// TestPUNameFilterCatchesEvasion is the point of routing this through the same
// matcher the message filter uses rather than a plain string compare: a name is
// exactly as easy to stylize as a message, and a filter that only caught the
// literal spelling would be trivially walked around.
func TestPUNameFilterCatchesEvasion(t *testing.T) {
	withWordEntries(t, []WordEntry{
		{Raw: "slurword", Pattern: "slurword", Severity: SeverityDefault, Mode: MatchSubstring},
	})
	for _, name := range []string{
		"s l u r w o r d",
		"s.l.u.r.w.o.r.d",
		"5lurw0rd",
		"SLURWORD",
		"ｓｌｕｒｗｏｒｄ", // fullwidth
	} {
		if nameAllowedInPU(&packet.PU{ID: 1, Type: puTypeShowname, Data: name}) {
			t.Errorf("name %q was allowed through; the PU filter must normalize exactly like the message filter", name)
		}
	}

	// Letter stuffing is caught only where the entry already carries the doubled
	// letter, because normalization collapses a run of 3+ down to TWO rather
	// than one -- collapsing to one would shrink "nigger" to "niger", a
	// substring of "nigeria". So "slurrrrword" normalizes to "slurrword" and
	// does NOT match the single-r entry above; an operator who wants that form
	// lists it. Asserted rather than left implicit, since it is the kind of gap
	// worth knowing about when writing a list.
	if !nameAllowedInPU(&packet.PU{ID: 1, Type: puTypeShowname, Data: "slurrrrword"}) {
		t.Error("stuffing collapsed all the way to a single letter; the 3+-to-2 rule has changed and " +
			"entries like \"nigger\" would now shrink into common-word collisions")
	}
	withWordEntries(t, []WordEntry{
		{Raw: "slurrword", Pattern: "slurrword", Severity: SeverityDefault, Mode: MatchSubstring},
	})
	if nameAllowedInPU(&packet.PU{ID: 1, Type: puTypeShowname, Data: "slurrrrrrword"}) {
		t.Error("stuffing was not collapsed at all against a doubled-letter entry")
	}
}

// TestPUFilterPassesNonPUPackets keeps the choke point honest: it sits on a path
// that carries ARUP, PR and everything else broadcast server-wide, and must be
// invisible to all of it.
func TestPUFilterPassesNonPUPackets(t *testing.T) {
	withWordEntries(t, []WordEntry{
		{Raw: "slurword", Pattern: "slurword", Severity: SeverityNuke, Mode: MatchSubstring},
	})
	for _, p := range []packet.Outgoing{
		&packet.CTToClient{Name: "slurword", Message: "slurword", IsFromServer: "1"},
		&packet.PR{ID: 1, Type: 0},
	} {
		if !puAllowed(p) {
			t.Errorf("%T was blocked by the PU name filter; it must only ever act on PU", p)
		}
	}
}

// TestPUFilterInertWithNoWordList checks the cost claim: with nothing loaded the
// filter must not even reach the matcher, so a server that never writes a word
// list pays nothing for this path existing.
func TestPUFilterInertWithNoWordList(t *testing.T) {
	withWordEntries(t, nil)
	if !nameAllowedInPU(&packet.PU{ID: 1, Type: puTypeShowname, Data: "anything at all"}) {
		t.Error("a name was blocked with an empty word list")
	}
}
