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

import (
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

var testNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func testWorld(callerUID int, players ...*TestPlayer) TestWorld {
	w := TestWorld{
		Areas:     []TestArea{{Name: "Area 0"}},
		Players:   make(map[int]*TestPlayer),
		CallerUID: callerUID,
	}
	for _, p := range players {
		w.Players[p.UID] = p
	}
	return w
}

func testPlayer(uid int, name string, perms uint64) *TestPlayer {
	return &TestPlayer{UID: uid, Name: name, Perms: perms, AreaName: "Area 0"}
}

func TestValidateCustomCommand(t *testing.T) {
	initCommands()
	valid := CustomCommand{Name: "boop", Desc: "boop", Category: "fun", Actions: []CustomAction{{Type: ActionMessage, Target: "@args", Text: "hi"}}}
	if err := validateCustomCommand(valid); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}
	if err := validateCustomCommand(CustomCommand{Name: "", Actions: []CustomAction{{Type: ActionMessage, Text: "hi"}}}); err == nil {
		t.Error("empty name must be rejected")
	}
	if err := validateCustomCommand(CustomCommand{Name: "ban", Actions: []CustomAction{{Type: ActionMessage, Text: "hi"}}}); err == nil {
		t.Error("name colliding with a built-in must be rejected")
	}
	if err := validateCustomCommand(CustomCommand{Name: "x", Actions: []CustomAction{{Type: ActionPunish, Effect: "not-a-real-effect"}}}); err == nil {
		t.Error("unknown punishment effect must be rejected")
	}
	if err := validateCustomCommand(CustomCommand{Name: "x", Actions: []CustomAction{{Type: CustomActionType("bogus")}}}); err == nil {
		t.Error("unknown action type must be rejected")
	}
	if err := validateCustomCommand(CustomCommand{Name: "x"}); err == nil {
		t.Error("a command with no actions must be rejected")
	}
}

func TestPunishmentTypeByName(t *testing.T) {
	if _, ok := punishmentTypeByName("uwu"); !ok {
		t.Error("uwu should resolve to a punishment type")
	}
	if _, ok := punishmentTypeByName("backward"); !ok {
		t.Error("backward should resolve to a punishment type")
	}
	if _, ok := punishmentTypeByName("definitely-not-real"); ok {
		t.Error("a bogus name must not resolve")
	}
	if len(allPunishmentNames()) < 50 {
		t.Errorf("expected a large punishment name set, got %d", len(allPunishmentNames()))
	}
}

