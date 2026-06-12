/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: the /lifo delivery punishment.

   /lifo buffers the target's IC messages and releases them in REVERSE
   arrival order: say three things and they broadcast backwards,
   third-second-first. Conversation-destroying in the best way.

   Mechanics: pktIC runs the entire validation/transform pipeline as normal,
   but instead of broadcasting it hands the finished MSPacket to
   lifoEnqueueIC. A queue flushes when it reaches lifoFlushCount messages or
   lifoFlushDelay after its first message, whichever comes first — so a
   single message still arrives within a few seconds, just suspiciously
   late. Queues are keyed by *Client, so a disconnect mid-buffer simply
   flushes to the area a moment later (broadcast helpers don't need the
   sender alive) and the entry is removed either way — no leak. */

package athena

import (
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

const (
	lifoFlushCount = 3
	lifoFlushDelay = 6 * time.Second
)

type lifoPending struct {
	ipid  string
	isMod bool
	a     *area.Area
	ms    *packet.MSPacket
}

type lifoQueue struct {
	entries []lifoPending
	timer   *time.Timer
}

var (
	lifoMu     sync.Mutex
	lifoQueues = map[*Client]*lifoQueue{}
)

// lifoBroadcastFn is swappable for tests; production releases through the
// normal area broadcaster.
var lifoBroadcastFn = func(e lifoPending) {
	broadcastToAreaFrom(e.ipid, e.isMod, e.a, e.ms)
}

// lifoEnqueueIC queues a finished outgoing IC packet for reversed release.
// The caller has already verified the speaker carries an active /lifo
// punishment. The packet is retained as-is; pktIC builds a fresh MSPacket
// per message so holding the pointer is safe.
func lifoEnqueueIC(client *Client, ms *packet.MSPacket) {
	entry := lifoPending{
		ipid: client.Ipid(),
		// Moderators aren't normally punished, but keep the shadow flag honest.
		isMod: permissions.IsModerator(client.Perms()),
		a:     client.Area(),
		ms:    ms,
	}

	var flushNow []lifoPending
	lifoMu.Lock()
	q := lifoQueues[client]
	if q == nil {
		q = &lifoQueue{}
		lifoQueues[client] = q
	}
	q.entries = append(q.entries, entry)
	if len(q.entries) >= lifoFlushCount {
		flushNow = q.entries
		q.entries = nil
		if q.timer != nil {
			q.timer.Stop()
		}
		delete(lifoQueues, client)
	} else if q.timer == nil {
		q.timer = time.AfterFunc(lifoFlushDelay, func() { lifoFlushClient(client) })
	}
	lifoMu.Unlock()

	if flushNow != nil {
		lifoRelease(flushNow)
	}
}

// lifoFlushClient drains a client's pending queue (timer path).
func lifoFlushClient(client *Client) {
	lifoMu.Lock()
	q := lifoQueues[client]
	var entries []lifoPending
	if q != nil {
		entries = q.entries
		delete(lifoQueues, client)
	}
	lifoMu.Unlock()
	lifoRelease(entries)
}

// lifoRelease broadcasts queued messages newest-first.
func lifoRelease(entries []lifoPending) {
	for i := len(entries) - 1; i >= 0; i-- {
		lifoBroadcastFn(entries[i])
	}
}

func cmdLifo(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLifo)
}
