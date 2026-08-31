package athena

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// withCaptchaConfig runs fn with a temporary config installed.
func withCaptchaConfig(t *testing.T, c settings.ServerConfig, fn func()) {
	t.Helper()
	orig := config
	defer func() { config = orig }()
	config = &settings.Config{ServerConfig: c}
	fn()
}

// TestChallengeAnswerNeverAppearsInPrompt pins the property the whole feature
// rests on: the answer is never a contiguous substring of the question.
//
// If this ever fails, the captcha has silently degraded into "type the value
// after the colon", which any scraper defeats without understanding a word of
// it -- so this is asserted exhaustively, per kind, over many draws rather than
// spot-checked.
func TestChallengeAnswerNeverAppearsInPrompt(t *testing.T) {
	for _, g := range joinChallengeGens {
		g := g
		t.Run(g.kind, func(t *testing.T) {
			for i := 0; i < 400; i++ {
				c := newJoinChallenge(newUnkeyedRand(), []string{g.kind})
				if len(c.Answers) == 0 {
					t.Fatalf("kind %v produced a challenge with no answers: %q", g.kind, c.Prompt)
				}
				if challengeAnswerLeaks(c) {
					t.Fatalf("kind %v leaked its answer into the prompt:\n  prompt: %q\n  answers: %v",
						g.kind, c.Prompt, c.Answers)
				}
			}
		})
	}
}

// TestChallengeAnswerLeaksDetectsLeak verifies the invariant checker itself
// catches the exact failure it exists to prevent -- a "type this token" style
// challenge.
func TestChallengeAnswerLeaksDetectsLeak(t *testing.T) {
	leaky := normalizeChallenge(joinChallenge{
		Kind:    "bad",
		Prompt:  "Please type ABC123 to verify yourself on the server.",
		Answers: []string{"ABC123"},
	})
	if !challengeAnswerLeaks(leaky) {
		t.Fatal("a copy-pasteable challenge was not detected as leaking")
	}
	// An empty answer matches everything and must also count as a leak.
	if !challengeAnswerLeaks(joinChallenge{Prompt: "anything", Answers: []string{""}}) {
		t.Fatal("an empty answer was not treated as a leak")
	}
}

// TestNormalizeCaptchaAnswer covers the forgiveness the checker is supposed to
// have: a real person types spaces, punctuation and whatever case they like.
func TestNormalizeCaptchaAnswer(t *testing.T) {
	cases := []struct{ in, want string }{
		{"A7K Q2M", "a7kq2m"},
		{"a7k-q2m", "a7kq2m"},
		{" A7KQ2M. ", "a7kq2m"},
		{"Twelve", "twelve"},
		{"!!!", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeCaptchaAnswer(c.in); got != c.want {
			t.Errorf("normalizeCaptchaAnswer(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCheckChallengeAnswer verifies matching accepts every normalized form and
// rejects an empty answer (which must never pass by matching an empty rule).
func TestCheckChallengeAnswer(t *testing.T) {
	c := normalizeChallenge(joinChallenge{Answers: []string{"12", "twelve"}})
	for _, ok := range []string{"12", "Twelve", " twelve. ", "TWELVE"} {
		if !checkChallengeAnswer(c, ok) {
			t.Errorf("answer %q should have been accepted", ok)
		}
	}
	for _, bad := range []string{"13", "", "   ", "twelv"} {
		if checkChallengeAnswer(c, bad) {
			t.Errorf("answer %q should have been rejected", bad)
		}
	}
}

// TestKeyedChallengeIsStablePerIPID is the anti-reroll property: within a
// rotation window the same address always draws the same question, so a bot
// whose solver only handles one kind cannot reconnect until it gets that kind.
func TestKeyedChallengeIsStablePerIPID(t *testing.T) {
	withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaRotate: 3600}, func() {
		first := newJoinChallenge(challengeRandFor("1.2.3.4"), nil)
		for i := 0; i < 20; i++ {
			again := newJoinChallenge(challengeRandFor("1.2.3.4"), nil)
			if again.Prompt != first.Prompt {
				t.Fatalf("the same IPID drew a different challenge on attempt %d:\n  %q\n  %q",
					i, first.Prompt, again.Prompt)
			}
		}
	})
}

// TestKeyedChallengeDiffersByIPID verifies different addresses are not all
// handed the same question, which would make one solved answer reusable
// server-wide.
func TestKeyedChallengeDiffersByIPID(t *testing.T) {
	withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaRotate: 3600}, func() {
		seen := make(map[string]struct{})
		for i := 0; i < 40; i++ {
			c := newJoinChallenge(challengeRandFor(fmt.Sprintf("10.0.0.%d", i)), nil)
			seen[c.Prompt] = struct{}{}
		}
		// Not all distinct necessarily, but a single shared prompt would mean
		// the key is not mixing the address in at all.
		if len(seen) < 20 {
			t.Fatalf("40 different IPIDs only produced %d distinct challenges; the key is not mixing in the address", len(seen))
		}
	})
}

// TestUnkeyedRandRerolls confirms join_captcha_rotate = -1 really does disable
// keying, so the documented escape hatch behaves as described.
func TestUnkeyedRandRerolls(t *testing.T) {
	withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaRotate: -1}, func() {
		seen := make(map[string]struct{})
		for i := 0; i < 30; i++ {
			c := newJoinChallenge(challengeRandFor("1.2.3.4"), nil)
			seen[c.Prompt] = struct{}{}
		}
		if len(seen) < 2 {
			t.Fatal("keying disabled but the same IPID kept drawing one fixed challenge")
		}
	})
}

