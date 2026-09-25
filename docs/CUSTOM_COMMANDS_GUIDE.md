# Writing Your Own Custom Commands

A hands-on guide to the Custom Command Builder. This walks you from "I want a
new slash command" to a working, tested, live command — using **`/shoe`** as the
worked example, then covering **every action type** you can compose with.

Custom commands are JSON definitions. You never touch Go: you either build them
through the console wizard, or write a small JSON file. They install live — no
restart, no rebuild, no `git pull`.

- **Where they live:** `config/custom_commands/` (one `.json` file per command).
- **Examples shipped with the server:** `config_sample/custom_commands/` — copy
  one into your `config/custom_commands/` and `customcmd reload`.
- **Reference (terse):** [docs/CUSTOM_COMMANDS.md](CUSTOM_COMMANDS.md) is the
  compact field reference. This guide is the tutorial.

---

## 1. Before you start

The builder is **off by default**. Enable it once in `config.toml`:

```toml
enable_custom_commands = true
```

Restart, then type `customcmd` in the server console (stdin) to open the menu.

---

## 2. Three ways to make a command

1. **The wizard (recommended for beginners).** Type `customcmd` and press `1`
   (or `new`). It prompts you for the name, description, permission, and a list
   of actions — Enter accepts a default everywhere.

2. **A one-liner.** Fast path for simple commands:

   ```
   customcmd quick mute uwu 10m          # mute + apply "uwu" effect for 10m
   customcmd quick boop message "boops you on the nose"
   ```

3. **A JSON file.** Write it by hand and either drop it in
   `config/custom_commands/` + `customcmd reload`, or `customcmd import <file>`.
   This is the most precise and the easiest to version-control.

Every one of these produces the same thing: a JSON definition.

---

## 3. Anatomy of a command

Here is the complete top-level shape, every field annotated:

```json
{
  "name": "shoe",            // the command word, WITHOUT the slash
  "desc": "Turn a player's periods into commas; sometimes their whole sentence becomes 'tuff'.",
  "category": "fun",         // shown in customcmd list; free-form, defaults to "custom"
  "reqPerms": "NONE",        // who may run it (see §7)
  "usage": "/shoe <uid>",    // shown on -h and on too-few-args
  "minArgs": 1,              // minimum words the player must type after /shoe
  "publicHelp": true,        // true = listed in /help; false = hidden
  "account": "",             // optional: lock to one account name (see §7)
  "actions": [ ... ]         // the ordered list of things it does
}
```

| Field | Required | Meaning |
|-------|----------|---------|
| `name` | ✅ | The command word, no slash, lowercase `a-z0-9_-`. Cannot shadow a built-in (`/ban`, `/help`, …). |
| `desc` | ✅ | One-line description (shown in `/help` and `customcmd list`). |
| `category` | — | Grouping label for `customcmd list`; defaults to `custom`. |
| `reqPerms` | — | Minimum permission role; defaults to `NONE` (everyone). |
| `usage` | — | Usage string; defaults to `/<name>`. |
| `minArgs` | — | Minimum args after the command before it runs; defaults to `0`. |
| `publicHelp` | — | Show in public `/help`; defaults to `false`. |
| `account` | — | If set, only that account (case-insensitive) can run it. |
| `actions` | ✅ | Ordered list of actions (see §5). At least one required. |

The **name** is the command players type: a file `shoe.json` with `"name":
"shoe"` becomes `/shoe`. Names are stored lowercase; `shoe.json`, `Shoe.json`,
and a `"name": "shoe"` field all resolve to `/shoe` (but write the field in
lowercase to avoid surprises).

## 4. The prime example: `/shoe`, line by line

`/shoe` is a great teacher because it is small but shows the two ideas that make
custom commands powerful: **targeting** and **a data-driven effect** (invented
with zero Go). Here is the whole file:

```json
{
  "name": "shoe",
  "desc": "Turn a player's periods into commas; sometimes their whole sentence becomes 'tuff'.",
  "category": "fun",
  "reqPerms": "NONE",
  "usage": "/shoe <uid>",
  "minArgs": 1,
  "publicHelp": true,
  "actions": [
    {
      "type": "text",
      "target": "@args",
      "duration": "10m",
      "replace": [
        { "from": ".", "to": "," }
      ],
      "random": ["thas tuff", "lowk tuff", "tuff"],
      "chance": 0.25
    }
  ]
}
```

Walkthrough:

- **`name` / `desc` / `category`** — the command word `/shoe`, its `/help`
  blurb, and the `fun` grouping shown in `customcmd list`.

