/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: /potions system.

   Self-applied fun effects players can opt into. Most potions just bind a
   short-duration punishment to the caller, leveraging the existing
   punishment text-transform pipeline. The "character" potion is a special
   case that doesn't fit the punishment model — it spawns a per-client
   ticker that rotates the player through random characters every 30s. */

package athena

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"
)

// defaultPotionDuration is the fallback duration when no -d flag is passed.
const defaultPotionDuration = 5 * time.Minute

// potionDef describes one drinkable potion.
type potionDef struct {
	name    string
	emoji   string
	desc    string
	apply   func(*Client, time.Duration) // Bind the effect; duration is the requested length.
	cleanup func(*Client)                // Optional override; defaults to RemovePunishment of the bound type.
	pType   PunishmentType
}

// potionRegistry is the canonical /potions catalogue. Keyed by lowercase name.
var potionRegistry = map[string]*potionDef{
	"drunk": {
		name:  "drunk",
		emoji: "🍺",
		desc:  "Slurs your words and shuffles letters around. (default 5 min)",
		pType: PunishmentDrunk,
	},
	"uwu": {
		name:  "uwu",
		emoji: "🌸",
		desc:  "Wewites yowo wowds wike this~ (default 5 min)",
		pType: PunishmentUwu,
	},
	"shy": {
		name:  "shy",
		emoji: "🥺",
		desc:  "U-um... s-stutters everything... (default 5 min)",
		pType: PunishmentDandere,
	},
	"dramatic": {
		name:  "dramatic",
		emoji: "🎭",
		desc:  "Speak'st thou in glorious Shakespearean tongue. (default 5 min)",
		pType: PunishmentShakespearean,
	},
	"pirate": {
		name:  "pirate",
		emoji: "🏴‍☠️",
		desc:  "Yarrr! Talk like a pirate. (default 5 min)",
		pType: PunishmentPirate,
	},
	"poet": {
		name:  "poet",
		emoji: "📜",
		desc:  "Reformats your speech into poetic flourish. (default 5 min)",
		pType: PunishmentPoet,
	},
	"caveman": {
		name:  "caveman",
		emoji: "🪨",
		desc:  "Talk simple. Words short. Grug happy. (default 5 min)",
		pType: PunishmentCaveman,
	},
	"fancy": {
		name:  "fancy",
		emoji: "🎩",
		desc:  "Replaces your text with fancy unicode characters. (default 5 min)",
		pType: PunishmentFancy,
	},
	"chef": {
		name:  "chef",
		emoji: "🍳",
		desc:  "Hwurdy-burdy Swedish-Chef-isms. (default 5 min)",
		pType: PunishmentChef,
	},
	"cherri": {
		name:  "cherri",
		emoji: "🍒",
		desc:  "Capitalizes Every Word You Say. (default 5 min)",
		pType: PunishmentCherri,
	},
	"omnidere": {
		name:  "omnidere",
		emoji: "💞",
		desc:  "Each line picks a random anime dere flavour. (default 5 min)",
		pType: PunishmentOmnidere,
	},
	"zalgo": {
		name:  "zalgo",
		emoji: "🌀",
		desc:  "C̴o̷r̶r̸u̵p̷t̶s̸ your text with creeping zalgo marks. (default 5 min)",
		pType: PunishmentZalgo,
	},
	// Special potion: not a punishment. The next player to speak in your area
	// receives a pair request from you — they still choose to accept, so
	// consent is preserved. Cleanup disarms the pending request.
	"love": {
		name:    "love",
		emoji:   "💘",
		desc:    "Auto-sends a pair request to the next player who speaks in your area. They still accept with /pair. (default 5 min)",
		apply:   startLovePotion,
		cleanup: stopLovePotion,
	},
	// Special potion: not a punishment. Auto-rotates the player's character
	// every 30 seconds for the requested duration. Cleanup cancels the rotation timer.
	"character": {
		name:    "character",
		emoji:   "🔄",
		desc:    "Rolls a random character every 30 seconds. (default 5 min)",
		apply:   startCharacterPotion,
		cleanup: stopCharacterPotion,
	},
}

// characterPotionState tracks the rotation goroutine for one client so
// /potion off can stop it. Keyed by *Client pointer (clients are unique).
var (
	characterPotionMu    sync.Mutex
	characterPotionState = map[*Client]chan struct{}{}
)

