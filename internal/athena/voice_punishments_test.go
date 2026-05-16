package athena

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// resetVoiceStutterState clears the package-level /voicestutter held-frame
// map so stutter tests don't bleed into one another.
func resetVoiceStutterState() {
	voiceStutterMu.Lock()
	voiceStutterHeld = map[int]string{}
	voiceStutterMu.Unlock()
}

// voicePunishedClient builds a bare client carrying one voice punishment.
// duration 0 = permanent.
func voicePunishedClient(uid int, pType PunishmentType, duration time.Duration) *Client {
	c := &Client{uid: uid}
	c.AddPunishmentBy(pType, duration, "test", IssuerMod)
	return c
}

// TestVoicePunishmentTypeRoundTrip checks that every voice punishment name
// parses to its enum and stringifies back, and that the five types are
// distinct from one another and from PunishmentNone.
func TestVoicePunishmentTypeRoundTrip(t *testing.T) {
	cases := map[string]PunishmentType{
		"voicemute":    PunishmentVoiceMute,
		"voicestatic":  PunishmentVoiceStatic,
		"voicegarble":  PunishmentVoiceGarble,
		"voicecutout":  PunishmentVoiceCutout,
		"voicestutter": PunishmentVoiceStutter,
	}
	seen := map[PunishmentType]bool{PunishmentNone: true}
	for name, want := range cases {
		if got := parsePunishmentType(name); got != want {
			t.Errorf("parsePunishmentType(%q) = %v, want %v", name, got, want)
		}
		if got := want.String(); got != name {
			t.Errorf("%v.String() = %q, want %q", want, got, name)
		}
		if seen[want] {
			t.Errorf("punishment type for %q collides with another type", name)
		}
		seen[want] = true
	}
}

// TestVoicePunishmentsDoNotTouchICText guards the invariant that voice
// punishments are inert in the IC text path — they only ever act in
// pktVSFrame.
func TestVoicePunishmentsDoNotTouchICText(t *testing.T) {
	const msg = "the witness is lying"
	for _, p := range []PunishmentType{
		PunishmentVoiceMute, PunishmentVoiceStatic, PunishmentVoiceGarble,
		PunishmentVoiceCutout, PunishmentVoiceStutter,
	} {
		if got := ApplyPunishmentToText(msg, p); got != msg {
			t.Errorf("ApplyPunishmentToText(_, %v) = %q, want unchanged", p, got)
		}
	}
}

// TestApplyVoiceFramePunishmentsNoPunishment confirms an unpunished speaker's
// frame is relayed verbatim.
func TestApplyVoiceFramePunishmentsNoPunishment(t *testing.T) {
	c := &Client{uid: 1}
	payload, relay := applyVoiceFramePunishments(c, 1, "OPUS")
	if !relay || payload != "OPUS" {
		t.Errorf("got (%q, %v), want (\"OPUS\", true)", payload, relay)
	}
}

// TestApplyVoiceFramePunishmentsMuteDropsAll confirms /voicemute drops every
// frame.
func TestApplyVoiceFramePunishmentsMuteDropsAll(t *testing.T) {
	c := voicePunishedClient(1, PunishmentVoiceMute, 0)
	for i := 0; i < 200; i++ {
		if payload, relay := applyVoiceFramePunishments(c, 1, "OPUS"); relay {
			t.Fatalf("frame %d relayed under /voicemute (payload %q)", i, payload)
		}
	}
}

// TestApplyVoiceFramePunishmentsStaticDropsSome confirms /voicestatic drops a
// chunk of frames but lets others through unmodified.
func TestApplyVoiceFramePunishmentsStaticDropsSome(t *testing.T) {
	c := voicePunishedClient(1, PunishmentVoiceStatic, 0)
	const n = 4000
	relayed := 0
	for i := 0; i < n; i++ {
		payload, relay := applyVoiceFramePunishments(c, 1, "OPUS")
		if relay {
			relayed++
			if payload != "OPUS" {
				t.Fatalf("/voicestatic mutated a relayed frame: %q", payload)
			}
		}
	}
	// Expectation ~40% relayed; wide bounds keep the test non-flaky.
	if relayed < 200 || relayed > n-200 {
		t.Errorf("/voicestatic relayed %d/%d frames — expected a partial drop", relayed, n)
	}
}

// TestApplyVoiceFramePunishmentsGarbleHarsherThanStatic confirms /voicegarble
// drops noticeably more frames than /voicestatic.
func TestApplyVoiceFramePunishmentsGarbleHarsherThanStatic(t *testing.T) {
	const n = 4000
	count := func(p PunishmentType) int {
		c := voicePunishedClient(1, p, 0)
		relayed := 0
		for i := 0; i < n; i++ {
			if _, relay := applyVoiceFramePunishments(c, 1, "OPUS"); relay {
				relayed++
			}
		}
		return relayed
	}
	static := count(PunishmentVoiceStatic)
	garble := count(PunishmentVoiceGarble)
	if garble >= static {
		t.Errorf("/voicegarble relayed %d, /voicestatic relayed %d — garble should be harsher", garble, static)
	}
	if garble == 0 {
		t.Errorf("/voicegarble dropped literally everything (%d) — expected a few frames through", garble)
	}
}

// TestVoiceCutoutMutedAtPhases checks the deterministic walkie-talkie gate.
func TestVoiceCutoutMutedAtPhases(t *testing.T) {
	cases := []struct {
		ms   int64
		uid  int
		want bool
	}{
		{0, 0, false},
		{voiceCutoutWindowMs - 1, 0, false},
		{voiceCutoutWindowMs, 0, true},
		{2*voiceCutoutWindowMs - 1, 0, true},
		{2 * voiceCutoutWindowMs, 0, false},
		{0, 1, true},  // UID offset flips the phase
		{0, 2, false}, // even UID offset is back in phase
		{voiceCutoutWindowMs, 1, false},
	}
	for _, tc := range cases {
		if got := voiceCutoutMutedAt(tc.ms, tc.uid); got != tc.want {
			t.Errorf("voiceCutoutMutedAt(%d, %d) = %v, want %v", tc.ms, tc.uid, got, tc.want)
		}
	}
}

