/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /leash (leash.go). */

package athena

import (
	"fmt"
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

// addClient creates an extra character in the leash world. Multi-dog tests use
// it instead of bloating every single-dog test's fixture.
func (w *leashTestWorld) addClient(t *testing.T, uid, char int) (*Client, *captureConn) {
	t.Helper()
	conn := &captureConn{}
	client := &Client{
		conn: conn, uid: uid, ipid: fmt.Sprintf("ip-leash-%d", uid), char: char, area: w.cafe,
		pair: ClientPairInfo{wanted_id: -1}, forcePairUID: -1,
	}
	clients.AddClient(client)
	clients.RegisterUID(client)
	return client, conn
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

// TestLeashMultipleDogsSharePack verifies the multi-dog claim: one Leash is
// shared by the master and every dog, every dog gets /dog, the master's legacy
// pair slot points at the first dog, and the leash-owned PairGroup renders the
// full pack in master-first order.
func TestLeashMultipleDogsSharePack(t *testing.T) {
	w := newLeashTestWorld(t, false)
	dog2, _ := w.addClient(t, 5, 3)

	cmdLeash(w.mod, []string{"2", "3", "4", "5"}, "usage")

	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}
	dogs := []*Client{w.dog, w.other, dog2}
	if got := l.Dogs(); len(got) != len(dogs) {
		t.Fatalf("leash has %d dogs, want %d", len(got), len(dogs))
	}
	for _, dog := range dogs {
		if dog.Leash() != l {
			t.Fatalf("%v does not share the master's Leash", oocDisplayName(dog))
		}
		if !l.IsDog(dog) {
			t.Fatalf("%v is not reported as a dog", oocDisplayName(dog))
		}
		if !dog.HasActivePunishment(PunishmentDog) {
			t.Fatalf("%v did not receive the /dog punishment", oocDisplayName(dog))
		}
		if dog.ForcePairUID() != w.master.Uid() {
			t.Fatalf("%v ForcePairUID = %d, want master %d",
				oocDisplayName(dog), dog.ForcePairUID(), w.master.Uid())
		}
	}
	if w.master.HasActivePunishment(PunishmentDog) {
		t.Error("master should not receive the /dog punishment")
	}
	if w.master.ForcePairUID() != w.dog.Uid() {
		t.Errorf("master ForcePairUID = %d, want first dog %d", w.master.ForcePairUID(), w.dog.Uid())
	}

	group := w.master.PairGroup()
	if group == nil || !group.IsLeash() {
		t.Fatal("leash did not install a leash-owned PairGroup")
	}
	for _, c := range append([]*Client{w.master}, dogs...) {
		if c.PairGroup() != group {
			t.Fatalf("%v is not in the leash group", oocDisplayName(c))
		}
	}
	accepted := group.acceptedMembers()
	wantOrder := []*Client{w.master, w.dog, w.other, dog2}
	if len(accepted) != len(wantOrder) {
		t.Fatalf("group has %d accepted members, want %d", len(accepted), len(wantOrder))
	}
	for i, want := range wantOrder {
		if accepted[i] != want {
			t.Fatalf("group order[%d] = %v, want %v", i, oocDisplayName(accepted[i]), oocDisplayName(want))
		}
	}
	if gp := group.buildGP(); gp.GroupID != "leash-2" || len(gp.Members) != len(wantOrder) {
		t.Fatalf("GP = %+v, want group leash-2 with %d members", gp, len(wantOrder))
	}

	// The standard pair slot anchors on the first dog for the master and on
	// the master for each dog; additional_chars carries the whole roster.
	ms := &MSToClient{}
	applyPairGroupInjection(w.master, ms)
	if ms.PairedCharID != w.dog.CharIDStr() {
		t.Errorf("master PairedCharID = %q, want first dog %q", ms.PairedCharID, w.dog.CharIDStr())
	}
	if len(ms.AdditionalChars) != len(wantOrder) {
		t.Fatalf("master additional_chars = %d, want %d", len(ms.AdditionalChars), len(wantOrder))
	}
	for i, want := range wantOrder {
		if got := ms.AdditionalChars[i].CharID; got != want.CharID() {
			t.Errorf("additional_chars[%d] = %d, want %d", i, got, want.CharID())
		}
	}
	dogMS := &MSToClient{}
	applyPairGroupInjection(w.other, dogMS)
	if dogMS.PairedCharID != w.master.CharIDStr() {
		t.Errorf("dog PairedCharID = %q, want master %q", dogMS.PairedCharID, w.master.CharIDStr())
	}

	// All dogs follow the master into another area.
	if !w.master.ChangeArea(w.pizza) {
		t.Fatal("master could not move to Pizza Room")
	}
	for _, dog := range dogs {
		if dog.Area() != w.pizza {
			t.Errorf("%v did not follow to Pizza Room; area = %v",
				oocDisplayName(dog), dog.Area().Name())
		}
	}
}

// TestLeashAddsDogsAndCapsAtSix verifies that re-issuing /leash grows the pack
// and that a seventh dog is refused without disturbing the existing six.
func TestLeashAddsDogsAndCapsAtSix(t *testing.T) {
	w := newLeashTestWorld(t, false)
	extras := make([]*Client, 0, 4)
	for i := 0; i < 4; i++ {
		c, _ := w.addClient(t, 5+i, 3+i)
		extras = append(extras, c)
	}

	cmdLeash(w.mod, []string{"2", "3"}, "usage")
	cmdLeash(w.mod, []string{"2", "4", "5", "6", "7", "8"}, "usage")

	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}
	if got := len(l.Dogs()); got != maxLeashDogs {
		t.Fatalf("leash has %d dogs, want %d", got, maxLeashDogs)
	}
	if w.master.ForcePairUID() != w.dog.Uid() {
		t.Errorf("master ForcePairUID = %d, want first dog %d", w.master.ForcePairUID(), w.dog.Uid())
	}
	allDogs := append([]*Client{w.dog, w.other}, extras...)
	for _, dog := range allDogs {
		if dog.Leash() != l {
			t.Errorf("%v is not on the six-dog leash", oocDisplayName(dog))
		}
	}

	spare, _ := w.addClient(t, 9, 7)
	cmdLeash(w.mod, []string{"2", "9"}, "usage")
	if spare.Leash() != nil {
		t.Fatal("seventh dog was added to the leash")
	}
	if spare.HasActivePunishment(PunishmentDog) {
		t.Error("seventh dog received /dog despite the cap")
	}
	if w.master.Leash() != l || len(l.Dogs()) != maxLeashDogs {
		t.Error("cap rejection disturbed the existing six-dog leash")
	}
	if got := w.modConn.String(); !strings.Contains(got, "at most 6") {
		t.Errorf("issuer was not told about the cap; got %q", got)
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

// TestLeashDogDisconnectKeepsRemainingPack verifies that losing one dog shrinks
// the pack instead of releasing every other dog, and that the master's legacy
// pair slot moves to the new first dog.
func TestLeashDogDisconnectKeepsRemainingPack(t *testing.T) {
	w := newLeashTestWorld(t, false)
	dog2, _ := w.addClient(t, 5, 3)
	cmdLeash(w.mod, []string{"2", "3", "4", "5"}, "usage")
	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}

	clearPairLinksOnDisconnect(w.dog)

	if w.dog.Leash() != nil {
		t.Fatal("disconnecting dog kept its leash pointer")
	}
	if w.master.Leash() != l || w.other.Leash() != l || dog2.Leash() != l {
		t.Fatal("disconnecting one dog released the rest of the pack")
	}
	if got := l.Dogs(); len(got) != 2 || got[0] != w.other || got[1] != dog2 {
		t.Fatalf("remaining dogs = %v, want [other dog2]", got)
	}
	if w.master.ForcePairUID() != w.other.Uid() {
		t.Errorf("master ForcePairUID = %d, want new first dog %d",
			w.master.ForcePairUID(), w.other.Uid())
	}
	if w.other.ForcePairUID() != w.master.Uid() || dog2.ForcePairUID() != w.master.Uid() {
		t.Error("remaining dogs lost their pair link to the master")
	}
	group := w.master.PairGroup()
	if group == nil || !group.IsLeash() || len(group.acceptedMembers()) != 3 {
		t.Fatal("leash group was not rebuilt for the remaining pack")
	}
	if w.dog.PairGroup() != nil {
		t.Error("disconnected dog is still in the leash group")
	}
	if got := w.masterConn.String(); !strings.Contains(got, leashAbandonPopup) {
		t.Errorf("master did not get the abandonment popup; got %q", got)
	}

	clearPairLinksOnDisconnect(w.other)
	if got := l.Dogs(); len(got) != 1 || got[0] != dog2 {
		t.Fatalf("remaining dogs after second disconnect = %v, want [dog2]", got)
	}
	if w.master.ForcePairUID() != dog2.Uid() {
		t.Errorf("master ForcePairUID = %d, want %d", w.master.ForcePairUID(), dog2.Uid())
	}

	clearPairLinksOnDisconnect(dog2)
	if w.master.Leash() != nil || w.master.ForcePairUID() != -1 || w.master.PairWantedID() != -1 {
		t.Errorf("last dog disconnect left master state: leash=%v force=%d wanted=%d",
			w.master.Leash(), w.master.ForcePairUID(), w.master.PairWantedID())
	}
	if w.master.PairGroup() != nil {
		t.Error("last dog disconnect left the master in a pair group")
	}
}

// TestLeashGroupCannotBeLeftOrDisbanded pins the escape hatches: a dog cannot
// leave or deny the render group, and /unforcegrouppair ignores leash groups.
func TestLeashGroupCannotBeLeftOrDisbanded(t *testing.T) {
	w := newLeashTestWorld(t, false)
	dog2, _ := w.addClient(t, 5, 3)
	cmdLeash(w.mod, []string{"2", "3", "4", "5"}, "usage")
	l := w.master.Leash()
	group := w.master.PairGroup()
	if l == nil || group == nil || !group.IsLeash() {
		t.Fatal("multi-dog leash did not install its group")
	}

	cmdLeaveGroup(w.dog, nil, "")
	if w.dog.Leash() != l || w.dog.PairGroup() != group {
		t.Fatal("/leavegroup let a dog out of the leash group")
	}
	if got := w.dogConn.String(); !strings.Contains(got, leashAbandonPopup) {
		t.Errorf("dog did not get the abandonment popup on /leavegroup; got %q", got)
	}

	cmdDeny(w.other, nil, "")
	if w.other.Leash() != l || w.other.PairGroup() != group {
		t.Fatal("/deny let a dog out of the leash group")
	}

	cmdUnforceGroupPair(w.mod, nil, "")
	if w.master.Leash() != l || w.dog.Leash() != l || w.other.Leash() != l || dog2.Leash() != l {
		t.Fatal("/unforcegrouppair broke the leash claim")
	}
	if w.master.PairGroup() != group || w.dog.PairGroup() != group ||
		w.other.PairGroup() != group || dog2.PairGroup() != group {
		t.Fatal("/unforcegrouppair removed the leash render group")
	}
	if got := len(group.acceptedMembers()); got != 4 {
		t.Errorf("leash group has %d members after escape attempts, want 4", got)
	}
}

// TestForceUnpairDetachesOnlyTargetDog verifies /forceunpair on one dog detaches
// that dog and leaves the rest of the pack intact.
func TestForceUnpairDetachesOnlyTargetDog(t *testing.T) {
	w := newLeashTestWorld(t, false)
	dog2, _ := w.addClient(t, 5, 3)
	cmdLeash(w.mod, []string{"2", "3", "4", "5"}, "usage")
	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}

	cmdForceUnpair(w.mod, []string{"3"}, "usage")

	if w.dog.Leash() != nil {
		t.Fatal("/forceunpair left the target dog on the leash")
	}
	if w.master.Leash() != l || w.other.Leash() != l || dog2.Leash() != l {
		t.Fatal("/forceunpair released the whole pack instead of one dog")
	}
	if got := l.Dogs(); len(got) != 2 || got[0] != w.other || got[1] != dog2 {
		t.Fatalf("remaining dogs = %v, want [other dog2]", got)
	}
	if w.master.ForcePairUID() != w.other.Uid() {
		t.Errorf("master ForcePairUID = %d, want new first dog %d",
			w.master.ForcePairUID(), w.other.Uid())
	}
	if w.dog.ForcePairUID() != -1 || w.dog.PairWantedID() != -1 {
		t.Errorf("target dog kept pair state: force=%d wanted=%d",
			w.dog.ForcePairUID(), w.dog.PairWantedID())
	}
	if got := w.masterConn.String(); !strings.Contains(got, "was broken by") {
		t.Errorf("master was not told the leash was broken; got %q", got)
	}
	if group := w.master.PairGroup(); group == nil || !group.IsLeash() || len(group.acceptedMembers()) != 3 {
		t.Fatal("leash group was not rebuilt after /forceunpair")
	}
}

