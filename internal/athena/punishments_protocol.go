/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: protocol-field punishments.

   These punishments never touch the message text — they weaponize the IC
   packet's other fields:

     /teleport     random self-offset per message; the sprite pops around the
                   viewport
     /shakecurse   forces the screenshake flag on every message
     /randomflip   coin-flips the sprite's horizontal flip per message
     /forcecolor   locks the text colour to a chosen value (customData, 0-9)
     /nopreanim    strips preanimations (PREANIM emote modifiers demoted,
                   preanim name cleared)
     /forcepreanim promotes idle/talk emote modifiers so the preanim plays

   applyProtocolPunishments runs in pktIC after the text-transform loop but
   BEFORE field validation, so every value written here must already be
   legal: Flip/Screenshake ∈ {"0","1"}, TextColor ∈ [0,9], offsets within
   [-100,100], EmoteModifier ∈ {0,1,2,5,6}.

   All six are standard punishments: MUTE-gated, -d/-r/-h, comma UID lists,
   `global`, /stack-able, DB-persisted and removable with /unpunish -t. */

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
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// Teleport keeps the sprite mostly on screen — fully off-screen is
// /hidedisplay's job (which also wins, since it is applied after this).
const (
	teleportMaxX = 75
	teleportMaxY = 50
)

// applyProtocolPunishments mutates the outgoing IC packet's non-text fields
// for the speaker's active protocol punishments. punishments is the active
// snapshot pktIC already fetched, so no extra lock is taken here.
func applyProtocolPunishments(ms *packet.MSPacket, punishments []PunishmentState) {
	for i := range punishments {
		p := &punishments[i]
		switch p.punishmentType {
		case PunishmentTeleport:
			x := rand.Intn(2*teleportMaxX+1) - teleportMaxX
			y := rand.Intn(2*teleportMaxY+1) - teleportMaxY
			ms.SelfOffset = encode(fmt.Sprintf("%d&%d", x, y))
		case PunishmentShakecurse:
			ms.Screenshake = "1"
		case PunishmentRandomflip:
			if rand.Intn(2) == 0 {
				ms.Flip = "1"
			} else {
				ms.Flip = "0"
			}
		case PunishmentForceColor:
			if c, err := strconv.Atoi(p.customData); err == nil && c >= 0 && c <= 9 {
				ms.TextColor = strconv.Itoa(c)
			}
		case PunishmentNoPreanim:
			switch ms.EmoteModifier {
			case "1", "2":
				ms.EmoteModifier = "0"
			case "6":
				ms.EmoteModifier = "5"
			}
			ms.PreAnim = "-"
		case PunishmentForcePreanim:
			// Only promote when the client actually named a preanim, so we
			// never point the viewport at an animation that doesn't exist.
			if ms.PreAnim != "" && ms.PreAnim != "-" {
				switch ms.EmoteModifier {
				case "0":
					ms.EmoteModifier = "1"
				case "5":
					ms.EmoteModifier = "6"
				}
			}
		}
	}
}

func cmdTeleport(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentTeleport)
}

func cmdShakecurse(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentShakecurse)
}

func cmdRandomflip(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentRandomflip)
}

func cmdNoPreanim(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentNoPreanim)
}

func cmdForcePreanim(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentForcePreanim)
}

// forceColorNames maps friendly colour names to AO2 text-colour indices.
// 0-5 are the universal client palette; 9 renders as rainbow on 2.8+.
var forceColorNames = map[string]int{
	"white":   0,
	"green":   1,
	"red":     2,
	"orange":  3,
	"blue":    4,
	"yellow":  5,
	"rainbow": 9,
}

// cmdForceColor handles /forcecolor. The chosen colour index is stored in the
// punishment's customData and persisted via the 0x1F reason convention so it
// survives reconnects (same mechanism as /translator's target language).
func cmdForceColor(client *Client, args []string, usage string) {
	args, hidden := extractHiddenFlag(args)

	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	if len(flags.Args()) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	uidArg := flags.Arg(0)
	colorArg := strings.ToLower(flags.Arg(1))
	color, ok := forceColorNames[colorArg]
	if !ok {
		v, err := strconv.Atoi(colorArg)
		if err != nil || v < 0 || v > 9 {
			client.SendServerMessage("Invalid colour. Use 0-9 or one of: white, green, red, orange, blue, yellow, rainbow.")
			return
		}
		color = v
	}
	colorStr := strconv.Itoa(color)

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
		client.SendServerMessage("Duration capped at 24 hours.")
	}

	tier := issuerTierFor(client)
	msg := fmt.Sprintf("🎨 Your IC text colour has been locked to '%v'", colorArg)
	if duration > 0 {
		msg += fmt.Sprintf(" for %v", duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}

	apply := func(c *Client) {
		c.AddPunishmentWithData(PunishmentForceColor, duration, *reason, colorStr)
		c.setPunishmentTier(PunishmentForceColor, tier)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		stored := colorStr + "\x1f" + *reason
		if err := db.UpsertTextPunishmentBy(c.Ipid(), int(PunishmentForceColor), expires, stored, int(tier)); err != nil {
			logger.LogErrorf("Failed to persist forcecolor for %v: %v", c.Ipid(), err)
		}
		if !hidden {
			c.SendServerMessage(msg)
		}
	}

	var count int
	var report string
	var skipped int
	var skippedReport string
	if strings.EqualFold(uidArg, "global") {
		targetArea := client.Area()
		issuerUID := client.Uid()
		clients.ForEach(func(c *Client) {
			if c.Area() != targetArea || c.Uid() == issuerUID || permissions.IsModerator(c.Perms()) {
				return
			}
			if punishmentSafeBlocked(c) {
				notePunishmentSafeSkip(&skipped, &skippedReport, c)
				return
			}
			apply(c)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		})
	} else {
		for _, c := range getUidList(strings.Split(uidArg, ",")) {
			if punishmentSafeBlocked(c) {
				notePunishmentSafeSkip(&skipped, &skippedReport, c)
				continue
			}
			apply(c)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	}

	report = strings.TrimSuffix(report, ", ")
	summary := fmt.Sprintf("Locked text colour '%v' on %v client(s).", colorArg, count)
	if hidden {
		summary += " (hidden)"
	}
	summary = appendPunishmentSafeNotice(summary, skipped, skippedReport)
	client.SendServerMessage(summary)
	addToBuffer(client, "CMD", fmt.Sprintf("Applied forcecolor (%v) to %v.", colorArg, report), false)
	alertPunishmentIssued(client, fmt.Sprintf("forcecolor (%s)", colorArg), report, count, duration, *reason, hidden)
}