// startCharacterPotion launches a goroutine that swaps the client to a
// random free character every 30 seconds for d, then exits.
// Idempotent: a second drink restarts the timer rather than running two.
func startCharacterPotion(c *Client, d time.Duration) {
	characterPotionMu.Lock()
	if existing, ok := characterPotionState[c]; ok {
		close(existing)
	}
	stop := make(chan struct{})
	characterPotionState[c] = stop
	characterPotionMu.Unlock()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		expiry := time.NewTimer(d)
		defer expiry.Stop()

		for {
			select {
			case <-stop:
				return
			case <-expiry.C:
				c.SendServerMessage("🔄 Your character potion has worn off.")
				characterPotionMu.Lock()
				delete(characterPotionState, c)
				characterPotionMu.Unlock()
				return
			case <-ticker.C:
				if c.IsTunged() {
					continue
				}
				newID := getRandomFreeChar(c)
				if newID == -1 {
					continue
				}
				c.ChangeCharacter(newID)
			}
		}
	}()
}

// stopCharacterPotion cancels any running character-potion goroutine for c.
func stopCharacterPotion(c *Client) {
	characterPotionMu.Lock()
	defer characterPotionMu.Unlock()
	if ch, ok := characterPotionState[c]; ok {
		close(ch)
		delete(characterPotionState, c)
	}
}

// cmdPotions lists available potions or, with "off", clears all active ones.
// /potion <name> drinks a specific potion.
func cmdPotions(client *Client, _ []string, _ string) {
	// Build a sorted list for deterministic output.
	names := make([]string, 0, len(potionRegistry))
	for n := range potionRegistry {
		names = append(names, n)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("\n🧪 Potions Cabinet\n")
	sb.WriteString("Drink one with /potion <name>. Default duration: 5 minutes.\n")
	sb.WriteString("Optional: /potion -d <duration> <name>  (e.g. /potion -d 30m drunk, max 24h)\n")
	sb.WriteString("Use /potion off to flush every active potion.\n\n")
	for _, n := range names {
		p := potionRegistry[n]
		sb.WriteString(fmt.Sprintf("  %s /potion %s — %s\n", p.emoji, p.name, p.desc))
	}
	client.SendServerMessage(sb.String())
}

// cmdPotion drinks a named potion or, with "off", clears all active potions.
// Accepts an optional -d <duration> flag (default 5m, max 24h).
func cmdPotion(client *Client, args []string, usage string) {
	if isPunishmentBlocked(client.Ipid()) {
		client.SendServerMessage("A moderator has disabled self-applied punishment commands for you.")
		return
	}
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	durationStr := flags.String("d", "5m", "")
	flags.Parse(args)
	remaining := flags.Args()

	if len(remaining) < 1 {
		client.SendServerMessage(usage)
		return
	}
	name := strings.ToLower(remaining[0])

	if name == "off" {
		clearAllPotions(client)
		client.SendServerMessage("🧪 All potion effects flushed. You're back to normal.")
		return
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil || duration <= 0 {
		client.SendServerMessage("Invalid duration format. Use formats like: 5m, 1h, 30m, 2h30m")
		return
	}
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
	}

	p, ok := potionRegistry[name]
	if !ok {
		client.SendServerMessage(fmt.Sprintf(
			"Unknown potion '%s'. Type /potions for the menu.", name))
		return
	}

	// Apply: either run the special apply hook or bind the punishment type.
	if p.apply != nil {
		p.apply(client, duration)
	} else if p.pType != PunishmentNone {
		client.AddPunishment(p.pType, duration, "potion:"+p.name)
	}

	client.SendServerMessage(fmt.Sprintf(
		"%s You drink the %s potion. Effect lasts %v. Sip /potion off to flush early.",
		p.emoji, p.name, duration))
	addToBuffer(client, "CMD", fmt.Sprintf("Drank potion %s.", p.name), false)
}

// clearAllPotions removes every active potion effect for a client. Cancels
// the character-rotation goroutine and removes any potion-bound punishments.
func clearAllPotions(c *Client) {
	stopCharacterPotion(c)
	for _, p := range potionRegistry {
		if p.cleanup != nil && p.name != "character" {
			p.cleanup(c)
		}
		if p.pType != PunishmentNone && c.HasPunishment(p.pType) {
			c.RemovePunishment(p.pType)
		}
	}
}
