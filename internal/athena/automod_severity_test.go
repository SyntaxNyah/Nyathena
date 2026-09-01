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

// Tests for the tiered word-list loading added on top of word_severity.go:
// buildWordEntries/loadWordListEntries in automod.go.
package athena

import (
	"os"
	"strings"
	"testing"
)

// writeTempWordlist writes content to a fresh temp file and returns its path,
// removing the file when the test ends.
func writeTempWordlist(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "athena-wordlist-severity-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	return f.Name()
}

// findEntry looks up an entry by (pattern, mode) -- the same key buildWordEntries
// dedupes on -- so tests can assert presence/absence/severity without depending
// on slice order beyond what's explicitly under test.
func findEntry(entries []WordEntry, pattern string, mode MatchMode) (WordEntry, bool) {
	for _, e := range entries {
		if e.Pattern == pattern && e.Mode == mode {
			return e, true
		}
	}
	return WordEntry{}, false
}

// An unmarked line must behave exactly as loadWordListFile has always
// treated it: SeverityDefault, MatchSubstring, and the identical normalized
// pattern the flat loader would have produced for the same line. This is the
// headline backward-compatibility guarantee -- an operator's existing
// banned_words.txt, with no '|' anywhere in it, must not change behaviour at
// all just because the tiered reader now also exists.
func TestBuildWordEntriesBackwardCompatible(t *testing.T) {
	const content = "badword\noffensive phrase\n"
	path := writeTempWordlist(t, content)

	flat, err := loadWordListFile(path)
	if err != nil {
		t.Fatalf("loadWordListFile: %v", err)
	}
	tiered, err := loadWordListEntries(path)
	if err != nil {
		t.Fatalf("loadWordListEntries: %v", err)
	}

	if len(tiered) != len(flat) {
		t.Fatalf("expected %d tiered entries to match the %d flat entries, got %d: %+v", len(flat), len(flat), len(tiered), tiered)
	}
	for _, w := range flat {
		e, ok := findEntry(tiered, w, MatchSubstring)
		if !ok {
			t.Errorf("flat entry %q has no corresponding tiered entry", w)
			continue
		}
		if e.Severity != SeverityDefault {
			t.Errorf("unmarked entry %q: got severity %v, want SeverityDefault", w, e.Severity)
		}
		if e.Mode != MatchSubstring {
			t.Errorf("unmarked entry %q: got mode %v, want MatchSubstring", w, e.Mode)
		}
	}
}

// Every tier and every mode keyword parses to the entry buildWordEntries is
// documented to produce.
func TestBuildWordEntriesParsesEveryTierAndMode(t *testing.T) {
	const content = `` +
		"plainword\n" +
		"watchword | watch\n" +
		"severeword | severe\n" +
		"nukeword | nuke\n" +
		"rape | severe | word\n" +
		"substrword | default | substring\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	cases := []struct {
		pattern string
		mode    MatchMode
		want    WordSeverity
	}{
		{"plainword", MatchSubstring, SeverityDefault},
		{"watchword", MatchSubstring, SeverityWatch},
		{"severeword", MatchSubstring, SeveritySevere},
		{"nukeword", MatchSubstring, SeverityNuke},
		{"rape", MatchWord, SeveritySevere},
		{"substrword", MatchSubstring, SeverityDefault},
	}
	for _, c := range cases {
		e, ok := findEntry(entries, c.pattern, c.mode)
		if !ok {
			t.Errorf("expected an entry for pattern %q mode %v, found none in %+v", c.pattern, c.mode, entries)
			continue
		}
		if e.Severity != c.want {
			t.Errorf("pattern %q: got severity %v, want %v", c.pattern, e.Severity, c.want)
		}
	}
}

// An unparseable tier is a warning and the entry is SKIPPED -- never
// downgraded to SeverityDefault or any other tier. Silently guessing "nuek"
// meant "default" would leave an operator believing a word is banned outright
// when it is in fact only mildly acted on (or not acted on at all).
func TestBuildWordEntriesUnparseableTierIsSkippedNotDowngraded(t *testing.T) {
	const content = "goodword | watch\nbadtier | nuek\nanothergood | severe\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if _, ok := findEntry(entries, "badtier", MatchSubstring); ok {
		t.Error("expected the entry with an unrecognized tier to be skipped entirely, but it was loaded")
	}
	for _, p := range []string{"goodword", "anothergood"} {
		if _, ok := findEntry(entries, p, MatchSubstring); !ok {
			t.Errorf("expected sibling entry %q to still load", p)
		}
	}
	if len(entries) != 2 {
		t.Errorf("expected exactly 2 entries to survive (the bad-tier line skipped), got %d: %+v", len(entries), entries)
	}
}

