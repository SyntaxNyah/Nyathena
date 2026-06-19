package athena

// Nyathena fork addition: tests for auto-unpair on disconnect.

import "testing"

// TestClearPairLinksOnDisconnect verifies that when one partner of a
// force-pair leaves the server, clearPairLinksOnDisconnect dissolves the pair
// on BOTH sides — clearing the surviving partner's ForcePairUID (a stale UID
// pointer that previously caused IC pair desyncs) and PairWantedID, plus the
// leaver's own state.
func TestClearPairLinksOnDisconnect(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := &Client{conn: &testConn{}, uid: 1, char: 10}
	b := &Client{conn: &testConn{}, uid: 2, char: 20}
	clients.AddClient(a)
	clients.AddClient(b)

	// Establish a mutual force-pair, exactly as cmdPair would after both accept.
	a.SetPairWantedID(b.CharID())
	a.SetForcePairUID(b.Uid())
	b.SetPairWantedID(a.CharID())
	b.SetForcePairUID(a.Uid())

	// B disconnects.
	clearPairLinksOnDisconnect(b)

	if a.ForcePairUID() != -1 {
		t.Errorf("surviving partner ForcePairUID = %d, want -1", a.ForcePairUID())
	}
	if a.PairWantedID() != -1 {
		t.Errorf("surviving partner PairWantedID = %d, want -1", a.PairWantedID())
	}
	if b.ForcePairUID() != -1 || b.PairWantedID() != -1 {
		t.Errorf("leaver pair state not cleared: ForcePairUID=%d PairWantedID=%d", b.ForcePairUID(), b.PairWantedID())
	}
}

// TestClearPairLinksOnDisconnectPendingRequest covers a one-sided pending
// request (the requester leaves before the target accepts): the target, who
// only references the leaver by PairWantedID == leaver's CharID, is cleared.
func TestClearPairLinksOnDisconnectPendingRequest(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	requester := &Client{conn: &testConn{}, uid: 1, char: 10}
	target := &Client{conn: &testConn{}, uid: 2, char: 20}
	clients.AddClient(requester)
	clients.AddClient(target)

	// requester wants target; target has reciprocated interest by CharID but no
	// force-pair has completed yet.
	requester.SetPairWantedID(target.CharID())
	target.SetPairWantedID(requester.CharID())

	clearPairLinksOnDisconnect(requester)

	if target.PairWantedID() != -1 {
		t.Errorf("target PairWantedID = %d, want -1 after requester left", target.PairWantedID())
	}
}

// TestClearPairLinksOnDisconnectNoPair verifies an unpaired client leaving
// does not disturb a bystander paired with someone else.
func TestClearPairLinksOnDisconnectNoPair(t *testing.T) {
	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	leaver := &Client{conn: &testConn{}, uid: 1, char: 10}
	bystander := &Client{conn: &testConn{}, uid: 2, char: 20}
	bystander.SetForcePairUID(99) // paired with someone else entirely
	bystander.SetPairWantedID(98)
	clients.AddClient(leaver)
	clients.AddClient(bystander)

	clearPairLinksOnDisconnect(leaver)

	if bystander.ForcePairUID() != 99 || bystander.PairWantedID() != 98 {
		t.Errorf("bystander pair state disturbed: ForcePairUID=%d PairWantedID=%d", bystander.ForcePairUID(), bystander.PairWantedID())
	}
}
