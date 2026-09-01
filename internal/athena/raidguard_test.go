package athena

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// withRaidConfig installs the shipped defaults for the duration of a test.
func withRaidConfig(t *testing.T) {
	t.Helper()
	old := config
	t.Cleanup(func() { config = old })
	config = settings.DefaultConfig()
}

// TestNoSingleSignalCanBan is the guard's headline safety property. The stated
// goal for this system is that it must never ban a legitimate player, and the
// first line of defence is arithmetic: the ban threshold is set above any one
// signal's weight, so no single behaviour -- however damning it looks in
// isolation -- can get a connection banned on its own.
func TestNoSingleSignalCanBan(t *testing.T) {
	withRaidConfig(t)
	banAt := config.RaidGuardScoreBan
	for k := SignalKind(0); k < numRaidSignals; k++ {
		w := raidSignalWeight[k]
		if w >= banAt {
			t.Errorf("signal %q weighs %d, at or above the ban threshold %d: it could ban a connection on its own",
				raidSignalName[k], w, banAt)
		}
		// A single signal must not even reach a kick.
		if v := verdictForTier(w, raidGuardScaleBase); v >= VerdictKick {
			t.Errorf("signal %q alone yields verdict %v; a lone signal must never exceed silence", raidSignalName[k], v)
		}
	}
}

// TestBanNeedsThreeSignals checks that reaching a ban requires several
// independent observations, not one behaviour repeated.
func TestBanNeedsThreeSignals(t *testing.T) {
	withRaidConfig(t)
	// The two heaviest signals together must still not be enough to ban.
	heaviest, second := 0, 0
	for k := SignalKind(0); k < numRaidSignals; k++ {
		w := raidSignalWeight[k]
		if w > heaviest {
			heaviest, second = w, heaviest
		} else if w > second {
			second = w
		}
	}
	// Must hold at every tier, including the strict one a brand-new connection
	// is judged at -- that is the tier a raid actually arrives in, and the tier
	// where the margin is thinnest.
	for _, tier := range []struct {
		name  string
		scale int
	}{
		{"strict", config.RaidGuardStrictScale},
		{"baseline", raidGuardScaleBase},
		{"lenient", config.RaidGuardLenientScale},
	} {
		if v := verdictForTier(heaviest+second, tier.scale); v >= VerdictBan {
			t.Errorf("%s tier: the two heaviest signals (%d+%d) reach %v; a ban must need at least three",
				tier.name, heaviest, second, v)
		}
		t.Logf("%s tier (%d%%): two heaviest signals (%d) -> %v",
			tier.name, tier.scale, heaviest+second, verdictForTier(heaviest+second, tier.scale))
	}
}

// TestRaidBanAllowed pins the invariant that a ban requires corroboration from
// other IPIDs *and* a concurrently detected raid. A lone player, whatever they
// do, satisfies neither.
func TestRaidBanAllowed(t *testing.T) {
	cases := []struct {
		correlated, underAttack, want bool
	}{
		{false, false, false},
		{true, false, false}, // one odd player during calm traffic
		{false, true, false}, // a raid is on, but this connection is not part of it
		{true, true, true},   // corroborated participant in an active raid
	}
	for _, c := range cases {
		if got := raidBanAllowed(c.correlated, c.underAttack); got != c.want {
			t.Errorf("raidBanAllowed(%v, %v) = %v, want %v", c.correlated, c.underAttack, got, c.want)
		}
	}
}

// TestPlaytimeTiersAreMonotonic checks the tier ladder does what it claims:
// more history must never make the guard treat you worse.
func TestPlaytimeTiersAreMonotonic(t *testing.T) {
	withRaidConfig(t)
	score := config.RaidGuardScoreKick
	strict := verdictForTier(score, config.RaidGuardStrictScale)
	base := verdictForTier(score, raidGuardScaleBase)
	lenient := verdictForTier(score, config.RaidGuardLenientScale)
	if strict < base {
		t.Errorf("a brand-new connection (%v) is treated more leniently than baseline (%v)", strict, base)
	}
	if lenient > base {
		t.Errorf("an established player (%v) is treated more harshly than baseline (%v)", lenient, base)
	}
	t.Logf("score %d -> strict(%d%%)=%v baseline(100%%)=%v lenient(%d%%)=%v",
		score, config.RaidGuardStrictScale, strict, base, config.RaidGuardLenientScale, lenient)
}