- **`reqPerms: "NONE"`** — anyone can run it. (It's a "fun" effect; you might
  gate a harsher variant behind `MUTE` or `ADMIN`.)

- **`minArgs: 1`** — requires one argument: the target's UID. If a player types
  `/shoe` with nothing after it, the server replies with `usage`
  (`/shoe <uid>`) and nothing happens.

- **`actions`** — exactly one action, of `type: "text"`:

  - **`target: "@args"`** — "the UIDs typed after the command". So `/shoe 7`
    applies to player 7. (`@args` reads the *first* argument as a UID list, so
    `/shoe 5,7,9` hits all three.)
  - **`duration: "10m"`** — the effect lasts 10 minutes.
  - **`replace: [ { "from": ".", "to": "," } ]`** — for every IC message the
    target sends, each `.` becomes a `,`. So `hello. world.` → `hello, world,`.
  - **`random: [...]` + `chance: 0.25`** — additionally, 25% of the time, the
    *entire* message is thrown away and replaced by a random pick from the list
    (`thas tuff` / `lowk tuff` / `tuff`). The find/replace runs first, then the
    random whole-message swap is rolled.

That's it. That single `text` action is a full punishment *type* — persisted
with the player, shown on `/getlog`, and cleared by `/unpunish` — that no one
had to code.

### The same effect, applied via `@global`

A common pattern is to hit everyone in the area instead of one target. Swap
`target`:

```json
{ "type": "text", "target": "@global", "duration": "5m",
  "replace": [{ "from": "r", "to": "w" }] }
```

Now every non-staff player in the caller's area gets "uwu"-style `r`→`w` for
five minutes. (Targets are covered fully in §6.)

## 5. Action reference (every type)

Actions run **top to bottom**, in the order listed. Order matters: mute *then*
punish *then* notify reads naturally; the reverse would fire the notify first.

Durations everywhere use `30s`, `10m`, `1h30m`, etc. An empty `duration` uses
the action's default.

### `message` — send an OOC message to target(s)

Sends a private server message (OOC) to each target.

| Field | Meaning |
|-------|---------|
| `target` | who gets it (see §6) |
| `text` | the message; `{caller.name}`, `{target.name}`, etc. are expanded |

```json
{ "type": "message", "target": "@args", "text": "You were booped by {caller.name}!" }
```

### `announce` — broadcast to the whole server

Prepend "[Announcement] " and send to **everyone** (no `target`).

| Field | Meaning |
|-------|---------|
| `text` | the message |

```json
{ "type": "announce", "text": "{caller.name} has opened the ball pit." }
```

Note: because there is no single target, `{target.*}` placeholders are empty
here — use `{caller.*}` and `{args.N}`.

### `punish` — apply a built-in punishment effect

Applies one of the server's ~200 named effects to the target(s).

| Field | Meaning |
|-------|---------|
| `target` | who gets it |
| `effect` | the effect name (see the list in §8) |
| `duration` | how long; default **10m**, capped at **24h** |
| `reason` | recorded on the punishment |

```json
{ "type": "punish", "target": "@args", "effect": "uwu", "duration": "10m", "reason": "too serious" }
```

### `unpunish` — remove an effect (or all of them)

| Field | Meaning |
|-------|---------|
| `target` | who |
| `effect` | the effect to remove, **or** `"all"` to clear everything |

```json
{ "type": "unpunish", "target": "@args", "effect": "all" }
```

### `kick` — disconnect the target(s)

| Field | Meaning |
|-------|---------|
| `target` | who |
| `reason` | shown on their disconnect |

```json
{ "type": "kick", "target": "@args", "reason": "You were asked to leave." }
```

### `ban` — ban the target(s)

| Field | Meaning |
|-------|---------|
| `target` | who |
| `duration` | ban length; default **24h** |
| `reason` | recorded on the ban |

```json
{ "type": "ban", "target": "@args", "duration": "48h", "reason": "griefing" }
```

### `mute` — mute the target(s)

| Field | Meaning |
|-------|---------|
| `target` | who |
| `scope` | `ic`, `ooc`, `both`, `music`, or `jud` (default `ic`) |
| `duration` | how long; default **0** = permanent (until unmuted) |
| `reason` | recorded |

```json
{ "type": "mute", "target": "@args", "scope": "both", "duration": "10m" }
```

### `unmute` — unmute the target(s)

| Field | Meaning |
|-------|---------|
| `target` | who |

```json
{ "type": "unmute", "target": "@args" }
```

### `move` — move target(s) to an area

