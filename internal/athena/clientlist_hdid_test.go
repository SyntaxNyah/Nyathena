package athena

import "testing"

// The per-device multiclient count must be idempotent on registration and must
// be released exactly once when the client leaves, so a reconnect loop cannot
// leak slots and a second tab still counts.
func TestClientListHDIDCounts(t *testing.T) {
	list := &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}
	first := &Client{ipid: "test-ip-one", hdid: "test-hdid-one"}
	second := &Client{ipid: "test-ip-two", hdid: "test-hdid-one"}

	list.AddClient(first)
	list.AddClient(second)
	list.RegisterHDID(first)
	list.RegisterHDID(second)
	list.RegisterHDID(second) // idempotent

	if got := list.CountByHDID("test-hdid-one"); got != 2 {
		t.Fatalf("HDID count = %d, want 2", got)
	}
	if got := list.CountByIPID("test-ip-one"); got != 1 {
		t.Fatalf("IPID count = %d, want 1", got)
	}

	list.RemoveClient(first)
	if got := list.CountByHDID("test-hdid-one"); got != 1 {
		t.Fatalf("HDID count after first removal = %d, want 1", got)
	}
	list.RemoveClient(second)
	if got := list.CountByHDID("test-hdid-one"); got != 0 {
		t.Fatalf("HDID count after second removal = %d, want 0", got)
	}
}
