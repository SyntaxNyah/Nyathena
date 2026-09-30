package athena

// Voice-chat packets (VS_*). Removed from the canonical aolib spec (aa8d0fb);
// Nyathena keeps them as a server extension, registered both-wire via
// packet.RegisterCodec (see voice_codecs.go + register.go). Wire contract:
//
//	S→C  VS_CAPS#<enabled>#<ptt_only>#<max_peers>#<codec>#<sample_rate>#<frame_ms>#<max_frame_bytes>#%
//	S→C  VS_PEERS#<csv_uids>#%
//	S→C  VS_JOIN#<uid>#%   VS_LEAVE#<uid>#%   VS_SPEAK#<uid>#<on>#%   VS_AUDIO#<from_uid>#<b64>#%
//	C→S  VS_JOIN#%         VS_LEAVE#%         VS_SPEAK#<on>#%         VS_FRAME#<b64>#%

import (
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// VS_CAPS is the server's voice capability advertisement (server→client).
type VS_CAPS struct {
	Enabled       bool   `json:"enabled"`
	PttOnly       bool   `json:"pttOnly"`
	MaxPeers      int    `json:"maxPeers"`
	Codec         string `json:"codec"`
	SampleRate    int    `json:"sampleRate"`
	FrameMs       int    `json:"frameMs"`
	MaxFrameBytes int    `json:"maxFrameBytes"`
}

func (p *VS_CAPS) Header() string { return "VS_CAPS" }

func (p *VS_CAPS) Args() []string {
	return []string{
		packet.BoolToWire(p.Enabled),
		packet.BoolToWire(p.PttOnly),
		packet.Itoa(p.MaxPeers),
		packet.EscapeFanta(p.Codec),
		packet.Itoa(p.SampleRate),
		packet.Itoa(p.FrameMs),
		packet.Itoa(p.MaxFrameBytes),
	}
}

func ParseVS_CAPS(body []string) (*VS_CAPS, error) {
	p := &VS_CAPS{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.Enabled = packet.WireToBool(get(0))
	p.PttOnly = packet.WireToBool(get(1))
	p.MaxPeers = packet.AtoiOrZero(get(2))
	p.Codec = packet.UnescapeFanta(get(3))
	p.SampleRate = packet.AtoiOrZero(get(4))
	p.FrameMs = packet.AtoiOrZero(get(5))
	p.MaxFrameBytes = packet.AtoiOrZero(get(6))
	return p, nil
}

// VS_AUDIO relays one Opus frame to a peer (server→client).
type VS_AUDIO struct {
	FromUID int    `json:"fromUid"`
	Payload string `json:"payload"`
}

func (p *VS_AUDIO) Header() string { return "VS_AUDIO" }

func (p *VS_AUDIO) Args() []string {
	return []string{packet.Itoa(p.FromUID), packet.EscapeFanta(p.Payload)}
}

func ParseVS_AUDIO(body []string) (*VS_AUDIO, error) {
	p := &VS_AUDIO{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.FromUID = packet.AtoiOrZero(get(0))
	p.Payload = packet.UnescapeFanta(get(1))
	return p, nil
}

// VS_FRAME carries one Opus frame upstream (client→server).
type VS_FRAME struct {
	Payload string `json:"payload"`
}

func (p *VS_FRAME) Header() string { return "VS_FRAME" }

func (p *VS_FRAME) Args() []string {
	return []string{packet.EscapeFanta(p.Payload)}
}

func ParseVS_FRAME(body []string) (*VS_FRAME, error) {
	p := &VS_FRAME{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.Payload = packet.UnescapeFanta(get(0))
	return p, nil
}

// VS_JOINToClient is the server's broadcast that a peer joined (server→client).
type VS_JOINToClient struct {
	UID int `json:"uid"`
}

func (p *VS_JOINToClient) Header() string { return "VS_JOIN" }

func (p *VS_JOINToClient) Args() []string {
	return []string{packet.Itoa(p.UID)}
}

func ParseVS_JOINToClient(body []string) (*VS_JOINToClient, error) {
	p := &VS_JOINToClient{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.UID = packet.AtoiOrZero(get(0))
	return p, nil
}

// VS_JOINToServer is a client's join request (client→server). No fields.
type VS_JOINToServer struct{}

func (p *VS_JOINToServer) Header() string { return "VS_JOIN" }

func (p *VS_JOINToServer) Args() []string { return nil }

func ParseVS_JOINToServer([]string) (*VS_JOINToServer, error) {
	return &VS_JOINToServer{}, nil
}

// VS_LEAVEToClient is the server's broadcast that a peer left (server→client).
type VS_LEAVEToClient struct {
	UID int `json:"uid"`
}

func (p *VS_LEAVEToClient) Header() string { return "VS_LEAVE" }

func (p *VS_LEAVEToClient) Args() []string {
	return []string{packet.Itoa(p.UID)}
}

func ParseVS_LEAVEToClient(body []string) (*VS_LEAVEToClient, error) {
	p := &VS_LEAVEToClient{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.UID = packet.AtoiOrZero(get(0))
	return p, nil
}

// VS_LEAVEToServer is a client's leave request (client→server). No fields.
type VS_LEAVEToServer struct{}

func (p *VS_LEAVEToServer) Header() string { return "VS_LEAVE" }

func (p *VS_LEAVEToServer) Args() []string { return nil }

func ParseVS_LEAVEToServer([]string) (*VS_LEAVEToServer, error) {
	return &VS_LEAVEToServer{}, nil
}

// VS_PEERS is the server's comma-separated roster of the area's voice peers
// (server→client). Clients split on ','; an empty list frames as VS_PEERS#%.
type VS_PEERS struct {
	Uids []int `json:"uids"`
}

func (p *VS_PEERS) Header() string { return "VS_PEERS" }

func (p *VS_PEERS) Args() []string {
	if len(p.Uids) == 0 {
		return nil
	}
	parts := make([]string, len(p.Uids))
	for i, u := range p.Uids {
		parts[i] = packet.Itoa(u)
	}
	return []string{strings.Join(parts, ",")}
}

func ParseVS_PEERS(body []string) (*VS_PEERS, error) {
	p := &VS_PEERS{}
	if len(body) > 0 && body[0] != "" {
		for _, f := range strings.Split(body[0], ",") {
			if f = strings.TrimSpace(f); f != "" {
				p.Uids = append(p.Uids, packet.AtoiOrZero(f))
			}
		}
	}
	return p, nil
}

// VS_SPEAKToClient is the server's broadcast of a speaking-state change
// (server→client).
type VS_SPEAKToClient struct {
	UID int  `json:"uid"`
	On  bool `json:"on"`
}

func (p *VS_SPEAKToClient) Header() string { return "VS_SPEAK" }

func (p *VS_SPEAKToClient) Args() []string {
	return []string{packet.Itoa(p.UID), packet.BoolToWire(p.On)}
}

func ParseVS_SPEAKToClient(body []string) (*VS_SPEAKToClient, error) {
	p := &VS_SPEAKToClient{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.UID = packet.AtoiOrZero(get(0))
	p.On = packet.WireToBool(get(1))
	return p, nil
}

// VS_SPEAKToServer is a client's speaking-state change (client→server).
type VS_SPEAKToServer struct {
	On bool `json:"on"`
}

func (p *VS_SPEAKToServer) Header() string { return "VS_SPEAK" }

func (p *VS_SPEAKToServer) Args() []string {
	return []string{packet.BoolToWire(p.On)}
}

func ParseVS_SPEAKToServer(body []string) (*VS_SPEAKToServer, error) {
	p := &VS_SPEAKToServer{}
	get := func(i int) string {
		if i < len(body) {
			return body[i]
		}
		return ""
	}
	p.On = packet.WireToBool(get(0))
	return p, nil
}

