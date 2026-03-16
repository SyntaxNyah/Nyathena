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

// ============================================================
// Mafia / Werewolf Social Deduction Minigame — State & Logic
// ============================================================
//
// Architecture mirrors the casino system: per-area state stored in a sync.Map.
// All game mutations are guarded by MafiaGame.mu.
//
// Phase flow:
//   Lobby → Day (discussion) → Night (actions) → Day → … → End
//
// Win conditions are checked at the end of each phase.

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// MafiaPhase represents the current game phase.
type MafiaPhase int

const (
	MafiaPhaseLobby         MafiaPhase = iota // waiting for players
	MafiaPhaseDayDiscussion                   // day: discussion / voting open
	MafiaPhaseNight                           // night: role actions
	MafiaPhaseEnded                           // game over
)

func (p MafiaPhase) String() string {
	switch p {
	case MafiaPhaseLobby:
		return "Lobby"
	case MafiaPhaseDayDiscussion:
		return "Day"
	case MafiaPhaseNight:
		return "Night"
	case MafiaPhaseEnded:
		return "Ended"
	default:
		return "Unknown"
	}
}

// RoleID identifies a Mafia role.
type RoleID int

const (
	RoleVillager   RoleID = iota // Town: no special ability; wins when all threats removed
	RoleMafia                    // Evil: kill 1 each night; wins when they equal/outnumber town
	RoleDetective                // Town: investigate 1 per night to learn alignment
	RoleDoctor                   // Town: protect 1 per night from night kills
	RoleJester                   // Neutral: wins ONLY if lynched during a day vote
	RoleSheriff                  // Town: shoot 1 per game (kills if Mafia, self-kills if wrong)
	RoleArsonist                 // Chaos: douse per night, ignite all doused simultaneously
	RoleSerialKiller             // Chaos: kills 1 per night; immune to single Mafia kill; wins alone
	RoleShapeshifter             // Mafia-aligned: copies another role's result on investigation
	RoleWitch                    // Neutral: redirect a player's night action to a different target
	RoleLawyer                   // Neutral: picks 1 client on game start; wins if client survives
	RoleBodyguard                // Town: protect 1; if attacker kills, bodyguard and attacker both die
	RoleVigilante                // Town: kill 1 per night; dies of guilt if target is Town-aligned
	RoleMayor                    // Town: reveal publicly; all future votes count double
	RoleEscort                   // Town: roleblock 1 per night; their action has no effect
	RoleGodfather                // Mafia: appears Town to Detective; immune to Sheriff shot (Sheriff dies)
	RoleSurvivor                 // Neutral: no ability; wins by surviving to game end
)

// roleInfo holds human-readable metadata for each role.
type roleInfo struct {
	Name      string
	Team      string // "Town" | "Mafia" | "Neutral" | "Chaos"
	Alignment string // "Good" | "Evil" | "Neutral"
	Desc      string
	WinCond   string
	Ability   string
}