| Field | Meaning |
|-------|---------|
| `target` | who |
| `area` | the exact area name (case-insensitive) |

```json
{ "type": "move", "target": "@args", "area": "lobby" }
```

### `run` — call any built-in command

This is the escape hatch: invoke any of the ~250 existing server commands as
one step. The built-in runs **as the caller**, so the caller needs permission
for that built-in too.

| Field | Meaning |
|-------|---------|
| `command` | the built-in name (no slash), e.g. `backward`, `uwu`, `kick` |
| `args` | arguments passed to it; `{args.N}` and friends are expanded |

```json
{ "type": "run", "command": "backward", "args": ["{args.0}", "-d", "5m"] }
```

This runs `/backward <the player's arg> -d 5m`. Chained `run`s make a
"stack several effects" command (see the `megapunish` example in §9).

### `random` — pick one branch

Chooses exactly one `choice` (each is a full action list) and runs only that
branch. Branches can nest.

| Field | Meaning |
|-------|---------|
| `choices` | array of `{ "actions": [ ... ] }` |

```json
{ "type": "random", "choices": [
  { "actions": [ { "type": "punish", "target": "@args", "effect": "uwu" } ] },
  { "actions": [ { "type": "punish", "target": "@args", "effect": "drunk" } ] }
] }
```

### `text` — your own data-driven effect (the `/shoe` action)

Invent a message transform with no Go. For every IC message the target sends,
the ordered `replace` rules run, then (with probability `chance`) the whole
message is swapped for a random pick from `random`.

| Field | Meaning |
|-------|---------|
| `target` | who |
| `duration` | how long; default **10m**, capped at **24h** |
| `replace` | ordered `{ "from", "to" }` find/replace rules |
| `random` | pool of whole-message replacements |
| `chance` | probability of the whole-message swap; default **0.25** |

At least one of `replace`/`random` is required.

```json
{ "type": "text", "target": "@args", "duration": "10m",
  "replace": [{ "from": ".", "to": "," }],
  "random": ["thas tuff", "lowk tuff", "tuff"], "chance": 0.25 }
```

### `wait` — pause before the next actions

Delays the **remaining** actions by `duration`. (If the caller disconnects in
the meantime, the rest are cancelled.)

| Field | Meaning |
|-------|---------|
| `duration` | how long to wait |

```json
{ "type": "wait", "duration": "3s" }
```

### `grant` / `revoke` — console-level command grants

Give (or take away) the right to run a specific command for one account, right
from inside a command. **Powerful — gate these behind `reqPerms: "ADMIN"`.**

| Field | Meaning |
|-------|---------|
| `target` | the account username (may use templating) |
| `command` | the command name to grant/revoke |

```json
{ "type": "grant", "target": "{args.0}", "command": "ban" }
{ "type": "revoke", "target": "{args.0}", "command": "ban" }
```

## 6. Targets & templating

### Target selectors

`target` accepts one of these values (or a comma-separated list of UIDs):

| Selector | Who it resolves to |
|----------|-------------------|
| `@self` (or empty) | The caller themself |
| `@args` | The UID(s) in the first argument — `/cmd 7` → player 7; `/cmd 5,7,9` → all three |
| `@global` | Non-staff players in the caller's area (excludes the caller) |
| `@area` | Everyone in the caller's area (excludes the caller) |
| `@all` | Everyone on the server (excludes the caller) |
| `5,7` | Literal UID list |

`@args` is the workhorse for "aim this at a player": set `minArgs: 1` and
`usage: "/cmd <uid>"`, then use `target: "@args"`.

### Placeholders

Inside `text`, `reason`, and `run` `args`, these are expanded:

| Placeholder | Expands to |
|-------------|------------|
| `{args.N}` | The Nth argument (0-based). `{args.0}` is the first word after the command. |
| `{caller.uid}` | UID of the player running the command |
| `{caller.name}` | Display name of the player running the command |
| `{target.uid}` | UID of the current target — **`message` action only** |
| `{target.name}` | Display name of the current target — **`message` action only** |

`{target.*}` only has a value in the `message` action (which iterates targets
one at a time). In `announce`, `run`, etc. there is no single target, so those
placeholders come out empty — use `{caller.*}` and `{args.N}` there.

Example combining both:

```json
{ "type": "message", "target": "@args",
  "text": "{target.name}, {caller.name} has challenged you to a duel!" }
```

## 7. Permissions & the account gate

### `reqPerms`

Set the minimum permission a player needs to run the command. These are the
exact keys the server knows:

