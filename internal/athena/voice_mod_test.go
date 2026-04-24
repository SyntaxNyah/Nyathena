package athena

import (
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// resetVoiceModState clears every in-memory voice-moderation map so tests
// don't leak state into one another.
func resetVoiceModState() {
	voiceModMu.Lock()
	voiceMutes = map[string]voiceRestriction{}
	voiceBans = map[string]voiceRestriction{}
	voiceJoinEvents = map[int][]time.Time{}
	voiceSigEvents = map[int][]time.Time{}
	voiceFirstSeen = map[string]time.Time{}
	voiceModMu.Unlock()
}

func newVoiceClientWithIPID(t *testing.T, uid int, ipid string, a *area.Area) (*Client, *captureConn) {
	t.Helper()
	conn := &captureConn{}
	c := &Client{conn: conn, uid: uid, ipid: ipid, pair: ClientPairInfo{wanted_id: -1}}
	c.SetArea(a)
	return c, conn
}

func TestPktVCJoinRespectsAreaVoiceAllowed(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
	})
	resetVoiceRooms()
	resetVoiceModState()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewAreaWithVoiceDefault(area.AreaData{}, 10, 10, area.EviAny, false)
	alice, aliceConn := newVoiceClientWithIPID(t, 1, "alice-ip", a)
	clients.AddClient(alice)
	clients.RegisterUID(alice)

	pktVCJoin(alice, &packet.Packet{Header: "VC_JOIN"})

	if inVoiceRoom(a, 1) {
		t.Fatal("alice joined voice despite area voice_allowed = false")
	}
	if !strings.Contains(aliceConn.String(), "Voice chat is not permitted in this area") {
		t.Errorf("alice did not receive area-disabled notice, got: %q", aliceConn.String())
	}
}

func TestPktVCJoinRejectsBannedIPID(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
	})
	resetVoiceRooms()
	resetVoiceModState()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, aliceConn := newVoiceClientWithIPID(t, 1, "alice-ip", a)
	clients.AddClient(alice)
	clients.RegisterUID(alice)

	SetVoiceBan("alice-ip", 0, "testing")

	pktVCJoin(alice, &packet.Packet{Header: "VC_JOIN"})

	if inVoiceRoom(a, 1) {
		t.Fatal("banned alice joined voice")
	}
	if !strings.Contains(aliceConn.String(), "banned from voice chat") {
		t.Errorf("alice did not receive ban notice, got: %q", aliceConn.String())
	}
}

func TestPktVCJoinRespectsMute(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
	})
	resetVoiceRooms()
	resetVoiceModState()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	bob, bobConn := newVoiceClientWithIPID(t, 1, "bob-ip", a)
	clients.AddClient(bob)
	clients.RegisterUID(bob)

	SetVoiceMute("bob-ip", 60*time.Second, "rudeness")

	pktVCJoin(bob, &packet.Packet{Header: "VC_JOIN"})

	if inVoiceRoom(a, 1) {
		t.Fatal("muted bob joined voice")
	}
	if !strings.Contains(bobConn.String(), "muted from voice chat") {
		t.Errorf("bob did not receive mute notice, got: %q", bobConn.String())
	}
}

func TestVoiceJoinRateLimitBlocksExcess(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
	})
	resetVoiceRooms()
	resetVoiceModState()

	config = voiceTestConfig(true, 6)
	config.JoinRateLimit = 2
	config.JoinRateLimitWindow = 60
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	c, conn := newVoiceClientWithIPID(t, 42, "c-ip", a)
	clients.AddClient(c)
	clients.RegisterUID(c)

	for i := 0; i < 2; i++ {
		pktVCJoin(c, &packet.Packet{Header: "VC_JOIN"})
		leaveVoiceForClient(c) // leave so we can re-join up to the limit
	}
	conn.buf.Reset()
	pktVCJoin(c, &packet.Packet{Header: "VC_JOIN"})

	if inVoiceRoom(a, 42) {
		t.Fatal("third VC_JOIN admitted despite join_rate_limit = 2")
	}
	if !strings.Contains(conn.String(), "rate limit") {
		t.Errorf("expected rate-limit notice, got: %q", conn.String())
	}
}

func TestKickVoiceByIPIDEjectsAllMatchingClients(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
	})
	resetVoiceRooms()
	resetVoiceModState()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	c1, _ := newVoiceClientWithIPID(t, 1, "shared-ip", a)
	c2, _ := newVoiceClientWithIPID(t, 2, "shared-ip", a)
	c3, _ := newVoiceClientWithIPID(t, 3, "other-ip", a)
	for _, c := range []*Client{c1, c2, c3} {
		clients.AddClient(c)
		clients.RegisterUID(c)
		pktVCJoin(c, &packet.Packet{Header: "VC_JOIN"})
	}

	got := kickVoiceByIPID("shared-ip")
	if got != 2 {
		t.Errorf("kicked %d; expected 2", got)
	}
	if inVoiceRoom(a, 1) || inVoiceRoom(a, 2) {
		t.Fatal("shared-ip clients still in voice room")
	}
	if !inVoiceRoom(a, 3) {
		t.Fatal("unrelated c3 was ejected")
	}
}

func TestKickAllVoiceFromAreaEjectsEveryone(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
	})
	resetVoiceRooms()
	resetVoiceModState()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	c1, _ := newVoiceClientWithIPID(t, 1, "ip-1", a)
	c2, _ := newVoiceClientWithIPID(t, 2, "ip-2", a)
	for _, c := range []*Client{c1, c2} {
		clients.AddClient(c)
		clients.RegisterUID(c)
		pktVCJoin(c, &packet.Packet{Header: "VC_JOIN"})
	}

	kickAllVoiceFromArea(a)

	if inVoiceRoom(a, 1) || inVoiceRoom(a, 2) {
		t.Fatal("voice room not fully drained after kickAllVoiceFromArea")
	}
}

func TestVoiceBanExpires(t *testing.T) {
	t.Cleanup(resetVoiceModState)
	resetVoiceModState()

	SetVoiceBan("short-ip", 1*time.Millisecond, "temp")
	time.Sleep(5 * time.Millisecond)

	if banned, _, _ := IsVoiceBanned("short-ip"); banned {
		t.Fatal("expired voice ban still reports as active")
	}
}

func TestAreaVoiceAllowedDefault(t *testing.T) {
	// With the zero-default NewArea constructor, voice must be allowed.
	a := area.NewArea(area.AreaData{}, 1, 1, area.EviAny)
	if !a.VoiceAllowed() {
		t.Error("NewArea default should allow voice")
	}

	// With a server default of false, an area with no explicit setting
	// inherits false.
	a = area.NewAreaWithVoiceDefault(area.AreaData{}, 1, 1, area.EviAny, false)
	if a.VoiceAllowed() {
		t.Error("area should inherit false from server default")
	}

	// With an explicit Voice_allowed = true, the area setting wins over the
	// server default.
	b := true
	a = area.NewAreaWithVoiceDefault(area.AreaData{Voice_allowed: &b}, 1, 1, area.EviAny, false)
	if !a.VoiceAllowed() {
		t.Error("explicit voice_allowed = true ignored")
	}
}