func TestRenderTemplate(t *testing.T) {
	ctx := renderContext{args: []string{"7"}, callerUID: 3, callerName: "mod", targetUID: 7, targetName: "bob", reason: "why", duration: 5 * time.Minute}
	got := renderTemplate("caller={caller.name} uid={caller.uid} target={target.name} uid={target.uid} arg0={args.0} r={reason} d={duration}", ctx)
	want := "caller=mod uid=3 target=bob uid=7 arg0=7 r=why d=5m0s"
	if got != want {
		t.Errorf("template mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestResolveTargetUIDs(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	mod2 := testPlayer(2, "mod2", permissions.PermissionField["MUTE"])
	plain := testPlayer(3, "bob", 0)
	plain2 := testPlayer(4, "ann", 0)
	otherArea := testPlayer(5, "zed", 0)
	otherArea.AreaName = "Area 1"
	w := testWorld(1, caller, mod2, plain, plain2, otherArea)

	got := resolveTargetUIDs(w, "@self", caller, nil)
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("@self = %v", got)
	}
	got = resolveTargetUIDs(w, "@args", caller, []string{"3,4"})
	if len(got) != 2 {
		t.Errorf("@args should resolve both UIDs, got %v", got)
	}
	got = resolveTargetUIDs(w, "@global", caller, nil)
	if len(got) != 2 { // plain + plain2 (both non-mods, non-caller, same area)
		t.Errorf("@global should exclude moderators and caller, got %v", got)
	}
	got = resolveTargetUIDs(w, "@area", caller, nil)
	if len(got) != 3 { // mod2 + plain + plain2 (same area, not caller)
		t.Errorf("@area should be 3 players, got %v", got)
	}
	got = resolveTargetUIDs(w, "@all", caller, nil)
	if len(got) != 4 { // everyone except caller
		t.Errorf("@all should be 4 players, got %v", got)
	}
	got = resolveTargetUIDs(w, "3", caller, nil)
	if len(got) != 1 || got[0] != 3 {
		t.Errorf("literal UID = %v", got)
	}
}

func TestExecuteMessageAndPurity(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	target := testPlayer(3, "bob", 0)
	w := testWorld(1, caller, target)

	cmd := CustomCommand{Name: "boop", MinArgs: 1, Actions: []CustomAction{{Type: ActionMessage, Target: "@args", Text: "booped by {caller.name}"}}}
	res := executeCustomCommandPure(w, cmd, []string{"3"}, testNow)

	if res.Decision != DecisionAccepted {
		t.Fatalf("expected accepted, got %v", res.Decision)
	}
	if len(res.Effects) != 1 || res.Effects[0].Kind != EffectMessage {
		t.Fatalf("expected one message effect, got %+v", res.Effects)
	}
	if res.Effects[0].Text != "booped by mod" {
		t.Errorf("templating failed: %q", res.Effects[0].Text)
	}
	// Purity: the input world must be untouched.
	if len(w.Players[3].Punishments) != 0 {
		t.Error("input world was mutated (punishments changed)")
	}
}

func TestExecutePunish(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	target := testPlayer(3, "bob", 0)
	w := testWorld(1, caller, target)

	cmd := CustomCommand{Name: "p", MinArgs: 1, Actions: []CustomAction{{Type: ActionPunish, Target: "@args", Effect: "uwu", Duration: "20m"}}}
	res := executeCustomCommandPure(w, cmd, []string{"3"}, testNow)

	if res.Decision != DecisionAccepted || len(res.Effects) != 1 {
		t.Fatalf("expected one punish effect, got %+v", res.Effects)
	}
	e := res.Effects[0]
	if e.Kind != EffectPunishApplied || e.Punishment != PunishmentUwu || e.Duration != 20*time.Minute {
		t.Fatalf("unexpected effect %+v", e)
	}
	ps := res.World.Players[3].Punishments
	if len(ps) != 1 || ps[0].Type != PunishmentUwu || !ps[0].ExpiresAt.Equal(testNow.Add(20*time.Minute)) {
		t.Fatalf("world state wrong: %+v", ps)
	}
}

func TestExecutePunishDedup(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	target := testPlayer(3, "bob", 0)
	target.Punishments = []TestPunishment{{Type: PunishmentUwu, ExpiresAt: testNow}}
	w := testWorld(1, caller, target)
	cmd := CustomCommand{Name: "p", MinArgs: 1, Actions: []CustomAction{{Type: ActionPunish, Target: "@args", Effect: "uwu", Duration: "10m"}}}
	res := executeCustomCommandPure(w, cmd, []string{"3"}, testNow)
	if len(res.World.Players[3].Punishments) != 1 {
		t.Errorf("same-type punishment must dedup, got %d", len(res.World.Players[3].Punishments))
	}
}

func TestExecutePunishSafeAreaAndKillSwitch(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	target := testPlayer(3, "bob", 0)
	target.AreaName = "Safe"
	w := testWorld(1, caller, target)
	w.Areas = append(w.Areas, TestArea{Name: "Safe", PunishmentSafe: true})

	cmd := CustomCommand{Name: "p", MinArgs: 1, Actions: []CustomAction{{Type: ActionPunish, Target: "@args", Effect: "uwu", Duration: "10m"}}}
	res := executeCustomCommandPure(w, cmd, []string{"3"}, testNow)
	if len(res.Effects) != 0 {
		t.Errorf("safe-area target must be skipped, got %+v", res.Effects)
	}

	w2 := testWorld(1, caller, target)
	w2.Config.PunishmentsDisabled = true
	res2 := executeCustomCommandPure(w2, cmd, []string{"3"}, testNow)
	if len(res2.Effects) != 0 {
		t.Errorf("kill-switch must suppress punish, got %+v", res2.Effects)
	}
}

func TestExecuteUnpunish(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	target := testPlayer(3, "bob", 0)
	target.Punishments = []TestPunishment{{Type: PunishmentUwu, ExpiresAt: testNow}, {Type: PunishmentBackward, ExpiresAt: testNow}}
	w := testWorld(1, caller, target)

	all := CustomCommand{Name: "c", MinArgs: 1, Actions: []CustomAction{{Type: ActionUnpunish, Target: "@args", Effect: "all"}}}
	res := executeCustomCommandPure(w, all, []string{"3"}, testNow)
	if len(res.World.Players[3].Punishments) != 0 {
		t.Errorf("unpunish all should clear, got %+v", res.World.Players[3].Punishments)
	}
	if len(res.Effects) != 1 || res.Effects[0].Punishment != PunishmentNone {
		t.Errorf("unpunish all effect wrong: %+v", res.Effects)
	}

	one := CustomCommand{Name: "c", MinArgs: 1, Actions: []CustomAction{{Type: ActionUnpunish, Target: "@args", Effect: "uwu"}}}
	res2 := executeCustomCommandPure(w, one, []string{"3"}, testNow)
	if len(res2.World.Players[3].Punishments) != 1 || res2.World.Players[3].Punishments[0].Type != PunishmentBackward {
		t.Errorf("unpunish one should leave the other, got %+v", res2.World.Players[3].Punishments)
	}
}

func TestExecuteKickAndBan(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["BAN"])
	target := testPlayer(3, "bob", 0)
	w := testWorld(1, caller, target)

	cmd := CustomCommand{Name: "k", MinArgs: 1, Actions: []CustomAction{{Type: ActionKick, Target: "@args", Reason: "bye"}}}
	res := executeCustomCommandPure(w, cmd, []string{"3"}, testNow)
	if _, exists := res.World.Players[3]; exists {
		t.Error("kicked player should be removed from the world")
	}
	if len(res.Effects) != 1 || res.Effects[0].Kind != EffectKick {
		t.Errorf("kick effect wrong: %+v", res.Effects)
	}

	w2 := testWorld(1, caller, target)
	ban := CustomCommand{Name: "b", MinArgs: 1, Actions: []CustomAction{{Type: ActionBan, Target: "@args", Duration: "1h"}}}
	res2 := executeCustomCommandPure(w2, ban, []string{"3"}, testNow)
	if _, exists := res2.World.Players[3]; exists {
		t.Error("banned player should be removed from the world")
	}
	if len(res2.Effects) != 1 || res2.Effects[0].Kind != EffectBan || res2.Effects[0].Duration != time.Hour {
		t.Errorf("ban effect wrong: %+v", res2.Effects)
	}
}

