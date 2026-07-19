/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /terminal. */

package athena

import (
	"strconv"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// seedLoggerForTerminalTest makes sure Info-level lines actually reach
// logger.RecentLines and restores CurrentLevel afterwards. It deliberately
// does not reset the ring buffer itself (logger has no exported reset, and
// other tests/log lines may already be in it) -- each test below only
// asserts on the count of its own uniquely-prefixed lines, which are
// guaranteed to be the most recent ones since nothing else logs between the
// seeding loop and the /terminal call in a non-parallel test.
func seedLoggerForTerminalTest(t *testing.T) {
	t.Helper()
	prevLevel := logger.CurrentLevel
	logger.CurrentLevel = logger.Info
	t.Cleanup(func() { logger.CurrentLevel = prevLevel })
}

func TestCmdTerminalDefaultLineCount(t *testing.T) {
	seedLoggerForTerminalTest(t)
	for i := 0; i < defaultTerminalLines+20; i++ {
		logger.LogInfo("terminal-default-test-" + strconv.Itoa(i))
	}

	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, ipid: "ip-term", char: -1, area: makeTestArea("Courtroom")}
	cmdTerminal(client, nil, "usage")

	got := strings.Count(conn.String(), "terminal-default-test-")
	if got != defaultTerminalLines {
		t.Fatalf("/terminal with no args showed %d matching lines, want default %d", got, defaultTerminalLines)
	}
}

func TestCmdTerminalRespectsCap(t *testing.T) {
	seedLoggerForTerminalTest(t)
	for i := 0; i < maxTerminalLines+50; i++ {
		logger.LogInfo("terminal-cap-test-" + strconv.Itoa(i))
	}

	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, ipid: "ip-term", char: -1, area: makeTestArea("Courtroom")}
	cmdTerminal(client, []string{"999999"}, "usage")

	got := strings.Count(conn.String(), "terminal-cap-test-")
	if got != maxTerminalLines {
		t.Fatalf("/terminal 999999 showed %d matching lines, want capped at %d", got, maxTerminalLines)
	}
}

func TestCmdTerminalExplicitCount(t *testing.T) {
	seedLoggerForTerminalTest(t)
	for i := 0; i < 30; i++ {
		logger.LogInfo("terminal-explicit-test-" + strconv.Itoa(i))
	}

	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, ipid: "ip-term", char: -1, area: makeTestArea("Courtroom")}
	cmdTerminal(client, []string{"7"}, "usage")

	got := strings.Count(conn.String(), "terminal-explicit-test-")
	if got != 7 {
		t.Fatalf("/terminal 7 showed %d matching lines, want 7", got)
	}
}

func TestCmdTerminalInvalidArg(t *testing.T) {
	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, ipid: "ip-term", char: -1, area: makeTestArea("Courtroom")}
	cmdTerminal(client, []string{"not-a-number"}, "Usage: /terminal [lines]")

	out := conn.String()
	if !strings.Contains(out, "Invalid line count") {
		t.Errorf("expected an invalid-argument message, got %q", out)
	}
}

func TestCmdTerminalZeroOrNegativeArg(t *testing.T) {
	for _, arg := range []string{"0", "-5"} {
		conn := &captureConn{}
		client := &Client{conn: conn, uid: 1, ipid: "ip-term", char: -1, area: makeTestArea("Courtroom")}
		cmdTerminal(client, []string{arg}, "Usage: /terminal [lines]")
		if !strings.Contains(conn.String(), "Invalid line count") {
			t.Errorf("/terminal %s: expected an invalid-argument message, got %q", arg, conn.String())
		}
	}
}
