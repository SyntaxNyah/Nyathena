/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: mechanic punishments and area traps.

   These don't transform text — they make punishments MOVE:

     /contagious <type> <uid>   plague mode. The target carries <type> plus a
                                contagion marker; anyone who speaks in the
                                area within 5 s of an infected player's
                                message catches both, and the plague keeps
                                spreading from them. Moderators are immune.
     /minefield <uid>           every IC message has a 1-in-6 chance to
                                detonate, stacking a random 2-minute
                                punishment on the speaker.
     /silencebell [type]        arms a one-shot trap on the issuer's area:
                                the next non-moderator to speak gets cursed
                                (random effect unless a type is given).

   Plus the 💘 love potion trigger (see commands_potions.go): an armed
   drinker auto-sends a pair REQUEST to the next person who speaks in their
   area — consent preserved, the target still accepts with /pair.

   All hooks run from punishmentMechanicsOnIC, called once at the end of
   pktIC after a successful broadcast. Every path early-outs on cheap checks
   (an atomic counter for love potions, one shared mutex + map lookups for
   the area state) so idle servers pay ~nothing. */

package athena

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// contagionWindow is how long after an infected player's message the next
// speaker can catch the plague.
const contagionWindow = 5 * time.Second

// minefieldDetonationDuration is how long each stepped-on mine's random
// punishment lasts.
const minefieldDetonationDuration = 2 * time.Minute

// contagionMark records "an infected player just spoke here".
type contagionMark struct {
	at    time.Time
	pType PunishmentType // the underlying punishment that spreads
	until time.Time      // when the carrier's contagion expires; zero = permanent
	uid   int            // the carrier (doesn't reinfect themselves)
	tier  IssuerTier     // original issuer tier, inherited by victims
}

// bellTrap is an armed /silencebell waiting for its victim.
type bellTrap struct {
	pType    PunishmentType // PunishmentNone = random from the megamaso pool
	duration time.Duration
	tier     IssuerTier
	armedBy  int
}

var (
	mechanicsMu   sync.Mutex
	areaContagion = map[*area.Area]contagionMark{}
	areaBellTrap  = map[*area.Area]bellTrap{}

	// lovePotionsArmed gates the per-message area scan: zero armed potions
	// means zero extra work on the IC hot path. The counter may run briefly
	// high when a potion expires unobserved; the scan self-corrects it.
	lovePotionsArmed atomic.Int32
)

// hasPunishmentType reports whether the active-punishment snapshot pktIC
// already fetched contains pType — a lock-free slice scan.
func hasPunishmentType(punishments []PunishmentState, pType PunishmentType) bool {
	for i := range punishments {
		if punishments[i].punishmentType == pType {
			return true
		}
	}
	return false
}

// findPunishmentState returns a pointer into the snapshot for pType, or nil.
func findPunishmentState(punishments []PunishmentState, pType PunishmentType) *PunishmentState {
	for i := range punishments {
		if punishments[i].punishmentType == pType {
			return &punishments[i]
		}
	}
	return nil
}

// punishmentMechanicsOnIC runs once per successfully-broadcast IC message.
// punishments is the speaker's active snapshot from pktIC (no lock needed).
func punishmentMechanicsOnIC(client *Client, punishments []PunishmentState) {
	if lovePotionsArmed.Load() > 0 {
		lovePotionOnIC(client)
	}

	a := client.Area()
	mechanicsMu.Lock()
	trap, bellArmed := areaBellTrap[a]
	mark, hasMark := areaContagion[a]
	mechanicsMu.Unlock()

	if bellArmed {
		bellTriggerOnIC(client, a, trap)
	}
	if hasPunishmentType(punishments, PunishmentMinefield) {
		minefieldRoll(client)
	}
	contagionOnIC(client, a, punishments, mark, hasMark)
}

// ── Contagion ─────────────────────────────────────────────────────────────