var roleInfoMap = map[RoleID]roleInfo{
	RoleVillager: {
		Name: "Villager", Team: "Town", Alignment: "Good",
		Desc:    "An ordinary member of the town with no special powers.",
		WinCond: "Eliminate all threats to the town (Mafia, Serial Killer, Arsonist).",
		Ability: "None.",
	},
	RoleMafia: {
		Name: "Mafia", Team: "Mafia", Alignment: "Evil",
		Desc:    "A sinister faction working in secret to eliminate the town.",
		WinCond: "The Mafia wins when they equal or outnumber all remaining town-aligned players.",
		Ability: "Each night, the Mafia collectively choose one player to eliminate.",
	},
	RoleDetective: {
		Name: "Detective", Team: "Town", Alignment: "Good",
		Desc:    "A sharp-eyed investigator hunting for the truth.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Each night, investigate one player and learn their alignment (Town / Mafia / Neutral / Chaos).",
	},
	RoleDoctor: {
		Name: "Doctor", Team: "Town", Alignment: "Good",
		Desc:    "A skilled healer protecting the innocent.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Each night, choose one player to protect from elimination. Cannot self-protect two nights in a row.",
	},
	RoleJester: {
		Name: "Jester", Team: "Neutral", Alignment: "Neutral",
		Desc:    "A chaotic trickster who WANTS to be lynched.",
		WinCond: "Be voted out and lynched by the town during a day phase.",
		Ability: "None. Causes confusion through false claims and erratic behaviour.",
	},
	RoleSheriff: {
		Name: "Sheriff", Team: "Town", Alignment: "Good",
		Desc:    "A gunslinger with one silver bullet.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Once per game during the day, shoot a player. If they are Mafia-aligned the shot is fatal; if innocent the Sheriff is killed instead.",
	},
	RoleArsonist: {
		Name: "Arsonist", Team: "Chaos", Alignment: "Neutral",
		Desc:    "A pyromaniac lurking in the shadows.",
		WinCond: "Be the last player standing after igniting everyone.",
		Ability: "Each night: douse a player (invisible). Once ready, use 'ignite' to kill ALL doused players at once.",
	},
	RoleSerialKiller: {
		Name: "Serial Killer", Team: "Chaos", Alignment: "Neutral",
		Desc:    "A lone predator who kills for sport.",
		WinCond: "Be the last player standing.",
		Ability: "Each night, eliminate one player. Immune to a single Mafia kill attempt.",
	},
	RoleShapeshifter: {
		Name: "Shapeshifter", Team: "Mafia", Alignment: "Evil",
		Desc:    "A Mafia member who can mimic any role.",
		WinCond: "Same as Mafia.",
		Ability: "Each night, copy another player; investigation reveals the copied role's alignment instead.",
	},
	RoleWitch: {
		Name: "Witch", Team: "Neutral", Alignment: "Neutral",
		Desc:    "A mysterious spellcaster who pulls invisible strings.",
		WinCond: "Survive until the game ends regardless of which faction wins.",
		Ability: "Once per night, redirect a player's action to a new target.",
	},
	RoleLawyer: {
		Name: "Lawyer", Team: "Neutral", Alignment: "Neutral",
		Desc:    "A cunning attorney defending one secret client.",
		WinCond: "Your chosen client must still be alive when the game ends.",
		Ability: "On game start you are secretly assigned a client. Win if that client is alive at the end.",
	},
	RoleBodyguard: {
		Name: "Bodyguard", Team: "Town", Alignment: "Good",
		Desc:    "A selfless protector willing to take a bullet.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Each night, protect one player. If an attacker targets that player, both the attacker and you die.",
	},
	RoleVigilante: {
		Name: "Vigilante", Team: "Town", Alignment: "Good",
		Desc:    "A lone hero who takes justice into their own hands.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Once per game at night, execute a player. If the target is Town-aligned, you die of guilt the following morning.",
	},
	RoleMayor: {
		Name: "Mayor", Team: "Town", Alignment: "Good",
		Desc:    "The respected leader whose word carries extra weight.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Once per game during the Day, use /mafia act reveal to publicly announce yourself. From then on every lynch vote you cast counts double.",
	},
	RoleEscort: {
		Name: "Escort", Team: "Town", Alignment: "Good",
		Desc:    "A charming distraction who keeps suspicious players occupied.",
		WinCond: "Eliminate all threats to the town.",
		Ability: "Each night, roleblock one player: their night action is cancelled for that night.",
	},
	RoleGodfather: {
		Name: "Godfather", Team: "Mafia", Alignment: "Evil",
		Desc:    "The Mafia's undisputed boss — clean hands, cold mind.",
		WinCond: "Same as Mafia.",
		Ability: "Appears Town-aligned to Detective. If the Sheriff shoots you, the bullet bounces — the Sheriff dies instead.",
	},
	RoleSurvivor: {
		Name: "Survivor", Team: "Neutral", Alignment: "Neutral",
		Desc:    "A bystander determined to make it out alive no matter what.",
		WinCond: "Survive until the game ends, regardless of which faction wins.",
		Ability: "None. Wins alongside whichever faction wins, as long as you are still alive.",
	},
}

// MafiaPlayer represents a player in the Mafia game.
type MafiaPlayer struct {
	Client       *Client
	Role         RoleID
	Alive        bool
	Doused       bool   // for Arsonist tracking
	NightAction  string // submitted night action primary target
	NightAction2 string // secondary target (Witch only)
	VoteTarget   string // OOCName of vote target during day
	Disconnected bool

	// Extended per-player state
	LastWill        string // message revealed when this player dies
	MayorRevealed   bool   // Mayor has publicly announced themselves
	VigilanteGuilty bool   // Vigilante killed a Town-aligned player; they die next morning
	VigilanteUsed   bool   // Vigilante has spent their one shot
}

func (p *MafiaPlayer) Name() string {
	if p.Client != nil {
		return p.Client.OOCName()
	}
	return "(disconnected)"
}

// MafiaGame holds the full state of one Mafia game running in an area.
type MafiaGame struct {
	mu sync.Mutex

	Area    *area.Area
	Phase   MafiaPhase
	Day     int // current day number (0 during first night)
	Players []*MafiaPlayer

	// Per-night transient tracking
	MafiaKillTarget string // agreed Mafia kill target name
	SKImmune        bool   // Serial Killer has used their immunity once

	// Phase auto-timer
	phaseTimer *time.Timer
	PhaseSecs  int // 0 = no auto-timer

	// Lawyer's assigned client name
	LawyerClientName string

	// Arsonist: track doused player names
	DousedNames map[string]bool

	// Bodyguard: bodyguard name → protected player name
	BodyguardProtects map[string]string

	// Sheriff: whether the shot has been used
	SheriffUsed bool

	// Graveyard: chronological log of all player deaths
	Graveyard []GraveyardEntry
}

// GraveyardEntry records one player's elimination.
type GraveyardEntry struct {
	Name     string
	Role     RoleID
	Cause    string // e.g. "Lynched", "Killed at night (Mafia)", "Shot (Sheriff)"
	Day      int
	LastWill string // copied from MafiaPlayer.LastWill at time of death
}

// mafiaStates stores per-area game state, mirroring casinoStates.
var mafiaStates sync.Map // key: *area.Area, value: *MafiaGame

// getMafiaGame returns the current MafiaGame for an area, or nil if none exists.
func getMafiaGame(a *area.Area) *MafiaGame {
	v, ok := mafiaStates.Load(a)
	if !ok {
		return nil
	}
	return v.(*MafiaGame)
}

