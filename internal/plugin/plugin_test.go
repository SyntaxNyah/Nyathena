package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The tests drive a real plugin process rather than a mock, since the whole
// point of this package is the process boundary: pipe framing, response
// matching and restart-after-crash are exactly what a mock would skip.
//
// The test binary re-executes itself as the plugin, keyed by an environment
// variable, which is the standard way to get a helper process without shipping
// a second binary.

const helperEnv = "ATHENA_PLUGIN_TEST_MODE"

// TestMain runs as the plugin when the harness sets the mode variable.
func TestMain(m *testing.M) {
	switch os.Getenv(helperEnv) {
	case "":
		os.Exit(m.Run())
	case "echo":
		helperEcho()
	case "crash":
		// Exits immediately, to exercise the supervisor's restart path.
		os.Exit(1)
	case "slow":
		helperSlow()
	}
	os.Exit(0)
}

// helperEcho answers every request by echoing its op back.
func helperEcho() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			return
		}
		if req.Op == "fail" {
			enc.Encode(Response{Id: req.Id, Error: "deliberate failure"}) //nolint:errcheck
			continue
		}
		data, _ := json.Marshal(map[string]string{"op": req.Op, "echo": string(req.Data)})
		enc.Encode(Response{Id: req.Id, Data: data}) //nolint:errcheck
	}
}

// helperSlow never answers, to exercise the request timeout.
func helperSlow() {
	dec := json.NewDecoder(os.Stdin)
	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// startHelper launches the test binary in the given helper mode.
func startHelper(t *testing.T, mode string, timeout time.Duration) *Plugin {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	os.Setenv(helperEnv, mode)
	t.Cleanup(func() { os.Unsetenv(helperEnv) })

	p := New("test", self, timeout, func(format string, v ...interface{}) {
		t.Logf(format, v...)
	})
	p.Start()
	t.Cleanup(p.Stop)
	waitRunning(t, p, 5*time.Second)
	return p
}

// waitRunning blocks until the plugin reports running, or fails the test.
func waitRunning(t *testing.T, p *Plugin, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if p.Running() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("plugin never came up")
}

// TestCallRoundTrip covers the basic request/response exchange.
func TestCallRoundTrip(t *testing.T) {
	p := startHelper(t, "echo", 3*time.Second)
	var out map[string]string
	if err := p.Call("challenge", map[string]string{"ipid": "1.2.3.4"}, &out); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if out["op"] != "challenge" {
		t.Errorf("op = %q, want challenge", out["op"])
	}
	if !strings.Contains(out["echo"], "1.2.3.4") {
		t.Errorf("request payload did not reach the plugin: %q", out["echo"])
	}
}

// TestConcurrentCallsAreMatchedById is the property that makes one pipe safe to
// share: interleaved requests must each receive their own response, never
// another caller's.
func TestConcurrentCallsAreMatchedById(t *testing.T) {
	p := startHelper(t, "echo", 5*time.Second)
	const n = 40
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			var out map[string]string
			want := fmt.Sprintf("ipid-%d", i)
			if err := p.Call("challenge", map[string]string{"ipid": want}, &out); err != nil {
				errs <- err
				return
			}
			if !strings.Contains(out["echo"], want) {
				errs <- fmt.Errorf("call %d got another caller's response: %q", i, out["echo"])
				return
			}
			errs <- nil
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

// TestCallSurfacesPluginError checks an error response reaches the caller
// rather than being mistaken for success.
func TestCallSurfacesPluginError(t *testing.T) {
	p := startHelper(t, "echo", 3*time.Second)
	if err := p.Call("fail", nil, nil); err == nil {
		t.Fatal("a plugin error response was reported as success")
	}
}

// TestCallTimesOut checks a plugin that never answers cannot block a caller
// indefinitely -- this runs on a connection's own goroutine during the join
// handshake, so a hung helper must not hang the join.
func TestCallTimesOut(t *testing.T) {
	p := startHelper(t, "slow", 200*time.Millisecond)
	start := time.Now()
	err := p.Call("challenge", nil, nil)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("call took %v; the timeout was not enforced", elapsed)
	}
}

// TestCallWhenNotRunning checks the unavailable path is an error, so callers
// must handle a missing plugin explicitly instead of silently continuing.
func TestCallWhenNotRunning(t *testing.T) {
	p := New("test", "", time.Second, func(string, ...interface{}) {})
	if err := p.Call("challenge", nil, nil); err != ErrUnavailable {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
}

// TestSuperviseRestartsAfterCrash checks a plugin that dies is brought back,
// so a helper that panics on one bad input does not stay down.
func TestSuperviseRestartsAfterCrash(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	os.Setenv(helperEnv, "crash")
	defer os.Unsetenv(helperEnv)

	// starts is written by the supervisor goroutine and read by this one on the
	// timeout path, so it has to be atomic even though the success path is
	// ordered by the channel close.
	var starts atomic.Int64
	done := make(chan struct{})
	var closeOnce sync.Once
	p := New("test", self, time.Second, func(format string, v ...interface{}) {
		if strings.Contains(format, "started") && starts.Add(1) >= 2 {
			closeOnce.Do(func() { close(done) })
		}
	})
	p.Start()
	defer p.Stop()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("plugin was started %d time(s); expected a restart after the crash", starts.Load())
	}
}
