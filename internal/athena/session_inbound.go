package athena

import (
	"bytes"
	"encoding/json"
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

// handleUnknownHeader frames an unmodeled header from the wire so its handler
// still runs. FL (client→server feature advertisement) and PW (password) are
// the two headers aolib's canonical registry doesn't model in this direction;
// everything else is dropped.
//
// The FL branch is wire-format aware on purpose. aolib models FL as
// server→client only (spec/packets/schemas/FL.schema.json:25), so a client's own
// FL always lands here — and it can arrive in either form:
//
//	FantaCode: FL#timeleap#
//	JSON:      {"$header":"FL","features":["timeleap"]}
//
// The JSON form must be decoded before aolib.NewPacket is tried: that parser is
// a '#'-splitter, and on a JSON frame it returns a nil Body *without an error*,
// so the positional path would hand pktFL an empty body and silently CLEAR the
// client's feature set.
func (client *Client) handleUnknownHeader(header string, wire []byte) {
	switch header {
	case "PW":
		if client.Uid() == -1 {
			return
		}
		pkt, err := aolib.NewPacket(strings.TrimSuffix(string(wire), "%"))
		if err != nil {
			return
		}
		pktPW(client, pkt)

	case "FL":
		if features, ok := jsonFLFeatures(wire); ok {
			client.AddFeatures(features...)
			return
		}
		pkt, err := aolib.NewPacket(strings.TrimSuffix(string(wire), "%"))
		if err != nil {
			return
		}
		pktFL(client, pkt)
	}
}

// jsonFLFeatures extracts {"$header":"FL","features":[...]} from a JSON frame.
// ok is false for any frame that is not a JSON FL, so the caller can fall back
// to the positional path unchanged.
func jsonFLFeatures(wire []byte) ([]string, bool) {
	trimmed := bytes.TrimSpace(wire)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var p struct {
		Header   string   `json:"$header"`
		Features []string `json:"features"`
	}
	if err := json.Unmarshal(trimmed, &p); err != nil || p.Header != "FL" {
		return nil, false
	}
	return p.Features, true
}