// newMafiaGame creates a fresh MafiaGame for the area and stores it.
func newMafiaGame(a *area.Area) *MafiaGame {
	g := &MafiaGame{
		Area:              a,
		Phase:             MafiaPhaseLobby,
		DousedNames:       make(map[string]bool),
		BodyguardProtects: make(map[string]string),
	}
	mafiaStates.Store(a, g)
	return g
}

// deleteMafiaGame removes the game state for an area.
func deleteMafiaGame(a *area.Area) {
	mafiaStates.Delete(a)
}

// addToGraveyard records a death. Must be called with g.mu held.
func (g *MafiaGame) addToGraveyard(p *MafiaPlayer, cause string) {
	g.Graveyard = append(g.Graveyard, GraveyardEntry{
		Name:     p.Name(),
		Role:     p.Role,
		Cause:    cause,
		Day:      g.Day,
		LastWill: p.LastWill,
	})
}

// deathAnnounce builds the public death line (includes last will if set).
func deathAnnounce(name, lastWill string) string {
	if lastWill != "" {
		return fmt.Sprintf("💀 %v was found dead!\n📜 Last Will of %v: %v", name, name, lastWill)
	}
	return fmt.Sprintf("💀 %v was found dead!", name)
}

// ============================================================
// Player helpers (call with g.mu held)
// ============================================================

func (g *MafiaGame) findPlayer(name string) *MafiaPlayer {
	nameLower := strings.ToLower(name)
	for _, p := range g.Players {
		if strings.ToLower(p.Name()) == nameLower {
			return p
		}
	}
	return nil
}

func (g *MafiaGame) findPlayerByClient(c *Client) *MafiaPlayer {
	for _, p := range g.Players {
		if p.Client == c {
			return p
		}
	}
	return nil
}

func (g *MafiaGame) alivePlayers() []*MafiaPlayer {
	var alive []*MafiaPlayer
	for _, p := range g.Players {
		if p.Alive {
			alive = append(alive, p)
		}
	}
	return alive
}

func (g *MafiaGame) aliveCount() int { return len(g.alivePlayers()) }

func (g *MafiaGame) aliveCountByTeam(team string) int {
	n := 0
	for _, p := range g.Players {
		if p.Alive && roleInfoMap[p.Role].Team == team {
			n++
		}
	}
	return n
}

func (g *MafiaGame) aliveCountByRole(r RoleID) int {
	n := 0
	for _, p := range g.Players {
		if p.Alive && p.Role == r {
			n++
		}
	}
	return n
}

// broadcastToGame sends a server message to all area clients.
func (g *MafiaGame) broadcastToGame(msg string) {
	sendAreaServerMessage(g.Area, "🎭 [Mafia] "+msg)
}

// privateMsg sends a private OOC message directly to one player.
func (g *MafiaGame) privateMsg(p *MafiaPlayer, msg string) {
	if p.Client != nil && !p.Disconnected {
		p.Client.SendServerMessage("🎭 [Mafia/Private] " + msg)
	}
}

// ============================================================
// Role assignment
// ============================================================

// defaultRolePool returns a balanced role distribution for n players.
func defaultRolePool(n int) []RoleID {
	switch {
	case n < 4:
		roles := []RoleID{RoleMafia}
		for i := 1; i < n; i++ {
			roles = append(roles, RoleVillager)
		}
		return roles
	case n == 4:
		return []RoleID{RoleMafia, RoleDetective, RoleDoctor, RoleVillager}
	case n == 5:
		return []RoleID{RoleMafia, RoleDetective, RoleDoctor, RoleVillager, RoleJester}
	case n == 6:
		return []RoleID{RoleMafia, RoleMafia, RoleDetective, RoleDoctor, RoleVillager, RoleJester}
	case n == 7:
		return []RoleID{RoleMafia, RoleMafia, RoleDetective, RoleDoctor, RoleSheriff, RoleVillager, RoleJester}
	case n == 8:
		return []RoleID{RoleMafia, RoleMafia, RoleDetective, RoleDoctor, RoleSheriff, RoleVigilante, RoleVillager, RoleJester}
	case n == 9:
		return []RoleID{RoleMafia, RoleMafia, RoleDetective, RoleDoctor, RoleSheriff, RoleVigilante, RoleEscort, RoleVillager, RoleJester}
	case n == 10:
		return []RoleID{RoleMafia, RoleMafia, RoleGodfather, RoleDetective, RoleDoctor, RoleSheriff, RoleVigilante, RoleEscort, RoleBodyguard, RoleJester}
	case n == 11:
		return []RoleID{RoleMafia, RoleMafia, RoleGodfather, RoleDetective, RoleDoctor, RoleSheriff, RoleVigilante, RoleEscort, RoleBodyguard, RoleJester, RoleArsonist}
	case n <= 13:
		pool := []RoleID{RoleMafia, RoleMafia, RoleGodfather, RoleShapeshifter, RoleDetective, RoleDoctor, RoleSheriff, RoleVigilante, RoleEscort, RoleBodyguard, RoleJester, RoleArsonist, RoleSerialKiller}
		return pool[:n]
	default:
		mafiaCount := n / 4
		if mafiaCount < 2 {
			mafiaCount = 2
		}
		roles := make([]RoleID, 0, n)
		roles = append(roles, RoleMafia)
		roles = append(roles, RoleGodfather)
		for i := 2; i < mafiaCount; i++ {
			roles = append(roles, RoleMafia)
		}
		specials := []RoleID{
			RoleDetective, RoleDoctor, RoleSheriff, RoleVigilante, RoleEscort, RoleBodyguard,
			RoleJester, RoleArsonist, RoleSerialKiller, RoleWitch, RoleLawyer, RoleShapeshifter,
			RoleMayor, RoleSurvivor,
		}
		for _, s := range specials {
			if len(roles) >= n {
				break
			}
			roles = append(roles, s)
		}
		for len(roles) < n {
			roles = append(roles, RoleVillager)
		}
		return roles
	}
}

