# Nyathena packet architecture

This document maps the paths a packet takes through the server, with emphasis on
the AO2 handshake, IC/OOC layering, ARUP fan-out, and the anti-raid surfaces that
were consolidated in the hardening work.

No player identifiers or capture contents are reproduced here.

## 1. Transport and dispatch

```
TCP / WSS (nhooyr websocket.NetConn)
        │
        ▼
Client.HandleClient read loop
  - JSON object or FantaCode '%' frame
  - maxPacketBytes cap (64 KiB)
  - raw packet rate limit
        │
        ▼
aolib.ClientSession.Receive
        │
        ├── typed On* hooks ─────────────► pktXxx handlers
        └── OnCustom / OnUnknownHeader ──► custom codec handlers
```

`session_inbound.go` is the single wiring point. Every inbound handler receives
a reconstructed positional `aolib.Packet`, so the packet handlers stay
independent of the wire format.

## 2. AO2 handshake

| Step | Direction | Packet | Handler | Server response |
|---|---|---|---|---|
| 1 | C→S | `HI` (HDID) | `pktHdid` | hash HDID, ban check, `ID` (player 0) |
| 2 | C→S | `ID` (software/version) | `pktId` | `PN`, `FL`, `ASS`, `VS_CAPS` |
| 3 | C→S | `askchaa` | `pktResCount` | capacity checks, `joining=true`, `SI` |
| 4 | C→S | `RC` | `pktReqChar` | `SC` character list (cached per reload) |
| 5 | C→S | `RM` | `pktReqAM` | `SM` area + music list (Fanta cached; JSON now cached) |
| 6 | C→S | `RD` | `pktReqDone` | UID, area 0, `DONE`, `BN`, `VS_CAPS`, ARUP state, roster sync, join broadcast, `ID` with real UID, MOTD/captcha |
| 7 | C→S | `CC` | `pktChangeChar` | rate limit, stuck/tung checks, `ChangeCharacter` |
| 8 | C→S | `CH` | `pktPing` | ping rate limit, `CHECK` |

Notes:

- A join changes the **player count**, not CM/status/lock state. The join path
  therefore sends CM/status/lock ARUPs to the joining client only; existing
  clients already have that state.
- `ID` and `VS_CAPS` are intentionally sent twice (early handshake and after
  `RD`). Clients use the second `ID` to learn their real UID, and some WebAO
  builds only initialize the voice panel after `DONE`.
- `SC` and JSON `SM` are serialized once per reload and shared. Before this,
  every join rebuilt several thousand character entries and the full JSON music
  list.

## 3. IC pipeline (`pktIC`)

```
CheckRateLimit
  └─ kick + rate-limit spree tracking
lockdown silence (cached playtime)
CanSpeakIC (area mute/lock)
join-captcha / guard-challenge gate
idle activity touch
parse MSToServer -> MSToClient once
forced showname / iniswap / position
punishments + message length + word filter + censors
raid guard observation
delivery:
  ├─ stealthmute            -> echo to sender only
  ├─ captcha-restricted     -> restricted delivery (self + other restricted)
  ├─ LIFO                   -> deferred queue
  └─ default                -> area broadcast (ignore-list aware)
SFX curse, IC history, mechanics, area log
```

The raid guard runs **before** the delivery switch, so a verdict earned by this
message can suppress this message rather than the next one.

## 4. OOC pipeline (`pktOOC`)

```
CheckRateLimit
per-IP OOC rate limit
lockdown silence (cached playtime)
parse CTToServer
if "/command":
  join-captcha command allowlist
  ParseCommand
login-attempt guard
join-captcha answer/gate
idle activity touch
username validation
automod on name + message (nuke tier wins)
raid guard word evidence
torment / stealthmute / captcha restriction
raid guard OOC observation
gold paywall
area broadcast (or /g global)
area log
```

