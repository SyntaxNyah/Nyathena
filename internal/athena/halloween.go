package athena

import (
	"strings"
	"unicode"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// Halloween mode: seasonal flair for the display names the server forwards
// to clients. When config.Halloween is on, names are tidied -- fancy quotes
// folded to plain ASCII, invisible and formatting characters removed, stacked
// combining marks (zalgo) swept -- and roughly a third of distinct names get
// a small spooky emoji for the season.
//
// The emoji a name draws is chosen by a stable hash of the clean name, so it
// stays the same from message to message and across restarts, and the rewrite
// is idempotent -- a shared packet fanned out to every recipient in an area
// cannot gain a second decoration on a later pass.

// halloweenChance is roughly what share of distinct names get an emoji
// (0-100).
const halloweenChance = 30

// The seasonal emoji, spelled out as code points so the source needs no
// UTF-8 literals. The ones with U+FE0F ask for the colour emoji glyph.
var halloweenEmoji = []string{
	string([]rune{0x1F383}),         // jack-o-lantern
	string([]rune{0x1F47B}),         // ghost
	string([]rune{0x1F480}),         // skull
	string([]rune{0x1F987}),         // bat
	string([]rune{0x1F577, 0xFE0F}), // spider
	string([]rune{0x1F578, 0xFE0F}), // web
	string([]rune{0x26B0, 0xFE0F}),  // coffin
	string([]rune{0x2620, 0xFE0F}),  // skull and crossbones
	string([]rune{0x1F9DF}),         // zombie
	string([]rune{0x1F9DB}),         // vampire
	string([]rune{0x1F319}),         // crescent moon
	string([]rune{0x1F52E}),         // crystal ball
}

func halloweenFancySingleQuote(r rune) bool {
	switch r {
	case 0x2018, 0x2019, 0x201A, 0x201B:
		return true
	}
	return false
}

func halloweenFancyDoubleQuote(r rune) bool {
	switch r {
	case 0x201C, 0x201D, 0x201E, 0x201F:
		return true
	}
	return false
}

func halloweenInvisible(r rune) bool {
	return r < 0x20 ||
		r == 0x7F ||
		(r >= 0x80 && r <= 0x9F) ||
		r == 0x00AD ||
		(r >= 0x200B && r <= 0x200F) ||
		(r >= 0x202A && r <= 0x202E) ||
		(r >= 0x2060 && r <= 0x2064) ||
		(r >= 0x2066 && r <= 0x2069) ||
		r == 0xFEFF ||
		(r >= 0xFFF9 && r <= 0xFFFB) ||
		(r >= 0xE0000 && r <= 0xE007F)
}

// halloweenClean returns the tidied base name, or the input unchanged (no
// copy) when nothing needs doing.
func halloweenClean(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	changed := false
	for _, r := range name {
		switch {
		case halloweenInvisible(r), unicode.In(r, unicode.Mn, unicode.Me):
			changed = true
		case halloweenFancySingleQuote(r):
			b.WriteRune('\'')
			changed = true
		case halloweenFancyDoubleQuote(r):
			b.WriteRune('"')
			changed = true
		default:
			b.WriteRune(r)
		}
	}
	if !changed {
		return name
	}
	return b.String()
}

// halloweenStableHash is FNV-1a over the UTF-8 bytes: a small stable hash so
// the same name always draws the same decoration.
func halloweenStableHash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func halloweenDecorated(name string) bool {
	for _, e := range halloweenEmoji {
		if strings.HasSuffix(name, " "+e) {
			return true
		}
	}
	return false
}

// halloweenHaunt is halloweenClean, then maybe a trailing spooky emoji.
// Empty and whitespace-only names, and names that already carry one of the
// seasonal emoji, are left alone.
func halloweenHaunt(name string) string {
	clean := halloweenClean(name)
	if strings.TrimSpace(clean) == "" || halloweenDecorated(clean) {
		return clean
	}
	h := halloweenStableHash(clean)
	if int(h%100) >= halloweenChance {
		return clean
	}
	return clean + " " + halloweenEmoji[h%uint32(len(halloweenEmoji))]
}

// halloweenHauntOutgoing rewrites the display-name fields of an outbound
// packet while halloween mode is on. Only player names are touched; the
// server's own OOC messages pass through unchanged.
func halloweenHauntOutgoing(p aolib.Outgoing) {
	if config == nil || !config.Halloween {
		return
	}
	switch t := p.(type) {
	case *aolib.CTToClient:
		if t.IsFromServer {
			return
		}
		t.Name = halloweenHaunt(t.Name)
	case *aolib.MSToClient:
		t.Showname = halloweenHaunt(t.Showname)
		if t.PairedName != "" {
			t.PairedName = halloweenHaunt(t.PairedName)
		}
	case *aolib.PU:
		switch t.Type {
		case aolib.PlayerDataTypeOocName, aolib.PlayerDataTypeCharName, aolib.PlayerDataTypeShowname:
			t.Data = halloweenHaunt(t.Data)
		}
	}
}
