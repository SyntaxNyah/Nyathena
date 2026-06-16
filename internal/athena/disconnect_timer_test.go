/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the /dc / /dctime idle timer. */

package athena

import (
	"io"
	"net"
	"testing"
)

// newDCTestClient builds a real Client (sendCh/done wired) over a net.Pipe
// whose peer is continuously drained so SendServerMessage never blocks.
func newDCTestClient(t *testing.T) *Client {
	t.Helper()
	a, b := net.Pipe()
	c := NewClient(a, "dc-test")
	go c.runWriter()
	go io.Copy(io.Discard, b) // drain everything the server writes
	t.Cleanup(func() {
		c.markClosed()
		b.Close()
	})
	return c
}

// TestDCCommandStateTransitions exercises the full argument grammar of
// /dctime and asserts the resulting per-client idle setting.
func TestDCCommandStateTransitions(t *testing.T) {
	c := newDCTestClient(t)

	// Bare /dctime → default 1-hour countdown.
	cmdDC(c, nil, "")
	if got := c.dcIdleMinutes.Load(); got != dcDefaultMinutes {
		t.Errorf("bare /dctime set %d minutes, want default %d", got, dcDefaultMinutes)
	}

	// Explicit minutes.
	cmdDC(c, []string{"30"}, "")
	if got := c.dcIdleMinutes.Load(); got != 30 {
		t.Errorf("/dctime 30 set %d, want 30", got)
	}

	// Over-cap value clamps.
	cmdDC(c, []string{"9999999"}, "")
	if got := c.dcIdleMinutes.Load(); got != dcMaxMinutes {
		t.Errorf("over-cap value set %d, want clamp %d", got, dcMaxMinutes)
	}

	// off / 0 / stop all disable.
	for _, off := range []string{"off", "0", "stop", "cancel", "disable", "none"} {
		cmdDC(c, []string{"60"}, "")
		cmdDC(c, []string{off}, "")
		if got := c.dcIdleMinutes.Load(); got != 0 {
			t.Errorf("/dctime %s left timer at %d, want 0", off, got)
		}
	}

	// Garbage input does not change a disabled timer.
	c.dcIdleMinutes.Store(0)
	cmdDC(c, []string{"banana"}, "")
	if got := c.dcIdleMinutes.Load(); got != 0 {
		t.Errorf("/dctime banana changed timer to %d, want 0", got)
	}

	// Negative is rejected.
	cmdDC(c, []string{"-5"}, "")
	if got := c.dcIdleMinutes.Load(); got != 0 {
		t.Errorf("/dctime -5 changed timer to %d, want 0", got)
	}

	// status never mutates the setting.
	cmdDC(c, []string{"45"}, "")
	cmdDC(c, []string{"status"}, "")
	if got := c.dcIdleMinutes.Load(); got != 45 {
		t.Errorf("/dctime status mutated the timer to %d, want 45", got)
	}
}

// TestDCActivityResetsClock confirms an IC/OOC activity touch advances the
// last-activity stamp the watcher reads.
func TestDCActivityResetsClock(t *testing.T) {
	c := newDCTestClient(t)
	c.dcLastActivityNano.Store(0)
	c.dcTouchActivity()
	if c.dcLastActivityNano.Load() == 0 {
		t.Error("dcTouchActivity did not record an activity timestamp")
	}
}
