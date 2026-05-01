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

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/xhit/go-str2duration/v2"
)

// issuerTierFor returns the IssuerTier corresponding to the issuing client's
// permission set. Admin > Shadow > Mod. Used so /unpunish can block a regular
// moderator from self-removing a punishment that staff stacked on them.
func issuerTierFor(client *Client) IssuerTier {
	perms := client.Perms()
	switch {
	case permissions.IsAdmin(perms):
		return IssuerAdmin
	case permissions.IsShadow(perms):
		return IssuerShadow
	case permissions.IsModerator(perms):
		return IssuerMod
	default:
		return IssuerSystem
	}
}

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

	tier := issuerTierFor(client)
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
		c.AddPunishmentBy(pType, duration, *reason, tier)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		if err := db.UpsertTextPunishmentBy(c.Ipid(), int(pType), expires, *reason, int(tier)); err != nil {
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

func cmdLag(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLag)
}

func cmdSubtitles(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSubtitles)
}

func cmdRoulette(client *Client, args []string, usage string) {
	if len(args) > 0 && args[0] == "join" {
		rrJoin(client)
		return
	}
	if !permissions.HasPermission(client.Perms(), permissions.PermissionField["MUTE"]) {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
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

// cmdUnpunish removes all or specific punishments from users.
//
// Self-removal protection: a moderator who is themselves carrying a punishment
// applied by an admin or shadow mod cannot lift that punishment from themselves.
// Admins and shadow mods are exempt from the protection (they can self-unpunish
// because they outrank the issuer or are the issuer's peer).
func cmdUnpunish(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	punishmentType := flags.String("t", "", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	// callerTier determines whether self-removal protection applies. Admins
	// can always self-unpunish; shadow mods can lift other shadow/admin
	// punishments off themselves; regular mods are blocked from removing
	// shadow/admin-issued punishments off their own UID.
	callerTier := issuerTierFor(client)
	callerUID := client.Uid()

	// /unpunish all — clear all punishments from every client in the moderator's area.
	if strings.EqualFold(flags.Arg(0), "all") {
		myArea := client.Area()
		var count, skipped int
		clients.ForEach(func(c *Client) {
			if c.Area() != myArea {
				return
			}
			// Self-removal protection: skip the caller if they're a regular mod
			// and carry a shadow/admin-issued punishment.
			if c.Uid() == callerUID && callerTier < IssuerShadow && c.HasProtectedPunishment() {
				skipped++
				return
			}
			c.RemoveAllPunishments()
			c.SetMuted(Unmuted)
			c.SetUnmuteTime(time.Time{})
			c.SetJailedUntil(time.Time{})
			if err := db.DeleteAllPunishments(c.Ipid()); err != nil {
				logger.LogErrorf("Failed to remove persistent punishments for %v: %v", c.Ipid(), err)
			}
			c.SendServerMessage("All punishments have been removed.")
			count++
		})
		summary := fmt.Sprintf("Removed all punishments from %v client(s) in this area.", count)
		if skipped > 0 {
			summary += " Your own punishments were issued by an admin or shadow mod and cannot be self-removed."
		}
		client.SendServerMessage(summary)
		addToBuffer(client, "CMD", fmt.Sprintf("Removed all punishments from entire area (%v client(s)).", count), false)
		return
	}

	toUnpunish := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var report string

	for _, c := range toUnpunish {
		// Self-removal protection runs once per target. Regular mods cannot
		// strip a shadow/admin-issued punishment off themselves; admins and
		// shadow mods bypass the gate.
		isSelf := c.Uid() == callerUID
		if isSelf && callerTier < IssuerShadow {
			if *punishmentType == "" {
				if c.HasProtectedPunishment() {
					client.SendServerMessage("You cannot remove all of your own punishments — at least one was issued by an admin or shadow mod. Ask staff to lift it.")
					continue
				}
			} else {
				pType := parsePunishmentType(*punishmentType)
				if pType != PunishmentNone && c.HasPunishment(pType) && c.PunishmentIssuerTier(pType) >= IssuerShadow {
					client.SendServerMessage(fmt.Sprintf("Punishment '%v' was issued by an admin or shadow mod and cannot be self-removed.", pType.String()))
					continue
				}
			}
		}

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
	case "lovebomb":
		return PunishmentLovebomb
	case "degrade":
		return PunishmentDegrade
	case "tourettes":
		return PunishmentTourettes
	case "slang":
		return PunishmentSlang
	case "thesaurusoverload":
		return PunishmentThesaurusOverload
	case "valleygirl":
		return PunishmentValleyGirl
	case "babytalk":
		return PunishmentBabytalk
	case "thirdperson":
		return PunishmentThirdPerson
	case "unreliablenarrator":
		return PunishmentUnreliableNarrator
	case "uncannyvalley":
		return PunishmentUncannyValley
	case "51":
		return Punishment51
	case "philosopher":
		return PunishmentPhilosopher
	case "poet":
		return PunishmentPoet
	case "upsidedown":
		return PunishmentUpsidedown
	case "sarcasm":
		return PunishmentSarcasm
	case "academic":
		return PunishmentAcademic
	case "recipe":
		return PunishmentRecipe
	case "quote":
		return PunishmentQuote
	case "translator":
		return PunishmentTranslator
	case "timewarp":
		return PunishmentTimewarp
	case "morse":
		return PunishmentMorse
	case "rickroll":
		return PunishmentRickroll
	case "vowelhell":
		return PunishmentVowelhell
	case "chef":
		return PunishmentChef
	case "karen":
		return PunishmentKaren
	case "passiveaggressive":
		return PunishmentPassiveAggressive
	case "nervous":
		return PunishmentNervous
	case "dreamsequence":
		return PunishmentDreamSequence
	case "icwarp":
		return PunishmentICWarp
	case "pickup":
		return PunishmentPickup
	case "brainrot":
		return PunishmentBrainrot
	case "gordonramsay":
		return PunishmentGordonRamsay
	case "cherri":
		return PunishmentCherri
	case "clown":
		return PunishmentClown
	case "jester":
		return PunishmentJester
	case "joker":
		return PunishmentJoker
	case "mime":
		return PunishmentMime
	case "biblebot":
		return PunishmentBiblebot
	case "smugdere":
		return PunishmentSmugdere
	case "deretsun":
		return PunishmentDeretsun
	case "bokodere":
		return PunishmentBokodere
	case "thugdere":
		return PunishmentThugdere
	case "teasedere":
		return PunishmentTeasedere
	case "dorodere":
		return PunishmentDorodere
	case "hinedere":
		return PunishmentHinedere
	case "hajidere":
		return PunishmentHajidere
	case "rindere":
		return PunishmentRindere
	case "utsudere":
		return PunishmentUtsudere
	case "darudere":
		return PunishmentDarudere
	case "butsudere":
		return PunishmentButsudere
	case "sdere":
		return PunishmentSDere
	case "mdere":
		return PunishmentMDere
	case "tsuyodere":
		return PunishmentTsuyodere
	case "omnidere":
		return PunishmentOmnidere
	case "megamaso":
		return PunishmentMegamaso
	case "sfxcurse":
		return PunishmentSfxCurse
	case "shrink":
		return PunishmentShrink
	case "grow":
		return PunishmentGrow
	case "wide":
		return PunishmentWide
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

	tier := issuerTierFor(client)
	for _, c := range toPunish {
		// Apply each punishment
		for _, pType := range punishmentTypes {
			c.AddPunishmentBy(pType, duration, *reason, tier)
			var expires int64
			if duration > 0 {
				expires = time.Now().UTC().Add(duration).Unix()
			}
			if err := db.UpsertTextPunishmentBy(c.Ipid(), int(pType), expires, *reason, int(tier)); err != nil {
				logger.LogErrorf("Failed to persist stacked punishment for %v: %v", c.Ipid(), err)
			}
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

// cmdLovebomb applies the lovebomb punishment.
//
// Subcommands (evaluated before any UID arguments):
//
//	/lovebomb global           – apply to all non-moderators in the caster's area
//	/lovebomb global off       – remove lovebomb from everyone in the caster's area
//
// UID-based forms:
//
//	/lovebomb <uid>            – apply to a specific uid (random area target per message)
//	/lovebomb <uid1> <uid2>    – uid1 will love-bomb uid2 specifically
//
// No arguments: displays usage information.
func cmdLovebomb(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	// Parse duration
	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
		client.SendServerMessage("Duration capped at 24 hours.")
	}

	// Helper: apply lovebomb to one client and persist it.
	apply := func(c *Client, targetUID int) {
		c.AddLovebombPunishment(targetUID, duration, *reason)
		msg := "You have been love bombed!"
		if duration > 0 {
			msg += fmt.Sprintf(" (for %v)", duration)
		}
		c.SendServerMessage(msg)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		if err := db.UpsertTextPunishment(c.Ipid(), int(PunishmentLovebomb), expires, *reason); err != nil {
			logger.LogErrorf("Failed to persist lovebomb for %v: %v", c.Ipid(), err)
		}
	}

	fargs := flags.Args()

	// ── Global subcommands ────────────────────────────────────────────────────
	if len(fargs) >= 1 && fargs[0] == "global" {
		if len(fargs) >= 2 && fargs[1] == "off" {
			// /lovebomb global off — remove from everyone in area
			var count int
			var report string
			targetArea := client.Area()
			clients.ForEach(func(c *Client) {
				if c.Area() != targetArea || !c.HasPunishment(PunishmentLovebomb) {
					return
				}
				c.RemovePunishment(PunishmentLovebomb)
				if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentLovebomb)); err != nil {
					logger.LogErrorf("Failed to remove lovebomb for %v: %v", c.Ipid(), err)
				}
				c.SendServerMessage("Love bomb punishment has been removed.")
				count++
				report += fmt.Sprintf("%v, ", c.Uid())
			})
			report = strings.TrimSuffix(report, ", ")
			client.SendServerMessage(fmt.Sprintf("Removed lovebomb from %v clients in area.", count))
			addToBuffer(client, "CMD", fmt.Sprintf("Removed area lovebomb from %v.", report), false)
			return
		}

		// /lovebomb global — apply to all non-moderators in area (excluding issuer)
		var count int
		var report string
		issuerUID := client.Uid()
		targetArea := client.Area()
		clients.ForEach(func(c *Client) {
			if c.Area() != targetArea || c.Uid() == issuerUID {
				return
			}
			if permissions.IsModerator(c.Perms()) {
				return
			}
			apply(c, -1)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		})
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Applied lovebomb to %v non-moderator clients in area.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Applied area lovebomb to %v.", report), false)
		return
	}

	// ── UID-based forms ───────────────────────────────────────────────────────
	var count int
	var report string

	switch len(fargs) {
	case 0:
		// No arguments — show usage description.
		client.SendServerMessage(usage)
		return
	case 1:
		// Specific uid(s), random area target
		for _, c := range getUidList(strings.Split(fargs[0], ",")) {
			apply(c, -1)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	case 2:
		// uid1 targets uid2
		targetUID, convErr := strconv.Atoi(fargs[1])
		if convErr != nil {
			client.SendServerMessage("Invalid target UID.")
			return
		}
		if _, lookupErr := getClientByUid(targetUID); lookupErr != nil {
			client.SendServerMessage(fmt.Sprintf("Target UID %v not found.", targetUID))
			return
		}
		for _, c := range getUidList(strings.Split(fargs[0], ",")) {
			apply(c, targetUID)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	default:
		client.SendServerMessage("Too many arguments:\n" + usage)
		return
	}

	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Applied lovebomb punishment to %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied lovebomb punishment to %v.", report), false)
}

// cmdUnlovebomb removes the lovebomb punishment from user(s).
func cmdUnlovebomb(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentLovebomb) {
			continue
		}
		c.RemovePunishment(PunishmentLovebomb)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentLovebomb)); err != nil {
			logger.LogErrorf("Failed to remove lovebomb for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Love bomb punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed lovebomb punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed lovebomb from %v.", report), false)
}

// cmdDegrade applies the degrade punishment.
func cmdDegrade(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDegrade)
}

// cmdTourettes applies the tourettes punishment.
func cmdTourettes(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTourettes)
}

// cmdSlang applies the slang punishment.
func cmdSlang(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSlang)
}

