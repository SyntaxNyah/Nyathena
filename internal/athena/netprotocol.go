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
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
	"github.com/MangosArentLiterature/Athena/internal/webhook"
)

// Documentation for AO2's network protocol can be found here:
// https://github.com/AttorneyOnline/docs/blob/master/docs/development/network.md

// commandRegex matches valid command names (e.g., /join, /join-tournament, /51), case-insensitively.
var commandRegex = regexp.MustCompile(`(?i)^/[a-z0-9]+(-[a-z0-9]+)*`)

// tstNavRegex matches testimony navigation controls (<, >, >N) in IC messages.
// Compiled once at package init to avoid repeated allocation during testimony playback.
var tstNavRegex = regexp.MustCompile(`[<>]([[:digit:]]+)?`)

// maxShownameLength is the maximum number of characters allowed in a showname.
const maxShownameLength = 30

// accountWelcomeMsg is the on-join welcome message shown to unauthenticated players
// when the casino is disabled but the optional account system is enabled
// (`enable_accounts = true`). It advertises the *non-gambling* half of the
// account system: free account creation, wardrobe, default cosmetic tag, and
// playtime tracking. It also documents every tag category available in /shop and
// how to equip a tag with /settag — because with the casino off, tags are free.
//
// Defined as a compile-time constant so there is zero runtime allocation per join.
const accountWelcomeMsg = "👤 Welcome! This server offers an optional free player account.\n\n" +
	"📋 What your account is for:\n" +
	"  • 👗 Wardrobe — save favourite characters and swap between them instantly.\n" +
	"  • 🏷️ Default tag — pick any cosmetic tag from the catalog and wear it next to your name.\n" +
	"  • ⏱️ Playtime tracking — hours persist across sessions and feed /playtime top.\n\n" +
	"🔑 Account commands:\n" +
	"  • /register <username> <password>  — create a free account (username 3–20, password 6+)\n" +
	"  • /login <username> <password>     — sign in on reconnect\n" +
	"  • /account                         — view your profile\n" +
	"  • /wardrobe                        — list favourite characters\n" +
	"  • /favourite <char>                — add/remove a character from the wardrobe\n" +
	"  • /playtime top                    — see the playtime leaderboard\n\n" +
	"🏷️ Default tags — every tag in /shop is FREE to equip:\n" +
	"  • 🎰 gambling   — 30 tags (Gambler, Lucky, High Roller, Whale, Godlike, Infinite…)\n" +
	"  • ⚖️ attorney   — 15 tags (Objection!, Take That!, Hold It!, Prosecutor, Magatama…)\n" +
	"  • 🌸 anime      — 15 tags (Weeb, Senpai, Kawaii, Waifu Haver, Protagonist…)\n" +
	"  • 🎮 gamer      — 15 tags (Noob, Tryhard, Speedrunner, PvP God, Gaming Chair…)\n" +
	"  • 🌷 girly      — 12 tags (Princess, Fairy, Queen, Sparkle, Divine Feminine…)\n" +
	"  • 😂 meme       — 18 tags (Bruh, NPC, Sus, Based, Gigachad, Meme Lord…)\n" +
	"  • 👑 prestige   — 10 tags (Newcomer, Veteran, Grandmaster, Overlord, Absolute Unit…)\n" +
	"  Browse a category:  /shop <category>   (e.g. /shop attorney)\n" +
	"  Equip a tag:        /settag <tag_id>   (e.g. /settag tag_objection)\n" +
	"  Remove your tag:    /settag none\n\n" +
	"💡 Accounts are entirely optional. Skip /register if you just want to play."

// casinoWelcomeMsg is the on-join welcome message shown to unauthenticated players when the
// casino is enabled. Defined as a compile-time constant so there is zero runtime allocation
// on every player join.
const casinoWelcomeMsg = "🎰 Welcome! This server runs the Nyathena Casino — virtual chips, games, jobs & more.\n\n" +
	"📖 Quick navigation:\n" +
	"  • /help          — see every available command\n" +
	"  • /casino        — open the casino dashboard (active tables, balance, game list)\n" +
	"  • /chips         — check your chip balance\n" +
	"  • /jobs          — list all jobs you can work to earn chips\n" +
	"  • /shop          — spend chips on permanent tags & upgrades\n\n" +
	"💰 Ways to earn chips:\n" +
	"  • Everyone starts with 500 chips automatically.\n" +
	"  • Earn 1 chip per hour of playtime (more with /shop passive upgrades!).\n" +
	"  • Work a job: /janitor /busker /paperboy /bailiffjob /clerk  (40–60 min cooldowns).\n" +
	"  • Unscramble events every 30–60 min — type the answer in IC chat to win 10 chips!\n\n" +
	"🍻 THE BAR — /bar menu | /bar buy <drink>\n" +
	"  34 drinks, every one with RISK and HUGE variance. Big wins or big losses!\n" +
	"  beer wine tequila moonshine absinthe mystery poison dragonblood goldenelixir voiddrink\n" +
	"  angelwine devilswhiskey ... use /bar menu to see them all!\n\n" +
	"🛒 Spend your chips at /shop:\n" +
	"  • 115+ cosmetic tags in 7 categories (from 100 chips!) · job passes · passive income\n" +
	"  → /shop <category> to browse  |  /shop buy <id> to purchase\n\n" +
	"📊 Leaderboards: /richest  /playtime top  /unscramble top  /jobtop\n\n" +
	"👗 Wardrobe: /favourite <char> · /wardrobe · /wardrobe <char>  (save favourite characters!)\n\n" +
	"💡 /register <username> <password>  |  /login <username> <password>\n" +
	"🔒 Passwords stored with bcrypt — never in plain text.\n" +
	"🔇 Use /gamble hide to toggle gambling broadcast messages."

