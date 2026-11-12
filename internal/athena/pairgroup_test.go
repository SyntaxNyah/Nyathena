package athena

// Nyathena fork addition: tests for /forcegrouppair (moderator excluded unless
// they list themselves), /grouppair self-pair rejection, and /unforcegrouppair
// (disbands only mod-forced groups in the area).

import (
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// newPairGroupTestClient builds a minimal in-area client for group-pair tests.
func newPairGroupTestClient(uid int, charID int, a *area.Area, name string) *Client {
	return &Client{
		conn:    &testConn{},
		uid:     uid,
		char:    charID,
		area:    a,
		oocName: name,
	}
}

// TestForceGroupPairExcludesModeratorWhenNotListed verifies /forcegrouppair
// groups only the listed players; an issuing moderator who doesn't list their
// own UID stays ungrouped.
func TestForceGroupPairExcludesModeratorWhenNotListed(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	mod := newPairGroupTestClient(99, -1, a, "Mod")
	p1 := newPairGroupTestClient(1, 0, a, "A")
	p2 := newPairGroupTestClient(2, 0, a, "B")

	for _, c := range []*Client{mod, p1, p2} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdForceGroupPair(mod, []string{"1", "2"}, "")

	if mod.PairGroup() != nil {
		t.Fatalf("moderator was added to the force-paired group: %v", mod.PairGroup())
	}
	if p1.PairGroup() == nil || p2.PairGroup() == nil {
		t.Fatal("targets were not grouped by /forcegrouppair")
	}
	if p1.PairGroup() != p2.PairGroup() {
		t.Fatal("targets ended up in different groups")
	}
	if !p1.PairGroup().IsForced() {
		t.Fatal("group was not marked forced")
	}
}

// TestForceGroupPairIncludesModeratorWhenListed verifies a moderator can join
// their own forced group by listing their own UID alongside the targets.
func TestForceGroupPairIncludesModeratorWhenListed(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	mod := newPairGroupTestClient(99, 0, a, "Mod")
	p1 := newPairGroupTestClient(1, 0, a, "A")

	for _, c := range []*Client{mod, p1} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdForceGroupPair(mod, []string{"99", "1"}, "")

	if mod.PairGroup() == nil {
		t.Fatal("moderator was not added when listing their own UID")
	}
	if p1.PairGroup() != mod.PairGroup() {
		t.Fatal("moderator and target ended up in different groups")
	}
	if !mod.PairGroup().IsForced() {
		t.Fatal("group was not marked forced")
	}
}

// TestGroupPairRejectsSelfPairing verifies the consent /grouppair path still
// refuses to pair a player with themselves.
func TestGroupPairRejectsSelfPairing(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	initiator := newPairGroupTestClient(1, 0, a, "A")

	clients.AddClient(initiator)
	clients.RegisterUID(initiator)

	cmdGroupPair(initiator, []string{"1"}, "")

	if initiator.PairGroup() != nil {
		t.Fatal("self-pairing /grouppair was not rejected")
	}
}

// TestUnforceGroupPairDissolvesOnlyForcedGroupsInArea verifies that
// /unforcegrouppair disbands force-paired groups in the caller's area, leaves
// consent-based groups alone, and does not touch other areas.
func TestUnforceGroupPairDissolvesOnlyForcedGroupsInArea(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)
	b := area.NewArea(area.AreaData{Name: "Basement"}, len(getCharacters()), 10, area.EviAny)

	mod := newPairGroupTestClient(99, -1, a, "Mod")
	p1 := newPairGroupTestClient(1, 0, a, "A")
	p2 := newPairGroupTestClient(2, 0, a, "B")
	initiator := newPairGroupTestClient(3, 0, a, "C")
	consent := newPairGroupTestClient(4, 0, a, "D")

	modB := newPairGroupTestClient(98, -1, b, "ModB")
	q1 := newPairGroupTestClient(5, 0, b, "E")
	q2 := newPairGroupTestClient(6, 0, b, "F")

	for _, c := range []*Client{mod, p1, p2, initiator, consent, modB, q1, q2} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	// Forced group in the mod's area.
	cmdForceGroupPair(mod, []string{"1", "2"}, "")
	// Consent group in the mod's area.
	cmdGroupPair(initiator, []string{"4"}, "")
	// Forced group in a different area.
	cmdForceGroupPair(modB, []string{"5", "6"}, "")

	cmdUnforceGroupPair(mod, nil, "")

	if p1.PairGroup() != nil || p2.PairGroup() != nil {
		t.Fatal("force-paired group in the caller's area was not disbanded")
	}
	if initiator.PairGroup() == nil || consent.PairGroup() == nil {
		t.Fatal("consent-based group was wrongly disbanded")
	}
	if initiator.PairGroup().IsForced() {
		t.Fatal("consent-based group was marked forced")
	}
	if q1.PairGroup() == nil || q2.PairGroup() == nil {
		t.Fatal("force-paired group in another area was wrongly disbanded")
	}
}
