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
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	deathrollDefaultLimit       = 100
	deathrollMinLimit           = 2
	deathrollMaxLimit           = 1_000_000
	deathrollMinPlayers         = 2
	deathrollPunishmentDuration = 10 * time.Minute
	deathrollLockDuration       = 10 * time.Minute
	deathrollMaxListedPlayers   = 30
)

// deathrollUsage is shown by /deathroll with no arguments, /help deathroll and -h.
const deathrollUsage = "Usage: /deathroll <host [limit] | join | start | roll | cancel | status>\n" +
	"  /deathroll host [limit]  — open a lobby (default limit 100; e.g. /deathroll host 5000)\n" +
	"  /deathroll join          — join the open lobby\n" +
	"  /deathroll start         — host only: shuffle players and begin (2+ needed)\n" +
	"  /deathroll roll          — current player: roll 1d<current limit>\n" +
	"  /deathroll cancel        — host only: cancel the open lobby\n" +
	"  /deathroll status        — show the current lobby/game"

// ── State ─────────────────────────────────────────────────────────────────────

// deathrollPhase is the lifecycle state of the single global lobby.
type deathrollPhase int

const (
	deathrollOpen deathrollPhase = iota
	deathrollInProgress
)

// deathrollPlayer snapshots a player's UID and OOC name at join time. Names are
// cached so embeds never need getClientByUid while deathrollMu is held.
type deathrollPlayer struct {
	uid  int
	name string
}

// deathrollLobby is the complete state of the current game. Every field is
// guarded by deathrollMu; helpers whose names end in Locked require it held.
// No network IO (including getClientByUid) may run while it is held.
type deathrollLobby struct {
	hostUID       int
	hostName      string
	startingLimit int
	upperBound    int
	players       []deathrollPlayer // join order, used while open
	order         []deathrollPlayer // shuffled at start, used in progress
	turn          int
	forfeited     map[int]bool
	phase         deathrollPhase
	lastRoll      int
	lastRoller    string
	lastBound     int
}

var (
	deathrollMu      sync.Mutex
	deathrollCurrent *deathrollLobby
)

// deathrollRollDie is a test seam, mirroring pickAreaRandomPunishment. It is
// never reassigned outside tests.
var deathrollRollDie = func(sides int) int {
	if sides < 1 {
		sides = 1
	}
	return rand.Intn(sides) + 1
}

// ── Entry point ───────────────────────────────────────────────────────────────

// cmdDeathroll dispatches /deathroll <subcommand>.
func cmdDeathroll(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage(deathrollUsage)
		return
	}
	switch strings.ToLower(args[0]) {
	case "host":
		deathrollHost(client, args[1:], usage)
	case "join":
		deathrollJoin(client)
	case "start":
		deathrollStart(client)
	case "roll":
		deathrollRoll(client)
	case "cancel":
		deathrollCancel(client)
	case "status":
		deathrollStatus(client)
	case "help", "?":
		client.SendServerMessage(deathrollUsage)
	default:
		client.SendServerMessage(fmt.Sprintf("Unknown /deathroll subcommand '%v'.\n%v", args[0], deathrollUsage))
	}
}

// ── Parsing / formatting helpers ──────────────────────────────────────────────

// deathrollParseLimit accepts /deathroll host, /deathroll host 5000,
// /deathroll host limit 5000, and /deathroll host -l 5000.
func deathrollParseLimit(args []string) (int, error) {
	if len(args) == 0 {
		return deathrollDefaultLimit, nil
	}
	if len(args) == 2 && (strings.EqualFold(args[0], "limit") || args[0] == "-l" || args[0] == "-limit") {
		args = args[1:]
	} else if len(args) > 1 {
		return 0, fmt.Errorf("Too many arguments.")
	}

	limit, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil {
		return 0, fmt.Errorf("The starting limit must be a whole number (e.g. /deathroll host 5000).")
	}
	if limit < deathrollMinLimit {
		return 0, fmt.Errorf("The starting limit must be at least %d.", deathrollMinLimit)
	}
	if limit > deathrollMaxLimit {
		return 0, fmt.Errorf("The starting limit is too large (maximum %d).", deathrollMaxLimit)
	}
	return limit, nil
}

