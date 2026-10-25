// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package athena

// Tests for modcall scoring and the deterministic modcall action.
//
// The incident these exist for: six fresh IPIDs re-ran the handshake and then
// called a moderator the moment they joined. The guard saw only the handshake
// replay -- 45, a watch -- because pktModcall had no hook, both of its
// cooldowns returned before anything was recorded, and the reason text never
// reached the correlator. Every property below is one of the things that were
// missing, stated so it cannot silently regress.

import (
	"fmt"
	"testing"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// modcallSignalFired reports whether one of the two modcall signals is among
// the signals a noteModcall call just fired.
func modcallSignalFired(fired []SignalKind, k SignalKind) bool {
	for _, got := range fired {
		if got == k {
			return true
		}
	}
	return false
}

// TestModcallBurstScoreStaysTinyForHumans pins the curve. This is the promise
// that a real player calling for help is not punished for it: the first call is
// worth nothing, and four calls still sit below the strict tier's watch
// threshold.
func TestModcallBurstScoreStaysTinyForHumans(t *testing.T) {
	withRaidConfig(t)

	const weight = 45
	cases := []struct {
		attempts int
		want     int
	}{
		{0, 0},
		{1, 0},
		{2, 4},
		{3, 13},
		{4, 27},
		{5, 45}, // exactly the signal's weight: the threshold the operator named
		{6, 67},
		{10, 135}, // 3x cap
		{500, 135},
	}
	for _, c := range cases {
		if got := raidModcallBurstScore(c.attempts, 5, weight); got != c.want {
			t.Errorf("score after %d attempt(s) = %d, want %d", c.attempts, got, c.want)
		}
	}

	// The load-bearing one: a human who calls twice must not even be alerted on
	// at the tier a brand-new connection is judged at.
	strictWatch := config.RaidGuardScoreWatch * config.RaidGuardStrictScale / 100
	if got := raidModcallBurstScore(2, 5, weight); got >= strictWatch {
		t.Errorf("two modcalls score %d, at or above the strict-tier watch threshold %d", got, strictWatch)
	}
	if got := raidModcallBurstScore(4, 5, weight); got >= strictWatch {
		t.Errorf("four modcalls score %d, at or above the strict-tier watch threshold %d", got, strictWatch)
	}
}

// TestModcallSpamFiresAtThreshold checks the signal is declared exactly at the
// configured attempt budget, exactly once, and that the curve is not charged
// twice for it.
func TestModcallSpamFiresAtThreshold(t *testing.T) {
	withRaidConfig(t)

	rs := newRaidState()
	var firedAt []int
	for n := 1; n <= 8; n++ {
		fired, _ := rs.noteModcall(modcallEvidence{Attempts: n, SinceJoin: time.Hour, Spoke: true})
		if modcallSignalFired(fired, SigModcallSpam) {
			firedAt = append(firedAt, n)
		}
	}
	if len(firedAt) != 1 || firedAt[0] != 5 {
		t.Fatalf("SigModcallSpam fired at attempts %v, want exactly [5]", firedAt)
	}

	score, signals, _ := rs.snapshot()
	want := raidModcallBurstScore(8, 5, raidSignalWeight[SigModcallSpam])
	if score != want {
		t.Errorf("score after 8 attempts = %d, want %d (the curve's total, charged once)", score, want)
	}
	found := false
	for _, s := range signals {
		if s == raidSignalName[SigModcallSpam] {
			found = true
		}
	}
	if !found {
		t.Errorf("signals %v do not name the modcall spam signal", signals)
	}
}

// TestModcallOnJoinFiresWithinWindowOnly pins the timing claim the nuke rests
// on: only a call inside raid_guard_modcall_join_ms is scored as a join call.
func TestModcallOnJoinFiresWithinWindowOnly(t *testing.T) {
	withRaidConfig(t)

	cases := []struct {
		since time.Duration
		want  bool
	}{
		{200 * time.Millisecond, true},
		{2 * time.Second, true},
		{10 * time.Second, false},
		{time.Hour, false},
	}
	for _, c := range cases {
		rs := newRaidState()
		fired, _ := rs.noteModcall(modcallEvidence{Attempts: 1, SinceJoin: c.since, Spoke: true})
		if got := modcallSignalFired(fired, SigModcallOnJoin); got != c.want {
			t.Errorf("modcall %v after joining: SigModcallOnJoin = %v, want %v", c.since, got, c.want)
		}
	}
}

// TestModcallNukeLadder walks all three deterministic rules and the cases they
// must refuse: a human who simply calls again later, and anybody who is not
// brand new.
func TestModcallNukeLadder(t *testing.T) {
	withRaidConfig(t)

	cases := []struct {
		name string
		ev   modcallEvidence
		want bool
	}{
		{"A: first call inside the join window", modcallEvidence{
			Attempts: 1, SinceJoin: time.Second, BrandNew: true, Spoke: true,
		}, true},
		{"B: second call inside the dwell", modcallEvidence{
			Attempts: 2, SinceJoin: 30 * time.Second, BrandNew: true, Spoke: true,
		}, true},
		{"C: second call later with a replayed handshake", modcallEvidence{
			Attempts: 2, SinceJoin: 10 * time.Minute, BrandNew: true, Spoke: true, HandshakeReplay: true,
		}, true},
		{"C: second call later with an impossible character pick", modcallEvidence{
			Attempts: 2, SinceJoin: 10 * time.Minute, BrandNew: true, Spoke: true, FastCharPick: true,
		}, true},
		{"C: second call later, never spoke", modcallEvidence{
			Attempts: 3, SinceJoin: 10 * time.Minute, BrandNew: true,
		}, true},
		{"C: silent but only called twice", modcallEvidence{
			Attempts: 2, SinceJoin: 10 * time.Minute, BrandNew: true,
		}, false},
		{"C: second call later with a carbon-copy reason", modcallEvidence{
			Attempts: 2, SinceJoin: 10 * time.Minute, BrandNew: true, Spoke: true, EchoedReason: true,
		}, true},
		{"human: called, talked, called again ten minutes later", modcallEvidence{
			Attempts: 2, SinceJoin: 10 * time.Minute, BrandNew: true, Spoke: true,
		}, false},
		{"established player, called 200ms after joining", modcallEvidence{
			Attempts: 1, SinceJoin: 200 * time.Millisecond, BrandNew: false, Spoke: true,
		}, false},
	}
	for _, c := range cases {
		rs := newRaidState()
		if _, got := rs.noteModcall(c.ev); got != c.want {
			t.Errorf("%s: nuke = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestModcallWindowCountsAndExpires covers the long rolling window: attempts
// accumulate per IPID, expire, and are bounded.
func TestModcallWindowCountsAndExpires(t *testing.T) {
	window := time.Minute
	now := time.Now()
	m := newModcallWindow(4)

	if got := m.Observe("a", now, window); got != 1 {
		t.Fatalf("first attempt counted %d, want 1", got)
	}
	if got := m.Observe("a", now.Add(time.Second), window); got != 2 {
		t.Fatalf("second attempt counted %d, want 2", got)
	}
	if got := m.Count("a", now.Add(time.Second), window); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
	if got := m.Count("a", now.Add(2*time.Minute), window); got != 0 {
		t.Errorf("attempts older than the window still counted: %d", got)
	}

	m.Observe("a", now, window)
	m.Observe("b", now, window)
	m.Observe("c", now, window)
	if got := m.DistinctIPIDs(now, window); got != 3 {
		t.Errorf("DistinctIPIDs = %d, want 3", got)
	}

	m.Reset("a")
	if got := m.Count("a", now, window); got != 0 {
		t.Errorf("Reset left %d attempt(s) behind", got)
	}

	// The entry cap drops new identities rather than inventing a count for them.
	tiny := newModcallWindow(1)
	tiny.Observe("a", now, window)
	if got := tiny.Observe("b", now, window); got != 0 {
		t.Errorf("an IPID past the entry cap reported %d attempts, want 0", got)
	}
}

// TestModcallFanoutMarksUnderAttack checks the layer-2 half: several distinct
// IPIDs calling at once is raid-shaped, and marks the attack flag.
func TestModcallFanoutMarksUnderAttack(t *testing.T) {
	withWiring(t)
	now := time.Now()
	for i := 0; i < raidGuardModcallFanout(); i++ {
		raidObserveModcall(fmt.Sprintf("fanout-%d", i), now)
	}
	if !raidGuardUnderAttack() {
		t.Errorf("%d distinct IPIDs modcalling at once did not mark the server under attack", raidGuardModcallFanout())
	}
}

// TestModcallReasonFanOutFiresDupeAcrossIPIDs is the honest route to a ban for
// exactly the incident this feature came from: fresh connections sending the
// same canned reason. It has to fire the cross-IPID signal and mark the attack
// without any gate being bypassed.
func TestModcallReasonFanOutFiresDupeAcrossIPIDs(t *testing.T) {
	withWiring(t)

	reason := "there is a script spamming the courtroom with slurs right now"
	var last *Client
	for i := 0; i < 4; i++ {
		c := wireTestClient(t, fmt.Sprintf("modcall-fanout-%d", i), 0)
		// Well past the join window so the deterministic nuke stays out of this:
		// this test is about the scored correlation, not the ladder.
		c.SetConnectedAt(time.Now().Add(-10 * time.Minute))
		raidGuardOnModcall(c, reason)
		last = c
	}
	rs := last.raidGuard()
	if rs == nil {
		t.Fatal("no raid state on the last fan-out client")
	}
	if !rs.firedSignal(SigDupeAcrossIPIDs) {
		t.Error("four IPIDs sending the same modcall reason did not fire SigDupeAcrossIPIDs")
	}
	if !raidGuardUnderAttack() {
		t.Error("a modcall fan-out did not mark the server under attack")
	}
}

// TestModcallNukeHonoursCeiling checks the deterministic action still obeys the
// operator's ceiling -- it is the one thing that skips the ban gate, not the
// thing that skips what this server is allowed to do automatically.
func TestModcallNukeHonoursCeiling(t *testing.T) {
	withRaidConfig(t)

	cases := []struct {
		maxAction string
		want      Verdict
	}{
		{"ban", VerdictBan},
		{"kick", VerdictKick},
		{"silence", VerdictSilence},
		{"captcha", VerdictChallenge},
		{"watch", VerdictWatch},
		{"nonsense", VerdictSilence}, // unparseable falls back to the shipped default
	}
	for _, c := range cases {
		config.RaidGuardMaxAction = c.maxAction
		if got := raidGuardModcallNukeVerdict(); got != c.want {
			t.Errorf("max_action %q: nuke resolves to %v, want %v", c.maxAction, got, c.want)
		}
	}
}

// TestModcallNukeNeverTouchesModerators is the same exemption every other
// action in this file honours. A moderator's modcall is staff doing their job.
func TestModcallNukeNeverTouchesModerators(t *testing.T) {
	withWiring(t)
	config.RaidGuardMaxAction = "ban"

	c := wireTestClient(t, "mod-nuke", permissions.PermissionField["BAN"])
	c.SetConnectedAt(time.Now().Add(-100 * time.Millisecond))
	if raidGuardOnModcall(c, "test") {
		t.Error("raidGuardOnModcall acted on a moderator")
	}
	if c.captchaRestricted.Load() || c.awaitingCaptcha.Load() {
		t.Error("a moderator was quarantined by the modcall path")
	}
}

// TestModcallRefusedAttemptStillScores is the ordering property pktModcall
// depends on: the guard hook runs ABOVE both cooldowns, so a script whose every
// attempt is refused still accumulates evidence instead of leaving no trace.
func TestModcallRefusedAttemptStillScores(t *testing.T) {
	withWiring(t)
	config.NewIPIDModcallCooldown = 60
	config.ModcallCooldown = 0

	ipid := "modcall-refused"
	ipFirstSeenTracker.mu.Lock()
	ipFirstSeenTracker.times[ipid] = time.Now()
	ipFirstSeenTracker.mu.Unlock()
	t.Cleanup(func() {
		ipFirstSeenTracker.mu.Lock()
		delete(ipFirstSeenTracker.times, ipid)
		ipFirstSeenTracker.mu.Unlock()
	})

	c := &Client{conn: &capturingConn{}, uid: 7, ipid: ipid}
	c.SetConnectedAt(time.Now().Add(-10 * time.Minute))
	rs := c.raidGuard()
	rs.mu.Lock()
	rs.msgCount = 1 // a real player who has spoken; keeps the nuke ladder out of this
	rs.mu.Unlock()

	pkt := &aolib.Packet{Body: []string{"help me", "-1"}}
	for i := 0; i < raidGuardModcallMinAttempts(); i++ {
		pktModcall(c, pkt)
	}

	if !rs.firedSignal(SigModcallSpam) {
		score, _, _ := rs.snapshot()
		t.Fatalf("five refused modcalls did not fire the spam signal (score %d)", score)
	}
	// And the refusal really did refuse: nothing was allowed through.
	if limited, _ := checkNewIPIDModcallCooldown(ipid); !limited {
		t.Error("the new-IPID cooldown did not refuse the attempts; this test proves nothing")
	}
}

// TestModcallDisabledSuppressesDelivery covers /modcall disable at the packet
// handler: no delivery, the attempt is counted, and the sender's own cooldown is
// not burned by an attempt the server refused on its behalf.
func TestModcallDisabledSuppressesDelivery(t *testing.T) {
	withWiring(t)
	config.ModcallCooldown = 60

	oldDisabled := modcallsDisabled.Load()
	oldSuppressed := modcallsSuppressed.Load()
	t.Cleanup(func() {
		modcallsDisabled.Store(oldDisabled)
		modcallsSuppressed.Store(oldSuppressed)
	})

	conn := &capturingConn{}
	c := &Client{conn: conn, uid: 9, ipid: "modcall-disabled"}
	c.SetConnectedAt(time.Now().Add(-10 * time.Minute))

	modcallsDisabled.Store(true)
	modcallsSuppressed.Store(0)

	pktModcall(c, &aolib.Packet{Body: []string{"someone is griefing", "-1"}})

	if got := modcallsSuppressed.Load(); got != 1 {
		t.Errorf("suppressed counter = %d, want 1", got)
	}
	if got := conn.snapshot(); len(got) == 0 {
		t.Error("the sender was not told modcalls are disabled")
	}
	if limited, _ := checkIPModcallCooldown("modcall-disabled"); limited {
		t.Error("a suppressed attempt consumed the sender's modcall cooldown")
	}
}
