package athena

import (
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// captureConn records everything the server writes to a client so tests can
// assert on packet content.
type captureConn struct {
	mu     sync.Mutex
	closed bool
	buf    strings.Builder
}

func (c *captureConn) Read(_ []byte) (int, error) { return 0, io.EOF }

func (c *captureConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	c.buf.Write(p)
	return len(p), nil
}

func (c *captureConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *captureConn) LocalAddr() net.Addr                { return testAddr("local") }
func (c *captureConn) RemoteAddr() net.Addr               { return testAddr("remote") }
func (c *captureConn) SetDeadline(_ time.Time) error      { return nil }
func (c *captureConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *captureConn) SetWriteDeadline(_ time.Time) error { return nil }

func (c *captureConn) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// resetVoiceRooms clears the package-level voice room state.  Tests run in
// sequence and share this global, so every test that touches voice must call
// this first to avoid bleed-over.
func resetVoiceRooms() {
	voiceMu.Lock()
	voiceRooms = map[*area.Area]map[int]struct{}{}
	voiceMu.Unlock()
}

func voiceTestConfig(enabled bool, maxPeers int) *settings.Config {
	cfg := &settings.Config{}
	cfg.VoiceConfig.EnableVoice = enabled
	cfg.VoiceConfig.PTTOnly = true
	cfg.VoiceConfig.MaxPeersPerArea = maxPeers
	cfg.VoiceConfig.MaxFrameBytes = 4000
	return cfg
}

func TestSendVoiceCapsEmitsAudioParameters(t *testing.T) {
	origConfig := config
	t.Cleanup(func() { config = origConfig })

	cases := []struct {
		name        string
		enabled     bool
		wantInOrder []string
	}{
		// Disabled branch advertises enabled=0 with the same audio params,
		// so a client can still display "voice unavailable" coherently.
		{"disabled", false, []string{"VS_CAPS#0#1#0#opus#48000#20#4000#%"}},
		// Enabled branch carries the configured max peers and the same
		// codec/sample-rate/frame-ms triple.
		{"enabled", true, []string{"VS_CAPS#1#1#6#opus#48000#20#4000#%"}},
	}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := voiceTestConfig(tc.enabled, 6)
			config = cfg

			client, conn := newVoiceClient(t, 1, a)
			sendVoiceCaps(client)
			out := conn.String()
			for _, want := range tc.wantInOrder {
				if !strings.Contains(out, want) {
					t.Errorf("sendVoiceCaps output missing %q\n  got: %q", want, out)
				}
			}
		})
	}
}

func newVoiceClient(t *testing.T, uid int, a *area.Area) (*Client, *captureConn) {
	t.Helper()
	conn := &captureConn{}
	c := &Client{conn: conn, uid: uid, pair: ClientPairInfo{wanted_id: -1}}
	c.SetArea(a)
	return c, conn
}

func TestPktVSJoinBroadcastsAndSendsPeerList(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, aliceConn := newVoiceClient(t, 1, a)
	bob, bobConn := newVoiceClient(t, 2, a)

	clients.AddClient(alice)
	clients.RegisterUID(alice)
	clients.AddClient(bob)
	clients.RegisterUID(bob)

	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(bob, &packet.Packet{Header: "VS_JOIN"})

	// Alice joined first — she gets an empty VS_PEERS and should then receive
	// bob's VS_JOIN broadcast.
	aliceOut := aliceConn.String()
	if !strings.Contains(aliceOut, "VS_PEERS##%") {
		t.Errorf("alice did not receive empty VS_PEERS, got: %q", aliceOut)
	}
	if !strings.Contains(aliceOut, "VS_JOIN#2#%") {
		t.Errorf("alice did not receive VS_JOIN for bob, got: %q", aliceOut)
	}

	// Bob joined second — his VS_PEERS should list alice, and he should NOT
	// receive his own VS_JOIN broadcast.
	bobOut := bobConn.String()
	if !strings.Contains(bobOut, "VS_PEERS#1#%") {
		t.Errorf("bob did not receive VS_PEERS with alice, got: %q", bobOut)
	}
	if strings.Contains(bobOut, "VS_JOIN#2#%") {
		t.Errorf("bob received his own VS_JOIN broadcast: %q", bobOut)
	}
}

