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
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// Lockdown whitelist passkeys let a player blocked by lockdown -- rejected at
// connect, or kicked by the playtime purge in server.go -- prove who they are
// to staff without either side needing to already know the other: the
// blocked-connection message hands them a passkey of the form
// "<their IPID>:<HMAC-SHA256(secretseed, IPID), truncated, hex>". Anyone
// holding a valid passkey has necessarily been issued it by this server (only
// the server knows secretseed), so /lockdown whitelist <passkey> can trust it
// on sight -- no pending-request table, no lookup, just recompute the MAC and
// compare. Redeeming a passkey both whitelists the IPID for lockdown's
// join-gate (same effect as /lockdown add) and permanently exempts it from
// the playtime purge/silence from then on, persisted in LOCKDOWN_EXEMPT
// (internal/db/db.go) the same way /musicban and /shadowdisconnect persist by
// IPID.
//
// The IPID half of the passkey is not itself secret -- moderators already see
// IPIDs everywhere (/getban, /gas, area logs, the [BAN] account-link alert)
// -- only the signature needs to be unforgeable, so there's no need for the
// token to hide it. The whole system is inert (SecretSeed == "") until an
// operator sets secretseed in config.toml.

const lockdownPasskeySigBytes = 16 // 128-bit truncated HMAC-SHA256: short enough to paste comfortably, far too large to brute-force

var (
	lockdownExemptMu sync.RWMutex
	lockdownExempt   = map[string]struct{}{}
)

// initLockdownExempt seeds the in-memory lockdown-exempt set from the
// database. Called once during server startup after the DB is opened. A DB
// error is logged but non-fatal -- the in-memory set just stays empty and
// entries repopulate as staff re-verifies passkeys.
func initLockdownExempt() {
	rows, err := db.ListLockdownExempts()
	if err != nil {
		logger.LogErrorf("lockdown: failed to load existing passkey exemptions from DB: %v", err)
		return
	}
	lockdownExemptMu.Lock()
	for _, le := range rows {
		lockdownExempt[le.Ipid] = struct{}{}
	}
	n := len(lockdownExempt)
	lockdownExemptMu.Unlock()
	if n > 0 {
		logger.LogInfof("lockdown: loaded %d passkey exemption(s) from database.", n)
	}
}

// isLockdownExempt reports whether the given IPID has a verified lockdown
// whitelist passkey on file, permanently exempting it from the playtime
// purge/silence and from the join-gate block, regardless of actual playtime.
// Hot path: one RLock + map lookup, no DB query.
func isLockdownExempt(ipid string) bool {
	lockdownExemptMu.RLock()
	_, ok := lockdownExempt[ipid]
	lockdownExemptMu.RUnlock()
	return ok
}

func addLockdownExemptMemory(ipid string) {
	lockdownExemptMu.Lock()
	lockdownExempt[ipid] = struct{}{}
	lockdownExemptMu.Unlock()
}

func removeLockdownExemptMemory(ipid string) {
	lockdownExemptMu.Lock()
	delete(lockdownExempt, ipid)
	lockdownExemptMu.Unlock()
}

// lockdownPasskeyEnabled reports whether the passkey system is configured.
func lockdownPasskeyEnabled() bool {
	return config.SecretSeed != ""
}

// lockdownPasskeyFor derives the deterministic passkey for ipid. Callers must
// check lockdownPasskeyEnabled first; this returns garbage (an unverifiable
// signature over an empty key) if SecretSeed is unset.
func lockdownPasskeyFor(ipid string) string {
	mac := hmac.New(sha256.New, []byte(config.SecretSeed))
	mac.Write([]byte(ipid))
	sig := mac.Sum(nil)[:lockdownPasskeySigBytes]
	return ipid + ":" + hex.EncodeToString(sig)
}

// lockdownPasskeyVerify checks a passkey produced by lockdownPasskeyFor and,
// if it's valid, returns the IPID it certifies. IPIDs (base64-encoded MD5
// hashes, see getIpid) never contain ':', so splitting on the first one
// unambiguously recovers the claimed IPID and signature.
func lockdownPasskeyVerify(passkey string) (string, bool) {
	if !lockdownPasskeyEnabled() {
		return "", false
	}
	idx := strings.Index(passkey, ":")
	if idx <= 0 || idx == len(passkey)-1 {
		return "", false
	}
	ipid, sigHex := passkey[:idx], passkey[idx+1:]
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(config.SecretSeed))
	mac.Write([]byte(ipid))
	expected := mac.Sum(nil)[:lockdownPasskeySigBytes]
	if !hmac.Equal(sig, expected) {
		return "", false
	}
	return ipid, true
}

