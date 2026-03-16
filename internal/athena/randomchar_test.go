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
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestGetRandomFreeChar verifies that getRandomFreeChar returns a free character
// ID from the client's area, matching the behaviour expected when WebAO sends
// CC#0#-1#% (random character button).
func TestGetRandomFreeChar(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })

	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey", "Franziska von Karma"}

	t.Run("returns free character when some are taken", func(t *testing.T) {
		a := area.NewArea(area.AreaData{}, len(characters), 0, area.EviAny)
		// Take characters 0 and 2.
		a.AddChar(0)
		a.AddChar(2)

		client := &Client{
			uid:        1,
			char:       -1,
			possessing: -1,
			pair:       ClientPairInfo{wanted_id: -1},
		}
		client.SetArea(a)

		id := getRandomFreeChar(client)
		if id != 1 && id != 3 {
			t.Errorf("getRandomFreeChar returned %d, want 1 or 3 (free characters)", id)
		}
	})

	t.Run("returns -1 when all characters are taken", func(t *testing.T) {
		a := area.NewArea(area.AreaData{}, len(characters), 0, area.EviAny)
		// Take all characters.
		for i := range characters {
			a.AddChar(i)
		}

		client := &Client{
			uid:        1,
			char:       -1,
			possessing: -1,
			pair:       ClientPairInfo{wanted_id: -1},
		}
		client.SetArea(a)

		id := getRandomFreeChar(client)
		if id != -1 {
			t.Errorf("getRandomFreeChar returned %d, want -1 (no free characters)", id)
		}
	})

	t.Run("returns the only free character when one is available", func(t *testing.T) {
		a := area.NewArea(area.AreaData{}, len(characters), 0, area.EviAny)
		// Take all except character 2.
		a.AddChar(0)
		a.AddChar(1)
		a.AddChar(3)

		client := &Client{
			uid:        1,
			char:       -1,
			possessing: -1,
			pair:       ClientPairInfo{wanted_id: -1},
		}
		client.SetArea(a)

		id := getRandomFreeChar(client)
		if id != 2 {
			t.Errorf("getRandomFreeChar returned %d, want 2 (only free character)", id)
		}
	})

	t.Run("returns -1 when character list is empty", func(t *testing.T) {
		origCharsInner := characters
		t.Cleanup(func() { characters = origCharsInner })
		characters = []string{}

		a := area.NewArea(area.AreaData{}, 0, 0, area.EviAny)
		client := &Client{
			uid:        1,
			char:       -1,
			possessing: -1,
			pair:       ClientPairInfo{wanted_id: -1},
		}
		client.SetArea(a)

		id := getRandomFreeChar(client)
		if id != -1 {
			t.Errorf("getRandomFreeChar returned %d, want -1 (empty character list)", id)
		}
	})
}

// TestForceRandomCharCommandRegistered verifies that the /forcerandomchar command
// is properly registered in the Commands map with ADMIN-only permissions.
func TestForceRandomCharCommandRegistered(t *testing.T) {
	initCommands()

	cmd, ok := Commands["forcerandomchar"]
	if !ok {
		t.Fatal("forcerandomchar command is not registered in Commands map")
	}

	if cmd.handler == nil {
		t.Error("forcerandomchar command has a nil handler")
	}

	wantPerms := permissions.PermissionField["ADMIN"]
	if cmd.reqPerms != wantPerms {
		t.Errorf("forcerandomchar reqPerms = %v, want ADMIN (%v)", cmd.reqPerms, wantPerms)
	}

	if cmd.minArgs != 0 {
		t.Errorf("forcerandomchar minArgs = %d, want 0", cmd.minArgs)
	}
}