// cmdThesaurusOverload applies the thesaurusoverload punishment.
func cmdThesaurusOverload(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentThesaurusOverload)
}

// cmdValleyGirl applies the valleygirl punishment.
func cmdValleyGirl(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentValleyGirl)
}

// cmdBabytalk applies the babytalk punishment.
func cmdBabytalk(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBabytalk)
}

// cmdThirdPerson applies the thirdperson punishment.
func cmdThirdPerson(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentThirdPerson)
}

// cmdUnreliableNarrator applies the unreliablenarrator punishment.
func cmdUnreliableNarrator(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUnreliableNarrator)
}

// cmdUncannyValley applies the uncannyvalley punishment.
func cmdUncannyValley(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUncannyValley)
}

// cmdUndegrade removes the degrade punishment from user(s).
func cmdUndegrade(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentDegrade) {
			continue
		}
		c.RemovePunishment(PunishmentDegrade)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentDegrade)); err != nil {
			logger.LogErrorf("Failed to remove degrade for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Degrade punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed degrade punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed degrade from %v.", report), false)
}

// cmdUnslang removes the slang punishment from user(s).
func cmdUnslang(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentSlang) {
			continue
		}
		c.RemovePunishment(PunishmentSlang)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentSlang)); err != nil {
			logger.LogErrorf("Failed to remove slang for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Slang punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed slang punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed slang from %v.", report), false)
}

