/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the wave-2 punishments. */

package athena

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// TestWave2PunishmentTypeRoundTrip checks that every wave-2 punishment name
// parses to its enum and stringifies back, and that no two share an enum.
func TestWave2PunishmentTypeRoundTrip(t *testing.T) {
	cases := map[string]PunishmentType{
		"zalgo":        PunishmentZalgo,
		"leetspeak":    PunishmentLeetspeak,
		"smallcaps":    PunishmentSmallcaps,
		"piglatin":     PunishmentPiglatin,
		"vaporwave":    PunishmentVaporwave,
		"lisp":         PunishmentLisp,
		"spoonerism":   PunishmentSpoonerism,
		"keysmash":     PunishmentKeysmash,
		"weeb":         PunishmentWeeb,
		"politician":   PunishmentPolitician,
		"clickbait":    PunishmentClickbait,
		"markov":       PunishmentMarkov,
		"alliteration": PunishmentAlliteration,
		"cipher":       PunishmentCipher,
		"teleport":     PunishmentTeleport,
		"shakecurse":   PunishmentShakecurse,
		"randomflip":   PunishmentRandomflip,
		"forcecolor":   PunishmentForceColor,
		"nopreanim":    PunishmentNoPreanim,
		"forcepreanim": PunishmentForcePreanim,
		"lifo":         PunishmentLifo,
		"contagious":   PunishmentContagious,
		"minefield":    PunishmentMinefield,
		"stealthmute":  PunishmentStealthMute,
	}
	seen := map[PunishmentType]string{}
	for name, want := range cases {
		if got := parsePunishmentType(name); got != want {
			t.Errorf("parsePunishmentType(%q) = %v, want %v", name, got, want)
		}
		if got := want.String(); got != name {
			t.Errorf("(%v).String() = %q, want %q", want, got, name)
		}
		if want == PunishmentNone {
			t.Errorf("%q maps to PunishmentNone", name)
		}
		if prev, dup := seen[want]; dup {
			t.Errorf("%q and %q share enum value %v", name, prev, want)
		}
		seen[want] = name
	}
}

// TestWave2TransformsProduceOutput sanity-checks every pure text transform:
// non-empty output, within the IC budget, and valid UTF-8.
func TestWave2TransformsProduceOutput(t *testing.T) {
	input := "Hello there partner, this is a perfectly normal test sentence about justice."
	transforms := map[string]func(string) string{
		"zalgo":        applyZalgo,
		"leetspeak":    applyLeetspeak,
		"smallcaps":    applySmallcaps,
		"piglatin":     applyPiglatin,
		"vaporwave":    applyVaporwave,
		"lisp":         applyLisp,
		"spoonerism":   applySpoonerism,
		"keysmash":     applyKeysmash,
		"weeb":         applyWeeb,
		"politician":   applyPolitician,
		"clickbait":    applyClickbait,
		"alliteration": applyAlliteration,
		"cipher":       applyCipher,
	}
	for name, fn := range transforms {
		for i := 0; i < 50; i++ { // several rolls: many transforms are randomized
			out := fn(input)
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s produced empty output", name)
			}
			if len(out) > icBudget() {
				t.Fatalf("%s produced %d bytes, exceeding the IC budget %d", name, len(out), icBudget())
			}
			if !utf8.ValidString(out) {
				t.Fatalf("%s produced invalid UTF-8: %q", name, out)
			}
		}
	}
}

// TestWave2TransformsViaDispatcher exercises the ApplyPunishmentToText switch
// so a missing case can't silently no-op a wave-2 type.
func TestWave2TransformsViaDispatcher(t *testing.T) {
	input := "The quick brown fox jumps over the lazy dog."
	for _, pType := range []PunishmentType{
		PunishmentZalgo, PunishmentLeetspeak, PunishmentSmallcaps,
		PunishmentPiglatin, PunishmentVaporwave, PunishmentLisp,
		PunishmentSpoonerism, PunishmentKeysmash, PunishmentWeeb,
		PunishmentPolitician, PunishmentClickbait, PunishmentAlliteration,
		PunishmentCipher,
	} {
		changed := false
		for i := 0; i < 30; i++ {
			if ApplyPunishmentToText(input, pType) != input {
				changed = true
				break
			}
		}
		if !changed {
			t.Errorf("ApplyPunishmentToText(%v) never altered the input — missing dispatch case?", pType)
		}
	}
}

