/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork addition: /admin hide|unhide|status.

   Lets an admin hide their ADMIN role from other moderators in /players and
   /gas. This is purely cosmetic: permissions, and thus every admin-only
   command, are completely unaffected. The target's UID, showname and IPID
   are always still shown -- only the "Mod: <name>" line is suppressed for
   viewers who aren't themselves an admin, mirroring how a shadow mod is
   already hidden from non-admin viewers (see writeEntry in
   commands_moderation.go).

   The setting is keyed by the moderator's logged-in account name rather than
   by *Client, so it survives a reconnect or re-login as the same account --
   it is cleared only by an explicit /admin unhide or a server restart (the
   map below is in-memory only and is never persisted to disk). */

package athena

import (
	"fmt"
	"strings"
	"sync"
)

var (
	adminHideMu         sync.RWMutex
	adminHiddenAccounts = make(map[string]bool)
)

// setAdminHidden hides or reveals modName's ADMIN role. A blank modName is a
// no-op since an unauthenticated connection can never hold ADMIN anyway.
func setAdminHidden(modName string, hidden bool) {
	key := strings.ToLower(strings.TrimSpace(modName))
	if key == "" {
		return
	}
	adminHideMu.Lock()
	defer adminHideMu.Unlock()
	if hidden {
		adminHiddenAccounts[key] = true
	} else {
		delete(adminHiddenAccounts, key)
	}
}

// isAdminHidden reports whether modName has hidden its ADMIN role from
// non-admin viewers via /admin hide.
func isAdminHidden(modName string) bool {
	if modName == "" {
		return false
	}
	adminHideMu.RLock()
	defer adminHideMu.RUnlock()
	return adminHiddenAccounts[strings.ToLower(modName)]
}

// cmdAdmin handles /admin hide|unhide|status.
func cmdAdmin(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "hide":
		setAdminHidden(client.ModName(), true)
		client.SendServerMessage("Your ADMIN role is now hidden from other moderators in /players and /gas " +
			"(your UID, showname and IPID are unaffected). This lasts until /admin unhide or a server restart.")
		addToBuffer(client, "CMD", "Enabled admin-hide mode.", false)
	case "unhide":
		setAdminHidden(client.ModName(), false)
		client.SendServerMessage("Your ADMIN role is visible to other moderators again.")
		addToBuffer(client, "CMD", "Disabled admin-hide mode.", false)
	case "status":
		state := "visible"
		if isAdminHidden(client.ModName()) {
			state = "hidden"
		}
		client.SendServerMessage(fmt.Sprintf("Your ADMIN role is currently %s to other moderators.", state))
	default:
		client.SendServerMessage("Invalid argument:\n" + usage)
	}
}
