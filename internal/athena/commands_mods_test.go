package athena

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// modsTestSetup builds a two-area world (Lobby, Courtroom) and a plain player
// who is used as the command's caller. Tests add moderators — and, where they
// need one, an admin viewer — through addModsTestClient.
func modsTestSetup(t *testing.T) (viewer *Client, viewerConn *captureConn, lobby, courtroom *area.Area) {
	t.Helper()
	newTestClients(t)

	origAreas := areas
	t.Cleanup(func() { areas = origAreas })
	lobby = area.NewArea(area.AreaData{Name: "Lobby"}, 5, 10, area.EviAny)
	courtroom = area.NewArea(area.AreaData{Name: "Courtroom"}, 5, 10, area.EviAny)
	areas = []*area.Area{lobby, courtroom}

	viewerConn = &captureConn{}
	viewer = &Client{conn: viewerConn, uid: 1, ipid: "ip-viewer", area: lobby, char: -1}
	clients.AddClient(viewer)
	clients.RegisterUID(viewer)
	return viewer, viewerConn, lobby, courtroom
}

// addModsTestClient registers one connected client with the given moderator
// name and permission bitfield so /mods can see it.
func addModsTestClient(t *testing.T, uid int, modName string, perms uint64, a *area.Area) *Client {
	t.Helper()
	c := &Client{
		conn:     &captureConn{},
		uid:      uid,
		ipid:     fmt.Sprintf("ip-%d", uid),
		area:     a,
		char:     -1,
		mod_name: modName,
		perms:    perms,
	}
	clients.AddClient(c)
	clients.RegisterUID(c)
	return c
}

// TestModsGroupsModeratorsByArea verifies /mods lists every moderator under
// their area in configured area order and UID order, and that non-staff
// connections (players, CMs, DJs) are not listed as moderators.
func TestModsGroupsModeratorsByArea(t *testing.T) {
	viewer, conn, lobby, courtroom := modsTestSetup(t)

	addModsTestClient(t, 12, "Miley", permissions.PermissionField["MUTE"], lobby)
	addModsTestClient(t, 20, "patapon", permissions.PermissionField["BAN"], courtroom)
	addModsTestClient(t, 5, "Hun", permissions.PermissionField["KICK"], courtroom)
	// Not staff for /mods' purposes: a regular player, a CM-only role and a
	// DJ-only role all lack moderator permissions.
	addModsTestClient(t, 9, "", permissions.PermissionField["NONE"], lobby)
	addModsTestClient(t, 8, "", permissions.PermissionField["CM"], lobby)
	addModsTestClient(t, 7, "", permissions.PermissionField["DJ"], courtroom)

	cmdMods(viewer, nil, "")

	out := conn.String()
	for _, want := range []string{
		"Moderators",
		"Lobby:\n  [12] Miley\n",
		"Courtroom:\n  [5] Hun\n  [20] patapon\n",
		"3 moderators online in 2 areas.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q; got %q", want, out)
		}
	}
	for _, unwanted := range []string{"[9]", "[8]", "[7]"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("non-moderator %q must not be listed; got %q", unwanted, out)
		}
	}
	if lobbyIdx, courtIdx := strings.Index(out, "Lobby:"), strings.Index(out, "Courtroom:"); lobbyIdx > courtIdx {
		t.Fatalf("areas should follow the configured order (Lobby before Courtroom); got %q", out)
	}
	if first, second := strings.Index(out, "[5] Hun"), strings.Index(out, "[20] patapon"); first > second {
		t.Fatalf("moderators in one area should be sorted by UID ascending; got %q", out)
	}
}