// validDeskMods is the set of accepted values for args[0] (desk_mod) in MS packets.
// Defined at package level to avoid a slice allocation on every IC message.
var validDeskMods = []string{"chat", "0", "1", "2", "3", "4", "5"}

type pktMapValue struct {
	Args     int
	MustJoin bool
	Func     func(client *Client, p *packet.Packet)
}

var PacketMap = map[string]pktMapValue{
	"HI":      {1, false, pktHdid},
	"ID":      {2, false, pktId},
	"askchaa": {0, false, pktResCount},
	"RC":      {0, false, pktReqChar},
	"RM":      {0, false, pktReqAM},
	"RD":      {0, false, pktReqDone},
	"CC":      {2, true, pktChangeChar},
	"MS":      {15, true, pktIC},
	"MC":      {2, true, pktAM},
	"HP":      {2, true, pktHP},
	"RT":      {1, true, pktWTCE},
	"CT":      {2, true, pktOOC},
	"PE":      {3, true, pktAddEvi},
	"DE":      {1, true, pktRemoveEvi},
	"EE":      {4, true, pktEditEvi},
	"CH":      {0, false, pktPing},
	"ZZ":      {0, true, pktModcall},
	"SETCASE": {7, true, pktSetCase},
	"CASEA":   {6, true, pktCaseAnn},
}

// Handles HI#%
func pktHdid(client *Client, p *packet.Packet) {
	if strings.TrimSpace(p.Body[0]) == "" || client.Uid() != -1 || client.Hdid() != "" {
		return
	}

	// Athena does not store the client's raw HDID, but rather, it's MD5 hash.
	// This is done not only for privacy reasons, but to ensure stored HDIDs will be a reasonable length.
	hash := md5.Sum([]byte(decode(p.Body[0])))
	client.SetHdid(base64.StdEncoding.EncodeToString(hash[:]))
	client.SetHdid(client.Hdid()[:len(client.Hdid())-2]) // Removes the trailing padding.

	if client.CheckBanned(db.HDID) {
		return
	}

	client.SendPacket("ID", "0", "Athena", encode(version)) // Why does the client need this? Nobody knows.
}

// Handles ID#%
func pktId(client *Client, _ *packet.Packet) {
	if client.Uid() != -1 {
		return
	}
	client.SendPacket("PN", strconv.Itoa(players.GetPlayerCount()), strconv.Itoa(config.MaxPlayers), encode(config.Desc))
	client.SendPacket("FL", "noencryption", "yellowtext", "prezoom", "flipping", "customobjections",
		"fastloading", "deskmod", "evidence", "cccc_ic_support", "arup", "casing_alerts",
		"modcall_reason", "looping_sfx", "additive", "effects", "y_offset", "expanded_desk_mods", "auth_packet") // god this is cursed

	if config.AssetURL != "" {
		client.SendPacket("ASS", config.AssetURL)
	}
}

// Handles askchaa#%
func pktResCount(client *Client, _ *packet.Packet) {
	if client.Uid() != -1 || client.Hdid() == "" {
		return
	}
	if players.GetPlayerCount() >= config.MaxPlayers {
		logger.LogInfo("Player limit reached")
		client.SendPacket("BD", "This server is currently full.")
		client.conn.Close()
		return
	}
	// Capacity lockdown: reject new connections when the player count has reached
	// the operator-configured threshold (0 = disabled).
	// Known IPs (returning players in the DB) bypass this cap, matching the behaviour
	// of serverLockdown which already exempts previously-seen IPIDs.
	if threshold := int(playerLockdownThreshold.Load()); threshold > 0 && players.GetPlayerCount() >= threshold {
		ipFirstSeenTracker.mu.Lock()
		_, known := ipFirstSeenTracker.times[client.Ipid()]
		ipFirstSeenTracker.mu.Unlock()
		if !known {
			logger.LogInfof("Connection from %v rejected (player capacity lockdown, threshold %v)", client.Ipid(), threshold)
			client.SendPacket("BD", "This server is not currently accepting new connections.")
			client.conn.Close()
			return
		}
	}
	client.joining = true // This simply exists to prevent skipping the askchaa#% packet and bypassing the player count check.
	client.SendPacket("SI", strconv.Itoa(len(characters)), strconv.Itoa(len(areas[0].Evidence())), strconv.Itoa(len(music)))
}

// Handles RC#%
func pktReqChar(client *Client, _ *packet.Packet) {
	client.SendPacket("SC", characters...)
}

// Handles RM#%
func pktReqAM(client *Client, _ *packet.Packet) {
	client.write(smPacket)
}

