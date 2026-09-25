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

// The custom-command executor, modelled as a pure function over a world
// snapshot. This is the heart of the testability story: a command is evaluated
// against an explicit input state (TestWorld) and produces an explicit output
// state (Result), with no reads of mutable globals, the clock, randomness, or
// I/O anywhere inside the core. The live server derives a TestWorld from real
// state, runs the same core, then applies the recorded effects.
package athena

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/xhit/go-str2duration/v2"
)

// TestConfig is the subset of server configuration that affects command
// behaviour. It is an explicit input so tests can flip a flag and observe the
// resulting output rather than relying on whatever the loaded config happens
// to be.
type TestConfig struct {
	EnableCasino        bool
	EnableAccounts      bool
	EnableVoice         bool
	PunishmentsDisabled bool
	BanLength           string
}

// TestArea is a snapshot of an area's command-relevant state.
type TestArea struct {
	Name           string
	Locked         bool
	AdminLocked    bool
	PunishmentSafe bool
}

// TestPunishment is one active punishment on a TestPlayer.
type TestPunishment struct {
	Type      PunishmentType
	ExpiresAt time.Time
	Reason    string
}

// TestPlayer is a snapshot of a connected player's command-relevant state.
type TestPlayer struct {
	UID           int
	IPID          string
	Name          string
	Perms         uint64
	AreaName      string
	Muted         MuteState
	Punishments   []TestPunishment
	Authenticated bool
	Account       string
}

// TestWorld is the complete input state a custom command is evaluated against.
type TestWorld struct {
	Config     TestConfig
	Areas      []TestArea
	Characters []string
	Players    map[int]*TestPlayer
	CallerUID  int
}

// Decision is the accept/reject outcome of a command evaluation.
type Decision int

const (
	DecisionAccepted Decision = iota
	DecisionRejectedBadArgs
	DecisionRejectedValidation
)

// EffectKind enumerates the concrete output effects a command can produce.
type EffectKind int

const (
	EffectMessage EffectKind = iota
	EffectAnnounce
	EffectPunishApplied
	EffectPunishRemoved
	EffectKick
	EffectBan
	EffectMute
	EffectUnmute
	EffectMove
	EffectRun
	EffectGrant
	EffectRevoke
	EffectWait
)

// Effect is one concrete output of running a command. Only the fields relevant
// to Kind are meaningful.
type Effect struct {
	Kind       EffectKind
	To         []int // resolved target UIDs (nil for broadcast/announce/run)
	Text       string
	Punishment PunishmentType
	Duration   time.Duration
	Reason     string
	Scope      MuteState
	Area       string
	Command    string
	Args       []string
	Account    string
}

// Result is the complete output state of a command evaluation.
type Result struct {
	Decision Decision
	Effects  []Effect
	World    TestWorld // the world AFTER applying effects
}

// cloneWorld returns a deep-enough copy of the world so the pure executor never
// mutates its input. Maps and player structs (and their punishment slices) are
// copied; the shared immutable bits (area/character strings) are left as-is.
func cloneWorld(w TestWorld) TestWorld {
	old := w.Players
	w.Players = make(map[int]*TestPlayer, len(old))
	for uid, p := range old {
		if p == nil {
			continue
		}
		np := *p
		np.Punishments = append([]TestPunishment(nil), p.Punishments...)
		w.Players[uid] = &np
	}
	return w
}