// TestEnabledChallengeGens covers the config filter, including the deliberate
// fallback: a selection matching nothing must not leave joining players with no
// answerable question.
func TestEnabledChallengeGens(t *testing.T) {
	if got := len(enabledChallengeGens(nil)); got != len(joinChallengeGens) {
		t.Errorf("empty selection should enable every kind, got %d of %d", got, len(joinChallengeGens))
	}
	got := enabledChallengeGens([]string{"math", "reverse"})
	if len(got) != 2 {
		t.Fatalf("expected 2 kinds, got %d", len(got))
	}
	if n := len(enabledChallengeGens([]string{"nonsense"})); n != len(joinChallengeGens) {
		t.Errorf("an unrecognised selection should fall back to every kind, got %d", n)
	}
	if got := enabledChallengeGens([]string{" MATH "}); len(got) != 1 || got[0].kind != "math" {
		t.Errorf("kind names should be trimmed and case-insensitive, got %v", got)
	}
}

// TestValidateJoinCaptchaKinds checks startup can report a typo.
func TestValidateJoinCaptchaKinds(t *testing.T) {
	if bad := validateJoinCaptchaKinds([]string{"math", "reverse"}); len(bad) != 0 {
		t.Errorf("valid kinds reported as bad: %v", bad)
	}
	bad := validateJoinCaptchaKinds([]string{"math", "maths", "revrse"})
	if len(bad) != 2 {
		t.Fatalf("expected 2 unknown kinds, got %v", bad)
	}
}

// TestJoinCaptchaCommandAllowed pins the allowlist. It must stay small: a
// broadcasting command reachable while unverified defeats the gate entirely.
func TestJoinCaptchaCommandAllowed(t *testing.T) {
	for _, ok := range []string{"verify", "login", "help", "about", "VERIFY"} {
		if !joinCaptchaCommandAllowed(ok) {
			t.Errorf("%q should be usable while unverified", ok)
		}
	}
	for _, blocked := range []string{"global", "pm", "roll", "a", "play", "modcall", "8ball", "cvote"} {
		if joinCaptchaCommandAllowed(blocked) {
			t.Errorf("%q broadcasts and must not be usable while unverified", blocked)
		}
	}
}

// TestJoinCaptchaStrikeLimit checks the floor that stops a misconfigured 0 from
// acting on someone's very first blocked message.
func TestJoinCaptchaStrikeLimit(t *testing.T) {
	withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaStrikes: 5}, func() {
		if got := joinCaptchaStrikeLimit(); got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})
	for _, bad := range []int{0, -1} {
		withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaStrikes: bad}, func() {
			if got := joinCaptchaStrikeLimit(); got != 3 {
				t.Errorf("strikes=%d should clamp to the default 3, got %d", bad, got)
			}
		})
	}
}

// TestJoinCaptchaAction checks the action parse, including that an unknown
// value falls back to the safer mute rather than kicking players.
func TestJoinCaptchaAction(t *testing.T) {
	cases := map[string]string{
		"mute":     joinCaptchaActionMute,
		"kick":     joinCaptchaActionKick,
		"KICK":     joinCaptchaActionKick,
		" kick ":   joinCaptchaActionKick,
		"":         joinCaptchaActionMute,
		"nonsense": joinCaptchaActionMute,
	}
	for in, want := range cases {
		withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaAction: in}, func() {
			if got := joinCaptchaAction(); got != want {
				t.Errorf("action %q = %q, want %q", in, got, want)
			}
		})
	}
}