```
NONE  CM  KICK  BAN  BYPASS_LOCK  MOD_EVI  MODIFY_AREA  MOVE_USERS
MOD_SPEAK  BAN_INFO  MOD_CHAT  MUTE  LOG  DJ  SHADOW  ADMIN
```

`NONE` means everyone. `ADMIN` means admins only. The others map to the
matching staff role/permission bit. A custom command is gated through the
*same* permission check as built-in commands, so role bits and console grants
(`grantcmd`) both apply normally.

### `account` — lock to a single account

Set `account` to a username and the command becomes **exclusively** that
account's, regardless of role or grants — anyone else gets "You do not have
permission to use that command."

```json
{ "name": "bigredbutton", "account": "theowner", "reqPerms": "NONE",
  "usage": "/bigredbutton", "actions": [ { "type": "announce", "text": "DO NOT PRESS." } ] }
```

The match is case-insensitive. Leave it empty (`""`) for normal role-based
permissions.

To hand a command to an account *without* touching its role, use the console
`grantcmd <username> <command>` — or pick **10** in the builder menu, which
prompts for the username and command. (Revoke with `revokecmd <username>
<command>`, or list everything with `grants`.)

## 8. Punishment effects reference

The `punish` action's `effect` field takes any of the server's named effects
(the same names `/punish <effect>` accepts). Here is the full set at the time
of writing, grouped for browsing:

**Message transforms** — `whisper`, `backward`, `stutterstep`, `elongate`,
`uppercase`, `lowercase`, `robotic`, `alternating`, `fancy`, `uwu`, `pirate`,
`shakespearean`, `caveman`, `emoji`, `invisible`, `slowpoke`, `fastspammer`,
`pause`, `lag`, `subtitles`, `censor`, `confused`, `paranoid`, `drunk`, `hiccup`,
`whistle`, `mumble`, `spaghetti`, `rng`, `essay`, `haiku`, `autospell`, `monkey`,
`snake`, `dog`, `cat`, `bird`, `cow`, `frog`, `duck`, `horse`, `lion`, `zoo`,
`bunny`, `emoticon`, `thesaurusoverload`, `valleygirl`, `babytalk`,
`thirdperson`, `unreliablenarrator`, `uncannyvalley`, `philosopher`, `poet`,
`upsidedown`, `sarcasm`, `academic`, `recipe`, `quote`, `translator`, `timewarp`,
`morse`, `rickroll`, `vowelhell`, `chef`, `karen`, `passiveaggressive`,
`nervous`, `dreamsequence`, `pickup`, `brainrot`, `gordonramsay`, `cherri`,
`albhed`, `clown`, `jester`, `joker`, `mime`, `biblebot`, `zalgo`, `leetspeak`,
`smallcaps`, `piglatin`, `vaporwave`, `lisp`, `spoonerism`, `keysmash`, `weeb`,
`politician`, `clickbait`, `markov`, `alliteration`, `cipher`, `shakecurse`,
`randomflip`, `medieval`, `cheese`, `sfxcurse`, `fromsoftware`, `51`.

**Personality ("dere")** — `tsundere`, `yandere`, `kuudere`, `dandere`,
`deredere`, `himedere`, `kamidere`, `undere`, `bakadere`, `mayadere`, `smugdere`,
`deretsun`, `bokodere`, `thugdere`, `teasedere`, `dorodere`, `hinedere`,
`hajidere`, `rindere`, `utsudere`, `darudere`, `butsudere`, `sdere`, `mdere`,
`tsuyodere`, `omnidere`.

**Voice** — `voicemute`, `voicestatic`, `voicegarble`, `voicecutout`,
`voicestutter`.

**Display / movement / misc** — `spotlight`, `hidedisplay`, `forcedisplay`,
`shrink`, `grow`, `wide`, `teleport`, `forcecolor`, `nopreanim`, `forcepreanim`,
`grounded`, `icwarp`, `roulette`, `torment`, `lovebomb`, `degrade`,
`tourettes`, `slang`, `megamaso`, `lifo`, `contagious`, `minefield`,
`stealthmute`, `trex`, `fish`.

> The authoritative, always-current list is what the wizard offers when you
> pick the `punish` action — `customcmd` → `1` → `punish` → it prints the names.
> The `/shoe`-style custom effect has an internal name of `texteffect`, but it
> is authored through the `text` action, not via `punish`.

## 9. Full worked examples

Each of these ships in `config_sample/custom_commands/` — copy them in and
reload to try them.

### `silence` — mute + effect + notify, in order

