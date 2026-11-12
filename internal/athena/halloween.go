package athena

import (
	"math/rand"
	"strings"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// Halloween mode: purely seasonal silliness for the display names on outbound
// messages. Each message independently rolls a 30% chance; a hit appends a
// random spooky emoji to the name that message is shown under. The roll is
// per message, not per name, so the same player's name wanders between
// decorations (or none) from message to message.

// halloweenChancePercent is the per-message chance (0-100) of decorating.
const halloweenChancePercent = 30

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

func halloweenDecorated(name string) bool {
	for _, e := range halloweenEmoji {
		if strings.HasSuffix(name, " "+e) {
			return true
		}
	}
	return false
}

// halloweenHaunt maybe appends a random spooky emoji to a name. Empty and
// whitespace-only names, and names that already carry a seasonal emoji, are
// left alone.
func halloweenHaunt(name string) string {
	if strings.TrimSpace(name) == "" || halloweenDecorated(name) {
		return name
	}
	if rand.Intn(100) >= halloweenChancePercent {
		return name
	}
	return name + " " + halloweenEmoji[rand.Intn(len(halloweenEmoji))]
}

// halloweenHauntOutgoing decorates the display-name fields of an outbound
// packet while halloween mode is on. The roll happens once per message: the
// first recipient's send mutates the shared broadcast packet and later
// recipients see the already-decorated name and pass through, so every
// recipient of a given message sees the same name. Only player names are
// touched; the server's own OOC messages pass through unchanged.
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