// Handles RD#%
func pktReqDone(client *Client, _ *packet.Packet) {
	if client.Uid() != -1 || !client.joining || client.Hdid() == "" {
		return
	}
	client.SetUid(uids.GetUid())
	clients.RegisterUID(client)
	client.SetConnectedAt(time.Now())
	client.lastPingNano.Store(time.Now().UnixNano()) // seed so the ping timeout window starts from join time
	players.AddPlayer()
	if config.Advertise {
		updatePlayers <- players.GetPlayerCount()
	}
	client.JoinArea(areas[0])
	client.SendPacket("DONE")
	sendCMArup()
	sendStatusArup()
	sendLockArup()
	// Notify the client of their actual UID so the player list widget filters correctly.
	client.SendPacket("ID", strconv.Itoa(client.Uid()), "Athena", encode(version))
	sendPlayerListToClient(client)
	broadcastPlayerJoin(client)
	if config.Motd != "" {
		client.SendServerMessage(config.Motd)
	}
	client.restorePunishments()

	// Casino on-join setup: seed chip balance and prompt unregistered players.
	// When the casino is off but the account system is enabled, the account
	// welcome message (wardrobe / default tags / playtime tracking) is shown
	// to unauthenticated joiners.
	if config.EnableCasino {
		ipid := client.Ipid()
		go func() {
			if err := db.EnsureChipBalance(ipid); err != nil {
				logger.LogErrorf("Failed to seed chip balance for %v: %v", ipid, err)
			}
		}()
		if !client.Authenticated() {
			client.SendServerMessage(casinoWelcomeMsg)
		}
	} else if config.EnableAccounts && !client.Authenticated() {
		client.SendServerMessage(accountWelcomeMsg)
	}

	logger.LogInfof("Client (IPID:%v UID:%v) joined the server", client.Ipid(), client.Uid())
}

// getRandomFreeChar returns a random free character ID in the client's area,
// or -1 if no characters are available.
func getRandomFreeChar(client *Client) int {
	var free []int
	for i := range characters {
		if !client.Area().IsTaken(i) {
			free = append(free, i)
		}
	}
	if len(free) == 0 {
		return -1
	}
	return free[rand.Intn(len(free))]
}

// Handles CC#%
func pktChangeChar(client *Client, p *packet.Packet) {
	// Check rate limit first
	if client.CheckRateLimit() {
		client.KickForRateLimit()
		return
	}
	newid, err := strconv.Atoi(p.Body[1])
	if err != nil {
		return
	}
	// WebAO sends -1 to indicate random character selection.
	if newid == -1 {
		newid = getRandomFreeChar(client)
		if newid == -1 {
			return // No free characters available
		}
	}
	if stuckID := client.charStuckID(); stuckID >= 0 && newid != stuckID {
		client.SendServerMessage(fmt.Sprintf("You are character stuck as %v and cannot change characters.", characters[stuckID]))
		return
	}
	if client.IsTunged() {
		client.SendServerMessage("You have been tunged and cannot change characters until the effect is removed.")
		return
	}
	client.ChangeCharacter(newid)
}

