package packet

import "testing"

func TestShoutParsesCustomSuffix(t *testing.T) {
	cases := map[string]int{"0": 0, "1": 1, "2": 2, "3": 3, "2&myshout": 2, "4&custom&x": 4}
	for in, want := range cases {
		ms := &MSPacket{ShoutModifier: in}
		got, err := ms.Shout()
		if err != nil || got != want {
			t.Errorf("Shout(%q) = %d, %v; want %d, nil", in, got, err, want)
		}
	}
	if _, err := (&MSPacket{ShoutModifier: "abc"}).Shout(); err == nil {
		t.Error("Shout(\"abc\") returned no error")
	}
}
