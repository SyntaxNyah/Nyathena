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
	"testing"
)

// countMegamasoPoolPunishments counts how many megamaso-pool effects the
// client is currently wearing — the drip only ever picks from that pool.
func countMegamasoPoolPunishments(c *Client) int {
	n := 0
	for _, p := range megamasoStackPool {
		if c.HasPunishment(p) {
			n++
		}
	}
	return n
}

func TestMatchPunishmentName(t *testing.T) {
	orig := getPunishmentNames()
	t.Cleanup(func() { setPunishmentNames(orig) })
	setPunishmentNames([]string{"blacklistname", "troublemaker"})

	if _, ok := matchPunishmentName("xxblacklistnamexx"); !ok {
		t.Error("expected substring match against 'blacklistname' to fire")
	}
	if _, ok := matchPunishmentName("phoenix wright"); ok {
		t.Error("expected no match for an unrelated showname")
	}
}

// Same guard as matchCensoredName: an empty entry must never match, since
// strings.Contains treats "" as a substring of every showname.
func TestMatchPunishmentName_IgnoresEmptyEntry(t *testing.T) {
	orig := getPunishmentNames()
	t.Cleanup(func() { setPunishmentNames(orig) })
	setPunishmentNames([]string{"", "blacklistname"})

	if matched, ok := matchPunishmentName("phoenix wright"); ok {
		t.Errorf("matchPunishmentName unexpectedly matched empty entry (matched=%q)", matched)
	}
	if _, ok := matchPunishmentName("blacklistname"); !ok {
		t.Error("matchPunishmentName failed to catch the real entry once an empty entry was also present")
	}
}

// A matching showname stains the IPID and immediately drips exactly one
// random punishment from the megamaso pool.
func TestCheckPunishmentShowname_MatchStainsAndDrips(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	orig := getPunishmentNames()
	t.Cleanup(func() { setPunishmentNames(orig) })
	setPunishmentNames([]string{"blacklistname"})

	client := &Client{conn: &testConn{}, uid: 50, ipid: "ip-punishname-match"}
	t.Cleanup(func() { unstainShownamePunish(client.Ipid()) })

	checkPunishmentShowname(client, "BlacklistName1")

	if !isShownamePunishStained(client.Ipid()) {
		t.Fatal("expected the IPID to be stained after a matching showname")
	}
	if got := countMegamasoPoolPunishments(client); got != 1 {
		t.Errorf("expected exactly 1 dripped punishment immediately after staining, got %d", got)
	}
}

// The drip is throttled per IPID: a second drip attempt right after the first
// (e.g. from a second connection sharing the IPID) must not stack another
// punishment before the interval has elapsed.
func TestDripShownamePunishment_ThrottledPerInterval(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	orig := getPunishmentNames()
	t.Cleanup(func() { setPunishmentNames(orig) })
	setPunishmentNames([]string{"blacklistname"})

	client := &Client{conn: &testConn{}, uid: 51, ipid: "ip-punishname-throttle"}
	t.Cleanup(func() { unstainShownamePunish(client.Ipid()) })

	checkPunishmentShowname(client, "blacklistname1")
	dripShownamePunishment(client)
	dripShownamePunishment(client)

	if got := countMegamasoPoolPunishments(client); got != 1 {
		t.Errorf("expected the drip to be throttled to 1 punishment, got %d", got)
	}
}

// The stain sticks to the IPID: switching to a clean showname while stained
// keeps the stain, and clearing the stain (as /unpunish does) means a clean
// showname no longer re-triggers anything — only a listed one does.
func TestShownamePunishStain_SurvivesRenameUntilUnpunished(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	orig := getPunishmentNames()
	t.Cleanup(func() { setPunishmentNames(orig) })
	setPunishmentNames([]string{"blacklistname"})

	client := &Client{conn: &testConn{}, uid: 52, ipid: "ip-punishname-rename"}
	t.Cleanup(func() { unstainShownamePunish(client.Ipid()) })

	checkPunishmentShowname(client, "blacklistname1")
	if !isShownamePunishStained(client.Ipid()) {
		t.Fatal("expected the IPID to be stained")
	}

	// Renaming to a clean showname does not clear the stain.
	checkPunishmentShowname(client, "Totally Reformed")
	if !isShownamePunishStained(client.Ipid()) {
		t.Error("expected the stain to survive a showname change")
	}

	// /unpunish clears the stain; a clean showname then stays clean.
	if !unstainShownamePunish(client.Ipid()) {
		t.Error("expected unstainShownamePunish to report a cleared stain")
	}
	checkPunishmentShowname(client, "Totally Reformed")
	if isShownamePunishStained(client.Ipid()) {
		t.Error("expected a clean showname after unpunish to stay unstained")
	}

	// ...but using a listed showname again re-triggers the stain.
	checkPunishmentShowname(client, "blacklistname1")
	if !isShownamePunishStained(client.Ipid()) {
		t.Error("expected a listed showname to re-stain after unpunish")
	}
}

// A non-matching showname on an unstained IPID is a complete no-op, and an
// empty punishment list disables the feature outright.
func TestCheckPunishmentShowname_NoMatchAndEmptyListAreNoOps(t *testing.T) {
	defer setupShownameCensorTestDB(t)()

	orig := getPunishmentNames()
	t.Cleanup(func() { setPunishmentNames(orig) })

	setPunishmentNames([]string{"blacklistname"})
	client := &Client{conn: &testConn{}, uid: 53, ipid: "ip-punishname-clean"}
	checkPunishmentShowname(client, "Phoenix Wright")
	if isShownamePunishStained(client.Ipid()) {
		t.Error("expected no stain for a non-matching showname")
	}
	if got := countMegamasoPoolPunishments(client); got != 0 {
		t.Errorf("expected no punishments for a non-matching showname, got %d", got)
	}

	setPunishmentNames(nil)
	checkPunishmentShowname(client, "blacklistname1")
	if isShownamePunishStained(client.Ipid()) {
		t.Error("expected no stain when the punishment name list is empty")
	}
}
