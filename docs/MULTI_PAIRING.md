# Multi-Pairing (Triplex / Quadplex / Quintuplex) — Client Support Contract

> Server: **Nyathena** (Athena fork)
> Audience: **AsyncAO** (Go) and **LemmyAO** (TypeScript / aolib-ts) maintainers
> Status: **Spec + server implementation land together.** This documents the wire
> contract future clients implement against.

---

## 0. The design principle (why this is clean)

The AO JSON protocol is **schema-driven**. Each packet is a JSON Schema
(draft-07) that pins every field's **name, type and valid values** — the
canonical source is [`OmniTroid/aolib-meta`](https://github.com/OmniTroid/aolib-meta)
(`schemas/packets/*.schema.json`). Language bindings are *generated* from those
schemas:

- **`aolib-ts`** — TypeScript (LemmyAO consumes it).
- **`aolib-go`** — Go (Nyathena's `internal/packet` + vendored `schemas/` is the
  reference; it is the Go side of this pair).

The whole point is **write the schema once, codegen the rest** — no hand-written
parser per language. JSON cares about field *names* (a positional FantaCode
parser ignores them); the schema is what makes the names canonical.

Multi-pairing is therefore **not a new packet and not a FantaCode change**. It is
a small, coordinated addition to the `MS` packet schema: one list field,
`additional_chars`.

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

> JSON itself is enabled with the existing `decryptor#JSON` handshake; the
> server auto-detects a JSON client from the first `{`-prefixed packet it sends.

---

## 3. The locked field: `additional_chars`

Added to **`schemas/packets/MSBroadcast.schema.json`** (server→client) as a
single **list** field — so the server is not bound by an arbitrary max partner
count:

```jsonc
// MSBroadcast.schema.json, inside "properties", after "effect":
"additional_chars": {
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "charid": { "type": "number" },
      "name":   { "type": "string" },
      "emote":  { "type": "string" },
      "offset": {
        "type": "object",
        "properties": { "x": { "type": "number" }, "y": { "type": "number" } },
        "required": ["x", "y"],
        "additionalProperties": false
      },
      "flip": { "type": "integer", "enum": [0, 1, 2, 3] }
    },
    "required": ["charid", "name", "emote", "offset", "flip"],
    "additionalProperties": false
  },
  "default": []
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

`additional_chars` is **optional** (not in `required`, default `[]`), so a
message with no extra partners simply omits it. `additionalProperties` stays
`false` — the field is now *known*, so strict validators accept it.

> `MSRequest.schema.json` (client→server) does **not** change. Clients never
> send the extra partners; the server injects them, mirroring how
> `paired_name`/`paired_emote`/`paired_offset`/`paired_flip` are server-injected.

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

> Open item: precise z-order among 3+ sprites (the JSON wire has no `^order`;
> render order = speaker, then `additional_chars` in list order).

---

## 5. Negotiation (OOC commands — for future client UI)

Multi-pair is formed by OOC commands (`CT` server messages). No new packet.

| command | meaning |
|---|---|
| `/triple <uid> <uid>` | invite two players to a 3-way |
| `/quad <uid> <uid> <uid>` | invite three players to a 4-way |
| `/quint <uid> <uid> <uid> <uid>` | invite four players to a 5-way |
| `/accept [uid]` | accept an incoming invite |
| `/deny [uid]` | decline an invite (dissolves the pending group) |
| `/pair-requests` | list incoming invites with roster + accept status |
| `/unpair` | leave / dissolve the group |

On every accept, all members get OOC status (roster + who has accepted). When
**every** member accepts, the group locks and rendering starts on the next IC
message from any member.

**Leave semantics (decided):** any member leaving (disconnect, area change,
`/unpair`, `/deny`) **dissolves the whole group** and notifies the rest.

---

## 6. Graceful degradation & old-protocol compatibility

The two hard guarantees that make this "no bugs for old clients":

1. **FantaCode is never touched.** Classic `#`-delimited `MS` stays at 30 fields
   (plus optional `blips`). Old/desktop clients render the standard pair from
   `paired_*` and are completely unaffected.
2. **`additionalProperties: false` is respected, not fought.** Because strict
   JSON clients reject unknown fields, the extra fields are added to the
   *canonical* schema (so updated clients accept them) **and** the server only
   emits them to clients that advertised `multi_pair` in their own `FL`. A JSON
   client on the old schema never receives a field it would reject.

Emission gating on the server: the extra fields are emitted only for JSON-mode
connections whose client advertised `multi_pair` (client→server `FL`). FantaCode
clients and non-supporting JSON clients get the standard 30-field MS with
`paired_*` intact.

---

## 7. Implementation — server (Nyathena / aolib-go)

The server already has the schema-driven JSON codec split across:

| file | change |
|---|---|
| `schemas/MSBroadcast.schema.json` | add the `additional_chars` list (§3) |
| `internal/packet/mspacket.go` | add `AdditionalChars []AdditionalChar` + `JSONExtra()` |
| `internal/packet/jsoncodec.go` | `BuildJSONPacket` merges `JSONExtra()` into the JSON object |
| `internal/packet/registry.go` | direction registry (`c2sDecoders` / `s2cDecoders`) — replaces the decode `switch` |
| `internal/packet/types.go` | add a `JSONOutgoing` interface (`JSONExtra() map[string]any`) |
| `internal/athena/client.go` | `Send` builds JSON via `BuildJSONPacket` for JSON clients; `pairGroup` + client feature state; capability gate |
| `internal/athena/netprotocol.go` | add `multi_pair` to `FL`; inject group partners in `pktIC`; handle client→server `FL` (`pktFL`) |
| `internal/athena/pairgroup.go` | `PairGroup` model + `/triple` `/quad` `/quint` `/accept` `/deny` `/pair-requests` |
| `internal/athena/commands_registry.go` | register the new commands |

`MSToClient.Args()` (the classic positional form) **stays at 30 fields** — that is
the FantaCode contract. `AdditionalChars` is carried only in `JSONExtra()`, which
`BuildJSONPacket` merges into the JSON object, so it never reaches FantaCode.

### Codec + session structure (aolib-go)

The packet codec is **registry-driven, not a `switch`**:

- `internal/packet/registry.go` maps each wire header to its decoder per
  direction (`c2sDecoders` for client→server, `s2cDecoders` for server→client).
  `codec.go` looks a header up in the registry instead of dispatching by hand.
- `internal/packet/types.go` gives the client→server packets (`HI`, `IDToServer`,
  `CC`, `MCToServer`) `Header()`/`Args()` — so they implement `Outgoing` and
  can be *sent* as well as parsed — and adds `ParseIDToClient`, `ParsePV`, and
  `ParseMCToClient` for the server→client direction. Bidirectional packets are
  split by receiver: `MSToServer` / `MSToClient`, `HPToServer` / `HPToClient`,
  `RTToServer` / `RTToClient`, `ZZToServer` / `ZZToClient`, and `IDToServer` /
  `IDToClient` (each `Header()` + `Args()`, with `Parse*` per direction).

On top of that, aolib-go publishes a **typed session layer** mirroring aolib-ts:
`aolib.NewServer(cfg)` (client-side, remote server) and `aolib.NewClient(cfg)`
(server-side, remote client) return `*ServerSession` / `*ClientSession` whose
`SendX` / `OnX` methods are keyed by header-as-type — wrong-direction calls
don't compile, `Receive` never panics (failures route to `SessionConfig` hooks),
and the surface is regenerated from the schemas by `cmd/aolib-gen`. See
`aolib-go/examples/{client,server}`.

---

## 8. Implementation — AsyncAO (Go client)

AsyncAO currently speaks **FantaCode** over WebSocket (it parses positional
`#`-split MS in `internal/protocol/ms.go`). Two options, in order of preference:

1. **(Recommended, long-term)** Adopt `aolib-go` for the JSON wire — the same
   codegen-from-schema approach as `aolib-ts`. Once AsyncAO speaks JSON, it
   reads `additional_chars` gated on the `multi_pair` flag, exactly like a
   TS client. Until then it keeps working over FantaCode and renders the
   standard pair (graceful).
2. **(FantaCode-only, interim)** Do nothing for now: on FantaCode, multi-pair
   fields are never present, so AsyncAO renders the standard 2-person scene.
   The feature is JSON-only by design, so there is nothing to parse.

Concrete steps for option 1:

- Add `multi_pair` to the feature parser (`internal/protocol/ms.go`), and send
  your own `FL` advertising it (client→server).
- Extend the `MS` struct with an `additional_chars` list and parse it only
  when the `multi_pair` flag is set (mirror the `cccc_ic_support` gate).
- In `internal/courtroom/courtroom.go`, render additional partner sprites beyond
  the single `Pair` layer, each with its own offset/flip and remembered style
  (by partner `char_id`).
- Optional: a pairing panel for `/triple`…`/quint` and `/pair-requests`.

---

## 9. Implementation — LemmyAO (TypeScript / aolib-ts)

LemmyAO already consumes `aolib-ts`, which is generated from the schemas. So the
change is mostly **upstream**:

1. Land the §3 schema diff in `aolib-meta` (and regenerate `aolib-ts`).
2. Bump LemmyAO's `aolib-ts` dependency.
3. In LemmyAO's renderer, when an incoming `MS` has a non-empty
   `additional_chars` list, draw each extra partner as a looping idle sprite
   with its own `offset`/`flip` and remembered style.
4. Optionally add UI for `/triple`…`/quint` + `/accept` / `/pair-requests`.

Because `aolib-ts` is schema-generated, **no hand-written parser changes are
needed** — the decoded `MS` object simply gains the new optional fields.

---

## 10. What still needs agreeing (before this is "pinned forever")

1. **Client capability handshake — resolved with bidirectional `FL`.** The
   client sends its own `FL` (the same packet the server sends server→client)
   listing the features it supports — `multi_pair` included. The server reads it
   (`pktFL`) and only emits `additional_chars` to clients that advertised
   `multi_pair`. No new packet type, no hardcoded client list.
2. **Multi-sprite z-order** (§4.2 open item).
3. Whether non-members need a roster broadcast (currently participants only).
