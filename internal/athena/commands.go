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
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
	"github.com/xhit/go-str2duration/v2"
)

type Command struct {
	handler  func(client *Client, args []string, usage string)
	minArgs  int
	usage    string
	desc     string
	reqPerms uint64
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
			usage:    "Usage: /ban -u <uid1>,<uid2>... | -i <ipid1>,<ipid2>... [-d duration] <reason>",
			desc:     "Bans user(s) from the server.",
			reqPerms: permissions.PermissionField["BAN"],
		},
		"bg": {
			handler:  cmdBg,
			minArgs:  1,
			usage:    "Usage: /bg <background>",
			desc:     "Sets the area's background.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"charselect": {
			handler:  cmdCharSelect,
			minArgs:  0,
			usage:    "Usage: /charselect [uid1],[uid2]...",
			desc:     "Return to character select.",
			reqPerms: permissions.PermissionField["NONE"],
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
		"editban": {
			handler:  cmdEditBan,
			minArgs:  2,
			usage:    "Usage: /editban [-d duration] [-r reason] <id1>,<id2>...",
			desc:     "Changes the reason of ban(s).",
			reqPerms: permissions.PermissionField["BAN"],
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
		"getban": {
			handler:  cmdGetBan,
			minArgs:  0,
			usage:    "Usage: /getban [-b banid | -i ipid]",
			desc:     "Prints ban(s) matching the search parameters, or prints the 5 most recent bans.",
			reqPerms: permissions.PermissionField["BAN_INFO"],
		},
		"global": {
			handler:  cmdGlobal,
			minArgs:  1,
			usage:    "Usage: /global <message>",
			desc:     "Sends a global message.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"invite": {
			handler:  cmdInvite,
			minArgs:  1,
			usage:    "Usage: /invite <uid1>,<uid2>...",
			desc:     "Invites user(s) to the current area.",
			reqPerms: permissions.PermissionField["CM"],
		},
		"jail": {
			handler:  cmdJail,
			minArgs:  1,
			usage:    "Usage: /jail <uid> [-d duration] [-r reason]",
			desc:     "Jails a player in their current area.",
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
		"lock": {
			handler:  cmdLock,
			minArgs:  0,
			usage:    "Usage: /lock [-s]\n-s: Sets the area to be spectatable.",
			desc:     "Locks the current area or sets it to spectatable.",
			reqPerms: permissions.PermissionField["CM"],
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
		"unpair": {
			handler:  cmdUnpair,
			minArgs:  0,
			usage:    "Usage: /unpair",
			desc:     "Cancels your current pair request or active pairing.",
			reqPerms: permissions.PermissionField["NONE"],
		},
		"pm": {
			handler:  cmdPM,
			minArgs:  2,
			usage:    "Usage: /pm <uid1>,<uid2>... <message>",
			desc:     "Sends a private message.",
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
		"status": {
			handler:  cmdStatus,
			minArgs:  1,
			usage:    "Usage: /status <status>",
			desc:     "Sets the current area's status.",
			reqPerms: permissions.PermissionField["CM"],
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
			usage:    "Usage /testimony <record|stop|play|update|insert|delete>",
			desc:     "Updates the current area's testimony recorder, or prints current testimony.",
			reqPerms: permissions.PermissionField["NONE"],
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
			minArgs:  1,
			usage:    "Usage: /pause [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Forces wait between messages.",
			reqPerms: permissions.PermissionField["MUTE"],
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
			minArgs:  1,
			usage:    "Usage: /roulette [-d duration] [-r reason] <uid1>,<uid2>...",
			desc:     "Random chance message doesn't send.",
			reqPerms: permissions.PermissionField["MUTE"],
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
	}
}

// ParseCommand calls the appropriate function for a given command.
func ParseCommand(client *Client, command string, args []string) {
	if command == "help" {
		var s []string
		for name, cmd := range Commands {
			if permissions.HasPermission(client.Perms(), cmd.reqPerms) || (cmd.reqPerms == permissions.PermissionField["CM"] && client.Area().HasCM(client.Uid())) {
				s = append(s, fmt.Sprintf("- /%v: %v", name, cmd.desc))
			}
		}
		sort.Strings(s)
		client.SendServerMessage("Recognized commands:\n" + strings.Join(s, "\n") + "\n\nTo view detailed usage on a command, do /<command> -h")
		return
	}

	cmd := Commands[command]
	if cmd.handler == nil {
		client.SendServerMessage("Invalid command.")
		return
	} else if permissions.HasPermission(client.Perms(), cmd.reqPerms) || (cmd.reqPerms == permissions.PermissionField["CM"] && client.Area().HasCM(client.Uid())) {
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
func cmdAbout(client *Client, _ []string, _ string) {
	client.SendServerMessage(fmt.Sprintf("Running Athena version %v.\nAthena is open source software; for documentation, bug reports, and source code, see: %v",
		version, "https://github.com/MangosArentLiterature/Athena."))
}

// Handles /allowcms
func cmdAllowCMs(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetCMsAllowed(true)
		result = "allowed"
	case "false":
		client.Area().SetCMsAllowed(false)
		result = "disallowed"
	default:
		client.SendServerMessage("Argument not recognized.")
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v CMs in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set allowing CMs to %v.", args[0]), false)
}

// Handles /allowiniswap
func cmdAllowIniswap(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetIniswapAllowed(true)
		result = "enabled"
	case "false":
		client.Area().SetIniswapAllowed(false)
		result = "disabled"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v iniswapping in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set iniswapping to %v.", args[0]), false)
}

// Handles /areainfo
func cmdAreaInfo(client *Client, _ []string, _ string) {
	out := fmt.Sprintf("\nBG: %v\nEvi mode: %v\nAllow iniswap: %v\nNon-interrupting pres: %v\nCMs allowed: %v\nForce BG list: %v\nBG locked: %v\nMusic locked: %v",
		client.Area().Background(), client.Area().EvidenceMode().String(), client.Area().IniswapAllowed(), client.Area().NoInterrupt(),
		client.Area().CMsAllowed(), client.Area().ForceBGList(), client.Area().LockBG(), client.Area().LockMusic())
	client.SendServerMessage(out)
}

// Handles /ban
func cmdBan(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	uids := &[]string{}
	ipids := &[]string{}
	flags.Var(&cmdParamList{uids}, "u", "")
	flags.Var(&cmdParamList{ipids}, "i", "")
	duration := flags.String("d", config.BanLen, "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	var toBan []*Client
	if len(*uids) > 0 {
		toBan = getUidList(*uids)
	} else if len(*ipids) > 0 {
		toBan = getIpidList(*ipids)
	} else {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	banTime, reason := time.Now().UTC().Unix(), strings.Join(flags.Args(), " ")
	var until int64
	if strings.ToLower(*duration) == "perma" {
		until = -1
	} else {
		parsedDur, err := str2duration.ParseDuration(*duration)
		if err != nil {
			client.SendServerMessage("Failed to ban: Cannot parse duration.")
			return
		}
		until = time.Now().UTC().Add(parsedDur).Unix()
	}

	var count int
	var report string
	for _, c := range toBan {
		id, err := db.AddBan(c.Ipid(), c.Hdid(), banTime, until, reason, client.ModName())
		if err != nil {
			continue
		}
		var untilS string
		if until == -1 {
			untilS = "∞"
		} else {
			untilS = time.Unix(until, 0).UTC().Format("02 Jan 2006 15:04 MST")
		}
		if !strings.Contains(report, c.Ipid()) {
			report += c.Ipid() + ", "
		}
		c.SendPacket("KB", fmt.Sprintf("%v\nUntil: %v\nID: %v", reason, untilS, id))
		c.conn.Close()
		count++
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Banned %v clients.", count))
	sendPlayerArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Banned %v from server for %v: %v.", report, *duration, reason), true)
}

// Handles /bg
func cmdBg(client *Client, args []string, _ string) {
	if client.Area().LockBG() && !permissions.HasPermission(client.Perms(), permissions.PermissionField["MODIFY_AREA"]) {
		client.SendServerMessage("You do not have permission to change the background in this area.")
		return
	}

	arg := strings.Join(args, " ")

	if client.Area().ForceBGList() && !sliceutil.ContainsString(backgrounds, arg) {
		client.SendServerMessage("Invalid background.")
		return
	}
	client.Area().SetBackground(arg)
	writeToArea(client.Area(), "BN", arg)
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the background to %v.", client.OOCName(), arg))
	addToBuffer(client, "CMD", fmt.Sprintf("Set BG to %v.", arg), false)
}

// Handles /charselect
func cmdCharSelect(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.ChangeCharacter(-1)
		client.SendPacket("DONE")
	} else {
		if !client.HasCMPermission() {
			client.SendServerMessage("You do not have permission to use that command.")
			return
		}
		toChange := getUidList(strings.Split(args[0], ","))
		var count int
		var report string
		for _, c := range toChange {
			if c.Area() != client.Area() || c.CharID() == -1 {
				continue
			}
			c.ChangeCharacter(-1)
			c.SendPacket("DONE")
			c.SendServerMessage("You were moved back to character select.")
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Moved %v users to character select.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Moved %v to character select.", report), false)
	}
}

// Handles /cm
func cmdCM(client *Client, args []string, _ string) {
	if client.CharID() == -1 {
		client.SendServerMessage("You are spectating; you cannot become a CM.")
		return
	} else if !client.Area().CMsAllowed() && !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}

	if len(args) == 0 {
		if client.Area().HasCM(client.Uid()) {
			client.SendServerMessage("You are already a CM in this area.")
			return
		} else if len(client.Area().CMs()) > 0 && !permissions.HasPermission(client.Perms(), permissions.PermissionField["CM"]) {
			client.SendServerMessage("This area already has a CM.")
			return
		}
		client.Area().AddCM(client.Uid())
		client.SendServerMessage("Successfully became a CM.")
		addToBuffer(client, "CMD", "CMed self.", false)
	} else {
		if !client.HasCMPermission() {
			client.SendServerMessage("You do not have permission to use that command.")
			return
		}
		toCM := getUidList(strings.Split(args[0], ","))
		var count int
		var report string
		for _, c := range toCM {
			if c.Area() != client.Area() || c.Area().HasCM(c.Uid()) {
				continue
			}
			c.Area().AddCM(c.Uid())
			c.SendServerMessage("You have become a CM in this area.")
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("CMed %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("CMed %v.", report), false)
	}
	sendCMArup()
}

// Handles /doc
func cmdDoc(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	clear := flags.Bool("c", false, "")
	flags.Parse(args)
	if len(args) == 0 {
		if client.Area().Doc() == "" {
			client.SendServerMessage("This area does not have a doc set.")
			return
		}
		client.SendServerMessage(client.Area().Doc())
		return
	} else {
		if !client.HasCMPermission() {
			client.SendServerMessage("You do not have permission to change the doc.")
			return
		} else if *clear {
			client.Area().SetDoc("")
			sendAreaServerMessage(client.Area(), fmt.Sprintf("%v cleared the doc.", client.OOCName()))
			return
		} else if len(flags.Args()) != 0 {
			client.Area().SetDoc(flags.Arg(0))
			sendAreaServerMessage(client.Area(), fmt.Sprintf("%v updated the doc.", client.OOCName()))
			return
		}
	}
}

// Handles /editban
func cmdEditBan(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.String("d", "", "")
	reason := flags.String("r", "", "")
	flags.Parse(args)
	useDur := *duration != ""
	useReason := *reason != ""

	if len(flags.Args()) == 0 || (!useDur && !useReason) {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toUpdate := strings.Split(flags.Arg(0), ",")
	var until int64
	if useDur {
		if strings.ToLower(*duration) == "perma" {
			until = -1
		} else {
			parsedDur, err := str2duration.ParseDuration(*duration)
			if err != nil {
				client.SendServerMessage("Failed to ban: Cannot parse duration.")
				return
			}
			until = time.Now().UTC().Add(parsedDur).Unix()
		}
	}

	var report string
	for _, s := range toUpdate {
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		if useDur {
			err = db.UpdateDuration(id, until)
			if err != nil {
				continue
			}
		}
		if useReason {
			err = db.UpdateReason(id, *reason)
			if err != nil {
				continue
			}
		}
		report += fmt.Sprintf("%v, ", s)
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Updated bans: %v", report))
	if useDur {
		addToBuffer(client, "CMD", fmt.Sprintf("Edited bans: %v to duration: %v.", report, duration), true)
	}
	if useReason {
		addToBuffer(client, "CMD", fmt.Sprintf("Edited bans: %v to reason: %v.", report, reason), true)
	}
}

// Handles /evimode
func cmdSetEviMod(client *Client, args []string, _ string) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to change the evidence mode.")
		return
	}
	switch args[0] {
	case "mods":
		if !permissions.HasPermission(client.Perms(), permissions.PermissionField["MOD_EVI"]) {
			client.SendServerMessage("You do not have permission for this evidence mode.")
			return
		}
		client.Area().SetEvidenceMode(area.EviMods)
	case "cms":
		client.Area().SetEvidenceMode(area.EviCMs)
	case "any":
		client.Area().SetEvidenceMode(area.EviAny)
	default:
		client.SendServerMessage("Invalid evidence mode.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the evidence mode to %v.", client.OOCName(), args[0]))
	addToBuffer(client, "CMD", fmt.Sprintf("Set the evidence mode to %v.", args[0]), false)
}

// Handles /forcebglist
func cmdForceBGList(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetForceBGList(true)
		result = "enforced"
	case "false":
		client.Area().SetForceBGList(false)
		result = "unenforced"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v the BG list in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set the BG list to %v.", args[0]), false)
}

// Handles /getban
func cmdGetBan(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	banid := flags.Int("b", -1, "")
	ipid := flags.String("i", "", "")
	flags.Parse(args)
	s := "Bans:\n----------"
	entry := func(b db.BanInfo) string {
		var d string
		if b.Duration == -1 {
			d = "∞"
		} else {
			d = time.Unix(b.Duration, 0).UTC().Format("02 Jan 2006 15:04 MST")
		}

		return fmt.Sprintf("\nID: %v\nIPID: %v\nHDID: %v\nBanned on: %v\nUntil: %v\nReason: %v\nModerator: %v\n----------",
			b.Id, b.Ipid, b.Hdid, time.Unix(b.Time, 0).UTC().Format("02 Jan 2006 15:04 MST"), d, b.Reason, b.Moderator)
	}
	if *banid > 0 {
		b, err := db.GetBan(db.BANID, *banid)
		if err != nil || len(b) == 0 {
			client.SendServerMessage("No ban with that ID exists.")
			return
		}
		s += entry(b[0])
	} else if *ipid != "" {
		bans, err := db.GetBan(db.IPID, *ipid)
		if err != nil || len(bans) == 0 {
			client.SendServerMessage("No bans with that IPID exist.")
			return
		}
		for _, b := range bans {
			s += entry(b)
		}
	} else {
		bans, err := db.GetRecentBans()
		if err != nil {
			logger.LogErrorf("while getting recent bans: %v", err)
			client.SendServerMessage("An unexpected error occured.")
			return
		}
		for _, b := range bans {
			s += entry(b)
		}
	}
	client.SendServerMessage(s)
}

// Handles /global
func cmdGlobal(client *Client, args []string, _ string) {
	if !client.CanSpeakOOC() {
		client.SendServerMessage("You are muted from sending OOC messages.")
		return
	}
	writeToAll("CT", fmt.Sprintf("[GLOBAL] %v", client.OOCName()), strings.Join(args, " "), "1")
}

// Handles /invite
func cmdInvite(client *Client, args []string, _ string) {
	if client.Area().Lock() == area.LockFree {
		client.SendServerMessage("This area is unlocked.")
		return
	}
	toInvite := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toInvite {
		if client.Area().AddInvited(c.Uid()) {
			c.SendServerMessage(fmt.Sprintf("You were invited to area %v.", client.Area().Name()))
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Invited %v users.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Invited %v to the area.", report), false)
}

// Handles /kick
func cmdKick(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	uids := &[]string{}
	ipids := &[]string{}
	flags.Var(&cmdParamList{uids}, "u", "")
	flags.Var(&cmdParamList{ipids}, "i", "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	var toKick []*Client
	if len(*uids) > 0 {
		toKick = getUidList(*uids)
	} else if len(*ipids) > 0 {
		toKick = getIpidList(*ipids)
	} else {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	var count int
	var report string
	reason := strings.Join(flags.Args(), " ")
	for _, c := range toKick {
		report += c.Ipid() + ", "
		c.SendPacket("KK", reason)
		c.conn.Close()
		count++
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Kicked %v clients.", count))
	sendPlayerArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Kicked %v from server for reason: %v.", report, reason), true)
}

// Handles /kickarea
func cmdAreaKick(client *Client, args []string, _ string) {
	if client.Area() == areas[0] {
		client.SendServerMessage("Failed to kick: Cannot kick a user from area 0.")
		return
	}
	toKick := getUidList(strings.Split(args[0], ","))

	var count int
	var report string
	for _, c := range toKick {
		if c.Area() != client.Area() || permissions.HasPermission(c.Perms(), permissions.PermissionField["BYPASS_LOCK"]) {
			continue
		}
		if c == client {
			client.SendServerMessage("You can't kick yourself from the area.")
			continue
		}
		c.ChangeArea(areas[0])
		c.SendServerMessage("You were kicked from the area!")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Kicked %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Kicked %v from area.", report), false)
}

// Handles /lock
func cmdLock(client *Client, args []string, _ string) {
	if sliceutil.ContainsString(args, "-s") { // Set area to spectatable.
		client.Area().SetLock(area.LockSpectatable)
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the area to spectatable.", client.OOCName()))
		addToBuffer(client, "CMD", "Set the area to spectatable.", false)
	} else { // Normal lock.
		if client.Area().Lock() == area.LockLocked {
			client.SendServerMessage("This area is already locked.")
			return
		} else if client.Area() == areas[0] {
			client.SendServerMessage("You cannot lock area 0.")
			return
		}
		client.Area().SetLock(area.LockLocked)
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v locked the area.", client.OOCName()))
		addToBuffer(client, "CMD", "Locked the area.", false)
	}
	for c := range clients.GetAllClients() {
		if c.Area() == client.Area() {
			c.Area().AddInvited(c.Uid())
		}
	}
	sendLockArup()
}

// Handles /lockbg
func cmdLockBG(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetLockBG(true)
		result = "locked"
	case "false":
		client.Area().SetLockBG(false)
		result = "unlocked"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v the background in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set the background to %v.", args[0]), false)
}

// Handles /lockmusic
func cmdLockMusic(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetLockMusic(true)
		result = "enabled"
	case "false":
		client.Area().SetLockMusic(false)
		result = "disabled"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v CM-only music in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set CM-only music list to %v.", args[0]), false)
}

// Handles /log
func cmdLog(client *Client, args []string, _ string) {
	wantedArea, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid area.")
		return
	}
	for i, a := range areas {
		if i == wantedArea {
			client.SendServerMessage(strings.Join(a.Buffer(), "\n"))
			return
		}
	}
	client.SendServerMessage("Invalid area.")
}

// Handles /login
func cmdLogin(client *Client, args []string, _ string) {
	if client.Authenticated() {
		client.SendServerMessage("You are already logged in.")
		return
	}
	auth, perms := db.AuthenticateUser(args[0], []byte(args[1]))
	addToBuffer(client, "AUTH", fmt.Sprintf("Attempted login as %v.", args[0]), true)
	if auth {
		client.SetAuthenticated(true)
		client.SetPerms(perms)
		client.SetModName(args[0])
		if permissions.IsModerator(perms) {
			client.SendServerMessage("Logged in as moderator.")
		}
		client.SendPacket("AUTH", "1")
		client.SendServerMessage(fmt.Sprintf("Welcome, %v.", args[0]))
		addToBuffer(client, "AUTH", fmt.Sprintf("Logged in as %v.", args[0]), true)
		return
	}
	client.SendPacket("AUTH", "0")
	addToBuffer(client, "AUTH", fmt.Sprintf("Failed login as %v.", args[0]), true)
}

// Handles /logout
func cmdLogout(client *Client, _ []string, _ string) {
	if !client.Authenticated() {
		client.SendServerMessage("You are not logged in.")
	}
	addToBuffer(client, "AUTH", fmt.Sprintf("Logged out as %v.", client.ModName()), true)
	client.RemoveAuth()
}

// Handles /mkusr
func cmdMakeUser(client *Client, args []string, _ string) {
	if db.UserExists(args[0]) {
		client.SendServerMessage("User already exists.")
		return
	}

	role, err := getRole(args[2])
	if err != nil {
		client.SendServerMessage("Invalid role.")
		return
	}
	err = db.CreateUser(args[0], []byte(args[1]), role.GetPermissions())
	if err != nil {
		logger.LogError(err.Error())
		client.SendServerMessage("Invalid username/password.")
		return
	}
	client.SendServerMessage("User created.")
	addToBuffer(client, "CMD", fmt.Sprintf("Created user %v.", args[0]), true)
}

// Handles /mod
func cmdMod(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	global := flags.Bool("g", false, "")
	flags.Parse(args)
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	msg := strings.Join(flags.Args(), " ")
	if *global {
		writeToAll("CT", fmt.Sprintf("[MOD] [GLOBAL] %v", client.OOCName()), msg, "1")
	} else {
		writeToArea(client.Area(), "CT", fmt.Sprintf("[MOD] %v", client.OOCName()), msg, "1")
	}
	addToBuffer(client, "OOC", msg, false)
}

// Handles /modchat
func cmdModChat(client *Client, args []string, _ string) {
	msg := strings.Join(args, " ")
	for c := range clients.GetAllClients() {
		if permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			c.SendPacket("CT", fmt.Sprintf("[MODCHAT] %v", client.OOCName()), msg, "1")
		}
	}
}

// Handles /motd
func cmdMotd(client *Client, _ []string, _ string) {
	client.SendServerMessage(config.Motd)
}

// Handles /move
func cmdMove(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	uids := &[]string{}
	flags.Var(&cmdParamList{uids}, "u", "")
	flags.Parse(args)

	if len(flags.Args()) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	areaID, err := strconv.Atoi(flags.Arg(0))
	if err != nil || areaID < 0 || areaID > len(areas)-1 {
		client.SendServerMessage("Invalid area.")
		return
	}
	wantedArea := areas[areaID]

	if len(*uids) > 0 {
		if !permissions.HasPermission(client.Perms(), permissions.PermissionField["MOVE_USERS"]) {
			client.SendServerMessage("You do not have permission to use that command.")
			return
		}
		toMove := getUidList(*uids)
		var count int
		var report string
		for _, c := range toMove {
			if !c.ChangeArea(wantedArea) {
				continue
			}
			c.SendServerMessage(fmt.Sprintf("You were moved to %v.", wantedArea.Name()))
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Moved %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Moved %v to %v.", report, wantedArea.Name()), false)
	} else {
		if !client.ChangeArea(wantedArea) {
			client.SendServerMessage("You are not invited to that area.")
		}
		client.SendServerMessage(fmt.Sprintf("Moved to %v.", wantedArea.Name()))
	}
}

// Handles /summon
func cmdSummon(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	
	areaID, err := strconv.Atoi(args[0])
	if err != nil || areaID < 0 || areaID > len(areas)-1 {
		client.SendServerMessage("Invalid area.")
		return
	}
	wantedArea := areas[areaID]
	
	// Get all connected clients
	allClients := clients.GetAllClients()
	
	var count int
	var reportBuilder strings.Builder
	
	// Move each client to the target area
	for c := range allClients {
		if !c.ChangeArea(wantedArea) {
			continue
		}
		
		// Send appropriate message based on whether this is the admin
		if c == client {
			c.SendServerMessage(fmt.Sprintf("Summoned all users to %v.", wantedArea.Name()))
		} else {
			c.SendServerMessage(fmt.Sprintf("You were summoned to %v.", wantedArea.Name()))
		}
		
		if reportBuilder.Len() > 0 {
			reportBuilder.WriteString(", ")
		}
		reportBuilder.WriteString(fmt.Sprintf("%v", c.Uid()))
		count++
	}
	
	report := reportBuilder.String()
	if count > 0 {
		addToBuffer(client, "CMD", fmt.Sprintf("Summoned %v user(s) (%v) to %v.", count, report, wantedArea.Name()), false)
	} else {
		client.SendServerMessage("No users were summoned.")
	}
}

// Handles /mute
func cmdMute(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	music := flags.Bool("m", false, "")
	jud := flags.Bool("j", false, "")
	ic := flags.Bool("ic", false, "")
	ooc := flags.Bool("ooc", false, "")
	duration := flags.Int("d", -1, "")
	flags.Parse(args)

	var m MuteState
	switch {
	case *ic && *ooc:
		m = ICOOCMuted
	case *ic:
		m = ICMuted
	case *ooc:
		m = OOCMuted
	case *music:
		m = MusicMuted
	case *jud:
		m = JudMuted
	default:
		m = ICMuted
	}
	msg := fmt.Sprintf("You have been muted from %v", m.String())
	if *duration != -1 {
		msg += fmt.Sprintf(" for %v seconds", *duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toMute := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var report string
	for _, c := range toMute {
		if c.Muted() == m {
			continue
		}
		c.SetMuted(m)
		if *duration == -1 {
			c.SetUnmuteTime(time.Time{})
		} else {
			c.SetUnmuteTime(time.Now().UTC().Add(time.Duration(*duration) * time.Second))
		}
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Muted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Muted %v.", report), false)
}

// Handles /narrator
func cmdNarrator(client *Client, _ []string, _ string) {
	client.ToggleNarrator()
}

// Handles /nointpres
func cmdNoIntPres(client *Client, args []string, _ string) {
	var result string
	switch args[0] {
	case "true":
		client.Area().SetNoInterrupt(true)
		result = "enabled"
	case "false":
		client.Area().SetNoInterrupt(false)
		result = "disabled"
	default:
		client.SendServerMessage("Argument not recognized.")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v has %v non-interrupting preanims in this area.", client.OOCName(), result))
	addToBuffer(client, "CMD", fmt.Sprintf("Set non-interrupting preanims to %v.", args[0]), false)
}

// Handles /parrot
func cmdParrot(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	duration := flags.Int("d", -1, "")
	flags.Parse(args)
	msg := "You have been turned into a parrot"
	if *duration != -1 {
		msg += fmt.Sprintf(" for %v seconds", *duration)
	}
	if *reason != "" {
		msg += " for reason: " + *reason
	}
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	toParrot := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var report string
	for _, c := range toParrot {
		if c.Muted() != Unmuted {
			continue
		}
		c.SetMuted(ParrotMuted)
		if *duration == -1 {
			c.SetUnmuteTime(time.Time{})
		} else {
			c.SetUnmuteTime(time.Now().UTC().Add(time.Duration(*duration) * time.Second))
		}
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Parroted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Parroted %v.", report), false)
}

// Handles /play
func cmdPlay(client *Client, args []string, _ string) {
	if !client.CanChangeMusic() {
		client.SendServerMessage("You are not allowed to change the music in this area.")
		return
	}
	s := strings.Join(args, " ")

	// Check if the song we got is a URL for streaming
	if _, err := url.ParseRequestURI(s); err == nil {
		s, err = url.QueryUnescape(s) // Unescape any URL encoding
		if err != nil {
			client.SendServerMessage("Error parsing URL.")
			return
		}
	}
	writeToArea(client.Area(), "MC", s, fmt.Sprint(client.CharID()), client.Showname(), "1", "0")
}

// Handles /players
func cmdPlayers(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	all := flags.Bool("a", false, "")
	flags.Parse(args)
	out := "\nPlayers\n----------\n"
	entry := func(c *Client, auth bool) string {
		s := fmt.Sprintf("[%v] %v\n", c.Uid(), c.CurrentCharacter())
		if auth {
			if permissions.IsModerator(c.Perms()) {
				s += fmt.Sprintf("Mod: %v\n", c.ModName())
			}
			s += fmt.Sprintf("IPID: %v\n", c.Ipid())
		}
		if c.OOCName() != "" {
			s += fmt.Sprintf("OOC: %v\n", c.OOCName())
		}
		return s
	}
	if *all {
		for _, a := range areas {
			out += fmt.Sprintf("%v:\n%v players online.\n", a.Name(), a.PlayerCount())
			for c := range clients.GetAllClients() {
				if c.Area() == a {
					out += entry(c, permissions.HasPermission(client.Perms(), permissions.PermissionField["BAN_INFO"]))
				}
			}
			out += "----------\n"
		}
	} else {
		out += fmt.Sprintf("%v:\n%v players online.\n", client.Area().Name(), client.Area().PlayerCount())
		for c := range clients.GetAllClients() {
			if c.Area() == client.Area() {
				out += entry(c, permissions.HasPermission(client.Perms(), permissions.PermissionField["BAN_INFO"]))
			}
		}
	}
	client.SendServerMessage(out)
}

// Handles /pm
func cmdPM(client *Client, args []string, _ string) {
	msg := strings.Join(args[1:], " ")
	toPM := getUidList(strings.Split(args[0], ","))
	for _, c := range toPM {
		c.SendPacket("CT", fmt.Sprintf("[PM] %v", client.OOCName()), msg, "1")
	}
}

// Handles /pair
func cmdPair(client *Client, args []string, _ string) {
	if client.CharID() < 0 {
		client.SendServerMessage("You have not selected a character.")
		return
	}

	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}

	if target == client {
		client.SendServerMessage("You cannot pair with yourself.")
		return
	}

	if target.Area() != client.Area() {
		client.SendServerMessage("That player is not in your area.")
		return
	}

	if target.CharID() < 0 {
		client.SendServerMessage("That player has not selected a character.")
		return
	}

	client.SetPairWantedID(target.CharID())

	// Check if the target is already requesting to pair with us (mutual pairing).
	if target.PairWantedID() == client.CharID() {
		client.SendServerMessage(fmt.Sprintf("Now pairing with %v.", target.OOCName()))
		target.SendServerMessage(fmt.Sprintf("%v accepted your pair request.", client.OOCName()))
	} else {
		client.SendServerMessage(fmt.Sprintf("Sent pair request to %v.", target.OOCName()))
		target.SendServerMessage(fmt.Sprintf("%v wants to pair with you. Type /pair %v to accept.", client.OOCName(), client.Uid()))
	}
}

// Handles /unpair
func cmdUnpair(client *Client, _ []string, _ string) {
	if client.PairWantedID() == -1 {
		client.SendServerMessage("You do not have an active pair request.")
		return
	}

	// Notify any client that was paired with us.
	for c := range clients.GetAllClients() {
		if c != client && c.Area() == client.Area() && c.PairWantedID() == client.CharID() {
			c.SendServerMessage(fmt.Sprintf("%v has cancelled the pair.", client.OOCName()))
		}
	}

	client.SetPairWantedID(-1)
	client.SendServerMessage("Pair cancelled.")
}

// Handles /possess - one-time possession that mimics target's appearance for a single message
func cmdPossess(client *Client, args []string, _ string) {
	// Get the target UID
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	// Get the target client
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}

	// Validate CharID is within bounds
	if target.CharID() < 0 || target.CharID() >= len(characters) {
		client.SendServerMessage("Target has an invalid character.")
		return
	}

	// Get the message to send
	msg := strings.Join(args[1:], " ")
	if msg == "" {
		client.SendServerMessage("Message cannot be empty.")
		return
	}

	// Encode the message
	encodedMsg := encode(msg)

	// Get the target's current emote from their pair info, or use "normal" as fallback
	targetEmote := target.PairInfo().emote
	if targetEmote == "" {
		targetEmote = "normal"
	}

	// Get the target's displayed character name (handles iniswap)
	// Use PairInfo().name if available (contains iniswapped character), otherwise use their actual character
	targetCharName := target.PairInfo().name
	if targetCharName == "" {
		// Defensive bounds check before accessing characters array
		if target.CharID() >= 0 && target.CharID() < len(characters) {
			targetCharName = characters[target.CharID()]
		} else {
			client.SendServerMessage("Target has an invalid character.")
			return
		}
	}

	// Get the character ID for the displayed character
	targetCharID := getCharacterID(targetCharName)
	if targetCharID == -1 {
		// If character name is not found, fall back to target's actual character
		targetCharID = target.CharID()
		// Defensive bounds check before accessing characters array
		if targetCharID >= 0 && targetCharID < len(characters) {
			targetCharName = characters[targetCharID]
		} else {
			client.SendServerMessage("Target has an invalid character.")
			return
		}
	}

	// Create the IC message packet args following the MS packet format
	// This is a ONE-TIME possession that copies the target's appearance completely
	icArgs := make([]string, 30)
	icArgs[0] = "chat"                        // desk_mod
	icArgs[1] = ""                            // pre-anim
	icArgs[2] = targetCharName                // character name (target's displayed character, including iniswap)
	icArgs[3] = targetEmote                   // emote (target's emote)
	icArgs[4] = encodedMsg                    // message (encoded)
	icArgs[5] = target.Pos()                  // position (target's position to spoof them)
	icArgs[6] = ""                            // sfx-name
	icArgs[7] = "0"                           // emote_mod
	icArgs[8] = strconv.Itoa(targetCharID)    // char_id (ID of target's displayed character)
	icArgs[9] = "0"                           // sfx-delay
	icArgs[10] = "0"                          // objection_mod
	icArgs[11] = "0"                          // evidence
	icArgs[12] = "0"                          // flipping
	icArgs[13] = "0"                          // realization
	// Use target's last text color, default to "0" (white) if none set
	targetTextColor := target.LastTextColor()
	if targetTextColor == "" {
		targetTextColor = "0"
	}
	icArgs[14] = targetTextColor              // text color (target's color)
	// Use target's showname, falling back to displayed character name
	showname := target.Showname()
	if strings.TrimSpace(showname) == "" {
		showname = targetCharName
	}
	icArgs[15] = showname                     // showname (target's showname)
	icArgs[16] = "-1"                         // pair_id
	icArgs[17] = ""                           // pair_charid (server pairing)
	icArgs[18] = ""                           // pair_emote (server pairing)
	icArgs[19] = ""                           // offset
	icArgs[20] = ""                           // pair_offset (server pairing)
	icArgs[21] = ""                           // pair_flip (server pairing)
	icArgs[22] = "0"                          // non-interrupting pre
	icArgs[23] = "0"                          // sfx-looping
	icArgs[24] = "0"                          // screenshake
	icArgs[25] = ""                           // frames_shake
	icArgs[26] = ""                           // frames_realization
	icArgs[27] = ""                           // frames_sfx
	icArgs[28] = "0"                          // additive
	icArgs[29] = ""                           // blank (reserved)

	// Send the IC message to the target's area
	writeToArea(target.Area(), "MS", icArgs...)

	// Log the possession (use original message for readability in logs)
	addToBuffer(client, "CMD", fmt.Sprintf("Possessed UID %v to say: \"%v\"", uid, msg), true)

	// Notify the admin
	client.SendServerMessage(fmt.Sprintf("Possessed UID %v for one message.", uid))
}

// Handles /unpossess
func cmdUnpossess(client *Client, args []string, _ string) {
	if client.Possessing() == -1 {
		client.SendServerMessage("You are not possessing anyone.")
		return
	}

	// Clear the possession link
	client.SetPossessing(-1)

	// Clear the saved possessed position
	client.SetPossessedPos("")

	// Log the action
	addToBuffer(client, "CMD", "Stopped possessing.", true)

	// Notify the admin
	client.SendServerMessage("Stopped possessing.")
}

// Handles /fullpossess - makes all admin's IC messages appear from target
func cmdFullPossess(client *Client, args []string, _ string) {
	// Get the target UID
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	// Get the target client
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}

	// Validate CharID is within bounds
	if target.CharID() < 0 || target.CharID() >= len(characters) {
		client.SendServerMessage("Target has an invalid character.")
		return
	}

	// Establish the persistent possession link
	client.SetPossessing(target.Uid())

	// Save the target's current position to spoof it
	client.SetPossessedPos(target.Pos())

	// Log the action
	addToBuffer(client, "CMD", fmt.Sprintf("Started full possession of UID %v.", uid), true)

	// Notify the admin
	client.SendServerMessage(fmt.Sprintf("Now fully possessing UID %v. All YOUR IC messages will appear as them. Use /unpossess to stop.", uid))
}

// Handles /rmusr
func cmdRemoveUser(client *Client, args []string, _ string) {
	if !db.UserExists(args[0]) {
		client.SendServerMessage("User does not exist.")
		return
	}
	err := db.RemoveUser(args[0])
	if err != nil {
		client.SendServerMessage("Failed to remove user.")
		logger.LogError(err.Error())
		return
	}
	client.SendServerMessage("Removed user.")

	for c := range clients.GetAllClients() {
		if c.Authenticated() && c.ModName() == args[0] {
			c.RemoveAuth()
		}
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Removed user %v.", args[0]), true)
}

// Handles /roll
func cmdRoll(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	private := flags.Bool("p", false, "")
	flags.Parse(args)
	b, _ := regexp.MatchString("([[:digit:]])d([[:digit:]])", flags.Arg(0))
	if !b {
		client.SendServerMessage("Argument not recognized.")
		return
	}
	s := strings.Split(flags.Arg(0), "d")
	num, _ := strconv.Atoi(s[0])
	sides, _ := strconv.Atoi(s[1])
	if num <= 0 || num > config.MaxDice || sides <= 0 || sides > config.MaxSide {
		client.SendServerMessage("Invalid num/side.")
		return
	}
	var result []string
	gen := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < num; i++ {
		result = append(result, fmt.Sprint(gen.Intn(sides)+1))
	}
	if *private {
		client.SendServerMessage(fmt.Sprintf("Results: %v.", strings.Join(result, ", ")))
	} else {
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v rolled %v. Results: %v.", client.OOCName(), flags.Arg(0), strings.Join(result, ", ")))
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Rolled %v.", flags.Arg(0)), false)
}

// Handles /setrole
func cmdChangeRole(client *Client, args []string, _ string) {
	role, err := getRole(args[1])
	if err != nil {
		client.SendServerMessage("Invalid role.")
		return
	}

	if !db.UserExists(args[0]) {
		client.SendServerMessage("User does not exist.")
		return
	}

	err = db.ChangePermissions(args[0], role.GetPermissions())
	if err != nil {
		client.SendServerMessage("Failed to change permissions.")
		logger.LogError(err.Error())
		return
	}
	client.SendServerMessage("Role updated.")

	for c := range clients.GetAllClients() {
		if c.Authenticated() && c.ModName() == args[0] {
			c.SetPerms(role.GetPermissions())
		}
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Updated role of %v to %v.", args[0], args[1]), true)
}

// Handles /status
func cmdStatus(client *Client, args []string, _ string) {
	switch strings.ToLower(args[0]) {
	case "idle":
		client.Area().SetStatus(area.StatusIdle)
	case "looking-for-players":
		client.Area().SetStatus(area.StatusPlayers)
	case "casing":
		client.Area().SetStatus(area.StatusCasing)
	case "recess":
		client.Area().SetStatus(area.StatusRecess)
	case "rp":
		client.Area().SetStatus(area.StatusRP)
	case "gaming":
		client.Area().SetStatus(area.StatusGaming)
	default:
		client.SendServerMessage("Status not recognized. Recognized statuses: idle, looking-for-players, casing, recess, rp, gaming")
		return
	}
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v set the status to %v.", client.OOCName(), args[0]))
	sendStatusArup()
	addToBuffer(client, "CMD", fmt.Sprintf("Set the status to %v.", args[0]), false)
}

// Handles swapevi
func cmdSwapEvi(client *Client, args []string, _ string) {
	if !client.CanAlterEvidence() {
		client.SendServerMessage("You are not allowed to alter evidence in this area.")
		return
	}
	evi1, err := strconv.Atoi(args[0])
	if err != nil {
		return
	}
	evi2, err := strconv.Atoi(args[1])
	if err != nil {
		return
	}
	if client.Area().SwapEvidence(evi1, evi2) {
		client.SendServerMessage("Evidence swapped.")
		writeToArea(client.Area(), "LE", client.Area().Evidence()...)
		addToBuffer(client, "CMD", fmt.Sprintf("Swapped posistions of evidence %v and %v.", evi1, evi2), false)
	} else {
		client.SendServerMessage("Invalid arguments.")
	}
}

// Handles /testimony
func cmdTestimony(client *Client, args []string, _ string) {
	if len(args) == 0 {
		if !client.Area().HasTestimony() {
			client.SendServerMessage("This area has no recorded testimony.")
			return
		}
		client.SendServerMessage(strings.Join(client.area.Testimony(), "\n"))
		return
	} else if !client.HasCMPermission() {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	switch args[0] {
	case "record":
		if client.Area().TstState() != area.TRIdle {
			client.SendServerMessage("The recorder is currently active.")
			return
		}
		client.Area().TstClear()
		client.Area().SetTstState(area.TRRecording)
		client.SendServerMessage("Recording testimony.")
	case "stop":
		client.Area().SetTstState(area.TRIdle)
		client.SendServerMessage("Recorder stopped.")
		client.Area().TstJump(0)
		writeToArea(client.Area(), "RT", "testimony1#1")
	case "play":
		if !client.Area().HasTestimony() {
			client.SendServerMessage("No testimony recorded.")
			return
		}
		client.Area().SetTstState(area.TRPlayback)
		client.SendServerMessage("Playing testimony.")
		writeToArea(client.Area(), "RT", "testimony2")
		writeToArea(client.Area(), "MS", client.Area().CurrentTstStatement())
	case "update":
		if client.Area().TstState() != area.TRPlayback {
			client.SendServerMessage("The recorder is not active.")
			return
		}
		client.Area().SetTstState(area.TRUpdating)
	case "insert":
		if client.Area().TstState() != area.TRPlayback {
			client.SendServerMessage("The recorder is not active.")
			return
		}
		client.Area().SetTstState(area.TRInserting)
	case "delete":
		if client.Area().TstState() != area.TRPlayback {
			client.SendServerMessage("The recorder is not active.")
			return
		}
		if client.Area().CurrentTstIndex() > 0 {
			err := client.Area().TstRemove()
			if err != nil {
				client.SendServerMessage("Failed to delete statement.")
			}
		}
	}
}

// Handles /unban
func cmdUnban(client *Client, args []string, _ string) {
	toUnban := strings.Split(args[0], ",")
	var report string
	for _, s := range toUnban {
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		err = db.UnBan(id)
		if err != nil {
			continue
		}
		report += fmt.Sprintf("%v, ", s)
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Nullified bans: %v", report))
	addToBuffer(client, "CMD", fmt.Sprintf("Nullified bans: %v", report), true)
}

// Handles /uncm
func cmdUnCM(client *Client, args []string, _ string) {
	if len(args) == 0 {
		if !client.Area().HasCM(client.Uid()) {
			client.SendServerMessage("You are not a CM in this area.")
			return
		}
		client.Area().RemoveCM(client.Uid())
		client.SendServerMessage("You are no longer a CM in this area.")
		addToBuffer(client, "CMD", "Un-CMed self.", false)
	} else {
		toCM := getUidList(strings.Split(args[0], ","))
		var count int
		var report string
		for _, c := range toCM {
			if c.Area() != client.Area() || !c.Area().HasCM(c.Uid()) {
				continue
			}
			c.Area().RemoveCM(c.Uid())
			c.SendServerMessage("You are no longer a CM in this area.")
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
		report = strings.TrimSuffix(report, ", ")
		client.SendServerMessage(fmt.Sprintf("Un-CMed %v users.", count))
		addToBuffer(client, "CMD", fmt.Sprintf("Un-CMed %v.", report), false)
	}
	sendCMArup()
}

// Handles /uninvite
func cmdUninvite(client *Client, args []string, _ string) {
	if client.Area().Lock() == area.LockFree {
		client.SendServerMessage("This area is unlocked.")
		return
	}
	toUninvite := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUninvite {
		if c == client || client.Area().HasCM(c.Uid()) {
			continue
		}
		if client.Area().RemoveInvited(c.Uid()) {
			if c.Area() == client.Area() && client.Area().Lock() == area.LockLocked && !permissions.HasPermission(c.Perms(), permissions.PermissionField["BYPASS_LOCK"]) {
				c.SendServerMessage("You were kicked from the area!")
				c.ChangeArea(areas[0])
			}
			c.SendServerMessage(fmt.Sprintf("You were uninvited from area %v.", client.Area().Name()))
			count++
			report += fmt.Sprintf("%v, ", c.Uid())
		}
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Uninvited %v users.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Uninvited %v to the area.", report), false)
}

// Handles /unlock
func cmdUnlock(client *Client, _ []string, _ string) {
	if client.Area().Lock() == area.LockFree {
		client.SendServerMessage("This area is not locked.")
		return
	}
	client.Area().SetLock(area.LockFree)
	client.Area().ClearInvited()
	sendLockArup()
	sendAreaServerMessage(client.Area(), fmt.Sprintf("%v unlocked the area.", client.OOCName()))
	addToBuffer(client, "CMD", "Unlocked the area.", false)
}

// Handles /unmute
func cmdUnmute(client *Client, args []string, _ string) {
	toUnmute := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnmute {
		if c.Muted() == Unmuted {
			continue
		}
		c.SetMuted(Unmuted)
		c.SendServerMessage("You have been unmuted.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Unmuted %v clients.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Unmuted %v.", report), false)
}

// Handles /jail
func cmdJail(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.String("d", "perma", "")
	reason := flags.String("r", "", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	uid, err := strconv.Atoi(flags.Arg(0))
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client not found.")
		return
	}

	var jailUntil time.Time
	if strings.ToLower(*duration) == "perma" {
		jailUntil = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		parsedDur, err := str2duration.ParseDuration(*duration)
		if err != nil {
			client.SendServerMessage("Failed to jail: Cannot parse duration.")
			return
		}
		jailUntil = time.Now().UTC().Add(parsedDur)
	}

	target.SetJailedUntil(jailUntil)
	
	msg := fmt.Sprintf("You have been jailed in %v.", target.Area().Name())
	if strings.ToLower(*duration) != "perma" {
		msg = fmt.Sprintf("You have been jailed in %v for %v.", target.Area().Name(), *duration)
	}
	if *reason != "" {
		msg += " Reason: " + *reason
	}
	target.SendServerMessage(msg)
	
	client.SendServerMessage(fmt.Sprintf("Jailed [%v] %v in %v.", uid, target.OOCName(), target.Area().Name()))
	
	logMsg := fmt.Sprintf("Jailed [%v] %v", uid, target.OOCName())
	if *reason != "" {
		logMsg += " for reason: " + *reason
	}
	addToBuffer(client, "CMD", logMsg, false)
}

// Handles /unjail
func cmdUnjail(client *Client, args []string, _ string) {
	toUnjail := getUidList(strings.Split(args[0], ","))
	var count int
	var report string
	for _, c := range toUnjail {
		if c.JailedUntil().IsZero() || time.Now().UTC().After(c.JailedUntil()) {
			continue
		}
		c.SetJailedUntil(time.Time{})
		c.SendServerMessage("You have been released from jail.")
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}
	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Released %v clients from jail.", count))
	addToBuffer(client, "CMD", fmt.Sprintf("Released %v from jail.", report), false)
}

// Handles /rps
func cmdRps(client *Client, args []string, _ string) {
	// Check cooldown (30 seconds)
	if time.Now().UTC().Before(client.LastRpsTime().Add(30 * time.Second)) && !client.LastRpsTime().IsZero() {
		remaining := time.Until(client.LastRpsTime().Add(30 * time.Second))
		client.SendServerMessage(fmt.Sprintf("Please wait %v seconds before playing RPS again.", int(remaining.Seconds())+1))
		return
	}

	choice := strings.ToLower(args[0])
	if choice != "rock" && choice != "paper" && choice != "scissors" {
		client.SendServerMessage("Invalid choice. Use: rock, paper, or scissors.")
		return
	}

	// Update last RPS time
	client.SetLastRpsTime(time.Now().UTC())

	// Generate random server choice
	choices := []string{"rock", "paper", "scissors"}
	gen := rand.New(rand.NewSource(time.Now().UnixNano()))
	serverChoice := choices[gen.Intn(3)]

	// Determine winner
	var result string
	if choice == serverChoice {
		result = "It's a tie!"
	} else if (choice == "rock" && serverChoice == "scissors") ||
		(choice == "paper" && serverChoice == "rock") ||
		(choice == "scissors" && serverChoice == "paper") {
		result = fmt.Sprintf("%v wins!", client.OOCName())
	} else {
		result = "Server wins!"
	}

	// Broadcast to area
	message := fmt.Sprintf("%v played %v, Server played %v. %v", client.OOCName(), choice, serverChoice, result)
	sendAreaServerMessage(client.Area(), message)
	addToBuffer(client, "GAME", fmt.Sprintf("Played RPS: %v vs %v - %v", choice, serverChoice, result), false)
}

// Handles /coinflip
func cmdCoinflip(client *Client, args []string, _ string) {
	choice := strings.ToLower(args[0])
	if choice != "heads" && choice != "tails" {
		client.SendServerMessage("Invalid choice. Use: heads or tails.")
		return
	}

	// Check if there's an active coinflip challenge in the area
	activeChallenge := client.Area().ActiveCoinflip()
	
	if activeChallenge == nil {
		// No active challenge - create a new one
		challenge := &area.CoinflipChallenge{
			PlayerName: client.OOCName(),
			Choice:     choice,
			CreatedAt:  time.Now().UTC(),
		}
		client.Area().SetActiveCoinflip(challenge)
		client.Area().SetLastCoinflipTime(time.Now().UTC())
		
		// Announce the challenge
		message := fmt.Sprintf("%v has chosen %v and is ready to coinflip! Type /coinflip %v to battle them!", 
			client.OOCName(), choice, oppositeChoice(choice))
		sendAreaServerMessage(client.Area(), message)
		addToBuffer(client, "GAME", fmt.Sprintf("Started coinflip challenge with %v", choice), false)
		
	} else {
		// There's an active challenge
		
		// Check if challenge has expired (30 seconds)
		if time.Now().UTC().After(activeChallenge.CreatedAt.Add(30 * time.Second)) {
			// Challenge expired, create new one
			challenge := &area.CoinflipChallenge{
				PlayerName: client.OOCName(),
				Choice:     choice,
				CreatedAt:  time.Now().UTC(),
			}
			client.Area().SetActiveCoinflip(challenge)
			client.Area().SetLastCoinflipTime(time.Now().UTC())
			
			message := fmt.Sprintf("Previous coinflip expired. %v has chosen %v and is ready to coinflip! Type /coinflip %v to battle them!", 
				client.OOCName(), choice, oppositeChoice(choice))
			sendAreaServerMessage(client.Area(), message)
			addToBuffer(client, "GAME", fmt.Sprintf("Started coinflip challenge with %v", choice), false)
			return
		}
		
		// Check if same player is trying to accept their own challenge
		if activeChallenge.PlayerName == client.OOCName() {
			client.SendServerMessage("You cannot accept your own coinflip challenge!")
			return
		}
		
		// Check if the choice is different from the challenger's choice
		if activeChallenge.Choice == choice {
			client.SendServerMessage(fmt.Sprintf("You must pick the opposite choice! The challenger picked %v, so you must pick %v.", 
				activeChallenge.Choice, oppositeChoice(activeChallenge.Choice)))
			return
		}
		
		// Battle time! Flip the coin
		gen := rand.New(rand.NewSource(time.Now().UnixNano()))
		coinResult := "heads"
		if gen.Intn(2) == 1 {
			coinResult = "tails"
		}
		
		// Determine winner
		var winner string
		if coinResult == activeChallenge.Choice {
			winner = activeChallenge.PlayerName
		} else {
			winner = client.OOCName()
		}
		
		// Announce result
		message := fmt.Sprintf("⚔️ COINFLIP BATTLE! %v (%v) vs %v (%v) - The coin landed on %v! 🎉 %v WINS! 🎉", 
			activeChallenge.PlayerName, activeChallenge.Choice,
			client.OOCName(), choice,
			coinResult, winner)
		sendAreaServerMessage(client.Area(), message)
		
		// Log for both players
		addToBuffer(client, "GAME", fmt.Sprintf("Coinflip battle: %v vs %v - Result: %v - Winner: %v", 
			activeChallenge.Choice, choice, coinResult, winner), false)
		
		// Clear the challenge
		client.Area().SetActiveCoinflip(nil)
	}
}

// oppositeChoice returns the opposite coinflip choice
func oppositeChoice(choice string) string {
	if choice == "heads" {
		return "tails"
	}
	return "heads"
}

// Handles /poll
func cmdPoll(client *Client, args []string, usage string) {
	// Check if there's already an active poll
	if client.Area().ActivePoll() != nil {
		client.SendServerMessage("There is already an active poll in this area.")
		return
	}

	// Check cooldown (5 minutes)
	if time.Now().UTC().Before(client.Area().LastPollTime().Add(5 * time.Minute)) && !client.Area().LastPollTime().IsZero() {
		remaining := time.Until(client.Area().LastPollTime().Add(5 * time.Minute))
		client.SendServerMessage(fmt.Sprintf("Please wait %v before creating another poll in this area.", remaining.Round(time.Second)))
		return
	}

	// Parse poll format: question|option1|option2|...
	fullArg := strings.Join(args, " ")
	parts := strings.Split(fullArg, "|")
	
	if len(parts) < 3 {
		client.SendServerMessage("Not enough poll options. Format: " + usage)
		return
	}

	question := strings.TrimSpace(parts[0])
	options := make([]string, 0)
	for i := 1; i < len(parts); i++ {
		opt := strings.TrimSpace(parts[i])
		if opt != "" {
			options = append(options, opt)
		}
	}

	if len(options) < 2 {
		client.SendServerMessage("Poll must have at least 2 options.")
		return
	}

	// Create poll
	poll := &area.Poll{
		ID:        time.Now().UnixNano(),
		Question:  question,
		Options:   options,
		CreatedAt: time.Now().UTC(),
		ClosesAt:  time.Now().UTC().Add(2 * time.Minute),
		CreatedBy: client.OOCName(),
	}

	client.Area().SetActivePoll(poll)
	client.Area().SetLastPollTime(time.Now().UTC())
	client.Area().SetPollVotes(make(map[int]int))
	client.Area().SetPlayerVotes(make(map[int]int))

	// Broadcast poll to area
	pollMsg := fmt.Sprintf("=== POLL ===\n%v\n", question)
	for i, opt := range options {
		pollMsg += fmt.Sprintf("%v. %v\n", i+1, opt)
	}
	pollMsg += fmt.Sprintf("\nUse /vote <number> to vote. Poll closes in 2 minutes.")
	sendAreaServerMessage(client.Area(), pollMsg)
	addToBuffer(client, "CMD", fmt.Sprintf("Created poll: %v", question), false)

	// Schedule auto-close after 2 minutes
	go func(a *area.Area, pollID int64) {
		time.Sleep(2 * time.Minute)
		currentPoll := a.ActivePoll()
		if currentPoll != nil && currentPoll.ID == pollID {
			// Close poll
			resultMsg := fmt.Sprintf("=== POLL CLOSED ===\n%v\nResults:\n", currentPoll.Question)
			votes := a.PollVotes()
			for i, opt := range currentPoll.Options {
				count := 0
				if votes != nil {
					count = votes[i+1]
				}
				resultMsg += fmt.Sprintf("%v. %v - %v votes\n", i+1, opt, count)
			}
			sendAreaServerMessage(a, resultMsg)
			a.ClearPoll()
		}
	}(client.Area(), poll.ID)
}

// Handles /vote
func cmdVote(client *Client, args []string, usage string) {
	// Check if there's an active poll
	poll := client.Area().ActivePoll()
	if poll == nil {
		client.SendServerMessage("There is no active poll in this area.")
		return
	}

	// Check if poll has expired
	if time.Now().UTC().After(poll.ClosesAt) {
		client.SendServerMessage("This poll has expired.")
		client.Area().ClearPoll()
		return
	}

	// Check if player has already voted
	if client.Area().HasPlayerVoted(client.Uid()) {
		client.SendServerMessage("You have already voted in this poll.")
		return
	}

	// Parse vote option
	option, err := strconv.Atoi(args[0])
	if err != nil || option < 1 || option > len(poll.Options) {
		client.SendServerMessage(fmt.Sprintf("Invalid option. Choose a number between 1 and %v.", len(poll.Options)))
		return
	}

	// Record vote
	client.Area().AddPlayerVote(client.Uid(), option)
	client.SendServerMessage(fmt.Sprintf("You voted for: %v", poll.Options[option-1]))

	// Broadcast updated results to area
	resultMsg := fmt.Sprintf("=== POLL UPDATE ===\n%v\nCurrent Results:\n", poll.Question)
	votes := client.Area().PollVotes()
	for i, opt := range poll.Options {
		count := 0
		if votes != nil {
			count = votes[i+1]
		}
		resultMsg += fmt.Sprintf("%v. %v - %v votes\n", i+1, opt, count)
	}
	sendAreaServerMessage(client.Area(), resultMsg)
	addToBuffer(client, "VOTE", fmt.Sprintf("Voted for option %v in poll", option), false)
}

// cmdPunishment is a generic handler for punishment commands
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
		c.AddPunishment(pType, duration, *reason)
		c.SendServerMessage(msg)
		count++
		report += fmt.Sprintf("%v, ", c.Uid())
	}

	report = strings.TrimSuffix(report, ", ")
	client.SendServerMessage(fmt.Sprintf("Applied '%v' punishment to %v clients.", pType.String(), count))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied '%v' punishment to %v.", pType.String(), report), false)
}

// Handlers for all punishment commands
func cmdWhisper(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentWhisper)
}

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

func cmdPause(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPause)
}

func cmdLag(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLag)
}

func cmdSubtitles(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSubtitles)
}

func cmdRoulette(client *Client, args []string, usage string) {
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

// cmdUnpunish removes all or specific punishments from users
func cmdUnpunish(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	punishmentType := flags.String("t", "", "")
	flags.Parse(args)

	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	toUnpunish := getUidList(strings.Split(flags.Arg(0), ","))
	var count int
	var report string

	for _, c := range toUnpunish {
		if *punishmentType == "" {
			// Remove all punishments
			punishments := c.GetActivePunishments()
			if len(punishments) == 0 {
				continue
			}
			c.RemoveAllPunishments()
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

	for _, c := range toPunish {
		// Apply each punishment
		for _, pType := range punishmentTypes {
			c.AddPunishment(pType, duration, *reason)
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

// cmdTournament manages punishment tournament mode
func cmdTournament(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	action := strings.ToLower(args[0])

	switch action {
	case "start":
		tournamentMutex.Lock()
		defer tournamentMutex.Unlock()

		if tournamentActive {
			client.SendServerMessage("A tournament is already active.")
			return
		}

		tournamentActive = true
		tournamentStartTime = time.Now().UTC()
		tournamentParticipants = make(map[int]*TournamentParticipant)

		client.SendServerMessage("Tournament started! Users can now join with /join-tournament")
		writeToAllClients("CT", "OOC", "🏆 TOURNAMENT STARTED! Join with /join-tournament to compete! Random punishments will be applied.")
		addToBuffer(client, "CMD", "Started punishment tournament", false)

	case "stop":
		tournamentMutex.Lock()
		defer tournamentMutex.Unlock()

		if !tournamentActive {
			client.SendServerMessage("No tournament is currently active.")
			return
		}

		// Determine winner
		var winner *TournamentParticipant
		var winnerClient *Client
		for uid, participant := range tournamentParticipants {
			if winner == nil || participant.messageCount > winner.messageCount {
				winner = participant
				winnerClient = clients.GetClientByUID(uid)
			}
		}

		tournamentActive = false

		if winner != nil && winnerClient != nil {
			duration := time.Since(tournamentStartTime).Round(time.Second)
			announcement := fmt.Sprintf("🏆 TOURNAMENT ENDED! Winner: UID %d with %d messages over %v! Congratulations!",
				winner.uid, winner.messageCount, duration)
			writeToAllClients("CT", "OOC", announcement)
			
			// Remove all punishments from winner
			winnerClient.RemoveAllPunishments()
			winnerClient.SendServerMessage("Congratulations! Your tournament punishments have been removed.")
		} else {
			writeToAllClients("CT", "OOC", "🏆 TOURNAMENT ENDED! No participants.")
		}

		tournamentParticipants = make(map[int]*TournamentParticipant)
		addToBuffer(client, "CMD", "Stopped punishment tournament", false)

	case "status":
		tournamentMutex.Lock()
		defer tournamentMutex.Unlock()

		if !tournamentActive {
			client.SendServerMessage("No tournament is currently active.")
			return
		}

		duration := time.Since(tournamentStartTime).Round(time.Second)
		msg := fmt.Sprintf("🏆 TOURNAMENT STATUS (Running for %v)\n", duration)
		msg += fmt.Sprintf("Participants: %d\n\n", len(tournamentParticipants))

		// Build leaderboard sorted by message count
		type leaderEntry struct {
			uid      int
			msgCount int
			duration time.Duration
		}
		var leaderboard []leaderEntry
		for uid, participant := range tournamentParticipants {
			leaderboard = append(leaderboard, leaderEntry{
				uid:      uid,
				msgCount: participant.messageCount,
				duration: time.Since(participant.joinedAt).Round(time.Second),
			})
		}

		// Sort by message count (descending)
		sort.Slice(leaderboard, func(i, j int) bool {
			return leaderboard[i].msgCount > leaderboard[j].msgCount
		})

		msg += "LEADERBOARD:\n"
		for i, entry := range leaderboard {
			rank := i + 1
			msg += fmt.Sprintf("%d. UID %d - %d messages (%v in tournament)\n",
				rank, entry.uid, entry.msgCount, entry.duration)
		}

		client.SendServerMessage(msg)

	default:
		client.SendServerMessage("Invalid action. Use: start, stop, or status")
	}
}

// cmdJoinTournament allows users to join the active tournament
func cmdJoinTournament(client *Client, args []string, usage string) {
	tournamentMutex.Lock()
	defer tournamentMutex.Unlock()

	if !tournamentActive {
		client.SendServerMessage("No tournament is currently active.")
		return
	}

	uid := client.Uid()
	if _, exists := tournamentParticipants[uid]; exists {
		client.SendServerMessage("You are already in the tournament!")
		return
	}

	// Add participant
	tournamentParticipants[uid] = &TournamentParticipant{
		uid:          uid,
		messageCount: 0,
		joinedAt:     time.Now().UTC(),
	}

	// Apply 2-3 random punishments
	allPunishments := []PunishmentType{
		PunishmentBackward, PunishmentStutterstep, PunishmentElongate,
		PunishmentUppercase, PunishmentLowercase, PunishmentRobotic,
		PunishmentAlternating, PunishmentUwu, PunishmentPirate,
		PunishmentConfused, PunishmentDrunk, PunishmentHiccup,
	}

	numPunishments := 2 + rand.Intn(2) // 2 or 3 punishments
	selectedPunishments := []PunishmentType{}
	
	// Randomly select unique punishments
	shuffled := make([]PunishmentType, len(allPunishments))
	copy(shuffled, allPunishments)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	for i := 0; i < numPunishments && i < len(shuffled); i++ {
		pType := shuffled[i]
		selectedPunishments = append(selectedPunishments, pType)
		client.AddPunishment(pType, 0, "Tournament Mode") // No expiration
	}

	// Build punishment list for message
	punishmentNames := []string{}
	for _, pType := range selectedPunishments {
		punishmentNames = append(punishmentNames, pType.String())
	}

	client.SendServerMessage(fmt.Sprintf("🏆 Joined tournament! You've been given: %s", strings.Join(punishmentNames, ", ")))
	writeToAllClients("CT", "OOC", fmt.Sprintf("🏆 UID %d joined the tournament!", uid))
	addToBuffer(client, "TOURNAMENT", "Joined tournament", false)
}
