package athena

import (
	"encoding/json"
	"strings"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// msCodec registers Nyathena's custom MS (in-character) packet as a both-wire
// codec. Nyathena keeps its own MSToClient/MSToServer types (string-typed wire
// fields, a custom-shout name, blips, and the JSON-only additional_chars), so
// aolib's generated MS types can't represent it. The codec bridges the two: the
// FantaCode form uses Nyathena's Args()/Parse*, while the JSON form converts to
// aolib's typed MSToClient/MSToServer so the wire is the canonical meta shape.
func msCodec() packetutil.Codec {
	return packetutil.Codec{
		EncodeFanta: func(p any) ([]string, error) { return p.(*MSToClient).Args(), nil },
		DecodeFanta: func(args []string) (any, error) { return ParseMSToServer(args), nil },
		EncodeJSON:  func(p any) ([]byte, error) { return encodeMSJSON(p.(*MSToClient)) },
		DecodeJSON: func(raw []byte) (any, error) {
			v, err := aolib.Decode(raw, aolib.WireJSON)
			if err != nil {
				return nil, err
			}
			// aolib.Decode yields aolib's typed MS; fold it back to Nyathena's
			// MSToServer via the shared positional Args so the codec's T matches.
			return ParseMSToServer(v.(aolib.Outgoing).Args()), nil
		},
	}
}

// encodeMSJSON converts Nyathena's string-typed MSToClient into aolib's typed
// MSToClient and marshals it, then injects the JSON-only additional_chars list
// when present. The custom-shout name and blips are FantaCode-only extensions
// and are intentionally omitted from the JSON form.
func encodeMSJSON(ms *MSToClient) ([]byte, error) {
	pid, pairOrder := splitPairedID(ms.PairedCharID)
	typed := aolib.MSToClient{
		DeskModifier:           ms.DeskModifier,
		Preanim:                ms.Preanim,
		Character:              ms.Character,
		Emote:                  ms.Emote,
		Message:                ms.Message,
		Side:                   ms.Side,
		SfxName:                ms.SfxName,
		EmoteModifier:          ms.EmoteModifier,
		CharID:                 packetutil.AtoiOrZero(ms.CharID),
		SfxDelay:               packetutil.AtoiOrZero(ms.SfxDelay),
		ShoutModifier:          ms.ShoutModifier,
		EvidenceID:             packetutil.AtoiOrZero(ms.Evidence),
		Flip:                   ms.Flip,
		Realization:            packetutil.WireToBool(ms.Realization),
		TextColor:              ms.TextColor,
		Showname:               ms.Showname,
		PairedCharID:           pid,
		PairedOrder:            pairOrder,
		PairedName:             ms.PairedName,
		PairedEmote:            ms.PairedEmote,
		Offset:                 parseMSOffset(ms.Offset),
		PairedOffset:           parseMSOffset(ms.PairedOffset),
		PairedFlip:             ms.PairedFlip,
		NoninterruptingPreanim: packetutil.WireToBool(ms.NoninterruptingPreanim),
		SfxLooping:             packetutil.WireToBool(ms.SfxLooping),
		Screenshake:            packetutil.WireToBool(ms.Screenshake),
		FramesShake:            ms.FramesShake,
		FramesRealization:      ms.FramesRealization,
		FramesSfx:              ms.FramesSfx,
		Additive:               packetutil.WireToBool(ms.Additive),
		Effect:                 parseMSEffect(ms.Effect),
	}

	b, err := aolib.Encode(&typed, aolib.WireJSON)
	if err != nil {
		return nil, err
	}
	if len(ms.AdditionalChars) > 0 {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err != nil {
			return nil, err
		}
		ac, _ := json.Marshal(ms.AdditionalChars)
		obj["additional_chars"] = ac
		b, err = json.Marshal(obj)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// parseMSOffset converts Nyathena's "x&y" offset string into aolib's Offset.
func parseMSOffset(s string) aolib.Offset {
	s = strings.ReplaceAll(s, "<and>", "&")
	parts := strings.SplitN(s, "&", 2)
	o := aolib.Offset{X: packetutil.AtoiOrZero(parts[0])}
	if len(parts) == 2 {
		o.Y = packetutil.AtoiOrZero(parts[1])
	}
	return o
}

// splitPairedID parses the classic "pid^order" pair suffix into its two parts.
// The FantaCode wire packs pair order onto paired_charid as a "^"-joined suffix
// (4^1 = partner 4, speaker behind); on the JSON wire the two are separate
// fields (paired_charid + paired_order), so this bridges them.
func splitPairedID(raw string) (pid, order int) {
	pidStr, orderStr, hasOrder := strings.Cut(raw, "^")
	pid = packetutil.AtoiOrZero(pidStr)
	if hasOrder {
		order = packetutil.AtoiOrZero(orderStr)
	}
	return pid, order
}

// parseMSEffect converts Nyathena's "name|folder|sound" effect string into
// aolib's Effect struct.
func parseMSEffect(s string) aolib.Effect {
	if s == "" {
		return aolib.Effect{}
	}
	parts := strings.SplitN(s, "|", 3)
	e := aolib.Effect{Name: aolib.UnescapeFanta(parts[0])}
	if len(parts) > 1 {
		e.Folder = aolib.UnescapeFanta(parts[1])
	}
	if len(parts) > 2 {
		e.Sound = aolib.UnescapeFanta(parts[2])
	}
	return e
}