func TestExecuteMuteUnmuteMove(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	target := testPlayer(3, "bob", 0)
	w := testWorld(1, caller, target)
	w.Areas = append(w.Areas, TestArea{Name: "Room 2"})

	mute := CustomCommand{Name: "m", MinArgs: 1, Actions: []CustomAction{{Type: ActionMute, Target: "@args", Scope: "both", Duration: "5m"}}}
	res := executeCustomCommandPure(w, mute, []string{"3"}, testNow)
	if res.World.Players[3].Muted != ICOOCMuted {
		t.Errorf("mute scope wrong: %v", res.World.Players[3].Muted)
	}

	move := CustomCommand{Name: "mv", MinArgs: 1, Actions: []CustomAction{{Type: ActionMove, Target: "@args", Area: "Room 2"}}}
	res2 := executeCustomCommandPure(w, move, []string{"3"}, testNow)
	if res2.World.Players[3].AreaName != "Room 2" {
		t.Errorf("move failed: %v", res2.World.Players[3].AreaName)
	}

	unmute := CustomCommand{Name: "um", MinArgs: 1, Actions: []CustomAction{{Type: ActionUnmute, Target: "@args"}}}
	res3 := executeCustomCommandPure(w, unmute, []string{"3"}, testNow)
	if res3.World.Players[3].Muted != Unmuted {
		t.Errorf("unmute failed: %v", res3.World.Players[3].Muted)
	}
}