// areaByName looks up a TestArea by name.
func areaByName(w TestWorld, name string) (TestArea, bool) {
	for _, a := range w.Areas {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return TestArea{}, false
}

// resolveTargetUIDs resolves a target selector to a list of UIDs present in the
// world. Selectors: "@self" (or empty), "@args" (first arg as a comma-separated
// UID list), "@global" (non-moderators in the caller's area, excluding the
// caller), "@area" (everyone in the caller's area except the caller), "@all"
// (everyone except the caller), or a literal comma-separated UID list.
func resolveTargetUIDs(w TestWorld, target string, caller *TestPlayer, args []string) []int {
	if caller == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "@self":
		return []int{caller.UID}
	case "@args":
		if len(args) == 0 {
			return nil
		}
		return uidsFromList(w, args[0])
	case "@global":
		var out []int
		for _, p := range w.Players {
			if p.UID == caller.UID || p.AreaName != caller.AreaName {
				continue
			}
			if permissions.IsModerator(p.Perms) {
				continue
			}
			out = append(out, p.UID)
		}
		return out
	case "@area":
		var out []int
		for _, p := range w.Players {
			if p.UID == caller.UID || p.AreaName != caller.AreaName {
				continue
			}
			out = append(out, p.UID)
		}
		return out
	case "@all":
		var out []int
		for _, p := range w.Players {
			if p.UID != caller.UID {
				out = append(out, p.UID)
			}
		}
		return out
	default:
		return uidsFromList(w, target)
	}
}

// uidsFromList parses a comma-separated UID list, keeping only UIDs that exist
// in the world.
func uidsFromList(w TestWorld, s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		var uid int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &uid); err != nil {
			continue
		}
		if _, ok := w.Players[uid]; ok {
			out = append(out, uid)
		}
	}
	return out
}

// resolveRandomInActions returns a copy of actions with every "random" node
// replaced by one of its branches, chosen by choice(len(choices)). The choice
// function is injected so the decision is explicit: tests pass a fixed choice,
// the live layer passes rand.Intn. Recurses into nested random branches.
func resolveRandomInActions(actions []CustomAction, choice func(int) int) []CustomAction {
	out := make([]CustomAction, 0, len(actions))
	for _, a := range actions {
		if a.Type == ActionRandom {
			if len(a.Choices) == 0 {
				continue
			}
			idx := choice(len(a.Choices))
			if idx < 0 || idx >= len(a.Choices) {
				idx = 0
			}
			out = append(out, resolveRandomInActions(a.Choices[idx].Actions, choice)...)
			continue
		}
		if len(a.Choices) > 0 {
			// A non-random node carrying choices (defensive) — recurse them too
			// so a stray nested random never survives into the pure core.
			a.Choices = nil
		}
		out = append(out, a)
	}
	return out
}

// parseDuration parses a human duration string, falling back to def on empty
// or invalid input. Supports the same formats as str2duration ("10m", "1h30m").
func parseDuration(s string, def time.Duration) time.Duration {
	if strings.TrimSpace(s) == "" {
		return def
	}
	d, err := str2duration.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return d
}

func addTestPunishment(p *TestPlayer, t PunishmentType, d time.Duration, reason string, now time.Time) {
	for i := range p.Punishments {
		if p.Punishments[i].Type == t {
			p.Punishments = append(p.Punishments[:i], p.Punishments[i+1:]...)
			break
		}
	}
	exp := time.Time{}
	if d > 0 {
		exp = now.Add(d)
	}
	p.Punishments = append(p.Punishments, TestPunishment{Type: t, ExpiresAt: exp, Reason: reason})
}

func removeTestPunishment(p *TestPlayer, t PunishmentType) {
	for i := range p.Punishments {
		if p.Punishments[i].Type == t {
			p.Punishments = append(p.Punishments[:i], p.Punishments[i+1:]...)
			return
		}
	}
}

// executeCustomCommandPure evaluates an already-random-resolved command against
// world and returns the full output state. It is pure: no globals, no clock,
// no randomness, no I/O — same inputs always produce the same Result.
func executeCustomCommandPure(world TestWorld, cmd CustomCommand, args []string, now time.Time) Result {
	world = cloneWorld(world)
	caller := world.Players[world.CallerUID]
	if caller == nil {
		return Result{Decision: DecisionRejectedValidation, World: world}
	}
	if len(args) < cmd.MinArgs {
		return Result{Decision: DecisionRejectedBadArgs, World: world}
	}

	var effects []Effect
	for _, a := range cmd.Actions {
		effects = runAction(&world, a, args, now, caller, effects)
	}
	return Result{Decision: DecisionAccepted, Effects: effects, World: world}
}