// TestLenientTierNeedsMoreEvidence checks that a score which would act on a new
// connection does nothing to a player with hours behind them.
func TestLenientTierNeedsMoreEvidence(t *testing.T) {
	withRaidConfig(t)
	score := config.RaidGuardScoreSilence
	if v := verdictForTier(score, raidGuardScaleBase); v != VerdictSilence {
		t.Fatalf("baseline verdict at the silence threshold = %v, want silence", v)
	}
	if v := verdictForTier(score, config.RaidGuardLenientScale); v >= VerdictSilence {
		t.Errorf("an established player hit %v at a score that only just silences a new connection", v)
	}
}

// TestObjectionNeedsSustainedUse checks the headline signal cannot fire on a
// player who shouts once. AO2 is a courtroom game; a single "Objection!" is the
// point of it, and only sustained use of the shout on essentially every message
// separates a raider from a real trial.
func TestObjectionNeedsSustainedUse(t *testing.T) {
	withRaidConfig(t)

	// A real trial: one shout, then ordinary dialogue.
	rs := newRaidState()
	for i, obj := range []int{2, 0, 0, 0, 0, 0, 0, 0} {
		rs.observe(Observation{IsIC: true, Text: "some ordinary courtroom dialogue", Objection: obj, SinceCharPick: time.Minute, Now: time.Now().Add(time.Duration(i) * time.Second)})
	}
	if rs.firedSignal(SigObjectionSpam) {
		t.Error("objection signal fired on a player who shouted once in eight messages")
	}

	// A raider: every message a shout.
	rs2 := newRaidState()
	for i := 0; i < 5; i++ {
		rs2.observe(Observation{IsIC: true, Text: "GET RAPED", Objection: 1, SinceCharPick: time.Minute, Now: time.Now()})
	}
	if !rs2.firedSignal(SigObjectionSpam) {
		t.Error("objection signal did not fire on five consecutive shouts")
	}
}

// TestShoutySpamNeedsAllThreeConditions checks the weakest signal is
// conservative: dramatic capslock is ordinary in this game, so shouting alone
// must not be enough.
func TestShoutySpamNeedsAllThreeConditions(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"short shout", "OBJECTION!", false},
		{"long caps, no repeat", "YOUR HONOR I MUST INSIST ON PRESENTING THIS DECISIVE EVIDENCE NOW", false},
		{"long mixed case with repeat", "I really think the witness is lying, the witness has lied before", false},
		{"long caps with repeat", "RAPED RAPED RAPED SHUTDOWN CLOVERR RISK RAPED RAPED", true},
	}
	for _, c := range cases {
		if got := isShoutySpam(c.text); got != c.want {
			t.Errorf("%s: isShoutySpam(%q) = %v, want %v", c.name, c.text, got, c.want)
		}
	}
}

// TestCorrelationIgnoresShortLines pins the floor that keeps ordinary players
// from correlating with each other. Two people independently typing the same
// short command is the single most likely false positive this system has, and
// it was measured at 57% of a clean capture before the floor was added.
func TestCorrelationIgnoresShortLines(t *testing.T) {
	for _, s := range []string{"/gas", "/cm", "hi", "lol", "wb"} {
		if fps := raidFingerprints(s, 15); len(fps) != 0 {
			t.Errorf("short line %q produced fingerprints %v; it must be ignored", s, fps)
		}
	}
	long := "all of you vibe coders go to hell you are no match for the raven"
	if fps := raidFingerprints(long, 15); len(fps) == 0 {
		t.Errorf("a substantial line produced no fingerprints: %q", long)
	}
}

