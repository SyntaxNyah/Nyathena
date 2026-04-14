package athena

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// capturingConn is a net.Conn that records every Write call for inspection.
type capturingConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *capturingConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (c *capturingConn) Close() error                       { return nil }
func (c *capturingConn) LocalAddr() net.Addr                { return testAddr("local") }
func (c *capturingConn) RemoteAddr() net.Addr               { return testAddr("remote") }
func (c *capturingConn) SetDeadline(_ time.Time) error      { return nil }
func (c *capturingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *capturingConn) SetWriteDeadline(_ time.Time) error { return nil }

func (c *capturingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *capturingConn) Written() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func TestTungSetsForcedIniswapToTargetCharIDForUID(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)

	admin := &Client{conn: &testConn{}, uid: 1, pair: ClientPairInfo{wanted_id: -1}}
	admin.SetCharID(0)
	admin.SetArea(a)
	target := &Client{conn: &testConn{}, uid: 2, pair: ClientPairInfo{wanted_id: -1}}
	target.SetCharID(2)
	target.SetArea(a)

	clients.AddClient(admin)
	clients.RegisterUID(admin)
	clients.AddClient(target)
	clients.RegisterUID(target)

	cmdTung(admin, []string{strconv.Itoa(target.Uid())}, "Usage: /tung <uid> [off] | /tung global [off]")

	gotName, gotID := target.ForcedIniswapInfo()
	if gotName != tungForcedCharacterName {
		t.Fatalf("forced iniswap name = %q, want %q", gotName, tungForcedCharacterName)
	}
	if gotID != "-1" {
		t.Fatalf("forced iniswap id = %q, want char id of %q (%q)", gotID, tungForcedCharacterName, "-1")
	}
}

func TestTungGlobalSetsForcedIniswapToEachClientCharID(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey", "Franziska von Karma"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	adminArea := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)
	otherArea := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)

	admin := &Client{conn: &testConn{}, uid: 10, pair: ClientPairInfo{wanted_id: -1}}
	admin.SetCharID(0)
	admin.SetArea(adminArea)
	inArea := &Client{conn: &testConn{}, uid: 11, pair: ClientPairInfo{wanted_id: -1}}
	inArea.SetCharID(2)
	inArea.SetArea(adminArea)
	outArea := &Client{conn: &testConn{}, uid: 12, pair: ClientPairInfo{wanted_id: -1}}
	outArea.SetCharID(3)
	outArea.SetArea(otherArea)

	clients.AddClient(admin)
	clients.RegisterUID(admin)
	clients.AddClient(inArea)
	clients.RegisterUID(inArea)
	clients.AddClient(outArea)
	clients.RegisterUID(outArea)

	cmdTung(admin, []string{"global"}, "Usage: /tung <uid> [off] | /tung global [off]")

	for _, c := range []*Client{admin, inArea} {
		gotName, gotID := c.ForcedIniswapInfo()
		if gotName != tungForcedCharacterName {
			t.Fatalf("uid %d forced iniswap name = %q, want %q", c.Uid(), gotName, tungForcedCharacterName)
		}
		if gotID != "-1" {
			t.Fatalf("uid %d forced iniswap id = %q, want char id of %q (%q)", c.Uid(), gotID, tungForcedCharacterName, "-1")
		}
	}

	gotName, gotID := outArea.ForcedIniswapInfo()
	if gotName != "" || gotID != "" {
		t.Fatalf("out-of-area client should be unchanged, got name=%q id=%q", gotName, gotID)
	}
}

// TestTungSendsPVToTargetWhenCharInList verifies that when the tung character is
// present in the server's characters list, /tung sends a PV#0#CID#<id>#% packet
// directly to the target so their emote panel updates on their own screen.
func TestTungSendsPVToTargetWhenCharInList(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	// Put the tung character in the list at a known index.
	const tungIndex = 1
	characters = []string{"Phoenix Wright", tungForcedCharacterName, "Maya Fey"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)

	admin := &Client{conn: &testConn{}, uid: 1, pair: ClientPairInfo{wanted_id: -1}}
	admin.SetCharID(0)
	admin.SetArea(a)

	const targetOrigCharID = 2 // Maya Fey
	targetConn := &capturingConn{}
	target := &Client{conn: targetConn, uid: 2, pair: ClientPairInfo{wanted_id: -1}}
	target.SetCharID(targetOrigCharID)
	target.SetArea(a)

	clients.AddClient(admin)
	clients.RegisterUID(admin)
	clients.AddClient(target)
	clients.RegisterUID(target)

	cmdTung(admin, []string{strconv.Itoa(target.Uid())}, "Usage: /tung <uid> [off] | /tung global [off]")

	// Expect PV#0#CID#<tungIndex>#%
	wantPV := fmt.Sprintf("PV#0#CID#%d#%%", tungIndex)
	if got := targetConn.Written(); !bytes.Contains([]byte(got), []byte(wantPV)) {
		t.Errorf("target connection did not receive %q; got %q", wantPV, got)
	}

	// Now remove tung and verify the original char ID is restored.
	targetConn.mu.Lock()
	targetConn.buf.Reset()
	targetConn.mu.Unlock()

	cmdTung(admin, []string{strconv.Itoa(target.Uid()), "off"}, "Usage: /tung <uid> [off] | /tung global [off]")

	wantRestore := fmt.Sprintf("PV#0#CID#%d#%%", targetOrigCharID)
	if got := targetConn.Written(); !bytes.Contains([]byte(got), []byte(wantRestore)) {
		t.Errorf("target connection did not receive restore packet %q; got %q", wantRestore, got)
	}
}

