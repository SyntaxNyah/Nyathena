/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the RecentLines ring buffer backing the
   in-game /terminal admin command. */

package logger

import (
	"fmt"
	"strings"
	"testing"
)

// resetRecentLines clears the ring buffer and restores CurrentLevel to Info
// so every logged line in a test is guaranteed to pass the level filter.
// Other tests in this package also log through LogInfo/LogWarning/etc., so
// isolation matters here.
func resetRecentLines(t *testing.T) {
	t.Helper()
	recentLinesMu.Lock()
	recentLines = nil
	recentLinesMu.Unlock()
	prevLevel := CurrentLevel
	CurrentLevel = Info
	t.Cleanup(func() {
		CurrentLevel = prevLevel
		recentLinesMu.Lock()
		recentLines = nil
		recentLinesMu.Unlock()
	})
}

func TestRecentLinesOrderingAndCount(t *testing.T) {
	resetRecentLines(t)

	for i := 0; i < 5; i++ {
		LogInfo(fmt.Sprintf("line-%d", i))
	}

	got := RecentLines(3)
	if len(got) != 3 {
		t.Fatalf("RecentLines(3) returned %d lines, want 3", len(got))
	}
	for i, want := range []string{"line-2", "line-3", "line-4"} {
		if !strings.Contains(got[i], want) {
			t.Errorf("line %d = %q, want to contain %q (oldest-first ordering)", i, got[i], want)
		}
	}
}

func TestRecentLinesMoreThanAvailable(t *testing.T) {
	resetRecentLines(t)

	LogInfo("only-one")
	got := RecentLines(50)
	if len(got) != 1 {
		t.Fatalf("RecentLines(50) with 1 logged line returned %d, want 1", len(got))
	}
	if !strings.Contains(got[0], "only-one") {
		t.Errorf("got %q, want to contain \"only-one\"", got[0])
	}
}

func TestRecentLinesZeroOrNegative(t *testing.T) {
	resetRecentLines(t)

	LogInfo("something")
	if got := RecentLines(0); got != nil {
		t.Errorf("RecentLines(0) = %v, want nil", got)
	}
	if got := RecentLines(-5); got != nil {
		t.Errorf("RecentLines(-5) = %v, want nil", got)
	}
}

func TestRecentLinesEmptyBuffer(t *testing.T) {
	resetRecentLines(t)
	if got := RecentLines(10); got != nil {
		t.Errorf("RecentLines(10) on an empty buffer = %v, want nil", got)
	}
}

// TestRecentLinesCapEviction verifies the ring buffer never grows past
// maxRecentLogLines and that eviction drops the oldest entries first.
func TestRecentLinesCapEviction(t *testing.T) {
	resetRecentLines(t)

	total := maxRecentLogLines + 10
	for i := 0; i < total; i++ {
		LogInfo(fmt.Sprintf("line-%d", i))
	}

	got := RecentLines(maxRecentLogLines + 100) // ask for more than the cap allows
	if len(got) != maxRecentLogLines {
		t.Fatalf("ring buffer returned %d lines, want capped at %d", len(got), maxRecentLogLines)
	}
	wantOldest := fmt.Sprintf("line-%d", total-maxRecentLogLines)
	if !strings.Contains(got[0], wantOldest) {
		t.Errorf("oldest surviving line = %q, want to contain %q (lines before it should have been evicted)", got[0], wantOldest)
	}
}

// TestRecentLinesRespectsLogLevel verifies lines filtered out by CurrentLevel
// never reach the ring buffer either -- /terminal should show exactly what
// the configured log level would have written to stdout/file.
func TestRecentLinesRespectsLogLevel(t *testing.T) {
	resetRecentLines(t)
	CurrentLevel = Error

	LogInfo("should be filtered")
	LogWarning("should also be filtered")
	LogError("kept")

	got := RecentLines(10)
	if len(got) != 1 {
		t.Fatalf("RecentLines returned %d lines, want 1 (only Error+ should pass CurrentLevel)", len(got))
	}
	if !strings.Contains(got[0], "kept") {
		t.Errorf("got %q, want to contain \"kept\"", got[0])
	}
}
