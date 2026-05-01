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
//
// We assert that 100 enqueues complete well below the old worst-case latency
// (5s × 100 = 500s). Anything over 1s indicates the synchronous-Write path
// has crept back in.
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
// many stuck "bot" clients. We measure how long it takes to broadcast IC to
// all of them. The victim must never wait on a stuck bot's socket.
func TestSendPacketBroadcastDoesNotStarveLegitimateClient(t *testing.T) {
	const numStuck = 100

	stuck := make([]*Client, numStuck)
	stuckPeers := make([]net.Conn, numStuck)
	for i := 0; i < numStuck; i++ {
		stuck[i], stuckPeers[i] = raidStuckClient(t)
	}

	// One victim whose peer drains everything immediately.
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

	// Simulate the broadcast loop from writeToArea: walk every client and
	// call SendPacket. The victim is included to match the real broadcast.
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

// TestSendPacketSlowConsumerDisconnects verifies the safety valve: if a
// client's outbound queue fills (writer can't drain because the consumer is
// stuck), SendPacket marks the client closed so subsequent packets are
// dropped cheaply rather than triggering further allocations.
func TestSendPacketSlowConsumerDisconnects(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	c := NewClient(a, "stuck-bot")
	go c.runWriter()

	// Fill the queue past sendQueueSize. The first packet may be picked up
	// by runWriter and held inside Write (blocking on the pipe), so allow
	// for that by sending sendQueueSize + 2 packets.
	for i := 0; i < sendQueueSize+2; i++ {
		c.SendPacket("MS", "filler")
	}

	if !c.closed.Load() {
		t.Fatalf("expected slow consumer to be marked closed after overflowing send queue")
	}

	// A subsequent SendPacket on a closed client must be a cheap no-op
	// rather than triggering further work or panics.
	c.SendPacket("MS", "after-close")
}
