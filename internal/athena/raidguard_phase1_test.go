package athena

import (
	"testing"
	"time"
)

// resetRaidLedgerForTest clears the package-level repeat ledger between tests.
func resetRaidLedgerForTest() {
	raidSystem.ledger.mu.Lock()
	raidSystem.ledger.entries = make(map[string]*raidLedgerEntry)
	raidSystem.ledger.mu.Unlock()
}

// A reconnect after the previous socket is gone is a reroll; a second tab that
// is still connected is not. The distinction is what keeps a household behind
// one NAT from being treated as a reconnect loop.
func TestRaidLedgerCountsSequentialRerollsNotConcurrentTabs(t *testing.T) {
	withRaidConfig(t)
	resetRaidLedgerForTest()
	t.Cleanup(resetRaidLedgerForTest)

	client := &Client{ipid: "test-ipid-reroll", hdid: "test-hdid-reroll"}
	base := time.Unix(1_700_000_000, 0).UTC()

	if got := raidLedgerNoteJoin(client, base); got != 0 {
		t.Fatalf("first join rerolls = %d, want 0", got)
	}
	if got := raidLedgerNoteJoin(client, base.Add(time.Second)); got != 0 {
		t.Fatalf("concurrent second tab rerolls = %d, want 0", got)
	}
	raidLedgerNoteDisconnect(client, base.Add(2*time.Second))
	raidLedgerNoteDisconnect(client, base.Add(2*time.Second))
	if got := raidLedgerNoteJoin(client, base.Add(3*time.Second)); got != 1 {
		t.Fatalf("first reconnect rerolls = %d, want 1", got)
	}
	raidLedgerNoteDisconnect(client, base.Add(4*time.Second))
	if got := raidLedgerNoteJoin(client, base.Add(5*time.Second)); got != 2 {
		t.Fatalf("second reconnect rerolls = %d, want 2", got)
	}
	raidLedgerNoteDisconnect(client, base.Add(6*time.Second))
	if got := raidLedgerNoteJoin(client, base.Add(7*time.Second)); got != 3 {
		t.Fatalf("third reconnect rerolls = %d, want 3", got)
	}
}

// A post-join askchaa is protocol-impossible and counts double.
func TestRaidLedgerCountsProtocolReplay(t *testing.T) {
	withRaidConfig(t)
	resetRaidLedgerForTest()
	t.Cleanup(resetRaidLedgerForTest)

	client := &Client{ipid: "test-ipid-replay", hdid: "test-hdid-replay"}
	base := time.Unix(1_700_000_100, 0).UTC()

	if got := raidLedgerNoteJoin(client, base); got != 0 {
		t.Fatalf("initial join rerolls = %d, want 0", got)
	}
	if got := raidLedgerNoteReplay(client, base.Add(time.Second)); got != 2 {
		t.Fatalf("first replay count = %d, want 2", got)
	}
	if got := raidLedgerNoteReplay(client, base.Add(2*time.Second)); got != 4 {
		t.Fatalf("second replay count = %d, want 4", got)
	}
}

// Three fast CCs make one churn flag; three flags reach the repeat threshold.
func TestRaidLedgerRapidCharacterChanges(t *testing.T) {
	withRaidConfig(t)
	resetRaidLedgerForTest()
	t.Cleanup(resetRaidLedgerForTest)

	client := &Client{ipid: "test-ipid-churn", hdid: "test-hdid-churn"}
	now := time.Unix(1_700_000_200, 0).UTC()

	if got := raidLedgerNoteCharChange(client, now); got != 0 {
		t.Fatalf("first CC flags = %d, want 0", got)
	}
	var flags int
	for cluster := 0; cluster < 3; cluster++ {
		for i := 0; i < raidGuardCCRapidLimit; i++ {
			now = now.Add(100 * time.Millisecond)
			flags = raidLedgerNoteCharChange(client, now)
		}
	}
	if flags != raidGuardCCFlagLimit() {
		t.Fatalf("rapid-CC flags = %d, want %d", flags, raidGuardCCFlagLimit())
	}
}

// The guard challenge must still be issued when join_captcha is off. Before
// this change VerdictChallenge degraded to VerdictWatch, which is the score-70
// connection that sat there forever.
func TestGuardChallengeIsNotDegradedToWatch(t *testing.T) {
	withRaidConfig(t)
	if config != nil {
		config.JoinCaptcha = false
	}
	oldLookup := playtimeLookup
	playtimeLookup = func(string) (int64, error) { return 0, nil }
	t.Cleanup(func() { playtimeLookup = oldLookup })

	client := &Client{
		ipid: "test-ipid-challenge",
		done: make(chan struct{}),
	}
	t.Cleanup(func() { close(client.done) })

	state := newRaidState()
	raidSystem.enforce(client, state, VerdictChallenge, "phase-1 test")

	if !client.guardChallenge.Load() {
		t.Fatal("guard challenge flag not set; VerdictChallenge was degraded to a no-op")
	}
	if !client.awaitingCaptcha.Load() {
		t.Fatal("guard challenge did not put the client into the awaiting state")
	}
}

