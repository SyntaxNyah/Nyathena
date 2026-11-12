package athena

// Nyathena fork addition: tests for /pairorder (reorders the group roster).

import (
	"reflect"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// makeAcceptedGroup forms a 3-person consent group [initiator, p1, p2] and
// returns the three clients plus the group.
func makeAcceptedGroup(t *testing.T) (*Client, *Client, *Client, *PairGroup) {
	t.Helper()
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	initiator := newPairGroupTestClient(1, 0, a, "A")
	p1 := newPairGroupTestClient(2, 1, a, "B")
	p2 := newPairGroupTestClient(3, 2, a, "C")

	for _, c := range []*Client{initiator, p1, p2} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdGroupPair(initiator, []string{"2", "3"}, "")
	g := initiator.PairGroup()
	if g == nil {
		t.Fatal("initiator was not grouped")
	}
	cmdAccept(p1, nil, "")
	cmdAccept(p2, nil, "")
	return initiator, p1, p2, g
}

// pairOrderUids returns the accepted members' UIDs in roster (z-)order.
func pairOrderUids(g *PairGroup) []int {
	members := g.acceptedMembers()
	out := make([]int, 0, len(members))
	for _, m := range members {
		out = append(out, m.Uid())
	}
	return out
}

func TestPairOrderToggleFlipsSpeaker(t *testing.T) {
	initiator, _, _, g := makeAcceptedGroup(t)

	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("initial order = %v, want [1 2 3]", got)
	}

	// initiator is front-most → toggle moves them to the back.
	cmdPairOrder(initiator, nil, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{2, 3, 1}) {
		t.Fatalf("after toggle order = %v, want [2 3 1]", got)
	}

	// now back-most → toggle moves them to the front again.
	cmdPairOrder(initiator, nil, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("after second toggle order = %v, want [1 2 3]", got)
	}
}

func TestPairOrderMovesSpecificMember(t *testing.T) {
	initiator, _, _, g := makeAcceptedGroup(t)

	// Move p2 (uid 3) to the front: [3,1,2]
	cmdPairOrder(initiator, []string{"3", "front"}, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{3, 1, 2}) {
		t.Fatalf("front order = %v, want [3 1 2]", got)
	}

	// Move p1 (uid 2) one step toward the front: [3,2,1]
	cmdPairOrder(initiator, []string{"2", "up"}, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{3, 2, 1}) {
		t.Fatalf("up order = %v, want [3 2 1]", got)
	}

	// Move the initiator (uid 1) one step toward the front: [3,1,2]
	cmdPairOrder(initiator, []string{"1", "up"}, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{3, 1, 2}) {
		t.Fatalf("second up order = %v, want [3 1 2]", got)
	}

	// Move p2 (uid 3) one step toward the back: [1,3,2]
	cmdPairOrder(initiator, []string{"3", "down"}, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, []int{1, 3, 2}) {
		t.Fatalf("down order = %v, want [1 3 2]", got)
	}
}

func TestPairOrderRejectsOutsider(t *testing.T) {
	initiator, _, _, g := makeAcceptedGroup(t)

	outsider := newPairGroupTestClient(9, 0, initiator.Area(), "Z")
	clients.AddClient(outsider)
	clients.RegisterUID(outsider)

	// The outsider is not in the group, so the move must be a no-op.
	before := pairOrderUids(g)
	cmdPairOrder(initiator, []string{"9", "front"}, "")
	if got := pairOrderUids(g); !reflect.DeepEqual(got, before) {
		t.Fatalf("outsider move changed order: %v → %v", before, got)
	}
}
