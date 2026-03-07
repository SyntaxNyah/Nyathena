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
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	rrJoinWindow      = 30 * time.Second // opt-in window duration
	rrCooldown        = 5 * time.Minute  // global delay between games
	rrChambers        = 6                // standard revolver chambers
	rrMinPlayers      = 2                // minimum players to start
	rrPunishDuration  = 15 * time.Minute // how long the loser's punishment lasts
	rrShotPause       = 3 * time.Second  // dramatic pause between shots
	rrDoubleBulletP   = 20               // % chance of 2-bullet game (integer 0–100)
	rrRicochetP       = 8                // % chance the shot ricochets to a random player
	rrChainShotP      = 8                // % chance a second player is also hit on BANG
	rrDoublePunishP   = 12               // % chance the victim receives two punishments
	rrReSpinP         = 10               // % chance the cylinder re-spins after a safe CLICK
	rrSurvivorCurseP  = 15               // % chance all survivors receive a minor curse after game ends
	rrCurseDuration   = 5 * time.Minute  // duration of survivor-curse punishments
)

// rrRules is broadcast when the join window opens.
const rrRules = `🔫 RUSSIAN ROULETTE — LOADING CHAMBERS... 🔫
Type /roulette join within 30 seconds to take a seat at the table.

📋 HOW TO PLAY:
• Players sit in a circle; the cylinder is spun once and the game begins.
• Each turn one chamber is fired — safe chambers go CLICK; the bullet goes BANG.
• The player hit receives a wild random punishment lasting 15 minutes.
• 🎰 CHAOS EVENTS that may strike at any time:
    – Double Bullet: 20% chance two bullets are loaded instead of one.
    – Ricochet: 8% chance the shot deflects onto a different player.
    – Chain Shot: 8% chance BANG hits a second random victim too!
    – Double Punishment: 12% chance the victim earns TWO punishments at once.
    – Cylinder Re-Spin: 10% chance the cylinder re-spins after a safe click!
    – Survivor Curse: 15% chance all survivors receive a parting gift too...
• A minimum of 2 players is required. May the odds be ever in your favour. 🎲`

// ── Punishment pool ──────────────────────────────────────────────────────────

// rrPunishmentPool is the full wild set available to Russian Roulette.
// Allocated once at package init; never modified at runtime.
var rrPunishmentPool = []PunishmentType{
	PunishmentBackward,
	PunishmentStutterstep,
	PunishmentElongate,
	PunishmentUppercase,
	PunishmentLowercase,
	PunishmentRobotic,
	PunishmentAlternating,
	PunishmentUwu,
	PunishmentPirate,
	PunishmentShakespearean,
	PunishmentCaveman,
	PunishmentDrunk,
	PunishmentHiccup,
	PunishmentConfused,
	PunishmentParanoid,
	PunishmentMumble,
	PunishmentSubtitles,
	PunishmentFancy,
	PunishmentEmoji,
	PunishmentSlowpoke,
	PunishmentFastspammer,
	PunishmentLag,
	PunishmentCensor,
	PunishmentSpaghetti,
	PunishmentTorment,
	PunishmentRng,
	PunishmentHaiku,
	PunishmentAutospell,
	PunishmentMonkey,
	PunishmentSnake,
	PunishmentDog,
	PunishmentCat,
	PunishmentBird,
	PunishmentCow,
	PunishmentFrog,
	PunishmentDuck,
	PunishmentHorse,
	PunishmentLion,
	PunishmentZoo,
	PunishmentBunny,
	PunishmentTsundere,
	PunishmentYandere,
	PunishmentKuudere,
	PunishmentDandere,
	PunishmentDeredere,
	PunishmentHimedere,
	PunishmentKamidere,
	PunishmentUndere,
	PunishmentBakadere,
	PunishmentMayadere,
	PunishmentEmoticon,
	PunishmentLovebomb,
	PunishmentDegrade,
	PunishmentTourettes,
	PunishmentSpotlight,
	PunishmentInvisible,
	PunishmentWhistle,
	PunishmentWhisper,
}