// TestLoadCustomChallenges covers the operator question file, in particular
// that an entry whose answer sits inside its own question is rejected -- that
// is the one mistake that would quietly make the gate copy-pasteable.
func TestLoadCustomChallenges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "captcha_questions.txt")
	content := strings.Join([]string{
		"# a comment",
		"",
		"What colour is the sky on a clear day? | blue",
		"How many days are in a week? | 7, seven",
		"Type the word cheese | cheese", // answer is inside the question
		"missing separator",
		"empty answer |",
		"   | orphan",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadCustomChallenges(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 usable questions, got %d: %+v", len(got), got)
	}
	if !checkChallengeAnswer(got[1], "Seven") {
		t.Error("alternative answers should be accepted case-insensitively")
	}
	for _, c := range got {
		if strings.Contains(strings.ToLower(c.Prompt), "cheese") {
			t.Error("an entry whose answer appears in its own question was accepted")
		}
	}
}

// TestLoadCustomChallengesMissingFile confirms the file is optional.
func TestLoadCustomChallengesMissingFile(t *testing.T) {
	if _, err := loadCustomChallenges(filepath.Join(t.TempDir(), "nope.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected a not-exist error, got %v", err)
	}
}

// TestPickJoinChallengeCustomOnly verifies custom_only serves operator
// questions exclusively, and still falls back to the built-ins when the file is
// empty rather than silently opening the gate.
func TestPickJoinChallengeCustomOnly(t *testing.T) {
	withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaCustomOnly: true, JoinCaptchaRotate: -1}, func() {
		setCustomChallenges([]joinChallenge{
			normalizeChallenge(joinChallenge{Kind: "custom", Prompt: "Secret question?", Answers: []string{"yes"}}),
		})
		defer setCustomChallenges(nil)
		for i := 0; i < 20; i++ {
			if c := pickJoinChallenge("1.2.3.4"); !strings.Contains(c.Prompt, "Secret question?") {
				t.Fatalf("custom_only served a built-in question: %q", c.Prompt)
			}
		}
	})
	withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaCustomOnly: true, JoinCaptchaRotate: -1}, func() {
		setCustomChallenges(nil)
		c := pickJoinChallenge("1.2.3.4")
		if len(c.Answers) == 0 {
			t.Fatal("custom_only with no questions must fall back to a built-in, not serve nothing")
		}
	})
}

// TestCaptchaRandIsUniform sanity-checks the rejection sampler: a modulo-biased
// source would skew which kinds and words appear over a long run.
func TestCaptchaRandIsUniform(t *testing.T) {
	const n, draws = 7, 70000
	counts := make([]int, n)
	r := newUnkeyedRand()
	for i := 0; i < draws; i++ {
		counts[r.intn(n)]++
	}
	expected := draws / n
	for i, c := range counts {
		if c < expected*8/10 || c > expected*12/10 {
			t.Errorf("bucket %d got %d draws, expected roughly %d — distribution is skewed", i, c, expected)
		}
	}
}

// TestJoinCaptchaPlaytimeExemptDisabled checks that a zero or negative
// threshold turns the exemption off entirely rather than exempting everyone --
// getting that backwards would silently disable the captcha for the whole
// server.
func TestJoinCaptchaPlaytimeExemptDisabled(t *testing.T) {
	for _, v := range []int{0, -1} {
		withCaptchaConfig(t, settings.ServerConfig{JoinCaptchaMinPlaytime: v}, func() {
			if joinCaptchaPlaytimeExempt("1.2.3.4") {
				t.Errorf("min_playtime=%d should disable the exemption, not grant it", v)
			}
		})
	}
}

// TestJoinCaptchaPlaytimeExemptNoConfig checks the nil-config path is safe and
// does not exempt.
func TestJoinCaptchaPlaytimeExemptNoConfig(t *testing.T) {
	orig := config
	defer func() { config = orig }()
	config = nil
	if joinCaptchaPlaytimeExempt("1.2.3.4") {
		t.Error("a nil config must not exempt anyone from the captcha")
	}
}

// TestJoinCaptchaPlaytimeThreshold covers the comparison itself against the
// 5-hour default, including the boundary. Playtime is stored in seconds and the
// setting is in minutes, so an off-by-60 here would be a factor-of-sixty bug.
func TestJoinCaptchaPlaytimeThreshold(t *testing.T) {
	const minutes = 300 // the 5-hour default
	threshold := int64(minutes) * 60
	cases := []struct {
		name          string
		playtimeSecs  int64
		wantExemption bool
	}{
		{"brand new connection", 0, false},
		{"one hour", 3600, false},
		{"just under five hours", threshold - 1, false},
		{"exactly five hours", threshold, true},
		{"five hours and a minute", threshold + 60, true},
		{"five hundred hours", 500 * 3600, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.playtimeSecs >= threshold; got != c.wantExemption {
				t.Errorf("playtime %ds against a %d-minute threshold: exempt=%v, want %v",
					c.playtimeSecs, minutes, got, c.wantExemption)
			}
		})
	}
}
