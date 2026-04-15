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

import "sync"

// forEachPool provides reusable []*Client snapshots for ForEach, eliminating
// one heap allocation per broadcast on the hot IC/OOC/ARUP path.
var forEachPool = sync.Pool{
	New: func() any { return make([]*Client, 0, 128) },
}

type ClientList struct {
	list       map[*Client]struct{}
	uidIndex   map[int]*Client
	ipidCounts map[string]int
	mu         sync.RWMutex
}

// AddClient adds a client to the list.
// The UID index is not populated here because clients have uid == -1 at
// connection time; call RegisterUID once the real UID has been assigned.
func (cl *ClientList) AddClient(c *Client) {
	cl.mu.Lock()
	cl.list[c] = struct{}{}
	cl.ipidCounts[c.Ipid()]++
	cl.mu.Unlock()
}

// RegisterUID inserts c into the UID index.
// Call this after the client's UID has been set via SetUid.
func (cl *ClientList) RegisterUID(c *Client) {
	cl.mu.Lock()
	if uid := c.Uid(); uid != -1 {
		cl.uidIndex[uid] = c
	}
	cl.mu.Unlock()
}

// RemoveClient removes a client from the list.
func (cl *ClientList) RemoveClient(c *Client) {
	cl.mu.Lock()
	delete(cl.list, c)
	if uid := c.Uid(); uid != -1 {
		delete(cl.uidIndex, uid)
	}
	if n := cl.ipidCounts[c.Ipid()]; n <= 1 {
		delete(cl.ipidCounts, c.Ipid())
	} else {
		cl.ipidCounts[c.Ipid()] = n - 1
	}
	cl.mu.Unlock()
}

// ForEach calls fn for every client in the list.
// It reuses a pooled []*Client snapshot to avoid a heap allocation on every
// call.  The read lock is held only while building the snapshot so callers may
// safely call any client method (including writes) without holding cl.mu.
func (cl *ClientList) ForEach(fn func(*Client)) {
	cl.mu.RLock()
	snap := forEachPool.Get().([]*Client)
	snap = snap[:0]
	for c := range cl.list {
		snap = append(snap, c)
	}
	cl.mu.RUnlock()
	for _, c := range snap {
		fn(c)
	}
	// Zero the pointer slots before returning to pool to allow the GC to
	// collect clients that disconnect while the slice header lives in the pool.
	for i := range snap {
		snap[i] = nil
	}
	forEachPool.Put(snap[:0])
}

// GetClientByUID returns a client by their UID, or nil if not found.
// O(1) lookup via the UID index map.
func (cl *ClientList) GetClientByUID(uid int) *Client {
	cl.mu.RLock()
	c := cl.uidIndex[uid]
	cl.mu.RUnlock()
	return c
}

// CountByIPID returns the number of connected clients with the given IPID.
// O(1) lookup via the IPID count map.
func (cl *ClientList) CountByIPID(ipid string) int {
	cl.mu.RLock()
	n := cl.ipidCounts[ipid]
	cl.mu.RUnlock()
	return n
}

// Count returns the total number of currently connected clients.
func (cl *ClientList) Count() int {
	cl.mu.RLock()
	n := len(cl.list)
	cl.mu.RUnlock()
	return n
}

// GetByIPID returns a slice of all clients whose IPID matches ipid.
// The slice is freshly allocated on each call; the read lock is held only
// for the iteration itself so callers may safely invoke client methods after.
func (cl *ClientList) GetByIPID(ipid string) []*Client {
	cl.mu.RLock()
	result := make([]*Client, 0, cl.ipidCounts[ipid])
	for c := range cl.list {
		if c.Ipid() == ipid {
			result = append(result, c)
		}
	}
	cl.mu.RUnlock()
	return result
}
