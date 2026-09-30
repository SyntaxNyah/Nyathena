# AsyncAO → `aolib-go` Migration Roadmap

**Status:** Plan (not yet started). Companion to this repo's own `aolib-go`
adoption — see `internal/packet` (strict standard) vs `internal/athena`
(server extensions).

**Goal.** Replace AsyncAO's hand-rolled wire layer — `internal/protocol` plus the
`switch p.Header` dispatcher in `internal/courtroom/session.go` / `voice.go` —
with the canonical `github.com/AO-Underground/aolib/aolib-go`, so AsyncAO (a WebSocket
AO2 **client**) speaks the same FantaCode **and** JSON wire as Nyathena and
LemmyAO, from one typed packet model instead of hand-parsed strings.

The two repos converge over the *same* spec: `spec/` → `aolib-go` codegen →
Nyathena (server) + AsyncAO (client) both derive from it. No per-repo
re-implementation of packet shapes.

---

## 1. The canonical library (source of truth)

Authoritative repo: `https://github.com/AO-Underground/aolib` — `aolib-go/` is its
Go submodule. Local clone you work against:

```
C:\Users\arbok\Documents\GitHub\aolib\
├── spec\            JSON Schema (draft-07) — the single source of truth
│   ├── packets\schemas\*.schema.json    65 packet schemas
│   ├── types\*.schema.json               8 enum/type schemas
│   └── assets\                            shared schema fragments
├── aolib-go\        generated Go  (module github.com/AO-Underground/aolib/aolib-go, go 1.19)
└── aolib-ts\        generated TypeScript (the published aolib-ts counterpart)
```

- **Regenerate `aolib-go`:** `cd aolib-go && go run ./cmd/aolib-gen -meta ../spec -out .`
- **Schema keywords** (the codegen contract): `x-fanta-codec` (bespoke Fanta
  encode/decode for a field), `x-receiver` (direction), `x-wire-ints` (enum→legacy
  int map), `x-fanta-unescape-amp` (legacy `<and>` tolerance), `x-enum-description`.

**Two wire forms, one typed model.** Each packet struct encodes to either form
and the session auto-detects JSON on inbound; outbound is flipped per-session
with `SetJSONMode`.

| | FantaCode | JSON |
|---|---|---|
| Framing | `HEADER#a#b#…#%` (positional) | `{"$header":"MS","desk_modifier":"shown",…}` |
| Enums | legacy ints (`x-wire-ints`) | strings (`"shown"`, `"def"`) |
| Offset | `x&y` | `{"x":0,"y":0}` |
| Booleans | `"1"`/`"0"` | `true`/`false` |

> The dual-wire design is **schema-driven**: Fanta needs the schema's field
> order, JSON needs its field names. A packet **not** in the spec therefore has
> *no* Fanta positional form — only JSON (see §7).

---

## 2. aolib-go API surface (exact)

Import: `github.com/AO-Underground/aolib/aolib-go` (package `aolib`).

### Wire primitives
```go
type WireMode int
const ( WireFanta WireMode = iota; WireJSON )

func Encode(p Outgoing, mode WireMode) ([]byte, error)  // c2s/s2c-agnostic; frames + escapes
func Decode(raw []byte, mode WireMode) (any, error)     // NOTE: decodes via c2sDecoders only
func NewPacket(data string) (*Packet, error)            // raw framing: Header + Body (already-escaped)
func (p Packet) String() string                          // does NOT escape — Body assumed escaped
type Outgoing interface { Header() string; Args() []string }

// Custom packets (non-canonical headers) — registered at runtime:
type Codec struct {
    EncodeFanta func(p any) ([]string, error)  // → positional fields (no header/%, no escape)
    DecodeFanta func(args []string) (any, error)
    EncodeJSON  func(p any) (string, error)    // object text; "$header" injected if omitted
    DecodeJSON  func(raw string) (any, error)
}
func RegisterCodec(header string, c Codec)   // panics unless all four funcs are set
func EscapeFanta(string) string              // exposed for codec string fields
func UnescapeFanta(string) string
```

Escaping is `# % $ &` → `<num> <percent> <dollar> <and>` (same set as AsyncAO's
`EncodeField`/`DecodeField`; **do not re-escape** — `Encode` already escapes via
`Args()`).

