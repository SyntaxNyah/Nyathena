package athena

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// One connection that has stopped reading must not be able to stall every
// broadcast on the server.
//
// It could. Client.write held client.mu across a BLOCKING socket write, and
// Client.Area() takes that same mutex -- so every broadcast, which calls
// Area() on each client to decide whether it is a recipient, queued behind
// whichever client was mid-write. pktReqAM used that path to push the ~45 KB
// music/area list on join, with no write deadline set (runWriter clears the
// deadline after each of its own writes), so on a socket whose send buffer is
// full the write blocks indefinitely and the mutex is never released.
//
// That is a server which stops responding and never crashes, and a raid is
// precisely the traffic that produces it: hundreds of connections joining at
// once, many of them bots that connect and never read.

// blockingConn blocks in Write until it is released.
type blockingConn struct {
	release chan struct{}
	entered chan struct{}
	once    bool
}

func newBlockingConn() *blockingConn {
	return &blockingConn{release: make(chan struct{}), entered: make(chan struct{}, 1)}
}

func (c *blockingConn) Read(_ []byte) (int, error) { return 0, io.EOF }
func (c *blockingConn) Write(p []byte) (int, error) {
	if !c.once {
		c.once = true
		select {
		case c.entered <- struct{}{}:
		default:
		}
		<-c.release
	}
	return len(p), nil
}
func (c *blockingConn) Close() error                       { return nil }
func (c *blockingConn) LocalAddr() net.Addr                { return testAddr("local") }
func (c *blockingConn) RemoteAddr() net.Addr               { return testAddr("remote") }
func (c *blockingConn) SetDeadline(_ time.Time) error      { return nil }
func (c *blockingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *blockingConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestStuckWriterCannotStallBroadcasts(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 64, 10, area.EviAny)
	cl := &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}

	// The connection that stops reading, plus ordinary players behind it.
	bc := newBlockingConn()
	stuck := &Client{char: -1, area: a, uid: 1, ipid: "stuck", conn: bc,
		sendCh: make(chan []byte, 64)}
	cl.list[stuck] = struct{}{}
	for i := 2; i <= 20; i++ {
		c := &Client{char: -1, area: a, uid: i, ipid: "ok", sendCh: make(chan []byte, 64)}
		cl.list[c] = struct{}{}
	}
	clients = cl

	// The join path: a big blocking write on a socket nobody is draining.
	go stuck.write("SM#" + string(make([]byte, 45000)) + "#%")
	select {
	case <-bc.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking write never started")
	}
	t.Cleanup(func() { close(bc.release) })

	// While that write is stuck, an ordinary broadcast must still complete.
	done := make(chan struct{})
	go func() {
		broadcastToArea(a, &packet.CTToClient{Name: "server", Message: "hello", IsFromServer: "1"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a broadcast stalled behind ONE connection that stopped reading; " +
			"every IC and OOC message on the server would freeze until that " +
			"socket drained or timed out")
	}
}

// The same guarantee for SendPacketSync, which is the path the lockdown purge
// uses to deliver its kick reason. A purge fired against a raid hands that to
// hundreds of connections that have stopped reading at once; each one holding
// client.mu for the full 30s write deadline would stall every broadcast on the
// server for as long as the purge takes to drain -- so the anti-raid response
// would itself be what froze the server.
func TestStuckSyncSendCannotStallBroadcasts(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 64, 10, area.EviAny)
	cl := &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}
	bc := newBlockingConn()
	// sendCh present, as NewClient always provides: without it SendPacket falls
	// back to the synchronous path and the broadcast would queue on writeMu,
	// which is a property of the test scaffolding rather than of the server.
	stuck := &Client{char: -1, area: a, uid: 1, ipid: "stuck", conn: bc,
		sendCh: make(chan []byte, 64)}
	cl.list[stuck] = struct{}{}
	for i := 2; i <= 20; i++ {
		c := &Client{char: -1, area: a, uid: i, ipid: "ok", sendCh: make(chan []byte, 64)}
		cl.list[c] = struct{}{}
	}
	clients = cl

	go stuck.SendPacketSync("KK", "You have been removed by lockdown.")
	select {
	case <-bc.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking write never started")
	}
	t.Cleanup(func() { close(bc.release) })

	done := make(chan struct{})
	go func() {
		broadcastToArea(a, &packet.CTToClient{Name: "server", Message: "hello", IsFromServer: "1"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a broadcast stalled behind one connection being kicked by the " +
			"lockdown purge; the raid response would freeze the server")
	}
}
