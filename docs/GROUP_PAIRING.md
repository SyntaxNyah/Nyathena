# Group Pairing (`/grouppair`) — Server Extension & Client Contract

> Server: **Nyathena** (Athena fork)
> Audience: **AsyncAO** (Go) and **LemmyAO** (TypeScript / aolib-ts) maintainers
> Status: **Implemented.** Supersedes the old fixed-size multi-pair (3–5) contract
> in `MULTI_PAIRING.md`.

---

## 0. Design principle

Group pairing is a **Nyathena server extension**, not a first-class aolib spec
change. It uses the library's custom-packet extension point
(`aolib.RegisterPacket` in Go, `registerPacket` in aolib-ts) for one new JSON-only
header, **`GP`**, plus the existing JSON-only `additional_chars` list on MS. The
canonical MS schema is untouched.

Ordering lives in **one place**: an ordered member list (front→back) that
**includes the speaker**. Both `GP.members` and `additional_chars` use this same
ordering.

---

## 1. Capability — `FL` feature `grouppair` (bidirectional)

- Server advertises `grouppair` in its server→client FL.
- Client advertises `grouppair` back in a client→server FL. The server only
  emits `GP` / `additional_chars` to peers that advertised `grouppair` **and**
  that are in JSON mode.

## 2. Custom packet `GP` (server→client, JSON-only)

Sent on every group change (formed / member accepted / member left / dissolved)
as an idempotent roster snapshot. Clients replace their group state on each.

```json
{
  "$header": "GP",
  "group_id": "100",
  "members": [
    { "uid": 100, "char_id": 0, "name": "Phoenix", "emote": "normal",
      "offset": { "x": 0, "y": 0 }, "flip": "none", "order": 0 },
    { "uid": 101, "char_id": 1, "name": "Maya", "emote": "normal",
      "offset": { "x": 0, "y": 0 }, "flip": "horizontal", "order": 1 }
  ]
}
```

- `members` is ordered **front→back** (`members[0]` front-most); `order` is the
  z-index. The speaker is always included at its position.
- `group_id` is the initiator's UID (string). Empty `members` means "dissolved".

## 3. MS render plane — `additional_chars` (JSON-only)

Nyathena's `msCodec()` injects `additional_chars` as the **full ordered roster
(speaker included)** for `grouppair` peers, so a group of N renders as one
ordered list. The canonical `paired_*` fields still carry the first accepted
partner for FantaCode/legacy clients.

```json
{ "charid": 0, "name": "Phoenix", "emote": "normal",
  "offset": { "x": 0, "y": 0 }, "flip": "none", "order": 0 }
```

## 4. Commands

| Command | Who | Behavior |
|---|---|---|
| `/grouppair <uid> [uid ...]` | any | initiator + UIDs form a group; each target must `/accept` |
| `/forcegrouppair <uid> [uid ...]` | mod | same, no accept needed; the moderator may list their own UID to join the group too |
| `/accept` | any | join a pending group (1 → pair, 2 → triple, …) |
| `/deny` | any | decline (only you leave the group) |
| `/leavegroup` | any | leave (group shrinks) |
| `/pairorder [<uid> <front/back/up/down>]` | any | reorder the roster (front→back); no args toggles your own front/back |
| disconnect | — | graceful auto-leave (group shrinks) |

Groups are keyed by **player UID** (not char id), so members may switch
characters without leaving. There is **no size limit**; a group dissolves only
when fewer than two members remain.

---

## 5. Rendering contract — `additional_chars` is authoritative per message

Both channels carry the **same ordered roster**, but for different purposes:

| channel | when | who receives it | purpose |
|---|---|---|---|
| `GP` | on group change (formed / accept / leave / dissolve) | group members only | group lifecycle + roster state |
| `additional_chars` | on **every** `MS` from a group member | everyone in the area | **the per-message render plane** |

Clients must render the group from `additional_chars` (present only when a group
member speaks), **not** from the `GP` roster. The roster is only for group
management (accept / leave / status). Rendering from the roster causes two
bugs:

