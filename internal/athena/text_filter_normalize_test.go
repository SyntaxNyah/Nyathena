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

// Bypass strings below are written as explicit \uXXXX / \UXXXXXXXX escapes
// (rather than literal glyphs) so the source stays unambiguous under any
// editor/encoding and the exact codepoints under test are visible in the diff.
func TestNormalizeForFilterDefeatsUnicodeBypass(t *testing.T) {
	const want = "nigger"
	cases := map[string]string{
		"mathematical bold script": "\U0001d4f7\U0001d4f2\U0001d4f0\U0001d4f0\U0001d4ee\U0001d4fb",
		"fullwidth":                "\uff4e\uff49\uff47\uff47\uff45\uff52",
		"zero-width inserted":      "n\u200bi\u200bg\u200bg\u200be\u200br",
		"combining marks inserted": "ni\u0301gge\u0301r",
	}
	for name, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("%s: normalizeForFilter(%q) = %q, want %q", name, input, got, want)
		}
	}
}

func TestNormalizeForFilterCyrillicHomoglyphs(t *testing.T) {
	// Cyrillic ES/A/TE look identical to Latin "cat" but are distinct codepoints.
	input := "\u0441\u0430\u0442"
	if got, want := normalizeForFilter(input), "cat"; got != want {
		t.Errorf("normalizeForFilter(%q) = %q, want %q", input, got, want)
	}
}

func TestNormalizeForFilterPlainASCIIUnaffected(t *testing.T) {
	if got, want := normalizeForFilter("Hello World"), "hello world"; got != want {
		t.Errorf("normalizeForFilter(%q) = %q, want %q", "Hello World", got, want)
	}
}

func TestMatchBannedWordCatchesUnicodeBypass(t *testing.T) {
	orig := getBannedWords()
	defer setBannedWords(orig)
	setBannedWords([]string{normalizeForFilter("nigger")})

	bypasses := []string{
		"\U0001d4f7\U0001d4f2\U0001d4f0\U0001d4f0\U0001d4ee\U0001d4fb", // mathematical bold script
		"\uff4e\uff49\uff47\uff47\uff45\uff52",                         // fullwidth
		"n\u200bi\u200bg\u200bg\u200be\u200br",                         // zero-width inserted
	}
	for _, msg := range bypasses {
		normalized := normalizeForFilter(msg)
		if _, ok := matchBannedWord(normalized); !ok {
			t.Errorf("matchBannedWord failed to catch bypass %q (normalized: %q)", msg, normalized)
		}
	}
}
