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

package logger

import (
	"regexp"
	"strings"
	"sync"
	"testing"
)

// TestLogFormatPreserved is the contract test for the pooled-buffer log
// path: the on-wire format must still be "<time>: LEVEL: <msg>\n" so that
// existing log parsers and operator tooling keep working unchanged.
func TestLogFormatPreserved(t *testing.T) {
	// Capture log output via the TUITap hook so we do not need to redirect
	// os.Stdout in a test. The tap receives the exact bytes a stdout writer
	// would see.
	var (
		mu  sync.Mutex
		got string
	)
	prev := TUITap
	TUITap = func(s string) {
		mu.Lock()
		got = s
		mu.Unlock()
	}
	defer func() { TUITap = prev }()

	prevLevel := CurrentLevel
	CurrentLevel = Info
	defer func() { CurrentLevel = prevLevel }()

	LogInfo("hello there")
	mu.Lock()
	captured := got
	mu.Unlock()

	// time.StampMilli = "Jan _2 15:04:05.000" — 3-letter month, right-aligned
	// day, 24-hour clock, 3-digit ms. Regex is intentionally strict so a drift
	// in framing is caught here rather than silently breaking downstream.
	re := regexp.MustCompile(`^[A-Z][a-z]{2} [ 0-9][0-9] [0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}: INFO: hello there\n$`)
	if !re.MatchString(captured) {
		t.Fatalf("format drift: %q", captured)
	}
}

// TestLogFormatAllLevels walks every level to make sure the label lookup
// is indexed correctly and no level silently drops.
func TestLogFormatAllLevels(t *testing.T) {
	var (
		mu       sync.Mutex
		captured []string
	)
	prev := TUITap
	TUITap = func(s string) {
		mu.Lock()
		captured = append(captured, s)
		mu.Unlock()
	}
	defer func() { TUITap = prev }()

	prevLevel := CurrentLevel
	CurrentLevel = Info
	defer func() { CurrentLevel = prevLevel }()

	LogInfo("a")
	LogWarning("b")
	LogError("c")
	LogFatal("d")

	mu.Lock()
	got := append([]string{}, captured...)
	mu.Unlock()

	if len(got) != 4 {
		t.Fatalf("captured %d lines, want 4", len(got))
	}
	want := []string{": INFO: a\n", ": WARN: b\n", ": ERROR: c\n", ": FATAL: d\n"}
	for i, suffix := range want {
		if !strings.HasSuffix(got[i], suffix) {
			t.Errorf("line %d suffix: got %q, want suffix %q", i, got[i], suffix)
		}
	}
}