// TestModsHidesShadowAndHiddenFromNonAdmins verifies the privacy rules /players
// already enforces apply to /mods: shadow mods, /admin hide'd admins and
// shadow-disconnected ghosts are invisible to non-admin viewers, while an
// admin viewer still sees all of them, tagged where relevant.
func TestModsHidesShadowAndHiddenFromNonAdmins(t *testing.T) {
	viewer, viewerConn, lobby, courtroom := modsTestSetup(t)

	addModsTestClient(t, 30, "sneaky", permissions.PermissionField["MUTE"]|permissions.PermissionField["SHADOW"], lobby)
	addModsTestClient(t, 31, "HiddenRoot", permissions.PermissionField["ADMIN"], courtroom)
	addModsTestClient(t, 32, "Root", permissions.PermissionField["ADMIN"], courtroom)
	ghost := addModsTestClient(t, 33, "Ghost", permissions.PermissionField["MUTE"], lobby)
	ghost.SetHidden(true)

	setAdminHidden("HiddenRoot", true)
	t.Cleanup(func() { setAdminHidden("HiddenRoot", false) })

	cmdMods(viewer, nil, "")

	out := viewerConn.String()
	if !strings.Contains(out, "[32] Root") {
		t.Fatalf("visible moderator should be listed; got %q", out)
	}
	for _, unwanted := range []string{"[30]", "sneaky", "[31]", "HiddenRoot", "[33]", "Ghost"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("hidden staff %q must not be listed to a non-admin; got %q", unwanted, out)
		}
	}
	for _, marker := range []string{"(shadow)", "(hidden)", "(admin)"} {
		if strings.Contains(out, marker) {
			t.Fatalf("role marker %q must not leak to a non-admin; got %q", marker, out)
		}
	}
	if !strings.Contains(out, "1 moderator online in 1 area.") {
		t.Fatalf("expected a single visible moderator in a single area; got %q", out)
	}

	// Now run the same command as an admin. The admin viewer is itself a
	// moderator in the Lobby, so it joins the listing.
	admin := addModsTestClient(t, 2, "Root2", permissions.PermissionField["ADMIN"], lobby)
	adminConn := admin.conn.(*captureConn)

	cmdMods(admin, nil, "")

	out = adminConn.String()
	for _, want := range []string{
		"[2] Root2 (admin)",
		"[30] sneaky (shadow)",
		"[31] HiddenRoot (hidden)",
		"[32] Root (admin)",
		"[33] Ghost",
		"5 moderators online in 2 areas.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("admin output should contain %q; got %q", want, out)
		}
	}
}

// TestModsEmptyState verifies the friendly message when the only connections
// online are not moderators.
func TestModsEmptyState(t *testing.T) {
	viewer, conn, lobby, _ := modsTestSetup(t)

	addModsTestClient(t, 9, "", permissions.PermissionField["NONE"], lobby)

	cmdMods(viewer, nil, "")

	out := conn.String()
	if !strings.Contains(out, "No moderators are currently online.") {
		t.Fatalf("expected the empty-state message; got %q", out)
	}
	if strings.Contains(out, "online in") {
		t.Fatalf("empty state should not print the summary line; got %q", out)
	}
}

// TestModsDisplayNameFallback covers a moderator connection whose account name
// was never populated: /mods falls back to the OOC name rather than printing a
// blank entry.
func TestModsDisplayNameFallback(t *testing.T) {
	viewer, conn, lobby, _ := modsTestSetup(t)

	c := addModsTestClient(t, 40, "", permissions.PermissionField["MUTE"], lobby)
	c.SetOocName("OOCOnly")

	cmdMods(viewer, nil, "")

	if out := conn.String(); !strings.Contains(out, "[40] OOCOnly") {
		t.Fatalf("expected OOC-name fallback; got %q", out)
	}
}

// TestModsCommandRegistration pins the public contract of the registry entry:
// everyone can run it, and it lives in the general help category.
func TestModsCommandRegistration(t *testing.T) {
	initCommands()
	cmd, ok := Commands["mods"]
	if !ok {
		t.Fatal("mods command is not registered")
	}
	if cmd.handler == nil {
		t.Error("mods command has a nil handler")
	}
	if cmd.reqPerms != permissions.PermissionField["NONE"] {
		t.Error("mods must stay public (reqPerms NONE)")
	}
	if cmd.category != "general" {
		t.Errorf("mods should live in the general help category, got %q", cmd.category)
	}
	if cmd.minArgs != 0 {
		t.Errorf("mods takes no arguments, got minArgs=%d", cmd.minArgs)
	}
}
