/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the /randomarea punishment. */

package athena

import (
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// TestRandomAreaTypeRoundTrip checks that "randomarea" parses to its enum and
// stringifies back, matching the round-trip convention used for every other
// punishment type.
func TestRandomAreaTypeRoundTrip(t *testing.T) {
	if got := parsePunishmentType("randomarea"); got != PunishmentRandomArea {
		t.Errorf("parsePunishmentType(%q) = %v, want %v", "randomarea", got, PunishmentRandomArea)
	}
	if got := PunishmentRandomArea.String(); got != "randomarea" {
		t.Errorf("PunishmentRandomArea.String() = %q, want %q", got, "randomarea")
	}
	if PunishmentRandomArea == PunishmentNone {
		t.Error("PunishmentRandomArea must not equal PunishmentNone")
	}
}

// TestRandomOtherAreaExcludesRestrictedAreas confirms randomOtherArea never
// returns the current area, a /lock-ed area, an /adminlock-ed area, or a
// punishment-safe area, and does eventually return each open candidate.
func TestRandomOtherAreaExcludesRestrictedAreas(t *testing.T) {
	origKill := punishmentsGloballyDisabled.Load()
	punishmentsGloballyDisabled.Store(false)
	t.Cleanup(func() { punishmentsGloballyDisabled.Store(origKill) })

	current := makeTestArea("Current")
	open1 := makeTestArea("Open1")
	open2 := makeTestArea("Open2")
	locked := makeTestArea("Locked")
	locked.SetLock(area.LockLocked)
	adminLocked := makeTestArea("AdminLocked")
	adminLocked.SetAdminLocked(true)
	safe := makeTestArea("Safe")
	safe.SetPunishmentSafe(true)

	cleanup := setupTestAreas([]*area.Area{current, open1, open2, locked, adminLocked, safe})
	defer cleanup()

	seen := map[*area.Area]bool{}
	for i := 0; i < 200; i++ {
		got := randomOtherArea(current)
		if got == nil {
			t.Fatal("randomOtherArea returned nil despite open candidates existing")
		}
		if got == current {
			t.Fatal("randomOtherArea returned the current area")
		}
		if got == locked {
			t.Fatal("randomOtherArea returned a /lock-ed area")
		}
		if got == adminLocked {
			t.Fatal("randomOtherArea returned an /adminlock-ed area")
		}
		if got == safe {
			t.Fatal("randomOtherArea returned a punishment-safe area")
		}
		seen[got] = true
	}
	if !seen[open1] || !seen[open2] {
		t.Error("randomOtherArea never returned one of the open candidates over 200 draws")
	}
}

// TestRandomOtherAreaReturnsNilWhenNoOpenCandidates confirms the helper fails
// safe (nil, not a panic or a restricted pick) when nothing qualifies.
func TestRandomOtherAreaReturnsNilWhenNoOpenCandidates(t *testing.T) {
	current := makeTestArea("Current")
	locked := makeTestArea("Locked")
	locked.SetLock(area.LockLocked)

	cleanup := setupTestAreas([]*area.Area{current, locked})
	defer cleanup()

	if got := randomOtherArea(current); got != nil {
		t.Fatalf("expected nil with no open candidates, got %v", got.Name())
	}
}

// TestAddPunishmentByArmsRandomAreaWatchOnce confirms applying the
// punishment starts exactly one watcher goroutine, and that re-applying it
// (e.g. a mod refreshing the duration) does not spawn a second one.
func TestAddPunishmentByArmsRandomAreaWatchOnce(t *testing.T) {
	c := newCurseTestClient(t)
	c.SetArea(makeTestArea("Solo"))

	c.AddPunishmentBy(PunishmentRandomArea, time.Minute, "test", IssuerMod)
	if !c.randomAreaWatcherStarted.Load() {
		t.Fatal("AddPunishmentBy(PunishmentRandomArea) did not start the watcher")
	}
	if !c.HasActivePunishment(PunishmentRandomArea) {
		t.Fatal("AddPunishmentBy(PunishmentRandomArea) did not record an active punishment")
	}

	c.AddPunishmentBy(PunishmentRandomArea, 2*time.Minute, "refreshed", IssuerMod)
	if !c.randomAreaWatcherStarted.Load() {
		t.Fatal("re-applying the punishment cleared the watcher-started flag")
	}
}

// TestRandomAreaWatchExitsOnDisconnect confirms the watcher goroutine exits
// promptly once the connection closes, so a punished-but-disconnected client
// can never leak a goroutine, mirroring curseRandomCharWatch's equivalent test.
func TestRandomAreaWatchExitsOnDisconnect(t *testing.T) {
	c := newCurseTestClient(t)
	c.SetArea(makeTestArea("Solo"))
	c.AddPunishmentBy(PunishmentRandomArea, time.Minute, "test", IssuerMod)

	c.markClosed()

	deadline := time.After(1 * time.Second)
	for {
		if !c.randomAreaWatcherStarted.Load() {
			return // watcher exited promptly via client.done
		}
		select {
		case <-deadline:
			t.Fatal("watcher goroutine did not exit promptly after disconnect")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// TestRandomAreaWatchExitsWhenPunishmentRemoved confirms that removing the
// punishment (expiry or /unpunish -t randomarea) makes the watcher exit on
// its next tick instead of continuing to warp the client forever. The wait
// bounds are temporarily shrunk so the test doesn't wait out a real
// 20-45 second tick.
func TestRandomAreaWatchExitsWhenPunishmentRemoved(t *testing.T) {
	origMin, origMax := randomAreaMinWait, randomAreaMaxWait
	randomAreaMinWait, randomAreaMaxWait = 10*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { randomAreaMinWait, randomAreaMaxWait = origMin, origMax })

	c := newCurseTestClient(t)
	c.SetArea(makeTestArea("Solo")) // non-nil so a tick that beats the removal can't panic

	c.AddPunishmentBy(PunishmentRandomArea, time.Minute, "test", IssuerMod)
	if !c.randomAreaWatcherStarted.Load() {
		t.Fatal("watcher did not start")
	}

	c.RemovePunishment(PunishmentRandomArea)

	deadline := time.After(2 * time.Second)
	for {
		if !c.randomAreaWatcherStarted.Load() {
			return // watcher noticed on its next tick and exited
		}
		select {
		case <-deadline:
			t.Fatal("watcher goroutine did not exit after the punishment was removed")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// TestRandomAreaWatchWarpsToADifferentOpenArea is an end-to-end check that an
// active punishment actually moves the client: with only one open
// destination available, the client must land there within a couple of
// shrunk ticks.
func TestRandomAreaWatchWarpsToADifferentOpenArea(t *testing.T) {
	origMin, origMax := randomAreaMinWait, randomAreaMaxWait
	randomAreaMinWait, randomAreaMaxWait = 10*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { randomAreaMinWait, randomAreaMaxWait = origMin, origMax })

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	start := makeTestArea("Start")
	dest := makeTestArea("Elsewhere")
	cleanupAreas := setupTestAreas([]*area.Area{start, dest})
	t.Cleanup(cleanupAreas)

	c := &Client{conn: &testConn{}, uid: 9, ipid: "ip-randomarea-warp", area: start, char: -1, jailAreaID: -1}
	clients.AddClient(c)
	clients.RegisterUID(c)

	c.AddPunishmentBy(PunishmentRandomArea, time.Minute, "test", IssuerMod)

	deadline := time.After(2 * time.Second)
warpLoop:
	for {
		if c.Area() == dest {
			break warpLoop // warped successfully
		}
		select {
		case <-deadline:
			t.Fatalf("client was not warped to the other area within 2s; still in %v", c.Area().Name())
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Stop the watcher and wait for it to fully exit before returning: it's
	// still running (the punishment has a minute left) and would otherwise
	// keep reading areas/clients on its next shrunk tick, racing the
	// t.Cleanup calls above that restore those globals once this test
	// function returns.
	c.RemovePunishment(PunishmentRandomArea)
	stopDeadline := time.After(2 * time.Second)
	for c.randomAreaWatcherStarted.Load() {
		select {
		case <-stopDeadline:
			t.Fatal("watcher did not stop after RemovePunishment")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
