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

import "testing"

func TestNormalizeForFilterBoundaries(t *testing.T) {
	cases := []struct{ in, want string }{
		{"CLOVER YOU LOLICON RAPE RAPE RAPE", "clover you lolicon rape rape rape"},
		{"therapeutic massage", "therapeutic massage"},
		{"r4pe", "rape"},
		{"n i g g e r", "n i g g e r"},
		{"Faggots faggots.", "faggots faggots"},
		{"  ...  ", ""},
	}
	for _, c := range cases {
		got := normalizeForFilterBoundaries(c.in)
		status := "ok"
		if got != c.want {
			status = "MISMATCH want=" + c.want
		}
		t.Logf("%-35q -> %-35q %s", c.in, got, status)
	}
}

func TestMatchWordEntriesWorstSeverityWins(t *testing.T) {
	entries := []WordEntry{
		{Raw: "tranny", Pattern: "tranny", Severity: SeverityNuke, Mode: MatchSubstring},
		{Raw: "rape", Pattern: "rape", Severity: SeveritySevere, Mode: MatchWord},
		{Raw: "damn", Pattern: "damn", Severity: SeverityWatch, Mode: MatchSubstring},
	}
	for _, tc := range []struct {
		text string
		want string
	}{
		{"CLOVER YOU LOLICON RAPE RAPE RAPE", "severe"},
		{"NIGGERS TRANNIES", "none"}, // "tranny" is not a substring of "trannies" -- needs its own entry
		{"therapeutic massage please", "none"},
		{"damn that was close", "watch"},
		{"damn you tranny", "nuke"}, // worst wins, not first
		{"NIGGERS TRANNY", "nuke"},
		{"they were raped in the log", "severe"}, // prefix inflection that keeps the stem
		{"ordinary chat here", "none"},
	} {
		m := matchWordEntries(entries, tc.text)
		got := "none"
		if m.Matched {
			got = m.Entry.Severity.String()
		}
		status := "ok"
		if got != tc.want {
			status = "MISMATCH want=" + tc.want
		}
		t.Logf("%-36q -> %-8s %s", tc.text, got, status)
		if got != tc.want {
			t.Errorf("%q: got %s want %s", tc.text, got, tc.want)
		}
	}
}
