package athena

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// Testimony playback: every place a recorded statement is re-broadcast to the
// area goes through broadcastTestimonyStatement, so the "a replayed statement
// belongs to nobody" rewrite (char id -1, showname fallback) can never be
// forgotten at a call site. See area.tstPlaybackString for the why.

// tstNavRegex matches an IC message that is *only* a testimony navigation
// control: "<" (previous statement), ">" (next statement) or ">N" (jump to
// statement N). The anchoring is the point -- the recorder used to search for
// the token anywhere in the message, so an ordinary line like "I <3 that" was
// swallowed (never broadcast) and rewound the testimony instead. Akashi only
// accepts the bare token for the same reason.
var tstNavRegex = regexp.MustCompile(`^([<>])([0-9]*)$`)

// parseTestimonyNav reports whether an IC message is a bare testimony
// navigation token, returning the direction ("<" or ">") and the optional jump
// target. Surrounding whitespace is ignored so a stray space from a client
// does not turn a navigation attempt into a chat message; anything else in the
// message makes it ordinary IC.
func parseTestimonyNav(msg string) (dir, id string, ok bool) {
	m := tstNavRegex.FindStringSubmatch(strings.TrimSpace(msg))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// broadcastTestimonyStatement re-broadcasts the recorder's current statement to
// the area in its playback form. The recorded statement keeps the speaker's
// char id; the playback form drops it to -1 so no client mistakes the replay
// for the echo of its own message and clears the player's input.
func broadcastTestimonyStatement(a *area.Area) {
	broadcastToArea(a, ParseMSToClientString(a.CurrentTstPlaybackStatement()))
}

// jumpTestimonyTarget reports whether id is a valid statement index for this
// area's testimony (0 is the title, so a jump must land on 1..len-1).
func jumpTestimonyTarget(a *area.Area, idStr string) (int, bool) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 || id >= a.TstLen() {
		return 0, false
	}
	return id, true
}