// Handles MS#%
func pktIC(client *Client, p *packet.Packet) {
	// Welcome to the MS packet validation hell.

	// Check rate limit first
	if client.CheckRateLimit() {
		client.KickForRateLimit()
		return
	}

	if !client.CanSpeakIC() { // Literally 1984
		client.SendServerMessage("You are not allowed to speak in this area.")
		return
	}
	// Clients can send differing numbers of arguments depending on their version.
	// Rather than individually check arguments, we simply copy the arguments that *do* exist.
	// Nonexisting args will simply be blank.
	args := make([]string, 26)
	copy(args, p.Body)

	// The MS#% packet sent from the server has a different number of args than the clients because of pairing.
	// For some godforsaken reason, AO2 places these new arguments in two different spots in the middle of the packet.
	// So two insertions are required.
	args = append(args[:19], args[17:]...)
	args = append(args[:20], args[18:]...)

	// Save the admin's own args before any fullpossess transformation so that
	// state updates (showname, pairInfo, textColor) always reflect the admin's
	// own character — not the target's — even during fullpossess.
	ownCharName := args[2]
	ownEmote := args[3]
	ownTextColor := args[14]
	ownShowname := args[15]
	hasForcedIniswap := false

	// If a moderator has forced a showname for this client, override whatever
	// name the client sent in the packet.
	if forced := client.ForcedShowname(); forced != "" {
		ownShowname = forced
		args[15] = forced
	}

	// If a moderator has forced an iniswap character for this client, override
	// the outgoing IC character name and ID. Both values are pre-computed at
	// command invocation so this hot path performs only a single mutex
	// acquisition and two string assignments — no map lookup or int conversion.
	if charName, charIDStr := client.ForcedIniswapInfo(); charName != "" {
		hasForcedIniswap = true
		ownCharName = charName
		args[2] = charName
		args[8] = charIDStr
	}

	// Track if we're in fullpossess mode for validation adjustments
	isPossessing := false

	// Full possession: Transform admin's IC messages to appear from target
	if client.Possessing() != -1 {
		target, err := getClientByUid(client.Possessing())
		if err != nil {
			// Target no longer exists, clear possession
			client.SetPossessing(-1)
			client.SetPossessedPos("")
			client.SendServerMessage("Target disconnected. Possession ended.")
		} else {
			isPossessing = true
			// Transform the message to use target's appearance
			// Use the saved target position (from when possession started) to fully spoof them

			// Get target's emote, or use "normal" as fallback
			targetEmote := target.PairInfo().emote
			if targetEmote == "" {
				targetEmote = "normal"
			}

			// Get the target's displayed character name (handles iniswap)
			// Use PairInfo().name if available (contains iniswapped character), otherwise use their actual character
			targetCharName := target.PairInfo().name
			if targetCharName == "" {
				// Bounds check before accessing characters array
				if target.CharID() >= 0 && target.CharID() < len(characters) {
					targetCharName = characters[target.CharID()]
				} else {
					// Invalid character, clear possession
					client.SetPossessing(-1)
					client.SetPossessedPos("")
					client.SendServerMessage("Target has invalid character. Possession ended.")
					return
				}
			}

			// Get the character ID for the displayed character
			targetCharID := getCharacterID(targetCharName)
			if targetCharID == -1 {
				// If character name is not found, fall back to target's actual character
				targetCharID = target.CharID()
				// Verify bounds before accessing characters array
				if targetCharID >= 0 && targetCharID < len(characters) {
					targetCharName = characters[targetCharID]
				} else {
					// Invalid character, clear possession
					client.SetPossessing(-1)
					client.SetPossessedPos("")
					client.SendServerMessage("Target has invalid character. Possession ended.")
					return
				}
			}

			// Replace character and appearance with target's (including their saved position)
			args[2] = targetCharName             // character name (target's displayed character, including iniswap)
			args[3] = targetEmote                // emote
			args[5] = client.PossessedPos()      // position (saved target position)
			args[8] = strconv.Itoa(targetCharID) // char_id (ID of target's displayed character)

			// Use target's text color
			targetTextColor := target.LastTextColor()
			if targetTextColor == "" {
				targetTextColor = "0"
			}
			args[14] = targetTextColor

			// Use target's showname, respecting any moderator-forced showname,
			// and falling back to the displayed character name.
			targetShowname := target.EffectiveShowname()
			if strings.TrimSpace(targetShowname) == "" {
				targetShowname = targetCharName
			}
			args[15] = targetShowname
		}
	}

	if pos := client.Pos(); pos != "" {
		args[5] = pos
	} else {
		client.SetPos(args[5])
	}

	// Check for expired punishments and collect the still-active ones in a single
	// lock acquisition (avoids a second mutex cycle + second time.Now() call).
	expired, punishments := client.CheckExpiredAndGetPunishments()
	if expired {
		client.SendServerMessage("One or more punishments have expired.")
	}

	// Apply punishment text modifications
	// Note: punishments is a copy of the active punishments
	// State modifications must use UpdatePunishmentState to persist changes
	for i := range punishments {
		p := &punishments[i]

		// Apply text modifications
		if args[4] != "" {
			decodedMsg := decode(args[4])
			var modifiedMsg string

			// Use state-aware version for punishments that need it
			if p.punishmentType == PunishmentTorment {
				client.UpdatePunishmentState(p.punishmentType, func(ps *PunishmentState) {
					modifiedMsg = ApplyPunishmentToTextWithState(decodedMsg, p.punishmentType, ps)
				})
			} else if p.punishmentType == PunishmentLovebomb {
				// Resolve the target's display name.
				var targetShowname string
				if p.targetUID >= 0 {
					if target, err := getClientByUid(p.targetUID); err == nil {
						targetShowname = clientDisplayName(target)
					}
				}
				if targetShowname == "" {
					// Reservoir-sample one random area member (excluding self) without
					// allocating a full slice — O(N) single pass, zero extra heap.
					var chosen *Client
					n := 0
					clients.ForEach(func(c *Client) {
						if c.Area() == client.Area() && c.Uid() != client.Uid() {
							n++
							if rand.Intn(n) == 0 {
								chosen = c
							}
						}
					})
					if chosen != nil {
						targetShowname = clientDisplayName(chosen)
					}
				}
				modifiedMsg = applyLovebombMessage(targetShowname)
			} else if p.punishmentType == PunishmentThirdPerson {
				displayName := clientDisplayName(client)
				modifiedMsg = applyThirdPersonWithName(decodedMsg, displayName)
			} else {
				modifiedMsg = ApplyPunishmentToText(decodedMsg, p.punishmentType)
			}
			args[4] = encode(modifiedMsg)
		}

		// Handle name modifications
		if p.punishmentType == PunishmentEmoji {
			args[3] = GetRandomEmoji()
		}
		if p.punishmentType == PunishmentUncannyValley {
			name := args[15]
			if strings.TrimSpace(name) == "" && client.CharID() >= 0 && client.CharID() < len(characters) {
				name = characters[client.CharID()]
			}
			if name != "" {
				args[15] = MutateShowname(name)
			}
		}
	}

	if client.IsParrot() { // Bring out the parrot please.
		args[4] = getParrotMsg()
	}
	if client.IsNarrator() {
		args[3] = ""
	}
	if flip := client.CheckAndToggleDanceFlip(); flip != "" {
		args[12] = flip
	}
	emote_mod, err := strconv.Atoi(args[7])
	if err != nil {
		return
	} else if emote_mod == 4 { // Value of 4 can crash the client.
		args[7] = "6"
	}
	objStr, _, _ := strings.Cut(args[10], "&")
	objection, err := strconv.Atoi(objStr)
	if err != nil {
		return
	}
	evi, err := strconv.Atoi(args[11])
	if err != nil {
		return
	}
	text, err := strconv.Atoi(args[14])
	if err != nil {
		return
	}

	if args[22] == "" {
		args[22] = "0"
	}
	if args[23] == "" {
		args[23] = "0"
	}
	if args[24] == "" {
		args[24] = "0"
	}
	if args[28] == "" || client.CharID() != client.Area().LastSpeaker() {
		args[28] = "0"
	}
	if (client.Area().NoInterrupt() && emote_mod != 0) || args[22] == "1" {
		args[22] = "1"
		if emote_mod == 1 || emote_mod == 2 {
			args[7] = "0"
		} else if emote_mod == 6 {
			args[7] = "5"
		}
	}

	// Decode the message text once; reused for length validation, testimony navigation, and automod.
	msgText := decode(args[4])

	// Single lock to obtain the stuck character ID; -1 means not stuck.
	// Used in both iniswap cases below to avoid redundant mutex acquisitions.
	stuckCharID := client.charStuckID()

	switch {
	case !sliceutil.ContainsString(validDeskMods, args[0]): // desk_mod
		return
	case !isPossessing && !hasForcedIniswap && !strings.EqualFold(characters[client.CharID()], args[2]) && !client.Area().IniswapAllowed(): // character name (skip check when possessing or forced iniswap)
		client.SendServerMessage("Iniswapping is not allowed in this area.")
		return
	case !isPossessing && !hasForcedIniswap && stuckCharID >= 0 && !strings.EqualFold(characters[stuckCharID], args[2]): // block iniswap when charstuck unless forced iniswap
		client.SendServerMessage(fmt.Sprintf("You are character stuck as %v and cannot iniswap.", characters[stuckCharID]))
		return
	case len(msgText) > config.MaxMsg: // message
		client.SendServerMessage("Your message exceeds the maximum message length!")
		return
	case args[4] == client.LastMsg():
		return
	case emote_mod < 0 || emote_mod > 6:
		return
	case !isPossessing && !hasForcedIniswap && args[8] != client.CharIDStr(): // char_id (skip check when possessing or forced iniswap)
		return
	case objection < 0 || objection > 4: // objection_mod
		return
	case evi < 0 || evi > len(client.Area().Evidence()): // evidence
		return
	case args[12] != "0" && args[12] != "1": // flipping
		return
	case args[13] != "0" && args[13] != "1": // realization
		return
	case text < 0 || text > 8: // text color (0-8 per AO2 protocol)
		return
	case len(args[15]) > maxShownameLength: // showname
		client.SendServerMessage("Your showname is too long!")
		return
	case args[22] != "0" && args[22] != "1": // non-interrupting preanim
		return
	case args[23] != "0" && args[23] != "1": // sfx looping
		return
	case args[24] != "0" && args[24] != "1": // screenshake
		return
	case args[28] != "0" && args[28] != "1": // additive
		return
	}

	// If force-paired, always sync the wanted CharID to the partner's current CharID,
	// keeping the pair active even when either player changes character or position.
	// Also sync the position so both characters are always broadcast at the same
	// courtroom position, preventing the pairing sprite from breaking on clients.
	if client.ForcePairUID() >= 0 {
		if partner, err := getClientByUid(client.ForcePairUID()); err == nil && partner.CharID() >= 0 {
			args[16] = partner.CharIDStr()
			client.SetPairWantedID(partner.CharID())
			partner.SetPairWantedID(client.CharID())
			if pos := partner.Pos(); pos != "" {
				args[5] = pos
				client.SetPos(pos)
			}
		}
	}

	// If the client used /pair to set a desired pair but has not selected one via the
	// in-client pair button (args[16] is absent or -1), inject the server-set pair
	// character ID so the pairing animation activates exactly as if the pair button
	// had been used.
	if (args[16] == "" || args[16] == "-1") && client.PairWantedID() != -1 {
		args[16] = strconv.Itoa(client.PairWantedID())
	}

	// Pairing validation
	if args[16] != "" && args[16] != "-1" {
		pidStr, _, _ := strings.Cut(args[16], "^")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return
		}
		if pid < 0 || pid > len(characters) || pid == client.CharID() {
			return
		}
		client.SetPairWantedID(pid)
		pairing := false
		clients.ForEach(func(c *Client) {
			if pairing {
				return
			}
			isForce := client.ForcePairUID() >= 0 && client.ForcePairUID() == c.Uid() &&
				c.ForcePairUID() >= 0 && c.ForcePairUID() == client.Uid()
			// If the client has a stored pair partner, skip any client that isn't
			// that specific partner to prevent false matches from position overlap.
			if client.ForcePairUID() >= 0 && !isForce {
				return
			}
			// Also guard the candidate: if c is already UID-committed to a different partner,
			// it must not be matched by anyone other than that partner.
			if c.ForcePairUID() >= 0 && c.ForcePairUID() != client.Uid() {
				return
			}
			if c.CharID() == pid && c.PairWantedID() == client.CharID() && (isForce || c.Pos() == client.Pos()) {
				pairinfo := c.PairInfo()
				args[17] = pairinfo.name
				args[18] = pairinfo.emote
				args[20] = pairinfo.offset
				args[21] = pairinfo.flip
				pairing = true
			}
		})
		if !pairing {
			args[16] = "-1^"
			args[17] = ""
			args[18] = ""
		}
	} else {
		// No pair attempted: ensure otherName/otherEmote are empty.
		args[17] = ""
		args[18] = ""
	}

	// Offset validation
	if args[19] != "" {
		offsets := strings.Split(decode(args[19]), "&")
		x_offset, err := strconv.Atoi(offsets[0])
		if err != nil {
			return
		} else if x_offset < -100 || x_offset > 100 {
			return
		}
		if len(offsets) > 1 {
			y_offset, err := strconv.Atoi(offsets[0])
			if err != nil {
				return
			} else if y_offset < -100 || y_offset > 100 {
				return
			}
		}
	}

	// Testimony recorder
	if client.Pos() == "wit" && client.Area().TstState() != area.TRIdle {
		switch client.Area().TstState() {
		case area.TRRecording:
			if client.Area().TstLen() >= config.MaxStatement+1 {
				client.SendServerMessage("Unable to add message: Max statements reached.")
				break
			}
			if client.Area().CurrentTstIndex() == 0 {
				args[4] = "~~\n-- " + args[4] + " --"
				args[14] = "3"
				writeToArea(client.Area(), "RT", "testimony1")
			}
			client.Area().TstAppend(strings.Join(args, "#"))
			client.Area().TstAdvance()
		case area.TRInserting:
			if client.Area().TstLen() >= config.MaxStatement {
				client.SendServerMessage("Unable to insert message: Max statements reached.")
				client.Area().SetTstState(area.TRPlayback)
				break
			}
			client.Area().TstInsert(strings.Join(args, "#"))
			client.Area().SetTstState(area.TRPlayback)
			client.Area().TstAdvance()
		case area.TRUpdating:
			if client.Area().CurrentTstIndex() == 0 {
				client.SendServerMessage("Cannot edit testimony title.")
				client.Area().SetTstState(area.TRPlayback)
				break
			}
			client.Area().TstUpdate(strings.Join(args, "#"))
			client.Area().SetTstState(area.TRPlayback)
		}
	}
	if client.Area().TstState() == area.TRPlayback {
		s := tstNavRegex.FindString(msgText)
		if s != "" {
			if strings.ContainsRune(s, '<') {
				client.Area().TstRewind()
				writeToArea(client.Area(), "MS", client.Area().CurrentTstStatement())
				return
			}
			_, idStr, _ := strings.Cut(s, ">")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				client.Area().TstAdvance()
				writeToArea(client.Area(), "MS", client.Area().CurrentTstStatement())
				return
			} else {
				if id > 0 && id < client.Area().TstLen() {
					client.Area().TstJump(id)
					writeToArea(client.Area(), "MS", client.Area().CurrentTstStatement())
					return
				}
			}
		}
	}

	// Use the admin's own (pre-transformation) values for state updates so that
	// the admin's showname, pairInfo and textColor are never overwritten with
	// the target's during fullpossess.
	client.SetPairInfo(ownCharName, ownEmote, args[12], args[19])
	client.SetLastMsg(args[4])
	client.SetLastTextColor(ownTextColor)
	newShowname := ownShowname
	if strings.TrimSpace(ownShowname) == "" {
		newShowname = characters[client.CharID()]
	}
	// Only broadcast a PU showname update when the showname actually changed.
	if client.UpdateShowname(newShowname) {
		writeToAll("PU", strconv.Itoa(client.Uid()), "2", decode(newShowname))
	}
	client.Area().SetLastSpeaker(client.CharID())

	// Track tournament message count
	if tournamentActive {
		tournamentMutex.Lock()
		if participant, exists := tournamentParticipants[client.Uid()]; exists {
			participant.messageCount++
		}
		tournamentMutex.Unlock()
	}

	// Quickdraw: record the reaction for any active duel.
	quickdrawOnIC(client, msgText)

	// Typing race: check whether the IC message matches the active race phrase.
	typingRaceOnIC(client, msgText)

	// Unscramble: check whether the IC message is the correct answer.
	if config != nil && config.EnableCasino {
		unscrambleOnIC(client, msgText)
	}

	// Automod: check the decoded message for banned words before broadcasting.
	if autoModCheck(client, msgText) {
		return
	}

	// Torment: ghost or delay the message without the client noticing.
	if isIPIDTormented(client.Ipid()) {
		handleTormentedIC(client, args)
		return
	}

	writeToAreaFrom(client.Ipid(), permissions.IsModerator(client.Perms()), client.Area(), "MS", args...)
	addToBuffer(client, "IC", "\""+args[4]+"\"", false)
}