// rrCursePunishmentPool is a lighter pool used for survivor curses.
var rrCursePunishmentPool = []PunishmentType{
	PunishmentStutterstep,
	PunishmentUppercase,
	PunishmentLowercase,
	PunishmentUwu,
	PunishmentHiccup,
	PunishmentConfused,
	PunishmentMumble,
	PunishmentSubtitles,
	PunishmentFancy,
	PunishmentEmoji,
	PunishmentEmoticon,
	PunishmentWhistle,
	PunishmentWhisper,
}

func randomRRPunishment() PunishmentType {
	return rrPunishmentPool[rand.Intn(len(rrPunishmentPool))]
}

// randomRRPunishmentExcluding returns a random punishment that differs from the excluded one.
func randomRRPunishmentExcluding(exclude PunishmentType) PunishmentType {
	for {
		p := randomRRPunishment()
		if p != exclude {
			return p
		}
	}
}

func randomRRCursePunishment() PunishmentType {
	return rrCursePunishmentPool[rand.Intn(len(rrCursePunishmentPool))]
}

// ── Flavour text ─────────────────────────────────────────────────────────────

var rrGunNames = []string{
	"Old Betsy", "The Equaliser", "Lady Luck", "The Grim Revolver",
	"Madame Six", "The Iron Dice", "Widow-Maker", "The Philosopher's Gun",
	"The Cursed Revenant", "Fate's Last Word", "The Thunderclap",
	"Doom's Finger", "Madame Misfortune", "The Silver Gamble",
}

var rrClickMessages = []string{
	"*click* — still breathing! 😅",
	"*click* — phew! That was close... 😰",
	"*click* — the cylinder spins on... 🌀",
	"*click* — SAFE! (for now) 😤",
	"*click* — the room exhales... 💨",
	"*CLICK* — luck favours you today! 🍀",
	"*click* — your heart skips a beat... 💓",
	"*click* — you dodged the reaper! 💀",
	"*CLICK* — another chamber survives! 🫀",
	"*click* — the odds grow against you... 📉",
	"*click* — the table holds its breath 😶",
	"*CLICK* — a hollow victory... for now. 🕯️",
}

var rrBangMessages = []string{
	"💥 BANG! The hammer falls!",
	"💥 BANG!! The gun roars!",
	"🔥 BANG! Fate has spoken!",
	"💀 BANG! The cylinder found its mark!",
	"🎆 **BANG!!** The house always wins!",
	"☠️  B-B-BANG! Nobody saw that coming!",
	"🔫 BANG! The game is over!",
	"💣 **BANG!!** The walls shake!",
	"🌋 BANG! Chaos reigns!",
	"⚡ BANG! The room goes white!",
	"🎯 BANG! Bullseye of doom!",
	"🩸 **BANG!!** Fate sealed with iron!",
}

var rrTensionMessages = []string{
	"😨 The table is deathly quiet...",
	"🥵 Sweat drips from every brow...",
	"🤐 Nobody dares to breathe...",
	"👀 All eyes are on the barrel...",
	"⏳ The moment of truth approaches...",
	"🫀 Hearts pound like war drums...",
	"🌡️  The temperature drops three degrees...",
	"🕯️  A candle flickers... then steadies...",
	"👁️  Something watches from the shadows...",
	"🦗 Even the crickets fall silent...",
}

var rrCriticalMessages = []string{
	"😱 CRITICAL MOMENT — the odds are harrowing now!",
	"💀 The final chambers loom... someone will fall!",
	"🩸 Blood runs cold as the last shots approach...",
	"🔥 Fate can only be delayed so long...",
	"⚠️  DANGER ZONE — the bullet is still out there!",
}

var rrChainMessages = []string{
	"💥⛓️  CHAIN SHOT! The shrapnel finds a second target!",
	"🔗 CHAIN REACTION! Two victims fall to one bullet!",
	"💥💥 DOUBLE TAP! The chaos doesn't stop at one!",
	"🌊 RICOCHET CHAIN! The destruction spreads!",
}

