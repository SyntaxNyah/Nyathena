# Punishment Commands Documentation

Nyathena includes 90+ punishment commands for moderators. These commands temporarily modify how player messages appear or behave, stack freely, auto-expire, and persist across area changes.

## Common Flags

Every punishment command that targets UIDs supports these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-d <duration>` | `10m` | How long the effect lasts (max 24h). Formats: `30s`, `5m`, `2h`, `1h30m`. |
| `-r <reason>` | (none) | Optional reason recorded in the log. |
| `-h` | (off) | **Hidden mode** — suppresses the per-target OOC notification so the punishment applies silently. The issuer's summary appends `(hidden)` so they can confirm. |

## Targeting

All commands accept:
- **Single UID:** `/tsundere 7`
- **Comma-separated UIDs:** `/tsundere 7,12,15`
- **`global` keyword:** `/tsundere global` — applies to every non-moderator in your current area.

```
/<command> [-d duration] [-r reason] [-h] global | <uid1>,<uid2>,...
```

**Examples:**
```
/uppercase -d 5m -r "shouting contest" 123
/uwu -d 1h 45,67,89
/tsundere global -h              # Silently tsundere the whole area
/backward -d 30m -r "backwards day" 12
```

## Removing Punishments

### `/unpunish`

The universal removal command. Clears **all** punishment types including lag.

```
/unpunish <uid1>,<uid2>,...              # Remove ALL punishments from target(s)
/unpunish -t <type> <uid1>,<uid2>,...    # Remove a specific punishment type
/unpunish all                            # Remove all punishments from every player in your area
```

`/unpunish all` also clears `/lag` (the torment list). `/unpunish -t lag <uid>` removes lag from a specific target.

**Examples:**
```
/unpunish 123                    # Remove everything from UID 123
/unpunish -t uppercase 45        # Remove only uppercase
/unpunish -t lag 7               # Remove lag from UID 7
/unpunish all                    # Wipe the entire area clean
```

### Self-Removal Protection

A regular moderator **cannot** `/unpunish` a punishment that an admin or shadow mod placed on them. All three forms are gated:

- `/unpunish <own-uid>` — refused if any punishment was issued by shadow/admin.
- `/unpunish -t <type> <own-uid>` — refused for that specific effect.
- `/unpunish all` — silently skips the caller's protected punishments.

Admins and shadow mods bypass the gate. The issuer tier persists across restarts via `PUNISHMENTS.ISSUER_TIER` (DB migration 18).

### Per-Effect Removal Commands

Some punishments have dedicated removal commands as a convenience:

| Remove | Lifts |
|--------|-------|
| `/unlag <uid>` | `/lag` |
| `/unsfx <uid>` | `/sfxcurse` |
| `/unshrink <uid>` | `/shrink` |
| `/ungrow <uid>` | `/grow` |
| `/unwide <uid>` | `/wide` |
| `/unicwarp <uid>` | `/icwarp` |
| `/unlovebomb <uid>` | `/lovebomb` |
| `/undegrade <uid>` | `/degrade` |
| `/unslang <uid>` | `/slang` |
| `/unthesaurusoverload <uid>` | `/thesaurusoverload` |
| `/unvalleygirl <uid>` | `/valleygirl` |
| `/unbabytalk <uid>` | `/babytalk` |
| `/unthirdperson <uid>` | `/thirdperson` |
| `/ununreliablenarrator <uid>` | `/unreliablenarrator` |
| `/ununcannyvalley <uid>` | `/uncannyvalley` |
| `/un51 <uid>` | `/51` |
| `/unphilosopher <uid>` | `/philosopher` |
| `/unpoet <uid>` | `/poet` |
| `/unupsidedown <uid>` | `/upsidedown` |
| `/unsarcasm <uid>` | `/sarcasm` |
| `/unacademic <uid>` | `/academic` |
| `/unrecipe <uid>` | `/recipe` |
| `/unquote <uid>` | `/quote` |
| `/untranslator curse <uid>` | `/translator` |

All of these can also be removed with `/unpunish -t <type> <uid>` or `/unpunish <uid>` (removes all).

---

## Text Effects (46)

Rewrite the target's IC text — light to heavy transformations. All support `global` and `-h`.

### Basic Transformations
| Command | Effect |
|---------|--------|
| `/whisper` | Messages only visible to mods and CMs |
| `/backward` | Reverses character order (`Hello` → `olleH`) |
| `/stutterstep` | Doubles every word (`Hello world` → `Hello Hello world world`) |
| `/elongate` | Repeats vowels (`Hello` → `Heeeelllooo`) |
| `/uppercase` | Forces UPPERCASE |
| `/lowercase` | Forces lowercase |
| `/robotic` | Replaces words with `[BEEP] [BOOP]` |
| `/alternating` | Creates `AlTeRnAtInG cAsE` |
| `/fancy` | Unicode fancy characters (`Hello` → `𝐇𝐞𝐥𝐥𝐨`) |
| `/uwu` | UwU speak (`Hello world` → `Hewwo worwd uwu`) |
| `/pirate` | Pirate speech (`ahoy me friend, arr!`) |
| `/shakespearean` | Shakespearean English (`Hark! Where art thou`) |
| `/caveman` | Caveman grunts (`UGH GRUNT`) |
| `/censor` | Randomly replaces words with `[CENSORED]` |
| `/fromsoftware` | Censors words from `fromsoft.txt` with asterisks |
| `/confused` | Randomly reorders words |
| `/paranoid` | Adds paranoid text (`they're watching`) |
| `/drunk` | Slurs, repeats, adds `*hic*` |
| `/hiccup` | Interrupts with `*hic*` |
| `/whistle` | Replaces letters with musical whistles (`♪♫~♬♪`) |
| `/mumble` | Obscures text, keeps first/last letters (`H***o w***d`) |
| `/slang` | Internet slang (`I don't know` → `idk`) |
| `/cherri` | Capitalizes Every Word |
| `/albhed` | Al Bhed cipher from FFX |
| `/morse` | Converts to Morse code dots and dashes |
| `/vowelhell` | Replaces every consonant with a random vowel |
| `/upsidedown` | Flips text using Unicode upside-down characters |
| `/autospell` | Intentionally "autocorrects" wrong (`the` → `teh`) |

