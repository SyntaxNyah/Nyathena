package athena

import (
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// The nonstandard packets register both-wire codecs in register.go's init();
// these tests exercise that path through aolib.Encode/Decode directly.

func TestTTCodecFantaRoundTrip(t *testing.T) {
	raw, err := packetutil.Encode(&TTPacket{Type: "0", Title: "Cross Examination"}, aolib.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "TT#0#Cross Examination#%" {
		t.Fatalf("fanta = %q", raw)
	}
	v, err := packetutil.Decode(raw, aolib.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	tt, ok := v.(*TTPacket)
	if !ok || tt.Type != "0" || tt.Title != "Cross Examination" {
		t.Fatalf("decoded = %#v", v)
	}
}

func TestTTCodecFantaEscapes(t *testing.T) {
	raw, _ := packetutil.Encode(&TTPacket{Type: "1", Title: "a#b"}, aolib.WireFanta)
	if string(raw) != "TT#1#a<num>b#%" {
		t.Fatalf("fanta escape = %q", raw)
	}
	v, err := packetutil.Decode(raw, aolib.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if tt := v.(*TTPacket); tt.Title != "a#b" {
		t.Fatalf("unescaped title = %q", tt.Title)
	}
}

func TestTTCodecJSONRoundTrip(t *testing.T) {
	raw, err := packetutil.Encode(&TTPacket{Type: "0", Title: "Cross Examination"}, aolib.WireJSON)
	if err != nil {
		t.Fatal(err)
	}
	v, err := packetutil.Decode(raw, aolib.WireJSON)
	if err != nil {
		t.Fatal(err)
	}
	tt, ok := v.(*TTPacket)
	if !ok || tt.Type != "0" || tt.Title != "Cross Examination" {
		t.Fatalf("decoded = %#v from %q", v, raw)
	}
}

func TestSETCASEAndCASEAFantaDecode(t *testing.T) {
	sc, err := packetutil.Decode([]byte("SETCASE##1#0#1#0#0#%"), aolib.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if s := sc.(*aolib.SETCASE); s.Cases != "" || !s.WillCm || s.WillDef || !s.WillPro {
		t.Fatalf("SETCASE decoded = %#v", s)
	}

	ca, err := packetutil.Decode([]byte("CASEA#case#1#1#0#0#0#%"), aolib.WireFanta)
	if err != nil {
		t.Fatal(err)
	}
	if c := ca.(*aolib.CASEAToServer); c.Title != "case" || !c.NeedDef || !c.NeedPro {
		t.Fatalf("CASEA decoded = %#v", c)
	}
}

func TestCodecJSONHotPathHelpers(t *testing.T) {
	// packetutil.Encode (outbound) → packetutil.Decode (inbound)
	// round-trips a codec-registered packet through the named-field JSON form.
	raw, err := packetutil.Encode(&TTPacket{Type: "0", Title: "Cross Examination"}, aolib.WireJSON)
	if err != nil || raw == nil {
		t.Fatalf("Encode returned nil or error: %v", err)
	}
	p, err := packetutil.Decode(raw, aolib.WireJSON)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	tt, ok := p.(*TTPacket)
	if !ok || tt.Type != "0" || tt.Title != "Cross Examination" {
		t.Fatalf("Decode = %#v, raw=%s", p, raw)
	}
}
