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
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/xhit/go-str2duration/v2"
)

func cmdPunishment(client *Client, args []string, usage string, pType PunishmentType) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	// Parse duration
	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}

	// Cap at 24 hours
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		duration = maxDuration
		client.SendServerMessage(fmt.Sprintf("Duration capped at 24 hours."))
	}

	toPunish := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var report string

	msg := fmt.Sprintf("You have been punished with '%v' effect", pType.String())
	if duration > 0 {
		msg += fmt.Sprintf(" for %v", duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}

	for _, c := range toPunish {
		c.AddPunishment(pType, duration, *reason)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		if err := db.UpsertTextPunishment(c.Ipid(), int(pType), expires, *reason); err != nil {
			logger.LogErrorf("Failed to persist text punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Applied '%v' punishment to %v clients.", pType.String(), count))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied '%v' punishment to %v.", pType.String(), report), false)
}

// Handlers for all punishment commands
func cmdBackward(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBackward)
}

func cmdStutterstep(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentStutterstep)
}

func cmdElongate(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentElongate)
}

func cmdUppercase(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUppercase)
}

func cmdLowercase(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLowercase)
}

func cmdRobotic(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRobotic)
}

func cmdAlternating(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentAlternating)
}

func cmdFancy(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentFancy)
}

func cmdUwu(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUwu)
}

func cmdPirate(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPirate)
}

func cmdShakespearean(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentShakespearean)
}

func cmdCaveman(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentCaveman)
}

func cmdEmoji(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentEmoji)
}

func cmdInvisible(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentInvisible)
}

func cmdSlowpoke(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSlowpoke)
}

func cmdFastspammer(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentFastspammer)
}

func cmdPause(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPause)
}

func cmdLag(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLag)
}

func cmdSubtitles(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSubtitles)
}

func cmdRoulette(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRoulette)
}

func cmdSpotlight(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSpotlight)
}

func cmdCensor(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentCensor)
}

func cmdConfused(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentConfused)
}

func cmdParanoid(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentParanoid)
}

func cmdDrunk(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDrunk)
}

func cmdHiccup(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHiccup)
}

func cmdWhistle(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentWhistle)
}

func cmdMumble(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMumble)
}

func cmdSpaghetti(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSpaghetti)
}

func cmdTorment(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTorment)
}

func cmdRng(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRng)
}

func cmdEssay(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentEssay)
}

func cmdHaiku(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHaiku)
}

func cmdAutospell(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentAutospell)
}

func cmdMonkey(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMonkey)
}

func cmdSnake(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSnake)
}

func cmdDog(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDog)
}

func cmdCat(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentCat)
}

func cmdBird(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBird)
}

func cmdCow(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentCow)
}

func cmdFrog(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentFrog)
}

func cmdDuck(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDuck)
}

func cmdHorse(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHorse)
}

func cmdLion(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLion)
}

func cmdZoo(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentZoo)
}

func cmdBunny(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBunny)
}

func cmdTsundere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTsundere)
}

func cmdYandere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentYandere)
}

func cmdKuudere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentKuudere)
}

func cmdDandere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDandere)
}

func cmdDeredere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDeredere)
}

func cmdHimedere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHimedere)
}

func cmdKamidere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentKamidere)
}

func cmdUndere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUndere)
}

func cmdBakadere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBakadere)
}

func cmdMayadere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMayadere)
}

func cmdEmoticon(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentEmoticon)
}