// assignRoles shuffles the role pool and assigns roles to players.
// Must be called with g.mu held.
func (g *MafiaGame) assignRoles() {
	pool := defaultRolePool(len(g.Players))
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for i, p := range g.Players {
		p.Role = pool[i]
		p.Alive = true
		p.Doused = false
		p.NightAction = ""
		p.VoteTarget = ""
	}

	// Assign Lawyer's client
	g.LawyerClientName = ""
	for _, p := range g.Players {
		if p.Role == RoleLawyer {
			candidates := make([]*MafiaPlayer, 0, len(g.Players)-1)
			for _, pp := range g.Players {
				if pp != p {
					candidates = append(candidates, pp)
				}
			}
			if len(candidates) > 0 {
				c := candidates[rand.Intn(len(candidates))]
				g.LawyerClientName = c.Name()
				g.privateMsg(p, fmt.Sprintf("Your client is: %v. They must survive for you to win!", c.Name()))
			}
			break
		}
	}

	// Notify Mafia members of each other
	var mafiaNames []string
	for _, p := range g.Players {
		if roleInfoMap[p.Role].Team == "Mafia" {
			mafiaNames = append(mafiaNames, p.Name())
		}
	}
	if len(mafiaNames) > 0 {
		mafiaList := strings.Join(mafiaNames, ", ")
		for _, p := range g.Players {
			if roleInfoMap[p.Role].Team == "Mafia" {
				g.privateMsg(p, fmt.Sprintf("Your Mafia team: %v", mafiaList))
			}
		}
	}
}

// ============================================================
// Win condition detection (call with g.mu held)
// ============================================================

// checkWin evaluates win conditions.
// Returns ("", false) if the game continues, or (description, true) when a faction wins.
func (g *MafiaGame) checkWin() (string, bool) {
	alive := g.alivePlayers()
	if len(alive) == 0 {
		return "Nobody — everyone is dead!", true
	}

	// Count by team/role
	mafiaAlive := g.aliveCountByTeam("Mafia")
	skAlive := g.aliveCountByRole(RoleSerialKiller)
	arsonistAlive := g.aliveCountByRole(RoleArsonist)

	// Count evil threats: Mafia (includes Shapeshifter via Team="Mafia"), Serial Killer, Arsonist.
	threats := mafiaAlive + skAlive + arsonistAlive

	// Count town-aligned
	townAlive := 0
	for _, p := range alive {
		if roleInfoMap[p.Role].Alignment == "Good" {
			townAlive++
		}
	}

	// Town win: all threats gone
	if threats == 0 {
		return "Town wins! All threats have been eliminated.", true
	}

	// Mafia win: equal or outnumber town
	if mafiaAlive > 0 && mafiaAlive >= townAlive {
		names := make([]string, 0)
		for _, p := range alive {
			if roleInfoMap[p.Role].Team == "Mafia" {
				names = append(names, p.Name())
			}
		}
		return fmt.Sprintf("Mafia wins! (%v)", strings.Join(names, ", ")), true
	}

	// Serial Killer solo win: only SK (+ neutral bystanders) alive
	if skAlive > 0 {
		nonNeutral := 0
		for _, p := range alive {
			if p.Role != RoleSerialKiller && roleInfoMap[p.Role].Alignment != "Neutral" {
				nonNeutral++
			}
		}
		if nonNeutral == 0 {
			for _, p := range alive {
				if p.Role == RoleSerialKiller {
					return fmt.Sprintf("Serial Killer (%v) wins — last one standing!", p.Name()), true
				}
			}
		}
	}

	// Arsonist solo win: only Arsonist (+ neutral bystanders) alive
	if arsonistAlive > 0 {
		nonNeutral := 0
		for _, p := range alive {
			if p.Role != RoleArsonist && roleInfoMap[p.Role].Alignment != "Neutral" {
				nonNeutral++
			}
		}
		if nonNeutral == 0 {
			for _, p := range alive {
				if p.Role == RoleArsonist {
					return fmt.Sprintf("Arsonist (%v) wins — reduced everything to ashes!", p.Name()), true
				}
			}
		}
	}

	return "", false
}

// ============================================================
// Phase transitions
// ============================================================