// TestTungNoPVWhenCharNotInList verifies that when the tung character is NOT in
// the characters list, /tung does not send a PV packet (which would supply a
// bogus -1 slot ID to the client).
func TestTungNoPVWhenCharNotInList(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	// tung character absent from the list.
	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)

	admin := &Client{conn: &testConn{}, uid: 1, pair: ClientPairInfo{wanted_id: -1}}
	admin.SetCharID(0)
	admin.SetArea(a)

	targetConn := &capturingConn{}
	target := &Client{conn: targetConn, uid: 2, pair: ClientPairInfo{wanted_id: -1}}
	target.SetCharID(2)
	target.SetArea(a)

	clients.AddClient(admin)
	clients.RegisterUID(admin)
	clients.AddClient(target)
	clients.RegisterUID(target)

	cmdTung(admin, []string{strconv.Itoa(target.Uid())}, "Usage: /tung <uid> [off] | /tung global [off]")

	// Must NOT contain PV#0#CID#-1 — that would pass an invalid slot to the client.
	badPV := "PV#0#CID#-1"
	if got := targetConn.Written(); bytes.Contains([]byte(got), []byte(badPV)) {
		t.Errorf("target connection should not receive %q when tung char is not in list; got %q", badPV, got)
	}
}

// TestTungLocksCharacterChange verifies that a tunged client cannot change
// characters via pktChangeChar (the CC packet), and that the lock is lifted
// once the tung effect is removed.
func TestTungLocksCharacterChange(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	characters = []string{"Phoenix Wright", tungForcedCharacterName, "Maya Fey"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	// Disable rate limiting so pktChangeChar doesn't panic on nil config.
	origCfg := config
	t.Cleanup(func() { config = origCfg })
	config = &settings.Config{}

	a := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)

	admin := &Client{conn: &testConn{}, uid: 1, pair: ClientPairInfo{wanted_id: -1}}
	admin.SetCharID(0)
	admin.SetArea(a)

	target := &Client{conn: &testConn{}, uid: 2, charStuckCharID: -1, pair: ClientPairInfo{wanted_id: -1}}
	target.SetCharID(0) // Phoenix Wright
	target.SetArea(a)

	clients.AddClient(admin)
	clients.RegisterUID(admin)
	clients.AddClient(target)
	clients.RegisterUID(target)

	// Apply tung.
	cmdTung(admin, []string{strconv.Itoa(target.Uid())}, "")

	if !target.IsTunged() {
		t.Fatal("expected target to be tunged after /tung")
	}

	// Attempt character change via pktChangeChar — should be blocked.
	// Build a minimal packet with Body[1] = "2" (Maya Fey index).
	p := &packet.Packet{Body: []string{"CC", "2"}}
	pktChangeChar(target, p)

	if target.CharID() != 0 {
		t.Errorf("tunged client should be blocked from changing character; char ID = %d, want 0", target.CharID())
	}

	// Remove tung.
	cmdTung(admin, []string{strconv.Itoa(target.Uid()), "off"}, "")

	if target.IsTunged() {
		t.Fatal("expected tung to be cleared after /tung off")
	}

	// Character change should now succeed.
	pktChangeChar(target, p)
	if target.CharID() != 2 {
		t.Errorf("untunged client should be able to change character; char ID = %d, want 2", target.CharID())
	}
}

func TestAreaIniswapAppliesAndClearsInCurrentArea(t *testing.T) {
	origChars := characters
	t.Cleanup(func() { characters = origChars })
	characters = []string{"Phoenix Wright", "Miles Edgeworth", "Maya Fey"}

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	adminArea := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)
	otherArea := area.NewArea(area.AreaData{}, len(characters), 10, area.EviAny)

	admin := &Client{conn: &testConn{}, uid: 1, pair: ClientPairInfo{wanted_id: -1}}
	admin.SetCharID(0)
	admin.SetArea(adminArea)

	inArea := &Client{conn: &testConn{}, uid: 2, pair: ClientPairInfo{wanted_id: -1}}
	inArea.SetCharID(1)
	inArea.SetArea(adminArea)

	outArea := &Client{conn: &testConn{}, uid: 3, pair: ClientPairInfo{wanted_id: -1}}
	outArea.SetCharID(2)
	outArea.SetArea(otherArea)

	for _, c := range []*Client{admin, inArea, outArea} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdAreaIniswap(admin, []string{"Maya", "Fey"}, "Usage: /areainiswap <character name> | /areainiswap off")

	wantID := strconv.Itoa(getCharacterID("Maya Fey"))
	for _, c := range []*Client{admin, inArea} {
		gotName, gotID := c.ForcedIniswapInfo()
		if gotName != "Maya Fey" || gotID != wantID {
			t.Fatalf("uid %d forced iniswap = (%q,%q), want (%q,%q)", c.Uid(), gotName, gotID, "Maya Fey", wantID)
		}
	}
	if gotName, gotID := outArea.ForcedIniswapInfo(); gotName != "" || gotID != "" {
		t.Fatalf("out-of-area client should be unchanged, got (%q,%q)", gotName, gotID)
	}

	cmdAreaIniswap(admin, []string{"off"}, "Usage: /areainiswap <character name> | /areainiswap off")
	for _, c := range []*Client{admin, inArea} {
		if gotName, gotID := c.ForcedIniswapInfo(); gotName != "" || gotID != "" {
			t.Fatalf("uid %d forced iniswap should be cleared, got (%q,%q)", c.Uid(), gotName, gotID)
		}
	}
}
