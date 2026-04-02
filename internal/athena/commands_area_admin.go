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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
)

func cmdAbout(client *Client, _ []string, _ string) {
	client.SendServerMessage(fmt.Sprintf("Running Athena version %v.\nAthena is open source software; for documentation, bug reports, and source code, see: %v",
		version, "https://github.com/MangosArentLiterature/Athena."))
}

// Handles /allowcms

func cmdAllowCMs(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetCMsAllowed(true)
		result = "allowed"
	case "false":
		client.Area().SetCMsAllowed(false)
		result = "disallowed"
	default:
		client.SendServerMessage("Argument not recognized.")
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v CMs in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set allowing CMs to %v.", args[0]), false)
}

// Handles /allowiniswap

func cmdAllowIniswap(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetIniswapAllowed(true)
		result = "enabled"
	case "false":
		client.Area().SetIniswapAllowed(false)
		result = "disabled"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v iniswapping in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set iniswapping to %v.", args[0]), false)
}

// Handles /areainfo

func cmdAreaInfo(client *Client, _ []string, _ string) {
	a := client.Area()
	casinoStatus := "disabled"
	if a.CasinoEnabled() {
		minBet := a.CasinoMinBet()
		maxBet := a.CasinoMaxBet()
		casinoStatus = "enabled"
		if minBet > 0 || maxBet > 0 {
			casinoStatus = fmt.Sprintf("enabled (bet: %d–%d)", minBet, maxBet)
		}
		if a.CasinoJackpot() {
			casinoStatus += fmt.Sprintf(", jackpot pool: %d", a.CasinoJackpotPool())
		}
	}
	out := fmt.Sprintf("\nBG: %v\nEvi mode: %v\nAllow iniswap: %v\nNon-interrupting pres: %v\nCMs allowed: %v\nForce BG list: %v\nBG locked: %v\nMusic locked: %v\nSpectate mode: %v\nCasino: %v",
		a.Background(), a.EvidenceMode().String(), a.IniswapAllowed(), a.NoInterrupt(),
		a.CMsAllowed(), a.ForceBGList(), a.LockBG(), a.LockMusic(), a.SpectateMode(), casinoStatus)
	client.SendServerMessage(out)
}

// Handles /ban

func cmdBg(client *Client, args []string, _ string) {
	if client.Area().LockBG() && !permissions.HasPermission(client.Perms(), permissions.PermissionField["MODIFY_AREA"]) {
		client.SendServerMessage("You do not have permission to change the background in this area.")
		return
	}

	arg := strings.Join(args, " ")

	if client.Area().ForceBGList() && !sliceutil.ContainsString(backgrounds, arg) {
		client.SendServerMessage("Invalid background.")
		return
	}
	client.Area().SetBackground(arg)
	writeToArea(client.Area(), "BN", arg)
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the background to %v.", client.OOCName(), arg))
	addToBuffer(client, "CMD", fmt.Sprintf("Set BG to %v.", arg), false)
}

// Handles /charselect

func cmdCharSelect(client *Client, args []string, _ string) {
	if len(args) == 0 {
		if stuckID := client.charStuckID(); stuckID >= 0 {
			client.SendServerMessage(fmt.Sprintf("You are character stuck as %v and cannot return to character select.", characters[stuckID]))
			return
		}
		client.ChangeCharacter(-1)
		client.SendPacket("DONE")
	} else {
		if !client.HasCMPermission() {
			client.SendServerMessage("You do not have permission to use that command.")
			return
		}
		toChange := getUidList(strings.Split(args[0], ","))
		var count int
		var report string
		for _, c := range toChange {
			if c.Area() != client.Area() || c.CharID() == -1 {
				continue
			}
			c.ChangeCharacter(-1)
			c.SendPacket("DONE")
			c.SendServerMessage("You were moved back to character select.")
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Moved %v users to character select.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Moved %v to character select.", report), false)
	}
}

// Handles /randomchar

func cmdRandomChar(client *Client, _ []string, _ string) {
	// Enforce 5-second rate limit.
	const cooldown = 5 * time.Second
	if last := client.LastRandomCharTime(); !last.IsZero() && time.Since(last) < cooldown {
		remaining := int(time.Until(last.Add(cooldown)).Seconds()) + 1
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("Please wait %d %s before using /randomchar again.", remaining, unit))
		return
	}
	newid := getRandomFreeChar(client)
	if newid == -1 {
		client.SendServerMessage("No free characters available.")
		return
	}
	client.SetLastRandomCharTime(time.Now())
	client.ChangeCharacter(newid)
}

