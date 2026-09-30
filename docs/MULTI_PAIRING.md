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
a small, coordinated addition to the `MS` packet schema: 15 new named fields.

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

## 2. Feature flag (`FL`)

`FL` carries an open string array (`features`). The server adds one flag:

| flag         | meaning |
|--------------|---------|
| `multi_pair` | the `MS` may carry an `additional_chars` list |

Because `additional_chars` is an **unbounded list** (not a fixed number of
slots), a single capability flag is the right granularity — a client either
reads the list or it doesn't. The group *size* (3/4/5) is a server-side command
concern (`/triple` `/quad` `/quint`), not a wire feature.

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
3. `third_*` / `fourth_*` / `fifth_*` each render one more partner, drawn like a
   pair partner: looping idle `(a)` animation for that partner's folder/emote,
   positioned by its own `offset`, `flip` applied.
4. Partner display skips while the speaker zooms (`emote_modifier` 5/6), same as
   pair display.
5. Each partner uses its **own** remembered restyle, keyed by the partner's
   `char_id` — not the speaker's style.

> Open item: precise z-order among 3+ sprites (the JSON wire has no `^order`;
> render order = speaker, then third/fourth/fifth in block order).

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
   emits them to clients that have negotiated support. A JSON client on the old
   schema never receives a field it would reject.

Emission gating on the server: the extra fields are emitted only for JSON-mode
connections, and only when the server believes the client supports multi-pair
(see §7). FantaCode clients and non-supporting JSON clients get the standard
30-field MS with `paired_*` intact.

---

## 7. Implementation — server (Nyathena / aolib-go)

The server already has the schema-driven JSON codec split across:

| file | change |
|---|---|
| `schemas/MSBroadcast.schema.json` | add the `additional_chars` list (§3) |
| `internal/packet/mspacket.go` | add `AdditionalChars []AdditionalChar` + `JSONExtra()` |
| `internal/packet/jsoncodec.go` | `BuildJSONPacket` merges `JSONExtra()` into the JSON object |
| `internal/packet/types.go` | add a `JSONOutgoing` interface (`JSONExtra() map[string]any`) |
| `internal/athena/client.go` | `Send` builds JSON via `BuildJSONPacket` for JSON clients; `pairGroup` + software/version state; capability gate |
| `internal/athena/netprotocol.go` | add `multi_pair` to `FL`; inject group partners in `pktIC`; store `IDClient` software/version |
| `internal/athena/pairgroup.go` | `PairGroup` model + `/triple` `/quad` `/quint` `/accept` `/deny` `/pair-requests` |
| `internal/athena/commands_registry.go` | register the new commands |

`MSPacket.Args()` (the classic positional form) **stays at 30 fields** — that is
the FantaCode contract. `AdditionalChars` is carried only in `JSONExtra()`, which
`BuildJSONPacket` merges into the JSON object, so it never reaches FantaCode.

---

## 8. Implementation — AsyncAO (Go client)

AsyncAO currently speaks **FantaCode** over WebSocket (it parses positional
`#`-split MS in `internal/protocol/ms.go`). Two options, in order of preference:

1. **(Recommended, long-term)** Adopt `aolib-go` for the JSON wire — the same
   codegen-from-schema approach as `aolib-ts`. Once AsyncAO speaks JSON, it
   reads `third_*`/`fourth_*`/`fifth_*` gated on the `FL` flags, exactly like a
   TS client. Until then it keeps working over FantaCode and renders the
   standard pair (graceful).
2. **(FantaCode-only, interim)** Do nothing for now: on FantaCode, multi-pair
   fields are never present, so AsyncAO renders the standard 2-person scene.
   The feature is JSON-only by design, so there is nothing to parse.

Concrete steps for option 1:

- Add `FeatureTriplex` / `FeatureQuadplex` / `FeatureQuintuplex` to the feature
  parser (`internal/protocol/ms.go`).
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

1. **Client capability handshake.** The JSON protocol currently has no
   client→server "supported features" packet — only server→client `FL`. To gate
   emission correctly (§6.2) we need a way for the server to learn that a JSON
   client supports multi-pair. **Interim:** the server now stores the
   `IDClient` `software`/`version` and only emits `additional_chars` to clients
   whose software string is in `multiPairCapableSoftware` (empty by default —
   add `"LemmyAO"`, etc. as clients ship support). **The clean fix** is a
   proper client→server capability packet in `aolib-meta` — flag to OmniTroid.
2. **Multi-sprite z-order** (§4.2 open item).
3. Whether non-members need a roster broadcast (currently participants only).
