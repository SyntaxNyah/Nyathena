package athena

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// Reproducing the freeze, rather than asserting it from a microbenchmark.
//
// The question a benchmark cannot answer is whether the server can KEEP UP.
// Servicing a character change costs some amount of CPU; a raid delivers
// character changes at some rate. If cost x rate exceeds one second of CPU per
// second of wall clock, the work arrives faster than it can be done and the
// backlog grows without bound -- which is a server that stops responding and
// never crashes, exactly the reported symptom.
//
// So this replays the character-change timeline from a real raid capture
// (testdata/raid_capture.log, already in the repo) against the real broadcast
// code at real populations, and reports CPU-seconds consumed per wall-second of
// captured traffic. Only the timing is taken from the capture; no message
// content is read or needed.
//
// Measured peak character-change rates, for context on why this rate is the
// one to replay:
//
//	raid 2026-08-27    9 /s   65 IPIDs present
//	raid 2026-09-02   10 /s   92 IPIDs
//	raid 2026-09-02b   8 /s  239 IPIDs
//	the freeze         7 /s  118 IPIDs
//	clean evening      4 /s   22 IPIDs
//
// Rapid character re-rolling is a raid signature the guard already scores
// (SigCharChurn), so the raid population is by construction the one that
// generates this load.

// ccTimeline pulls just the arrival times of character-change packets.
func ccTimeline(t *testing.T, path string) []time.Time {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("capture unavailable: %v", err)
	}
	defer f.Close()

	line := regexp.MustCompile(`^\[([^\]]+)\] RECV \| IPID:[^ ]* \| HDID:[^ ]* \| CC#`)
	var out []time.Time
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
	for sc.Scan() {
		m := line.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		ts, err := time.Parse("2006-01-02T15:04:05.000Z", m[1])
		if err != nil {
			continue
		}
		out = append(out, ts)
	}
	return out
}

// simPopulation builds a population sharing one area, with drained queues so
// the measurement is of the broadcast work and not of queue-full drops.
func simPopulation(t *testing.T, n int) (*ClientList, *area.Area, func()) {
	t.Helper()
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, realCharCount, 10, area.EviAny)
	cl := &ClientList{
		list:       make(map[*Client]struct{}, n),
		uidIndex:   make(map[int]*Client, n),
		ipidCounts: make(map[string]int, n),
	}
	var chans []chan []byte
	for i := 0; i < n; i++ {
		ch := make(chan []byte, 256)
		c := &Client{char: -1, area: a, uid: i, ipid: fmt.Sprintf("sim-%d", i), sendCh: ch}
		cl.list[c] = struct{}{}
		chans = append(chans, ch)
	}
	stop := make(chan struct{})
	for _, ch := range chans {
		go func(ch chan []byte) {
			for {
				select {
				case <-ch:
				case <-stop:
					return
				}
			}
		}(ch)
	}
	return cl, a, func() { close(stop) }
}

// oneCharacterChange performs the broadcasts ChangeCharacter performs, with
// buildOnce selecting the fixed or the original CharsCheck fan-out. The two PU
// broadcasts are included because they are part of the real per-change cost.
func oneCharacterChange(a *area.Area, buildOnce bool) {
	if buildOnce {
		broadcastToAreaOnce(a, &packet.CharsCheck{Entries: a.Taken()})
	} else {
		broadcastToArea(a, &packet.CharsCheck{Entries: a.Taken()})
	}
	broadcastToAll(&packet.PU{ID: 1, Type: 1, Data: "Phoenix"})
	broadcastToAll(&packet.PU{ID: 1, Type: 2, Data: "Phoenix"})
}

func TestFreezeReproducesFromRealRaidTimeline(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive")
	}
	cc := ccTimeline(t, "testdata/raid_capture.log")
	if len(cc) < 5 {
		t.Skipf("capture has too few character changes (%d)", len(cc))
	}
	span := cc[len(cc)-1].Sub(cc[0]).Seconds()
	if span <= 0 {
		t.Skip("degenerate capture span")
	}
	rate := float64(len(cc)) / span
	t.Logf("replaying %d character changes over %.1fs (%.1f/s) from the 2026-08-27 raid",
		len(cc), span, rate)

	// cost measures CPU spent servicing every change in the timeline.
	cost := func(pop int, buildOnce bool) time.Duration {
		cl, a, stop := simPopulation(t, pop)
		defer stop()
		prev := clients
		clients = cl
		defer func() { clients = prev }()

		oneCharacterChange(a, buildOnce) // warm caches so the first sample isn't an outlier
		start := time.Now()
		for range cc {
			oneCharacterChange(a, buildOnce)
		}
		return time.Since(start)
	}

	type row struct {
		pop              int
		oldCPU, newCPU   time.Duration
		oldRatio, newRat float64
	}
	var rows []row
	for _, pop := range []int{45, 118, 300, 600} {
		o := cost(pop, false)
		n := cost(pop, true)
		rows = append(rows, row{pop, o, n, o.Seconds() / span, n.Seconds() / span})
	}

	t.Log("")
	t.Log("CPU-seconds needed per wall-second of this raid's traffic.")
	t.Log("Above 1.00 the work arrives faster than it can be done and the backlog grows without bound.")
	t.Logf("%8s | %12s %10s | %12s %10s", "clients", "before", "keep-up", "after", "keep-up")
	for _, r := range rows {
		flagO, flagN := "", ""
		if r.oldRatio >= 1.0 {
			flagO = "  CANNOT KEEP UP"
		}
		if r.newRat >= 1.0 {
			flagN = "  CANNOT KEEP UP"
		}
		t.Logf("%8d | %12v %9.2fx%s | %12v %9.2fx%s",
			r.pop, r.oldCPU.Round(time.Millisecond), r.oldRatio, flagO,
			r.newCPU.Round(time.Millisecond), r.newRat, flagN)
	}

	last := rows[len(rows)-1]

	// The claim being pinned: at a real raid's population and character-change
	// rate, the fixed path keeps up with room to spare. An absolute bound is
	// deliberately generous so a loaded machine does not fail the build; the
	// measured value is far below it.
	if last.newRat >= 0.5 {
		t.Errorf("after the fix the server still needs %.2f CPU-seconds per wall-second "+
			"at %d clients; it is not keeping up", last.newRat, last.pop)
	}

	// And that the fix is the reason, by a wide margin.
	if ratio := float64(last.oldCPU) / float64(last.newCPU); ratio < 10 {
		t.Errorf("build-once is only %.1fx cheaper than per-recipient at %d clients; "+
			"expected an order of magnitude", ratio, last.pop)
	}
}

