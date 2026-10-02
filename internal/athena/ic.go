package athena

// Nyathena's in-character ("MS") aolib. Canonical aolib-meta models the
// standard MS (int char_id/sfx_delay/evidence_id, bool flags, Offset objects),
// but Nyathena keeps the historical string-shaped MS and only migrates the
// enum slots to the library's string enums. The wire helpers and enum wire
// maps are the exported primitives from internal/aolib.
//
// Nyathena-only extensions that aolib-meta does not define:
//   - Blips (2.10.2+, field 30)
//   - AdditionalChars (JSON-only multi-pair list)
//   - PairedCharID carrying a "pid^order" suffix
//   - ShoutName carrying the "&name" custom-objection sub-value

import (
	"strings"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// AdditionalChar is one on-screen partner beyond the standard pair.
type AdditionalChar struct {
	CharID int          `json:"charid"`
	Name   string       `json:"name"`
	Emote  string       `json:"emote"`
	Side   string       `json:"side"`
	Offset aolib.Offset `json:"offset"`
	Flip   aolib.Flip   `json:"flip"`
	Order  int          `json:"order"` // z-offset: 0 behind the speaker (default), 1 in front
}

// MSToServer is the client -> server MS (26 fields + Blips).
type MSToServer struct {
	DeskModifier           aolib.DeskModifier
	Preanim                string
	Character              string
	Emote                  string
	Message                string
	Side                   aolib.Side
	SfxName                string
	EmoteModifier          aolib.EmoteModifier
	CharID                 string
	SfxDelay               string
	ShoutModifier          aolib.ShoutModifier
	ShoutName              string
	Evidence               string
	Flip                   aolib.Flip
	Realization            string
	TextColor              aolib.TextColor
	Showname               string
	PairedCharID           string
	Offset                 string
	NoninterruptingPreanim string
	SfxLooping             string
	Screenshake            string
	FramesShake            string
	FramesRealization      string
	FramesSfx              string
	Additive               string
	Effect                 string
	Blips                  string
}

// MSToClient is the server -> client MS (30 fields + Blips + additional_chars).
type MSToClient struct {
	DeskModifier           aolib.DeskModifier
	Preanim                string
	Character              string
	Emote                  string
	Message                string
	Side                   aolib.Side
	SfxName                string
	EmoteModifier          aolib.EmoteModifier
	CharID                 string
	SfxDelay               string
	ShoutModifier          aolib.ShoutModifier
	ShoutName              string
	Evidence               string
	Flip                   aolib.Flip
	Realization            string
	TextColor              aolib.TextColor
	Showname               string
	PairedCharID           string
	PairedName             string
	PairedEmote            string
	Offset                 string
	PairedOffset           string
	PairedFlip             aolib.Flip
	NoninterruptingPreanim string
	SfxLooping             string
	Screenshake            string
	FramesShake            string
	FramesRealization      string
	FramesSfx              string
	Additive               string
	Effect                 string
	Blips                  string

	AdditionalChars []AdditionalChar
}

// Header returns the AO2 header "MS".
func (ms *MSToServer) Header() string { return "MS" }

// Header returns the AO2 header "MS".
func (ms *MSToClient) Header() string { return "MS" }

// Shout returns the numeric wire value of the shout/objection modifier. The
// field itself is a typed string enum; callers that score or range-check the
// value (the IC handler and the raid guard) want the wire integer. The error
// is always nil and exists so the historical two-value call sites keep their
// shape.
func (ms *MSToServer) Shout() (int, error) {
	return packetutil.ShoutModifierToWire[ms.ShoutModifier], nil
}

// Shout returns the numeric wire value of the shout/objection modifier.
func (ms *MSToClient) Shout() (int, error) {
	return packetutil.ShoutModifierToWire[ms.ShoutModifier], nil
}

// parseDeskModifier parses the desk_modifier wire token, mapping the legacy
// "chat" alias to the shown value.
func parseDeskModifier(s string) aolib.DeskModifier {
	if s == "chat" {
		return aolib.DeskModifierShown
	}
	return packetutil.DeskModifierFromWire[packetutil.AtoiOrZero(s)]
}

// parseShout splits a shout_modifier wire token ("4&name") into the typed enum
// and the optional custom name.
func parseShout(raw string) (aolib.ShoutModifier, string) {
	if n, name, ok := strings.Cut(raw, "&"); ok {
		return packetutil.ShoutModifierFromWire[packetutil.AtoiOrZero(n)], name
	}
	return packetutil.ShoutModifierFromWire[packetutil.AtoiOrZero(raw)], ""
}

// shoutWire encodes a shout modifier, appending "&name" for custom shouts.
func shoutWire(m aolib.ShoutModifier, name string) string {
	s := packetutil.Itoa(packetutil.ShoutModifierToWire[m])
	if m == aolib.ShoutModifierCustom && name != "" {
		s += "&" + name
	}
	return s
}

// ParseMSToServer decodes the client -> server MS body (enum slots mapped to
// string enums; every other slot is the raw wire string).
func ParseMSToServer(body []string) *MSToServer {
	ms := &MSToServer{}
	get := func(i int) string { return packetutil.GetStr(body, i) }
	ms.DeskModifier = parseDeskModifier(get(0))
	ms.Preanim = get(1)
	ms.Character = get(2)
	ms.Emote = get(3)
	ms.Message = get(4)
	ms.Side = aolib.Side(get(5))
	ms.SfxName = get(6)
	ms.EmoteModifier = packetutil.EmoteModifierFromWire[packetutil.AtoiOrZero(get(7))]
	ms.CharID = get(8)
	ms.SfxDelay = get(9)
	ms.ShoutModifier, ms.ShoutName = parseShout(get(10))
	ms.Evidence = get(11)
	ms.Flip = packetutil.FlipFromWire[packetutil.AtoiOrZero(get(12))]
	ms.Realization = get(13)
	ms.TextColor = packetutil.TextColorFromWire[packetutil.AtoiOrZero(get(14))]
	ms.Showname = get(15)
	ms.PairedCharID = get(16)
	ms.Offset = get(17)
	ms.NoninterruptingPreanim = get(18)
	ms.SfxLooping = get(19)
	ms.Screenshake = get(20)
	ms.FramesShake = get(21)
	ms.FramesRealization = get(22)
	ms.FramesSfx = get(23)
	ms.Additive = get(24)
	ms.Effect = get(25)
	ms.Blips = get(26)
	return ms
}

// ParseMSToClient decodes the server -> client MS body.
func ParseMSToClient(body []string) *MSToClient {
	ms := &MSToClient{}
	get := func(i int) string { return packetutil.GetStr(body, i) }
	ms.DeskModifier = parseDeskModifier(get(0))
	ms.Preanim = get(1)
	ms.Character = get(2)
	ms.Emote = get(3)
	ms.Message = get(4)
	ms.Side = aolib.Side(get(5))
	ms.SfxName = get(6)
	ms.EmoteModifier = packetutil.EmoteModifierFromWire[packetutil.AtoiOrZero(get(7))]
	ms.CharID = get(8)
	ms.SfxDelay = get(9)
	ms.ShoutModifier, ms.ShoutName = parseShout(get(10))
	ms.Evidence = get(11)
	ms.Flip = packetutil.FlipFromWire[packetutil.AtoiOrZero(get(12))]
	ms.Realization = get(13)
	ms.TextColor = packetutil.TextColorFromWire[packetutil.AtoiOrZero(get(14))]
	ms.Showname = get(15)
	ms.PairedCharID = get(16)
	ms.PairedName = get(17)
	ms.PairedEmote = get(18)
	ms.Offset = get(19)
	ms.PairedOffset = get(20)
	ms.PairedFlip = packetutil.FlipFromWire[packetutil.AtoiOrZero(get(21))]
	ms.NoninterruptingPreanim = get(22)
	ms.SfxLooping = get(23)
	ms.Screenshake = get(24)
	ms.FramesShake = get(25)
	ms.FramesRealization = get(26)
	ms.FramesSfx = get(27)
	ms.Additive = get(28)
	ms.Effect = get(29)
	ms.Blips = get(30)
	return ms
}

// ParseMSToClientString splits a server-format MS body joined by '#'.
func ParseMSToClientString(s string) *MSToClient {
	if s == "" {
		return &MSToClient{}
	}
	return ParseMSToClient(strings.Split(s, "#"))
}

// ToClient converts the client -> server form into the server -> client form.
func (ms *MSToServer) ToClient() *MSToClient {
	return &MSToClient{
		DeskModifier:           ms.DeskModifier,
		Preanim:                ms.Preanim,
		Character:              ms.Character,
		Emote:                  ms.Emote,
		Message:                ms.Message,
		Side:                   ms.Side,
		SfxName:                ms.SfxName,
		EmoteModifier:          ms.EmoteModifier,
		CharID:                 ms.CharID,
		SfxDelay:               ms.SfxDelay,
		ShoutModifier:          ms.ShoutModifier,
		ShoutName:              ms.ShoutName,
		Evidence:               ms.Evidence,
		Flip:                   ms.Flip,
		Realization:            ms.Realization,
		TextColor:              ms.TextColor,
		Showname:               ms.Showname,
		PairedCharID:           ms.PairedCharID,
		Offset:                 ms.Offset,
		NoninterruptingPreanim: ms.NoninterruptingPreanim,
		SfxLooping:             ms.SfxLooping,
		Screenshake:            ms.Screenshake,
		FramesShake:            ms.FramesShake,
		FramesRealization:      ms.FramesRealization,
		FramesSfx:              ms.FramesSfx,
		Additive:               ms.Additive,
		Effect:                 ms.Effect,
		Blips:                  ms.Blips,
	}
}

// Args returns the client -> server MS body (26 fields + optional Blips).
func (ms *MSToServer) Args() []string {
	args := []string{
		packetutil.Itoa(packetutil.DeskModifierToWire[ms.DeskModifier]),
		ms.Preanim,
		ms.Character,
		ms.Emote,
		ms.Message,
		string(ms.Side),
		ms.SfxName,
		packetutil.Itoa(packetutil.EmoteModifierToWire[ms.EmoteModifier]),
		ms.CharID,
		ms.SfxDelay,
		shoutWire(ms.ShoutModifier, ms.ShoutName),
		ms.Evidence,
		packetutil.Itoa(packetutil.FlipToWire[ms.Flip]),
		ms.Realization,
		packetutil.Itoa(packetutil.TextColorToWire[ms.TextColor]),
		ms.Showname,
		ms.PairedCharID,
		ms.Offset,
		ms.NoninterruptingPreanim,
		ms.SfxLooping,
		ms.Screenshake,
		ms.FramesShake,
		ms.FramesRealization,
		ms.FramesSfx,
		ms.Additive,
		ms.Effect,
	}
	if ms.Blips != "" {
		args = append(args, ms.Blips)
	}
	return args
}

// Args returns the server -> client MS body (30 fields + optional Blips).
func (ms *MSToClient) Args() []string {
	args := []string{
		packetutil.Itoa(packetutil.DeskModifierToWire[ms.DeskModifier]),
		ms.Preanim,
		ms.Character,
		ms.Emote,
		ms.Message,
		string(ms.Side),
		ms.SfxName,
		packetutil.Itoa(packetutil.EmoteModifierToWire[ms.EmoteModifier]),
		ms.CharID,
		ms.SfxDelay,
		shoutWire(ms.ShoutModifier, ms.ShoutName),
		ms.Evidence,
		packetutil.Itoa(packetutil.FlipToWire[ms.Flip]),
		ms.Realization,
		packetutil.Itoa(packetutil.TextColorToWire[ms.TextColor]),
		ms.Showname,
		ms.PairedCharID,
		ms.PairedName,
		ms.PairedEmote,
		ms.Offset,
		ms.PairedOffset,
		packetutil.Itoa(packetutil.FlipToWire[ms.PairedFlip]),
		ms.NoninterruptingPreanim,
		ms.SfxLooping,
		ms.Screenshake,
		ms.FramesShake,
		ms.FramesRealization,
		ms.FramesSfx,
		ms.Additive,
		ms.Effect,
	}
	if ms.Blips != "" {
		args = append(args, ms.Blips)
	}
	return args
}

// JSONExtra returns the JSON-only MS fields: the multi-pair "additional_chars"
// list, or nil when there are no extra partners.
func (ms *MSToClient) JSONExtra() map[string]any {
	if len(ms.AdditionalChars) == 0 {
		return nil
	}
	return map[string]any{"additional_chars": ms.AdditionalChars}
}

// ServerString joins Args with '#'.
func (ms *MSToClient) ServerString() string {
	return strings.Join(ms.Args(), "#")
}
