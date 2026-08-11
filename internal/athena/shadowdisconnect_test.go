/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the /shadowdisconnect admin stealth ban. */

package athena

import (
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// setupShadowDisconnectTestDB opens a throwaway temp DB, mirroring
// setupAreaMuteTestDB in commands_area_mute_test.go. HandleClient's
// CheckBanned call touches db.IsBanned, which needs a non-nil db handle.
func setupShadowDisconnectTestDB(t *testing.T) func() {
	t.Helper()
	tmp, err := os.CreateTemp("", "athena-shadowdisconnect-*.db")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmp.Close()
	db.DBPath = tmp.Name()
	if err := db.Open(); err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return func() {
		db.Close()
		os.Remove(tmp.Name())
	}
}

// newShadowDisconnectTestClient builds a real Client (sendCh/done wired) over
// a net.Pipe whose peer is continuously drained, mirroring
// newCurseTestClient in curse_randomchar_test.go.
func newShadowDisconnectTestClient(t *testing.T, ipid string) (*Client, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	c := NewClient(a, ipid)
	go c.runWriter()
	go io.Copy(io.Discard, b) // drain everything the server writes
	t.Cleanup(func() {
		c.markClosed()
		b.Close()
	})
	return c, b
}

// TestIsShadowDisconnectedMemoryLifecycle confirms the in-memory set
// reflects add/remove/clear exactly, independent of the database.
func TestIsShadowDisconnectedMemoryLifecycle(t *testing.T) {
	const ipid = "shadowtest_lifecycle"
	t.Cleanup(func() { removeShadowDisconnectMemory(ipid) })

	if isShadowDisconnected(ipid) {
		t.Fatal("expected ipid to not be shadow-disconnected before it's added")
	}

	addShadowDisconnectMemory(ipid)
	if !isShadowDisconnected(ipid) {
		t.Fatal("expected ipid to be shadow-disconnected after addShadowDisconnectMemory")
	}

	removeShadowDisconnectMemory(ipid)
	if isShadowDisconnected(ipid) {
		t.Fatal("expected ipid to no longer be shadow-disconnected after removeShadowDisconnectMemory")
	}
}

// TestClearShadowDisconnectMemoryWipesEverything confirms the bulk-clear used
// by /shadowundisconnect all empties the set entirely, not just one entry.
func TestClearShadowDisconnectMemoryWipesEverything(t *testing.T) {
	ipids := []string{"shadowtest_clear_a", "shadowtest_clear_b", "shadowtest_clear_c"}
	for _, ipid := range ipids {
		addShadowDisconnectMemory(ipid)
	}

	clearShadowDisconnectMemory()

	for _, ipid := range ipids {
		if isShadowDisconnected(ipid) {
			t.Fatalf("expected %v to be cleared after clearShadowDisconnectMemory", ipid)
		}
	}
}

// TestShadowDisconnectNowClosesConnection confirms shadowDisconnectNow
// terminates the connection (closed flag set, done channel closed) without
// requiring any packet to be readable first -- the whole point being that no
// clean disconnect packet is ever sent to a shadow-disconnected client.
func TestShadowDisconnectNowClosesConnection(t *testing.T) {
	c, _ := newShadowDisconnectTestClient(t, "shadowtest_now")

	c.shadowDisconnectNow()

	if !c.closed.Load() {
		t.Fatal("expected shadowDisconnectNow to mark the client closed")
	}
	select {
	case <-c.done:
	default:
		t.Fatal("expected shadowDisconnectNow to close the done channel")
	}
}

// TestShadowDisconnectNowIdempotent confirms calling shadowDisconnectNow
// (which itself calls markClosed) is safe even if the connection was already
// torn down -- mirrors the multiclient loop in cmdShadowDisconnect where
// every connection sharing an IPID is dropped in a single pass.
func TestShadowDisconnectNowIdempotent(t *testing.T) {
	c, _ := newShadowDisconnectTestClient(t, "shadowtest_idempotent")

	c.shadowDisconnectNow()
	c.shadowDisconnectNow() // must not panic or block

	if !c.closed.Load() {
		t.Fatal("expected client to remain closed")
	}
}

// TestHandleClientRejectsShadowDisconnectedIpid confirms the defense-in-depth
// check at the top of HandleClient drops a shadow-disconnected IPID
// immediately, before it can register as a live client.
func TestHandleClientRejectsShadowDisconnectedIpid(t *testing.T) {
	defer setupShadowDisconnectTestDB(t)()

	const ipid = "shadowtest_handleclient"
	addShadowDisconnectMemory(ipid)
	t.Cleanup(func() { removeShadowDisconnectMemory(ipid) })

	a, b := net.Pipe()
	defer b.Close()
	c := NewClient(a, ipid)

	done := make(chan struct{})
	go func() {
		c.HandleClient()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleClient did not return promptly for a shadow-disconnected IPID")
	}

	if !c.closed.Load() {
		t.Fatal("expected the connection to be closed")
	}
}