// Handles /randombg

func cmdRandomBg(client *Client, _ []string, _ string) {
	if len(backgrounds) == 0 {
		client.SendServerMessage("No backgrounds are available.")
		return
	}
	a := client.Area() // cache to avoid repeated mutex acquisitions
	if a.LockBG() && !permissions.HasPermission(client.Perms(), permissions.PermissionField["MODIFY_AREA"]) {
		client.SendServerMessage("You do not have permission to change the background in this area.")
		return
	}
	// Enforce 5-second rate limit with a single atomic check-and-update.
	const cooldown = 5 * time.Second
	if ok, remaining := client.CheckAndUpdateRandomBgCooldown(cooldown); !ok {
		secs := int(remaining.Seconds()) + 1
		unit := "seconds"
		if secs == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("Please wait %d %s before using /randombg again.", secs, unit))
		return
	}
	bg := backgrounds[rand.Intn(len(backgrounds))]
	a.SetBackground(bg)
	writeToArea(a, "BN", bg)
	sendAreaServerMessage(a, fmt.Sprintf("%v set the background to a random one (%v).", client.OOCName(), bg))
	addToBuffer(client, "CMD", fmt.Sprintf("Set BG to random (%v).", bg), false)
}

// Handles /forcerandomchar [uid]

func cmdForceRandomChar(client *Client, args []string, _ string) {
	// If a UID argument is provided, target only that specific player.
	if len(args) >= 1 {
		uid, err := strconv.Atoi(args[0])
		if err != nil {
			client.SendServerMessage("Invalid UID.")
			return
		}
		target, err := getClientByUid(uid)
		if err != nil {
			client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", uid))
			return
		}
		newid := getRandomFreeChar(target)
		if newid == -1 {
			client.SendServerMessage("No free characters available for that player.")
			return
		}
		target.ChangeCharacter(newid)
		target.SendServerMessage("An admin forced you to a random character.")
		client.SendServerMessage(fmt.Sprintf("Forced UID %v to a random character.", uid))
		addToBuffer(client, "CMD", fmt.Sprintf("Force random char on UID %v.", uid), false)
		return
	}

	// No UID provided — target all players in the current area.
	var count int
	var reportBuilder strings.Builder
	clients.ForEach(func(c *Client) {
		if c.Area() != client.Area() {
			return
		}
		newid := getRandomFreeChar(c)
		if newid == -1 {
			return
		}
		c.ChangeCharacter(newid)
		if c != client {
			c.SendServerMessage("An admin forced all players in the area to a random character.")
		}
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		reportBuilder.WriteString(fmt.Sprintf("%v", c.Uid()))
		count++
	})
	if count > 0 {
		client.SendServerMessage(fmt.Sprintf("Forced %v player(s) in the area to a random character.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Force random char on %v user(s) (%v) in area.", count, reportBuilder.String()), false)
	} else {
		client.SendServerMessage("No players in the area had their character changed.")
	}
}

// Handles /cm

