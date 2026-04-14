package athena

import (
	"strconv"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

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

// TestTungLocksCharacterChange verifies that a tunged client cannot change
// characters via pktChangeChar (the CC packet), and that the lock is lifted
// once /tung global off is applied.
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

	// Apply tung globally (affects everyone in the area, including target).
	cmdTung(admin, []string{"global"}, "")

	if !target.IsTunged() {
		t.Fatal("expected target to be tunged after /tung global")
	}

	// Attempt character change via pktChangeChar — should be blocked.
	// Build a minimal packet with Body[1] = "2" (Maya Fey index).
	p := &packet.Packet{Body: []string{"CC", "2"}}
	pktChangeChar(target, p)

	if target.CharID() != 0 {
		t.Errorf("tunged client should be blocked from changing character; char ID = %d, want 0", target.CharID())
	}

	// Remove tung globally.
	cmdTung(admin, []string{"global", "off"}, "")

	if target.IsTunged() {
		t.Fatal("expected tung to be cleared after /tung global off")
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
