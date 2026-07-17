/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the /curserandomchar admin curse. */

package athena

import (
	"io"
	"net"
	"testing"
	"time"
)

// newCurseTestClient builds a real Client (sendCh/done wired) over a
// net.Pipe whose peer is continuously drained so SendServerMessage never
// blocks.
func newCurseTestClient(t *testing.T) *Client {
	t.Helper()
	a, b := net.Pipe()
	c := NewClient(a, "curse-test")
	go c.runWriter()
	go io.Copy(io.Discard, b) // drain everything the server writes
	t.Cleanup(func() {
		c.markClosed()
		b.Close()
	})
	return c
}

// TestArmCurseRandomCharStartsWatcherOnce confirms arming sets the active
// flag and spawns exactly one watcher goroutine, and that re-arming an
// already-armed client does not spawn a second one.
func TestArmCurseRandomCharStartsWatcherOnce(t *testing.T) {
	c := newCurseTestClient(t)

	c.armCurseRandomChar()
	if !c.curseRandomCharActive.Load() {
		t.Fatal("armCurseRandomChar did not set the active flag")
	}
	if !c.curseRandomCharWatcherStarted.Load() {
		t.Fatal("armCurseRandomChar did not start the watcher")
	}

	// Re-arming must not attempt to spawn a second goroutine (CAS gate).
	c.armCurseRandomChar()
	if !c.curseRandomCharActive.Load() {
		t.Fatal("re-arming cleared the active flag")
	}
}

// TestDisarmCurseRandomCharStopsWatcher confirms disarming causes the
// watcher goroutine to notice and exit (resetting the started flag) within
// its worst-case 5-second tick interval, without needing a disconnect.
func TestDisarmCurseRandomCharStopsWatcher(t *testing.T) {
	c := newCurseTestClient(t)
	c.armCurseRandomChar()

	c.disarmCurseRandomChar()
	if c.curseRandomCharActive.Load() {
		t.Fatal("disarmCurseRandomChar did not clear the active flag")
	}

	deadline := time.After(6 * time.Second)
	for {
		if !c.curseRandomCharWatcherStarted.Load() {
			return // watcher exited cleanly
		}
		select {
		case <-deadline:
			t.Fatal("watcher goroutine did not exit within 6s of being disarmed")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestCurseRandomCharWatchExitsOnDisconnect confirms the watcher goroutine
// exits promptly (not waiting out its random tick interval) once the
// connection closes, so a cursed-but-disconnected client can never leak a
// goroutine.
func TestCurseRandomCharWatchExitsOnDisconnect(t *testing.T) {
	c := newCurseTestClient(t)
	c.armCurseRandomChar()

	c.markClosed()

	deadline := time.After(1 * time.Second)
	for {
		if !c.curseRandomCharWatcherStarted.Load() {
			return // watcher exited promptly via client.done
		}
		select {
		case <-deadline:
			t.Fatal("watcher goroutine did not exit promptly after disconnect")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
