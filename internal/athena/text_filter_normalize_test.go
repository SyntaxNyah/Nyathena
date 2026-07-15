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

import (
	"os"
	"testing"
)

// Bypass strings below are written as explicit \uXXXX / \UXXXXXXXX escapes
// (rather than literal glyphs) so the source stays unambiguous under any
// editor/encoding and the exact codepoints under test are visible in the diff.
//
// normalizeForFilter collapses consecutive duplicate letters (to defeat
// letter-stuffing evasion), so "nigger" itself normalizes to "niger" (the
// double g collapses). Every case below is expected to land on that same
// normalized form.
func TestNormalizeForFilterDefeatsUnicodeBypass(t *testing.T) {
	const want = "niger"
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

func TestNormalizeForFilterArmenianAndCherokeeHomoglyphs(t *testing.T) {
	if got, want := normalizeForFilter("\u0570"), "h"; got != want {
		t.Errorf("Armenian HO: normalizeForFilter(%q) = %q, want %q", "\u0570", got, want)
	}
	if got, want := normalizeForFilter("\uab75"), "i"; got != want {
		t.Errorf("Cherokee small V: normalizeForFilter(%q) = %q, want %q", "\uab75", got, want)
	}
}

// Spacing/punctuation inserted between every letter of a word must not
// survive normalization, since substring matching never sees the banned
// word as a contiguous run otherwise.
func TestNormalizeForFilterDefeatsSpacingAndPunctuationInsertion(t *testing.T) {
	const want = "niger"
	cases := map[string]string{
		"spaced":            "n i g g e r",
		"dotted":            "n.i.g.g.e.r",
		"hyphenated":        "n-i-g-g-e-r",
		"underscored":       "n_i_g_g_e_r",
		"mixed punctuation": "n.i-g_g'e r",
	}
	for name, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("%s: normalizeForFilter(%q) = %q, want %q", name, input, got, want)
		}
	}
}

// Repeated/stuffed letters must collapse down to the same normalized form
// as the plain word, regardless of how many extra copies are inserted.
func TestNormalizeForFilterDefeatsLetterStuffing(t *testing.T) {
	const want = "niger"
	cases := []string{"niggger", "niggggggger", "nniiggger"}
	for _, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("normalizeForFilter(%q) = %q, want %q", input, got, want)
		}
	}
}

// Common leetspeak digit/symbol substitutions fold to their Latin letter.
func TestNormalizeForFilterDefeatsLeetspeak(t *testing.T) {
	const want = "niger"
	cases := []string{"n1gg3r", "n1gg33r", "n!gg3r"}
	for _, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("normalizeForFilter(%q) = %q, want %q", input, got, want)
		}
	}
}

// Plain ASCII text with no evasion tricks still normalizes deterministically:
// lowercased, non-letters (including spaces) dropped, doubled letters
// collapsed. This documents the new behavior rather than treating the input
// as passed through unchanged.
func TestNormalizeForFilterPlainASCIIBaseline(t *testing.T) {
	if got, want := normalizeForFilter("Hello World"), "heloworld"; got != want {
		t.Errorf("normalizeForFilter(%q) = %q, want %q", "Hello World", got, want)
	}
	if got, want := normalizeForFilter("moderator"), "moderator"; got != want {
		t.Errorf("normalizeForFilter(%q) = %q, want %q", "moderator", got, want)
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
		"n i g g e r",                                                  // spacing insertion
		"n.i.g.g.e.r",                                                  // punctuation insertion
		"niggggger",                                                    // letter stuffing
		"n1gg3r",                                                       // leetspeak
	}
	for _, msg := range bypasses {
		normalized := normalizeForFilter(msg)
		if _, ok := matchBannedWord(normalized); !ok {
			t.Errorf("matchBannedWord failed to catch bypass %q (normalized: %q)", msg, normalized)
		}
	}
}

// Regression test: a word list entry with no letters at all (a "---" style
// divider, a phone number, a postcode fragment like "69" made only of digits
// outside the leetspeak table, ...) normalizes to the empty string. Since
// strings.Contains treats "" as a substring of every message, letting such an
// entry into bannedWords used to make automod fire on every single message
// from every player. matchBannedWord must never match on an empty entry.
func TestMatchBannedWordIgnoresEmptyEntry(t *testing.T) {
	orig := getBannedWords()
	defer setBannedWords(orig)
	setBannedWords([]string{"", normalizeForFilter("nigger")})

	ordinary := []string{"hello", "good game", "moving to area 3", ""}
	for _, msg := range ordinary {
		if matched, ok := matchBannedWord(normalizeForFilter(msg)); ok {
			t.Errorf("matchBannedWord(%q) unexpectedly matched empty entry (matched=%q)", msg, matched)
		}
	}
	// The real entry must still work alongside the poisoned empty one.
	if _, ok := matchBannedWord(normalizeForFilter("nigger")); !ok {
		t.Error("matchBannedWord failed to catch the real entry once an empty entry was also present")
	}
}

// Regression test for the same bug at the load boundary: a wordlist line
// that has no letters after normalization must be dropped rather than stored
// as an empty entry.
func TestLoadWordListFile_SkipsEntriesThatNormalizeToEmpty(t *testing.T) {
	f, err := os.CreateTemp("", "athena-wordlist-empty-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("nigger\n---\n69\n===\nmoderator\n")
	f.Close()

	words, err := loadWordListFile(f.Name())
	if err != nil {
		t.Fatalf("loadWordListFile returned error: %v", err)
	}
	for _, w := range words {
		if w == "" {
			t.Fatalf("loadWordListFile returned an empty entry from %v", words)
		}
	}
	want := map[string]bool{normalizeForFilter("nigger"): false, normalizeForFilter("moderator"): false}
	if len(words) != len(want) {
		t.Fatalf("expected %d entries (the divider/number-only lines should be dropped), got %d: %v", len(want), len(words), words)
	}
	for _, w := range words {
		if _, ok := want[w]; !ok {
			t.Errorf("unexpected entry %q", w)
		}
		want[w] = true
	}
	for w, seen := range want {
		if !seen {
			t.Errorf("expected entry %q was not loaded", w)
		}
	}
}