// runAction applies one action to the world, appending the resulting effects.
// It mutates *w in place (w is already a private copy from executeCustomCommandPure).
func runAction(w *TestWorld, a CustomAction, args []string, now time.Time, caller *TestPlayer, effects []Effect) []Effect {
	base := renderContext{args: args, callerUID: caller.UID, callerName: caller.Name}
	switch a.Type {
	case ActionMessage:
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			tp := w.Players[uid]
			if tp == nil {
				continue
			}
			ctx := base
			ctx.targetUID, ctx.targetName = tp.UID, tp.Name
			effects = append(effects, Effect{Kind: EffectMessage, To: []int{uid}, Text: renderTemplate(a.Text, ctx)})
		}

	case ActionAnnounce:
		effects = append(effects, Effect{Kind: EffectAnnounce, Text: renderTemplate(a.Text, base)})

	case ActionPunish:
		if w.Config.PunishmentsDisabled {
			return effects
		}
		pType, ok := punishmentTypeByName(a.Effect)
		if !ok {
			return effects
		}
		d := parseDuration(a.Duration, 10*time.Minute)
		if d > 24*time.Hour {
			d = 24 * time.Hour
		}
		reason := renderTemplate(a.Reason, base)
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			tp := w.Players[uid]
			if tp == nil {
				continue
			}
			if ar, ok := areaByName(*w, tp.AreaName); ok && ar.PunishmentSafe {
				continue
			}
			addTestPunishment(tp, pType, d, reason, now)
			effects = append(effects, Effect{Kind: EffectPunishApplied, To: []int{uid}, Punishment: pType, Duration: d, Reason: reason})
		}

	case ActionUnpunish:
		removeAll := strings.EqualFold(a.Effect, "all")
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			tp := w.Players[uid]
			if tp == nil {
				continue
			}
			if removeAll {
				tp.Punishments = nil
				effects = append(effects, Effect{Kind: EffectPunishRemoved, To: []int{uid}, Punishment: PunishmentNone})
				continue
			}
			pType, ok := punishmentTypeByName(a.Effect)
			if !ok {
				continue
			}
			removeTestPunishment(tp, pType)
			effects = append(effects, Effect{Kind: EffectPunishRemoved, To: []int{uid}, Punishment: pType})
		}

	case ActionKick:
		reason := renderTemplate(a.Reason, base)
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			effects = append(effects, Effect{Kind: EffectKick, To: []int{uid}, Reason: reason})
			delete(w.Players, uid)
		}

	case ActionBan:
		d := parseDuration(a.Duration, 24*time.Hour)
		reason := renderTemplate(a.Reason, base)
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			effects = append(effects, Effect{Kind: EffectBan, To: []int{uid}, Duration: d, Reason: reason})
			delete(w.Players, uid)
		}

	case ActionMute:
		scope := parseMuteScope(a.Scope)
		d := parseDuration(a.Duration, 0)
		reason := renderTemplate(a.Reason, base)
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			tp := w.Players[uid]
			if tp == nil {
				continue
			}
			tp.Muted = scope
			effects = append(effects, Effect{Kind: EffectMute, To: []int{uid}, Scope: scope, Duration: d, Reason: reason})
		}

	case ActionUnmute:
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			tp := w.Players[uid]
			if tp == nil {
				continue
			}
			tp.Muted = Unmuted
			effects = append(effects, Effect{Kind: EffectUnmute, To: []int{uid}})
		}

	case ActionMove:
		if _, ok := areaByName(*w, a.Area); !ok {
			return effects
		}
		for _, uid := range resolveTargetUIDs(*w, a.Target, caller, args) {
			tp := w.Players[uid]
			if tp == nil {
				continue
			}
			tp.AreaName = a.Area
			effects = append(effects, Effect{Kind: EffectMove, To: []int{uid}, Area: a.Area})
		}

	case ActionRun:
		runArgs := make([]string, len(a.Args))
		for i, s := range a.Args {
			runArgs[i] = renderTemplate(s, base)
		}
		effects = append(effects, Effect{Kind: EffectRun, Command: a.Command, Args: runArgs})

	case ActionWait:
		effects = append(effects, Effect{Kind: EffectWait, Duration: parseDuration(a.Duration, 0)})

	case ActionGrant, ActionRevoke:
		account := renderTemplate(a.Target, base)
		kind := EffectGrant
		if a.Type == ActionRevoke {
			kind = EffectRevoke
		}
		effects = append(effects, Effect{Kind: kind, Account: account, Command: a.Command})
	}
	return effects
}

