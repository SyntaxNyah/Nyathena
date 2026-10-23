# Teleport (`/tp`) — Server Extension & Client Contract

> Server: **Nyathena** (Athena fork)
> Clients: **AsyncAO** (Go) and **LemmyAO** (TypeScript / aolib-ts)
> Status: **groundwork landed** on the `timeleap` branch of all three repos
> (the branch kept its original name; the feature was renamed).
> POC scope: save/restore of a room snapshot. See §12 for what is deliberately
> not done yet. §13 specifies **D-Mail (`DM`)**, a sibling extension that is
> **designed but not implemented**.

**Companion documents**

| Repo | Document |
|---|---|
| Nyathena (this repo) | `docs/TELEPORT.md` |
| AsyncAO | <https://github.com/SyntaxNyah/AsyncAO/blob/timeleap/docs/TELEPORT.md> |
| LemmyAO | <https://github.com/SyntaxNyah/LemmyAO/blob/timeleap/docs/TELEPORT.md> |
| Background: the extension this one is modelled on | `docs/GROUP_PAIRING.md` |

---

## Why this exists

**Teleport lets a room jump back to an earlier moment.** Someone saves the room as
it is — the IC log, who was standing where, the background, the music, the
evidence — and later restores it, so the scene comes back exactly as it was.

What players use it for:

* **Flashbacks.** "This is what actually happened that night" — rewind and replay
  the moment instead of re-typing it.
* **Recovery.** A scene got wiped by an area change, a bot flood, or a misclick:
  restore the last good state instead of rebuilding it by hand.
* **Branching.** Save at a decision point, play one path, restore, play another.

"1:1" is the requirement, not a slogan: a restore should reproduce the room, not
approximate it. That is why the snapshot carries a structured transcript rather
than the formatted mod log (§6).

Teleport is **not** D-Mail (§13). A D-Mail sends a message that *claims* the past
changed and plays a worldline shift; Teleport actually puts the room back. A
D-Mail can trigger a Teleport — that is the one-way bridge.

---

## 0. Design principle

Teleport is a **Nyathena server extension**, not a first-class aolib spec change.
It uses the library's custom-packet extension point for one new **JSON-only**
header, `TP`:

* registered with `packetutil.RegisterLocal("TP", tpCodec())` (`register.go`) —
  the same call GP (GroupPair) uses, so it stays out of aolib's inbound registry;
* **absent from `aolib/spec/`** and from the conformance vectors;
* no FantaCode form, ever. `tpCodec` returns an explicit error for the Fanta
  closures instead of leaving them nil (GP leaves them nil, which is a latent
  nil-func panic if the gate ever regresses).

**Where TP differs from GP, deliberately.** GP is gated on JSON mode alone
(`supportsGroupPair()` returns `client.jsonMode.Load()`), so it is pushed to
every JSON peer whether or not that peer advertised `grouppair`. TP requires
**both** guards:

```go
func (client *Client) supportsTeleport() bool {
	return client.JSONMode() && client.SupportsFeature(tpFeature) // tpFeature = "teleport"
}
```

The second guard is not cosmetic: `Send()` bypasses the codec entirely for
FantaCode clients, so without `JSONMode()` a Fanta peer would receive a
meaningless empty `TP#%` frame. And without `SupportsFeature()` a JSON peer that
has no TP handler would have its room silently rewritten.

---

## 1. Capability — `FL` feature `teleport` (bidirectional)

* The server advertises `teleport` in its server→client `FL` (sent from `pktId`).
* The client advertises `teleport` back in a **client→server FL**. aolib models
  `FL` as server→client only, so the client's advert arrives through
  `handleUnknownHeader` and is parsed positionally.
* Either wire form is accepted:

```
FL#teleport#                    FantaCode (LemmyAO, AsyncAO)
{"$header":"FL","features":["teleport"]}   JSON (typed clients)
```

