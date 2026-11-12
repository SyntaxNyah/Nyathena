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
	"math/rand"
	"strings"
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// World events are server-driven bonuses. They are never a gamble: nobody
// pays, loses or risks anything by claiming one.

type worldEventKind struct {
	key      string
	name     string
	aliases  []string
	spawnMsg string // %s is replaced with the area name
}

var worldEventKinds = map[string]worldEventKind{
	"ship": {
		key:     "ship",
		name:    "Wrecked Ship",
		aliases: []string{"ship", "wreck", "wrecked", "wreckage"},
		spawnMsg: "🚢 WRECKED SHIP SPOTTED! A battered vessel has washed ashore in %s.\n" +
			"The first person to claim the wreckage gets the loot!\n" +
			"Type /claim ship or /claimwreck in that area!",
	},
	"satchel": {
		key:     "satchel",
		name:    "Lost Satchel",
		aliases: []string{"satchel", "bag", "lost"},
		spawnMsg: "🎒 A LOST SATCHEL has washed up in %s.\n" +
			"The first person to reach it gets whatever is inside!\n" +
			"Type /claim satchel in that area!",
	},
	"merchant": {
		key:     "merchant",
		name:    "Wandering Merchant",
		aliases: []string{"merchant", "stall", "trader"},
		spawnMsg: "🧺 A WANDERING MERCHANT has set up a tiny stall in %s.\n" +
			"The first person to say hello gets a small gift!\n" +
			"Type /claim merchant in that area!",
	},
	"meteor": {
		key:     "meteor",
		name:    "Meteor Shower",
		aliases: []string{"meteor", "star", "shower"},
		spawnMsg: "☄️ A METEOR SHOWER has left a handful of warm pebbles in %s.\n" +
			"The first person to collect them gets the loot!\n" +
			"Type /claim meteor in that area!",
	},
	"picnic": {
		key:     "picnic",
		name:    "Picnic Basket",
		aliases: []string{"picnic", "basket", "lunch"},
		spawnMsg: "🧺 A PICNIC BASKET has been left unattended in %s.\n" +
			"The first person to claim it gets the goodies!\n" +
			"Type /claim picnic in that area!",
	},
}

type worldEventClaim struct {
	uid  int
	ipid string
	name string
	at   time.Time
	seq  int64
}

type activeWorldEvent struct {
	id        int64
	kind      worldEventKind
	area      *area.Area
	spawnedAt time.Time
	claims    []worldEventClaim
	resolving bool
	timeout   *time.Timer
}

var worldEventState = struct {
	sync.Mutex
	active   *activeWorldEvent
	nextID   int64
	claimSeq int64
}{}

const worldEventClaimWindow = 250 * time.Millisecond