// Mirrors the tier case for an unparseable match mode: skipped, not silently
// treated as substring mode.
func TestBuildWordEntriesUnparseableModeIsSkippedNotDowngraded(t *testing.T) {
	const content = "goodword | severe | word\nbadmode | severe | wrod\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if _, ok := findEntry(entries, "badmode", MatchSubstring); ok {
		t.Error("expected the entry with an unrecognized match mode to be skipped (found under substring)")
	}
	if _, ok := findEntry(entries, "badmode", MatchWord); ok {
		t.Error("expected the entry with an unrecognized match mode to be skipped (found under word)")
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 entry to survive, got %d: %+v", len(entries), entries)
	}
}

// A line with more than 3 '|'-separated fields is a warning and a skip.
func TestBuildWordEntriesTooManyFieldsIsSkipped(t *testing.T) {
	const content = "toomany | severe | word | extra\ngoodword | watch\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if _, ok := findEntry(entries, "toomany", MatchWord); ok {
		t.Error("expected the entry with 4 fields to be skipped")
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 entry to survive, got %d: %+v", len(entries), entries)
	}
}

// The headline fix this whole feature exists for: "rape" as a plain
// substring entry is rejected by the common-word collision gate (it's a
// substring of "therapeutic"), exactly as loadWordListFile has always
// rejected it -- but "word" mode accepts it, because a boundary match
// structurally cannot fire inside "therapeutic" in the first place.
func TestWordModeAcceptsRapeSubstringModeRejectsIt(t *testing.T) {
	const content = "rape | severe\nrape | severe | word\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if _, ok := findEntry(entries, "rape", MatchSubstring); ok {
		t.Error(`expected "rape" as a plain substring entry to be rejected (collides with "therapeutic"), but it loaded`)
	}
	e, ok := findEntry(entries, "rape", MatchWord)
	if !ok {
		t.Fatal(`expected "rape | severe | word" to load -- word mode must accept it despite the substring collision`)
	}
	if e.Severity != SeveritySevere {
		t.Errorf(`"rape" (word mode): got severity %v, want SeveritySevere`, e.Severity)
	}

	// And prove it end to end: the loaded entry actually catches "raped" in a
	// message but ignores "therapeutic".
	if m := matchWordEntries(entries, "they were raped"); !m.Matched || m.Entry.Mode != MatchWord {
		t.Errorf(`expected the word-mode "rape" entry to match "they were raped", got %+v`, m)
	}
	if m := matchWordEntries(entries, "a therapeutic massage"); m.Matched {
		t.Errorf(`expected no match against "a therapeutic massage", got %+v`, m)
	}
}

// The substring gates (minNormalizedEntryLen and collidesWithCommonWords)
// still apply, unchanged, to substring-mode tiered entries -- tiering is
// additive, not a way to bypass the existing safety checks.
func TestBuildWordEntriesSubstringGatesStillApply(t *testing.T) {
	const content = "tR0N | severe\nl36 | nuke\ngoodslur | nuke\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if _, ok := findEntry(entries, "tron", MatchSubstring); ok {
		t.Error(`expected "tR0N" (normalizes to "tron", collides with common words) to be rejected even with a tier`)
	}
	if _, ok := findEntry(entries, "le", MatchSubstring); ok {
		t.Error(`expected "l36" (normalizes to "le", too short) to be rejected even with a tier`)
	}
	if _, ok := findEntry(entries, "goodslur", MatchSubstring); !ok {
		t.Error(`expected "goodslur | nuke" to load normally`)
	}
}

