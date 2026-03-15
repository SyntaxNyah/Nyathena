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

type ClientList struct {
	list map[*Client]struct{}
	mu   sync.RWMutex
}

// AddClient adds a client to the list.
func (cl *ClientList) AddClient(c *Client) {
	cl.mu.Lock()
	cl.list[c] = struct{}{}
	cl.mu.Unlock()
}

// RemoveClient removes a client from the list.
func (cl *ClientList) RemoveClient(c *Client) {
	cl.mu.Lock()
	delete(cl.list, c)
	cl.mu.Unlock()
}

// ForEach calls fn for every client in the list.
// It builds a slice snapshot under the read lock and releases the lock before
// invoking fn, so callers may safely call any client method (including writes)
// without holding cl.mu.  Using a slice instead of a map reduces allocation
// overhead on the hot broadcast path.
func (cl *ClientList) ForEach(fn func(*Client)) {
	cl.mu.RLock()
	snap := make([]*Client, 0, len(cl.list))
	for c := range cl.list {
		snap = append(snap, c)
	}
	cl.mu.RUnlock()
	for _, c := range snap {
		fn(c)
	}
}

// GetAllClients returns a snapshot of all clients in the list.
// A snapshot is returned so callers can iterate safely without holding a lock.
func (cl *ClientList) GetAllClients() map[*Client]struct{} {
	cl.mu.RLock()
	snapshot := make(map[*Client]struct{}, len(cl.list))
	for k := range cl.list {
		snapshot[k] = struct{}{}
	}
	cl.mu.RUnlock()
	return snapshot
}

// GetClientByUID returns a client by their UID, or nil if not found.
func (cl *ClientList) GetClientByUID(uid int) *Client {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	for client := range cl.list {
		if client.Uid() == uid {
			return client
		}
	}
	return nil
}

// CountByIPID returns the number of connected clients with the given IPID.
func (cl *ClientList) CountByIPID(ipid string) int {
	cl.mu.RLock()
	n := 0
	for c := range cl.list {
		if c.Ipid() == ipid {
			n++
		}
	}
	cl.mu.RUnlock()
	return n
}

// GetByIPID returns a slice of all clients whose IPID matches ipid.
// The slice is freshly allocated on each call; the read lock is held only
// for the iteration itself so callers may safely invoke client methods after.
func (cl *ClientList) GetByIPID(ipid string) []*Client {
	cl.mu.RLock()
	var result []*Client
	for c := range cl.list {
		if c.Ipid() == ipid {
			result = append(result, c)
		}
	}
	cl.mu.RUnlock()
	return result
}
