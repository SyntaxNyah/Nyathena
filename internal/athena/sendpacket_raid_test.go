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
	"net"
	"testing"
	"time"
)

// raidStuckClient builds a Client whose underlying conn is one half of a
// net.Pipe. The other half is captured by the test and never read from, which
// reproduces what a flood-bot does in production: connect, then refuse to
// drain its receive buffer. Any synchronous Write to such a connection will
// block until the 5-second deadline trips. The writer goroutine is started
// just like in HandleClient so the queue is actually being drained (and
// stalled) by a real goroutine rather than left dormant.
func raidStuckClient(t *testing.T) (c *Client, peer net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	c = NewClient(a, "stuck-bot")
	go c.runWriter()
	return c, b
}

// TestSendPacketDoesNotBlockOnStuckConsumers reproduces the connection-flood
// freeze: with hundreds of stuck consumers in an area, a single IC broadcast
// (which iterates every client and calls SendPacket on each) used to block
// the broadcaster's goroutine for ~5 seconds per stuck socket, freezing the
// sender's input handling even though connection rate-limits had already
// rejected the bots' new connection attempts.
//
// With the per-client outbound queue, SendPacket only enqueues; the actual
// TCP write happens on a dedicated writer goroutine per client. A broadcast
// across N stuck consumers therefore completes in microseconds regardless of
// how many sockets are jammed.
func TestSendPacketDoesNotBlockOnStuckConsumers(t *testing.T) {
	const numStuck = 100

	stuck := make([]*Client, numStuck)
	peers := make([]net.Conn, numStuck)
	for i := 0; i < numStuck; i++ {
		stuck[i], peers[i] = raidStuckClient(t)
	}
	t.Cleanup(func() {
		for i := range stuck {
			stuck[i].markClosed()
			peers[i].Close()
		}
	})

	args := []string{
		"chat", "0", "Phoenix Wright", "(a)pointing", "Did you do it?",
		"wit", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0",
		"", "", "", "0", "1", "0", "0", "0",
	}

	start := time.Now()
	for _, c := range stuck {
		c.SendPacket("MS", args...)
	}
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("broadcasting to %d stuck consumers took %v — the SendPacket fast path is blocking on the socket again (expected < 1s)", numStuck, elapsed)
	}
}

// TestSendPacketBroadcastDoesNotStarveLegitimateClient is the end-to-end
// version: a real "victim" client whose pipe IS being drained, surrounded by
// many stuck "bot" clients. The victim must never wait on a stuck bot's
// socket.
func TestSendPacketBroadcastDoesNotStarveLegitimateClient(t *testing.T) {
	const numStuck = 100

	stuck := make([]*Client, numStuck)
	stuckPeers := make([]net.Conn, numStuck)
	for i := 0; i < numStuck; i++ {
		stuck[i], stuckPeers[i] = raidStuckClient(t)
	}

	vA, vB := net.Pipe()
	victim := NewClient(vA, "real-player")
	go victim.runWriter()
	drained := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := vB.Read(buf); err != nil {
				close(drained)
				return
			}
		}
	}()

	t.Cleanup(func() {
		victim.markClosed()
		vB.Close()
		<-drained
		for i := range stuck {
			stuck[i].markClosed()
			stuckPeers[i].Close()
		}
	})

	args := []string{
		"chat", "0", "Phoenix Wright", "(a)pointing", "Hold it!",
		"wit", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0",
		"", "", "", "0", "1", "0", "0", "0",
	}

	all := append(append([]*Client(nil), stuck...), victim)
	start := time.Now()
	for _, c := range all {
		c.SendPacket("MS", args...)
	}
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("victim's broadcast across %d stuck bots + 1 real client took %v — legitimate IC sends are still being starved by stuck consumers (expected < 1s)", numStuck, elapsed)
	}
}

// TestSendPacketBurstDoesNotDisconnectLegitimateClient guards the v1
// regression that caused #315: sendPlayerListToClient emits ~5 SendPackets
// per existing player on join, so on a busy server one new joiner gets
// hundreds of packets back-to-back. v1 disconnected on queue overflow, which
// killed every join. v2/v3 drops the packet on overflow instead.
func TestSendPacketBurstDoesNotDisconnectLegitimateClient(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	c := NewClient(a, "joining-player")
	go c.runWriter()
	t.Cleanup(func() { c.markClosed() })

	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := b.Read(buf); err != nil {
				return
			}
		}
	}()

	const burst = 4000
	for i := 0; i < burst; i++ {
		c.SendPacket("PU", "1", "0", "Phoenix Wright")
	}

	if c.closed.Load() {
		t.Fatalf("legitimate client was disconnected by a %d-packet burst — the queue-overflow disconnect regression returned", burst)
	}
}

// TestSendPacketTransientWriteErrorDoesNotDisconnect guards the v2
// regression that prompted reverting that PR: a brief network blip that
// makes a single Write hit its 5-second deadline must NOT mark the client
// closed. The old synchronous SendPacket explicitly ignored Write errors
// for exactly this reason; v3 does the same.
func TestSendPacketTransientWriteErrorDoesNotDisconnect(t *testing.T) {
	a, b := net.Pipe()
	c := NewClient(a, "flaky-network-player")

	writerExited := make(chan struct{})
	go func() {
		c.runWriter()
		close(writerExited)
	}()

	// Don't drain `b` — the writer's first Write will hit the 5s deadline
	// (net.Pipe respects deadlines). The writer must NOT proactively kick
	// the client; only an explicit conn close should do that.
	c.SendPacket("MS", "filler")
	time.Sleep(6 * time.Second)

	if c.closed.Load() {
		t.Fatalf("transient Write error caused the writer to disconnect the client — the v2 regression that kicked legitimate users on flaky networks returned")
	}

	c.markClosed()
	b.Close()
	select {
	case <-writerExited:
	case <-time.After(2 * time.Second):
		t.Fatalf("writer did not exit after markClosed")
	}
}

// TestSendPacketWriterExitsOnConnClose confirms the only path the writer
// uses to terminate: when the underlying conn is explicitly closed (cleanup,
// ping timeout, or any of the kick paths), the next Write returns
// net.ErrClosed and the goroutine exits. This is what reaps stuck bots —
// the existing rate limiters and ping timeout call conn.Close() directly,
// surfacing here as net.ErrClosed.
func TestSendPacketWriterExitsOnConnClose(t *testing.T) {
	a, b := net.Pipe()
	c := NewClient(a, "soon-to-disconnect")

	writerDone := make(chan struct{})
	go func() {
		c.runWriter()
		close(writerDone)
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := b.Read(buf); err != nil {
				return
			}
		}
	}()

	c.SendPacket("MS", "filler")

	c.markClosed()
	b.Close()

	select {
	case <-writerDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("writer goroutine did not exit within 2s after markClosed")
	}
}