// cmdUnthesaurusoverload removes the thesaurusoverload punishment from user(s).
func cmdUnthesaurusoverload(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentThesaurusOverload) {
			continue
		}
		c.RemovePunishment(PunishmentThesaurusOverload)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentThesaurusOverload)); err != nil {
			logger.LogErrorf("Failed to remove thesaurusoverload for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Thesaurus overload punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed thesaurusoverload punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed thesaurusoverload from %v.", report), false)
}

// cmdUnvalleygirl removes the valleygirl punishment from user(s).
func cmdUnvalleygirl(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentValleyGirl) {
			continue
		}
		c.RemovePunishment(PunishmentValleyGirl)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentValleyGirl)); err != nil {
			logger.LogErrorf("Failed to remove valleygirl for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Valley girl punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed valleygirl punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed valleygirl from %v.", report), false)
}

// cmdUnbabytalk removes the babytalk punishment from user(s).
func cmdUnbabytalk(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentBabytalk) {
			continue
		}
		c.RemovePunishment(PunishmentBabytalk)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentBabytalk)); err != nil {
			logger.LogErrorf("Failed to remove babytalk for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Baby talk punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed babytalk punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed babytalk from %v.", report), false)
}

// cmdUnthirdperson removes the thirdperson punishment from user(s).
func cmdUnthirdperson(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentThirdPerson) {
			continue
		}
		c.RemovePunishment(PunishmentThirdPerson)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentThirdPerson)); err != nil {
			logger.LogErrorf("Failed to remove thirdperson for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Third person punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed thirdperson punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed thirdperson from %v.", report), false)
}

