/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /antipunishshield. */

package athena

import (
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

type shieldTestWorld struct {
	mod, target, other             *Client
	modConn, targetConn, otherConn *captureConn
	area                           *area.Area
}

// resetAntiPunishShieldStore clears the process-wide restart store so one test
// cannot leak its HDID into the next.
func resetAntiPunishShieldStore(t *testing.T) {
	t.Helper()
	clear := func() {
		antiPunishShieldStore.Lock()
		antiPunishShieldStore.byHDID = make(map[string]int32)
		antiPunishShieldStore.byIPID = make(map[string]int32)
		antiPunishShieldStore.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

func newShieldTestWorld(t *testing.T) *shieldTestWorld {
	t.Helper()
	resetAntiPunishShieldStore(t)
	t.Cleanup(setupAreaMuteTestDB(t))
	newTestClients(t)

	origDisabled := punishmentsGloballyDisabled.Load()
	SetPunishmentsGloballyDisabled(false)
	t.Cleanup(func() { SetPunishmentsGloballyDisabled(origDisabled) })

	a := makeLeashTestArea("Courtroom")
	t.Cleanup(setupTestAreas([]*area.Area{a}))

	modConn := &captureConn{}
	targetConn := &captureConn{}
	otherConn := &captureConn{}
	mod := &Client{
		conn: modConn, uid: 1, ipid: "ip-shield-mod", hdid: "hdid-shield-mod",
		char: -1, area: a, perms: permissions.PermissionField["MUTE"], mod_name: "Mod",
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	target := &Client{
		conn: targetConn, uid: 2, ipid: "ip-shield-target", hdid: "hdid-shield-target",
		char: 0, area: a,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	other := &Client{
		conn: otherConn, uid: 3, ipid: "ip-shield-other", hdid: "hdid-shield-other",
		char: 1, area: a,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	for _, c := range []*Client{mod, target, other} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	return &shieldTestWorld{
		mod: mod, target: target, other: other,
		modConn: modConn, targetConn: targetConn, otherConn: otherConn, area: a,
	}
}

func shieldContainsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestAntiPunishShieldRegistered(t *testing.T) {
	initCommands()
	cmd, ok := Commands["antipunishshield"]
	if !ok {
		t.Fatal("antipunishshield command is not registered")
	}
	if cmd.minArgs != 1 {
		t.Errorf("minArgs = %d, want 1", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["ADMIN"] {
		t.Errorf("reqPerms = %d, want ADMIN", cmd.reqPerms)
	}
}

func TestAntiPunishShieldCommandAndPersistence(t *testing.T) {
	w := newShieldTestWorld(t)

	cmdAntiPunishShield(w.mod, []string{"2"}, "usage")
	if got := w.target.AntiPunishShieldMode(); got != antiPunishShieldSpoof {
		t.Fatalf("mode = %d, want spoof (%d)", got, antiPunishShieldSpoof)
	}
	if got := w.modConn.String(); !strings.Contains(got, "spoof mode") {
		t.Errorf("issuer was not told the mode; got %q", got)
	}

	cmdAntiPunishShield(w.mod, []string{"2", "backfire"}, "usage")
	if got := w.target.AntiPunishShieldMode(); got != antiPunishShieldBackfire {
		t.Fatalf("mode = %d, want backfire (%d)", got, antiPunishShieldBackfire)
	}

	// A reconnect with the same HDID must get the shield back from the
	// process-wide store.
	clone := &Client{conn: &captureConn{}, hdid: w.target.Hdid(), ipid: "ip-shield-clone"}
	applyStoredAntiPunishShield(clone)
	if got := clone.AntiPunishShieldMode(); got != antiPunishShieldBackfire {
		t.Errorf("reconnected clone mode = %d, want backfire", got)
	}
}

func TestAntiPunishShieldSpoofsDog(t *testing.T) {
	w := newShieldTestWorld(t)

	cmdAntiPunishShield(w.mod, []string{"2"}, "usage")
	cmdDog(w.mod, []string{"2"}, "usage")

	if w.target.HasActivePunishment(PunishmentDog) {
		t.Fatal("shielded target actually received the dog punishment")
	}
	if got := w.modConn.String(); !strings.Contains(got, "Applied 'dog'") {
		t.Errorf("issuer did not get the normal success summary; got %q", got)
	}
	if got := w.targetConn.String(); strings.Contains(got, "punished with") {
		t.Errorf("shielded target was told it was punished; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "BB#") ||
		!strings.Contains(got, "tried to punish you") ||
		!strings.Contains(got, "punishment protected") {
		t.Errorf("shielded target did not get the sneaky shield popup; got %q", got)
	}

	spoofs := w.target.AntiPunishSpoofs()
	if len(spoofs) != 1 {
		t.Fatalf("spoof entries = %d, want 1", len(spoofs))
	}
	if _, ok := spoofs[w.mod]; !ok {
		t.Fatal("issuer is missing from the spoof map")
	}

	if !broadcastSpoofedIC(w.target, &MSToClient{Message: encode("hello")}) {
		t.Fatal("broadcastSpoofedIC reported no spoofs")
	}
	if got := w.targetConn.String(); !strings.Contains(got, "hello") {
		t.Errorf("target did not see the normal message; got %q", got)
	}
	if got := w.otherConn.String(); !strings.Contains(got, "hello") {
		t.Errorf("bystander did not see the normal message; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, "hello") {
		t.Errorf("issuer saw the normal message instead of the spoof; got %q", got)
	}
	if got := w.modConn.String(); !shieldContainsAny(got, dogSounds) {
		t.Errorf("issuer did not see a dog-spoofed message; got %q", got)
	}

	// Full removal clears the fake effects as well as real punishments.
	cmdUnpunish(w.mod, []string{"2"}, "usage")
	if spoofs := w.target.AntiPunishSpoofs(); len(spoofs) != 0 {
		t.Errorf("spoofs survived /unpunish: %v", spoofs)
	}
}

// TestAntiPunishShieldBlocksEveryInsert pins the central guard: even a direct
// AddPunishment* call that bypasses a command handler cannot land on a
// shielded client.
func TestAntiPunishShieldBlocksEveryInsert(t *testing.T) {
	w := newShieldTestWorld(t)
	cmdAntiPunishShield(w.mod, []string{"2"}, "usage")

	w.target.AddPunishmentBy(PunishmentUppercase, time.Minute, "direct", IssuerSystem)
	w.target.AddPunishmentWithData(PunishmentShrink, time.Minute, "direct", "-25")
	w.target.AddLovebombPunishment(-1, time.Minute, "direct")
	w.target.AddICWarpPunishment(w.area, time.Minute, "direct")

	if w.target.HasPunishment(PunishmentUppercase) ||
		w.target.HasPunishment(PunishmentShrink) ||
		w.target.HasPunishment(PunishmentLovebomb) ||
		w.target.HasPunishment(PunishmentICWarp) {
		t.Errorf("a direct punishment insert landed on a shielded client: %v", w.target.GetActivePunishments())
	}
}

func TestAntiPunishShieldBackfire(t *testing.T) {
	w := newShieldTestWorld(t)

	cmdAntiPunishShield(w.mod, []string{"2", "backfire"}, "usage")
	cmdDog(w.mod, []string{"2"}, "usage")

	if w.target.HasActivePunishment(PunishmentDog) {
		t.Fatal("shielded target actually received the dog punishment")
	}
	if !w.mod.HasActivePunishment(PunishmentDog) {
		t.Fatal("backfire did not apply the dog punishment to the issuer")
	}
	if got := w.modConn.String(); !strings.Contains(got, "backfired") {
		t.Errorf("issuer was not told about the backfire; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "backfired on them instead") {
		t.Errorf("shielded target was not told it backfired; got %q", got)
	}
	if spoofs := w.target.AntiPunishSpoofs(); len(spoofs) != 1 {
		t.Errorf("spoof entries = %d, want 1", len(spoofs))
	}
}

func TestAntiPunishShieldOOCSpoof(t *testing.T) {
	w := newShieldTestWorld(t)

	cmdAntiPunishShield(w.mod, []string{"2"}, "usage")
	w.target.RecordAntiPunishSpoof(w.mod, []PunishmentType{PunishmentDog}, time.Minute)

	const ooc = "unique-ooc-spoof-text"
	if !broadcastSpoofedOOC(w.target, "[2] Target", ooc) {
		t.Fatal("broadcastSpoofedOOC reported no spoofs")
	}
	if got := w.targetConn.String(); !strings.Contains(got, ooc) {
		t.Errorf("target did not see the normal OOC line; got %q", got)
	}
	if got := w.otherConn.String(); !strings.Contains(got, ooc) {
		t.Errorf("bystander did not see the normal OOC line; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, ooc) {
		t.Errorf("issuer saw the normal OOC line instead of the spoof; got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "CT#") || !shieldContainsAny(got, dogSounds) {
		t.Errorf("issuer did not see a dog-spoofed OOC line; got %q", got)
	}
}
