/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /admin hide|unhide|status. */

package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestAdminHideStateToggle pins the plain state map behind /admin hide|unhide:
// case-insensitive lookup, and hide/unhide round-tripping.
func TestAdminHideStateToggle(t *testing.T) {
	const name = "TestAdminHideStateToggle-Admin"
	t.Cleanup(func() { setAdminHidden(name, false) })

	if isAdminHidden(name) {
		t.Fatal("expected not hidden before any /admin hide call")
	}
	setAdminHidden(name, true)
	if !isAdminHidden(strings.ToUpper(name)) {
		t.Fatal("isAdminHidden should be case-insensitive")
	}
	setAdminHidden(name, false)
	if isAdminHidden(name) {
		t.Fatal("expected visible again after /admin unhide")
	}
}

// TestIsAdminHiddenBlankName verifies a blank mod name (never a real admin,
// since ADMIN requires authentication) is never reported as hidden.
func TestIsAdminHiddenBlankName(t *testing.T) {
	if isAdminHidden("") {
		t.Fatal("blank mod name must never be treated as hidden")
	}
}

// TestCmdAdminCommand exercises the /admin hide|unhide|status handler end to
// end (through a real Client + captured conn) rather than just the
// underlying map, so the ModName plumbing is covered too.
func TestCmdAdminCommand(t *testing.T) {
	const name = "TestCmdAdminCommand-Admin"
	t.Cleanup(func() { setAdminHidden(name, false) })

	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, ipid: "ip-admin", char: -1,
		area: makeTestArea("Courtroom"), perms: permissions.PermissionField["ADMIN"], mod_name: name}

	cmdAdmin(client, []string{"status"}, "usage")
	if !strings.Contains(conn.String(), "currently visible") {
		t.Fatalf("expected initial status to report visible, got %q", conn.String())
	}

	cmdAdmin(client, []string{"hide"}, "usage")
	if !isAdminHidden(name) {
		t.Fatal("/admin hide did not set the hidden state")
	}

	cmdAdmin(client, []string{"status"}, "usage")
	if !strings.Contains(conn.String(), "currently hidden") {
		t.Fatalf("expected status to report hidden after /admin hide, got %q", conn.String())
	}

	cmdAdmin(client, []string{"unhide"}, "usage")
	if isAdminHidden(name) {
		t.Fatal("/admin unhide did not clear the hidden state")
	}
}

// TestCmdPlayersHidesAdminRoleFromNonAdmins is the end-to-end regression test
// for the /gas display rule: a hidden admin's UID, character slot and IPID
// must still show to a regular moderator, but the "Mod: <name>" line must be
// completely absent -- exactly like a shadow mod is hidden from a non-admin
// viewer. An admin viewer must still see the line, tagged "(hidden)" so
// fellow staff aren't kept in the dark. A non-hidden admin is included as a
// control so the test would fail if hiding leaked onto the wrong client.
func TestCmdPlayersHidesAdminRoleFromNonAdmins(t *testing.T) {
	pf := permissions.PermissionField

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	testArea := makeTestArea("Courtroom")
	t.Cleanup(setupTestAreas([]*area.Area{testArea}))

	const hiddenName = "TestCmdPlayers-HiddenAdmin"
	const visibleName = "TestCmdPlayers-VisibleAdmin"
	t.Cleanup(func() { setAdminHidden(hiddenName, false) })
	setAdminHidden(hiddenName, true)

	hiddenAdmin := &Client{conn: &captureConn{}, uid: 2, ipid: "ip-hidden-admin", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: hiddenName}
	visibleAdmin := &Client{conn: &captureConn{}, uid: 3, ipid: "ip-visible-admin", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: visibleName}
	modConn := &captureConn{}
	regularMod := &Client{conn: modConn, uid: 4, ipid: "ip-mod", char: -1,
		area: testArea, perms: pf["MUTE"] | pf["BAN_INFO"], mod_name: "RegularMod"}
	adminConn := &captureConn{}
	viewerAdmin := &Client{conn: adminConn, uid: 5, ipid: "ip-viewer-admin", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: "ViewerAdmin"}

	for _, c := range []*Client{hiddenAdmin, visibleAdmin, regularMod, viewerAdmin} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	// A regular moderator (not an admin) must not see the hidden admin's
	// role, but must still see their UID and IPID, and must still see the
	// non-hidden admin's role as a control.
	cmdPlayers(regularMod, []string{"-a"}, "")
	modOut := modConn.String()
	if strings.Contains(modOut, "Mod: "+hiddenName) {
		t.Errorf("regular mod should not see the hidden admin's role; got:\n%s", modOut)
	}
	if !strings.Contains(modOut, "IPID: ip-hidden-admin") {
		t.Errorf("regular mod should still see the hidden admin's IPID; got:\n%s", modOut)
	}
	if !strings.Contains(modOut, "[2] Spectator") {
		t.Errorf("regular mod should still see the hidden admin's UID/character; got:\n%s", modOut)
	}
	if !strings.Contains(modOut, "Mod: "+visibleName) {
		t.Errorf("regular mod should see the non-hidden admin's role (control); got:\n%s", modOut)
	}

	// An admin viewer must still see the hidden admin's role, tagged so it's
	// clear the hiding is deliberate, and the non-hidden admin's role plain.
	cmdPlayers(viewerAdmin, []string{"-a"}, "")
	adminOut := adminConn.String()
	if !strings.Contains(adminOut, "Mod: "+hiddenName+" (hidden)") {
		t.Errorf("admin viewer should see the hidden admin's role tagged \"(hidden)\"; got:\n%s", adminOut)
	}
	if !strings.Contains(adminOut, "Mod: "+visibleName+"\n") {
		t.Errorf("admin viewer should see the non-hidden admin's role untagged; got:\n%s", adminOut)
	}
}
