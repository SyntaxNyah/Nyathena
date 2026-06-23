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

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// newTestClients swaps in a fresh global client list for the duration of a test
// and restores the original on cleanup.
func newTestClients(t *testing.T) {
	t.Helper()
	orig := clients
	t.Cleanup(func() { clients = orig })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}
}

// TestApplyPossessedPairFieldsSpoofsPartner verifies that possessing a paired
// player copies the *target's* partner onto the outgoing packet, so the partner
// sprite still renders in the viewport (the pair-desync fix the whole feature
// hinges on).
func TestApplyPossessedPairFieldsSpoofsPartner(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey"})
	newTestClients(t)

	// target (char 0) and partner (char 1) are mutually paired at the same pos.
	target := &Client{uid: 1, char: 0, pos: "def", possessing: -1, forcePairUID: -1, pair: ClientPairInfo{wanted_id: 1}}
	partner := &Client{uid: 2, char: 1, pos: "def", possessing: -1, forcePairUID: -1, pair: ClientPairInfo{wanted_id: 0}}
	partner.SetPairInfo("Miles Edgeworth", "confident", "1", "5&-3")
	clients.AddClient(target)
	clients.RegisterUID(target)
	clients.AddClient(partner)
	clients.RegisterUID(partner)

	ms := &packet.MSPacket{OtherCharID: "-1"}
	applyPossessedPairFields(ms, target)

	if ms.OtherCharID != "1" {
		t.Errorf("OtherCharID = %q, want \"1\" (partner's char slot)", ms.OtherCharID)
	}
	if ms.OtherName != "Miles Edgeworth" {
		t.Errorf("OtherName = %q, want \"Miles Edgeworth\"", ms.OtherName)
	}
	if ms.OtherEmote != "confident" {
		t.Errorf("OtherEmote = %q, want \"confident\"", ms.OtherEmote)
	}
	if ms.OtherOffset != "5&-3" {
		t.Errorf("OtherOffset = %q, want \"5&-3\"", ms.OtherOffset)
	}
	if ms.OtherFlip != "1" {
		t.Errorf("OtherFlip = %q, want \"1\"", ms.OtherFlip)
	}
}

// TestApplyPossessedPairFieldsForcePairIgnoresPosition verifies that a UID-locked
// (force) pair is spoofed even when the two players stand at different positions.
func TestApplyPossessedPairFieldsForcePairIgnoresPosition(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})
	newTestClients(t)

	target := &Client{uid: 1, char: 0, pos: "def", forcePairUID: 2, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	partner := &Client{uid: 2, char: 1, pos: "wit", forcePairUID: 1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	partner.SetPairInfo("Miles Edgeworth", "normal", "0", "")
	clients.AddClient(target)
	clients.RegisterUID(target)
	clients.AddClient(partner)
	clients.RegisterUID(partner)

	ms := &packet.MSPacket{OtherCharID: "-1"}
	applyPossessedPairFields(ms, target)

	if ms.OtherCharID != "1" {
		t.Errorf("OtherCharID = %q, want \"1\" (force-pair partner's char), positions differ", ms.OtherCharID)
	}
	if ms.OtherName != "Miles Edgeworth" {
		t.Errorf("OtherName = %q, want \"Miles Edgeworth\"", ms.OtherName)
	}
}

// TestApplyPossessedPairFieldsNoPartnerClears verifies that possessing an
// unpaired player clears the pair slots (rather than leaking the possessor's
// pair), rendering the target solo exactly as their own message would.
func TestApplyPossessedPairFieldsNoPartnerClears(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})
	newTestClients(t)

	target := &Client{uid: 1, char: 0, pos: "def", possessing: -1, forcePairUID: -1, pair: ClientPairInfo{wanted_id: -1}}
	clients.AddClient(target)
	clients.RegisterUID(target)

	// Pre-load the packet with a stale (possessor's) pair to ensure it's wiped.
	ms := &packet.MSPacket{OtherCharID: "7", OtherName: "Stale", OtherEmote: "x", OtherOffset: "9&9", OtherFlip: "1"}
	applyPossessedPairFields(ms, target)

	if ms.OtherCharID != "-1" {
		t.Errorf("OtherCharID = %q, want \"-1\"", ms.OtherCharID)
	}
	if ms.OtherName != "" || ms.OtherEmote != "" || ms.OtherOffset != "" || ms.OtherFlip != "" {
		t.Errorf("pair fields not cleared: name=%q emote=%q offset=%q flip=%q",
			ms.OtherName, ms.OtherEmote, ms.OtherOffset, ms.OtherFlip)
	}
}