1. a new unpaired joiner never received `GP`, so they render nothing;
2. a non-member speaks (their `MS` has no `additional_chars`), but a group
   member's client still holds the roster, so the non-member is drawn as a fake
   "4th partner".

## 6. Offset semantics

`offset` is `"x&y"`, in percent of the viewport. The FantaCode wire escapes `&`
as `<and>`, so the **stored** value is `"x<and>y"`. `parseOffset` (in
`pairgroup.go`) and `parseMSOffset` (in `ms_codec.go`) must both do
`strings.ReplaceAll(s, "<and>", "&")` **before** splitting on `&` — otherwise a
two-axis offset silently collapses to `{x:0, y:0}` and members pile up at the
default position.

## 7. `side` field

Each `GP.members[i]` and `additional_chars[i]` carries `"side"` (the member's AO
position, from `Client.Pos()`), so clients can place a member on their **own**
bench instead of the speaker's. Example member with side:

```json
{ "uid": 100, "char_id": 0, "name": "Phoenix", "emote": "normal",
  "side": "def", "offset": { "x": 0, "y": 0 }, "flip": "none", "order": 0 }
```

## 8. Gotchas / pitfalls

- **Gating is JSON-mode, not FL.** `Client.supportsGroupPair()` returns
  `client.jsonMode.Load()`; it does **not** consult the client's advertised
  `grouppair` FL. `GP` / `additional_chars` therefore flow to any JSON peer.
- **FL direction.** aolib models `FL` as server→client only (`x-receiver:
  client`); there is no typed client→server `FL`. Clients advertise their own
  support out-of-band (LemmyAO sends raw `FL#grouppair#%`; AsyncAO uses
  `NewTypedPacket(&aolib.FL{...})` + `reply`).
- **`<and>` unescape.** See §6 — forgetting it zeroes two-axis offsets.
- **Order is front→back, speaker included.** `members[0]` is front-most. The
  renderer draws the non-speaker members behind the speaker in roster order.
- **Legacy `paired_*` still carries the first partner.** The canonical pair
  fields are filled from the first accepted non-speaker member, so FantaCode /
  legacy clients still render a correct 2-person scene.

---

## 9. Pair order — `/pairorder`

The roster order (front→back, `members[0]` front-most) is user-reorderable:

| form | effect |
|---|---|
| `/pairorder` | toggle the caller front↔back |
| `/pairorder <uid> front\|back` | move that member to the front/back of the list |
| `/pairorder <uid> up\|down` | move that member one step toward the front/back |

Each successful reorder re-broadcasts `GP`, and the next `MS`'s
`additional_chars` reflects the new order automatically (it derives from
`acceptedMembers()` = `members` order). On FantaCode, group members still see
only the first partner via the `paired_*` fields; the full N-way order is a
JSON-only concept (`additional_chars` / `GP`). The classic non-group pair order
is unchanged: it rides the client-side `^0`/`^1` suffix on FantaCode, which
`/pairorder` does not touch.

---

## 10. Classic pair order — `paired_order`

The classic 2-person pair order is the aolib `MS.paired_order` field (integer
`0` = speaker in front, `1` = speaker behind). It is **first-class in aolib**:
on FantaCode it packs onto `paired_charid` as `<id>^1` (the `x-fanta-suffix-of`
codec hint), and on JSON it is a separate `paired_order` field. Nyathena's JSON
bridge (`encodeMSJSON` / `DecodeJSON` in `ms_codec.go`) preserves it in both
directions — `splitPairedID` parses the `^order` suffix for the JSON encode, and
aolib's `Args()` re-packs it on decode — so a FantaCode speaker's `^order`
reaches JSON clients and vice versa. This is distinct from `/pairorder` (§9),
which reorders the unbounded group roster.

Note: `paired_order` is live in aolib-go **v2.6.1** (Nyathena) and aolib-ts
**2.6.1** (LemmyAO, once published to npm — the local `aolib` repo's
`ts/package.json` is already at 2.6.1).
