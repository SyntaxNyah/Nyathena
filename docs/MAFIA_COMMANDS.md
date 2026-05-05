# 🎭 Mafia Minigame — Complete Guide

## Overview

The Mafia minigame is a social deduction game played inside an area on the
Nyathena server. Players are secretly assigned roles — either **Town**, **Mafia**,
or **Neutral** — and must work together (or against each other) to achieve their
win condition before the game ends.

The game alternates between **Day** and **Night** phases:

- **Day**: All players discuss, and the town votes to lynch a suspect.
- **Night**: Special roles submit private actions while the Mafia quietly picks a
  kill target.

---

## Quick-Start

```
/mafia create          ← Host opens a lobby
/mafia join            ← Others join
/mafia start           ← Host starts when ≥3 players are ready
```

---

## Rules

### Win Conditions

| Team | Win Condition |
|------|--------------|
| **Town** | Eliminate every Mafia member, Serial Killer, and Arsonist. |
| **Mafia** | Equal or outnumber all Town-aligned players combined. |
| **Serial Killer** | Be the last non-neutral faction standing. |
| **Arsonist** | Be the last non-neutral faction standing. |
| **Jester** | Get yourself lynched by the town. |
| **Survivor** | Still be alive when the game ends, regardless of who wins. |

### Lynch Rules

- During the **Day Discussion** phase each alive player may cast one vote with
  `/mafia vote <name>`.
- Change or clear your vote at any time with `/mafia vote <name>` again or
  `/mafia skip`.
- A player is immediately lynched when they reach a **simple majority** of
  alive voters (⌊alive ÷ 2⌋ + 1).
- If no majority is reached when the phase ends (or everyone skips), **no one
  is lynched**.
- **Mayor exception**: A revealed Mayor's vote counts as **2** votes.

### Night Rules

- During the **Night** phase, speaking in IC chat is still possible, but
  role actions are submitted privately with `/mafia act`.
- **Mafia members share a kill target** — the last submitted target wins.
- The Escort's roleblock is processed before all other actions.
- Doctor protection cancels a single kill on the protected player.
- All kills are resolved simultaneously at the start of the next Day.

### Death and Last Wills

- When a player dies (lynched, killed at night, or shot by the Sheriff) their
  **last will** is read aloud, if they set one with `/mafia will <text>`.
- Dead players remain in the area and can observe, but may not influence the
  game.

### Disconnects

- If an alive player disconnects they are automatically killed and removed
  from the game.

---

## Commands

### Everyone

| Command | Phase | Description |
|---------|-------|-------------|
| `/mafia help` | Any | Show command usage. |
| `/mafia roles` | Any | List all roles with descriptions. |
| `/mafia create` | — | Open a new lobby in this area. |
| `/mafia join` | Lobby | Join the open lobby. |
| `/mafia leave` | Any | Leave the lobby or resign from the game. |
| `/mafia players` / `/mafia status` | Any | Show the player list and game state. |
| `/mafia vote <name>` | Day | Vote to lynch the named player. |
| `/mafia skip` | Day | Clear your vote (no-lynch). |
| `/mafia tally` | Day | Show the current vote standings. |
| `/mafia act <target>` | Night | Submit your night action (see role table below). |
| `/mafia shoot <name>` | Day | Sheriff only: spend your one-time shot. |
| `/mafia will <text>` | Any | Set your last will (revealed when you die). |
| `/mafia whisper <name> <message>` | Active game | Send a private in-game message to another alive player. Everyone else sees that a whisper happened, but not its contents. |
| `/mafia graveyard` / `/mafia dead` | Any | Show the graveyard with cause of death for each eliminated player. |

### Host / CM / Mod Only

| Command | Description |
|---------|-------------|
| `/mafia start` | Start the game and assign roles. |
| `/mafia day` | Force-advance to the Day phase. |
| `/mafia night` | Force-advance to the Night phase. |
| `/mafia resolve` | Force-resolve the current phase immediately. |
| `/mafia stop` | Abort the game. |
| `/mafia kick <name>` | Remove a player from the lobby or game. |
| `/mafia reveal` | Force-reveal all roles to the area. |
| `/mafia timer <seconds>` | Set the phase auto-timer (`0` = disabled). |

---

## Roles

