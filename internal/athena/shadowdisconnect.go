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
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// /shadowdisconnect — admin-only stealth ban. Unlike /ban or /kick, a
// shadow-disconnected IPID is never told why (or that) it's blocked: every
// future connection attempt is dropped in a way indistinguishable from an
// ordinary network failure, forever, until an admin lifts it with
// /shadowundisconnect. Persisted by IPID in the SHADOW_DISCONNECTS table
// (see internal/db/db.go), the same way /musicban and /curserandomchar
// persist — an in-memory-only flag would vanish the instant the target
// reconnects, since a fresh connection gets a brand new *Client.
//
// The in-memory set (shadowDisconnects) is seeded from the DB at startup so
// the reject-at-connect checks are a single RWMutex map lookup, cheap enough
// to run before any AO2/WebSocket handshake work happens (see the checks in
// server.go's ListenTCP/HandleWS, and the defense-in-depth check in
// client.go's HandleClient — mirroring how the torment list re-checks at
// both the accept loop and HandleClient entry).
//
// Producing a *plausible* error matters for the WebAO/WSS transport: this
// package's client.conn is a websocket.NetConn (nhooyr.io/websocket v1.8.7)
// wrapping a real *websocket.Conn. Calling conn.Close() on it — what /kick,
// /ban and every other disconnect path in this codebase do — performs a
// *clean* WebSocket closing handshake (status 1000, "Normal Closure"),
// which reads to a browser as a deliberate, well-behaved close, not an
// error. client.go's defaultWriteDeadline doc comment already documents the
// workaround this file relies on: nhooyr.io/websocket closes the
// underlying connection outright, with no Close frame, the instant a single
// Write hits an expired deadline. shadowDisconnectNow forces exactly that
// path — an already-past deadline plus a doomed Write — so a WebAO client
// sees a generic, unexplained connection error (abnormal closure) instead
// of a clean disconnect. For a raw TCP AO2 client, TCP has no such framing
// to begin with, so a bare conn.Close() with no packet sent is already
// indistinguishable from a network drop.

var (
	shadowDisconnectsMu sync.RWMutex
	shadowDisconnects   = map[string]struct{}{}
)

// initShadowDisconnects seeds the in-memory shadow-disconnect set from the
// database. Called once during server startup after the DB is opened. A DB
// error is logged but non-fatal — the in-memory set just stays empty and
// entries will repopulate as staff re-issues them.
func initShadowDisconnects() {
	rows, err := db.ListShadowDisconnects()
	if err != nil {
		logger.LogErrorf("shadowdisconnect: failed to load existing entries from DB: %v", err)
		return
	}
	shadowDisconnectsMu.Lock()
	for _, sd := range rows {
		shadowDisconnects[sd.Ipid] = struct{}{}
	}
	n := len(shadowDisconnects)
	shadowDisconnectsMu.Unlock()
	if n > 0 {
		logger.LogInfof("shadowdisconnect: loaded %d entry(s) from database.", n)
	}
}

// isShadowDisconnected reports whether the given IPID currently carries an
// active /shadowdisconnect listing. Hot path: one RLock + map lookup, no DB
// query — called on every raw connection attempt, before a session exists.
func isShadowDisconnected(ipid string) bool {
	shadowDisconnectsMu.RLock()
	_, ok := shadowDisconnects[ipid]
	shadowDisconnectsMu.RUnlock()
	return ok
}

// addShadowDisconnectMemory inserts an IPID into the in-memory set. Pair
// with db.AddShadowDisconnect for persistence.
func addShadowDisconnectMemory(ipid string) {
	shadowDisconnectsMu.Lock()
	shadowDisconnects[ipid] = struct{}{}
	shadowDisconnectsMu.Unlock()
}

// removeShadowDisconnectMemory drops an IPID from the in-memory set. Pair
// with db.RemoveShadowDisconnect for persistence.
func removeShadowDisconnectMemory(ipid string) {
	shadowDisconnectsMu.Lock()
	delete(shadowDisconnects, ipid)
	shadowDisconnectsMu.Unlock()
}

// clearShadowDisconnectMemory empties the in-memory set. Pair with
// db.ClearAllShadowDisconnects for the /shadowundisconnect all bulk-lift.
func clearShadowDisconnectMemory() {
	shadowDisconnectsMu.Lock()
	shadowDisconnects = map[string]struct{}{}
	shadowDisconnectsMu.Unlock()
}

// shadowDisconnectNow immediately and silently terminates this connection.
// No packet is sent, and for a WebSocket client the closure is forced onto
// nhooyr.io/websocket's abrupt path (see the package doc above) rather than
// the clean handshake a plain conn.Close() alone would perform, so the
// disconnect reads as a generic connection error rather than a deliberate
// kick.
func (client *Client) shadowDisconnectNow() {
	client.conn.SetWriteDeadline(time.Now()) //nolint:errcheck
	client.conn.Write([]byte{0})             //nolint:errcheck
	client.markClosed()
}

