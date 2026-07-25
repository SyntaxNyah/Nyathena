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

// Nyathena fork addition: tests for the mod-only live IPID feed (PU type 4).
// This must NEVER reach a non-BAN_INFO recipient — that's the entire point
// of routing it through a per-recipient-checked function instead of
// broadcastToAll, so these tests assert both the positive (mods get it) and
// negative (non-mods never do) cases explicitly.

import (
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

func TestBroadcastIPIDToModsOnlyReachesModRecipients(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	modConn := &capturingConn{}
	mod := &Client{conn: modConn, uid: 1, char: -1, perms: permissions.PermissionField["BAN_INFO"]}
	clients.AddClient(mod)
	clients.RegisterUID(mod)

	plainConn := &capturingConn{}
	plain := &Client{conn: plainConn, uid: 2, char: -1, perms: permissions.PermissionField["NONE"]}
	clients.AddClient(plain)
	clients.RegisterUID(plain)

	broadcastIPIDToMods(3, "target-ipid")

	wantMod := "PU#3#4#target-ipid#%"
	if got := modConn.snapshot(); len(got) != 1 || got[0] != wantMod {
		t.Errorf("mod recipient got %v, want exactly [%q]", got, wantMod)
	}
	if got := plainConn.snapshot(); len(got) != 0 {
		t.Errorf("non-mod recipient got %v, want nothing — IPID must never reach a client the server hasn't verified as BAN_INFO", got)
	}
}

func TestSendModIPIDsToClientBackfillsOnlyForModRecipient(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	// Two already-joined, visible players whose IPIDs a backfill should cover.
	existingA := &Client{conn: &testConn{}, uid: 10, char: -1, ipid: "ipid-a"}
	existingB := &Client{conn: &testConn{}, uid: 11, char: -1, ipid: "ipid-b"}
	// A hidden player must be excluded, matching sendPlayerListToClient's
	// existing precedent for the other PU field types.
	hidden := &Client{conn: &testConn{}, uid: 12, char: -1, ipid: "ipid-hidden", hidden: true}
	for _, c := range []*Client{existingA, existingB, hidden} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	modConn := &capturingConn{}
	newMod := &Client{conn: modConn, uid: 20, char: -1, perms: permissions.PermissionField["BAN_INFO"]}
	clients.AddClient(newMod)
	clients.RegisterUID(newMod)

	plainConn := &capturingConn{}
	newPlain := &Client{conn: plainConn, uid: 21, char: -1, perms: permissions.PermissionField["NONE"]}
	clients.AddClient(newPlain)
	clients.RegisterUID(newPlain)

	sendModIPIDsToClient(newMod)
	sendModIPIDsToClient(newPlain)

	got := modConn.snapshot()
	wantA, wantB := "PU#10#4#ipid-a#%", "PU#11#4#ipid-b#%"
	foundA, foundB, foundHidden := false, false, false
	for _, w := range got {
		switch w {
		case wantA:
			foundA = true
		case wantB:
			foundB = true
		case "PU#12#4#ipid-hidden#%":
			foundHidden = true
		}
	}
	if !foundA || !foundB {
		t.Errorf("mod backfill = %v, want both %q and %q present", got, wantA, wantB)
	}
	if foundHidden {
		t.Errorf("mod backfill included a hidden player's IPID: %v", got)
	}

	if got := plainConn.snapshot(); len(got) != 0 {
		t.Errorf("non-mod recipient got %v from a backfill call, want nothing", got)
	}
}