`handleUnknownHeader` decodes the JSON form **before** trying `aolib.NewPacket`.
That ordering matters: `aolib.NewPacket` is a `#`-splitter and returns a nil body
*without an error* for a JSON frame, which used to call `SetFeatures(nil)` and
clear the feature set.

Adverts are **additive**: `AddFeatures` unions into the client's set and treats
an empty frame as a no-op, so a peer that sends one frame per feature (LemmyAO)
cannot erase a feature advertised in an earlier frame. `SetFeatures` still
exists for tests but is deprecated.

---

## 2. Custom packet `TP` (server→client, JSON-only)

Sent on demand from `/tp <id>`, and as a private ack from `/tp save`. The client
replaces its snapshot wholesale on every `restore` (idempotent, exactly like GP).

```json
{
  "$header": "TP",
  "op": "restore",
  "snapshot_id": "wr-1",
  "label": "Radio Tower",
  "divergence": "1.048596",
  "issued_by": 100,
  "issued_at": 1761177600,
  "ttl_s": 900,
  "mode": "authoritative",
  "state": {
    "area": {
      "name": "Radio Tower",
      "background": "sg_rt",
      "side": "def",
      "music": { "name": "Gate of Steiner", "char_id": -1, "showname": "", "looping": true },
      "status": 1,
      "evidence": ["Divergence Meter"],
      "testimony": []
    },
    "cast": [
      { "uid": 100, "char_id": 3, "name": "okabe", "emote": "normal", "showname": "",
        "side": "def", "offset": { "x": 0, "y": 0 }, "flip": "none", "order": 0 }
    ],
    "log": [
      { "ts": 1761177500, "uid": 100, "char_id": 3, "name": "okabe", "emote": "normal",
        "showname": "", "side": "def", "text": "El Psy Kongroo.", "color": 0 }
    ]
  }
}
```

| Field | Type | Notes |
|---|---|---|
| `op` | `"restore"` \| `"save"` | `save` is a private ack to the issuer; it carries no `log` and must not re-render |
| `snapshot_id` | string | `wr-N`, minted per area, never reused |
| `label` | string | free text from `/tp save`; display only |
| `divergence` | string | presentation only (meter readout). D3 (encoding real state) is deferred |
| `issued_by` / `issued_at` | int / unix seconds | audit |
| `ttl_s` | int | 900 in the POC; `0` would mean "never expires" |
| `mode` | `"authoritative"` \| `"overlay"` | see §4 |
| `state.area.side` | string | the **issuer's** pos — `Area` stores no side of its own |
| `state.cast[]` | object | ordered by char id, `order` renumbered contiguously |
| `state.log[]` | object | oldest first, bounded (§6) |

Column types mirror aolib's JSON forms (`offset` is `{x,y}`, `flip` is
`"none"`/`"horizontal"`), so a client can feed these straight into its renderer.

---

## 3. Commands

| Command | Who | Behaviour |
|---|---|---|
| `/tp save <name>` | anyone (`NONE`) | captures the current area into a new snapshot, stores it with a 15-minute TTL, replies with the id, and sends a private `op:"save"` ack to the issuer |
| `/tp <id>` | anyone | restores `authoritative` (default) — the room is rolled back for **everyone**, and the transcript goes to TP-capable peers |
| `/tp <id> overlay` | anyone | restores `overlay` — nothing changes server-side; only TP-capable peers rewind their own view |
| `/tp list` | anyone | lists the ids still inside their TTL, with labels |

`registerTeleportCommand()` is called from `server.go` between `initCommands()`
and `validateCommands()`, which is the documented contract for `RegisterCommand`
(the registry panics before `initCommands`).

Permissions are `PermissionField["NONE"]` for the POC. Making restore CM/mod-only
is a one-line `reqPerms` change — see §15, item P1-3.

---

## 4. Restore modes

**`authoritative`** (default). `applyTeleportRoom` mutates the area and
broadcasts the first-class packets every client already understands:

