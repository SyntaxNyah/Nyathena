/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: /randomarea — a punishment that periodically
   force-warps the target to a random open area, independent of anything
   they say or do. It belongs alongside /contagious and /minefield as a
   punishment that "moves on its own": those act on a per-message roll or
   spread window, this one acts on a per-connection timer instead, since
   there is no IC/OOC message to hook an area-change off of. */

package athena

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// randomAreaMinWait and randomAreaMaxWait bound the delay between forced
// area-warps while /randomarea is active on a connection. They are vars
// rather than consts purely so tests can shrink them to exercise the watcher
// loop without waiting out a real 20-45 second tick.
var (
	randomAreaMinWait = 20 * time.Second
	randomAreaMaxWait = 45 * time.Second
)

// armRandomAreaWatch lazily starts the per-connection watcher goroutine that
// performs the actual warps. CAS-gated so re-applying the punishment — a mod
// refreshing the duration, /stack, or restorePunishments re-arming it on
// reconnect — never spawns a second goroutine racing the first.
func (client *Client) armRandomAreaWatch() {
	if client.randomAreaWatcherStarted.CompareAndSwap(false, true) {
		go client.randomAreaWatch()
	}
}

// randomAreaWatch repeatedly force-warps the client to a random open area on
// a random interval, until the punishment expires or is /unpunish-ed
// (HasActivePunishment goes false) or the connection closes (client.done).
// Mirrors the leak-free shape of curseRandomCharWatch/shownamePunishWatch:
// selecting on client.done guarantees this goroutine can never outlive the
// connection, regardless of how long the punishment itself is supposed to
// last.
func (client *Client) randomAreaWatch() {
	defer client.randomAreaWatcherStarted.Store(false)
	for {
		wait := randomAreaMinWait
		if span := randomAreaMaxWait - randomAreaMinWait; span > 0 {
			wait += time.Duration(rand.Int63n(int64(span) + 1))
		}
		timer := time.NewTimer(wait)
		select {
		case <-client.done:
			timer.Stop()
			return
		case <-timer.C:
			if !client.HasActivePunishment(PunishmentRandomArea) {
				return
			}
			// A carrier sheltering in a punishment-safe area (or the
			// console-only global kill switch being armed) is left alone
			// for this tick rather than treated as expired — the
			// punishment may still have time left once they leave the
			// safe area, or the switch may flip back on.
			if punishmentSafeArea(client.Area()) {
				continue
			}
			if dest := randomOtherArea(client.Area()); dest != nil {
				client.forceChangeArea(dest)
				client.SendServerMessage(fmt.Sprintf("💨 You've been randomly warped to %v!", dest.Name()))
			}
		}
	}
}

// randomOtherArea picks a random area other than current that is fully open
// — not /lock-ed, not /adminlock-ed, and not punishment-safe — so
// /randomarea can never dump its target into a room a CM deliberately
// restricted, or into a shelter meant to be free of punishment effects.
// Returns nil if no such area exists (e.g. a single-area server, or every
// other area currently restricted).
func randomOtherArea(current *area.Area) *area.Area {
	candidates := make([]*area.Area, 0, len(areas))
	for _, a := range areas {
		if a == current {
			continue
		}
		if a.Lock() != area.LockFree || a.AdminLocked() {
			continue
		}
		if punishmentSafeArea(a) {
			continue
		}
		candidates = append(candidates, a)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[rand.Intn(len(candidates))]
}

// cmdRandomArea handles /randomarea. Standard punishment command: requires
// MUTE, supports -d/-r/-h, comma-separated UIDs and `global`, and stacks and
// persists exactly like every other cmdPunishment-backed effect. Unlike most
// of them it does nothing on the target's own IC/OOC messages — the warps
// happen on their own via the watcher goroutine armed in AddPunishmentBy.
func cmdRandomArea(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRandomArea)
}
