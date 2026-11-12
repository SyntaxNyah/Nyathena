// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package athena

import (
	"fmt"
	"strings"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// A burst of character changes must publish only the final char name, not one
// PU per change.
func TestRequestPlayerDataCoalescesLatest(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, _ := benchClients(1)
	var c *Client
	for x := range cl.list {
		c = x
	}
	c.uid = 12
	cl.uidIndex[12] = c
	clients = cl

	for i := 0; i < 50; i++ {
		requestPlayerData(c, aolib.PlayerDataTypeCharName, fmt.Sprintf("char-%d", i))
	}
	flushPlayerData()

	select {
	case b := <-c.sendCh:
		if !strings.Contains(string(b), "char-49") {
			t.Fatalf("published a stale char name: %q", b)
		}
	default:
		t.Fatal("client received no coalesced PU")
	}
	select {
	case b := <-c.sendCh:
		t.Fatalf("client received more than one PU for one field: %q", b)
	default:
	}
}

// Char name and showname are separate fields, so both must be published even
// when they are requested in the same window.
func TestRequestPlayerDataKeepsTypes(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, _ := benchClients(1)
	var c *Client
	for x := range cl.list {
		c = x
	}
	c.uid = 13
	cl.uidIndex[13] = c
	clients = cl

	requestPlayerData(c, aolib.PlayerDataTypeCharName, "Phoenix")
	requestPlayerData(c, aolib.PlayerDataTypeShowname, "Nick")
	flushPlayerData()

	select {
	case b := <-c.sendCh:
		if strings.Count(string(b), "PU#") != 2 {
			t.Fatalf("expected both PU fields in one batch, got %q", b)
		}
	default:
		t.Fatal("client received no coalesced PU batch")
	}
}

// A player who disconnects before the flush must not have a queued PU applied
// to a reused UID.
func TestRequestPlayerDataDropsClosedClient(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, _ := benchClients(1)
	var c *Client
	for x := range cl.list {
		c = x
	}
	c.uid = 14
	cl.uidIndex[14] = c
	clients = cl

	requestPlayerData(c, aolib.PlayerDataTypeCharName, "Phoenix")
	c.closed.Store(true)
	flushPlayerData()

	select {
	case b := <-c.sendCh:
		t.Fatalf("closed client received a stale PU: %q", b)
	default:
	}
}
