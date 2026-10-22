package athena

// Nyathena fork addition: TimeLeap (TL) tests. The load-bearing case is
// TestTimeLeapRequiresBothGuards — GP's shortcut (JSON mode alone) is exactly
// what TL must not copy, and it is the one property a future refactor is most
// likely to break silently.

import (
	"strings"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

func TestTimeLeapRequiresBothGuards(t *testing.T) {
	cases := []struct {
		name     string
		jsonMode bool
		features []string
		want     bool
	}{
		{"json + advertised", true, []string{"timeleap"}, true},
		{"json only", true, nil, false},                       // the grouppair bug
		{"fanta + advertised", false, []string{"timeleap"}, false}, // TL is JSON-only
		{"fanta, nothing advertised", false, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{}
			c.jsonMode.Store(tc.jsonMode)
			c.SetFeatures(tc.features)
			if got := c.supportsTimeLeap(); got != tc.want {
				t.Fatalf("supportsTimeLeap() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTLPacketEncodesAsJSONWithHeader pins the wire shape: packetutil adds
// $header itself, so the codec must not (and the frame must stay JSON).
func TestTLPacketEncodesAsJSONWithHeader(t *testing.T) {
	p := &TLPacket{Op: "restore", SnapshotID: "wr-1", Label: "radio", Mode: tlModeAuthoritative}

	raw, err := packetutil.Encode(p, aolib.WireJSON)
	if err != nil {
		t.Fatalf("JSON encode: %v", err)
	}
	if !strings.HasPrefix(string(raw), "{") {
		t.Fatalf("frame is not JSON: %s", raw)
	}
	if !strings.Contains(string(raw), `"$header":"TL"`) {
		t.Fatalf("frame missing $header TL: %s", raw)
	}
	if !strings.Contains(string(raw), `"snapshot_id":"wr-1"`) {
		t.Fatalf("frame missing snapshot_id: %s", raw)
	}
}

// TestTLFantaEncodeErrors verifies the codec refuses FantaCode instead of
// nil-panicking (the hazard gpCodec leaves open).
func TestTLFantaEncodeErrors(t *testing.T) {
	if _, err := packetutil.Encode(&TLPacket{}, aolib.WireFanta); err == nil {
		t.Fatal("TL encoded as FantaCode; it is JSON-only")
	}
}

// TestTLCaptureAndStore covers the snapshot's room fields, the transcript
// mapping and the store round-trip.
func TestTLCaptureAndStore(t *testing.T) {
	a := area.NewArea(area.AreaData{Name: "Radio Tower", Bg: "sg_rt"}, len(getCharacters()), 10, area.EviAny)

	tlLogFor(a).append(tlLogLine{
		UID: 7, CharID: 3, Name: "okabe", Emote: "normal", Side: "def",
		Text: "El Psy Kongroo.", Color: 0,
	})

	issuer := &Client{uid: 7, char: 3, pos: "def"}
	p := captureTimeLeap(a, "radio", tlModeAuthoritative, issuer)

	if p.SnapshotID == "" || !strings.HasPrefix(p.SnapshotID, "wr-") {
		t.Fatalf("snapshot id = %q", p.SnapshotID)
	}
	if p.State.Area.Name != "Radio Tower" || p.State.Area.Background != "sg_rt" {
		t.Fatalf("area = %+v", p.State.Area)
	}
	if p.State.Area.Side != "def" {
		t.Fatalf("side = %q, want the issuer's pos", p.State.Area.Side)
	}
	if p.TTLSeconds != int(tlDefaultTTL.Seconds()) {
		t.Fatalf("ttl = %d", p.TTLSeconds)
	}
	if len(p.State.Log) != 1 || p.State.Log[0].Text != "El Psy Kongroo." || p.State.Log[0].CharID != 3 {
		t.Fatalf("log = %+v", p.State.Log)
	}

	storeTimeLeap(a, p)
	got, ok := lookupTimeLeap(a, p.SnapshotID)
	if !ok || got != p {
		t.Fatalf("lookup(%q) = %v, %v", p.SnapshotID, got, ok)
	}
	if ids := listTimeLeaps(a); len(ids) != 1 || !strings.Contains(ids[0], "radio") {
		t.Fatalf("list = %v", ids)
	}
	if _, ok := lookupTimeLeap(a, "wr-does-not-exist"); ok {
		t.Fatal("unknown id resolved")
	}
}

// TestTLTranscriptTrimsToCap keeps the per-area buffer bounded, which is what
// keeps the TL frame under the socket read cap.
func TestTLTranscriptTrimsToCap(t *testing.T) {
	a := area.NewArea(area.AreaData{Name: "Cap"}, len(getCharacters()), 10, area.EviAny)
	log := tlLogFor(a)
	for i := 0; i < tlLogMax+10; i++ {
		log.append(tlLogLine{CharID: i, Text: "line"})
	}
	if got := log.len(); got != tlLogMax {
		t.Fatalf("retained %d lines, want %d", got, tlLogMax)
	}
	snap := log.snapshot()
	if snap[0].CharID != 10 {
		t.Fatalf("oldest retained CharID = %d, want 10 (first 10 should have rotated out)", snap[0].CharID)
	}
}
