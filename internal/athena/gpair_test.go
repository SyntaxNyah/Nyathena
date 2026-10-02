package athena

import (
	"strings"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// TestParseOffsetUnescapesAnd verifies that parseOffset unescapes the wire
// "<and>" form of "&", so two-axis offsets survive the round trip.
func TestParseOffsetUnescapesAnd(t *testing.T) {
	cases := map[string]aolib.Offset{
		"":            {},
		"0":           {X: 0, Y: 0},
		"50":          {X: 50, Y: 0},
		"50&20":       {X: 50, Y: 20},
		"50<and>20":   {X: 50, Y: 20},
		"-50<and>-20": {X: -50, Y: -20},
	}
	for in, want := range cases {
		if got := parseOffset(in); got != want {
			t.Errorf("parseOffset(%q) = %+v, want %+v", in, got, want)
		}
	}
}

// TestGPEncodeJSON verifies that the JSON-only GP packet routes through its
// custom codec and carries the $header, group_id and ordered members.
func TestGPEncodeJSON(t *testing.T) {
	gp := &GP{GroupID: "100", Members: []GPMember{
		{UID: 100, CharID: 0, Name: "Phoenix", Order: 0},
		{UID: 101, CharID: 1, Name: "Maya", Order: 1},
	}}

	b, err := packetutil.Encode(gp, aolib.WireJSON)
	if err != nil {
		t.Fatalf("packetutil.Encode(GP): %v", err)
	}

	got := string(b)
	for _, want := range []string{
		`"$header":"GP"`,
		`"group_id":"100"`,
		`"members"`,
		`"uid":100`,
		`"uid":101`,
		`"name":"Phoenix"`,
		`"name":"Maya"`,
		`"order":0`,
		`"order":1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("encoded GP %q missing %q", got, want)
		}
	}
}
