package athena

import (
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

func TestPunishmentTranslatorString(t *testing.T) {
	if PunishmentTranslator.String() != "translator" {
		t.Errorf("PunishmentTranslator.String(): expected %q, got %q", "translator", PunishmentTranslator.String())
	}
}

func TestResolveLanguage(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"french", "fr"},
		{"French", "fr"},
		{"  Spanish  ", "es"},
		{"japanese", "ja"},
		{"fr", "fr"},      // raw 2-letter code accepted as-is
		{"zh-CN", "zh-cn"}, // passthrough lowercases to the code we store
		{"", ""},
		{"not-a-language-name-that-is-too-long", ""},
	}
	for _, c := range cases {
		got := resolveLanguage(c.in)
		if got != c.want {
			t.Errorf("resolveLanguage(%q): expected %q, got %q", c.in, c.want, got)
		}
	}
}

func TestTranslatorEnabledGates(t *testing.T) {
	saved := config
	defer func() { config = saved }()

	// No config → disabled.
	config = nil
	if translatorEnabled() {
		t.Error("translatorEnabled() should be false when config is nil")
	}

	// Feature flag off → disabled.
	cfg := settings.DefaultConfig()
	cfg.EnableTranslator = false
	cfg.TranslatorAPIKey = "key"
	config = cfg
	if translatorEnabled() {
		t.Error("translatorEnabled() should be false when EnableTranslator is false")
	}

	// Flag on but no key → disabled.
	cfg.EnableTranslator = true
	cfg.TranslatorAPIKey = ""
	if translatorEnabled() {
		t.Error("translatorEnabled() should be false when API key is blank")
	}

	// Flag on, key set → enabled.
	cfg.TranslatorAPIKey = "somekey"
	if !translatorEnabled() {
		t.Error("translatorEnabled() should be true when flag on and key set")
	}

	// Empty URL → disabled (guards accidental mis-config).
	cfg.TranslatorAPIURL = ""
	if translatorEnabled() {
		t.Error("translatorEnabled() should be false when URL is blank")
	}
}

// TestApplyTranslatorFallback verifies that applyTranslator returns the
// original text unchanged whenever the feature is disabled, so a
// mis-configured server never swallows a player's message.
func TestApplyTranslatorFallback(t *testing.T) {
	saved := config
	defer func() { config = saved }()
	config = nil

	in := "Hello, this is a test."
	got := applyTranslator(in, "french")
	if got != in {
		t.Errorf("applyTranslator should return input unchanged when disabled: got %q", got)
	}
}
