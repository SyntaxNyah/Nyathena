package athena

import (
	"encoding/json"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// voicePacketCodec builds a both-wire codec for a single-shape voice packet: the same
// type encodes (Args) and decodes (parse), and JSON marshals/unmarshals the
// struct directly (its json tags + the injected "$header").
func voicePacketCodec[T aolib.Outgoing](parse func([]string) (T, error)) packetutil.Codec {
	return packetutil.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(T).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return parse(args) },
		EncodeJSON:  func(p any) ([]byte, error) { return json.Marshal(p) },
		DecodeJSON: func(raw []byte) (any, error) {
			var v T
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	}
}

func vsCapsCodec() packetutil.Codec  { return voicePacketCodec[*VS_CAPS](ParseVS_CAPS) }
func vsAudioCodec() packetutil.Codec { return voicePacketCodec[*VS_AUDIO](ParseVS_AUDIO) }
func vsFrameCodec() packetutil.Codec { return voicePacketCodec[*VS_FRAME](ParseVS_FRAME) }
func vsPeersCodec() packetutil.Codec { return voicePacketCodec[*VS_PEERS](ParseVS_PEERS) }

// The bidirectional headers (VS_JOIN / VS_LEAVE / VS_SPEAK) carry a different
// shape per direction. From Nyathena's server perspective: encode the ToClient
// broadcast, decode the ToServer request.
func vsJoinCodec() packetutil.Codec {
	return packetutil.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*VS_JOINToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseVS_JOINToServer(args) },
		EncodeJSON:  func(p any) ([]byte, error) { return json.Marshal(p) },
		DecodeJSON: func(raw []byte) (any, error) {
			var v VS_JOINToServer
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, err
			}
			return &v, nil
		},
	}
}

func vsLeaveCodec() packetutil.Codec {
	return packetutil.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*VS_LEAVEToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseVS_LEAVEToServer(args) },
		EncodeJSON:  func(p any) ([]byte, error) { return json.Marshal(p) },
		DecodeJSON: func(raw []byte) (any, error) {
			var v VS_LEAVEToServer
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, err
			}
			return &v, nil
		},
	}
}

func vsSpeakCodec() packetutil.Codec {
	return packetutil.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*VS_SPEAKToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseVS_SPEAKToServer(args) },
		EncodeJSON:  func(p any) ([]byte, error) { return json.Marshal(p) },
		DecodeJSON: func(raw []byte) (any, error) {
			var v VS_SPEAKToServer
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, err
			}
			return &v, nil
		},
	}
}