// TestCorrelationMatchesEditedRepeats checks shingling catches the variation
// raiders actually use -- the same line with a command prefix or an extra slur
// bolted on -- which exact whole-message matching misses.
func TestCorrelationMatchesEditedRepeats(t *testing.T) {
	base := "sD mastyra Mint SyntaxNyah Salanto all of you vibe coders go to hell"
	variants := []string{
		"/g sD mastyra Mint SyntaxNyah Salanto all of you vibe coders go to hell",
		"sD mastyra Mint SyntaxNyah Salanto all of you vibe coders go to hell NIGGERS",
	}
	want := raidFingerprints(base, 15)
	if len(want) == 0 {
		t.Fatal("base line produced no fingerprints")
	}
	set := make(map[uint64]struct{}, len(want))
	for _, fp := range want {
		set[fp] = struct{}{}
	}
	for _, v := range variants {
		shared := 0
		for _, fp := range raidFingerprints(v, 15) {
			if _, ok := set[fp]; ok {
				shared++
			}
		}
		if shared == 0 {
			t.Errorf("edited repeat shared no fingerprint with the original: %q", v)
		}
	}
}

// TestCorrelationWindowIsBounded checks a raid cannot exhaust memory through the
// very structure built to detect it.
func TestCorrelationWindowIsBounded(t *testing.T) {
	w := NewCorrelationWindow(10*time.Second, 256)
	now := time.Now()
	for i := 0; i < 20000; i++ {
		w.Observe(uint64(i), fmt.Sprintf("ipid-%d", i%500), now)
	}
	if n := w.Len(); n > 512 {
		t.Errorf("correlation window holds %d entries after 20000 unique fingerprints; it must stay bounded", n)
	}
}

// TestCorrelationNeedsDistinctIPIDs checks one connection repeating itself never
// looks like coordination, however many times it says the same thing.
func TestCorrelationNeedsDistinctIPIDs(t *testing.T) {
	w := NewCorrelationWindow(10*time.Second, 256)
	now := time.Now()
	for i := 0; i < 50; i++ {
		if got := w.Observe(1234, "lonely-ipid", now); got != 1 {
			t.Fatalf("same IPID repeating itself counted as %d distinct IPIDs", got)
		}
	}
}

// TestVerdictNeverDowngrades checks the engine only ever escalates, so a
// connection cannot talk its way back down after being acted on.
func TestVerdictNeverDowngrades(t *testing.T) {
	rs := newRaidState()
	if !rs.escalate(VerdictSilence) {
		t.Fatal("first escalation to silence was refused")
	}
	if rs.escalate(VerdictWatch) {
		t.Error("engine downgraded from silence to watch")
	}
	if !rs.escalate(VerdictKick) {
		t.Error("engine refused a genuine escalation from silence to kick")
	}
}

// TestSignalsFireOnce checks a repeated behaviour cannot inflate a score without
// bound, which would otherwise let a chatty connection reach a ban by doing one
// unusual thing many times.
func TestSignalsFireOnce(t *testing.T) {
	withRaidConfig(t)
	rs := newRaidState()
	for i := 0; i < 200; i++ {
		rs.observe(Observation{IsIC: true, Text: "RAPED RAPED RAPED SHUTDOWN CLOVERR RISK RAPED RAPED", Objection: 1, SinceCharPick: time.Minute, Now: time.Now()})
	}
	score, signals, _ := rs.snapshot()
	max := 0
	for k := SignalKind(0); k < numRaidSignals; k++ {
		max += raidSignalWeight[k]
	}
	if score > max {
		t.Errorf("score %d exceeds the sum of all signal weights %d; a signal is firing more than once", score, max)
	}
	t.Logf("200 identical messages -> score %d, signals %v", score, signals)
}

// TestTwoSignalsCannotDisconnect checks that no pair of signals, at any playtime
// tier, can take an action the player cannot undo themselves.
//
// The weights are calibrated on two captures, and the handshake-ordering signal
// in particular is an empirical observation that was never confirmed against
// real client source. This test is what makes that acceptable: even if that
// weight is wrong, two signals can reach a quarantine the player lifts by
// answering the pending captcha, and no further.
func TestTwoSignalsCannotDisconnect(t *testing.T) {
	withRaidConfig(t)
	for a := SignalKind(0); a < numRaidSignals; a++ {
		for b := a + 1; b < numRaidSignals; b++ {
			rs := newRaidState()
			rs.markFired(a)
			rs.markFired(b)
			if n := rs.firedCount(); n != 2 {
				t.Fatalf("expected 2 fired signals, got %d", n)
			}
			for _, scale := range []int{config.RaidGuardStrictScale, raidGuardScaleBase, config.RaidGuardLenientScale} {
				v := clampDisconnect(verdictForTier(rs.score, scale), rs.firedCount())
				if v >= VerdictKick {
					t.Errorf("signals %q+%q at scale %d reach %v with only 2 signals fired",
						raidSignalName[a], raidSignalName[b], scale, v)
				}
			}
		}
	}
}