// ── Live binding (impure shell) ──────────────────────────────────────────────

// buildWorldFromLive derives a TestWorld from the live server state so the pure
// executor runs against exactly what a real player would experience.
func buildWorldFromLive(caller *Client) TestWorld {
	banLen := ""
	if config != nil {
		banLen = config.BanLen
	}
	w := TestWorld{
		Config: TestConfig{
			EnableCasino:        config != nil && config.EnableCasino,
			EnableAccounts:      config != nil && config.EnableAccounts,
			EnableVoice:         config != nil && config.EnableVoice,
			PunishmentsDisabled: punishmentsGloballyDisabled.Load(),
			BanLength:           banLen,
		},
		Players:   make(map[int]*TestPlayer),
		CallerUID: caller.Uid(),
	}
	for _, a := range areas {
		w.Areas = append(w.Areas, TestArea{
			Name:           a.Name(),
			Locked:         a.Lock() == area.LockLocked,
			AdminLocked:    a.AdminLocked(),
			PunishmentSafe: a.PunishmentSafe(),
		})
	}
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 {
			return
		}
		tp := &TestPlayer{
			UID:           c.Uid(),
			IPID:          c.Ipid(),
			Name:          clientDisplayName(c),
			Perms:         c.Perms(),
			Muted:         c.Muted(),
			Authenticated: c.Authenticated(),
		}
		if c.Area() != nil {
			tp.AreaName = c.Area().Name()
		}
		if tp.Authenticated {
			tp.Account = c.ModName()
		}
		for _, p := range c.Punishments() {
			tp.Punishments = append(tp.Punishments, TestPunishment{Type: p.punishmentType, ExpiresAt: p.expiresAt, Reason: p.reason})
		}
		w.Players[tp.UID] = tp
	})
	return w
}

// findAreaByNameLive returns the live area matching name, or nil.
func findAreaByNameLive(name string) *area.Area {
	for _, a := range areas {
		if strings.EqualFold(a.Name(), name) {
			return a
		}
	}
	return nil
}

// executeCustomCommandLive runs a custom command against the live server: it
// derives a world, resolves random branches with the real RNG, evaluates the
// pure core, and applies the resulting effects.
func executeCustomCommandLive(caller *Client, cmd CustomCommand, args []string) Result {
	cmd = cmd.resolved()
	cmd.Actions = resolveRandomInActions(cmd.Actions, rand.Intn)
	res := executeCustomCommandPure(buildWorldFromLive(caller), cmd, args, time.Now())
	if res.Decision == DecisionAccepted {
		applyEffectsLive(caller, res.Effects)
	}
	return res
}

// applyEffectsLive applies effects in order. A wait effect schedules the
// remaining effects on a goroutine, cancelled if the caller disconnects first.
func applyEffectsLive(caller *Client, effects []Effect) {
	for i, e := range effects {
		if e.Kind == EffectWait {
			rest := effects[i+1:]
			if e.Duration > 0 {
				go func() {
					t := time.NewTimer(e.Duration)
					defer t.Stop()
					select {
					case <-t.C:
						applyEffectsLive(caller, rest)
					case <-caller.done:
					}
				}()
			} else {
				go applyEffectsLive(caller, rest)
			}
			return
		}
		applyEffectLive(caller, e)
	}
}

