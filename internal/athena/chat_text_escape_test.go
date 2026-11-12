package athena

import (
	"encoding/json"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// TestCTJSONCarriesRawText pins the OOC regression: a CTToClient message with
// reserved characters must reach JSON-mode clients as raw text, never as the
// AO2 escape tokens. aolib.CTToClient.Args() escapes for FantaCode, so the
// struct fields must be left raw.
func TestCTJSONCarriesRawText(t *testing.T) {
	out := &aolib.CTToClient{Name: "OOC", Message: "50% off #1 & more $$", IsFromServer: true}
	raw, err := packetutil.Encode(out, aolib.WireJSON)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	var got struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if got.Name != "OOC" {
		t.Errorf("JSON name = %q, want raw %q", got.Name, "OOC")
	}
	if got.Message != "50% off #1 & more $$" {
		t.Errorf("JSON message = %q, want raw %q", got.Message, "50% off #1 & more $$")
	}
}

// TestCTFantaEscapesReservedText confirms the FantaCode path still escapes the
// reserved characters after the JSON fix, so the injection defence is intact.
func TestCTFantaEscapesReservedText(t *testing.T) {
	out := &aolib.CTToClient{Name: "OOC", Message: "50% off #1", IsFromServer: true}
	raw, err := packetutil.Encode(out, aolib.WireFanta)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	want := "CT#OOC#50<percent> off <num>1#1#%"
	if string(raw) != want {
		t.Fatalf("Fanta = %q, want %q", raw, want)
	}
}

// TestMSJSONDecodesEncodedText pins the IC side of the same bug: MSToClient
// stores its text fields AO2-encoded (raw-wire form), so the JSON codec must
// decode them before marshaling.
func TestMSJSONDecodesEncodedText(t *testing.T) {
	ms := &MSToClient{
		DeskModifier:  aolib.DeskModifierShown,
		Character:     encode("lucifer"),
		Emote:         encode("smirk"),
		Message:       encode("50% off #1 & more"),
		Side:          aolib.SideDef,
		EmoteModifier: aolib.EmoteModifierNoPreanim,
		CharID:        "3376",
		SfxDelay:      "0",
		ShoutModifier: aolib.ShoutModifierNone,
		Evidence:      "0",
		Flip:          aolib.FlipNone,
		Realization:   "0",
		TextColor:     aolib.TextColorWhite,
		Showname:      encode("A&B"),
		PairedCharID:  "-1",
		PairedFlip:    aolib.FlipNone,
		Offset:        "0&0",
		PairedOffset:  "0&0",
		Effect:        "",
	}
	raw, err := packetutil.Encode(ms, aolib.WireJSON)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	var got struct {
		Character string `json:"character"`
		Message   string `json:"message"`
		Showname  string `json:"showname"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if got.Character != "lucifer" {
		t.Errorf("JSON character = %q, want %q", got.Character, "lucifer")
	}
	if got.Message != "50% off #1 & more" {
		t.Errorf("JSON message = %q, want %q", got.Message, "50% off #1 & more")
	}
	if got.Showname != "A&B" {
		t.Errorf("JSON showname = %q, want %q", got.Showname, "A&B")
	}
}
