package packet

import (
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTripFanta(t *testing.T) {
	hi := HI{HDID: "abcd1234"}
	raw, err := Encode(&hi, WireFanta)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(raw) != "HI#abcd1234#%" {
		t.Fatalf("Encode(Fanta) = %q", raw)
	}

	v, err := Decode(raw, WireFanta)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, ok := v.(*HI)
	if !ok {
		t.Fatalf("Decode type = %T, want *HI", v)
	}
	if got.HDID != "abcd1234" {
		t.Fatalf("Decode hdid = %#v", got.HDID)
	}
}

func TestEncodeFLJSON(t *testing.T) {
	fl := FL{Features: []string{"multi_pair"}}
	raw, err := Encode(&fl, WireJSON)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The JSON form carries the named features array.
	if !strings.Contains(string(raw), `"features"`) || !strings.Contains(string(raw), "multi_pair") {
		t.Fatalf("Encode(JSON) = %s", raw)
	}
}

func TestDecodeUnknownHeaderFallsBackToPacket(t *testing.T) {
	v, err := Decode([]byte("NOPE#1#%"), WireFanta)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	pkt, ok := v.(*Packet)
	if !ok || pkt.Header != "NOPE" {
		t.Fatalf("Decode unknown = %#v", v)
	}
}
