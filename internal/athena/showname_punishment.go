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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// punishmentNamesFile lists shownames (or substrings of them) that trigger the
// showname punisher: the first IC message sent under a matching showname
// "stains" the speaker's IPID, and while stained they receive a fresh random
// punishment (from the megamaso pool) every minute. Unlike censored_names.txt
// this doesn't silence anyone — it just keeps piling comedy effects onto them
// until a moderator lifts everything with /unpunish.
//
// The stain sticks to the IPID, not the showname: switching to a clean
// showname while stained does NOT stop the drip. Only /unpunish (all-form)
// clears the stain — after that, speaking under a clean showname is fine, and
// only using a listed showname again re-triggers it.
//
// Like censored_names.txt the file is optional (missing file = feature off),
// independent of automod_enabled, and hot-reloadable via /reload.
const punishmentNamesFile = "punishment_names.txt"

const (
	// shownamePunishInterval is how often a stained IPID receives another
	// random punishment while connected.
	shownamePunishInterval = time.Minute
	// shownamePunishEffectDuration is how long each individual dripped
	// punishment lasts. With a one-minute drip this settles at roughly ten
	// concurrently active effects — chaotic but bounded.
	shownamePunishEffectDuration = 10 * time.Minute
)

// shownamePunishStains is the in-memory registry of stained IPIDs. The value
// records which punishment_names.txt entry triggered the stain (for messages
// and logs) and when the last punishment was dripped, so multiple connections
// sharing one IPID never drip faster than shownamePunishInterval combined.
//
// The stain deliberately lives in memory keyed by IPID (not on the *Client):
// it survives reconnects and showname changes for the server's lifetime, and
// the punishments themselves persist in the DB like every other punishment.
var shownamePunishStains = struct {
	mu  sync.Mutex
	set map[string]*shownameStain
}{set: make(map[string]*shownameStain)}

type shownameStain struct {
	matched     string    // the normalized punishment_names.txt entry that fired
	lastApplied time.Time // last time a punishment was dripped onto this IPID
}

// initShownamePunisher loads punishment_names.txt at startup. A missing file
// is not an error: checkPunishmentShowname gates on an empty list, so the
// feature is simply inactive until the file exists and the server is started
// or reloaded.
func initShownamePunisher() {
	path := filepath.Join(settings.ConfigPath, punishmentNamesFile)
	names, err := loadWordListFile(path)
	if err != nil {
		return
	}
	setPunishmentNames(names)
	logger.LogInfof("showname punisher: loaded %d punishment name(s) from %q", len(names), path)
}

// matchPunishmentName performs a substring search of s (expected to already be
// normalizeForFilter'd) against every punishment_names.txt entry. Empty
// entries are skipped for the same reason as matchCensoredName: "" is a
// substring of everything and would stain every speaker unconditionally.
func matchPunishmentName(s string) (string, bool) {
	for _, name := range getPunishmentNames() {
		if name == "" {
			continue
		}
		if strings.Contains(s, name) {
			return name, true
		}
	}
	return "", false
}

// isShownamePunishStained reports whether the IPID is currently stained.
func isShownamePunishStained(ipid string) bool {
	shownamePunishStains.mu.Lock()
	_, ok := shownamePunishStains.set[ipid]
	shownamePunishStains.mu.Unlock()
	return ok
}

// unstainShownamePunish clears the stain from an IPID. Called from /unpunish
// (full-removal forms); any running watcher goroutines notice on their next
// tick and exit. Returns whether a stain was actually present.
func unstainShownamePunish(ipid string) bool {
	shownamePunishStains.mu.Lock()
	_, ok := shownamePunishStains.set[ipid]
	delete(shownamePunishStains.set, ipid)
	shownamePunishStains.mu.Unlock()
	return ok
}

// checkPunishmentShowname tests a decoded showname against
// punishment_names.txt. On a match the speaker's IPID is stained (idempotent),
// one punishment is dripped immediately, and the per-connection drip watcher
// is armed. Called from pktIC for every IC message carrying a showname; the
// empty-list early-out keeps it near-free when the feature is unused.
func checkPunishmentShowname(client *Client, showname string) {
	if showname == "" || len(getPunishmentNames()) == 0 {
		return
	}
	ipid := client.Ipid()
	if isShownamePunishStained(ipid) {
		// Already stained (e.g. reconnected mid-stain and speaking again):
		// just make sure this connection's watcher is running.
		client.armShownamePunishWatcher()
		return
	}
	matched, ok := matchPunishmentName(normalizeForFilter(showname))
	if !ok {
		return
	}

	shownamePunishStains.mu.Lock()
	shownamePunishStains.set[ipid] = &shownameStain{matched: matched}
	shownamePunishStains.mu.Unlock()

	logger.LogInfof("showname punisher: stained %v (uid %d, showname %q) — matched entry %q", ipid, client.Uid(), showname, matched)
	client.SendServerMessage(fmt.Sprintf(
		"⚡ The showname %q is on this server's punishment list. A random punishment will now strike you every minute — changing your showname won't save you; only a moderator can lift it with /unpunish.", showname))

	dripShownamePunishment(client)
	client.armShownamePunishWatcher()
}

