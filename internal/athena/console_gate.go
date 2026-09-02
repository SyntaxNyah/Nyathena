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
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// A small set of commands are powerful in a way that is invisible to the person
// they are used on: possession speaks as somebody and silences them so they
// cannot say otherwise, and a shadow-disconnect looks to its target exactly
// like a bad network. Holding ADMIN is no longer enough to use any of them.
// Each use has to be armed first from the server console, and arming it grants
// exactly one use.
//
// Console is the right authority for this because it is the one place that
// cannot be reached over the network at all: it needs shell access to the host,
// which is a different and stronger thing than any in-game credential. It is
// the same reasoning behind the console-only `punishment disable` kill switch,
// and this is deliberately the mirror image of that one -- there, console is
// the only thing that can take a power away; here, it is the only thing that
// can hand one out.
//
// A grant is consumed on the command actually taking effect, not on the attempt,
// so mistyping a UID costs nothing and the console operator is not re-arming for
// typos. It is one authorised *action*, not one keystroke.

// consoleGrantTTL bounds how long an unused grant stays armed. Without it, a
// grant armed and then forgotten would sit there indefinitely and the power
// would be continuously available again -- the exact thing this gate exists to
// prevent. Five minutes is long enough to arm one, walk to the other machine
// and use it.
const consoleGrantTTL = 5 * time.Minute

// consoleGatedCommands is the set of in-game commands that require a console
// grant, mapped to a short description shown by `grant status`.
//
// Both possession aliases are listed. /truepossess and /fullpossess are the
// same handler behind two names, so gating one without the other would gate
// nothing.
var consoleGatedCommands = map[string]string{
	"possess":          "speak one IC message as another player",
	"fullpossess":      "become a player and silence them until /unpossess",
	"truepossess":      "become a player and silence them until /unpossess",
	"shadowdisconnect": "stealth-ban an IPID with no message, ever",
}

var consoleGrants = struct {
	mu     sync.Mutex
	armed  map[string]time.Time // command -> when the grant expires
	nowFor func() time.Time     // overridable in tests
}{
	armed:  make(map[string]time.Time),
	nowFor: time.Now,
}

func consoleGrantNow() time.Time {
	if consoleGrants.nowFor != nil {
		return consoleGrants.nowFor()
	}
	return time.Now()
}

// isConsoleGated reports whether cmd needs a grant before it can run.
func isConsoleGated(cmd string) bool {
	_, ok := consoleGatedCommands[strings.ToLower(cmd)]
	return ok
}

// grantConsoleUse arms exactly one use of cmd. Re-arming an already-armed
// command refreshes its window rather than stacking a second use: the gate's
// whole claim is that one arming buys one use, and a console operator who
// types the command twice should not silently be handing out two.
func grantConsoleUse(cmd string) (ok bool, err string) {
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	if !isConsoleGated(cmd) {
		return false, fmt.Sprintf("%q is not a console-gated command. Gated: %v",
			cmd, consoleGatedCommandNames())
	}
	consoleGrants.mu.Lock()
	already := false
	if exp, found := consoleGrants.armed[cmd]; found && consoleGrantNow().Before(exp) {
		already = true
	}
	consoleGrants.armed[cmd] = consoleGrantNow().Add(consoleGrantTTL)
	consoleGrants.mu.Unlock()

	verb := "Armed"
	if already {
		verb = "Re-armed (window refreshed, still one use)"
	}
	logger.LogInfof("%v one use of /%v. Expires in %v if unused.", verb, cmd, consoleGrantTTL)
	logger.WriteAudit(fmt.Sprintf("%v | CONSOLE_GRANT | ARMED | /%v | ttl=%v",
		time.Now().UTC().Format("15:04:05"), cmd, consoleGrantTTL))
	return true, ""
}

// revokeConsoleUse disarms cmd, or every command when cmd is "all".
func revokeConsoleUse(cmd string) (n int, err string) {
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	consoleGrants.mu.Lock()
	defer consoleGrants.mu.Unlock()
	if cmd == "all" {
		n = len(consoleGrants.armed)
		consoleGrants.armed = make(map[string]time.Time)
		logger.WriteAudit(fmt.Sprintf("%v | CONSOLE_GRANT | REVOKED | all (%d)",
			time.Now().UTC().Format("15:04:05"), n))
		return n, ""
	}
	if !isConsoleGated(cmd) {
		return 0, fmt.Sprintf("%q is not a console-gated command. Gated: %v",
			cmd, consoleGatedCommandNames())
	}
	if _, found := consoleGrants.armed[cmd]; !found {
		return 0, ""
	}
	delete(consoleGrants.armed, cmd)
	logger.WriteAudit(fmt.Sprintf("%v | CONSOLE_GRANT | REVOKED | /%v",
		time.Now().UTC().Format("15:04:05"), cmd))
	return 1, ""
}