func TestExecuteRun(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["ADMIN"])
	w := testWorld(1, caller)

	cmd := CustomCommand{Name: "r", MinArgs: 1, Actions: []CustomAction{{Type: ActionRun, Command: "backward", Args: []string{"{args.0}", "-d", "5m"}}}}
	res := executeCustomCommandPure(w, cmd, []string{"3"}, testNow)
	if len(res.Effects) != 1 || res.Effects[0].Kind != EffectRun {
		t.Fatalf("expected run effect, got %+v", res.Effects)
	}
	if res.Effects[0].Command != "backward" || strings.Join(res.Effects[0].Args, " ") != "3 -d 5m" {
		t.Errorf("run args templated wrong: %+v", res.Effects[0])
	}
}

func TestExecuteRandomResolution(t *testing.T) {
	actions := []CustomAction{{
		Type: ActionRandom,
		Choices: []CustomActionList{
			{Actions: []CustomAction{{Type: ActionMessage, Target: "@self", Text: "one"}}},
			{Actions: []CustomAction{{Type: ActionMessage, Target: "@self", Text: "two"}}},
		},
	}}
	resolved := resolveRandomInActions(actions, func(int) int { return 1 })
	if len(resolved) != 1 || resolved[0].Text != "two" {
		t.Errorf("random must pick the injected choice, got %+v", resolved)
	}
}

func TestExecuteMinArgsRejection(t *testing.T) {
	caller := testPlayer(1, "mod", permissions.PermissionField["MUTE"])
	w := testWorld(1, caller)
	cmd := CustomCommand{Name: "x", MinArgs: 1, Actions: []CustomAction{{Type: ActionMessage, Target: "@self", Text: "hi"}}}
	res := executeCustomCommandPure(w, cmd, nil, testNow)
	if res.Decision != DecisionRejectedBadArgs {
		t.Errorf("expected bad-args rejection, got %v", res.Decision)
	}
}

// TestGrantCommandAcceptsCustomName proves the grantcmd console verb can grant
// a custom command to an account, extending the existing per-command grant
// mechanism to runtime-defined commands.
func TestGrantCommandAcceptsCustomName(t *testing.T) {
	cleanup := setupCommandGrantsTestDB(t)
	defer cleanup()
	initCommands()
	setCustomCommands(map[string]CustomCommand{"mycmd": {Name: "mycmd", Desc: "test", Actions: []CustomAction{{Type: ActionMessage, Target: "@self", Text: "hi"}}}})
	defer setCustomCommands(nil)

	if _, err := grantCommand("alice", "mycmd", "console"); err != nil {
		t.Fatalf("granting a custom command should work: %v", err)
	}
	if _, err := grantCommand("alice", "definitely-not-real", "console"); err == nil {
		t.Error("granting an unknown command name must still be rejected")
	}
}

// TestCustomCommandsEnabled covers the opt-in gate: the feature is inert unless
// enable_custom_commands is set, so servers that don't use it pay nothing.
func TestCustomCommandsEnabled(t *testing.T) {
	prev := config
	defer func() { config = prev }()

	config = &settings.Config{ServerConfig: settings.ServerConfig{EnableCustomCommands: false}}
	if customCommandsEnabled() {
		t.Error("customCommandsEnabled() must be false when the flag is off")
	}
	config = &settings.Config{ServerConfig: settings.ServerConfig{EnableCustomCommands: true}}
	if !customCommandsEnabled() {
		t.Error("customCommandsEnabled() must be true when the flag is on")
	}
	config = nil
	if customCommandsEnabled() {
		t.Error("customCommandsEnabled() must be false when config is nil")
	}
}