// TestForcePairDetachesOnlyTargetDog verifies /forcepair over one dog detaches
// that dog and installs the new pair without dissolving the remaining pack.
func TestForcePairDetachesOnlyTargetDog(t *testing.T) {
	w := newLeashTestWorld(t, false)
	dog2, _ := w.addClient(t, 5, 3)
	spare, _ := w.addClient(t, 6, 4)
	cmdLeash(w.mod, []string{"2", "3", "4", "5"}, "usage")
	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}

	cmdForcePair(w.mod, []string{"5", "6"}, "usage")

	if dog2.Leash() != nil {
		t.Fatal("/forcepair left the target dog on the leash")
	}
	if w.master.Leash() != l || w.dog.Leash() != l || w.other.Leash() != l {
		t.Fatal("/forcepair released the rest of the pack")
	}
	if got := l.Dogs(); len(got) != 2 {
		t.Fatalf("leash has %d dogs after /forcepair, want 2", len(got))
	}
	if dog2.ForcePairUID() != spare.Uid() || spare.ForcePairUID() != dog2.Uid() {
		t.Errorf("new force-pair not installed: dog2=%d spare=%d",
			dog2.ForcePairUID(), spare.ForcePairUID())
	}
	if group := w.master.PairGroup(); group == nil || !group.IsLeash() || len(group.acceptedMembers()) != 3 {
		t.Fatal("leash group was not rebuilt after /forcepair")
	}
}