// Handles MC#%
func pktAM(client *Client, p *packet.Packet) {
	// For reasons beyond mortal understanding, this packet serves two purposes: music changes, and area changes.

	// Check rate limit first
	if client.CheckRateLimit() {
		client.KickForRateLimit()
		return
	}

	if client.CharIDStr() != p.Body[1] {
		return
	}

	decodedSong := decode(p.Body[0])
	if sliceutil.ContainsString(music, decodedSong) {
		if !client.CanChangeMusic() {
			client.SendServerMessage("You are not allowed to change the music in this area.")
			return
		}
		song := p.Body[0]
		name := client.Showname()
		effects := "0"
		if !strings.ContainsRune(decodedSong, '.') { // Chosen song is a category, and should stop the music.
			song = "~stop.mp3"
			addToBuffer(client, "MUSIC", "Stopped the music.", false)
		} else {
			addToBuffer(client, "MUSIC", fmt.Sprintf("Changed music to %v.", decodedSong), false)
		}
		if len(p.Body) > 2 {
			name = p.Body[2]
		}
		if len(p.Body) > 3 {
			effects = p.Body[3]
		}
		writeToArea(client.Area(), "MC", song, p.Body[1], name, "1", "0", effects)
	} else if strings.Contains(areaNames, decodedSong) {
		if decodedSong == client.Area().Name() {
			return
		}
		for _, a := range areas {
			if a.Name() == decodedSong {
				if !client.ChangeArea(a) {
					// Mods already received an OOC message from ChangeArea (lock
					// warning or jail notice); only non-mods need this generic reply.
					if !permissions.IsModerator(client.Perms()) {
						client.SendServerMessage("You are not invited to that area.")
					}
					return
				}
				client.SendServerMessage(fmt.Sprintf("Moved to %v.", a.Name()))
				return
			}
		}
	}
}

