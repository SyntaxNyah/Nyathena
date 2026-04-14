# Punishment Commands Documentation

Athena now includes 41 fun, non-harmful punishment commands for moderators to use on players. These commands temporarily modify how player messages appear or behave, adding entertainment value while maintaining server control.

## Overview

All punishment commands:
- **Require MUTE permission** (moderator-only)
- Support `-d <duration>` flag (default: 10m, max: 24h)
- Support `-r <reason>` flag for logging
- Accept multiple UIDs: `<uid1>,<uid2>,...`
- Are thread-safe and automatically expire
- Can be removed with `/unpunish`

## Usage Pattern

```
/<command> [-d duration] [-r reason] <uid1>,<uid2>,...
```

**Examples:**
```
/uppercase -d 5m -r "shouting contest" 123
/uwu -d 1h 45,67,89
/backward -d 30m -r "talking backwards day" 12
```

## Removing Punishments

```
/unpunish <uid1>,<uid2>,...              # Remove all punishments
/unpunish -t <type> <uid1>,<uid2>,...    # Remove specific punishment
```

**Examples:**
```
/unpunish 123                    # Remove all punishments from UID 123
/unpunish -t uppercase 45        # Remove only uppercase punishment
```

## Text Modification Commands (14)

### `/whisper`
Makes messages only visible to mods and CMs. Players can still send messages, but only staff can see them.

### `/backward`
Reverses character order in messages.
- Input: "Hello world"
- Output: "dlrow olleH"

### `/stutterstep`
Doubles every word in the message.
- Input: "Hello world"
- Output: "Hello Hello world world"

### `/elongate`
Repeats vowels in messages.
- Input: "Hello"
- Output: "Heeeelllooo"

### `/uppercase`
Forces all messages to UPPERCASE.
- Input: "Hello world"
- Output: "HELLO WORLD"

### `/lowercase`
Forces all messages to lowercase.
- Input: "HELLO WORLD"
- Output: "hello world"

### `/robotic`
Replaces words with robotic sounds.
- Input: "Hello world"
- Output: "[BEEP] [BOOP]"

### `/alternating`
Creates AlTeRnAtInG cAsE.
- Input: "hello"
- Output: "HeLlO"

### `/fancy`
Converts to Unicode fancy characters (mathematical bold).
- Input: "Hello"
- Output: "𝐇𝐞𝐥𝐥𝐨"

### `/uwu`
Converts to UwU speak.
- Input: "Hello world"
- Output: "Hewwo worwd uwu"

### `/pirate`
Converts to pirate speech.
- Input: "Hello my friend"
- Output: "ahoy me friend, arr!"

### `/shakespearean`
Converts to Shakespearean English.
- Input: "Where are you going"
- Output: "Hark! Where art thou going"

### `/caveman`
Converts to caveman grunts.
- Input: "Hello world test"
- Output: "UGH GRUNT"

### `/slang`
Converts messages to internet slang abbreviations.
- Input: "I don't know, got to go"
- Output: "idk, gtg"
- Input: "be right back, talk to you later"
- Output: "brb, ttyl"
- Remove with: `/unslang <uid1>,<uid2>,...`

## Visibility/Cosmetic Commands (2)

### `/emoji`
Replaces player's name with random emoji each message.
- Effect: Name appears as 😀, 🎃, 🦄, etc.

### `/invisible`
Prevents player from seeing other players' messages (isolation punishment).

## Timing Effects Commands (4)

### `/slowpoke`
Delays messages before sending them.

### `/fastspammer`
Heavily rate limits messages (anti-spam punishment).

### `/pause`
Forces wait time between messages.

### `/lag`
Batches and delays messages to simulate lag.

## Social Chaos Commands (3)

### `/subtitles`
Adds confusing subtitle annotations.
- Input: "Hello"
- Output: "Hello [ominous music playing]"

### `/tourettes`
Randomly inserts loud outburst words in the middle of messages. Picks from several variant categories each time for unpredictable results:
- **Swearing** – censored-style expletives (SHIT, DAMN, BALLS, etc.)
- **Random objects** – everyday nouns shouted out of nowhere (REFRIGERATOR, PICKLE, FLAMINGO, etc.)
- **Nonsense exclamations** – absurd outbursts (BLARGH!, GADZOOKS!, KERFUFFLE!, etc.)
- **Animal sounds** – sudden creature noises (SQUAWK, COCK-A-DOODLE-DOO, RIBBIT, etc.)
- Input: "I think we should go"
- Output: "I BLARGH! think REFRIGERATOR we should DAMN go"

### `/roulette`
Random chance that each message doesn't send (message lottery).

