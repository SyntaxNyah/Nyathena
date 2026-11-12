/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork addition: /toggle global -- a self-service /global-hide toggle. */

package athena

import "strings"

// Self-service /global-hide toggle (/toggle global). Some players find the
// constant stream of server-wide /global (and /g) broadcasts distracting and
// would rather just not see them. This mutes only those broadcasts for the
// toggling client: local (area) OOC stays visible, and the toggle never
// touches what anyone else sees. It defaults back to visible on every fresh
// connection. Direct messages (/pm) and staff/system broadcasts (/mod -g,
// /modchat, /announce, moderation alerts) are deliberately untouched -- see
// broadcastOOCToAll in server.go, the only call site that consults this flag.

// GlobalHidden reports whether this client has muted player-initiated
// /global broadcasts for their current session via /toggle global.
func (client *Client) GlobalHidden() bool {
	return client.globalHidden.Load()
}

// SetGlobalHidden mutes or unmutes player-initiated /global broadcasts for
// this client's session.
func (client *Client) SetGlobalHidden(hidden bool) {
	client.globalHidden.Store(hidden)
}

// cmdToggle handles /toggle <global>. Currently the only supported setting
// is "global", which flips /global-broadcast hiding on/off for the caller;
// running it again switches it back.
func cmdToggle(client *Client, args []string, usage string) {
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "global":
		if client.GlobalHidden() {
			client.SetGlobalHidden(false)
			client.SendServerMessage("🔊 /global and /g broadcasts are now visible again.")
		} else {
			client.SetGlobalHidden(true)
			client.SendServerMessage("🔇 /global and /g broadcasts are now hidden. Local OOC, direct messages and staff announcements still get through. Run /toggle global again to show them.")
		}
	default:
		client.SendServerMessage("Invalid argument:\n" + usage)
	}
}
