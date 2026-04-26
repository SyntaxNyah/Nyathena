# Nyathena

## Project Overview

Nyathena is a fork of [Athena](https://github.com/MangosArentLiterature/Athena), a lightweight AO2 (Attorney Online 2) server written in Go. It extends upstream Athena with a large set of original features:

- A full **Discord bot** integration (slash commands, embeds, moderation bridge)
- A **casino system** with 10 distinct games and persistent virtual currency ("Nyathena Chips")
- A **Mafia social-deduction minigame** playable inside any server area
- **42+ punishment commands** for moderators, with stacking, tournaments, and a coinflip challenge system
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
| `automod_action` | `"ban"` | AutoMod action: `ban`, `kick`, `mute`, or `torment` |
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

### Punishment System (41 Commands)

All punishment commands require `MUTE` permission, support `-d <duration>` (max 24 h) and `-r <reason>`, accept comma-separated UIDs, and auto-expire. Multiple types stack on a single player.

**Remove:** `/unpunish <uid>` (all) or `/unpunish -t <type> <uid>` (specific)

#### Text Modification (14)
`/whisper`, `/backward`, `/stutterstep`, `/elongate`, `/uppercase`, `/lowercase`, `/robotic`, `/alternating`, `/fancy`, `/uwu`, `/pirate`, `/shakespearean`, `/caveman`, `/slang`

#### Visibility / Cosmetic (2)
`/emoji`, `/invisible`

#### Timing Effects (4)
`/slowpoke`, `/fastspammer`, `/pause`, `/lag`

#### Social Chaos (4)
`/subtitles`, `/tourettes`, `/roulette`, `/spotlight`

#### Text Processing (7)
`/censor`, `/confused`, `/paranoid`, `/drunk`, `/hiccup`, `/whistle`, `/mumble`

#### Complex Effects (4)
`/spaghetti`, `/torment`, `/rng`, `/essay`

#### Advanced (2)
`/haiku`, `/autospell`

#### Fun Personality (6)
`/thesaurusoverload`, `/valleygirl`, `/babytalk`, `/thirdperson`, `/unreliablenarrator`, `/uncannyvalley`

#### Themed Quote (1)
`/gordonramsay` — replaces every IC line with a Gordon Ramsay kitchen tirade (60+ quotes). Requires `MUTE`.

#### Punishment Stacking
```
/stack <type1> <type2> [...] [-d duration] [-r reason] <uid1>,<uid2>,...
```

#### Punishment Tournament Mode
Voluntary competitive mode where participants receive 2–3 random punishments; most IC messages sent wins.
```
/tournament start|status|stop   # requires MUTE
/join-tournament                # any user
```

#### Coinflip Challenge
Area-scoped 30-second PvP challenge. Players must choose opposite sides.
```
/coinflip <heads|tails>
```

### Persistent Pairing
UID-based mutual pairing surviving area/character changes. Dissolves on disconnect.
```
/pair <uid>
/unpair
```

### Character Curse
One-time forced character swap (requires KICK). Target may freely change afterwards.
```
/charcurse <uid> <charname>
```

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
Word-list-based automatic enforcement. Actions: permanent ban (silent), kick, mute, or torment (random disconnects every 30–60 s). Covers IC message text, IC showname, OOC message text, and OOC username — slurs in any of those fields trigger the configured action.

### Doki Area Effect
Per-area chaos mode for literature-club-themed areas. Enable with `doki_area = true` on the area entry in `areas.toml`. Each IC message rolls independently:
- 1/300 — replace text with a Haschen-themed Monika-style quote
- 1/200 — replace text with a dark Haschen anagram (with hint)
- 1/150 — replace text with a zalgo-corrupted Haschen catchphrase
- 1/100 — zalgo-scramble the player's actual text
- 1/250 — surprise-swap the area background to a random one

Stacks freely with `mirror_area` and `punishment_area` on the same area.

### Tiered `/randomsong`
`/randomsong` plays a random track from `music.txt`. Cooldown is tiered:
- regular users — `random_song_cooldown` (default 20 s)
- DJ permission — `random_song_cooldown_dj` (default 5 s)
- moderators — `random_song_cooldown_mod` (default 0 s, unlimited)

### Shadow Mod Visibility
Shadow moderators (`SHADOW` perm bit, no `ADMIN`) are completely hidden from `/gas`/`/players` for non-admin viewers — no "Mod:" line is shown at all. Only admins see them, labelled as `Mod: <name> (shadow)`. Previously the line still rendered as `Mod: Moderator`, which let other moderators infer staff status.

### IPHub VPN Firewall
When `iphub_api_key` is set, moderators can run `/firewall on|off`. New IPs are checked against IPHub; VPN/proxy IPs are rejected. Known IPs are never re-checked (respects 1,000 requests/day free tier).

### Other Features
- Hot Potato area minigame
- Quick Draw area minigame
- Chip Giveaway system
- Area Roulette
- Wardrobe/character management commands
- `/randomchar`, `/possess`
- In-place server restart via `syscall.Exec`
- Testimony recorder (inherited from upstream Athena)

## Testing

```bash
go test -v ./...
```

Tests cover: punishment stacking, coinflip logic, rate limiting (with race detector), persistent pairing, per-area logging benchmarks, giveaway, hot potato, quick draw, and roulette. All tests pass with `-race`.

## License

GNU Affero General Public License v3.0 (inherited from upstream Athena).
