package athena

import (
	"strings"
	"testing"
)

// ── pickHangmanWord ───────────────────────────────────────────────────────────

// TestPickHangmanWordThemes verifies that every theme returns a word from the
// expected pool and that the "random" (default) theme draws from the full set.
func TestPickHangmanWordThemes(t *testing.T) {
	themes := map[string][]string{
		"animals":   hangmanWordsAnimals,
		"courtroom": hangmanWordsCourtroom,
		"nature":    hangmanWordsNature,
		"food":      hangmanWordsFood,
		"random":    hangmanWordsAll,
	}
	for theme, pool := range themes {
		set := make(map[string]bool, len(pool))
		for _, w := range pool {
			set[w] = true
		}
		// Sample 20 times per theme to gain confidence.
		for i := 0; i < 20; i++ {
			w := pickHangmanWord(theme)
			if !set[w] {
				t.Errorf("theme %q: word %q not found in expected pool", theme, w)
			}
		}
	}
}

// TestPickHangmanWordUnknownThemeFallsBackToAll verifies that an unknown theme
// returns a word from the combined pool (same behaviour as "random").
func TestPickHangmanWordUnknownThemeFallsBackToAll(t *testing.T) {
	set := make(map[string]bool, len(hangmanWordsAll))
	for _, w := range hangmanWordsAll {
		set[w] = true
	}
	for i := 0; i < 20; i++ {
		w := pickHangmanWord("nonsense_theme")
		if !set[w] {
			t.Errorf("unknown theme: word %q not found in combined pool", w)
		}
	}
}

// ── hangmanDisplayWord ────────────────────────────────────────────────────────

func TestHangmanDisplayWordAllHidden(t *testing.T) {
	word := "cat"
	revealed := []bool{false, false, false}
	got := hangmanDisplayWord(word, revealed)
	want := "_ _ _"
	if got != want {
		t.Errorf("hangmanDisplayWord all-hidden: got %q, want %q", got, want)
	}
}

func TestHangmanDisplayWordAllRevealed(t *testing.T) {
	word := "cat"
	revealed := []bool{true, true, true}
	got := hangmanDisplayWord(word, revealed)
	want := "c a t"
	if got != want {
		t.Errorf("hangmanDisplayWord all-revealed: got %q, want %q", got, want)
	}
}

func TestHangmanDisplayWordPartial(t *testing.T) {
	word := "elephant"
	revealed := []bool{true, false, false, false, false, false, false, true}
	got := hangmanDisplayWord(word, revealed)
	// e _ _ _ _ _ _ t
	if !strings.HasPrefix(got, "e") || !strings.HasSuffix(got, "t") {
		t.Errorf("hangmanDisplayWord partial: unexpected result %q", got)
	}
}

// ── hangmanAllRevealed ────────────────────────────────────────────────────────

func TestHangmanAllRevealedTrue(t *testing.T) {
	if !hangmanAllRevealed([]bool{true, true, true}) {
		t.Error("expected all-true slice to report all revealed")
	}
}

func TestHangmanAllRevealedFalse(t *testing.T) {
	if hangmanAllRevealed([]bool{true, false, true}) {
		t.Error("expected partial slice to report not all revealed")
	}
}

func TestHangmanAllRevealedEmpty(t *testing.T) {
	if !hangmanAllRevealed([]bool{}) {
		t.Error("expected empty slice to report all revealed")
	}
}

// ── hangmanWrongStr ───────────────────────────────────────────────────────────

func TestHangmanWrongStrEmpty(t *testing.T) {
	if got := hangmanWrongStr(nil, 0); got != "(none)" {
		t.Errorf("hangmanWrongStr empty: got %q", got)
	}
}

func TestHangmanWrongStrLettersOnly(t *testing.T) {
	got := hangmanWrongStr([]rune{'a', 'b', 'c'}, 0)
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") || !strings.Contains(got, "C") {
		t.Errorf("hangmanWrongStr letters: got %q, expected A, B, C", got)
	}
}

func TestHangmanWrongStrWordGuesses(t *testing.T) {
	got := hangmanWrongStr([]rune{'x'}, 2)
	// Should contain X and two ★ symbols.
	if strings.Count(got, "★") != 2 {
		t.Errorf("hangmanWrongStr 2 wrong words: got %q, want 2 × ★", got)
	}
}

// ── hangmanTotalWrong ─────────────────────────────────────────────────────────

func TestHangmanTotalWrong(t *testing.T) {
	st := &hangmanState{
		wrongLetters:   []rune{'a', 'b'},
		wrongWordCount: 2,
	}
	if got := hangmanTotalWrong(st); got != 4 {
		t.Errorf("hangmanTotalWrong: got %d, want 4", got)
	}
}

// ── Punishment pool reuse ─────────────────────────────────────────────────────

// TestHangmanPunishmentPoolNonEmpty ensures the reused pool is never empty,
// which would cause a panic in hangmanPunishLosers.
func TestHangmanPunishmentPoolNonEmpty(t *testing.T) {
	if len(hotPotatoPunishmentPool) == 0 {
		t.Fatal("hotPotatoPunishmentPool is empty — hangman punishments would panic")
	}
}

// ── Word pools populated ──────────────────────────────────────────────────────

func TestHangmanAllWordsInitialised(t *testing.T) {
	if len(hangmanWordsAll) == 0 {
		t.Fatal("hangmanWordsAll is empty after init()")
	}
	expected := len(hangmanWordsAnimals) + len(hangmanWordsCourtroom) +
		len(hangmanWordsNature) + len(hangmanWordsFood)
	if len(hangmanWordsAll) != expected {
		t.Errorf("hangmanWordsAll length: got %d, want %d", len(hangmanWordsAll), expected)
	}
}

// TestHangmanWordPoolsMinLength ensures all per-theme pools have enough words to
// prevent degenerate games.
func TestHangmanWordPoolsMinLength(t *testing.T) {
	const min = 10
	pools := map[string][]string{
		"animals":   hangmanWordsAnimals,
		"courtroom": hangmanWordsCourtroom,
		"nature":    hangmanWordsNature,
		"food":      hangmanWordsFood,
	}
	for name, pool := range pools {
		if len(pool) < min {
			t.Errorf("pool %q has only %d words (want at least %d)", name, len(pool), min)
		}
	}
}

// TestHangmanWordPoolsLettersOnly verifies every word in every pool contains
// only ASCII letters so guesses can be safely normalised.
func TestHangmanWordPoolsLettersOnly(t *testing.T) {
	for _, w := range hangmanWordsAll {
		for _, ch := range w {
			if ch < 'a' || ch > 'z' {
				t.Errorf("word %q contains non-lowercase-ASCII character %q", w, ch)
			}
		}
	}
}

// ── hangmanArt ────────────────────────────────────────────────────────────────

func TestHangmanArtLength(t *testing.T) {
	if len(hangmanArt) != hangmanMaxWrong+1 {
		t.Errorf("hangmanArt has %d stages, want %d (hangmanMaxWrong+1)", len(hangmanArt), hangmanMaxWrong+1)
	}
}