// consoleGrantStatus reports every gated command and whether it is armed, for
// the console's `grant status`.
func consoleGrantStatus() []string {
	consoleGrants.mu.Lock()
	defer consoleGrants.mu.Unlock()
	now := consoleGrantNow()
	out := make([]string, 0, len(consoleGatedCommands))
	for _, cmd := range consoleGatedCommandNames() {
		state := "locked"
		if exp, found := consoleGrants.armed[cmd]; found {
			if now.Before(exp) {
				state = fmt.Sprintf("ARMED (one use, %v left)", exp.Sub(now).Truncate(time.Second))
			} else {
				state = "locked (grant expired unused)"
			}
		}
		out = append(out, fmt.Sprintf("  /%-17s %-50s %v", cmd, consoleGatedCommands[cmd], state))
	}
	return out
}

// consoleGatedCommandNames lists the gated commands in a stable order.
func consoleGatedCommandNames() []string {
	names := make([]string, 0, len(consoleGatedCommands))
	for c := range consoleGatedCommands {
		names = append(names, c)
	}
	sort.Strings(names)
	return names
}

// grantIsArmed reports whether cmd currently has a live, unexpired grant,
// without spending it.
func grantIsArmed(cmd string) bool {
	consoleGrants.mu.Lock()
	defer consoleGrants.mu.Unlock()
	exp, found := consoleGrants.armed[strings.ToLower(cmd)]
	return found && consoleGrantNow().Before(exp)
}

// consumeConsoleGrant is the gate itself, called from a gated handler once its
// arguments have validated and immediately before it takes effect.
//
// It returns false and tells the caller why when no grant is armed. On success
// the grant is spent, so the next invocation needs a fresh one from the console.
//
// Placement matters: call this after validating the target, so a mistyped UID
// does not burn a grant, and before anything observable happens, so a refused
// command leaves no trace on its target.
func consumeConsoleGrant(client *Client, cmd string) bool {
	cmd = strings.ToLower(cmd)
	consoleGrants.mu.Lock()
	exp, found := consoleGrants.armed[cmd]
	live := found && consoleGrantNow().Before(exp)
	if live {
		delete(consoleGrants.armed, cmd)
	} else if found {
		// Expired: clear it so status stops reporting a stale entry.
		delete(consoleGrants.armed, cmd)
	}
	consoleGrants.mu.Unlock()

	if !live {
		if client != nil {
			client.SendServerMessage(fmt.Sprintf(
				"/%v is locked. It can only be used once the server console arms it, "+
					"and each arming is good for a single use. Ask whoever has console "+
					"access to run: grant %v", cmd, cmd))
		}
		who, uid := "console", -1
		if client != nil {
			who, uid = client.Ipid(), client.Uid()
		}
		logger.LogInfof("Refused /%v: no console grant armed (IPID:%v UID:%v)", cmd, who, uid)
		logger.WriteAudit(fmt.Sprintf("%v | CONSOLE_GRANT | REFUSED | /%v | IPID:%v | UID:%v",
			time.Now().UTC().Format("15:04:05"), cmd, who, uid))
		return false
	}

	who, uid := "console", -1
	if client != nil {
		who, uid = client.Ipid(), client.Uid()
	}
	logger.LogInfof("Console grant for /%v consumed by IPID:%v UID:%v. Re-locked.", cmd, who, uid)
	logger.WriteAudit(fmt.Sprintf("%v | CONSOLE_GRANT | CONSUMED | /%v | IPID:%v | UID:%v",
		time.Now().UTC().Format("15:04:05"), cmd, who, uid))
	return true
}

// resetConsoleGrants clears all state. Test helper.
func resetConsoleGrants() {
	consoleGrants.mu.Lock()
	defer consoleGrants.mu.Unlock()
	consoleGrants.armed = make(map[string]time.Time)
	consoleGrants.nowFor = time.Now
}
