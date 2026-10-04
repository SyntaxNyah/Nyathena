package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// areaKickTestSetup builds a two-area world (lobby = default area 0, courtroom)
// and two clients — a CM and their target — both in the courtroom. It returns
// the capture connections so callers can assert on what each client received.
func areaKickTestSetup(t *testing.T) (cm, target *Client, cmConn, targetConn *captureConn, lobby, courtroom *area.Area) {
	t.Helper()
	newTestClients(t)

	origAreas := areas
	t.Cleanup(func() { areas = origAreas })
	lobby = area.NewArea(area.AreaData{Name: "Lobby"}, 5, 10, area.EviAny)
	courtroom = area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	areas = []*area.Area{lobby, courtroom}

	cmConn = &captureConn{}
	targetConn = &captureConn{}
	cm = &Client{conn: cmConn, uid: 1, ipid: "ip-cm", area: courtroom, char: -1}
	target = &Client{conn: targetConn, uid: 2, ipid: "ip-target", area: courtroom, char: -1}
	for _, c := range []*Client{cm, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	return cm, target, cmConn, targetConn, lobby, courtroom
}

// TestAreaKickWithReasonBroadcastsCTAndPopup verifies that /kickarea with a
// reason moves the target, pops a BB (MOTD-style) up on the kicked player with
// the reason, and announces the kick to the area as an OOC CT line.
func TestAreaKickWithReasonBroadcastsCTAndPopup(t *testing.T) {
	cm, target, cmConn, targetConn, lobby, _ := areaKickTestSetup(t)

	cmdAreaKick(cm, []string{"2", "Being", "too", "loud"}, "")

	if target.Area() != lobby {
		t.Fatalf("target was not moved to the default area")
	}
	if out := targetConn.String(); !strings.Contains(out, "BB#") || !strings.Contains(out, "Being too loud") {
		t.Fatalf("target should receive a BB popup carrying the reason; got %q", out)
	}
	if out := cmConn.String(); !strings.Contains(out, "CT#") || !strings.Contains(out, "Being too loud") {
		t.Fatalf("area should receive a CT OOC announcement carrying the reason; got %q", out)
	}
}

// TestAreaKickWithoutReasonStaysSilent verifies that /kickarea with no reason
// keeps the old behavior: the target is moved with the plain per-player notice
// and there is no BB popup and no area-wide OOC announcement.
func TestAreaKickWithoutReasonStaysSilent(t *testing.T) {
	cm, target, cmConn, targetConn, lobby, _ := areaKickTestSetup(t)

	cmdAreaKick(cm, []string{"2"}, "")

	if target.Area() != lobby {
		t.Fatalf("target was not moved to the default area")
	}
	if out := targetConn.String(); strings.Contains(out, "BB#") {
		t.Fatalf("target should NOT receive a BB popup without a reason; got %q", out)
	}
	if out := cmConn.String(); strings.Contains(out, " from the area: ") {
		t.Fatalf("area should NOT receive an announcement without a reason; got %q", out)
	}
}
