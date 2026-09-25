# Custom Command Builder

> **New here?** Start with the [hands-on guide](CUSTOM_COMMANDS_GUIDE.md) — it
> walks through building a command using `/shoe` as the worked example. This
> page is the compact field reference.

A console-only system for creating slash commands **at runtime** — no restart,
no `git pull`, no Go. An operator builds a command out of small "actions" using
a beginner-friendly menu (or a JSON file), tests it against a fake player, and
can grant it to any account in the database.

## Enabling

The builder is **off by default**. Enable it in `config.toml`:

```toml
enable_custom_commands = true
```

Then restart the server (or run `customcmd reload` once it is on). While
disabled, the `customcmd` console command prints a notice, no definition files
are scanned, and no custom commands are dispatched in-game.

## Opening the menu

From the server console (stdin):

```
customcmd
```

This opens a numbered menu. Press a number (or type a keyword) and hit Enter.
Press Enter on any `[default]` to accept it. Type `?` at any menu for help.

One-liners (no menu):

```
customcmd list                     list commands
customcmd show <name>              print one command's JSON
customcmd test <name> 5,7          dry-run against fake players
customcmd import <file.json>       install a file (or `customcmd import paste`)
customcmd delete <name>            remove a command
customcmd reload                   re-read all JSON files from disk
customcmd validate                  check every file for errors
customcmd example                  list example templates
customcmd quick mute uwu 10m       build a command in one line
```

Every command is a JSON file in `config/custom_commands/`. You can edit those
files by hand and run `customcmd reload` (or `reload` / SIGHUP) to pick up the
changes without restarting.

## Definition format

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
    { "type": "mute",    "target": "@args", "scope": "ic", "duration": "10m" },
    { "type": "punish",  "target": "@args", "effect": "whisper", "duration": "10m" },
    { "type": "message", "target": "@args", "text": "You have been silenced by {caller.name}." }
  ]
}
```

`reqPerms` is one of: `NONE`, `CM`, `DJ`, `MOD_SPEAK`, `MUTE`, `KICK`, `BAN`,
`MODIFY_AREA`, `MOVE_USERS`, `BYPASS_LOCK`, `BAN_INFO`, `MOD_CHAT`, `LOG`,
`SHADOW`, `ADMIN`.

`account` (optional) is a hard gate: when set, only the authenticated account
with that name (case-insensitive) can run the command, regardless of role
permissions. Pick it in the wizard with "Custom account name", or set it in JSON
as `"account": "someusername"`.

## Actions

| `type` | what it does | key fields |
|--------|--------------|------------|
| `message` | send an OOC message | `target`, `text` |
| `announce` | broadcast server-wide | `text` |
| `punish` | apply a punishment effect | `target`, `effect`, `duration`, `reason` |
| `unpunish` | remove an effect (or `"all"`) | `target`, `effect` |
| `kick` | disconnect the target(s) | `target`, `reason` |
| `ban` | ban the target(s) | `target`, `duration`, `reason` |
| `mute` / `unmute` | mute / unmute | `target`, `scope`, `duration` |
| `move` | move target(s) to an area | `target`, `area` |
| `run` | run any existing built-in command | `command`, `args` |
| `text` | a custom text effect (find/replace + random whole-message swap) | `target`, `replace`, `random`, `chance`, `duration` |
| `random` | pick one branch | `choices` |
| `wait` | pause before the next action | `duration` |
| `grant` / `revoke` | console-level command grants | `target` (account), `command` |

`scope` for mute is `ic`, `ooc`, `both`, `music`, or `jud`.
Durations use `30s`, `10m`, `1h30m` etc.

### Text effects (`text` action)

`text` applies a per-message transform to every IC message the target sends for
the duration. It has two optional parts, applied in order:

- `replace` — an ordered list of `{"from": "...", "to": "..."}` rules
  (e.g. `{"from": ".", "to": ","}` turns every period into a comma).
- `random` — a list of strings; with probability `chance` (default `0.25`) the
  whole message is replaced by a random entry from the list.

This is how operator-invented effects like `/shoe` are expressed without code:

```json
{
  "type": "text", "target": "@args", "duration": "10m",
  "replace": [{ "from": ".", "to": "," }],
  "random": ["thas tuff", "lowk tuff", "tuff"], "chance": 0.25
}
```

## Targets

`@self`, `@args` (UIDs typed after the command), `@global` (non-staff in the
area), `@area`, `@all`, or literal UIDs like `5,7`.

## Templating

Inside `text`, `reason`, and `run` args: `{args.N}`, `{caller.uid}`,
`{caller.name}`, `{target.uid}`, `{target.name}`, `{reason}`, `{duration}`.

## Granting to an account

Any custom command can be granted to one account without changing its role:

```
grantcmd  <username> <customcmd>
revokecmd <username> <customcmd>
grants    [username]
```

## Example commands

See `config_sample/custom_commands/` and `customcmd example` for `boop`,
`silence`, `megapunish`, `rouletteparty`, `cleanup`, `shoe`, and more.