### Personality Overlays
| Command | Effect |
|---------|--------|
| `/thesaurusoverload` | Comically pompous synonyms and smug parentheticals |
| `/valleygirl` | Valley-girl filler, vowel stretching, dramatic tone |
| `/babytalk` | Toddler phonetics and stage directions (`*tiny stomp*`) |
| `/thirdperson` | Third-person narration with mood tags |
| `/unreliablenarrator` | Hedges, contradictions, self-doubting commentary |
| `/uncannyvalley` | Glitchy system notes, subtly mutated display name |
| `/chef` | Swedish-Chef filter — bork bork bork! |
| `/karen` | Escalating complaints and manager demands |
| `/passiveaggressive` | Chilly, performatively-polite framings. It's fine. Really. |
| `/nervous` | Stuttering, um/uh fillers, trailing apologies |
| `/sarcasm` | Sarcastic parenthetical commentary |
| `/academic` | Overly formal academic language |
| `/philosopher` | Appends deep philosophical questions |
| `/poet` | Lyrical poetic flourishes |
| `/quote` | Wraps messages in quotation marks (50% chance) |
| `/spaghetti` | Combines 2-3 random effects together |
| `/essay` | Requires minimum 50 characters per message |
| `/haiku` | Requires 5-7-5 syllable format |
| `/dreamsequence` | Rewrites as surreal, dreamlike fragments |
| `/timewarp` | Shuffles word order |
| `/rng` | Random effect from pool each message |

---

## Themed Quote Replacers (8)

Discard the player's text and substitute a themed line per message. All support `global` and `-h`.

| Command | Effect |
|---------|--------|
| `/gordonramsay` | Gordon Ramsay kitchen tirades (60+ quotes) |
| `/biblebot` | Random Bible verses |
| `/grounded` | GoAnimate-style "YOU ARE GROUNDED" tirades |
| `/mime` | Silent mime actions (*gestures wordlessly*) |
| `/subtitles` | Confusing subtitle annotations (`[ominous music playing]`) |
| `/spotlight` | Announces all actions publicly (`📣 EVERYONE LOOK:`) |
| `/recipe` | Reformats as cooking recipe steps (4-step verb rotation) |
| `/rickroll` | Meme-styled lyric-adjacent stand-in lines |
| `/pickup` | Catastrophically cheesy pickup lines |
| `/brainrot` | Maximum skibidi sigma brainrot energy |

