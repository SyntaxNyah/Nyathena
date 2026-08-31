package athena

import (
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// These tests drive the production hook functions in raidguard_wire.go, not the
// scoring engine directly. The replay harness proves the engine separates a raid
// from ordinary traffic; it says nothing about whether the packet handlers
// actually feed the engine the right values, or whether the feature gate and the
// moderator exemption really hold on the path a real packet takes. A wiring bug
// -- a hook never reached, the wrong field passed, the gate checked in the wrong
// place -- would be invisible to the replay and total in production.

// wireTestClient builds a *Client usable by the hooks without a network or a DB.
func wireTestClient(t *testing.T, ipid string, perms uint64) *Client {
	t.Helper()
	c := &Client{conn: &testConn{}, uid: 1, ipid: ipid, perms: perms}
	c.SetAcceptedAt(time.Now().Add(-500 * time.Millisecond))
	return c
}

// withWiring installs config, an empty client list and an enabled feature gate.
func withWiring(t *testing.T) {
	t.Helper()
	oldConfig, oldClients, oldActive := config, clients, raidGuardActive.Load()
	t.Cleanup(func() {
		config = oldConfig
		clients = oldClients
		raidGuardActive.Store(oldActive)
		resetRaidGuardState()
	})
	config = settings.DefaultConfig()
	clients = &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}
	resetRaidGuardState()
	raidGuardActive.Store(true)
}

func icPacket(showname, shout string) *packet.MSPacket {
	return &packet.MSPacket{Showname: showname, Message: "placeholder", ShoutModifier: shout}
}

// TestWireFeatureGateOff is the property the whole hot path rests on: with the
// guard disabled, a packet must not allocate a raidState or record anything.
func TestWireFeatureGateOff(t *testing.T) {
	withWiring(t)
	raidGuardActive.Store(false)

	c := wireTestClient(t, "gate-off", 0)
	for i := 0; i < 10; i++ {
		raidGuardOnIC(c, icPacket("shouty", "1"), "GET RAPED GET RAPED GET RAPED NOW")
		raidGuardOnOOC(c, "namechurn"+string(rune('a'+i)), "GET RAPED GET RAPED GET RAPED NOW")
		raidGuardOnCharPick(c)
		raidGuardOnHandshakeStep(c)
	}
	c.mu.Lock()
	rs := c.raid
	c.mu.Unlock()
	if rs != nil {
		t.Error("the guard allocated per-connection state while disabled")
	}
}

// TestWireModeratorNeverObserved checks a moderator's traffic never reaches the
// engine at all -- not merely that it is never acted on. An exemption applied
// only at enforcement time would still let a moderator's messages populate the
// cross-IPID correlation window and drag other people's scores up.
func TestWireModeratorNeverObserved(t *testing.T) {
	withWiring(t)
	c := wireTestClient(t, "mod-ipid", permissions.PermissionField["BAN"])
	if !permissions.IsModerator(c.Perms()) {
		t.Fatal("test client is not a moderator; the exemption is not being exercised")
	}
	for i := 0; i < 10; i++ {
		raidGuardOnIC(c, icPacket("shouty", "1"), "GET RAPED GET RAPED GET RAPED NOW")
		raidGuardOnHandshakeStep(c)
	}
	c.mu.Lock()
	rs := c.raid
	c.mu.Unlock()
	if rs != nil {
		t.Error("a moderator's traffic allocated raid-guard state")
	}
	if raidGuardUnderAttack() {
		t.Error("a moderator's traffic moved the server-wide under-attack flag")
	}
}

// TestWireObjectionReachesEngine checks the headline signal survives the trip
// through the real IC hook -- that the objection value the packet handler parsed
// is the one the engine scores.
func TestWireObjectionReachesEngine(t *testing.T) {
	withWiring(t)
	c := wireTestClient(t, "objection-ipid", 0)
	for i := 0; i < 5; i++ {
		raidGuardOnIC(c, icPacket("Mintisanigger", "1"), "you cannot stop the raven")
	}
	rs := c.raidGuard()
	if rs == nil {
		t.Fatal("no raid state was created by the IC hook")
	}
	if !rs.firedSignal(SigObjectionSpam) {
		score, sigs, _ := rs.snapshot()
		t.Errorf("objection signal did not fire through the IC hook (score=%d signals=%v)", score, sigs)
	}
}

// TestWireObjectionZeroIsClean is the negative control: the same traffic without
// the shout modifier must not trip the signal. Without this, a hook that passed
// a constant would pass the test above.
func TestWireObjectionZeroIsClean(t *testing.T) {
	withWiring(t)
	c := wireTestClient(t, "clean-ipid", 0)
	for i := 0; i < 5; i++ {
		raidGuardOnIC(c, icPacket("Phoenix", "0"), "I object to that line of questioning")
	}
	if rs := c.raidGuard(); rs != nil && rs.firedSignal(SigObjectionSpam) {
		t.Error("objection signal fired on messages carrying no shout modifier")
	}
}