Commands that carry arbitrary text to other players (`/g`, `/pm`) pass through
`oocCommandAllowed`, which applies the same content, torment, stealthmute and
captcha restrictions the normal OOC path uses.

## 5. ARUP and roster fan-out

`ARUP` carries four update types:

| Type | Meaning |
|---|---|
| `0` | visible player count per area |
| `1` | area status |
| `2` | case manager per area |
| `3` | lock state per area |

A join or leave changes only the player count. The server now:

- serializes each ARUP once per wire format (`broadcastToAllOnce`);
- sends CM/status/lock to the joining client only;
- coalesces player-count ARUPs on a 200 ms ticker, so a mass kick or join wave
  produces a handful of updates instead of one full broadcast per event.

The roster sync now builds one snapshot: FantaCode clients receive concatenated
`PR`/`PU` frames in one queue entry, while JSON/WebSocket clients receive one
frame per packet because a browser parses one JSON value per message. The join
broadcast likewise uses one client-list sweep and concatenates Fanta frames per
recipient. `PU` types 1–3 (character name, showname, area ID) were the bulk of
the old per-player packet burst.

## 6. Anti-raid surfaces

| Surface | Where | Notes |
|---|---|---|
| Per-connection behavioural score | `raidguard.go` | per-signal heat; protocol signals decay at 3× the operator half-life, slur evidence at 2× |
| Cross-IPID correlation / arrival burst | `raidguard_corr.go` | shingles, echo breadth, modcall fan-out |
| Packet hooks | `raidguard_wire.go` | one translation point per packet event |
| Per-identity repeat ledger | `raidguard_ledger.go` | IPID + hashed HDID, survives reconnects |
| Join captcha + guard challenge | `joincaptcha*.go` | guard challenges work even when the join gate is off |
| Playtime cache | `playtime_cache.go` | one SQLite read per IPID per five minutes |
| Connection admission / lockdown | `server.go`, `clientlist.go` | per-device multiclient limit (`multiclient_limit`) plus optional per-IP backstop; rate limits scale with `admission_normal_scale` outside an attack; firewall and lockdown TTL |
| Message/raw/ping rate limits | `client.go`, `server.go` | per connection/IPID |
| Automod nuke | `automod_nuke.go` | deterministic content rule, not behavioural score |
| Lockdown passkeys | `lockdown_passkey.go` | staff-issued exemption |

The repeat ledger is the cross-reconnect piece the old score lacked:

- handshake rerolls are completed joins after the previous connection is gone;
  a completed session of `raid_guard_reroll_max_session` seconds clears the tally,
  so a player who reconnects after stable play starts clean;
- a post-join `askchaa` counts double;
- rapid `CC` clusters become churn flags;
- brand-new identities are banned for `raid_guard_ban_duration`, established
  identities are kicked, and playtime-exempt identities are only alerted.

A guard challenge also counts non-chat activity: a character pick, a post-join
`askchaa`, or a modcall while the challenge is pending consumes the same strike
budget as a blocked message. A script cannot avoid the gate by never speaking,
and an unanswered challenge is closed at the deadline rather than left muted.

## 7. Consolidation status

`raidGuardSystem` (`raidguard_system.go`) is the single owner of the
cross-connection anti-raid state:

- the layer-2 correlation windows, echo window, modcall window, arrival window
  and under-attack timer;
- the per-identity repeat ledger;
- the cached playtime store.

It is also the entry point for the challenge/action decisions (`challenge`,
`silence`, `enforce`, `repeatOffence`). Per-connection evidence and the pending
challenge fields stay on `Client`/`raidState` because they must die with the
socket; the decision functions operate on those values through the system.
Packet hooks remain thin calls into the system. Do not introduce an interface per
signal on the message hot path: a table of signal definitions and one concrete
`raidGuardSystem` is faster and simpler. Interfaces belong at cold boundaries
(`PlaytimeStore`, `Clock`, `AlertSink`).