// TestAutoLockdownIsOptIn checks the guard's one server-wide action stays off
// unless an operator asks for it. It affects players who have done nothing but
// try to connect at a bad moment, so it must never engage by default.
func TestAutoLockdownIsOptIn(t *testing.T) {
	withRaidConfig(t)
	if config.RaidGuardAutoLockdown {
		t.Error("raid_guard_auto_lockdown defaults to on; it must be opt-in")
	}

	oldClients := clients
	t.Cleanup(func() { clients = oldClients; serverLockdown.Store(false); resetRaidGuardState() })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	serverLockdown.Store(false)
	markRaidAttack(time.Now())
	if serverLockdown.Load() {
		t.Error("a detected raid engaged lockdown with raid_guard_auto_lockdown off")
	}
	if !raidGuardUnderAttack() {
		t.Error("markRaidAttack did not set the under-attack flag")
	}

	config.RaidGuardAutoLockdown = true
	resetRaidGuardState()
	markRaidAttack(time.Now())
	if !serverLockdown.Load() {
		t.Error("a detected raid did not engage lockdown with raid_guard_auto_lockdown on")
	}
}

// TestPacketFloodAutobanIsHonoured pins the flag that gates the raw-packet-flood
// ban. It was documented and settable for a long time while nothing read it, so
// an operator who turned it off still got 173-year bans out of that path. A
// config switch that silently does nothing is worse than no switch, because it
// is trusted.
func TestPacketFloodAutobanIsHonoured(t *testing.T) {
	withRaidConfig(t)
	src, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("reading client.go: %v", err)
	}
	body := string(src)
	i := strings.Index(body, "if client.CheckRawPacketRateLimit() {")
	if i < 0 {
		t.Fatal("could not find the raw-packet-flood branch in client.go")
	}
	block := body[i:]
	if j := strings.Index(block, "\n\t\tvar pkt"); j > 0 {
		block = block[:j]
	}
	if !strings.Contains(block, "config.PacketFloodAutoban") {
		t.Error("the raw-packet-flood branch does not read config.PacketFloodAutoban; " +
			"setting packet_flood_autoban = false would silently still ban")
	}
	if !strings.Contains(block, "autoBanPacketFlooder") {
		t.Error("the raw-packet-flood branch no longer bans at all; the flag should gate the ban, not remove it")
	}
}

// TestChallengeRungRespectsCaptchaToggle checks that an operator who turns the
// join captcha off does not keep getting captchas from the raid guard. The
// challenge rung borrows the captcha's machinery, so with that switched off it
// degrades to an alert -- and specifically not to silence, which without a
// question the player can answer is harsher than the rung above it.
func TestChallengeRungRespectsCaptchaToggle(t *testing.T) {
	withRaidConfig(t)
	oldClients := clients
	t.Cleanup(func() { clients = oldClients })
	clients = &ClientList{
		list:       make(map[*Client]struct{}),
		uidIndex:   make(map[int]*Client),
		ipidCounts: make(map[string]int),
	}

	config.JoinCaptcha = false
	config.RaidGuardMaxAction = "ban"

	c := &Client{conn: &testConn{}, uid: 1, ipid: "captcha-off-ipid"}
	rs := newRaidState()
	rs.mu.Lock()
	rs.score = config.RaidGuardScoreChallenge
	rs.mu.Unlock()

	raidGuardEnforce(c, rs, VerdictChallenge, "test")
	if _, _, acted := rs.snapshot(); acted != VerdictWatch {
		t.Errorf("with the captcha off, a challenge verdict acted as %v; want watch", acted)
	}
	if c.awaitingCaptcha.Load() {
		t.Error("the guard put a client into the captcha flow on a server with join_captcha = false")
	}
	if c.captchaRestricted.Load() {
		t.Error("the guard silenced a client that should only have been alerted on")
	}
}

