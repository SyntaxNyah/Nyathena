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
// many stuck "bot" clients. The victim must never wait on a stuck bot's socket.
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

// TestSendPacketBurstDoesNotDisconnectLegitimateClient proves the v1 fix's
// regression doesn't return: a normal join can issue 4–5 SendPackets per
// existing player via sendPlayerListToClient. On a server with 200 players
// that's ~1000 packets to one new joiner in rapid succession. The v1 fix
// disconnected on queue overflow, which kicked legitimate joiners on busy
// servers. v2 drops packets instead of disconnecting, so even a 4000-packet
// burst on a slow consumer (here: a pipe whose reader keeps up but is slow)
// must NOT mark the client closed.
func TestSendPacketBurstDoesNotDisconnectLegitimateClient(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	c := NewClient(a, "joining-player")
	go c.runWriter()
	t.Cleanup(func() { c.markClosed() })

	// Drain the pipe at a relaxed rate — represents a player on a slower
	// connection. The writer goroutine writes as the kernel/pipe accepts.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := b.Read(buf); err != nil {
				return
			}
		}
	}()

	// 4× the join burst on a 1000-player server (5 packets per existing
	// player). Far more than v1's 256-packet queue could hold — and far more
	// than what would happen in a single tick on a real server.
	const burst = 4000
	for i := 0; i < burst; i++ {
		c.SendPacket("PU", "1", "0", "Phoenix Wright")
	}

	if c.closed.Load() {
		t.Fatalf("legitimate client was disconnected by a %d-packet burst — v1 disconnect-on-overflow regression returned", burst)
	}
}

// TestSendPacketStuckConsumerDisconnectsViaWriteTimeout confirms the
// disconnect path: when the writer goroutine's Write fails (5-second
// deadline trips on a stuck pipe), markClosed runs and tears the client
// down. We force the timeout by closing the peer side mid-test.
func TestSendPacketStuckConsumerDisconnectsViaWriteTimeout(t *testing.T) {
	a, b := net.Pipe()
	c := NewClient(a, "stuck-bot")
	go c.runWriter()

	c.SendPacket("MS", "filler")

	// Closing the peer makes the next Write return io.ErrClosedPipe almost
	// immediately, simulating what the 5s SetWriteDeadline does on a real
	// stuck TCP socket.
	b.Close()
	c.SendPacket("MS", "filler") // queue this so runWriter picks it up

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.closed.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("stuck consumer was not disconnected via writer timeout path within 2s")
}
