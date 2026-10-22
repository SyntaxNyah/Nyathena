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

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/area"
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
const deathrollUsage = "Usage: /deathroll <host [limit] | join | start | roll | cancel | limit <n> | status | hide>\n" +
	"  /deathroll host [limit]  — open a lobby in YOUR ROOM (default limit 100; e.g. /deathroll host 5000)\n" +
	"  /deathroll join          — join the open lobby in your room\n" +
	"  /deathroll start         — host only: shuffle players and begin (2+ needed)\n" +
	"  /deathroll roll          — current player: roll 1d<current limit>\n" +
	"  /deathroll cancel        — host only: cancel the open lobby\n" +
	"  /deathroll limit <n>     — host only: change the starting limit before the start\n" +
	"  /deathroll status        — show the lobby/game in your room\n" +
	"  /deathroll hide          — toggle Deathroll broadcasts you are not part of"

// ── State ─────────────────────────────────────────────────────────────────────

// deathrollPhase is the lifecycle state of a room's lobby.
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

// deathrollLobby is the complete state of one room's game. Every field is
// guarded by deathrollMu; helpers whose names end in Locked require it held.
// No network IO (including getClientByUid) may run while it is held.
type deathrollLobby struct {
	area          *area.Area
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
	deathrollMu    sync.Mutex
	deathrollRooms = make(map[*area.Area]*deathrollLobby)
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
	case "limit":
		deathrollLimit(client, args[1:], usage)
	case "status":
		deathrollStatus(client)
	case "hide":
		deathrollHideToggle(client)
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
	return deathrollValidateLimit(strings.TrimSpace(args[0]))
}