### `/spotlight`
Announces all actions publicly with attention-grabbing prefix.
- Input: "Hello"
- Output: "📣 EVERYONE LOOK: Hello"

## Text Processing Commands (7)

### `/censor`
Randomly replaces words with [CENSORED].
- Input: "Hello world test"
- Output: "Hello [CENSORED] test"

### `/confused`
Randomly reorders words in messages.
- Input: "one two three"
- Output: "three one two"

### `/paranoid`
Adds paranoid text to messages.
- Input: "Hello"
- Output: "Hello (they're watching)"

### `/drunk`
Slurs and repeats words with random hiccups.
- Input: "Hello world"
- Output: "Heello hello worrld *hic*"

### `/hiccup`
Interrupts words with "hic".
- Input: "Hello world"
- Output: "Hello *hic* world"

### `/whistle`
Replaces letters with musical whistles.
- Input: "Hello"
- Output: "♪♫~♬♪"

### `/mumble`
Obscures message text (keeps first/last letters).
- Input: "Hello world"
- Output: "H***o w***d"

## Complex Effects Commands (3)

### `/spaghetti`
Combines 2-3 random effects together for chaotic results.

### `/torment`
Cycles through different effects each message (uppercase → backward → uwu → robotic → confused).

### `/rng`
Applies a random effect from a pool each message.

### `/essay`
Requires minimum 50 characters per message.
- Shorter messages get a warning appended

## Advanced Commands (2)

### `/haiku`
Requires 5-7-5 syllable format (validation note added).

### `/autospell`
Intentionally "autocorrects" words incorrectly.
- "the" → "teh"
- "you" → "u"
- "there" → "their"

## Fun Personality Commands (6)

### `/thesaurusoverload`
Replaces ordinary words with comically pompous synonyms and adds smug parenthetical notes.
- Input: `"i want to go now"`
- Output: `"I desire to peregrinate forthwith (ergo, QED)."`
- Input: `"stop following me"`
- Output: `"Desist following me (per se)."`
- Remove with: `/unthesaurusoverload <uid1>,<uid2>...`

**Usage:** `/thesaurusoverload [-d duration] [-r reason] <uid1>,<uid2>...`

**Example:**
```
/thesaurusoverload 12 -d 10m -r "Stop typing like a normal person"
```

---

### `/valleygirl`
Injects valley-girl filler words, stretches vowels, and adds dramatic tone.
- Input: `"stop following me"`
- Output: `"Okay sooo like… literally stop following meee?? I can't even."`
- Input: `"no"`
- Output: `"like, nooo?? I literally can't."`
- Remove with: `/unvalleygirl <uid1>,<uid2>...`

**Usage:** `/valleygirl [-d duration] [-r reason] <uid1>,<uid2>...`

**Example:**
```
/valleygirl 5 -d 30m -r "Take a deep breath"
```

---

### `/babytalk`
Converts messages to toddler-style speech with phonetic substitutions and stage directions (distinct from `/uwu`).
- Input: `"give me my evidence back right now"`
- Output: `"gib me my ev-idence bac wight now!! *tiny stomp*"`
- Input: `"please be careful"`
- Output: `"pwease be caweful *bottom lip wobbles*"`
- Remove with: `/unbabytalk <uid1>,<uid2>...`

**Usage:** `/babytalk [-d duration] [-r reason] <uid1>,<uid2>...`

**Example:**
```
/babytalk 19 -d 15m -r "No more grown-up words for you"
```

---

### `/thirdperson`
Forces messages into third-person narration using the player's display name. Adds mood tags based on punctuation and capitalisation.
- Input: `"hello everyone"` (player: Phoenix)
- Output: `"Phoenix announces to the room: \"hello everyone\""`
- Input: `"WHAT??"` (player: Phoenix)
- Output: `"Phoenix demands: \"WHAT??\" [feral][confused]"`
- Remove with: `/unthirdperson <uid1>,<uid2>...`

**Usage:** `/thirdperson [-d duration] [-r reason] <uid1>,<uid2>...`

**Example:**
```
/thirdperson 3 -d 1h -r "Narration arc"
```

---

### `/unreliablenarrator`
Makes the speaker sound like an untrustworthy narrator by adding hedges, contradictions, and suspicious commentary.
- Input: `"i didn't do it"`
- Output: `"I allegedly didn't do it (…or so I recall.)"`
- Input: `"i saw them vent"`
- Output: `"I supposedly saw them vent (citation: vibes)"`
- Remove with: `/ununreliablenarrator <uid1>,<uid2>...`

**Usage:** `/unreliablenarrator [-d duration] [-r reason] <uid1>,<uid2>...`