// cmdUnunreliablenarrator removes the unreliablenarrator punishment from user(s).
func cmdUnunreliablenarrator(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentUnreliableNarrator) {
			continue
		}
		c.RemovePunishment(PunishmentUnreliableNarrator)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentUnreliableNarrator)); err != nil {
			logger.LogErrorf("Failed to remove unreliablenarrator for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Unreliable narrator punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed unreliablenarrator punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed unreliablenarrator from %v.", report), false)
}

// cmdUnuncannyvalley removes the uncannyvalley punishment from user(s).
func cmdUnuncannyvalley(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentUncannyValley) {
			continue
		}
		c.RemovePunishment(PunishmentUncannyValley)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentUncannyValley)); err != nil {
			logger.LogErrorf("Failed to remove uncannyvalley for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Uncanny valley punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed uncannyvalley punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed uncannyvalley from %v.", report), false)
}

func cmd51(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, Punishment51)
}

// cmdUn51 removes the 51 punishment from user(s).
func cmdUn51(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(Punishment51) {
			continue
		}
		c.RemovePunishment(Punishment51)
		if err := db.DeleteTextPunishment(c.Ipid(), int(Punishment51)); err != nil {
			logger.LogErrorf("Failed to remove 51 punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("51 punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed 51 punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed 51 from %v.", report), false)
}

func cmdPhilosopher(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPhilosopher)
}

// cmdUnphilosopher removes the philosopher punishment from user(s).
func cmdUnphilosopher(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentPhilosopher) {
			continue
		}
		c.RemovePunishment(PunishmentPhilosopher)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentPhilosopher)); err != nil {
			logger.LogErrorf("Failed to remove philosopher punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Philosopher punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed philosopher punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed philosopher from %v.", report), false)
}

func cmdPoet(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPoet)
}

// cmdUnpoet removes the poet punishment from user(s).
func cmdUnpoet(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentPoet) {
			continue
		}
		c.RemovePunishment(PunishmentPoet)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentPoet)); err != nil {
			logger.LogErrorf("Failed to remove poet punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Poet punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed poet punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed poet from %v.", report), false)
}

