package athena

import (
	"encoding/json"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// Both-wire codecs for Nyathena's nonstandard packets (TT). SETCASE and CASEA
// became canonical in aolib 2.6.0, so they use aolib's generated types directly;
// TT remains a server extension registered here.

// ttCodec encodes/decodes TT (testimony title overlay).
func ttCodec() packetutil.Codec {
	return packetutil.Codec{
		EncodeFanta: func(p any) ([]string, error) {
			t := p.(*TTPacket)
			return []string{aolib.EscapeFanta(t.Type), aolib.EscapeFanta(t.Title)}, nil
		},
		DecodeFanta: func(args []string) (any, error) {
			t := &TTPacket{}
			if len(args) > 0 {
				t.Type = aolib.UnescapeFanta(args[0])
			}
			if len(args) > 1 {
				t.Title = aolib.UnescapeFanta(args[1])
			}
			return t, nil
		},
		EncodeJSON: func(p any) ([]byte, error) {
			t := p.(*TTPacket)
			return json.Marshal(map[string]string{"type": t.Type, "title": t.Title})
		},
		DecodeJSON: func(raw []byte) (any, error) {
			var m struct {
				Type  string `json:"type"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, err
			}
			return &TTPacket{Type: m.Type, Title: m.Title}, nil
		},
	}
}
