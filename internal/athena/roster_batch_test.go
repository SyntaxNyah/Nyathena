package athena

import (
	"encoding/json"
	"strings"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"
)

// A FantaCode client can receive several frames concatenated in one queue
// entry; a JSON/WebSocket client must receive one JSON value per message.
func TestBroadcastToAllBatchFantaVsJSON(t *testing.T) {
	withRaidConfig(t)
	prev := clients
	t.Cleanup(func() { clients = prev })

	fanta := &Client{uid: 1, sendCh: make(chan []byte, 8)}
	jsonClient := &Client{uid: 2, sendCh: make(chan []byte, 8)}
	jsonClient.jsonMode.Store(true)

	cl := &ClientList{
		list:       map[*Client]struct{}{fanta: {}, jsonClient: {}},
		uidIndex:   map[int]*Client{1: fanta, 2: jsonClient},
		ipidCounts: map[string]int{},
	}
	clients = cl

	broadcastToAllBatch([]aolib.Outgoing{
		&aolib.PR{ID: 1, Type: aolib.PlayerListUpdateAdd},
		&aolib.PU{ID: 1, Type: aolib.PlayerDataTypeCharName, Data: "Test Character"},
	})

	select {
	case batch := <-fanta.sendCh:
		if got := strings.Count(string(batch), "%"); got != 2 {
			t.Fatalf("Fanta batch contained %d frames, want 2: %q", got, string(batch))
		}
	default:
		t.Fatal("Fanta client received no batch")
	}

	for i := 0; i < 2; i++ {
		select {
		case frame := <-jsonClient.sendCh:
			if !json.Valid(frame) {
				t.Fatalf("JSON frame %d is not a complete value: %q", i, string(frame))
			}
		default:
			t.Fatalf("JSON client received fewer than 2 frames")
		}
	}
	select {
	case frame := <-jsonClient.sendCh:
		t.Fatalf("JSON client received an unexpected extra frame: %q", string(frame))
	default:
	}
}
