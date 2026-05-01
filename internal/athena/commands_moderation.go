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
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/webhook"
	"github.com/xhit/go-str2duration/v2"
)

// tungForcedCharacterName is the asset folder name for the tung tung sahur character
// hosted on the web asset database (miku.pizza). This is not a server-list
// character, so no server-side character ID exists for it.
const tungForcedCharacterName = "tung tung sahur"

func cmdBan(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	uids := &[]string{}
	ipids := &[]string{}
	flags.Var(&cmdParamList{uids}, "u", "")
	flags.Var(&cmdParamList{ipids}, "i", "")
	duration := flags.String("d", config.BanLen, "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	if len(*uids) == 0 && len(*ipids) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	banTime, reason := time.Now().UTC().Unix(), strings.Join(flags.Args(), " ")
	var until int64
	if strings.ToLower(*duration) == "perma" {
		until = -1
	} else {
		parsedDur, err := str2duration.ParseDuration(*duration)
		if err != nil {
			client.SendServerMessage("Failed to ban: Cannot parse duration.")
			return
		}
		until = time.Now().UTC().Add(parsedDur).Unix()
	}

	var untilS string
	if until == -1 {
		untilS = "∞"
	} else {
		untilS = time.Unix(until, 0).UTC().Format("02 Jan 2006 15:04 MST")
	}

	var count int
	var reportBuilder strings.Builder
	seenIPIDs := make(map[string]struct{})
	if len(*uids) > 0 {
		for _, c := range getUidList(*uids) {
			id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, until, reason, client.StoredModName())
			if err != nil {
				continue
			}
			if _, seen := seenIPIDs[c.Ipid()]; !seen {
				seenIPIDs[c.Ipid()] = struct{}{}
				if reportBuilder.Len() > 0 {
					reportBuilder.WriteString(", ")
				}
				reportBuilder.WriteString(c.Ipid())
			}
			c.SendPacketSync("KB", fmt.Sprintf("%v\nUntil: %v\nID: %v", reason, untilS, id))
			c.conn.Close()
			forgetIP(c.Ipid())
			deleteAccountForIPID(c.Ipid())
			count++
			if err := webhook.PostBan(c.CurrentCharacter(), c.Showname(), c.OOCName(), c.Ipid(), c.Uid(), id, *duration, reason, client.DisplayModName()); err != nil {
				logger.LogErrorf("while posting ban webhook: %v", err)
			}
		}
	} else {
		for _, ipid := range *ipids {
			onlineClients := getClientsByIpid(ipid)
			if len(onlineClients) == 0 {
				// Offline ban – no HDID available.
				id, err := db.AddBan(ipid, "", banTime, until, reason, client.StoredModName())
				if err != nil {
					continue
				}
				forgetIP(ipid)
				deleteAccountForIPID(ipid)
				if err := webhook.PostBan("N/A", "N/A", "N/A", ipid, -1, id, *duration, reason, client.DisplayModName()); err != nil {
					logger.LogErrorf("while posting ban webhook: %v", err)
				}
			} else {
				// Online ban – record each unique HDID so the ban holds if the user
				// reconnects from a different IP address.
				banIDByHdid := make(map[string]int)
				for _, c := range onlineClients {
					if _, done := banIDByHdid[c.Hdid()]; done {
						continue
					}
					id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, until, reason, client.StoredModName())
					if err == nil {
						banIDByHdid[c.Hdid()] = id
					}
				}
				if len(banIDByHdid) == 0 {
					continue
				}
				forgetIP(ipid)
				deleteAccountForIPID(ipid)
				for _, c := range onlineClients {
					if id, ok := banIDByHdid[c.Hdid()]; ok {
						c.SendPacketSync("KB", fmt.Sprintf("%v\nUntil: %v\nID: %v", reason, untilS, id))
						if err := webhook.PostBan(c.CurrentCharacter(), c.Showname(), c.OOCName(), ipid, c.Uid(), id, *duration, reason, client.DisplayModName()); err != nil {
							logger.LogErrorf("while posting ban webhook: %v", err)
						}
					} else {
						c.SendPacketSync("KB", fmt.Sprintf("%v\nUntil: %v", reason, untilS))
					}
					c.conn.Close()
				}
			}
			if _, seen := seenIPIDs[ipid]; !seen {
				seenIPIDs[ipid] = struct{}{}
				if reportBuilder.Len() > 0 {
					reportBuilder.WriteString(", ")
				}
				reportBuilder.WriteString(ipid)
			}
			count++
		}
	}
	report := reportBuilder.String()
	if len(*ipids) > 0 {
		client.SendServerMessage(fmt.Sprintf("Banned %v IPID(s).", count))
	} else {
		client.SendServerMessage(fmt.Sprintf("Banned %v clients.", count))
	}
	sendPlayerArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Banned %v from server for %v: %v.", report, *duration, reason), true)
}

// deleteAccountForIPID removes the player account linked to the given IPID (if any).
// Any currently-connected session using that account is also logged out.
// Called automatically whenever a ban is issued so banned players cannot log back in.
func deleteAccountForIPID(ipid string) {
	username, err := db.GetUsernameByIPID(ipid)
	if err != nil || username == "" {
		return // no linked account — nothing to do
	}
	if err := db.RemoveUser(username); err != nil {
		logger.LogErrorf("deleteAccountForIPID: failed to remove account %q (IPID %v): %v", username, ipid, err)
		return
	}
	// Log out any connected session that was using the now-deleted account.
	clients.ForEach(func(c *Client) {
		if c.Authenticated() && c.ModName() == username {
			c.RemoveAuth()
		}
	})
	logger.LogInfof("deleteAccountForIPID: removed account %q linked to banned IPID %v", username, ipid)
}

// Handles /bg