// deathrollName returns the OOC name to show in announcements, falling back to
// UID n for clients that somehow have neither an OOC name nor a showname.
func deathrollName(client *Client) string {
	name := strings.TrimSpace(oocDisplayName(client))
	if name == "" {
		name = fmt.Sprintf("UID %d", client.Uid())
	}
	return name
}

// deathrollPlayerList formats the joined-player roster, capping very long lists
// so one packet stays reasonable.
func deathrollPlayerList(players []deathrollPlayer) string {
	if len(players) == 0 {
		return "(none)"
	}
	shown := players
	extra := 0
	if len(shown) > deathrollMaxListedPlayers {
		extra = len(shown) - deathrollMaxListedPlayers
		shown = shown[:deathrollMaxListedPlayers]
	}
	names := make([]string, 0, len(shown)+1)
	for _, p := range shown {
		names = append(names, p.name)
	}
	if extra > 0 {
		names = append(names, fmt.Sprintf("and %d more", extra))
	}
	return strings.Join(names, ", ")
}

func deathrollOrderString(players []deathrollPlayer) string {
	names := make([]string, 0, len(players))
	for _, p := range players {
		names = append(names, p.name)
	}
	return strings.Join(names, " → ")
}

func deathrollPlayerIndex(players []deathrollPlayer, uid int) int {
	for i, p := range players {
		if p.uid == uid {
			return i
		}
	}
	return -1
}

// ── State helpers (deathrollMu held) ──────────────────────────────────────────

// deathrollActivePlayersLocked returns the players still in the game, in turn
// order, excluding anyone who forfeited.
func deathrollActivePlayersLocked(st *deathrollLobby) []deathrollPlayer {
	active := make([]deathrollPlayer, 0, len(st.order))
	for _, p := range st.order {
		if !st.forfeited[p.uid] {
			active = append(active, p)
		}
	}
	return active
}

// deathrollAdvanceTurnLocked moves st.turn to the next non-forfeited player.
// Returns false when every entry is forfeited.
func deathrollAdvanceTurnLocked(st *deathrollLobby) bool {
	if len(st.order) == 0 {
		return false
	}
	for i := 0; i < len(st.order); i++ {
		st.turn = (st.turn + 1) % len(st.order)
		if !st.forfeited[st.order[st.turn].uid] {
			return true
		}
	}
	return false
}

// ── Embeds / announcements ────────────────────────────────────────────────────

// deathrollLobbyEmbedLocked builds the lobby "embed". AO2 has no message
// editing, so callers post this fresh whenever the roster changes.
func deathrollLobbyEmbedLocked(st *deathrollLobby) string {
	var b strings.Builder
	b.WriteString("🎲 DEATHROLL LOBBY\n")
	b.WriteString("────────────────────────────\n")
	b.WriteString(fmt.Sprintf("Host: %s\n", st.hostName))
	b.WriteString(fmt.Sprintf("Starting limit: 1d%d\n", st.startingLimit))
	b.WriteString(fmt.Sprintf("Players (%d): %s\n", len(st.players), deathrollPlayerList(st.players)))
	b.WriteString("Status: OPEN — waiting for the host to start\n")
	b.WriteString("────────────────────────────\n")
	b.WriteString("Join: /deathroll join\n")
	b.WriteString("Start (host): /deathroll start\n")
	b.WriteString("Cancel (host): /deathroll cancel")
	return b.String()
}

// deathrollGameEmbedLocked builds the in-progress board.
func deathrollGameEmbedLocked(st *deathrollLobby) string {
	var b strings.Builder
	b.WriteString("🎲 DEATHROLL — IN PROGRESS\n")
	b.WriteString("────────────────────────────\n")
	b.WriteString(fmt.Sprintf("Turn order: %s\n", deathrollOrderString(st.order)))
	b.WriteString(fmt.Sprintf("Current limit: 1d%d\n", st.upperBound))
	current := "(unknown)"
	if len(st.order) > 0 && st.turn >= 0 && st.turn < len(st.order) {
		current = st.order[st.turn].name
	}
	b.WriteString(fmt.Sprintf("Current turn: %s\n", current))
	if st.lastRoller == "" {
		b.WriteString("Last roll: —\n")
	} else {
		b.WriteString(fmt.Sprintf("Last roll: %s rolled %d (out of %d)\n", st.lastRoller, st.lastRoll, st.lastBound))
	}
	var forfeited []string
	for _, p := range st.order {
		if st.forfeited[p.uid] {
			forfeited = append(forfeited, p.name)
		}
	}
	if len(forfeited) > 0 {
		b.WriteString(fmt.Sprintf("Forfeited: %s\n", strings.Join(forfeited, ", ")))
	}
	b.WriteString("────────────────────────────\n")
	b.WriteString("Current player: /deathroll roll")
	return b.String()
}

