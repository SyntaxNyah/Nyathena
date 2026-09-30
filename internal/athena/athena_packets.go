package athena

// Athena-only packets that have no canonical aolib-meta schema. They live here,
// on the server side, and are registered into the library registry via
// packet.RegisterDecoder / packet.RegisterServerDecoder (see register.go).

import "fmt"

// TTPacket sets the testimony title overlay. Wire: TT#{type}#{title}#%.
type TTPacket struct {
	Type  string
	Title string
}

func (p *TTPacket) Header() string { return "TT" }
func (p *TTPacket) Args() []string { return []string{p.Type, p.Title} }

// ParseTT decodes a TT body.
func ParseTT(body []string) (*TTPacket, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("TT: expected 2 fields, got %d", len(body))
	}
	return &TTPacket{Type: body[0], Title: body[1]}, nil
}

// SETCASE indicates which case roles the player is willing to fill.
// Wire: SETCASE#{caselist}#{cm}#{def}#{pro}#{judge}#{jury}#{steno}#%.
type SETCASE struct {
	Caselist string
	CM       string // "0"/"1"
	Def      string
	Pro      string
	Judge    string
	Jury     string
	Steno    string
}

func (p *SETCASE) Header() string { return "SETCASE" }
func (p *SETCASE) Args() []string {
	return []string{p.Caselist, p.CM, p.Def, p.Pro, p.Judge, p.Jury, p.Steno}
}

// ParseSETCASE decodes a SETCASE body.
func ParseSETCASE(body []string) (*SETCASE, error) {
	if len(body) != 7 {
		return nil, fmt.Errorf("SETCASE: expected 7 fields, got %d", len(body))
	}
	return &SETCASE{
		Caselist: body[0], CM: body[1], Def: body[2], Pro: body[3],
		Judge: body[4], Jury: body[5], Steno: body[6],
	}, nil
}

// CASEA is the case-announcement packet.
// Wire: CASEA#{case_title}#{need_def}#{need_pro}#{need_judge}#{need_jury}#{need_steno}#%.
type CASEA struct {
	CaseTitle string
	NeedDef   string
	NeedPro   string
	NeedJudge string
	NeedJury  string
	NeedSteno string
}

func (p *CASEA) Header() string { return "CASEA" }
func (p *CASEA) Args() []string {
	return []string{p.CaseTitle, p.NeedDef, p.NeedPro, p.NeedJudge, p.NeedJury, p.NeedSteno}
}

// ParseCASEA decodes a CASEA body.
func ParseCASEA(body []string) (*CASEA, error) {
	if len(body) != 6 {
		return nil, fmt.Errorf("CASEA: expected 6 fields, got %d", len(body))
	}
	return &CASEA{
		CaseTitle: body[0], NeedDef: body[1], NeedPro: body[2],
		NeedJudge: body[3], NeedJury: body[4], NeedSteno: body[5],
	}, nil
}
