# Nyathena — Moderator Commands Guide

Every command in here requires a moderator-class permission. Required permission bits are noted in the right-hand column of each table. Detailed punishment-by-punishment text effects are documented separately in [`PUNISHMENT_COMMANDS.md`](PUNISHMENT_COMMANDS.md).

For player-facing commands, see [`PLAYER_COMMANDS.md`](PLAYER_COMMANDS.md).

---

## Permission Tiers (overview)

| Tier | Bits typically granted | Notes |
|------|-----------------------|-------|
| `MUTE` | The minimum tier for punishments | All punishment text effects, gag, tournament |
| `KICK` | + ability to remove players from the server | Includes /charcurse |
| `BAN` | + connection-level moderation | /ban, /unban, /firewall |
| `MOVE_USERS` | Move/summon players between areas | /summon |
| `MODIFY_AREA` | Override area settings | BG/music locks, force CMs |
| `BAN_INFO` | View ban records | /getban, /listbans |
| `ADMIN` | Server runtime configuration | /arealog, /reloadplaytime, /createtag |
| `SHADOW` | Stealth moderator | Hidden from /gas/players for non-admins |

Permission bits are configured in `config/roles.toml`. Multiple bits are granted as a bitfield — see the role definitions for combinations.

---

## Account Management (Mod CLI / `/login`)

| Command | Permission | Description |
|---------|-----------|-------------|
| `/login <username> <password>` | NONE | Sign in to your moderator account |
| `/logout` | NONE | Sign out |
| `mkusr <username> <password>` (CLI) | server stdin | Create the first moderator account |

---

## Connection-Level Moderation

| Command | Permission | Description |
|---------|-----------|-------------|
| `/ban -u <uid> [-d duration] <reason>` | BAN | Ban by UID |
| `/ban -i <ipid> [-d duration] <reason>` | BAN | Ban by IPID (works on offline targets) |
| `/unban <ban-id>` | BAN | Lift a ban |
| `/getban [-b banid \| -i ipid]` | BAN_INFO | Look up bans |
| `/editban [-d duration] [-r reason] <ids>` | BAN | Edit ban metadata |
| `/kick <uid>` | KICK | Disconnect a player |
| `/kickother` | NONE | Kick stale ghost connections sharing your HDID |
| `/firewall on\|off` | BAN | Toggle the IPHub VPN/proxy firewall (requires `iphub_api_key` in config). Also exposed as a Discord slash command. |
| `/lockdown [add <uid>\|whitelist all]` | BAN | Toggle server lockdown, or whitelist players |

---

## Voice Moderation

| Command | Permission | Description |
|---------|-----------|-------------|
| `/mute <uid> [-d duration]` | MUTE | Prevent IC speech |
| `/unmute <uid>` | MUTE | Restore IC speech |
| `/gag <uid>` | MUTE | Replace IC text with gibberish |
| `/ungag <uid>` | MUTE | Remove gag |
| `/parrot <uid> [-d duration]` | MUTE | IC text replaced with random parrot phrase |
| `/oocmute <uid>` | MUTE | Mute from OOC chat |
| `/oocunmute <uid>` | MUTE | Restore OOC chat |

---

## Area Control