// deathrollHostGuide is sent privately to the host the moment they open a lobby.
func deathrollHostGuide(limit int) string {
	return fmt.Sprintf(`🛠️ DEATHROLL HOST GUIDE
How to play: players take turns rolling 1dN, where N is the previous roll.
Roll a 1 and you lose; everyone still standing wins.
Your starting limit: 1d%d. Set it with /deathroll host <limit> (default %d, e.g. /deathroll host 5000).
How players join: they run /deathroll join while the lobby is open.
How to start: /deathroll start once at least %d players have joined.
How to cancel: /deathroll cancel (before the game starts).
Punishment: the loser receives a random /maso effect for %v, locked so they cannot remove or reroll it during that window.
The lobby stays open until you start it — the "one minute" notice is just encouragement.`,
		limit, deathrollDefaultLimit, deathrollMinPlayers, deathrollPunishmentDuration)
}

// deathrollAnnouncement is the server-wide OOC notice posted when a lobby opens.
func deathrollAnnouncement(host string, limit int) string {
	return fmt.Sprintf(`🎲 DEATHROLL — A GAME IS STARTING! 🎲
%s has opened a Deathroll lobby.
Starting roll limit: 1d%d
How to join: /deathroll join — you have one minute to join!
The host starts with /deathroll start once at least %d players have joined.
Rules: take turns rolling 1d<previous roll>. Roll a 1 and you LOSE. Everyone still standing wins.
Punishment: the loser gets the /maso effect, locked for %v.`,
		host, limit, deathrollMinPlayers, deathrollPunishmentDuration)
}

// ── /deathroll host ───────────────────────────────────────────────────────────

func deathrollHost(client *Client, args []string, usage string) {
	limit, err := deathrollParseLimit(args)
	if err != nil {
		client.SendServerMessage(err.Error() + "\n" + usage)
		return
	}

	name := deathrollName(client)

	deathrollMu.Lock()
	if st := deathrollCurrent; st != nil {
		phase := st.phase
		host := st.hostName
		deathrollMu.Unlock()
		if phase == deathrollInProgress {
			client.SendServerMessage(fmt.Sprintf("A Deathroll game is already in progress (hosted by %s). Wait for it to end.", host))
		} else {
			client.SendServerMessage(fmt.Sprintf("A Deathroll lobby is already open (hosted by %s). Join with /deathroll join — only one lobby can be open at a time.", host))
		}
		return
	}
	st := &deathrollLobby{
		hostUID:       client.Uid(),
		hostName:      name,
		startingLimit: limit,
		upperBound:    limit,
		players:       []deathrollPlayer{{uid: client.Uid(), name: name}},
		forfeited:     make(map[int]bool),
		phase:         deathrollOpen,
	}
	deathrollCurrent = st
	embed := deathrollLobbyEmbedLocked(st)
	deathrollMu.Unlock()

	client.SendServerMessage(deathrollHostGuide(limit))
	sendGlobalServerMessage(deathrollAnnouncement(name, limit))
	sendGlobalServerMessage(embed)
	addToBuffer(client, "DEATHROLL", fmt.Sprintf("Hosted Deathroll (limit 1d%d)", limit), false)
}

// ── /deathroll join ───────────────────────────────────────────────────────────

