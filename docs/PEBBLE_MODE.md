# Pebble Mode

Pebble Mode is a no-stakes alternative economy. It can run on its own or
alongside the legacy Nyathena Chips casino.

## Configuration

```toml
[Server]
casino_enabled = false       # legacy casino (canonical)
enable_casino = false        # deprecated alias; either one enables it
pebble_mode = false          # master Pebble switch
pebble_event_chance = 5      # daily Wrecked Ship chance is 1 in N
pebble_event_timeout_minutes = 5
```

Pebble Mode is enabled in every area by default. Moderators with
`MODIFY_AREA` can disable mini-games or world events in one area:

```
/pebbleset minigames true|false
/pebbleset events true|false
/pebbleset status
```

## Rules

- Pebbles cannot be wagered, lost, bet, traded, or gifted.
- No Pebbles are awarded for playtime, idling, hourly ticks, typing races, or
  mini-games.
- Pebbles do not gate permissions or create a hierarchy.
- Existing non-gambling cosmetic tags are free for everyone. Gambling-themed
  tags are not used in Pebble Mode.

## Earning Pebbles

| Command | Reward | Cooldown |
|---|---:|---|
| `/daily` | +3 Pebbles | 20 hours |
| `/explore` | +2 Pebbles | 2 hours |
| `/cast` | +1 Pebble | 1 hour |
| Jobs (`/busker`, `/janitor`, `/paperboy`, `/clerk`, `/bailiffjob`) | existing job amounts | existing cooldowns |
| Unscramble events | +10 Pebbles | event-driven |
| World events | 5-50 Pebbles / tags | random |

Activity rewards are fixed. Jobs keep their existing random bonus chances.
World event loot is server-random and costs the player nothing.

## Balance and leaderboards

```
/pebble
/pebble top [n]      # default 10, max 50
/pebble area [n]     # live players in your area
```

Pebbles are stored per IPID in the `PEBBLES` table and survive sessions.

## Casual mini-games

```
/minigame
/minigame slots
/minigame blackjack [deal|hit|stand]
/minigame crash [start|cashout]
/minigame top [slots|blackjack|crash]
```

These are for fun only. They track streaks and high scores, never currency.

## Duck pond and quiet activities

```
/pond                 # describe the pond
/pond feed            # free, awards nothing, returns a fortune quote
/pond fortune
/ducks                # alias of /pond
/fortune
/cloudwatch
/stargaze
/nap
```

Feeding the ducks does not change your Pebble balance. Fortune quotes are
selected deterministically from IPID, UTC date and feed count, so there is no
RNG outcome.

## Shop and tags

```
/shop                 # Pebble shop overview
/shop classic         # all existing non-gambling tags (free)
/shop <category>      # attorney, anime, gamer, girly, meme, prestige
/shop activity        # cheap Pebble tags
/shop community
/shop buy <item_id>   # buy a Pebble-priced tag
/shop items
/settag <id>|none
```

Paid Pebble tags are intentionally few and cheap: `tag_explorer` (3 Pebbles)
and `tag_friendly` (5 Pebbles). There are no job passes, cooldown upgrades or
passive income upgrades in Pebble Mode.

## World events

- The **Wrecked Ship** rolls once per day at `1 / pebble_event_chance`.
- Smaller ambient events roll every 4-8 hours: Lost Satchel, Wandering
  Merchant, Meteor Shower, Picnic Basket.
- Events only spawn in public, unlocked areas that have players and have Pebble
  events enabled.
- `/claim <kind>` or `/claimwreck` claims the active event in your area.
- Simultaneous claims resolve by earliest millisecond timestamp.
- Unclaimed events disappear after `pebble_event_timeout_minutes`.
- Loot is always a bonus: Pebbles, a temporary tag, or a permanent cosmetic tag.

Players can hide global event alerts with `/events off` and restore them with
`/events on`. The preference persists on linked accounts.

## Staff

```
/pebbleset minigames true|false
/pebbleset events true|false
/grantpebbles <uid> <amount>
```

`/grantpebbles` is `ADMIN` only and targets an online player.
`/casinoenable` and `/casinoset` control only the legacy casino; they never
touch Pebble mini-games or world events.
