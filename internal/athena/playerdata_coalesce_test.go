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

// A burst of character changes must publish only the final current value, not
// one PU per change.
func TestRequestPlayerDataCoalescesLatest(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	origChars := getCharacters()
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})
	t.Cleanup(func() { setCharacters(origChars) })
	cl, _ := benchClients(1)
	var c *Client
	for x := range cl.list {
		c = x
	}
	c.uid = 12
	c.char = 0
	cl.uidIndex[12] = c
	clients = cl

	for i := 0; i < 50; i++ {
		requestPlayerData(c, aolib.PlayerDataTypeCharName, fmt.Sprintf("stale-%d", i))
	}
	flushPlayerData()

	select {
	case b := <-c.sendCh:
		if !strings.Contains(string(b), "Phoenix Wright") || strings.Contains(string(b), "stale-") {
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
	origChars := getCharacters()
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})
	t.Cleanup(func() { setCharacters(origChars) })
	cl, _ := benchClients(1)
	var c *Client
	for x := range cl.list {
		c = x
	}
	c.uid = 13
	c.char = 0
	c.showname = "Nick"
	cl.uidIndex[13] = c
	clients = cl

	requestPlayerData(c, aolib.PlayerDataTypeCharName, "ignored")
	requestPlayerData(c, aolib.PlayerDataTypeShowname, "ignored")
	flushPlayerData()

	select {
	case b := <-c.sendCh:
		if strings.Count(string(b), "PU#") != 2 {
			t.Fatalf("expected both PU fields in one batch, got %q", b)
		}
		if !strings.Contains(string(b), "Phoenix Wright") || !strings.Contains(string(b), "Nick") {
			t.Fatalf("PU batch did not use the latest values: %q", b)
		}
	default:
		t.Fatal("client received no coalesced PU batch")
	}
}

// A showname change that lands while a PU is queued must win over the stale
// snapshot, otherwise the trailing flush overwrites a moderator's /showname.
func TestRequestPlayerDataDoesNotOverwriteNewerShowname(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, _ := benchClients(1)
	var c *Client
	for x := range cl.list {
		c = x
	}
	c.uid = 15
	cl.uidIndex[15] = c
	clients = cl

	requestPlayerData(c, aolib.PlayerDataTypeShowname, "Old Name")
	c.SetShowname("New Name")
	flushPlayerData()

	select {
	case b := <-c.sendCh:
		if strings.Contains(string(b), "Old Name") || !strings.Contains(string(b), "New Name") {
			t.Fatalf("trailing PU overwrote the newer showname: %q", b)
		}
	default:
		t.Fatal("client received no coalesced showname")
	}
}

// The PU flusher can run after a test (or a reload) has cleared the roster;
// CurrentCharacter must report a safe spectator value rather than panic.
func TestCurrentCharacterOutOfRange(t *testing.T) {
	orig := getCharacters()
	setCharacters(nil)
	t.Cleanup(func() { setCharacters(orig) })

	c := &Client{char: 1}
	if got := c.CurrentCharacter(); got != "Spectator" {
		t.Fatalf("CurrentCharacter with empty roster = %q, want Spectator", got)
	}
	c.char = -1
	if got := c.CurrentCharacter(); got != "Spectator" {
		t.Fatalf("CurrentCharacter for spectator = %q, want Spectator", got)
	}
}

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
