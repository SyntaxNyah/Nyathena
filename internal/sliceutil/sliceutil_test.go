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

package sliceutil

import "testing"

func TestContainsString(t *testing.T) {
	tests := []struct {
		name      string
		container []string
		value     string
		want      bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"empty string found", []string{"", "a"}, "", true},
		{"empty string not found", []string{"a", "b"}, "", false},
		{"single element match", []string{"x"}, "x", true},
		{"single element no match", []string{"x"}, "y", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsString(tt.container, tt.value)
			if got != tt.want {
				t.Errorf("ContainsString(%v, %q) = %v, want %v", tt.container, tt.value, got, tt.want)
			}
		})
	}
}

func TestContainsInt(t *testing.T) {
	tests := []struct {
		name      string
		container []int
		value     int
		want      bool
	}{
		{"found", []int{1, 2, 3}, 2, true},
		{"not found", []int{1, 2, 3}, 4, false},
		{"empty slice", []int{}, 0, false},
		{"nil slice", nil, 0, false},
		{"zero found", []int{0, 1}, 0, true},
		{"negative found", []int{-1, 0, 1}, -1, true},
		{"single element match", []int{5}, 5, true},
		{"single element no match", []int{5}, 6, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsInt(tt.container, tt.value)
			if got != tt.want {
				t.Errorf("ContainsInt(%v, %d) = %v, want %v", tt.container, tt.value, got, tt.want)
			}
		})
	}
}
