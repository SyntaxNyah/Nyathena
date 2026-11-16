/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /antipunishshield. */

package athena

import (
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

type shieldTestWorld struct {
	mod, target, other             *Client
	modConn, targetConn, otherConn *captureConn
	area, otherArea                *area.Area
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

	// /ban reads the server-configured default duration even when the test
	// passes -d, so the fixture needs a non-nil config.
	origConfig := config
	config = &settings.Config{ServerConfig: settings.ServerConfig{BanLen: "3d"}}
	t.Cleanup(func() { config = origConfig })

	a := makeLeashTestArea("Courtroom")
	b := makeLeashTestArea("Lobby")
	t.Cleanup(setupTestAreas([]*area.Area{a, b}))

	modConn := &captureConn{}
	targetConn := &captureConn{}
	otherConn := &captureConn{}
	mod := &Client{
		conn: modConn, uid: 1, ipid: "ip-shield-mod", hdid: "hdid-shield-mod",
		char: -1, area: a, perms: permissions.PermissionField["MUTE"] | permissions.PermissionField["MOVE_USERS"], mod_name: "Mod",
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
		modConn: modConn, targetConn: targetConn, otherConn: otherConn, area: a, otherArea: b,
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

// isClosed reports whether the server has closed this test connection.
func (c *captureConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
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
	if got := w.modConn.String(); strings.Contains(got, "backfired") {
		t.Errorf("moderator was told about the backfire; it must land silently, got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "Applied 'dog'") {
		t.Errorf("moderator did not get the normal success summary; got %q", got)
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

// TestAntiPunishShieldBackfireMove pins the /move backfire: a moderator who
// tries to drag a shielded player into another area makes the trip themselves,
// while the shielded player stays put.
func TestAntiPunishShieldBackfireMove(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldBackfire)

	cmdMove(w.mod, []string{"-u", "2", "1"}, "usage")

	if w.target.Area() != w.area {
		t.Fatalf("shielded target should not have moved; got %v", w.target.Area().Name())
	}
	if w.mod.Area() != w.otherArea {
		t.Fatalf("issuer should have been moved instead; got %v", w.mod.Area().Name())
	}
	if got := w.modConn.String(); strings.Contains(got, "bounced") {
		t.Errorf("issuer was told the move bounced; got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "Moved 1 users.") {
		t.Errorf("issuer did not get the normal success summary; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "backfired on them instead") {
		t.Errorf("shielded target was not told it backfired; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "move you") {
		t.Errorf("shielded target popup did not name the action; got %q", got)
	}
}

// TestAntiPunishShieldSpoofMove pins spoof-mode /move: the target is protected,
// the issuer is not redirected, and the summary still counts the fake success.
func TestAntiPunishShieldSpoofMove(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldSpoof)

	cmdMove(w.mod, []string{"-u", "2", "1"}, "usage")

	if w.target.Area() != w.area {
		t.Fatalf("shielded target should not have moved; got %v", w.target.Area().Name())
	}
	if w.mod.Area() != w.area {
		t.Fatalf("spoof mode should not move the issuer; got %v", w.mod.Area().Name())
	}
	if got := w.modConn.String(); !strings.Contains(got, "Moved 1 users.") {
		t.Errorf("issuer did not get the fake success summary; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, "bounced") {
		t.Errorf("issuer was told about a spoof-mode bounce; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "It didn't stick") {
		t.Errorf("shielded target was not told the move was blocked; got %q", got)
	}
}

// TestAntiPunishShieldMoveSelfAllowed verifies the shield never traps its owner:
// a shielded player moving themselves is a voluntary action, not a punishment.
func TestAntiPunishShieldMoveSelfAllowed(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldBackfire)

	cmdMove(w.target, []string{"1"}, "usage")

	if w.target.Area() != w.otherArea {
		t.Fatalf("shielded player should be able to move themselves; got %v", w.target.Area().Name())
	}
}

// TestAntiPunishShieldBackfireMoveRespectsLeash verifies the bounce cannot be
// used as a leash escape hatch: a dog moderator stays with its master.
func TestAntiPunishShieldBackfireMoveRespectsLeash(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldBackfire)

	// Make the moderator a dog of "other" (both need a selected character).
	w.mod.SetCharID(2)
	cmdLeash(w.other, []string{"3", "1"}, "usage")
	if w.mod.Leash() == nil || !w.mod.Leash().IsDog(w.mod) {
		t.Fatal("setup: moderator is not leashed")
	}

	cmdMove(w.mod, []string{"-u", "2", "1"}, "usage")

	if w.target.Area() != w.area {
		t.Fatalf("shielded target should not have moved; got %v", w.target.Area().Name())
	}
	if w.mod.Area() != w.area {
		t.Fatalf("leashed moderator should not be bounced out of its master's area; got %v", w.mod.Area().Name())
	}
	if got := w.modConn.String(); strings.Contains(got, "anti-punishment shield") {
		t.Errorf("issuer was told about the shield; got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "Moved 1 users.") {
		t.Errorf("issuer did not get the normal success summary; got %q", got)
	}
}

// TestAntiPunishShieldBackfireForceName pins the /forcename backfire: the
// shielded player keeps its name and the moderator is renamed instead.
func TestAntiPunishShieldBackfireForceName(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldBackfire)
	const name = "Bounced Name"

	cmdForceName(w.mod, []string{"2", name}, "usage")

	if got := w.target.ForcedShowname(); got != "" {
		t.Errorf("shielded target was renamed: %q", got)
	}
	if got := w.mod.ForcedShowname(); got != encode(name) {
		t.Errorf("moderator forced showname = %q, want %q", got, encode(name))
	}
	if got := w.modConn.String(); strings.Contains(got, "bounced") {
		t.Errorf("issuer was told the forcename bounced; got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "Forced UID 2's showname") {
		t.Errorf("issuer did not get the normal success reply; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "backfired on them instead") {
		t.Errorf("shielded target was not told it backfired; got %q", got)
	}
}

// TestAntiPunishShieldSpoofForceName pins spoof-mode /forcename: neither player
// is renamed, the moderator sees the usual success reply, and the target is
// quietly warned.
func TestAntiPunishShieldSpoofForceName(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldSpoof)
	const name = "Spoofed Name"

	cmdForceName(w.mod, []string{"2", name}, "usage")

	if got := w.target.ForcedShowname(); got != "" {
		t.Errorf("shielded target was renamed: %q", got)
	}
	if got := w.mod.ForcedShowname(); got != "" {
		t.Errorf("spoof mode renamed the moderator: %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "Forced UID 2's showname") {
		t.Errorf("issuer did not get the fake success reply; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "It didn't stick") {
		t.Errorf("shielded target was not told the rename was blocked; got %q", got)
	}
}

// TestAntiPunishShieldBackfireKick pins the /kick backfire: the shielded
// player stays connected and the issuing moderator is disconnected instead.
func TestAntiPunishShieldBackfireKick(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldBackfire)

	cmdKick(w.mod, []string{"-u", "2", "bounced"}, "usage")

	if w.targetConn.isClosed() {
		t.Fatal("shielded target was disconnected")
	}
	if !w.modConn.isClosed() {
		t.Fatal("issuer was not disconnected by the backfire")
	}
	if got := w.targetConn.String(); !strings.Contains(got, "backfired on them instead") {
		t.Errorf("shielded target was not told it backfired; got %q", got)
	}
	if got := w.targetConn.String(); strings.Contains(got, "bounced") {
		t.Errorf("shielded target received the kick reason; got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "bounced") {
		t.Errorf("issuer did not receive the kick packet; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, "anti-punishment shield") {
		t.Errorf("issuer was told about the shield; got %q", got)
	}
}

// TestAntiPunishShieldSpoofKick pins spoof-mode /kick: the target stays
// connected, the issuer is not kicked, and the summary still says success.
func TestAntiPunishShieldSpoofKick(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldSpoof)

	cmdKick(w.mod, []string{"-u", "2", "bounced"}, "usage")

	if w.targetConn.isClosed() {
		t.Fatal("shielded target was disconnected")
	}
	if w.modConn.isClosed() {
		t.Fatal("spoof mode disconnected the issuer")
	}
	if got := w.modConn.String(); !strings.Contains(got, "Kicked 1 clients.") {
		t.Errorf("issuer did not get the fake success summary; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, "anti-punishment shield") {
		t.Errorf("issuer was told about the shield; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "It didn't stick") {
		t.Errorf("shielded target was not told the kick was blocked; got %q", got)
	}
}

// TestAntiPunishShieldBackfireBan pins the /ban backfire: the shielded player's
// IPID stays unbanned and the issuing moderator's IPID is banned instead.
func TestAntiPunishShieldBackfireBan(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldBackfire)

	cmdBan(w.mod, []string{"-u", "2", "-d", "1h", "bounced"}, "usage")

	targetBanned, _, err := db.IsBanned(db.IPID, w.target.Ipid())
	if err != nil {
		t.Fatalf("IsBanned(target): %v", err)
	}
	if targetBanned {
		t.Fatal("shielded target was banned")
	}
	issuerBanned, _, err := db.IsBanned(db.IPID, w.mod.Ipid())
	if err != nil {
		t.Fatalf("IsBanned(issuer): %v", err)
	}
	if !issuerBanned {
		t.Fatal("issuer was not banned by the backfire")
	}
	if !w.modConn.isClosed() {
		t.Fatal("issuer was not disconnected by the ban backfire")
	}
	if w.targetConn.isClosed() {
		t.Fatal("shielded target was disconnected")
	}
	if got := w.targetConn.String(); !strings.Contains(got, "backfired on them instead") {
		t.Errorf("shielded target was not told it backfired; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, "anti-punishment shield") {
		t.Errorf("issuer was told about the shield; got %q", got)
	}
}

// TestAntiPunishShieldSpoofBan pins spoof-mode /ban: neither IPID is banned
// and the issuer gets the normal success summary.
func TestAntiPunishShieldSpoofBan(t *testing.T) {
	w := newShieldTestWorld(t)
	w.target.SetAntiPunishShield(antiPunishShieldSpoof)

	cmdBan(w.mod, []string{"-u", "2", "-d", "1h", "bounced"}, "usage")

	targetBanned, _, err := db.IsBanned(db.IPID, w.target.Ipid())
	if err != nil {
		t.Fatalf("IsBanned(target): %v", err)
	}
	if targetBanned {
		t.Fatal("shielded target was banned")
	}
	issuerBanned, _, err := db.IsBanned(db.IPID, w.mod.Ipid())
	if err != nil {
		t.Fatalf("IsBanned(issuer): %v", err)
	}
	if issuerBanned {
		t.Fatal("spoof mode banned the issuer")
	}
	if got := w.modConn.String(); !strings.Contains(got, "Banned 1 IPID(s).") {
		t.Errorf("issuer did not get the fake success summary; got %q", got)
	}
	if got := w.modConn.String(); strings.Contains(got, "anti-punishment shield") {
		t.Errorf("issuer was told about the shield; got %q", got)
	}
	if got := w.targetConn.String(); !strings.Contains(got, "It didn't stick") {
		t.Errorf("shielded target was not told the ban was blocked; got %q", got)
	}
}

// TestDiscordKickRespectsAntiPunishShield pins the Discord bridge: a shielded
// player is not disconnected, the bot still gets a success (nil error), and the
// target gets the sneaky popup; an unshielded player is still kicked.
func TestDiscordKickRespectsAntiPunishShield(t *testing.T) {
	t.Run("shielded", func(t *testing.T) {
		w := newShieldTestWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		if err := (&ServerAdapter{}).KickPlayer(2, "discord kick"); err != nil {
			t.Fatalf("KickPlayer returned an error instead of a fake success: %v", err)
		}
		if w.targetConn.isClosed() {
			t.Fatal("shielded target was kicked over Discord")
		}
		if got := w.targetConn.String(); !strings.Contains(got, "a Discord moderator") {
			t.Errorf("shielded target did not get the Discord shield popup; got %q", got)
		}
	})

	t.Run("unshielded", func(t *testing.T) {
		w := newShieldTestWorld(t)

		if err := (&ServerAdapter{}).KickPlayer(2, "discord kick"); err != nil {
			t.Fatalf("KickPlayer: %v", err)
		}
		if !w.targetConn.isClosed() {
			t.Fatal("unshielded target was not kicked over Discord")
		}
	})
}
