package athena

import (
	"encoding/json"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// Both-wire codecs for Nyathena's nonstandard packets (TT / SETCASE / CASEA).
// They are registered in register.go via packet.RegisterCodec, matching the
// canonical aolib-go extension point: a codec owns its header in BOTH FantaCode
// and JSON, so these packets round-trip through packet.Encode/Decode in either
// wire mode (the generated registry only covers canonical headers).

// ttCodec encodes/decodes TT (testimony title overlay).
func ttCodec() packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) {
			t := p.(*TTPacket)
			return []string{packet.EscapeFanta(t.Type), packet.EscapeFanta(t.Title)}, nil
		},
		DecodeFanta: func(args []string) (any, error) {
			t := &TTPacket{}
			if len(args) > 0 {
				t.Type = packet.UnescapeFanta(args[0])
			}
			if len(args) > 1 {
				t.Title = packet.UnescapeFanta(args[1])
			}
			return t, nil
		},
		EncodeJSON: func(p any) (string, error) {
			t := p.(*TTPacket)
			b, err := json.Marshal(map[string]string{"type": t.Type, "title": t.Title})
			return string(b), err
		},
		DecodeJSON: func(raw string) (any, error) {
			var m struct {
				Type  string `json:"type"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				return nil, err
			}
			return &TTPacket{Type: m.Type, Title: m.Title}, nil
		},
	}
}

// setcaseCodec encodes/decodes SETCASE (case-role subscription prefs).
func setcaseCodec() packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) {
			s := p.(*SETCASE)
			return []string{s.Caselist, s.CM, s.Def, s.Pro, s.Judge, s.Jury, s.Steno}, nil
		},
		DecodeFanta: func(args []string) (any, error) {
			s := &SETCASE{}
			get := func(i int) string {
				if i < len(args) {
					return args[i]
				}
				return ""
			}
			s.Caselist, s.CM, s.Def, s.Pro, s.Judge, s.Jury, s.Steno =
				get(0), get(1), get(2), get(3), get(4), get(5), get(6)
			return s, nil
		},
		EncodeJSON: func(p any) (string, error) {
			s := p.(*SETCASE)
			b, err := json.Marshal(map[string]string{
				"caselist": s.Caselist, "cm": s.CM, "def": s.Def, "pro": s.Pro,
				"judge": s.Judge, "jury": s.Jury, "steno": s.Steno,
			})
			return string(b), err
		},
		DecodeJSON: func(raw string) (any, error) {
			var m struct {
				Caselist string `json:"caselist"`
				CM       string `json:"cm"`
				Def      string `json:"def"`
				Pro      string `json:"pro"`
				Judge    string `json:"judge"`
				Jury     string `json:"jury"`
				Steno    string `json:"steno"`
			}
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				return nil, err
			}
			return &SETCASE{
				Caselist: m.Caselist, CM: m.CM, Def: m.Def, Pro: m.Pro,
				Judge: m.Judge, Jury: m.Jury, Steno: m.Steno,
			}, nil
		},
	}
}

// caseaCodec encodes/decodes CASEA (case announcement).
func caseaCodec() packet.Codec {
	return packet.Codec{
		EncodeFanta: func(p any) ([]string, error) {
			c := p.(*CASEA)
			return []string{
				packet.EscapeFanta(c.CaseTitle),
				c.NeedDef, c.NeedPro, c.NeedJudge, c.NeedJury, c.NeedSteno,
			}, nil
		},
		DecodeFanta: func(args []string) (any, error) {
			c := &CASEA{}
			get := func(i int) string {
				if i < len(args) {
					return args[i]
				}
				return ""
			}
			c.CaseTitle = packet.UnescapeFanta(get(0))
			c.NeedDef, c.NeedPro, c.NeedJudge, c.NeedJury, c.NeedSteno =
				get(1), get(2), get(3), get(4), get(5)
			return c, nil
		},
		EncodeJSON: func(p any) (string, error) {
			c := p.(*CASEA)
			b, err := json.Marshal(map[string]string{
				"case_title": c.CaseTitle, "need_def": c.NeedDef, "need_pro": c.NeedPro,
				"need_judge": c.NeedJudge, "need_jury": c.NeedJury, "need_steno": c.NeedSteno,
			})
			return string(b), err
		},
		DecodeJSON: func(raw string) (any, error) {
			var m struct {
				CaseTitle string `json:"case_title"`
				NeedDef   string `json:"need_def"`
				NeedPro   string `json:"need_pro"`
				NeedJudge string `json:"need_judge"`
				NeedJury  string `json:"need_jury"`
				NeedSteno string `json:"need_steno"`
			}
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				return nil, err
			}
			return &CASEA{
				CaseTitle: m.CaseTitle, NeedDef: m.NeedDef, NeedPro: m.NeedPro,
				NeedJudge: m.NeedJudge, NeedJury: m.NeedJury, NeedSteno: m.NeedSteno,
			}, nil
		},
	}
}