func TestPigLatinClassics(t *testing.T) {
	cases := map[string]string{
		"hello": "ellohay",
		"Hello": "Ellohay",
		"apple": "appleyay",
		"three": "eethray",
	}
	for in, want := range cases {
		if got := pigLatinCore(in); got != want {
			t.Errorf("pigLatinCore(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSmallcapsMapsLetters(t *testing.T) {
	if got := applySmallcaps("ABC def"); got != "ᴀʙᴄ ᴅᴇꜰ" {
		t.Errorf("applySmallcaps = %q, want %q", got, "ᴀʙᴄ ᴅᴇꜰ")
	}
}

func TestRot13RoundTrip(t *testing.T) {
	in := "Objection! The Witness Is LYING."
	if got := rot13(rot13(in)); got != in {
		t.Errorf("rot13 round trip = %q, want %q", got, in)
	}
}

func TestLispKeepsSh(t *testing.T) {
	got := applyLisp("she sells seashells")
	if !strings.HasPrefix(got, "she ") {
		t.Errorf("applyLisp should keep 'sh' intact, got %q", got)
	}
	if !strings.Contains(got, "th") {
		t.Errorf("applyLisp should convert bare 's', got %q", got)
	}
}

// TestCipherEscalation verifies the state-aware cipher walks through all
// three layers in order and then re-arms.
func TestCipherEscalation(t *testing.T) {
	state := &PunishmentState{}
	first := applyCipherWithState("attorney online", state)
	second := applyCipherWithState("attorney online", state)
	third := applyCipherWithState("attorney online", state)
	fourth := applyCipherWithState("attorney online", state)

	if !strings.Contains(first, "ROT13") {
		t.Errorf("message 1 should be ROT13, got %q", first)
	}
	if !strings.Contains(second, "BINARY") {
		t.Errorf("message 2 should be BINARY, got %q", second)
	}
	if !strings.Contains(third, "BASE64") {
		t.Errorf("message 3 should be BASE64, got %q", third)
	}
	if !strings.Contains(third, "decryption attempt") {
		t.Errorf("cycle end should report a failed decryption, got %q", third)
	}
	if !strings.Contains(fourth, "ROT13") {
		t.Errorf("message 4 should re-arm at ROT13, got %q", fourth)
	}
	if !strings.Contains(first, rot13("attorney online")) {
		t.Errorf("ROT13 layer should contain the rotated text, got %q", first)
	}
}

// TestWeebCorpusSize pins the documented "200-300+" romaji corpus claim.
func TestWeebCorpusSize(t *testing.T) {
	if n := weebRomajiEntryCount(); n < 250 {
		t.Errorf("weeb romaji corpus shrank to %d entries; want >= 250", n)
	}
}

func TestFitICBudgetTruncatesOnRuneBoundary(t *testing.T) {
	long := strings.Repeat("ｗ", 500) // 3-byte runes
	out := fitICBudget(long)
	if len(out) > icBudget() {
		t.Errorf("fitICBudget left %d bytes, budget is %d", len(out), icBudget())
	}
	if !utf8.ValidString(out) {
		t.Errorf("fitICBudget split a rune: %q", out[len(out)-4:])
	}
}

// newTestArea builds a minimal area for transform tests.
func newTestArea() *area.Area {
	return area.NewArea(area.AreaData{Name: "Testing Grounds"}, 5, 10, area.EviAny)
}

// TestMarkovUsesCorpusAndFallsBack covers both markov paths: with area
// history the output draws on corpus words; without history it still mangles
// the message via the timewarp fallback.
func TestMarkovUsesCorpusAndFallsBack(t *testing.T) {
	a := newTestArea()

	// Fallback path: no history recorded yet.
	out := applyMarkov("one two three four five six", a)
	if strings.TrimSpace(out) == "" {
		t.Fatal("markov fallback produced empty output")
	}

	// Corpus path.
	corpus := []string{
		"the defendant was clearly guilty of the crime",
		"the witness saw the defendant at the scene",
		"justice demands the truth from every witness",
		"the court will now hear the testimony",
	}
	for _, m := range corpus {
		a.RecordICMessage("ipid-test", m)
	}
	corpusWords := map[string]bool{}
	for _, m := range corpus {
		for _, w := range strings.Fields(m) {
			corpusWords[w] = true
		}
	}
	out = applyMarkov("anything at all really", a)
	if strings.TrimSpace(out) == "" {
		t.Fatal("markov produced empty output with corpus")
	}
	for _, w := range strings.Fields(out) {
		if !corpusWords[w] {
			t.Fatalf("markov emitted %q which is not in the area corpus (output %q)", w, out)
		}
	}
}

// TestRecentICMessagesWindow checks the new area accessor returns recorded
// messages and respects the cap.
func TestRecentICMessagesWindow(t *testing.T) {
	a := newTestArea()
	for i := 0; i < 10; i++ {
		a.RecordICMessage("ipid-a", "message from a")
		a.RecordICMessage("ipid-b", "message from b")
	}
	got := a.RecentICMessages(5)
	if len(got) != 5 {
		t.Errorf("RecentICMessages(5) returned %d messages", len(got))
	}
	if got = a.RecentICMessages(0); got != nil {
		t.Errorf("RecentICMessages(0) should return nil, got %v", got)
	}
}

// TestApplyProtocolPunishments verifies each protocol punishment writes only
// validator-legal values into the MS packet.
func TestApplyProtocolPunishments(t *testing.T) {
	mk := func(pType PunishmentType, data string) []PunishmentState {
		return []PunishmentState{{punishmentType: pType, customData: data}}
	}

	ms := &packet.MSPacket{Screenshake: "0"}
	applyProtocolPunishments(ms, mk(PunishmentShakecurse, ""))
	if ms.Screenshake != "1" {
		t.Errorf("shakecurse: Screenshake = %q, want \"1\"", ms.Screenshake)
	}

	for i := 0; i < 30; i++ {
		ms = &packet.MSPacket{Flip: "0"}
		applyProtocolPunishments(ms, mk(PunishmentRandomflip, ""))
		if ms.Flip != "0" && ms.Flip != "1" {
			t.Fatalf("randomflip wrote illegal Flip %q", ms.Flip)
		}
	}

	ms = &packet.MSPacket{TextColor: "0"}
	applyProtocolPunishments(ms, mk(PunishmentForceColor, "9"))
	if ms.TextColor != "9" {
		t.Errorf("forcecolor: TextColor = %q, want \"9\"", ms.TextColor)
	}
	ms = &packet.MSPacket{TextColor: "0"}
	applyProtocolPunishments(ms, mk(PunishmentForceColor, "57")) // out of range: ignored
	if ms.TextColor != "0" {
		t.Errorf("forcecolor out-of-range: TextColor = %q, want \"0\"", ms.TextColor)
	}

	ms = &packet.MSPacket{EmoteModifier: "1", PreAnim: "slam"}
	applyProtocolPunishments(ms, mk(PunishmentNoPreanim, ""))
	if ms.EmoteModifier != "0" || ms.PreAnim != "-" {
		t.Errorf("nopreanim: EmoteModifier=%q PreAnim=%q", ms.EmoteModifier, ms.PreAnim)
	}

	ms = &packet.MSPacket{EmoteModifier: "0", PreAnim: "slam"}
	applyProtocolPunishments(ms, mk(PunishmentForcePreanim, ""))
	if ms.EmoteModifier != "1" {
		t.Errorf("forcepreanim: EmoteModifier = %q, want \"1\"", ms.EmoteModifier)
	}
	ms = &packet.MSPacket{EmoteModifier: "0", PreAnim: "-"}
	applyProtocolPunishments(ms, mk(PunishmentForcePreanim, ""))
	if ms.EmoteModifier != "0" {
		t.Errorf("forcepreanim without a named preanim should not promote, got %q", ms.EmoteModifier)
	}

	for i := 0; i < 30; i++ {
		ms = &packet.MSPacket{}
		applyProtocolPunishments(ms, mk(PunishmentTeleport, ""))
		offsets := strings.Split(decode(ms.SelfOffset), "&")
		if len(offsets) != 2 {
			t.Fatalf("teleport wrote malformed SelfOffset %q", ms.SelfOffset)
		}
		for _, o := range offsets {
			v, err := strconv.Atoi(o)
			if err != nil || v < -100 || v > 100 {
				t.Fatalf("teleport wrote out-of-range offset %q", ms.SelfOffset)
			}
		}
	}
}

// TestLifoReleasesInReverseOrder swaps the broadcast hook and checks queue
// order, flush-on-count, and the timer flush path.
func TestLifoReleasesInReverseOrder(t *testing.T) {
	var released []string
	orig := lifoBroadcastFn
	lifoBroadcastFn = func(e lifoPending) { released = append(released, e.ms.Message) }
	defer func() { lifoBroadcastFn = orig }()

	a := newTestArea()
	client := &Client{uid: 42, area: a}
	client.AddPunishment(PunishmentLifo, time.Minute, "test")

	for _, m := range []string{"first", "second", "third"} {
		lifoEnqueueIC(client, &packet.MSPacket{Message: m})
	}
	if len(released) != 3 {
		t.Fatalf("expected flush at %d messages, got %d released", lifoFlushCount, len(released))
	}
	if released[0] != "third" || released[1] != "second" || released[2] != "first" {
		t.Errorf("lifo release order = %v, want [third second first]", released)
	}

	// Timer path: a lone message flushes via lifoFlushClient.
	released = nil
	lifoEnqueueIC(client, &packet.MSPacket{Message: "lonely"})
	lifoFlushClient(client)
	if len(released) != 1 || released[0] != "lonely" {
		t.Errorf("timer flush released %v, want [lonely]", released)
	}
	// Queue must be empty afterwards (no double release).
	lifoFlushClient(client)
	if len(released) != 1 {
		t.Errorf("second flush re-released messages: %v", released)
	}
}

// TestHasPunishmentTypeSnapshot covers the lock-free snapshot helper used at
// the pktIC broadcast gate.
func TestHasPunishmentTypeSnapshot(t *testing.T) {
	snapshot := []PunishmentState{
		{punishmentType: PunishmentUwu},
		{punishmentType: PunishmentStealthMute},
	}
	if !hasPunishmentType(snapshot, PunishmentStealthMute) {
		t.Error("hasPunishmentType missed stealthmute in snapshot")
	}
	if hasPunishmentType(snapshot, PunishmentLifo) {
		t.Error("hasPunishmentType found lifo that isn't there")
	}
	if findPunishmentState(snapshot, PunishmentUwu) == nil {
		t.Error("findPunishmentState missed uwu")
	}
}

// TestLovePotionArmDisarm checks the arm/disarm bookkeeping that gates the
// per-message scan.
func TestLovePotionArmDisarm(t *testing.T) {
	c := &Client{}
	if !c.armLovePotion(time.Minute) {
		t.Error("first arm should report newly-armed")
	}
	if c.armLovePotion(time.Minute) {
		t.Error("re-arm while active should not report newly-armed")
	}
	if !c.LovePotionActive() {
		t.Error("potion should be active after arming")
	}
	if !c.disarmLovePotion() {
		t.Error("disarm should report it was armed")
	}
	if c.disarmLovePotion() {
		t.Error("double disarm should report not-armed")
	}
	if c.LovePotionActive() {
		t.Error("potion should be inactive after disarm")
	}
}

// TestMegamasoPoolHasNoPlaceholders guards the pool extension: every entry
// must stringify to a real name and parse back (so /unpunish -t works on
// every mine /minefield can drop).
func TestMegamasoPoolHasNoPlaceholders(t *testing.T) {
	for _, p := range megamasoStackPool {
		name := p.String()
		if name == "none" {
			t.Errorf("megamasoStackPool contains unmapped type %d", int(p))
		}
		if parsePunishmentType(name) != p {
			t.Errorf("pool entry %q does not round-trip", name)
		}
	}
}