func pebbleEventTimeout() time.Duration {
	minutes := 5
	if config != nil && config.PebbleEventTimeoutMinutes > 0 {
		minutes = config.PebbleEventTimeoutMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func pebbleEventChance() int {
	if config != nil && config.PebbleEventChance > 0 {
		return config.PebbleEventChance
	}
	return 5
}

// sendGlobalEventMessage broadcasts an event alert, skipping players who have
// opted out with /events off.
func sendGlobalEventMessage(message string) {
	out := &aolib.CTToClient{Name: serverName, Message: message, IsFromServer: true}
	clients.ForEach(func(client *Client) {
		if !client.EventAlertsHide() {
			client.Send(out)
		}
	})
}

// eligibleWorldEventAreas returns public, unlocked areas with at least one
// player and Pebble events enabled.
func eligibleWorldEventAreas() []*area.Area {
	var eligible []*area.Area
	for _, a := range areas {
		if a == nil {
			continue
		}
		if a.AdminLocked() || a.Lock() != area.LockFree {
			continue
		}
		if a.PlayerCount() <= 0 || !a.PebbleEvents() {
			continue
		}
		eligible = append(eligible, a)
	}
	return eligible
}

// spawnWorldEvent starts an event in a specific area. Returns the event id, or
// 0 if another event is already active.
func spawnWorldEvent(kindKey string, a *area.Area, timeout time.Duration) int64 {
	kind, ok := worldEventKinds[kindKey]
	if !ok || a == nil {
		return 0
	}
	worldEventState.Lock()
	if worldEventState.active != nil {
		worldEventState.Unlock()
		return 0
	}
	worldEventState.nextID++
	id := worldEventState.nextID
	ev := &activeWorldEvent{
		id:        id,
		kind:      kind,
		area:      a,
		spawnedAt: time.Now(),
	}
	worldEventState.active = ev
	worldEventState.Unlock()

	sendGlobalEventMessage(fmt.Sprintf(kind.spawnMsg, a.Name()))
	logger.LogInfof("Pebble world event %q spawned in %q", kind.name, a.Name())

	ev.timeout = time.AfterFunc(timeout, func() {
		worldEventState.Lock()
		if worldEventState.active != nil && worldEventState.active.id == id {
			areaName := worldEventState.active.area.Name()
			worldEventState.active = nil
			worldEventState.Unlock()
			sendAreaServerMessage(a, fmt.Sprintf("🌊 The %s in %s has drifted away. Maybe next time!",
				kind.name, areaName))
			return
		}
		worldEventState.Unlock()
	})
	return id
}

func spawnRandomWorldEvent(kindKey string) int64 {
	eligible := eligibleWorldEventAreas()
	if len(eligible) == 0 {
		return 0
	}
	return spawnWorldEvent(kindKey, eligible[rand.Intn(len(eligible))], pebbleEventTimeout())
}

// startPebbleWorldEventLoop runs the daily ship roll and the smaller ambient
// events. Each iteration only spawns when no event is already active.
func startPebbleWorldEventLoop() {
	go func() {
		for {
			time.Sleep(randomInterval(23*time.Hour, 25*time.Hour))
			if !pebbleEnabled() {
				continue
			}
			if rand.Intn(pebbleEventChance()) != 0 {
				continue
			}
			spawnRandomWorldEvent("ship")
		}
	}()
	go func() {
		ambient := []string{"satchel", "merchant", "meteor", "picnic"}
		for {
			time.Sleep(randomInterval(4*time.Hour, 8*time.Hour))
			if !pebbleEnabled() {
				continue
			}
			if rand.Intn(3) != 0 {
				continue
			}
			spawnRandomWorldEvent(ambient[rand.Intn(len(ambient))])
		}
	}()
}

// ── Claiming ──────────────────────────────────────────────────────────────────

func cmdClaim(client *Client, args []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	hint := ""
	if len(args) > 0 {
		hint = strings.ToLower(args[0])
	}
	submitWorldEventClaim(client, hint)
}

func cmdClaimWreck(client *Client, _ []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	submitWorldEventClaim(client, "ship")
}

func submitWorldEventClaim(client *Client, hint string) {
	ok, msg := submitWorldEventClaimAt(client, hint, time.Now())
	client.SendServerMessage(msg)
	if !ok {
		return
	}
	// Resolution happens after a short window so the earliest millisecond
	// timestamp wins even when several players claim at once.
	worldEventState.Lock()
	ev := worldEventState.active
	worldEventState.Unlock()
	if ev != nil {
		time.AfterFunc(worldEventClaimWindow, func() { resolveWorldEventClaims(ev.id) })
	}
}

// submitWorldEventClaimAt is the testable core of a claim. It returns whether
// the claim was accepted and a player-facing message.
func submitWorldEventClaimAt(client *Client, hint string, at time.Time) (bool, string) {
	worldEventState.Lock()
	defer worldEventState.Unlock()
	ev := worldEventState.active
	if ev == nil {
		return false, "There is nothing to claim here right now."
	}
	if client.Area() != ev.area {
		return false, fmt.Sprintf("The %s is in %s, not here.", ev.kind.name, ev.area.Name())
	}
	if hint != "" && !worldEventKindMatches(ev.kind, hint) {
		return false, fmt.Sprintf("There is no %s to claim here.", hint)
	}
	worldEventState.claimSeq++
	ev.claims = append(ev.claims, worldEventClaim{
		uid:  client.Uid(),
		ipid: client.Ipid(),
		name: client.OOCName(),
		at:   at,
		seq:  worldEventState.claimSeq,
	})
	if !ev.resolving {
		ev.resolving = true
	}
	return true, fmt.Sprintf("⚓ You reach for the %s...", strings.ToLower(ev.kind.name))
}

func worldEventKindMatches(kind worldEventKind, hint string) bool {
	for _, alias := range kind.aliases {
		if hint == alias {
			return true
		}
	}
	return hint == kind.key || hint == strings.ToLower(kind.name)
}

// resolveWorldEventClaims picks the earliest millisecond timestamp and awards
// the loot exactly once.
func resolveWorldEventClaims(eventID int64) {
	worldEventState.Lock()
	ev := worldEventState.active
	if ev == nil || ev.id != eventID || len(ev.claims) == 0 {
		worldEventState.Unlock()
		return
	}
	winner := ev.claims[0]
	for _, c := range ev.claims[1:] {
		if c.at.Before(winner.at) || (c.at.Equal(winner.at) && c.seq < winner.seq) {
			winner = c
		}
	}
	kind := ev.kind
	areaName := ev.area.Name()
	if ev.timeout != nil {
		ev.timeout.Stop()
	}
	worldEventState.active = nil
	worldEventState.Unlock()

	loot := rollWorldEventLoot(kind.key)
	description := applyWorldEventLoot(winner.ipid, loot)
	if err := db.LogPebbleEventClaim(kind.name, winner.ipid, description); err != nil {
		logger.LogErrorf("worldevent: log failed: %v", err)
	}

	// Message the winner even if they disconnected during the claim window.
	for _, c := range getClientsByIpid(winner.ipid) {
		c.SendServerMessage(fmt.Sprintf("⚓ You salvaged the %s! You found %s!", strings.ToLower(kind.name), description))
	}
	if ev.area != nil {
		sendAreaServerMessage(ev.area, fmt.Sprintf("⚓ %s has claimed the %s in %s!",
			winner.name, strings.ToLower(kind.name), areaName))
	}
}

// ── Loot ──────────────────────────────────────────────────────────────────────

type worldEventLoot struct {
	pebbles int64
	tempTag string
	permTag string
	label   string
}

func rollWorldEventLoot(kind string) worldEventLoot {
	if kind == "ship" {
		roll := rand.Intn(100)
		switch {
		case roll < 55:
			n := int64(10 + rand.Intn(41))
			return worldEventLoot{pebbles: n, label: fmt.Sprintf("%d Pebbles", n)}
		case roll < 75:
			return worldEventLoot{tempTag: "tag_temp_salvager", label: "a [Salvager] tag (24 hours)"}
		case roll < 90:
			it := pebbleShopItems[rand.Intn(len(pebbleShopItems))]
			return worldEventLoot{permTag: it.id, label: fmt.Sprintf("the [%s] tag", it.name)}
		default:
			return worldEventLoot{
				pebbles: 25,
				tempTag: "tag_temp_salvager",
				label:   "25 Pebbles and a [Salvager] tag (24 hours)",
			}
		}
	}
	// Smaller ambient events: always a bonus, never a loss.
	if rand.Intn(100) < 70 {
		n := int64(5 + rand.Intn(11))
		return worldEventLoot{pebbles: n, label: fmt.Sprintf("%d Pebbles", n)}
	}
	tempByKind := map[string]string{
		"satchel":  "tag_temp_castaway",
		"merchant": "tag_temp_trader",
		"meteor":   "tag_temp_stargazer",
		"picnic":   "tag_temp_picnicker",
	}
	if tag, ok := tempByKind[kind]; ok {
		return worldEventLoot{tempTag: tag, label: fmt.Sprintf("a [%s] tag (24 hours)", pebbleTempTagNames[tag])}
	}
	return worldEventLoot{pebbles: 5, label: "5 Pebbles"}
}

func applyWorldEventLoot(ipid string, loot worldEventLoot) string {
	if loot.pebbles > 0 {
		if _, err := db.AddPebbles(ipid, loot.pebbles); err != nil {
			logger.LogErrorf("worldevent: AddPebbles failed for %v: %v", ipid, err)
		}
	}
	if loot.tempTag != "" {
		if err := db.GrantTempTag(ipid, loot.tempTag, time.Now().Add(24*time.Hour)); err != nil {
			logger.LogErrorf("worldevent: GrantTempTag failed for %v: %v", ipid, err)
		}
	}
	if loot.permTag != "" {
		if err := db.GrantShopItem(ipid, loot.permTag); err != nil {
			logger.LogErrorf("worldevent: GrantShopItem failed for %v: %v", ipid, err)
		}
	}
	if loot.label == "" {
		return "a little pocket lint"
	}
	return loot.label
}

// ── /events alert preference ──────────────────────────────────────────────────

func cmdEvents(client *Client, args []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	action := "status"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}
	switch action {
	case "status":
		client.SendServerMessage(fmt.Sprintf(
			"🌍 World-event alerts: %v\nUse /events off to hide them for yourself, or /events on to show them again.",
			boolStr(!client.EventAlertsHide(), "ON", "OFF")))
	case "off", "hide", "mute":
		client.SetEventAlertsHide(true)
		if client.Authenticated() {
			db.SetEventAlertsHide(client.ModName(), true) //nolint:errcheck
		}
		client.SendServerMessage("🔇 World-event alerts hidden. You can still use /claim if you see an event in your area.")
	case "on", "alerts", "show":
		client.SetEventAlertsHide(false)
		if client.Authenticated() {
			db.SetEventAlertsHide(client.ModName(), false) //nolint:errcheck
		}
		client.SendServerMessage("🔔 World-event alerts enabled.")
	default:
		client.SendServerMessage("Usage: /events [status|off|on]")
	}
}

// registerWorldEventCommands installs claim/events commands.
func registerWorldEventCommands() {
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
	RegisterCommand("claim", cmd(cmdClaim, 0,
		"Usage: /claim [ship|satchel|merchant|meteor|picnic]",
		"Claim the active world event in your area."))
	RegisterCommand("claimwreck", cmd(cmdClaimWreck, 0,
		"Usage: /claimwreck",
		"Claim the Wrecked Ship if it is in your area."))
	RegisterCommand("events", cmd(cmdEvents, 0,
		"Usage: /events [status|off|on]",
		"Show, hide or re-enable global world-event alerts for yourself."))
}