---

## Persona / Personality (5)

Wraps every line in a persona's prefix/suffix flavour. Supports `global` and `-h`.

| Command | Effect |
|---------|--------|
| `/clown` | Clown honks and circus filler |
| `/jester` | Theatrical jester flourishes and bell-jingle text |
| `/joker` | Chaotic laughter throughout every message. HAHAHA! |
| `/tourettes` | Random outbursts: swearing, objects, nonsense, animal sounds |
| `/translator` | Translates IC messages to another language via DeepL API |

### `/translator` usage

Requires `enable_translator_punishment = true` and `translator_api_key` set in config.

```
/translator curse [-d duration] [-r reason] [-h] <uid> <language>
/translator curse global random        # Per-word random language
/untranslator curse <uid>              # Remove
/untranslator curse global             # Remove from everyone
```

Languages: English names (`french`), ISO codes (`fr`), or `random` for per-word chaos.

---

## Dere Archetypes (26)

Anime-style relationship-trope flavour. All support `global` and `-h`.

| Original 10 | Nyathena Additions (15) | Special |
|-------------|------------------------|---------|
| `/tsundere` `/yandere` `/kuudere` `/dandere` `/deredere` `/himedere` `/kamidere` `/undere` `/bakadere` `/mayadere` | `/smugdere` `/deretsun` `/bokodere` `/thugdere` `/teasedere` `/dorodere` `/hinedere` `/hajidere` `/rindere` `/utsudere` `/darudere` `/butsudere` `/sdere` `/mdere` `/tsuyodere` | `/omnidere` — picks a random dere flavour for **every** IC message |

---

## Animal Filters (12)

Replace text with animal sounds. All support `global` and `-h`.

| Command | Sound |
|---------|-------|
| `/monkey` | ook, eek, ooh ooh |
| `/snake` | hissss, ssssnake |
| `/dog` | woof, arf, grr, bork |
| `/cat` | meow, purrr~, mrrrow |
| `/bird` | tweet, chirp, squawk |
| `/cow` | moo, mooo, MOOO |
| `/frog` | ribbit, croak |
| `/duck` | quack, QUACK |
| `/horse` | neigh, whinny, snort |
| `/lion` | ROAR, grrr, rawr |
| `/bunny` | *thump*, *binky!*, *flops* |
| `/zoo` | Random animal sound per message |

---

## Visibility / Cosmetic (8)

| Command | Effect |
|---------|--------|
| `/emoji` | Replaces player's name with random emojis each message |
| `/invisible` | Prevents player from seeing other players' messages |
| `/shrink [offset]` | Locks vertical sprite offset negative (default -25) |
| `/grow [offset]` | Locks vertical sprite offset positive (default +25) |
| `/wide [offset]` | Locks horizontal sprite offset (default +50) |
| `/areainiswap <char>` | Forces everyone in area to display as a chosen character (KICK perm) |
| `/hidedisplay <uid>` | Pushes the target's own sprite off-screen via a fixed self-offset; their text still shows. Funny on pairs (only the partner renders) |
| `/forcedisplay <uid>` | Pins the target's character onto every IC sprite in the area (moderators exempt). While active, no other character can show in the viewport — the whole room renders as that one character |

Remove offset locks with `/unshrink`, `/ungrow`, `/unwide`, or `/areainiswap off`. Remove the display punishments with `/unpunish -t hidedisplay <uid>` or `/unpunish -t forcedisplay <uid>` (or plain `/unpunish <uid>` to clear everything).

---

## Timing & Throughput (3)

| Command | Effect |
|---------|--------|
| `/slowpoke` | Delays messages before sending |
| `/fastspammer` | Heavily rate limits messages |
| `/lag` | Adds IPID to torment list (ghost/delayed messages, silent disconnect timer) |

**`/lag` note:** This is IPID-scoped (affects all sessions from the same IP). Remove with `/unlag <uid>`, `/unpunish -t lag <uid>`, or `/unpunish <uid>`.

---

## Audio / SFX (2)

