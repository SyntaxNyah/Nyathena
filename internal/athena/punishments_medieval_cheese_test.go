/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /medieval and /cheese. */

package athena

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestMedievalAndCheeseProduceOutput sanity-checks both transforms over many
// rolls: non-empty, within the IC budget, valid UTF-8.
func TestMedievalAndCheeseProduceOutput(t *testing.T) {
	input := "Hey friend, you are really good and I think this is awesome today."
	for name, fn := range map[string]func(string) string{
		"medieval": applyMedieval,
		"cheese":   applyCheese,
	} {
		for i := 0; i < 100; i++ {
			out := fn(input)
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s produced empty output", name)
			}
			if len(out) > icBudget() {
				t.Fatalf("%s produced %d bytes, exceeding IC budget %d", name, len(out), icBudget())
			}
			if !utf8.ValidString(out) {
				t.Fatalf("%s produced invalid UTF-8: %q", name, out)
			}
		}
	}
}

// TestMedievalDispatch confirms the transform is reachable via the dispatcher
// and that it actually rewrites text.
func TestMedievalDispatch(t *testing.T) {
	input := "you are my friend"
	changed := false
	for i := 0; i < 30; i++ {
		if ApplyPunishmentToText(input, PunishmentMedieval) != input {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("ApplyPunishmentToText(PunishmentMedieval) never altered the input — missing dispatch case?")
	}
}

// TestCheeseDispatchReplaces confirms /cheese discards the input entirely and
// substitutes a line from the corpus.
func TestCheeseDispatchReplaces(t *testing.T) {
	pool := map[string]bool{}
	for _, s := range cheeseStatements {
		pool[s] = true
	}
	for i := 0; i < 50; i++ {
		out := ApplyPunishmentToText("this should be discarded", PunishmentCheese)
		if out == "this should be discarded" {
			t.Fatal("cheese did not replace the input")
		}
		if !pool[out] {
			t.Fatalf("cheese emitted %q which is not in the corpus", out)
		}
	}
}

// TestMedievalVariationCount pins the documented "100+ combinations" claim.
func TestMedievalVariationCount(t *testing.T) {
	if n := medievalVariationCount(); n < 100 {
		t.Errorf("medieval herald×flourish combinations dropped to %d; want >= 100", n)
	}
}

// TestCheeseCorpusSize pins the documented "100+ statements" claim and ensures
// no two lines are identical.
func TestCheeseCorpusSize(t *testing.T) {
	if n := cheeseLineCount(); n < 100 {
		t.Errorf("cheese corpus shrank to %d statements; want >= 100", n)
	}
	seen := map[string]bool{}
	for _, s := range cheeseStatements {
		if seen[s] {
			t.Errorf("duplicate cheese statement: %q", s)
		}
		seen[s] = true
	}
}