func cmdCM(client *Client, args []string, _ string) {
	if client.CharID() == -1 {
		client.SendServerMessage("You are spectating; you cannot become a CM.")
		return
	} else if !client.Area().CMsAllowed() && !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}

	if len(args) == 0 {
		if client.Area().HasCM(client.Uid()) {
			client.SendServerMessage("You are already a CM in this area.")
			return
		} else if len(client.Area().CMs()) > 0 && !permissions.HasPermission(client.Perms(), permissions.PermissionField["CM"]) {
			client.SendServerMessage("This area already has a CM.")
			return
		}
		client.Area().AddCM(client.Uid())
		client.SendServerMessage("Successfully became a CM.")
		addToBuffer(client, "CMD", "CMed self.", false)
	} else {
		if !client.HasCMPermission() {
			client.SendServerMessage("You do not have permission to use that command.")
			return
		}
		toCM := getUidList(strings.Split(args[0], ","))
		var count int
		var report string
		for _, c := range toCM {
			if c.Area() != client.Area() || c.Area().HasCM(c.Uid()) {
				continue
			}
			c.Area().AddCM(c.Uid())
			c.SendServerMessage("You have become a CM in this area.")
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("CMed %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("CMed %v.", report), false)
	}
	sendCMArup()
}

// Handles /doc

func cmdDoc(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	clear := flags.Bool("c", false, "")
	flags.Parse(args)
	if len(args) == 0 {
		if client.Area().Doc() == "" {
			client.SendServerMessage("This area does not have a doc set.")
			return
		}
		client.SendServerMessage(client.Area().Doc())
		return
	} else {
		if !client.HasCMPermission() {
			client.SendServerMessage("You do not have permission to change the doc.")
			return
		} else if *clear {
			client.Area().SetDoc("")
			sendAreaServerMessage(client.Area(), fmt.Sprintf("%v cleared the doc.", client.OOCName()))
			return
		} else if len(flags.Args()) != 0 {
			client.Area().SetDoc(flags.Arg(0))
			sendAreaServerMessage(client.Area(), fmt.Sprintf("%v updated the doc.", client.OOCName()))
			return
		}
	}
}

// Handles /editban

func cmdSetEviMod(client *Client, args []string, _ string) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to change the evidence mode.")
		return
	}
	switch args[0] {
	case "mods":
		if !permissions.HasPermission(client.Perms(), permissions.PermissionField["MOD_EVI"]) {
			client.SendServerMessage("You do not have permission for this evidence mode.")
			return
		}
		client.Area().SetEvidenceMode(area.EviMods)
	case "cms":
		client.Area().SetEvidenceMode(area.EviCMs)
	case "any":
		client.Area().SetEvidenceMode(area.EviAny)
	default:
		client.SendServerMessage("Invalid evidence mode.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the evidence mode to %v.", client.OOCName(), args[0]))
	addToBuffer(client, "CMD", fmt.Sprintf("Set the evidence mode to %v.", args[0]), false)
}

// Handles /bglist

func cmdBgList(client *Client, _ []string, _ string) {
	if len(backgrounds) == 0 {
		client.SendServerMessage("No backgrounds are available.")
		return
	}
	client.SendServerMessage(bgListStr)
}

// Handles /forcebglist

func cmdForceBGList(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetForceBGList(true)
		result = "enforced"
	case "false":
		client.Area().SetForceBGList(false)
		result = "unenforced"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v the BG list in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set the BG list to %v.", args[0]), false)
}

// Handles /getban

