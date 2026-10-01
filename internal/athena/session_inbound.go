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

	// Custom codec headers (both-wire, registered via RegisterCodec).
	for _, h := range []struct {
		header   string
		mustJoin bool
		fn       func(*Client, *aolib.Packet)
	}{
		{"FL", false, pktFL},
		{"MS", true, pktIC},
		{"TT", true, pktTT},
		{"SETCASE", true, pktSetCase},
		{"CASEA", true, pktCaseAnn},
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

// handleUnknownHeader frames an unmodeled header (PW) positionally from the
// Fanta wire so its handler still runs; other unknown headers are dropped.
func (client *Client) handleUnknownHeader(header string, wire []byte) {
	if header != "PW" {
		return
	}
	pkt, err := aolib.NewPacket(strings.TrimSuffix(string(wire), "%"))
	if err != nil {
		return
	}
	if client.Uid() == -1 {
		return
	}
	pktPW(client, pkt)
}
