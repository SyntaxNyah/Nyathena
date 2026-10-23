package athena

// Structured IC transcript for Teleport.
//
// Area already keeps two histories, and neither can replay a scene:
//   - Area.buffer (areas.go) holds FORMATTED mod-log lines ("[HH:MM:SS] name:
//     text") pruned to a 24h window;
//   - Area.icMessages keeps raw text per IPID, for /icwarp.
// A TP snapshot needs the fields an MSToClient needs, so this keeps them
// alongside the message instead of trying to parse either of those back.

import (
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// tpLogLine is one broadcast IC message in replayable form.
type tpLogLine struct {
	TS       int64
	UID      int
	CharID   int
	Name     string // character folder as the room saw it (post-iniswap)
	Emote    string
	Showname string
	Side     string
	Text     string // post-censor, unescaped
	Color    int
}

// tpLogMax bounds memory per area. 256 lines is several scenes' worth and keeps
// the TP frame comfortably under the reader's packet cap.
const tpLogMax = 256

// tpLogs maps *area.Area -> *tpLog. Areas are static config, so entries live for
// the process lifetime and nothing needs eviction.
var tpLogs sync.Map

type tpLog struct {
	mu    sync.Mutex
	lines []tpLogLine
}

func tpLogFor(a *area.Area) *tpLog {
	v, _ := tpLogs.LoadOrStore(a, &tpLog{})
	return v.(*tpLog)
}

func (l *tpLog) append(x tpLogLine) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if x.TS == 0 {
		x.TS = time.Now().Unix()
	}
	l.lines = append(l.lines, x)
	if len(l.lines) > tpLogMax {
		l.lines = append(l.lines[:0], l.lines[len(l.lines)-tpLogMax:]...)
	}
}

func (l *tpLog) snapshot() []tpLogLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]tpLogLine(nil), l.lines...)
}

// len reports how many lines are retained (for /tp status and tests).
func (l *tpLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.lines)
}
