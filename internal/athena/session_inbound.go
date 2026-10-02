package athena

import (
	"strings"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// handle gates a reconstructed positional packet on the mustJoin check and
// hands it to the existing pktXxx handler. p is any typed packet (canonical or
// custom codec) whose Args() yields the positional body the handlers expect.
func (client *Client) handle(mustJoin bool, fn func(*Client, *aolib.Packet), p aolib.Outgoing) {
	if mustJoin && client.Uid() == -1 {
		return
	}
	fn(client, &aolib.Packet{Body: p.Args()})
}

// registerInbound wires the aolib session's typed On*/OnCustom hooks so every
// inbound packet reaches its existing pktXxx handler through a reconstructed
// positional body. Canonical headers use the typed .on() helpers; Nyathena's
// custom headers (MS, FL, TT, SETCASE, CASEA, VS_*) use OnCustom.
func (client *Client) registerInbound() {
	s := client.sess

	// Canonical client→server headers.
	s.OnHI(func(p *aolib.HI) { client.handle(false, pktHdid, p) })
	s.OnID(func(p *aolib.IDToServer) { client.handle(false, pktId, p) })
	s.OnAskchaa(func(p *aolib.Askchaa) { client.handle(false, pktResCount, p) })
	s.OnRC(func(p *aolib.RC) { client.handle(false, pktReqChar, p) })
	s.OnRM(func(p *aolib.RM) { client.handle(false, pktReqAM, p) })
	s.OnRD(func(p *aolib.RD) { client.handle(false, pktReqDone, p) })
	s.OnCC(func(p *aolib.CC) { client.handle(true, pktChangeChar, p) })
	s.OnMC(func(p *aolib.MCToServer) { client.handle(true, pktAM, p) })
	s.OnHP(func(p *aolib.HPToServer) { client.handle(true, pktHP, p) })
	s.OnRT(func(p *aolib.RTToServer) { client.handle(true, pktWTCE, p) })
	s.OnCT(func(p *aolib.CTToServer) { client.handle(true, pktOOC, p) })
	s.OnPE(func(p *aolib.PE) { client.handle(true, pktAddEvi, p) })
	s.OnDE(func(p *aolib.DE) { client.handle(true, pktRemoveEvi, p) })
	s.OnEE(func(p *aolib.EE) { client.handle(true, pktEditEvi, p) })
	s.OnCH(func(p *aolib.CH) { client.handle(false, pktPing, p) })
	s.OnZZ(func(p *aolib.ZZToServer) { client.handle(true, pktModcall, p) })
	s.OnMA(func(p *aolib.MA) { client.handle(true, pktMA, p) })
	s.OnMS(func(p *aolib.MSToServer) { client.handle(true, pktIC, p) })
	// SETCASE / CASEA are canonical since aolib 2.6.0 and consume their typed
	// forms directly (the mustJoin gate is inlined).
	s.OnSETCASE(func(p *aolib.SETCASE) {
		if client.Uid() == -1 {
			return
		}
		pktSetCase(client, p)
	})
	s.OnCASEA(func(p *aolib.CASEAToServer) {
		if client.Uid() == -1 {
			return
		}
		pktCaseAnn(client, p)
	})

	// Custom codec headers (both-wire, registered via RegisterPacket).
	for _, h := range []struct {
		header   string
		mustJoin bool
		fn       func(*Client, *aolib.Packet)
	}{
		{"TT", true, pktTT},
		{"VS_JOIN", true, pktVSJoin},
		{"VS_LEAVE", true, pktVSLeave},
		{"VS_FRAME", true, pktVSFrame},
		{"VS_SPEAK", true, pktVSSpeak},
	} {
		h := h
		_ = s.OnCustom(h.header, func(p any) {
			if o, ok := p.(aolib.Outgoing); ok {
				client.handle(h.mustJoin, h.fn, o)
			}
		})
	}
}

// handleUnknownHeader frames an unmodeled header positionally from the Fanta
// wire so its handler still runs. FL (client→server feature advertisement) and
// PW (password) are the two headers aolib's canonical registry doesn't model in
// this direction; everything else is dropped.
func (client *Client) handleUnknownHeader(header string, wire []byte) {
	pkt, err := aolib.NewPacket(strings.TrimSuffix(string(wire), "%"))
	if err != nil {
		return
	}
	switch header {
	case "PW":
		if client.Uid() == -1 {
			return
		}
		pktPW(client, pkt)
	case "FL":
		// aolib models FL as server→client only; the client's own FL arrives here.
		pktFL(client, pkt)
	}
}
