package athena

import (
	"strings"
	"testing"
)

func TestEscapeOutgoingEscapesReservedCharacters(t *testing.T) {
	got := escapeOutgoing("CT", []string{"a#b", "c%d", "e$f", "g&h"})
	want := []string{"a<num>b", "c<percent>d", "e<dollar>f", "g<and>h"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("escapeOutgoing(CT)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEscapeOutgoingPreservesAmpersandForEvidence(t *testing.T) {
	got := escapeOutgoing("LE", []string{"name&desc&image#1%"})
	if got[0] != "name&desc&image<num>1<percent>" {
		t.Errorf("escapeOutgoing(LE) = %q", got[0])
	}
	if got := escapeOutgoing("SC", []string{"name&desc&evidence"}); got[0] != "name&desc&evidence" {
		t.Errorf("escapeOutgoing(SC) = %q", got[0])
	}
}

func TestEscapeOutgoingIsIdempotent(t *testing.T) {
	in := []string{"a#b%c$d&e"}
	once := escapeOutgoing("CT", in)
	twice := escapeOutgoing("CT", once)
	if strings.Join(once, "|") != strings.Join(twice, "|") {
		t.Errorf("escapeOutgoing is not idempotent: %q vs %q", once, twice)
	}
}

func TestEscapeOutgoingBlocksKickPacketForgery(t *testing.T) {
	payload := "x%KK#welcome to skrape kid say goodbye to your dreams#%"
	got := escapeOutgoing("CT", []string{payload})[0]
	for _, forbidden := range []string{"%", "#"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("escapeOutgoing leaked raw %q: %q", forbidden, got)
		}
	}
}
