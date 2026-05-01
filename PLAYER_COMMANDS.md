# Nyathena — Player Commands Guide

Every command in here is available to regular players (no moderator privileges required). Cooldowns and exceptions are noted inline.

For moderator-only commands, see [`MOD_COMMANDS.md`](MOD_COMMANDS.md). For casino games, see [`CASINO_COMMANDS.md`](CASINO_COMMANDS.md). For the social-deduction Mafia minigame, see [`MAFIA_COMMANDS.md`](MAFIA_COMMANDS.md).

---

## Quick Reference

```
/help                    Browse command categories
/help <category>         List commands in a category (e.g. /help general)
/help <command>          Show usage for a specific command
/about                   Server version + fork credits
```

---

## General / Movement

| Command | Description |
|---------|-------------|
| `/area <name>` | Move to a named area |
| `/areas` | List all areas |
| `/areainfo` | Show settings for the current area |
| `/areadesc` | Show this area's entry description |
| `/ga` | List players in your current area |
| `/gas` | List players in **all** areas (empty areas are hidden) |
| `/players` | Same as /ga |
| `/find <name>` | Find which area a player is in |
| `/pos [pos]` | Show or set your IC position (def, pro, wit, jud, hld, hlp) |
| `/charselect` | Return to character select |
| `/randomchar` | Switch to a random free character (5s cooldown — DJs and mods bypass it) |
| `/dance` | Toggle dance mode (sprite flips on every IC message) |

---

## Chat

| Command | Description |
|---------|-------------|
| `/global <message>` | Send a server-wide OOC message. Shows your `[tag]` like local OOC. |
| `/pm <uid> <message>` | Private message a specific player |
| `/erp` | Toggle the area's ERP mode (if allowed) |
| `/8ball <question>` | Ask the Magic 8-Ball. Answers come from `8ball.txt` or a built-in classic list. |
| `/getmusic` | Show the URL of the song playing in this area and re-send the MC packet to just you (handy when your client's audio bugged out). |

---

## Pairing

| Command | Description |
|---------|-------------|
| `/pair <uid>` | Request to pair with a player. Mutual `/pair` finalizes the pairing. Messages reference each player's **showname** (in-character name) when set. |
| `/unpair` | Cancel your pair. Full bidirectional reset — clears state on every peer that referenced you, so no desyncs. |

---

## Accounts (when accounts/casino are enabled)

| Command | Description |
|---------|-------------|
| `/register <username> <password>` | Create a free player account. Captcha confirmation required by default. |
| `/captcha <token>` | Confirm a pending registration |
| `/login <username> <password>` | Sign in to your account |
| `/logout` | Sign out |
| `/account` | View your account info |
| `/profile [uid]` | Show a profile card. DJs get a 💿 vinyl badge. |
| `/playtime` | Show the playtime leaderboard (page 1, 25 entries) |
| `/playtime top <page>` | Browse subsequent pages of 25 |
| `/resetusername <new>` | Rename your account (keeps playtime/chips/wardrobe). Capped at **3 renames per account**. |

---

## Mini-games

| Command | Description |
|---------|-------------|
| `/rps <rock\|paper\|scissors>` | **PvP** rock-paper-scissors. The first call posts an open challenge with a hidden choice; the second player commits blind and the result is announced. 30s window per player. |
| `/coinflip <heads\|tails>` | Area-scoped 30-second PvP coinflip — opposite sides only |
| `/roll <n>d<m>` | Roll dice (e.g. `/roll 2d6`) |
| `/maso` | Apply a random punishment to yourself for 10 minutes (re-roll allowed) |
| `/megamaso` | Like `/maso` but **stacking**: each repeat adds another random punishment to the pile instead of replacing it. |

---

## Potions (self-applied 5-min effects)

```
/potions             # menu
/potion <name>       # drink one
/potion off          # flush every active potion
```

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
| `omnidere` | Each line picks a random anime dere flavour |
| `character` | Auto-rotates your character every 30 seconds |

---

## Tournament (opt-in)

| Command | Description |
|---------|-------------|
| `/join-tournament` | Join an active punishment tournament. You'll get 2–3 random punishments; whoever sends the most IC messages wins. |

---

## Custom Tags (cosmetic, requires account)

| Command | Description |
|---------|-------------|
| `/shop` | Browse shop tags |
| `/buytag <id>` | Buy a tag (chips required if casino enabled) |
| `/settag <id>` | Equip a tag — shows `[Tag Name]` in `/gas`, `/players`, OOC, `/global` |
| `/cleartag` | Remove your equipped tag |
| `/listcustomtags` | List every admin-minted custom tag |

---

## Notes

- **Permission gating**: Anything in this guide is callable by every connected player. Moderator-only commands (mute, kick, ban, the punishment commands, `/firewall`, `/lockdown`, etc.) are documented in `MOD_COMMANDS.md`.
- **Cooldowns**: Many commands have small per-user cooldowns. The 5s `/randomchar` cooldown is bypassed for DJs and moderators.
- **Tags in OOC**: Local OOC and `/global` both show your equipped tag in front of your name as `[Tag] Name`.
