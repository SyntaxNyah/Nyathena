![Athena logo](resource/logo.png)

# Nyathena

**Nyathena** is a fork of [Athena](https://github.com/MangosArentLiterature/Athena) — a lightweight AO2 (Attorney Online 2) server written in Go — maintained by **SyntaxNyah**. Full credit for the original server goes to [MangosArentLiterature](https://github.com/MangosArentLiterature).

Nyathena extends the upstream with a large suite of original features — the centrepiece being a **90+ command punishment system** built specifically to give moderators an embarrassing and ridiculous amount of creative control over players. Backwards text, dere archetypes, voice corruption, quote replacers, coinflip PvP challenges, forced haiku, tournament chaos — it's purposefully excessive. On top of that: a Discord bot, a casino with 10 games, a Mafia minigame, server-relayed voice chat, and much more — all while retaining the original's concurrency model, TOML configuration, and lightweight footprint.

---

## What's Different From Upstream Athena

### Discord Bot Integration
A full Discord bot (`discordgo`) with slash commands, mod-bridge embeds, area/player listings, and security toggles — all manageable from Discord without being logged into the AO server.

| Discord slash | Effect |
|---------------|--------|
| `/firewall on\|off` | Toggle IPHub VPN/proxy firewall |
| `/lockdown on\|off` | Toggle server-wide new-IPID lockdown |
| `/lockdown whitelist_all` | Whitelist all connected IPIDs for lockdown |

### Casino System (`enable_casino = true`)
Virtual currency (**Nyathena Chips**) backed by SQLite. New players start with 500 chips (cap 10,000,000).

| Game | Command |
|------|---------|
| Blackjack (6-deck, split/double/insurance) | `/bj` |
| Texas Hold'em (up to 9 players) | `/poker` |
| Slots with area jackpot pool | `/slots` |
| European Roulette | `/croulette` |
| Baccarat | `/baccarat` |
| Craps | `/craps` |
| Crash multiplier | `/crash` |
| Minesweeper grid | `/mines` |
| Keno | `/keno` |
| Prize wheel | `/wheel` |
| Bar (33 drinks) | `/bar` |

Economy commands: `/chips`, `/chips top`, `/chips give`, `/richest`. Earn without gambling via `/busker`, `/janitor`, `/paperboy`, `/clerk`, `/bailiffjob`. Unscramble events every 30 min–3 h.

Shop (`/shop`): 30 cosmetic tags, job cooldown passes, reward bonus passes.

### Mafia Social Deduction Minigame
Full in-server Mafia (4–20+ players) with day/night phases, lynch voting, night actions, last wills, graveyard, and whisper.

**Roles:** Villager, Detective, Doctor, Sheriff, Bodyguard, Vigilante, Mayor, Escort, Mafia, Shapeshifter, Godfather, Jester, Witch, Lawyer, Arsonist, Serial Killer, Survivor.

See `MAFIA_COMMANDS.md` for the full reference.

### Punishment System (90+ Commands)
All punishment commands support `-d <duration>`, `-r <reason>`, and `-h` (hidden — suppresses the per-target OOC notification). They accept comma-separated UIDs and auto-expire.

**Categories:** text modification (15), visibility/cosmetic (5), timing effects (4), social chaos (4), text processing (7), complex effects (4), advanced (2), personality archetypes (10), dere archetypes (25), themed quote replacers (2), audio (2), voice chat (5).

**Dere archetypes (25):** `/tsundere`, `/yandere`, `/kuudere`, `/dandere`, `/deredere`, `/himedere`, `/kamidere`, `/undere`, `/bakadere`, `/mayadere`, `/smugdere`, `/deretsun`, `/bokodere`, `/thugdere`, `/teasedere`, `/dorodere`, `/hinedere`, `/hajidere`, `/rindere`, `/utsudere`, `/darudere`, `/butsudere`, `/sdere`, `/mdere`, `/tsuyodere`, and `/omnidere` (random dere per message).

**Self-applied:** `/maso` (random single effect, rerolls on repeat), `/megamaso` (stack-on-stack mode).

**PvP:** `/coinflip <heads|tails>` — area-scoped 30-second challenge; players must pick opposite sides.

**Stacking:**
```
/stack <type1> <type2> [...] [-d duration] [-r reason] <uid>
```

**Tournament mode:** `/tournament start|status|stop` — volunteers get 2–3 random punishments; most IC messages sent wins.

**Remove:** `/unpunish <uid>` (all) or `/unpunish -t <type> <uid>`. Admins and shadow mods cannot be unpunished by regular mods.

### Server-Relayed Voice Chat (`enable_voice = true`)
Opt-in voice relay: client → Athena → area peers. No peer-to-peer paths — peers never learn each other's IPs. 20 ms Opus frames at 48 kHz, base64-encoded. Server treats frames as opaque blobs (no decode, CGO-free).

Voice punishment commands (manipulate frame flow, not audio DSP):
- `/voicemute` — drops all frames
- `/voicestatic` — drops ~60% of frames
- `/voicegarble` — drops ~88% of frames
- `/voicecutout` — gates frames on ~650 ms on/off cycle
- `/voicestutter` — randomly replays previous frame

Moderation: `/vmute`, `/vban`, `/voicearea on|off`.

### Potions System
Self-applied fun effects: `/potion <name>` (default 5 min, `-d` to override). `/potions` lists the cabinet; `/potion off` flushes all.

Potions: `drunk`, `uwu`, `shy`, `dramatic`, `pirate`, `poet`, `caveman`, `fancy`, `chef`, `cherri`, `omnidere`, `character` (auto-rotates your sprite every 30 s).

### Persistent Pairing
UID-based mutual pairing that survives area/character changes. Pair messages use the player's showname when set. `/unpair` is a full bidirectional reset.

### Per-Area Logging
Daily-rotating log files under `logs/<AreaName>/`. Format: `[HH:MM:SS] | ACTION | CHARACTER | IPID | HDID | SHOWNAME | OOC_NAME | MESSAGE`. Enable with `enable_area_logging = true`.

### AutoMod
Word-list-based enforcement covering IC text, IC showname, OOC text, and OOC username. Actions: `ban`, `kick`, `mute`, or `torment`.

### IPHub VPN Firewall
When `iphub_api_key` is set, `/firewall on|off` (in-game or Discord) checks new IPs against IPHub. Known IPs are cached and never re-checked (respects the 1,000 req/day free tier).

### Custom Tags (Admin)
Admins mint cosmetic tags at runtime without rebuilding. Custom tags never appear in `/shop`.

```
/createtag <id> <display name>
/grantcustomtag <username> <tag_id>
/revokecustomtag <username> <tag_id>
/deletetag <id>
/listcustomtags
```

### Other Additions
- **Doki Area Effect** — literature-club-themed chaos per area (`doki_area = true`)
- **Shadow Mod Visibility** — shadow mods hidden from `/gas`/`/players` for non-admins
- **`/charshuffle` / `/uncharshuffle`** — Sattolo-permutes everyone's character; originals remembered
- **`/reversename` / `/unreversename`** — flips showname rune order, stacks cleanly with `/forcename`
- **`/charcurse <uid> <charname>`** — one-time forced character swap
- **`/rps`** — PvP rock-paper-scissors with hidden simultaneous commitment
- **`/randomsong`** tiered cooldown (user / DJ / mod)
- **`/randomchar`** DJ/mod cooldown bypass
- **`/getmusic`** — re-sends current music packet to the requester
- **`/8ball <question>`** — customisable response pool via `config/8ball.txt`
- **`/resetusername`** — account rename (capped at 3 per account)
- **`/playtime top`** pagination (25 per page)
- **`/reloadplaytime`** — admin IPID re-link and orphan merge
- **`/profile`** DJ insignia (💿)
- **`/global`** tag display in prefix
- **`/gas`** empty-area suppression
- Locked-room kick lockout bug fix
- Hot Potato, Quick Draw, Chip Giveaway, Area Roulette minigames
- Wardrobe management, `/possess`, in-place server restart via `syscall.Exec`

---

## Inherited Features (Upstream Athena)

- WebAO (plain WebSocket) support
- WebSocket Secure (WSS) via reverse proxy or direct TLS
- Concurrent client handling
- Moderator user system with configurable TOML roles
- Robust command system
- Privacy-oriented logging
- Testimony recorder
- CLI command parser (`mkusr`, etc.)
- bcrypt password storage

---

## Quick Start

```bash
# Build
make build

# Copy sample config
cp -r config_sample config

# Edit config/config.toml, areas.toml, roles.toml, etc.

# Run
./bin/athena

# Create first moderator account
mkusr    # type in the CLI prompt
```

Binaries for common platforms are available on the [releases page](https://github.com/syntaxnyah/nyathena/releases).

Pass `-c /path/to/config` for a custom config directory. Pass `-nocli` to disable stdin.

---

## Configuration

All configuration lives in `config/config.toml`. Key options beyond upstream defaults:

| Key | Purpose |
|-----|---------|
| `enable_casino` | Enable casino and player account system |
| `iphub_api_key` | IPHub key for VPN/proxy firewall |
| `automod_enabled` / `automod_action` | AutoMod word enforcement |
| `enable_voice` | Server-relayed voice chat |
| `enable_area_logging` | Per-area rotating log files |
| `webhook_url` | Discord webhook for modcall notifications |
| `punishment_webhook_url` | Discord webhook for ban/kick embeds |
| `[Discord] bot_token` / `guild_id` | Discord bot credentials |

See `CLAUDE.md` for the full configuration reference.

### WSS Setup

**Via reverse proxy (recommended):**
```toml
enable_webao_secure = true
webao_secure_port   = 443
# leave tls_cert_path / tls_key_path blank
```

**Direct TLS:**
```toml
enable_webao_secure = true
webao_secure_port   = 443
tls_cert_path       = "/path/to/cert.crt"
tls_key_path        = "/path/to/key.key"
```

---

## Building From Source

**Requirements:** Go 1.19+

```bash
make build    # produces bin/athena
make test     # go test -v -race ./...
make all      # build + test
make release  # goreleaser (requires goreleaser)
```

---

## License

GNU Affero General Public License v3.0, inherited from upstream Athena.

---

## Credits

### Upstream — Athena by MangosArentLiterature

Nyathena would not exist without **[Athena](https://github.com/MangosArentLiterature/Athena)** and the exceptional work of [MangosArentLiterature](https://github.com/MangosArentLiterature).

Choosing Go for an AO2 server was a genuinely inspired decision. The language's concurrency model — goroutines, channels, the `sync` primitives — maps almost perfectly onto the "one goroutine per client" model that a game server needs, and the result is a codebase that is fast, readable, and a genuine pleasure to extend. The clean package layout (`athena`, `db`, `packet`, `permissions`, `settings`, …), the TOML-first configuration philosophy, the privacy-conscious logging design, and the no-nonsense CLI — all of it reflects careful thought about what a server should actually be.

Everything in Nyathena is built on top of that foundation. The sqlite layer, the moderator role system, the packet parser, the testimony recorder, the WebSocket stack — every single one of Nyathena's 90+ punishment commands and casino games and Discord integrations hangs off work that MangosArentLiterature designed and wrote first. None of the Nyathena additions would have been nearly as clean or straightforward to implement without the quality of the base they sit on.

If you are running an AO2 server and you do not need the extra chaos, **please use upstream Athena** — it is a more focused, better-tested, and purpose-built project. Nyathena is a heavily opinionated fork that leans into moderator mischief; upstream Athena is the real thing.

Full, sincere credit and gratitude to MangosArentLiterature for building something worth forking.

---

**Nyathena** fork and all additions by [SyntaxNyah](https://github.com/syntaxnyah).