func contagionOnIC(client *Client, a *area.Area, punishments []PunishmentState, mark contagionMark, hasMark bool) {
	now := time.Now()

	// Carriers refresh the area's plague marker on every message.
	if cp := findPunishmentState(punishments, PunishmentContagious); cp != nil {
		pType := parsePunishmentType(cp.customData)
		if pType == PunishmentNone {
			return
		}
		mechanicsMu.Lock()
		areaContagion[a] = contagionMark{at: now, pType: pType, until: cp.expiresAt, uid: client.Uid(), tier: cp.issuerTier}
		mechanicsMu.Unlock()
		return
	}

	// Healthy speaker: did they speak too close to an infected one?
	if !hasMark || now.Sub(mark.at) > contagionWindow || mark.uid == client.Uid() {
		return
	}
	if permissions.IsModerator(client.Perms()) {
		return
	}
	var remaining time.Duration
	if !mark.until.IsZero() {
		remaining = time.Until(mark.until)
		if remaining <= 0 {
			// Source plague already burned out; clear the stale marker.
			mechanicsMu.Lock()
			if m, ok := areaContagion[a]; ok && m.uid == mark.uid {
				delete(areaContagion, a)
			}
			mechanicsMu.Unlock()
			return
		}
	}

	infectClient(client, mark.pType, remaining, mark.tier)

	// The victim is now a carrier; move the marker to them immediately so
	// the chain doesn't wait for their next message.
	mechanicsMu.Lock()
	areaContagion[a] = contagionMark{at: now, pType: mark.pType, until: time.Now().UTC().Add(remaining), uid: client.Uid(), tier: mark.tier}
	if remaining == 0 {
		areaContagion[a] = contagionMark{at: now, pType: mark.pType, uid: client.Uid(), tier: mark.tier}
	}
	mechanicsMu.Unlock()

	client.SendServerMessage(fmt.Sprintf("🤧 You caught '%v'! You are now contagious — anyone who speaks near you may be next…", mark.pType.String()))
	sendAreaServerMessage(a, fmt.Sprintf("☣️ %v caught '%v'! The plague spreads…", clientDisplayName(client), mark.pType.String()))
	logger.LogInfof("Contagion: UID %v IPID %v caught %v", client.Uid(), client.Ipid(), mark.pType.String())
}

// infectClient applies the underlying punishment plus the contagion marker
// and persists both. remaining == 0 means permanent (matches the source).
func infectClient(client *Client, pType PunishmentType, remaining time.Duration, tier IssuerTier) {
	client.AddPunishmentBy(pType, remaining, "caught the plague", tier)
	client.AddPunishmentWithData(PunishmentContagious, remaining, "caught the plague", pType.String())
	client.setPunishmentTier(PunishmentContagious, tier)

	var expires int64
	if remaining > 0 {
		expires = time.Now().UTC().Add(remaining).Unix()
	}
	if err := db.UpsertTextPunishmentBy(client.Ipid(), int(pType), expires, "caught the plague", int(tier)); err != nil {
		logger.LogErrorf("Failed to persist contagion payload for %v: %v", client.Ipid(), err)
	}
	stored := pType.String() + "\x1f" + "caught the plague"
	if err := db.UpsertTextPunishmentBy(client.Ipid(), int(PunishmentContagious), expires, stored, int(tier)); err != nil {
		logger.LogErrorf("Failed to persist contagion marker for %v: %v", client.Ipid(), err)
	}
}

