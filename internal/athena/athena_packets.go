package athena

// Athena-only packets that have no canonical aolib-meta schema. They live here,
// on the server side, and are registered into the library registry via
// aolib.RegisterPacket (see register.go). SETCASE and CASEA became canonical in
// aolib 2.6.0, so they use aolib's generated types directly.

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