// TestApplyPossessedPairFieldsNonMutualClears verifies that a one-sided pair (the
// "partner" actually wants someone else) does not render — it shows the target
// solo, matching the in-line pairing rule.
func TestApplyPossessedPairFieldsNonMutualClears(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey"})
	newTestClients(t)

	target := &Client{uid: 1, char: 0, pos: "def", possessing: -1, forcePairUID: -1, pair: ClientPairInfo{wanted_id: 1}}
	// Candidate is char 1 but wants char 2, not the target's char 0 — not mutual.
	other := &Client{uid: 2, char: 1, pos: "def", possessing: -1, forcePairUID: -1, pair: ClientPairInfo{wanted_id: 2}}
	other.SetPairInfo("Miles Edgeworth", "normal", "0", "")
	clients.AddClient(target)
	clients.RegisterUID(target)
	clients.AddClient(other)
	clients.RegisterUID(other)

	ms := &packet.MSPacket{OtherCharID: "-1"}
	applyPossessedPairFields(ms, target)

	if ms.OtherCharID != "-1" || ms.OtherName != "" {
		t.Errorf("expected solo render, got OtherCharID=%q OtherName=%q", ms.OtherCharID, ms.OtherName)
	}
}

// TestTruePossessGateBalances verifies the activeTruePossess hot-path gate is
// kept exact: it moves only on real state transitions and returns to its
// starting value after a mute is set and cleared.
func TestTruePossessGateBalances(t *testing.T) {
	base := activeTruePossess.Load()
	c := &Client{uid: 1, pair: ClientPairInfo{wanted_id: -1}}

	if c.TruePossessed() {
		t.Fatal("fresh client should not be true-possessed")
	}
	if c.TruePossessedBy() != -1 {
		t.Fatalf("fresh client TruePossessedBy = %d, want -1", c.TruePossessedBy())
	}

	c.SetTruePossessed(true, 42)
	if activeTruePossess.Load() != base+1 {
		t.Fatalf("gate = %d after first mute, want %d", activeTruePossess.Load(), base+1)
	}
	if !c.TruePossessed() || c.TruePossessedBy() != 42 {
		t.Fatalf("expected muted by 42, got muted=%v by=%d", c.TruePossessed(), c.TruePossessedBy())
	}

	// Idempotent re-set must NOT move the gate.
	c.SetTruePossessed(true, 42)
	if activeTruePossess.Load() != base+1 {
		t.Fatalf("gate = %d after redundant mute, want %d (must not double-count)", activeTruePossess.Load(), base+1)
	}

	c.SetTruePossessed(false, -1)
	if activeTruePossess.Load() != base {
		t.Fatalf("gate = %d after clear, want %d", activeTruePossess.Load(), base)
	}
	if c.TruePossessed() || c.TruePossessedBy() != -1 {
		t.Fatalf("expected cleared, got muted=%v by=%d", c.TruePossessed(), c.TruePossessedBy())
	}

	// Idempotent clear must NOT move the gate below base.
	c.SetTruePossessed(false, -1)
	if activeTruePossess.Load() != base {
		t.Fatalf("gate = %d after redundant clear, want %d", activeTruePossess.Load(), base)
	}
}