func deathrollJoin(client *Client) {
	name := deathrollName(client)
	uid := client.Uid()

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby open right now. Start one with /deathroll host [limit].")
		return
	}
	if st.phase != deathrollOpen {
		deathrollMu.Unlock()
		client.SendServerMessage("The Deathroll game has already started — you cannot join late.")
		return
	}
	if deathrollPlayerIndex(st.players, uid) >= 0 {
		deathrollMu.Unlock()
		client.SendServerMessage("You have already joined this Deathroll lobby.")
		return
	}
	st.players = append(st.players, deathrollPlayer{uid: uid, name: name})
	count := len(st.players)
	embed := deathrollLobbyEmbedLocked(st)
	deathrollMu.Unlock()

	client.SendServerMessage(fmt.Sprintf("🎲 You joined the Deathroll lobby! (%d player(s) so far)", count))
	sendGlobalServerMessage(fmt.Sprintf("🎲 %s joined Deathroll! (%d player(s))", name, count))
	sendGlobalServerMessage(embed)
}

// ── /deathroll start ──────────────────────────────────────────────────────────

func deathrollStart(client *Client) {
	uid := client.Uid()

	// Snapshot the roster, then verify connectivity outside the game lock:
	// getClientByUid takes the client-list lock and must not nest inside ours.
	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby open right now. Start one with /deathroll host [limit].")
		return
	}
	if st.phase != deathrollOpen {
		deathrollMu.Unlock()
		client.SendServerMessage("The Deathroll game has already started.")
		return
	}
	if st.hostUID != uid {
		deathrollMu.Unlock()
		client.SendServerMessage("Only the host can start the Deathroll game.")
		return
	}
	roster := append([]deathrollPlayer(nil), st.players...)
	deathrollMu.Unlock()

	connected := make([]deathrollPlayer, 0, len(roster))
	for _, p := range roster {
		if _, err := getClientByUid(p.uid); err == nil {
			connected = append(connected, p)
		}
	}
	if len(connected) < deathrollMinPlayers {
		deathrollMu.Lock()
		if deathrollCurrent == st {
			st.players = connected
		}
		deathrollMu.Unlock()
		client.SendServerMessage(fmt.Sprintf("You need at least %d players to start Deathroll (%d joined).", deathrollMinPlayers, len(connected)))
		return
	}

	rand.Shuffle(len(connected), func(i, j int) {
		connected[i], connected[j] = connected[j], connected[i]
	})

	deathrollMu.Lock()
	if deathrollCurrent != st || st.phase != deathrollOpen || st.hostUID != uid {
		deathrollMu.Unlock()
		client.SendServerMessage("The Deathroll lobby changed while starting — please try again.")
		return
	}
	st.players = connected
	st.order = append([]deathrollPlayer(nil), connected...)
	st.turn = 0
	st.upperBound = st.startingLimit
	st.lastRoll = 0
	st.lastRoller = ""
	st.lastBound = 0
	st.phase = deathrollInProgress
	st.forfeited = make(map[int]bool)
	embed := deathrollGameEmbedLocked(st)
	first := st.order[0]
	limit := st.upperBound
	order := deathrollOrderString(st.order)
	deathrollMu.Unlock()

	sendGlobalServerMessage(fmt.Sprintf(
		"🎲 DEATHROLL BEGINS!\nTurn order: %s\nFirst up: %s — roll 1d%d with /deathroll roll!",
		order, first.name, limit))
	sendGlobalServerMessage(embed)
	addToBuffer(client, "DEATHROLL", "Started Deathroll", false)
}

// ── /deathroll roll ───────────────────────────────────────────────────────────

