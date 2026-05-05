# Hangman Mini-game — Command Reference

Hangman is an area-scoped social deduction / party word-guessing game.  
The host picks a word (or lets the server choose one from a theme) and every player in the area can join and guess.  
Wrong-guessers receive a random punishment when the word isn't solved!

---

## Quick-start

```
/hangman start              ← host opens a game with a random word
/hangman join               ← players opt in during the 30-second window
/hangman guess e            ← guess the letter E
/hangman guess elephant     ← guess the full word
/hangman status             ← print the current board
```

---

## Commands

### `/hangman start [options]`

Starts a Hangman game in your current area.  
The host is automatically opted in as a participant.  
A 30-second window opens for others to join.

| Option | Example | Notes |
|--------|---------|-------|
| *(none)* | `/hangman start` | random word from the combined pool |
| `animals` | `/hangman start animals` | word from the animal theme |
| `courtroom` | `/hangman start courtroom` | word from the courtroom/law theme |
| `nature` | `/hangman start nature` | word from the nature/geography theme |
| `food` | `/hangman start food` | word from the food theme |
| `random` | `/hangman start random` | explicit random (same as no option) |
| `custom <word>` | `/hangman start custom objection` | host supplies the secret word (3–30 letters, no spaces) |

> **Cooldown:** 3 minutes per area between games.

---

### `/hangman join`

Opt in to the active game during the 30-second sign-up window.  
You can only join before the game starts.

---

### `/hangman guess <letter|word>`

Submit a guess during an active game.

- `/hangman guess a` — guess the **single letter** A  
- `/hangman guess attorney` — guess the **full word**

Rules:
- Letters must be alphabetical (no digits or symbols).  
- Each single letter can only be guessed **once** per game.  
- A wrong full-word guess costs **one strike** (shown as ★ on the board).  
- A correct full-word guess **immediately wins** the game.

---

### `/hangman status`

Prints the current board to **you only** (private message).  
Shows the gallows art, the masked word, wrong guesses, and remaining strikes.

---

### `/hangman stop`

Forcibly ends the game and reveals the answer.  
Requires: game host, CM, or moderator.

---

## How a game plays out

1. **Host** types `/hangman start [theme|custom word]`.  
2. Server announces the game, rules, and word length in area OOC.  
3. Players type `/hangman join` within 30 seconds.  
4. Game begins — the masked word is shown:  `_ _ _ _ _ _ _ _`
5. Players take turns guessing letters or the full word via `/hangman guess`.  
6. After each guess the updated board is broadcast to the area.

### Win condition

All letters are guessed before 6 wrong guesses are reached → **everyone is safe**.

### Loss condition

6 wrong guesses are reached → the hangman is complete → **game over**.  
Every player who made at least one wrong guess receives a random timed punishment.  
Players who never guessed incorrectly (or never guessed at all) are unaffected.

---

## Gallows progression

```
 ___      ___      ___      ___      ___      ___      ___
|   |    |   |    |   |    |   |    |   |    |   |    |   |
|        |   O    |   O    |   O    |   O    |   O    |   O
|        |        |   |    |  /|    |  /|\   |  /|\   |  /|\
|        |        |        |        |        |  /     |  / \
|___     |___     |___     |___     |___     |___     |___
 0/6      1/6      2/6      3/6      4/6      5/6      6/6
```

---

## Word themes

| Theme | Description |
|-------|-------------|
| `animals` | Mammals, reptiles, birds, sea creatures, and mythical critters |
| `courtroom` | Legal terms from Attorney Online lore and real-world law |
| `nature` | Geographical features, weather phenomena, biomes |
| `food` | International dishes, fruits, vegetables, and ingredients |
| `random` | Combined pool of all themes (default) |
| `custom <word>` | Host-supplied word — kept secret from participants |

---

## Punishment

Losers (players who made at least one wrong guess) receive a random timed punishment lasting **10 minutes**.  
The punishment is drawn from the same pool as Hot Potato:

> Backward, Stutterstep, Elongate, Uppercase, Lowercase, Robotic, Alternating, Uwu, Pirate, Caveman, Drunk, Hiccup, Confused, Paranoid, Mumble, Subtitles

Players who guessed only correct letters, or who only used `/hangman status`, escape punishment entirely.

---

## Notes

- Each area runs its own independent Hangman game — games in different areas do not interfere.
- Area CMs and moderators can always `/hangman stop` regardless of whether they are the host.
- The host is auto-enrolled; they do **not** need to type `/hangman join` separately.
- There is no time limit once the game starts — the word stays up until solved or 6 wrong guesses.
