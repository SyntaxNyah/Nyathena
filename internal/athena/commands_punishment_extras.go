/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: extra punishment slash-command handlers.

   Plain-wrapper handlers for the new punishment types (cherri, clown,
   jester, joker, mime, biblebot, plus the additional dere archetypes and
   the omnidere combiner). Each just calls cmdPunishment with the matching
   PunishmentType — keeping the upstream commands_punishment.go untouched
   for easier rebases. */

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

	str2duration "github.com/xhit/go-str2duration/v2"
)

// /cherri
func cmdCherri(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentCherri)
}

// /clown
func cmdClown(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentClown)
}

// /jester
func cmdJester(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentJester)
}

// /joker
func cmdJoker(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentJoker)
}

// /mime
func cmdMime(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMime)
}

// /biblebot
func cmdBiblebot(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBiblebot)
}

// /smugdere etc.
func cmdSmugdere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSmugdere)
}
func cmdDeretsun(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDeretsun)
}
func cmdBokodere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentBokodere)
}
func cmdThugdere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentThugdere)
}
func cmdTeasedere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTeasedere)
}
func cmdDorodere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDorodere)
}
func cmdHinedere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHinedere)
}
func cmdHajidere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentHajidere)
}
func cmdRindere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRindere)
}
func cmdUtsudere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentUtsudere)
}
func cmdDarudere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentDarudere)
}
func cmdButsudere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentButsudere)
}
func cmdSDere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSDere)
}
func cmdMDere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMDere)
}
func cmdTsuyodere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTsuyodere)
}
func cmdOmnidere(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentOmnidere)
}

// megamasoStackPool is the pool /megamaso draws each random punishment from.
// Reuses the same broad transform list /maso uses so the chaos stays varied.
var megamasoStackPool = []PunishmentType{
	PunishmentBackward, PunishmentStutterstep, PunishmentElongate, PunishmentUppercase,
	PunishmentLowercase, PunishmentRobotic, PunishmentAlternating, PunishmentFancy,
	PunishmentUwu, PunishmentPirate, PunishmentShakespearean, PunishmentCaveman,
	PunishmentCensor, PunishmentConfused, PunishmentParanoid, PunishmentDrunk,
	PunishmentHiccup, PunishmentWhistle, PunishmentMumble, PunishmentSpaghetti,
	PunishmentRng, PunishmentEssay, PunishmentAutospell, PunishmentSubtitles,
	PunishmentSpotlight, PunishmentTsundere, PunishmentYandere, PunishmentKuudere,
	PunishmentDandere, PunishmentDeredere, PunishmentBakadere, PunishmentSlang,
	PunishmentValleyGirl, PunishmentBabytalk, PunishmentUnreliableNarrator,
	PunishmentSarcasm, PunishmentChef, PunishmentKaren, PunishmentNervous,
	PunishmentClown, PunishmentJester, PunishmentJoker, PunishmentSmugdere,
	PunishmentDeretsun, PunishmentThugdere, PunishmentTeasedere, PunishmentRindere,
	PunishmentDarudere, PunishmentTsuyodere, PunishmentCherri,
}

// Handles /megamaso
//
// Self-applied "max chaos" mode. First call rolls a random punishment and
// applies it for 10 minutes. Each subsequent /megamaso while still under the
// effect ADDs another random punishment to the stack (instead of resetting),
// so the player can pile on as many as they like. Re-rolls only stop being
// available when the stack expires.
func cmdMegamaso(client *Client, _ []string, _ string) {
	// Pick a random punishment that the player isn't already wearing, falling
	// back to "any" if everything is somehow already applied.
	var pick PunishmentType
	for tries := 0; tries < 16; tries++ {
		candidate := megamasoStackPool[rand.Intn(len(megamasoStackPool))]
		if !client.HasPunishment(candidate) {
			pick = candidate
			break
		}
	}
	if pick == PunishmentNone {
		pick = megamasoStackPool[rand.Intn(len(megamasoStackPool))]
	}

	const stackDuration = 10 * time.Minute
	client.AddPunishment(pick, stackDuration, "megamaso stack")

	// Count active megamaso-pool punishments so the player knows how big
	// their personal pile-up is.
	stackSize := 0
	for _, p := range megamasoStackPool {
		if client.HasPunishment(p) {
			stackSize++
		}
	}
	client.SendServerMessage(fmt.Sprintf(
		"💥 /megamaso: stacked '%v' onto your punishment pile. Stack size now %d. Type /megamaso again to keep stacking — each adds a new random effect for 10 minutes.",
		pick.String(), stackSize))
	addToBuffer(client, "CMD", fmt.Sprintf("Megamaso stacked %v.", pick.String()), false)
}

