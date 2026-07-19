# Nyathena

## Project Overview

Nyathena is a fork of [Athena](https://github.com/MangosArentLiterature/Athena), a lightweight AO2 (Attorney Online 2) server written in Go. It extends upstream Athena with a large set of original features:

- A full **Discord bot** integration (slash commands, embeds, moderation bridge)
- A **casino system** with 10 distinct games and persistent virtual currency ("Nyathena Chips")
- A **Mafia social-deduction minigame** playable inside any server area
- **120+ punishment commands** for moderators, with stacking, tournaments, contagion/trap mechanics, and a coinflip challenge system
- Persistent pairing, per-area logging, configurable rate limiting, AutoMod, IPHub VPN firewall, and more

Module path (retained from upstream): `github.com/MangosArentLiterature/Athena`

## Tech Stack

| Dependency | Version | Purpose |
|-----------|---------|---------|
| `github.com/bwmarrin/discordgo` | v0.28.1 | Discord bot (Gateway, slash commands, embeds) |
| `github.com/ecnepsnai/discord` | v1.2.1 | Discord webhook support |
| `github.com/BurntSushi/toml` | v1.2.0 | TOML config parsing |
| `modernc.org/sqlite` | v1.18.0 | SQLite database (no cgo required) |
| `golang.org/x/crypto` | — | bcrypt password hashing |
| `nhooyr.io/websocket` | v1.8.7 | WebAO WebSocket support |
| `github.com/gorilla/websocket` | v1.4.2 | Additional WebSocket (indirect) |
| `github.com/xhit/go-str2duration/v2` | v2.0.0 | Duration string parsing for punishment timers |

**Go version requirement:** 1.19

## Architecture

### Entry Point

`athena.go` — parses CLI flags, loads config, then starts goroutines for:
- `athena.ListenTCP()` — AO2 TCP connections
- `athena.StartDiscordBot()` — Discord bot
- `athena.ListenWS()` / `athena.ListenWSS()` — WebAO plain/secure WebSocket
- `athena.ListenInput()` — CLI stdin (unless `-nocli`)

Shutdown via OS signal (`SIGINT`/`SIGTERM`) or a `FatalError` channel. Server restart supported via `syscall.Exec`.

### Internal Packages

| Package | Role |
|---------|------|
| `athena` | Core server logic, all command handlers, casino, mafia, punishments, pairing, jobs, shop, unscramble, hot potato, quick draw, giveaway, roulette, coinflip, AutoMod, IPHub |
| `db` | SQLite wrapper; chip balances, accounts, bans |
| `discord/bot` | Discord bot: slash commands, mod bridge, embeds, area/player listings |
| `logger` | Multi-level structured logger (stdout + log file) |
| `ms` | Master server advertisement |
| `packet` | AO2 protocol packet parsing |
| `permissions` | Role-based permission bitfield system |
| `playercount` | Concurrent player counting |
| `settings` | TOML config loading |
| `sliceutil`, `uidheap`, `uidmanager`, `webhook` | Utilities |

### Database

SQLite at `config/athena.db`. Stores:
- Moderator accounts (bcrypt-hashed passwords)
- Ban records
- Nyathena Chip balances (per IPID)
- Player account registrations
- Shop inventory / purchased items / active tags
- Job cooldowns and playtime tracking
- Unscramble win records

## Build & Run

```bash
make build        # go build -v -o bin/athena athena.go
make test         # go test -v ./...
make all          # build + test
make release      # goreleaser (requires goreleaser installed)
```

```bash
./bin/athena                          # config dir: ./config
./bin/athena -c /path/to/config       # custom config directory
./bin/athena -nocli                   # disable stdin CLI
```

**First run:** after build, copy `config_sample/` to `config/`, edit config files, then launch and run `mkusr` in the CLI to create the first moderator account.

## Configuration

Copy `config_sample/` to `config/` before first run.

### config/config.toml — [Server]

| Key | Default | Description |
|-----|---------|-------------|
| `addr` | `""` | Listen address (blank = all interfaces) |
| `port` | `27016` | TCP port |
| `name` | `"Unnamed Server"` | Server name |
| `description` | — | Server description |
| `motd` | — | Message of the day |
| `max_players` | `100` | Maximum connections |
| `max_message_length` | `256` | Maximum IC/OOC message byte length |
| `default_ban_duration` | `"3d"` | Default ban length |
| `multiclient_limit` | `16` | Max connections per IP |
| `asset_url` | `""` | URL for WebAO assets |
| `webhook_url` | `""` | Discord webhook URL for modcall notifications |
| `webhook_ping_role_id` | `""` | Discord role ID to ping on modcall |
| `punishment_webhook_url` | `""` | Discord webhook for ban/kick embeds |
| `enable_webao` | `false` | Enable plain WebSocket (WebAO) |
| `webao_port` | `27017` | WebSocket port |
| `enable_webao_secure` | `false` | Enable WSS (secure WebSocket) |
| `webao_secure_port` | `443` | WSS port |
| `tls_cert_path` / `tls_key_path` | `""` | TLS cert/key (leave blank for reverse proxy) |
| `webao_allowed_origin` | `"web.aceattorneyonline.com"` | Allowed WebSocket Origin (glob supported, `*` = any) |
| `message_rate_limit` | `20` | Max IC/OOC/music packets per window (0 = off) |
| `message_rate_limit_window` | `10` | Window in seconds |
| `ooc_rate_limit` / `ooc_rate_limit_window` | `4` / `1` | OOC-specific rate limit |
| `connection_rate_limit` / `connection_rate_limit_window` | `10` / `10` | Per-IP connection rate |
| `conn_flood_autoban` | `true` | Auto-ban IPs that flood connections |
| `conn_flood_autoban_threshold` | `6` | Rejections before auto-ban |
| `raw_packet_rate_limit` / `raw_packet_rate_limit_window` | `20` / `2` | Raw AO2 packet rate |
| `new_ipid_ooc_cooldown` | `10` | Seconds new IPIDs wait before OOC |
| `new_ipid_modcall_cooldown` | `60` | Seconds new IPIDs wait before modcall |
| `modcall_cooldown` | `0` | Seconds between modcalls per user |
| `automod_enabled` | `false` | Enable AutoMod banned-word enforcement |
| `automod_wordlist` | `"banned_words.txt"` | Path to banned-words file |
| `automod_action` | `"shadow"` | AutoMod action: `shadow` (shadow-send + torment list), `ban`, `kick`, `mute`, or `torment` |
| `iphub_api_key` | `""` | IPHub API key for VPN/proxy detection |
| `enable_casino` | `false` | Enable casino and player account system |
| `register_captcha` | `true` | Require captcha on `/register` |

### config/config.toml — [Discord]

| Key | Description |
|-----|-------------|
| `bot_token` | Discord bot token (blank = bot disabled) |
| `guild_id` | Discord server ID for slash command registration |
| `mod_role_id` | Discord role ID allowed to run moderation slash commands |

### Other Config Files

| File | Purpose |
|------|---------|
| `areas.toml` | Area definitions |
| `roles.toml` | Moderator role permissions |
| `characters.txt` | Allowed characters |
| `music.txt` | Music list |
| `8ball.txt` | (Optional) `/8ball` response pool. Falls back to a built-in 20-line classic list if missing or empty. |
| `backgrounds.txt` | Background list |
| `banned_words.txt` | AutoMod word list |
| `parrot.txt` | Parrot command word list |

### Discord Bot Setup

1. Create a bot at https://discord.com/developers/applications
2. Enable the **Message Content** intent
3. Copy the bot token into `[Discord]` → `bot_token`
4. Set `guild_id` to your Discord server ID
5. Optionally set `mod_role_id` to restrict moderation slash commands
6. Invite the bot with `applications.commands` and `bot` scopes

## Features Beyond Base Athena

### Punishment System (120+ Commands)

All punishment commands require `MUTE` permission unless noted. They support `-d <duration>` (max 24 h), `-r <reason>`, `-h` (hidden — suppresses the per-target OOC notification so the punishment applies silently; the issuer summary appends `(hidden)` so the mod can confirm), comma-separated UIDs, and the `global` keyword (applies to every non-moderator in the issuer's area). Multiple types stack on a single player.

The `-h` flag works on every applicator: single-effect commands (`/tsundere 7 -h`, `/tsundere global -h`), `/stack`, `/lovebomb`, `/sfxcurse`, `/shrink` / `/grow` / `/wide`, `/randompunishall` (also suppresses the area-wide "unleashed random chaos" announcement), `/translator curse`, and `/icwarp <uid>`. Self-applied effects (`/megamaso`, `/maso`) and the PvP `/coinflip` mini-game are unaffected since they aren't moderator-issued.

`/help punishment` is grouped by sub-theme (text effects / dere archetypes / animal filters / themed quotes / persona / protocol-viewport / audio / voice / timing / traps & contagion / stacking / removal) so the long list is scannable.

**Inspect:** `/punishments` lists your own active punishments with remaining durations (includes lag, mute, jail). Moderators run `/punishments <uid>` to inspect any player — entries show custom data, reason, and issuer tier.

**Remove:** `/unpunish <uid>` (all including lag) or `/unpunish -t <type> <uid>` (specific — works with `/unpunish -t lag <uid>`). `/unpunish all` clears every player in the area including lag. A non-numeric target is treated as a raw IPID (`/unpunish <ipid>`), clearing persisted punishments for offline players too. Full-removal forms also clear the `punishment_names.txt` showname stain (see "Punished Showname Random-Punishment Drip" below). Self-removal is gated for staff-issued punishments — see "/unpunish Self-Removal Protection" below.

#### Text Effects (60)
`/whisper`, `/backward`, `/stutterstep`, `/elongate`, `/uppercase`, `/lowercase`, `/robotic`, `/alternating`, `/fancy`, `/uwu`, `/pirate`, `/shakespearean`, `/caveman`, `/censor`, `/fromsoftware`, `/confused`, `/paranoid`, `/drunk`, `/hiccup`, `/whistle`, `/mumble`, `/slang`, `/cherri`, `/albhed`, `/morse`, `/vowelhell`, `/upsidedown`, `/autospell`, `/thesaurusoverload`, `/valleygirl`, `/babytalk`, `/thirdperson`, `/unreliablenarrator`, `/uncannyvalley`, `/chef`, `/karen`, `/passiveaggressive`, `/nervous`, `/sarcasm`, `/academic`, `/philosopher`, `/poet`, `/quote`, `/spaghetti`, `/essay`, `/rng`, `/haiku`, `/dreamsequence`, `/timewarp`

Wave-2 additions (14): `/zalgo` (combining-mark corruption), `/leetspeak`, `/smallcaps`, `/piglatin`, `/vaporwave` (ｆｕｌｌｗｉｄｔｈ), `/lisp`, `/spoonerism`, `/keysmash`, `/weeb` (350+ romaji corpus — word swaps, honorifics on names, interjections, desu~ particles), `/politician` (never answers directly), `/clickbait` (headlines star the speaker by name), `/markov` (babble generated from the **area's own recent chat history**; falls back to word shuffle in fresh areas), `/alliteration`, `/cipher` (escalates ROT13 → BINARY → BASE64 per message, then "decryption fails" and re-arms). Expanding transforms clamp output to the server's max IC length (`fitICBudget`) so punished messages are never dropped by the post-transform length check.

`/medieval` — rewrites the target's IC text into Olde-English / medieval speak: ~90 word-for-word swaps (`you`→`thou`, `your`→`thy`, `is`→`be`, `yes`→`aye`, `now`→`anon`…), a random heralds' cry prepended (`Hark!`, `Forsooth,`, `Prithee,`, `Zounds!`…) and a random courtly flourish appended (`…by my troth.`, `…mine liege.`, `…verily.`). The prefix and suffix are each rolled independently on top of the per-word swaps, so a single line has **100+ distinct renderings** (herald × flourish alone is 38 × 30 = 1,140 combinations, asserted by `medievalVariationCount` in tests). Transform in `internal/athena/punishments_medieval_cheese.go`.

#### Themed Quote Replacers (11)
`/gordonramsay`, `/biblebot`, `/grounded`, `/mime`, `/subtitles`, `/spotlight`, `/recipe`, `/rickroll`, `/pickup`, `/brainrot`, `/cheese`

`/cheese` discards the target's text entirely and replaces every message with one of **100+ "statements about cheese"** (facts, puns, varieties, and the running gag that cheese is, technically, a sauce). Same shared punishment plumbing as the others (`-d`/`-r`/`-h`, comma UID lists, `global`, `/stack`, persistence, `/unpunish -t cheese`). Corpus size is pinned by `cheeseLineCount` in tests so it can't silently shrink. Like `/medieval` it requires `MUTE` (mod-only) and is **not** in the self-applied `/maso`/`/megamaso` pool.

#### Persona / Personality (5)
`/clown`, `/jester`, `/joker`, `/tourettes`, `/translator`

#### Visibility / Cosmetic (8)
`/emoji`, `/invisible`, `/shrink`, `/grow`, `/wide`, `/areainiswap` (vertical/horizontal sprite-offset locks; reverse with `/unshrink` / `/ungrow` / `/unwide`), plus the two viewport-control punishments below.

- `/hidedisplay <uid>` — pushes the target's own sprite off-screen via a fixed self-offset so their text still appears in the chat box but their character does not. Looks especially funny on pairs (only the partner's sprite renders). Standard punishment flags: `-d`/`-r`/`-h`, comma UID lists, `global`, stackable, persists across sessions, removable with `/unpunish -t hidedisplay <uid>`.
- `/forcedisplay <uid>` — pins the target's character onto every non-moderator IC message in their area, overwriting each speaker's outgoing sprite and clearing the pair fields. While active, no other character can show in the viewport — the whole room renders as that one character. Moderators are exempt (their own sprite still shows). The hot-path lookup is gated by an atomic counter, so servers that never use `/forcedisplay` pay nothing on every IC packet. Same flag set as `/hidedisplay`; removable with `/unpunish -t forcedisplay <uid>`.

#### Protocol / Viewport Effects (6)
Weaponize the IC packet's non-text fields. Applied in `pktIC` (`applyProtocolPunishments`, `internal/athena/punishments_protocol.go`) **before** field validation, so every written value is validator-legal by construction. Standard flags, persistence, `/stack`, `/unpunish -t`.
- `/teleport <uid>` — random self-offset per message (±75 x, ±50 y); the sprite pops around the viewport. `/hidedisplay` wins if both are active.
- `/shakecurse <uid>` — forces the screenshake flag on every message
- `/randomflip <uid>` — coin-flips the sprite's horizontal facing per message
- `/forcecolor <uid> <0-9|white|green|red|orange|blue|yellow|rainbow>` — locks IC text colour (stored in customData; persisted via the 0x1F reason convention like `/translator`)
- `/nopreanim <uid>` — strips preanimations (PREANIM modifiers demoted, preanim name cleared)
- `/forcepreanim <uid>` — promotes idle/talk modifiers so the named preanim plays

#### Timing Effects (4)
`/slowpoke`, `/fastspammer`, `/lag`, plus:
- `/lifo <uid>` — buffers the target's IC messages and releases them in **reverse arrival order** (flush at 3 messages or 6 s, whichever first). Implemented in `internal/athena/punishments_lifo.go`; queues are keyed by `*Client` and self-flush, so disconnects can't leak.

#### Traps & Contagion (4)
Mechanic punishments hooked after the IC broadcast (`punishmentMechanicsOnIC`, `internal/athena/punishments_mechanics.go`) with cheap early-outs (atomic gate for love potions, one shared mutex for the area maps).
- `/contagious <type> <uid|global>` — plague mode: target gets `<type>` + a contagion marker; anyone who speaks within **5 s** of an infected player's message catches both and keeps spreading it. Victims inherit remaining duration and issuer tier. Moderators immune; area gets ☣️ announcements. `contagious`/`lag`/`minefield`/`lifo`/`stealthmute` can't be made contagious.
- `/minefield <uid|global>` — every message has a **1-in-6** chance to detonate a random 2-minute punishment (megamaso pool) with a 💥 announcement.
- `/silencebell [type] [-d dur]` — arms a one-shot trap on the issuer's area: the next non-moderator to speak is cursed (random unless a type is given). `status` / `off` subcommands. Dramatic 🔔 announcements on arm and trigger.
- `/stealthmute <uid|global>` — the target's IC **and** OOC messages echo back to them but reach nobody; they are never notified (the `-h` semantics are forced). Area logs tag suppressed OOC lines `(stealthmuted)`. Stealthmuted messages never trigger traps/contagion/love potions. Lift with `/unpunish -t stealthmute`.

#### Audio (2)
- `/sfxcurse <uid> <sfx-url>` — forces an SFX file to play on every IC message; URL must be `http(s)://…` or under `/base/sounds/…`
- `/unsfx <uid>` — lifts the SFX curse

#### Voice Chat (5)
Sabotage a player's server-relayed voice-chat audio. Require `enable_voice` and `MUTE`; support `-d`/`-r`/`-h`, comma-separated UIDs, `global`, `/stack`, `/unpunish` and DB persistence exactly like the text punishments. They act on the punished speaker's Opus frame *flow* in the documented `pktVSFrame` hook — the relay never decodes audio, so they stay CGO-free (no pitch-shift). Unlike `/vmute` (which ejects an IPID from voice entirely), these keep the target in the room and just corrupt what the room hears.
- `/voicemute <uid>` — drops every one of the target's voice frames; silent to the room
- `/voicestatic <uid>` — drops ~60% of frames; choppy, breaking up
- `/voicegarble <uid>` — drops ~88% of frames; barely intelligible
- `/voicecutout <uid>` — gates frames on a ~650 ms on/off cycle; walkie-talkie effect
- `/voicestutter <uid>` — randomly replays the previous frame; a glitchy voice stutter

#### Dere Archetypes (25)
Original 10: `/tsundere`, `/yandere`, `/kuudere`, `/dandere`, `/deredere`, `/himedere`, `/kamidere`, `/undere`, `/bakadere`, `/mayadere`

Nyathena additions: `/smugdere`, `/deretsun`, `/bokodere`, `/thugdere`, `/teasedere`, `/dorodere`, `/hinedere`, `/hajidere`, `/rindere`, `/utsudere`, `/darudere`, `/butsudere`, `/sdere`, `/mdere`, `/tsuyodere`

`/omnidere` picks a random dere flavour for **every** IC message from the full pool — maximum tonal whiplash.

#### Punishment Stacking
```
/stack <type1> <type2> [...] [-d duration] [-r reason] [-h] global | <uid1>,<uid2>,...
```

#### Punishment Tournament Mode
Voluntary competitive mode where participants receive 2–3 random punishments; most IC messages sent wins.
```
/tournament start|status|stop   # requires MUTE
/join-tournament                # any user
```

#### `/megamaso` — Self-applied Stack Mode
Self-applied "max chaos" mode. The first call rolls a random punishment; each subsequent `/megamaso` while still under the effect **adds another random punishment to the stack** instead of replacing it. Lets a player pile on as many concurrent effects as they like. Duration defaults to 10 minutes and can be set with `-d` (max 24 h).

```
/megamaso              # any user — default 10 min per layer
/megamaso -d 1h        # each stacked layer lasts 1 hour
```

#### `/maso` — Self-applied Single Effect
Rolls a random punishment from the pool and applies it to yourself. Typing `/maso` again rerolls to a different effect and resets the timer. Duration defaults to 10 minutes and can be set with `-d` (max 24 h).

```
/maso              # any user — default 10 min
/maso -d 30m       # apply for 30 minutes
/maso              # (again while active) reroll
```

#### Coinflip Challenge
Area-scoped 30-second PvP challenge. Players must choose opposite sides.
```
/coinflip <heads|tails>
```

#### Punishment Audit Log
By default, every `ADMIN` gets a live `[AUDIT]` OOC alert whenever a moderator issues a punishment-system command (dere archetypes, text effects, protocol/viewport curses, traps, voice curses, `/stack`, etc.) — naming the issuing moderator (real name, not the shadow-anonymized one — admins always see through `SHADOW`), the punishment, the target(s), and the reason/duration, even when the command was run with `-h` (which only suppresses the *target*-facing notice, never the admin-facing one). Every trip is also written to the server's persistent audit log (`logger.WriteAudit`) so the record survives a restart. The issuing admin (if the issuer is themselves an admin) never gets their own alert. Self-applied effects (`/maso`, `/megamaso`), the showname-punisher drip, and a bystander catching a contagion are never reported since they aren't a moderator issuing a fresh command.

```
/punishaudit <on|off>   # ADMIN — toggle these alerts for your own session (default: on)
```

Implemented in `internal/athena/punishment_audit.go` (`alertPunishmentIssued`), hooked into every punishment command's apply path — `cmdPunishment` (the shared applicator behind the great majority of punishment commands), `/stack`, `/lovebomb`, `/sfxcurse`, `/shrink`/`/grow`/`/wide`, `/forcecolor`, `/contagious`, `/silencebell`, `/translator curse`, `/randompunishall`, `/icwarp`, and `/charcurse`.

### Potions System
Self-applied fun effects accessed via `/potion <name>`. Duration defaults to **5 minutes** and can be set with `-d` (max 24 h): `/potion -d 30m drunk`. `/potions` lists the cabinet; `/potion off` flushes every active potion.

| Potion | Effect |
|--------|--------|
| `drunk` | Slurs and shuffles letters |
| `uwu` | Wewites yowo wowds wike this |
| `shy` | Stuttering, hesitant speech |
| `dramatic` | Shakespearean tongue |
| `pirate` | Yarrr! |
| `poet` | Poetic flourish |
| `caveman` | Talk simple. Words short. |
| `fancy` | Unicode fancy characters |
| `chef` | Swedish-Chef-isms |
| `cherri` | Capitalizes Every Word |
| `omnidere` | Each line picks a random dere flavour |
| `zalgo` | C̴o̷r̶r̸u̵p̷t̶s̸ your text with creeping zalgo marks |
| `love` | 💘 Auto-sends a pair request to the next player who speaks in your area (consent preserved — they still accept with `/pair`; mutual interest completes instantly) |
| `character` | Auto-rotates your character every 30 seconds |

The `character` potion is a per-client goroutine, not a punishment — `/potion off` cancels its rotation cleanly. The `love` potion is likewise special: it arms a per-client flag (atomic-counter-gated on the IC hot path) and `/potion off` disarms it.

### Persistent Pairing
UID-based mutual pairing surviving area/character changes. Pair messages (like every OOC-channel/server message — `/global`, `/pm`, `/roll`, `/rps`, `/8ball`, etc.) label players by their **OOC name** via the shared `oocDisplayName` helper, falling back to showname then character only when the OOC name is blank so the label is never empty.

**Auto-unpair on disconnect.** When either partner leaves the server, `clientCleanup` calls `clearPairLinksOnDisconnect` (mirrors `/unpair`'s full bidirectional scan) while the leaver's UID is still valid, clearing `PairWantedID`/`ForcePairUID` on the leaver **and** on every client that referenced them (by UID or CharID) and notifying the surviving partner. This prevents the IC pair desync where a partner kept a stale pair pointing at the departed player's now-recyclable UID/CharID — and stops a stale pending request from auto-completing against a recycled slot later.

A pending pair request never blocks speech: while waiting for the partner to accept, the requester's IC messages go out with a plain `-1` no-pair value (never the `-1^` order-suffixed form, which some desktop forks drop outright and webAO parses as `NaN`).

`/unpair` is a full bidirectional reset: it clears `PairWantedID` and `ForcePairUID` on every client that references the canceller (by UID or by current/historical CharID), preventing the desync where a stale pair-wanted-id lingered on a peer after the canceller's character changed.

**Discoverability:** `/lfp` toggles a Looking-For-Pair flag; `/pairlist` lists every flagged player in the area with UID, display name and character.
```
/pair <uid>
/unpair
/lfp
/pairlist
```

### Character Curse
One-time forced character swap (requires KICK). Target may freely change afterwards.
```
/charcurse <uid> <charname>
```

### Forced Position (`/forcepos`)
CM tool for staging a scene: pushes one or more players in the caller's own area into a specific courtroom position — the same `def`/`pro`/`wit`/`jud`/`hld`/`hlp`/`jur`/`sea` set `/pos` lets a player choose for themself. Gated on the `CM` permission, so both server CM-permission holders and area-designated CMs (`/cm`) can use it, same as `/invite`, `/lock`, and `/spectate`.

```
/forcepos <uid> <position>              # force one player
/forcepos <uid1>,<uid2>,... <position>  # force several players at once
/forcepos all <position>                # force everyone in the caller's area
```

Not a punishment — nothing is persisted or stacked, and the target is free to run `/pos` themself again right after. Targets outside the caller's area are silently skipped (mirroring `/charselect`'s per-UID force branch), so a CM can't reach into a different room. Implemented in `internal/athena/commands_area_admin.go` (`cmdForcePos`), reusing the same `validPositions` list `/pos` validates against.

### Random Character Curse (`/curserandomchar`)
ADMIN-only curse (`internal/athena/curse_randomchar.go`) that forces the target's character to randomly change every 1–5 seconds, forever, until an admin lifts it.

```
/curserandomchar <uid>      # ADMIN — arm the curse
/uncurserandomchar <uid>    # ADMIN — lift it
```

Unlike the `character` potion's per-session auto-rotation, the curse is **persisted by IPID** in the `RANDOMCHAR_CURSES` table (DB migration 23), the same way `/musicban` persists — a curse tied only to the live `*Client` would vanish the instant the target reconnects, since a fresh connection gets a brand-new `*Client`. Instead, `restoreRandomCharCurse` (called from `pktReqDone` right alongside `restorePunishments`) checks the joining client's IPID against the table and re-arms the curse on every join, so **relogging cannot be used to escape it** — nor can a full server restart.

Each armed connection runs a single per-client watcher goroutine that picks a new random 1–5 second interval every tick, swaps to a random free character via the same `getRandomFreeChar`/`ChangeCharacter` path `/randomchar` uses (skips a tick if the target is tunged), and exits cleanly via `client.done` the moment the connection closes — so a cursed player who disconnects can never leak a goroutine, and `/uncurserandomchar` clearing the active flag makes the next tick exit on its own.

### Possession
Speak through another player's character. Shares one sprite-spoof + pair-spoof pipeline (`internal/athena/possess.go`, applied in `pktIC`):

- `/possess <uid> <message>` (ADMIN) — one message rendered exactly as the target.
- `/fullpossess <uid>` (ADMIN) — persistent silencing possession; identical to `/truepossess`.
- `/truepossess <uid>` (ADMIN) — persistent silencing possession; identical to `/fullpossess`. (Both names kept; both share `beginPossession`.)
- `/unpossess` (SHADOW) — stop a full/true possession; lifts the mute.

**Pair-spoof (applies to all flavours).** A possessed IC message reproduces the *target's* pairing, not the possessor's: `applyPossessedPairFields` resolves the target's partner (normal `/pair` or UID-locked `/forcepair`) and stamps the partner's sprite (`OtherCharID`/`OtherName`/`OtherEmote`/`OtherOffset`/`OtherFlip`) onto the packet, and the possessed message also adopts the target's own self-offset/flip. Without this, possessing a paired player dropped their partner from the viewport — an obvious "this is a possess" tell. The possessor's own pair-info state is preserved (the bottom-of-`pktIC` `SetPairInfo` uses saved `ownFlip`/`ownSelfOffset`).

**Silencing (`/fullpossess` and `/truepossess`).** The target is marked `trueMuted` (per-client flag, hot-path-gated by the `activeTruePossess` atomic counter so unused servers pay nothing). While active: their IC and OOC are echoed back to *only them* (stealthmute semantics — their client looks normal) but reach nobody; their OOC commands (`/global`, `/pm`, `/modchat`, `/a`, …) are swallowed undispatched; and their showname / OOC name are frozen (the PU broadcasts are skipped) so they can't rename into a distress signal — which also keeps the possessor's spoofed messages pinned to the target's original showname. Suppressed lines are logged tagged `(truepossessed)` / `(suppressed during /truepossess)` for staff audit. The mute is lifted by `/unpossess`, switching target, or either party disconnecting (`endTruePossession` + `clientCleanup`, keeping the atomic gate balanced). Admins only — shadow mods no longer have access to any possession command.

### Admin Lock (`/adminlock`)
`/adminlock` (ADMIN) toggles an admin-only seal on the caller's area: an admin-locked area refuses entry to **everyone but administrators** — even moderators and shadow mods who hold `BYPASS_LOCK`, and even players on the invite list. Unlike `/lock`, there is no emergency-bypass or invite escape hatch. Players already inside are not evicted (it blocks new entries only). The check sits at the top of `Client.ChangeArea`, before the normal lock logic. The area is also set to `LockLocked` so it displays as locked in ARUP; a new `area.adminLocked` bool (cleared on `Area.Reset`) carries the extra seal. A non-admin cannot `/unlock` or `/lock` an admin-locked area out from under it — only `/adminlock` (by an admin) lifts it, which reopens the area (`LockFree`, invites cleared). Area 0 cannot be admin-locked.

### Mafia Social Deduction Minigame
Fully featured in-server Mafia (4–20+ players) with automatic role pools, day/night phases, lynch voting, night actions, last wills, graveyard, whisper, and phase timers.

**Roles — Town:** Villager, Detective, Doctor, Sheriff, Bodyguard, Vigilante, Mayor, Escort
**Roles — Mafia:** Mafia, Shapeshifter, Godfather
**Roles — Neutral:** Jester, Witch, Lawyer, Arsonist, Serial Killer, Survivor

Key commands: `/mafia create|join|start|vote|act|tally|graveyard|whisper|will`

See `MAFIA_COMMANDS.md` for the full reference.

### Casino System

Enabled with `enable_casino = true`. Requires player accounts (`/register`).

**Virtual Currency — Nyathena Chips:** New connections start with 500 chips; max 10,000,000. Stored in SQLite.

**Games:**

| Command | Game |
|---------|------|
| `/bj` | Blackjack (6-deck, split, double, insurance; up to 6 players) |
| `/poker` | Texas Hold'em (up to 9 players, 500-chip buy-in) |
| `/slots` | Slots with area jackpot pool |
| `/croulette` | European Roulette (single-zero) |
| `/baccarat` | Baccarat (player / banker / tie) |
| `/craps` | Craps lite (pass / don't-pass) |
| `/crash` | Crash multiplier game |
| `/mines` | Minesweeper-style grid (1–24 mines, 5×5) |
| `/keno` | Keno (pick 1–10 numbers from 1–80) |
| `/wheel` | Prize wheel (~92.5% RTP) |
| `/bar` | Bar with 33 drinks of wildly varying variance |

**Economy:** `/chips`, `/chips top`, `/chips area`, `/chips give`, `/richest`

**Earning Without Gambling:**
- Jobs with cooldowns: `/busker`, `/janitor`, `/paperboy`, `/clerk`, `/bailiffjob`
- Unscramble events every 30 min–3 h (first correct IC answer wins 10 chips)

**Shop (`/shop`):** 30 cosmetic tags (1,000–10,000,000 chips), job cooldown reduction passes, job reward bonus passes.

**Staff:** `/casinoenable`, `/casinoset`, `/grantchips`

See `CASINO_COMMANDS.md` for the full reference.

### Custom Tags (Admin)

Admins can mint cosmetic tags at runtime without rebuilding the server. Custom tags share the same equip system as the built-in shop tags (`/settag`, `[Name]` shown in `/gas` and `/players`) but have their own DB table (`CUSTOM_TAGS`) and are only ownable via admin grant — they never appear in `/shop` and never cost chips.

| Command | Permission | Purpose |
|---------|------------|---------|
| `/createtag <id> <display name>` | `ADMIN` | Create a new custom tag. Id must be 2–32 lowercase letters/digits/underscores; display name (≤30 chars, no `[` or `]`) is what shows in brackets. Rejects ids that collide with built-in shop tags. |
| `/deletetag <id>` | `ADMIN` | Delete a custom tag and clean up every grant + active equip referencing it. Built-in tag ids are protected. |
| `/grantcustomtag <username> <tag_id>` | `ADMIN` | Grant any tag (built-in or custom) to a registered player account. Looks up the account's IPID by username — the player must have logged in at least once so the IPID is linked. Idempotent. |
| `/revokecustomtag <username> <tag_id>` | `ADMIN` | Revoke a previously granted tag from an account; clears it from their active equip if equipped. |
| `/listcustomtags` | none | List every custom tag with its id, name, creator, and creation date. Visible to all players. |

**Workflow:**
```
admin> /createtag founder ⭐ Founder
admin> /grantcustomtag alice founder
alice> /settag founder       → alice now wears [⭐ Founder]
```

### Per-Area Logging

Daily-rotating log files per area under `logs/<AreaName>/<AreaName-YYYY-MM-DD.txt>`.
Format: `[HH:MM:SS] | ACTION | CHARACTER | IPID | HDID | SHOWNAME | OOC_NAME | MESSAGE`
Actions: IC, OOC, AREA, MUSIC, CMD, AUTH, MOD, JUD, EVI.
Enabled with `enable_area_logging = true` in `[Logging]`.

### AutoMod
Word-list-based automatic enforcement. Covers IC message text, IC showname, OOC message text, and OOC username — slurs in any of those fields trigger the configured action.

**Actions:** `shadow` (default), `ban` (permanent, silent), `kick`, `mute`, or `torment` (drop the message + torment list).

**`shadow` (default action):** the censored message is **shadow-sent** — echoed back to the sender so their client shows it as sent, while the packet is dropped for every other client, so nobody ever sees the slur. The speaker's IPID is also added to the torment list (same list as `/lag`: ghost/delayed messages and random silent disconnects, persisted across reconnects). Clean messages are unaffected — only messages that trip the censor are swallowed, but once tripped the speaker stays on the torment list until a moderator lifts it. In `pktIC` the shadow trip is folded into the existing stealthmute `silenced` delivery path (so the sender's echo is a fully-processed, well-formed packet) and runs **before** the torment branch, so a censored message can never leak out through `handleTormentedIC`'s delayed rebroadcast; it is also excluded from the area's IC history so `/markov`/`/icwarp` can't regurgitate it. Area logs tag suppressed lines `(censored)`.

**Staff alerts:** every censor trip (any action, and the showname censor below) sends a `[CENSOR]` OOC alert to everyone holding `MOD_CHAT` — who tripped, where, the matched entry, the (truncated) text, and what the server did about it. Every alert carries the opt-out hint; each staff member can mute them per-session with `/censoralerts off` (`/censoralerts on` re-enables; bare `/censoralerts` shows the current state). Manual torment additions (`/lag`) deliberately never alert — only censor trips do.

**Torment list tooling:** `/tormentlist` (MUTE) lists every IPID on the torment/lag list with any connected sessions (UID, name, area) or `offline`. `/untorment <ipid>` (BAN) removes one entry; `/untorment all` purges the entire list — every IPID, in memory and in the DB (which also cancels all pending torment disconnect timers).

**Filter-evasion normalization.** Both `banned_words.txt`/`censored_names.txt` entries and the text being checked are run through `normalizeForFilter` (`internal/athena/text_filter_normalize.go`) before matching, so stylizing, spacing out, or leetspeaking a slur doesn't evade the filter. Only letters survive normalization — everything else is either substituted into a letter or dropped — which defeats:
- stylized Unicode letters — mathematical bold/script/fraktur, fullwidth, circled, superscript, etc. — via NFKD compatibility decomposition (`golang.org/x/text/unicode/norm`), which folds e.g. `𝓷𝓲𝓰𝓰𝓮𝓻` or fullwidth `ｎｉｇｇｅｒ` back to plain `nigger`
- combining marks (accents, "zalgo" corruption) stacked onto letters, and zero-width/format characters (ZWSP, ZWNJ, ZWJ, word joiner, BOM, soft hyphen, ...) inserted mid-word — the latter fall out naturally since only Unicode letters survive
- non-Latin homoglyphs — Cyrillic, Greek, Armenian, and Cherokee letters that render almost identically to a Latin letter (`а`/`е`/`о`/`р`/`с`, `α`/`ο`/`ρ`, Armenian `հ`, Cherokee `Ꮟ`, etc.) — via a table cross-checked against Unicode's own `confusables.txt` (the data UTS #39 security profiles are built from), not hand-guessed
- leetspeak digit/symbol substitutions (`n1gg3r`, `$h1t`) — a small, high-confidence set (`0137458@$!`) that skips ambiguous digits like `2`/`6`/`9` which collide too often with ordinary numbers in chat
- spacing/punctuation insertion (`n i g g e r`, `n.i.g.g.e.r`, `n-i-g-g-e-r`) — any character that isn't a letter after substitution is dropped entirely, so inserted separators never break up a banned substring
- letter stuffing (`niggggger`) — a run of **3 or more** identical consecutive letters collapses down to 2 (not 1), applied identically to both sides of the match

Runs collapse to 2 rather than 1 on purpose: a genuinely double-lettered entry needs to keep both copies, or it silently turns into something dangerous. Collapsing to 1 would shrink `nigger` itself to `niger` (a substring of the common word `nigeria`) and `ngger` to `nger` (a substring of `anger`/`danger`/`finger`/`stronger`/`messenger`/...). Capping at 2 keeps those intact while still defeating obvious 3+-repeat stuffing.

**Load-time safety gate.** Normalization can still turn a specific-looking entry into something dangerously generic — e.g. a postcode fragment `l36` normalizes to `le` (digit `3`→`e` via leetspeak, digit `6` dropped), and `tR0N` normalizes to `tron`, which is a substring of `electronic`/`strong`/`astronomy`. `loadWordListFile` rejects (and logs a warning naming the entry) any normalized result that:
- is shorter than `minNormalizedEntryLen` (4 letters) — below that, a substring match is either unconditional (empty) or broad enough to fire on huge swaths of ordinary chat
- collides with a common English word — `collidesWithCommonWords` (`internal/athena/common_words.go`) checks the normalized entry against an embedded ~10,000-word frequency list ([google-10000-english](https://github.com/first20hours/google-10000-english)) and rejects it if it's a substring of some other, unrelated common word (an entry that equals a real word outright is fine — only being a fragment *inside* a different word counts)

Both checks run once at load time (startup and `/reload`), not on the hot per-message path. This trades word-boundary awareness for evasion resistance: even with both gates, a banned entry can in principle match across what used to be separate words. That's an extension of the false-positive risk substring matching already had (e.g. `ass` inside `class`), not a new category — keep `banned_words.txt`/`censored_names.txt` entries as specific as practical, and check the startup/`/reload` logs for skipped-entry warnings after editing either file.

### Censored Showname Shadow-Send
`config/censored_names.txt` lists shownames (or substrings of them, case-insensitive) that nobody is allowed to speak under — independent of `automod_enabled`/`banned_words.txt`. Matching goes through the same `normalizeForFilter` Unicode-bypass normalization as AutoMod. Every IC message a player sends while their showname matches an entry is:
- **shadow-dropped** — echoed back to only the sender (their client shows it as sent) while no other client ever receives it, including the very message that tripped the check.
- **torment-listed** — their IPID goes into the torment set exactly like `/lag` (persisted), so they get the ghost/delayed-message treatment and eventual silent disconnects until a moderator removes them (`/untorment <ipid|all>`, `/unpunish -t lag`, or `/unlag`).
- **reported to staff** — the same `[CENSOR]` OOC alert as AutoMod trips, with the `/censoralerts off` opt-out hint.

There is deliberately **no permanent stealthmute anymore**: a player who switches to a clean showname talks normally again (they stay on the torment list until a moderator lifts it), but every message sent under a censored showname is swallowed and re-alerts staff. Implemented in `internal/athena/showname_censor.go`, hooked into `pktIC` in `netprotocol.go` right alongside the automod checks — before the torment branch, so a censored message can never leak through the delayed torment rebroadcast. The list is hot-reloadable via `/reload` like `parrot.txt`/`backgrounds.txt`; a missing file simply leaves the feature inactive.

### Punished Showname Random-Punishment Drip
`config/punishment_names.txt` lists shownames (or substrings, case-insensitive, matched through the same `normalizeForFilter` Unicode-bypass normalization as AutoMod) that trigger the **showname punisher**. The first IC message sent under a matching showname **stains the speaker's IPID**: one random punishment from the `/megamaso` pool is applied immediately, and another every minute after that (each lasting 10 minutes, persisted to the DB like any moderator-issued punishment), so effects pile up until staff intervene. Unlike the censored-showname shadow-mute, nothing is silenced — the drip is loud and visible to the target.

The stain sticks to the **IPID, not the showname**: renaming to a clean showname (or reconnecting — the drip watcher re-arms on join, `restoreShownamePunishStain` in `pktReqDone`) does not stop the drip. It stops only when a moderator runs a full-removal `/unpunish` form — `/unpunish <uid>`, `/unpunish <ipid>` (non-numeric targets are treated as raw IPIDs, `/musicunban`-style, so offline players can be cleared too), or `/unpunish all` — all of which clear the stain along with the punishments. After being unpunished, speaking under a different (clean) showname is fine; only using a listed showname again re-triggers the stain. Multiclients sharing a stained IPID are throttled to one combined drip per minute.

Implemented in `internal/athena/showname_punishment.go`, hooked into `pktIC` alongside the censored-showname check. The file is optional (missing = feature off), independent of `automod_enabled`, and hot-reloadable via `/reload`; entries go through `loadWordListFile`'s normalization and safety gates. The per-connection drip watcher goroutine mirrors the leak-free `curseRandomCharWatch` shape (exits on `client.done` or when the stain is cleared).

### Doki Area Effect
Per-area chaos mode for literature-club-themed areas. Enable with `doki_area = true` on the area entry in `areas.toml`. Each IC message rolls independently:
- 1/300 — replace text with a Haschen-themed Monika-style quote
- 1/100 — replace text with one of 130+ original Haschen-themed poems (cute, wholesome, devotional, unhinged, gorey, horror, or letter-scrambled horror; capped at 256 chars)
- 1/200 — replace text with a dark Haschen anagram (unscrambles to a hidden Haschen-themed phrase)
- 1/150 — replace text with a zalgo-corrupted Haschen catchphrase
- 1/100 — zalgo-scramble the player's actual text
- 1/250 — surprise-swap the area background to a random one

Stacks freely with `mirror_area` and `punishment_area` on the same area.

### Per-Area Judge Button Toggle
The WT/CE judge buttons (Witness Testimony, Cross Examination and the verdict gavels — the `RT` packet handled by `pktWTCE`) can be disabled per area so players can't spam them. They default to **enabled** (upstream behaviour preserved).

- **Config:** add `judge = false` to an area's entry in `areas.toml` to disable the buttons. Omit the line (or set `judge = true`) to keep them on. The field is tri-state (`*bool`) so an absent value means "enabled".
- **Runtime:** staff with `MODIFY_AREA` run `/judge <true|false>` (also accepts `on`/`off`) to flip the buttons live without a restart.
- When disabled, an attempt to play WT/CE is rejected with *"The judge buttons are disabled in this area."* — the check runs before the existing `CanJud()`/character checks in `pktWTCE`.

### Tiered `/randomsong`
`/randomsong` plays a random track from `music.txt`. Cooldown is tiered:
- regular users — `random_song_cooldown` (default 20 s)
- DJ permission — `random_song_cooldown_dj` (default 5 s)
- moderators — `random_song_cooldown_mod` (default 0 s, unlimited)

### Shadow Mod Visibility
Shadow moderators (`SHADOW` perm bit, no `ADMIN`) are completely hidden from `/gas`/`/players` for non-admin viewers — no "Mod:" line is shown at all. Only admins see them, labelled as `Mod: <name> (shadow)`. Previously the line still rendered as `Mod: Moderator`, which let other moderators infer staff status.

`/hide` — vanishing entirely from `/players`, `/gas`, and room player counts (toggle) — is `ADMIN`-only. Shadow mods do not carry the ADMIN sentinel, so despite their other stealth traits they cannot `/hide`; only full admins can go invisible.

**Shadow mods are `/ignore`-able.** Real moderators and admins can never be silenced with `/ignore` (the command refuses, and their IC/OOC messages bypass every recipient's ignore list). Shadow mods are deliberately exempt from that protection: an un-ignorable sender betrays staff status, so to anyone who ignores them a shadow mod must disappear exactly like a normal player. The single source of truth is `senderBypassesIgnore(perm)` (`internal/athena/client.go`) — `IsModerator(perm) && !IsShadow(perm)` — applied at the `/ignore` guard (`cmdIgnore`) and at all three ignore-list bypass sites (IC broadcast, OOC broadcast, and the buffered `/lifo` release). Pinned by `TestSenderBypassesIgnore`.

### Admin Role Hiding (`/admin hide`)
`/admin hide` (`ADMIN`) lets an admin hide their `ADMIN` role from `/players`/`/gas` for non-admin viewers — their UID, showname, character and IPID are completely unaffected; only the `Mod: <name>` line is suppressed, exactly like a shadow mod is hidden from non-admin viewers. Unlike shadow-mod hiding, other **admins** still see the line, tagged `Mod: <name> (hidden)` so staff oversight is never lost.

```
/admin hide      # ADMIN — hide your ADMIN role from other moderators
/admin unhide    # ADMIN — reveal it again
/admin status    # ADMIN — check whether you're currently hidden
```

The setting is keyed by the moderator's **account name** (not the live connection), so it survives a reconnect or re-login as the same account — it is cleared only by an explicit `/admin unhide` or a server restart (the state is in-memory only, never persisted to disk). Implemented in `internal/athena/admin_hide.go`; the display gate lives alongside the existing shadow-mod check in `writeEntry` inside `cmdPlayers` (`internal/athena/commands_moderation.go`).

### `/unpunish` Self-Removal Protection
Punishments now record the issuer's permission tier (`IssuerSystem`/`IssuerMod`/`IssuerShadow`/`IssuerAdmin`) in the `PUNISHMENTS.ISSUER_TIER` column. A regular moderator cannot use `/unpunish` to lift a punishment that an admin or shadow mod applied to them — `/unpunish self`, `/unpunish -t <type> self`, and the self-target slice of `/unpunish all` are all gated. Admins and shadow mods bypass the gate. Persists across restarts via DB migration 18.

### Persistent Music Ban (`/musicban`)
`/musicban <uid> [-r reason]` bans the target's IPID from playing music — both jukebox entries (`music.txt`) and streaming URLs — across sessions. Persists via the `MUSIC_BANS` table (DB migration 22). Hot-path check is a single RWMutex map lookup, seeded from the DB at startup.

**Quiet-area carve-out:** if the area has **fewer than 3 people** in it, the ban is **bypassed** and the music change is allowed — banned players can still set the mood in empty/small rooms but can't bother a populated one. Moderators are always exempt. Area-change MC packets are unaffected (a music-banned player can still move between rooms).

| Command | Permission | Behaviour |
|---------|------------|-----------|
| `/musicban <uid> [-r reason]` | `MUTE` | Persistently ban the target's IPID. Idempotent; re-banning overwrites the reason and issuer. |
| `/musicunban <uid\|ipid>` | `MUTE` | Lift a music-ban. Accepts a connected target's UID or a raw IPID, so offline players can still be unbanned. |
| `/musicbans` | `MUTE` | List every active music-ban with reason, issuer and timestamp (newest first). |

### Hot Config Reload (`/reload`)
`/reload` (in-game, `ADMIN`) atomically re-reads every supported config/data file from disk and swaps it in without restarting the server. Also available as the `reload` CLI command on stdin and via `SIGHUP`.

Each reloadable list lives behind a `sync/atomic.Pointer` (see `internal/athena/livereload.go`), so a swap is a single atomic store — readers on the hot IC path never lock and never see a torn value. A parse error in any one file aborts the whole reload before anything is published, so a bad file never leaves the running server half-updated. Validated end-to-end with `go test -race`.

**Reloaded:**
- `characters.txt` — **append-only** (see safety constraint below)
- `music.txt` — full reload; the pre-built SM packet sent on every client join is rebuilt in lockstep
- `cdns.txt`
- `backgrounds.txt` — `/bglist`'s cached output string is rebuilt in lockstep
- `parrot.txt`
- `8ball.txt` (optional; missing file leaves the current value intact)
- `banned_words.txt` (only when automod is enabled)
- `censored_names.txt` (optional; independent of automod_enabled; missing file leaves the current value intact)
- `config.toml` motd and description (the existing hot-config whitelist)

**`characters.txt` safety constraint — append-only.** Connected AO2 clients reference characters by **slot index**, so inserting in the middle, removing, reordering or renaming an existing slot would silently desync every connected player (the person on slot 2600 would suddenly be on whatever character used to be slot 2595). To prevent that, the reload **validates that every existing slot is unchanged** and only accepts entries appended at the end of the file. If the new file changes any pre-existing slot, the reload is rejected with a precise message naming the first bad slot (e.g. `"characters.txt: slot 12 changed from 'X' to 'Y' — character reload is append-only; add new characters at the END of the file (restart the server to reorder, rename or insert)"`). When new characters are appended, every area's character-slot table is grown first via a new `Area.GrowTaken(n)` method so newly-selectable slots can never panic the IC path with an out-of-bounds.

**NOT reloaded** (would require invasive work and is unsafe without restart): areas, listener ports/addr, rate-limit windows, max_players, roles, the server name. These are still restart-only.

### In-Game Console Log Viewer (`/terminal`)
`/terminal [lines]` (`ADMIN`) prints the last N lines of the server's console/log output as a single OOC message, so an admin without shell access to the host can still check on the server. Defaults to 50 lines when no argument is given; capped at 500 per request.

```
/terminal          # last 50 lines
/terminal 100      # last 100 lines
```

Backed by a bounded in-memory ring buffer (`logger.RecentLines`, capped at 2000 lines) that every `log()` call feeds independently of `LogStdOut`/`LogFile`/the TUI's own tap, so `/terminal` works whether or not stdout or file logging is enabled, and reflects the same `CurrentLevel` filtering as the real console. Implemented in `internal/athena/terminal_log.go` and `internal/logger/logger.go`.

### IPHub VPN Firewall
When `iphub_api_key` is set, moderators can run `/firewall on|off` from in-game **or** from Discord (`/firewall on|off` slash command). New IPs are checked against IPHub; VPN/proxy IPs are rejected. Known IPs are never re-checked (respects 1,000 requests/day free tier).

### Discord Bot Security Toggles
Mirrors the in-game security commands so moderators don't have to be logged in to the AO server during an incident.

| Discord slash | Behaviour |
|---------------|-----------|
| `/firewall on` / `/firewall off` | Toggle the IPHub VPN/proxy firewall. Refuses to enable if `iphub_api_key` is unset. |
| `/lockdown on` / `/lockdown off` | Toggle the server-wide new-IPID lockdown. |
| `/lockdown whitelist_all` | Whitelist every currently-connected IPID so they can rejoin during lockdown. |

### `/charshuffle` and `/uncharshuffle`
Mirrors `/nameshuffle` but operates on character IDs. Randomly permutes everyone's character (char 1 → char 2, char 2 → char 5, etc.) using Sattolo's algorithm so every player ends up on someone else's sprite. Originals are remembered per-client so `/uncharshuffle` puts everyone back exactly. Requires `MUTE`.

### `/reversename` and `/unreversename`
Flips the rune order of a player's showname (`Phoenix` → `xineohP`). `/reversename <uid>` (or a comma-separated UID list) targets specific players; `/reversename global` reverses every player in the caller's area. `/unreversename` restores with the same target forms. The forced showname in effect before the flip is remembered per client, so `/unreversename` restores it exactly — even when the reverse was stacked on top of a `/forcename`. Reverses by runes (multi-byte/accented characters survive) and re-encodes so AO2 escape sequences are never split. Requires `KICK`.

### `/getmusic`
For every player. Prints the URL of the song currently playing in the area **and** re-sends the MC packet to just the requesting client. Useful when a client's audio handling glitched and the song never started — the user can either copy the URL or have the bot poke their player to restart playback locally.

### Music Sync on Area Entry (Bug Fix)
Joining an area — the initial connect into area 0, or any subsequent `/area` change — now sends the area's currently-playing track to the joining client as an MC packet. Previously nothing synced a fresh arrival to music already in progress: the client just sat in silence until the track changed again or the player thought to run `/getmusic` by hand. The fix lives in the shared `JoinArea` (`internal/athena/client.go`), so both the initial join and every area change are covered from one place. No packet is sent when the area has no `CurrentSong()` yet (a fresh area, or nothing has played since the last restart).

### `/area mute` / `/area unmute`
Area-wide moderation for CMs and moderators. `/area mute` silences **everyone in the caller's area except CMs and moderators** — both IC and OOC (`ICOOCMuted`) — and persists by IPID exactly like `/mute`, so the mute survives a reconnect until it is lifted. `/area unmute` **reverses it so people can talk again**. The caller, area CMs, CM-permission holders, and moderators are all exempt.

The two commands are a clean inverse pair that never disturbs separate individual mutes: `/area mute` only silences players who are currently unmuted (it won't clobber a deliberate `/mute`), and `/area unmute` only lifts the `ICOOCMuted` state that `/area mute` set (an IC-only or OOC-only individual mute is left intact).

Gated on the `CM` permission (which every mod role carries, and which `clientCanUseCommand` also grants to area CMs), so both CMs and moderators can run it. Registered as the `area` command with `mute`/`unmute` sub-commands (`internal/athena/commands_area_mute.go`); listed under the `area` help category (`/help area`, `/area -h`).

### `/8ball <question>`
For every player. Picks an answer from `config/8ball.txt` if present, otherwise from the built-in 20 classic Magic 8-Ball responses. The sample shipped in `config_sample/8ball.txt` adds a few cheeky extras.

### `/resetusername <new-username>`
Lets a logged-in player rename their account without losing their playtime, chips, wardrobe, tags, or anything else tied to their account. Capped at **3 renames per account** (DB column `USERS.USERNAME_RESETS`, migration 19).

### `/removerole <username>` (Admin)
Demotes a user to a default player account **without deleting it**. Sits between `/setrole` (which requires a named role from `roles.toml`) and `/rmusr` (which purges the account entirely): `/removerole` zeroes the account's permissions via `db.ChangePermissions(username, 0)`, so a DJ or moderator becomes a plain registered player while keeping their username, password, linked IPID, chips, playtime, wardrobe and tags. Any connected session signed in to that account is updated live — it stays logged in but drops to zero permissions, and a former moderator is sent `AUTH#-1` to clear the AO2 "logged in as moderator" badge. Requires `ADMIN`. No-ops with a clear message if the target is already a default (zero-permission) player account.

### `/playtime` Pagination
`/playtime top` lists 25 entries per page; pass a page number for the next 25 (`/playtime top 2` = positions 26–50). Both player accounts and moderator accounts (including shadow mods) are eligible — there's no permission filter.

Admins can adjust an account's stored playtime directly: `/playtime add <username> <duration>` grants time, `/playtime remove <username> <duration>` deducts it (clamped at 0), and `/playtime set <username> <duration>` overwrites the total outright. Duration accepts any combination of `ns`/`us`/`ms`/`s`/`m`/`h`/`d`/`w` units (e.g. `1000h`, `3d12h30m`). The target account must have logged in at least once (so its IPID is linked); the change applies immediately to the leaderboard, and the player is notified live if online.

`/reloadplaytime` (admin) re-links every registered account to its IPID and merges any orphaned playtime — fixes the bug where an account created on a long-running anonymous IPID didn't show pre-existing hours on the leaderboard until the server restarted.

### `/profile` DJ Insignia
Players with the `DJ` permission bit (and no moderator privileges) get a 💿 vinyl badge on their `/profile` card so DJs are visible at a glance. Mods are unaffected — they have their own staff lines.

### `/global` Tag Display
`/global` now shows the sender's `[tag]` in the prefix, matching local-OOC formatting. `/g` is a plain alias of `/global` (same permissions, same handler) for players who want a shorter command to type.

### `/status lfp` Shorthand
`/status` (CM) sets the current area's AO2 status (`idle`, `looking-for-players`, `casing`, `recess`, `rp`, `gaming`). `lfp` is accepted as a shorthand for `looking-for-players` — `/status lfp` and `/status looking-for-players` set the exact same status.

### `/gas` Empty-Area Suppression
`/gas` (the all-areas player listing) hides empty areas to keep the listing scannable on servers with many areas. The empty-area count is shown at the bottom.

### `/randomchar` DJ/Mod Cooldown Bypass
`/randomchar` keeps its 5-second cooldown for regular users, but **DJs and moderators bypass it entirely** so they can swap freely while running events.

### `/rps` (Player vs Player)
`/rps <rock|paper|scissors>` is now PvP. The first call commits a **hidden** choice and posts an open challenge to the area; the second player commits blind (so they can't game-theory the first move) and the result is resolved. 30-second window per player.

### Locked-Room Kick Lockout (Bug Fix)
Previously, `/lock` added every current occupant to the area's invite list, so a CM kicking a player from the locked area didn't actually keep them out — they'd walk right back in. The kick command now also pulls the target's UID from the invite list, so they can't return until the room is unlocked or they're explicitly re-invited.

### `/invite` in Spectate Mode + Public `/spectate` Help
- **`/invite <uid>` now works in spectate mode.** Previously `/invite` bailed with "This area is unlocked." whenever the area wasn't `/lock`-ed, which trapped CMs who had only enabled **spectate mode** (which leaves the area `LockFree`) and wanted to let someone speak. `cmdInvite` now invites to *enter* in a locked area **and** to *speak* (the spectate-invite list, same as `/spectate invite <uid>`) when spectate mode is on; both apply if both are active. In a genuinely unrestricted area the message now tells the CM how to restrict it (`/lock` for entry, `/spectate` for speaking) instead of failing silently.
- **`/spectate` is now listed in `/help` for everyone.** A new `publicHelp` flag on the `Command` struct lets a command appear in `/help` (and show usage via `/help <cmd>`) regardless of the viewer's permissions. `/spectate` sets it so all players can discover how spectate mode works, even though running it still requires `CM`. The flag is honored at all three `/help` filter sites (category overview, category drill-down, single-command lookup).

### Recipe Step Variety
`/recipe` rotates through Step 1 → Step 4 with **separate verb pools per step** (prep / combine / cook / plate). A stream of `/recipe` lines now reads like a real recipe instead of the same template.

### Server-Relayed Voice Chat
Opt-in voice chat. Every audio frame travels **client → Athena → other clients in the same area** — there is no peer-to-peer path, so peers cannot learn each other's IPs via packet sniffing (which is what TURN was previously used for in the now-removed WebRTC mode). Athena itself sees every peer's IP, exactly as it does for chat packets.

Audio: 20 ms Opus frames at 48 kHz, base64-encoded on the wire. The server treats each frame as an opaque blob and just re-broadcasts it (one fan-out per joined peer). No Opus decode happens server-side, so the relay is codec-agnostic and CGO-free.

The `pktVSFrame` voice-effects hook (between the rate-limit check and the broadcast call) is now used by the **voice-chat punishments** — `/voicemute`, `/voicestatic`, `/voicegarble`, `/voicecutout`, `/voicestutter` — in `internal/athena/voice_punishments.go`. They drop, gate, or stale-repeat a punished speaker's frames; because the relay never decodes audio, they manipulate frame *flow* only. Codec-level DSP such as pitch-shifting would need an Opus decoder and CGO and stays out of scope. See **Voice Chat (5)** under the punishment list above.

Protocol (`internal/athena/voice.go`):
- `VS_CAPS#<enabled>#<ptt>#<max_peers>#<codec>#<sample_rate>#<frame_ms>#<max_frame_bytes>#%` — S→C handshake
- `VS_JOIN` / `VS_LEAVE` / `VS_PEERS` — room membership
- `VS_FRAME` (C→S) / `VS_AUDIO#<from_uid>#<b64>#%` (S→C) — audio relay
- `VS_SPEAK#<uid>#<on_off>#%` — speaking indicator

Moderation: IPID-scoped `/vmute` and `/vban`, per-area `/voicearea on|off`, per-UID join and frame rate limits, new-IPID cooldown — all in `internal/athena/voice_mod.go` and `voice_commands.go`. Disabled by default; toggle with `enable_voice = true` under `[Voice]`.

### Streaming Music URLs
Both the `/play <url>` command and a client-sent **MC** packet carrying a raw `http(s)://` URL (e.g. WebAO typing a custom track) play streaming audio. The URL's host must be in `config/cdns.txt`; an un-whitelisted host is rejected with an OOC **"Illegal origin"** notice rather than being silently dropped. The URL is re-broadcast byte-for-byte — the server never mangles it.

The outgoing `MC` packet's numeric fields (`looping`/`channel`/`effects`) always default to `"0"` at the single serialization point (`packet.MCToClient.Args`). This fixes the regression where `/play` and `/randomsong` emitted an empty effects field (`…##%`), which AO2 clients fail to parse as a number and drop silently.

### MS JSON-Schema Validation
JSON-mode connections have their **MS** (in-character) packets validated against draft-07 JSON schemas vendored in the top-level `schemas/` folder (from [OmniTroid/aolib-schemas](https://github.com/OmniTroid/aolib-schemas)):
- **Inbound** MS is validated against `MSRequest.schema.json` in `packet.ParseJSON`; an invalid packet is rejected and dropped (logged).
- **Outbound** MS is validated against `MSBroadcast.schema.json` in `Client.SendPacket`; a packet that fails the schema is dropped (logged) before reaching a JSON-mode client.

To satisfy the schemas, the JSON-mode MS encoder emits proper types (numbers, booleans, and `{x,y}` offset objects) rather than strings. Schemas are embedded via `//go:embed` in `athena.go` and compiled once at startup (`packet.CompileMSSchemas`). FantaCode (classic desktop AO2) traffic is never validated and is unaffected; if the schemas fail to load, validation is silently disabled. Library: `github.com/santhosh-tekuri/jsonschema/v5`.

### `/punishments` Inspection & `/clients` Multiclient Listing
- `/punishments [uid]` — lists active punishments with remaining durations, custom data, reasons, and (for mod viewers) issuer tiers. Covers the out-of-slice effects too: lag (torment list), mute, jail. No permission needed for self-inspection; the `<uid>` form requires `MUTE`. Implemented in `internal/athena/commands_qol.go`.
- `/clients <uid>` — lists every connection sharing the target's IPID with UID, character, area, OOC name and showname. Requires `MUTE`.

### Self-Service Idle Auto-Disconnect (`/dc` / `/dctime`)
Lets **any** player opt into being automatically disconnected after a chosen stretch of inactivity — the same convenience WebAO's idle timeout offers, but user-controlled. Implemented in `internal/athena/disconnect_timer.go`; no permission required (`general` category).

- **Opt-in:** OFF by default. Nobody is ever auto-disconnected unless they personally enable it.
- **Isolated to the caller:** the per-connection watcher goroutine only ever closes the connection of the client that set it. It sends no packet to anyone else and cannot affect another player. (Both clarifications the requester stressed.)
- **Forgiving:** sending an IC or OOC message resets the countdown (`dcTouchActivity`, hooked in `pktIC`/`pktOOC`), so the timer fires only after genuine AFK inactivity — exactly like the WebAO behaviour it mirrors.
- **Default:** running the command with no number arms a **1-hour** countdown.

| Command | Behaviour |
|---------|-----------|
| `/dctime` (or `/dc`) | Enable with the default 60-minute idle window. |
| `/dctime <minutes>` | Disconnect after `<minutes>` of inactivity (capped at 7 days so the deadline arithmetic can't overflow). |
| `/dctime off` | Cancel the timer. Also accepts `0`, `stop`, `cancel`, `disable`, `none`. |
| `/dctime status` | Show the current setting without changing it. |

`/dc` is a plain alias of `/dctime`. A single watcher goroutine is spawned lazily on first enable (CAS-gated) and lives for the rest of the connection, no-opping while disabled and exiting on `client.done`, so re-enabling never respawns it and there is no start/stop race. The watcher re-checks every 10 s, so the disconnect lands within ~10 s of the deadline — plenty precise for an AFK timer. Documented in `/help` via the command's registry `desc`/`usage`.

### Other Features
- Hot Potato area minigame
- Quick Draw area minigame
- Chip Giveaway system
- Area Roulette
- Wardrobe/character management commands
- `/randomchar`, `/possess`
- In-place server restart via `syscall.Exec`
- Testimony recorder (inherited from upstream Athena)
- `/about` credits SyntaxNyah's fork and full credit to MangosArentLiterature's upstream Athena.

## Testing

```bash
go test -v ./...
```

Tests cover: punishment stacking, coinflip logic, rate limiting (with race detector), persistent pairing, per-area logging benchmarks, giveaway, hot potato, quick draw, and roulette. All tests pass with `-race`.

## License

GNU Affero General Public License v3.0 (inherited from upstream Athena).