// startDay transitions to a new Day Discussion phase.
func (g *MafiaGame) startDay(dayNum int) {
	g.mu.Lock()
	if g.Phase == MafiaPhaseEnded {
		g.mu.Unlock()
		return
	}
	g.Phase = MafiaPhaseDayDiscussion
	g.Day = dayNum
	for _, p := range g.Players {
		p.NightAction = ""
		p.NightAction2 = ""
		p.VoteTarget = ""
	}
	g.MafiaKillTarget = ""
	g.BodyguardProtects = make(map[string]string)
	phaseSecs := g.PhaseSecs

	// Guilty Vigilante dies of grief this morning
	var guiltyVig *MafiaPlayer
	for _, p := range g.Players {
		if p.Alive && p.VigilanteGuilty {
			guiltyVig = p
			p.Alive = false
			p.VigilanteGuilty = false
			g.addToGraveyard(p, "Died of guilt (Vigilante)")
			break
		}
	}
	g.mu.Unlock()

	if guiltyVig != nil {
		msg := fmt.Sprintf("💔 %v, the Vigilante, could not bear the guilt of killing an innocent and took their own life.", guiltyVig.Name())
		if guiltyVig.LastWill != "" {
			msg += "\n📜 Last Will: " + guiltyVig.LastWill
		}
		g.broadcastToGame(msg)
		g.mu.Lock()
		winner, won := g.checkWin()
		g.mu.Unlock()
		if won {
			g.endGame(winner)
			return
		}
	}

	g.broadcastToGame(fmt.Sprintf("☀️  Day %d begins! Discuss and use /mafia vote <name>. Type /mafia skip to pass the vote.", dayNum))
	g.broadcastToGame("💡 Tips: /mafia tally (vote standings) | /mafia will <text> (set last will) | /mafia whisper <player> <msg>")

	if phaseSecs > 0 {
		g.schedulePhaseEnd(time.Duration(phaseSecs)*time.Second, func() {
			g.resolveDay()
		})
	}
}

// startNight transitions to the Night phase.
func (g *MafiaGame) startNight() {
	g.mu.Lock()
	if g.Phase == MafiaPhaseEnded {
		g.mu.Unlock()
		return
	}
	g.Phase = MafiaPhaseNight
	for _, p := range g.Players {
		p.NightAction = ""
		p.NightAction2 = ""
	}
	phaseSecs := g.PhaseSecs
	g.mu.Unlock()

	g.broadcastToGame("🌙 Night falls… Submit your actions privately using /mafia act <target>.")
	g.broadcastToGame("Arsonist: /mafia act douse <target> OR /mafia act ignite | Witch: /mafia act <player> <newtarget>")

	// Role-specific private prompts
	g.mu.Lock()
	for _, p := range g.Players {
		if !p.Alive {
			continue
		}
		switch p.Role {
		case RoleMafia, RoleShapeshifter:
			g.privateMsg(p, "Night: /mafia act <target> — choose the Mafia kill target. Coordinate with your team!")
		case RoleDetective:
			g.privateMsg(p, "Night: /mafia act <target> — investigate a player's alignment.")
		case RoleDoctor:
			g.privateMsg(p, "Night: /mafia act <target> — protect a player from being killed tonight.")
		case RoleBodyguard:
			g.privateMsg(p, "Night: /mafia act <target> — guard someone. If attacked, you and the attacker both die!")
		case RoleVigilante:
			if !p.VigilanteUsed {
				g.privateMsg(p, "Night: /mafia act <target> — execute a player. ⚠️ If they are Town-aligned, YOU die of guilt tomorrow morning!")
			} else {
				g.privateMsg(p, "You have already used your Vigilante shot.")
			}
		case RoleEscort:
			g.privateMsg(p, "Night: /mafia act <target> — roleblock someone, cancelling their night action.")
		case RoleArsonist:
			g.privateMsg(p, "Night: /mafia act douse <target> to pour gasoline, OR /mafia act ignite to burn all doused targets.")
		case RoleWitch:
			g.privateMsg(p, "Night: /mafia act <player> <newtarget> — redirect that player's action to a new target.")
		case RoleSerialKiller:
			g.privateMsg(p, "Night: /mafia act <target> — silently eliminate a player.")
		}
	}
	g.mu.Unlock()

	if phaseSecs > 0 {
		g.schedulePhaseEnd(time.Duration(phaseSecs)*time.Second, func() {
			g.resolveNight()
		})
	}
}

// schedulePhaseEnd sets a timer that calls fn after d; cancels any existing timer.
func (g *MafiaGame) schedulePhaseEnd(d time.Duration, fn func()) {
	g.mu.Lock()
	if g.phaseTimer != nil {
		g.phaseTimer.Stop()
	}
	g.phaseTimer = time.AfterFunc(d, fn)
	g.mu.Unlock()
}

func (g *MafiaGame) cancelTimer() {
	g.mu.Lock()
	if g.phaseTimer != nil {
		g.phaseTimer.Stop()
		g.phaseTimer = nil
	}
	g.mu.Unlock()
}

// ============================================================
// Day resolution: lynch vote
// ============================================================