func deathrollRoll(client *Client) {
	uid := client.Uid()

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll game in progress.")
		return
	}
	if st.phase != deathrollInProgress {
		deathrollMu.Unlock()
		client.SendServerMessage("The Deathroll game has not started yet — the host must run /deathroll start.")
		return
	}
	if len(st.order) == 0 || st.turn < 0 || st.turn >= len(st.order) {
		deathrollMu.Unlock()
		client.SendServerMessage("The Deathroll game is in an invalid state; the host should cancel it.")
		return
	}

	// Defensive: if the current entry is forfeited (the disconnect handler has
	// not advanced the turn yet), move to the next active player first.
	if st.forfeited[st.order[st.turn].uid] {
		if !deathrollAdvanceTurnLocked(st) {
			deathrollCurrent = nil
			deathrollMu.Unlock()
			sendGlobalServerMessage("🏆 Everyone forfeited — Deathroll ends with no winner.")
			return
		}
	}

	roller := st.order[st.turn]
	if roller.uid != uid {
		name := roller.name
		deathrollMu.Unlock()
		client.SendServerMessage(fmt.Sprintf("It's not your turn — %s must roll next.", name))
		return
	}

	bound := st.upperBound
	if bound < 1 {
		bound = 1
	}
	roll := deathrollRollDie(bound)
	st.lastRoll = roll
	st.lastRoller = roller.name
	st.lastBound = bound

	if roll == 1 {
		// Game over. Snapshot the survivors, clear the lobby, then do all IO
		// (including the punishment) outside the lock.
		loser := roller
		survivors := make([]deathrollPlayer, 0, len(st.order))
		for _, p := range st.order {
			if p.uid != loser.uid && !st.forfeited[p.uid] {
				survivors = append(survivors, p)
			}
		}
		deathrollCurrent = nil
		deathrollMu.Unlock()
		deathrollFinish(loser, survivors, bound)
		return
	}

	st.upperBound = roll
	active := deathrollActivePlayersLocked(st)
	if len(active) <= 1 {
		// Everyone else left; the roller is the last player standing.
		winner := roller
		deathrollCurrent = nil
		deathrollMu.Unlock()
		sendGlobalServerMessage(fmt.Sprintf(
			"🎲 %s rolled %d (out of %d).\n🏆 Everyone else forfeited — %s wins Deathroll by default!",
			roller.name, roll, bound, winner.name))
		addToBuffer(client, "DEATHROLL", "Won Deathroll by forfeit", false)
		return
	}
	if !deathrollAdvanceTurnLocked(st) {
		deathrollMu.Unlock()
		client.SendServerMessage("Deathroll could not advance the turn; the game is stuck.")
		return
	}
	next := st.order[st.turn]
	nextLimit := st.upperBound
	deathrollMu.Unlock()

	sendGlobalServerMessage(fmt.Sprintf(
		"🎲 %s rolled %d (out of %d).\n➡️ Next up: %s — roll 1d%d with /deathroll roll!",
		roller.name, roll, bound, next.name, nextLimit))
	addToBuffer(client, "DEATHROLL", fmt.Sprintf("Rolled %d out of %d", roll, bound), false)
}

// deathrollFinish applies the /maso punishment to the loser and announces the
// result. Called with deathrollMu released.
func deathrollFinish(loser deathrollPlayer, survivors []deathrollPlayer, bound int) {
	winnerText := "🏆 Winner: nobody — no players remain"
	switch len(survivors) {
	case 0:
	case 1:
		winnerText = fmt.Sprintf("🏆 Winner: %s — last player standing!", survivors[0].name)
	default:
		names := make([]string, 0, len(survivors))
		for _, p := range survivors {
			names = append(names, p.name)
		}
		winnerText = fmt.Sprintf("🏆 Winners (last players standing): %s", strings.Join(names, ", "))
	}

	loserClient, err := getClientByUid(loser.uid)
	if err != nil {
		sendGlobalServerMessage(fmt.Sprintf(
			"💀 DEATHROLL OVER!\n%s rolled a 1 (out of %d) — LOSER!\n%s\nThey left before the punishment could be applied.",
			loser.name, bound, winnerText))
		return
	}

	pType := randomMasoPunishment(PunishmentNone)
	loserClient.AddPunishmentLocked(pType, deathrollPunishmentDuration, deathrollLockDuration, "Deathroll: rolled a 1")
	loserClient.SendServerMessage(fmt.Sprintf(
		"💀 You rolled a 1 (out of %d) and lost Deathroll! Punished with the maso effect '%v' for %v — locked, so you cannot remove or reroll it during that window.",
		bound, pType, deathrollPunishmentDuration))
	sendGlobalServerMessage(fmt.Sprintf(
		"💀 DEATHROLL OVER!\n%s rolled a 1 (out of %d) — LOSER!\n%s\n🎭 Punishment: the maso effect '%v' for %v, LOCKED (no removing or rerolling).",
		loser.name, bound, winnerText, pType, deathrollPunishmentDuration))
	addToBuffer(loserClient, "DEATHROLL", fmt.Sprintf("Lost Deathroll; locked maso effect %v", pType), false)
}

// ── /deathroll cancel ─────────────────────────────────────────────────────────