// TestApplyVoiceFramePunishmentsCutoutOnlyDropsOrRelays confirms /voicecutout
// only ever drops a frame or relays it untouched — it never substitutes.
func TestApplyVoiceFramePunishmentsCutoutOnlyDropsOrRelays(t *testing.T) {
	c := voicePunishedClient(1, PunishmentVoiceCutout, 0)
	for i := 0; i < 500; i++ {
		payload, relay := applyVoiceFramePunishments(c, 1, "OPUS")
		if relay && payload != "OPUS" {
			t.Fatalf("/voicecutout mutated a relayed frame: %q", payload)
		}
	}
}

// TestApplyVoiceFramePunishmentsStutter confirms /voicestutter never drops a
// frame, only ever emits the current or immediately-previous frame, and does
// substitute stale frames over a run.
func TestApplyVoiceFramePunishmentsStutter(t *testing.T) {
	resetVoiceStutterState()
	t.Cleanup(resetVoiceStutterState)

	const uid = 777
	c := voicePunishedClient(uid, PunishmentVoiceStutter, 0)

	frames := make([]string, 200)
	for i := range frames {
		frames[i] = fmt.Sprintf("frame-%d", i)
	}

	substitutions := 0
	for i, f := range frames {
		payload, relay := applyVoiceFramePunishments(c, uid, f)
		if !relay {
			t.Fatalf("/voicestutter dropped frame %d — it must never drop", i)
		}
		switch {
		case payload == f:
			// current frame relayed as-is
		case i > 0 && payload == frames[i-1]:
			substitutions++
		default:
			t.Fatalf("/voicestutter frame %d emitted %q — not current or previous", i, payload)
		}
	}
	if substitutions < 3 {
		t.Errorf("/voicestutter substituted only %d/200 frames — expected a glitchy run", substitutions)
	}
}

// TestApplyVoiceFramePunishmentsExpired confirms an expired voice punishment
// stops affecting frames even though its row is still on the client.
func TestApplyVoiceFramePunishmentsExpired(t *testing.T) {
	c := voicePunishedClient(1, PunishmentVoiceMute, 1*time.Millisecond)
	time.Sleep(15 * time.Millisecond)
	payload, relay := applyVoiceFramePunishments(c, 1, "OPUS")
	if !relay || payload != "OPUS" {
		t.Errorf("expired /voicemute still suppressing frames: got (%q, %v)", payload, relay)
	}
}

// TestClearVoiceStutterState confirms held frames are dropped on UID release.
func TestClearVoiceStutterState(t *testing.T) {
	resetVoiceStutterState()
	t.Cleanup(resetVoiceStutterState)

	setStutterFrame(42, "held")
	if _, ok := getStutterFrame(42); !ok {
		t.Fatal("setStutterFrame did not store the frame")
	}
	clearVoiceStutterState(42)
	if _, ok := getStutterFrame(42); ok {
		t.Error("clearVoiceStutterState did not drop the held frame")
	}
}

// TestPktVSFrameVoiceMutePunishmentSuppressesRelay is the end-to-end check:
// a /voicemute'd speaker's frames never reach peers, and lifting the
// punishment restores the relay.
func TestPktVSFrameVoiceMutePunishmentSuppressesRelay(t *testing.T) {
	origConfig := config
	origClients := clients
	t.Cleanup(func() {
		config = origConfig
		clients = origClients
		resetVoiceRooms()
		resetVoiceModState()
		resetVoiceStutterState()
	})
	resetVoiceRooms()
	resetVoiceModState()
	resetVoiceStutterState()

	config = voiceTestConfig(true, 6)
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{}, 10, 10, area.EviAny)
	alice, _ := newVoiceClient(t, 1, a)
	bob, bobConn := newVoiceClient(t, 2, a)
	for _, c := range []*Client{alice, bob} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	pktVSJoin(alice, &packet.Packet{Header: "VS_JOIN"})
	pktVSJoin(bob, &packet.Packet{Header: "VS_JOIN"})

	// Baseline: an unpunished frame reaches bob.
	bobConn.buf.Reset()
	pktVSFrame(alice, &packet.Packet{Header: "VS_FRAME", Body: []string{"CLEAN"}})
	if !strings.Contains(bobConn.String(), "VS_AUDIO#1#CLEAN#%") {
		t.Fatalf("baseline frame did not reach bob, got: %q", bobConn.String())
	}

	// /voicemute alice — every frame should now be dropped.
	alice.AddPunishmentBy(PunishmentVoiceMute, 0, "test", IssuerMod)
	bobConn.buf.Reset()
	for i := 0; i < 20; i++ {
		pktVSFrame(alice, &packet.Packet{Header: "VS_FRAME", Body: []string{"MUTED"}})
	}
	if strings.Contains(bobConn.String(), "VS_AUDIO") {
		t.Errorf("/voicemute did not suppress relay, bob got: %q", bobConn.String())
	}

	// Lifting the punishment restores the relay.
	alice.RemovePunishment(PunishmentVoiceMute)
	bobConn.buf.Reset()
	pktVSFrame(alice, &packet.Packet{Header: "VS_FRAME", Body: []string{"AGAIN"}})
	if !strings.Contains(bobConn.String(), "VS_AUDIO#1#AGAIN#%") {
		t.Errorf("relay not restored after the punishment was lifted, bob got: %q", bobConn.String())
	}
}