// resolveDay tallies votes, announces the result, and transitions to Night.
func (g *MafiaGame) resolveDay() {
	g.mu.Lock()
	if g.Phase != MafiaPhaseDayDiscussion {
		g.mu.Unlock()
		return
	}

	// Tally votes accounting for Mayor double-vote
	tally := make(map[string]int)
	for _, p := range g.Players {
		if p.Alive && p.VoteTarget != "" {
			weight := 1
			if p.Role == RoleMayor && p.MayorRevealed {
				weight = 2
			}
			tally[strings.ToLower(p.VoteTarget)] += weight
		}
	}

	aliveCount := g.aliveCount()
	majority := aliveCount/2 + 1
	var lynchKey string
	var maxVotes int
	for key, votes := range tally {
		if votes > maxVotes {
			maxVotes = votes
			lynchKey = key
		}
	}

	// Find the actual player for the winning vote key
	var lynchTarget *MafiaPlayer
	if lynchKey != "" && maxVotes >= majority {
		for _, p := range g.Players {
			if p.Alive && strings.ToLower(p.Name()) == lynchKey {
				lynchTarget = p
				break
			}
		}
	}
	g.mu.Unlock()

	if lynchTarget == nil {
		g.broadcastToGame("🗳️  No majority reached — nobody is lynched today.")
	} else {
		g.mu.Lock()
		lynchTarget.Alive = false
		role := lynchTarget.Role
		info := roleInfoMap[role]
		lastWill := lynchTarget.LastWill
		g.addToGraveyard(lynchTarget, "Lynched")
		g.mu.Unlock()

		msg := fmt.Sprintf("⚖️  %v has been voted out! They were the %v (%v team).", lynchTarget.Name(), info.Name, info.Team)
		if lastWill != "" {
			msg += fmt.Sprintf("\n📜 Last Will of %v: %v", lynchTarget.Name(), lastWill)
		}
		g.broadcastToGame(msg)

		// Jester win
		if role == RoleJester {
			g.broadcastToGame(fmt.Sprintf("🃏 THE JESTER WINS! %v wanted to be lynched all along!", lynchTarget.Name()))
			g.endGame(fmt.Sprintf("Jester (%v) wins — they got what they wanted!", lynchTarget.Name()))
			return
		}

		g.mu.Lock()
		winner, won := g.checkWin()
		g.mu.Unlock()
		if won {
			g.endGame(winner)
			return
		}
	}

	g.startNight()
}

// ============================================================
// Night resolution
// ============================================================