func deathrollCancel(client *Client) {
	uid := client.Uid()

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby to cancel.")
		return
	}
	if st.hostUID != uid {
		deathrollMu.Unlock()
		client.SendServerMessage("Only the host can cancel the Deathroll lobby.")
		return
	}
	if st.phase != deathrollOpen {
		deathrollMu.Unlock()
		client.SendServerMessage("The game has already started — it cannot be cancelled.")
		return
	}
	host := st.hostName
	deathrollCurrent = nil
	deathrollMu.Unlock()

	sendGlobalServerMessage(fmt.Sprintf("🛑 The Deathroll lobby was cancelled by host %s.", host))
	addToBuffer(client, "DEATHROLL", "Cancelled Deathroll lobby", false)
}

// ── /deathroll status ─────────────────────────────────────────────────────────

func deathrollStatus(client *Client) {
	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby or game right now. Start one with /deathroll host [limit].")
		return
	}
	var embed string
	if st.phase == deathrollOpen {
		embed = deathrollLobbyEmbedLocked(st)
	} else {
		embed = deathrollGameEmbedLocked(st)
	}
	deathrollMu.Unlock()
	client.SendServerMessage(embed)
}

// ── Disconnect handling ───────────────────────────────────────────────────────

// handleDeathrollDisconnect removes an open-lobby member, cancels a lobby whose
// host left, or forfeits an in-progress player. Forfeiting is deliberately
// chosen over "skip": a skipped player would otherwise hold the turn forever,
// and ending the game when only one active player remains gives the survivors a
// clean result. A player who already left cannot receive the loser punishment.
//
// Called from Client.teardown while the departing client is still in the client
// list, so it must not rely on that client being absent.
func handleDeathrollDisconnect(client *Client) {
	uid := client.Uid()

	deathrollMu.Lock()
	st := deathrollCurrent
	if st == nil {
		deathrollMu.Unlock()
		return
	}
	idx := deathrollPlayerIndex(st.players, uid)
	if idx < 0 {
		deathrollMu.Unlock()
		return
	}
	name := st.players[idx].name

	if st.phase == deathrollOpen {
		st.players = append(st.players[:idx], st.players[idx+1:]...)
		if st.hostUID == uid {
			host := st.hostName
			deathrollCurrent = nil
			deathrollMu.Unlock()
			sendGlobalServerMessage(fmt.Sprintf("🛑 Deathroll lobby cancelled — host %s disconnected.", host))
			return
		}
		count := len(st.players)
		embed := deathrollLobbyEmbedLocked(st)
		deathrollMu.Unlock()
		sendGlobalServerMessage(fmt.Sprintf("🎲 %s left the Deathroll lobby. (%d player(s) remaining)", name, count))
		sendGlobalServerMessage(embed)
		return
	}

	// In progress: forfeit the departing player.
	st.forfeited[uid] = true
	wasTurn := len(st.order) > 0 && st.turn >= 0 && st.turn < len(st.order) && st.order[st.turn].uid == uid
	active := deathrollActivePlayersLocked(st)
	if len(active) <= 1 {
		winnerText := "🏆 Nobody is left to win."
		if len(active) == 1 {
			winnerText = fmt.Sprintf("🏆 %s wins Deathroll by default — everyone else forfeited!", active[0].name)
		}
		deathrollCurrent = nil
		deathrollMu.Unlock()
		sendGlobalServerMessage(fmt.Sprintf(
			"🏳️ %s left the Deathroll game and forfeits.\n%s\nNo punishment could be applied to a player who already left.",
			name, winnerText))
		return
	}

	var next deathrollPlayer
	hasNext := false
	if wasTurn {
		if deathrollAdvanceTurnLocked(st) {
			next = st.order[st.turn]
			hasNext = true
		}
	} else if len(st.order) > 0 && st.turn >= 0 && st.turn < len(st.order) {
		next = st.order[st.turn]
		hasNext = true
	}
	limit := st.upperBound
	deathrollMu.Unlock()

	msg := fmt.Sprintf("🏳️ %s left the Deathroll game and forfeits.", name)
	if wasTurn && hasNext {
		msg += fmt.Sprintf("\n➡️ Next up: %s — roll 1d%d with /deathroll roll!", next.name, limit)
	}
	sendGlobalServerMessage(msg)
}