### Town Roles

Town players win when every Mafia member, Serial Killer, and Arsonist is
eliminated.

---

#### 🏘️ Villager

> *An ordinary resident who relies on deduction and discussion.*

| | |
|-|-|
| **Team** | Town |
| **Ability** | None |
| **Win** | Eliminate all threats |

No special power. Villagers must vote wisely, pay attention to behaviour, and
trust the investigators.

---

#### 🔎 Detective

> *A sharp-eyed investigator who reads people for a living.*

| | |
|-|-|
| **Team** | Town |
| **Night action** | `/mafia act <target>` — investigate a player |
| **Win** | Eliminate all threats |

At the start of each morning the Detective learns whether their target is
**Good**-aligned, **Evil**-aligned, or **Neutral**-aligned.

> ⚠️ The **Shapeshifter** and **Godfather** both appear **Good** to the Detective.

---

#### 💊 Doctor

> *A skilled healer who can keep someone alive through the night.*

| | |
|-|-|
| **Team** | Town |
| **Night action** | `/mafia act <target>` — protect a player |
| **Win** | Eliminate all threats |

If any killer targets the protected player that night, the kill is cancelled.
The attacker is not informed of the protection.

---

#### 🔫 Sheriff

> *A lone gunslinger with one bullet and the courage to use it.*

| | |
|-|-|
| **Team** | Town |
| **Day action** | `/mafia shoot <name>` — one-time shot |
| **Win** | Eliminate all threats |

The Sheriff can shoot a player **once per game** during the Day phase.

- If the target is **Mafia**, they die.
- If the target is the **Godfather**, the bullet bounces — the Sheriff dies.
- If the target is anyone else, the gun **backfires** and the Sheriff dies.

Use this power wisely.

---

#### 🛡️ Bodyguard

> *A selfless protector willing to take a bullet.*

| | |
|-|-|
| **Team** | Town |
| **Night action** | `/mafia act <target>` — guard a player |
| **Win** | Eliminate all threats |

If the Mafia (or any killer) targets the guarded player, the Bodyguard steps
in front: **the attacker and the Bodyguard both die**, but the guarded player
lives.

---

#### 🕵️ Vigilante

> *A lone hero who takes justice into their own hands.*

| | |
|-|-|
| **Team** | Town |
| **Night action** | `/mafia act <target>` — execute a player (once per game) |
| **Win** | Eliminate all threats |

The Vigilante may kill **one player** during the Night phase — but there is a
heavy price for a mistake.

- If the target is **Evil** or **Neutral**, the kill is clean.
- If the target is **Town-aligned** (Good), the Vigilante **dies of guilt at
  the start of the following morning**.

Their last will is revealed alongside their guilt death.

---

#### 🎙️ Mayor

> *The respected leader whose word carries extra weight.*

| | |
|-|-|
| **Team** | Town |
| **Day action** | `/mafia act reveal` — announce yourself publicly (once per game) |
| **Win** | Eliminate all threats |

After revealing, every lynch vote the Mayor casts counts as **2 votes** for
the rest of the game. This is shown in the vote tally as `(×2 Mayor vote)`.

Revealing is a double-edged sword: it makes you a powerful ally of the town,
but also a high-priority target for the Mafia.

---

#### 💃 Escort

> *A charming distraction who keeps suspicious players occupied.*

| | |
|-|-|
| **Team** | Town |
| **Night action** | `/mafia act <target>` — roleblock a player |
| **Win** | Eliminate all threats |

The Escort's target cannot perform their night action that night. Roleblock is
processed before any other action:

- A **Doctor** who is roleblocked cannot protect anyone.
- A **Detective** who is roleblocked gets no investigation result.
- A **Serial Killer** who is roleblocked cannot kill.
- A **Vigilante** who is roleblocked cannot fire their shot.
- The **Mafia's consensus kill** is NOT blocked by the Escort (it is a team
  action, not a single player's action).

The Escort privately learns that their target was blocked.

---

### Mafia Roles

Mafia players share a nightly kill target. They know each other's identities
from the start. They win when they equal or outnumber all Town-aligned players.

---

#### 🔪 Mafia

> *A member of the organised crime syndicate.*