// TestBanStillWorksWithCaptchaOff checks the thing an operator actually cares
// about when turning the captcha off: the autoban half is untouched by it.
func TestBanStillWorksWithCaptchaOff(t *testing.T) {
	withRaidConfig(t)
	config.JoinCaptcha = false
	if v := verdictForTier(config.RaidGuardScoreBan, raidGuardScaleBase); v != VerdictBan {
		t.Errorf("ban verdict = %v with the captcha off; the two features must be independent", v)
	}
	if v := verdictForTier(config.RaidGuardScoreKick, raidGuardScaleBase); v != VerdictKick {
		t.Errorf("kick verdict = %v with the captcha off", v)
	}
}

// TestCorrelationWindowExpiresFromFirstSighting pins the tumbling-window
// semantics corrEntry documents. Under the previous last-touch expiry an entry
// stayed alive as long as anything kept touching it, so a line said by a
// different player every few seconds accumulated IPIDs without bound and would
// eventually cross any threshold -- turning an area catchphrase into
// "coordinated". The tally must instead describe one window of wall clock and
// nothing longer, which is the claim the signal actually makes.
func TestCorrelationWindowExpiresFromFirstSighting(t *testing.T) {
	w := NewCorrelationWindow(10*time.Second, 4096)
	const fp = 0xC0FFEE
	start := time.Now()

	// Four different IPIDs say the same line, each 4 seconds after the last.
	// Every gap is inside the window, so last-touch expiry would keep one entry
	// alive across all of them and report 4.
	counts := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		counts = append(counts, w.Observe(fp, fmt.Sprintf("ipid-%d", i), start.Add(time.Duration(i)*4*time.Second)))
	}
	t.Logf("one line said by four IPIDs at 4s intervals -> counts %v", counts)

	// The first two land inside the first window and tally together; by +8s the
	// entry is not yet 10s old either, so three is expected. The fourth, at
	// +12s, is past the window measured from the FIRST sighting, so the tally
	// restarts at 1 rather than reaching 4.
	if got := counts[len(counts)-1]; got != 1 {
		t.Errorf("after the window elapsed from the first sighting the tally was %d, want 1 -- entries are "+
			"expiring from last touch, so a slowly-recurring line can accumulate IPIDs forever", got)
	}
}

// TestEchoBreadthCountsDistinctLines pins the difference between depth and
// breadth. Two people quoting one line at each other is depth, and ordinary; it
// must read as breadth 1 no matter how long they keep it up. Only several
// DIFFERENT lines echoing at once is a shape a pair of players cannot produce.
func TestEchoBreadthCountsDistinctLines(t *testing.T) {
	e := newEchoWindow(30 * time.Second)
	now := time.Now()

	for i := 0; i < 20; i++ {
		if got := e.Credit(0xAAAA, now.Add(time.Duration(i)*time.Second)); got != 1 {
			t.Fatalf("one line echoed %d times reported breadth %d, want 1 -- breadth must count distinct "+
				"lines, or two players quoting each other look like a raid", i+1, got)
		}
	}
	if got := e.Credit(0xBBBB, now.Add(20*time.Second)); got != 2 {
		t.Errorf("a second distinct line reported breadth %d, want 2", got)
	}
	// And breadth decays: nothing said for a whole window means nothing echoing.
	if got := e.Breadth(now.Add(2 * time.Minute)); got != 0 {
		t.Errorf("breadth %d a full two minutes later, want 0 -- echoes must age out", got)
	}
}