// Handles HP#%
func pktHP(client *Client, p *packet.Packet) {
	if client.CharID() == -1 || !client.CanJud() {
		client.SendServerMessage("You are not allowed to change the penalty bar in this area.")
		return
	}
	bar, err := strconv.Atoi(p.Body[0])
	if err != nil {
		return
	}
	value, err := strconv.Atoi(p.Body[1])

	if err != nil {
		return
	}
	if !client.Area().SetHP(bar, value) {
		return
	}
	writeToArea(client.Area(), "HP", p.Body[0], p.Body[1])

	var side string
	switch bar {
	case 1:
		side = "Defense"
	case 2:
		side = "Prosecution"
	}
	addToBuffer(client, "JUD", fmt.Sprintf("Set %v HP to %v.", side, value), false)
}

// Handles RT#%
func pktWTCE(client *Client, p *packet.Packet) {
	if client.CharID() == -1 || !client.CanJud() {
		client.SendServerMessage("You are not allowed to play WT/CE in this area.")
		return
	}
	if len(p.Body) >= 2 {
		writeToArea(client.Area(), "RT", p.Body[0], p.Body[1])
	} else {
		writeToArea(client.Area(), "RT", p.Body[0])
	}
	addToBuffer(client, "JUD", "Played WT/CE animation.", false)
}

