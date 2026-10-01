# Nyathena → aolib-go dependency migration

**Status:** Planned (not started; the module path is unblocked at v2.3.0). This is the roadmap for replacing
Nyathena's local `internal/packet` fork with the canonical
`github.com/AO-Underground/aolib/aolib-go` module as a real `go.mod` dependency.

## Why

Nyathena currently vendors a **fork** of aolib-go's generated code as
`internal/packet` (package `github.com/MangosArentLiterature/Athena/internal/packet`).
`go.mod` has **no** aolib-go dependency. The fork diverges from the canonical
library in three ways:

1. **Schema-based JSON codec** — `internal/packet/jsoncodec.go` +
   `jsonschema.go` (positional `jsonSchema` maps) vs aolib-go's struct-marshal
   JSON (`codec.go` `encodeJSON` / `jsonDecoderFor`).
2. **String-shaped MS** — `internal/athena/ic.go` `MSToClient` (string fields)
   vs aolib-go's typed `MSToClient` (int/bool/`Offset` fields).
3. **No session layer** — Nyathena uses `Client.Send` + `PacketMap` dispatch;
   aolib-go has `NewServer`/`NewClient` + typed `Send*`/`On*`.

The goal is to depend on the canonical library and register Nyathena's
extensions through its extension points (`RegisterCodec` / `SendCustom` /
`OnCustom`), so Nyathena, AsyncAO, and LemmyAO converge on one codebase.

## Module path (resolved)

`aolib-go/go.mod` already declares `module github.com/AO-Underground/aolib/aolib-go`
(fixed at v2.3.0), so Nyathena can `require` the authoritative path directly — no
`replace` needed for the module path.

## Plan

1. ~~**Fix the module path**~~ — already `github.com/AO-Underground/aolib/aolib-go` (v2.3.0).
2. Add `require github.com/AO-Underground/aolib/aolib-go` + a `replace`
   directive (→ local clone) to Nyathena's `go.mod`.
3. Replace `internal/packet` imports with `aolib` (package rename across
   `internal/athena`).
4. Adapt MS: string-shaped `MSToClient` → the canonical typed struct
   (int/bool/`Offset`); keep Nyathena's `JSONExtra`/`additional_chars` extension.
5. Swap the schema-based JSON codec → aolib-go's struct-marshal JSON.
6. Keep Nyathena extensions via `aolib.RegisterCodec`.

## Extensions to preserve (already non-canonical)

| Extension | Where | Registration |
|---|---|---|
| MS `additional_chars` (multi-pair) | `internal/athena/ic.go` | `JSONExtra` (JSON-only) — needs a home under aolib-go |
| `TT` / `SETCASE` / `CASEA` | `internal/athena/codecs.go` | `RegisterCodec` |
| `VS_*` voice | `internal/athena/voice_packets.go` + `voice_codecs.go` | `RegisterCodec` |

## References

- `docs/ASYNCAO_MIGRATION.md` — the AsyncAO-side mirror of this migration.
- `docs/MULTI_PAIRING.md` — the multi-pair (`additional_chars`) contract.
