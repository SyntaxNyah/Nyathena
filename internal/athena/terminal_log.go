/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork addition: /terminal, an in-game window onto the server's
   console/log output for admins who don't have shell access to the host. */

package athena

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

const (
	defaultTerminalLines = 50
	maxTerminalLines     = 500
)

// cmdTerminal handles /terminal [lines]. Prints the most recent N formatted
// log lines from the logger's in-memory ring buffer (logger.RecentLines) --
// the same lines that appear on the server's stdout/log file -- as a single
// OOC message. Defaults to 50 lines when no argument is given; capped at 500
// so a single request can't be used to force a huge send.
func cmdTerminal(client *Client, args []string, usage string) {
	n := defaultTerminalLines
	if len(args) > 0 {
		v, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil || v <= 0 {
			client.SendServerMessage("Invalid line count:\n" + usage)
			return
		}
		n = v
	}
	if n > maxTerminalLines {
		n = maxTerminalLines
	}

	lines := logger.RecentLines(n)
	if len(lines) == 0 {
		client.SendServerMessage("No log output has been captured yet.")
		return
	}
	client.SendServerMessage(fmt.Sprintf("Last %d server log line(s):\n%s", len(lines), strings.Join(lines, "\n")))
}