// Handles CT#%
func pktOOC(client *Client, p *packet.Packet) {
	// Check rate limit first
	if client.CheckRateLimit() {
		client.KickForRateLimit()
		return
	}

	// Check OOC-specific rate limit per IP; kick on excess to prevent flood even across reconnections.
	if checkIPOOCRateLimit(client.Ipid()) {
		client.KickForRateLimit()
		return
	}

	username := decode(strings.TrimSpace(p.Body[0]))
	if username == "" || username == config.Name || len(username) > 30 || strings.ContainsAny(username, "[]") {
		client.SendServerMessage("Invalid username.")
		return
	} else if len(p.Body[1]) > config.MaxMsg {
		client.SendServerMessage("Your message exceeds the maximum message length!")
		return
	} else if strings.TrimSpace(p.Body[1]) == "" {
		return
	}
	var usernameTaken bool
	clients.ForEach(func(c *Client) {
		if c.OOCName() == p.Body[0] && c != client {
			usernameTaken = true
		}
	})
	if usernameTaken {
		client.SendServerMessage("That username is already taken.")
		return
	}
	client.SetOocName(username)

	if strings.HasPrefix(p.Body[1], "/") {
		if client.Uid() != -1 {
			writeToAll("PU", strconv.Itoa(client.Uid()), "0", username)
		}
		decoded := decode(p.Body[1])
		match := commandRegex.FindString(decoded)
		command := strings.ToLower(strings.TrimPrefix(match, "/"))
		args := strings.Split(decoded, " ")[1:]
		ParseCommand(client, command, args)
		return
	}
	if client.IsJailed() {
		client.SendServerMessage("You are jailed and cannot speak in OOC.")
		return
	}
	if !client.CanSpeakOOC() {
		client.SendServerMessage("You are muted from speaking in OOC.")
		return
	}
	// Check new-IPID OOC cooldown; commands are exempt so new users can still interact with the server.
	if limited, remaining := checkNewIPIDOOCCooldown(client.Ipid()); limited {
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("New users must wait %d %s before using OOC chat.", remaining, unit))
		return
	}
	// Only broadcast the OOC name update once all checks pass, to prevent amplification attacks
	// where bots flood CT packets causing mass PU broadcasts to all connected clients.
	if client.Uid() != -1 {
		writeToAll("PU", strconv.Itoa(client.Uid()), "0", username)
	}
	msg := p.Body[1]
	// Reject duplicate OOC: if the last message sent in this area is identical, drop silently.
	if last, ok := areaLastOOCMsg.Load(client.Area()); ok {
		if lastStr, ok := last.(string); ok && lastStr == msg {
			return
		}
	}
	areaLastOOCMsg.Store(client.Area(), msg)
	// Automod: check the OOC message for banned words before broadcasting.
	if autoModCheck(client, decode(msg)) {
		return
	}
	// Torment: ghost or delay the OOC message without the client noticing.
	if isIPIDTormented(client.Ipid()) {
		handleTormentedOOC(client, encode(client.OOCName()), msg)
		return
	}
	writeToAreaFrom(client.Ipid(), permissions.IsModerator(client.Perms()), client.Area(), "CT", encode(client.OOCName()), msg, "0")
	addToBuffer(client, "OOC", "\""+msg+"\"", false)
}

// Handles PE#%
func pktAddEvi(client *Client, p *packet.Packet) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to alter evidence in this area.")
		return
	}
	client.Area().AddEvidence(strings.Join(p.Body, "&"))
	writeToArea(client.Area(), "LE", client.Area().Evidence()...)
	addToBuffer(client, "EVI", fmt.Sprintf("Added evidence: %v | %v", p.Body[0], p.Body[1]), false)
}