* `a.SetBackground(bg)` + `broadcastToArea(a, &aolib.BN{Background: bg, Position: side})`
* `a.SetCurrentSong(name)` + `broadcastToArea(a, &aolib.MCToClient{...})`

Then `restoreTeleport` sends `TP` to gated clients so they can replay the
transcript. Result: every peer agrees on background and music; TP-capable peers
also rewind the conversation.

**`overlay`.** Only `restoreTeleport` runs. The room keeps its live state and
TP-capable peers deliberately show an older scene. This is the "divergence is
the feature" mode — useful for previewing a snapshot before committing to it.

> POC limitation: in `authoritative` mode the **transcript** still reaches only
> TP-capable peers. Broadcasting 256 replayed `MS` frames to everyone would work
> but is a deliberate follow-up (§15, P0-2) because it interacts with ignore
> lists, rate limits and the IC censor pipeline.

---

## 5. What a snapshot captures

| Snapshot field | Source | Notes |
|---|---|---|
| `area.name` | `Area.Name()` | live `/area rename` value |
| `area.background` | `Area.Background()` | |
| `area.side` | `Client.Pos()` of the issuer | `Area` has no side field |
| `area.music` | `Area.CurrentSong()` | `char_id` is `-1`, `looping` true |
| `area.status` | `Area.Status()` | enum `StatusIdle..StatusGaming` |
| `area.evidence` | `Area.Evidence()` | |
| `area.testimony` | `Area.Testimony()` | captured, but clients do not re-apply it yet |
| `cast[]` | every UID-registered client with `Area() == a` | `char_id`, emote, showname, side, `PairInfo` offset/flip |
| `log[]` | `tpLogFor(a).snapshot()` | §6 |

Not captured: pairing groups (TP never mutates pairing), permissions, OOC,
punishments, polls, casino state, HP bars, and any court/killing-game state
(D3 non-goal).

---

## 6. The transcript buffer

`tp_log.go` adds a per-area, structured IC history of **256 lines**. It exists
because neither existing history can replay a scene:

* `Area.buffer` stores *formatted* mod-log lines (`[HH:MM:SS] name: text`) on a
  24-hour rolling window — parsing them back into an `MSToClient` is lossy;
* `Area.icMessages` stores pre-punishment *text only*, keyed by IPID, for
  `/icwarp`.

Each line records `ts, uid, char_id, name, emote, showname, side, text, color`,
appended from `pktIC` **after** the delivery switch and only when the message was
not silenced/restricted/censored — i.e. what the room actually heard.

Memory: 256 lines x ~200 bytes x number of areas (≈3 MB worst case for 60 areas).
Nothing is persisted; a server restart forgets every transcript and every stored
snapshot.

---

## 7. Wire path (identical to GP)

```
/tp <id>
  -> restoreTeleport
     -> supportsTeleport()            JSONMode && SupportsFeature("teleport")
        -> Client.Send(&TPPacket{})   client.go
           -> packetutil.Encode(p, WireJSON)   codec.go
              -> encodeCustom -> EncodeJSON    tpCodec
                 -> ensureHeader              injects "$header":"TP"
                    -> sendCh (JSON frame)
```

`Client.Send` picks the encoding per client, so the gate above is the only thing
standing between a FantaCode peer and an empty `TP#%` frame.

---

## 8. Files and commits

| File | Role |
|---|---|
| `internal/athena/tp.go` | types, `tpCodec`, `supportsTeleport`, store, capture/restore, `/tp` |
| `internal/athena/tp_log.go` | structured per-area IC ring buffer |
| `internal/athena/tp_test.go` | guards, codec, capture/store, buffer cap |
| `internal/athena/register.go` | `RegisterLocal("TP", tpCodec())` beside GP |
| `internal/athena/netprotocol.go` | `"teleport"` in the FL literal; transcript hook in `pktIC` |
| `internal/athena/server.go` | `registerTeleportCommand()` after `initCommands()` |
| `internal/athena/session_inbound.go` | JSON-aware FL shim (`jsonFLFeatures`) |
| `internal/athena/client.go` | `AddFeatures` (union) + deprecated `SetFeatures` |