func TestPktVSJoinRejectsWhenVoiceDisabled(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(false, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, _ := newVoiceClient(t, 1, a)
	clients.AddClient(alice)
	clients.RegisterUID(alice)

	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	if inVoiceRoom(a, 1) {
		t.Fatal("alice was added to voice room even though voice is disabled")
	}
}

func TestPktVSJoinRejectsAtMaxPeers(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(true, 2)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	c1, _ := newVoiceClient(t, 1, a)
	c2, _ := newVoiceClient(t, 2, a)
	c3, conn3 := newVoiceClient(t, 3, a)
	for _, c := range []*Client{c1, c2, c3} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	pktVSJoin(c1, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(c2, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(c3, &packet.Packet{Header: "VS_JOIN"})

	if inVoiceRoom(a, 3) {
		t.Fatal("third peer was admitted past the configured max")
	}
	if !strings.Contains(conn3.String(), "Voice chat is full") {
		t.Errorf("third peer did not receive full notice, got: %q", conn3.String())
	}
}

func TestPktVSFrameBroadcastsToOtherPeersOnly(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, aliceConn := newVoiceClient(t, 1, a)
	bob, bobConn := newVoiceClient(t, 2, a)
	carol, carolConn := newVoiceClient(t, 3, a)
	for _, c := range []*Client{alice, bob, carol} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(bob, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(carol, &packet.Packet{Header: "VS_JOIN"})

	aliceConn.buf.Reset()
	bobConn.buf.Reset()
	carolConn.buf.Reset()

	pktVSFrame(alice, &packet.Packet{Header: "VS_FRAME", Body: []string{"OPUSPAYLOAD"}})

	if !strings.Contains(bobConn.String(), "VS_AUDIO#1#OPUSPAYLOAD#%") {
		t.Errorf("bob did not receive relayed audio, got: %q", bobConn.String())
	}
	if !strings.Contains(carolConn.String(), "VS_AUDIO#1#OPUSPAYLOAD#%") {
		t.Errorf("carol did not receive relayed audio, got: %q", carolConn.String())
	}
	if strings.Contains(aliceConn.String(), "VS_AUDIO") {
		t.Errorf("alice received her own audio echoed back, got: %q", aliceConn.String())
	}
}

func TestPktVSFrameIgnoresCrossAreaTarget(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a1 := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	a2 := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, _ := newVoiceClient(t, 1, a1)
	bob, bobConn := newVoiceClient(t, 2, a2)
	for _, c := range []*Client{alice, bob} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(bob, &packet.Packet{Header: "VS_JOIN"})

	bobConn.buf.Reset()
	pktVSFrame(alice, &packet.Packet{Header: "VS_FRAME", Body: []string{"OPUSPAYLOAD"}})

	if strings.Contains(bobConn.String(), "VS_AUDIO") {
		t.Errorf("bob in another area should not receive audio, got: %q", bobConn.String())
	}
}

func TestPktVSFrameDropsOversized(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(true, 6)
	config.MaxFrameBytes = 16
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, _ := newVoiceClient(t, 1, a)
	bob, bobConn := newVoiceClient(t, 2, a)
	for _, c := range []*Client{alice, bob} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(bob, &packet.Packet{Header: "VS_JOIN"})
	bobConn.buf.Reset()

	// 32 bytes — twice the 16-byte cap.
	pktVSFrame(alice, &packet.Packet{Header: "VS_FRAME", Body: []string{strings.Repeat("A", 32)}})

	if strings.Contains(bobConn.String(), "VS_AUDIO") {
		t.Errorf("bob received an oversized frame that should have been dropped, got: %q", bobConn.String())
	}
}

func TestLeaveVoiceForClientBroadcastsLeave(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
	})
	resetVoiceRooms()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, _ := newVoiceClient(t, 1, a)
	bob, bobConn := newVoiceClient(t, 2, a)
	for _, c := range []*Client{alice, bob} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(bob, &packet.Packet{Header: "VS_JOIN"})
	bobConn.buf.Reset()

	leaveVoiceForClient(alice)

	if inVoiceRoom(a, 1) {
		t.Fatal("alice still in voice room after leaveVoiceForClient")
	}
	if !strings.Contains(bobConn.String(), "VS_LEAVE#1#%") {
		t.Errorf("bob did not receive VS_LEAVE, got: %q", bobConn.String())
	}
}
