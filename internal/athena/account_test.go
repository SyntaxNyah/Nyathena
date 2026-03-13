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

import "testing"

// TestFormatPlaytime verifies that formatPlaytime converts seconds to the
// expected human-readable string for all relevant cases.
func TestFormatPlaytime(t *testing.T) {
	cases := []struct {
		secs int64
		want string
	}{
		{-1, "less than a minute"},
		{0, "less than a minute"},
		{30, "0m"},                  // < 60s: minutes = 0
		{59, "0m"},                  // one second short of a minute
		{60, "1m"},                  // exactly one minute
		{90, "1m"},                  // 1m 30s → 1m (seconds truncated)
		{3600, "1h 0m"},             // exactly one hour
		{3661, "1h 1m"},             // 1h 1m 1s
		{7322, "2h 2m"},             // 2h 2m 2s
	}
	for _, tc := range cases {
		got := formatPlaytime(tc.secs)
		if got != tc.want {
			t.Errorf("formatPlaytime(%d) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}