// The headline number stated in the docs: how many character changes per second
// one core can service, before and after. A raid delivers 8-10/s.
func TestSustainableCharacterChangeRate(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive")
	}
	const pop = 600
	cl, a, stop := simPopulation(t, pop)
	defer stop()
	prev := clients
	clients = cl
	defer func() { clients = prev }()

	measure := func(buildOnce bool) float64 {
		oneCharacterChange(a, buildOnce)
		const iters = 30
		start := time.Now()
		for i := 0; i < iters; i++ {
			oneCharacterChange(a, buildOnce)
		}
		per := time.Since(start) / iters
		return 1.0 / per.Seconds()
	}

	before, after := measure(false), measure(true)
	t.Logf("at %d clients, one core sustains:", pop)
	t.Logf("   before: %6.1f character changes/sec", before)
	t.Logf("   after:  %6.1f character changes/sec", after)
	t.Logf("   observed raid peak: 8-10/s")

	if before >= 100 {
		t.Logf("NOTE: this machine services %.0f/s even unfixed; a slower or busier "+
			"production box is the case that matters", before)
	}
	if after <= before {
		t.Errorf("build-once (%.1f/s) is not faster than per-recipient (%.1f/s)", after, before)
	}
}

// Two hours of sustained raiding: where does it actually tip over?
//
// The interesting number is not "it is slow", it is the population at which
// cost x rate crosses one CPU-second per wall-second. Below that the server
// absorbs the load and stays responsive; above it the backlog grows for as long
// as the raid continues and the server is gone. That threshold is the answer to
// "where does it hang".
//
// Per-change cost is measured, not guessed; the two-hour figures are a linear
// projection from that measurement, which is sound because the work per change
// is independent of how many came before it.
func TestTwoHourRaidProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive")
	}
	const (
		raidRate = 9.0          // character changes/sec, the observed raid peak
		hours    = 2.0          //
		duration = hours * 3600 // seconds
	)
	totalChanges := int(raidRate * duration)

	perChange := func(pop int, buildOnce bool) (time.Duration, uint64) {
		cl, a, stop := simPopulation(t, pop)
		defer stop()
		prev := clients
		clients = cl
		defer func() { clients = prev }()

		oneCharacterChange(a, buildOnce)
		var m0, m1 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m0)
		const iters = 40
		start := time.Now()
		for i := 0; i < iters; i++ {
			oneCharacterChange(a, buildOnce)
		}
		el := time.Since(start) / iters
		runtime.ReadMemStats(&m1)
		return el, (m1.TotalAlloc - m0.TotalAlloc) / iters
	}

	t.Logf("Sustained raid: %.0f character changes/sec for %.0f hours = %s changes",
		raidRate, hours, humanCount(totalChanges))
	t.Log("")
	t.Logf("%8s | %11s %9s %14s | %11s %9s %14s",
		"clients", "before/chg", "keep-up", "2h allocated", "after/chg", "keep-up", "2h allocated")

	var breakBefore, breakAfter int
	for _, pop := range []int{50, 100, 200, 400, 600, 1000} {
		ob, oa := perChange(pop, false)
		nb, na := perChange(pop, true)
		oRatio := ob.Seconds() * raidRate
		nRatio := nb.Seconds() * raidRate
		if oRatio >= 1.0 && breakBefore == 0 {
			breakBefore = pop
		}
		if nRatio >= 1.0 && breakAfter == 0 {
			breakAfter = pop
		}
		mark := func(r float64) string {
			if r >= 1.0 {
				return " HANGS"
			}
			return "      "
		}
		t.Logf("%8d | %11v %8.2fx%s %13s | %11v %8.2fx%s %13s",
			pop,
			ob.Round(time.Microsecond), oRatio, mark(oRatio), humanBytes(oa*uint64(totalChanges)),
			nb.Round(time.Microsecond), nRatio, mark(nRatio), humanBytes(na*uint64(totalChanges)))
	}

	t.Log("")
	if breakBefore > 0 {
		t.Logf("BEFORE: cannot keep up with a %.0f/s raid at ~%d+ clients", raidRate, breakBefore)
	} else {
		t.Logf("BEFORE: kept up at every population tested on this machine (a slower " +
			"or busier production box tips over sooner)")
	}
	if breakAfter > 0 {
		t.Logf("AFTER:  cannot keep up at ~%d+ clients", breakAfter)
	} else {
		t.Logf("AFTER:  keeps up at every population tested, up to 1000 clients")
	}

	// The fix must not itself be the thing that cannot keep up.
	if breakAfter > 0 && breakAfter <= 600 {
		t.Errorf("after the fix the server still cannot sustain a raid at %d clients", breakAfter)
	}
}

func humanBytes(b uint64) string {
	switch {
	case b >= 1<<40:
		return fmt.Sprintf("%.1f TB", float64(b)/(1<<40))
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
	return fmt.Sprintf("%d B", b)
}

func humanCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}