// Handles /sfxcurse <uid> <sfx-url>
//
// Forces the target to emit the specified SFX file on every IC message.
// Implementation note: the SFX URL is stored in the punishment's CustomData
// field. The IC packet path consults HasPunishment(PunishmentSfxCurse) and
// rewrites the SFX/SFXANIM fields when present.
func cmdSfxCurse(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	durationStr := flags.String("d", "1h", "")
	reason := flags.String("r", "sfxcurse", "")
	flags.Parse(args)

	if len(flags.Args()) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	uidArg := flags.Arg(0)
	sfx := flags.Arg(1)
	// Sanity-check the SFX URL — must be http/https or a bare path under /base/sounds/.
	if !strings.HasPrefix(sfx, "http://") && !strings.HasPrefix(sfx, "https://") &&
		!strings.HasPrefix(sfx, "/base/sounds/") {
		client.SendServerMessage("SFX must be an http(s) URL or a path under /base/sounds/.")
		return
	}
	if u, err := url.Parse(sfx); err != nil || (u.Scheme != "" && u.Host == "") {
		client.SendServerMessage("Invalid SFX URL.")
		return
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration.")
		return
	}
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
	}

	toCurse := getUidList(strings.Split(uidArg, ","))
	count := 0
	for _, c := range toCurse {
		c.AddPunishmentWithData(PunishmentSfxCurse, duration, *reason, sfx)
		c.SendServerMessage(fmt.Sprintf("🔊 You are now SFX-cursed: every IC message will play %s", sfx))
		count++
	}
	client.SendServerMessage(fmt.Sprintf("Applied SFX curse to %d client(s).", count))
	addToBuffer(client, "CMD", fmt.Sprintf("SFX curse: %s", sfx), false)
}

// Handles /unsfx <uid>
func cmdUnSfx(client *Client, args []string, _ string) {
	toClear := getUidList(strings.Split(args[0], ","))
	count := 0
	for _, c := range toClear {
		if !c.HasPunishment(PunishmentSfxCurse) {
			continue
		}
		c.RemovePunishment(PunishmentSfxCurse)
		c.SendServerMessage("Your SFX curse has been lifted.")
		count++
	}
	client.SendServerMessage(fmt.Sprintf("Cleared SFX curse from %d client(s).", count))
}

// applyShrinkGrowWide is the shared helper for /shrink, /grow, and /wide.
// It parses the optional duration/reason flags, validates the offset value
// (capped at the AO2 protocol limit of ±100), and stores the offset as the
// punishment's CustomData. The IC packet path applies the offset on send.
func applyShrinkGrowWide(client *Client, args []string, usage string, pType PunishmentType, defaultOffset int) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	durationStr := flags.String("d", "1h", "")
	reason := flags.String("r", pType.String(), "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	uidArg := flags.Arg(0)
	offset := defaultOffset
	if len(flags.Args()) >= 2 {
		v, err := strconv.Atoi(flags.Arg(1))
		if err != nil {
			client.SendServerMessage("Offset must be an integer between -100 and 100.")
			return
		}
		offset = v
	}
	if offset < -100 || offset > 100 {
		client.SendServerMessage("Offset must be between -100 and 100.")
		return
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration.")
		return
	}
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
	}

	toCurse := getUidList(strings.Split(uidArg, ","))
	count := 0
	for _, c := range toCurse {
		c.AddPunishmentWithData(pType, duration, *reason, strconv.Itoa(offset))
		c.SendServerMessage(fmt.Sprintf("📐 You have been %v'd. Your sprite offset is locked at %d.", pType.String(), offset))
		count++
	}
	client.SendServerMessage(fmt.Sprintf("Applied %v to %d client(s) (offset %d).", pType.String(), count, offset))
}

// /shrink: lock the target into a negative vertical offset (default -25).
func cmdShrink(client *Client, args []string, usage string) {
	applyShrinkGrowWide(client, args, usage, PunishmentShrink, -25)
}

// /grow: lock the target into a positive vertical offset (default +25).
func cmdGrow(client *Client, args []string, usage string) {
	applyShrinkGrowWide(client, args, usage, PunishmentGrow, 25)
}

// /wide: lock the target into a horizontal offset (default +50).
func cmdWide(client *Client, args []string, usage string) {
	applyShrinkGrowWide(client, args, usage, PunishmentWide, 50)
}

// removeOffsetPunishment is shared by /unshrink, /ungrow, /unwide.
func removeOffsetPunishment(client *Client, args []string, pType PunishmentType, label string) {
	toClear := getUidList(strings.Split(args[0], ","))
	count := 0
	for _, c := range toClear {
		if !c.HasPunishment(pType) {
			continue
		}
		c.RemovePunishment(pType)
		c.SendServerMessage(fmt.Sprintf("Your %s effect has been removed.", label))
		count++
	}
	client.SendServerMessage(fmt.Sprintf("Removed %s effect from %d client(s).", label, count))
}

func cmdUnshrink(client *Client, args []string, _ string) {
	removeOffsetPunishment(client, args, PunishmentShrink, "shrink")
}
func cmdUngrow(client *Client, args []string, _ string) {
	removeOffsetPunishment(client, args, PunishmentGrow, "grow")
}
func cmdUnwide(client *Client, args []string, _ string) {
	removeOffsetPunishment(client, args, PunishmentWide, "wide")
}

// /fromsoftware
func cmdFromSoftware(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentFromSoftware)
}
