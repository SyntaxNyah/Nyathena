# Rich Text in OOC & the BB Popup — Client Expansion Guide

Forward-looking design note: how a **custom WebAO fork or desktop client** can add
colour / bold / italics to server messages — the OOC (`CT`) chat and the `BB`
popup that `/roommotd` and the server MOTD use — **without breaking existing
clients**.

Vanilla AO2 clients render `CT` and `BB` as plain text, so any markup a server
emits today must degrade gracefully: an unmodified client must still show clean,
readable words, not a wall of tags.

## 1. The constraint

- `CT` and `BB` are plain-text fields. There is no colour/bold field on the wire.
- The server escapes exactly four characters on the wire: `%` `#` `$` `&`
  (→ `<percent>` `<num>` `<dollar>` `<and>`). Everything else — newlines and
  control characters included — passes through untouched.
- AsyncAO (`AsyncAO/`) draws `BB` notices in a single fixed colour
  (`servernotice.go`: title `ColText`, body `ColTextDim`) and word-wraps the
  body; it does not parse markup. OOC is drawn the same way.

## 2. Backward-compatibility requirement

A vanilla client must show the **message words** with no visible markup. That
rules out printable tag syntax like `[b]…[/b]` (a vanilla client would print the
brackets) and points at a convention whose delimiters are **non-printing**.

## 3. Recommended convention: ANSI SGR control sequences

Carry the terminal-standard SGR (Select Graphic Rendition) codes inside the
`CT`/`BB` text field:

| Code (bytes)                  | Meaning              |
|-------------------------------|----------------------|
| `ESC [ 0 m`  (`\x1b[0m`)      | reset                |
| `ESC [ 1 m`  (`\x1b[1m`)      | bold                 |
| `ESC [ 3 m`  (`\x1b[3m`)      | italic               |
| `ESC [ 38 ; 2 ; R ; G ; B m`  | 24-bit colour        |
| `ESC [ 38 ; 5 ; N m`          | 256-colour (N 0–255) |

Why SGR:

- **Zero visible noise for old clients.** `ESC` (0x1B) is non-printing. A vanilla
  client that sanitises control characters shows nothing at all; one that does
  not shows at most a small glyph, while every word of the message is untouched.
- **Standard and future-proof.** It is the same encoding terminals and chat
  systems already use, so parsing is well understood.
- **It does not touch the wire framing.** `#` (field separator) and `%` (packet
  terminator) are the only framing bytes, and SGR never emits them.

### Fallback (readable-but-noisy): printable tags

If a fork prefers human-readable markup in the raw log, use a low-collision form
and accept that vanilla clients print it:

    [c=#RRGGBB]text[/c]  [b]text[/b]  [i]text[/i]

Reach for this only when the text must survive copy-paste into a plain editor,
at the cost of visible tags in old clients.

## 4. Server side (Nyathena) — no protocol change needed

- The server already stores and forwards `CT`/`BB` text verbatim (modulo the four
  wire escapes). To carry SGR, a CM or operator simply includes the sequences in
  the text; `/roommotd` already translates `\n` to a newline and could in future
  also translate a friendlier input syntax (e.g. `{red}` → SGR) before storing.
- **Censor**: run the word filter on the text with SGR sequences stripped, so
  colouring a slur cannot evade AutoMod (e.g. `ni\x1b[1mgg\x1b[0mer` must still
  match). `autoModCheckTiered`'s evasion normalisation should be extended to drop
  `ESC [ … m` runs.
- **Length caps**: count the *visible* text, not the SGR bytes, when enforcing a
  motd length cap.
- **JSON mode**: `ESC` must be emitted as `\u001b` by the JSON encoder and parsed
  back by the client; `packet.BuildJSON` / the schema validator must not reject
  control characters in those fields.

## 5. Client implementation guide

### FantaCode path

1. After decoding `%` `#` `$` `&`, scan the `CT`/`BB` message for `ESC [ … m`.
2. Split the string into `(style, text)` runs. Track the active SGR state
   (bold, italic, foreground colour); `ESC [ 0 m` resets.
3. Render each run with its style. In AsyncAO this means replacing the
   single-colour `LabelClipped` body loop in `serverNoticeBody`/`drawServerNotice`
   with per-run draws, and adding a bold/italic-capable font (or a synthetic
   embolden pass) since the chrome font may not ship bold/italic faces.
4. Sanitise *unmatched* `ESC` sequences to nothing so a stray byte cannot corrupt
   layout.

### JSON path

1. The JSON codec must allow `\u001b` in `CT`/`BB` message fields (schema update:
   do not reject control characters in those fields).
2. Parse runs the same way; the run-splitting is shared between both wire paths.

## 6. Validation checklist

- Vanilla AO2 desktop + WebAO + an unmodified AsyncAO still show the **words**
  with no `[b]`/`ESC` garbage.
- A colouring client renders colour/bold/italic and resets correctly at the end
  of the styled run.
- `autoModCheckTiered` still matches a slur that has been split by SGR codes.
- `#` `%` `$` `&` still escape correctly when mixed with SGR bytes.

## 7. Related

- `/roommotd` (server) — `internal/athena/commands_area_admin.go`, the `BB` popup.
- `escapeOutgoing` / `encode` — `internal/athena/netprotocol.go`.
- AsyncAO notice rendering — `AsyncAO/internal/ui/servernotice.go`.
