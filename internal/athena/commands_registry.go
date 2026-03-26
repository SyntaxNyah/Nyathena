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
	"sort"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
)

type Command struct {
	handler   func(client *Client, args []string, usage string)
	minArgs   int
	usage     string
	desc      string
	reqPerms  uint64
	casinoCmd bool // when true, command is hidden/disabled if EnableCasino is false
}

var Commands map[string]Command

func initCommands() {
	Commands = map[string]Command{
		"about": {
			handler:  cmdAbout,
			minArgs:  0,
			usage:    "Usage: /about",
			desc:     "Prints Athena version information.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"add": {
			handler:  cmdAdd,
			minArgs:  0,
			usage:    "Usage: /add",
			desc:     "Inserts the next IC message from the witness into the testimony after the current statement.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"allowcms": {
			handler:  cmdAllowCMs,
			minArgs:  1,
			usage:    "Usage: /allowcms <true|false>",
			desc:     "Toggles allowing CMs on or off.",
			reqPerms: permissions.PermissionField["MODIFY_AREA"],
		},
		"allowiniswap": {
			handler:  cmdAllowIniswap,
			minArgs:  1,
			usage:    "Usage: /allowiniswap <true|false>",
			desc:     "Toggles iniswapping on or off.",
			reqPerms: permissions.PermissionField["MODIFY_AREA"],
		},
		"areainfo": {
			handler:  cmdAreaInfo,
			minArgs:  0,
			usage:    "Usage: /areainfo",
			desc:     "Prints area settings.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"ban": {
			handler:  cmdBan,
			minArgs:  3,
			usage:    "Usage: /ban -u <uid1>,<uid2>... | -i <ipid1>,<ipid2>... [-d duration] <reason>\n-i supports offline IPIDs.",
			desc:     "Bans user(s) from the server. Use -i to ban by IPID (supports offline users).",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"bg": {
			handler:  cmdBg,
			minArgs:  1,
			usage:    "Usage: /bg <background>",
			desc:     "Sets the area's background.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"bglist": {
			handler:  cmdBgList,
			minArgs:  0,
			usage:    "Usage: /bglist",
			desc:     "Lists all available backgrounds.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"charselect": {
			handler:  cmdCharSelect,
			minArgs:  0,
			usage:    "Usage: /charselect [uid1],[uid2]...",
			desc:     "Return to character select.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"charstuck": {
			handler:  cmdCharStuck,
			minArgs:  1,
			usage:    "Usage: /charstuck [-d duration] [-r reason] <uid>",
			desc:     "Locks a player to their current character, preventing character changes.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"charcurse": {
			handler:  cmdCharCurse,
			minArgs:  2,
			usage:    "Usage: /charcurse <uid> <charname>",
			desc:     "Forces a player to a specific character. The player may still change characters freely.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"cm": {
			handler:  cmdCM,
			minArgs:  0,
			usage:    "Usage: /cm [uid1],[uid2]...",
			desc:     "Promote to area CM.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"doc": {
			handler:  cmdDoc,
			minArgs:  0,
			usage:    "Usage: /doc [-c] [doc]\n-c: Clear the doc.",
			desc:     "Prints or sets the area's document.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"delete": {
			handler:  cmdDelete,
			minArgs:  0,
			usage:    "Usage: /delete",
			desc:     "Deletes the current testimony statement.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"dance": {
			handler:  cmdDance,
			minArgs:  0,
			usage:    "Usage: /dance",
			desc:     "Toggles dance mode. Flips your sprite on each message you send.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"editban": {
			handler:  cmdEditBan,
			minArgs:  2,
			usage:    "Usage: /editban [-d duration] [-r reason] <id1>,<id2>...",
			desc:     "Changes the reason of ban(s).",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"erp": {
			handler:  cmdErp,
			minArgs:  0,
			usage:    "Usage: /erp",
			desc:     "The AO ERP command. Super fun!",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"examine": {
			handler:  cmdExamine,
			minArgs:  0,
			usage:    "Usage: /examine",
			desc:     "Starts cross-examination playback.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"evimode": {
			handler:  cmdSetEviMod,
			minArgs:  1,
			usage:    "Usage: /evimode <mode>",
			desc:     "Sets the area's evidence mode.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"forcebglist": {
			handler:  cmdForceBGList,
			minArgs:  1,
			usage:    "Usage: /forcebglist <true|false>",
			desc:     "Toggles enforcing the server BG list on or off.",
			reqPerms: permissions.PermissionField["MODIFY_AREA"],
		},
		"firewall": {
			handler:  cmdFirewall,
			minArgs:  1,
			usage:    "Usage: /firewall <on|off>",
			desc:     "Enables or disables IPHub VPN/proxy screening for new connections. Requires iphub_api_key in config.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"getban": {
			handler:  cmdGetBan,
			minArgs:  0,
			usage:    "Usage: /getban [-b banid | -i ipid]",
			desc:     "Prints ban(s) matching the search parameters, or prints the 5 most recent bans.",
			reqPerms: permissions.PermissionField["BAN_INFO"],
		},
		"gas": {
			handler:  func(client *Client, _ []string, _ string) { cmdPlayers(client, []string{"-a"}, "") },
			minArgs:  0,
			usage:    "Usage: /gas",
			desc:     "Shows players in all areas.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"global": {
			handler:  cmdGlobal,
			minArgs:  1,
			usage:    "Usage: /global <message>",
			desc:     "Sends a global message.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"hide": {
			handler:  cmdHide,
			minArgs:  0,
			usage:    "Usage: /hide",
			desc:     "Toggles hiding yourself from the player list, /players, /gas, and room player counts.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"invite": {
			handler:  cmdInvite,
			minArgs:  1,
			usage:    "Usage: /invite <uid1>,<uid2>...",
			desc:     "Invites user(s) to the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"ignore": {
			handler:  cmdIgnore,
			minArgs:  1,
			usage:    "Usage: /ignore <uid>",
			desc:     "Permanently ignores a user based on their IPID. Their IC and OOC messages will no longer be shown to you, even after they reconnect.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"unignore": {
			handler:  cmdUnignore,
			minArgs:  1,
			usage:    "Usage: /unignore <uid>",
			desc:     "Removes a permanent ignore for a user, allowing their messages to be shown to you again.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"jail": {
			handler:  cmdJail,
			minArgs:  1,
			usage:    "Usage: /jail <uid> [area_id] [-d duration] [-r reason]",
			desc:     "Jails a player in the given area (or their current area). They cannot leave and are returned there on reconnect.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"kick": {
			handler:  cmdKick,
			minArgs:  3,
			usage:    "Usage: /kick -u <uid1>,<uid2>... | -i <ipid1>,<ipid2>... <reason>",
			desc:     "Kicks user(s) from the server.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"kickarea": {
			handler:  cmdAreaKick,
			minArgs:  1,
			usage:    "Usage: /kickarea <uid1>,<uid2>...",
			desc:     "Kicks user(s) from the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"kickother": {
			handler:  cmdKickOther,
			minArgs:  0,
			usage:    "Usage: /kickother",
			desc:     "Kicks all other connections sharing your IP (ghost clients).",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"lock": {
			handler:  cmdLock,
			minArgs:  0,
			usage:    "Usage: /lock [-s]\n-s: Sets the area to be spectatable.",
			desc:     "Locks the current area or sets it to spectatable.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"lockdown": {
			handler:  cmdLockdown,
			minArgs:  0,
			usage:    "Usage: /lockdown",
			desc:     "Toggles server lockdown. While active, only previously-known IPIDs can connect.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"botban": {
			handler:  cmdBotBan,
			minArgs:  0,
			usage:    "Usage: /botban",
			desc:     "Bans all spectators with less than 2 minutes of total playtime.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"setglobalnewiplimit": {
			handler:  cmdSetGlobalNewIPLimit,
			minArgs:  1,
			usage:    "Usage: /setglobalnewiplimit <limit>\nSets the global new-IP rate limit. Use 0 to disable.",
			desc:     "Sets the global new-IP rate limit at runtime.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"setglobalipwindow": {
			handler:  cmdSetGlobalIPWindow,
			minArgs:  1,
			usage:    "Usage: /setglobalipwindow <seconds>\nSets the global new-IP rate limit time window in seconds.",
			desc:     "Sets the global new-IP rate limit window at runtime.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"purgedb": {
			handler:  cmdPurgeDB,
			minArgs:  0,
			usage:    "Usage: /purgedb",
			desc:     "Purges all entries from the known-IP database. Use with caution.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"lockbg": {
			handler:  cmdLockBG,
			minArgs:  1,
			usage:    "Usage: /lockbg <true|false>",
			desc:     "Toggles locking the BG on or off.",
			reqPerms: permissions.PermissionField["MODIFY_AREA"],
		},
		"lockmusic": {
			handler:  cmdLockMusic,
			minArgs:  1,
			usage:    "Usage: /lockmusic <true|false>",
			desc:     "Toggles CM only music on or off.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"log": {
			handler:  cmdLog,
			minArgs:  1,
			usage:    "Usage: /log <area>",
			desc:     "Prints an area's log buffer.",
			reqPerms: permissions.PermissionField["LOG"],
		},
		"login": {
			handler:  cmdLogin,
			minArgs:  2,
			usage:    "Usage: /login <username> <password>",
			desc:     "Logs in as moderator.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"logout": {
			handler:  cmdLogout,
			minArgs:  0,
			usage:    "Usage: /logout",
			desc:     "Logs out as moderator.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"mkusr": {
			handler:  cmdMakeUser,
			minArgs:  3,
			usage:    "Usage: /mkusr <username> <password> <role>",
			desc:     "Creates a new moderator user.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"mod": {
			handler:  cmdMod,
			minArgs:  1,
			usage:    "Usage: /mod [-g] <message>\n-g: Send the message globally.",
			desc:     "Sends a message speaking officially as a moderator.",
			reqPerms: permissions.PermissionField["MOD_SPEAK"],
		},
		"modchat": {
			handler:  cmdModChat,
			minArgs:  1,
			usage:    "Usage: /modchat <message>",
			desc:     "Sends a message to other moderators.",
			reqPerms: permissions.PermissionField["MOD_CHAT"],
		},
		"motd": {
			handler:  cmdMotd,
			minArgs:  0,
			usage:    "Usage /motd",
			desc:     "Sends the server's message of the day.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"move": {
			handler:  cmdMove,
			minArgs:  1,
			usage:    "Usage: /move [-u <uid1,<uid2>...] <area>",
			desc:     "Moves to an area.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"mute": {
			handler:  cmdMute,
			minArgs:  1,
			usage:    "Usage: /mute [-ic][-ooc][-m][-j][-d duration][-r reason] <uid1>,<uid2>...\n-ic: Mute IC.\n-ooc: Mute OOC.\n-m: Mute music.\n-j: Mute judge.",
			desc:     "Mutes users(s) from IC, OOC, changing music, and/or judge controls.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"narrator": {
			handler:  cmdNarrator,
			minArgs:  0,
			usage:    "Usage: /narrator",
			desc:     "Toggles narrator mode on or off.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"nointpres": {
			handler:  cmdNoIntPres,
			minArgs:  1,
			usage:    "Usage: /nointpres <true|false>",
			desc:     "Toggles non-interrupting preanims in the current area on or off.",
			reqPerms: permissions.PermissionField["MODIFY_AREA"],
		},
		"parrot": {
			handler:  cmdParrot,
			minArgs:  1,
			usage:    "Usage: /parrot [-d duration][-r reason] <uid1>,<uid2>...",
			desc:     "Parrots user(s).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"play": {
			handler:  cmdPlay,
			minArgs:  1,
			usage:    "Usage: /play <song>",
			desc:     "Plays a song.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"players": {
			handler:  cmdPlayers,
			minArgs:  0,
			usage:    "Usage: /players [-a]\n-a: Target all areas.",
			desc:     "Shows players in the current or all areas.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"pair": {
			handler:  cmdPair,
			minArgs:  1,
			usage:    "Usage: /pair <uid>",
			desc:     "Sends or accepts a pair request with the specified player.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"forcepair": {
			handler:  cmdForcePair,
			minArgs:  2,
			usage:    "Usage: /forcepair <uid1> <uid2>",
			desc:     "Forces two players to pair without requiring mutual consent.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"forcename": {
			handler:  cmdForceName,
			minArgs:  2,
			usage:    "Usage: /forcename <uid> <name>",
			desc:     "Forces a player to use a specific showname in IC messages.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"unforcename": {
			handler:  cmdUnforceName,
			minArgs:  1,
			usage:    "Usage: /unforcename <uid>",
			desc:     "Removes a forced showname from a player.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"nameshuffle": {
			handler:  cmdNameShuffle,
			minArgs:  0,
			usage:    "Usage: /nameshuffle",
			desc:     "Randomly shuffles the shownames of all players in the current area.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"unnameshuffle": {
			handler:  cmdUnnameShuffle,
			minArgs:  0,
			usage:    "Usage: /unnameshuffle",
			desc:     "Restores all players' own shownames in the current area, undoing any name shuffle.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"unpair": {
			handler:  cmdUnpair,
			minArgs:  0,
			usage:    "Usage: /unpair",
			desc:     "Cancels your current pair request or active pairing.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"forcerandomchar": {
			handler:  cmdForceRandomChar,
			minArgs:  0,
			usage:    "Usage: /forcerandomchar [uid]",
			desc:     "Forces all players in the current area (or a specific player by UID) to select a random free character.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"forceunpair": {
			handler:  cmdForceUnpair,
			minArgs:  1,
			usage:    "Usage: /forceunpair <uid>",
			desc:     "Forces a player to unpair from their current pair.",
			reqPerms: permissions.PermissionField["KICK"],
		},
		"pm": {
			handler:  cmdPM,
			minArgs:  2,
			usage:    "Usage: /pm <uid1>,<uid2>... <message>",
			desc:     "Sends a private message.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"pos": {
			handler:  cmdPos,
			minArgs:  0,
			usage:    "Usage: /pos [position]",
			desc:     "Shows your current position or changes it to the given position.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"possess": {
			handler:  cmdPossess,
			minArgs:  2,
			usage:    "Usage: /possess <uid> <message>",
			desc:     "Makes target say a message once, copying their appearance.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"fullpossess": {
			handler:  cmdFullPossess,
			minArgs:  1,
			usage:    "Usage: /fullpossess <uid>",
			desc:     "Makes all YOUR IC messages appear as the target until /unpossess.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"unpossess": {
			handler:  cmdUnpossess,
			minArgs:  0,
			usage:    "Usage: /unpossess",
			desc:     "Stops full possession of a player.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"poll": {
			handler:  cmdPoll,
			minArgs:  1,
			usage:    "Usage: /poll [question]|[option1]|[option2]|[option3...]",
			desc:     "Creates a poll in the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"rmusr": {
			handler:  cmdRemoveUser,
			minArgs:  1,
			usage:    "Usage: /rmusr <username>",
			desc:     "Removes a moderator user.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"roll": {
			handler:  cmdRoll,
			minArgs:  1,
			usage:    "Usage: /roll [-p] <dice>d<sides>\n-p: Sets the roll to be private.",
			desc:     "Rolls dice.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"randomchar": {
			handler:  cmdRandomChar,
			minArgs:  0,
			usage:    "Usage: /randomchar",
			desc:     "Selects a random free character.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"randombg": {
			handler:  cmdRandomBg,
			minArgs:  0,
			usage:    "Usage: /randombg",
			desc:     "Sets the area's background to a random one from the server list. Usable once every 5 seconds.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"randomsong": {
			handler:  cmdRandomSong,
			minArgs:  0,
			usage:    "Usage: /randomsong",
			desc:     "Plays a random song from the server music list. Usable by everyone with a 10-second cooldown per user.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"rps": {
			handler:  cmdRps,
			minArgs:  1,
			usage:    "Usage: /rps <rock|paper|scissors>",
			desc:     "Play rock-paper-scissors.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"coinflip": {
			handler:  cmdCoinflip,
			minArgs:  1,
			usage:    "Usage: /coinflip <heads|tails>",
			desc:     "Challenge another player to a coinflip.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"setrole": {
			handler:  cmdChangeRole,
			minArgs:  2,
			usage:    "Usage: /setrole <username> <role>",
			desc:     "Changes a moderator user's role.",
			reqPerms: permissions.PermissionField["ADMIN"],
		},
		"spectate": {
			handler:  cmdSpectate,
			minArgs:  0,
			usage:    "Usage: /spectate [invite|uninvite <uid1>,<uid2>...]",
			desc:     "Toggles spectate mode, or invites/uninvites users to speak in IC during spectate mode.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"status": {
			handler:  cmdStatus,
			minArgs:  1,
			usage:    "Usage: /status <status>",
			desc:     "Sets the current area's status.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"suicide": {
			handler:  cmdSuicide,
			minArgs:  0,
			usage:    "Usage: /suicide",
			desc:     "If you want to die.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"summon": {
			handler:  cmdSummon,
			minArgs:  1,
			usage:    "Usage: /summon <area>",
			desc:     "Summons all users to the specified area.",
			reqPerms: permissions.PermissionField["MOVE_USERS"],
		},
		"swapevi": {
			handler:  cmdSwapEvi,
			minArgs:  2,
			usage:    "Usage: /swapevi <id1> <id2>",
			desc:     "Swaps index of evidence.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"testimony": {
			handler:  cmdTestimony,
			minArgs:  0,
			usage:    "Usage: /testimony <record|stop|play|update|insert|delete>\nUse /testimony record to start recording. Witnesses must be in /pos wit for their IC messages to be recorded.",
			desc:     "Manages the area's testimony recorder. Use /testimony record to start recording. Witnesses must be in /pos wit for their IC messages to be captured.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"testify": {
			handler:  cmdTestify,
			minArgs:  0,
			usage:    "Usage: /testify",
			desc:     "Starts recording IC messages as testimony.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"unban": {
			handler:  cmdUnban,
			minArgs:  1,
			usage:    "Usage: /unban <id1>,<id2>...",
			desc:     "Nullifies ban(s).",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"uncm": {
			handler:  cmdUnCM,
			minArgs:  0,
			usage:    "Usage: /uncm [uid1],[uid2]...",
			desc:     "Removes CM(s) from the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"update": {
			handler:  cmdUpdate,
			minArgs:  0,
			usage:    "Usage: /update",
			desc:     "Updates the current testimony statement with the next IC message from the witness.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"uninvite": {
			handler:  cmdUninvite,
			minArgs:  1,
			usage:    "Usage: /uninvite <uid1>,<uid2>...",
			desc:     "Uninvites user(s) from the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"unjail": {
			handler:  cmdUnjail,
			minArgs:  1,
			usage:    "Usage: /unjail <uid1>,<uid2>...",
			desc:     "Releases user(s) from jail.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"uncharstuck": {
			handler:  cmdUnCharStuck,
			minArgs:  1,
			usage:    "Usage: /uncharstuck <uid1>,<uid2>...",
			desc:     "Removes the character-stuck restriction from user(s).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unlock": {
			handler:  cmdUnlock,
			minArgs:  0,
			usage:    "Usage: /unlock",
			desc:     "Unlocks the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"unmute": {
			handler:  cmdUnmute,
			minArgs:  1,
			usage:    "Usage: /unmute <uid1>,<uid2>...",
			desc:     "Unmutes user(s).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"untorment": {
			handler:  cmdUntorment,
			minArgs:  1,
			usage:    "Usage: /untorment <ipid>",
			desc:     "Removes an IPID from the automod torment list.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"vote": {
			handler:  cmdVote,
			minArgs:  1,
			usage:    "Usage: /vote <option_number>",
			desc:     "Vote on the active poll.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		// Punishment commands - Text Modification
		"whisper": {
			handler:  cmdWhisper,
			minArgs:  1,
			usage:    "Usage: /whisper [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Makes messages only visible to mods and CMs.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"backward": {
			handler:  cmdBackward,
			minArgs:  1,
			usage:    "Usage: /backward [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Reverses character order in messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"stutterstep": {
			handler:  cmdStutterstep,
			minArgs:  1,
			usage:    "Usage: /stutterstep [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Doubles every word in messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"elongate": {
			handler:  cmdElongate,
			minArgs:  1,
			usage:    "Usage: /elongate [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Repeats vowels in messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"uppercase": {
			handler:  cmdUppercase,
			minArgs:  1,
			usage:    "Usage: /uppercase [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces messages to UPPERCASE.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"lowercase": {
			handler:  cmdLowercase,
			minArgs:  1,
			usage:    "Usage: /lowercase [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces messages to lowercase.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"robotic": {
			handler:  cmdRobotic,
			minArgs:  1,
			usage:    "Usage: /robotic [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with [BEEP] [BOOP] robotic sounds.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"alternating": {
			handler:  cmdAlternating,
			minArgs:  1,
			usage:    "Usage: /alternating [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Makes messages AlTeRnAtInG cAsE.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"fancy": {
			handler:  cmdFancy,
			minArgs:  1,
			usage:    "Usage: /fancy [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts messages to Unicode fancy characters.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"uwu": {
			handler:  cmdUwu,
			minArgs:  1,
			usage:    "Usage: /uwu [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts messages to UwU speak.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"pirate": {
			handler:  cmdPirate,
			minArgs:  1,
			usage:    "Usage: /pirate [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts messages to pirate speech.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"shakespearean": {
			handler:  cmdShakespearean,
			minArgs:  1,
			usage:    "Usage: /shakespearean [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts messages to Shakespearean English.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"caveman": {
			handler:  cmdCaveman,
			minArgs:  1,
			usage:    "Usage: /caveman [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts messages to caveman grunts.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Visibility/Cosmetic
		"emoji": {
			handler:  cmdEmoji,
			minArgs:  1,
			usage:    "Usage: /emoji [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces name with random emojis.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"invisible": {
			handler:  cmdInvisible,
			minArgs:  1,
			usage:    "Usage: /invisible [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Prevents user from seeing other players' messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Timing Effects
		"slowpoke": {
			handler:  cmdSlowpoke,
			minArgs:  1,
			usage:    "Usage: /slowpoke [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Delays messages before sending.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"fastspammer": {
			handler:  cmdFastspammer,
			minArgs:  1,
			usage:    "Usage: /fastspammer [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Rate limits messages heavily.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"pause": {
			handler:  cmdPause,
			minArgs:  0,
			usage:    "Usage: /pause",
			desc:     "Stops testimony recording.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"lag": {
			handler:  cmdLag,
			minArgs:  1,
			usage:    "Usage: /lag [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Batches and delays messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Social Chaos
		"subtitles": {
			handler:  cmdSubtitles,
			minArgs:  1,
			usage:    "Usage: /subtitles [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Adds confusing subtitles to messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"roulette": {
			handler:  cmdRoulette,
			minArgs:  0,
			usage:    "Usage: /roulette join | /roulette [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Join Russian Roulette game, or apply roulette punishment to user(s) (requires MUTE permission).",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"spotlight": {
			handler:  cmdSpotlight,
			minArgs:  1,
			usage:    "Usage: /spotlight [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Announces all actions publicly.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Text Processing
		"censor": {
			handler:  cmdCensor,
			minArgs:  1,
			usage:    "Usage: /censor [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces words with [CENSORED].",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"confused": {
			handler:  cmdConfused,
			minArgs:  1,
			usage:    "Usage: /confused [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Randomly reorders words in messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"paranoid": {
			handler:  cmdParanoid,
			minArgs:  1,
			usage:    "Usage: /paranoid [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Adds paranoid text to messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"drunk": {
			handler:  cmdDrunk,
			minArgs:  1,
			usage:    "Usage: /drunk [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Slurs and repeats words in messages.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"hiccup": {
			handler:  cmdHiccup,
			minArgs:  1,
			usage:    "Usage: /hiccup [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Interrupts words with 'hic'.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"whistle": {
			handler:  cmdWhistle,
			minArgs:  1,
			usage:    "Usage: /whistle [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces letters with whistles.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"mumble": {
			handler:  cmdMumble,
			minArgs:  1,
			usage:    "Usage: /mumble [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Obscures message text.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Complex Effects
		"spaghetti": {
			handler:  cmdSpaghetti,
			minArgs:  1,
			usage:    "Usage: /spaghetti [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Combines multiple random effects.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"torment": {
			handler:  cmdTorment,
			minArgs:  1,
			usage:    "Usage: /torment [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Cycles through different effects.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"rng": {
			handler:  cmdRng,
			minArgs:  1,
			usage:    "Usage: /rng [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Applies random effect from pool each message.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"essay": {
			handler:  cmdEssay,
			minArgs:  1,
			usage:    "Usage: /essay [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Requires minimum 50 characters.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Advanced
		"haiku": {
			handler:  cmdHaiku,
			minArgs:  1,
			usage:    "Usage: /haiku [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Requires 5-7-5 syllable format.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"autospell": {
			handler:  cmdAutospell,
			minArgs:  1,
			usage:    "Usage: /autospell [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Autocorrects to wrong words.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Punishment commands - Animal Sounds
		"monkey": {
			handler:  cmdMonkey,
			minArgs:  1,
			usage:    "Usage: /monkey [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with monkey noises (ook, eek, ooh ooh).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"snake": {
			handler:  cmdSnake,
			minArgs:  1,
			usage:    "Usage: /snake [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Makes messages hissss like a ssssnake.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"dog": {
			handler:  cmdDog,
			minArgs:  1,
			usage:    "Usage: /dog [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with dog sounds (woof, arf, grr, bork).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"cat": {
			handler:  cmdCat,
			minArgs:  1,
			usage:    "Usage: /cat [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with cat sounds (meow, purrr~, mrrrow).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"bird": {
			handler:  cmdBird,
			minArgs:  1,
			usage:    "Usage: /bird [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with bird sounds (tweet, chirp, squawk).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"cow": {
			handler:  cmdCow,
			minArgs:  1,
			usage:    "Usage: /cow [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with cow sounds (moo, mooo, MOOO).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"frog": {
			handler:  cmdFrog,
			minArgs:  1,
			usage:    "Usage: /frog [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with frog sounds (ribbit, croak).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"duck": {
			handler:  cmdDuck,
			minArgs:  1,
			usage:    "Usage: /duck [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with duck sounds (quack, QUACK).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"horse": {
			handler:  cmdHorse,
			minArgs:  1,
			usage:    "Usage: /horse [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with horse sounds (neigh, whinny, snort).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"lion": {
			handler:  cmdLion,
			minArgs:  1,
			usage:    "Usage: /lion [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with lion sounds (ROAR, grrr, rawr).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"zoo": {
			handler:  cmdZoo,
			minArgs:  1,
			usage:    "Usage: /zoo [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Applies a random animal sound punishment to each message.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"bunny": {
			handler:  cmdBunny,
			minArgs:  1,
			usage:    "Usage: /bunny [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces messages with bunny sounds (*thump*, *binky!*, *flops*).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"tsundere": {
			handler:  cmdTsundere,
			minArgs:  1,
			usage:    "Usage: /tsundere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "It's not like I wanted to punish you, b-baka!! Wraps messages in tsundere denial.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"yandere": {
			handler:  cmdYandere,
			minArgs:  1,
			usage:    "Usage: /yandere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Hehehe~ wraps messages in obsessive yandere flavour.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"kuudere": {
			handler:  cmdKuudere,
			minArgs:  1,
			usage:    "Usage: /kuudere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Delivers messages in cold, emotionless monotone.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"dandere": {
			handler:  cmdDandere,
			minArgs:  1,
			usage:    "Usage: /dandere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Makes messages extremely shy and hesitant with stutters.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"deredere": {
			handler:  cmdDeredere,
			minArgs:  1,
			usage:    "Usage: /deredere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Wraps messages in over-the-top lovey-dovey sweetness.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"himedere": {
			handler:  cmdHimedere,
			minArgs:  1,
			usage:    "Usage: /himedere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Makes messages imperious and royalty-like, commoner.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"kamidere": {
			handler:  cmdKamidere,
			minArgs:  1,
			usage:    "Usage: /kamidere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Delivers messages as a self-proclaimed god to unworthy mortals.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"undere": {
			handler:  cmdUndere,
			minArgs:  1,
			usage:    "Usage: /undere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces messages to agree with everything unconditionally.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"bakadere": {
			handler:  cmdBakadere,
			minArgs:  1,
			usage:    "Usage: /bakadere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Inserts clumsy, airheaded interjections into every message.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"mayadere": {
			handler:  cmdMayadere,
			minArgs:  1,
			usage:    "Usage: /mayadere [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Wraps messages in eerie, enigmatic mystery. Kukuku~",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"emoticon": {
			handler:  cmdEmoticon,
			minArgs:  1,
			usage:    "Usage: /emoticon [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces user to speak only in emoticons (:P, :D, :3, etc.).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		// Social Torment Punishments
		"lovebomb": {
			handler:  cmdLovebomb,
			minArgs:  0,
			usage:    "Usage: /lovebomb [global [off]] | /lovebomb [-d duration] [-r reason] [uid1 [uid2]]\n  global           – love-bomb all non-moderators in the area.\n  global off       – remove lovebomb from everyone in the area.\n  -d <duration>    – duration (e.g. 10m, 1h). Default: 10m. Max: 24h.\n  -r <reason>      – optional reason for the log.\n  1 uid            – apply to that uid (random area target per message).\n  2 uids           – uid1 will love-bomb uid2 specifically.",
			desc:     "Forces IC messages to be replaced with silly love declarations. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unlovebomb": {
			handler:  cmdUnlovebomb,
			minArgs:  1,
			usage:    "Usage: /unlovebomb <uid1>,<uid2>...",
			desc:     "Removes lovebomb punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"degrade": {
			handler:  cmdDegrade,
			minArgs:  1,
			usage:    "Usage: /degrade [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces IC messages to be replaced with degrading self-insults. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"undegrade": {
			handler:  cmdUndegrade,
			minArgs:  1,
			usage:    "Usage: /undegrade <uid1>,<uid2>...",
			desc:     "Removes degrade punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"tourettes": {
			handler:  cmdTourettes,
			minArgs:  1,
			usage:    "Usage: /tourettes [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Causes random outbursts to be inserted into IC messages (swearing, random objects, nonsense, animal noises). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"slang": {
			handler:  cmdSlang,
			minArgs:  1,
			usage:    "Usage: /slang [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts messages to internet slang abbreviations (e.g. 'i don't know' -> 'idk', 'got to go' -> 'gtg').",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unslang": {
			handler:  cmdUnslang,
			minArgs:  1,
			usage:    "Usage: /unslang <uid1>,<uid2>...",
			desc:     "Removes slang punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"thesaurusoverload": {
			handler:  cmdThesaurusOverload,
			minArgs:  1,
			usage:    "Usage: /thesaurusoverload [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces IC messages to use comically pompous synonyms and smug parentheticals. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unthesaurusoverload": {
			handler:  cmdUnthesaurusoverload,
			minArgs:  1,
			usage:    "Usage: /unthesaurusoverload <uid1>,<uid2>...",
			desc:     "Removes thesaurusoverload punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"valleygirl": {
			handler:  cmdValleyGirl,
			minArgs:  1,
			usage:    "Usage: /valleygirl [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Injects valley-girl filler words, vowel stretching, and dramatic tone into IC messages. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unvalleygirl": {
			handler:  cmdUnvalleygirl,
			minArgs:  1,
			usage:    "Usage: /unvalleygirl <uid1>,<uid2>...",
			desc:     "Removes valleygirl punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"babytalk": {
			handler:  cmdBabytalk,
			minArgs:  1,
			usage:    "Usage: /babytalk [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Converts IC messages to toddler-style baby talk with phonetic substitutions and stage directions. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unbabytalk": {
			handler:  cmdUnbabytalk,
			minArgs:  1,
			usage:    "Usage: /unbabytalk <uid1>,<uid2>...",
			desc:     "Removes babytalk punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"thirdperson": {
			handler:  cmdThirdPerson,
			minArgs:  1,
			usage:    "Usage: /thirdperson [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces IC messages into third-person narration using the player's display name, with mood tags. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unthirdperson": {
			handler:  cmdUnthirdperson,
			minArgs:  1,
			usage:    "Usage: /unthirdperson <uid1>,<uid2>...",
			desc:     "Removes thirdperson punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unreliablenarrator": {
			handler:  cmdUnreliableNarrator,
			minArgs:  1,
			usage:    "Usage: /unreliablenarrator [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Makes IC messages sound suspiciously unreliable with hedges, contradictions, and self-doubting commentary. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"ununreliablenarrator": {
			handler:  cmdUnunreliablenarrator,
			minArgs:  1,
			usage:    "Usage: /ununreliablenarrator <uid1>,<uid2>...",
			desc:     "Removes unreliablenarrator punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"uncannyvalley": {
			handler:  cmdUncannyValley,
			minArgs:  1,
			usage:    "Usage: /uncannyvalley [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Adds glitchy system notes to IC messages and subtly mutates the player's display name each message. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"ununcannyvalley": {
			handler:  cmdUnuncannyvalley,
			minArgs:  1,
			usage:    "Usage: /ununcannyvalley <uid1>,<uid2>...",
			desc:     "Removes uncannyvalley punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"51": {
			handler:  cmd51,
			minArgs:  1,
			usage:    "Usage: /51 [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Replaces each IC message with a random line from the 51-messages story. Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"un51": {
			handler:  cmdUn51,
			minArgs:  1,
			usage:    "Usage: /un51 <uid1>,<uid2>...",
			desc:     "Removes 51 punishment from user(s). Moderator only.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"unpunish": {
			handler:  cmdUnpunish,
			minArgs:  1,
			usage:    "Usage: /unpunish [-t punishment_type] <uid1>,<uid2>...\n-t: Specific punishment type to remove (omit to remove all).",
			desc:     "Removes punishment(s) from user(s).",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"stack": {
			handler:  cmdStack,
			minArgs:  2,
			usage:    "Usage: /stack <punishment1> <punishment2> [<punishment3>...] [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Applies multiple punishment effects to user(s) simultaneously.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"tournament": {
			handler:  cmdTournament,
			minArgs:  1,
			usage:    "Usage: /tournament <start|stop|status>",
			desc:     "Manages punishment tournament mode.",
			reqPerms: permissions.PermissionField["MUTE"],
		},
		"join-tournament": {
			handler:  cmdJoinTournament,
			minArgs:  0,
			usage:    "Usage: /join-tournament",
			desc:     "Join the active punishment tournament.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"hotpotato": {
			handler:  cmdHotPotato,
			minArgs:  0,
			usage:    "Usage: /hotpotato | /hotpotato accept | /hotpotato pass",
			desc:     "Start or join a Hot Potato mini-game event. The carrier can use /hotpotato pass to pass the potato randomly.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"giveaway": {
			handler:  cmdGiveaway,
			minArgs:  1,
			usage:    "Usage: /giveaway start <item> | /giveaway enter",
			desc:     "Start a giveaway or enter an active one.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"quickdraw": {
			handler:  cmdQuickdraw,
			minArgs:  1,
			usage:    "Usage: /quickdraw <uid> | /quickdraw accept | /quickdraw decline",
			desc:     "Challenge another player to a quickdraw duel. The loser gets a random punishment.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"russianroulette": {
			handler:  cmdRussianRoulette,
			minArgs:  0,
			usage:    "Usage: /russianroulette | /russianroulette join",
			desc:     "Start or join a Russian Roulette game. The unlucky loser receives a wild random punishment.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		// Casino commands
		"bj": {
			handler:   cmdBlackjack,
			minArgs:   0,
			usage:     "Usage: /bj join|bet <amount>|deal|hit|stand|double|split|insurance|status|leave",
			desc:      "Play blackjack. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"poker": {
			handler:   cmdPoker,
			minArgs:   0,
			usage:     "Usage: /poker join|ready|hand|check|call|bet <n>|raise <n>|fold|allin|status|leave",
			desc:      "Play Texas Hold'em poker. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"slots": {
			handler:   cmdSlots,
			minArgs:   0,
			usage:     "Usage: /slots [spin [amount]] | /slots max | /slots jackpot | /slots stats",
			desc:      "Play slot machines. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"croulette": {
			handler:   cmdCasinoRoulette,
			minArgs:   2,
			usage:     "Usage: /croulette bet <red|black|even|odd|low|high|number <n>> <amount>",
			desc:      "Play European roulette. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"baccarat": {
			handler:   cmdBaccarat,
			minArgs:   2,
			usage:     "Usage: /baccarat <player|banker|tie> <amount>",
			desc:      "Play baccarat. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"craps": {
			handler:   cmdCraps,
			minArgs:   3,
			usage:     "Usage: /craps bet <pass|nopass> <amount>",
			desc:      "Play craps. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"crash": {
			handler:   cmdCrash,
			minArgs:   1,
			usage:     "Usage: /crash bet <amount> | /crash cashout",
			desc:      "Play crash. 45s cooldown between rounds; cashout locked for first 5s (instant eject = loss). Requires casino to be enabled.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"mines": {
			handler:   cmdMines,
			minArgs:   1,
			usage:     "Usage: /mines start <mines> <bet> | /mines pick <n> | /mines cashout | /mines quit",
			desc:      "Play mines. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"keno": {
			handler:   cmdKeno,
			minArgs:   3,
			usage:     "Usage: /keno pick <numbers...> <bet>",
			desc:      "Play keno. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"wheel": {
			handler:   cmdWheel,
			minArgs:   2,
			usage:     "Usage: /wheel spin <bet>",
			desc:      "Spin the prize wheel. Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"plinko": {
			handler:   cmdPlinko,
			minArgs:   0,
			usage:     "Usage: /plinko drop <low|med|high> <bet>",
			desc:      "Drop a chip down the Plinko peg board. Risk level controls payout spread (low: 0.3x-2.5x, med: 0.1x-5x, high: 0x-12x). Requires casino to be enabled in the area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"bar": {
			handler:   cmdBar,
			minArgs:   0,
			usage:     "Usage: /bar menu | /bar buy <drink>",
			desc:      fmt.Sprintf("Visit the bar! %d drinks each with RISK and wild variance — huge wins or big losses. Use /bar menu to see all drinks.", len(barMenu)),
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"rob": {
			handler:   cmdRob,
			minArgs:   0,
			usage:     "Usage: /rob [bank|casino|vault|atm|store|mint|armored|museum]",
			desc:      "Attempt to rob a location for chips. 20% success rate — catastrophic failures drain your chips and may mute you.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"gamble": {
			handler:   cmdGamble,
			minArgs:   1,
			usage:     "Usage: /gamble hide",
			desc:      "Toggle visibility of gambling broadcast messages in the area chat.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"chips": {
			handler:   cmdChipsEnhanced,
			minArgs:   0,
			usage:     "Usage: /chips [top [n]] | [area [n]] | [give <uid> <amount>]",
			desc:      "Check your Nyathena Chip balance, view leaderboards, or give chips to another player.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"richest": {
			handler:   cmdRichest,
			minArgs:   0,
			usage:     "Usage: /richest [n]",
			desc:      "Show the global chip leaderboard (top 10 richest players by default, max 50).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"casino": {
			handler:   cmdCasino,
			minArgs:   0,
			usage:     "Usage: /casino [status]",
			desc:      "View the casino dashboard or status for the current area.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"casinoenable": {
			handler:   cmdCasinoEnable,
			minArgs:   1,
			usage:     "Usage: /casinoenable <true|false>",
			desc:      "Enables or disables the casino for this area.",
			reqPerms:  permissions.PermissionField["MODIFY_AREA"],
			casinoCmd: true,
		},
		"casinoset": {
			handler:   cmdCasinoSet,
			minArgs:   2,
			usage:     "Usage: /casinoset <minbet|maxbet|maxtables|jackpot> <value>",
			desc:      "Configures casino settings for this area.",
			reqPerms:  permissions.PermissionField["MODIFY_AREA"],
			casinoCmd: true,
		},
		"grantchips": {
			handler:   cmdGrantChips,
			minArgs:   2,
			usage:     "Usage: /grantchips <uid> <amount>",
			desc:      "Admin: Grant any amount of chips to an online player by UID.",
			reqPerms:  permissions.PermissionField["ADMIN"],
			casinoCmd: true,
		},
		"register": {
			handler:   cmdRegister,
			minArgs:   2,
			usage:     "Usage: /register <username> <password>",
			desc:      "Start creating a free player account (a captcha confirmation is required). Tracks chips, playtime, and leaderboard standings.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"captcha": {
			handler:   cmdCaptcha,
			minArgs:   1,
			usage:     "Usage: /captcha <token>",
			desc:      "Complete a pending /register by entering the captcha token you were given.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"account": {
			handler:   cmdAccount,
			minArgs:   0,
			usage:     "Usage: /account",
			desc:      "View your account profile: username, chip balance, and playtime.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"playtime": {
			handler:  cmdPlaytimeTop,
			minArgs:  0,
			usage:    "Usage: /playtime [top] [n]",
			desc:     "Show the playtime leaderboard. Displays account names for registered players.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"unscramble": {
			handler:   cmdUnscramble,
			minArgs:   0,
			usage:     "Usage: /unscramble [top [n]]",
			desc:      "Check your unscramble wins or view the unscramble leaderboard. Answer active puzzles in IC chat to win chips!",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"jobs": {
			handler:   cmdJobs,
			minArgs:   0,
			usage:     "Usage: /jobs",
			desc:      "List all available jobs that earn small chip rewards.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"jobtop": {
			handler:   cmdJobTop,
			minArgs:   0,
			usage:     "Usage: /jobtop [n]",
			desc:      "Show the job earnings leaderboard (top chip earners from jobs).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"janitor": {
			handler:   cmdJanitor,
			minArgs:   0,
			usage:     "Usage: /janitor",
			desc:      "Work as a janitor to earn chips (45-minute cooldown).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"busker": {
			handler:   cmdBusker,
			minArgs:   0,
			usage:     "Usage: /busker",
			desc:      "Busk for tips outside the courthouse to earn chips (30-minute cooldown).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"paperboy": {
			handler:   cmdPaperboy,
			minArgs:   0,
			usage:     "Usage: /paperboy",
			desc:      "Deliver newspapers and briefs to earn chips (60-minute cooldown).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"bailiffjob": {
			handler:   cmdBailiffJob,
			minArgs:   0,
			usage:     "Usage: /bailiffjob",
			desc:      "Stand guard duty as a bailiff to earn chips (2-hour cooldown).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"clerk": {
			handler:   cmdClerk,
			minArgs:   0,
			usage:     "Usage: /clerk",
			desc:      "File paperwork as a clerk to earn chips (90-minute cooldown).",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"shop": {
			handler:   cmdShop,
			minArgs:   0,
			usage:     "Usage: /shop | /shop <category> | /shop buy <item_id> | /shop items | /shop passes | /shop passive",
			desc:      "Browse the Nyathena Shop: 115+ cosmetic tags, job passes, and passive income upgrades. Categories: gambling attorney anime gamer girly meme prestige.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"settag": {
			handler:   cmdSetTag,
			minArgs:   1,
			usage:     "Usage: /settag <tag_id> | /settag none",
			desc:      "Equip or swap a purchased cosmetic tag. Your active tag appears next to your name in /gas and /players.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"favourite": {
			handler:   cmdFavourite,
			minArgs:   1,
			usage:     "Usage: /favourite <char name>",
			desc:      "Toggle a character in your wardrobe favourites. Add or remove with the same command.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		"wardrobe": {
			handler:   cmdWardrobe,
			minArgs:   0,
			usage:     "Usage: /wardrobe | /wardrobe <char name>",
			desc:      "View your saved favourite characters, or swap to one instantly.",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
		// ── Mafia / Werewolf social deduction minigame ──────────────────────
		"mafia": {
			handler:  cmdMafia,
			minArgs:  0,
			usage:    mafiaUsage,
			desc:     "Social deduction minigame (Mafia/Werewolf). Type /mafia help for subcommands.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"werewolf": {
			handler:  cmdMafia,
			minArgs:  0,
			usage:    mafiaUsage,
			desc:     "Alias for /mafia — social deduction minigame.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		// ── Lotto scratch card ───────────────────────────────────────────────
		"lotto": {
			handler:   cmdLotto,
			minArgs:   2,
			usage:     "Usage: /lotto buy <ticket_cost>",
			desc:      "Buy an instant scratch-card lottery ticket. Three matching symbols = big win!",
			reqPerms:  permissions.PermissionField["NONE"],
			casinoCmd: true,
		},
	}
}

// ParseCommand calls the appropriate function for a given command.
func ParseCommand(client *Client, command string, args []string) {
	casinoEnabled := config != nil && config.EnableCasino

	if command == "help" {
		var s []string
		for name, cmd := range Commands {
			// Hide casino/account commands when the feature is disabled.
			if cmd.casinoCmd && !casinoEnabled {
				continue
			}
			if permissions.HasPermission(client.Perms(), cmd.reqPerms) || (cmd.reqPerms == permissions.PermissionField["CM"] && client.Area().HasCM(client.Uid())) {
				s = append(s, fmt.Sprintf("- /%v: %v", name, cmd.desc))
			}
		}
		sort.Strings(s)

		// Build a context-aware header.
		var header string
		if client.Authenticated() {
			header = fmt.Sprintf("Logged in as: %v\n\n", client.ModName())
			if casinoEnabled {
				header += "👗 Your Wardrobe (Favourites):\n" +
					"  Ever feel overwhelmed by the huge character list? Save your go-to characters!\n" +
					"  • /favourite <char>   — add or remove a character from your wardrobe (toggles).\n" +
					"  • /wardrobe           — view all your saved favourites.\n" +
					"  • /wardrobe <char>    — instantly swap to any character in your wardrobe.\n" +
					fmt.Sprintf("  You can save up to %d characters. Favourites are tied to your account.\n\n", db.MaxFavourites)
			}
		} else if casinoEnabled {
			header = "💡 Player Accounts (optional):\n" +
				"  • Already have an account? Use /login <username> <password> — no new account needed.\n" +
				"  • New here? /register <username> <password> creates a free account that tracks\n" +
				"    chips, playtime, unscramble wins, and casino standings. No extra permissions granted.\n" +
				"  • 🔒 Passwords are stored with bcrypt (industry-standard one-way hashing).\n" +
				"    Your password is never stored in plain text.\n\n" +
				"👗 Wardrobe (requires free account):\n" +
				"  Ever been overwhelmed by the huge character list? Your Wardrobe lets you save a\n" +
				"  personal shortlist of favourite characters and swap to them instantly!\n" +
				"  • /favourite <char>   — add or remove a character from your saved favourites.\n" +
				"  • /wardrobe           — view your personal favourites list.\n" +
				"  • /wardrobe <char>    — swap to any character in your wardrobe in one command.\n" +
				fmt.Sprintf("  Save up to %d characters — no more scrolling through the entire list!\n\n", db.MaxFavourites) +
				"🎰 Casino Tips:\n" +
				"  • /chips                        — check your chip balance.\n" +
				"  • /chips give <uid> <amount>    — send chips to another player.\n" +
				"  • /chips top                    — see the global chip leaderboard.\n\n" +
				"💰 Earn Chips Without Gambling:\n" +
				"  • /jobs                         — list all available jobs (small rewards, unique cooldowns).\n" +
				"  • /janitor /busker /paperboy     — work a job to earn chips.\n" +
				"  • /bailiffjob /clerk             — more jobs available.\n" +
				"  • /jobtop                        — see the job earnings leaderboard.\n" +
				"  • Unscramble events post every 30–60 min — answer in IC chat to win 10 chips!\n" +
				"  • /unscramble                   — see your wins & any active puzzle.\n" +
				"  • /unscramble top               — see the unscramble leaderboard.\n\n"
		}

		client.SendServerMessage(header + "Recognized commands:\n" + strings.Join(s, "\n") + "\n\nTo view detailed usage on a command, do /<command> -h")
		return
	}

	cmd := Commands[command]
	if cmd.handler == nil {
		client.SendServerMessage("Invalid command.")
		return
	}
	// Block casino/account commands when the feature is disabled server-wide.
	if cmd.casinoCmd && !casinoEnabled {
		client.SendServerMessage("The casino and player account system is not enabled on this server.")
		return
	}
	if permissions.HasPermission(client.Perms(), cmd.reqPerms) || (cmd.reqPerms == permissions.PermissionField["CM"] && client.Area().HasCM(client.Uid())) {
		if sliceutil.ContainsString(args, "-h") {
			client.SendServerMessage(cmd.usage)
			return
		} else if len(args) < cmd.minArgs {
			client.SendServerMessage("Not enough arguments.\n" + cmd.usage)
			return
		}
		cmd.handler(client, args, cmd.usage)
	} else {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
}

// Handles /about