| Command | Permission | Description |
|---------|-----------|-------------|
| `/lock` | NONE (CM) | Lock the area; current occupants get auto-invited |
| `/unlock` | NONE (CM) | Unlock the area |
| `/lock -s` | NONE (CM) | Set area to spectatable (joiners enter as spectators) |
| `/invite <uid>` | NONE (CM) | Add a UID to the invite list |
| `/uninvite <uid>` | NONE (CM) | Remove from invite list |
| `/kick <uid>` (in-area) | NONE (CM) | Eject a player from the area. Now also pulls them from the invite list, so they can't walk back into a locked room. |
| `/cleararea` | MOVE_USERS | Move all players out of an area to the lobby |
| `/forcemove <uid> <area>` | MOVE_USERS | Force-move a player |
| `/summon <area>` | MOVE_USERS | Summon all players to an area |
| `/jail <uid>` | MUTE | Restrict a player to the jail area |
| `/unjail <uid>` | MUTE | Lift jail |
| `/bg <bg>` | DJ / CM / MODIFY_AREA | Set background (DJs rate-limited to once per minute) |
| `/lockbg true\|false` | MODIFY_AREA | Lock/unlock background changes |
| `/lockmusic true\|false` | MODIFY_AREA | Lock/unlock music changes |
| `/forcebglist true\|false` | MODIFY_AREA | Force the server BG list on this area |
| `/allowiniswap true\|false` | MODIFY_AREA | Permit iniswapping |
| `/allowcms true\|false` | MODIFY_AREA | Permit area CMs |
| `/evimode <mode>` | NONE (CM) | Set evidence mode (any/cms/mods) |
| `/status <status>` | NONE (CM) | Set area status |
| `/spectate [invite\|uninvite <uids>]` | NONE (CM) | Manage spectate-mode invitations |
| `/areadesc [-c] [text]` | NONE | Set/clear area entry description |

---

## Per-Player Modifiers

| Command | Permission | Description |
|---------|-----------|-------------|
| `/charstuck [-d duration] <uid>` | MUTE | Lock to current character |
| `/charcurse <uid> <charname>` | KICK | Force a one-time character swap (target may change afterward) |
| `/forcepair <uid1> <uid2>` | MUTE | Force two players into a UID-tracked pair |
| `/forceunpair <uid>` | MUTE | Break a forced pair |
| `/setrole <uid> <role>` | ADMIN | Set a player's role (permission tier) |
| `/clearpos <uid>` | MOVE_USERS | Clear forced position |
| `/showname <uid> <name>` | MUTE | Force a showname |
| `/clearshowname <uid>` | MUTE | Clear forced showname |
| `/nameshuffle` | MUTE | Randomly permute every player's showname in this area |
| `/unnameshuffle` | MUTE | Restore shuffled shownames |
| `/charshuffle` | MUTE | Randomly permute every player's character in this area (Sattolo's algorithm — guaranteed derangement) |
| `/uncharshuffle` | MUTE | Restore characters to pre-shuffle state |

---

## Punishments — Quick Index

> All punishments share these flags: `-d <duration>` (max 24h), `-r <reason>`, comma-separated UIDs. Multiple punishments stack. Use `/help punishment` in-game for the **subcategorized** browser. Per-effect docs live in `PUNISHMENT_COMMANDS.md`.

| Group | Commands |
|-------|----------|
| Text effects | `/whisper /backward /stutterstep /elongate /uppercase /lowercase /robotic /alternating /fancy /uwu /pirate /shakespearean /caveman /slang /cherri` |
| Visibility | `/emoji /invisible` |
| Sprite offset | `/shrink /grow /wide` (and `/unshrink /ungrow /unwide`) |
| Timing | `/slowpoke /fastspammer /pause /lag` |
| Social chaos | `/subtitles /tourettes /roulette /spotlight` |
| Text processing | `/censor /confused /paranoid /drunk /hiccup /whistle /mumble` |
| Complex effects | `/spaghetti /torment /rng /essay` |
| Advanced | `/haiku /autospell` |
| Personalities | `/thesaurusoverload /valleygirl /babytalk /thirdperson /unreliablenarrator /uncannyvalley /clown /jester /joker /mime` |
| Themed quote replacers | `/gordonramsay /biblebot /recipe /rickroll /pickup /brainrot` |
| Audio | `/sfxcurse <uid> <sfx-url>` and `/unsfx` |
| Dere archetypes (25) | `/tsundere /yandere /kuudere /dandere /deredere /himedere /kamidere /undere /bakadere /mayadere /smugdere /deretsun /bokodere /thugdere /teasedere /dorodere /hinedere /hajidere /rindere /utsudere /darudere /butsudere /sdere /mdere /tsuyodere` |
| Combiners | `/omnidere` (random dere per line), `/stack <type1> <type2> ...` |
| Removal | `/unpunish <uid>`, `/unpunish -t <type> <uid>` |

