package packet

// Enum <-> legacy wire-integer maps used by the hand-written MS packet
// (mspacket.go). The canonical packets encode these internally in aolib-go;
// MS stays local, so it needs its own copies. Values follow each enum's
// x-wire-ints in aolib-meta.

var deskModifierToWire = map[DeskModifier]int{
	DeskModifierHidden:                      0,
	DeskModifierShown:                       1,
	DeskModifierHideDuringPreanim:           2,
	DeskModifierShowDuringPreanim:           3,
	DeskModifierHideAndCenterDuringPreanim:  4,
	DeskModifierShowDuringPreanimThenCenter: 5,
}

var deskModifierFromWire = map[int]DeskModifier{
	0: DeskModifierHidden,
	1: DeskModifierShown,
	2: DeskModifierHideDuringPreanim,
	3: DeskModifierShowDuringPreanim,
	4: DeskModifierHideAndCenterDuringPreanim,
	5: DeskModifierShowDuringPreanimThenCenter,
}

var emoteModifierToWire = map[EmoteModifier]int{
	EmoteModifierNoPreanim:           0,
	EmoteModifierPreanim:             1,
	EmoteModifierPreanimAndObjection: 2,
	EmoteModifierUnused3:             3,
	EmoteModifierUnused4:             4,
	EmoteModifierZoom:                5,
	EmoteModifierObjectionZoom:       6,
}

var emoteModifierFromWire = map[int]EmoteModifier{
	0: EmoteModifierNoPreanim,
	1: EmoteModifierPreanim,
	2: EmoteModifierPreanimAndObjection,
	3: EmoteModifierUnused3,
	4: EmoteModifierUnused4,
	5: EmoteModifierZoom,
	6: EmoteModifierObjectionZoom,
}

var flipToWire = map[Flip]int{
	FlipNone:                  0,
	FlipHorizontal:            1,
	FlipVertical:              2,
	FlipHorizontalAndVertical: 3,
}

var flipFromWire = map[int]Flip{
	0: FlipNone,
	1: FlipHorizontal,
	2: FlipVertical,
	3: FlipHorizontalAndVertical,
}

var shoutModifierToWire = map[ShoutModifier]int{
	ShoutModifierNone:      0,
	ShoutModifierHoldIt:    1,
	ShoutModifierObjection: 2,
	ShoutModifierTakeThat:  3,
	ShoutModifierCustom:    4,
}

var shoutModifierFromWire = map[int]ShoutModifier{
	0: ShoutModifierNone,
	1: ShoutModifierHoldIt,
	2: ShoutModifierObjection,
	3: ShoutModifierTakeThat,
	4: ShoutModifierCustom,
}

var textColorToWire = map[TextColor]int{
	TextColorWhite:   0,
	TextColorGreen:   1,
	TextColorRed:     2,
	TextColorOrange:  3,
	TextColorBlue:    4,
	TextColorYellow:  5,
	TextColorPink:    6,
	TextColorCyan:    7,
	TextColorGrey:    8,
	TextColorRainbow: 9,
}

var textColorFromWire = map[int]TextColor{
	0: TextColorWhite,
	1: TextColorGreen,
	2: TextColorRed,
	3: TextColorOrange,
	4: TextColorBlue,
	5: TextColorYellow,
	6: TextColorPink,
	7: TextColorCyan,
	8: TextColorGrey,
	9: TextColorRainbow,
}
