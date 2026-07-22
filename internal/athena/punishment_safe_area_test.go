/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for punishment-safe areas (antipunish).
   Verifies that moderator-issued punishment-system commands are refused
   against a target standing in an antipunish area, while real moderation
   enforcement (/ban, /mute, /kick) is unaffected, and that /punishmentsafe
   requires ADMIN. */

package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestCmdPunishmentRefusesInSafeArea verifies the shared cmdPunishment
// applicator (used by the great majority of punishment commands) skips a
// target standing in a punishment-safe area, both for the UID-list form and
// the "global" form, while still punishing targets outside such areas.
func TestCmdPunishmentRefusesInSafeArea(t *testing.T) {
	defer setupAreaMuteTestDB(t)()
	newTestClients(t)

	pf := permissions.PermissionField
	safeArea := makeTestArea("Safe Haven")
	safeArea.SetPunishmentSafe(true)
	normalArea := makeTestArea("Courtroom")

	modConn := &captureConn{}
	mod := &Client{conn: modConn, uid: 1, ipid: "ip-mod", char: -1, area: normalArea, perms: pf["MUTE"], mod_name: "Mod"}

	protectedConn := &captureConn{}
	protected := &Client{conn: protectedConn, uid: 2, ipid: "ip-protected", char: -1, area: safeArea}

	unprotectedConn := &captureConn{}
	unprotected := &Client{conn: unprotectedConn, uid: 3, ipid: "ip-unprotected", char: -1, area: normalArea}

	for _, c := range []*Client{mod, protected, unprotected} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdPunishment(mod, []string{"2,3"}, "usage", PunishmentTsundere)

	if protected.HasPunishment(PunishmentTsundere) {
		t.Errorf("target in a punishment-safe area should not have been punished")
	}
	if !unprotected.HasPunishment(PunishmentTsundere) {
		t.Errorf("target outside a punishment-safe area should have been punished")
	}
	if strings.Contains(protectedConn.String(), "tsundere") {
		t.Errorf("protected target should not receive a punishment notification; got %q", protectedConn.String())
	}
	if !strings.Contains(modConn.String(), "punishment-safe") {
		t.Errorf("issuing mod should be told a target was shielded by a punishment-safe area; got %q", modConn.String())
	}
}

// TestCmdPunishmentGlobalSkipsSafeArea verifies the "global" form of
// cmdPunishment refuses to punish anyone when the issuer's own area is
// punishment-safe.
func TestCmdPunishmentGlobalSkipsSafeArea(t *testing.T) {
	defer setupAreaMuteTestDB(t)()
	newTestClients(t)

	pf := permissions.PermissionField
	safeArea := makeTestArea("Safe Haven")
	safeArea.SetPunishmentSafe(true)

	modConn := &captureConn{}
	mod := &Client{conn: modConn, uid: 1, ipid: "ip-mod", char: -1, area: safeArea, perms: pf["MUTE"], mod_name: "Mod"}
	targetConn := &captureConn{}
	target := &Client{conn: targetConn, uid: 2, ipid: "ip-target", char: -1, area: safeArea}

	for _, c := range []*Client{mod, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdPunishment(mod, []string{"global"}, "usage", PunishmentUwu)

	if target.HasPunishment(PunishmentUwu) {
		t.Errorf("no player in a punishment-safe area should be punished via /<effect> global")
	}
}

// TestCmdCharCurseRefusesInSafeArea verifies /charcurse — one of the
// alertPunishmentIssued call sites outside the shared cmdPunishment
// applicator — is also gated.
func TestCmdCharCurseRefusesInSafeArea(t *testing.T) {
	newTestClients(t)

	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})

	pf := permissions.PermissionField
	safeArea := area.NewArea(area.AreaData{Name: "Safe Haven"}, 5, 10, area.EviCMs)
	safeArea.SetPunishmentSafe(true)

	modConn := &captureConn{}
	mod := &Client{conn: modConn, uid: 1, ipid: "ip-mod", char: -1, area: safeArea, perms: pf["MUTE"], mod_name: "Mod"}
	targetConn := &captureConn{}
	target := &Client{conn: targetConn, uid: 2, ipid: "ip-target", char: 0, area: safeArea}

	for _, c := range []*Client{mod, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdCharCurse(mod, []string{"2", "Miles", "Edgeworth"}, "usage")

	if target.CharID() != 0 {
		t.Errorf("char-curse should have been refused for a target in a punishment-safe area, but character changed to %v", target.CharID())
	}
	if !strings.Contains(modConn.String(), "punishment-safe") {
		t.Errorf("mod should be told the target is shielded by a punishment-safe area; got %q", modConn.String())
	}
}

// TestCmdMuteWorksInSafeArea verifies real moderation enforcement (/mute,
// standing in for /ban and /kick which share the same getUidList/SendSync
// shape) is entirely unaffected by punishment-safe areas.
func TestCmdMuteWorksInSafeArea(t *testing.T) {
	defer setupAreaMuteTestDB(t)()
	newTestClients(t)

	pf := permissions.PermissionField
	safeArea := makeTestArea("Safe Haven")
	safeArea.SetPunishmentSafe(true)

	mod := &Client{conn: &captureConn{}, uid: 1, ipid: "ip-mod", char: -1, area: safeArea, perms: pf["MUTE"], mod_name: "Mod"}
	target := &Client{conn: &captureConn{}, uid: 2, ipid: "ip-target", char: -1, area: safeArea}

	for _, c := range []*Client{mod, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdMute(mod, []string{"2"}, "usage")

	if target.Muted() != ICMuted {
		t.Errorf("/mute should still work against a target in a punishment-safe area; got %v", target.Muted())
	}
}

// TestCmdPunishmentSafeAreaToggle exercises the /punishmentsafe <true|false>
// runtime toggle end to end.
func TestCmdPunishmentSafeAreaToggle(t *testing.T) {
	conn := &captureConn{}
	a := makeTestArea("Courtroom")
	client := &Client{conn: conn, uid: 1, ipid: "ip-toggle", char: -1, area: a}

	if a.PunishmentSafe() {
		t.Fatal("punishment-safe mode should default to disabled")
	}

	cmdPunishmentSafeArea(client, []string{"true"}, "usage")
	if !a.PunishmentSafe() {
		t.Fatal("/punishmentsafe true did not enable punishment-safe mode")
	}

	cmdPunishmentSafeArea(client, []string{"false"}, "usage")
	if a.PunishmentSafe() {
		t.Fatal("/punishmentsafe false did not disable punishment-safe mode")
	}
}

// TestPunishmentSafeAreaCommandRequiresAdmin pins that /punishmentsafe is
// ADMIN-gated: it's a policy control meant to override what mods and shadow
// mods can do, so a regular moderator must not be able to flip it off
// themselves.
func TestPunishmentSafeAreaCommandRequiresAdmin(t *testing.T) {
	initCommands()
	cmd, ok := Commands["punishmentsafe"]
	if !ok {
		t.Fatal("command \"punishmentsafe\" not registered")
	}
	if cmd.reqPerms != permissions.PermissionField["ADMIN"] {
		t.Errorf("punishmentsafe reqPerms = %v, want ADMIN (%v)", cmd.reqPerms, permissions.PermissionField["ADMIN"])
	}
}