### Sessions (the primary API)
```go
type SessionConfig struct {
    Send             func(wire []byte)           // REQUIRED — delivers one encoded packet
    OnMalformedFrame func(err error, wire []byte)
    OnUnknownHeader  func(header string, wire []byte)
    OnDecodeError    func(header string, err error, wire []byte)
    OnUnhandled      func(header string, packet any)
    OnHandlerError   func(header string, err error, packet any)
}

func NewServer(cfg SessionConfig) *ServerSession  // CLIENT code — remote server
func NewClient(cfg SessionConfig) *ClientSession  // SERVER code — one remote client
func (s *ServerSession) Receive(raw []byte)       // never panics
func (s *ServerSession) SetJSONMode(bool) / JSONMode() bool
func (s *ServerSession) SendCustom(header string, payload any) error        // both-wire, via RegisterCodec
func (s *ServerSession) OnCustom(header string, h func(any)) error          // codec-decoded value
```

**AsyncAO is a client**, so it uses `aolib.NewServer(...)` → `*ServerSession`
(`Send*` = client→server, `On*` = server→client). The intended usage is the
typed methods, exactly like `aolib-go/examples/client` — never `Encode` directly:

```go
server := aolib.NewServer(aolib.SessionConfig{Send: func(wire []byte) { ws.Write(wire) }})
server.OnID(func(p *aolib.IDToClient) { playerID = p.PlayerID })
server.OnSM(func(p *aolib.SM) { /* music list */ })
server.SendHI(&aolib.HI{HDID: "stub-hwid"})
server.SendCC(&aolib.CC{PlayerID: playerID, CharID: 0, CharPassword: "..."})
```

Typed per-header methods are generated (`SendHI`, `OnID`, `SendMC`, `OnMS`, …).
Dispatch runs off one registry — `c2sDecoders`/`s2cDecoders` (Fanta) and
`c2sJSON`/`s2cJSON` (JSON) in `registry_gen.go` — **no giant switch**.
`Receive` never panics; every failure routes to exactly one `SessionConfig` hook.

> **⚠ Known gap (the session surface is hand-written, not code-generated):** the
> typed `Send*`/`On*` surface is **not exhaustive**. `cmd/aolib-gen` emits only
> `packets_gen.go`, `registry_gen.go`, `enums_gen.go`, `types_gen.go` — it does
> **not** emit `session_client.go`/`session_server.go`, which are hand-written and
> stale. So the client-side `ServerSession` has **no** typed `Send*` for
> `askchaa`, `CH`, `CT`, `DE`, `EE`, `PE`, `RC`, `RD`, `RM` (and `ClientSession`
> lacks `On*` for `askchaa`, `CH`, `RC`, `RD`, `RM`). Those headers are
> still in `c2sDecoders` (so they *decode*), but their typed `Send*` methods are
> missing. The proper fix is extending `cmd/aolib-gen` to emit the typed surface
> (a library change) — do **not** call `aolib.Encode` directly, which bypasses
> the session's wire-mode handling. This matters a lot to a client — see §8.

---

## 3. AsyncAO's current packet layer (what gets replaced)

Module `github.com/SyntaxNyah/AsyncAO`, Go `1.25.0`, WebSocket
(`github.com/coder/websocket v1.8.12`). Fanta-only today.

### `internal/protocol/packet.go`
```go
type Packet struct { Header string; Fields []string }   // Fields are already-UNescaped
func NewPacket(header string, fields ...string) Packet  // build (unescaped fields)
func (p Packet) String() string                          // serialize WITH escaping: HEADER#f#f#%
func (p Packet) Field(i int) string                      // decoded field, "" if absent
func ParsePacket(message string) (Packet, error)         // requires #% suffix; unescapes fields
func EncodeField(string) string / DecodeField(string) string   // # % $ & <-> <num> <percent> <dollar> <and>
func SanitizeText(string) string
```

### `internal/protocol/ms.go`
```go
type ChatMessage struct { /* 32 positional MS fields: DeskMod, PreEmote, CharName,
    Emote, Message, Side, SFXName, EmoteMod, CharID, SFXDelay, Objection,
    CustomShout, EvidenceID, Flip, Realization, TextColor, Showname, Pair,
    SelfOffsetX/Y, Immediate, LoopingSFX, Screenshake, FrameShake, FrameRealize,
    FrameSFX, Additive, Effects, Blipname, Slide */ }
func ParseMS(fields []string, features FeatureSet, charListSize int) (*ChatMessage, error)
// MSMinimum = 15, MSMaximum = 32, plus emote/desk/shout consts, NormalizeOutgoingEmoteMod, formatPairID, …
```
Also: `features.go` (`FeatureSet`, `FeatureEffects`, `FeatureCustomObjections`,
`FeaturePrezoom`, `FeatureAuthPacket`, …), `pairing.go`, `demoms.go`, `conn.go`,
`dialerror.go`.

