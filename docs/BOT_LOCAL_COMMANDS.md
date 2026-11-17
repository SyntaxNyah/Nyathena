# Local-file IC bot commands

No command contacts an external API. The bot sends server-generated IC packets, as before; no additional player account or separate process is required.

## Configuration

Put the following under `[Server]` in the deployed `config/config.toml`:

```toml
dailyquote = true
icebreaker = true
bot_text_color = "yellow"
bot_ini_character = "hans christian andersen (fgo)"
bot_emote = "adult/normal"
```

For `/dailyquote` and `/icebreaker`, `bot_ini_character` overrides the bot sprite. If the setting is blank, the first configured roster character is used; if the roster is empty, the final fallback is `Phoenix`. The default Andersen override is defined in `settings.DefaultConfig()`. `bot_emote` is the actual animation asset name (`adult/normal`), corresponding to the requested emote labelled `19 - normal` in that character's INI. This depends on the installed character assets matching those names. The position is witness. `bot_text_color` accepts `white`, `green`, `red`, `orange`, `blue`, `yellow`, `pink`, `cyan`, `grey`, or `rainbow` and applies to both bot commands. If you prefer a different colour, edit it and use `/reload`.

## Text files

- `config/quote.txt`: `MM-DD | quotation — author`; the supplied 365 date-indexed lines are included. February 29 defaults to February 28 if missing. The date uses the server's local timezone.
- `config/icebreakers.txt`: one question per line, randomly chosen on each successful `/icebreaker`. The supplied 100 questions are included.

Blank lines and comment lines starting with `#` are ignored. The files must be UTF-8 and cannot contain a literal `#` in an IC line, because it is the AO2 packet field delimiter. Invalid files are rejected by `/reload` rather than partly published.

## Commands

- `/dailyquote` — the same quote for every request on the same date.
- `/icebreaker` — a random question from the local file, not tied to the date.

Both commands share one five-minute cooldown per room and a single server-wide limit of three simultaneous sessions total.

## Hot reload

An administrator can use `/reload` to reread the two data files and the bot configuration switches, text colour and animation settings. The new immutable file snapshot and setting snapshot are published safely for live connections. A syntax error in a new file aborts the reload without swapping in that file's data. `characters.txt` retains the repository's existing append-only reload restriction.

Copy the supplied `config_sample` files into the **existing VPS config directory** (`config/`) when deploying. Files supplied in `config_sample` are examples and are not automatically read from that directory at runtime. The maintainer should verify that the Andersen sprite and `adult/normal` animation are available to the actual WebAO and desktop clients.

## Verification

Run the repository's build and test workflows, including `go test ./...` and `go test -race ./internal/athena` when the environment supports them. Test live `/reload`, quote date changes, icebreaker output, and the selected character assets before deploying the public server.
