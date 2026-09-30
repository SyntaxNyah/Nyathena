# Multi-Pairing (Triplex / Quadplex / Quintuplex) — Server Extension & Client Contract

> Server: **Nyathena** (Athena fork)
> Audience: **AsyncAO** (Go) and **LemmyAO** (TypeScript / aolib-ts) maintainers
> Status: **Implemented in Nyathena.** This documents the wire contract clients
> implement against.

---

## 0. Design principle

The AO wire protocol has one canonical source of truth: the
[`AO-Underground/aolib`](https://github.com/AO-Underground/aolib) supermodule —
JSON Schema (draft-07) in `spec/`, generated Go in `aolib-go/`
(module `github.com/AO-Underground/aolib/aolib-go`), generated TypeScript in
`aolib-ts/`. Bindings are generated from the schemas by `aolib-go/cmd/aolib-gen`;
Nyathena consumes the generated Go as `internal/packet`.

The canonical `MS` (`spec/packets/schemas/MSToClient.schema.json`) is **strict**
(`"additionalProperties": false`) and models the standard 30-field message only —
it has **no** multi-pairing field.

Multi-pairing is therefore a **Nyathena server extension**, layered on top of the
canonical MS the same way Nyathena layers its other server extensions (blips,
custom shout name, `paired_charid` `^order` suffix — see `internal/athena/ic.go`).
It is **not a new packet, not a change to the canonical schema, and not a
FantaCode change**: it is one extra JSON field, `additional_chars`, carried by
Nyathena's `MSToClient` through the `JSONOutgoing` hook.

How that hook works: `internal/packet` defines `JSONOutgoing` (an `Outgoing`
whose `JSONExtra() map[string]any` supplies JSON-only fields). `BuildJSONPacket`
(`internal/packet/jsoncodec.go`) merges `JSONExtra()` into the JSON object, so
the extension reaches JSON clients only and never touches the FantaCode `Args()`.

---

## 1. TL;DR

- Multi-pairing lets **3 / 4 / 5** characters share one IC scene.
- It lives **only on the JSON wire** (named fields). The classic FantaCode
  (`#`-delimited) `MS` is **unchanged** and never carries the extra fields.
- The standard pair fields (`paired_charid` / `paired_name` / `paired_emote` /
  `paired_offset` / `paired_flip`) keep their meaning and always hold the
  **first** partner, so every client — JSON or FantaCode — renders at least a
  correct 2-person scene.
- Each additional partner is a typed record appended to the `MS` JSON object.
- Reading them is gated on one `FL` feature: `multi_pair`.

---

## 2. Feature flag (`FL`) — bidirectional

`FL` carries an open string array (`features`). It is **bidirectional**: the
server sends `FL` to advertise what it supports, and the **client sends its own
`FL`** to advertise what *it* supports. That is the capability handshake — no
new packet type is needed.

| flag         | meaning |
|--------------|---------|
| `multi_pair` | the `MS` may carry an `additional_chars` list |

The server sends `multi_pair` in its `FL` (so clients know it can emit the
list), and the client sends `multi_pair` in its own `FL` (so the server gates
emission on it — no hardcoded client list). Because `additional_chars` is an
**unbounded list**, a single flag is the right granularity. Group *size* (3/4/5)
is a command concern (`/triple` `/quad` `/quint`), not a wire feature.

Nyathena advertises `multi_pair` in its server→client `FL`
(`internal/athena/netprotocol.go`), and reads the client→server `FL` in `pktFL`.
Emission is gated by `Client.supportsMultiPair()` = JSON mode **and** the client
advertising `multi_pair`.

---

## 3. The extension field: `additional_chars`

`additional_chars` is **not** in the canonical schema — it is a field on
Nyathena's `MSToClient` (`internal/athena/ic.go`), carried as JSON-only extra
data via `JSONExtra()`. Its element shape:

```go
type AdditionalChar struct {
    CharID int           `json:"charid"`
    Name   string        `json:"name"`
    Emote  string        `json:"emote"`
    Offset packet.Offset `json:"offset"` // {x, y}
    Flip   int           `json:"flip"`   // 0..3
}
```

Each entry is one on-screen partner beyond the standard pair. Its fields are
exactly what a client needs to draw it:

- `charid` — which character (character-list index)
- `name`   — character folder name
- `emote`  — idle animation to play (looping `(a)<emote>`)
- `offset` — screen position `{x, y}` (percent of viewport)
- `flip`   — mirror flag (`0..3`)

The standard pair stays in `paired_charid` / `paired_name` / `paired_emote` /
`paired_offset` / `paired_flip` and always holds the **first** partner, so
FantaCode/legacy clients still render a correct 2-person scene.
`additional_chars` carries the **rest** (2nd..Nth partner).

`additional_chars` is **optional** (omitted when empty — `JSONExtra()` returns
`nil` for no partners). The canonical schema stays `"additionalProperties":
false`; the extension is merged by the server *after* the canonical object is
built, so strict validators are unaffected as long as the field is only sent to
clients that advertised `multi_pair`.

> The client→server `MSToServer` does **not** change. Clients never send the
> extra partners; the server injects them (mirroring how `paired_*` are
> server-injected).

---

## 4. Wire contract

### 4.1 Direction & atomicity

- Server → client only. The extra partners ride the same `MS` object as the
  speaker, so they are atomically tied to the message (no ordering/correlation
  problems, unlike a separate packet).

### 4.2 Rendering semantics

1. The message author is the **speaker** (normal `MS` handling).
2. `paired_*` renders the **first** partner exactly as today's pair.
3. Each `additional_chars` entry renders one more partner, drawn like a
   pair partner: looping idle `(a)` animation for that partner's folder/emote,
   positioned by its own `offset`, `flip` applied.
4. Partner display skips while the speaker zooms (`emote_modifier` 5/6), same as
   pair display.
5. Each partner uses its **own** remembered restyle, keyed by the partner's
   `char_id` — not the speaker's style.

> Open item: precise z-order among 3+ sprites. The JSON list carries no order
> field — render order = speaker, then `additional_chars` in list order. (The
> Fanta pair has a `^order` suffix on `paired_charid`, but the JSON extension
> does not; see §10.)

---

## 5. Negotiation (OOC commands)

Multi-pair is formed by OOC commands (`CT` server messages). No new packet.

| command | meaning |
|---|---|
| `/pair <uid>` | 2-person pair (the pre-existing baseline) |
| `/triple <uid> <uid>` | invite two players to a 3-way |
| `/quad <uid> <uid> <uid>` | invite three players to a 4-way |
| `/quint <uid> <uid> <uid> <uid>` | invite four players to a 5-way |
| `/accept` | accept an incoming invite |
| `/deny` | decline an invite (dissolves the pending group) |
| `/pair-requests` | list the group's roster + accept status |
| `/unpair` | leave / dissolve the group |
| `/forcepair <uid1> <uid2>` · `/forceunpair <uid>` | moderator overrides |
| `/lfp` · `/pairlist` | looking-for-pair flag + roster |

On every accept, all members get OOC status (roster + who has accepted). When
**every** member accepts, the group locks and rendering starts on the next IC
message from any member.

**Leave semantics:** any member leaving (disconnect, area change, `/unpair`,
`/deny`) **dissolves the whole group** and notifies the rest.

---

## 6. Graceful degradation & old-protocol compatibility

The two hard guarantees that make this "no bugs for old clients":

1. **FantaCode is never touched.** Classic `#`-delimited `MS` stays at 30 fields
   (plus optional `blips`). Old/desktop clients render the standard pair from
   `paired_*` and are completely unaffected.
2. **The canonical schema stays strict; the extension is gated.** Because the
   server only emits `additional_chars` to JSON clients that advertised
   `multi_pair` in their own `FL`, a client that hasn't opted in never receives
   a field it would reject. FantaCode clients and non-supporting JSON clients
   get the standard 30-field MS with `paired_*` intact.


---

## 7. Implementation — server (Nyathena)

| file | role |
|---|---|
| `internal/athena/ic.go` | `MSToClient` (30 fields + `blips` + `AdditionalChars []AdditionalChar`) with `JSONExtra()`; `MSToServer` (26 fields + blips); `ParseMSToServer` |
| `internal/packet/outgoing.go` | `Outgoing` + `JSONOutgoing` interfaces (the extension hook) |
| `internal/packet/jsoncodec.go` | `BuildJSONPacket` merges `JSONExtra()` into the JSON object |
| `internal/athena/client.go` | JSON-mode send path (`BuildJSONPacket`) + `supportsMultiPair()` gate |
| `internal/athena/netprotocol.go` | `multi_pair` in server `FL`; `pktFL` reads client→server `FL` |
| `internal/athena/pairgroup.go` | `PairGroup` model + `applyPairGroupInjection` + commands |
| `internal/athena/commands_registry.go` | registers `/pair` `/triple` `/quad` `/quint` `/accept` `/deny` `/pair-requests` `/unpair` `/forcepair` `/forceunpair` `/lfp` `/pairlist` |

`MSToClient.Args()` (the classic positional form) **stays at 30 fields** — that
is the FantaCode contract. `AdditionalChars` is carried only in `JSONExtra()`,
which `BuildJSONPacket` merges into the JSON object, so it never reaches
FantaCode. `applyPairGroupInjection` fills `paired_*` from the first partner and
appends the rest to `AdditionalChars`.

---

## 8. Implementation — AsyncAO (Go client)

AsyncAO currently speaks **FantaCode** over WebSocket. Its migration to the
canonical library is tracked in
[`docs/ASYNCAO_MIGRATION.md`](ASYNCAO_MIGRATION.md). For multi-pairing
specifically:

1. **(FantaCode, today)** Multi-pair fields are never present, so AsyncAO renders
   the standard 2-person scene. Nothing to parse — the feature is JSON-only.
2. **(JSON, per the migration roadmap)** Once AsyncAO adopts
   `github.com/AO-Underground/aolib/aolib-go` and speaks JSON, it should:
   - advertise `multi_pair` in its own client→server `FL`;
   - extend its `MS` model with an `additional_chars` list and parse it only
     when `multi_pair` is set (mirror the `cccc_ic_support` gate);
   - render additional partner sprites beyond the single pair layer, each with
     its own offset/flip and remembered style (keyed by `char_id`).

Note: because `additional_chars` is a Nyathena extension (not in the canonical
`aolib-go` schema), AsyncAO must add this field locally — it will not appear in
the generated `MSToClient`. See §10 for the promote-to-spec option.

---

## 9. Implementation — LemmyAO (TypeScript / aolib-ts)

LemmyAO consumes `aolib-ts`, generated from the canonical `AO-Underground/aolib`
spec. Because `additional_chars` is **not** in that spec, LemmyAO will **not**
get it from codegen. Two paths:

1. **(Local handler)** Add an `additional_chars` field to LemmyAO's MS model and
   render it when the server advertises `multi_pair` (and LemmyAO has advertised
   its own support). Until then, LemmyAO simply never advertises `multi_pair`, so
   it never receives the field.
2. **(Promote to spec)** Land `additional_chars` in the canonical
   `MSToClient.schema.json` and regenerate `aolib-ts`/`aolib-go`, so both
   libraries get it for free — at which point §10's open question is answered.

---

## 10. What still needs agreeing

1. **Promote-to-spec vs. stay-an-extension.** Today `additional_chars` is a
   Nyathena-only extension. If LemmyAO (or any other JSON client) should render
   multi-pair, either promote `additional_chars` into the canonical
   `AO-Underground/aolib` `MSToClient` schema (and regenerate both bindings), or
   have each client add a local handler. This is the one decision that "pins
   forever" the extension's shape.
2. **Multi-sprite z-order.** The JSON `additional_chars` list has no order field;
   render order = list order. If clients need an explicit z-order for 3+ sprites,
   add an `order` (or equivalent) field to `AdditionalChar` before it is promoted.

