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
// normalizeForFilter only collapses runs of 3+ identical letters (down to 2),
// so "nigger" itself keeps its double g intact rather than shrinking to
// "niger" — see TestNormalizeForFilterPreservesDoubleLetters for why that
// matters. Every case below is expected to land on "nigger", not "niger".
func TestNormalizeForFilterDefeatsUnicodeBypass(t *testing.T) {
	const want = "nigger"
	cases := map[string]string{
		"mathematical bold script": "\U0001d4f7\U0001d4f2\U0001d4f0\U0001d4f0\U0001d4ee\U0001d4fb",
		"fullwidth":                "ｎｉｇｇｅｒ",
		"zero-width inserted":      "n​i​g​g​e​r",
		"combining marks inserted": "níggér",
	}
	for name, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("%s: normalizeForFilter(%q) = %q, want %q", name, input, got, want)
		}
	}
}

func TestNormalizeForFilterCyrillicHomoglyphs(t *testing.T) {
	// Cyrillic ES/A/TE look identical to Latin "cat" but are distinct codepoints.
	input := "сат"
	if got, want := normalizeForFilter(input), "cat"; got != want {
		t.Errorf("normalizeForFilter(%q) = %q, want %q", input, got, want)
	}
}

func TestNormalizeForFilterArmenianAndCherokeeHomoglyphs(t *testing.T) {
	if got, want := normalizeForFilter("հ"), "h"; got != want {
		t.Errorf("Armenian HO: normalizeForFilter(%q) = %q, want %q", "հ", got, want)
	}
	if got, want := normalizeForFilter("ꭵ"), "i"; got != want {
		t.Errorf("Cherokee small V: normalizeForFilter(%q) = %q, want %q", "ꭵ", got, want)
	}
}

// Spacing/punctuation inserted between every letter of a word must not
// survive normalization, since substring matching never sees the banned
// word as a contiguous run otherwise.
func TestNormalizeForFilterDefeatsSpacingAndPunctuationInsertion(t *testing.T) {
	const want = "nigger"
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

// Runs of 3+ identical letters (obvious stuffing, e.g. someone leaning on a
// key) collapse down to 2, regardless of how many extra copies are inserted.
func TestNormalizeForFilterDefeatsLetterStuffing(t *testing.T) {
	const want = "nigger"
	cases := []string{"niggger", "niggggggger", "niggggggggggger"}
	for _, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("normalizeForFilter(%q) = %q, want %q", input, got, want)
		}
	}
}

// Regression test: collapsing runs down to a single letter (rather than 2)
// used to shrink legitimately double-lettered entries into something that
// collides with common English words. "nigger" itself would shrink to
// "niger", which is a substring of "nigeria"; "ngger" would shrink to
// "nger", a substring of "anger"/"danger"/"finger"/"stronger"/"messenger"/
// countless other everyday words. Capping collapsed runs at 2 instead of 1
// keeps genuine double letters intact so entries stay as specific as they
// were typed, without giving up on defeating 3+-repeat stuffing.
func TestNormalizeForFilterPreservesDoubleLetters(t *testing.T) {
	cases := map[string]string{
		"nigger": "nigger",
		"ngger":  "ngger",
		"troon":  "troon",
	}
	for input, want := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("normalizeForFilter(%q) = %q, want %q (double letters must survive intact)", input, got, want)
		}
	}
}

// Common leetspeak digit/symbol substitutions fold to their Latin letter.
func TestNormalizeForFilterDefeatsLeetspeak(t *testing.T) {
	const want = "nigger"
	cases := []string{"n1gg3r", "n1ggg3r", "n!gg3r"}
	for _, input := range cases {
		if got := normalizeForFilter(input); got != want {
			t.Errorf("normalizeForFilter(%q) = %q, want %q", input, got, want)
		}
	}
}

// Plain ASCII text with no evasion tricks still normalizes deterministically:
// lowercased, non-letters (including spaces) dropped, double letters kept.
func TestNormalizeForFilterPlainASCIIBaseline(t *testing.T) {
	if got, want := normalizeForFilter("Hello World"), "helloworld"; got != want {
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
		"ｎｉｇｇｅｒ",      // fullwidth
		"n​i​g​g​e​r", // zero-width inserted
		"n i g g e r", // spacing insertion
		"n.i.g.g.e.r", // punctuation insertion
		"niggggger",   // letter stuffing
		"n1gg3r",      // leetspeak
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
// as an empty entry. "trebkil" is a nonsense placeholder verified not to
// collide with any common word (see TestCollidesWithCommonWords) so this
// test stays focused on the empty-normalization case.
func TestLoadWordListFile_SkipsEntriesThatNormalizeToEmpty(t *testing.T) {
	f, err := os.CreateTemp("", "athena-wordlist-empty-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("nigger\n---\n69\n===\ntrebkil\n")
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
	want := map[string]bool{normalizeForFilter("nigger"): false, normalizeForFilter("trebkil"): false}
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

// Regression test for the production incident this was built to prevent: a
// postcode fragment "l36" normalizes to "le" (the digit '3' leetspeak-maps
// to 'e', the unmapped digit '6' is dropped) — a 2-letter needle that's a
// substring of a huge fraction of ordinary English ("hello", "please",
// "level", ...). A wordlist entry that short must never be loaded.
func TestLoadWordListFile_RejectsEntriesShorterThanMinLen(t *testing.T) {
	f, err := os.CreateTemp("", "athena-wordlist-short-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("l36\n1TW\nnigger\n")
	f.Close()

	words, err := loadWordListFile(f.Name())
	if err != nil {
		t.Fatalf("loadWordListFile returned error: %v", err)
	}
	if len(words) != 1 || words[0] != normalizeForFilter("nigger") {
		t.Fatalf("expected only the nigger entry to survive (l36->le and 1TW->itw are too short), got %v", words)
	}
}

// Regression test: a wordlist entry can clear the length floor and still be
// dangerously broad. "tR0N" normalizes to "tron" (4 letters, passes
// minNormalizedEntryLen) but is a substring of "electronic", "strong",
// "astronomy", ... loadWordListFile must reject it via the common-word
// collision guard even though it isn't short enough to be caught by length
// alone.
func TestLoadWordListFile_RejectsCommonWordCollisions(t *testing.T) {
	f, err := os.CreateTemp("", "athena-wordlist-collision-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("tR0N\ntroon\n")
	f.Close()

	words, err := loadWordListFile(f.Name())
	if err != nil {
		t.Fatalf("loadWordListFile returned error: %v", err)
	}
	if len(words) != 1 || words[0] != "troon" {
		t.Fatalf("expected only \"troon\" to survive (tR0N normalizes to \"tron\", which collides with common words), got %v", words)
	}
}

func TestCollidesWithCommonWords(t *testing.T) {
	if hits := collidesWithCommonWords("tron"); len(hits) == 0 {
		t.Error(`expected "tron" to collide with common words like "electronic"/"strong"/"astronomy"`)
	}
	if hits := collidesWithCommonWords(normalizeForFilter("nigger")); len(hits) != 0 {
		t.Errorf(`expected "nigger" not to collide with any common word, got %v`, hits)
	}
	// An entry that IS itself a real word (not a substring of some other,
	// unrelated word) must not be flagged as colliding with itself.
	if hits := collidesWithCommonWords("trebkil"); len(hits) != 0 {
		t.Errorf(`expected "trebkil" not to collide with any common word, got %v`, hits)
	}
}
