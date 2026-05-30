# AO2 packet JSON schemas

JSON-Schema (draft-07) definitions for the Attorney Online 2 wire protocol in
its JSON encoding. Each file describes one packet header; headers that differ
by direction have separate `…Request` (client→server) and `…Broadcast`
(server→client) schemas.

## Provenance

Vendored from [OmniTroid/aolib-schemas](https://github.com/OmniTroid/aolib-schemas).
They document the same field names as the upstream AO2
`docs/Development/network/Packet Reference.md`, but pin the **types** —
`char_id` is a number, `realization` is a boolean, `offset` is an `{x, y}`
object, `side` is a fixed enum, and so on. This is the "no type nonsense"
contract for the JSON wire format added alongside FantaCode.

## What Nyathena enforces

Validation is wired for the **MS** (in-character) packet only:

| File | Direction | Enforced in |
|------|-----------|-------------|
| `MSRequest.schema.json` | inbound (client→server) | `packet.ParseJSON` — an invalid MS is rejected and dropped (logged) |
| `MSBroadcast.schema.json` | outbound (server→client) | `Client.SendPacket` — an MS that fails the schema is dropped (logged) before reaching a JSON-mode client |

Both schemas are embedded into the binary (`//go:embed` in `athena.go`) and
compiled once at startup via `packet.CompileMSSchemas`. Validation only applies
to **JSON-mode** connections; FantaCode (classic desktop AO2) traffic is never
validated and is unaffected. If the schemas fail to load, validation is
silently disabled and the server behaves exactly as before.

The remaining schema files are vendored for reference and to make extending
validation to other packets straightforward.
