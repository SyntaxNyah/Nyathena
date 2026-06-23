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
| `/mkusr <username> <password> <role>` | ADMIN | Create a moderator user |
| `/setrole <username> <role>` | ADMIN | Change a user's role (permission tier) |
| `/removerole <username>` | ADMIN | Strip a user's role, resetting them to a default player account. **Keeps the account** (login, chips, playtime, tags) — does not delete it. Use this to demote a DJ/mod without purging their account. |
| `/resetpass <username> <new_password>` | ADMIN | Reset a user's password |
| `/rmusr <username>` | ADMIN | **Delete** a moderator user account entirely |

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
| `/adminlock` | ADMIN | Toggle an **admin-only seal**: nobody but admins can enter — not even mods or shadow mods with `BYPASS_LOCK`, and not even invited players. Players already inside are not evicted. A non-admin cannot `/unlock` or `/lock` an admin-locked area; only `/adminlock` (by an admin) lifts it. |
| `/invite <uid>` | NONE (CM) | Invite a UID. In a **locked** area this grants entry; in **spectate mode** it also grants the right to speak in IC (same as `/spectate invite`). Requires the area to be locked or in spectate mode — in a plain unlocked area it explains how to restrict the area first instead of doing nothing. |
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
| `/spectate [invite\|uninvite <uids>]` | NONE (CM) | Toggle spectate mode, or grant/revoke IC speaking rights while it's on. Listed in `/help` for **all** players (not just CMs) so everyone can discover how spectate mode works, though only CMs can run it. |
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
| `/clients <uid>` | MUTE | List every connection sharing the target's IPID — multiclient overview with UID, character, area, OOC name and showname |
| `/punishments <uid>` | MUTE | Inspect any player's active punishments with remaining durations, issuer tiers, lag/mute/jail status |

---

## Possession

Speak through another player's character. All three flavours fully copy the target's appearance — character, emote, position, text colour, showname — **and** spoof the target's pairing, so if the target is paired their partner's sprite still renders next to them in the viewport (no "the pair vanished" tell).

