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
| `/forcegrouppair <uid> [uid ...]` | mod | same, no accept needed |
| `/accept` | any | join a pending group (1 → pair, 2 → triple, …) |
| `/deny` | any | decline (only you leave the group) |
| `/leavegroup` | any | leave (group shrinks) |
| disconnect | — | graceful auto-leave (group shrinks) |

Groups are keyed by **player UID** (not char id), so members may switch
characters without leaving. There is **no size limit**; a group dissolves only
when fewer than two members remain.
