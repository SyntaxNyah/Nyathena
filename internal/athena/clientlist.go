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