// resolveNight processes all night actions and transitions to the next Day.
func (g *MafiaGame) resolveNight() {
	g.mu.Lock()
	if g.Phase != MafiaPhaseNight {
		g.mu.Unlock()
		return
	}

	// Snapshot player list for processing (slice of pointers to live structs)
	players := make([]*MafiaPlayer, len(g.Players))
	copy(players, g.Players)

	// --- Witch redirect map: original target name (lower) → new target name (lower) ---
	redirect := make(map[string]string)
	for _, p := range players {
		if !p.Alive || p.Role != RoleWitch {
			continue
		}
		if p.NightAction != "" && p.NightAction2 != "" {
			redirect[strings.ToLower(p.NightAction)] = strings.ToLower(p.NightAction2)
		}
	}

	applyRedirect := func(name string) string {
		lower := strings.ToLower(name)
		if newT, ok := redirect[lower]; ok {
			return newT
		}
		return lower
	}

	findAlive := func(name string) *MafiaPlayer {
		lower := strings.ToLower(name)
		for _, p := range players {
			if p.Alive && strings.ToLower(p.Name()) == lower {
				return p
			}
		}
		return nil
	}

	// --- Escort roleblocks (applied before defensive/offensive actions) ---
	roleblocked := make(map[string]bool) // lower player name → roleblocked
	for _, p := range players {
		if !p.Alive || p.Role != RoleEscort || p.NightAction == "" {
			continue
		}
		finalName := applyRedirect(p.NightAction)
		t := findAlive(finalName)
		if t == nil {
			t = findAlive(p.NightAction)
		}
		if t != nil {
			roleblocked[strings.ToLower(t.Name())] = true
			g.privateMsg(p, fmt.Sprintf("You kept %v busy all night — their action was blocked!", t.Name()))
		}
	}

	// --- Collect protective actions (skip if roleblocked) ---
	// doctorProtects: lowercase player name → true
	doctorProtects := make(map[string]bool)
	// bgProtects: lowercase protected name → bodyguard player
	bgProtects := make(map[string]*MafiaPlayer)

	for _, p := range players {
		if !p.Alive || roleblocked[strings.ToLower(p.Name())] {
			continue
		}
		target := strings.ToLower(p.NightAction)
		switch p.Role {
		case RoleDoctor:
			if target != "" {
				doctorProtects[target] = true
			}
		case RoleBodyguard:
			if target != "" {
				bgProtects[target] = p
			}
		}
	}

	type pendingKill struct {
		target   *MafiaPlayer
		by       string
		byPlayer *MafiaPlayer // nil for faction kills
	}
	var kills []pendingKill
	var msgs []string

	// --- Mafia kill (team consensus; not blocked by Escort) ---
	mafiaTarget := g.MafiaKillTarget
	if mafiaTarget == "" {
		// Auto-pick a random non-Mafia alive player if no consensus was submitted
		var candidates []*MafiaPlayer
		for _, p := range players {
			if p.Alive && roleInfoMap[p.Role].Team != "Mafia" {
				candidates = append(candidates, p)
			}
		}
		if len(candidates) > 0 {
			mafiaTarget = candidates[rand.Intn(len(candidates))].Name()
		}
	}
	if mafiaTarget != "" {
		finalName := applyRedirect(mafiaTarget)
		t := findAlive(finalName)
		if t == nil {
			t = findAlive(mafiaTarget)
		}
		if t != nil {
			tLower := strings.ToLower(t.Name())
			if doctorProtects[tLower] {
				msgs = append(msgs, "🏥 Someone was healed last night and survived!")
			} else if bg, ok := bgProtects[tLower]; ok && bg.Alive {
				// Bodyguard intercepts; both bodyguard and an attacker die
				bg.Alive = false
				g.addToGraveyard(bg, "Killed protecting (Bodyguard)")
				msgs = append(msgs, deathAnnounce(bg.Name(), bg.LastWill))
				msgs[len(msgs)-1] = "🛡️  " + bg.Name() + " died protecting someone last night!\n" + msgs[len(msgs)-1]
				for _, mp := range players {
					if mp.Alive && roleInfoMap[mp.Role].Team == "Mafia" {
						mp.Alive = false
						g.addToGraveyard(mp, "Killed by Bodyguard")
						msgs = append(msgs, fmt.Sprintf("⚔️  %v was killed by the Bodyguard!", mp.Name()))
						if mp.LastWill != "" {
							msgs[len(msgs)-1] += "\n📜 Last Will of " + mp.Name() + ": " + mp.LastWill
						}
						break
					}
				}
			} else {
				kills = append(kills, pendingKill{t, "Mafia", nil})
			}
		}
	}

	// --- Serial Killer kill (skip if roleblocked) ---
	for _, p := range players {
		if !p.Alive || p.Role != RoleSerialKiller || p.NightAction == "" {
			continue
		}
		if roleblocked[strings.ToLower(p.Name())] {
			g.privateMsg(p, "You were kept busy last night — your action was blocked!")
			continue
		}
		finalName := applyRedirect(p.NightAction)
		t := findAlive(finalName)
		if t == nil {
			t = findAlive(p.NightAction)
		}
		if t != nil {
			tLower := strings.ToLower(t.Name())
			if doctorProtects[tLower] {
				msgs = append(msgs, "🏥 Someone survived an attack last night!")
			} else {
				kills = append(kills, pendingKill{t, "Serial Killer", p})
			}
		}
	}

	// --- Arsonist: ignite or douse (skip if roleblocked) ---
	for _, p := range players {
		if !p.Alive || p.Role != RoleArsonist || p.NightAction == "" {
			continue
		}
		if roleblocked[strings.ToLower(p.Name())] {
			g.privateMsg(p, "You were kept busy last night — your action was blocked!")
			continue
		}
		action := p.NightAction
		actionLower := strings.ToLower(action)
		if actionLower == "ignite" {
			var ignited []string
			for _, dp := range players {
				if dp.Alive && g.DousedNames[strings.ToLower(dp.Name())] {
					kills = append(kills, pendingKill{dp, "Arsonist", p})
					ignited = append(ignited, dp.Name())
				}
			}
			if len(ignited) > 0 {
				msgs = append(msgs, fmt.Sprintf("🔥 The Arsonist ignites! %v go up in flames!", strings.Join(ignited, ", ")))
				g.DousedNames = make(map[string]bool)
			} else {
				msgs = append(msgs, "🔥 The Arsonist tried to ignite but nobody was doused!")
			}
		} else if strings.HasPrefix(actionLower, "douse ") {
			targetName := action[6:]
			finalName := applyRedirect(targetName)
			t := findAlive(finalName)
			if t == nil {
				t = findAlive(targetName)
			}
			if t != nil {
				g.DousedNames[strings.ToLower(t.Name())] = true
				g.privateMsg(p, fmt.Sprintf("You doused %v. Use /mafia act ignite when ready!", t.Name()))
			}
		}
	}

	// --- Vigilante kill (skip if roleblocked or already used) ---
	for _, p := range players {
		if !p.Alive || p.Role != RoleVigilante || p.NightAction == "" || p.VigilanteUsed {
			continue
		}
		if roleblocked[strings.ToLower(p.Name())] {
			g.privateMsg(p, "You were kept busy last night — your action was blocked!")
			continue
		}
		finalName := applyRedirect(p.NightAction)
		t := findAlive(finalName)
		if t == nil {
			t = findAlive(p.NightAction)
		}
		if t != nil {
			tLower := strings.ToLower(t.Name())
			if doctorProtects[tLower] {
				msgs = append(msgs, "🏥 Someone survived a Vigilante attack thanks to a Doctor!")
			} else {
				kills = append(kills, pendingKill{t, "Vigilante", p})
			}
		}
	}

	// --- Apply kills (deduplicated, SK immunity, guilt tracking) ---
	alreadyKilled := make(map[string]bool)
	for _, k := range kills {
		if !k.target.Alive {
			continue
		}
		kLower := strings.ToLower(k.target.Name())
		if alreadyKilled[kLower] {
			continue
		}
		// Serial Killer is immune to the first Mafia kill attempt
		if k.target.Role == RoleSerialKiller && k.by == "Mafia" && !g.SKImmune {
			g.SKImmune = true
			msgs = append(msgs, "⚔️  Someone survived a night attack thanks to their immunity!")
			continue
		}
		k.target.Alive = false
		alreadyKilled[kLower] = true
		g.addToGraveyard(k.target, "Killed at night ("+k.by+")")
		msgs = append(msgs, deathAnnounce(k.target.Name(), k.target.LastWill))

		// Vigilante guilt: if they killed a Town-aligned player
		if k.byPlayer != nil && k.byPlayer.Role == RoleVigilante {
			k.byPlayer.VigilanteUsed = true
			if roleInfoMap[k.target.Role].Alignment == "Good" {
				k.byPlayer.VigilanteGuilty = true
			}
		}
	}

	// --- Detective results (private, skip if roleblocked) ---
	for _, p := range players {
		if !p.Alive || p.Role != RoleDetective || p.NightAction == "" {
			continue
		}
		if roleblocked[strings.ToLower(p.Name())] {
			g.privateMsg(p, "You were kept busy last night — your investigation was blocked!")
			continue
		}
		finalName := applyRedirect(p.NightAction)
		t := findAlive(finalName)
		if t == nil {
			// May have been killed this night; search all
			tName := strings.ToLower(finalName)
			for _, pp := range players {
				if strings.ToLower(pp.Name()) == tName {
					t = pp
					break
				}
			}
		}
		if t != nil {
			alignment := roleInfoMap[t.Role].Alignment
			// Shapeshifter and Godfather both appear Town to Detective
			if t.Role == RoleShapeshifter || t.Role == RoleGodfather {
				alignment = "Good"
			}
			g.privateMsg(p, fmt.Sprintf("🔍 Investigation: %v is %v-aligned.", t.Name(), alignment))
		}
	}

	nextDay := g.Day + 1
	g.mu.Unlock()

	for _, m := range msgs {
		g.broadcastToGame(m)
	}

	// Check win after all kills are applied
	g.mu.Lock()
	winner, won := g.checkWin()
	g.mu.Unlock()
	if won {
		g.endGame(winner)
		return
	}

	g.startDay(nextDay)
}

