package packet

// This file holds the public extension API. The library strictly models
// canonical aolib-meta; servers that need nonstandard packets (or nonstandard
// parsing of a standard header) register them here instead of editing the
// generated registry.

// RegisterDecoder registers (or overrides) the client→server decoder for a
// packet header. The decoder receives the positional body with the header and
// trailing '#' already stripped.
func RegisterDecoder(header string, dec Decoder) {
	c2sDecoders[header] = dec
}

// RegisterServerDecoder registers (or overrides) the server→client decoder for
// a packet header.
func RegisterServerDecoder(header string, dec Decoder) {
	s2cDecoders[header] = dec
}
