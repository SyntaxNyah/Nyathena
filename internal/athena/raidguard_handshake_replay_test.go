package athena

import (
	"strings"
	"testing"
)

// mustScore reads a raidState's accumulated score through the same accessor
// production uses, so these tests cannot drift from it.
func mustScore(rs *raidState) int {
	score, _, _ := rs.snapshot()
	return score
}

// SigHandshakeReplay fires on an askchaa that arrives after the connection has
// already joined. These tests pin the three things that make it safe to weight
// it as highly as it is: it does not fire on the ordinary handshake, it does
// not fire on a client whose first askchaa went unanswered and retried, and on
// its own it never reaches a verdict the player cannot undo themselves.

func TestHandshakeReplayFiresOnlyAfterJoin(t *testing.T) {
	t.Run("ordinary handshake does not fire", func(t *testing.T) {
		rs := newRaidState()
		rs.noteAskchaa() // pre-join: the normal first step
		if rs.firedSignal(SigHandshakeReplay) {
			t.Fatal("a normal pre-join askchaa fired the replay signal")
		}
		if got := mustScore(rs); got != 0 {
			t.Fatalf("normal handshake scored %d, want 0", got)
		}
	})

	t.Run("pre-join retry does not fire", func(t *testing.T) {
		// pktResCount returns without sending SI when the HDID is not set yet,
		// so a client can legitimately send askchaa again while still unjoined.
		// The signal keys on having joined, not on the count, precisely so this
		// case is not swept up.
		rs := newRaidState()
		rs.noteAskchaa()
		rs.noteAskchaa()
		rs.noteAskchaa()
		if rs.firedSignal(SigHandshakeReplay) {
			t.Fatal("pre-join askchaa retries fired the replay signal")
		}
	})

	t.Run("post-join fires, once", func(t *testing.T) {
		rs := newRaidState()
		rs.noteAskchaa() // the real one
		if !rs.noteAskchaaPostJoin() {
			t.Fatal("post-join askchaa did not fire")
		}
		if rs.noteAskchaaPostJoin() {
			t.Error("post-join askchaa fired twice; signals must fire at most once")
		}
		if got, want := mustScore(rs), raidSignalWeight[SigHandshakeReplay]; got != want {
			t.Fatalf("score = %d, want %d", got, want)
		}
	})
}

// The weight is chosen so this signal ALONE lands on the captcha rung for a
// brand-new connection and the quarantine rung once the server is already known
// to be under attack -- both of which a real player can undo without staff --
// while never reaching kick or ban at any tier.
//
// It matters that this holds on the arithmetic rather than only via
// clampDisconnect's three-signal floor: the floor is a backstop for a
// mis-calibrated weight, and a weight that leans on the backstop has spent it.
func TestHandshakeReplayAloneNeverDisconnects(t *testing.T) {
	withRaidConfig(t)
	w := raidSignalWeight[SigHandshakeReplay]

	for _, tc := range []struct {
		name  string
		scale int
		want  Verdict
	}{
		// scalePct as raidGuardTier composes it: the playtime tier, multiplied
		// by raid_guard_under_attack_scale (70) while a raid is in progress.
		{"strict", 70, VerdictChallenge},
		{"strict under attack", 70 * 70 / 100, VerdictSilence},
		{"baseline", 100, VerdictWatch},
		{"baseline under attack", 70, VerdictChallenge},
		{"lenient", 200, VerdictClean},
		{"lenient under attack", 200 * 70 / 100, VerdictClean},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withRaidConfig(t)
			got := verdictForTier(w, tc.scale)
			if got != tc.want {
				t.Errorf("verdictForTier(%d, %d) = %v, want %v", w, tc.scale, got, tc.want)
			}
			if got >= VerdictKick {
				t.Errorf("this signal alone reached %v at scale %d; it must never "+
					"reach an action the player cannot undo", got, tc.scale)
			}
		})
	}
}

// The ban gate is unaffected by adding a signal: a ban still needs this
// connection's own text corroborated from other IPIDs AND a concurrent
// server-wide raid, neither of which one person can produce.
func TestHandshakeReplayCannotReachTheBanGate(t *testing.T) {
	if raidBanAllowed(false, true) || raidBanAllowed(true, false) || raidBanAllowed(false, false) {
		t.Fatal("ban gate accepted incomplete evidence")
	}
	rs := newRaidState()
	rs.noteAskchaa()
	rs.noteAskchaaPostJoin()
	if rs.firedSignal(SigDupeAcrossIPIDs) {
		t.Fatal("the handshake replay signal must not imply cross-IPID corroboration")
	}
}

// Replayed over the committed captures, the signal must be silent on both
// clean ones. This is the assertion that matters: the weight above is only
// defensible if ordinary players do not trip it.
func TestHandshakeReplaySilentOnCleanCaptures(t *testing.T) {
	withRaidConfig(t)
	for _, path := range []string{
		"testdata/normal_capture.log",
		"testdata/aftermath_capture.log",
	} {
		t.Run(path, func(t *testing.T) {
			withRaidConfig(t)
			var fired []string
			for _, r := range replayCaptureFile(t, path) {
				for _, s := range r.signals {
					if s == raidSignalName[SigHandshakeReplay] {
						fired = append(fired, r.ipid)
					}
				}
			}
			if len(fired) > 0 {
				t.Errorf("handshake-replay signal fired on %d clean connection(s): %v",
					len(fired), strings.Join(fired, ", "))
			}
		})
	}
}

// And it must actually fire on a real raid, or it is not worth its weight.
func TestHandshakeReplayFiresOnRaidCapture(t *testing.T) {
	withRaidConfig(t)
	var n int
	for _, r := range replayCaptureFile(t, "testdata/raid_capture.log") {
		for _, s := range r.signals {
			if s == raidSignalName[SigHandshakeReplay] {
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("handshake-replay signal never fired on the raid capture")
	}
	t.Logf("handshake-replay fired on %d raid connections", n)
}

// End to end through the real hook, which is where the pre/post-join
// distinction actually lives. wireTestClient hands out a joined connection
// (uid 1), so the pre-join case needs its uid set explicitly.
func TestWireHandshakeReplay(t *testing.T) {
	withWiring(t)

	joining := wireTestClient(t, "still-joining", 0)
	joining.SetUid(-1)
	raidGuardOnAskchaa(joining)
	raidGuardOnAskchaa(joining) // a retry while the server is ignoring us
	if rs := joining.raidGuard(); rs != nil && rs.firedSignal(SigHandshakeReplay) {
		t.Error("askchaa before the connection joined fired the replay signal")
	}

	joined := wireTestClient(t, "already-joined", 0) // uid 1
	raidGuardOnAskchaa(joined)
	rs := joined.raidGuard()
	if rs == nil || !rs.firedSignal(SigHandshakeReplay) {
		t.Fatal("askchaa after joining did not fire the replay signal")
	}
	// Recording the askchaa must still have happened, or the next list request
	// would be charged the ordering signal on top of this one.
	for i := 0; i < 3; i++ {
		raidGuardOnHandshakeStep(joined)
	}
	if rs.firedSignal(SigHandshakeAnomaly) {
		t.Error("a replayed handshake was also charged the ordering signal; " +
			"one behaviour must not fire two signals")
	}
}