// Handles DE#%
func pktRemoveEvi(client *Client, p *packet.Packet) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to alter evidence in this area.")
		return
	}
	id, err := strconv.Atoi(p.Body[0])
	if err != nil {
		return
	}
	client.Area().RemoveEvidence(id)
	writeToArea(client.Area(), "LE", client.Area().Evidence()...)
	addToBuffer(client, "EVI", fmt.Sprintf("Removed evidence %v.", id), false)
}

// Handles EE#%
func pktEditEvi(client *Client, p *packet.Packet) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to alter evidence in this area.")
		return
	}
	id, err := strconv.Atoi(p.Body[0])
	if err != nil {
		return
	}
	client.Area().EditEvidence(id, strings.Join(p.Body[1:], "&"))
	writeToArea(client.Area(), "LE", client.Area().Evidence()...)
	addToBuffer(client, "EVI", fmt.Sprintf("Updated evidence %v to %v | %v", id, p.Body[1], p.Body[2]), false)
}

// Handles CH#%
func pktPing(client *Client, _ *packet.Packet) {
	if checkIPPingRateLimit(client.Ipid()) {
		return
	}
	client.lastPingNano.Store(time.Now().UnixNano())
	client.SendPacket("CHECK")
}

// Handles ZZ#%
func pktModcall(client *Client, p *packet.Packet) {
	if limited, remaining := checkNewIPIDModcallCooldown(client.Ipid()); limited {
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("New users must wait %d %s before sending a modcall.", remaining, unit))
		return
	}
	if limited, remaining := checkIPModcallCooldown(client.Ipid()); limited {
		unit := "seconds"
		if remaining == 1 {
			unit = "second"
		}
		client.SendServerMessage(fmt.Sprintf("You must wait %d %s before sending another modcall.", remaining, unit))
		return
	}
	setIPModcallTime(client.Ipid())
	var s string
	if len(p.Body) >= 1 {
		s = p.Body[0]
	}
	addToBuffer(client, "MOD", fmt.Sprintf("Called moderator for reason: %v", s), false)
	modcallMsg := fmt.Sprintf("MODCALL\n----------\nArea: %v\nUser: [%v] %v\nShowname: %v\nOOC Name: %v\nIPID: %v\nReason: %v",
		client.Area().Name(), client.Uid(), client.CurrentCharacter(), client.EffectiveShowname(), client.OOCName(), client.Ipid(), s)
	clients.ForEach(func(c *Client) {
		if c.Authenticated() && permissions.IsModerator(c.Perms()) {
			c.SendPacket("ZZ", modcallMsg)
		}
	})
	if enableDiscord {
		err := webhook.PostModcall(client.CurrentCharacter(), client.EffectiveShowname(), client.OOCName(), client.Ipid(), client.Area().Name(), s, client.Uid())
		if err != nil {
			logger.LogError(err.Error())
		}
	}
	logger.WriteReport(client.Area().Name(), client.Area().Buffer())
}

// Handles SETCASE#%
func pktSetCase(client *Client, p *packet.Packet) {
	for i, r := range p.Body[2:] {
		if i >= 4 {
			break
		}
		b, err := strconv.ParseBool(r)
		if err != nil {
			return
		}
		client.SetRoleAlert(i, b)
	}
}

// Handles CASEA#%
func pktCaseAnn(client *Client, p *packet.Packet) {
	// Let future generations know I spent far too long trying to make this work.
	// Partially because of my own stupidity, and partially because this is the worst packet in AO2.

	if client.CharID() == -1 || !client.HasCMPermission() {
		client.SendServerMessage("You are not allowed to send case alerts in this area.")
		return
	}
	newPacket := fmt.Sprintf("CASEA#CASE ANNOUNCEMENT: %v in %v needs players for %v#%v#1#%%",
		client.CurrentCharacter(), client.Area().Name(), p.Body[0], strings.Join(p.Body[1:], "#")) // Due to a bug, old client versions require this packet to have an extra arg.

	// Pre-parse the requested role flags once so we don't re-parse per recipient.
	// Use a fixed-size array to avoid any heap allocation; bail out immediately
	// on any malformed value (preserves original behaviour).
	var alertRoles [4]bool
	nRoles := 0
	for i, r := range p.Body[1:] {
		if i >= 4 {
			break
		}
		b, err := strconv.ParseBool(r)
		if err != nil {
			return
		}
		alertRoles[i] = b
		nRoles++
	}

	clients.ForEach(func(c *Client) {
		if c == client {
			return
		}
		for i := 0; i < nRoles; i++ {
			if alertRoles[i] && c.AlertRole(i) {
				c.write(newPacket)
				break
			}
		}
	})
}

// decoder and encoder are package-level, pre-compiled replacers for the AO2 percent-encoding scheme.
// Using package-level vars avoids re-allocating a new strings.Replacer on every encode/decode call.
// strings.Replacer.Replace is safe for concurrent use; these vars must never be reassigned after init.
var (
	decoder = strings.NewReplacer("<percent>", "%", "<num>", "#", "<dollar>", "$", "<and>", "&")
	encoder = strings.NewReplacer("%", "<percent>", "#", "<num>", "$", "<dollar>", "&", "<and>")
)

// decode returns a given AO2-encoded string in its decoded form.
func decode(s string) string {
	return decoder.Replace(s)
}

// encode returns a decoded string in AO2-encoded form.
func encode(s string) string {
	return encoder.Replace(s)
}
