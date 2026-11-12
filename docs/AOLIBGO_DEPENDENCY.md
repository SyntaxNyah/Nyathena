# Nyathena → aolib-go dependency migration

**Status:** Done — Nyathena depends on
`github.com/AO-Underground/aolib/go/v2` (v2.4.3) as a real `go.mod`
dependency. The local `internal/packet` fork and its hand-rolled JSON
encoder/validator are removed.

## What changed

| Before | After |
|---|---|
| `internal/packet` (vendored aolib fork) | `github.com/AO-Underground/aolib/go/v2` (aliased `aolib`) |
| `internal/packet/jsoncodec.go` + `jsonschema.go` (positional table-driven JSON + `jsonschema/v5` validation) | aolib-go's struct-marshal JSON + built-in `jsonschema/v6` validation on `Encode`/`Decode` |
| `schemas/` (vendored MSRequest/MSBroadcast) | removed — aolib validates against its own schemas |
| string-shaped `MSToClient` (numeric/offset fields as strings) | kept as a custom codec (`ms_codec.go`) that bridges to aolib's typed `MSToClient` |

## How the pieces map now

- **Canonical packets** — typed `aolib.*` structs; JSON via `aolib.Encode(p, WireJSON)` (enum names, JSON numbers, booleans, `{x,y}` offsets). FantaCode via `aolib.Encode(p, WireFanta)` / `aolib.NewPacket`.
- **`internal/packetutil`** — re-exposes the small set of things aolib keeps unexported: wire-int enum maps (`DeskModifierFromWire`, `TextColorToWire`, `PenaltyBarToWire`, …), FantaCode helpers (`Itoa`, `AtoiOrZero`, `BoolToWire`, …), and the custom-packet codec dispatch (`RegisterCodec` / `Encode` / `Decode` / `DecodeToBody` / `BuildJSONFromArgs`) plus an s2c parser registry for the type-erased `SendPacket(header, args…)` path.
- **Extensions** — registered via the canonical `aolib.RegisterCodec` both-wire point: `FL` (bidirectional), `MS` (typed JSON bridge + `additional_chars`), `TT`, `SETCASE`, `CASEA`, and the `VS_*` voice packets.

## Extensions preserved

| Extension | Where | Registration |
|---|---|---|
| MS `additional_chars` (multi-pair) | `internal/athena/ms_codec.go` | `msCodec()` — `EncodeJSON` injects `additional_chars` |
| `TT` / `SETCASE` / `CASEA` | `internal/athena/codecs.go` | `RegisterCodec` |
| `VS_*` voice | `internal/athena/voice_codecs.go` | `RegisterCodec` |
| `FL` (bidirectional) | `internal/athena/register.go` | `RegisterCodec` |

## References

- `docs/ASYNCAO_MIGRATION.md` — the AsyncAO-side mirror of this migration.
- `docs/MULTI_PAIRING.md` — the multi-pair (`additional_chars`) contract.