// cmdUnpunish removes all or specific punishments from users
func cmdUnpunish(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	punishmentType := flags.String("t", "", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	toUnpunish := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var report string

	for _, c := range toUnpunish {
		if *punishmentType == "" {
			// Remove all punishments (text, mute, and jail) from memory and DB.
			c.RemoveAllPunishments()
			c.SetMuted(Unmuted)
			c.SetUnmuteTime(time.Time{})
			c.SetJailedUntil(time.Time{})
			if err := db.DeleteAllPunishments(c.Ipid()); err != nil {
				logger.LogErrorf("Failed to remove persistent punishments for %v: %v", c.Ipid(), err)
			}
			c.SendServerMessage("All punishments have been removed.")
		} else {
			// Remove specific punishment type
			pType := parsePunishmentType(*punishmentType)
			if pType == PunishmentNone {
				client.SendServerMessage(fmt.Sprintf("Unknown punishment type: %v", *punishmentType))
				continue
			}
			if !c.HasPunishment(pType) {
				continue
			}
			c.RemovePunishment(pType)
			if err := db.DeleteTextPunishment(c.Ipid(), int(pType)); err != nil {
				logger.LogErrorf("Failed to remove persistent punishment for %v: %v", c.Ipid(), err)
			}
			c.SendServerMessage(fmt.Sprintf("Punishment '%v' has been removed.", pType.String()))
		}
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed punishments from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed punishments from %v.", report), false)
}

// parsePunishmentType converts a string to PunishmentType
func parsePunishmentType(s string) PunishmentType {
	switch strings.ToLower(s) {
	case "whisper":
		return PunishmentWhisper
	case "backward":
		return PunishmentBackward
	case "stutterstep":
		return PunishmentStutterstep
	case "elongate":
		return PunishmentElongate
	case "uppercase":
		return PunishmentUppercase
	case "lowercase":
		return PunishmentLowercase
	case "robotic":
		return PunishmentRobotic
	case "alternating":
		return PunishmentAlternating
	case "fancy":
		return PunishmentFancy
	case "uwu":
		return PunishmentUwu
	case "pirate":
		return PunishmentPirate
	case "shakespearean":
		return PunishmentShakespearean
	case "caveman":
		return PunishmentCaveman
	case "emoji":
		return PunishmentEmoji
	case "invisible":
		return PunishmentInvisible
	case "slowpoke":
		return PunishmentSlowpoke
	case "fastspammer":
		return PunishmentFastspammer
	case "pause":
		return PunishmentPause
	case "lag":
		return PunishmentLag
	case "subtitles":
		return PunishmentSubtitles
	case "roulette":
		return PunishmentRoulette
	case "spotlight":
		return PunishmentSpotlight
	case "censor":
		return PunishmentCensor
	case "confused":
		return PunishmentConfused
	case "paranoid":
		return PunishmentParanoid
	case "drunk":
		return PunishmentDrunk
	case "hiccup":
		return PunishmentHiccup
	case "whistle":
		return PunishmentWhistle
	case "mumble":
		return PunishmentMumble
	case "spaghetti":
		return PunishmentSpaghetti
	case "torment":
		return PunishmentTorment
	case "rng":
		return PunishmentRng
	case "essay":
		return PunishmentEssay
	case "haiku":
		return PunishmentHaiku
	case "autospell":
		return PunishmentAutospell
	case "monkey":
		return PunishmentMonkey
	case "snake":
		return PunishmentSnake
	case "dog":
		return PunishmentDog
	case "cat":
		return PunishmentCat
	case "bird":
		return PunishmentBird
	case "cow":
		return PunishmentCow
	case "frog":
		return PunishmentFrog
	case "duck":
		return PunishmentDuck
	case "horse":
		return PunishmentHorse
	case "lion":
		return PunishmentLion
	case "zoo":
		return PunishmentZoo
	case "bunny":
		return PunishmentBunny
	case "tsundere":
		return PunishmentTsundere
	case "yandere":
		return PunishmentYandere
	case "kuudere":
		return PunishmentKuudere
	case "dandere":
		return PunishmentDandere
	case "deredere":
		return PunishmentDeredere
	case "himedere":
		return PunishmentHimedere
	case "kamidere":
		return PunishmentKamidere
	case "undere":
		return PunishmentUndere
	case "bakadere":
		return PunishmentBakadere
	case "mayadere":
		return PunishmentMayadere
	default:
		return PunishmentNone
	}
}

// cmdStack applies multiple punishment effects to user(s) simultaneously
func cmdStack(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	if len(flags.Args()) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	// Parse duration
	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}

	// Cap at 24 hours
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		duration = maxDuration
		client.SendServerMessage(fmt.Sprintf("Duration capped at 24 hours."))
	}

	// Parse punishment types (all args except the last one which is UIDs)
	flagArgs := flags.Args()
	if len(flagArgs) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	// Last argument is the UID list
	uidStr := flagArgs[len(flagArgs)-1]
	punishmentNames := flagArgs[:len(flagArgs)-1]

	// Validate and parse all punishment types
	var punishmentTypes []PunishmentType
	for _, name := range punishmentNames {
		pType := parsePunishmentType(name)
		if pType == PunishmentNone {
			client.SendServerMessage(fmt.Sprintf("Unknown punishment type: %v", name))
			return
		}
		punishmentTypes = append(punishmentTypes, pType)
	}

	// Apply punishments to users
	toPunish := getUidList(strings.Split(uidStr, ","))
	var count int
	var report string

	msg := fmt.Sprintf("You have been punished with stacked effects: ")
	punishmentNamesList := []string{}
	for _, pType := range punishmentTypes {
		punishmentNamesList = append(punishmentNamesList, "'"+pType.String()+"'")
	}
	msg += strings.Join(punishmentNamesList, ", ")

	if duration > 0 {
		msg += fmt.Sprintf(" for %v", duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}

	for _, c := range toPunish {
		// Apply each punishment
		for _, pType := range punishmentTypes {
			c.AddPunishment(pType, duration, *reason)
		}
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	punishmentList := strings.Join(punishmentNamesList, ", ")
	client.SendServerMessage(fmt.Sprintf("Applied stacked punishments [%v] to %v clients.", punishmentList, count))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied stacked punishments [%v] to %v.", punishmentList, report), false)
}

// cmdTournament manages punishment tournament mode