func cmdInvite(client *Client, args []string, _ string) {
	if client.Area().Lock() == area.LockFree {
		client.SendServerMessage("This area is unlocked.")
		return
	}
	toInvite := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toInvite {
		if client.Area().AddInvited(c.Uid()) {
			c.SendServerMessage(fmt.Sprintf("You were invited to area %v.", client.Area().Name()))
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Invited %v users.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Invited %v to the area.", report), false)
}

// Handles /kick

func cmdAreaKick(client *Client, args []string, _ string) {
	if client.Area() == areas[0] {
		client.SendServerMessage("Failed to kick: Cannot kick a user from area 0.")
		return
	}
	toKick := getUidList(strings.Split(args[0], ","))

	var count int
	var report string
	for _, c := range toKick {
		if c.Area() != client.Area() || permissions.HasPermission(c.Perms(), permissions.PermissionField["BYPASS_LOCK"]) {
			continue
		}
		if c == client {
			client.SendServerMessage("You can't kick yourself from the area.")
			continue
		}
		c.ChangeArea(areas[0])
		c.SendServerMessage("You were kicked from the area!")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Kicked %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Kicked %v from area.", report), false)
}

// Handles /kickother

func cmdKickOther(client *Client, args []string, _ string) {
	var count int
	for _, c := range getClientsByIpid(client.Ipid()) {
		if c == client {
			continue
		}
		c.SendPacket("KK", "Ghost client kicked.")
		c.conn.Close()
		count++
	}
	client.SendServerMessage(fmt.Sprintf("Kicked %v ghost client(s).", count))
	sendPlayerArup()
}

// Handles /lock

func cmdLock(client *Client, args []string, _ string) {
	if sliceutil.ContainsString(args, "-s") { // Set area to spectatable.
		client.Area().SetLock(area.LockSpectatable)
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the area to spectatable.", client.OOCName()))
		addToBuffer(client, "CMD", "Set the area to spectatable.", false)
	} else { // Normal lock.
		if client.Area().Lock() == area.LockLocked {
			client.SendServerMessage("This area is already locked.")
			return
		} else if client.Area() == areas[0] {
			client.SendServerMessage("You cannot lock area 0.")
			return
		}
		client.Area().SetLock(area.LockLocked)
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v locked the area.", client.OOCName()))
		addToBuffer(client, "CMD", "Locked the area.", false)
	}
	targetArea := client.Area()
	clients.ForEach(func(c *Client) {
		if c.Area() == targetArea {
			targetArea.AddInvited(c.Uid())
		}
	})
	sendLockArup()
}

// Handles /lockbg

func cmdLockBG(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetLockBG(true)
		result = "locked"
	case "false":
		client.Area().SetLockBG(false)
		result = "unlocked"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v the background in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set the background to %v.", args[0]), false)
}

// Handles /lockmusic

func cmdLockMusic(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetLockMusic(true)
		result = "enabled"
	case "false":
		client.Area().SetLockMusic(false)
		result = "disabled"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v CM-only music in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set CM-only music list to %v.", args[0]), false)
}

// Handles /log

func cmdLog(client *Client, args []string, _ string) {
	wantedArea, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid area.")
		return
	}
	for i, a := range areas {
		if i == wantedArea {
			client.SendServerMessage(strings.Join(a.Buffer(), "\n"))
			return
		}
	}
	client.SendServerMessage("Invalid area.")
}

// Handles /login

func cmdMotd(client *Client, _ []string, _ string) {
	client.SendServerMessage(config.Motd)
}

// Handles /move

func cmdMove(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	uids := &[]string{}
	flags.Var(&cmdParamList{uids}, "u", "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	areaID, err := strconv.Atoi(flags.Arg(0))
	if err != nil || areaID < 0 || areaID > len(areas)-1 {
		client.SendServerMessage("Invalid area.")
		return
	}
	wantedArea := areas[areaID]

	if len(*uids) > 0 {
		if !permissions.HasPermission(client.Perms(), permissions.PermissionField["MOVE_USERS"]) {
			client.SendServerMessage("You do not have permission to use that command.")
			return
		}
		toMove := getUidList(*uids)
		var count int
		var report string
		for _, c := range toMove {
			if !c.ChangeArea(wantedArea) {
				continue
			}
			c.SendServerMessage(fmt.Sprintf("You were moved to %v.", wantedArea.Name()))
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Moved %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Moved %v to %v.", report, wantedArea.Name()), false)
	} else {
		if !client.ChangeArea(wantedArea) {
			client.SendServerMessage("You are not invited to that area.")
		}
		client.SendServerMessage(fmt.Sprintf("Moved to %v.", wantedArea.Name()))
	}
}

// Handles /summon

func cmdSummon(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	
	areaID, err := strconv.Atoi(args[0])
	if err != nil || areaID < 0 || areaID > len(areas)-1 {
		client.SendServerMessage("Invalid area.")
		return
	}
	wantedArea := areas[areaID]
	wantedAreaName := wantedArea.Name()

	var count int
	var reportBuilder strings.Builder

	// Move each client to the target area
	clients.ForEach(func(c *Client) {
		if !c.ChangeArea(wantedArea) {
			return
		}

		// Send appropriate message based on whether this is the admin
		if c == client {
			c.SendServerMessage(fmt.Sprintf("Summoned all users to %v.", wantedAreaName))
		} else {
			c.SendServerMessage(fmt.Sprintf("You were summoned to %v.", wantedAreaName))
		}

		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		reportBuilder.WriteString(fmt.Sprintf("%v", c.Uid()))
		count++
	})
	
	report := reportBuilder.String()
	if count > 0 {
		addToBuffer(client, "CMD", fmt.Sprintf("Summoned %v user(s) (%v) to %v.", count, report, wantedArea.Name()), false)
	} else {
		client.SendServerMessage("No users were summoned.")
	}
}

// Handles /mute

func cmdNarrator(client *Client, _ []string, _ string) {
	client.ToggleNarrator()
}

// Handles /nointpres

func cmdNoIntPres(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetNoInterrupt(true)
		result = "enabled"
	case "false":
		client.Area().SetNoInterrupt(false)
		result = "disabled"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v non-interrupting preanims in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set non-interrupting preanims to %v.", args[0]), false)
}

// Handles /play

func cmdPlay(client *Client, args []string, _ string) {
	if !client.CanChangeMusic() {
		client.SendServerMessage("You are not allowed to change the music in this area.")
		return
	}
	s := strings.Join(args, " ")

	// Check if the song we got is a URL for streaming
	if _, err := url.ParseRequestURI(s); err == nil {
		s, err = url.QueryUnescape(s) // Unescape any URL encoding
		if err != nil {
			client.SendServerMessage("Error parsing URL.")
			return
		}
	}
	writeToArea(client.Area(), "MC", s, fmt.Sprint(client.CharID()), client.Showname(), "1", "0")
}

// Handles /randomsong

func cmdRandomSong(client *Client, _ []string, _ string) {
	if !client.CanChangeMusic() {
		client.SendServerMessage("You are not allowed to change the music in this area.")
		return
	}
	// Enforce configurable cooldown with a single atomic check-and-update.
	if cd := config.RandomSongCooldown; cd > 0 {
		cooldown := time.Duration(cd) * time.Second
		if ok, remaining := client.CheckAndUpdateRandomSongCooldown(cooldown); !ok {
			secs := int(remaining.Seconds()) + 1
			unit := "seconds"
			if secs == 1 {
				unit = "second"
			}
			client.SendServerMessage(fmt.Sprintf("Please wait %d %s before using /randomsong again.", secs, unit))
			return
		}
	}
	// Collect playable songs from the jukebox list (music.txt).
	// Category headers in music.txt have no '.'; skip them just as pktAM does.
	playable := make([]string, 0, len(music))
	for _, entry := range music {
		if strings.ContainsRune(entry, '.') {
			playable = append(playable, entry)
		}
	}
	if len(playable) == 0 {
		client.SendServerMessage("No songs are available.")
		return
	}
	song := playable[rand.Intn(len(playable))]
	writeToArea(client.Area(), "MC", song, fmt.Sprint(client.CharID()), client.Showname(), "1", "0")
	addToBuffer(client, "CMD", fmt.Sprintf("Played random song (%v).", song), false)
}

// Handles /players

func cmdStatus(client *Client, args []string, _ string) {
	switch strings.ToLower(args[0]) {
	case "idle":
		client.Area().SetStatus(area.StatusIdle)
	case "looking-for-players":
		client.Area().SetStatus(area.StatusPlayers)
	case "casing":
		client.Area().SetStatus(area.StatusCasing)
	case "recess":
		client.Area().SetStatus(area.StatusRecess)
	case "rp":
		client.Area().SetStatus(area.StatusRP)
	case "gaming":
		client.Area().SetStatus(area.StatusGaming)
	default:
		client.SendServerMessage("Status not recognized. Recognized statuses: idle, looking-for-players, casing, recess, rp, gaming")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the status to %v.", client.OOCName(), args[0]))
	sendStatusArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Set the status to %v.", args[0]), false)
}

// Handles swapevi

func cmdSwapEvi(client *Client, args []string, _ string) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to alter evidence in this area.")
		return
	}
	evi1, err := strconv.Atoi(args[0])
	if err != nil {
		return
	}
	evi2, err := strconv.Atoi(args[1])
	if err != nil {
		return
	}
	if client.Area().SwapEvidence(evi1, evi2) {
		client.SendServerMessage("Evidence swapped.")
		writeToArea(client.Area(), "LE", client.Area().Evidence()...)
		addToBuffer(client, "CMD", fmt.Sprintf("Swapped posistions of evidence %v and %v.", evi1, evi2), false)
	} else {
		client.SendServerMessage("Invalid arguments.")
	}
}

// Handles /testify

func cmdTestify(client *Client, _ []string, _ string) {
	if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	if client.Area().TstState() != area.TRIdle {
		client.SendServerMessage("The recorder is currently active.")
		return
	}
	client.Area().TstClear()
	client.Area().SetTstState(area.TRRecording)
	client.SendServerMessage("Recording testimony.")
}

// Handles /pause (stops testimony recording)

func cmdPause(client *Client, _ []string, _ string) {
	if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	client.Area().SetTstState(area.TRIdle)
	client.SendServerMessage("Recorder stopped.")
	client.Area().TstJump(0)
	writeToArea(client.Area(), "RT", "testimony1#1")
}

// Handles /examine

func cmdExamine(client *Client, _ []string, _ string) {
	if !client.Area().HasTestimony() {
		client.SendServerMessage("No testimony recorded.")
		return
	}
	client.Area().SetTstState(area.TRPlayback)
	client.SendServerMessage("Starting cross-examination.")
	writeToArea(client.Area(), "RT", "testimony2")
	writeToArea(client.Area(), "MS", client.Area().CurrentTstStatement())
}

// Handles /update

func cmdUpdate(client *Client, _ []string, _ string) {
	if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	if client.Area().TstState() != area.TRPlayback {
		client.SendServerMessage("The recorder is not in playback mode.")
		return
	}
	client.Area().SetTstState(area.TRUpdating)
	client.SendServerMessage("Send the new statement in IC to update the current one.")
}

// Handles /add

func cmdAdd(client *Client, _ []string, _ string) {
	if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	if client.Area().TstState() != area.TRPlayback {
		client.SendServerMessage("The recorder is not in playback mode.")
		return
	}
	client.Area().SetTstState(area.TRInserting)
	client.SendServerMessage("Send the new statement in IC to add it to the testimony.")
}

// Handles /delete

func cmdDelete(client *Client, _ []string, _ string) {
	if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	if client.Area().TstState() != area.TRPlayback {
		client.SendServerMessage("The recorder is not in playback mode.")
		return
	}
	if client.Area().CurrentTstIndex() > 0 {
		err := client.Area().TstRemove()
		if err != nil {
			client.SendServerMessage("Failed to delete statement.")
		}
	} else {
		client.SendServerMessage("Cannot delete the testimony title.")
	}
}

// Handles /testimony

func cmdTestimony(client *Client, args []string, _ string) {
	if len(args) == 0 {
		if !client.Area().HasTestimony() {
			client.SendServerMessage("This area has no recorded testimony.")
			return
		}
		client.SendServerMessage(strings.Join(client.area.Testimony(), "\n"))
		return
	} else if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	switch args[0] {
	case "record":
		if client.Area().TstState() != area.TRIdle {
			client.SendServerMessage("The recorder is currently active.")
			return
		}
		client.Area().TstClear()
		client.Area().SetTstState(area.TRRecording)
		client.SendServerMessage("Recording testimony.")
	case "stop":
		client.Area().SetTstState(area.TRIdle)
		client.SendServerMessage("Recorder stopped.")
		client.Area().TstJump(0)
		writeToArea(client.Area(), "RT", "testimony1#1")
	case "play":
		if !client.Area().HasTestimony() {
			client.SendServerMessage("No testimony recorded.")
			return
		}
		client.Area().SetTstState(area.TRPlayback)
		client.SendServerMessage("Playing testimony.")
		writeToArea(client.Area(), "RT", "testimony2")
		writeToArea(client.Area(), "MS", client.Area().CurrentTstStatement())
	case "update":
		if client.Area().TstState() != area.TRPlayback {
			client.SendServerMessage("The recorder is not active.")
			return
		}
		client.Area().SetTstState(area.TRUpdating)
	case "insert":
		if client.Area().TstState() != area.TRPlayback {
			client.SendServerMessage("The recorder is not active.")
			return
		}
		client.Area().SetTstState(area.TRInserting)
	case "delete":
		if client.Area().TstState() != area.TRPlayback {
			client.SendServerMessage("The recorder is not active.")
			return
		}
		if client.Area().CurrentTstIndex() > 0 {
			err := client.Area().TstRemove()
			if err != nil {
				client.SendServerMessage("Failed to delete statement.")
			}
		}
	}
}

// Handles /unban

func cmdUnCM(client *Client, args []string, _ string) {
	if len(args) == 0 {
		if !client.Area().HasCM(client.Uid()) {
			client.SendServerMessage("You are not a CM in this area.")
			return
		}
		client.Area().RemoveCM(client.Uid())
		client.SendServerMessage("You are no longer a CM in this area.")
		addToBuffer(client, "CMD", "Un-CMed self.", false)
	} else {
		toCM := getUidList(strings.Split(args[0], ","))
		var count int
		var report string
		for _, c := range toCM {
			if c.Area() != client.Area() || !c.Area().HasCM(c.Uid()) {
				continue
			}
			c.Area().RemoveCM(c.Uid())
			c.SendServerMessage("You are no longer a CM in this area.")
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Un-CMed %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Un-CMed %v.", report), false)
	}
	sendCMArup()
}

// Handles /uninvite

func cmdUninvite(client *Client, args []string, _ string) {
	if client.Area().Lock() == area.LockFree {
		client.SendServerMessage("This area is unlocked.")
		return
	}
	toUninvite := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUninvite {
		if c == client || client.Area().HasCM(c.Uid()) {
			continue
		}
		if client.Area().RemoveInvited(c.Uid()) {
			if c.Area() == client.Area() && client.Area().Lock() == area.LockLocked && !permissions.HasPermission(c.Perms(), permissions.PermissionField["BYPASS_LOCK"]) {
				c.SendServerMessage("You were kicked from the area!")
				c.ChangeArea(areas[0])
			}
			c.SendServerMessage(fmt.Sprintf("You were uninvited from area %v.", client.Area().Name()))
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Uninvited %v users.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Uninvited %v to the area.", report), false)
}

// Handles /unlock

func cmdUnlock(client *Client, _ []string, _ string) {
	if client.Area().Lock() == area.LockFree {
		client.SendServerMessage("This area is not locked.")
		return
	}
	client.Area().SetLock(area.LockFree)
	client.Area().ClearInvited()
	sendLockArup()
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v unlocked the area.", client.OOCName()))
	addToBuffer(client, "CMD", "Unlocked the area.", false)
}

// Handles /spectate

func cmdSpectate(client *Client, args []string, usage string) {
	if len(args) == 0 {
		// Toggle spectate mode
		if client.Area().SpectateMode() {
			client.Area().SetSpectateMode(false)
			client.SendServerMessage("Spectate mode disabled.")
			addToBuffer(client, "CMD", "Disabled spectate mode.", false)
		} else {
			client.Area().SetSpectateMode(true)
			client.SendServerMessage("Spectate mode enabled. Only CMs and invited players can speak in IC.")
			addToBuffer(client, "CMD", "Enabled spectate mode.", false)
		}
		return
	}

	switch args[0] {
	case "invite":
		if len(args) < 2 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		if !client.Area().SpectateMode() {
			client.SendServerMessage("Spectate mode is not enabled.")
			return
		}
		toInvite := getUidList(strings.Split(args[1], ","))
		var count int
		var report string
		for _, c := range toInvite {
			if c.Area() != client.Area() {
				continue
			}
			if client.Area().AddSpectateInvited(c.Uid()) {
				c.SendServerMessage("You were invited to speak in IC during spectate mode.")
				count++
				report += fmt.Sprintf("%v, ", c.Uid())
			}
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Spectate-invited %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Spectate-invited %v to speak in IC.", report), false)
	case "uninvite":
		if len(args) < 2 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		if !client.Area().SpectateMode() {
			client.SendServerMessage("Spectate mode is not enabled.")
			return
		}
		toUninvite := getUidList(strings.Split(args[1], ","))
		var count int
		var report string
		for _, c := range toUninvite {
			if c == client || client.Area().HasCM(c.Uid()) {
				continue
			}
			if client.Area().RemoveSpectateInvited(c.Uid()) {
				c.SendServerMessage("You are no longer invited to speak in IC during spectate mode.")
				count++
				report += fmt.Sprintf("%v, ", c.Uid())
			}
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Spectate-uninvited %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Spectate-uninvited %v from speaking in IC.", report), false)
	default:
		client.SendServerMessage("Unknown subcommand. " + usage)
	}
}

// Handles /unmute

// cmdAreaDesc prints or updates the area's entry description.
// Any CM or moderator with MODIFY_AREA permission can use this command.
// Usage: /areadesc [-c] [description]
func cmdAreaDesc(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	clear := flags.Bool("c", false, "")
	flags.Parse(args)

	if len(args) == 0 {
		desc := client.Area().Description()
		if desc == "" {
			client.SendServerMessage("This area does not have a description set.")
		} else {
			client.SendServerMessage("Area description: " + desc)
		}
		return
	}

	if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to change the area description.")
		return
	}

	if *clear {
		client.Area().SetDescription("")
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v cleared the area description.", client.OOCName()))
		addToBuffer(client, "CMD", "Cleared area description.", false)
		return
	}

	if len(flags.Args()) != 0 {
		newDesc := strings.Join(flags.Args(), " ")
		client.Area().SetDescription(newDesc)
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v updated the area description.", client.OOCName()))
		addToBuffer(client, "CMD", fmt.Sprintf("Set area description: %v", newDesc), false)
	}
}