// TestWeakCorrelationNeverArmsTheBanGate is the invariant that makes a weak
// threshold of two IPIDs safe. The echo signal is evidence and carries score,
// but raidBanAllowed must remain reachable only through the strong signal, so no
// number of people quoting each other can arm a ban between them.
func TestWeakCorrelationNeverArmsTheBanGate(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()
	config = settings.DefaultConfig()

	resetRaidGuardState()
	t.Cleanup(resetRaidGuardState)

	rs := newRaidState()
	const line = "this is a long enough line to be fingerprinted at all"
	now := time.Now()

	// Two IPIDs, back and forth, many times: the strongest form of "quoting
	// each other" two people can manage.
	for i := 0; i < 25; i++ {
		raidGuardCorrelate(rs, "ipid-a", line, now.Add(time.Duration(i)*time.Second))
		raidGuardCorrelate(rs, "ipid-b", line, now.Add(time.Duration(i)*time.Second))
	}

	if !rs.hasFired(SigEchoedAcrossIPIDs) {
		t.Errorf("two IPIDs repeating one line never fired the echo signal; the weak threshold is not working")
	}
	if rs.hasFired(SigDupeAcrossIPIDs) {
		t.Errorf("two IPIDs repeating one line fired SigDupeAcrossIPIDs -- the strong threshold has collapsed " +
			"onto the weak one, and with it the ban gate")
	}
	if raidBanAllowed(rs.hasFired(SigDupeAcrossIPIDs), raidGuardUnderAttack()) {
		t.Errorf("raidBanAllowed is true after two players quoted each other; a ban must never be reachable " +
			"without corroboration a lone pair cannot manufacture")
	}
	// Breadth is the other half: one line, however loudly, is breadth 1.
	if b := raidGuardEchoBreadth(); b >= raidCorrBreadth() {
		t.Errorf("echo breadth reached %d (threshold %d) from a single repeated line; breadth must count "+
			"distinct lines", b, raidCorrBreadth())
	}
}

// TestCorrThresholdsClampSanely checks that a mistyped config degrades rather
// than inverting the design: a weak level above the strong one would make the
// cheaper signal outrank the one that gates bans.
func TestCorrThresholdsClampSanely(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = settings.DefaultConfig()
	config.RaidGuardCorrIPIDs = 3
	config.RaidGuardCorrIPIDsWeak = 9
	if _, weak, strong := raidCorrThresholds(); weak > strong {
		t.Errorf("weak=%d strong=%d: a weak level above the strong one was accepted", weak, strong)
	}

	config.RaidGuardCorrIPIDsWeak = 1
	if _, weak, _ := raidCorrThresholds(); weak < 2 {
		t.Errorf("weak=%d: one IPID saying its own line is not corroboration and must never be accepted "+
			"as the threshold", weak)
	}
}

// TestSlurSeverityMapsToSignals checks each word-severity tier feeds the one
// signal the design calls for, and only that one: a severe hit must not also
// register as a merely-flagged hit, and vice versa.
func TestSlurSeverityMapsToSignals(t *testing.T) {
	severe := newRaidState()
	if fired := severe.noteWordHit(SeveritySevere); len(fired) != 1 || fired[0] != SigSlurSevere {
		t.Errorf("noteWordHit(SeveritySevere) fired %v, want [SigSlurSevere]", fired)
	}
	if !severe.hasFired(SigSlurSevere) || severe.hasFired(SigSlurFlagged) {
		t.Error("a severe word-list hit must fire SigSlurSevere and only SigSlurSevere")
	}

	for _, sev := range []WordSeverity{SeverityDefault, SeverityWatch} {
		rs := newRaidState()
		if fired := rs.noteWordHit(sev); len(fired) != 1 || fired[0] != SigSlurFlagged {
			t.Errorf("noteWordHit(%v) fired %v, want [SigSlurFlagged]", sev, fired)
		}
		if !rs.hasFired(SigSlurFlagged) || rs.hasFired(SigSlurSevere) {
			t.Errorf("a %v word-list hit must fire SigSlurFlagged and only SigSlurFlagged", sev)
		}
	}
}

// TestSlurNukeFiresNothing checks a nuke-tier hit never reaches the raid
// guard's scoring engine at all. A nuke ends the connection deterministically
// through AutoMod's own path (word_severity.go); scoring something that is
// already being banned is pointless, and folding it in here would quietly
// give one signal the power to act alone -- exactly what this file's design
// forbids everywhere else.
func TestSlurNukeFiresNothing(t *testing.T) {
	rs := newRaidState()
	if fired := rs.noteWordHit(SeverityNuke); len(fired) != 0 {
		t.Errorf("noteWordHit(SeverityNuke) fired %v; a nuke hit must never reach the raid guard's own scoring", fired)
	}
	if rs.hasFired(SigSlurSevere) || rs.hasFired(SigSlurFlagged) {
		t.Error("SeverityNuke fired a slur signal despite noteWordHit reporting none")
	}
	if score, _, _ := rs.snapshot(); score != 0 {
		t.Errorf("SeverityNuke added %d to the score; it must add nothing", score)
	}
}

