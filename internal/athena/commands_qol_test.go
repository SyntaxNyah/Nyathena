package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// /clients iterates connections that can briefly have a UID but no area while
// a join or cleanup is in flight. It must report "unknown" rather than
// dereferencing a nil *Area and taking down the packet handler.
func TestCmdClientsHandlesNilArea(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })

	cl := &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, 1, 1, area.EviAny)
	caller := &Client{uid: 1, ipid: "shared-ipid", area: a, char: -1, sendCh: make(chan []byte, 8)}
	other := &Client{uid: 2, ipid: "shared-ipid", char: -1, sendCh: make(chan []byte, 8)}
	cl.AddClient(caller)
	cl.AddClient(other)
	cl.RegisterUID(caller)
	cl.RegisterUID(other)
	clients = cl

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("cmdClients panicked on a client with no area: %v", r)
		}
	}()
	cmdClients(caller, []string{"1"}, "/clients <uid>")

	select {
	case msg := <-caller.sendCh:
		if !strings.Contains(string(msg), "unknown") {
			t.Fatalf("nil-area connection was not reported safely: %q", msg)
		}
	default:
		t.Fatal("cmdClients sent no response")
	}
}
