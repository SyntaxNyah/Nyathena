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

func TestContainsStringCaseInsensitive(t *testing.T) {
	tests := []struct {
		name      string
		container []string
		value     string
		expected  bool
	}{
		{
			name:      "Exact match",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "Ace Attorney/Prelude/[AA] Opening.opus",
			expected:  true,
		},
		{
			name:      "Lowercase value, mixed case container",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "ace attorney/prelude/[aa] opening.opus",
			expected:  true,
		},
		{
			name:      "Uppercase value, mixed case container",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "ACE ATTORNEY/PRELUDE/[AA] OPENING.OPUS",
			expected:  true,
		},
		{
			name:      "Not found",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "Nonexistent Song.opus",
			expected:  false,
		},
		{
			name:      "Empty container",
			container: []string{},
			value:     "anything",
			expected:  false,
		},
		{
			name:      "Category name - exact match",
			container: []string{"Prelude", "Trial", "Questioning"},
			value:     "Trial",
			expected:  true,
		},
		{
			name:      "Category name - case insensitive",
			container: []string{"Prelude", "Trial", "Questioning"},
			value:     "TRIAL",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsStringCaseInsensitive(tt.container, tt.value)
			if result != tt.expected {
				t.Errorf("ContainsStringCaseInsensitive(%v, %q) = %v, want %v", tt.container, tt.value, result, tt.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	// Verify the original function still works for exact matches
	tests := []struct {
		name      string
		container []string
		value     string
		expected  bool
	}{
		{
			name:      "Exact match",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "Ace Attorney/Prelude/[AA] Opening.opus",
			expected:  true,
		},
		{
			name:      "Case mismatch - should NOT match",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "ace attorney/prelude/[aa] opening.opus",
			expected:  false,
		},
		{
			name:      "Not found",
			container: []string{"Ace Attorney/Prelude/[AA] Opening.opus", "Trial"},
			value:     "Nonexistent Song.opus",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsString(tt.container, tt.value)
			if result != tt.expected {
				t.Errorf("ContainsString(%v, %q) = %v, want %v", tt.container, tt.value, result, tt.expected)
			}
		})
	}
}
