/* Athena - A server for Attorney Online 2 written in Go
Copyright (C) 2022 MangosArentLiterature <mango@transmenace.dev>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>. */

package athena

// Nyathena fork addition: regression test for a UID-reuse race in
// clientCleanup that could corrupt any client's live player list (the PR/PU
// push mechanism that lets clients like AsyncAO track joins/leaves without
// polling /gas).

import (
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/settings"
	"github.com/MangosArentLiterature/Athena/internal/uidmanager"
)

// capturingConn is like testConn but records every write, so a test can
// inspect exactly what was sent to a client instead of discarding it.
type capturingConn struct {
	mu      sync.Mutex
	written []string
}

func (c *capturingConn) Read(_ []byte) (int, error) { return 0, io.EOF }

func (c *capturingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.written = append(c.written, string(p))
	c.mu.Unlock()
	return len(p), nil
}

func (c *capturingConn) Close() error                       { return nil }
func (c *capturingConn) LocalAddr() net.Addr                { return testAddr("local") }
func (c *capturingConn) RemoteAddr() net.Addr               { return testAddr("remote") }
func (c *capturingConn) SetDeadline(_ time.Time) error      { return nil }
func (c *capturingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *capturingConn) SetWriteDeadline(_ time.Time) error { return nil }

func (c *capturingConn) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.written))
	copy(out, c.written)
	return out
}

// TestClientCleanupReleasesUIDAfterRemovalIsVisible is a regression test for
// a UID-reuse race that used to exist in clientCleanup: the departing
// client's UID was pushed back onto the free pool (uids.ReleaseUid) before
// the PR "remove" broadcast went out and before the client was dropped from
// the UID registry (clients.RemoveClient). A concurrently-joining client
// could pop that same recycled UID, register itself, and broadcast its own
// PR/PU "add" for it — which any client building a live roster purely from
// PR/PU (exactly what this mechanism exists so AsyncAO doesn't have to poll
// /gas for) could see immediately clobbered by the departing client's
// belated "remove" for the same UID number, corrupting the roster.
//
// This doesn't rely on sleep-based timing to catch a regression. uids'
// internal mutex is shared between Len() and ReleaseUid, so the moment a
// concurrent poller observes the pool grow, everything the cleanup goroutine
// did beforehand — the broadcast, the registry removal — is guaranteed
// visible too (Go's mutex happens-before rule). If ReleaseUid is ever moved
// back to its old, early position, this test fails deterministically rather
// than flaking.
func TestClientCleanupReleasesUIDAfterRemovalIsVisible(t *testing.T) {
	origClients, origAreas, origAreaIndexMap, origUids, origConfig := clients, areas, areaIndexMap, uids, config
	t.Cleanup(func() {
		clients, areas, areaIndexMap, uids, config = origClients, origAreas, origAreaIndexMap, origUids, origConfig
	})
	config = &settings.Config{}

	testArea := makeTestArea("Lobby")
	areas = []*area.Area{testArea}
	areaIndexMap = map[*area.Area]int{testArea: 0}
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	var uidPool uidmanager.UidManager
	uidPool.InitHeap(1)
	uid := uidPool.GetUid() // the only uid in the pool; pool is now empty, as if a client already holds it
	uids = &uidPool

	// Two occupants so clientCleanup's PlayerCount()<=1 branch (area.Reset())
	// doesn't fire and complicate the scenario under test.
	testArea.AddChar(-1)
	testArea.AddChar(-1)

	spyConn := &capturingConn{}
	spy := &Client{conn: spyConn, uid: 99, char: -1, area: testArea,
		forcePairUID: -1, pair: ClientPairInfo{wanted_id: -1},
		done: make(chan struct{})}
	clients.AddClient(spy)
	clients.RegisterUID(spy)

	departing := &Client{conn: &testConn{}, uid: uid, char: -1, area: testArea,
		forcePairUID: -1, pair: ClientPairInfo{wanted_id: -1},
		done: make(chan struct{})}
	clients.AddClient(departing)
	clients.RegisterUID(departing)

	if n := uidPool.Len(); n != 0 {
		t.Fatalf("uid pool Len() = %d before cleanup, want 0 (uid held by departing client)", n)
	}

	done := make(chan struct{})
	go func() {
		departing.clientCleanup()
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for uidPool.Len() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for clientCleanup to release the uid")
		}
		time.Sleep(time.Microsecond)
	}
	// From this point on, everything clientCleanup did before calling
	// uids.ReleaseUid is guaranteed visible (mutex happens-before) — the
	// checks below are not racing the cleanup goroutine.

	if c := clients.GetClientByUID(uid); c != nil {
		t.Errorf("GetClientByUID(%d) = %v once the uid was released, want nil — the departing client's registry entry should already be gone", uid, c)
	}

	want := "PR#" + strconv.Itoa(uid) + "#1#%"
	found := false
	for _, w := range spyConn.snapshot() {
		if w == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("spy client had not received %q by the time the uid was released; a client building its roster from PR/PU would see the uid become reusable before learning the old occupant left, got %v", want, spyConn.snapshot())
	}

	<-done
}