```json
{
  "name": "silence",
  "desc": "Mute + whisper + notify a player.",
  "category": "moderation",
  "reqPerms": "MUTE",
  "usage": "/silence <uid>",
  "minArgs": 1,
  "publicHelp": true,
  "actions": [
    { "type": "mute",     "target": "@args", "scope": "ic", "duration": "10m", "reason": "being too loud" },
    { "type": "punish",   "target": "@args", "effect": "whisper", "duration": "10m" },
    { "type": "message",  "target": "@args", "text": "You have been silenced by {caller.name}." }
  ]
}
```

Three actions, top to bottom: mute IC for 10m, apply the `whisper` effect for
10m, then tell the target who did it.

### `megapunish` — stack effects via `run`

```json
{
  "name": "megapunish",
  "desc": "Stack several punishments via run.",
  "category": "punishment",
  "reqPerms": "ADMIN",
  "usage": "/megapunish <uid>",
  "minArgs": 1,
  "publicHelp": false,
  "actions": [
    { "type": "run", "command": "backward", "args": ["{args.0}", "-d", "5m"] },
    { "type": "run", "command": "uwu",      "args": ["{args.0}", "-d", "5m"] },
    { "type": "run", "command": "tsundere", "args": ["{args.0}", "-d", "5m"] }
  ]
}
```

Three built-in commands chained, each handed the target UID via `{args.0}`.

### `rouletteparty` — one random punishment

```json
{
  "name": "rouletteparty",
  "desc": "Random punishment for the target.",
  "category": "punishment",
  "reqPerms": "MUTE",
  "usage": "/rouletteparty <uid>",
  "minArgs": 1,
  "publicHelp": true,
  "actions": [
    { "type": "random", "choices": [
      { "actions": [ { "type": "punish", "target": "@args", "effect": "uwu",      "duration": "5m" } ] },
      { "actions": [ { "type": "punish", "target": "@args", "effect": "backward", "duration": "5m" } ] },
      { "actions": [ { "type": "punish", "target": "@args", "effect": "drunk",    "duration": "5m" } ] }
    ] }
  ]
}
```

### `cleanup` — clear everything

```json
{
  "name": "cleanup",
  "desc": "Lift every effect and unmute.",
  "category": "moderation",
  "reqPerms": "MUTE",
  "usage": "/cleanup <uid>",
  "minArgs": 1,
  "publicHelp": true,
  "actions": [
    { "type": "unpunish", "target": "@args", "effect": "all" },
    { "type": "unmute",   "target": "@args" },
    { "type": "message",  "target": "@args", "text": "You have been cleaned up by {caller.name}." }
  ]
}
```

---

## 10. Authoring workflow & testing

From the console (stdin):

```
customcmd                           open the menu
customcmd quick mute uwu 10m        build a command in one line
customcmd list                      list commands (with their category)
customcmd show <name>               print one command's JSON
customcmd test <name> 5,7           dry-run against fake players
customcmd import <file.json>        install a file (or: import paste)
customcmd validate                  check every file for errors
customcmd reload                    re-read all JSON files from disk
customcmd delete <name>             remove a command
customcmd example                   list example templates
```

**Test before you ship.** `customcmd test <name> <uid>` runs the command against
a fake player and prints exactly what would happen (effects, targets, durations)
without touching the live server. Do this after every edit.

**Editing by hand:** each command is one `.json` file in
`config/custom_commands/`. Edit the file, then `customcmd reload` (or the main
`reload` / SIGHUP) — no restart needed. `customcmd validate` names the exact
file and field on any error.

---

## 11. Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `unknown action type "text"` | Your server build predates the `text` action — `git pull && go build` and restart. |
| `name "X" collides with a built-in command` | Custom names can't shadow built-ins; pick a different name. |
| `invalid JSON: invalid character ...` | The pasted/edited JSON has stray characters (e.g. a truncated `END` line). Validate the file, or use `customcmd import <file>` on a clean file. |
| `Not enough arguments.` | `minArgs` is higher than what was typed; check `usage`. |
| `run` action does nothing / "no permission" | The built-in runs **as the caller**, so the caller needs permission for it too. |
| `punish`/`text` skips a target | That player is in a punishment-safe area, or punishments are globally disabled. |
| Effect ends at 24h | `punish` and `text` durations are capped at 24h; `mute` with no duration is permanent. |
| Command not listed in `/help` | Set `publicHelp: true`. |

---

That's the whole system. Start from the `/shoe` example, change one field at a
time, and `customcmd test` after each change. Every command you write is just a
JSON file you can version-control, share, and reload without a restart.






