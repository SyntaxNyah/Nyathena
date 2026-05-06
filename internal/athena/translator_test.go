package athena

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
		{"french", "FR"},
		{"French", "FR"},
		{"  Spanish  ", "ES"},
		{"japanese", "JA"},
		{"fr", "FR"},       // raw 2-letter code accepted and uppercased
		{"zh-CN", "ZH-CN"}, // passthrough uppercases to DeepL wire format
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

// resetTranslatorState clears cache + breaker between tests so they don't
// contaminate each other.
func resetTranslatorState() {
	translatorCache.mu.Lock()
	translatorCache.m = make(map[string]string)
	translatorCache.mu.Unlock()
	translatorFailStreak.Store(0)
	translatorOpenUntil.Store(0)
}

// TestApplyTranslatorSingleLang spins up a fake DeepL-style server and
// verifies single-language translation flows end-to-end via applyTranslator.
func TestApplyTranslatorSingleLang(t *testing.T) {
	saved := config
	defer func() { config = saved }()
	resetTranslatorState()

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// DeepL response shape; translations[] length must match the number of
		// `text=` params in the request (one here, for the single-string call).
		w.Write([]byte(`{"translations":[{"detected_source_language":"EN","text":"bonjour"}]}`))
	}))
	defer ts.Close()

	cfg := settings.DefaultConfig()
	cfg.EnableTranslator = true
	cfg.TranslatorAPIURL = ts.URL
	cfg.TranslatorAPIKey = "test-key"
	config = cfg

	got := applyTranslator("hello", "french")
	if got != "bonjour" {
		t.Errorf("expected %q, got %q", "bonjour", got)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 upstream call, got %d", calls.Load())
	}

	// Second call hits the cache and makes no new request.
	got = applyTranslator("hello", "french")
	if got != "bonjour" {
		t.Errorf("cached path returned %q", got)
	}
	if calls.Load() != 1 {
		t.Errorf("cache should have absorbed the 2nd call, got %d upstream calls", calls.Load())
	}
}

// TestApplyTranslatorBudget guarantees that a hung upstream cannot stall the
// IC pipeline for longer than the overall budget.  Key lag-avoidance test.
func TestApplyTranslatorBudget(t *testing.T) {
	saved := config
	defer func() { config = saved }()
	resetTranslatorState()

	// unblock signals the fake server to stop hanging so t.Cleanup can return
	// promptly after the applyTranslator call has already returned.
	unblock := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-unblock:
		case <-time.After(10 * time.Second):
		}
	}))
	t.Cleanup(func() {
		close(unblock)
		ts.Close()
	})

	cfg := settings.DefaultConfig()
	cfg.EnableTranslator = true
	cfg.TranslatorAPIURL = ts.URL
	cfg.TranslatorAPIKey = "test-key"
	config = cfg

	start := time.Now()
	got := applyTranslator("stall me please", "french")
	elapsed := time.Since(start)

	// Generous slack (2x budget) to tolerate CI jitter; the point is that
	// elapsed is bounded and not the full 10s the server would otherwise take.
	maxWait := 2 * translatorOverallBudget
	if elapsed > maxWait {
		t.Errorf("applyTranslator took %v; expected under %v", elapsed, maxWait)
	}
	if got != "stall me please" {
		t.Errorf("expected fallback to original text, got %q", got)
	}
}

// TestTranslatorCircuitBreaker verifies that repeated failures open the
// breaker and subsequent calls short-circuit instead of hitting the upstream.
func TestTranslatorCircuitBreaker(t *testing.T) {
	saved := config
	defer func() { config = saved }()
	resetTranslatorState()

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := settings.DefaultConfig()
	cfg.EnableTranslator = true
	cfg.TranslatorAPIURL = ts.URL
	cfg.TranslatorAPIKey = "test-key"
	config = cfg

	// Trigger enough failures to trip the breaker. Unique inputs avoid the
	// cache short-circuit.
	for i := 0; i < translatorFailThreshold; i++ {
		applyTranslator("call-"+string(rune('A'+i)), "french")
	}
	if !breakerOpen() {
		t.Fatalf("expected circuit breaker to be open after %d failures", translatorFailThreshold)
	}

	before := calls.Load()
	// Breaker-open calls must not hit the upstream.
	applyTranslator("should-not-reach-upstream", "french")
	if calls.Load() != before {
		t.Errorf("breaker-open call leaked to upstream: before=%d after=%d", before, calls.Load())
	}
}

// TestCheckAndUpdateTranslateCooldown verifies that the per-client cooldown
// gate correctly allows the first call and blocks subsequent calls within the
// cooldown window.
func TestCheckAndUpdateTranslateCooldown(t *testing.T) {
	c := &Client{}

	// First call should always be allowed.
	ok, remaining := c.CheckAndUpdateTranslateCooldown(25 * time.Second)
	if !ok {
		t.Fatal("first CheckAndUpdateTranslateCooldown call should be allowed")
	}
	if remaining != 0 {
		t.Errorf("remaining should be 0 on first allowed call, got %v", remaining)
	}

	// Immediate second call must be denied.
	ok, remaining = c.CheckAndUpdateTranslateCooldown(25 * time.Second)
	if ok {
		t.Fatal("second CheckAndUpdateTranslateCooldown call within window should be denied")
	}
	if remaining <= 0 || remaining > 25*time.Second {
		t.Errorf("remaining should be between 0 and 25s, got %v", remaining)
	}
}