### `internal/courtroom/session.go` + `voice.go`
`HandlePacket(p protocol.Packet) []Event` is a ~700-line `switch p.Header`
handling incoming S2C; outgoing goes through `s.reply(protocol.NewPacket("…", …))`.
`voice.go` owns the `VS_*` state machine (comma-separated `VS_PEERS`, see §6).

---

## 4. Header coverage: canonical vs AsyncAO

Canonical `aolib-go` = **55 schemas → 48 unique headers** (10 bidirectional
headers have separate `…ToServer`/`…ToClient` schemas). The direction
registries in `registry_gen.go` enumerate them exactly:

- **client→server (18):** `askchaa`, `CC`, `CH`, `CT`, `DE`, `EE`, `HI`, `HP`,
  `ID`, `MA`, `MC`, `MS`, `PE`, `RC`, `RD`, `RM`, `RT`, `ZZ`.
- **server→client (37):** `ARUP`, `ASS`, `AUTH`, `BB`, `BD`, `BN`, `CHECK`, `CI`,
  `CT`, `CharsCheck`, `DONE`, `decryptor`, `EI`, `EM`, `FA`, `FL`, `FM`, `HP`,
  `ID`, `JD`, `KB`, `KK`, `LE`, `MC`, `MS`, `PN`, `PR`, `PU`, `PV`, `RMC`, `RT`,
  `SC`, `SI`, `SM`, `SP`, `TI`, `ZZ`.

Note `ASS`, `AUTH`, `CharsCheck`, `CHECK`, `decryptor` **are** canonical. The
whole voice family (`VS_*`) was **removed** from the spec (commit `aa8d0fb`) and
is now non-canonical (see the table below).

**AsyncAO headers that are NOT canonical** (handled today but absent from
`spec/` — must be kept as local shims or promoted into the spec):

| Header | Dir | Purpose | AsyncAO site |
|---|---|---|---|
| `SD` | s2c | position dropdown, `*`-separated | `session.go` (`set_pos_dropdown`) |
| `CASEA` | s2c | legacy case announcement (removed upstream; tsuserver ships it) | `session.go` |
| `SETCASE` | c2s | case-role subscription prefs | `session.go` `SetCasingPrefs` |
| `MU` / `UM` | s2c | mute / unmute a character | `session.go` |
| `checkconnection` | s2c | keepalive compat (no live server sends it) | `session.go` |
| `VS_*` (voice) | both | voice chat — `VS_CAPS`/`VS_PEERS`/`VS_JOIN`/`VS_LEAVE`/`VS_SPEAK`/`VS_AUDIO`/`VS_FRAME` | `voice.go` |

