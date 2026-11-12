/* Athena - A server for Attorney Online 2 written in Go
Copyright (C) 2022 MangosArentLiterature <mango@transmenace.dev>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>. */

package athena

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// pebbleEnabled reports whether the server is currently operating in Pebble
// Mode. Safe when config is nil.
func pebbleEnabled() bool {
	return config != nil && config.PebbleOn()
}

// pebbleCurrencyName returns the currency label used by shared activities.
func pebbleCurrencyName() string {
	if pebbleEnabled() {
		return "Pebbles"
	}
	return "chips"
}

// awardActivityReward credits an activity reward in the active currency and
// returns the new balance plus the currency label. Pebble Mode always wins
// when both economies are enabled, so activity payouts never mint chips in a
// Pebble server.
func awardActivityReward(ipid string, amount int64) (int64, string, error) {
	if pebbleEnabled() {
		bal, err := db.AddPebbles(ipid, amount)
		return bal, "Pebbles", err
	}
	bal, err := db.AddChips(ipid, amount)
	return bal, "chips", err
}

// pebbleModeGuard sends the standard "not enabled" message and reports
// whether the caller should stop.
func pebbleModeGuard(client *Client) bool {
	if pebbleEnabled() {
		return false
	}
	client.SendServerMessage("Pebble Mode is not enabled on this server.")
	return true
}

// shortIPID returns a privacy-preserving display fallback for anonymous
// leaderboard entries.
func shortIPID(ipid string) string {
	if len(ipid) > 8 {
		return ipid[:8] + "…"
	}
	if ipid == "" {
		return "Anonymous"
	}
	return ipid
}

// pebbleDisplayName resolves a linked username, falling back to a shortened
// IPID so anonymous players still appear on leaderboards.
func pebbleDisplayName(name, ipid string) string {
	if name != "" {
		return name
	}
	return shortIPID(ipid)
}

// pebbleWelcomeMsg is sent once on join when Pebble Mode is on.
const pebbleWelcomeMsg = "🪨 Pebble Mode is on! /pebble for your balance, /minigame for casual games,\n" +
	"   /daily /explore /cast for a few Pebbles, and /pond to feed the ducks.\n" +
	"   No stakes, no grinding — just things to do."

// ── Daily / one-off activities ────────────────────────────────────────────────

// pebbleActivity describes a low-friction, cooldown-gated Pebble activity.
// Rewards are deliberately fixed: no RNG decides whether or how much a player
// earns.
type pebbleActivity struct {
	key      string
	reward   int64
	cooldown int64 // seconds
	emoji    string
}

var pebbleActivities = []pebbleActivity{
	{key: "daily", reward: 3, cooldown: 20 * 60 * 60, emoji: "🌅"},
	{key: "explore", reward: 2, cooldown: 2 * 60 * 60, emoji: "🧭"},
	{key: "cast", reward: 1, cooldown: 60 * 60, emoji: "🎣"},
}

func pebbleActivityByKey(key string) (pebbleActivity, bool) {
	for _, a := range pebbleActivities {
		if a.key == key {
			return a, true
		}
	}
	return pebbleActivity{}, false
}

// pebbleActivityFlavour returns deterministic flavour text for an activity.
// It uses the caller's IPID and the current UTC day, so a player sees a stable
// result for the day without any random roll.
func pebbleActivityFlavour(ipid, key string) string {
	switch key {
	case "daily":
		lines := []string{
			"You took a slow walk around the courthouse and said hello to everyone.",
			"You watered the little plants by the lobby and felt useful.",
			"You found a sunny bench and watched the clouds for a while.",
		}
		return lines[deterministicIndex(ipid+"daily"+utcDay(), len(lines))]
	case "explore":
		lines := []string{
			"You explored the gardens behind the courthouse.",
			"You followed a narrow hallway and found a quiet reading nook.",
			"You wandered down to the riverbank and skipped a stone.",
			"You checked on the old clock tower and listened to it tick.",
		}
		return lines[deterministicIndex(ipid+"explore"+utcDay(), len(lines))]
	case "cast":
		lines := []string{
			"You cast a line and watched the ripples spread.",
			"You sat by the water and let the afternoon pass.",
			"You practised your casting and nearly caught a leaf.",
			"You shared your snack with a very bold duck.",
		}
		return lines[deterministicIndex(ipid+"cast"+utcDay(), len(lines))]
	}
	return "You did something pleasant for a little while."
}

