package athena

// Registers Nyathena's nonstandard packets into the library's decode registry.
// The library (internal/packet) is a strict aolib-meta model; anything Nyathena
// parses differently — the extended MS, the Athena-only TT/SETCASE/CASEA, and
// the bidirectional FL — is wired up here instead of being baked into the
// library.

import "github.com/MangosArentLiterature/Athena/internal/packet"

func init() {
	packet.RegisterDecoder("MS", func(b []string) (any, error) { return ParseMSToServer(b), nil })
	packet.RegisterServerDecoder("MS", func(b []string) (any, error) { return ParseMSToClient(b), nil })
	// FL is bidirectional in Nyathena: the client advertises its own feature
	// list for the multi-pair handshake. Canonical aolib-meta marks it s2c only.
	packet.RegisterDecoder("FL", func(b []string) (any, error) { return packet.ParseFL(b) })

	// Nonstandard headers (TT / SETCASE / CASEA) are registered as full
	// both-wire codecs — the canonical aolib-go RegisterCodec extension point —
	// so they carry a FantaCode AND a JSON form through packet.Encode/Decode.
	packet.RegisterCodec("TT", ttCodec())
	packet.RegisterCodec("SETCASE", setcaseCodec())
	packet.RegisterCodec("CASEA", caseaCodec())
}