| | |
|-|-|
| **Team** | Mafia |
| **Night action** | `/mafia act <target>` — submit the Mafia kill target |
| **Win** | Equal or outnumber Town |

All Mafia members can submit a kill target; the **last submitted target** is
used. Coordinate with your team in private.

---

#### 🎭 Shapeshifter

> *A master of disguise hiding in plain sight.*

| | |
|-|-|
| **Team** | Mafia |
| **Night action** | `/mafia act <target>` — submit the Mafia kill target |
| **Win** | Equal or outnumber Town |

Functionally identical to the Mafia role, but appears **Good-aligned** to the
Detective — making them nearly impossible to catch through investigation alone.

---

#### 👴 Godfather

> *The Mafia's undisputed boss — clean hands, cold mind.*

| | |
|-|-|
| **Team** | Mafia |
| **Night action** | `/mafia act <target>` — submit the Mafia kill target |
| **Win** | Equal or outnumber Town |

The Godfather has two passive immunities:

1. **Detective**: Appears **Town-aligned** (Good), just like the Shapeshifter.
2. **Sheriff**: If the Sheriff shoots the Godfather, the **bullet bounces** —
   the Sheriff dies instead, and the Godfather is unharmed.

The Godfather is the hardest Mafia member to expose.

---

### Neutral Roles

Neutral roles have unique win conditions and belong to neither Town nor Mafia.

---

#### 🃏 Jester

> *A chaotic trickster who just wants to be noticed.*

| | |
|-|-|
| **Team** | Neutral |
| **Ability** | None |
| **Win** | Get yourself lynched |

The Jester acts suspicious on purpose. If they are successfully **voted out and
lynched** by the town, they win — and the game ends immediately.

---

#### 🧙 Witch

> *A shadowy manipulator who bends others to her will.*

| | |
|-|-|
| **Team** | Neutral |
| **Night action** | `/mafia act <player> <newtarget>` — redirect someone's action |
| **Win** | Survive to game end |

The Witch chooses a player and redirects their night action to a new target.
For example, redirecting the Doctor to protect yourself, or sending the
Vigilante after the wrong player.

---

#### ⚖️ Lawyer

> *A morally questionable defender of the guilty.*

| | |
|-|-|
| **Team** | Neutral |
| **Ability** | None |
| **Win** | Their assigned client survives to the end |

The Lawyer is secretly given a **client** at the start of the game. The Lawyer
wins if their client is still alive when the game ends, regardless of faction.

---

#### 🔥 Arsonist

> *A quiet danger who patiently waits for the right moment to strike.*

| | |
|-|-|
| **Team** | Neutral |
| **Night action** | `/mafia act douse <target>` — pour gasoline on a player |
| | `/mafia act ignite` — ignite all doused targets simultaneously |
| **Win** | Be the last non-neutral faction standing |

The Arsonist spends nights soaking players in gasoline. When ready, they
ignite everyone at once in a single catastrophic night. Doctor protection does
**not** save doused players.

---

#### 🔪 Serial Killer

> *A lone predator who answers to no one.*

| | |
|-|-|
| **Team** | Neutral |
| **Night action** | `/mafia act <target>` — eliminate a player |
| **Win** | Be the last non-neutral faction standing |

The Serial Killer acts alone and kills once per night. They are **immune to the
first Mafia kill attempt** (the hit rolls off). After that, they can be killed
like anyone else.

---

#### 🛡️ Survivor

> *A bystander determined to make it out alive no matter what.*

| | |
|-|-|
| **Team** | Neutral |
| **Ability** | None |
| **Win** | Still be alive when the game ends |

The Survivor has no special power and no loyalty to any faction. They win
**alongside** the dominant faction as long as they are still breathing when
the game concludes. Because they have no ability, they are essentially an
extra target that benefits from flying under the radar.

---

## Night Action Reference

| Role | Command | Notes |
|------|---------|-------|
| Mafia / Shapeshifter / Godfather | `/mafia act <target>` | Consensus kill; last submitted wins |
| Detective | `/mafia act <target>` | Investigate alignment |
| Doctor | `/mafia act <target>` | Prevent one kill |
| Bodyguard | `/mafia act <target>` | Step in front; attacker and BG both die |
| Vigilante | `/mafia act <target>` | One-time kill; guilt-death if Town target |
| Escort | `/mafia act <target>` | Roleblock; cancels target's action |
| Serial Killer | `/mafia act <target>` | Solo kill each night |
| Arsonist | `/mafia act douse <target>` | Pour gasoline |
| Arsonist | `/mafia act ignite` | Burn all doused players |
| Witch | `/mafia act <player> <newtarget>` | Redirect action |
| Mayor | `/mafia act reveal` *(Day only)* | Announce publicly; double vote |

