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
	"strconv"
	"strings"
	"time"

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
	out := "\nPlayers\n----------\n"
	entry := func(c *Client, auth bool) string {
		s := fmt.Sprintf("[%v] %v\n", c.Uid(), c.CurrentCharacter())
		if auth {
			if permissions.IsModerator(c.Perms()) {
				s += fmt.Sprintf("Mod: %v\n", c.ModName())
			}
			s += fmt.Sprintf("IPID: %v\n", c.Ipid())
		}
		if c.OOCName() != "" {
			s += fmt.Sprintf("OOC: %v\n", c.OOCName())
		}
		return s
	}
	if *all {
		for _, a := range areas {
			out += fmt.Sprintf("%v:\n%v players online.\n", a.Name(), a.PlayerCount())
			for c := range clients.GetAllClients() {
				if c.Area() == a {
					out += entry(c, permissions.HasPermission(client.Perms(), permissions.PermissionField["BAN_INFO"]))
				}
			}
			out += "----------\n"
		}
	} else {
		out += fmt.Sprintf("%v:\n%v players online.\n", client.Area().Name(), client.Area().PlayerCount())
		for c := range clients.GetAllClients() {
			if c.Area() == client.Area() {
				out += entry(c, permissions.HasPermission(client.Perms(), permissions.PermissionField["BAN_INFO"]))
			}
		}
	}
	client.SendServerMessage(out)
}

// Handles /pm

func cmdPM(client *Client, args []string, _ string) {
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
	for _, c := range toPM {
		c.SendPacket("CT", fmt.Sprintf("[PM] %v", client.OOCName()), msg, "1")
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

	var jailUntil time.Time
	if strings.ToLower(*duration) == "perma" {
		jailUntil = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		parsedDur, err := str2duration.ParseDuration(*duration)
		if err != nil {
			client.SendServerMessage("Failed to jail: Cannot parse duration.")
			return
		}
		jailUntil = time.Now().UTC().Add(parsedDur)
	}

	target.SetJailedUntil(jailUntil)
	if err := db.UpsertJail(target.Ipid(), jailUntil.Unix(), *reason); err != nil {
		logger.LogErrorf("Failed to persist jail for %v: %v", target.Ipid(), err)
	}
	msg := fmt.Sprintf("You have been jailed in %v.", target.Area().Name())
	if strings.ToLower(*duration) != "perma" {
		msg = fmt.Sprintf("You have been jailed in %v for %v.", target.Area().Name(), *duration)
	}
	if *reason != "" {
		msg += " Reason: " + *reason
	}
	target.SendServerMessage(msg)
	
	client.SendServerMessage(fmt.Sprintf("Jailed [%v] %v in %v.", uid, target.OOCName(), target.Area().Name()))
	
	logMsg := fmt.Sprintf("Jailed [%v] %v", uid, target.OOCName())
	if *reason != "" {
		logMsg += " for reason: " + *reason
	}
	addToBuffer(client, "CMD", logMsg, false)
}

// Handles /unjail

func cmdUnjail(client *Client, args []string, _ string) {
	toUnjail := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnjail {
		if c.JailedUntil().IsZero() || time.Now().UTC().After(c.JailedUntil()) {
			continue
		}
		c.SetJailedUntil(time.Time{})
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