(`TT`, the cross-examination title, is the canonical *example* of a nonstandard
packet in aolib's README but AsyncAO does not currently use it.)

**Canonical headers AsyncAO never touches** (available, ignorable): `CI`/`EI`/`EM`
(server-side evidence), `MA`, `RMC`.

---

## 5. Mapping: old AsyncAO API → aolib-go

| AsyncAO today | aolib-go |
|---|---|
| `protocol.NewPacket("CH", "7")` → `reply` | `server.SendCH(&aolib.CH{…})` *(no typed `SendCH` today — see §2 gap)* |
| `protocol.NewPacket("CC", pid, cid, hdid)` | `server.SendCC(&aolib.CC{PlayerID, CharID, CharPassword})` |
| `protocol.NewPacket("CT", name, text)` | `server.SendCT(&aolib.CTToServer{…})` *(no typed `SendCT` today — see §2 gap)* |
| `protocol.ParsePacket(msg)` + `switch p.Header` | `server.Receive(raw)` + `OnX(h)` handlers (or type-switch on `aolib.Decode`) |
| `protocol.ParseMS(fields, features, n)` | `aolib.ParseMSToClient(body)` → `*aolib.MSToClient` |
| `p.Field(i)` / `p.Fields` | typed struct fields (`p.PlayerID`, `p.UpdateData`, …) |
| `protocol.EncodeField` / `DecodeField` | internal — `Encode`/`Args` already escape; **don't re-escape** |
| `msg.Packet(features)` (outgoing MS build) | keep local — AsyncAO's MS *send* has Nyathena/LemmyAO quirks (`NormalizeOutgoingEmoteMod`, `formatPairID`) not in aolib-go |
| `protocol.Packet{Header, Fields}.String()` (escapes) | `aolib.Packet.String()` does **not** escape — use the session's typed `SendX`, not raw framing |

The **direction split** is the big win: AsyncAO's `HandlePacket` switch collapses
into `On*` registrations, and wrong-direction sends become compile errors.

---

## 6. Voice chat (`VS_*`) is non-canonical

Voice chat is **not** part of the canonical protocol — commit `aa8d0fb` removed
the whole `VS_*` family (`VS_AUDIO`, `VS_CAPS`, `VS_FRAME`, `VS_JOIN`,
`VS_LEAVE`, `VS_PEERS`, `VS_SPEAK`) from the spec and both libs. So there is no
canonical `VS_PEERS` codec to get wrong: each server that speaks voice owns its
own `VS_*` packets as an extension.

Nyathena keeps voice as a server extension: the `VS_*` types live in
`internal/athena/voice_packets.go`, registered both-wire via
`packet.RegisterCodec` (`voice_codecs.go`). `VS_PEERS` is comma-separated
(`VS_PEERS#1,2,3#%`) — matching what AsyncAO/LemmyAO actually parse — and an
empty roster frames as `VS_PEERS#%`.

AsyncAO should treat `VS_*` exactly like `TT`/`SETCASE`/`CASEA`: a non-canonical
header it registers as a codec (or parses locally) rather than expecting it from
`aolib-go`.
---

## 7. Non-canonical packets: how to keep speaking them

aolib-go only models the canonical protocol. For the §4 non-canonical set
(`SD`, `CASEA`, `SETCASE`, `MU`/`UM`, `checkconnection`, `VS_*` voice):

1. **`RegisterCodec` — both-wire (the canonical extension point).** Register a
   `Codec` for the header with all four functions (Fanta encode/decode + JSON
   encode/decode). The session then ships and receives it via
   `SendCustom`/`OnCustom` in **whichever wire mode is active** — a custom packet
   now always has a Fanta form, not JSON alone. This is exactly how Nyathena
   registers `TT`/`SETCASE`/`CASEA`/`VS_*` (`internal/athena/codecs.go` +
   `voice_codecs.go`, `internal/athena/register.go`).

2. **Promote into `spec/`** — the "true schema" route. Add the schema (with
   `x-fanta-codec` where the field shape is non-generic), regenerate, and the
   packet becomes a *canonical* typed packet with `SendX`/`OnX`. Best for
   anything genuinely shared across Nyathena/LemmyAO/AsyncAO.

3. **Local Fanta shim** — still valid if a header must stay Fanta and you don't
   want a full codec; but `RegisterCodec` is preferred because one registration
   yields both wires.

Recommend: (1) for server-specific extensions (`TT`/`SETCASE`/`CASEA`/`VS_*`/`SD`/
`MU`/`UM`), (2) once a header is confirmed shared.

---

## 8. Phased roadmap

Each phase compiles and passes `go test ./...` before the next.

**Phase 0 — Dependency + wiring.** Add `require github.com/AO-Underground/aolib/aolib-go`
with a `replace` to the local clone (path or `file:`), `go mod tidy`. Keep
`internal/protocol` intact — it stays as the Fanta shim during transition.

**Phase 1 — Outgoing (send).** Swap `s.reply(protocol.NewPacket(...))` for typed
`Send*` where available (`CC`, `HI`, `ID`, `HP`, `MC`, `MS`, `RT`, `ZZ`). For
the §2-gap sends (`CH`, `CT`, `DE`, `EE`, `PE`, `RC`, `RD`, `RM`, `askchaa`)
use `aolib.Encode(typed, WireFanta)` + `conn.Write` — or first extend
`cmd/aolib-gen` to emit the missing `Send*`/`On*` (cleaner; one-time codegen fix).

**Phase 2 — Incoming (receive).** Route the WebSocket read path into
`server.Receive(raw)`; replace the `switch` arms with `On*` handlers
(`OnID`, `OnFL`, `OnMC`, `OnMS`, `OnBB`, …). Map `SessionConfig` hooks to the
existing event/debug lanes (`OnDecodeError` → `EventDebug`, `OnUnhandled` →
the current default arm). Voice: `VS_*` is non-canonical, so register it as
both-wire codecs (`RegisterCodec`) and route `SendCustom`/`OnCustom`, not the
canonical `On*` registry.

**Phase 3 — MS.** Incoming: use `aolib.ParseMSToClient` then convert to the
existing `ChatMessage` (or adopt `MSToClient` directly). Outgoing: keep the
current `OutgoingMS` builder (its Nyathena/LemmyAO quirks are not in aolib-go);
if it emits via `Encode`, stop hand-escaping.

**Phase 4 — Non-canonical + JSON.** Move `SD`/`CASEA`/`SETCASE`/`MU`/`UM` to
`RegisterCodec` + `SendCustom`/`OnCustom` (both-wire) or local Fanta shims per §7. Optionally adopt
`SetJSONMode(true)` if Nyathena offers JSON.

**Phase 5 — Delete `internal/protocol`.** Once no caller remains, remove the
hand-rolled `Packet`/`ParsePacket`/`ParseMS` (keep `FeatureSet` if still needed
for outgoing MS gating) and retire the switch. `go test ./...` + a live
connect-and-chat smoke test against Nyathena.

---

## 9. Gotchas / pitfalls

- **Don't double-escape.** `aolib.Encode` already escapes via `Args()`; the raw
  `aolib.Packet.String()` does **not**. Mixing them corrupts text with `#`/`%`/`$`/`&`.
- **`aolib.Decode` is c2s-only** (uses `c2sDecoders`). To decode S2C in isolation,
  use `Receive` on a `ServerSession` — not `aolib.Decode`.
- **MS send path is AsyncAO-specific.** Keep `NormalizeOutgoingEmoteMod` /
  `formatPairID`; aolib-go's `MSToClient` models the canonical 2.8/2.9 layout but
  not these server-compat normalizations.
- **FeatureSet gating stays local.** aolib-go parses `FL` but does not model
  AsyncAO's `FeatureSet` semantics (cccc_ic_support, custom_objections, effects,
  prezoom, auth_packet); keep `internal/protocol/features.go` or port it.
