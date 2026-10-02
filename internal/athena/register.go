package athena

// Registers Nyathena's nonstandard packets into the library's decode registry.
// The library (internal/packet) is a strict aolib-meta model; anything Nyathena
// parses differently — the extended MS, the Athena-only TT, the bidirectional FL
// and the VS_* voice extensions — is wired up here via aolib.RegisterPacket
// instead of being baked into the library. SETCASE and CASEA are canonical since
// aolib 2.6.0 and use the generated session hooks directly.

import (
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

func init() {
	// MS is canonical in aolib; inbound uses the session's OnMS. Its outbound
	// shape carries Nyathena extensions (blips, additional_chars) that aolib's
	// generated MSToClient can't represent, so it stays in the local dispatch
	// table for the outbound encode path only.
	packetutil.RegisterLocal("MS", msCodec())

	// TT is a Nyathena server extension, registered as a full both-wire codec —
	// the canonical aolib-go RegisterPacket extension point — so it carries a
	// FantaCode AND a JSON form through aolib.Encode/Decode.
	packetutil.Register[*TTPacket]("TT", ttCodec())

	// Voice chat (VS_*) — removed from the canonical spec (aa8d0fb); registered
	// here as Nyathena server extensions, both-wire like TT.
	packetutil.Register[*VS_CAPS]("VS_CAPS", vsCapsCodec())
	packetutil.Register[*VS_AUDIO]("VS_AUDIO", vsAudioCodec())
	packetutil.Register[*VS_FRAME]("VS_FRAME", vsFrameCodec())
	packetutil.Register[*VS_PEERS]("VS_PEERS", vsPeersCodec())
	packetutil.Register[*VS_JOINToServer]("VS_JOIN", vsJoinCodec())
	packetutil.Register[*VS_LEAVEToServer]("VS_LEAVE", vsLeaveCodec())
	packetutil.Register[*VS_SPEAKToServer]("VS_SPEAK", vsSpeakCodec())
}
