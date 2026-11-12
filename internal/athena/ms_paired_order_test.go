package athena

import (
	"encoding/json"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/area"
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

// TestPairIDWithOrder pins the suffix carried across a force-pair id rewrite:
// only a real partner's "^1" survives; bare ids, explicit "^0", unpaired ids
// and out-of-range orders normalize to the bare partner id.
func TestPairIDWithOrder(t *testing.T) {
	cases := []struct{ name, sent, want string }{
		{"speaker behind", "4^1", "7^1"},
		{"speaker behind on slot zero", "0^1", "7^1"},
		{"bare partner", "4", "7"},
		{"explicit default", "4^0", "7"},
		{"unpaired with order", "-1^1", "7"},
		{"out of range order", "4^2", "7"},
		{"empty", "", "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pairIDWithOrder("7", tc.sent); got != tc.want {
				t.Fatalf("pairIDWithOrder(7, %q) = %q, want %q", tc.sent, got, tc.want)
			}
		})
	}
}

// TestForcePairInjectionKeepsPairOrder pins the fix for normal /pair pairing:
// the force-pair branch rewrites the partner id on every MS, but it must keep
// the client's "^1" so "To behind" survives.
func TestForcePairInjectionKeepsPairOrder(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	speaker := newPairGroupTestClient(1, 0, a, "A")
	partner := newPairGroupTestClient(2, 1, a, "B")
	speaker.SetCharID(0)
	partner.SetCharID(1)

	for _, c := range []*Client{speaker, partner} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	speaker.SetForcePairUID(partner.Uid())
	partner.SetForcePairUID(speaker.Uid())

	ms := &MSToClient{PairedCharID: "1^1", Side: aolib.SideDef}
	applyForcePairInjection(speaker, ms)
	if ms.PairedCharID != "1^1" {
		t.Fatalf("force-pair PairedCharID = %q, want %q", ms.PairedCharID, "1^1")
	}
	if speaker.PairWantedID() != partner.CharID() {
		t.Errorf("speaker wanted id = %d, want %d", speaker.PairWantedID(), partner.CharID())
	}
	if partner.PairWantedID() != speaker.CharID() {
		t.Errorf("partner wanted id = %d, want %d", partner.PairWantedID(), speaker.CharID())
	}

	// A bare incoming id keeps the default order.
	ms = &MSToClient{PairedCharID: "1", Side: aolib.SideDef}
	applyForcePairInjection(speaker, ms)
	if ms.PairedCharID != "1" {
		t.Fatalf("bare force-pair PairedCharID = %q, want %q", ms.PairedCharID, "1")
	}
}

// TestForcePairInjectionWithoutPairLeavesID pins that the helper is a no-op
// outside a force pair.
func TestForcePairInjectionWithoutPairLeavesID(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)

	speaker := newPairGroupTestClient(1, 0, a, "A")
	speaker.SetCharID(0)
	speaker.SetForcePairUID(-1)
	clients.AddClient(speaker)
	clients.RegisterUID(speaker)

	ms := &MSToClient{PairedCharID: "1^1", Side: aolib.SideDef}
	applyForcePairInjection(speaker, ms)
	if ms.PairedCharID != "1^1" {
		t.Fatalf("unpaired PairedCharID = %q, want unchanged", ms.PairedCharID)
	}
}