var rrReSpinMessages = []string{
	"🌀 CHAOS EVENT: Someone nudged the cylinder — IT SPINS AGAIN!",
	"😈 A mischievous hand re-spins the cylinder mid-game!",
	"🎰 WILD RESPIN! The odds reset... or do they get worse?",
	"🌪️  CHAOS RESPIN! The cylinder whirls with renewed menace!",
}

var rrDoublePunishMessages = []string{
	"💀💀 DOUBLE CURSED! Fate was not satisfied with one punishment!",
	"🎭 TWO FOR ONE! The universe piles on!",
	"⚡⚡ DOUBLE TROUBLE! Two punishments hit at once!",
	"🌪️  CHAOS COMBO! The loser earns a bonus curse!",
}

var rrSurvivorCurseMessages = []string{
	"😈 Surviving was never truly free... the curse spreads!",
	"🦠 SURVIVOR TAX! Nobody escapes the table unscathed!",
	"👻 The ghost of the bullet curses the survivors!",
	"🌑 DARK FORTUNE! Even the lucky pay a price today...",
}

// ── State ────────────────────────────────────────────────────────────────────

type rrState struct {
	mu         sync.Mutex
	joinActive bool
	gameActive bool
	players    []int     // UIDs in shuffled turn order
	lastEnd    time.Time // when the last game ended (drives cooldown)
}

var rr = rrState{}

// isRRCoolingDown reports whether the global cooldown is active and how many
// whole seconds remain.
func isRRCoolingDown() (bool, int) {
	rr.mu.Lock()
	end := rr.lastEnd
	rr.mu.Unlock()

	if end.IsZero() {
		return false, 0
	}
	if rem := rrCooldown - time.Since(end); rem > 0 {
		return true, int((rem+time.Second-1)/time.Second)
	}
	return false, 0
}

// ── Command entry point ───────────────────────────────────────────────────────

// cmdRoulette handles /roulette and /roulette join.
func cmdRussianRoulette(client *Client, args []string, usage string) {
	if len(args) > 0 && args[0] == "join" {
		rrJoin(client)
		return
	}
	// No subcommand: start a new game or join the open window.
	rr.mu.Lock()
	joinOpen := rr.joinActive
	rr.mu.Unlock()

	if joinOpen {
		rrJoin(client)
	} else {
		rrStart(client)
	}
}

// ── Start ─────────────────────────────────────────────────────────────────────

// rrStart opens the join window for a new Russian Roulette game.
func rrStart(client *Client) {
	rr.mu.Lock()

	if rr.joinActive || rr.gameActive {
		rr.mu.Unlock()
		client.SendServerMessage("A Russian Roulette game is already in progress.")
		return
	}

	if !rr.lastEnd.IsZero() {
		if rem := rrCooldown - time.Since(rr.lastEnd); rem > 0 {
			rr.mu.Unlock()
			client.SendServerMessage(fmt.Sprintf("Roulette is on cooldown. Please wait %d seconds.",
				int((rem+time.Second-1)/time.Second)))
			return
		}
	}

	rr.joinActive = true
	rr.players = rr.players[:0] // reuse backing array if present
	rr.mu.Unlock()

	sendGlobalServerMessage(rrRules)
	addToBuffer(client, "CMD", "Started Russian Roulette join window", false)

	// Auto-enrol the starter.
	rrJoin(client)

	go rrJoinTimer(client.OOCName())
}

// ── Join ──────────────────────────────────────────────────────────────────────

// rrJoin opts a player into the open join window.
func rrJoin(client *Client) {
	uid := client.Uid()

	rr.mu.Lock()
	if !rr.joinActive {
		rr.mu.Unlock()
		client.SendServerMessage("There is no Russian Roulette game to join right now.")
		return
	}
	for _, p := range rr.players {
		if p == uid {
			rr.mu.Unlock()
			client.SendServerMessage("You are already seated at the table.")
			return
		}
	}
	rr.players = append(rr.players, uid)
	count := len(rr.players)
	rr.mu.Unlock()

	client.SendServerMessage(fmt.Sprintf("🔫 You took a seat at the table! (%d player(s) so far)", count))
	sendGlobalServerMessage(fmt.Sprintf("🔫 %v sits down for Roulette! (%d player(s))", client.OOCName(), count))
}

