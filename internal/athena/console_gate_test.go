package athena

import (
	"strings"
	"testing"
	"time"
)

// The console gate exists because ADMIN is no longer sufficient for the handful
// of commands whose effect is invisible to the person they are used on. These
// tests pin the properties that claim rests on.

func TestConsoleGateRefusesUntilArmed(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	for _, cmd := range consoleGatedCommandNames() {
		if consumeConsoleGrant(nil, cmd) {
			t.Errorf("/%v ran with no grant armed", cmd)
		}
	}
}

func TestConsoleGrantIsGoodForExactlyOneUse(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	for _, cmd := range consoleGatedCommandNames() {
		if ok, err := grantConsoleUse(cmd); !ok {
			t.Fatalf("arming /%v failed: %v", cmd, err)
		}
		if !consumeConsoleGrant(nil, cmd) {
			t.Fatalf("/%v refused immediately after being armed", cmd)
		}
		if consumeConsoleGrant(nil, cmd) {
			t.Errorf("/%v ran twice on one grant; arming must buy a single use", cmd)
		}
	}
}

// Arming twice must not stack into two uses -- the gate's entire claim is that
// one arming buys one use, and a console operator repeating themselves should
// not silently hand out a second.
func TestReArmingDoesNotStackUses(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	grantConsoleUse("possess")
	grantConsoleUse("possess")
	grantConsoleUse("possess")

	if !consumeConsoleGrant(nil, "possess") {
		t.Fatal("armed grant was refused")
	}
	if consumeConsoleGrant(nil, "possess") {
		t.Error("three armings yielded more than one use")
	}
}

// An unused grant must lapse. A grant armed and forgotten would otherwise leave
// the power permanently available again, which is what the gate exists to stop.
func TestUnusedGrantExpires(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	now := time.Now()
	consoleGrants.mu.Lock()
	consoleGrants.nowFor = func() time.Time { return now }
	consoleGrants.mu.Unlock()

	grantConsoleUse("shadowdisconnect")

	// Still inside the window.
	consoleGrants.mu.Lock()
	consoleGrants.nowFor = func() time.Time { return now.Add(consoleGrantTTL - time.Second) }
	consoleGrants.mu.Unlock()
	if !grantIsArmed("shadowdisconnect") {
		t.Fatal("grant lapsed early")
	}

	// Past it.
	consoleGrants.mu.Lock()
	consoleGrants.nowFor = func() time.Time { return now.Add(consoleGrantTTL + time.Second) }
	consoleGrants.mu.Unlock()
	if consumeConsoleGrant(nil, "shadowdisconnect") {
		t.Error("an expired grant was still usable")
	}
}

func TestRevokeDisarms(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	grantConsoleUse("possess")
	grantConsoleUse("fullpossess")

	if n, _ := revokeConsoleUse("possess"); n != 1 {
		t.Errorf("revoke possess removed %d, want 1", n)
	}
	if consumeConsoleGrant(nil, "possess") {
		t.Error("/possess ran after its grant was revoked")
	}
	if !consumeConsoleGrant(nil, "fullpossess") {
		t.Error("revoking one command disarmed another")
	}

	grantConsoleUse("possess")
	grantConsoleUse("truepossess")
	if n, _ := revokeConsoleUse("all"); n != 2 {
		t.Errorf("revoke all removed %d, want 2", n)
	}
}

// An unknown name must be rejected rather than silently accepted: a console
// operator who fat-fingers `grant posses` should be told, not left believing
// they have armed something.
func TestGrantRejectsUnknownCommand(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	if ok, err := grantConsoleUse("posses"); ok || err == "" {
		t.Error("a misspelled command name was accepted")
	}
	if ok, err := grantConsoleUse("ban"); ok || err == "" {
		t.Error("an ungated command was accepted by grant")
	}
}

// Both possession names share one handler, so gating one without the other
// would gate nothing at all. This is the trap that makes the alias worth an
// explicit assertion.
func TestBothPossessionAliasesAreGated(t *testing.T) {
	for _, name := range []string{"possess", "fullpossess", "truepossess"} {
		if !isConsoleGated(name) {
			t.Errorf("/%v is not console-gated; the other possession alias would bypass the gate", name)
		}
	}
	if !isConsoleGated("shadowdisconnect") {
		t.Error("/shadowdisconnect is not console-gated")
	}
}

// The manual path onto the torment list is gone from the command registry.
// Removal stays available in game -- applying needs console, lifting does not,
// and that asymmetry is what keeps a mistake fixable by whoever is online.
func TestLagRemovedButRemovalToolingKept(t *testing.T) {
	initCommands()

	if _, ok := Commands["lag"]; ok {
		t.Error("/lag is still registered; the torment list must only be applied from the console")
	}
	for _, name := range []string{"unlag", "untorment", "tormentlist"} {
		if _, ok := Commands[name]; !ok {
			t.Errorf("/%v should still be registered; only applying a torment is restricted", name)
		}
	}
}

// The help listing must not advertise a command that no longer exists.
func TestPunishmentHelpDoesNotListLag(t *testing.T) {
	for _, group := range punishmentHelpGroups {
		for _, c := range group.cmds {
			if c == "lag" {
				t.Error("punishment help still lists /lag")
			}
		}
	}
}

func TestGrantStatusNamesEveryGatedCommand(t *testing.T) {
	resetConsoleGrants()
	t.Cleanup(resetConsoleGrants)

	joined := strings.Join(consoleGrantStatus(), "\n")
	for _, name := range consoleGatedCommandNames() {
		if !strings.Contains(joined, "/"+name) {
			t.Errorf("grant status omits /%v", name)
		}
	}
	if !strings.Contains(joined, "locked") {
		t.Error("grant status does not report the locked state")
	}
	grantConsoleUse("possess")
	if !strings.Contains(strings.Join(consoleGrantStatus(), "\n"), "ARMED") {
		t.Error("grant status does not report an armed grant")
	}
}