// ============================================================
// Game end
// ============================================================

func (g *MafiaGame) endGame(reason string) {
	g.mu.Lock()
	if g.Phase == MafiaPhaseEnded {
		g.mu.Unlock()
		return
	}
	g.Phase = MafiaPhaseEnded
	if g.phaseTimer != nil {
		g.phaseTimer.Stop()
		g.phaseTimer = nil
	}

	var sb strings.Builder
	sb.WriteString("\n🎭 ── GAME OVER ──\n")
	sb.WriteString(fmt.Sprintf("Result: %v\n\n", reason))

	// Announce Survivor wins (any Survivor still alive wins alongside the main winner)
	var survivors []string
	for _, p := range g.Players {
		if p.Role == RoleSurvivor && p.Alive {
			survivors = append(survivors, p.Name())
		}
	}
	if len(survivors) > 0 {
		sb.WriteString(fmt.Sprintf("🛡️  Survivor(s) also win: %v\n\n", strings.Join(survivors, ", ")))
	}

	sb.WriteString("Role reveal:\n")
	for _, p := range g.Players {
		info := roleInfoMap[p.Role]
		status := "💀 dead"
		if p.Alive {
			status = "✅ alive"
		}
		sb.WriteString(fmt.Sprintf("  %v — %v (%v) [%v]", p.Name(), info.Name, info.Team, status))
		if p.LastWill != "" {
			sb.WriteString(fmt.Sprintf(" | Will: %v", p.LastWill))
		}
		sb.WriteString("\n")
	}
	a := g.Area
	msg := sb.String()
	g.mu.Unlock()

	sendAreaServerMessage(a, msg)
	deleteMafiaGame(a)
}

// ============================================================
// Disconnect handling
// ============================================================

// handleMafiaDisconnect removes a disconnected client from any active Mafia game.
func handleMafiaDisconnect(client *Client) {
	mafiaStates.Range(func(_, value interface{}) bool {
		g := value.(*MafiaGame)
		g.mu.Lock()
		p := g.findPlayerByClient(client)
		if p == nil {
			g.mu.Unlock()
			return true
		}
		p.Disconnected = true
		name := p.Name()
		phase := g.Phase
		wasAlive := p.Alive

		if phase == MafiaPhaseLobby {
			// Remove from lobby
			newList := make([]*MafiaPlayer, 0, len(g.Players)-1)
			for _, pp := range g.Players {
				if pp != p {
					newList = append(newList, pp)
				}
			}
			g.Players = newList
			g.mu.Unlock()
			g.broadcastToGame(fmt.Sprintf("📤 %v left the lobby.", name))
			return true
		}

		if wasAlive {
			p.Alive = false
			g.addToGraveyard(p, "Disconnected")
		}
		winner, won := g.checkWin()
		g.mu.Unlock()

		if wasAlive {
			g.broadcastToGame(fmt.Sprintf("📴 %v disconnected and has been removed from the game.", name))
			if won {
				g.endGame(winner)
			}
		}
		return true
	})
}