// ── Join timer ────────────────────────────────────────────────────────────────

// rrJoinTimer waits for the join window to close, then kicks off the game or
// cancels if too few players opted in.
func rrJoinTimer(starterName string) {
	time.Sleep(rrJoinWindow)

	rr.mu.Lock()
	if !rr.joinActive {
		rr.mu.Unlock()
		return // already cancelled
	}

	// Filter players who are still connected.
	n := 0
	for _, uid := range rr.players {
		if _, err := getClientByUid(uid); err == nil {
			rr.players[n] = uid
			n++
		}
	}
	rr.players = rr.players[:n]

	if n < rrMinPlayers {
		rr.joinActive = false
		rr.lastEnd = time.Now().UTC()
		rr.mu.Unlock()
		sendGlobalServerMessage(fmt.Sprintf(
			"🔫 Russian Roulette cancelled — not enough players joined (need %d, got %d).", rrMinPlayers, n))
		return
	}

	// Shuffle player order and load the cylinder.
	rand.Shuffle(n, func(i, j int) { rr.players[i], rr.players[j] = rr.players[j], rr.players[i] })

	bullets := rrInitialBullets()
	rr.joinActive = false
	rr.gameActive = true
	players := make([]int, n)
	copy(players, rr.players)
	rr.mu.Unlock()

	gunName := rrGunNames[rand.Intn(len(rrGunNames))]
	bulletWord := "bullet"
	if bullets > 1 {
		bulletWord = "bullets"
	}
	sendGlobalServerMessage(fmt.Sprintf(
		"🔫 %v raises '%v' — %d %s loaded into %d chambers. The cylinder spins...\n%s",
		starterName, gunName, bullets, bulletWord, rrChambers,
		rrTensionMessages[rand.Intn(len(rrTensionMessages))],
	))

	go rrRun(players, bullets)
}

// ── Game loop ─────────────────────────────────────────────────────────────────

// rrInitialBullets returns the number of bullets to load for a new cylinder spin.
func rrInitialBullets() int {
	if rand.Intn(100) < rrDoubleBulletP {
		return 2
	}
	return 1
}

