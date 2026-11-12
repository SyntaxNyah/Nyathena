package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

func setupChangeCharTest(t *testing.T) (*area.Area, *Client, *ClientList) {
	t.Helper()
	origChars := getCharacters()
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})
	t.Cleanup(func() { setCharacters(origChars) })

	origClients := clients
	cl := &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}
	clients = cl
	t.Cleanup(func() { clients = origClients })

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)
	c := &Client{uid: 1, ipid: "change-char", char: -1, area: a, sendCh: make(chan []byte, 64)}
	cl.AddClient(c)
	cl.RegisterUID(c)
	return a, c, cl
}

// A client retrying the same CC after missing the PV must be acknowledged, not
// ignored as if the character were taken by someone else.
func TestChangeCharacterIdempotent(t *testing.T) {
	_, c, _ := setupChangeCharTest(t)

	c.ChangeCharacter(1)
	if got := c.CharID(); got != 1 {
		t.Fatalf("first change left char=%d, want 1", got)
	}
	c.ChangeCharacter(1)
	if got := c.CharID(); got != 1 {
		t.Fatalf("retry changed char=%d, want 1", got)
	}

	pvs := 0
	for len(c.sendCh) > 0 {
		b := <-c.sendCh
		if strings.Contains(string(b), "PV#") {
			pvs++
		}
	}
	if pvs < 2 {
		t.Fatalf("retry was not acknowledged with a PV (got %d PV packets)", pvs)
	}
}

// A slot marked taken by a connection that is no longer present must not lock
// every player out of that character forever.
func TestChangeCharacterRepairsStaleSlot(t *testing.T) {
	a, c, _ := setupChangeCharTest(t)
	a.AddChar(1) // stale occupancy: no connected client actually holds char 1

	c.ChangeCharacter(1)
	if got := c.CharID(); got != 1 {
		t.Fatalf("stale slot was not repaired; char=%d, want 1", got)
	}
	if !a.IsTaken(1) {
		t.Error("character 1 should be held by the client after repair")
	}
}

// A genuinely taken slot must produce feedback so the client stops waiting at
// spectator, rather than being silently ignored.
func TestChangeCharacterTakenSlotResponds(t *testing.T) {
	a, c, cl := setupChangeCharTest(t)
	other := &Client{uid: 2, ipid: "other-char", char: 1, area: a, sendCh: make(chan []byte, 8)}
	cl.AddClient(other)
	cl.RegisterUID(other)
	a.AddChar(1)

	c.ChangeCharacter(1)
	if got := c.CharID(); got != -1 {
		t.Fatalf("taken slot changed char=%d, want -1", got)
	}

	var gotCharsCheck, gotMessage bool
	for len(c.sendCh) > 0 {
		b := string(<-c.sendCh)
		if strings.Contains(b, "CharsCheck#") {
			gotCharsCheck = true
		}
		if strings.Contains(b, "already taken") {
			gotMessage = true
		}
	}
	if !gotCharsCheck {
		t.Error("taken slot did not trigger a fresh CharsCheck")
	}
	if !gotMessage {
		t.Error("taken slot did not explain why the request failed")
	}
}