// doPebbleActivity handles /daily, /explore and /cast.
func doPebbleActivity(client *Client, key string) {
	if pebbleModeGuard(client) {
		return
	}
	act, ok := pebbleActivityByKey(key)
	if !ok {
		client.SendServerMessage("Unknown activity.")
		return
	}
	onCooldown, remaining, err := db.CheckAndSetJobCooldown(
		client.Ipid(), "pebble_"+act.key, act.cooldown)
	if err != nil {
		client.SendServerMessage("Something went wrong. Please try again later.")
		return
	}
	if onCooldown {
		client.SendServerMessage(fmt.Sprintf(
			"%s You've already done that recently. Try again in %s.",
			act.emoji, formatCooldownRemaining(remaining)))
		return
	}
	newBal, err := db.AddPebbles(client.Ipid(), act.reward)
	if err != nil {
		client.SendServerMessage("Something went wrong awarding Pebbles. Please try again later.")
		return
	}
	client.SendServerMessage(fmt.Sprintf(
		"%s %s\n+%d Pebbles | Balance: %d Pebbles",
		act.emoji, pebbleActivityFlavour(client.Ipid(), act.key), act.reward, newBal))
}

// formatCooldownRemaining renders a seconds count as "Xh Ym" / "Ym".
func formatCooldownRemaining(seconds int64) string {
	if seconds <= 0 {
		return "a moment"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", seconds)
}

// ── /pebble ───────────────────────────────────────────────────────────────────

func cmdPebble(client *Client, args []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	if len(args) == 0 {
		bal, err := db.GetPebbleBalance(client.Ipid())
		if err != nil {
			client.SendServerMessage("Could not retrieve your Pebble balance.")
			return
		}
		client.SendServerMessage(fmt.Sprintf(
			"🪨 Your Pebble balance: %d\n"+
				"  /pebble top [n]   — global Pebble leaderboard\n"+
				"  /pebble area [n]  — Pebble leaderboard for this area\n"+
				"  /daily · /explore · /cast — a few Pebbles on long cooldowns\n"+
				"  /minigame · /pond · /shop — things to do, no stakes",
			bal))
		return
	}

	switch strings.ToLower(args[0]) {
	case "top":
		pebbleTopGlobal(client, args[1:])
	case "area":
		pebbleTopArea(client, args[1:])
	case "daily":
		doPebbleActivity(client, "daily")
	case "explore":
		doPebbleActivity(client, "explore")
	case "fish", "cast":
		doPebbleActivity(client, "cast")
	case "help":
		printPebbleHelp(client)
	default:
		client.SendServerMessage("Usage: /pebble [top [n] | area [n] | daily | explore | cast | help]")
	}
}

func parseLeaderboardN(client *Client, args []string, usage string) (int, bool) {
	n := 10
	if len(args) > 0 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v <= 0 || v > 50 {
			client.SendServerMessage(usage)
			return 0, false
		}
		n = v
	}
	return n, true
}

func pebbleTopGlobal(client *Client, args []string) {
	n, ok := parseLeaderboardN(client, args, "Usage: /pebble top [n]  (1-50, default 10)")
	if !ok {
		return
	}
	entries, err := db.GetTopPebbleBalances(n)
	if err != nil || len(entries) == 0 {
		client.SendServerMessage("No Pebble data available yet.")
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🪨 Global Pebble Leaderboard (Top %d)\n", len(entries)))
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %2d. %v — %d Pebbles\n", i+1,
			pebbleDisplayName(e.Username, e.IPID), e.Balance))
	}
	client.SendServerMessage(sb.String())
}

func pebbleTopArea(client *Client, args []string) {
	n, ok := parseLeaderboardN(client, args, "Usage: /pebble area [n]  (1-50, default 10)")
	if !ok {
		return
	}
	type entry struct {
		name    string
		balance int64
	}
	myArea := client.Area()
	var ipids []string
	var inArea []*Client
	clients.ForEach(func(c *Client) {
		if c.Area() != myArea || c.Uid() == -1 {
			return
		}
		ipids = append(ipids, c.Ipid())
		inArea = append(inArea, c)
	})
	if len(inArea) == 0 {
		client.SendServerMessage("No players in this area.")
		return
	}
	names, _ := db.GetUsernamesByIPIDs(ipids)
	balances, _ := db.GetPebbleBalancesByIPIDs(ipids)
	entries := make([]entry, 0, len(inArea))
	for _, c := range inArea {
		name := c.OOCName()
		if u := names[c.Ipid()]; u != "" {
			name = u
		}
		entries = append(entries, entry{name: name, balance: balances[c.Ipid()]})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].balance > entries[j].balance })
	if len(entries) > n {
		entries = entries[:n]
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🪨 Pebble Leaderboard — %v (Top %d)\n", myArea.Name(), len(entries)))
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %2d. %v — %d Pebbles\n", i+1, e.name, e.balance))
	}
	client.SendServerMessage(sb.String())
}

