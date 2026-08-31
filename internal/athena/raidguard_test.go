package athena

import (
	"fmt"
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
