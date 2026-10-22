package athena

// Structured IC transcript for TimeLeap.
//
// Area already keeps two histories, and neither can replay a scene:
//   - Area.buffer (areas.go) holds FORMATTED mod-log lines ("[HH:MM:SS] name:
//     text") pruned to a 24h window;
//   - Area.icMessages keeps raw text per IPID, for /icwarp.
// A TL snapshot needs the fields an MSToClient needs, so this keeps them
// alongside the message instead of trying to parse either of those back.

import (
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// tlLogLine is one broadcast IC message in replayable form.
type tlLogLine struct {
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

// tlLogMax bounds memory per area. 256 lines is several scenes' worth and keeps
// the TL frame comfortably under the reader's packet cap.
const tlLogMax = 256

// tlLogs maps *area.Area -> *tlLog. Areas are static config, so entries live for
// the process lifetime and nothing needs eviction.
var tlLogs sync.Map

type tlLog struct {
	mu    sync.Mutex
	lines []tlLogLine
}

func tlLogFor(a *area.Area) *tlLog {
	v, _ := tlLogs.LoadOrStore(a, &tlLog{})
	return v.(*tlLog)
}

func (l *tlLog) append(x tlLogLine) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if x.TS == 0 {
		x.TS = time.Now().Unix()
	}
	l.lines = append(l.lines, x)
	if len(l.lines) > tlLogMax {
		l.lines = append(l.lines[:0], l.lines[len(l.lines)-tlLogMax:]...)
	}
}

func (l *tlLog) snapshot() []tlLogLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]tlLogLine(nil), l.lines...)
}

// len reports how many lines are retained (for /tl status and tests).
func (l *tlLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.lines)
}
