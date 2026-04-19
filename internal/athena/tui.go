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

package athena

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// The TUI is a stdlib-only, read-only operator dashboard that repaints a
// status panel to stdout every refresh tick. It deliberately avoids external
// dependencies (bubbletea, tcell, etc.) so adding it cannot perturb the
// dependency tree, and it does not accept keystroke input so it cannot be
// mis-driven into a bad server state.
//
// Usage:
//   go athena.StartTUI(ctx)
//
// When the TUI is running, callers should also pass -nocli (or set LogStdOut
// false) so the repaints do not interleave with stdin command echoes and
// stdout log lines. The TUI itself does NOT disable stdout logging; that is
// the caller's decision.

const (
	// tuiRefresh is how often the dashboard repaints. Kept conservative to
	// avoid flicker on slow terminals and to keep CPU cost negligible.
	tuiRefresh = 2 * time.Second

	// ansiClear clears the screen and homes the cursor. \x1b[H moves the
	// cursor to row 1 col 1; \x1b[2J erases the entire display.
	ansiClear = "\x1b[H\x1b[2J"
	// ansiReset restores default attributes (color, bold, etc.) at teardown.
	ansiReset = "\x1b[0m"
	// ansiBold / ansiDim are used only as cosmetic accents.
	ansiBold = "\x1b[1m"
	ansiDim  = "\x1b[2m"
)

// tuiLogTail captures the last N log lines so the TUI can render them as a
// scrolling feed beneath the status dashboard. It's a tiny ring buffer so
// there is no growth over time. Protected by its own mutex so the TUI paint
// goroutine can read while logger writes append.
type tuiLogRing struct {
	mu    sync.Mutex
	lines []string // up to tuiLogMaxLines entries
}

const tuiLogMaxLines = 12

var tuiRing = &tuiLogRing{lines: make([]string, 0, tuiLogMaxLines)}

// tuiAppendLog is installed as logger.TUITap while the TUI is running, so
// the dashboard can show a recent-log pane without fighting the logger for
// stdout. The logger clears the tap on shutdown.
func tuiAppendLog(line string) {
	tuiRing.mu.Lock()
	if len(tuiRing.lines) == tuiLogMaxLines {
		copy(tuiRing.lines, tuiRing.lines[1:])
		tuiRing.lines = tuiRing.lines[:tuiLogMaxLines-1]
	}
	tuiRing.lines = append(tuiRing.lines, line)
	tuiRing.mu.Unlock()
}

func (r *tuiLogRing) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// StartTUI begins the repainting loop. It blocks until stop is closed (so the
// caller should typically invoke it from its own goroutine). Cleanup restores
// terminal attributes and the logger tap before returning.
func StartTUI(stop <-chan struct{}) {
	// Install the log tap and disable stdout logging while the TUI owns the
	// screen. Both are restored on return.
	prevTap := logger.TUITap
	logger.TUITap = tuiAppendLog
	wasStdOut := logger.LogStdOut
	logger.LogStdOut = false
	defer func() {
		logger.TUITap = prevTap
		logger.LogStdOut = wasStdOut
		fmt.Print(ansiReset + "\n")
	}()

	paint()
	ticker := time.NewTicker(tuiRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			paint()
		}
	}
}

// paint renders one frame of the dashboard to stdout.
// Deliberately keeps data access read-only: it never mutates Client, Area,
// or any server state.
func paint() {
	var b strings.Builder
	b.Grow(4096)
	b.WriteString(ansiClear)

	// Header line: server name, version, uptime, connected player count.
	name := "Unnamed"
	max := 0
	if config != nil {
		name = config.Name
		max = config.MaxPlayers
	}
	b.WriteString(ansiBold)
	b.WriteString(fmt.Sprintf("Athena %s  |  %s\n", version, name))
	b.WriteString(ansiReset)
	b.WriteString(strings.Repeat("-", 60))
	b.WriteByte('\n')

	// Player stats.
	online := 0
	if clients != nil {
		online = clients.Count()
	}
	b.WriteString(fmt.Sprintf("Players:    %d / %d\n", online, max))
	b.WriteString(fmt.Sprintf("Time (UTC): %s\n", time.Now().UTC().Format("2006-01-02 15:04:05")))
	b.WriteByte('\n')

	// Area table.
	b.WriteString(ansiBold)
	b.WriteString("Areas\n")
	b.WriteString(ansiReset)
	if len(areas) == 0 {
		b.WriteString(ansiDim + "  (no areas loaded)\n" + ansiReset)
	} else {
		type row struct {
			name  string
			count int
		}
		rows := make([]row, 0, len(areas))
		for _, a := range areas {
			rows = append(rows, row{name: a.Name(), count: a.PlayerCount()})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("  %-30s  %3d\n", trunc(r.name, 30), r.count))
		}
	}
	b.WriteByte('\n')

	// Log tail.
	b.WriteString(ansiBold)
	b.WriteString("Recent log\n")
	b.WriteString(ansiReset)
	snap := tuiRing.snapshot()
	if len(snap) == 0 {
		b.WriteString(ansiDim + "  (no entries yet)\n" + ansiReset)
	} else {
		for _, line := range snap {
			b.WriteString("  ")
			b.WriteString(strings.TrimRight(line, "\n"))
			b.WriteByte('\n')
		}
	}

	b.WriteString(ansiDim)
	b.WriteString(fmt.Sprintf("\nRefresh %s — TUI is read-only; send SIGINT to stop the server.\n",
		tuiRefresh))
	b.WriteString(ansiReset)

	_, _ = os.Stdout.WriteString(b.String())
}

// trunc limits a string to n runes, appending a single ellipsis rune if it
// had to cut. Used so long area names don't wrap and break the table layout.
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
