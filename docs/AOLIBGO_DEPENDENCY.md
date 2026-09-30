# Nyathena → aolib-go dependency migration

**Status:** Deferred (planned, not started). This is the roadmap for replacing
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

## Blocker: module path

`aolib-go/go.mod` still declares `module github.com/SyntaxNyah/aolib-go`, **not**
`github.com/AO-Underground/aolib/aolib-go`. Fix that one line before Nyathena can
`require` the authoritative path.

## Plan

1. **Fix the module path** in `aolib-go/go.mod` → `github.com/AO-Underground/aolib/aolib-go`.
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