// TestSlurAloneNeverPunishes is the safety property behind both new weights.
//
// Note this is a STRONGER claim than the file's general invariant, and
// deliberately so. "No single signal rises above an alert" is not true of the
// guard as a whole -- SigHandshakeAnomaly (50) already reaches the challenge
// rung on its own at the strict tier, where the threshold is 60 * 70% = 42 --
// and the codebase's actual floor is TestNoSingleSignalCanBan's "never reaches
// a disconnect alone". The two word-list weights are held to the tighter bar
// because they are the only signals driven by CONTENT rather than behaviour,
// and content is the thing an ordinary player can trip by saying one word. 40
// was chosen to sit just under that 42 line for exactly this reason, so if
// someone later raises it to 45 this test fails rather than the property
// quietly evaporating.
//
// The original property, restated:
// a slur is strong evidence of intent but zero evidence of a coordinated
// fan-out, so on its own it must never reach a PUNITIVE rung -- Silence or
// worse, an action stronger than something the connection resolves itself by
// answering the pending captcha -- at any playtime tier, including the strict
// one a brand-new connection is judged at. This is the loud version of
// TestNoSingleSignalCanBan, specifically for the two signals added here.
//
// The boundary is deliberately VerdictSilence, not VerdictWatch: at the
// strict tier a severe slur alone (45) DOES cross into VerdictChallenge
// (effective threshold 42) -- exactly like the pre-existing SigHandshakeAnomaly
// (50) already does at that same tier, which TestNoSingleSignalCanBan's
// baseline-only check never exercised. Challenge is the self-service rung: it
// hands the connection a question it can answer itself, the same recourse
// VerdictSilence itself leaves available (raidGuardSilence's own doc comment).
// Silence is where the line actually belongs, and it is also the strongest
// verdict a bare two-signal combination (slur + one more) can ever reach
// regardless of weight, since clampDisconnect refuses Kick/Ban below
// minSignalsToDisconnect -- see TestTwoSignalsCannotDisconnect.
func TestSlurAloneNeverPunishes(t *testing.T) {
	withRaidConfig(t)
	for _, sig := range []SignalKind{SigSlurSevere, SigSlurFlagged} {
		w := signalWeight(sig)
		for _, tier := range []struct {
			name  string
			scale int
		}{
			{"strict", config.RaidGuardStrictScale},
			{"baseline", raidGuardScaleBase},
			{"lenient", config.RaidGuardLenientScale},
		} {
			if v := verdictForTier(w, tier.scale); v > VerdictWatch {
				t.Errorf("%s alone (weight %d) at the %s tier (%d%%) reached %v; a slur alone must NEVER "+
					"reach a punitive rung (silence or worse), at any playtime tier", raidSignalName[sig], w, tier.name, tier.scale, v)
			}
		}
	}
}

// TestSlurCannotArmBan checks the ban gate's cross-IPID-corroboration
// requirement holds even when a severe slur is stacked with two other
// independent, non-corroboration signals: the combination reaches a
// ban-level SCORE at the strict tier, but raidBanAllowed still refuses to let
// that become a ban without SigDupeAcrossIPIDs having fired. A slur, however
// severe, is not evidence of a fan-out, and cannot buy its way past the one
// check that is.
func TestSlurCannotArmBan(t *testing.T) {
	withRaidConfig(t)
	rs := newRaidState()
	rs.markFired(SigSlurSevere)
	rs.markFired(SigHandshakeAnomaly)
	rs.markFired(SigObjectionSpam)
	score, signals, _ := rs.snapshot()

	if v := verdictForTier(score, config.RaidGuardStrictScale); v < VerdictBan {
		t.Fatalf("severe slur + handshake anomaly + objection spam (score %d, signals %v) does not even reach "+
			"a ban-level score (%v) at the strict tier; the fixture needs strengthening for this test to mean "+
			"anything", score, signals, v)
	}
	if rs.hasFired(SigDupeAcrossIPIDs) {
		t.Fatal("fixture accidentally fired the corroboration signal itself")
	}
	// Even with the server concurrently "under attack" (the most permissive
	// half of the gate), no ban is allowed without corroboration for THIS
	// connection.
	if raidBanAllowed(rs.hasFired(SigDupeAcrossIPIDs), true) {
		t.Error("a ban-level score built from a severe slur plus two other signals was allowed to arm a ban " +
			"without cross-IPID corroboration -- raidBanAllowed's gate has been weakened")
	}
}