func cmdUpsidedown(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUpsidedown)
}

// cmdUnupsidedown removes the upsidedown punishment from user(s).
func cmdUnupsidedown(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentUpsidedown) {
			continue
		}
		c.RemovePunishment(PunishmentUpsidedown)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentUpsidedown)); err != nil {
			logger.LogErrorf("Failed to remove upsidedown punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Upsidedown punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed upsidedown punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed upsidedown from %v.", report), false)
}

func cmdSarcasm(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSarcasm)
}

// cmdUnsarcasm removes the sarcasm punishment from user(s).
func cmdUnsarcasm(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentSarcasm) {
			continue
		}
		c.RemovePunishment(PunishmentSarcasm)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentSarcasm)); err != nil {
			logger.LogErrorf("Failed to remove sarcasm punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Sarcasm punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed sarcasm punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed sarcasm from %v.", report), false)
}

func cmdAcademic(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentAcademic)
}

// cmdUnacademic removes the academic punishment from user(s).
func cmdUnacademic(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentAcademic) {
			continue
		}
		c.RemovePunishment(PunishmentAcademic)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentAcademic)); err != nil {
			logger.LogErrorf("Failed to remove academic punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Academic punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed academic punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed academic from %v.", report), false)
}

func cmdRecipe(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRecipe)
}

// cmdUnrecipe removes the recipe punishment from user(s).
func cmdUnrecipe(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentRecipe) {
			continue
		}
		c.RemovePunishment(PunishmentRecipe)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentRecipe)); err != nil {
			logger.LogErrorf("Failed to remove recipe punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Recipe punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed recipe punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed recipe from %v.", report), false)
}

func cmdQuote(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentQuote)
}

// cmdUnquote removes the quote punishment from user(s).
func cmdUnquote(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentQuote) {
			continue
		}
		c.RemovePunishment(PunishmentQuote)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentQuote)); err != nil {
			logger.LogErrorf("Failed to remove quote punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Quote punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed quote punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed quote from %v.", report), false)
}

