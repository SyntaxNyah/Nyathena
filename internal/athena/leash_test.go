/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /leash (leash.go). */

package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// leashTestWorld is a two-room world used by the leash tests. Teto Cafe can be
// marked punishment-safe to reproduce the "leash through antipunish" case.
type leashTestWorld struct {
	mod, master, dog, other                 *Client
	modConn, masterConn, dogConn, otherConn *captureConn
	cafe, pizza                             *area.Area
}

// makeLeashTestArea needs enough character slots for the master, dog and
// bystander; the shared makeTestArea helper only allocates one.
func makeLeashTestArea(name string) *area.Area {
	return area.NewArea(
		area.AreaData{Name: name, Bg: "default"},
		10, // charlen
		10, // bufsize
		area.EviCMs,
	)
}

func newLeashTestWorld(t *testing.T, safeCafe bool) *leashTestWorld {
	t.Helper()
	t.Cleanup(setupAreaMuteTestDB(t))
	newTestClients(t)

	origDisabled := punishmentsGloballyDisabled.Load()
	SetPunishmentsGloballyDisabled(false)
	t.Cleanup(func() { SetPunishmentsGloballyDisabled(origDisabled) })

	cafe := makeLeashTestArea("Teto Cafe")
	pizza := makeLeashTestArea("Pizza Room")
	if safeCafe {
		cafe.SetPunishmentSafe(true)
	}
	t.Cleanup(setupTestAreas([]*area.Area{cafe, pizza}))

	modConn := &captureConn{}
	masterConn := &captureConn{}
	dogConn := &captureConn{}
	otherConn := &captureConn{}

	mod := &Client{
		conn: modConn, uid: 1, ipid: "ip-leash-mod", char: -1, area: cafe,
		perms: permissions.PermissionField["MUTE"], mod_name: "Mod",
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	master := &Client{
		conn: masterConn, uid: 2, ipid: "ip-leash-master", char: 0, area: cafe,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	dog := &Client{
		conn: dogConn, uid: 3, ipid: "ip-leash-dog", char: 1, area: cafe,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	other := &Client{
		conn: otherConn, uid: 4, ipid: "ip-leash-other", char: 2, area: cafe,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	for _, c := range []*Client{mod, master, dog, other} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	return &leashTestWorld{
		mod: mod, master: master, dog: dog, other: other,
		modConn: modConn, masterConn: masterConn, dogConn: dogConn, otherConn: otherConn,
		cafe: cafe, pizza: pizza,
	}
}

func (w *leashTestWorld) issue(t *testing.T) {
	t.Helper()
	cmdLeash(w.mod, []string{"2", "3"}, "usage")
}

// TestLeashCommandRegistered pins the command's permission and minimum args.
func TestLeashCommandRegistered(t *testing.T) {
	initCommands()
	cmd, ok := Commands["leash"]
	if !ok {
		t.Fatal("leash command is not registered")
	}
	if cmd.minArgs != 2 {
		t.Errorf("leash minArgs = %d, want 2", cmd.minArgs)
	}
	if cmd.reqPerms != permissions.PermissionField["KICK"] {
		t.Errorf("leash reqPerms = %d, want KICK", cmd.reqPerms)
	}
	if cmd.category != "punishment" {
		t.Errorf("leash category = %q, want punishment", cmd.category)
	}
}

// TestLeashClaimsAndAppliesDog verifies the core claim: both clients point at
// the same Leash, the pair is installed both ways, and the /dog effect lands on
// the dog only.
func TestLeashClaimsAndAppliesDog(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}
	if w.dog.Leash() != l {
		t.Fatal("master and dog do not share the same Leash")
	}
	if l.Master() != w.master || l.Dog() != w.dog {
		t.Fatalf("leash ends = (%v, %v), want (master, dog)", l.Master(), l.Dog())
	}
	if w.master.ForcePairUID() != w.dog.Uid() || w.dog.ForcePairUID() != w.master.Uid() {
		t.Fatalf("pair links = (%d, %d), want (%d, %d)",
			w.master.ForcePairUID(), w.dog.ForcePairUID(), w.dog.Uid(), w.master.Uid())
	}
	if !w.dog.HasActivePunishment(PunishmentDog) {
		t.Error("dog did not receive the /dog punishment")
	}
	if w.master.HasActivePunishment(PunishmentDog) {
		t.Error("master should not receive the /dog punishment")
	}
	if got := w.dogConn.String(); !strings.Contains(got, "leashed by") {
		t.Errorf("dog was not told about the leash; got %q", got)
	}
	if got := w.masterConn.String(); !strings.Contains(got, "You have leashed") {
		t.Errorf("master was not told about the leash; got %q", got)
	}
	if got := w.modConn.String(); !strings.Contains(got, "Leashed") {
		t.Errorf("issuer did not get a leash summary; got %q", got)
	}
}

// TestLeashClaimsPreviousPartner verifies that force-leashing over an existing
// pair clears the old partner's state and pops the claim notice at them.
func TestLeashClaimsPreviousPartner(t *testing.T) {
	w := newLeashTestWorld(t, false)

	w.dog.SetPairWantedID(w.other.CharID())
	w.other.SetPairWantedID(w.dog.CharID())
	w.dog.SetForcePairUID(w.other.Uid())
	w.other.SetForcePairUID(w.dog.Uid())

	w.issue(t)

	if w.other.ForcePairUID() != -1 || w.other.PairWantedID() != -1 {
		t.Fatalf("previous partner still holds pair state: ForcePairUID=%d PairWantedID=%d",
			w.other.ForcePairUID(), w.other.PairWantedID())
	}
	if got := w.otherConn.String(); !strings.Contains(got, "BB#Your partner has been leashed#%") {
		t.Errorf("previous partner did not receive the claim popup; got %q", got)
	}
	if got := w.masterConn.String(); strings.Contains(got, "Your partner has been leashed") {
		t.Errorf("master should not receive the claim popup; got %q", got)
	}
	if got := w.dogConn.String(); strings.Contains(got, "Your partner has been leashed") {
		t.Errorf("dog should not receive the claim popup; got %q", got)
	}
}

// TestLeashBypassesPunishmentSafeArea pins the requested exception: /leash
// applies even when the dog is standing in an antipunish room.
func TestLeashBypassesPunishmentSafeArea(t *testing.T) {
	w := newLeashTestWorld(t, true)
	w.issue(t)

	if w.dog.Leash() == nil {
		t.Fatal("leash was not created in a punishment-safe area")
	}
	if !w.dog.HasActivePunishment(PunishmentDog) {
		t.Error("/dog effect was blocked by the punishment-safe area")
	}
	if got := w.modConn.String(); strings.Contains(got, "punishment-safe") {
		t.Errorf("issuer should not be told the dog was safe-area blocked; got %q", got)
	}
}

// TestLeashNeitherEndCanUnpair verifies /unpair is not an escape hatch for
// either end of a leash. Full /unpunish and /forceunpair are the removal paths.
func TestLeashNeitherEndCanUnpair(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	cmdUnpair(w.dog, nil, "")

	if w.dog.Leash() == nil || w.master.Leash() == nil {
		t.Fatal("dog's /unpair dissolved the leash")
	}
	if w.dog.ForcePairUID() != w.master.Uid() {
		t.Fatalf("dog pair was cleared: ForcePairUID=%d", w.dog.ForcePairUID())
	}
	if got := w.dogConn.String(); !strings.Contains(got, "BB#Dogs don't abandon their master#%") {
		t.Errorf("dog did not receive the abandonment popup; got %q", got)
	}

	cmdUnpair(w.master, nil, "")

	if w.dog.Leash() == nil || w.master.Leash() == nil {
		t.Fatal("master's /unpair dissolved the leash")
	}
	if w.master.ForcePairUID() != w.dog.Uid() {
		t.Fatalf("master pair was cleared: ForcePairUID=%d", w.master.ForcePairUID())
	}
	if got := w.masterConn.String(); !strings.Contains(got, "cannot unpair while holding a leash") {
		t.Errorf("master was not refused; got %q", got)
	}
}

// assertLeashBroken checks that both ends lost the claim and their pair state.
func assertLeashBroken(t *testing.T, w *leashTestWorld) {
	t.Helper()
	if w.dog.Leash() != nil || w.master.Leash() != nil {
		t.Fatalf("leash survived: dog=%v master=%v", w.dog.Leash(), w.master.Leash())
	}
	if w.dog.ForcePairUID() != -1 || w.master.ForcePairUID() != -1 {
		t.Errorf("pair state survived: dog=%d master=%d",
			w.dog.ForcePairUID(), w.master.ForcePairUID())
	}
}

// TestUnpunishBreaksLeash verifies the full /unpunish forms release the claim,
// while /unpunish -t dog only removes the barking effect.
func TestUnpunishBreaksLeash(t *testing.T) {
	t.Run("uid full removal", func(t *testing.T) {
		w := newLeashTestWorld(t, false)
		w.issue(t)

		cmdUnpunish(w.mod, []string{"3"}, "usage")

		assertLeashBroken(t, w)
		if got := w.dogConn.String(); !strings.Contains(got, "leash was removed") {
			t.Errorf("dog was not told the leash was removed; got %q", got)
		}
		if got := w.masterConn.String(); !strings.Contains(got, "leash was removed") {
			t.Errorf("master was not told the leash was removed; got %q", got)
		}
	})

	t.Run("unpunish all", func(t *testing.T) {
		w := newLeashTestWorld(t, false)
		w.issue(t)

		cmdUnpunish(w.mod, []string{"all"}, "usage")

		assertLeashBroken(t, w)
	})

	t.Run("-t dog keeps the leash", func(t *testing.T) {
		w := newLeashTestWorld(t, false)
		w.issue(t)

		cmdUnpunish(w.mod, []string{"-t", "dog", "3"}, "usage")

		if w.dog.Leash() == nil || w.master.Leash() == nil {
			t.Fatal("/unpunish -t dog released the leash")
		}
		if w.dog.HasActivePunishment(PunishmentDog) {
			t.Error("/unpunish -t dog did not remove the dog effect")
		}
	})
}

// TestForceUnpairBreaksLeash verifies a moderator can override the claim.
func TestForceUnpairBreaksLeash(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	cmdForceUnpair(w.mod, []string{"3"}, "usage")

	if w.dog.Leash() != nil || w.master.Leash() != nil {
		t.Fatal("/forceunpair did not dissolve both leash pointers")
	}
	if w.master.ForcePairUID() != -1 || w.dog.ForcePairUID() != -1 {
		t.Errorf("pair state survived /forceunpair: master=%d dog=%d",
			w.master.ForcePairUID(), w.dog.ForcePairUID())
	}
	if got := w.masterConn.String(); !strings.Contains(got, "was broken by") {
		t.Errorf("master was not told the leash was broken; got %q", got)
	}
}

// TestLeashDogFollowsMasterThroughSafeAreas is the Teto Cafe -> Pizza Room ->
// Teto Cafe scenario: the dog follows both ways even though Teto Cafe is
// punishment-safe, and the leash/dog effect survive the trip.
func TestLeashDogFollowsMasterThroughSafeAreas(t *testing.T) {
	w := newLeashTestWorld(t, true)
	w.issue(t)

	if !w.master.ChangeArea(w.pizza) {
		t.Fatal("master could not move to Pizza Room")
	}
	if w.dog.Area() != w.pizza {
		t.Fatalf("dog did not follow to Pizza Room; area = %v", w.dog.Area().Name())
	}

	if !w.master.ChangeArea(w.cafe) {
		t.Fatal("master could not move back to Teto Cafe")
	}
	if w.dog.Area() != w.cafe {
		t.Fatalf("dog did not follow back to Teto Cafe; area = %v", w.dog.Area().Name())
	}
	if w.dog.Leash() == nil || w.master.Leash() == nil {
		t.Error("leash was lost during the trip")
	}
	if !w.dog.HasActivePunishment(PunishmentDog) {
		t.Error("dog effect was lost during the trip")
	}
}

// TestLeashedDogCannotWander verifies the player-initiated movement guard: a
// dog may move to its master's area but nowhere else.
func TestLeashedDogCannotWander(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	if !w.dog.leashBlocksAreaChange(w.pizza) {
		t.Fatal("dog was allowed to leave the master's area")
	}
	if w.dog.Area() != w.cafe {
		t.Fatalf("dog moved despite the guard: %v", w.dog.Area().Name())
	}
	if got := w.dogConn.String(); !strings.Contains(got, "BB#Dogs don't abandon their master#%") {
		t.Errorf("dog did not receive the abandonment popup; got %q", got)
	}

	// Moving to the master's own area is following, not abandoning.
	w.master.ChangeArea(w.pizza)
	w.dog.SetArea(w.cafe) // put the dog back behind on purpose
	if w.dog.leashBlocksAreaChange(w.pizza) {
		t.Fatal("dog was blocked from moving to the master's area")
	}
}

// TestLeashDogDisconnectPopsAbandonment verifies the surviving master gets the
// requested popup when the dog leaves the server.
func TestLeashDogDisconnectPopsAbandonment(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	clearPairLinksOnDisconnect(w.dog)

	if w.master.Leash() != nil || w.dog.Leash() != nil {
		t.Fatal("disconnect did not clear the leash pointers")
	}
	if w.master.ForcePairUID() != -1 || w.master.PairWantedID() != -1 {
		t.Errorf("master pair state survived dog disconnect: ForcePairUID=%d PairWantedID=%d",
			w.master.ForcePairUID(), w.master.PairWantedID())
	}
	if got := w.masterConn.String(); !strings.Contains(got, "BB#Dogs don't abandon their master#%") {
		t.Errorf("master did not receive the abandonment popup; got %q", got)
	}
}

// TestLeashMasterDisconnectFreesDog verifies the dog is released when the
// master disconnects.
func TestLeashMasterDisconnectFreesDog(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	clearPairLinksOnDisconnect(w.master)

	if w.dog.Leash() != nil || w.master.Leash() != nil {
		t.Fatal("disconnect did not clear the leash pointers")
	}
	if w.dog.ForcePairUID() != -1 || w.dog.PairWantedID() != -1 {
		t.Errorf("dog pair state survived master disconnect: ForcePairUID=%d PairWantedID=%d",
			w.dog.ForcePairUID(), w.dog.PairWantedID())
	}
	if got := w.dogConn.String(); !strings.Contains(got, "Your master disconnected") {
		t.Errorf("dog was not told the master disconnected; got %q", got)
	}
}

// TestForcePairBreaksLeash verifies a moderator force-pairing over a leash
// clears the claim instead of leaving a stale dog pointer.
func TestForcePairBreaksLeash(t *testing.T) {
	w := newLeashTestWorld(t, false)
	w.issue(t)

	cmdForcePair(w.mod, []string{"2", "4"}, "usage")

	if w.master.Leash() != nil || w.dog.Leash() != nil {
		t.Fatal("/forcepair did not dissolve the leash")
	}
	if w.dog.ForcePairUID() != -1 || w.dog.PairWantedID() != -1 {
		t.Errorf("old dog state survived /forcepair: ForcePairUID=%d PairWantedID=%d",
			w.dog.ForcePairUID(), w.dog.PairWantedID())
	}
	if w.master.ForcePairUID() != w.other.Uid() || w.other.ForcePairUID() != w.master.Uid() {
		t.Errorf("new force-pair was not installed: master=%d other=%d",
			w.master.ForcePairUID(), w.other.ForcePairUID())
	}
}