Branch `timeleap`: `49c6811` (JSON FL adverts), `0300e07` (additive FL merge),
`506ae81` (TP), on top of `85247fa`.

---

## 9. Client contract

A client that wants Teleport MUST:

1. Register the `TP` header with its binding's custom-packet registry
   (`aolib.RegisterPacket` / `registerPacket`) **before** any traffic.
2. Advertise `teleport` back to the server as a **positional** FL frame
   (`FL#teleport#`) once the server's FL lists it. A typed/JSON FL is accepted by
   Nyathena, but the positional form works against unpatched servers too.
3. Treat `op:"save"` as a no-op for rendering (it carries no transcript).
4. On `op:"restore"`, replace local TP state wholesale and re-stage from
   `state` — background + side, music, transcript oldest-first, then the cast.
5. Never mutate server-visible state on its own: TP is a view.

A client that does none of this sees nothing at all, because of the two gates.

---

## 10. Verifying a change

```powershell
go build ./...
go test ./internal/athena -run 'FL|TP|Teleport' -count=1 -v
```

Expected: all pass. The full `internal/athena` suite currently reports **three
pre-existing failures** unrelated to TP — `TestAreaRenameKeepsWritingTheSameLogFile`
(Windows temp-file lock), `TestContentGatePrecedesEveryLeakInPktIC` (a stale
source-text canary that is identical on the base commit) and
`TestYouTubeDestDir` (expects a Linux path). Fixing those is P0-6 below, because
otherwise a real TP regression is indistinguishable from the noise.

Manual smoke: two LemmyAO tabs, `/tp save radio`, speak three lines, `/bg other`,
then `/tp wr-1` from either tab.

---

## 11. Known limitations (POC)

| Limitation | Impact |
|---|---|
| No persistence | restart loses snapshots and transcripts |
| Authoritative transcript reaches only TP clients | non-TP peers keep live text while the room's bg/music rewind |
| `evidence`/`testimony` captured but not applied | clients do not restore them yet |
| `area.side` = issuer's pos | a room-wide side is not stored server-side |
| `divergence` is a constant | no real "how far did we jump" metric (D3) |
| Restore is room-scoped | `/tp` uses the caller's current area only |
| `tpLogMax` is a compile-time constant | no per-server tuning |
| 15-minute TTL is hard-coded | `tpDefaultTTL` |

---

## 12. Next steps (detailed)

### P0 — before calling the groundwork "shippable"

**P0-1 Live smoke + wire capture.** Run Nyathena + one LemmyAO + one AsyncAO.
Confirm in the network log that (a) the client advert is `FL#teleport#` and not a
JSON object, (b) `TP` arrives as JSON with `$header`, (c) a FantaCode client
receives nothing. This is the only end-to-end proof; unit tests cover the pieces,
not the handshake.

**P0-2 Authoritative transcript for every peer.** Today `authoritative` rewinds
bg/music for all, but text only for TP clients. Options: (a) replay the stored
log as `MS` to everyone, honouring ignore lists and the censor pipeline;
(b) leave it and document TP as a "spectator rewind". Decide before more code —
it changes the meaning of the feature. Acceptance: two clients, one without TP,
both end with the same visible scene.

**P0-3 Persistence.** Snapshots and transcripts are in-memory. Either document
"restart = all worldlines lost" loudly (operators will otherwise file bugs), or
persist to SQLite (the server already has a DB layer). Prefer documenting for the
POC and planning persistence with the D4 decision.

**P0-4 Close the LemmyAO test gap.** `bun` was unavailable when this landed, so
`bun test` has not run for the TP modules. Add the missing tests and wire them
into CI (the repo already has a CI workflow).

