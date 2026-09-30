package athena

import (
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// The nonstandard packets register both-wire codecs in register.go's init();
// these tests exercise that path through packet.Encode/Decode directly.

func TestTTCodecFantaRoundTrip(t *testing.T) {
	raw, err := packet.Encode(&TTPacket{Type: "0", Title: "Cross Examination"}, packet.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "TT#0#Cross Examination#%" {
		t.Fatalf("fanta = %q", raw)
	}
	v, err := packet.Decode(raw, packet.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	tt, ok := v.(*TTPacket)
	if !ok || tt.Type != "0" || tt.Title != "Cross Examination" {
		t.Fatalf("decoded = %#v", v)
	}
}

func TestTTCodecFantaEscapes(t *testing.T) {
	raw, _ := packet.Encode(&TTPacket{Type: "1", Title: "a#b"}, packet.WireFanta)
	if string(raw) != "TT#1#a<num>b#%" {
		t.Fatalf("fanta escape = %q", raw)
	}
	v, err := packet.Decode(raw, packet.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if tt := v.(*TTPacket); tt.Title != "a#b" {
		t.Fatalf("unescaped title = %q", tt.Title)
	}
}

func TestTTCodecJSONRoundTrip(t *testing.T) {
	raw, err := packet.Encode(&TTPacket{Type: "0", Title: "Cross Examination"}, packet.WireJSON)
	if err != nil {
		t.Fatal(err)
	}
	v, err := packet.Decode(raw, packet.WireJSON)
	if err != nil {
		t.Fatal(err)
	}
	tt, ok := v.(*TTPacket)
	if !ok || tt.Type != "0" || tt.Title != "Cross Examination" {
		t.Fatalf("decoded = %#v from %q", v, raw)
	}
}

func TestSETCASEAndCASEAFantaDecode(t *testing.T) {
	sc, err := packet.Decode([]byte("SETCASE##1#0#1#0#0#%"), packet.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if s := sc.(*SETCASE); s.CM != "1" || s.Def != "0" || s.Pro != "1" {
		t.Fatalf("SETCASE decoded = %#v", s)
	}

	ca, err := packet.Decode([]byte("CASEA#case#1#1#0#0#0#%"), packet.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if c := ca.(*CASEA); c.CaseTitle != "case" || c.NeedDef != "1" || c.NeedPro != "1" {
		t.Fatalf("CASEA decoded = %#v", c)
	}
}

func TestCodecJSONHotPathHelpers(t *testing.T) {
	// EncodeJSON (outbound) → CodecJSONToBody (inbound) round-trips a
	// codec-registered packet through the named-field JSON form.
	raw := packet.EncodeJSON(&TTPacket{Type: "0", Title: "Cross Examination"})
	if raw == nil {
		t.Fatal("EncodeJSON returned nil")
	}
	body, ok := packet.CodecJSONToBody("TT", string(raw))
	if !ok || len(body) != 2 || body[0] != "0" || body[1] != "Cross Examination" {
		t.Fatalf("CodecJSONToBody = %v (ok=%v), raw=%s", body, ok, raw)
	}

	// A header with no codec is not owned by CodecJSONToBody.
	if _, ok := packet.CodecJSONToBody("FL", `{"$header":"FL"}`); ok {
		t.Fatal("CodecJSONToBody claimed a non-codec header")
	}
}

