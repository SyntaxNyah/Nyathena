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
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// The duck pond and the other tiny time-wasters deliberately contain no RNG.
// Flavour text is selected with a deterministic hash of the player's IPID and
// the current UTC day, so it feels varied without any chance-based outcome.
// None of these commands change a player's Pebble balance.

func utcDay() string {
	return time.Now().UTC().Format("2006-01-02")
}

// deterministicIndex maps a seed string to an index in [0, n).
func deterministicIndex(seed string, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return int(h.Sum32() % uint32(n))
}

var duckPondFortunes = []string{
	"A quiet afternoon is not wasted time.",
	"Someone is glad you are here, even if they have not said so.",
	"The ducks think your shoes are excellent.",
	"A small kindness today will find its way back to you.",
	"You are allowed to rest before you are finished.",
	"Good things are floating quietly toward you.",
	"The pond does not hurry, and neither should you.",
	"You will make someone laugh this week.",
	"A snack shared is twice as good.",
	"Your patience is a kind of courage.",
	"The best conversations happen sideways.",
	"Today is a good day to notice something small and lovely.",
	"You are doing better than the little voice says.",
	"Somewhere, a duck is rooting for you.",
	"A stranger will remember your kindness longer than you think.",
	"The clouds are putting on a show just for you.",
	"You have good taste in ponds.",
	"Take the long way home sometimes.",
	"Nothing is wrong with a slow afternoon.",
	"Your next idea is closer than it feels.",
	"You make the room a little softer.",
	"The ripples will settle. They always do.",
	"A nap is a legitimate plan.",
	"Someone would like to hear about your day.",
	"You are more than the sum of your to-do list.",
	"Fortune favours the person who feeds the ducks.",
	"Keep a little room in your week for nonsense.",
	"The world is better with you paying attention to it.",
}

var duckPondReactions = []string{
	"The ducks paddle over with tremendous enthusiasm.",
	"A small duck inspects your pebble, then decides it is perfect.",
	"The ducks form a polite semicircle and wait for more.",
	"One duck quacks approvingly. Another agrees.",
	"The pond erupts into dignified duck excitement.",
	"A duck does a tiny spin in the water.",
	"The ducks line up like they are expecting a meeting.",
	"Two ducks race each other to the ripples.",
	"A duck gives you a long, thoughtful look.",
	"The ducks seem to conclude that you are trustworthy.",
}

var duckPondState = struct {
	sync.Mutex
	feeds    map[string]int
	lastFeed map[string]time.Time
	lastNap  map[string]time.Time
}{feeds: make(map[string]int), lastFeed: make(map[string]time.Time), lastNap: make(map[string]time.Time)}

const duckFeedCooldown = 20 * time.Second
const napCooldown = 5 * time.Minute

// duckFortune returns a deterministic fortune for the player, day and feed
// count. Same inputs always produce the same quote.
func duckFortune(ipid string, day string, feedCount int) string {
	seed := ipid + "|" + day + "|" + fmt.Sprintf("%d", feedCount)
	return duckPondFortunes[deterministicIndex(seed, len(duckPondFortunes))]
}

func duckReaction(ipid string, day string, feedCount int) string {
	seed := ipid + "|reaction|" + day + "|" + fmt.Sprintf("%d", feedCount)
	return duckPondReactions[deterministicIndex(seed, len(duckPondReactions))]
}

// cleanupSillyActivityState drops per-player bookkeeping on disconnect so the
// maps stay bounded.
func cleanupSillyActivityState(ipid string) {
	duckPondState.Lock()
	delete(duckPondState.feeds, ipid)
	delete(duckPondState.lastFeed, ipid)
	delete(duckPondState.lastNap, ipid)
	duckPondState.Unlock()
}

// cmdPond handles /pond and /ducks.
func cmdPond(client *Client, args []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	sub := "look"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}
	switch sub {
	case "look", "visit", "":
		client.SendServerMessage(
			"\n🦆 The Duck Pond\n" +
				"A quiet little pond behind the courthouse. Several ducks are floating\n" +
				"around, judging everyone equally.\n\n" +
				"  /pond feed     — toss a pebble and say hello\n" +
				"  /pond fortune  — ask the ducks for a fortune\n\n" +
				"Feeding the ducks costs nothing and earns nothing. That is the point.")
	case "feed":
		feedDucks(client)
	case "fortune", "cookie":
		duckPondState.Lock()
		count := duckPondState.feeds[client.Ipid()]
		duckPondState.Unlock()
		client.SendServerMessage(fmt.Sprintf(
			"🥠 The ducks hand you a fortune cookie:\n\"%s\"",
			duckFortune(client.Ipid(), utcDay(), count)))
	default:
		client.SendServerMessage("Usage: /pond [feed | fortune]")
	}
}

