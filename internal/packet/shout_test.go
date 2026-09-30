package packet

import "testing"

func TestShoutParsesCustomSuffix(t *testing.T) {
	cases := map[string]ShoutModifier{"0": ShoutModifierNone, "1": ShoutModifierHoldIt, "2": ShoutModifierObjection, "3": ShoutModifierTakeThat, "2&myshout": ShoutModifierObjection, "4&custom&x": ShoutModifierCustom}
	for in, want := range cases {
		if got, _ := parseShout(in); got != want {
			t.Errorf("parseShout(%q) = %q; want %q", in, got, want)
		}
	}
	if _, name := parseShout("4&custom&x"); name != "custom&x" {
		t.Errorf("parseShout custom name = %q; want %q", name, "custom&x")
	}
}