// applyEffectLive applies one concrete effect to the live server.
func applyEffectLive(caller *Client, e Effect) {
	switch e.Kind {
	case EffectMessage:
		for _, uid := range e.To {
			if c, err := getClientByUid(uid); err == nil {
				c.SendServerMessage(e.Text)
			}
		}

	case EffectAnnounce:
		msg := "[Announcement] " + e.Text
		clients.ForEach(func(c *Client) {
			if c.Uid() != -1 {
				c.SendServerMessage(msg)
			}
		})

	case EffectPunishApplied:
		tier := issuerTierFor(caller)
		for _, uid := range e.To {
			c, err := getClientByUid(uid)
			if err != nil || punishmentSafeBlocked(c) {
				continue
			}
			c.AddPunishmentBy(e.Punishment, e.Duration, e.Reason, tier)
			var expires int64
			if e.Duration > 0 {
				expires = time.Now().UTC().Add(e.Duration).Unix()
			}
			if err := db.UpsertTextPunishmentBy(c.Ipid(), int(e.Punishment), expires, e.Reason, int(tier)); err != nil {
				logger.LogErrorf("custom command: persist punishment: %v", err)
			}
			c.SendServerMessage(fmt.Sprintf("You have been punished with the '%v' effect.", e.Punishment.String()))
		}

	case EffectPunishRemoved:
		for _, uid := range e.To {
			c, err := getClientByUid(uid)
			if err != nil {
				continue
			}
			if e.Punishment == PunishmentNone {
				c.RemoveAllPunishments()
				if err := db.DeleteAllPunishments(c.Ipid()); err != nil {
					logger.LogErrorf("custom command: clear punishments: %v", err)
				}
			} else {
				c.RemovePunishment(e.Punishment)
				if err := db.DeleteTextPunishment(c.Ipid(), int(e.Punishment)); err != nil {
					logger.LogErrorf("custom command: remove punishment: %v", err)
				}
			}
		}

	case EffectKick:
		for _, uid := range e.To {
			c, err := getClientByUid(uid)
			if err != nil {
				continue
			}
			c.SendSync(&packet.KK{Reason: e.Reason})
			c.conn.Close()
		}

	case EffectBan:
		for _, uid := range e.To {
			c, err := getClientByUid(uid)
			if err != nil {
				continue
			}
			d := e.Duration
			if d <= 0 {
				d = 24 * time.Hour
			}
			banTime := time.Now().UTC().Unix()
			until := time.Now().UTC().Add(d).Unix()
			if _, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, until, e.Reason, caller.StoredModName()); err != nil {
				logger.LogErrorf("custom command: ban: %v", err)
			}
			c.SendSync(&packet.KB{Reason: e.Reason})
			c.conn.Close()
		}

	case EffectMute:
		for _, uid := range e.To {
			c, err := getClientByUid(uid)
			if err != nil {
				continue
			}
			c.SetMuted(e.Scope)
			var expires int64
			if e.Duration > 0 {
				c.SetUnmuteTime(time.Now().UTC().Add(e.Duration))
				expires = time.Now().UTC().Add(e.Duration).Unix()
			} else {
				c.SetUnmuteTime(time.Time{})
			}
			if err := db.UpsertMute(c.Ipid(), int(e.Scope), expires); err != nil {
				logger.LogErrorf("custom command: persist mute: %v", err)
			}
			c.SendServerMessage(fmt.Sprintf("You have been muted (%v).", e.Scope.String()))
		}

	case EffectUnmute:
		for _, uid := range e.To {
			c, err := getClientByUid(uid)
			if err != nil {
				continue
			}
			c.SetMuted(Unmuted)
			c.SetUnmuteTime(time.Time{})
			if err := db.DeleteMute(c.Ipid()); err != nil {
				logger.LogErrorf("custom command: unmute: %v", err)
			}
		}

	case EffectMove:
		ar := findAreaByNameLive(e.Area)
		if ar == nil {
			return
		}
		for _, uid := range e.To {
			if c, err := getClientByUid(uid); err == nil {
				c.ChangeArea(ar)
			}
		}

	case EffectRun:
		if builtin, ok := Commands[strings.ToLower(e.Command)]; ok {
			builtin.handler(caller, e.Args, builtin.usage)
		}

	case EffectGrant:
		if _, err := grantCommand(e.Account, e.Command, "customcmd"); err != nil {
			logger.LogErrorf("custom command grant failed: %v", err)
		}

	case EffectRevoke:
		if err := revokeCommand(e.Account, e.Command); err != nil {
			logger.LogErrorf("custom command revoke failed: %v", err)
		}
	}
}