// cmdTranslator applies the translator punishment.  Usage:
//
//	/translator curse [-d duration] [-r reason] <uid1>,<uid2>,... <language>
//
// The literal subcommand word "curse" is required to match the documented
// surface (matches the /translator curse ID language ergonomics).  Language
// may be an English name ("french"), an ISO code ("fr"), or the special
// keyword "random" for per-word random translation.
func cmdTranslator(client *Client, args []string, usage string) {
	if !config.EnableTranslator {
		client.SendServerMessage(
			"The translator punishment is disabled on this server.\n" +
				"To enable: in config.toml under [Server], set enable_translator_punishment = true\n" +
				"and translator_api_key = \"<your DeepL API key>\", then restart the server.")
		return
	}
	if config.TranslatorAPIKey == "" {
		client.SendServerMessage(
			"The translator punishment is enabled but no API key is configured.\n" +
				"Set translator_api_key in config.toml under [Server] and restart the server.")
		return
	}

	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	positional := flags.Args()
	if len(positional) < 3 || !strings.EqualFold(positional[0], "curse") {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	targetArg := positional[1]
	language := positional[2]

	code := resolveLanguage(language)
	if !strings.EqualFold(language, "random") && code == "" {
		client.SendServerMessage(fmt.Sprintf(
			"Unknown language: %v.\n"+
				"  • English names — french, spanish, japanese, german, russian, arabic, ...\n"+
				"  • ISO codes     — fr, es, ja, de, ru, ar, zh-CN, ...\n"+
				"  • Keyword       — random  (each word translated into a different language)\n"+
				"Example: /translator curse 7 random\n"+
				"         /translator curse global random", language))
		return
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		duration = maxDuration
		client.SendServerMessage("Duration capped at 24 hours.")
	}

	// Resolve targets: either an explicit UID list or "global" — every client
	// in the invoking mod's current area except the mod themselves.  "global"
	// is scoped to the mod's AREA, not the whole server, matching other
	// area-scoped punishments and keeping blast radius contained.
	var toPunish []*Client
	isGlobal := strings.EqualFold(targetArg, "global")
	if isGlobal {
		myArea := client.Area()
		myUid := client.Uid()
		clients.ForEach(func(c *Client) {
			if c.Area() == myArea && c.Uid() != -1 && c.Uid() != myUid {
				toPunish = append(toPunish, c)
			}
		})
		if len(toPunish) == 0 {
			client.SendServerMessage("No other players are in this area to curse.")
			return
		}
	} else {
		toPunish = getUidList(strings.Split(targetArg, ","))
	}

	customData := strings.ToLower(language)
	var count int
	var report string

	msg := fmt.Sprintf("You have been cursed by the translator — your messages will be translated to '%v'", language)
	if duration > 0 {
		msg += fmt.Sprintf(" for %v", duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}

	for _, c := range toPunish {
		c.AddPunishmentWithData(PunishmentTranslator, duration, *reason, customData)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		// Pack customData into the reason column with a 0x1F separator so the
		// target language survives a server restart via restorePunishments.
		stored := customData + "\x1f" + *reason
		if err := db.UpsertTextPunishment(c.Ipid(), int(PunishmentTranslator), expires, stored); err != nil {
			logger.LogErrorf("Failed to persist translator punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	scope := "clients"
	if isGlobal {
		scope = fmt.Sprintf("clients in area %q", client.Area().Name())
	}
	client.SendServerMessage(fmt.Sprintf("Applied translator (%v) punishment to %v %v.", language, count, scope))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied translator (%v) punishment to %v.", language, report), false)
}

// cmdUntranslator removes the translator punishment from user(s).
//
//	/untranslator curse <uid1>,<uid2>,...
//	/untranslator curse global
//
// The "curse" subcommand word is required to mirror /translator curse.  The
// "global" target sweeps every client on the server currently affected by the
// translator punishment — useful for one-shot cleanup after mass-cursing.
func cmdUntranslator(client *Client, args []string, usage string) {
	if len(args) < 2 || !strings.EqualFold(args[0], "curse") {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	var toUnpunish []*Client
	isGlobal := strings.EqualFold(args[1], "global")
	if isGlobal {
		clients.ForEach(func(c *Client) {
			if c.HasPunishment(PunishmentTranslator) {
				toUnpunish = append(toUnpunish, c)
			}
		})
	} else {
		toUnpunish = getUidList(strings.Split(args[1], ","))
	}
	var count int
	var report string
	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentTranslator) {
			continue
		}
		c.RemovePunishment(PunishmentTranslator)
		if err := db.DeleteTextPunishment(c.Ipid(), int(PunishmentTranslator)); err != nil {
			logger.LogErrorf("Failed to remove translator punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage("Translator punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	if isGlobal {
		client.SendServerMessage(fmt.Sprintf("Removed translator punishment from %v clients (global sweep).", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Removed translator (global) from %v.", report), false)
		return
	}
	client.SendServerMessage(fmt.Sprintf("Removed translator punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed translator from %v.", report), false)
}

// randompunishPool is the set of punishments available to /randompunishall.
var randompunishPool = rrPunishmentPool

// cmdRandomPunishAll applies a random punishment to every client currently in
// the caller's area. Each client receives an independently chosen punishment.
// Requires MUTE permission. Blocked if the area has disabled random punishment
// via /togglerandompunish.
func cmdRandomPunishAll(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	durationStr := flags.String("d", "10m", "")
	reason := flags.String("r", "", "")
	flags.Parse(args)

	if !client.Area().RandomPunishEnabled() {
		client.SendServerMessage("Random punishment is disabled in this area.")
		return
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		duration = maxDuration
		client.SendServerMessage("Duration capped at 24 hours.")
	}

	// Collect clients in the same area (CharID == -1 means no character selected / spectator).
	var targets []*Client
	clients.ForEach(func(c *Client) {
		if c.Area() == client.Area() && c.CharID() != -1 {
			targets = append(targets, c)
		}
	})

	if len(targets) == 0 {
		client.SendServerMessage("No players in this area to punish.")
		return
	}

	msg := "You have been hit by /randompunishall"
	if duration > 0 {
		msg += fmt.Sprintf(" for %v", duration)
	}
	if *reason != "" {
		msg += " – reason: " + *reason
	}

	tier := issuerTierFor(client)
	var report string
	for _, c := range targets {
		pType := randompunishPool[rand.Intn(len(randompunishPool))]
		c.AddPunishmentBy(pType, duration, *reason, tier)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		if err := db.UpsertTextPunishmentBy(c.Ipid(), int(pType), expires, *reason, int(tier)); err != nil {
			logger.LogErrorf("Failed to persist randompunishall punishment for %v: %v", c.Ipid(), err)
		}
		c.SendServerMessage(fmt.Sprintf("%v (%v)", msg, pType.String()))
		report += fmt.Sprintf("%v(%v), ", c.Uid(), pType.String())
	}

	report = strings.TrimSuffix(report, ", ")
	sendAreaServerMessage(client.Area(), fmt.Sprintf("🎲 %v has unleashed random chaos on the area!", client.OOCName()))
	client.SendServerMessage(fmt.Sprintf("Applied random punishments to %v clients.", len(targets)))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied randompunishall: %v.", report), false)
}

// cmdToggleRandomPunish enables or disables /randompunishall for the caller's area.
// Requires CM or MUTE permission.
func cmdToggleRandomPunish(client *Client, args []string, usage string) {
	a := client.Area()
	newState := !a.RandomPunishEnabled()
	a.SetRandomPunishEnabled(newState)
	if newState {
		sendAreaServerMessage(a, fmt.Sprintf("%v has enabled random punishment for this area.", client.OOCName()))
	} else {
		sendAreaServerMessage(a, fmt.Sprintf("%v has disabled random punishment for this area.", client.OOCName()))
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Toggled random punishment to %v.", newState), false)
}

// Handlers for the additional novelty punishments.
func cmdTimewarp(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTimewarp)
}

func cmdMorse(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMorse)
}

func cmdRickroll(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRickroll)
}

func cmdPickup(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPickup)
}

func cmdBrainrot(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBrainrot)
}

func cmdVowelhell(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVowelhell)
}

func cmdChef(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentChef)
}

func cmdKaren(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentKaren)
}

func cmdPassiveAggressive(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPassiveAggressive)
}

func cmdNervous(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentNervous)
}

