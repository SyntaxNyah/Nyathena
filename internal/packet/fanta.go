package packet

// Fanta wire-format helpers, used by the generated Args()/Parse* methods and
// the hand-written MS packet. Mirrors aolib-ts's fanta walker:
//
//   - strings escape #/&/%/$ as <num>/<and>/<percent>/<dollar>
//   - enums with x-wire-ints map to their legacy integer
//   - objects (Offset) join sub-tokens with &
//   - booleans are "1"/"0"

import (
	"strconv"
	"strings"
)

// EscapeFanta escapes the chat-format metacharacters on encode.
func EscapeFanta(s string) string {
	s = strings.ReplaceAll(s, "#", "<num>")
	s = strings.ReplaceAll(s, "&", "<and>")
	s = strings.ReplaceAll(s, "%", "<percent>")
	s = strings.ReplaceAll(s, "$", "<dollar>")
	return s
}

// UnescapeFanta inverts EscapeFanta on decode.
func UnescapeFanta(s string) string {
	s = strings.ReplaceAll(s, "<num>", "#")
	s = strings.ReplaceAll(s, "<and>", "&")
	s = strings.ReplaceAll(s, "<percent>", "%")
	s = strings.ReplaceAll(s, "<dollar>", "$")
	return s
}

// Itoa is a short alias for strconv.Itoa.
func Itoa(n int) string { return strconv.Itoa(n) }

// AtoiOrZero parses a base-10 integer, returning 0 on empty/malformed input.
func AtoiOrZero(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// GetStr returns body[i] or "" if i is out of range.
func GetStr(body []string, i int) string {
	if i < len(body) {
		return body[i]
	}
	return ""
}

// BoolToWire encodes a boolean as "1"/"0".
func BoolToWire(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// WireToBool decodes a "1"/"0" token to a boolean.
func WireToBool(s string) bool { return s == "1" }

// OffsetToWire encodes an Offset as "x&y".
func OffsetToWire(o Offset) string { return Itoa(o.X) + "&" + Itoa(o.Y) }

// OffsetFromWire decodes "x&y" (tolerating the legacy <and> escape) into an
// Offset.
func OffsetFromWire(s string) Offset {
	s = strings.ReplaceAll(s, "<and>", "&")
	parts := strings.SplitN(s, "&", 2)
	o := Offset{X: AtoiOrZero(parts[0])}
	if len(parts) == 2 {
		o.Y = AtoiOrZero(parts[1])
	}
	return o
}

// IntsToStrs maps an int slice to its decimal string form.
func IntsToStrs(ns []int) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = Itoa(n)
	}
	return out
}

// StrsToInts maps a string slice to ints (lenient).
func StrsToInts(ss []string) []int {
	out := make([]int, len(ss))
	for i, s := range ss {
		out[i] = AtoiOrZero(s)
	}
	return out
}