**Example:**
```
/unreliablenarrator 8 -d 20m -r "Stop gaslighting the courtroom"
```

---

### `/uncannyvalley`
Makes the player's identity feel slightly wrong by:
- Subtly mutating their displayed name each message (safe character swaps, vowel homoglyphs, transpositions — no true impersonation)
- Appending glitchy system notes to messages

Name mutations cycle through: vowel homoglyphs (`a→α`, `e→ε`, `o→ο`), trailing underscores, character transpositions, and duplications.

**Easter egg:** If they say "I'm fine", it gets an unsettling `:)` appended.

- Display name over time: `Phoenix` → `Phœnix` → `Pheonix` → `Phoenix_` → `Phoønix`
- Input: `"im fine"` → Output: `"im fine :) [checksum mismatch]"`
- Input: `"hello"` → Output: `"hello [signal distortion detected]"`
- Remove with: `/ununcannyvalley <uid1>,<uid2>...`

**Usage:** `/uncannyvalley [-d duration] [-r reason] <uid1>,<uid2>...`

**Example:**
```
/uncannyvalley 14 -d 45m -r "Become slightly incorrect"
```

## Area/Character Effect Commands (1)

### `/areainiswap`
Forces everyone in the current area to display as a chosen character from the server character list (same effect as `/tung` but with any character you specify). Everyone's IC sprite and emote panel switches to the target character instantly. Targets cannot change characters while the effect is active.

- `/areainiswap <character name>` — apply to the whole area
- `/areainiswap off` — remove the effect from the whole area

**Permission:** KICK

**Example:**
```
/areainiswap Maya Fey         # Force everyone in the area to display as Maya Fey
/areainiswap off              # Clear the effect for everyone in the area
```

> **Note:** The character name must match an entry in the server's character list. The name is case-insensitive. This is an area-scoped command — only players in your current area are affected.

---

## Safety Features
- **Maximum duration:** 24 hours
- **No stacking:** Same punishment type overwrites previous
- **Text length limit:** 2000 characters maximum
- **Thread-safe:** Proper mutex locking
- **Auto-expiry:** Punishments automatically remove when time expires
- **Validation:** All inputs validated, no DoS vectors
- **Logging:** All punishment actions logged with moderator name and reason

## Technical Details

### Duration Format
Durations support these formats:
- `30s` - 30 seconds
- `5m` - 5 minutes
- `2h` - 2 hours
- `1h30m` - 1 hour 30 minutes

### Permission Required
All punishment commands require the `MUTE` permission. This is typically assigned to moderators and admins.

### State Tracking
Each punishment tracks:
- Type
- Expiration time
- Reason
- Message count (for cycling effects)
- Last message time (for timing effects)

### Multiple Punishments
A player can have multiple different punishment types active simultaneously. Text modifications are applied in the order they were added.

## Best Practices

1. **Use reasonable durations** - Start with 5-10 minutes for first-time punishments
2. **Provide clear reasons** - Use `-r` flag to document why the punishment was applied
3. **Monitor effectiveness** - Some combinations can be overwhelming
4. **Remove early if needed** - Use `/unpunish` if a punishment is too harsh
5. **Be creative but fair** - These are meant to be fun, not genuinely harmful

## Examples of Effective Use

### Light Teasing
```
/uwu -d 5m -r "being too serious" 123
/pirate -d 10m -r "talk like a pirate day" 45,67
```

### Moderate Disruption
```
/stutterstep -d 15m -r "spamming" 123
/confused -d 20m -r "trolling in IC" 45
```

### Creative Combinations
```
/emoji -d 1h 123                    # Mystery player
/spaghetti -d 10m -r "chaos" 67    # Pure chaos
```

### Temporary Isolation
```
/invisible -d 30m -r "timeout" 123  # Can't see others
/whisper -d 15m -r "quiet time" 45  # Others can't see them
```

## Troubleshooting

**Q: Punishment doesn't seem to apply?**
- Verify you have MUTE permission
- Check if the UID is correct and player is connected
- Ensure duration format is valid

**Q: Can't remove a punishment?**
- Use `/unpunish <uid>` to remove all punishments
- Use `/unpunish -t <type> <uid>` for specific type

**Q: Player complains punishment is too harsh?**
- Remove it early with `/unpunish`
- Adjust duration for future use

**Q: Multiple punishments conflict?**
- Text modifications stack in application order
- Some combinations may be overwhelming - use `/unpunish` to reset

## Notes

- Punishments persist across area changes
- Punishments are lost on disconnect
- All punishment actions are logged in the server buffer
- Expired punishments are automatically cleaned up when messages are sent
