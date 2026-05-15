/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: /reversename and /unreversename.

   Mirrors /forcename and /nameshuffle but flips the rune order of a player's
   effective showname ("Phoenix" -> "xineohP"). It works on a single UID, a
   comma-separated UID list, or every player in the caller's area via the
   "global" keyword.

   The forced-showname value in effect before the flip is remembered per client
   so /unreversename restores it exactly — even when the reverse was stacked on
   top of a /forcename. */

package athena

import (
	"fmt"
	"strconv"
	"strings"
)

// ReverseShowname flips the rune order of the client's effective showname and
// stores the result as a forced showname. The forced-showname value in effect
// beforehand is captured so RestoreShowname can put it back exactly. Returns
// the new decoded (display-form) showname and true on success; returns "",
// false when the name is already reversed or the effective showname is empty
// (nothing to flip).
func (client *Client) ReverseShowname() (string, bool) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.nameReversed {
		return "", false
	}
	// Inline EffectiveShowname under the same lock: forced name wins if set.
	current := client.forcedShowname
	if current == "" {
		current = client.showname
	}
	// forcedShowname/showname are stored AO2-encoded; decode before flipping so
	// escape sequences (<num>, <percent>, ...) are not split, then re-encode.
	plain := decode(current)
	if plain == "" {
		return "", false
	}
	reversed := reverseRunes(plain)
	client.preReverseShowname = client.forcedShowname
	client.forcedShowname = encode(reversed)
	client.nameReversed = true
	return reversed, true
}

// RestoreShowname undoes a ReverseShowname, restoring the forced showname that
// was in effect beforehand. Returns the decoded effective showname and true on
// success; returns "", false when the client's name was not reversed.
func (client *Client) RestoreShowname() (string, bool) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if !client.nameReversed {
		return "", false
	}
	client.forcedShowname = client.preReverseShowname
	client.preReverseShowname = ""
	client.nameReversed = false
	if client.forcedShowname != "" {
		return decode(client.forcedShowname), true
	}
	return decode(client.showname), true
}

// reverseNameTargets resolves the target argument shared by /reversename and
// /unreversename. "global" (any case) yields every joined client in the
// caller's area; otherwise the argument is a comma-separated UID list resolved
// server-wide, mirroring the punishment commands.
func reverseNameTargets(client *Client, arg string) []*Client {
	if strings.EqualFold(arg, "global") {
		targetArea := client.Area()
		var l []*Client
		clients.ForEach(func(c *Client) {
			if c.Uid() != -1 && c.Area() == targetArea {
				l = append(l, c)
			}
		})
		return l
	}
	return getUidList(strings.Split(arg, ","))
}

// cmdReverseName flips the showname of one or more players, or every player in
// the caller's area when "global" is given.
func cmdReverseName(client *Client, args []string, _ string) {
	targets := reverseNameTargets(client, args[0])
	if len(targets) == 0 {
		client.SendServerMessage("No matching players found. Specify a UID, a comma-separated UID list, or \"global\".")
		return
	}

	var count int
	var report strings.Builder
	for _, c := range targets {
		reversed, changed := c.ReverseShowname()
		if !changed {
			continue
		}
		writeToAll("PU", strconv.Itoa(c.Uid()), "2", reversed)
		c.SendServerMessage("A moderator has reversed your showname.")
		count++
		if report.Len() > 0 {
			report.WriteString(", ")
		}
		fmt.Fprintf(&report, "%d", c.Uid())
	}
	if count == 0 {
		client.SendServerMessage("No shownames were reversed — the target(s) are already reversed or have no showname set.")
		return
	}
	client.SendServerMessage(fmt.Sprintf("Reversed the showname of %d player(s).", count))
	addToBuffer(client, "CMD", fmt.Sprintf("reversed shownames of %v", report.String()), true)
}

// cmdUnreverseName restores shownames flipped by /reversename, undoing a single
// UID, a UID list, or the whole area when "global" is given.
func cmdUnreverseName(client *Client, args []string, _ string) {
	targets := reverseNameTargets(client, args[0])
	if len(targets) == 0 {
		client.SendServerMessage("No matching players found. Specify a UID, a comma-separated UID list, or \"global\".")
		return
	}

	var count int
	var report strings.Builder
	for _, c := range targets {
		restored, changed := c.RestoreShowname()
		if !changed {
			continue
		}
		writeToAll("PU", strconv.Itoa(c.Uid()), "2", restored)
		c.SendServerMessage("A moderator has restored your showname.")
		count++
		if report.Len() > 0 {
			report.WriteString(", ")
		}
		fmt.Fprintf(&report, "%d", c.Uid())
	}
	if count == 0 {
		client.SendServerMessage("No shownames were restored — none of the target(s) had a reversed showname.")
		return
	}
	client.SendServerMessage(fmt.Sprintf("Restored the showname of %d player(s).", count))
	addToBuffer(client, "CMD", fmt.Sprintf("restored reversed shownames of %v", report.String()), true)
}