// TestUnpunishDogDetachesOnlyThatDog verifies full /unpunish on one dog keeps
// the rest of the pack, while /unpunish on the master releases everyone.
func TestUnpunishDogDetachesOnlyThatDog(t *testing.T) {
	w := newLeashTestWorld(t, false)
	dog2, _ := w.addClient(t, 5, 3)
	cmdLeash(w.mod, []string{"2", "3", "4", "5"}, "usage")
	l := w.master.Leash()
	if l == nil {
		t.Fatal("master has no leash")
	}

	cmdUnpunish(w.mod, []string{"5"}, "usage")

	if dog2.Leash() != nil {
		t.Fatal("unpunished dog stayed on the leash")
	}
	if w.master.Leash() != l || w.dog.Leash() != l || w.other.Leash() != l {
		t.Fatal("unpunishing one dog released the rest of the pack")
	}
	if got := l.Dogs(); len(got) != 2 {
		t.Fatalf("leash has %d dogs after unpunishing the third, want 2", len(got))
	}
	if w.master.ForcePairUID() != w.dog.Uid() {
		t.Errorf("master ForcePairUID = %d, want unchanged first dog %d",
			w.master.ForcePairUID(), w.dog.Uid())
	}
	if dog2.ForcePairUID() != -1 || dog2.PairWantedID() != -1 {
		t.Errorf("unpunished dog kept pair state: force=%d wanted=%d",
			dog2.ForcePairUID(), dog2.PairWantedID())
	}

	cmdUnpunish(w.mod, []string{"2"}, "usage")
	if w.master.Leash() != nil || w.dog.Leash() != nil || w.other.Leash() != nil {
		t.Fatal("unpunishing the master did not release the whole pack")
	}
	if w.master.ForcePairUID() != -1 || w.dog.ForcePairUID() != -1 || w.other.ForcePairUID() != -1 {
		t.Error("master unpunish left pair state behind")
	}
}
