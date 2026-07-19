/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for the punishment audit alert
   (alertPunishmentIssued) and the /punishaudit toggle command. */

package athena

import (
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Every test below builds struct-literal Clients backed by a captureConn.
// SendPacket detects the nil sendCh and falls back to the synchronous write
// path (see the comment on Client.SendPacket), so no writer goroutine or
// net.Pipe is needed to observe what gets sent.

func TestAlertPunishmentIssuedNotifiesAdminsOnly(t *testing.T) {
	pf := permissions.PermissionField
	testArea := makeTestArea("Courtroom")

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	issuer := &Client{conn: &captureConn{}, uid: 1, ipid: "ip-issuer", char: -1,
		area: testArea, perms: pf["MUTE"], mod_name: "RegularMod"}
	adminConn := &captureConn{}
	admin := &Client{conn: adminConn, uid: 2, ipid: "ip-admin", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: "TheAdmin"}
	modConn := &captureConn{}
	otherMod := &Client{conn: modConn, uid: 3, ipid: "ip-othermod", char: -1,
		area: testArea, perms: pf["MUTE"], mod_name: "OtherMod"}
	playerConn := &captureConn{}
	player := &Client{conn: playerConn, uid: 4, ipid: "ip-player", char: -1, area: testArea}

	for _, c := range []*Client{issuer, admin, otherMod, player} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	alertPunishmentIssued(issuer, "tsundere", "5", 1, 10*time.Minute, "test reason", false)

	adminOut := adminConn.String()
	if !strings.Contains(adminOut, "RegularMod") {
		t.Errorf("admin should be alerted with the issuer's name; got %q", adminOut)
	}
	if !strings.Contains(adminOut, "tsundere") {
		t.Errorf("admin alert should name the punishment; got %q", adminOut)
	}
	if strings.Contains(modConn.String(), "tsundere") {
		t.Errorf("a regular moderator (non-admin) should never receive a punishment audit alert; got %q", modConn.String())
	}
	if strings.Contains(playerConn.String(), "tsundere") {
		t.Errorf("a plain player should never receive a punishment audit alert; got %q", playerConn.String())
	}
}

// TestAlertPunishmentIssuedSkipsIssuerAndNonModerators verifies the issuer
// never gets their own alert (they already got a SendServerMessage
// confirmation from the command itself) and that a non-moderator "issuer"
// (which should never happen given every punishment command requires at
// least MUTE, but is cheap to guard) is a silent no-op.
func TestAlertPunishmentIssuedSkipsIssuerAndNonModerators(t *testing.T) {
	pf := permissions.PermissionField
	testArea := makeTestArea("Courtroom")

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	adminIssuerConn := &captureConn{}
	adminIssuer := &Client{conn: adminIssuerConn, uid: 1, ipid: "ip-admin-issuer", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: "AdminIssuer"}
	otherAdminConn := &captureConn{}
	otherAdmin := &Client{conn: otherAdminConn, uid: 2, ipid: "ip-other-admin", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: "OtherAdmin"}

	clients.AddClient(adminIssuer)
	clients.RegisterUID(adminIssuer)
	clients.AddClient(otherAdmin)
	clients.RegisterUID(otherAdmin)

	alertPunishmentIssued(adminIssuer, "yandere", "9", 1, 0, "", false)

	if strings.Contains(adminIssuerConn.String(), "yandere") {
		t.Errorf("issuer should not receive their own punishment audit alert; got %q", adminIssuerConn.String())
	}
	if !strings.Contains(otherAdminConn.String(), "yandere") {
		t.Errorf("a different admin should still receive the alert; got %q", otherAdminConn.String())
	}

	// A plain, non-moderator "issuer" must never fire an alert at all.
	newTestClients(t)
	nonModConn := &captureConn{}
	nonMod := &Client{conn: nonModConn, uid: 5, ipid: "ip-nonmod", char: -1, area: testArea}
	adminConn2 := &captureConn{}
	admin2 := &Client{conn: adminConn2, uid: 6, ipid: "ip-admin2", char: -1, area: testArea, perms: pf["ADMIN"]}
	clients.AddClient(nonMod)
	clients.AddClient(admin2)
	alertPunishmentIssued(nonMod, "yandere", "9", 1, 0, "", false)
	if adminConn2.String() != "" {
		t.Errorf("a non-moderator issuer must never trigger a punishment audit alert; got %q", adminConn2.String())
	}
}

// TestAlertPunishmentIssuedRespectsToggle verifies /punishaudit off actually
// silences the alert for that admin's session.
func TestAlertPunishmentIssuedRespectsToggle(t *testing.T) {
	pf := permissions.PermissionField
	testArea := makeTestArea("Courtroom")
	newTestClients(t)

	issuer := &Client{conn: &captureConn{}, uid: 1, ipid: "ip-issuer2", char: -1,
		area: testArea, perms: pf["MUTE"], mod_name: "RegularMod2"}
	adminConn := &captureConn{}
	admin := &Client{conn: adminConn, uid: 2, ipid: "ip-admin3", char: -1,
		area: testArea, perms: pf["ADMIN"], mod_name: "QuietAdmin"}
	admin.SetPunishmentAuditDisabled(true)

	clients.AddClient(issuer)
	clients.AddClient(admin)

	alertPunishmentIssued(issuer, "kuudere", "3", 1, 0, "", false)

	if adminConn.String() != "" {
		t.Errorf("admin with /punishaudit off should receive nothing; got %q", adminConn.String())
	}
}

// TestCmdPunishAuditToggle exercises the /punishaudit on|off|<no args>
// handler end to end.
func TestCmdPunishAuditToggle(t *testing.T) {
	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, ipid: "ip-toggle", char: -1, area: makeTestArea("Courtroom")}

	if client.PunishmentAuditDisabled() {
		t.Fatal("punishment audit alerts should default to enabled")
	}

	cmdPunishAudit(client, []string{"off"}, "usage")
	if !client.PunishmentAuditDisabled() {
		t.Fatal("/punishaudit off did not disable alerts")
	}
	if !strings.Contains(conn.String(), "now OFF") {
		t.Errorf("expected confirmation message, got %q", conn.String())
	}

	cmdPunishAudit(client, []string{"on"}, "usage")
	if client.PunishmentAuditDisabled() {
		t.Fatal("/punishaudit on did not re-enable alerts")
	}
}

// TestNewAdminCommandsRequireAdminPermission pins that the three new
// commands are all ADMIN-gated, matching how they're documented and how the
// audit alert itself is only ever delivered to ADMIN holders.
func TestNewAdminCommandsRequireAdminPermission(t *testing.T) {
	initCommands()
	admin := permissions.PermissionField["ADMIN"]
	for _, name := range []string{"admin", "terminal", "punishaudit"} {
		cmd, ok := Commands[name]
		if !ok {
			t.Fatalf("command %q not registered", name)
		}
		if cmd.reqPerms != admin {
			t.Errorf("%s reqPerms = %v, want ADMIN (%v)", name, cmd.reqPerms, admin)
		}
	}
}
