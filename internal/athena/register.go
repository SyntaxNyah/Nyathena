package athena

// Registers Nyathena's nonstandard packets into the library's decode registry.
// The library (internal/packet) is a strict aolib-meta model; anything Nyathena
// parses differently — the extended MS, the Athena-only TT/SETCASE/CASEA, and
// the bidirectional FL — is wired up here instead of being baked into the
// library.

import (
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

func init() {
	// FL is bidirectional in Nyathena: the client advertises its own feature
	// list for the multi-pair handshake. Canonical aolib-meta marks it s2c only.
	packetutil.RegisterCodec("FL", flCodec())

	// MS is Nyathena's custom in-character packet (string-typed wire fields,
	// custom-shout name, blips, and the JSON-only additional_chars). It carries
	// its own both-wire codec so its JSON form matches the canonical meta shape.
	packetutil.RegisterCodec("MS", msCodec())

	// Nonstandard headers (TT / SETCASE / CASEA) are registered as full
	// both-wire codecs — the canonical aolib-go RegisterCodec extension point —
	// so they carry a FantaCode AND a JSON form through aolib.Encode/Decode.
	packetutil.RegisterCodec("TT", ttCodec())
	packetutil.RegisterCodec("SETCASE", setcaseCodec())
	packetutil.RegisterCodec("CASEA", caseaCodec())

	// Voice chat (VS_*) — removed from the canonical spec (aa8d0fb); registered
	// here as Nyathena server extensions, both-wire like TT/SETCASE/CASEA.
	packetutil.RegisterCodec("VS_CAPS", vsCapsCodec())
	packetutil.RegisterCodec("VS_AUDIO", vsAudioCodec())
	packetutil.RegisterCodec("VS_FRAME", vsFrameCodec())
	packetutil.RegisterCodec("VS_PEERS", vsPeersCodec())
	packetutil.RegisterCodec("VS_JOIN", vsJoinCodec())
	packetutil.RegisterCodec("VS_LEAVE", vsLeaveCodec())
	packetutil.RegisterCodec("VS_SPEAK", vsSpeakCodec())
}