// TestEndTruePossessionOwnershipGuard verifies endTruePossession lifts the mute
// only for the possessor that set it, and never for an unrelated one.
func TestEndTruePossessionOwnershipGuard(t *testing.T) {
	newTestClients(t)

	owner := &Client{uid: 10, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	other := &Client{uid: 11, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	target := &Client{uid: 12, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	for _, c := range []*Client{owner, other, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	t.Cleanup(func() { target.SetTruePossessed(false, -1) }) // keep the gate balanced

	target.SetTruePossessed(true, owner.Uid())
	owner.SetPossessing(target.Uid())
	other.SetPossessing(target.Uid()) // also "possessing" the target, but not the muter

	// A possessor who did not set the mute must not lift it.
	endTruePossession(other)
	if !target.TruePossessed() {
		t.Error("endTruePossession(other) wrongly lifted a mute it did not set")
	}

	// The owning possessor lifts it.
	endTruePossession(owner)
	if target.TruePossessed() {
		t.Error("endTruePossession(owner) failed to lift the mute it set")
	}
}

// TestPossessionCommandsRegistered verifies the command registry: /truepossess is
// shadow/admin-gated, /unpossess and /hide are reachable by shadow mods, and
// /fullpossess remains admin-only.
func TestPossessionCommandsRegistered(t *testing.T) {
	initCommands()

	shadow := permissions.PermissionField["SHADOW"]
	admin := permissions.PermissionField["ADMIN"]
	regularMod := permissions.PermissionField["MUTE"] | permissions.PermissionField["KICK"]

	tp, ok := Commands["truepossess"]
	if !ok {
		t.Fatal("truepossess is not registered")
	}
	if tp.handler == nil {
		t.Error("truepossess has a nil handler")
	}
	if tp.reqPerms != shadow {
		t.Errorf("truepossess reqPerms = %v, want SHADOW (%v)", tp.reqPerms, shadow)
	}
	if tp.minArgs != 1 {
		t.Errorf("truepossess minArgs = %d, want 1", tp.minArgs)
	}
	// SHADOW gate => shadow mods and admins pass, regular mods don't.
	if !permissions.HasPermission(shadow, tp.reqPerms) {
		t.Error("a shadow mod should be able to run /truepossess")
	}
	if !permissions.HasPermission(admin, tp.reqPerms) {
		t.Error("an admin should be able to run /truepossess")
	}
	if permissions.HasPermission(regularMod, tp.reqPerms) {
		t.Error("a regular (non-shadow) mod should NOT be able to run /truepossess")
	}

	if hide := Commands["hide"]; hide.reqPerms != shadow {
		t.Errorf("hide reqPerms = %v, want SHADOW (%v) so shadow mods can vanish", hide.reqPerms, shadow)
	}
	if permissions.HasPermission(regularMod, Commands["hide"].reqPerms) {
		t.Error("a regular (non-shadow) mod should NOT be able to run /hide")
	}
	if unp := Commands["unpossess"]; unp.reqPerms != shadow {
		t.Errorf("unpossess reqPerms = %v, want SHADOW (%v) so shadow mods can stop a /truepossess", unp.reqPerms, shadow)
	}
	// /fullpossess now also silences the target and is shadow-accessible.
	if fp := Commands["fullpossess"]; fp.reqPerms != shadow {
		t.Errorf("fullpossess reqPerms = %v, want SHADOW (%v)", fp.reqPerms, shadow)
	}
	if permissions.HasPermission(regularMod, Commands["fullpossess"].reqPerms) {
		t.Error("a regular (non-shadow) mod should NOT be able to run /fullpossess")
	}
	if al, ok := Commands["adminlock"]; !ok {
		t.Error("adminlock is not registered")
	} else if al.reqPerms != admin {
		t.Errorf("adminlock reqPerms = %v, want ADMIN (%v)", al.reqPerms, admin)
	}
}

// TestAdminLockBlocksNonAdmins verifies /adminlock seals an area against every
// non-admin — including a BYPASS_LOCK moderator and a shadow mod — while admins
// still enter, and that lifting it reopens the area.
func TestAdminLockBlocksNonAdmins(t *testing.T) {
	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})
	newTestClients(t)

	origAreas := areas
	t.Cleanup(func() { areas = origAreas })
	landing := area.NewArea(area.AreaData{Name: "Landing"}, len(getCharacters()), 10, area.EviAny)
	sealed := area.NewArea(area.AreaData{Name: "Sealed"}, len(getCharacters()), 10, area.EviAny)
	areas = []*area.Area{landing, sealed}

	sealed.SetAdminLocked(true)
	sealed.SetLock(area.LockLocked)

	mkClient := func(uid int, perms uint64) *Client {
		c := &Client{conn: &testConn{}, uid: uid, char: 0, forcePairUID: -1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
		c.SetPerms(perms)
		c.SetArea(landing)
		clients.AddClient(c)
		clients.RegisterUID(c)
		return c
	}

	// A BYPASS_LOCK mod and a shadow mod would normally walk through /lock — but
	// not /adminlock. Pre-invite them too, to prove the invite escape hatch is
	// also closed.
	bypassMod := mkClient(1, permissions.PermissionField["BYPASS_LOCK"]|permissions.PermissionField["MUTE"])
	shadowMod := mkClient(2, permissions.PermissionField["SHADOW"]|permissions.PermissionField["BYPASS_LOCK"])
	admin := mkClient(3, permissions.PermissionField["ADMIN"])
	sealed.AddInvited(bypassMod.Uid())
	sealed.AddInvited(shadowMod.Uid())

	if bypassMod.ChangeArea(sealed) {
		t.Error("a BYPASS_LOCK mod should NOT enter an admin-locked area")
	}
	if shadowMod.ChangeArea(sealed) {
		t.Error("a shadow mod should NOT enter an admin-locked area")
	}
	if !admin.ChangeArea(sealed) {
		t.Error("an admin SHOULD enter an admin-locked area")
	}

	// Lift the seal: the mod can now enter.
	sealed.SetAdminLocked(false)
	sealed.SetLock(area.LockFree)
	if !bypassMod.ChangeArea(sealed) {
		t.Error("after lifting the admin-lock, the mod should be able to enter")
	}
}