| Command | Permission | Description |
|---------|-----------|-------------|
| `/possess <uid> <message>` | SHADOW | Make the target say one message, rendered exactly as them. |
| `/fullpossess <uid>` | SHADOW | Become the target persistently — see below. Identical to `/truepossess`. |
| `/truepossess <uid>` | SHADOW | Become the target persistently — see below. Identical to `/fullpossess`. |
| `/unpossess` | SHADOW | Stop a full/true possession (lifts the target's mute). |

`/fullpossess` and `/truepossess` are the **same command** (two names for the same behaviour): every one of *your* IC messages renders as the target until `/unpossess`, **and** the target is silently muted — their own IC and OOC are echoed only back to them (so their client still looks normal) but reach nobody, their commands (`/global`, `/pm`, `/modchat`, …) are swallowed, and their showname / OOC name are frozen. Combined with the pair-spoof, an onlooker sees the target talking normally (with their partner) while you drive every line, and the target has no in-game channel to shout "it's not me". Suppressed IC/OOC and swallowed commands are still written to the area log (tagged `(truepossessed)` / `(suppressed during /truepossess)`) for staff audit. A possession ends automatically — and the mute lifts — if either party disconnects.

---

## Punishments — Quick Index

> All punishments share these flags: `-d <duration>` (max 24h), `-r <reason>`, `-h` (hidden — apply silently), comma-separated UIDs, and `global` (apply to every non-mod in your area). Multiple punishments stack. Use `/help punishment` in-game for the **subcategorized** browser. Per-effect docs live in `PUNISHMENT_COMMANDS.md`.

| Group | Commands |
|-------|----------|
| Text effects (60) | `/whisper /backward /stutterstep /elongate /uppercase /lowercase /robotic /alternating /fancy /uwu /pirate /shakespearean /caveman /censor /fromsoftware /confused /paranoid /drunk /hiccup /whistle /mumble /slang /cherri /albhed /morse /vowelhell /upsidedown /autospell /thesaurusoverload /valleygirl /babytalk /thirdperson /unreliablenarrator /uncannyvalley /chef /karen /passiveaggressive /nervous /sarcasm /academic /philosopher /poet /quote /spaghetti /essay /rng /haiku /dreamsequence /timewarp /zalgo /leetspeak /smallcaps /piglatin /vaporwave /lisp /spoonerism /keysmash /weeb /politician /clickbait /markov /alliteration /cipher` |
| Themed quote replacers | `/gordonramsay /biblebot /grounded /mime /subtitles /spotlight /recipe /rickroll /pickup /brainrot` |
| Persona / personality | `/clown /jester /joker /tourettes /translator` |
| Dere archetypes (26) | `/tsundere /yandere /kuudere /dandere /deredere /himedere /kamidere /undere /bakadere /mayadere /smugdere /deretsun /bokodere /thugdere /teasedere /dorodere /hinedere /hajidere /rindere /utsudere /darudere /butsudere /sdere /mdere /tsuyodere /omnidere` |
| Animal filters (12) | `/monkey /snake /dog /cat /bird /cow /frog /duck /horse /lion /zoo /bunny` |
| Visibility / cosmetic | `/emoji /invisible /shrink /grow /wide /areainiswap /hidedisplay /forcedisplay` (and `/unshrink /ungrow /unwide`) |
| Protocol / viewport (6) | `/teleport /shakecurse /randomflip /forcecolor /nopreanim /forcepreanim` |
| Timing | `/slowpoke /fastspammer /lag /lifo` |
| Audio | `/sfxcurse <uid> <sfx-url>` and `/unsfx` |
| Voice chat (5) | `/voicemute /voicestatic /voicegarble /voicecutout /voicestutter` |
| Traps & contagion (4) | `/contagious <type> /minefield /silencebell /stealthmute` |
| Stacking / chaos | `/stack /torment /lovebomb /degrade /emoticon /51 /icwarp /megamaso /maso /randompunishall /togglerandompunish /tournament` |
| Inspection | `/punishments [uid]` — active punishments with remaining durations (players: self only) |
| Removal | `/unpunish <uid>`, `/unpunish -t <type> <uid>`, `/unpunish all`, `/unlag`, plus per-effect `un-` commands |
| Self-chaos block | `/blockpunishment /unblockpunishment` |

### Hidden flag (`-h`)

Appending `-h` to any punishment command suppresses the OOC notification to the target — the punishment applies silently. The issuer's summary appends `(hidden)` for confirmation. Works on all applicators: single-effect, `/stack`, `/lovebomb`, `/sfxcurse`, `/shrink`/`/grow`/`/wide`, `/randompunishall` (also hides the area-wide announcement), `/translator curse`, and `/icwarp`.

```
/tsundere 7 -h                    # Silent tsundere
/stack backward uwu global -h     # Silent stack on entire area
/randompunishall -h                # No area announcement
```

### `/sfxcurse` example

```
/sfxcurse 12 https://miku.pizza/base/sounds/general/meow.opus
/sfxcurse 12 https://cdn.discordapp.com/attachments/123456789/987654321/boom.opus
/sfxcurse global https://example.com/honk.opus
```
The target's IC packet's SFX field is overwritten with the URL on every line until `/unsfx 12`.

**URL handling:**
- URLs that contain `/base/sounds/` (standard AO2 asset-server paths) have their filename stem extracted and sent to clients, which resolve it locally or via the configured asset URL.
- All other `http(s)://` URLs (Discord CDN, custom hosting, etc.) are forwarded as-is so that clients supporting URL-based audio can stream the file directly.

### `/unpunish` — Full Coverage

`/unpunish` now covers **every** active punishment including `/lag` (the torment list). Forms:

- `/unpunish <uid>` — removes all punishments, mute, jail, and lag from a specific target.
- `/unpunish -t lag <uid>` — removes only lag.
- `/unpunish all` — wipes all punishments from every player in your current area.

### `/unpunish` Self-Removal Protection

The DB records the issuing tier of every punishment in `PUNISHMENTS.ISSUER_TIER`. A regular moderator **cannot** lift a punishment that an admin or shadow mod placed on them — `/unpunish self`, `/unpunish -t <type> self`, and the self-target slice of `/unpunish all` are all gated. Admins and shadow mods bypass the gate.

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

## Music Ban (persistent, per-IPID)

| Command | Permission | Description |
|---------|-----------|-------------|
| `/musicban <uid> [-r reason]` | MUTE | Persistently ban the target's IPID from playing music (both jukebox entries and streaming URLs) across sessions. Idempotent — re-banning overwrites the reason and issuer. |
| `/musicunban <uid\|ipid>` | MUTE | Lift a music-ban. Accepts a connected target's UID or a raw IPID, so offline players can still be unbanned. |
| `/musicbans` | MUTE | List every active music-ban with its reason, issuer, and timestamp (newest first). |

Music bans are stored in the `MUSIC_BANS` table (DB migration 22) and cached in-memory for a single-RWMutex-map-lookup hot path on the MC handler.

**Quiet-area carve-out:** if the area has **fewer than 3 people** in it, the ban is **bypassed** and the music change is allowed — banned players can still set the mood in empty/small rooms but can't bother a populated one. Moderators are always exempt. Area-change MC packets are unaffected.

---

## Server Admin

| Command | Permission | Description |
|---------|-----------|-------------|
| `/arealog enable\|disable` | ADMIN | Toggle area-log silencing for the current area |
| `/reloadplaytime` | ADMIN | Re-link every registered account to its IPID and merge orphaned playtime. Fixes the bug where a fresh account on a long-running anonymous IPID didn't appear on the leaderboard. |
| `/reload` | ADMIN | Hot-reload all supported config/data files at runtime without restarting. See "Hot config reload" below. |
| `/restart` | ADMIN | In-place server restart via `syscall.Exec` |
| `/casinoenable` | ADMIN | Toggle casino in this area |
| `/casinoset <key> <value>` | ADMIN | Configure casino limits / jackpot |
| `/grantchips <username> <amount>` | ADMIN | Add chips to an account |

### Hot config reload (`/reload`)

`/reload` (in-game, ADMIN) atomically re-reads every supported config/data file from disk and swaps it in without restarting. Also available as the `reload` CLI command on stdin and via `SIGHUP`.

Each reloadable list lives behind a `sync/atomic.Pointer` so a swap is a single atomic store — readers on the hot IC path never lock and never see a torn value. A parse error in any one file aborts the whole reload before anything is published, so a bad file never leaves the running server half-updated.

**Reloaded:**
- `characters.txt` — **append-only** (see safety constraint below)
- `music.txt` — full reload; the pre-built SM packet sent on every client join is rebuilt in lockstep
- `cdns.txt`, `backgrounds.txt` (with `/bglist` cache rebuilt), `parrot.txt`
- `8ball.txt` (optional; missing file leaves the current value intact)
- `banned_words.txt` (only when automod is enabled)
- `config.toml` motd and description

**`characters.txt` safety constraint — append-only.** Connected AO2 clients reference characters by **slot index**, so inserting in the middle, removing, reordering or renaming an existing slot would silently desync every connected player. The reload **validates that every existing slot is unchanged** and only accepts entries appended at the end of the file. If the new file changes any pre-existing slot, the reload is rejected with a precise message naming the first bad slot — change those operations require a restart.

**NOT reloaded** (would require invasive work and is unsafe without restart): areas, listener ports/addr, rate-limit windows, max_players, roles, the server name.

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

Shadow mods (and admins) can also **`/hide`** themselves — vanishing entirely from `/players`, `/gas`, and room player counts — so they can lurk on an area unseen. `/hide` again to reappear. Regular (non-shadow) moderators cannot `/hide`.