// cmdContagious handles /contagious <type> [-d] [-r] [-h] global|<uids>.
func cmdContagious(client *Client, args []string, usage string) {
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

	pType := parsePunishmentType(flags.Arg(0))
	switch pType {
	case PunishmentNone:
		client.SendServerMessage(fmt.Sprintf("Unknown punishment type: %v", flags.Arg(0)))
		return
	case PunishmentContagious, PunishmentLag, PunishmentMinefield, PunishmentLifo, PunishmentStealthMute:
		client.SendServerMessage(fmt.Sprintf("'%v' cannot be made contagious.", pType.String()))
		return
	}

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
	uidArg := flags.Arg(1)

	apply := func(c *Client) {
		c.AddPunishmentBy(pType, duration, *reason, tier)
		c.AddPunishmentWithData(PunishmentContagious, duration, *reason, pType.String())
		c.setPunishmentTier(PunishmentContagious, tier)
		var expires int64
		if duration > 0 {
			expires = time.Now().UTC().Add(duration).Unix()
		}
		if err := db.UpsertTextPunishmentBy(c.Ipid(), int(pType), expires, *reason, int(tier)); err != nil {
			logger.LogErrorf("Failed to persist contagion payload for %v: %v", c.Ipid(), err)
		}
		stored := pType.String() + "\x1f" + *reason
		if err := db.UpsertTextPunishmentBy(c.Ipid(), int(PunishmentContagious), expires, stored, int(tier)); err != nil {
			logger.LogErrorf("Failed to persist contagion marker for %v: %v", c.Ipid(), err)
		}
		if !hidden {
			c.SendServerMessage(fmt.Sprintf("🤧 You have been infected with contagious '%v'. Anyone who speaks near you may catch it…", pType.String()))
		}
	}

	var count int
	var report string
	if strings.EqualFold(uidArg, "global") {
		targetArea := client.Area()
		issuerUID := client.Uid()
		clients.ForEach(func(c *Client) {
			if c.Area() != targetArea || c.Uid() == issuerUID || permissions.IsModerator(c.Perms()) {
				return
			}
			apply(c)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		})
	} else {
		for _, c := range getUidList(strings.Split(uidArg, ",")) {
			apply(c)
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	}

	report = strings.TrimSuffix(report, ", ")
	summary := fmt.Sprintf("Infected %v client(s) with contagious '%v'.", count, pType.String())
	if hidden {
		summary += " (hidden)"
	}
	client.SendServerMessage(summary)
	if count > 0 && !hidden {
		sendAreaServerMessage(client.Area(), "☣️ A sneeze echoes through the area. Someone in here doesn't look so good…")
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Applied contagious '%v' to %v.", pType.String(), report), false)
	alertPunishmentIssued(client, fmt.Sprintf("contagious (%s)", pType.String()), report, count, duration, *reason, hidden)
}

// ── Minefield ─────────────────────────────────────────────────────────────

func minefieldRoll(client *Client) {
	if rand.Intn(6) != 0 {
		return
	}
	// Pick a mine the speaker isn't already wearing, like /megamaso does.
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

	client.AddPunishment(pick, minefieldDetonationDuration, "minefield detonation")
	expires := time.Now().UTC().Add(minefieldDetonationDuration).Unix()
	if err := db.UpsertTextPunishment(client.Ipid(), int(pick), expires, "minefield detonation"); err != nil {
		logger.LogErrorf("Failed to persist minefield detonation for %v: %v", client.Ipid(), err)
	}

	client.SendServerMessage(fmt.Sprintf("💥 You stepped on a mine! '%v' for %v.", pick.String(), minefieldDetonationDuration))
	sendAreaServerMessage(client.Area(), fmt.Sprintf("💥 %v stepped on a mine! (%v for %v)", clientDisplayName(client), pick.String(), minefieldDetonationDuration))
}

func cmdMinefield(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMinefield)
}

// ── Silence bell ──────────────────────────────────────────────────────────

func bellTriggerOnIC(client *Client, a *area.Area, trap bellTrap) {
	// Moderators and the arming mod ring right past the bell.
	if permissions.IsModerator(client.Perms()) || client.Uid() == trap.armedBy {
		return
	}

	// Consume the trap; a concurrent speaker may have beaten us to it.
	mechanicsMu.Lock()
	current, ok := areaBellTrap[a]
	if !ok || current != trap {
		mechanicsMu.Unlock()
		return
	}
	delete(areaBellTrap, a)
	mechanicsMu.Unlock()

	pick := trap.pType
	if pick == PunishmentNone {
		pick = megamasoStackPool[rand.Intn(len(megamasoStackPool))]
	}

	client.AddPunishmentBy(pick, trap.duration, "the bell tolled", trap.tier)
	var expires int64
	if trap.duration > 0 {
		expires = time.Now().UTC().Add(trap.duration).Unix()
	}
	if err := db.UpsertTextPunishmentBy(client.Ipid(), int(pick), expires, "the bell tolled", int(trap.tier)); err != nil {
		logger.LogErrorf("Failed to persist silencebell curse for %v: %v", client.Ipid(), err)
	}

	client.SendServerMessage(fmt.Sprintf("🔔 The bell tolls for thee. Cursed with '%v' for %v.", pick.String(), trap.duration))
	sendAreaServerMessage(a, fmt.Sprintf("🔔 The bell tolls for %v! Cursed with '%v' (%v).", clientDisplayName(client), pick.String(), trap.duration))
}

// cmdSilencebell handles /silencebell [type] [-d duration] | off | status.
func cmdSilencebell(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	a := client.Area()

	if len(flags.Args()) > 0 {
		switch strings.ToLower(flags.Arg(0)) {
		case "off":
			mechanicsMu.Lock()
			_, armed := areaBellTrap[a]
			delete(areaBellTrap, a)
			mechanicsMu.Unlock()
			if armed {
				client.SendServerMessage("🔔 The silence bell has been disarmed.")
			} else {
				client.SendServerMessage("No silence bell is armed in this area.")
			}
			return
		case "status":
			mechanicsMu.Lock()
			trap, armed := areaBellTrap[a]
			mechanicsMu.Unlock()
			if !armed {
				client.SendServerMessage("No silence bell is armed in this area.")
				return
			}
			effect := "random"
			if trap.pType != PunishmentNone {
				effect = trap.pType.String()
			}
			client.SendServerMessage(fmt.Sprintf("🔔 A silence bell is armed here: effect '%v' for %v.", effect, trap.duration))
			return
		}
	}

	pick := PunishmentNone
	if len(flags.Args()) > 0 {
		pick = parsePunishmentType(flags.Arg(0))
		if pick == PunishmentNone {
			client.SendServerMessage(fmt.Sprintf("Unknown punishment type: %v\n%v", flags.Arg(0), usage))
			return
		}
		switch pick {
		case PunishmentContagious, PunishmentLag, PunishmentMinefield, PunishmentLifo, PunishmentStealthMute:
			client.SendServerMessage(fmt.Sprintf("'%v' cannot be loaded into the bell.", pick.String()))
			return
		}
	}

	duration, err := str2duration.ParseDuration(*durationStr)
	if err != nil {
		client.SendServerMessage("Invalid duration format. Use format like: 10m, 1h, 30s")
		return
	}
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
		client.SendServerMessage("Duration capped at 24 hours.")
	}

	mechanicsMu.Lock()
	areaBellTrap[a] = bellTrap{pType: pick, duration: duration, tier: issuerTierFor(client), armedBy: client.Uid()}
	mechanicsMu.Unlock()

	effect := "a random curse"
	if pick != PunishmentNone {
		effect = "'" + pick.String() + "'"
	}
	client.SendServerMessage(fmt.Sprintf("🔔 Silence bell armed: the next non-moderator to speak receives %v for %v.", effect, duration))
	sendAreaServerMessage(a, "🔔 A silence bell tolls through the area… the next soul to speak shall be cursed.")
	addToBuffer(client, "CMD", fmt.Sprintf("Armed silencebell (%v, %v).", effect, duration), false)
	alertPunishmentIssued(client, fmt.Sprintf("silencebell (%s, armed)", effect), "", 0, duration, "", false)
}

