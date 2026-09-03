package athena

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// realCharCount is the character-list size on the server the hang capture came
// from (its SI packet reported 4465). CharsCheck carries one entry per
// character, which is what makes this one packet worth special-casing.
const realCharCount = 4465

func benchClients(n int) (*ClientList, *area.Area) {
	a := area.NewArea(area.AreaData{Name: "Courtroom"}, realCharCount, 10, area.EviAny)
	cl := &ClientList{
		list:       make(map[*Client]struct{}, n),
		uidIndex:   make(map[int]*Client, n),
		ipidCounts: make(map[string]int, n),
	}
	for i := 0; i < n; i++ {
		c := &Client{
			char:   -1,
			area:   a,
			ipid:   fmt.Sprintf("bench-%d", i),
			sendCh: make(chan []byte, 4096),
		}
		cl.list[c] = struct{}{}
	}
	return cl, a
}

func charsCheck(a *area.Area) *packet.CharsCheck {
	return &packet.CharsCheck{Entries: a.Taken()}
}

// The bytes each recipient receives must be byte-identical to what the
// per-recipient path produced. Serializing once is only safe if it is also
// exactly equivalent.
func TestPrebuiltMatchesPerRecipientBytes(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, a := benchClients(3)
	clients = cl

	broadcastToAreaOnce(a, charsCheck(a))

	var got [][]byte
	for c := range cl.list {
		select {
		case b := <-c.sendCh:
			got = append(got, b)
		default:
			t.Fatal("a client in the area received nothing")
		}
	}
	if len(got) != 3 {
		t.Fatalf("got %d packets, want 3", len(got))
	}

	// Rebuild the same packet the way SendPacket would, and compare.
	p := charsCheck(a)
	var want bytes.Buffer
	want.WriteString(p.Header())
	for _, arg := range p.Args() {
		want.WriteByte('#')
		want.WriteString(arg)
	}
	want.WriteString("#%")
	for i, b := range got {
		if !bytes.Equal(b, want.Bytes()) {
			t.Fatalf("recipient %d bytes differ from the per-recipient encoding", i)
		}
	}
	if !strings.HasPrefix(string(got[0]), "CharsCheck#") || !strings.HasSuffix(string(got[0]), "#%") {
		t.Error("wire framing lost")
	}
}

// Every recipient must share ONE backing array -- that is the entire point.
func TestPrebuiltSharesOneBuffer(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, a := benchClients(8)
	clients = cl

	broadcastToAreaOnce(a, charsCheck(a))

	var first []byte
	for c := range cl.list {
		b := <-c.sendCh
		if first == nil {
			first = b
			continue
		}
		if &b[0] != &first[0] {
			t.Fatal("recipients got separate allocations; the payload is still " +
				"being rebuilt per client")
		}
	}
}

// Clients outside the area must not receive it.
func TestPrebuiltRespectsAreaScoping(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, a := benchClients(4)
	other := area.NewArea(area.AreaData{Name: "Basement"}, realCharCount, 10, area.EviAny)
	outsider := &Client{char: -1, area: other, ipid: "outsider", sendCh: make(chan []byte, 8)}
	cl.list[outsider] = struct{}{}
	clients = cl

	broadcastToAreaOnce(a, charsCheck(a))

	select {
	case <-outsider.sendCh:
		t.Error("a client in another area received the broadcast")
	default:
	}
}

// A full queue must drop, never block the broadcaster -- same contract as
// SendPacket, and the reason a stuck raider connection cannot stall a fan-out.
func TestPrebuiltDropsRatherThanBlocking(t *testing.T) {
	stuck := &Client{char: -1, ipid: "stuck", sendCh: make(chan []byte)} // unbuffered, nobody reading
	done := make(chan struct{})
	go func() {
		stuck.sendPrebuilt([]byte("CharsCheck#0#%"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sendPrebuilt blocked on a client that is not reading; a single " +
			"stuck connection would stall the whole fan-out")
	}
}

func BenchmarkCharsCheckPerRecipient(b *testing.B) {
	prev := clients
	b.Cleanup(func() { clients = prev })
	cl, a := benchClients(600)
	clients = cl
	drain(cl)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcastToArea(a, charsCheck(a))
	}
}

func BenchmarkCharsCheckBuildOnce(b *testing.B) {
	prev := clients
	b.Cleanup(func() { clients = prev })
	cl, a := benchClients(600)
	clients = cl
	drain(cl)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcastToAreaOnce(a, charsCheck(a))
	}
}

// drain keeps every send queue empty so the benchmark measures the broadcast
// itself rather than queue-full drops.
func drain(cl *ClientList) {
	for c := range cl.list {
		ch := c.sendCh
		go func() {
			for range ch {
			}
		}()
	}
}
