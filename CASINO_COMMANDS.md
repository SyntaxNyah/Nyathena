# Casino Commands — Nyathena Chips

This document covers the full casino system, including **Nyathena Chips** (the server's virtual currency), all casino games, leaderboards, and staff configuration commands.

---

## Nyathena Chips

**Nyathena Chips** are a persistent virtual currency tied to your IPID (connection fingerprint).

- Every new connection starts with **100 chips** automatically.
- Balances persist across sessions.
- Chips cannot go negative — you can only spend what you have.
- **Maximum balance: 10,000,000 chips (10 million).** Winnings are capped at this ceiling to prevent runaway inflation.
- **Default maximum bet: 1,000,000 chips (1 million)** per wager when no area-specific limit is configured. Staff can raise or lower this per-area with `/casinoset maxbet`.

---

## `/chips` — Currency Commands

| Command | Description |
|---------|-------------|
| `/chips` | Show your current chip balance. |
| `/chips top [n]` | Show the global chip leaderboard (top 10 by default, max 50). |
| `/chips area [n]` | Show the chip leaderboard for players currently in your area. |
| `/chips give <uid> <amount>` | Transfer chips to another player (max 500 per transfer, 10-minute cooldown). |
| `/richest [n]` | Shortcut for `/chips top` — show the richest players by chip balance (top 10 by default, max 50). |

### Examples

```
/chips
> 💰 Your balance: 350 Nyathena Chips

/chips top 5
> 🏆 Global Chip Leaderboard (Top 5)
>  1. abc123 — 12000 chips
>  2. def456 — 8400 chips
>  ...

/richest 5
> 🏆 Global Chip Leaderboard (Top 5)
>  1. abc123 — 12000 chips
>  2. def456 — 8400 chips
>  ...

/chips area
> 🏆 Area Chip Leaderboard — Courtroom (Top 10)
>  1. Athena — 5000 chips
>  2. Apollo — 3200 chips

/chips give 3 100
> Sent 100 chips to Phoenix. Your balance: 250 chips.
```

### Anti-abuse constraints for `/chips give`
- Maximum 500 chips per transfer.
- 10-minute cooldown between transfers.
- You cannot give chips to yourself.

---

## `/casino` — Dashboard

| Command | Description |
|---------|-------------|
| `/casino` | Display the casino dashboard for your current area (active tables, balance, game list). |
| `/casino status` | Display detailed status: active tables, current players, slots stats, jackpot pool. |

---

## Casino Games

> **All games require the casino to be enabled in the current area** (see staff commands below).  
> All bets are in Nyathena Chips. Minimum and maximum bets are set per-area.

---

### 🃏 Blackjack — `/bj`

Standard 6-deck blackjack with split, double down, and insurance.

| Subcommand | Description |
|------------|-------------|
| `/bj join` | Join or create the blackjack table (up to 6 players). |
| `/bj bet <amount>` | Place your bet before the deal. |
| `/bj deal` | Start the round (any joined player can trigger once all bets are placed). |
| `/bj hit` | Draw another card. |
| `/bj stand` | End your turn. |
| `/bj double` | Double your bet and draw exactly one more card (first action only). |
| `/bj split` | Split a pair into two hands (doubles your bet). |
| `/bj insurance` | Place an insurance side-bet when dealer shows an Ace (costs half your bet). |
| `/bj status` | Show the current state of the table (dealer's visible card, players, hands). |
| `/bj leave` | Leave the table. |

**Rules:**
- Blackjack (Ace + 10-value on deal) pays **3:2**.
- Dealer hits on soft 16, stands on soft 17.
- Insurance pays **2:1**.
- 60-second turn timer — auto-stand on timeout.

**Examples:**
```
/bj join
/bj bet 50
/bj deal
/bj hit
/bj stand
```

---

### ♠️ Poker (Texas Hold'em) — `/poker`

Full Texas Hold'em with blinds, all streets, and hand evaluation.

| Subcommand | Description |
|------------|-------------|
| `/poker join` | Join or create the poker table (up to 9 players, 500-chip buy-in). |
| `/poker ready` | Signal ready to start (game begins when all seated players are ready). |
| `/poker hand` | View your hole cards privately. |
| `/poker check` | Check (pass action, only when no bet is outstanding). |
| `/poker call` | Call the current bet. |
| `/poker bet <amount>` | Place a bet. |
| `/poker raise <amount>` | Raise the current bet. |
| `/poker fold` | Fold your hand. |
| `/poker allin` | Go all-in with your remaining stack. |
| `/poker status` | Show the pot, community cards, and active players. |
| `/poker leave` | Leave the table (fold if in a hand). |

**Rules:**
- Small blind: 25 chips. Big blind: 50 chips.
- Hole cards are shown privately; community cards and pot are broadcast to the area.
- 60-second turn timer — auto-fold on timeout.

---

### 🎰 Slots — `/slots`

| Subcommand | Description |
|------------|-------------|
| `/slots spin [amount]` | Spin the slots with an optional bet (default: 10 chips). |
| `/slots max` | Spin with the area's maximum bet. |
| `/slots jackpot` | View the current jackpot pool amount. |
| `/slots stats` | View area-wide slots statistics (total spins, payout, jackpots hit). |

**Symbols and payouts** (from highest to lowest):

| Combination | Payout |
|-------------|--------|
| 🎰🎰🎰 (Jackpot) | Full jackpot pool |
| 💎💎💎 | 100× bet |
| ⭐⭐⭐ | 50× bet |
| 🍇🍇🍇 | 20× bet |
| 🍊🍊🍊 | 10× bet |
| 🍋🍋🍋 | 5× bet |
| 🍒🍒🍒 | 3× bet |
| 🍒🍒 (any position) | 2× bet |
| Any 2 matching | 1× bet (push) |
| No match | 0 (loss) |

**Jackpot:** 5% of each losing spin is added to the area jackpot pool. Jackpot is only available when the area staff enables it.

**Rate limit:** Maximum 5 spins per 10 seconds.

---

### 🔴 European Roulette — `/croulette`

```
/croulette bet <type> <amount>
```

| Bet Type | Description | Payout |
|----------|-------------|--------|
| `red` | Red numbers | 1:1 |
| `black` | Black numbers | 1:1 |
| `even` | Even numbers (not 0) | 1:1 |
| `odd` | Odd numbers | 1:1 |
| `low` | Numbers 1–18 | 1:1 |
| `high` | Numbers 19–36 | 1:1 |
| `number <n>` | Straight up (0–36) | 35:1 |

**Example:**
```
/croulette bet red 100
/croulette bet number 17 50
```

---

### 🎴 Baccarat — `/baccarat`

```
/baccarat <player|banker|tie> <amount>
```

| Bet | Payout |
|-----|--------|
| `player` | 1:1 |
| `banker` | 0.95:1 (5% commission) |
| `tie` | 8:1 |

---

### 🎲 Craps (Lite) — `/craps`

```
/craps bet <pass|nopass> <amount>
```

**Pass line rules:**
- First roll 7 or 11: **pass wins**.
- First roll 2, 3, or 12: **pass loses** (craps out).
- Any other number becomes the **point**; keep rolling until point repeats (pass wins) or 7 appears (pass loses).

Pass pays **1:1**; don't-pass is the opposite outcome.

---

### 📈 Crash — `/crash`

Bet before launch, then cash out before the multiplier crashes.

```
/crash bet <amount>   — start a round with a bet
/crash cashout        — cash out at the current multiplier
```

- The multiplier grows every second starting at 1.00×.
- The game crashes at a random point between 1.2× and 20×.
- If you don't `/crash cashout` before the crash, you lose your bet.

---

### 💣 Mines — `/mines`

Navigate a minefield for increasing multipliers.

```
/mines start <mines> <bet>  — start a game (1–24 mines on 5×5 grid)
/mines pick <n>             — reveal cell n (1–25)
/mines cashout              — collect your current winnings
/mines quit                 — give up (lose your bet)
```

- Each safe pick increases your multiplier.
- Hitting a mine ends the game and you lose your bet.
- The more mines, the higher the potential multiplier per pick.

---

### 🎱 Keno — `/keno`

Pick numbers and hope they match the draw.

```
/keno pick <n1> <n2> ... <bet>
```

- Pick 1–10 numbers from 1–80.
- 20 numbers are drawn randomly.
- Payout scales by how many of your picks match (the more picks you make, the higher the jackpot for matching all of them).

**Example:**
```
/keno pick 7 14 21 28 35 100
```

---

### 🎡 Prize Wheel — `/wheel`

```
/wheel spin <bet>
```

**Wheel segments (casino-realistic odds, ~92.5% RTP):**

| Multiplier | Probability |
|------------|-------------|
| 0× (miss)  | 60% |
| 1.5×       | 17% |
| 2×         | 13% |
| 3×         | 7%  |
| 5×         | 2%  |
| 10×        | 1%  |

---

### 🔇 Gamble Hide — `/gamble hide`

Toggle whether you see gambling broadcast messages in the area chat.

```
/gamble hide
```

- Run once to **hide** all gambling result announcements (wheel spins, slot wins, roulette results, etc.) from your chat.
- Run again to **show** them again.
- This setting lasts until you disconnect.

---

## Staff Commands (MODIFY_AREA permission required)

### `/casinoenable <true|false>`
Enable or disable the casino for the current area.

```
/casinoenable true
/casinoenable false
```

### `/casinoset <setting> <value>`
Configure casino settings for the current area.

| Setting | Type | Description |
|---------|------|-------------|
| `minbet` | integer | Minimum bet in chips (0 = no limit) |
| `maxbet` | integer | Maximum bet in chips (0 = no limit) |
| `maxtables` | integer | Maximum simultaneous active tables (0 = no limit) |
| `jackpot` | true/false | Enable or disable the slots jackpot for this area |

**Examples:**
```
/casinoset minbet 10
/casinoset maxbet 500
/casinoset maxtables 2
/casinoset jackpot true
```

---

## `/areainfo` Integration

`/areainfo` now includes a **Casino** line showing:
- Whether the casino is enabled or disabled.
- Bet limits (if configured).
- Current jackpot pool (if jackpot is enabled).

**Example output:**
```
BG: courtroom
Evi mode: Mods
...
Casino: enabled (bet: 10–500), jackpot pool: 1200
```

---

## Anti-spam / Rate Limits

| Feature | Limit |
|---------|-------|
| Slots spins | Max 5 spins per 10 seconds |
| Chips give cooldown | 10 minutes between transfers |
| Chips give max | 500 chips per transfer |
| BJ/Poker turn timer | 60 seconds — auto-stand/fold |

---

## Persistence

All chip balances are stored in the server's SQLite database (`CHIPS` table). Balances survive server restarts. New IPIDs receive 100 chips on first connection.