// lockdownPasskeyMessage returns the operator-configured explanation shown to
// a player blocked by lockdown, with their passkey filled in. Callers must
// check lockdownPasskeyEnabled first.
func lockdownPasskeyMessage(ipid string) string {
	id := lockdownPasskeyFor(ipid)
	tmpl := config.LockdownString
	if strings.Contains(tmpl, "{id}") {
		return strings.ReplaceAll(tmpl, "{id}", id)
	}
	if tmpl == "" {
		return "This is not a ban. Give this code to staff to be let in: " + id
	}
	return tmpl + " " + id
}

// lockdownRejectionMessage builds the message shown to an IPID rejected by
// lockdown's join-gate: the base "server is in lockdown" text, plus (when the
// passkey system is configured) their passkey and instructions to relay it to
// staff. Falls back to just base when the passkey system is disabled.
func lockdownRejectionMessage(base, ipid string) string {
	if !lockdownPasskeyEnabled() {
		return base
	}
	return base + " " + lockdownPasskeyMessage(ipid)
}

// cmdLockdownWhitelistPasskey handles the "/lockdown whitelist <passkey>"
// form: verifies the passkey, then whitelists the IPID it certifies for
// lockdown's join-gate and permanently exempts it from the playtime purge.
func cmdLockdownWhitelistPasskey(client *Client, passkey string) {
	if !lockdownPasskeyEnabled() {
		client.SendServerMessage("Lockdown passkeys are not configured on this server (secretseed is unset in config.toml).")
		return
	}
	ipid, ok := lockdownPasskeyVerify(passkey)
	if !ok {
		client.SendServerMessage("Invalid lockdown passkey.")
		return
	}
	recordIPFirstSeen(ipid)
	if err := db.MarkIPKnown(ipid); err != nil {
		logger.LogErrorf("lockdown whitelist: failed to mark IPID %s known: %v", ipid, err)
	}
	if err := db.AddLockdownExempt(ipid, client.StoredModName(), time.Now().UTC().Unix()); err != nil {
		client.SendServerMessage(fmt.Sprintf("Verified the passkey, but failed to persist the exemption: %v", err))
		return
	}
	addLockdownExemptMemory(ipid)
	msg := fmt.Sprintf("Passkey verified for IPID %v. They're whitelisted for lockdown and permanently exempt from the playtime purge from now on.", ipid)
	client.SendServerMessage(msg)
	addToBuffer(client, "CMD", fmt.Sprintf("Verified lockdown passkey for IPID %v; whitelisted and exempted from the playtime purge.", ipid), true)
}

// cmdLockdownUnwhitelist handles "/lockdown unwhitelist <uid|ipid>": removes
// a previously-verified passkey exemption. Accepts a connected target's UID
// or a raw IPID so an offline IPID's exemption can still be revoked.
func cmdLockdownUnwhitelist(client *Client, arg string) {
	ipid := arg
	if uid, err := strconv.Atoi(arg); err == nil {
		target := clients.GetClientByUID(uid)
		if target == nil {
			client.SendServerMessage("No client found with that UID.")
			return
		}
		ipid = target.Ipid()
	}
	if err := db.RemoveLockdownExempt(ipid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			client.SendServerMessage(fmt.Sprintf("IPID %v does not have a lockdown passkey exemption.", ipid))
			return
		}
		client.SendServerMessage(fmt.Sprintf("Failed to remove the exemption: %v", err))
		return
	}
	removeLockdownExemptMemory(ipid)
	client.SendServerMessage(fmt.Sprintf("Removed the lockdown passkey exemption for IPID %v.", ipid))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed lockdown passkey exemption for IPID %v.", ipid), true)
}

// cmdLockdownExemptList handles "/lockdown exemptlist": lists every IPID
// currently exempt from the lockdown playtime purge via a verified passkey.
func cmdLockdownExemptList(client *Client) {
	rows, err := db.ListLockdownExempts()
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to read the exemption list: %v", err))
		return
	}
	if len(rows) == 0 {
		client.SendServerMessage("No active lockdown passkey exemptions.")
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Lockdown passkey exemptions (%d):\n", len(rows))
	for _, le := range rows {
		when := time.Unix(le.IssuedAt, 0).UTC().Format("2006-01-02 15:04 MST")
		fmt.Fprintf(&sb, "  • %v — verified %v by %v\n", le.Ipid, when, le.IssuedBy)
	}
	client.SendServerMessage(strings.TrimRight(sb.String(), "\n"))
}
