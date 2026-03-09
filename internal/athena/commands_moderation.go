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
	"flag"
	"fmt"
	"io"
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
	var report string
	if len(*uids) > 0 {
		for _, c := range getUidList(*uids) {
			id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, until, reason, client.ModName())
			if err != nil {
				continue
			}
			if !strings.Contains(report, c.Ipid()) {
				report += c.Ipid() + ", "
			}
			c.SendPacket("KB", fmt.Sprintf("%v\nUntil: %v\nID: %v", reason, untilS, id))
			c.conn.Close()
			forgetIP(c.Ipid())
			count++
			if err := webhook.PostBan(c.CurrentCharacter(), c.Showname(), c.OOCName(), c.Ipid(), c.Uid(), id, *duration, reason, client.ModName()); err != nil {
				logger.LogErrorf("while posting ban webhook: %v", err)
			}
		}
	} else {
		for _, ipid := range *ipids {
			onlineClients := getClientsByIpid(ipid)
			if len(onlineClients) == 0 {
				// Offline ban – no HDID available.
				id, err := db.AddBan(ipid, "", banTime, until, reason, client.ModName())
				if err != nil {
					continue
				}
				forgetIP(ipid)
				if err := webhook.PostBan("N/A", "N/A", "N/A", ipid, -1, id, *duration, reason, client.ModName()); err != nil {
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
					id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, until, reason, client.ModName())
					if err == nil {
						banIDByHdid[c.Hdid()] = id
					}
				}
				if len(banIDByHdid) == 0 {
					continue
				}
				forgetIP(ipid)
				for _, c := range onlineClients {
					if id, ok := banIDByHdid[c.Hdid()]; ok {
						c.SendPacket("KB", fmt.Sprintf("%v\nUntil: %v\nID: %v", reason, untilS, id))
						if err := webhook.PostBan(c.CurrentCharacter(), c.Showname(), c.OOCName(), ipid, c.Uid(), id, *duration, reason, client.ModName()); err != nil {
							logger.LogErrorf("while posting ban webhook: %v", err)
						}
					} else {
						c.SendPacket("KB", fmt.Sprintf("%v\nUntil: %v", reason, untilS))
					}
					c.conn.Close()
				}
			}
			if !strings.Contains(report, ipid) {
				report += ipid + ", "
			}
			count++
		}
	}
	report = strings.TrimSuffix(report, ", ")
	if len(*ipids) > 0 {
		client.SendServerMessage(fmt.Sprintf("Banned %v IPID(s).", count))
	} else {
		client.SendServerMessage(fmt.Sprintf("Banned %v clients.", count))
	}
	sendPlayerArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Banned %v from server for %v: %v.", report, *duration, reason), true)
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

	var report string
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
		report += fmt.Sprintf("%v, ", s)
	}
	report = strings.TrimSuffix(report, ", ")
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
	s := "Bans:\n----------"
	entry := func(b db.BanInfo) string {
		var d string
		if b.Duration == -1 {
			d = "∞"
		} else {
			d = time.Unix(b.Duration, 0).UTC().Format("02 Jan 2006 15:04 MST")
		}

		return fmt.Sprintf("\nID: %v\nIPID: %v\nHDID: %v\nBanned on: %v\nUntil: %v\nReason: %v\nModerator: %v\n----------",
			b.Id, b.Ipid, b.Hdid, time.Unix(b.Time, 0).UTC().Format("02 Jan 2006 15:04 MST"), d, b.Reason, b.Moderator)
	}
	if *banid > 0 {
		b, err := db.GetBan(db.BANID, *banid)
		if err != nil || len(b) == 0 {
			client.SendServerMessage("No ban with that ID exists.")
			return
		}
		s += entry(b[0])
	} else if *ipid != "" {
		bans, err := db.GetBan(db.IPID, *ipid)
		if err != nil || len(bans) == 0 {
			client.SendServerMessage("No bans with that IPID exist.")
			return
		}
		for _, b := range bans {
			s += entry(b)
		}
	} else {
		bans, err := db.GetRecentBans()
		if err != nil {
			logger.LogErrorf("while getting recent bans: %v", err)
			client.SendServerMessage("An unexpected error occured.")
			return
		}
		for _, b := range bans {
			s += entry(b)
		}
	}
	client.SendServerMessage(s)
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
	writeToAll("CT", fmt.Sprintf("[GLOBAL] %v", client.OOCName()), strings.Join(args, " "), "1")
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
	var report string
	reason := strings.Join(flags.Args(), " ")
	for _, c := range toKick {
		report += c.Ipid() + ", "
		c.SendPacket("KK", reason)
		c.conn.Close()
		count++
		if err := webhook.PostKick(c.CurrentCharacter(), c.Showname(), c.OOCName(), c.Ipid(), reason, client.ModName(), c.Uid()); err != nil {
			logger.LogErrorf("while posting kick webhook: %v", err)
		}
	}
	report = strings.TrimSuffix(report, ", ")
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
		if permissions.IsModerator(perms) {
			client.SendServerMessage("Logged in as moderator.")
		}
		client.SendPacket("AUTH", "1")
		client.SendServerMessage(fmt.Sprintf("Welcome, %v.", args[0]))
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
	}
	addToBuffer(client, "AUTH", fmt.Sprintf("Logged out as %v.", client.ModName()), true)
	client.RemoveAuth()
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
	for c := range clients.GetAllClients() {
		if permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			c.SendPacket("CT", fmt.Sprintf("[MODCHAT] %v", client.OOCName()), msg, "1")
		}
	}
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
	var report string
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
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
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
	var report string
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
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Parroted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Parroted %v.", report), false)
}

