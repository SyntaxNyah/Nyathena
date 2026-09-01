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

// The tier an operator writes is the tier that gets enforced.
//
// This exists because that stopped being true in production and nothing
// caught it. Every unit test passed: buildWordEntries parsed the tiers
// correctly, autoModCheckTiered handled each tier correctly, matchWordEntries
// returned the right severity. The bug lived in the seam between them --
// effectiveWordEntries merged the flat legacy list back in at SeverityDefault,
// so every entry appeared twice, and since the worst match wins, "watch"
// silently became "default". On a server with automod_action = "torment" that
// meant real players were having messages dropped, being torment-listed and
// being kicked for words explicitly marked alert-only.
//
// Both halves were individually reasonable. loadWordListFile had just started
// stripping the "| tier | mode" suffix so /giveaway would still filter a word
// defined only with a tier -- correct on its own, and it turned what used to
// be an inert whole-line needle ("retardwatch", matching nothing) into the
// real word at default severity.
//
// So these tests drive the PRODUCTION load path, both lists from one file, and
// assert on what comes out the other end rather than on any single function.
package athena

import (
	"os"
	"path/filepath"
	"testing"
)

// writeWordList puts a list on disk and loads it exactly as initAutoMod does:
// the same file into both the tiered list and the flat legacy list.
func writeWordList(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "banned_words.txt")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	tiered, err := loadWordListEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := loadWordListFile(path)
	if err != nil {
		t.Fatal(err)
	}
	prevT, prevF := getWordEntries(), getBannedWords()
	setWordEntries(tiered)
	setBannedWords(flat)
	t.Cleanup(func() { setWordEntries(prevT); setBannedWords(prevF) })
}

// TestTierSurvivesTheLegacyMerge is the regression. A watch entry must still
// be watch after the flat list has been folded in.
func TestTierSurvivesTheLegacyMerge(t *testing.T) {
	writeWordList(t, `
alertonly | watch
crudeword | default
badword | severe
slurword | nuke
`)
	for _, tc := range []struct {
		text string
		want WordSeverity
	}{
		{"you are alertonly", SeverityWatch},
		{"you are crudeword", SeverityDefault},
		{"you are badword", SeveritySevere},
		{"you are slurword", SeverityNuke},
	} {
		m := matchWordEntries(effectiveWordEntries(), tc.text)
		if !m.Matched {
			t.Errorf("%q matched nothing", tc.text)
			continue
		}
		if m.Entry.Severity != tc.want {
			t.Errorf("%q -> %s (entry %q); the file says %s. The operator's tier is not being enforced.",
				tc.text, m.Entry.Severity, m.Entry.Raw, tc.want)
		}
	}
}

// TestWatchNeverPunishes is the property the whole tier exists for, asserted
// through autoModCheckTiered rather than through the matcher, so it covers the
// action taken and not merely the severity reported.
func TestWatchNeverPunishes(t *testing.T) {
	oldAction := autoModAction
	defer func() { autoModAction = oldAction }()

	writeWordList(t, "alertonly | watch\n")

	// A real connection, because every branch of autoModCheckTiered reads the
	// client for its logs and alerts. The point of this test is the ACTION
	// taken, so it has to go through the real function rather than the matcher.
	offender := &Client{char: -1}

	// Every configured action, including the harshest, must leave a watch-tier
	// hit delivered: watch means "tell staff", never "do something to them".
	for _, action := range []autoModActionKind{
		autoModActionShadow, autoModActionTorment, autoModActionKick,
		autoModActionMute, autoModActionBan,
	} {
		autoModAction = action
		m, result, kick := autoModCheckTiered(offender, "you are alertonly", "IC message")
		if !m.Matched || m.Entry.Severity != SeverityWatch {
			t.Fatalf("action %v: watch entry did not match as watch", action)
		}
		if result != autoModPass {
			t.Errorf("action %v: watch-tier hit returned %v, want autoModPass -- the message must be delivered", action, result)
		}
		if kick {
			t.Errorf("action %v: watch-tier hit asked for a kick", action)
		}
	}
}

// TestLegacyOnlyWordsStillFilter guards the other direction: the dedupe must
// not throw away a flat-list word that the tiered list genuinely does not
// cover, or a plain list loaded from elsewhere would stop being enforced.
func TestLegacyOnlyWordsStillFilter(t *testing.T) {
	writeWordList(t, "alertonly | watch\n")
	// Add a word that exists ONLY in the flat list, as a separate source would.
	setBannedWords(append(getBannedWords(), "legacyonlyword"))

	m := matchWordEntries(effectiveWordEntries(), "this has legacyonlyword in it")
	if !m.Matched {
		t.Fatal("a flat-list-only word stopped being filtered; the dedupe is too aggressive")
	}
	if m.Entry.Severity != SeverityDefault {
		t.Errorf("flat-list-only word came out as %s, want default", m.Entry.Severity)
	}
}

// TestNoDuplicateEntriesFromOneFile pins the mechanism rather than a symptom:
// loading one file must not produce two entries per word. The duplication is
// what let the wrong tier win, and it is cheap to assert directly.
func TestNoDuplicateEntriesFromOneFile(t *testing.T) {
	writeWordList(t, "alpha | watch\nbeta | nuke\ngamma\n")
	seen := map[string]int{}
	for _, e := range effectiveWordEntries() {
		seen[e.Pattern]++
	}
	for pat, n := range seen {
		if n > 1 {
			t.Errorf("pattern %q appears %d times after loading a single file; duplicates let the "+
				"harshest copy win regardless of what the operator wrote", pat, n)
		}
	}
}
