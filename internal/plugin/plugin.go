// Package plugin runs an external helper program alongside the server and
// exchanges JSON messages with it over a pipe.
//
// Why a subprocess rather than Go's plugin package: buildmode=plugin requires
// the host and the .so to be built with the identical Go toolchain and the
// identical version of every shared dependency, so a routine `go get` breaks
// every plugin until each is rebuilt -- an unacceptable failure mode for a
// server that is expected to stay up. A subprocess has none of that coupling,
// can be written in any language, cannot take the server down when it crashes,
// and keeps a clean arms-length boundary between the AGPL server and whatever
// licence the operator's own helper carries.
//
// # Protocol
//
// Newline-delimited JSON in both directions. The server writes one Request per
// line to the plugin's stdin; the plugin writes one Response per line to its
// stdout, echoing the request's Id. Responses may arrive in any order -- each
// is matched by Id -- so a plugin is free to answer concurrently.
//
// Anything the plugin writes to stderr is copied into the server log, prefixed
// with the plugin name, which is the intended way for a plugin to report.
//
// Plugins must not write anything but protocol lines to stdout.
package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Request is one message from the server to a plugin.
type Request struct {
	Id   uint64          `json:"id"`
	Op   string          `json:"op"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Response is one message from a plugin back to the server.
type Response struct {
	Id    uint64          `json:"id"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Logf receives plugin lifecycle and stderr messages.
type Logf func(format string, v ...interface{})

// ErrUnavailable is returned when no plugin process is currently running.
var ErrUnavailable = errors.New("plugin not running")

// Plugin is a supervised external helper process.
//
// A Plugin is safe for concurrent use: Call multiplexes over a single pipe and
// matches responses by id, and the supervisor restarts the process (with
// backoff) whenever it exits, so a plugin that crashes on a bad input recovers
// on its own rather than staying down until someone notices.
type Plugin struct {
	name    string
	command string
	args    []string
	timeout time.Duration
	logf    Logf

	nextID atomic.Uint64

	mu      sync.Mutex
	stdin   io.WriteCloser
	running bool
	pending map[uint64]chan Response

	stopOnce sync.Once
	stopCh   chan struct{}
}

// New creates a plugin runner. The command string is split on whitespace, so
// simple arguments can be given inline ("./captcha --config foo").
// Call Start to launch it.
func New(name, command string, timeout time.Duration, logf Logf) *Plugin {
	fields := strings.Fields(command)
	p := &Plugin{
		name:    name,
		timeout: timeout,
		logf:    logf,
		pending: make(map[uint64]chan Response),
		stopCh:  make(chan struct{}),
	}
	if len(fields) > 0 {
		p.command = fields[0]
		p.args = fields[1:]
	}
	if p.timeout <= 0 {
		p.timeout = 3 * time.Second
	}
	return p
}

// Start launches the plugin and supervises it until Stop is called.
func (p *Plugin) Start() {
	if p.command == "" {
		return
	}
	go p.supervise()
}

// Stop terminates the plugin and stops the supervisor.
func (p *Plugin) Stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
	p.mu.Lock()
	if p.stdin != nil {
		p.stdin.Close()
	}
	p.mu.Unlock()
}

// Running reports whether a plugin process is currently up.
func (p *Plugin) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// supervise runs the plugin, restarting it with capped exponential backoff
// whenever it exits. A plugin that fails instantly and repeatedly therefore
// costs a log line every 30 seconds rather than a spin loop.
func (p *Plugin) supervise() {
	backoff := time.Second
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}
		start := time.Now()
		if err := p.runOnce(); err != nil {
			p.logf("plugin %v exited: %v", p.name, err)
		} else {
			p.logf("plugin %v exited", p.name)
		}
		select {
		case <-p.stopCh:
			return
		default:
		}
		// A process that stayed up a while was healthy; reset the backoff so a
		// long-lived plugin that finally crashes restarts promptly.
		if time.Since(start) > 30*time.Second {
			backoff = time.Second
		}
		p.logf("plugin %v restarting in %v", p.name, backoff)
		select {
		case <-p.stopCh:
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// runOnce launches the process and pumps its stdout until it exits.
func (p *Plugin) runOnce() error {
	cmd := exec.Command(p.command, p.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	p.logf("plugin %v started (%v)", p.name, p.command)

	p.mu.Lock()
	p.stdin = stdin
	p.running = true
	p.mu.Unlock()

	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			p.logf("plugin %v: %v", p.name, sc.Text())
		}
	}()

	// Read responses until the pipe closes. Long lines are expected (a
	// challenge prompt plus its answers), so the scanner gets a generous
	// buffer rather than the 64KiB default.
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			p.logf("plugin %v: unparseable response: %v", p.name, err)
			continue
		}
		p.deliver(resp)
	}

	p.mu.Lock()
	p.running = false
	p.stdin = nil
	// Fail every in-flight request rather than leaving callers blocked until
	// their individual timeouts expire.
	for id, ch := range p.pending {
		select {
		case ch <- Response{Id: id, Error: "plugin exited"}:
		default:
		}
		delete(p.pending, id)
	}
	p.mu.Unlock()

	return cmd.Wait()
}

// deliver routes a response to the caller waiting on its id.
func (p *Plugin) deliver(resp Response) {
	p.mu.Lock()
	ch, ok := p.pending[resp.Id]
	if ok {
		delete(p.pending, resp.Id)
	}
	p.mu.Unlock()
	if !ok {
		return // late response to a timed-out request
	}
	select {
	case ch <- resp:
	default:
	}
}

// Call sends one request and waits for its response.
//
// out, when non-nil, is unmarshalled from the response's data field. A plugin
// that is down, slow, or answering with an error surfaces as an error here so
// every caller has to decide explicitly what to do without it -- there is no
// silent success.
func (p *Plugin) Call(op string, in interface{}, out interface{}) error {
	var raw json.RawMessage
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		raw = b
	}
	id := p.nextID.Add(1)
	req := Request{Id: id, Op: op, Data: raw}
	line, err := json.Marshal(req)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	ch := make(chan Response, 1)
	p.mu.Lock()
	if !p.running || p.stdin == nil {
		p.mu.Unlock()
		return ErrUnavailable
	}
	p.pending[id] = ch
	w := p.stdin
	// The write happens under the lock so two concurrent Calls cannot interleave
	// bytes of different lines on the pipe.
	_, werr := w.Write(line)
	p.mu.Unlock()
	if werr != nil {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return werr
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	select {
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return fmt.Errorf("plugin %v: %v timed out after %v", p.name, op, p.timeout)
	case resp := <-ch:
		if resp.Error != "" {
			return fmt.Errorf("plugin %v: %v", p.name, resp.Error)
		}
		if out != nil && len(resp.Data) > 0 {
			return json.Unmarshal(resp.Data, out)
		}
		return nil
	}
}