func printPebbleHelp(client *Client) {
	client.SendServerMessage(
		"\n🪨 Pebble Mode\n" +
			"  /pebble · /pebble top [n] · /pebble area [n]\n" +
			"  /daily +3 Pebbles (20h) · /explore +2 (2h) · /cast +1 (1h)\n" +
			"  /minigame slots|blackjack|crash — no stakes, no Pebble rewards\n" +
			"  /pond · /ducks · /fortune — feed the ducks, read a fortune\n" +
			"  /shop · /shop classic (free tags) · /settag · /mytags\n" +
			"  /claim ship · /claimwreck — claim world-event loot\n" +
			"  /events off — hide world-event alerts for yourself\n\n" +
			"Pebbles are earned only through optional activities and world events.\n" +
			"No playtime grinding. No trading. No stakes.")
}

// ── Staff commands ────────────────────────────────────────────────────────────

func cmdGrantPebbles(client *Client, args []string, usage string) {
	if len(args) < 2 {
		client.SendServerMessage(usage)
		return
	}
	targetUID, err := strconv.Atoi(args[0])
	if err != nil || targetUID < 0 {
		client.SendServerMessage("Invalid UID.")
		return
	}
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		client.SendServerMessage("Amount must be a positive integer.")
		return
	}
	target := clients.GetClientByUID(targetUID)
	if target == nil {
		client.SendServerMessage("Player not found.")
		return
	}
	newBal, err := db.AddPebbles(target.Ipid(), amount)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to grant Pebbles: %v", err))
		return
	}
	client.SendServerMessage(fmt.Sprintf("Granted %d Pebbles to %v. Their new balance: %d Pebbles.",
		amount, target.OOCName(), newBal))
	target.SendServerMessage(fmt.Sprintf("🪨 An admin granted you %d Pebbles! Balance: %d Pebbles.",
		amount, newBal))
	addToBuffer(client, "CMD", fmt.Sprintf("Granted %d Pebbles to UID %d (%v).",
		amount, targetUID, target.OOCName()), false)
}

func cmdPebbleSet(client *Client, args []string, _ string) {
	const usage = "Usage: /pebbleset <minigames|events|status> <true|false>"
	if len(args) == 0 {
		client.SendServerMessage(usage)
		return
	}
	a := client.Area()
	switch strings.ToLower(args[0]) {
	case "minigames":
		if len(args) < 2 {
			client.SendServerMessage(usage)
			return
		}
		v, err := strconv.ParseBool(args[1])
		if err != nil {
			client.SendServerMessage("minigames must be true or false.")
			return
		}
		a.SetPebbleMinigames(v)
		client.SendServerMessage(fmt.Sprintf("Pebble mini-games in %v: %v.", a.Name(), boolStr(v, "enabled", "disabled")))
	case "events":
		if len(args) < 2 {
			client.SendServerMessage(usage)
			return
		}
		v, err := strconv.ParseBool(args[1])
		if err != nil {
			client.SendServerMessage("events must be true or false.")
			return
		}
		a.SetPebbleEvents(v)
		client.SendServerMessage(fmt.Sprintf("Pebble world events in %v: %v.", a.Name(), boolStr(v, "enabled", "disabled")))
	case "status":
		client.SendServerMessage(fmt.Sprintf(
			"🪨 Pebble settings for %v\n  Mini-games: %v\n  World events: %v\n  Pebble Mode (server): %v",
			a.Name(), boolStr(a.PebbleMinigames(), "enabled", "disabled"),
			boolStr(a.PebbleEvents(), "enabled", "disabled"),
			boolStr(pebbleEnabled(), "enabled", "disabled")))
	default:
		client.SendServerMessage(usage)
	}
}

