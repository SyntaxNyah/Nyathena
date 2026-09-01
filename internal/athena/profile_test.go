/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /profile's account-privacy behaviour.

   /profile <uid> used to resolve the target's linked account purely from
   their IPID (db.GetUsernameByIPID), regardless of whether the target had
   actually authenticated this connection. That let anyone out a stranger's
   real account off a stale IPID link left over from a past session, with no
   action from the target required. The fix: viewing someone ELSE's profile
   only resolves the account if they are currently authenticated; viewing
   your own profile is unaffected either way. */

package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// TestProfileHidesAccountForUnauthenticatedOtherPlayer is the core
// regression test for the privacy fix described above.
func TestProfileHidesAccountForUnauthenticatedOtherPlayer(t *testing.T) {
	defer setupAreaMuteTestDB(t)()
	newTestClients(t)

	if err := db.CreateUser("secretaccount", []byte("password123"), 0); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := db.LinkIPIDToUser("secretaccount", "ip-target"); err != nil {
		t.Fatalf("LinkIPIDToUser: %v", err)
	}

	viewer := &Client{conn: &captureConn{}, uid: 1, ipid: "ip-viewer", char: -1, jailAreaID: -1}
	target := &Client{conn: &captureConn{}, uid: 2, ipid: "ip-target", char: -1, jailAreaID: -1}
	for _, c := range []*Client{viewer, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	// The target hasn't authenticated this connection: their account must
	// stay hidden from another player, even though the DB still has their
	// IPID linked from some past session.
	viewer.conn = &captureConn{}
	cmdProfile(viewer, []string{"2"}, "")
	out := viewer.conn.(*captureConn).String()
	if strings.Contains(out, "secretaccount") {
		t.Fatalf("profile leaked the target's account name while they were unauthenticated:\n%s", out)
	}
	if !strings.Contains(out, "(guest)") {
		t.Fatalf("expected an unauthenticated target to show as (guest), got:\n%s", out)
	}

	// Once the target actually authenticates this connection (/login or
	// /register), viewing them shows the account.
	target.SetAuthenticated(true)
	viewer.conn = &captureConn{}
	cmdProfile(viewer, []string{"2"}, "")
	out = viewer.conn.(*captureConn).String()
	if !strings.Contains(out, "secretaccount") {
		t.Fatalf("expected the account name to show once the target authenticated, got:\n%s", out)
	}

	// Self-view always resolves your own linked account, authenticated or
	// not — it's your own information, not something being outed to a peer.
	target.SetAuthenticated(false)
	target.conn = &captureConn{}
	cmdProfile(target, nil, "")
	out = target.conn.(*captureConn).String()
	if !strings.Contains(out, "secretaccount") {
		t.Fatalf("expected self-view to show the caller's own linked account even when unauthenticated, got:\n%s", out)
	}
}
