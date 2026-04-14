package athena

import (
	"strconv"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

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
	if gotID != target.CharIDStr() {
		t.Fatalf("forced iniswap id = %q, want target char id %q", gotID, target.CharIDStr())
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
		if gotID != c.CharIDStr() {
			t.Fatalf("uid %d forced iniswap id = %q, want %q", c.Uid(), gotID, c.CharIDStr())
		}
	}

	gotName, gotID := outArea.ForcedIniswapInfo()
	if gotName != "" || gotID != "" {
		t.Fatalf("out-of-area client should be unchanged, got name=%q id=%q", gotName, gotID)
	}
}
