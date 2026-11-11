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
	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
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
	p      aolib.Outgoing
	fanta  []byte
	jsonb  []byte
	jsonNo bool // JSON encode failed; do not retry per client
}

func newPrebuilt(p aolib.Outgoing) *prebuiltPacket {
	return &prebuiltPacket{p: p}
}

// forClient returns the wire bytes for one recipient's encoding mode.
func (pb *prebuiltPacket) forClient(client *Client) []byte {
	if client.jsonMode.Load() {
		if pb.jsonb == nil && !pb.jsonNo {
			b, err := packetutil.Encode(pb.p, aolib.WireJSON)
			if err != nil {
				logger.LogWarningf("dropped outbound %v - JSON encode failed: %v", pb.p.Header(), err)
				pb.jsonNo = true
			} else {
				pb.jsonb = b
			}
		}
		if pb.jsonb == nil {
			return nil
		}
		return pb.jsonb
	}
	if pb.fanta == nil {
		pb.fanta = encodeFanta(pb.p)
	}
	return pb.fanta
}

// encodeFanta serializes one outgoing packet in FantaCode form. Shared by the
// per-broadcast prebuilt path and the handshake-list cache so the two cannot
// drift on escaping or framing.
func encodeFanta(p aolib.Outgoing) []byte {
	esc := escapeOutgoing(p.Header(), p.Args())
	n := len(p.Header()) + 2
	for _, c := range esc {
		n += 1 + len(c)
	}
	buf := make([]byte, 0, n)
	buf = append(buf, p.Header()...)
	for _, c := range esc {
		buf = append(buf, '#')
		buf = append(buf, c...)
	}
	return append(buf, '#', '%')
}

// broadcastToAreaOnce is broadcastToArea for a packet whose payload is
// identical for every recipient, serializing it once rather than per client.
// Use it for large fixed payloads (CharsCheck); for a handful of short fields
// the saving is not worth the extra indirection.
func broadcastToAreaOnce(a *area.Area, p aolib.Outgoing) {
	pb := newPrebuilt(p)
	clients.ForEach(func(client *Client) {
		if client.Area() == a {
			client.sendPrebuilt(pb.forClient(client))
		}
	})
}

// broadcastToAllOnce is broadcastToAll with one serialization per wire format
// instead of one per recipient. ARUP player-count/CM/status/lock updates and the
// PR/PU join burst all carry identical bytes to every client, so this removes
// the per-recipient Args()/escape/encode work that dominated the join fan-out.
func broadcastToAllOnce(p aolib.Outgoing) {
	if !puAllowed(p) {
		return
	}
	// halloweenHauntOutgoing mutates CT/MS/PU fields. It must run exactly once
	// before the shared buffer is built; the old per-recipient Send path ran it
	// once per client, which also meant a randomized transform could differ
	// between recipients.
	halloweenHauntOutgoing(p)
	pb := newPrebuilt(p)
	clients.ForEach(func(client *Client) {
		if client.Uid() != -1 {
			client.sendPrebuilt(pb.forClient(client))
		}
	})
}

// broadcastToAllBatch fans several packets to every joined client with one
// client-list sweep and one serialization per packet per wire format. FantaCode
// recipients get the frames concatenated into one queue entry; JSON recipients
// get one frame per packet because a WebAO client parses one JSON value per
// WebSocket message.
func broadcastToAllBatch(packets []aolib.Outgoing) {
	pbs := make([]*prebuiltPacket, 0, len(packets))
	for _, p := range packets {
		if !puAllowed(p) {
			continue
		}
		halloweenHauntOutgoing(p)
		pbs = append(pbs, newPrebuilt(p))
	}
	if len(pbs) == 0 {
		return
	}
	clients.ForEach(func(client *Client) {
		if client.Uid() == -1 {
			return
		}
		if !client.jsonMode.Load() {
			var batch []byte
			for _, pb := range pbs {
				batch = append(batch, pb.forClient(client)...)
			}
			client.sendPrebuilt(batch)
			return
		}
		for _, pb := range pbs {
			client.sendPrebuilt(pb.forClient(client))
		}
	})
}

// broadcastToAreaFromOnce is broadcastToAreaFrom with one serialization per
// wire format. The ignore list and moderator bypass are per recipient, but the
// packet bytes are not.
func broadcastToAreaFromOnce(senderIPID string, senderIsMod bool, a *area.Area, p aolib.Outgoing) {
	halloweenHauntOutgoing(p)
	pb := newPrebuilt(p)
	clients.ForEach(func(client *Client) {
		if client.Area() == a && (senderIsMod || !client.IgnoresIPID(senderIPID)) {
			client.sendPrebuilt(pb.forClient(client))
		}
	})
}