// Handles /play

func cmdPlayers(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	all := flags.Bool("a", false, "")
	flags.Parse(args)

	isAdmin   := permissions.HasPermission(client.Perms(), permissions.PermissionField["ADMIN"])
	hasBanInfo := permissions.HasPermission(client.Perms(), permissions.PermissionField["BAN_INFO"])
	targetArea := client.Area()

	// Group clients by area in a single snapshot pass.
	type areaClients struct {
		list []*Client
	}
	grouped := make(map[*area.Area]*areaClients, len(areas))
	for c := range clients.GetAllClients() {
		a := c.Area()
		if !*all && a != targetArea {
			continue
		}
		if !isAdmin && c.Hidden() {
			continue
		}
		ac := grouped[a]
		if ac == nil {
			ac = &areaClients{}
			grouped[a] = ac
		}
		ac.list = append(ac.list, c)
	}

	// writeEntry appends a single client's info to the builder.
	writeEntry := func(b *strings.Builder, c *Client) {
		if c.Hidden() {
			b.WriteString("[HIDDEN] ")
		}
		fmt.Fprintf(b, "[%v] %v\n", c.Uid(), c.CurrentCharacter())
		if hasBanInfo {
			if permissions.IsModerator(c.Perms()) {
				fmt.Fprintf(b, "Mod: %v\n", c.ModName())
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
		for _, a := range areas {
			printArea(&out, a)
			out.WriteString("----------\n")
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
		c.SendPacket("CT", fmt.Sprintf("[PM] %v", client.OOCName()), msg, "1")
		recipientNames = append(recipientNames, c.OOCName())
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

	for c := range clients.GetAllClients() {
		if c.Authenticated() && c.ModName() == args[0] {
			c.RemoveAuth()
		}
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Removed user %v.", args[0]), true)
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

	for c := range clients.GetAllClients() {
		if c.Authenticated() && c.ModName() == args[0] {
			c.SetPerms(role.GetPermissions())
		}
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Updated role of %v to %v.", args[0], args[1]), true)
}

// Handles /status

func cmdUnban(client *Client, args []string, _ string) {
	toUnban := strings.Split(args[0], ",")
	var report string
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
		report += fmt.Sprintf("%v, ", s)
		if dbErr == nil && len(bans) > 0 {
			b := bans[0]
			var durStr string
			if b.Duration == -1 {
				durStr = "Permanent"
			} else {
				durStr = time.Unix(b.Duration, 0).UTC().Format("02 Jan 2006 15:04 MST")
			}
			if err := webhook.PostUnban(id, b.Ipid, b.Reason, durStr, b.Moderator, client.ModName()); err != nil {
				logger.LogErrorf("while posting unban webhook: %v", err)
			}
		}
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Nullified bans: %v", report))
	addToBuffer(client, "CMD", fmt.Sprintf("Nullified bans: %v", report), true)
}

// Handles /uncm

func cmdUnmute(client *Client, args []string, _ string) {
	toUnmute := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
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
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
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
		target.Ipid(), areaName, durationDisplay, *reason, client.OOCName(), uid); err != nil {
		logger.LogErrorf("Failed to post jail webhook: %v", err)
	}
}

// Handles /unjail

func cmdUnjail(client *Client, args []string, _ string) {
	toUnjail := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
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
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
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
	for c := range clients.GetAllClients() {
		if c.Uid() != -1 && c.Area() == targetArea {
			targets = append(targets, c)
			names = append(names, c.EffectiveShowname())
		}
	}

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

	// Apply shuffled shownames and broadcast PU updates.
	for i, c := range targets {
		c.SetForcedShowname(names[i])
		writeToAll("PU", strconv.Itoa(c.Uid()), "2", decode(names[i]))
		c.SendServerMessage("A moderator has shuffled the shownames in this area.")
	}

	client.SendServerMessage(fmt.Sprintf("Shuffled shownames of %d players in the area.", len(targets)))
	addToBuffer(client, "CMD", fmt.Sprintf("shuffled shownames of %d players in area %v", len(targets), targetArea.Name()), true)
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


// cmdLockdown toggles server lockdown mode.
// While active, only previously-known IPIDs (those already in the server's
// first-seen tracker) are allowed to connect; all new IPIDs are rejected.
func cmdLockdown(client *Client, _ []string, _ string) {
	active := !serverLockdown.Load()
	serverLockdown.Store(active)
	if active {
		writeToAll("CT", "OOC", "🔒 Server lockdown is now ACTIVE. New connections are restricted to known players.", "1")
		client.SendServerMessage("Lockdown enabled. New IPIDs will be rejected.")
		addToBuffer(client, "CMD", "Enabled server lockdown.", true)
	} else {
		writeToAll("CT", "OOC", "🔓 Server lockdown has been LIFTED. New connections are now allowed.", "1")
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
		writeToAll("CT", "OOC", "🔥 VPN firewall is now ACTIVE. New connections will be screened against IPHub.", "1")
		client.SendServerMessage("Firewall enabled. New IPs will be checked via IPHub.")
		addToBuffer(client, "CMD", "Enabled IPHub firewall.", true)
	case "off":
		firewallActive.Store(false)
		writeToAll("CT", "OOC", "🔓 VPN firewall has been DISABLED. New connections are no longer screened.", "1")
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

	for c := range clients.GetAllClients() {
		if c.CharID() != -1 {
			// Not a spectator – skip.
			continue
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
			continue
		}

		id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, -1, "Botban: spectator with insufficient playtime.", client.ModName())
		if err != nil {
			logger.LogErrorf("botban: failed to ban IPID %v: %v", c.Ipid(), err)
			continue
		}
		c.SendPacket("KB", fmt.Sprintf("Botban: spectator with insufficient playtime.\nUntil: ∞\nID: %v", id))
		c.conn.Close()
		forgetIP(c.Ipid())
		count++
		bannedIPIDs[c.Ipid()] = struct{}{}
	}

	// Build the report string from unique IPIDs.
	var reportParts []string
	for ipid := range bannedIPIDs {
		reportParts = append(reportParts, ipid)
	}
	report := strings.Join(reportParts, ", ")

	client.SendServerMessage(fmt.Sprintf("Botban complete. Banned %v spectator(s).", count))
	if count > 0 {
		addToBuffer(client, "CMD", fmt.Sprintf("Botbanned %v spectator(s): %v", count, report), true)
		if err := webhook.PostBotBan(count, report, client.ModName()); err != nil {
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