func cmdGordonRamsay(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentGordonRamsay)
}

func cmdDreamSequence(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDreamSequence)
}

// cmdICWarp handles /icwarp <uid> and /icwarp global on|off.
// Per-user: punishes the target so their IC messages are replaced with a
// random past IC message they sent in the current area (last 24 hours).
// Global: applies the same effect to every player in the area except the
// moderator who issued the command.
func cmdICWarp(client *Client, args []string, usage string) {
	// Global mode: /icwarp global on|off
	if strings.ToLower(args[0]) == "global" {
		if len(args) < 2 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		switch strings.ToLower(args[1]) {
		case "on":
			client.Area().SetICWarpGlobal(true, client.Uid())
			writeToArea(client.Area(), "CT", encode("Server"),
				encode("[Global IC Warp is now ON — everyone's messages will replay their own past messages!]"), "1")
			addToBuffer(client, "CMD", "Enabled global IC warp in area.", false)
		case "off":
			client.Area().SetICWarpGlobal(false, -1)
			writeToArea(client.Area(), "CT", encode("Server"),
				encode("[Global IC Warp is now OFF.]"), "1")
			addToBuffer(client, "CMD", "Disabled global IC warp in area.", false)
		default:
			client.SendServerMessage("Invalid argument. Use: /icwarp global on|off")
		}
		return
	}

	// Per-user mode.
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args) //nolint:errcheck

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}
	maxDuration := 24 * time.Hour
	if duration > maxDuration {
		duration = maxDuration
		client.SendServerMessage("Duration capped at 24 hours.")
	}

	toPunish := getUidList(strings.Split(flags.Arg(0), ","))
	targetArea := client.Area()
	var count int
	var report string

	msg := "You have been punished with 'icwarp' effect"
	if duration > 0 {
		msg += fmt.Sprintf(" for %v", duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}

	for _, c := range toPunish {
		c.AddICWarpPunishment(targetArea, duration, *reason)
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Applied 'icwarp' punishment to %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied 'icwarp' punishment to %v.", report), false)
}

// cmdUniCWarp removes the icwarp punishment from user(s).
func cmdUniCWarp(client *Client, args []string, usage string) {
	toUnpunish := getUidList(strings.Split(args[0], ","))
	var count int
	var report string

	for _, c := range toUnpunish {
		if !c.HasPunishment(PunishmentICWarp) {
			continue
		}
		c.RemovePunishment(PunishmentICWarp)
		c.SendServerMessage("Your IC warp punishment has been removed.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Removed icwarp punishment from %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Removed icwarp punishment from %v.", report), false)
}
