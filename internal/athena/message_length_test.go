package athena

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestMessageLengthCountsRunesNotBytes locks in the fix for the bug where a
// visually short IC/OOC message was rejected with "Your message exceeds the
// maximum message length!" because it carried invisible zero-width characters.
//
// pktIC/pktOOC enforce config.MaxMsg with utf8.RuneCountInString (characters),
// not len (UTF-8 bytes). A message of, say, "Dictator Emerald\cgBut why am I
// triggering it?" plus a run of zero-width characters (U+2060 WORD JOINER,
// U+200C ZERO WIDTH NON-JOINER) is only a few dozen characters but, because each
// invisible character is 3 bytes, can blow past a 256-*byte* ceiling while
// staying well under a 256-*character* one.
func TestMessageLengthCountsRunesNotBytes(t *testing.T) {
	const limit = 256 // config.MaxMsg default

	visible := "Dictator Emerald\\cgBut why am I triggering it?"
	// A long run of invisible zero-width characters, as gets pasted in from a
	// watermarked/steganographic source. Enough to exceed the byte ceiling.
	invisible := strings.Repeat("⁠‌", 60) // 120 runes, 360 bytes
	msg := visible + invisible

	bytes := len(msg)
	runes := utf8.RuneCountInString(msg)

	if bytes <= limit {
		t.Fatalf("test setup invalid: want byte length > %d, got %d", limit, bytes)
	}
	if runes > limit {
		t.Fatalf("test setup invalid: want rune count <= %d, got %d", limit, runes)
	}

	// The byte-based check (the old behaviour) would have rejected this message.
	if !(bytes > limit) {
		t.Errorf("expected old byte check to reject (bytes=%d limit=%d)", bytes, limit)
	}
	// The rune-based check (the current behaviour) accepts it.
	if utf8.RuneCountInString(msg) > limit {
		t.Errorf("rune-based check wrongly rejected a %d-character message (bytes=%d)", runes, bytes)
	}
}

// TestMessageLengthStillRejectsTooManyRunes ensures the limit is still enforced:
// a message that genuinely exceeds the character ceiling is rejected.
func TestMessageLengthStillRejectsTooManyRunes(t *testing.T) {
	const limit = 256

	msg := strings.Repeat("a", limit+1)
	if utf8.RuneCountInString(msg) <= limit {
		t.Fatalf("expected %d-character message to exceed limit %d", utf8.RuneCountInString(msg), limit)
	}
}
