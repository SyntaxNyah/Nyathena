package athena

import (
	"encoding/json"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// TestMSJSONTypedOutput verifies Nyathena's custom MS codec converts its
// string-typed wire fields into aolib's typed JSON form.
func TestMSJSONTypedOutput(t *testing.T) {
	ms := &MSToClient{
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
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if got["char_id"] != float64(3376) {
		t.Errorf("char_id = %#v, want number 3376 in %s", got["char_id"], raw)
	}
	if got["desk_modifier"] != "shown" {
		t.Errorf("desk_modifier = %#v, want %q", got["desk_modifier"], "shown")
	}
	if got["realization"] != false {
		t.Errorf("realization = %#v, want false", got["realization"])
	}
}

// TestAolibJSONTypedOutput pins the "no type nonsense" JSON contract that drove
// the migration to aolib-go: enum fields carry their string name, integer
// fields are JSON numbers, and boolean fields are real booleans.
func TestAolibJSONTypedOutput(t *testing.T) {
	cases := []struct {
		name string
		p    aolib.Outgoing
		want map[string]any
	}{
		{
			name: "ARUP enum update_type + string tail",
			p:    &aolib.ARUP{UpdateType: aolib.AreaUpdateTypeLocked, UpdateData: []string{"FREE", "LOCKED"}},
			want: map[string]any{
				"$header":     "ARUP",
				"update_type": "locked",
				"update_data": []any{"FREE", "LOCKED"},
			},
		},
		{
			name: "PV numeric player_id/char_id",
			p:    &aolib.PV{PlayerID: 0, CharID: 4},
			want: map[string]any{
				"$header":   "PV",
				"player_id": float64(0),
				"char_id":   float64(4),
			},
		},
		{
			name: "MA numeric player_id/duration_minutes",
			p:    &aolib.MA{PlayerID: 7, DurationMinutes: 30, Reason: "test"},
			want: map[string]any{
				"$header":          "MA",
				"player_id":        float64(7),
				"duration_minutes": float64(30),
				"reason":           "test",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := packetutil.Encode(tc.p, aolib.WireJSON)
			if err != nil {
				t.Fatalf("Encode error: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal %s: %v", raw, err)
			}
			for k, want := range tc.want {
				g, ok := got[k]
				if !ok {
					t.Fatalf("missing key %q in %s", k, raw)
				}
				gj, _ := json.Marshal(g)
				wj, _ := json.Marshal(want)
				if string(gj) != string(wj) {
					t.Fatalf("key %q = %s, want %s in %s", k, gj, wj, raw)
				}
			}
		})
	}
}
