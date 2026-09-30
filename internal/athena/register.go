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
	packet.RegisterDecoder("TT", func(b []string) (any, error) { return ParseTT(b) })
	packet.RegisterDecoder("SETCASE", func(b []string) (any, error) { return ParseSETCASE(b) })
	packet.RegisterDecoder("CASEA", func(b []string) (any, error) { return ParseCASEA(b) })
	// FL is bidirectional in Nyathena: the client advertises its own feature
	// list for the multi-pair handshake. Canonical aolib-meta marks it s2c only.
	packet.RegisterDecoder("FL", func(b []string) (any, error) { return packet.ParseFL(b) })
}
