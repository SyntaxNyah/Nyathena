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
	rrJoinWindow     = 30 * time.Second // opt-in window duration
	rrCooldown       = 5 * time.Minute  // global delay between games
	rrChambers       = 6                // standard revolver chambers
	rrMinPlayers     = 2                // minimum players to start
	rrPunishDuration = 15 * time.Minute // how long the loser's punishment lasts
	rrShotPause      = 3 * time.Second  // dramatic pause between shots
	rrDoubleBulletP  = 15               // % chance of 2-bullet game (integer 0–100)
	rrRicochetP      = 5                // % chance the shot ricochets to a random player
)

// rrRules is broadcast when the join window opens.
const rrRules = `🔫 RUSSIAN ROULETTE — LOADING CHAMBERS... 🔫
Type /roulette join within 30 seconds to take a seat at the table.

📋 HOW TO PLAY:
• Players sit in a circle; the cylinder is spun once and the game begins.
• Each turn one chamber is fired — safe chambers go CLICK; the bullet goes BANG.
• The player hit receives a wild random punishment lasting 15 minutes.
• Rare chaos events: double-bullet loads and ricochets can strike at any time.
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
	PunishmentCensor,
	PunishmentSpaghetti,
	PunishmentTorment,
	PunishmentRng,
	PunishmentHaiku,
	PunishmentMonkey,
	PunishmentSnake,
	PunishmentDog,
	PunishmentCat,
	PunishmentBird,
	PunishmentCow,
	PunishmentFrog,
	PunishmentDuck,
	PunishmentHorse,
	PunishmentZoo,
	PunishmentBunny,
	PunishmentTsundere,
	PunishmentYandere,
	PunishmentKuudere,
	PunishmentDandere,
	PunishmentDeredere,
	PunishmentEmoticon,
	PunishmentWhisper,
}

func randomRRPunishment() PunishmentType {
	return rrPunishmentPool[rand.Intn(len(rrPunishmentPool))]
}

// ── Flavour text ─────────────────────────────────────────────────────────────

var rrGunNames = []string{
	"Old Betsy", "The Equaliser", "Lady Luck", "The Grim Revolver",
	"Madame Six", "The Iron Dice", "Widow-Maker", "The Philosopher's Gun",
}

var rrClickMessages = []string{
	"*click* — still breathing! 😅",
	"*click* — phew! That was close... 😰",
	"*click* — the cylinder spins on... 🌀",
	"*click* — SAFE! (for now) 😤",
	"*click* — the room exhales... 💨",
	"*CLICK* — luck favours you today! 🍀",
	"*click* — your heart skips a beat... 💓",
}

var rrBangMessages = []string{
	"💥 BANG! The hammer falls!",
	"💥 BANG!! The gun roars!",
	"🔥 BANG! Fate has spoken!",
	"💀 BANG! The cylinder found its mark!",
	"🎆 **BANG!!** The house always wins!",
	"☠️  B-B-BANG! Nobody saw that coming!",
	"🔫 BANG! The game is over!",
}

var rrTensionMessages = []string{
	"😨 The table is deathly quiet...",
	"🥵 Sweat drips from every brow...",
	"🤐 Nobody dares to breathe...",
	"👀 All eyes are on the barrel...",
	"⏳ The moment of truth approaches...",
}

// ── State ────────────────────────────────────────────────────────────────────

type rrState struct {
	mu         sync.Mutex
	joinActive bool
	gameActive bool
	players    []int     // UIDs in shuffled turn order
	remaining  int       // unfired chambers left
	bullets    int       // bullets remaining in cylinder
	turnIdx    int       // index of the current shooter
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

	bullets := 1
	if rand.Intn(100) < rrDoubleBulletP {
		bullets = 2
	}
	rr.joinActive = false
	rr.gameActive = true
	rr.remaining = rrChambers
	rr.bullets = bullets
	rr.turnIdx = 0
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

		// Tension flavour every other round.
		if i > 0 && i%2 == 0 {
			sendGlobalServerMessage(rrTensionMessages[rand.Intn(len(rrTensionMessages))])
			time.Sleep(time.Second)
		}

		// Probability of hitting a bullet this chamber.
		hit := rand.Intn(remaining) < alive

		// Ricochet: rare chance — redirect to a random OTHER player.
		victim := shooterUID
		victimName := shooterName
		if hit && rand.Intn(100) < rrRicochetP && len(players) > 1 {
			// Pick any player other than the shooter.
			j := rand.Intn(len(players) - 1)
			others := players
			k := 0
			for _, p := range others {
				if p != shooterUID {
					if k == j {
						victim = p
						break
					}
					k++
				}
			}
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
			if victim == shooterUID {
				if err == nil {
					shooter.AddPunishment(pType, rrPunishDuration, "Russian Roulette: shot")
					shooter.SendServerMessage(fmt.Sprintf(
						"💀 The bullet was yours! Punished with '%v' for %v. Good game!", pType, rrPunishDuration))
				}
			} else {
				if vc, verr := getClientByUid(victim); verr == nil {
					vc.AddPunishment(pType, rrPunishDuration, "Russian Roulette: ricochet")
					vc.SendServerMessage(fmt.Sprintf(
						"💀 The ricochet found YOU! Punished with '%v' for %v. Unlucky!", pType, rrPunishDuration))
				}
			}

			sendGlobalServerMessage(fmt.Sprintf(
				"☠️  ROULETTE OVER! %v drew the short straw and received '%v'. Better luck next life!",
				victimName, pType,
			))

			// Announce the survivors.
			if len(players) > 1 {
				survivors := make([]string, 0, len(players)-1)
				for _, p := range players {
					if p != victim {
						if c, cerr := getClientByUid(p); cerr == nil {
							survivors = append(survivors, c.OOCName())
						}
					}
				}
				if len(survivors) > 0 {
					sendGlobalServerMessage(fmt.Sprintf("🏆 Survivors: %v — well played!", joinNames(survivors)))
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

		// If all chambers exhausted without a hit (only possible with 0 bullets
		// remaining after decrement), pick a random victim anyway.
		if remaining == 0 {
			time.Sleep(rrShotPause)
			victimIdx := rand.Intn(len(players))
			victim = players[victimIdx]
			if vc, verr := getClientByUid(victim); verr == nil {
				victimName = vc.OOCName()
			}
			pType := randomRRPunishment()
			sendGlobalServerMessage(fmt.Sprintf(
				"😱 ALL CHAMBERS CLEARED... but wait — the gun fires on its own!\n"+
					"💥 MISFIRE! %v is claimed by fate! Punished with '%v'!",
				victimName, pType,
			))
			if vc, verr := getClientByUid(victim); verr == nil {
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