- **Module path.** The authoritative import is
  `github.com/AO-Underground/aolib/aolib-go` (the `aolib-go/` subdirectory of the
  `AO-Underground/aolib` supermodule). The checked-out `aolib-go/go.mod` still
  declares `module github.com/SyntaxNyah/aolib-go` — correct it (or use a
  `replace` directive) before importing.
- **`SessionConfig.Send` is a raw write hook**, not a per-packet method — wire it
  to the WebSocket `Write` once, not per packet.

---

## 10. Verification / acceptance

1. `cd aolib-go && go test ./...` (codegen determinism + conformance vectors).
2. `cd AsyncAO && go build ./... && go test ./...`.
3. Wire round-trip: AsyncAO-encoded frames decode in Nyathena (`internal/packet`)
   and back, for `HI/ID/PN/SI/SC/CharsCheck/SM/FL/DONE/CT/MS/MC/…` (plus the
   Nyathena `VS_*` voice extension, which is non-canonical).
4. Live smoke test: connect to a local Nyathena, complete the handshake, send an
   IC line, join voice, confirm `VS_PEERS` carries every peer (comma form).
5. Confirm the §4 non-canonical set still works against Nyathena.

---

## 11. File map

| Concern | File |
|---|---|
| Spec (source of truth) | `aolib\spec\packets\schemas\*.schema.json` |
| Go codegen | `aolib\aolib-go\cmd\aolib-gen\main.go` |
| Generated packets | `aolib\aolib-go\{packets_gen,registry_gen,session_client,session_server,types_gen,enums_gen}.go` |
| Wire codec / framing | `aolib\aolib-go\{codec,fanta,aopacket,session,outgoing}.go` |
| AsyncAO today | `AsyncAO\internal\protocol\{packet,ms,features,pairing,conn}.go`; `AsyncAO\internal\courtroom\{session,voice}.go` |
| Nyathena precedent | `Nyathena\internal\packet` (strict) + `Nyathena\internal\athena` (extensions) |



