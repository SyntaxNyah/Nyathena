# TimeLeap (`/tl`) — Server Extension & Client Contract

> Server: **Nyathena** (Athena fork)
> Clients: **AsyncAO** (Go) and **LemmyAO** (TypeScript / aolib-ts)
> Status: **groundwork landed** on the `timeleap` branch of all three repos.
> POC scope: save/restore of a room snapshot. See §15 for what is deliberately
> not done yet.

**Companion documents**

| Repo | Document |
|---|---|
| Nyathena (this repo) | `docs/TIMELEAP.md` |
| AsyncAO | <https://github.com/SyntaxNyah/AsyncAO/blob/timeleap/docs/TIMELEAP.md> |
| LemmyAO | <https://github.com/SyntaxNyah/LemmyAO/blob/timeleap/docs/TIMELEAP.md> |
| Background: the extension this one is modelled on | `docs/GROUP_PAIRING.md` |

---

## 0. Design principle

TimeLeap is a **Nyathena server extension**, not a first-class aolib spec change.
It uses the library's custom-packet extension point for one new **JSON-only**
header, `TL`:

* registered with `packetutil.RegisterLocal("TL", tlCodec())` (`register.go`) —
  the same call GP uses, so it stays out of aolib's inbound registry;
* **absent from `aolib/spec/`** and from the conformance vectors;
* no FantaCode form, ever. `tlCodec` returns an explicit error for the Fanta
  closures instead of leaving them nil (GP leaves them nil, which is a latent
  nil-func panic if the gate ever regresses).

**Where TL differs from GP, deliberately.** GP is gated on JSON mode alone
(`supportsGroupPair()` returns `client.jsonMode.Load()`), so it is pushed to
every JSON peer whether or not that peer advertised `grouppair`. TL requires
**both** guards:

```go
func (client *Client) supportsTimeLeap() bool {
	return client.JSONMode() && client.SupportsFeature(tlFeature) // tlFeature = "timeleap"
}
```

The second guard is not cosmetic: `Send()` bypasses the codec entirely for
FantaCode clients, so without `JSONMode()` a Fanta peer would receive a
meaningless empty `TL#%` frame. And without `SupportsFeature()` a JSON peer that
has no TL handler would have its room silently rewritten.

---

## 1. Capability — `FL` feature `timeleap` (bidirectional)

* The server advertises `timeleap` in its server→client `FL` (sent from `pktId`).
* The client advertises `timeleap` back in a **client→server FL**. aolib models
  `FL` as server→client only, so the client's advert arrives through
  `handleUnknownHeader` and is parsed positionally.
* Either wire form is accepted:

```
FL#timeleap#                    FantaCode (LemmyAO, AsyncAO)
{"$header":"FL","features":["timeleap"]}   JSON (typed clients)
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

## 2. Custom packet `TL` (server→client, JSON-only)

Sent on demand from `/tl <id>`, and as a private ack from `/tl save`. The client
replaces its snapshot wholesale on every `restore` (idempotent, exactly like GP).

```json
{
  "$header": "TL",
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
| `label` | string | free text from `/tl save`; display only |
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
| `/tl save <name>` | anyone (`NONE`) | captures the current area into a new snapshot, stores it with a 15-minute TTL, replies with the id, and sends a private `op:"save"` ack to the issuer |
| `/tl <id>` | anyone | restores `authoritative` (default) — the room is rolled back for **everyone**, and the transcript goes to TL-capable peers |
| `/tl <id> overlay` | anyone | restores `overlay` — nothing changes server-side; only TL-capable peers rewind their own view |
| `/tl list` | anyone | lists the ids still inside their TTL, with labels |

`registerTimeLeapCommand()` is called from `server.go` between `initCommands()`
and `validateCommands()`, which is the documented contract for `RegisterCommand`
(the registry panics before `initCommands`).

Permissions are `PermissionField["NONE"]` for the POC. Making restore CM/mod-only
is a one-line `reqPerms` change — see §15, item P1-3.

---

## 4. Restore modes

**`authoritative`** (default). `applyTimeLeapRoom` mutates the area and
broadcasts the first-class packets every client already understands:

* `a.SetBackground(bg)` + `broadcastToArea(a, &aolib.BN{Background: bg, Position: side})`
* `a.SetCurrentSong(name)` + `broadcastToArea(a, &aolib.MCToClient{...})`

Then `restoreTimeLeap` sends `TL` to gated clients so they can replay the
transcript. Result: every peer agrees on background and music; TL-capable peers
also rewind the conversation.

**`overlay`.** Only `restoreTimeLeap` runs. The room keeps its live state and
TL-capable peers deliberately show an older scene. This is the "divergence is
the feature" mode — useful for previewing a snapshot before committing to it.

> POC limitation: in `authoritative` mode the **transcript** still reaches only
> TL-capable peers. Broadcasting 256 replayed `MS` frames to everyone would work
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
| `log[]` | `tlLogFor(a).snapshot()` | §6 |

Not captured: pairing groups (TL never mutates pairing), permissions, OOC,
punishments, polls, casino state, HP bars, and any court/killing-game state
(D3 non-goal).

---

## 6. The transcript buffer

`tl_log.go` adds a per-area, structured IC history of **256 lines**. It exists
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
/tl <id>
  -> restoreTimeLeap
     -> supportsTimeLeap()            JSONMode && SupportsFeature("timeleap")
        -> Client.Send(&TLPacket{})   client.go
           -> packetutil.Encode(p, WireJSON)   codec.go
              -> encodeCustom -> EncodeJSON    tlCodec
                 -> ensureHeader              injects "$header":"TL"
                    -> sendCh (JSON frame)
```

`Client.Send` picks the encoding per client, so the gate above is the only thing
standing between a FantaCode peer and an empty `TL#%` frame.

---

## 8. Files and commits

| File | Role |
|---|---|
| `internal/athena/tl.go` | types, `tlCodec`, `supportsTimeLeap`, store, capture/restore, `/tl` |
| `internal/athena/tl_log.go` | structured per-area IC ring buffer |
| `internal/athena/tl_test.go` | guards, codec, capture/store, buffer cap |
| `internal/athena/register.go` | `RegisterLocal("TL", tlCodec())` beside GP |
| `internal/athena/netprotocol.go` | `"timeleap"` in the FL literal; transcript hook in `pktIC` |
| `internal/athena/server.go` | `registerTimeLeapCommand()` after `initCommands()` |
| `internal/athena/session_inbound.go` | JSON-aware FL shim (`jsonFLFeatures`) |
| `internal/athena/client.go` | `AddFeatures` (union) + deprecated `SetFeatures` |

Branch `timeleap`: `49c6811` (JSON FL adverts), `0300e07` (additive FL merge),
`506ae81` (TL), on top of `85247fa`.

---

## 9. Client contract

A client that wants TimeLeap MUST:

1. Register the `TL` header with its binding's custom-packet registry
   (`aolib.RegisterPacket` / `registerPacket`) **before** any traffic.
2. Advertise `timeleap` back to the server as a **positional** FL frame
   (`FL#timeleap#`) once the server's FL lists it. A typed/JSON FL is accepted by
   Nyathena, but the positional form works against unpatched servers too.
3. Treat `op:"save"` as a no-op for rendering (it carries no transcript).
4. On `op:"restore"`, replace local TL state wholesale and re-stage from
   `state` — background + side, music, transcript oldest-first, then the cast.
5. Never mutate server-visible state on its own: TL is a view.

A client that does none of this sees nothing at all, because of the two gates.

---

## 10. Verifying a change

```powershell
go build ./...
go test ./internal/athena -run 'FL|TL|TimeLeap' -count=1 -v
```

Expected: all pass. The full `internal/athena` suite currently reports **three
pre-existing failures** unrelated to TL — `TestAreaRenameKeepsWritingTheSameLogFile`
(Windows temp-file lock), `TestContentGatePrecedesEveryLeakInPktIC` (a stale
source-text canary that is identical on the base commit) and
`TestYouTubeDestDir` (expects a Linux path). Fixing those is P0-6 below, because
otherwise a real TL regression is indistinguishable from the noise.

Manual smoke: two LemmyAO tabs, `/tl save radio`, speak three lines, `/bg other`,
then `/tl wr-1` from either tab.

---

## 11. Known limitations (POC)

| Limitation | Impact |
|---|---|
| No persistence | restart loses snapshots and transcripts |
| Authoritative transcript reaches only TL clients | non-TL peers keep live text while the room's bg/music rewind |
| `evidence`/`testimony` captured but not applied | clients do not restore them yet |
| `area.side` = issuer's pos | a room-wide side is not stored server-side |
| `divergence` is a constant | no real "how far did we jump" metric (D3) |
| Restore is room-scoped | `/tl` uses the caller's current area only |
| `tlLogMax` is a compile-time constant | no per-server tuning |
| 15-minute TTL is hard-coded | `tlDefaultTTL` |

---

## 12. Next steps (detailed)

### P0 — before calling the groundwork "shippable"

**P0-1 Live smoke + wire capture.** Run Nyathena + one LemmyAO + one AsyncAO.
Confirm in the network log that (a) the client advert is `FL#timeleap#` and not a
JSON object, (b) `TL` arrives as JSON with `$header`, (c) a FantaCode client
receives nothing. This is the only end-to-end proof; unit tests cover the pieces,
not the handshake.

**P0-2 Authoritative transcript for every peer.** Today `authoritative` rewinds
bg/music for all, but text only for TL clients. Options: (a) replay the stored
log as `MS` to everyone, honouring ignore lists and the censor pipeline;
(b) leave it and document TL as a "spectator rewind". Decide before more code —
it changes the meaning of the feature. Acceptance: two clients, one without TL,
both end with the same visible scene.

**P0-3 Persistence.** Snapshots and transcripts are in-memory. Either document
"restart = all worldlines lost" loudly (operators will otherwise file bugs), or
persist to SQLite (the server already has a DB layer). Prefer documenting for the
POC and planning persistence with the D4 decision.

**P0-4 Close the LemmyAO test gap.** `bun` was unavailable when this landed, so
`bun test` has not run for the TL modules. Add the missing tests and wire them
into CI (the repo already has a CI workflow).

**P0-5 Apply evidence + testimony on clients.** The snapshot carries both; the
clients ignore them. Either implement (`applyEvidenceList` / `applyTestimonyState`
in LemmyAO, the `LE`/`RT` paths in AsyncAO) or drop the fields from the packet so
the schema does not promise something nobody honours.

**P0-6 Fix the three pre-existing test failures.** A red suite hides new red.
The stale canary (`censor_ordering_test.go`) explicitly says "update, not delete".

### P1 — correctness and operability

**P1-1 Permission model.** `/tl <id>` currently requires `NONE`, so any player
can rewind the room for TL-capable peers. Recommend `CM` for `authoritative` and
`NONE` for `overlay` (or split into `/tl` vs `/tlview`). Decide with the client
owners; the command registry makes this a one-line change.

**P1-2 Config knobs.** `tlDefaultTTL` and `tlLogMax` are constants. Move them to
`config.toml` (`[TimeLeap] ttl_seconds`, `max_log_lines`, `enabled`) and drop
`"timeleap"` from the FL literal when disabled — that is the documented kill
switch (no TL frames can be emitted once the capability disappears).

**P1-3 Per-area side.** `Area` has no side; snapshots currently claim the
issuer's. Options: add `Area.Side()` (touches `areas.go` + `pktIC`), or derive it
from the last speaker in the transcript (no schema change). The second is
preferred for the POC.

**P1-4 Cast fidelity.** Clients should place each member at its recorded
`offset`/`flip`, not just the last speaker. Verify against a GP group so TL and GP
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
* Measure worst-case `TL` frame size at 256 lines; assert it against
  `maxPacketBytes` in a test rather than by estimate.
* Cross-reference `docs/GROUP_PAIRING.md`: GP is the older extension and still
  lacks the FL guard; consider fixing GP the same way once TL has proven the
  pattern (a separate, small PR).
* `CHANGELOG`/release-notes entry, and a note in `NEW_FEATURES.md`.
* Re-audit the FL path for other unknown C2S headers that may arrive as JSON.