func cmdEditBan(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.String("d", "", "")
	reason := flags.String("r", "", "")
	flags.Parse(args)
	useDur := *duration != ""
	useReason := *reason != ""

	if len(flags.Args()) == 0 || (!useDur && !useReason) {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUpdate := strings.Split(flags.Arg(0), ",")
	var until int64
	if useDur {
		if strings.ToLower(*duration) == "perma" {
			until = -1
		} else {
			parsedDur, err := str2duration.ParseDuration(*duration)
			if err != nil {
				client.SendServerMessage("Failed to ban: Cannot parse duration.")
				return
			}
			until = time.Now().UTC().Add(parsedDur).Unix()
		}
	}

	var reportBuilder strings.Builder
	for _, s := range toUpdate {
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		if useDur {
			err = db.UpdateDuration(id, until)
			if err != nil {
				continue
			}
		}
		if useReason {
			err = db.UpdateReason(id, *reason)
			if err != nil {
				continue
			}
		}
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		reportBuilder.WriteString(s)
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Updated bans: %v", report))
	if useDur {
		addToBuffer(client, "CMD", fmt.Sprintf("Edited bans: %v to duration: %v.", report, duration), true)
	}
	if useReason {
		addToBuffer(client, "CMD", fmt.Sprintf("Edited bans: %v to reason: %v.", report, reason), true)
	}
}

// Handles /evimode

func cmdGetBan(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	banid := flags.Int("b", -1, "")
	ipid := flags.String("i", "", "")
	flags.Parse(args)
	var sb strings.Builder
	sb.WriteString("Bans:\n----------")
	entry := func(b db.BanInfo) {
		var d string
		if b.Duration == -1 {
			d = "∞"
		} else {
			d = time.Unix(b.Duration, 0).UTC().Format("02 Jan 2006 15:04 MST")
		}
		fmt.Fprintf(&sb, "\nID: %v\nIPID: %v\nHDID: %v\nBanned on: %v\nUntil: %v\nReason: %v\nModerator: %v\n----------",
			b.Id, b.Ipid, b.Hdid, time.Unix(b.Time, 0).UTC().Format("02 Jan 2006 15:04 MST"), d, b.Reason, RenderStoredModName(b.Moderator, client.Perms()))
	}
	if *banid > 0 {
		b, err := db.GetBan(db.BANID, *banid)
		if err != nil || len(b) == 0 {
			client.SendServerMessage("No ban with that ID exists.")
			return
		}
		entry(b[0])
	} else if *ipid != "" {
		bans, err := db.GetBan(db.IPID, *ipid)
		if err != nil || len(bans) == 0 {
			client.SendServerMessage("No bans with that IPID exist.")
			return
		}
		for _, b := range bans {
			entry(b)
		}
	} else {
		bans, err := db.GetRecentBans()
		if err != nil {
			logger.LogErrorf("while getting recent bans: %v", err)
			client.SendServerMessage("An unexpected error occured.")
			return
		}
		for _, b := range bans {
			entry(b)
		}
	}
	client.SendServerMessage(sb.String())
}

// Handles /global

func cmdGlobal(client *Client, args []string, _ string) {
	if client.IsJailed() {
		client.SendServerMessage("You are jailed and cannot send OOC messages.")
		return
	}
	if !client.CanSpeakOOC() {
		client.SendServerMessage("You are muted from sending OOC messages.")
		return
	}
	if limited, remaining := checkNewIPIDOOCCooldown(client.Ipid()); limited {
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("New users must wait %d %s before using OOC chat.", remaining, unit))
		return
	}
	tag := formatTagDisplay(db.GetActiveTag(client.Ipid()))
	if tag != "" {
		tag += " "
	}
	writeToAll("CT", fmt.Sprintf("[GLOBAL] [UID %d] %s%v", client.Uid(), tag, client.OOCName()), strings.Join(args, " "), "1")
}

// Handles /hide

func cmdHide(client *Client, _ []string, _ string) {
	if client.Hidden() {
		client.Area().AddVisiblePlayer()
		client.SetHidden(false)
		broadcastPlayerJoin(client)
		sendPlayerArup()
		client.SendServerMessage("You are now visible.")
		addToBuffer(client, "CMD", "Disabled hide mode.", false)
	} else {
		client.Area().RemoveVisiblePlayer()
		client.SetHidden(true)
		writeToAll("PR", strconv.Itoa(client.Uid()), "1")
		sendPlayerArup()
		client.SendServerMessage("You are now hidden from the player list and room counts.")
		addToBuffer(client, "CMD", "Enabled hide mode.", false)
	}
}

// Handles /invite

func cmdKick(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	uids := &[]string{}
	ipids := &[]string{}
	flags.Var(&cmdParamList{uids}, "u", "")
	flags.Var(&cmdParamList{ipids}, "i", "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	var toKick []*Client
	if len(*uids) > 0 {
		toKick = getUidList(*uids)
	} else if len(*ipids) > 0 {
		toKick = getIpidList(*ipids)
	} else {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	var count int
	var reportBuilder strings.Builder
	reason := strings.Join(flags.Args(), " ")
	for _, c := range toKick {
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		reportBuilder.WriteString(c.Ipid())
		c.SendPacketSync("KK", reason)
		c.conn.Close()
		count++
		if err := webhook.PostKick(c.CurrentCharacter(), c.Showname(), c.OOCName(), c.Ipid(), reason, client.DisplayModName(), c.Uid()); err != nil {
			logger.LogErrorf("while posting kick webhook: %v", err)
		}
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Kicked %v clients.", count))
	sendPlayerArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Kicked %v from server for reason: %v.", report, reason), true)
}

// Handles /kickarea

func cmdLogin(client *Client, args []string, _ string) {
	if client.Authenticated() {
		client.SendServerMessage("You are already logged in.")
		return
	}
	auth, perms := db.AuthenticateUser(args[0], []byte(args[1]))
	addToBuffer(client, "AUTH", fmt.Sprintf("Attempted login as %v.", args[0]), true)
	if auth {
		client.SetAuthenticated(true)
		client.SetPerms(perms)
		client.SetModName(args[0])
		// Link the current IPID to this account so leaderboards can show names.
		db.LinkIPIDToUser(args[0], client.Ipid()) //nolint:errcheck
		// Restore the account's gamble-hide preference for this session.
		if hide, err := db.GetGambleHide(args[0]); err == nil {
			client.SetGambleHide(hide)
		}
		// Restore the account's active cosmetic tag so it shows without re-equipping.
		if tag := db.GetAccountActiveTag(args[0]); tag != "" {
			db.SetActiveTag(client.Ipid(), tag) //nolint:errcheck
		}
		// Playtime-based auto-trust: if the account has accumulated at least one
		// hour of play, silently add the current IPID to the lockdown whitelist
		// and clear any new-IPID OOC cooldown.  This lets regulars whose IP
		// changed reconnect seamlessly without moderator intervention.
		if playtime, err := db.GetPlaytime(client.Ipid()); err == nil && playtime >= 3600 {
			ipFirstSeenTracker.mu.Lock()
			ipFirstSeenTracker.times[client.Ipid()] = time.Unix(0, 0)
			ipFirstSeenTracker.mu.Unlock()
			db.MarkIPKnown(client.Ipid()) //nolint:errcheck
		}
		if permissions.IsModerator(perms) {
			client.SendServerMessage("Logged in as moderator.")
			// AUTH#1 triggers the AO2 client's "Logged in as a moderator" popup.
			// Only send it for actual moderators; player and DJ-only accounts get
			// chat-based feedback instead so the client doesn't mislabel them.
			client.SendPacket("AUTH", "1")
		} else {
			client.SendServerMessage("Logged in to your account.")
		}
		client.SendServerMessage(fmt.Sprintf("Welcome back, %v.", args[0]))
		addToBuffer(client, "AUTH", fmt.Sprintf("Logged in as %v.", args[0]), true)
		return
	}
	client.SendPacket("AUTH", "0")
	addToBuffer(client, "AUTH", fmt.Sprintf("Failed login as %v.", args[0]), true)
}

// Handles /logout

func cmdLogout(client *Client, _ []string, _ string) {
	if !client.Authenticated() {
		client.SendServerMessage("You are not logged in.")
		return
	}
	addToBuffer(client, "AUTH", fmt.Sprintf("Logged out as %v.", client.DisplayModName()), true)
	if permissions.IsModerator(client.Perms()) {
		client.RemoveAuth()
	} else {
		client.RemoveAccountAuth()
	}
}

// Handles /mkusr

func cmdMakeUser(client *Client, args []string, _ string) {
	if db.UserExists(args[0]) {
		client.SendServerMessage("User already exists.")
		return
	}

	role, err := getRole(args[2])
	if err != nil {
		client.SendServerMessage("Invalid role.")
		return
	}
	err = db.CreateUser(args[0], []byte(args[1]), role.GetPermissions())
	if err != nil {
		logger.LogError(err.Error())
		client.SendServerMessage("Invalid username/password.")
		return
	}
	client.SendServerMessage("User created.")
	addToBuffer(client, "CMD", fmt.Sprintf("Created user %v.", args[0]), true)
}

// Handles /mod

func cmdMod(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	global := flags.Bool("g", false, "")
	flags.Parse(args)
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	msg := strings.Join(flags.Args(), " ")
	if *global {
		writeToAll("CT", fmt.Sprintf("[MOD] [GLOBAL] %v", client.OOCName()), msg, "1")
	} else {
		writeToArea(client.Area(), "CT", fmt.Sprintf("[MOD] %v", client.OOCName()), msg, "1")
	}
	addToBuffer(client, "OOC", msg, false)
}

// Handles /modchat

func cmdModChat(client *Client, args []string, _ string) {
	msg := strings.Join(args, " ")
	senderIsShadow := permissions.IsShadow(client.Perms())
	realSender := client.OOCName()
	clients.ForEach(func(c *Client) {
		if permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			// Shadow mods appear as "Moderator" to everyone except admins.
			senderLabel := realSender
			if senderIsShadow && !permissions.IsAdmin(c.Perms()) {
				senderLabel = "Moderator"
			}
			c.SendPacket("CT", fmt.Sprintf("[MODCHAT] %v", senderLabel), msg, "1")
		}
	})
}

// Handles /motd

func cmdMute(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	music := flags.Bool("m", false, "")
	jud := flags.Bool("j", false, "")
	ic := flags.Bool("ic", false, "")
	ooc := flags.Bool("ooc", false, "")
	duration := flags.Int("d", -1, "")
	flags.Parse(args)

	var m MuteState
	switch {
	case *ic && *ooc:
		m = ICOOCMuted
	case *ic:
		m = ICMuted
	case *ooc:
		m = OOCMuted
	case *music:
		m = MusicMuted
	case *jud:
		m = JudMuted
	default:
		m = ICMuted
	}
	msg := fmt.Sprintf("You have been muted from %v", m.String())
	if *duration != -1 {
		msg += fmt.Sprintf(" for %v seconds", *duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toMute := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var reportBuilder strings.Builder
	for _, c := range toMute {
		if c.Muted() == m {
			continue
		}
		c.SetMuted(m)
		var expires int64
		if *duration == -1 {
			c.SetUnmuteTime(time.Time{})
			expires = 0
		} else {
			t := time.Now().UTC().Add(time.Duration(*duration) * time.Second)
			c.SetUnmuteTime(t)
			expires = t.Unix()
		}
		if err := db.UpsertMute(c.Ipid(), int(m), expires); err != nil {
			logger.LogErrorf("Failed to persist mute for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage(msg)
		count++
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		fmt.Fprintf(&reportBuilder, "%v", c.Uid())
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Muted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Muted %v.", report), false)
}

// Handles /narrator

func cmdParrot(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	duration := flags.Int("d", -1, "")
	flags.Parse(args)
	msg := "You have been turned into a parrot"
	if *duration != -1 {
		msg += fmt.Sprintf(" for %v seconds", *duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toParrot := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var reportBuilder strings.Builder
	for _, c := range toParrot {
		if c.Muted() != Unmuted {
			continue
		}
		c.SetMuted(ParrotMuted)
		var expires int64
		if *duration == -1 {
			c.SetUnmuteTime(time.Time{})
			expires = 0
		} else {
			t := time.Now().UTC().Add(time.Duration(*duration) * time.Second)
			c.SetUnmuteTime(t)
			expires = t.Unix()
		}
		if err := db.UpsertMute(c.Ipid(), int(ParrotMuted), expires); err != nil {
			logger.LogErrorf("Failed to persist parrot mute for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage(msg)
		count++
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		fmt.Fprintf(&reportBuilder, "%v", c.Uid())
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Parroted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Parroted %v.", report), false)
}

// Handles /play

func cmdPlayers(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	all := flags.Bool("a", false, "")
	flags.Parse(args)

	isAdmin := permissions.HasPermission(client.Perms(), permissions.PermissionField["ADMIN"])
	hasBanInfo := permissions.HasPermission(client.Perms(), permissions.PermissionField["BAN_INFO"])
	targetArea := client.Area()

	// Group clients by area in a single snapshot pass.
	type areaClients struct {
		list []*Client
	}
	grouped := make(map[*area.Area]*areaClients, len(areas))
	allFlag := *all
	clients.ForEach(func(c *Client) {
		a := c.Area()
		if !allFlag && a != targetArea {
			return
		}
		if !isAdmin && c.Hidden() {
			return
		}
		ac := grouped[a]
		if ac == nil {
			ac = &areaClients{}
			grouped[a] = ac
		}
		ac.list = append(ac.list, c)
	})

	// writeEntry appends a single client's info to the builder.
	writeEntry := func(b *strings.Builder, c *Client) {
		if c.Hidden() {
			b.WriteString("[HIDDEN] ")
		}
		prefix := formatTagDisplay(db.GetActiveTag(c.Ipid()))
		if prefix != "" {
			prefix += " "
		}
		fmt.Fprintf(b, "%s[%v] %v\n", prefix, c.Uid(), c.CurrentCharacter())
		if hasBanInfo {
			if permissions.IsModerator(c.Perms()) {
				// Shadow mods: hide the entire "Mod:" line from non-admin viewers.
				// Previously the line still printed as "Mod: Moderator", which
				// let other moderators infer the target was staff. Now only
				// admins see anything for a shadow mod; everyone else sees no
				// mod line at all (the player looks like a regular user).
				if permissions.IsShadow(c.Perms()) {
					if isAdmin {
						fmt.Fprintf(b, "Mod: %v (shadow)\n", c.ModName())
					}
				} else {
					fmt.Fprintf(b, "Mod: %v\n", c.ModName())
				}
			}
			fmt.Fprintf(b, "IPID: %v\n", c.Ipid())
		}
		if ooc := c.OOCName(); ooc != "" {
			fmt.Fprintf(b, "OOC: %v\n", ooc)
		}
	}

	// printArea appends one area's section to the builder.
	printArea := func(b *strings.Builder, a *area.Area) {
		count := a.VisiblePlayerCount()
		fmt.Fprintf(b, "%v:\n%v players online.\n", a.Name(), count)
		if ac := grouped[a]; ac != nil {
			for _, c := range ac.list {
				writeEntry(b, c)
			}
		}
	}

	var out strings.Builder
	out.WriteString("\nPlayers\n----------\n")
	if *all {
		// /gas hides empty areas to keep the list usable on servers with many areas.
		// "Empty" = nobody visible to the requester. Admins still see hidden players,
		// so an area with only hidden occupants is empty for everyone else but not them.
		shown := 0
		hiddenAreas := 0
		for _, a := range areas {
			ac := grouped[a]
			if ac == nil || len(ac.list) == 0 {
				hiddenAreas++
				continue
			}
			printArea(&out, a)
			out.WriteString("----------\n")
			shown++
		}
		if shown == 0 {
			out.WriteString("(no areas have visible players)\n----------\n")
		}
		if hiddenAreas > 0 {
			fmt.Fprintf(&out, "%d empty area(s) hidden.\n", hiddenAreas)
		}
	} else {
		printArea(&out, targetArea)
	}
	client.SendServerMessage(out.String())
}

// Handles /pm

func cmdPM(client *Client, args []string, _ string) {
	if client.IsJailed() {
		client.SendServerMessage("You are jailed and cannot send OOC messages.")
		return
	}
	if !client.CanSpeakOOC() {
		client.SendServerMessage("You are muted from sending OOC messages.")
		return
	}
	if limited, remaining := checkNewIPIDOOCCooldown(client.Ipid()); limited {
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("New users must wait %d %s before using OOC chat.", remaining, unit))
		return
	}
	msg := strings.Join(args[1:], " ")
	toPM := getUidList(strings.Split(args[0], ","))
	var recipientNames []string
	for _, c := range toPM {
		c.SendPacket("CT", fmt.Sprintf("[PM] [UID %d] %v", client.Uid(), client.OOCName()), msg, "1")
		recipientNames = append(recipientNames, fmt.Sprintf("[%d] %v", c.Uid(), c.OOCName()))
	}
	// Echo the message back to the sender so they can see what they sent.
	if len(recipientNames) > 0 {
		client.SendPacket("CT", fmt.Sprintf("[PM → %v] %v", strings.Join(recipientNames, ", "), client.OOCName()), msg, "1")
	}
}

// validPositions is the set of positions a player can move to with /pos.
var validPositions = []string{"def", "pro", "wit", "jud", "hld", "hlp", "jur", "sea"}

// Handles /pos

func cmdRemoveUser(client *Client, args []string, _ string) {
	if !db.UserExists(args[0]) {
		client.SendServerMessage("User does not exist.")
		return
	}
	err := db.RemoveUser(args[0])
	if err != nil {
		client.SendServerMessage("Failed to remove user.")
		logger.LogError(err.Error())
		return
	}
	client.SendServerMessage("Removed user.")

	removedUser := args[0]
	clients.ForEach(func(c *Client) {
		if c.Authenticated() && c.ModName() == removedUser {
			c.RemoveAuth()
		}
	})
	addToBuffer(client, "CMD", fmt.Sprintf("Removed user %v.", args[0]), true)
}

// Handles /resetpass

func cmdResetPassword(client *Client, args []string, _ string) {
	username := args[0]
	newPassword := args[1]

	if !db.UserExists(username) {
		client.SendServerMessage("User does not exist.")
		return
	}

	err := db.UpdatePassword(username, []byte(newPassword))
	if err != nil {
		client.SendServerMessage("Failed to reset password.")
		logger.LogError(err.Error())
		return
	}

	client.SendServerMessage(fmt.Sprintf("Password for user %v has been reset.", username))
	addToBuffer(client, "CMD", fmt.Sprintf("Reset password for user %v.", username), true)
}

// Handles /roll

func cmdChangeRole(client *Client, args []string, _ string) {
	role, err := getRole(args[1])
	if err != nil {
		client.SendServerMessage("Invalid role.")
		return
	}

	if !db.UserExists(args[0]) {
		client.SendServerMessage("User does not exist.")
		return
	}

	err = db.ChangePermissions(args[0], role.GetPermissions())
	if err != nil {
		client.SendServerMessage("Failed to change permissions.")
		logger.LogError(err.Error())
		return
	}
	client.SendServerMessage("Role updated.")

	targetUser := args[0]
	newPerms := role.GetPermissions()
	clients.ForEach(func(c *Client) {
		if c.Authenticated() && c.ModName() == targetUser {
			c.SetPerms(newPerms)
		}
	})
	addToBuffer(client, "CMD", fmt.Sprintf("Updated role of %v to %v.", args[0], args[1]), true)
}

// Handles /status

func cmdUnban(client *Client, args []string, _ string) {
	toUnban := strings.Split(args[0], ",")
	var reportBuilder strings.Builder
	for _, s := range toUnban {
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		// Look up ban details before nullifying so the webhook embed is informative.
		bans, dbErr := db.GetBan(db.BANID, id)
		err = db.UnBan(id)
		if err != nil {
			continue
		}
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		reportBuilder.WriteString(s)
		if dbErr == nil && len(bans) > 0 {
			b := bans[0]
			var durStr string
			if b.Duration == -1 {
				durStr = "Permanent"
			} else {
				durStr = time.Unix(b.Duration, 0).UTC().Format("02 Jan 2006 15:04 MST")
			}
			if err := webhook.PostUnban(id, b.Ipid, b.Reason, durStr, RenderStoredModName(b.Moderator, 0), client.DisplayModName()); err != nil {
				logger.LogErrorf("while posting unban webhook: %v", err)
			}
		}
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Nullified bans: %v", report))
	addToBuffer(client, "CMD", fmt.Sprintf("Nullified bans: %v", report), true)
}

// Handles /uncm

func cmdUnmute(client *Client, args []string, _ string) {
	toUnmute := getUidList(strings.Split(args[0], ","))
	var count int
	var reportBuilder strings.Builder
	for _, c := range toUnmute {
		if c.Muted() == Unmuted {
			continue
		}
		c.SetMuted(Unmuted)
		if err := db.DeleteMute(c.Ipid()); err != nil {
			logger.LogErrorf("Failed to remove persistent mute for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("You have been unmuted.")
		count++
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		fmt.Fprintf(&reportBuilder, "%v", c.Uid())
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Unmuted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Unmuted %v.", report), false)
}

// Handles /jail

func cmdJail(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.String("d", "perma", "")
	reason := flags.String("r", "", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	uid, err := strconv.Atoi(flags.Arg(0))
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client not found.")
		return
	}

	// Optional area argument: /jail <uid> <area_id> [-d ...] [-r ...]
	jailAreaID := -1
	if flags.NArg() >= 2 {
		id, err := strconv.Atoi(flags.Arg(1))
		if err != nil {
			client.SendServerMessage("Invalid area ID: must be a number.")
			return
		}
		if id < 0 || id >= len(areas) {
			client.SendServerMessage(fmt.Sprintf("Area ID %d is out of range (0–%d).", id, len(areas)-1))
			return
		}
		jailAreaID = id
	}

	isPerma := strings.ToLower(*duration) == "perma"
	var jailUntil time.Time
	if isPerma {
		jailUntil = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		parsedDur, err := str2duration.ParseDuration(*duration)
		if err != nil {
			client.SendServerMessage("Failed to jail: Cannot parse duration.")
			return
		}
		jailUntil = time.Now().UTC().Add(parsedDur)
	}

	// Force-move the target to the jail area first (before setting jailed state so
	// any existing jail doesn't block the move), then apply the jail.
	if jailAreaID >= 0 {
		target.forceChangeArea(areas[jailAreaID])
	}

	target.SetJailedUntil(jailUntil)
	target.SetJailAreaID(jailAreaID)
	if err := db.UpsertJail(target.Ipid(), jailUntil.Unix(), *reason, jailAreaID); err != nil {
		logger.LogErrorf("Failed to persist jail for %v: %v", target.Ipid(), err)
	}

	var areaName string
	if jailAreaID >= 0 {
		areaName = areas[jailAreaID].Name()
	} else {
		areaName = target.Area().Name()
	}

	msg := fmt.Sprintf("You have been jailed in %v.", areaName)
	if !isPerma {
		msg = fmt.Sprintf("You have been jailed in %v for %v.", areaName, *duration)
	}
	if *reason != "" {
		msg += " Reason: " + *reason
	}
	target.SendServerMessage(msg)

	client.SendServerMessage(fmt.Sprintf("Jailed [%v] %v in %v.", uid, target.OOCName(), areaName))

	logMsg := fmt.Sprintf("Jailed [%v] %v in %v", uid, target.OOCName(), areaName)
	if *reason != "" {
		logMsg += " for reason: " + *reason
	}
	addToBuffer(client, "CMD", logMsg, false)

	durationDisplay := *duration
	if isPerma {
		durationDisplay = "Permanent"
	}
	if err := webhook.PostJail(target.CurrentCharacter(), target.Showname(), target.OOCName(),
		target.Ipid(), areaName, durationDisplay, *reason, client.DisplayModName(), uid); err != nil {
		logger.LogErrorf("Failed to post jail webhook: %v", err)
	}
}

// Handles /unjail

func cmdUnjail(client *Client, args []string, _ string) {
	toUnjail := getUidList(strings.Split(args[0], ","))
	var count int
	var reportBuilder strings.Builder
	for _, c := range toUnjail {
		if !c.IsJailed() {
			continue
		}
		c.SetJailedUntil(time.Time{})
		c.SetJailAreaID(-1)
		if err := db.DeleteJail(c.Ipid()); err != nil {
			logger.LogErrorf("Failed to remove persistent jail for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("You have been released from jail.")
		count++
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		fmt.Fprintf(&reportBuilder, "%v", c.Uid())
	}
	report := reportBuilder.String()
	client.SendServerMessage(fmt.Sprintf("Released %v clients from jail.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Released %v from jail.", report), false)
}

// cmdForceName forces a client to use a specific showname in IC messages.
func cmdForceName(client *Client, args []string, _ string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	// Command args are already decoded (plain text); validate the visible length.
	name := strings.Join(args[1:], " ")
	if len(name) > maxShownameLength {
		client.SendServerMessage(fmt.Sprintf("Forced showname is too long (max %d characters).", maxShownameLength))
		return
	}
	// Store as AO2-encoded so it can be placed directly into the IC packet's
	// showname field (args[15]) without an extra encode step on every message.
	target.SetForcedShowname(encode(name))
	// PU and in-server messages use the decoded (display) form.
	writeToAll("PU", strconv.Itoa(target.Uid()), "2", name)
	target.SendServerMessage(fmt.Sprintf("A moderator has forced your showname to \"%s\".", name))
	client.SendServerMessage(fmt.Sprintf("Forced UID %v's showname to \"%s\".", uid, name))
	addToBuffer(client, "CMD", fmt.Sprintf("forced showname of UID %v to \"%s\"", uid, name), true)
}

// cmdUnforceName removes a forced showname from a client.
func cmdUnforceName(client *Client, args []string, _ string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	if target.ForcedShowname() == "" {
		client.SendServerMessage(fmt.Sprintf("UID %v does not have a forced showname.", uid))
		return
	}
	target.SetForcedShowname("")
	target.SendServerMessage("Your forced showname has been removed by a moderator.")
	client.SendServerMessage(fmt.Sprintf("Removed forced showname from UID %v.", uid))
	addToBuffer(client, "CMD", fmt.Sprintf("removed forced showname from UID %v", uid), true)
}

// cmdNameShuffle randomly reassigns all shownames within the current area.
// Each player receives another player's effective showname so that every name
// is displaced but none is lost.
func cmdNameShuffle(client *Client, _ []string, _ string) {
	targetArea := client.Area()

	// Fast path: skip the full client-list scan when the area clearly lacks
	// enough participants. PlayerCount() is an O(1) cached counter that only
	// counts clients that have fully joined an area (UID != -1), so it is a
	// reliable lower bound. A second check below handles any edge-case races.
	if targetArea.PlayerCount() < 2 {
		client.SendServerMessage("There are not enough players in this area to shuffle names (need at least 2).")
		return
	}

	// Collect joined clients and their shownames in a single pass.
	// Pre-allocate with the cached player count to avoid repeated re-allocs.
	n := targetArea.PlayerCount()
	targets := make([]*Client, 0, n)
	names := make([]string, 0, n)
	clients.ForEach(func(c *Client) {
		if c.Uid() != -1 && c.Area() == targetArea {
			targets = append(targets, c)
			names = append(names, c.EffectiveShowname())
		}
	})

	if len(targets) < 2 {
		client.SendServerMessage("There are not enough players in this area to shuffle names (need at least 2).")
		return
	}

	// Sattolo algorithm: single O(n) in-place pass that guarantees every
	// element moves to a new index (a cyclic derangement). The bound is i
	// (not i+1 as in Fisher-Yates) to exclude self-swaps and ensure the
	// result is always a derangement; no copy or retry loop is needed.
	for i := len(names) - 1; i > 0; i-- {
		j := rand.Intn(i) // [0, i-1] inclusive
		names[i], names[j] = names[j], names[i]
	}

	// Apply shuffled shownames, send per-player messages, and collect PU data.
	uidStrs := make([]string, len(targets))
	decodedNames := make([]string, len(targets))
	for i, c := range targets {
		c.SetForcedShowname(names[i])
		uidStrs[i] = strconv.Itoa(c.Uid())
		decodedNames[i] = decode(names[i])
		c.SendServerMessage("A moderator has shuffled the shownames in this area.")
	}
	// Broadcast all PU updates in a single pass instead of one writeToAll per client.
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 {
			return
		}
		for i, uid := range uidStrs {
			c.SendPacket("PU", uid, "2", decodedNames[i])
		}
	})

	client.SendServerMessage(fmt.Sprintf("Shuffled shownames of %d players in the area.", len(targets)))
	addToBuffer(client, "CMD", fmt.Sprintf("shuffled shownames of %d players in area %v", len(targets), targetArea.Name()), true)
}

// cmdUnnameShuffle removes all forced shownames in the current area, restoring
// each player's own showname and broadcasting a PU update to all clients.
func cmdUnnameShuffle(client *Client, _ []string, _ string) {
	targetArea := client.Area()

	// Pre-allocate with the cached player count to avoid repeated re-allocs.
	// PlayerCount() is O(1); the slice may end up shorter if not all players
	// have a forced showname, but this avoids any mid-loop heap growth.
	resetTargets := make([]*Client, 0, targetArea.PlayerCount())
	clients.ForEach(func(c *Client) {
		if c.Uid() != -1 && c.Area() == targetArea && c.ForcedShowname() != "" {
			resetTargets = append(resetTargets, c)
		}
	})

	if len(resetTargets) == 0 {
		client.SendServerMessage("No players in this area have a forced showname.")
		return
	}

	// Clear forced shownames, send per-player messages, and collect PU data.
	uidStrs := make([]string, len(resetTargets))
	restoredNames := make([]string, len(resetTargets))
	for i, c := range resetTargets {
		c.SetForcedShowname("")
		uidStrs[i] = strconv.Itoa(c.Uid())
		restoredNames[i] = decode(c.Showname())
		c.SendServerMessage("A moderator has restored shownames in this area.")
	}
	// Broadcast all PU updates in a single pass instead of one writeToAll per client.
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 {
			return
		}
		for i, uid := range uidStrs {
			c.SendPacket("PU", uid, "2", restoredNames[i])
		}
	})

	client.SendServerMessage(fmt.Sprintf("Restored shownames of %d players in the area.", len(resetTargets)))
	addToBuffer(client, "CMD", fmt.Sprintf("restored shownames of %d players in area %v", len(resetTargets), targetArea.Name()), true)
}

// cmdTung forces a real iniswap to the tung tung sahur character (asset folder
// "tttomoetachibana" on the web asset database) for all players in the caller's
// current area. The players' character slots are unchanged — the IC packet's
// char_name and char_id fields are overridden so every observer's client loads
// assets from the tung tung sahur folder.
// Usage:
//
//	/tung global
//	/tung global off
func cmdTung(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	if !strings.EqualFold(args[0], "global") {
		client.SendServerMessage("Invalid argument. " + usage)
		return
	}

	disable := len(args) >= 2 && strings.EqualFold(args[1], "off")
	// Resolve the char_id that corresponds to tungForcedCharacterName so that
	// observers' IC packets have a matching char_name/char_id pair. WebAO
	// validates these fields and renders the character invisible when they
	// disagree. getCharacterID returns -1 when the character is not in the
	// server list; -1 is safe because the char_id validation is skipped for
	// forced iniswaps in the IC handler.
	tungID := getCharacterID(tungForcedCharacterName)
	tungIDStr := strconv.Itoa(tungID)
	targetArea := client.Area()
	capacity := targetArea.PlayerCount()
	// Phase 1: modify each area client's state in one ForEach pass and collect
	// the per-client data needed for the PU broadcast.  PV is sent to each
	// target here so they see their own panel update immediately.
	uidStrs := make([]string, 0, capacity)
	if disable {
		charNames := make([]string, 0, capacity)
		clients.ForEach(func(c *Client) {
			if c.Uid() == -1 || c.Area() != targetArea {
				return
			}
			origIDStr := c.CharIDStr()
			c.SetForcedIniswapChar("", "")
			uidStrs = append(uidStrs, strconv.Itoa(c.Uid()))
			charNames = append(charNames, c.CurrentCharacter())
			// Restore the client's emote panel to their real character.
			c.SendPacket("PV", "0", "CID", origIDStr)
		})
		// Phase 2: broadcast all PU updates in a single pass over all clients,
		// replacing the N separate writeToAll calls (each a full ForEach) with one.
		if len(uidStrs) > 0 {
			clients.ForEach(func(c *Client) {
				if c.Uid() == -1 {
					return
				}
				for i, uid := range uidStrs {
					c.SendPacket("PU", uid, "1", charNames[i])
				}
			})
		}
		affected := len(uidStrs)
		client.SendServerMessage(fmt.Sprintf("Removed tung effect from %d client(s) in this area.", affected))
		addToBuffer(client, "CMD", fmt.Sprintf("Removed tung effect from %d clients in area %v.", affected, targetArea.Name()), true)
	} else {
		clients.ForEach(func(c *Client) {
			if c.Uid() == -1 || c.Area() != targetArea {
				return
			}
			c.SetForcedIniswapChar(tungForcedCharacterName, tungIDStr)
			uidStrs = append(uidStrs, strconv.Itoa(c.Uid()))
			// Switch the client's emote panel to the tung character so
			// their buttons and animations update on their own screen too.
			if tungID >= 0 {
				c.SendPacket("PV", "0", "CID", tungIDStr)
			}
		})
		// Phase 2: broadcast all PU updates in a single pass over all clients.
		if len(uidStrs) > 0 {
			clients.ForEach(func(c *Client) {
				if c.Uid() == -1 {
					return
				}
				for _, uid := range uidStrs {
					c.SendPacket("PU", uid, "1", tungForcedCharacterName)
				}
			})
		}
		affected := len(uidStrs)
		client.SendServerMessage(fmt.Sprintf("Applied tung effect to %d client(s) in this area.", affected))
		addToBuffer(client, "CMD", fmt.Sprintf("Applied tung effect to %d clients in area %v.", affected, targetArea.Name()), true)
	}
}

// cmdUntung is a convenience alias for /tung global off.
// Usage:
//
//	/untung global
func cmdUntung(client *Client, args []string, _ string) {
	cmdTung(client, []string{"global", "off"}, "Usage: /untung global")
}

// cmdAreaIniswap forces everyone in the caller's current area to iniswap as a
// moderator-specified character from the server character list, or removes that
// forced iniswap when called with "off".
// Usage:
//
//	/areainiswap <character name>
//	/areainiswap off
func cmdAreaIniswap(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	targetArea := client.Area()
	if strings.EqualFold(args[0], "off") {
		// Phase 1: clear forced iniswap state, collect per-client PU data, send PV.
		capacity := targetArea.PlayerCount()
		uidStrs := make([]string, 0, capacity)
		charNames := make([]string, 0, capacity)
		clients.ForEach(func(c *Client) {
			if c.Uid() == -1 || c.Area() != targetArea {
				return
			}
			origIDStr := c.CharIDStr()
			c.SetForcedIniswapChar("", "")
			uidStrs = append(uidStrs, strconv.Itoa(c.Uid()))
			charNames = append(charNames, c.CurrentCharacter())
			c.SendPacket("PV", "0", "CID", origIDStr)
		})
		// Phase 2: broadcast all PU updates in a single pass instead of one
		// writeToAll call per affected client (each a full ForEach).
		if len(uidStrs) > 0 {
			clients.ForEach(func(c *Client) {
				if c.Uid() == -1 {
					return
				}
				for i, uid := range uidStrs {
					c.SendPacket("PU", uid, "1", charNames[i])
				}
			})
		}
		affected := len(uidStrs)
		client.SendServerMessage(fmt.Sprintf("Removed area iniswap effect from %d client(s).", affected))
		addToBuffer(client, "CMD", fmt.Sprintf("Removed area iniswap effect from %d clients in area %v.", affected, targetArea.Name()), true)
		return
	}

	charName := strings.TrimSpace(strings.Join(args, " "))
	charID := getCharacterID(charName)
	if charID < 0 {
		client.SendServerMessage(fmt.Sprintf("Character %q was not found in the character list.", charName))
		return
	}
	charName = characters[charID]
	charIDStr := strconv.Itoa(charID)

	// Phase 1: apply forced iniswap state, collect affected UIDs, send PV to each target.
	capacity := targetArea.PlayerCount()
	uidStrs := make([]string, 0, capacity)
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 || c.Area() != targetArea {
			return
		}
		c.SetForcedIniswapChar(charName, charIDStr)
		uidStrs = append(uidStrs, strconv.Itoa(c.Uid()))
		c.SendPacket("PV", "0", "CID", charIDStr)
	})
	// Phase 2: broadcast all PU updates in a single pass.
	if len(uidStrs) > 0 {
		clients.ForEach(func(c *Client) {
			if c.Uid() == -1 {
				return
			}
			for _, uid := range uidStrs {
				c.SendPacket("PU", uid, "1", charName)
			}
		})
	}
	affected := len(uidStrs)
	client.SendServerMessage(fmt.Sprintf("Applied area iniswap as %q to %d client(s).", charName, affected))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied area iniswap as %q to %d clients in area %v.", charName, affected, targetArea.Name()), true)
}

// cmdUntorment removes an IPID from the automod torment list.
func cmdUntorment(client *Client, args []string, usage string) {
	ipid := strings.TrimSpace(args[0])
	if ipid == "" {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	if !isIPIDTormented(ipid) {
		client.SendServerMessage(fmt.Sprintf("IPID %v is not on the torment list.", ipid))
		return
	}
	removeTormentedIP(ipid)
	client.SendServerMessage(fmt.Sprintf("Removed %v from the torment list.", ipid))
	addToBuffer(client, "CMD", fmt.Sprintf("removed IPID %v from torment list", ipid), true)
}

// cmdLockdown toggles server lockdown mode, or manages the lockdown whitelist.
// Subcommands:
//
//	/lockdown           - toggle lockdown on/off
//	/lockdown add <UID> - whitelist a connected player's IPID so they can join during lockdown
//	/lockdown whitelist all - whitelist all IPIDs currently connected to the server
func cmdLockdown(client *Client, args []string, usage string) {
	// Subcommand dispatch.
	if len(args) >= 1 {
		switch args[0] {
		case "add":
			if len(args) < 2 {
				client.SendServerMessage("Not enough arguments:\n" + usage)
				return
			}
			uid, err := strconv.Atoi(args[1])
			if err != nil {
				client.SendServerMessage("Invalid UID.")
				return
			}
			target := clients.GetClientByUID(uid)
			if target == nil {
				client.SendServerMessage("No client found with that UID.")
				return
			}
			ipid := target.Ipid()
			recordIPFirstSeen(ipid)
			if err := db.MarkIPKnown(ipid); err != nil {
				logger.LogErrorf("lockdown add: failed to persist IPID %s: %v", ipid, err)
			}
			client.SendServerMessage(fmt.Sprintf("Whitelisted IPID %v (UID %v) for lockdown.", ipid, uid))
			addToBuffer(client, "CMD", fmt.Sprintf("Whitelisted IPID %v (UID %v) for lockdown.", ipid, uid), true)
			return
		case "whitelist":
			if len(args) < 2 || args[1] != "all" {
				client.SendServerMessage("Not enough arguments:\n" + usage)
				return
			}
			count := 0
			clients.ForEach(func(c *Client) {
				if c.Uid() != -1 {
					ipid := c.Ipid()
					recordIPFirstSeen(ipid)
					if err := db.MarkIPKnown(ipid); err != nil {
						logger.LogErrorf("lockdown whitelist all: failed to persist IPID %s: %v", ipid, err)
					}
					count++
				}
			})
			client.SendServerMessage(fmt.Sprintf("Whitelisted %v IPID(s) from the server for lockdown.", count))
			addToBuffer(client, "CMD", fmt.Sprintf("Whitelisted %v IPID(s) server-wide for lockdown.", count), true)
			return
		default:
			client.SendServerMessage("Unknown subcommand:\n" + usage)
			return
		}
	}

	// Toggle lockdown.
	active := !serverLockdown.Load()
	serverLockdown.Store(active)
	if active {
		clients.ForEach(func(c *Client) {
			if c.Uid() != -1 && permissions.IsModerator(c.Perms()) {
				c.SendPacket("CT", "OOC", "🔒 Server lockdown is now ACTIVE. New connections are restricted to known players.", "1")
			}
		})
		client.SendServerMessage("Lockdown enabled. New IPIDs will be rejected.")
		addToBuffer(client, "CMD", "Enabled server lockdown.", true)
	} else {
		clients.ForEach(func(c *Client) {
			if c.Uid() != -1 && permissions.IsModerator(c.Perms()) {
				c.SendPacket("CT", "OOC", "🔓 Server lockdown has been LIFTED. New connections are now allowed.", "1")
			}
		})
		client.SendServerMessage("Lockdown disabled. New IPIDs are now allowed.")
		addToBuffer(client, "CMD", "Disabled server lockdown.", true)
	}
}

// cmdFirewall toggles the IPHub VPN/proxy firewall gate.
// Usage: /firewall on | /firewall off
// While the firewall is active, every new connection whose IP has not been seen
// before is checked against the IPHub API.  IPs classified as VPNs or proxies
// (block=1) are rejected immediately.  Already-known IPs and previously-checked
// IPs (cached within this session) are never sent to the API, keeping usage well
// within the free-tier daily limit of 1 000 requests.
// Requires an iphub_api_key to be set in config.toml.
func cmdFirewall(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	switch args[0] {
	case "on":
		if config.IPHubAPIKey == "" {
			client.SendServerMessage("Cannot enable firewall: no iphub_api_key is configured in config.toml.")
			return
		}
		firewallActive.Store(true)
		clients.ForEach(func(c *Client) {
			if c.Uid() != -1 && permissions.HasPermission(c.Perms(), permissions.PermissionField["BAN"]) {
				c.SendPacket("CT", "OOC", "🔥 VPN firewall is now ACTIVE. New connections will be screened against IPHub.", "1")
			}
		})
		client.SendServerMessage("Firewall enabled. New IPs will be checked via IPHub.")
		addToBuffer(client, "CMD", "Enabled IPHub firewall.", true)
	case "off":
		firewallActive.Store(false)
		clients.ForEach(func(c *Client) {
			if c.Uid() != -1 && permissions.HasPermission(c.Perms(), permissions.PermissionField["BAN"]) {
				c.SendPacket("CT", "OOC", "🔓 VPN firewall has been DISABLED. New connections are no longer screened.", "1")
			}
		})
		client.SendServerMessage("Firewall disabled.")
		addToBuffer(client, "CMD", "Disabled IPHub firewall.", true)
	default:
		client.SendServerMessage("Invalid argument. " + usage)
	}
}

// cmdBotBan bans all currently-connected spectators whose total playtime
// (accumulated from previous sessions plus the current session) is less than
// the configured botban_playtime_threshold (default 120 seconds).
// This is intended as a rapid response to bot floods.
func cmdBotBan(client *Client, _ []string, _ string) {
	threshold := int64(config.BotBanPlaytimeThreshold)
	banTime := time.Now().UTC().Unix()
	var count int
	bannedIPIDs := make(map[string]struct{})

	clients.ForEach(func(c *Client) {
		if c.CharID() != -1 {
			// Not a spectator – skip.
			return
		}

		// Accumulate DB playtime + current session time.
		dbPlaytime, err := db.GetPlaytime(c.Ipid())
		if err != nil {
			logger.LogErrorf("botban: failed to get playtime for IPID %v: %v", c.Ipid(), err)
		}
		var sessionSecs int64
		if connAt := c.ConnectedAt(); !connAt.IsZero() {
			sessionSecs = int64(time.Since(connAt).Seconds())
		}
		totalPlaytime := dbPlaytime + sessionSecs

		if totalPlaytime >= threshold {
			return
		}

		id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, -1, "Botban: spectator with insufficient playtime.", client.StoredModName())
		if err != nil {
			logger.LogErrorf("botban: failed to ban IPID %v: %v", c.Ipid(), err)
			return
		}
		c.SendPacketSync("KB", fmt.Sprintf("Botban: spectator with insufficient playtime.\nUntil: ∞\nID: %v", id))
		c.conn.Close()
		forgetIP(c.Ipid())
		count++
		bannedIPIDs[c.Ipid()] = struct{}{}
	})

	// Build the report string from unique IPIDs.
	var reportParts []string
	for ipid := range bannedIPIDs {
		reportParts = append(reportParts, ipid)
	}
	report := strings.Join(reportParts, ", ")

	client.SendServerMessage(fmt.Sprintf("Botban complete. Banned %v spectator(s).", count))
	if count > 0 {
		addToBuffer(client, "CMD", fmt.Sprintf("Botbanned %v spectator(s): %v", count, report), true)
		if err := webhook.PostBotBan(count, report, client.DisplayModName()); err != nil {
			logger.LogErrorf("while posting botban webhook: %v", err)
		}
	}
	sendPlayerArup()
}

// cmdSetGlobalNewIPLimit updates the global new-IP rate limit at runtime.
// The new value takes effect immediately for all subsequent connections.
func cmdSetGlobalNewIPLimit(client *Client, args []string, usage string) {
	val, err := strconv.Atoi(args[0])
	if err != nil || val < 0 {
		client.SendServerMessage("Invalid value. Must be a non-negative integer (0 = disabled).\n" + usage)
		return
	}
	config.GlobalNewIPRateLimit = val
	client.SendServerMessage(fmt.Sprintf("Global new-IP rate limit set to %v.", val))
	addToBuffer(client, "CMD", fmt.Sprintf("Set global new-IP rate limit to %v.", val), true)
}

// cmdSetGlobalIPWindow updates the global new-IP rate limit time window at runtime.
// The new value takes effect immediately for all subsequent connections.
func cmdSetGlobalIPWindow(client *Client, args []string, usage string) {
	val, err := strconv.Atoi(args[0])
	if err != nil || val <= 0 {
		client.SendServerMessage("Invalid value. Must be a positive integer (seconds).\n" + usage)
		return
	}
	config.GlobalNewIPRateLimitWindow = val
	client.SendServerMessage(fmt.Sprintf("Global new-IP rate limit window set to %v second(s).", val))
	addToBuffer(client, "CMD", fmt.Sprintf("Set global new-IP rate limit window to %v seconds.", val), true)
}

// cmdSetPlayerLimit updates the player capacity lockdown threshold at runtime.
// While active, new join attempts are rejected once the connected player count
// reaches the threshold.  Use 0 to disable the threshold.
func cmdSetPlayerLimit(client *Client, args []string, usage string) {
	val, err := strconv.Atoi(args[0])
	if err != nil || val < 0 || val > math.MaxInt32 {
		client.SendServerMessage("Invalid value. Must be a non-negative integer (0 = disabled).\n" + usage)
		return
	}
	playerLockdownThreshold.Store(int32(val))
	if val == 0 {
		client.SendServerMessage("Player capacity lockdown disabled.")
		addToBuffer(client, "CMD", "Disabled player capacity lockdown.", true)
	} else {
		client.SendServerMessage(fmt.Sprintf("Player capacity lockdown set to %v. New connections will be rejected once %v player(s) are connected.", val, val))
		addToBuffer(client, "CMD", fmt.Sprintf("Set player capacity lockdown threshold to %v.", val), true)
	}
}

// cmdPurgeDB purges all entries from the KNOWN_IPS table and clears the
// in-memory first-seen tracker.  After this command every IPID will be treated
// as completely new on its next connection.
func cmdPurgeDB(client *Client, _ []string, _ string) {
	n, err := db.PurgeKnownIPs()
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Failed to purge KNOWN_IPS: %v", err))
		logger.LogErrorf("purgedb: %v", err)
		return
	}
	resetKnownIPTracker()
	client.SendServerMessage(fmt.Sprintf("Purged %v known IP record(s) from the database.", n))
	addToBuffer(client, "CMD", fmt.Sprintf("Purged %v known IP record(s) from the database.", n), true)
}

// Handles /charstuck

func cmdCharStuck(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.String("d", "perma", "")
	reason := flags.String("r", "", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	uid, err := strconv.Atoi(flags.Arg(0))
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client not found.")
		return
	}

	if target.CharID() == -1 {
		client.SendServerMessage("Target is not on a character. They must be on a character to be stuck.")
		return
	}

	isPerma := strings.ToLower(*duration) == "perma"
	var stuckUntil time.Time
	if isPerma {
		stuckUntil = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		parsedDur, err := str2duration.ParseDuration(*duration)
		if err != nil {
			client.SendServerMessage("Failed to apply char-stuck: Cannot parse duration.")
			return
		}
		stuckUntil = time.Now().UTC().Add(parsedDur)
	}

	charID := target.CharID()
	charName := characters[charID]

	target.SetCharStuck(charID, stuckUntil)

	if err := db.UpsertCharStuck(target.Ipid(), charID, stuckUntil.Unix(), *reason); err != nil {
		logger.LogErrorf("Failed to persist char-stuck for %v: %v", target.Ipid(), err)
	}

	msg := fmt.Sprintf("You have been stuck on %v and cannot change characters.", charName)
	if !isPerma {
		msg = fmt.Sprintf("You have been stuck on %v for %v and cannot change characters.", charName, *duration)
	}
	if *reason != "" {
		msg += " Reason: " + *reason
	}
	target.SendServerMessage(msg)

	client.SendServerMessage(fmt.Sprintf("Stuck [%v] %v on character %v.", uid, target.OOCName(), charName))

	logMsg := fmt.Sprintf("Stuck [%v] %v on character %v", uid, target.OOCName(), charName)
	if *reason != "" {
		logMsg += " for reason: " + *reason
	}
	addToBuffer(client, "CMD", logMsg, false)
}

// Handles /uncharstuck

func cmdUnCharStuck(client *Client, args []string, _ string) {
	toUnstuck := getUidList(strings.Split(args[0], ","))
	var count int
	var sb strings.Builder
	for _, c := range toUnstuck {
		if !c.IsCharStuck() {
			continue
		}
		c.ClearCharStuck()
		if err := db.DeleteCharStuck(c.Ipid()); err != nil {
			logger.LogErrorf("Failed to remove char-stuck for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Your character-stuck restriction has been lifted.")
		count++
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(c.Uid()))
	}
	client.SendServerMessage(fmt.Sprintf("Lifted char-stuck from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Lifted char-stuck from %v.", sb.String()), false)
}

// Handles /charcurse

func cmdCharCurse(client *Client, args []string, usage string) {
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

	charName := strings.Join(args[1:], " ")
	charID := getCharacterID(charName)
	if charID == -1 {
		client.SendServerMessage(fmt.Sprintf("Character \"%s\" not found.", charName))
		return
	}

	if target.Area().IsTaken(charID) && target.CharID() != charID {
		client.SendServerMessage(fmt.Sprintf("Character \"%s\" is already taken in that area.", charName))
		return
	}

	target.ChangeCharacter(charID)
	target.SendServerMessage(fmt.Sprintf("A moderator has forced you to play as %s. You may change characters freely.", charName))
	client.SendServerMessage(fmt.Sprintf("Forced UID %d to character %s.", uid, charName))
	addToBuffer(client, "CMD", fmt.Sprintf("Char-cursed UID %d to character %s.", uid, charName), false)
}

// cmdIgnore permanently ignores a user based on their IPID so their IC and OOC
// messages are no longer shown to the caller. The ignore persists across
// reconnections. The target is warned without revealing the caller's IPID.
func cmdIgnore(client *Client, args []string, usage string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client not found.")
		return
	}
	if target == client {
		client.SendServerMessage("You cannot ignore yourself.")
		return
	}

	if target.Authenticated() && permissions.IsModerator(target.Perms()) {
		client.SendServerMessage("You cannot ignore a moderator or administrator.")
		return
	}

	targetIPID := target.Ipid()
	if client.IgnoresIPID(targetIPID) {
		client.SendServerMessage("You are already permanently ignoring that user.")
		return
	}

	client.AddIgnoredIPID(targetIPID)
	if err := db.AddIgnoredIP(client.Ipid(), targetIPID); err != nil {
		logger.LogErrorf("Failed to persist ignore for %v -> %v: %v", client.Ipid(), targetIPID, err)
	}

	// Warn the target without revealing the ignorer's IPID.
	target.SendServerMessage("⚠️ Warning: You have been permanently ignored by another user. This will persist across your reconnections.")

	client.SendServerMessage(fmt.Sprintf("You are now permanently ignoring user [%d]. This will persist across their reconnections.", uid))
	addToBuffer(client, "CMD", fmt.Sprintf("permanently ignored UID %d (IPID: %v)", uid, targetIPID), false)
}

// cmdUnignore removes a permanent IPID-based ignore for the given UID.
func cmdUnignore(client *Client, args []string, usage string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client not found.")
		return
	}

	targetIPID := target.Ipid()
	if !client.IgnoresIPID(targetIPID) {
		client.SendServerMessage("You are not ignoring that user.")
		return
	}

	client.RemoveIgnoredIPID(targetIPID)
	if err := db.RemoveIgnoredIP(client.Ipid(), targetIPID); err != nil {
		logger.LogErrorf("Failed to remove persistent ignore for %v -> %v: %v", client.Ipid(), targetIPID, err)
	}

	client.SendServerMessage(fmt.Sprintf("Unignored user [%d].", uid))
	addToBuffer(client, "CMD", fmt.Sprintf("unignored UID %d (IPID: %v)", uid, targetIPID), false)
}

// cmdModnote manages per-IPID freeform moderator notes.
// Usage: /modnote add <ipid> <note>
//
//	/modnote list <ipid>
//	/modnote delete <id>
func cmdModnote(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		ipid := args[1]
		note := strings.Join(args[2:], " ")
		if err := db.AddModnote(ipid, note, client.StoredModName()); err != nil {
			logger.LogErrorf("Failed to add modnote for IPID %v: %v", ipid, err)
			client.SendServerMessage("Failed to add note.")
			return
		}
		client.SendServerMessage(fmt.Sprintf("Note added for IPID %v.", ipid))
		addToBuffer(client, "CMD", fmt.Sprintf("Added modnote for IPID %v: %v", ipid, note), true)

	case "list":
		if len(args) < 2 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		ipid := args[1]
		notes, err := db.GetModnotes(ipid)
		if err != nil {
			logger.LogErrorf("Failed to fetch modnotes for IPID %v: %v", ipid, err)
			client.SendServerMessage("Failed to retrieve notes.")
			return
		}
		if len(notes) == 0 {
			client.SendServerMessage(fmt.Sprintf("No notes found for IPID %v.", ipid))
			return
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Notes for IPID %v:\n", ipid)
		for _, n := range notes {
			ts := time.Unix(n.AddedAt, 0).UTC().Format("2006-01-02 15:04 UTC")
			fmt.Fprintf(&b, "[%d] %v | by %v | %v\n", n.ID, ts, RenderStoredModName(n.AddedBy, client.Perms()), n.Note)
		}
		client.SendServerMessage(strings.TrimRight(b.String(), "\n"))

	case "delete":
		if len(args) < 2 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			client.SendServerMessage("Invalid note ID.")
			return
		}
		if err := db.DeleteModnote(id); err != nil {
			if err == sql.ErrNoRows {
				client.SendServerMessage(fmt.Sprintf("No note with ID %d found.", id))
			} else {
				logger.LogErrorf("Failed to delete modnote ID %d: %v", id, err)
				client.SendServerMessage("Failed to delete note.")
			}
			return
		}
		client.SendServerMessage(fmt.Sprintf("Deleted note #%d.", id))
		addToBuffer(client, "CMD", fmt.Sprintf("Deleted modnote #%d.", id), true)

	default:
		client.SendServerMessage("Unknown subcommand. " + usage)
	}
}
