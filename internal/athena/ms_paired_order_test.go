package athena

import (
	"encoding/json"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// pairedOrderMS builds a minimal valid MSToClient so encodeMSJSON has all the
// fields the schema expects; only PairedCharID varies per test.
func pairedOrderMS(pairedCharID string) *MSToClient {
	return &MSToClient{
		DeskModifier:  aolib.DeskModifierShown,
		Character:     "lucifer",
		Message:       "hi",
		Side:          aolib.SideDef,
		EmoteModifier: aolib.EmoteModifierNoPreanim,
		CharID:        "3376",
		SfxDelay:      "0",
		ShoutModifier: aolib.ShoutModifierNone,
		Evidence:      "0",
		Flip:          aolib.FlipNone,
		Realization:   "0",
		TextColor:     aolib.TextColorWhite,
		PairedCharID:  pairedCharID,
		PairedFlip:    aolib.FlipNone,
		Offset:        "0&0",
		PairedOffset:  "0&0",
		Effect:        "",
	}
}

// TestMSJSONPairedOrderBridgesSuffix pins that the classic "pid^order" suffix
// reaches JSON clients as paired_charid + paired_order (separate fields), so a
// FantaCode speaker's pair z-order survives the JSON bridge.
func TestMSJSONPairedOrderBridgesSuffix(t *testing.T) {
	raw, err := packetutil.Encode(pairedOrderMS("4^1"), aolib.WireJSON)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if got["paired_charid"] != float64(4) {
		t.Errorf("paired_charid = %#v, want 4 in %s", got["paired_charid"], raw)
	}
	if got["paired_order"] != float64(1) {
		t.Errorf("paired_order = %#v, want 1 in %s", got["paired_order"], raw)
	}
}

// TestMSJSONBarePairedIDStaysBare pins the default-order case: a bare partner
// id (no ^ suffix) serializes paired_order as 0, never a spurious suffix.
func TestMSJSONBarePairedIDStaysBare(t *testing.T) {
	raw, err := packetutil.Encode(pairedOrderMS("4"), aolib.WireJSON)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if got["paired_charid"] != float64(4) {
		t.Errorf("paired_charid = %#v, want 4", got["paired_charid"])
	}
	if o, ok := got["paired_order"]; ok && o != float64(0) {
		t.Errorf("paired_order = %#v, want absent or 0", got["paired_order"])
	}
}
