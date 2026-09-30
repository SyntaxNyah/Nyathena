package packet

import "testing"

func TestShoutParsesCustomSuffix(t *testing.T) {
	cases := map[string]ShoutModifier{"0": ShoutModifierNone, "1": ShoutModifierHoldIt, "2": ShoutModifierObjection, "3": ShoutModifierTakeThat, "2&myshout": ShoutModifierObjection, "4&custom&x": ShoutModifierCustom}
	for in, want := range cases {
		ms := &MSToClient{ShoutModifier: in}
		got, err := ms.Shout()
		if err != nil || got != want {
			t.Errorf("Shout(%q) = %d, %v; want %d, nil", in, got, err, want)
		}
	}
	if _, err := (&MSToClient{ShoutModifier: "abc"}).Shout(); err == nil {
		t.Error("Shout(\"abc\") returned no error")
	}
}