Roles not listed above (Villager, Jester, Lawyer, Survivor) have **no night
action**.

---

## Tips & Strategy

### General
- Use `/mafia will <text>` at the start of the game to leave clues for the
  town if you are killed.
- Use `/mafia graveyard` to review who has died and how, which may help narrow
  down who certain roles are.
- Use `/mafia tally` during the day to see real-time vote standings without
  waiting for others to announce theirs.
- Use `/mafia whisper <name> <msg>` to coordinate privately without revealing
  your role publicly. Remember: the area can see *that* you whispered, just
  not *what* you said.

### Town
- The Detective should investigate players who are quiet or who push hard on
  innocents. Share results carefully — the Mafia will target you.
- The Doctor should prioritise protecting the Detective or other confirmed town
  members.
- The Sheriff should wait for solid reads before shooting — a backfire wastes
  your life and reveals nothing.
- The Vigilante should **not** shoot on Night 1 unless absolutely certain.
  Killing a Town member means dying the next morning.
- The Mayor should only reveal when their double vote is decisive, or when the
  town desperately needs a trusted swing vote.
- The Escort can cripple the Serial Killer or a known Mafia member — but be
  careful not to block the Doctor accidentally.

### Mafia
- Stagger your kills to avoid obvious patterns.
- Target the Detective or Doctor first to cripple town information.
- If the Godfather is identified, the Sheriff will likely try to shoot — be
  ready to defend him socially.
- Vote with the town against easy Neutral targets early to build credibility.

### Neutral
- **Jester**: Act suspicious but not so obviously that the town suspects you of
  being suspicious *on purpose*. Provoke heated arguments.
- **Witch**: Redirecting the Doctor onto yourself on a night you expect an
  attack can save your life.
- **Arsonist**: Douse quietly over several nights. Ignite when the Doctor
  appears distracted elsewhere.
- **Serial Killer**: Use your one-time immunity early by provoking the Mafia
  into targeting you, then clear the field.
- **Survivor**: Keep a low profile. Don't get lynched, and don't become the
  Mafia's late-game cleanup target.

---

## Admin Reference

### Area Setup

Enable the game in any area by running `/mafia create`. No extra configuration
is needed — role pools are assigned automatically based on player count.

### Phase Timer

```
/mafia timer 120   ← each phase auto-resolves after 120 seconds
/mafia timer 0     ← disable auto-timer (manual-only)
```

### Balanced Role Pools (Auto-assigned)

| Players | Roles |
|---------|-------|
| 4 | Mafia, Detective, Doctor, Villager |
| 5 | Mafia, Detective, Doctor, Villager, Jester |
| 6 | Mafia ×2, Detective, Doctor, Villager, Jester |
| 7 | Mafia ×2, Detective, Doctor, Sheriff, Villager, Jester |
| 8 | Mafia ×2, Detective, Doctor, Sheriff, Vigilante, Villager, Jester |
| 9 | Mafia ×2, Detective, Doctor, Sheriff, Vigilante, Escort, Villager, Jester |
| 10 | Mafia ×2, Godfather, Detective, Doctor, Sheriff, Vigilante, Escort, Bodyguard, Jester |
| 11 | Mafia ×2, Godfather, Detective, Doctor, Sheriff, Vigilante, Escort, Bodyguard, Jester, Arsonist |
| 12–13 | Mafia ×2, Godfather, Shapeshifter, Detective, Doctor, Sheriff, Vigilante, Escort, Bodyguard, Jester, Arsonist *(, Serial Killer)* |
| 14+ | Dynamic — 1 Godfather + ⌊n/4⌋ Mafia, then specials fill in order |

Custom role pools can be created by modifying the server's role configuration.

---

*See also: [`CASINO_COMMANDS.md`](CASINO_COMMANDS.md) for the Casino minigame.*