**P0-5 Apply evidence + testimony on clients.** The snapshot carries both; the
clients ignore them. Either implement (`applyEvidenceList` / `applyTestimonyState`
in LemmyAO, the `LE`/`RT` paths in AsyncAO) or drop the fields from the packet so
the schema does not promise something nobody honours.

**P0-6 Fix the three pre-existing test failures.** A red suite hides new red.
The stale canary (`censor_ordering_test.go`) explicitly says "update, not delete".

### P1 — correctness and operability

**P1-1 Permission model.** `/tp <id>` currently requires `NONE`, so any player
can rewind the room for TP-capable peers. Recommend `CM` for `authoritative` and
`NONE` for `overlay` (or split into `/tp` vs `/tpview`). Decide with the client
owners; the command registry makes this a one-line change.

**P1-2 Config knobs.** `tpDefaultTTL` and `tpLogMax` are constants. Move them to
`config.toml` (`[Teleport] ttl_seconds`, `max_log_lines`, `enabled`) and drop
`"teleport"` from the FL literal when disabled — that is the documented kill
switch (no TP frames can be emitted once the capability disappears).

**P1-3 Per-area side.** `Area` has no side; snapshots currently claim the
issuer's. Options: add `Area.Side()` (touches `areas.go` + `pktIC`), or derive it
from the last speaker in the transcript (no schema change). The second is
preferred for the POC.

**P1-4 Cast fidelity.** Clients should place each member at its recorded
`offset`/`flip`, not just the last speaker. Verify against a GP group so TP and GP
do not disagree about z-order.

**P1-5 Decide D2 (forward travel).** `op` is `"save" | "restore"` today. If
forward travel is ever wanted, it needs its own op and a rule for what "forward"
means when the live room has changed. Document the decision even if the answer is
"never".

**P1-6 Decide D3 (divergence).** Either define a real metric (e.g. distance
between snapshot and live state) or delete the field before clients depend on a
constant.

### P2 — polish

* Operator-supplied meter art (see the LemmyAO doc) and a licence note.
* Measure worst-case `TP` frame size at 256 lines; assert it against
  `maxPacketBytes` in a test rather than by estimate.
* Cross-reference `docs/GROUP_PAIRING.md`: GP is the older extension and still
  lacks the FL guard; consider fixing GP the same way once TP has proven the
  pattern (a separate, small PR).
* `CHANGELOG`/release-notes entry, and a note in `NEW_FEATURES.md`.
---

## 13. D-Mail (`DM`) — sibling extension

> **Status: specification only.** There is no `DM` codec, no `"dmail"` capability
> and no `/dmail` command yet. This section exists so the design is reviewable
> before it is built, and so the pieces it shares with TP (the divergence meter,
> the snapshot bridge) are not invented twice.

### 13.1 What D-Mail is

In Steins;Gate a D-Mail is a text message pushed into the past through the
PhoneWave. Changing the past shifts the **worldline**, and the worldline is
identified by its **divergence number** (e.g. `1.048596`). Only a "Reading
Steiner" holder remembers the worldline that was overwritten; everyone else
experiences déjà vu.

As a protocol feature, DM delivers a *message plus a worldline shift*, where TP
delivers a *room state*. Same infrastructure, different payload:

| | TP (implemented) | D-Mail (this section) |
|---|---|---|
| What travels | a room snapshot | a message + divergence pair |
| Effect | restore state to a past point | deliver a message "from the past", shift the worldline |
| Worldline change | optional (`divergence` is flavour) | **required** (`divergence_from` → `divergence_to`) |
| Reader | TP-capable client re-stages | DM-capable client shows a popup + meter roll |
| Memory | n/a | `reading_steiner` decides who sees the prior-worldline hint |
| Scope | one area | one area (POC) |

### 13.2 Header, capability, guards