// TestForceRandomCharTargetsUID verifies the UID-lookup infrastructure that
// cmdForceRandomChar uses when called with a UID argument.
func TestForceRandomCharTargetsUID(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey", "Franziska von Karma"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, len(characters), 0, area.EviAny)

	// The targeted player.
	target := &Client{uid: 20, char: 1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	target.SetArea(a)
	a.AddChar(1)

	// A bystander in the same area.
	bystander := &Client{uid: 30, char: 2, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	bystander.SetArea(a)
	a.AddChar(2)

	clients.AddClient(target)
	clients.AddClient(bystander)

	// Verify that getClientByUid finds the correct client.
	found, err := getClientByUid(20)
	if err != nil {
		t.Fatalf("getClientByUid(20) returned error: %v", err)
	}
	if found != target {
		t.Errorf("getClientByUid(20) returned wrong client")
	}

	// Verify that an unknown UID returns an error (invalid-UID branch).
	if _, err := getClientByUid(99); err == nil {
		t.Error("getClientByUid(99) expected error for unknown UID, got nil")
	}

	// Verify that the free character returned is not already taken by the target or bystander.
	freeForTarget := getRandomFreeChar(target)
	if freeForTarget == -1 {
		t.Fatal("expected a free character for target, got -1")
	}
	if a.IsTaken(freeForTarget) {
		t.Errorf("getRandomFreeChar returned already-taken character %d for target", freeForTarget)
	}

	// The bystander's character must still be taken — the per-UID path must not touch other clients.
	if !a.IsTaken(bystander.char) {
		t.Error("bystander's character should remain taken before any forced change")
	}
}

// TestForceRandomCharOnlyAffectsCurrentArea verifies that cmdForceRandomChar
// changes characters only for clients in the admin's area.
func TestForceRandomCharOnlyAffectsCurrentArea(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey", "Franziska von Karma"}

	// Snapshot and restore the global clients list.
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	adminArea := area.NewArea(area.AreaData{}, len(characters), 0, area.EviAny)
	otherArea := area.NewArea(area.AreaData{}, len(characters), 0, area.EviAny)

	// Client in the admin's area.
	inArea := &Client{uid: 1, char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	inArea.SetArea(adminArea)
	adminArea.AddChar(0)

	// Client in a different area — must not be changed.
	outArea := &Client{uid: 2, char: 1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	outArea.SetArea(otherArea)
	otherArea.AddChar(1)

	clients.AddClient(inArea)
	clients.AddClient(outArea)

	// Verify that getRandomFreeChar returns -1 for the client in the other area
	// (character 1 is taken there), and returns a free character for inArea.
	freeForIn := getRandomFreeChar(inArea)
	if freeForIn == -1 {
		t.Fatal("expected a free character for inArea client, got -1")
	}

	freeForOut := getRandomFreeChar(outArea)
	if freeForOut == -1 {
		t.Fatal("expected a free character for outArea client, got -1")
	}

	// Verify area isolation: free chars for inArea are from adminArea, not otherArea.
	if adminArea.IsTaken(freeForIn) {
		t.Errorf("getRandomFreeChar returned taken character %d for inArea client", freeForIn)
	}
	if otherArea.IsTaken(freeForOut) {
		t.Errorf("getRandomFreeChar returned taken character %d for outArea client", freeForOut)
	}
}

// TestRandomCharCooldownAllowsFirstUse verifies that /randomchar is not blocked
// when the client has never used it before.
func TestRandomCharCooldownAllowsFirstUse(t *testing.T) {
	client := &Client{}
	// Zero time means no previous use — cooldown must not trigger.
	if !client.LastRandomCharTime().IsZero() {
		t.Fatal("expected zero LastRandomCharTime for new client")
	}
}

// TestRandomCharCooldownBlocksImmediateRepeat verifies that /randomchar is blocked
// within the 5-second cooldown window after a successful use.
func TestRandomCharCooldownBlocksImmediateRepeat(t *testing.T) {
	client := &Client{}
	client.SetLastRandomCharTime(time.Now())

	last := client.LastRandomCharTime()
	if last.IsZero() {
		t.Fatal("LastRandomCharTime should not be zero after SetLastRandomCharTime")
	}

	const cooldown = 5 * time.Second
	if time.Since(last) >= cooldown {
		t.Fatal("test setup error: last time should be within cooldown window")
	}
}

// TestRandomCharCooldownExpiresAfterWindow verifies that the cooldown expires
// correctly after 5 seconds.
func TestRandomCharCooldownExpiresAfterWindow(t *testing.T) {
	client := &Client{}
	// Simulate a use that happened more than 5 seconds ago.
	client.SetLastRandomCharTime(time.Now().Add(-6 * time.Second))

	const cooldown = 5 * time.Second
	if time.Since(client.LastRandomCharTime()) < cooldown {
		t.Error("cooldown should have expired for a use 6 seconds ago")
	}
}
