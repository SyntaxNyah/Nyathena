package athena

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ─── Philosopher punishment ────────────────────────────────────────────────

func TestApplyPhilosopher(t *testing.T) {
	input := "Hello world"
	result := applyPhilosopher(input)

	// Must start with the original message.
	if !strings.HasPrefix(result, input) {
		t.Errorf("applyPhilosopher(%q) does not start with input; got %q", input, result)
	}

	// Must append something extra.
	if result == input {
		t.Errorf("applyPhilosopher(%q) returned the unchanged input", input)
	}

	// The appended text must be one of the known philosophical questions.
	appended := strings.TrimSpace(strings.TrimPrefix(result, input))
	found := false
	for _, q := range philosopherQuestions {
		if appended == q {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("appended text %q is not in philosopherQuestions", appended)
	}
}

func TestApplyPhilosopherTableDriven(t *testing.T) {
	cases := []string{"test", "A long message with punctuation!", "", "x"}
	for _, tc := range cases {
		got := applyPhilosopher(tc)
		if !strings.HasPrefix(got, tc) {
			t.Errorf("applyPhilosopher(%q): result %q does not contain input as prefix", tc, got)
		}
	}
}

// ─── Poet punishment ──────────────────────────────────────────────────────

func TestApplyPoet(t *testing.T) {
	input := "the sky is blue"
	result := applyPoet(input)

	if !strings.Contains(result, input) {
		t.Errorf("applyPoet(%q) = %q does not contain original text", input, result)
	}

	// Must include a prefix.
	hasPrefix := false
	for _, p := range poeticPrefixes {
		if strings.HasPrefix(result, p) {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		t.Errorf("applyPoet(%q) = %q does not start with a poetic prefix", input, result)
	}

	// Must include a suffix.
	hasSuffix := false
	for _, s := range poeticSuffixes {
		if strings.Contains(result, s) {
			hasSuffix = true
			break
		}
	}
	if !hasSuffix {
		t.Errorf("applyPoet(%q) = %q does not end with a poetic suffix", input, result)
	}
}

// ─── Upsidedown punishment ────────────────────────────────────────────────

func TestApplyUpsidedownLength(t *testing.T) {
	// The number of Unicode code points should be preserved.
	cases := []string{"hello", "abc", "AaBbCc", "123", "test!"}
	for _, tc := range cases {
		got := applyUpsidedown(tc)
		origLen := utf8.RuneCountInString(tc)
		gotLen := utf8.RuneCountInString(got)
		if origLen != gotLen {
			t.Errorf("applyUpsidedown(%q): rune count changed from %d to %d", tc, origLen, gotLen)
		}
	}
}

func TestApplyUpsidedownReversed(t *testing.T) {
	// The first rune of the result should map from the last rune of the input.
	input := "ab"
	result := applyUpsidedown(input)
	// Reversed, 'b' → 'q' should be first.
	firstRune, _ := utf8.DecodeRuneInString(result)
	if firstRune != upsidedownMap['b'] {
		t.Errorf("applyUpsidedown(%q) first rune: got %q, want %q", input, firstRune, upsidedownMap['b'])
	}
}

func TestApplyUpsidedownKnownChars(t *testing.T) {
	// Spot-check a few well-known flipped characters.
	cases := []struct {
		in   string
		want rune
	}{
		{"a", 'ɐ'},
		{"e", 'ǝ'},
		{"h", 'ɥ'},
	}
	for _, tc := range cases {
		// Single character input — reversed is still a single character.
		got := applyUpsidedown(tc.in)
		r, _ := utf8.DecodeRuneInString(got)
		if r != tc.want {
			t.Errorf("applyUpsidedown(%q) = %q (rune %c), want %c", tc.in, got, r, tc.want)
		}
	}
}

func TestApplyUpsidedownEmpty(t *testing.T) {
	if got := applyUpsidedown(""); got != "" {
		t.Errorf("applyUpsidedown(\"\") = %q, want \"\"", got)
	}
}

// ─── Sarcasm punishment ───────────────────────────────────────────────────

func TestApplySarcasm(t *testing.T) {
	input := "I had a great idea"
	result := applySarcasm(input)

	if !strings.HasPrefix(result, input) {
		t.Errorf("applySarcasm(%q) = %q does not start with input", input, result)
	}

	appended := strings.TrimSpace(strings.TrimPrefix(result, input))
	found := false
	for _, c := range sarcasmCommentaries {
		if appended == c {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applySarcasm: appended %q is not in sarcasmCommentaries", appended)
	}
}

// ─── Academic punishment ──────────────────────────────────────────────────

func TestApplyAcademic(t *testing.T) {
	input := "cats are nice"
	result := applyAcademic(input)

	if !strings.Contains(result, input) {
		t.Errorf("applyAcademic(%q) = %q does not contain original text", input, result)
	}

	hasPrefix := false
	for _, p := range academicPrefixes {
		if strings.HasPrefix(result, p) {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		t.Errorf("applyAcademic(%q) = %q has no academic prefix", input, result)
	}
}

// ─── Recipe punishment ────────────────────────────────────────────────────

func TestApplyRecipe(t *testing.T) {
	input := "stir the pot"
	result := applyRecipe(input)

	if !strings.Contains(result, input) {
		t.Errorf("applyRecipe(%q) = %q does not contain original text", input, result)
	}

	if !strings.HasPrefix(result, "Step 1:") {
		t.Errorf("applyRecipe(%q) = %q does not start with \"Step 1:\"", input, result)
	}

	hasEnding := false
	for _, e := range recipeEndings {
		if strings.HasSuffix(result, e) {
			hasEnding = true
			break
		}
	}
	if !hasEnding {
		t.Errorf("applyRecipe(%q) = %q does not end with a known recipe ending", input, result)
	}
}

// ─── Typing race helpers ──────────────────────────────────────────────────

func TestNormaliseTypingPhrase(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"  spaces   everywhere  ", "spaces everywhere"},
		{"OBJECTION!", "objection"},
		{"it's a trap", "it's a trap"},
		{"multiple   spaces", "multiple spaces"},
		{"", ""},
		{"123 abc", "123 abc"},
		{"hello, world!", "hello world"},
	}
	for _, tc := range cases {
		got := normaliseTypingPhrase(tc.input)
		if got != tc.expected {
			t.Errorf("normaliseTypingPhrase(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormaliseTypingPhraseMatchesPhrase(t *testing.T) {
	// Simulate: phrase posted, player types it back with slightly different
	// punctuation/capitalisation — should still match.
	phrase := "the quick brown fox"
	if normaliseTypingPhrase(phrase) != normaliseTypingPhrase("The Quick Brown Fox.") {
		t.Error("normalised phrases with different case/punctuation should be equal")
	}
}

// ─── Newspaper section builders (smoke tests) ────────────────────────────

func TestNewspaperSectionBuildersDontPanic(t *testing.T) {
	// These call DB functions that return early on nil db, so they just
	// exercise the formatting paths without a live database.
	sections := []string{
		NewspaperSectionChipLeaderboard,
		NewspaperSectionPlaytimeTop,
		NewspaperSectionUnscrambleTop,
		NewspaperSectionRecentBans,
		NewspaperSectionServerStats,
		NewspaperSectionWeather,
		NewspaperSectionHoroscope,
		NewspaperSectionWordOfTheDay,
		NewspaperSectionCasino,
		NewspaperSectionAreaHighlight,
		NewspaperSectionJobMarket,
		NewspaperSectionPunishmentReport,
		NewspaperSectionTip,
		NewspaperSectionClassifieds,
		NewspaperSectionHolidayGreeting,
		"unknown_section", // should return "" gracefully
	}
	for _, s := range sections {
		// Must not panic, and must return a string (possibly empty).
		result := buildNewspaperSection(s)
		_ = result // result content depends on DB state; just ensure no panic
	}
}

func TestNewspaperWeatherIsNonEmpty(t *testing.T) {
	result := buildSectionWeather()
	if strings.TrimSpace(result) == "" {
		t.Error("buildSectionWeather() returned empty string")
	}
}

func TestNewspaperHoroscopeIsNonEmpty(t *testing.T) {
	result := buildSectionHoroscope()
	if strings.TrimSpace(result) == "" {
		t.Error("buildSectionHoroscope() returned empty string")
	}
}

func TestNewspaperTipIsNonEmpty(t *testing.T) {
	result := buildSectionTip()
	if strings.TrimSpace(result) == "" {
		t.Error("buildSectionTip() returned empty string")
	}
}

func TestNewspaperClassifiedsIsNonEmpty(t *testing.T) {
	result := buildSectionClassifieds()
	if strings.TrimSpace(result) == "" {
		t.Error("buildSectionClassifieds() returned empty string")
	}
}

func TestNewspaperWordOfTheDayIsNonEmpty(t *testing.T) {
	result := buildSectionWordOfTheDay()
	if strings.TrimSpace(result) == "" {
		t.Error("buildSectionWordOfTheDay() returned empty string")
	}
}