// A word-mode pattern that normalizes to more than one token can never match
// anything (matchesWordBoundary only ever compares against a single token),
// so it must be rejected at load time rather than silently kept as dead
// weight.
func TestBuildWordEntriesMultiWordWordModeRejected(t *testing.T) {
	const content = "hate speech | severe | word\ngoodword | severe | word\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if _, ok := findEntry(entries, "hate speech", MatchWord); ok {
		t.Error("expected the multi-word word-mode entry to be rejected")
	}
	for _, e := range entries {
		if e.Mode == MatchWord {
			t.Logf("surviving word-mode entry: %+v", e)
		}
	}
	if _, ok := findEntry(entries, "goodword", MatchWord); !ok {
		t.Error("expected the sibling single-token word-mode entry to survive")
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 entry to survive, got %d: %+v", len(entries), entries)
	}
}

// The same (pattern, mode) pair listed twice at different tiers dedupes down
// to one entry at the HIGHEST severity seen -- never the first, and never the
// last; the worse tier always wins, since that can only make a message's
// eventual verdict more accurate.
func TestBuildWordEntriesDuplicatePatternsKeepHighestSeverity(t *testing.T) {
	const content = "slur | watch\nslur | default\nslur | nuke\nslur | severe\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	if len(entries) != 1 {
		t.Fatalf("expected the 4 duplicate lines to dedupe to exactly 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Severity != SeverityNuke {
		t.Errorf("expected the surviving entry to keep the highest severity seen (nuke), got %v", entries[0].Severity)
	}

	// Order shouldn't matter either -- nuke listed first still wins.
	const content2 = "slur | nuke\nslur | watch\n"
	entries2 := buildWordEntries(splitLinesForTest(content2), "test")
	if len(entries2) != 1 || entries2[0].Severity != SeverityNuke {
		t.Errorf("expected nuke to win regardless of listing order, got %+v", entries2)
	}
}

// buildWordEntries must return its slice sorted severity-descending (and by
// ascending pattern length within a tier) -- matchWordEntries relies on this
// ordering to short-circuit the instant a nuke is found.
func TestBuildWordEntriesSortedSeverityDescending(t *testing.T) {
	const content = "" +
		"defaultword | default\n" +
		"nukeword | nuke\n" + // 8 letters
		"watchword | watch\n" +
		"severeword | severe\n" +
		"nuke | nuke\n" // 4 letters -- shorter nuke-tier pattern, must sort BEFORE nukeword above
	entries := buildWordEntries(splitLinesForTest(content), "test")

	for i := 1; i < len(entries); i++ {
		if entries[i-1].Severity < entries[i].Severity {
			t.Fatalf("entries not sorted severity-descending at index %d: %+v", i, entries)
		}
		if entries[i-1].Severity == entries[i].Severity && len(entries[i-1].Pattern) > len(entries[i].Pattern) {
			t.Fatalf("entries not sorted by ascending pattern length within a tier at index %d: %+v", i, entries)
		}
	}
	if entries[0].Severity != SeverityNuke {
		t.Errorf("expected the nuke-tier entry first, got %+v", entries[0])
	}
	if entries[len(entries)-1].Severity != SeverityWatch {
		t.Errorf("expected the watch-tier entry last, got %+v", entries[len(entries)-1])
	}
}

// End-to-end: matchWordEntries must return the nuke-tier match when a
// message contains both a mild (watch-tier) word and a nuke-tier word --
// worst-match-wins, not first-match, exactly like TestMatchWordEntriesWorstSeverityWins
// in word_severity_test.go, but exercised through the real loader here
// instead of hand-built entries.
func TestMatchWordEntriesReturnsNukeOverMildInSameMessage(t *testing.T) {
	const content = "darn | watch\nnukeslur | nuke\n"
	entries := buildWordEntries(splitLinesForTest(content), "test")

	m := matchWordEntries(entries, "well darn, nukeslur to you too")
	if !m.Matched || m.Entry.Severity != SeverityNuke {
		t.Fatalf("expected the nuke-tier entry to win over the watch-tier entry present in the same message, got %+v", m)
	}

	// And a message containing only the mild word gets the mild verdict.
	m2 := matchWordEntries(entries, "well darn")
	if !m2.Matched || m2.Entry.Severity != SeverityWatch {
		t.Fatalf("expected the watch-tier entry alone to match at watch severity, got %+v", m2)
	}
}

// splitLinesForTest mimics what readWordListLines hands buildWordEntries:
// trimmed, non-blank, non-comment lines, in order. Kept local to this file
// (rather than depending on readWordListLines' file-reading side effects) so
// buildWordEntries can be exercised directly against inline literals.
func splitLinesForTest(content string) []string {
	var lines []string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