* Header `DM`, JSON-only, server→client, registered with
  `packetutil.RegisterLocal("DM", dmCodec())` — the same call as `GP` and `TP`.
* FL feature `"dmail"`, advertised beside `"grouppair"` and `"teleport"`.
* Guard, identical discipline to TP:

```go
func (client *Client) supportsDMail() bool {
	return client.JSONMode() && client.SupportsFeature(dmFeature) // dmFeature = "dmail"
}
```

One emission gate, one private-ack gate — no other place may decide to send `DM`.

### 13.3 Packet schema

```json
{
  "$header": "DM",
  "id": "dm-0001",
  "sender_uid": 100,
  "sender_name": "Okabe",
  "sent_at": 1700000000,
  "target_time": 1699990000,
  "target_area": "lab",
  "subject": "Don't let Mayuri go out",
  "body": "She dies at 18:00. Keep her inside.",
  "divergence_from": "1.048596",
  "divergence_to": "1.130205",
  "reading_steiner": true,
  "animation": "divergence_roll",
  "snapshot_id": "wr-3",
  "mode": "overlay"
}
```

| Field | Type | Purpose |
|---|---|---|
| `id` | string | unique id, `dm-N`; also the `/dmail` handle |
| `sender_uid` / `sender_name` | int / string | who sent it |
| `sent_at` | int64 | real send time |
| `target_time` | int64 | the past moment it claims to arrive at; **must be < `sent_at`** |
| `target_area` | string | area name the message is scoped to |
| `subject` / `body` | string | the message |
| `divergence_from` / `divergence_to` | string | the worldline shift — the point of the packet |
| `reading_steiner` | bool | whether recipients should be told they remember the prior worldline |
| `animation` | `"divergence_roll"` \| `"popup_only"` \| `"none"` | client hint, never authority |
| `snapshot_id` | string, optional | **bridge**: a TP snapshot in the same area to restore when the D-Mail lands |
| `mode` | `"authoritative"` \| `"overlay"`, optional | how the bridge restores; defaults to the snapshot's mode |

`snapshot_id`/`mode` are additions to the original design sketch. Without them a
D-Mail can only *claim* the past changed; with them it can actually change it,
using the TP machinery that already exists.

### 13.4 Delivery modes

**`broadcast`** — the popup goes to every DM-gated client in `target_area`.
**`targeted`** — only the listed UIDs (max a handful; the POC uses one recipient).

Both are gated per client, so a non-DM peer in the room sees nothing at all.

### 13.5 Server pieces (file plan)

| File | Change |
|---|---|
| `internal/athena/dm.go` | **new**: `DMPacket`, `dmCodec`, `supportsDMail`, the id store (TTL like TP), `sendDMail`, `cmdDMail`, `registerDMailCommand` |
| `internal/athena/register.go` | `packetutil.RegisterLocal("DM", dmCodec())` after `TP` |
| `internal/athena/netprotocol.go` | `"dmail"` in the FL literal |
| `internal/athena/server.go` | `registerDMailCommand()` between `initCommands()` and `validateCommands()` |

