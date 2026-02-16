# New Features Documentation

## Feature 1: Stacking Punishments

### Overview
Multiple different punishment types can now be applied to a single user simultaneously. The effects combine sequentially, creating interesting and varied punishment experiences.

### Usage
**Individual Commands**: Apply punishments one at a time using existing commands (e.g., `/uppercase`, `/backward`, `/uwu`)

**Stack Command**: Apply multiple punishments at once:
```
/stack <punishment1> <punishment2> [<punishment3>...] [-d duration] [-r reason] <uid1>,<uid2>...
```

### Examples
```
/stack uppercase backward -d 15m -r "Being silly" 5,7,9
/stack uwu pirate robotic -d 1h 12
```

### Notes
- Different punishment types stack and apply sequentially
- Adding the same punishment type twice will replace the first one
- Punishments are applied in order of addition for consistent results

---

## Feature 2: Punishment Tournament Mode

### Overview
A voluntary game mode where users compete with random punishment effects. Participants try to communicate effectively while under punishment effects. The user with the most messages wins!

### Commands

**Start Tournament** (Requires MUTE permission):
```
/tournament start
```

**Join Tournament** (Any user):
```
/join-tournament
```
- Automatically applies 2-3 random punishments to the participant
- Punishments have no expiration during the tournament

**View Status** (Requires MUTE permission):
```
/tournament status
```
Shows:
- Tournament duration
- Number of participants
- Leaderboard sorted by message count
- Time each participant has been in the tournament

**Stop Tournament** (Requires MUTE permission):
```
/tournament stop
```
- Announces winner (highest message count)
- Removes all punishments from winner
- Clears tournament state

### Participation Flow
1. Admin starts tournament with `/tournament start`
2. Users voluntarily join with `/join-tournament`
3. Random punishments are applied (2-3 per user)
4. Users send IC messages (counted automatically)
5. Admin checks leaderboard with `/tournament status`
6. Admin ends tournament with `/tournament stop`
7. Winner announced to all users

### Notes
- Tournament mode is server-wide
- Only one tournament can be active at a time
- Message counts are tracked automatically in IC messages
- Random punishments include: backward, stutterstep, elongate, uppercase, lowercase, robotic, alternating, uwu, pirate, confused, drunk, hiccup

---

## Technical Implementation Details

### Thread Safety
- Tournament state uses mutex locks for thread safety
- Safe for concurrent access by multiple clients

### State Management
- Tournament state stored in `tournamentParticipants` map
- Initialized in `InitServer()`

### Message Processing
- Tournament message counting happens in `pktIC` during IC message processing

### Testing
All features include comprehensive tests:
- Stacking punishment tests
- Punishment replacement tests
- Type conversion tests
- Sequential punishment application tests
- Tournament participant creation tests
