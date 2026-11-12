# Deathroll Mini-game — Command Reference

Deathroll is a **room-scoped** gambling/party mini-game. The host opens a lobby in their current room, players in that room join, and then everyone takes turns rolling `1dN`, where `N` is the previous roll. Roll a **1** and you lose — and receive the `/maso` punishment effect, locked for 10 minutes.

Each room has its own lobby and its own OOC messages: **only people in that room see the announcements**, never the whole server. Different rooms can run their own games at the same time.

---

## Quick-start

```
/deathroll host 5000      ← host opens a room lobby with a 1d5000 starting limit
/deathroll join           ← players in the room join while the lobby is open
/deathroll start          ← host starts once at least 2 players are in
/deathroll roll           ← the current player rolls 1d<current limit>
/deathroll limit 1000     ← host can change the starting limit before the start
/deathroll status         ← show the current lobby or game board
/deathroll hide           ← stop seeing Deathroll broadcasts you are not part of
```

---

## Commands

### `/deathroll host [limit]`

Opens a lobby **in your current room**. The host is automatically added as the first player. Only that room sees the lobby announcements.

| Form | Example | Notes |
|------|---------|-------|
| *(no limit)* | `/deathroll host` | starting limit `100` |
| positional | `/deathroll host 5000` | starting limit `5000` |
| `limit` keyword | `/deathroll host limit 5000` | same as above |
| `-l` / `-limit` | `/deathroll host -l 5000` | same as above |

- Minimum limit: **2**; maximum: **1,000,000**.
- The host receives a private host guide covering the rules, the configured limit, how players join, how to start, and the punishment.
- A room OOC announcement tells everyone in that room how to join, the limit, and that they have one minute to join.
- Need to change the limit after opening? Use `/deathroll limit <n>` before the start.

> **The one-minute window is informational.** There is no auto-start or auto-cancel timer — the lobby stays open until the host runs `/deathroll start` or `/deathroll cancel`.

---

### `/deathroll join`

Adds you to the open lobby. Rejected when:

- no lobby is open;
- you have already joined;
- the game has already started (late joins are not allowed).

Each successful join posts an updated lobby embed (AO2 has no message editing, so a fresh embed is posted).

---

### `/deathroll start`

Host-only. Requires at least **2 connected players**.

- Shuffles the player order.
- Sets the current upper bound to the host's configured starting limit.
- Announces the turn order and whose turn it is.

The first player then uses `/deathroll roll`; the server does not auto-roll their turn.

---

### `/deathroll roll`

The current player rolls `1d<current upper bound>`.

- The roll is announced with the roller, the result, what it was out of, and who rolls next.
- A non-1 result becomes the new upper bound and the turn passes to the next player.
- Rolling a **1** ends the game immediately.

Rolling from outside your turn is rejected with the name of the player who must roll.

---

### `/deathroll cancel`

Host-only, and only before the game starts. Cancels the lobby and clears it.

---

### `/deathroll limit <n>`

Host-only, and only before the game starts. Changes the lobby's starting limit (and the first roll bound) without cancelling and re-hosting.

- Example: `/deathroll limit 5000`
- Minimum **2**, maximum **1,000,000**.
- Posts the updated lobby embed to the room.

---

### `/deathroll hide`

Toggles your personal Deathroll broadcast filter.

- Hidden: you stop receiving lobby/join/roll/end announcements for games **you are not part of**.
- You always receive messages for a game you joined, even while hidden.
- This is a per-session preference and is not saved across reconnects.
- Run `/deathroll hide` again to show broadcasts again.

---

### `/deathroll status`

Privately shows the current lobby embed (if open) or the current game board (turn order, current limit, current player, last roll, and any forfeits).

---

## How a game plays out

1. Host runs `/deathroll host [limit]` in their room.
2. Everyone **in that room** sees the OOC announcement and lobby embed.
3. Players in that room run `/deathroll join`.
4. Host runs `/deathroll start` once at least 2 players have joined.
5. The shuffled order is announced. The first player runs `/deathroll roll`.
6. Every non-1 roll becomes the new limit; play passes to the next player.
7. The first player to roll a 1 loses, the game ends, and the punishment is applied.

### Winning

Everyone still standing when a player rolls a 1 wins. With exactly 2 players this is a single winner; with 3+ players the end message lists all survivors as the winners.

### Losing

The player who rolls a 1 loses and receives a random effect from the same pool `/maso` uses, lasting **10 minutes**.

---

## Loser punishment and the 10-minute lock

The loser receives the effect server-side:

- It is chosen from the same stateless pool as `/maso`.
- It lasts **10 minutes**.
- It is **locked for 10 minutes**: the loser cannot remove or opt out of it during that window.

The lock is enforced against:

- `/unpunish` when the loser targets **themselves** (including via their own IPID);
- `/maso` rerolling the same effect;
- `/potion off` flushing the same effect.

Another moderator targeting the loser can still clear the punishment. Normal expiry, database restores, and moderator-issued punishments are unaffected.

> The lock lives in memory, like other mini-game punishments. Reconnecting clears it; only the in-session window is enforced.

---

## Disconnects and forfeits

- Leaving an **open lobby** removes you from the roster.
- If the **host** disconnects while the lobby is open, the lobby is cancelled (there is no one left who can start it).
- Leaving an **in-progress game** forfeits you:
  - if it was your turn, the turn advances to the next active player;
  - if only one active player remains, the game ends and that player wins by default;
  - no punishment is applied to a player who already left.

There is no turn timer, so a player who stays connected but never rolls can stall an in-progress game. The host cannot cancel after the start; the game has to be finished (or the stalling player has to leave).

---

## Notes

- **One lobby per room** — each room can have its own open lobby or game at the same time.
- A player can only be in one Deathroll lobby/game at a time across all rooms.
- The host is auto-joined and can be the loser.
- The starting limit is configurable per game (`/deathroll host 5000` or `/deathroll limit 5000`); the default `100` is only a default.
- Rolls use `rand.Intn`, so large limits such as `5000` (or much larger) work normally.
- `/deathroll hide` only hides broadcasts for games you are not in; participants always receive their game's messages.
- `/help minigames` lists `/deathroll` alongside the other mini-games.