// TestSlurSignalFiresOnce checks repeated slurs cannot inflate the score
// beyond each signal firing once, matching every other signal in this file
// (see TestSignalsFireOnce). Without this, a connection that keeps talking
// could grind its way to a ban purely by repeating one flagged word many
// times, which would be exactly the single-behaviour-inflation rule 1 exists
// to prevent.
func TestSlurSignalFiresOnce(t *testing.T) {
	withRaidConfig(t)
	rs := newRaidState()
	for i := 0; i < 50; i++ {
		rs.noteWordHit(SeveritySevere)
		rs.noteWordHit(SeverityDefault)
		rs.noteWordHit(SeverityWatch)
	}
	score, signals, _ := rs.snapshot()
	want := signalWeight(SigSlurSevere) + signalWeight(SigSlurFlagged)
	if score != want {
		t.Errorf("score after 50 rounds of repeated severe+default+watch hits = %d, want %d "+
			"(SigSlurSevere and SigSlurFlagged each firing exactly once); signals=%v", score, want, signals)
	}
}

// TestRaidGuardOnWordHitNoop checks the wire hook is inert for a nil client,
// for an unmatched WordListMatch, and for a nuke-tier match -- none of those
// three should so much as allocate a raidState, mirroring how the other wire
// hooks treat their own do-nothing inputs (TestWireFeatureGateOff).
func TestRaidGuardOnWordHitNoop(t *testing.T) {
	withWiring(t)

	// A nil client must never panic, and there is nothing to allocate into.
	raidGuardOnWordHit(nil, WordListMatch{Matched: true, Entry: WordEntry{Severity: SeveritySevere}})

	c := wireTestClient(t, "wordhit-noop-ipid", 0)
	raidGuardOnWordHit(c, WordListMatch{Matched: false, Entry: WordEntry{Severity: SeveritySevere}})
	c.mu.Lock()
	rs := c.raid
	c.mu.Unlock()
	if rs != nil {
		t.Error("an unmatched WordListMatch allocated raid-guard state")
	}

	raidGuardOnWordHit(c, WordListMatch{Matched: true, Entry: WordEntry{Severity: SeverityNuke}})
	c.mu.Lock()
	rs = c.raid
	c.mu.Unlock()
	if rs != nil {
		t.Error("a nuke-tier match allocated raid-guard state through the wire hook -- nuke must never reach " +
			"the raid guard's scoring engine")
	}
}

// TestRaidGuardOnWordHitFires checks a real severe match reaches the engine
// through the wire hook, and is evaluated (not merely recorded): a lone
// severe slur is exactly heavy enough to reach VerdictWatch and no further,
// so that is what the hook's own escalation bookkeeping should land on.
func TestRaidGuardOnWordHitFires(t *testing.T) {
	withWiring(t)
	c := wireTestClient(t, "wordhit-fires-ipid", 0)
	raidGuardOnWordHit(c, WordListMatch{Matched: true, Entry: WordEntry{Raw: "test", Severity: SeveritySevere}})

	rs := c.raidGuard()
	if rs == nil || !rs.firedSignal(SigSlurSevere) {
		t.Fatal("a matched severe word-list hit did not fire SigSlurSevere through the wire hook")
	}
	if _, _, acted := rs.snapshot(); acted != VerdictWatch {
		t.Errorf("acted verdict after one severe slur = %v, want watch (the only level a lone slur signal "+
			"can ever reach)", acted)
	}
}