// rrRun executes the full game loop in its own goroutine.
// It cycles through players in order; each turn a chamber is fired.
// The bullet probability is recalculated per shot (authentic RR mechanics).
func rrRun(players []int, bullets int) {
	remaining := rrChambers
	alive := bullets // bullets still in the cylinder

	for i := 0; ; i++ {
		time.Sleep(rrShotPause)

		shooterUID := players[i%len(players)]
		shooter, err := getClientByUid(shooterUID)

		shooterName := fmt.Sprintf("UID %d", shooterUID)
		if err == nil {
			shooterName = shooter.OOCName()
		}

		// Tension flavour: regular tension every other round; critical messages when ≤2 remain.
		if i > 0 {
			if remaining <= 2 {
				sendGlobalServerMessage(rrCriticalMessages[rand.Intn(len(rrCriticalMessages))])
				time.Sleep(time.Second)
			} else if i%2 == 0 {
				sendGlobalServerMessage(rrTensionMessages[rand.Intn(len(rrTensionMessages))])
				time.Sleep(time.Second)
			}
		}

		// Probability of hitting a bullet this chamber.
		hit := rand.Intn(remaining) < alive

		// Ricochet: rare chance — redirect to a random OTHER player.
		victim := shooterUID
		victimName := shooterName
		if hit && rand.Intn(100) < rrRicochetP && len(players) > 1 {
			eligible := make([]int, 0, len(players)-1)
			for _, p := range players {
				if p != shooterUID {
					eligible = append(eligible, p)
				}
			}
			victim = eligible[rand.Intn(len(eligible))]
			if vc, verr := getClientByUid(victim); verr == nil {
				victimName = vc.OOCName()
			}
			sendGlobalServerMessage(fmt.Sprintf(
				"💫 RICOCHET! The bullet deflects off %v's wristwatch and veers toward %v!",
				shooterName, victimName,
			))
			time.Sleep(time.Second)
		}

		if hit {
			// ── BANG ──────────────────────────────────────────────────────────
			bangMsg := rrBangMessages[rand.Intn(len(rrBangMessages))]
			sendGlobalServerMessage(fmt.Sprintf("%s\n%v takes the hit!", bangMsg, victimName))

			pType := randomRRPunishment()

			// Double Punishment: victim earns two punishments at once.
			doubleHit := rand.Intn(100) < rrDoublePunishP
			var pType2 PunishmentType
			if doubleHit {
				pType2 = randomRRPunishmentExcluding(pType)
				sendGlobalServerMessage(rrDoublePunishMessages[rand.Intn(len(rrDoublePunishMessages))])
				time.Sleep(time.Second)
			}

			applyVictimPunishment := func(uid int, reason string) {
				vc, verr := getClientByUid(uid)
				if verr != nil {
					return
				}
				vc.AddPunishment(pType, rrPunishDuration, reason)
				if doubleHit {
					vc.AddPunishment(pType2, rrPunishDuration, reason+" (bonus)")
					vc.SendServerMessage(fmt.Sprintf(
						"💀 Doubly cursed! Punished with '%v' AND '%v' for %v. Brutal!", pType, pType2, rrPunishDuration))
				} else {
					vc.SendServerMessage(fmt.Sprintf(
						"💀 You took the hit! Punished with '%v' for %v.", pType, rrPunishDuration))
				}
			}

			if victim == shooterUID {
				if err == nil {
					applyVictimPunishment(victim, "Russian Roulette: shot")
				}
			} else {
				applyVictimPunishment(victim, "Russian Roulette: ricochet")
			}

			// Chain Shot: a second random player also takes a (different) punishment.
			if rand.Intn(100) < rrChainShotP && len(players) > 1 {
				chainMsg := rrChainMessages[rand.Intn(len(rrChainMessages))]
				sendGlobalServerMessage(chainMsg)
				time.Sleep(time.Second)
				// Build an explicit list of eligible players (everyone except the current victim).
				eligible := make([]int, 0, len(players)-1)
				for _, p := range players {
					if p != victim {
						eligible = append(eligible, p)
					}
				}
				if len(eligible) > 0 {
					chainUID := eligible[rand.Intn(len(eligible))]
					chainPType := randomRRPunishmentExcluding(pType)
					if chainC, cerr := getClientByUid(chainUID); cerr == nil {
						chainC.AddPunishment(chainPType, rrPunishDuration, "Russian Roulette: chain shot")
						chainC.SendServerMessage(fmt.Sprintf(
							"⛓️  The chain shot caught YOU! Punished with '%v' for %v.", chainPType, rrPunishDuration))
						sendGlobalServerMessage(fmt.Sprintf(
							"⛓️  Chain shot claims %v — punished with '%v'!", chainC.OOCName(), chainPType))
						addToBuffer(chainC, "ROULETTE",
							fmt.Sprintf("Chain-shot victim in Russian Roulette; punished with %v", chainPType), false)
					}
				}
			}

			pLabel := fmt.Sprintf("'%v'", pType)
			if doubleHit {
				pLabel = fmt.Sprintf("'%v' & '%v'", pType, pType2)
			}
			sendGlobalServerMessage(fmt.Sprintf(
				"☠️  ROULETTE OVER! %v drew the short straw and received %v. Better luck next life!",
				victimName, pLabel,
			))

			// Announce the survivors.
			survivors := make([]string, 0, len(players)-1)
			survivorUIDs := make([]int, 0, len(players)-1)
			for _, p := range players {
				if p != victim {
					if c, cerr := getClientByUid(p); cerr == nil {
						survivors = append(survivors, c.OOCName())
						survivorUIDs = append(survivorUIDs, p)
					}
				}
			}
			if len(survivors) > 0 {
				sendGlobalServerMessage(fmt.Sprintf("🏆 Survivors: %v — well played!", joinNames(survivors)))
			}

			// Survivor Curse: rare chance all survivors also get a minor punishment.
			if len(survivorUIDs) > 0 && rand.Intn(100) < rrSurvivorCurseP {
				time.Sleep(time.Second)
				sendGlobalServerMessage(rrSurvivorCurseMessages[rand.Intn(len(rrSurvivorCurseMessages))])
				time.Sleep(time.Second)
				for _, sUID := range survivorUIDs {
					if sc, scerr := getClientByUid(sUID); scerr == nil {
						cursePType := randomRRCursePunishment()
						sc.AddPunishment(cursePType, rrCurseDuration, "Russian Roulette: survivor curse")
						sc.SendServerMessage(fmt.Sprintf(
							"👻 Survivor curse! Punished with '%v' for %v.", cursePType, rrCurseDuration))
					}
				}
			}

			// Log to buffer.
			if loser, lerr := getClientByUid(victim); lerr == nil {
				addToBuffer(loser, "ROULETTE",
					fmt.Sprintf("Shot in Russian Roulette; punished with %v", pType), false)
			}

			// Close the game.
			rr.mu.Lock()
			rr.gameActive = false
			rr.players = rr.players[:0]
			rr.lastEnd = time.Now().UTC()
			rr.mu.Unlock()
			return
		}

		// ── CLICK ─────────────────────────────────────────────────────────────
		remaining--
		sendGlobalServerMessage(fmt.Sprintf(
			"%v's turn — %s (%d/%d chambers remain)",
			shooterName,
			rrClickMessages[rand.Intn(len(rrClickMessages))],
			remaining, rrChambers,
		))

		// Cylinder Re-Spin: rare chance the cylinder resets mid-game.
		if remaining > 0 && rand.Intn(100) < rrReSpinP {
			time.Sleep(time.Second)
			sendGlobalServerMessage(rrReSpinMessages[rand.Intn(len(rrReSpinMessages))])
			remaining = rrChambers
			alive = rrInitialBullets()
			time.Sleep(time.Second)
			sendGlobalServerMessage(fmt.Sprintf(
				"🔄 Cylinder reset: %d bullet(s) lurk in %d fresh chambers!", alive, remaining))
		}

		// If all chambers exhausted without a hit (only possible with 0 bullets
		// remaining after decrement), pick a random victim anyway.
		if remaining == 0 {
			time.Sleep(rrShotPause)
			victim = players[rand.Intn(len(players))]
			pType := randomRRPunishment()
			vc, verr := getClientByUid(victim)
			if verr == nil {
				victimName = vc.OOCName()
			}
			sendGlobalServerMessage(fmt.Sprintf(
				"😱 ALL CHAMBERS CLEARED... but wait — the gun fires on its own!\n"+
					"💥 MISFIRE! %v is claimed by fate! Punished with '%v'!",
				victimName, pType,
			))
			if verr == nil {
				vc.AddPunishment(pType, rrPunishDuration, "Russian Roulette: misfire")
				vc.SendServerMessage(fmt.Sprintf(
					"💀 The misfire got YOU! Punished with '%v' for %v.", pType, rrPunishDuration))
				addToBuffer(vc, "ROULETTE",
					fmt.Sprintf("Misfire victim in Russian Roulette; punished with %v", pType), false)
			}

			rr.mu.Lock()
			rr.gameActive = false
			rr.players = rr.players[:0]
			rr.lastEnd = time.Now().UTC()
			rr.mu.Unlock()
			return
		}
	}
}

// joinNames joins a string slice with commas and "and" before the last element.
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	out := make([]byte, 0, 64)
	for i, n := range names {
		if i > 0 {
			if i == len(names)-1 {
				out = append(out, []byte(", and ")...)
			} else {
				out = append(out, []byte(", ")...)
			}
		}
		out = append(out, n...)
	}
	return string(out)
}
