package athena

import (
	"strconv"

	packet "github.com/MangosArentLiterature/Athena/internal/packet"
)

// takenToInts converts an area's taken-character list (wire strings) into the
// []int form aolib's CharsCheck.Taken expects. Non-numeric entries decode to 0.
func takenToInts(ss []string) []int {
	out := make([]int, 0, len(ss))
	for _, s := range ss {
		n, _ := strconv.Atoi(s)
		out = append(out, n)
	}
	return out
}

// Enum <-> legacy wire-integer conversions for the typed aolib packet enums.
// aolib-go keeps its own maps unexported, so the app-side validation and
// protocol-punishment code that reasons in wire integers needs local copies
// keyed off the exported enum constants.

var emoteModifierWire = map[packet.EmoteModifier]int{
	packet.EmoteModifierNoPreanim:           0,
	packet.EmoteModifierPreanim:             1,
	packet.EmoteModifierPreanimAndObjection: 2,
	packet.EmoteModifierUnused3:             3,
	packet.EmoteModifierUnused4:             4,
	packet.EmoteModifierZoom:                5,
	packet.EmoteModifierObjectionZoom:       6,
}

var shoutModifierWire = map[packet.ShoutModifier]int{
	packet.ShoutModifierNone:      0,
	packet.ShoutModifierHoldIt:    1,
	packet.ShoutModifierObjection: 2,
	packet.ShoutModifierTakeThat:  3,
	packet.ShoutModifierCustom:    4,
}

var textColorWire = map[packet.TextColor]int{
	packet.TextColorWhite:   0,
	packet.TextColorGreen:   1,
	packet.TextColorRed:     2,
	packet.TextColorOrange:  3,
	packet.TextColorBlue:    4,
	packet.TextColorYellow:  5,
	packet.TextColorPink:    6,
	packet.TextColorCyan:    7,
	packet.TextColorGrey:    8,
	packet.TextColorRainbow: 9,
}

var textColorFromWireInt = map[int]packet.TextColor{
	0: packet.TextColorWhite,
	1: packet.TextColorGreen,
	2: packet.TextColorRed,
	3: packet.TextColorOrange,
	4: packet.TextColorBlue,
	5: packet.TextColorYellow,
	6: packet.TextColorPink,
	7: packet.TextColorCyan,
	8: packet.TextColorGrey,
	9: packet.TextColorRainbow,
}

var flipWire = map[packet.Flip]int{
	packet.FlipNone:                  0,
	packet.FlipHorizontal:            1,
	packet.FlipVertical:              2,
	packet.FlipHorizontalAndVertical: 3,
}

var flipFromWireInt = map[int]packet.Flip{
	0: packet.FlipNone,
	1: packet.FlipHorizontal,
	2: packet.FlipVertical,
	3: packet.FlipHorizontalAndVertical,
}

// flipFromWire maps a legacy flip wire integer to its enum value.
func flipFromWire(n int) packet.Flip { return flipFromWireInt[n] }

// textColorFromWire maps a legacy text-color wire integer to its enum value.
func textColorFromWire(n int) packet.TextColor { return textColorFromWireInt[n] }