// registerPebbleCommands installs every Pebble/casual command. Called from
// initCommands after the base registry literal is built.
func registerPebbleCommands() {
	cmd := func(handler func(*Client, []string, string), minArgs int, usage, desc string) Command {
		return Command{
			handler:   handler,
			minArgs:   minArgs,
			usage:     usage,
			desc:      desc,
			reqPerms:  permissions.PermissionField["NONE"],
			pebbleCmd: true,
			category:  "pebble",
		}
	}

	RegisterCommand("pebble", cmd(cmdPebble, 0,
		"Usage: /pebble [top [n] | area [n] | daily | explore | cast | help]",
		"Show your Pebble balance, /pebble top [n], or /pebble area [n]."))
	RegisterCommand("pebbles", cmd(cmdPebble, 0,
		"Usage: /pebbles [top [n] | area [n] | daily | explore | cast | help]",
		"Alias of /pebble."))
	RegisterCommand("daily", cmd(func(c *Client, _ []string, _ string) { doPebbleActivity(c, "daily") }, 0,
		"Usage: /daily", "A once-a-day +3 Pebble activity (20-hour cooldown)."))
	RegisterCommand("explore", cmd(func(c *Client, _ []string, _ string) { doPebbleActivity(c, "explore") }, 0,
		"Usage: /explore", "Explore the area for +2 Pebbles (2-hour cooldown)."))
	RegisterCommand("cast", cmd(func(c *Client, _ []string, _ string) { doPebbleActivity(c, "cast") }, 0,
		"Usage: /cast", "Cast a line by the water for +1 Pebble (1-hour cooldown)."))
	RegisterCommand("grantpebbles", Command{
		handler:   cmdGrantPebbles,
		minArgs:   2,
		usage:     "Usage: /grantpebbles <uid> <amount>",
		desc:      "Admin: grant Pebbles to an online player by UID.",
		reqPerms:  permissions.PermissionField["ADMIN"],
		pebbleCmd: true,
		category:  "admin",
	})
	RegisterCommand("pebbleset", Command{
		handler:   cmdPebbleSet,
		minArgs:   1,
		usage:     "Usage: /pebbleset <minigames|events|status> <true|false>",
		desc:      "Configure Pebble mini-games and world events for this area.",
		reqPerms:  permissions.PermissionField["MODIFY_AREA"],
		pebbleCmd: true,
		category:  "pebble",
	})
	registerSillyCommands()
	registerMiniGameCommands()
	registerWorldEventCommands()
}

// renderPebbleHelp builds the /help pebble listing. It deliberately pulls in
// every command that participates in Pebble Mode (including the shop, tags,
// jobs and unscramble) rather than only commands whose category happens to be
// "pebble". Permission and mode filters are applied per command, so regulars
// see the full player-facing set while /pebbleset and /grantpebbles remain
// hidden from them.
func renderPebbleHelp(client *Client, casinoOn, pebbleOn, accountsOn, voiceOn bool) string {
	if !pebbleOn {
		return "Pebble Mode is not enabled on this server."
	}
	groups := []struct {
		title string
		names []string
	}{
		{"Balance and leaderboards", []string{"pebble", "pebbles"}},
		{"Earn Pebbles", []string{"daily", "explore", "cast", "jobs", "busker", "janitor", "paperboy", "clerk", "bailiffjob", "jobtop", "unscramble"}},
		{"Casual mini-games (no stakes, no Pebble rewards)", []string{"minigame"}},
		{"Duck pond and quiet things", []string{"pond", "ducks", "fortune", "cloudwatch", "stargaze", "nap"}},
		{"Shop and tags", []string{"shop", "settag", "mytags"}},
		{"World events", []string{"claim", "claimwreck", "events"}},
		{"Staff", []string{"pebbleset", "grantpebbles"}},
	}

	var sb strings.Builder
	sb.WriteString("\n🪨 Pebble Mode Commands\n")
	sb.WriteString("No stakes, no trading, no playtime grinding.\n")
	any := false
	for _, group := range groups {
		var lines []string
		for _, name := range group.names {
			cmd, ok := Commands[name]
			if !ok {
				continue
			}
			if !commandModeAllowed(cmd, casinoOn, pebbleOn, accountsOn, voiceOn) {
				continue
			}
			if !clientCanUseCommand(client, name, cmd) && !cmd.publicHelp {
				continue
			}
			lines = append(lines, fmt.Sprintf("  /%s — %s", name, cmd.desc))
		}
		if len(lines) == 0 {
			continue
		}
		any = true
		sb.WriteString("\n" + group.title + ":\n")
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n")
	}
	if !any {
		return "No Pebble commands are available to you right now."
	}
	sb.WriteString("\nFree classic tags: /shop classic\n")
	sb.WriteString("Cheap Pebble tags: /shop activity | /shop community\n")
	sb.WriteString("Detailed usage for any command: /<command> -h")
	return sb.String()
}
