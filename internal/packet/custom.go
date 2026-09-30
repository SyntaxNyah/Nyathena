package packet

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Codec fully defines one packet header's wire form in BOTH formats (FantaCode
// and JSON). It is the same idea as a canonical packet's schema, only
// registered at runtime by a caller for a custom (non-canonical) header — e.g.
// Nyathena's TT / SETCASE / CASEA, which the canonical spec does not model.
//
// The rule: a codec MUST implement both FantaCode and JSON — there is no
// JSON-only or Fanta-only custom packet. Each function maps the raw wire
// payload for one format to or from the caller's typed packet value.
//
// The library owns framing. On FantaCode it strips the header and trailing "%"
// and hands the codec the positional fields, then re-wraps HEADER#a#b#%. On
// JSON it guarantees the "$header" key; the codec sees and produces the object
// text. EscapeFanta / UnescapeFanta are exposed for codecs whose string fields
// may contain the chat metacharacters.
type Codec struct {
	// EncodeFanta returns the FantaCode positional fields (no header, no
	// trailing "%") for a typed packet.
	EncodeFanta func(p any) ([]string, error)
	// DecodeFanta turns FantaCode positional fields into a typed packet.
	DecodeFanta func(args []string) (any, error)
	// EncodeJSON returns the JSON object text for a typed packet; the library
	// injects "$header" if the codec omits it.
	EncodeJSON func(p any) (string, error)
	// DecodeJSON turns the JSON object text (including "$header") into a typed
	// packet.
	DecodeJSON func(raw string) (any, error)
}

var codecs = map[string]Codec{}

// RegisterCodec registers (or overrides) the codec for a header. It panics if
// any of the four wire directions is missing, enforcing the both-formats rule.
func RegisterCodec(header string, c Codec) {
	if c.EncodeFanta == nil || c.DecodeFanta == nil || c.EncodeJSON == nil || c.DecodeJSON == nil {
		panic(fmt.Sprintf("packet: codec for %q must implement both FantaCode and JSON (all four functions)", header))
	}
	codecs[header] = c
}

// encodeCustom serializes a typed custom packet through its registered codec in
// the given wire mode, applying framing.
func encodeCustom(header string, p any, mode WireMode) ([]byte, error) {
	c, ok := codecs[header]
	if !ok {
		return nil, fmt.Errorf("packet: no codec registered for header %q", header)
	}
	switch mode {
	case WireFanta:
		args, err := c.EncodeFanta(p)
		if err != nil {
			return nil, err
		}
		return frameFanta(header, args), nil
	case WireJSON:
		raw, err := c.EncodeJSON(p)
		if err != nil {
			return nil, err
		}
		return ensureHeader([]byte(raw), header)
	default:
		return nil, fmt.Errorf("packet: unknown wire mode %d", mode)
	}
}

// ensureHeader guarantees the JSON object carries a matching "$header" key.
func ensureHeader(raw []byte, header string) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("packet: codec JSON for %q is not an object: %w", header, err)
	}
	h, _ := json.Marshal(header)
	obj["$header"] = h
	return json.Marshal(obj)
}

// decodeCustom attempts to decode raw through a registered codec. ok reports
// whether a codec owns the header, so the caller can fall back to the generated
// registry when false.
func decodeCustom(raw []byte, mode WireMode) (header string, p any, ok bool, err error) {
	switch mode {
	case WireFanta:
		pkt, e := NewPacket(strings.TrimSuffix(string(raw), "%"))
		if e != nil {
			return "", nil, false, e
		}
		c, found := codecs[pkt.Header]
		if !found {
			return "", nil, false, nil
		}
		p, e = c.DecodeFanta(pkt.Body)
		return pkt.Header, p, true, e
	case WireJSON:
		pkt, e := ParseJSON(string(raw))
		if e != nil {
			return "", nil, false, e
		}
		c, found := codecs[pkt.Header]
		if !found {
			return "", nil, false, nil
		}
		p, e = c.DecodeJSON(string(raw))
		return pkt.Header, p, true, e
	default:
		return "", nil, false, fmt.Errorf("packet: unknown wire mode %d", mode)
	}
}

// EncodeJSON serializes an Outgoing to JSON: codec-registered headers route
// through their codec's EncodeJSON (named fields), everything else through the
// schema-based BuildJSON. Returns nil when the packet can't be encoded.
func EncodeJSON(p Outgoing) []byte {
	if _, ok := codecs[p.Header()]; ok {
		b, err := encodeCustom(p.Header(), p, WireJSON)
		if err != nil {
			return nil
		}
		return b
	}
	return BuildJSON(p.Header(), p.Args())
}

// CodecJSONToBody decodes a JSON frame for a codec-registered header into the
// positional body the packet handlers parse (via the codec's DecodeJSON →
// EncodeFanta), so non-canonical packets work over JSON too. ok is false when
// no codec owns the header (or the frame fails to decode).
func CodecJSONToBody(header, raw string) ([]string, bool) {
	c, ok := codecs[header]
	if !ok {
		return nil, false
	}
	p, err := c.DecodeJSON(raw)
	if err != nil {
		return nil, false
	}
	args, err := c.EncodeFanta(p)
	if err != nil {
		return nil, false
	}
	return args, true
}