// dripShownamePunishment applies one random punishment from the megamaso pool
// to the client and persists it, respecting shownamePunishInterval per IPID so
// multiclients sharing a stained IPID don't multiply the drip rate. The very
// first drip (zero lastApplied) always fires.
func dripShownamePunishment(client *Client) {
	ipid := client.Ipid()

	shownamePunishStains.mu.Lock()
	stain, ok := shownamePunishStains.set[ipid]
	if !ok {
		shownamePunishStains.mu.Unlock()
		return
	}
	if !stain.lastApplied.IsZero() && time.Since(stain.lastApplied) < shownamePunishInterval-time.Second {
		shownamePunishStains.mu.Unlock()
		return
	}
	stain.lastApplied = time.Now()
	matched := stain.matched
	shownamePunishStains.mu.Unlock()

	// Pick a punishment the target isn't already wearing, falling back to
	// "any" if the whole pool is somehow already applied — same roll /megamaso
	// and /minefield use.
	var pick PunishmentType
	for tries := 0; tries < 16; tries++ {
		candidate := megamasoStackPool[rand.Intn(len(megamasoStackPool))]
		if !client.HasPunishment(candidate) {
			pick = candidate
			break
		}
	}
	if pick == PunishmentNone {
		pick = megamasoStackPool[rand.Intn(len(megamasoStackPool))]
	}

	reason := fmt.Sprintf("Punished showname (matched %q)", matched)
	client.AddPunishmentBy(pick, shownamePunishEffectDuration, reason, IssuerSystem)
	expires := time.Now().UTC().Add(shownamePunishEffectDuration).Unix()
	if err := db.UpsertTextPunishmentBy(ipid, int(pick), expires, reason, int(IssuerSystem)); err != nil {
		logger.LogErrorf("showname punisher: failed to persist punishment for %v: %v", ipid, err)
	}
	client.SendServerMessage(fmt.Sprintf("⚡ Random punishment applied: '%v' (%v). Your cursed showname strikes again.", pick.String(), shownamePunishEffectDuration))
	logger.LogInfof("showname punisher: dripped '%v' onto %v (uid %d)", pick.String(), ipid, client.Uid())
}

// armShownamePunishWatcher lazily starts the per-connection drip goroutine.
// Idempotent — the CAS gate guarantees at most one watcher per connection, so
// re-arming on every matching IC message or on reconnect never double-spawns.
func (client *Client) armShownamePunishWatcher() {
	if client.shownamePunishWatcherStarted.CompareAndSwap(false, true) {
		go client.shownamePunishWatch()
	}
}

// shownamePunishWatch drips one random punishment per minute onto the client
// while their IPID remains stained. Mirrors the leak-free shape of
// curseRandomCharWatch: selecting on client.done guarantees the goroutine can
// never outlive the connection, and an /unpunish clearing the stain makes the
// next tick exit on its own.
func (client *Client) shownamePunishWatch() {
	defer client.shownamePunishWatcherStarted.Store(false)
	for {
		timer := time.NewTimer(shownamePunishInterval)
		select {
		case <-client.done:
			timer.Stop()
			return
		case <-timer.C:
			if !isShownamePunishStained(client.Ipid()) {
				return
			}
			dripShownamePunishment(client)
		}
	}
}

// restoreShownamePunishStain re-arms the drip watcher after a (re)connect if
// the joining client's IPID is still stained. Called once after a client
// successfully joins, alongside restorePunishments — so relogging (or
// switching to a clean showname) cannot be used to escape the drip.
func (client *Client) restoreShownamePunishStain() {
	if !isShownamePunishStained(client.Ipid()) {
		return
	}
	client.armShownamePunishWatcher()
	client.SendServerMessage("⚡ Your IPID is still marked by the showname punishment list — random punishments keep coming every minute until a moderator lifts them with /unpunish.")
}