### `/sfxcurse` example

```
/sfxcurse 12 https://miku.pizza/base/sounds/general/meow.opus
```
The target's IC packet's SFX field is overwritten with that URL on every line until `/unsfx 12`.

### `/unpunish` Self-Removal Protection

The DB now records the issuing tier of every punishment in `PUNISHMENTS.ISSUER_TIER`. A regular moderator **cannot** lift a punishment that an admin or shadow mod placed on them — `/unpunish self`, `/unpunish -t <type> self`, and the self-target slice of `/unpunish all` are all gated. Admins and shadow mods bypass the gate.

---

## Punishment Tournaments

| Command | Permission | Description |
|---------|-----------|-------------|
| `/tournament start\|status\|stop` | MUTE | Run a punishment tournament. Volunteers join via `/join-tournament` and accumulate 2–3 random punishments — most IC messages sent wins. |

---

## Custom Tags (cosmetic, admin-managed)

| Command | Permission | Description |
|---------|-----------|-------------|
| `/createtag <id> <display name>` | ADMIN | Mint a new custom tag at runtime |
| `/deletetag <id>` | ADMIN | Delete a custom tag and clean up grants/equips |
| `/grantcustomtag <username> <id>` | ADMIN | Grant a tag to an account (account must have logged in once) |
| `/revokecustomtag <username> <id>` | ADMIN | Revoke a granted tag |
| `/listcustomtags` | NONE | List every custom tag |

---

## Server Admin

| Command | Permission | Description |
|---------|-----------|-------------|
| `/arealog enable\|disable` | ADMIN | Toggle area-log silencing for the current area |
| `/reloadplaytime` | ADMIN | Re-link every registered account to its IPID and merge orphaned playtime. Fixes the bug where a fresh account on a long-running anonymous IPID didn't appear on the leaderboard. |
| `/restart` | ADMIN | In-place server restart via `syscall.Exec` |
| `/casinoenable` | ADMIN | Toggle casino in this area |
| `/casinoset <key> <value>` | ADMIN | Configure casino limits / jackpot |
| `/grantchips <username> <amount>` | ADMIN | Add chips to an account |

---

## Discord Bot Slash Commands

> Requires the bot's role to have the configured `mod_role_id` (or the player's permission tier to be granted via account linking).

| Slash | Description |
|-------|-------------|
| `/players` | List connected players |
| `/info <player>` | Player info card |
| `/find <player>` | Locate a player's area |
| `/status` | Server status |
| `/mute /unmute /ban /unban /kick /gag /ungag /warn /warnings` | Moderation actions |
| `/parrot /drunk /slowpoke /roulette /spotlight /whisper /stutterstep /backward` | Apply punishments |
| `/pm /announce /announce_player` | Communication |
| `/forcemove /cleararea /lock /unlock` | Area control |
| `/logs /auditlog /banlist` | Audit & logs |
| `/firewall on\|off` | Toggle IPHub VPN screening |
| `/lockdown on\|off\|whitelist_all` | Toggle server lockdown / whitelist all currently-connected players |
| `/restart` | Restart the server (Admin only) |

---

## Stealth Mode

Shadow moderators (`SHADOW` perm bit, no `ADMIN`) are hidden from `/gas` and `/players` for non-admin viewers — no `Mod:` line is shown at all. Only admins see anything for a shadow mod, labelled `Mod: <name> (shadow)`.

Shadow mods are still visible on `/playtime top` (the leaderboard does not filter by permission), and they are NOT exempt from `/unpunish` self-removal protection — which means a regular mod cannot lift a shadow-mod-issued punishment on themselves.
