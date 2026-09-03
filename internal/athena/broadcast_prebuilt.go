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
	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// A broadcast sends the SAME bytes to every recipient, but SendPacket
// serializes them again for each one: it walks the whole argument slice and
// allocates a fresh buffer per client. For most packets that is a handful of
// short fields and does not matter.
//
// CharsCheck is the exception, and by a wide margin. It carries one entry per
// character on the server -- 4,465 of them here -- so a single character change
// costs (clients in area) x 4,465 loop iterations and (clients in area) x ~9 KB
// of immediately-garbage allocation, to send an identical 9 KB payload N times.
//
// Measured on a capture taken while the server was struggling: 2.16 seconds of
// traffic, 6.1 MB of outbound payload, of which CharsCheck was 75.8% -- 4.6 MB
// produced by just EIGHT character changes, at 64 recipients each. That is
// 0.58 MB per character change with 45 people online. The cost scales with
// population, and rapid character re-rolling is a documented raid signature
// (SigCharChurn), so a raid drives exactly the input that maximises it.
//
// Nothing here is a lock or a blocking write -- SendPacket is already
// asynchronous and ForEach already releases its lock before sending, both of
// which were fixed earlier. What was left was the CPU and the allocator, which
// is why the server went unresponsive without ever crashing.
//
// Serializing once and handing every recipient the same immutable buffer turns
// that N into 1. Safe because runWriter only ever reads the slice
// (conn.Write(buf) and string(buf) for the network log) and never mutates it,
// so one buffer can back any number of queued sends.

// sendPrebuilt enqueues an already-serialized packet, skipping the per-client
// serialization SendPacket does. The buffer MUST NOT be modified afterwards:
// it is shared with every other recipient of the same broadcast.
//
// Mirrors SendPacket's contract exactly otherwise -- non-blocking, and a full
// queue drops the packet rather than blocking the broadcaster or disconnecting
// a slow reader.
func (client *Client) sendPrebuilt(buf []byte) {
	if buf == nil || client.closed.Load() {
		return
	}
	if client.sendCh == nil {
		// Struct-literal clients in tests have no queue; fall back so they
		// still observe the write, exactly as SendPacket does.
		client.write(string(buf))
		return
	}
	select {
	case client.sendCh <- buf:
	default:
	}
}

// prebuiltPacket lazily serializes one outgoing packet into each wire format,
// at most once per format per broadcast, and hands out the shared buffer.
type prebuiltPacket struct {
	header string
	args   []string
	fanta  []byte
	jsonb  []byte
	jsonNo bool // BuildJSON returned nil; do not retry per client
}

func newPrebuilt(p packet.Outgoing) *prebuiltPacket {
	return &prebuiltPacket{header: p.Header(), args: p.Args()}
}

// forClient returns the wire bytes for one recipient's encoding mode.
func (pb *prebuiltPacket) forClient(client *Client) []byte {
	if client.jsonMode.Load() {
		if pb.jsonb == nil && !pb.jsonNo {
			pb.jsonb = packet.BuildJSON(pb.header, pb.args)
			if pb.jsonb == nil {
				pb.jsonNo = true
			}
		}
		if pb.jsonb == nil {
			return nil
		}
		// The MS broadcast schema is enforced per packet, not per recipient --
		// the bytes are identical, so one verdict covers everyone.
		if pb.header == "MS" {
			if err := packet.ValidateMSBroadcast(pb.jsonb); err != nil {
				logger.LogWarningf("dropped outbound %v — MSBroadcast schema validation failed: %v", pb.header, err)
				pb.jsonb, pb.jsonNo = nil, true
				return nil
			}
		}
		return pb.jsonb
	}
	if pb.fanta == nil {
		n := len(pb.header) + 2
		for _, c := range pb.args {
			n += 1 + len(c)
		}
		buf := make([]byte, 0, n)
		buf = append(buf, pb.header...)
		for _, c := range pb.args {
			buf = append(buf, '#')
			buf = append(buf, c...)
		}
		pb.fanta = append(buf, '#', '%')
	}
	return pb.fanta
}

// broadcastToAreaOnce is broadcastToArea for a packet whose payload is
// identical for every recipient, serializing it once rather than per client.
// Use it for large fixed payloads (CharsCheck); for a handful of short fields
// the saving is not worth the extra indirection.
func broadcastToAreaOnce(a *area.Area, p packet.Outgoing) {
	pb := newPrebuilt(p)
	clients.ForEach(func(client *Client) {
		if client.Area() == a {
			client.sendPrebuilt(pb.forClient(client))
		}
	})
}
