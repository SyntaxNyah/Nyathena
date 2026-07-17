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
	"math/rand"
	"strconv"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// /curserandomchar — admin-only curse that forces the target's character to
// randomly change every 1-5 seconds, forever, until an admin lifts it with
// /uncurserandomchar.
//
// The curse is persisted by IPID in the RANDOMCHAR_CURSES table (see
// internal/db/db.go), the same way /musicban persists. That's deliberate:
// a plain in-memory flag tied to the *Client would vanish the instant the
// target reconnects (a fresh connection gets a brand new *Client), which
// would make relogging a trivial escape hatch. Keying by IPID and re-arming
// on join (restoreRandomCharCurse, called from pktReqDone) means the curse
// survives disconnects, relogs, and even a full server restart.

// armCurseRandomChar marks the client as cursed and lazily starts the
// per-connection watcher goroutine that performs the actual character
// swaps. Idempotent — safe to call on an already-cursed client (e.g. on
// every join for a persistently-cursed IPID).
func (client *Client) armCurseRandomChar() {
	client.curseRandomCharActive.Store(true)
	if client.curseRandomCharWatcherStarted.CompareAndSwap(false, true) {
		go client.curseRandomCharWatch()
	}
}

// disarmCurseRandomChar lifts the curse for this connection. The watcher
// goroutine (if running) notices on its next tick, at most 5 seconds later,
// and exits on its own.
func (client *Client) disarmCurseRandomChar() {
	client.curseRandomCharActive.Store(false)
}

// curseRandomCharWatch repeatedly swaps the client to a random free
// character on a random 1-5 second interval, until the curse is lifted
// (disarmCurseRandomChar) or the connection closes (client.done). Mirrors
// the leak-free shape of dcIdleWatcher (disconnect_timer.go): selecting on
// client.done guarantees this goroutine can never outlive the connection,
// regardless of how long the curse itself is supposed to last.
func (client *Client) curseRandomCharWatch() {
	defer client.curseRandomCharWatcherStarted.Store(false)
	for {
		wait := time.Duration(1+rand.Intn(5)) * time.Second // 1-5 seconds, inclusive
		timer := time.NewTimer(wait)
		select {
		case <-client.done:
			timer.Stop()
			return
		case <-timer.C:
			if !client.curseRandomCharActive.Load() {
				return
			}
			if !client.IsTunged() {
				if newid := getRandomFreeChar(client); newid != -1 {
					client.ChangeCharacter(newid)
				}
			}
		}
	}
}

// restoreRandomCharCurse re-arms an active /curserandomchar curse after the
// client (re)connects. Called once after a client successfully joins,
// alongside restorePunishments.
func (client *Client) restoreRandomCharCurse() {
	cursed, err := db.IsRandomCharCursed(client.Ipid())
	if err != nil {
		logger.LogErrorf("Error checking random-char curse for %v: %v", client.Ipid(), err)
		return
	}
	if !cursed {
		return
	}
	client.armCurseRandomChar()
	client.SendServerMessage("You are still cursed: your character will keep randomly changing every 1-5 seconds. Only an admin can lift it, with /uncurserandomchar.")
}

// cmdCurseRandomChar handles /curserandomchar <uid>. ADMIN only (enforced by
// the command registry). Persists the curse by the target's IPID so it
// survives a reconnect, then arms it immediately for the live connection.
func cmdCurseRandomChar(client *Client, args []string, _ string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %d does not exist.", uid))
		return
	}

	if err := db.AddRandomCharCurse(target.Ipid(), client.ModName(), time.Now().UTC().Unix()); err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to persist curse: %v", err))
		return
	}
	target.armCurseRandomChar()

	target.SendServerMessage("You have been cursed: your character will now randomly change every 1-5 seconds. This persists even if you reconnect — only an admin can remove it, with /uncurserandomchar.")
	client.SendServerMessage(fmt.Sprintf("Cursed UID %d (IPID %v) with random character changes.", uid, target.Ipid()))
	addToBuffer(client, "CMD", fmt.Sprintf("Cursed UID %d (IPID %v) with random-char.", uid, target.Ipid()), true)
}

// cmdUnCurseRandomChar handles /uncurserandomchar <uid>. ADMIN only.
func cmdUnCurseRandomChar(client *Client, args []string, _ string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %d does not exist.", uid))
		return
	}

	dbErr := db.RemoveRandomCharCurse(target.Ipid())
	target.disarmCurseRandomChar()
	if dbErr != nil {
		if errors.Is(dbErr, sql.ErrNoRows) {
			client.SendServerMessage(fmt.Sprintf("UID %d is not currently cursed.", uid))
			return
		}
		client.SendServerMessage(fmt.Sprintf("Failed to remove curse: %v", dbErr))
		return
	}

	target.SendServerMessage("The random-character curse has been lifted from you.")
	client.SendServerMessage(fmt.Sprintf("Removed the random-char curse from UID %d.", uid))
	addToBuffer(client, "CMD", fmt.Sprintf("Un-cursed UID %d (IPID %v) from random-char.", uid, target.Ipid()), true)
}