| Command | Description |
|---------|-------------|
| `/sfxcurse <uid> <sfx-url>` | Forces an SFX file on every IC message. Accepts http(s) URLs or `/base/sounds/` paths. |
| `/unsfx <uid>` | Lifts the SFX curse |

```
/sfxcurse 12 https://example.com/boom.opus
/sfxcurse 12 /base/sounds/general/meow.opus
/sfxcurse global https://example.com/honk.opus    # SFX curse the whole area
```

External URLs must be on a whitelisted CDN (see `config/cdns.txt`).

---

## Voice Chat Punishments (5)

Sabotage a player's server-relayed voice-chat audio. Requires `enable_voice = true` and `MUTE` permission. All support `-d`, `-r`, `-h`, comma-separated UIDs, `global`, `/stack`, and `/unpunish`.

| Command | Effect |
|---------|--------|
| `/voicemute` | Drops every frame — silent to the room |
| `/voicestatic` | Drops ~60% of frames — choppy, breaking up |
| `/voicegarble` | Drops ~88% of frames — barely intelligible |
| `/voicecutout` | Gates frames on ~650ms on/off cycle — walkie-talkie |
| `/voicestutter` | Randomly replays stale frames — glitchy stutter |

These manipulate frame *flow* only (no Opus decode), so they're CGO-free.

---

## Stacking / Chaos (11)

| Command | Description |
|---------|-------------|
| `/stack <type1> <type2> [...] <uid\|global>` | Apply multiple effects simultaneously. Supports `global` and `-h`. |
| `/torment` | Cycles through different effects each message |
| `/lovebomb` | Replaces IC messages with silly love declarations. Supports `global` and `-h`. |
| `/degrade` | Replaces IC messages with degrading self-insults |
| `/emoticon` | Forces speech in emoticons only (:P, :D, :3) |
| `/51` | Replaces each message with a random line from the 51-messages story |
| `/icwarp` | Replaces IC messages with random past messages from the same area |
| `/megamaso` | Self-applied stacking chaos (any player, no permissions required) |
| `/maso` | Self-applied single random punishment (any player) |
| `/randompunishall` | Random punishment on every player in the area. `-h` also suppresses the area announcement. |
| `/togglerandompunish` | Enable/disable `/randompunishall` for this area (CM perm) |
| `/tournament start\|stop\|status` | Punishment tournament mode |

### `/stack` example
```
/stack backward uwu pirate -d 15m -r "triple chaos" 7
/stack tsundere yandere global -h          # Silent stack on entire area
```

### `/icwarp` usage
```
/icwarp 42                    # Per-user warp
/icwarp global on             # Area-wide warp (you're exempt)
/icwarp global off            # Turn off area-wide warp
/unicwarp 42                  # Remove per-user warp
```

### Self-Applied Commands (no permissions required)

**`/maso`** — Rolls a random punishment. Typing again rerolls. Default 10 min.
```
/maso              # Random punishment for 10 min
/maso -d 30m       # 30 minutes
```

**`/megamaso`** — Each call ADDS another random punishment to the stack.
```
/megamaso           # Stack a random effect (10 min)
/megamaso -d 1h     # Each layer lasts 1 hour
/megamaso           # Keep stacking
```

---

## Self-Chaos Block (2)

| Command | Description |
|---------|-------------|
| `/blockpunishment <uid>` | Prevent a player from using `/maso`, `/megamaso`, `/potion`, `/coinflip` |
| `/unblockpunishment <uid>` | Restore access |

---

## Safety Features

- **Maximum duration:** 24 hours (auto-capped)
- **Auto-expiry:** Punishments remove automatically when time expires
- **Thread-safe:** Proper mutex locking
- **Text length limit:** 2000 characters maximum
- **Logging:** All punishment actions logged with moderator name and reason
- **Persist across area changes** — punishments follow the player
- **DB persistence** — punishments survive server restarts (except `/icwarp`)

## Technical Notes

### Duration Format
`30s` (seconds), `5m` (minutes), `2h` (hours), `1h30m` (combined).

### Permission Required
Most punishment commands require `MUTE` permission. Exceptions:
- `/maso`, `/megamaso` — no permission (self-applied)
- `/areainiswap` — requires `KICK`
- `/togglerandompunish` — requires `CM`

### Multiple Punishments
A player can have multiple punishment types active simultaneously. Text modifications apply in the order they were added.
