/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the AO Gold / Skrapegropen paywall. */

package athena

import (
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

func isGoldPaywallLine(s string) bool {
	for _, line := range goldPaywallLines {
		if s == line {
			return true
		}
	}
	return false
}

// TestGoldCommandAliasesRegistered pins all three names to the same MUTE-level
// punishment entry.
func TestGoldCommandAliasesRegistered(t *testing.T) {
	initCommands()
	for _, name := range []string{"gold", "premium", "paywall"} {
		cmd, ok := Commands[name]
		if !ok {
			t.Errorf("%s command is not registered", name)
			continue
		}
		if cmd.reqPerms != permissions.PermissionField["MUTE"] {
			t.Errorf("%s reqPerms = %d, want MUTE", name, cmd.reqPerms)
		}
		if cmd.category != "punishment" {
			t.Errorf("%s category = %q, want punishment", name, cmd.category)
		}
	}
}

// TestGoldPaywallPoolFixesTypo pins the corrected "requires" line and makes
// sure the original misspelling cannot creep back in.
func TestGoldPaywallPoolFixesTypo(t *testing.T) {
	for _, line := range goldPaywallLines {
		if strings.Contains(line, "reuires") {
			t.Errorf("pool still contains the misspelling: %q", line)
		}
	}
	want := "OOPS! This message requires a Premium Skrapegropen Account."
	found := false
	for _, line := range goldPaywallLines {
		if line == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("corrected OOPS line is missing from the pool; pool = %q", goldPaywallLines)
	}
}

// TestGoldPaywallLineRotates verifies that repeated messages draw more than one
// branding variant, so AO Gold and Skrapegropen Premium both appear.
func TestGoldPaywallLineRotates(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		line := goldPaywallLine()
		if !isGoldPaywallLine(line) {
			t.Fatalf("goldPaywallLine returned a line outside the pool: %q", line)
		}
		seen[line] = struct{}{}
	}
	if len(seen) < 2 {
		t.Errorf("only one paywall variant was drawn in 200 messages: %q", seen)
	}
}

// TestApplyGoldPaywallOOC verifies the OOC helper only replaces a message when
// the speaker actually carries the punishment.
func TestApplyGoldPaywallOOC(t *testing.T) {
	clean := &Client{}
	if got := applyGoldPaywall(clean, "hello"); got != "hello" {
		t.Errorf("unpunished OOC message was changed: %q", got)
	}

	punished := &Client{}
	punished.AddPunishment(PunishmentGold, time.Minute, "test")
	got := applyGoldPaywall(punished, "hello")
	if got == "hello" {
		t.Fatal("punished OOC message was not replaced")
	}
	if !isGoldPaywallLine(got) {
		t.Errorf("OOC replacement was not a pool line: %q", got)
	}
}

// TestApplyPunishmentToTextGold verifies the IC text path uses the same pool.
func TestApplyPunishmentToTextGold(t *testing.T) {
	got := ApplyPunishmentToText("hello", PunishmentGold)
	if !isGoldPaywallLine(got) {
		t.Errorf("IC gold replacement was not a pool line: %q", got)
	}
}

// TestCmdGoldAppliesAndUnpunishRemoves drives the real command end-to-end.
func TestCmdGoldAppliesAndUnpunishRemoves(t *testing.T) {
	t.Cleanup(setupAreaMuteTestDB(t))
	newTestClients(t)

	a := makeTestArea("Courtroom")
	t.Cleanup(setupTestAreas([]*area.Area{a}))

	modConn := &captureConn{}
	targetConn := &captureConn{}
	mod := &Client{
		conn: modConn, uid: 1, ipid: "ip-gold-mod", char: -1, area: a,
		perms: permissions.PermissionField["MUTE"], mod_name: "Mod",
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	target := &Client{
		conn: targetConn, uid: 2, ipid: "ip-gold-target", char: 0, area: a,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	for _, c := range []*Client{mod, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdGold(mod, []string{"2"}, "usage")

	if !target.HasActivePunishment(PunishmentGold) {
		t.Fatal("/gold did not apply the paywall punishment")
	}
	if got := targetConn.String(); !strings.Contains(got, "gold") {
		t.Errorf("target was not told about the gold punishment; got %q", got)
	}
	if got := modConn.String(); !strings.Contains(got, "Applied 'gold'") {
		t.Errorf("issuer did not get the punishment summary; got %q", got)
	}

	cmdUnpunish(mod, []string{"-t", "gold", "2"}, "usage")

	if target.HasActivePunishment(PunishmentGold) {
		t.Error("/unpunish -t gold did not remove the paywall")
	}
}