// TestWireCharPickTiming checks the two character signals fire through the real
// hook, which is the only place acceptedAt/charPickedAt are read.
func TestWireCharPickTiming(t *testing.T) {
	withWiring(t)
	c := wireTestClient(t, "charpick-ipid", 0)
	c.SetAcceptedAt(time.Now().Add(-50 * time.Millisecond))

	raidGuardOnCharPick(c) // 50ms after accept: no handshake could have finished
	rs := c.raidGuard()
	if rs == nil {
		t.Fatal("no raid state was created by the char-pick hook")
	}
	if !rs.firedSignal(SigFastCharPick) {
		t.Error("fast char pick did not fire through the hook")
	}
	if c.CharPickedAt().IsZero() {
		t.Error("the hook did not record charPickedAt; the charpick->speech delta can never fire")
	}

	raidGuardOnCharPick(c) // immediate re-roll
	if !rs.firedSignal(SigCharChurn) {
		t.Error("character re-roll did not fire through the hook")
	}
}

// TestWireFastCharPickToleratesAutoPick is the regression guard for a real
// false-positive path found by reading the LemmyAO client: its ?char= share-link
// parameter auto-picks a character the instant the handshake completes, with no
// human delay at all. The fastest full handshake in the clean capture was 330ms,
// and six of nineteen finished inside one second -- so the original 1000ms
// threshold would have fired on an ordinary player following a share link.
//
// The threshold must stay below the time a handshake physically takes, since an
// auto-pick cannot happen before the character list has arrived.
func TestWireFastCharPickToleratesAutoPick(t *testing.T) {
	withWiring(t)
	if config.RaidGuardFastCharPickMs >= 330 {
		t.Fatalf("raid_guard_fast_charpick_ms is %dms; the fastest observed real handshake was 330ms, "+
			"so an auto-picking player would be flagged", config.RaidGuardFastCharPickMs)
	}
	// A share-link auto-pick landing right as the fastest observed handshake ends.
	c := wireTestClient(t, "autopick-ipid", 0)
	c.SetAcceptedAt(time.Now().Add(-330 * time.Millisecond))
	raidGuardOnCharPick(c)
	if rs := c.raidGuard(); rs != nil && rs.firedSignal(SigFastCharPick) {
		t.Error("a ?char= auto-pick at the fastest observed handshake time was flagged as a bot")
	}
}

// TestWireHandshakeOrdering checks the ordering signal fires only in the
// direction claimed: RC/RM/RD before this connection's own askchaa. A client
// that completes the handshake properly and later re-requests a list must stay
// clean, since real clients do re-request.
func TestWireHandshakeOrdering(t *testing.T) {
	withWiring(t)

	bad := wireTestClient(t, "bad-order", 0)
	raidGuardOnHandshakeStep(bad)
	rs := bad.raidGuard()
	if rs == nil || !rs.firedSignal(SigHandshakeAnomaly) {
		t.Error("RC/RM/RD before askchaa did not fire the handshake signal")
	}

	good := wireTestClient(t, "good-order", 0)
	raidGuardOnAskchaa(good)
	for i := 0; i < 5; i++ {
		raidGuardOnHandshakeStep(good) // legitimate later re-requests
	}
	if rs := good.raidGuard(); rs != nil && rs.firedSignal(SigHandshakeAnomaly) {
		t.Error("a client that sent askchaa first was flagged for re-requesting a list")
	}
}

// TestWireCorrelationAcrossIPIDs checks the fan-out signal works end to end
// through the OOC hook: one connection repeating itself must never correlate,
// while several distinct connections saying the same thing must.
func TestWireCorrelationAcrossIPIDs(t *testing.T) {
	withWiring(t)
	line := "all of you vibe coders go to hell you are no match for the raven"

	solo := wireTestClient(t, "solo-ipid", 0)
	for i := 0; i < 10; i++ {
		raidGuardOnOOC(solo, "solo", line)
	}
	if rs := solo.raidGuard(); rs != nil && rs.firedSignal(SigDupeAcrossIPIDs) {
		t.Error("one connection repeating itself was treated as coordinated")
	}
	if raidGuardUnderAttack() {
		t.Error("one connection repeating itself flipped the server-wide under-attack flag")
	}

	var last *Client
	for i := 0; i < config.RaidGuardCorrIPIDs; i++ {
		last = wireTestClient(t, "raider-"+string(rune('a'+i)), 0)
		raidGuardOnOOC(last, "raider", line)
	}
	if rs := last.raidGuard(); rs == nil || !rs.firedSignal(SigDupeAcrossIPIDs) {
		t.Errorf("%d distinct IPIDs saying the same line did not correlate", config.RaidGuardCorrIPIDs)
	}
	if !raidGuardUnderAttack() {
		t.Error("a corroborated fan-out did not set the under-attack flag")
	}
}