Commands (CM-gated — a D-Mail rewrites the room's worldline):

```
/dmail list
/dmail send <subject> | <body> [broadcast|targeted] [<uid>]
    -- divergence_to defaults to a configured next worldline, target_time to now
/dmail send ... snapshot <wr-id>     -- also restore that snapshot when it lands
```

On send: validate `target_time < now`, resolve `target_area` (defaults to the
issuer's), mint `dm-N`, then `sendDMail` to gated clients. If `snapshot_id` is
set, look it up in the same area and drive the TP restore (`applyTeleportRoom`
for `authoritative`, nothing for `overlay`) — the DM is the cause, the TP is the
effect. Broadcast the popup **before** the restore so the client can animate the
roll while the room changes under it.

### 13.6 Divergence numbers

The number is the packet's payload and its UI. In the source material it is a
leading digit and six decimals, and the leading digit names the **attractor
field**:

| Range | Field |
|---|---|
| `0.000000`–`0.999999` | Alpha |
| `1.000000`–`1.999999` | Beta |
| `2.000000`–`2.999999` | Gamma |
| `3.000000`–`3.999999` | Delta |
| `4.000000`–`4.999999` | Epsilon |
| `5.000000`–`5.999999` | Omega |
| `1.048596` | Steins Gate (the "true" worldline) |

For the POC the value is **authored**, not computed: the sender (or a server
config default) supplies `divergence_to`. Deriving it from game state, and
enforcing attractor-field transitions (no Alpha → Omega jumps), are both
explicitly deferred — see §13.9.

Server-side validation is therefore narrow: the string must parse as
`d.dddddd`, and `divergence_from != divergence_to` (a D-Mail that changes nothing
is a bug, not a shift).

### 13.7 Reading Steiner

`reading_steiner` is a **client rendering hint**, not a server permission:

* the DM packet is identical for every recipient;
* each client decides what to do with the flag, based on its own holder state
  (a per-client preference, a role, or simply "always show the hint");
* the server does not track who is a holder in the POC, and must not: the flag
  describes the fiction, not access.

Suggested client rendering (both clients): RS holder → meter roll plus a badge
naming the prior worldline (`1.048596 → 1.130205`); non-holder → the same roll,
no prior number, one line of flavour ("you feel like you have seen this before").

### 13.8 The D-Mail ↔ TP bridge

* **Message only** — `snapshot_id` absent. Popup + roll. Nothing server-side
  changes; the worldline shifted only in the fiction.
* **Message that changes the past** — `snapshot_id` present. The server restores
  that snapshot (`mode` decides whether the room itself rolls back or only
  DM/TP-capable clients do), then sends `DM`.

Ordering matters: `DM` first, `TP` second. The popup and the roll are the
*explanation*; the restore is the *effect*. Reversing them makes the room change
before anyone is told why. On the client the two arrive as separate packets, so
the TP restore must not wipe a popup that is already on screen — see each client
doc.

### 13.9 Deferred (POC non-goals)

| Item | Decision |
|---|---|
| Global (cross-area) D-Mails | deferred — room-scoped |
| Scheduled delivery (arrives later in real time) | deferred — immediate |
| Inbox / history / replay UI | deferred — popup only |
| Computing `divergence_to` from game state | deferred — authored |
| Attractor-field enforcement | deferred — any `from`→`to` allowed |
| Per-client Reading Steiner registry | deferred — client-side hint only |
| Reply/threading between D-Mails | deferred — not modelled |

### 13.10 Tests to write when DM is built

1. `TestDMailRequiresBothGuards` — the four-case table, same shape as TP's.
2. `TestDMailRejectsFutureTargetTime` — `target_time >= sent_at` is refused.
3. `TestDMailRejectsBadDivergence` — non-numeric or `from == to` refused.
4. `TestDMailBroadcastSkipsUngatedPeers` — three recipients, only the gated two
   receive `DM`.
5. `TestDMailBridgeRestoresSnapshot` — with `snapshot_id`, the room's background
   changes and `TP` follows `DM` (assert ordering).
6. `TestDMailFantaEncodeErrors` — the codec refuses FantaCode like TP's does.

### 13.11 Where the visuals live

Nyathena renders nothing. The meter and the popup are client features:

* **LemmyAO** — `src/divergenceMeter.ts` already implements the roll
  (`animateDivergence(target, ms)`), and `stateRestore.ts` already re-stages a TP
  snapshot, so DM on the browser side is mostly a popup + a call into those.
* **AsyncAO** — the meter does not exist yet; it is listed as P1 in that client's
  doc. A DM popup also needs an overlay, which the client already has for
  notices.

Both client docs carry a D-Mail section with their own file plan.