func feedDucks(client *Client) {
	now := time.Now()
	ipid := client.Ipid()
	duckPondState.Lock()
	last := duckPondState.lastFeed[ipid]
	if !last.IsZero() && now.Sub(last) < duckFeedCooldown {
		remaining := duckFeedCooldown - now.Sub(last)
		duckPondState.Unlock()
		client.SendServerMessage(fmt.Sprintf(
			"The ducks are still nibbling. Try again in %d seconds.", int(remaining.Seconds())+1))
		return
	}
	duckPondState.lastFeed[ipid] = now
	duckPondState.feeds[ipid]++
	count := duckPondState.feeds[ipid]
	duckPondState.Unlock()

	client.SendServerMessage(fmt.Sprintf(
		"🦆 You toss a little pebble into the pond.\n%s\n🥠 The ducks hand you a fortune cookie:\n\"%s\"",
		duckReaction(ipid, utcDay(), count),
		duckFortune(ipid, utcDay(), count)))
}

func cmdFortune(client *Client, _ []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	duckPondState.Lock()
	count := duckPondState.feeds[client.Ipid()]
	duckPondState.Unlock()
	client.SendServerMessage(fmt.Sprintf(
		"🥠 A fortune cookie drifts over from the duck pond:\n\"%s\"",
		duckFortune(client.Ipid(), utcDay(), count)))
}

// ── Tiny zero-reward time-wasters ─────────────────────────────────────────────

var cloudShapes = []string{
	"a very long cat",
	"a sleeping dragon",
	"someone else's laundry",
	"a perfect circle, which is suspicious",
	"a courthouse, but fluffier",
	"a duck. The pond is influencing you.",
	"an extremely slow race car",
	"a cloud that looks like it knows something",
}

var constellationNames = []string{
	"The Gavel",
	"The Paperclip",
	"The Sleepy Duck",
	"The Long Hallway",
	"The Coffee Cup",
	"The Unfinished Sentence",
	"The Quiet Bench",
	"The Second Gavel",
}

func cmdCloudwatch(client *Client, _ []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	shape := cloudShapes[deterministicIndex(client.Ipid()+"cloud"+utcDay(), len(cloudShapes))]
	client.SendServerMessage(fmt.Sprintf(
		"☁️ You lie back and watch the clouds. One of them is clearly %s.\n"+
			"You feel a little more settled. No Pebbles, no points — just clouds.", shape))
}

func cmdStargaze(client *Client, _ []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	name := constellationNames[deterministicIndex(client.Ipid()+"star"+utcDay(), len(constellationNames))]
	client.SendServerMessage(fmt.Sprintf(
		"🌙 You find a quiet spot and look up.\n"+
			"Tonight you can just about make out %s.\n"+
			"The stars do not award Pebbles. They are just there.", name))
}

func cmdNap(client *Client, _ []string, _ string) {
	if pebbleModeGuard(client) {
		return
	}
	ipid := client.Ipid()
	now := time.Now()
	duckPondState.Lock()
	last := duckPondState.lastNap[ipid]
	if !last.IsZero() && now.Sub(last) < napCooldown {
		remaining := napCooldown - now.Sub(last)
		duckPondState.Unlock()
		client.SendServerMessage(fmt.Sprintf(
			"You are still pleasantly drowsy. Try again in %d minutes.", int(remaining.Minutes())+1))
		return
	}
	duckPondState.lastNap[ipid] = now
	duckPondState.Unlock()
	client.SendServerMessage("😴 You curl up somewhere warm and take a short nap.\n" +
		"Nothing happens. It is wonderful.")
}

// registerSillyCommands installs the duck pond and the other zero-reward
// time-wasters. Called from registerPebbleCommands.
func registerSillyCommands() {
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
	RegisterCommand("pond", cmd(cmdPond, 0,
		"Usage: /pond [feed | fortune]",
		"Visit the duck pond. Feeding the ducks is free, earns nothing, and sometimes comes with a fortune."))
	RegisterCommand("ducks", cmd(cmdPond, 0,
		"Usage: /ducks [feed | fortune]",
		"Alias of /pond."))
	RegisterCommand("fortune", cmd(cmdFortune, 0,
		"Usage: /fortune",
		"Ask the ducks for a fortune-cookie quote."))
	RegisterCommand("cloudwatch", cmd(cmdCloudwatch, 0,
		"Usage: /cloudwatch",
		"Watch the clouds. No rewards, no stakes, just clouds."))
	RegisterCommand("stargaze", cmd(cmdStargaze, 0,
		"Usage: /stargaze",
		"Look at the stars for a moment."))
	RegisterCommand("nap", cmd(cmdNap, 0,
		"Usage: /nap",
		"Take a short nap. Absolutely nothing happens."))
}