// ── Love potion ───────────────────────────────────────────────────────────

// LovePotionActive reports whether the client's love potion is armed and
// unexpired.
func (client *Client) LovePotionActive() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.lovePotionUntil.IsZero() && time.Now().Before(client.lovePotionUntil)
}

// armLovePotion arms the love potion for d. Returns true when the client was
// not previously armed (so the caller can bump the global counter once).
func (client *Client) armLovePotion(d time.Duration) bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	wasArmed := !client.lovePotionUntil.IsZero() && time.Now().Before(client.lovePotionUntil)
	client.lovePotionUntil = time.Now().Add(d)
	return !wasArmed
}

// disarmLovePotion clears the armed state. Returns true when the client was
// armed (expired-but-uncleared counts: the counter still needs decrementing).
func (client *Client) disarmLovePotion() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	armed := !client.lovePotionUntil.IsZero()
	client.lovePotionUntil = time.Time{}
	return armed
}

// lovePotionOnIC fires armed love potions at the speaker: each armed client
// in the area sends the speaker a pair request, exactly as if they had typed
// /pair <speaker>. If the speaker was already requesting them, the pair
// completes mutually — otherwise the speaker still has to accept.
func lovePotionOnIC(speaker *Client) {
	if speaker.CharID() < 0 {
		return
	}
	a := speaker.Area()
	clients.ForEach(func(c *Client) {
		if c == speaker || c.Area() != a {
			return
		}
		c.mu.Lock()
		armedUntil := c.lovePotionUntil
		c.mu.Unlock()
		if armedUntil.IsZero() {
			return
		}
		if time.Now().After(armedUntil) {
			// Expired unobserved — clean up and fix the gate counter.
			if c.disarmLovePotion() {
				lovePotionsArmed.Add(-1)
			}
			return
		}
		if c.CharID() < 0 {
			return
		}
		if c.disarmLovePotion() {
			lovePotionsArmed.Add(-1)
		}

		c.SetPairWantedID(speaker.CharID())
		if speaker.PairWantedID() == c.CharID() {
			c.SetForcePairUID(speaker.Uid())
			speaker.SetForcePairUID(c.Uid())
			c.SendServerMessage(fmt.Sprintf("💘 The love potion takes hold — now pairing with %v!", oocDisplayName(speaker)))
			speaker.SendServerMessage(fmt.Sprintf("💘 %v's love potion takes hold — now pairing with them!", oocDisplayName(c)))
		} else {
			c.SendServerMessage(fmt.Sprintf("💘 Your love potion kicks in! You sent a pair request to %v.", oocDisplayName(speaker)))
			speaker.SendServerMessage(fmt.Sprintf("💘 %v (under a love potion) wants to pair with you. Type /pair %v to accept.", oocDisplayName(c), c.Uid()))
		}
	})
}

// startLovePotion / stopLovePotion are the potionRegistry hooks.
func startLovePotion(c *Client, d time.Duration) {
	if c.armLovePotion(d) {
		lovePotionsArmed.Add(1)
	}
}

func stopLovePotion(c *Client) {
	if c.disarmLovePotion() {
		lovePotionsArmed.Add(-1)
	}
}