// Heat must expire: a single false flag should not stay at full weight for the
// rest of the session. This drives the clock explicitly rather than sleeping.
func TestRaidHeatDecaysByHalfLife(t *testing.T) {
	withRaidConfig(t)
	base := time.Unix(1_700_000_300, 0).UTC()

	state := newRaidState()
	state.mu.Lock()
	state.score = 80
	state.scoreAt = base
	state.mu.Unlock()

	got, _, _ := state.snapshotAt(base.Add(raidGuardHeatHalfLife()))
	if got != 40 {
		t.Fatalf("score after one half-life = %d, want 40", got)
	}
	got, _, _ = state.snapshotAt(base.Add(2 * raidGuardHeatHalfLife()))
	if got != 20 {
		t.Fatalf("score after two half-lives = %d, want 20", got)
	}
}

func TestRaidHeatDecayDisabledByZeroHalfLife(t *testing.T) {
	withRaidConfig(t)
	if config != nil {
		config.RaidGuardHeatHalfLife = 0
	}
	base := time.Unix(1_700_000_400, 0).UTC()

	state := newRaidState()
	state.mu.Lock()
	state.score = 70
	state.scoreAt = base
	state.mu.Unlock()

	got, _, _ := state.snapshotAt(base.Add(24 * time.Hour))
	if got != 70 {
		t.Fatalf("score with decay disabled = %d, want 70", got)
	}
}

// A script can avoid the IC/OOC captcha gate by only changing characters or
// replaying the handshake. Those actions must also consume the guard challenge
// budget and terminate the connection.
func TestGuardChallengeCountsContinuedCharacterActivity(t *testing.T) {
	withRaidConfig(t)
	client := &Client{
		ipid: "test-ipid-guard-activity",
		char: -1,
		done: make(chan struct{}),
		conn: &capturingConn{},
	}
	client.guardChallenge.Store(true)
	client.awaitingCaptcha.Store(true)
	client.captchaStrikes.Store(raidGuardChallengeStrikes() - 1)

	if !client.guardChallengeActivity() {
		t.Fatal("continued activity did not exhaust the guard challenge")
	}
	if !client.closed.Load() {
		t.Fatal("continued activity left the connection alive")
	}
}

// Protocol evidence must outlive weak content shape: at five minutes a
// handshake-replay bucket has lost one third of its life while shouted spam has
// lost a full half-life.
func TestRaidHeatUsesPerSignalHalfLives(t *testing.T) {
	withRaidConfig(t)
	base := time.Unix(1_700_000_500, 0).UTC()

	state := newRaidState()
	state.mu.Lock()
	state.heat[SigHandshakeReplay] = 45
	state.heatAt[SigHandshakeReplay] = base
	state.heat[SigShoutySpam] = 10
	state.heatAt[SigShoutySpam] = base
	state.mu.Unlock()

	got, _, _ := state.snapshotAt(base.Add(5 * time.Minute))
	if got < 40 || got > 42 {
		t.Fatalf("heat at 5m = %d, want about 41 (slow protocol + fast content decay)", got)
	}

	got, _, _ = state.snapshotAt(base.Add(15 * time.Minute))
	if got < 23 || got > 25 {
		t.Fatalf("heat at 15m = %d, want about 24", got)
	}
}

// A stable session clears the reroll tally: a real player who reconnects after
// several minutes of play must not be treated like a fast reconnect script.
func TestRaidLedgerStableSessionResetsRerolls(t *testing.T) {
	withRaidConfig(t)
	resetRaidLedgerForTest()
	t.Cleanup(resetRaidLedgerForTest)

	client := &Client{ipid: "test-ipid-stable", hdid: "test-hdid-stable"}
	base := time.Unix(1_700_000_600, 0).UTC()

	if got := raidLedgerNoteJoin(client, base); got != 0 {
		t.Fatalf("initial join rerolls = %d, want 0", got)
	}
	raidLedgerNoteDisconnect(client, base.Add(time.Second))
	if got := raidLedgerNoteJoin(client, base.Add(2*time.Second)); got != 1 {
		t.Fatalf("quick reconnect rerolls = %d, want 1", got)
	}

	// This session lasts almost seven minutes, well past the 300s reset.
	raidLedgerNoteDisconnect(client, base.Add(400*time.Second))
	if got := raidLedgerNoteJoin(client, base.Add(401*time.Second)); got != 0 {
		t.Fatalf("rerolls after a stable session = %d, want 0", got)
	}
}