// deathrollValidateLimit parses and bounds-checks one limit value.
func deathrollValidateLimit(raw string) (int, error) {
	limit, err := strconv.Atoi(raw)
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

// ── State helpers ─────────────────────────────────────────────────────────────

// deathrollParticipantSetLocked builds the set of UIDs that always receive this
// lobby's messages, even if they toggled /deathroll hide. Caller holds deathrollMu.
func deathrollParticipantSetLocked(st *deathrollLobby) map[int]bool {
	set := make(map[int]bool, len(st.players)+len(st.order))
	for _, p := range st.players {
		set[p.uid] = true
	}
	for _, p := range st.order {
		set[p.uid] = true
	}
	return set
}

// deathrollFindByPlayerLocked returns the lobby a UID belongs to. Used for
// disconnect cleanup and for players who changed rooms mid-game. Caller holds
// deathrollMu.
func deathrollFindByPlayerLocked(uid int) (*deathrollLobby, bool) {
	for _, st := range deathrollRooms {
		if deathrollPlayerIndex(st.players, uid) >= 0 {
			return st, true
		}
		if deathrollPlayerIndex(st.order, uid) >= 0 {
			return st, true
		}
	}
	return nil, false
}

// deathrollFindByHostLocked returns the lobby hosted by uid, if any. Caller
// holds deathrollMu.
func deathrollFindByHostLocked(uid int) (*deathrollLobby, bool) {
	for _, st := range deathrollRooms {
		if st.hostUID == uid {
			return st, true
		}
	}
	return nil, false
}

// deathrollDeleteLocked removes st from its room, but only if the map still
// points at this exact lobby (so a newer lobby is never clobbered). Caller
// holds deathrollMu.
func deathrollDeleteLocked(st *deathrollLobby) {
	if st.area != nil && deathrollRooms[st.area] == st {
		delete(deathrollRooms, st.area)
	}
}

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

// deathrollSend delivers a Deathroll OOC message. Participants always receive
// it (they need turn/roll information even if they hid broadcasts); anyone else
// must be in the lobby's room and must not have toggled /deathroll hide.
// Callers build participants under deathrollMu and call this after unlocking.
func deathrollSend(a *area.Area, participants map[int]bool, message string) {
	if a == nil {
		return
	}
	out := &aolib.CTToClient{Name: serverName, Message: message, IsFromServer: true}
	clients.ForEach(func(c *Client) {
		if participants[c.Uid()] {
			c.Send(out)
			return
		}
		if c.Area() != a || c.DeathrollHide() {
			return
		}
		c.Send(out)
	})
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
	b.WriteString("Change limit (host): /deathroll limit <n>\n")
	b.WriteString("Cancel (host): /deathroll cancel\n")
	b.WriteString("Hide announcements: /deathroll hide")
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
Your starting limit: 1d%d. Set it with /deathroll host <limit> (default %d, e.g. /deathroll host 5000),
or change it before the start with /deathroll limit <limit>.
This lobby is ROOM-SCOPED: only your current room sees its OOC messages.
How players join: they run /deathroll join while the lobby is open.
How to start: /deathroll start once at least %d players have joined.
How to cancel: /deathroll cancel (before the game starts).
Punishment: the loser receives a random /maso effect for %v, locked so they cannot remove or reroll it during that window.
The lobby stays open until you start it — the "one minute" notice is just encouragement.
Anyone who does not want to see Deathroll announcements can use /deathroll hide.`,
		limit, deathrollDefaultLimit, deathrollMinPlayers, deathrollPunishmentDuration)
}

// deathrollAnnouncement is the room OOC notice posted when a lobby opens.
func deathrollAnnouncement(host string, limit int) string {
	return fmt.Sprintf(`🎲 DEATHROLL — A GAME IS STARTING IN THIS ROOM! 🎲
%s has opened a Deathroll lobby here.
Starting roll limit: 1d%d
How to join: /deathroll join — you have one minute to join!
The host starts with /deathroll start once at least %d players have joined.
Rules: take turns rolling 1d<previous roll>. Roll a 1 and you LOSE. Everyone still standing wins.
Punishment: the loser gets the /maso effect, locked for %v.
Use /deathroll hide to stop seeing these announcements.`,
		host, limit, deathrollMinPlayers, deathrollPunishmentDuration)
}

// ── /deathroll host ───────────────────────────────────────────────────────────

func deathrollHost(client *Client, args []string, usage string) {
	a := client.Area()
	if a == nil {
		client.SendServerMessage("You must be in an area to host Deathroll.")
		return
	}
	limit, err := deathrollParseLimit(args)
	if err != nil {
		client.SendServerMessage(err.Error() + "\n" + usage)
		return
	}
	name := deathrollName(client)
	uid := client.Uid()

	deathrollMu.Lock()
	if other, already := deathrollFindByPlayerLocked(uid); already {
		room := "this room"
		if other.area != a {
			room = "another room"
		}
		deathrollMu.Unlock()
		client.SendServerMessage(fmt.Sprintf("You are already in a Deathroll lobby/game in %s. Finish it before hosting another.", room))
		return
	}
	if existing := deathrollRooms[a]; existing != nil {
		phase := existing.phase
		host := existing.hostName
		deathrollMu.Unlock()
		if phase == deathrollInProgress {
			client.SendServerMessage(fmt.Sprintf("A Deathroll game is already in progress in this room (hosted by %s). Wait for it to end.", host))
		} else {
			client.SendServerMessage(fmt.Sprintf("A Deathroll lobby is already open in this room (hosted by %s). Join it with /deathroll join.", host))
		}
		return
	}
	st := &deathrollLobby{
		area:          a,
		hostUID:       uid,
		hostName:      name,
		startingLimit: limit,
		upperBound:    limit,
		players:       []deathrollPlayer{{uid: uid, name: name}},
		forfeited:     make(map[int]bool),
		phase:         deathrollOpen,
	}
	deathrollRooms[a] = st
	participants := deathrollParticipantSetLocked(st)
	embed := deathrollLobbyEmbedLocked(st)
	deathrollMu.Unlock()

	client.SendServerMessage(deathrollHostGuide(limit))
	deathrollSend(a, participants, deathrollAnnouncement(name, limit))
	deathrollSend(a, participants, embed)
	addToBuffer(client, "DEATHROLL", fmt.Sprintf("Hosted room-scoped Deathroll (limit 1d%d)", limit), false)
}

// ── /deathroll join ───────────────────────────────────────────────────────────

func deathrollJoin(client *Client) {
	a := client.Area()
	if a == nil {
		client.SendServerMessage("You must be in an area to join Deathroll.")
		return
	}
	name := deathrollName(client)
	uid := client.Uid()

	deathrollMu.Lock()
	if other, already := deathrollFindByPlayerLocked(uid); already {
		deathrollMu.Unlock()
		if other.area == a {
			client.SendServerMessage("You have already joined this Deathroll lobby.")
		} else {
			client.SendServerMessage("You are already in a Deathroll lobby/game in another room. Finish it before joining another.")
		}
		return
	}
	st := deathrollRooms[a]
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby open in this room. Start one with /deathroll host [limit].")
		return
	}
	if st.phase != deathrollOpen {
		deathrollMu.Unlock()
		client.SendServerMessage("The Deathroll game has already started — you cannot join late.")
		return
	}
	st.players = append(st.players, deathrollPlayer{uid: uid, name: name})
	count := len(st.players)
	participants := deathrollParticipantSetLocked(st)
	embed := deathrollLobbyEmbedLocked(st)
	deathrollMu.Unlock()

	client.SendServerMessage(fmt.Sprintf("🎲 You joined the Deathroll lobby! (%d player(s) so far)", count))
	deathrollSend(a, participants, fmt.Sprintf("🎲 %s joined Deathroll! (%d player(s))", name, count))
	deathrollSend(a, participants, embed)
}

// ── /deathroll start ──────────────────────────────────────────────────────────

func deathrollStart(client *Client) {
	a := client.Area()
	if a == nil {
		client.SendServerMessage("You must be in the room where the Deathroll lobby was opened.")
		return
	}
	uid := client.Uid()

	// Snapshot the roster, then verify connectivity outside the game lock:
	// getClientByUid takes the client-list lock and must not nest inside ours.
	deathrollMu.Lock()
	st := deathrollRooms[a]
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby open in this room.")
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
		var embed string
		var participants map[int]bool
		if deathrollRooms[a] == st {
			st.players = connected
			participants = deathrollParticipantSetLocked(st)
			embed = deathrollLobbyEmbedLocked(st)
		}
		deathrollMu.Unlock()
		client.SendServerMessage(fmt.Sprintf("You need at least %d players to start Deathroll (%d joined).", deathrollMinPlayers, len(connected)))
		if embed != "" {
			deathrollSend(a, participants, embed)
		}
		return
	}

	rand.Shuffle(len(connected), func(i, j int) {
		connected[i], connected[j] = connected[j], connected[i]
	})

	deathrollMu.Lock()
	if deathrollRooms[a] != st || st.phase != deathrollOpen || st.hostUID != uid {
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
	participants := deathrollParticipantSetLocked(st)
	embed := deathrollGameEmbedLocked(st)
	first := st.order[0]
	limit := st.upperBound
	order := deathrollOrderString(st.order)
	deathrollMu.Unlock()

	deathrollSend(a, participants, fmt.Sprintf(
		"🎲 DEATHROLL BEGINS IN THIS ROOM!\nTurn order: %s\nFirst up: %s — roll 1d%d with /deathroll roll!",
		order, first.name, limit))
	deathrollSend(a, participants, embed)
	addToBuffer(client, "DEATHROLL", "Started Deathroll", false)
}

// ── /deathroll roll ───────────────────────────────────────────────────────────

func deathrollRoll(client *Client) {
	a := client.Area()
	uid := client.Uid()

	deathrollMu.Lock()
	// Prefer the game this player is actually in, so someone who changed rooms
	// mid-game can still take their turn. Otherwise use the room's lobby.
	var st *deathrollLobby
	if found, ok := deathrollFindByPlayerLocked(uid); ok {
		st = found
	} else if a != nil {
		st = deathrollRooms[a]
	}
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll game in progress in this room.")
		return
	}
	if deathrollPlayerIndex(st.players, uid) < 0 {
		deathrollMu.Unlock()
		client.SendServerMessage("You are not in this Deathroll game.")
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
			room := st.area
			participants := deathrollParticipantSetLocked(st)
			deathrollDeleteLocked(st)
			deathrollMu.Unlock()
			deathrollSend(room, participants, "🏆 Everyone forfeited — Deathroll ends with no winner.")
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
		// Game over. Snapshot the survivors and recipients, clear the room's
		// lobby, then do all IO (including the punishment) outside the lock.
		loser := roller
		survivors := make([]deathrollPlayer, 0, len(st.order))
		for _, p := range st.order {
			if p.uid != loser.uid && !st.forfeited[p.uid] {
				survivors = append(survivors, p)
			}
		}
		room := st.area
		participants := deathrollParticipantSetLocked(st)
		deathrollDeleteLocked(st)
		deathrollMu.Unlock()
		deathrollFinish(room, participants, loser, survivors, bound)
		return
	}

	st.upperBound = roll
	active := deathrollActivePlayersLocked(st)
	if len(active) <= 1 {
		// Everyone else left; the roller is the last player standing.
		winner := roller
		room := st.area
		participants := deathrollParticipantSetLocked(st)
		deathrollDeleteLocked(st)
		deathrollMu.Unlock()
		deathrollSend(room, participants, fmt.Sprintf(
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
	room := st.area
	participants := deathrollParticipantSetLocked(st)
	deathrollMu.Unlock()

	deathrollSend(room, participants, fmt.Sprintf(
		"🎲 %s rolled %d (out of %d).\n➡️ Next up: %s — roll 1d%d with /deathroll roll!",
		roller.name, roll, bound, next.name, nextLimit))
	addToBuffer(client, "DEATHROLL", fmt.Sprintf("Rolled %d out of %d", roll, bound), false)
}

// deathrollFinish applies the /maso punishment to the loser and announces the
// result in the lobby's room. Called with deathrollMu released.
func deathrollFinish(room *area.Area, participants map[int]bool, loser deathrollPlayer, survivors []deathrollPlayer, bound int) {
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
		deathrollSend(room, participants, fmt.Sprintf(
			"💀 DEATHROLL OVER!\n%s rolled a 1 (out of %d) — LOSER!\n%s\nThey left before the punishment could be applied.",
			loser.name, bound, winnerText))
		return
	}

	pType := randomMasoPunishment(PunishmentNone)
	loserClient.AddPunishmentLocked(pType, deathrollPunishmentDuration, deathrollLockDuration, "Deathroll: rolled a 1")
	loserClient.SendServerMessage(fmt.Sprintf(
		"💀 You rolled a 1 (out of %d) and lost Deathroll! Punished with the maso effect '%v' for %v — locked, so you cannot remove or reroll it during that window.",
		bound, pType, deathrollPunishmentDuration))
	deathrollSend(room, participants, fmt.Sprintf(
		"💀 DEATHROLL OVER!\n%s rolled a 1 (out of %d) — LOSER!\n%s\n🎭 Punishment: the maso effect '%v' for %v, LOCKED (no removing or rerolling).",
		loser.name, bound, winnerText, pType, deathrollPunishmentDuration))
	addToBuffer(loserClient, "DEATHROLL", fmt.Sprintf("Lost Deathroll; locked maso effect %v", pType), false)
}

// ── /deathroll cancel ─────────────────────────────────────────────────────────

func deathrollCancel(client *Client) {
	a := client.Area()
	uid := client.Uid()

	deathrollMu.Lock()
	st := deathrollRooms[a]
	if st == nil {
		if found, ok := deathrollFindByHostLocked(uid); ok {
			st = found
		}
	}
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby to cancel in this room.")
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
	room := st.area
	participants := deathrollParticipantSetLocked(st)
	deathrollDeleteLocked(st)
	deathrollMu.Unlock()

	deathrollSend(room, participants, fmt.Sprintf("🛑 The Deathroll lobby was cancelled by host %s.", host))
	addToBuffer(client, "DEATHROLL", "Cancelled Deathroll lobby", false)
}

// ── /deathroll limit ──────────────────────────────────────────────────────────

func deathrollLimit(client *Client, args []string, usage string) {
	if len(args) != 1 {
		client.SendServerMessage("Usage: /deathroll limit <n>\n" + usage)
		return
	}
	limit, err := deathrollValidateLimit(strings.TrimSpace(args[0]))
	if err != nil {
		client.SendServerMessage(err.Error())
		return
	}

	uid := client.Uid()
	deathrollMu.Lock()
	st := deathrollRooms[client.Area()]
	if st == nil {
		if found, ok := deathrollFindByHostLocked(uid); ok {
			st = found
		}
	}
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby to change.")
		return
	}
	if st.hostUID != uid {
		deathrollMu.Unlock()
		client.SendServerMessage("Only the host can change the starting limit.")
		return
	}
	if st.phase != deathrollOpen {
		deathrollMu.Unlock()
		client.SendServerMessage("The game has already started — the starting limit can no longer be changed.")
		return
	}
	st.startingLimit = limit
	st.upperBound = limit
	host := st.hostName
	room := st.area
	participants := deathrollParticipantSetLocked(st)
	embed := deathrollLobbyEmbedLocked(st)
	deathrollMu.Unlock()

	client.SendServerMessage(fmt.Sprintf("🎲 Deathroll starting limit changed to 1d%d.", limit))
	deathrollSend(room, participants, fmt.Sprintf("🎲 %s changed the Deathroll starting limit to 1d%d.", host, limit))
	deathrollSend(room, participants, embed)
	addToBuffer(client, "DEATHROLL", fmt.Sprintf("Changed starting limit to 1d%d", limit), false)
}

// ── /deathroll status ─────────────────────────────────────────────────────────

func deathrollStatus(client *Client) {
	a := client.Area()
	uid := client.Uid()

	deathrollMu.Lock()
	var st *deathrollLobby
	if found, ok := deathrollFindByPlayerLocked(uid); ok {
		st = found
	} else if a != nil {
		st = deathrollRooms[a]
	}
	if st == nil {
		deathrollMu.Unlock()
		client.SendServerMessage("There is no Deathroll lobby or game in this room. Start one with /deathroll host [limit].")
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

// ── /deathroll hide ───────────────────────────────────────────────────────────

func deathrollHideToggle(client *Client) {
	if client.DeathrollHide() {
		client.SetDeathrollHide(false)
		client.SendServerMessage("🎲 Deathroll announcements are now visible again.")
	} else {
		client.SetDeathrollHide(true)
		client.SendServerMessage("🔇 Deathroll announcements are now hidden. You will still see messages for games you join. Use /deathroll hide again to show them.")
	}
	addToBuffer(client, "CMD", "Toggled Deathroll announcement hide", false)
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
	st, found := deathrollFindByPlayerLocked(uid)
	if !found {
		deathrollMu.Unlock()
		return
	}
	idx := deathrollPlayerIndex(st.players, uid)
	if idx < 0 {
		deathrollMu.Unlock()
		return
	}
	name := st.players[idx].name
	room := st.area

	if st.phase == deathrollOpen {
		st.players = append(st.players[:idx], st.players[idx+1:]...)
		if st.hostUID == uid {
			host := st.hostName
			participants := deathrollParticipantSetLocked(st)
			deathrollDeleteLocked(st)
			deathrollMu.Unlock()
			deathrollSend(room, participants, fmt.Sprintf("🛑 Deathroll lobby cancelled — host %s disconnected.", host))
			return
		}
		count := len(st.players)
		participants := deathrollParticipantSetLocked(st)
		embed := deathrollLobbyEmbedLocked(st)
		deathrollMu.Unlock()
		deathrollSend(room, participants, fmt.Sprintf("🎲 %s left the Deathroll lobby. (%d player(s) remaining)", name, count))
		deathrollSend(room, participants, embed)
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
		participants := deathrollParticipantSetLocked(st)
		deathrollDeleteLocked(st)
		deathrollMu.Unlock()
		deathrollSend(room, participants, fmt.Sprintf(
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
	participants := deathrollParticipantSetLocked(st)
	deathrollMu.Unlock()

	msg := fmt.Sprintf("🏳️ %s left the Deathroll game and forfeits.", name)
	if wasTurn && hasNext {
		msg += fmt.Sprintf("\n➡️ Next up: %s — roll 1d%d with /deathroll roll!", next.name, limit)
	}
	deathrollSend(room, participants, msg)
}
