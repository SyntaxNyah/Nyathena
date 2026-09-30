package athena

import (
	"encoding/json"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// voiceCodec builds a both-wire codec for a single-shape voice packet: the same
// type encodes (Args) and decodes (parse), and JSON marshals/unmarshals the
// struct directly (its json tags + the injected "$header").
func voiceCodec[T packet.Outgoing](parse func([]string) (T, error)) packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(T).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return parse(args) },
		EncodeJSON:  func(p any) (string, error) { b, err := json.Marshal(p); return string(b), err },
		DecodeJSON: func(raw string) (any, error) {
			var v T
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	}
}

func vsCapsCodec() packet.Codec  { return voiceCodec[*VS_CAPS](ParseVS_CAPS) }
func vsAudioCodec() packet.Codec { return voiceCodec[*VS_AUDIO](ParseVS_AUDIO) }
func vsFrameCodec() packet.Codec { return voiceCodec[*VS_FRAME](ParseVS_FRAME) }
func vsPeersCodec() packet.Codec { return voiceCodec[*VS_PEERS](ParseVS_PEERS) }

// The bidirectional headers (VS_JOIN / VS_LEAVE / VS_SPEAK) carry a different
// shape per direction. From Nyathena's server perspective: encode the ToClient
// broadcast, decode the ToServer request.
func vsJoinCodec() packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*VS_JOINToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseVS_JOINToServer(args) },
		EncodeJSON:  func(p any) (string, error) { b, err := json.Marshal(p); return string(b), err },
		DecodeJSON: func(raw string) (any, error) {
			var v VS_JOINToServer
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				return nil, err
			}
			return &v, nil
		},
	}
}

func vsLeaveCodec() packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*VS_LEAVEToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseVS_LEAVEToServer(args) },
		EncodeJSON:  func(p any) (string, error) { b, err := json.Marshal(p); return string(b), err },
		DecodeJSON: func(raw string) (any, error) {
			var v VS_LEAVEToServer
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				return nil, err
			}
			return &v, nil
		},
	}
}

func vsSpeakCodec() packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*VS_SPEAKToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseVS_SPEAKToServer(args) },
		EncodeJSON:  func(p any) (string, error) { b, err := json.Marshal(p); return string(b), err },
		DecodeJSON: func(raw string) (any, error) {
			var v VS_SPEAKToServer
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				return nil, err
			}
			return &v, nil
		},
	}
}