// cmdShadowDisconnect handles /shadowdisconnect <uid|ipid> [-r reason].
// ADMIN only (enforced by the command registry). Accepts a connected
// target's UID or a raw IPID, so an offline player can be preemptively
// shadow-banned. Persists the listing by IPID, drops every live connection
// sharing that IPID right now, and silently drops every future connection
// attempt from it — forever, until lifted with /shadowundisconnect.
func cmdShadowDisconnect(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	reason := ""
	rest := args
	for i := 0; i < len(rest); i++ {
		if rest[i] == "-r" && i+1 < len(rest) {
			reason = strings.Join(rest[i+1:], " ")
			rest = rest[:i]
			break
		}
	}
	if len(rest) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	arg := strings.TrimSpace(rest[0])
	ipid := arg
	uidNote := ""
	if uid, err := strconv.Atoi(arg); err == nil {
		target, terr := getClientByUid(uid)
		if terr != nil {
			client.SendServerMessage(fmt.Sprintf("Client with UID %d not found; pass an IPID directly to shadow-disconnect an offline player.", uid))
			return
		}
		if permissions.IsModerator(target.Perms()) {
			client.SendServerMessage("You cannot shadow-disconnect a moderator.")
			return
		}
		ipid = target.Ipid()
		uidNote = fmt.Sprintf(" (UID %d)", uid)
	}

	// Console gate. The target has resolved and the moderator check has passed,
	// so a mistyped UID or a protected target is already rejected without
	// spending the grant, and a refusal here leaves the target's connection
	// untouched.
	//
	// It has to sit before the write rather than after it: on the far side, a
	// refusal would leave an unauthorised row already persisted. The cost of
	// that ordering is that a database failure on the next line spends the
	// grant even though nothing happened -- rare, recoverable by re-arming, and
	// the right way round to be wrong.
	if !consumeConsoleGrant(client, "shadowdisconnect") {
		return
	}

	if err := db.AddShadowDisconnect(ipid, reason, client.ModName(), time.Now().UTC().Unix()); err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to persist shadow-disconnect: %v", err))
		return
	}
	addShadowDisconnectMemory(ipid)

	dropped := 0
	for _, c := range getClientsByIpid(ipid) {
		c.shadowDisconnectNow()
		dropped++
	}

	summary := fmt.Sprintf("IPID %v%v is now shadow-disconnected: every future connection attempt will be dropped silently — no ban message, no reason given — until an admin lifts it with /shadowundisconnect.", ipid, uidNote)
	if dropped > 0 {
		summary += fmt.Sprintf(" %d active connection(s) were just dropped.", dropped)
	}
	if reason != "" {
		summary += " Reason: " + reason
	}
	client.SendServerMessage(summary)
	addToBuffer(client, "CMD", fmt.Sprintf("Shadow-disconnected IPID %v%v.", ipid, uidNote), true)
	logger.WriteAudit(fmt.Sprintf("%v | SHADOWDISCONNECT | IPID:%v | %v | By: %v",
		time.Now().UTC().Format("15:04:05"), ipid, reason, client.ModName()))
}

// cmdShadowUndisconnect handles /shadowundisconnect <uid|ipid|all>. ADMIN
// only. Accepts a connected target's UID or a raw IPID (so an offline
// player can be lifted too), or the literal "all" to clear the entire list
// at once.
func cmdShadowUndisconnect(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	arg := strings.TrimSpace(args[0])

	if strings.EqualFold(arg, "all") {
		n, err := db.ClearAllShadowDisconnects()
		if err != nil {
			client.SendServerMessage(fmt.Sprintf("Failed to clear the shadow-disconnect list: %v", err))
			return
		}
		clearShadowDisconnectMemory()
		entryWord := "entries"
		if n == 1 {
			entryWord = "entry"
		}
		summary := fmt.Sprintf("Cleared the entire shadow-disconnect list (%d %v removed).", n, entryWord)
		client.SendServerMessage(summary)
		addToBuffer(client, "CMD", summary, true)
		logger.WriteAudit(fmt.Sprintf("%v | SHADOWUNDISCONNECT | ALL (%d removed) | By: %v",
			time.Now().UTC().Format("15:04:05"), n, client.ModName()))
		return
	}

	ipid := arg
	if uid, err := strconv.Atoi(arg); err == nil {
		target, terr := getClientByUid(uid)
		if terr != nil {
			client.SendServerMessage(fmt.Sprintf("Client with UID %d not found; pass an IPID directly to lift a shadow-disconnect on an offline player.", uid))
			return
		}
		ipid = target.Ipid()
	}

	if err := db.RemoveShadowDisconnect(ipid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			client.SendServerMessage(fmt.Sprintf("IPID %v is not currently shadow-disconnected.", ipid))
			return
		}
		client.SendServerMessage(fmt.Sprintf("Failed to remove shadow-disconnect: %v", err))
		return
	}
	removeShadowDisconnectMemory(ipid)

	summary := fmt.Sprintf("Lifted the shadow-disconnect on IPID %v. They can connect normally again.", ipid)
	client.SendServerMessage(summary)
	addToBuffer(client, "CMD", summary, true)
	logger.WriteAudit(fmt.Sprintf("%v | SHADOWUNDISCONNECT | IPID:%v | By: %v",
		time.Now().UTC().Format("15:04:05"), ipid, client.ModName()))
}

// cmdShadowDisconnectList handles /shadowdisconnectlist. ADMIN only. Lists
// every active shadow-disconnect entry with its reason and issuer, newest
// first.
func cmdShadowDisconnectList(client *Client, _ []string, _ string) {
	rows, err := db.ListShadowDisconnects()
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to read the shadow-disconnect list: %v", err))
		return
	}
	if len(rows) == 0 {
		client.SendServerMessage("No active shadow-disconnects.")
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Active shadow-disconnects (%d):\n", len(rows))
	for _, sd := range rows {
		when := time.Unix(sd.IssuedAt, 0).UTC().Format("2006-01-02 15:04 MST")
		reason := sd.Reason
		if reason == "" {
			reason = "(no reason given)"
		}
		fmt.Fprintf(&sb, "  • %v — issued %v by %v — %v\n", sd.Ipid, when, sd.IssuedBy, reason)
	}
	client.SendServerMessage(strings.TrimRight(sb.String(), "\n"))
}
