package packetutil

import (
	"encoding/json"
	"fmt"
	"strings"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// Codec is a both-wire codec for a custom header. It is packetutil's own
// type-erased shape (aolib 2.6.0's typed RegisterPacket is wrapped below), kept
// so the outbound encode path can dispatch any payload without a type switch.
type Codec struct {
	EncodeFanta func(p any) ([]string, error)
	DecodeFanta func(args []string) (any, error)
	EncodeJSON  func(p any) ([]byte, error)
	DecodeJSON  func(raw []byte) (any, error)
}

// customCodecs holds the registered codecs for Nyathena's nonstandard headers.
// aolib keeps its own registry private, so Nyathena tracks them here for its
// own outbound wire dispatch.
var customCodecs = map[string]Codec{}

// Register installs a both-wire codec for a custom header in aolib (typed, for
// the session's inbound decode + OnCustom dispatch) and in packetutil's table
// (type-erased, for the outbound encode path). T is the inbound payload type the
// codec's Decode* functions return; for bidirectional headers (MS, VS_JOIN/…) the
// outbound direction uses a different payload, which packetutil's own Encode
// handles, so the aolib-registered Encode closures are simply unused.
func Register[T any](header string, c Codec) {
	customCodecs[header] = c
	aolib.RegisterPacket(header, aolib.PacketOptions[T]{
		Fanta: &aolib.Fanta[T]{
			Encode: func(t T) ([]string, error) { return c.EncodeFanta(t) },
			Decode: func(args []string) (T, error) {
				v, err := c.DecodeFanta(args)
				if err != nil {
					var zero T
					return zero, err
				}
				return v.(T), nil
			},
		},
		JSON: &aolib.JSONForm[T]{
			Encode: func(t T) ([]byte, error) { return c.EncodeJSON(t) },
			Decode: func(raw []byte) (T, error) {
				v, err := c.DecodeJSON(raw)
				if err != nil {
					var zero T
					return zero, err
				}
				return v.(T), nil
			},
		},
	})
}

// RegisterLocal registers a codec in packetutil's outbound dispatch table only,
// without aolib.RegisterPacket. Used for headers aolib already models canonically
// (MS) but whose Nyathena outbound shape carries extra fields (blips,
// additional_chars) the canonical type can't represent.
func RegisterLocal(header string, c Codec) {
	customCodecs[header] = c
}

// IsCustom reports whether a header has a Nyathena custom codec.
func IsCustom(header string) bool {
	_, ok := customCodecs[header]
	return ok
}

// Encode serializes a typed packet, routing custom headers through their codec
// and everything else through aolib's canonical encoder.
func Encode(p aolib.Outgoing, mode aolib.WireMode) ([]byte, error) {
	if c, ok := customCodecs[p.Header()]; ok {
		return encodeCustom(p.Header(), p, c, mode)
	}
	return aolib.Encode(p, mode)
}

// Decode parses a raw wire frame into its typed packet (custom headers through
// their codec, everything else through aolib's canonical decoder). Unrecognised
// headers fall back to *aolib.Packet.
func Decode(raw []byte, mode aolib.WireMode) (any, error) {
	if mode == aolib.WireFanta {
		header := ""
		if pkt, err := aolib.NewPacket(strings.TrimSuffix(string(raw), "%")); err == nil {
			header = pkt.Header
		}
		if c, ok := customCodecs[header]; ok {
			pkt, err := aolib.NewPacket(strings.TrimSuffix(string(raw), "%"))
			if err != nil {
				return nil, err
			}
			return c.DecodeFanta(pkt.Body)
		}
		return aolib.Decode(raw, mode)
	}

	header, err := aolib.ReadHeader(raw)
	if err != nil {
		return nil, err
	}
	if c, ok := customCodecs[header]; ok {
		return c.DecodeJSON(raw)
	}
	return aolib.Decode(raw, mode)
}

func encodeCustom(header string, p any, c Codec, mode aolib.WireMode) ([]byte, error) {
	switch mode {
	case aolib.WireFanta:
		args, err := c.EncodeFanta(p)
		if err != nil {
			return nil, err
		}
		return frameFanta(header, args), nil
	case aolib.WireJSON:
		raw, err := c.EncodeJSON(p)
		if err != nil {
			return nil, err
		}
		return ensureHeader(raw, header)
	default:
		return nil, fmt.Errorf("packetutil: unknown wire mode %d", mode)
	}
}

func frameFanta(header string, args []string) []byte {
	var b strings.Builder
	b.WriteString(header)
	for _, a := range args {
		b.WriteByte('#')
		b.WriteString(a)
	}
	b.WriteString("#%")
	return []byte(b.String())
}

func ensureHeader(raw []byte, header string) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("packetutil: codec JSON for %q is not an object: %w", header, err)
	}
	h, _ := json.Marshal(header)
	obj["$header"] = h
	return json.Marshal(obj)
}

// escapeWireArgs escapes positional args for Fanta framing so aolib's decoder
// can split on '#' without splitting the args' content. '&' is the sub-field
// separator for LE/SC, so it is preserved there — mirroring the encoderNoAnd
// Nyathena's own Fanta path uses for those two headers.
func escapeWireArgs(header string, args []string) []string {
	escapeAmp := header != "LE" && header != "SC"
	out := make([]string, len(args))
	for i, a := range args {
		a = strings.ReplaceAll(a, "#", "<num>")
		if escapeAmp {
			a = strings.ReplaceAll(a, "&", "<and>")
		}
		a = strings.ReplaceAll(a, "%", "<percent>")
		a = strings.ReplaceAll(a, "$", "<dollar>")
		out[i] = a
	}
	return out
}

// BuildJSONFromArgs encodes a type-erased (header, positional-args) packet to
// its JSON wire form. Custom headers route through their codec; canonical
// headers are decoded through aolib's public directional decoders (the single
// source of truth for the positional→typed mapping) and re-encoded as JSON.
// Unknown headers emit the bare envelope.
func BuildJSONFromArgs(header string, args []string) ([]byte, error) {
	if c, ok := customCodecs[header]; ok {
		p, err := c.DecodeFanta(args)
		if err != nil {
			return nil, err
		}
		return encodeCustom(header, p, c, aolib.WireJSON)
	}

	frame := frameFanta(header, escapeWireArgs(header, args))
	for _, decode := range []func([]byte, aolib.WireMode) (any, error){aolib.DecodeToClient, aolib.DecodeToServer} {
		p, err := decode(frame, aolib.WireFanta)
		if err != nil {
			continue
		}
		if o, ok := p.(aolib.Outgoing); ok {
			return aolib.Encode(o, aolib.WireJSON)
		}
	}

	// Unknown/empty packet: emit the bare envelope.
	obj := map[string]json.RawMessage{}
	h, _ := json.Marshal(header)
	obj["$header"] = h
	return json.Marshal(obj)
}
