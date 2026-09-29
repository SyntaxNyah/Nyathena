/* Athena - A server for Attorney Online 2 written in Go
Copyright (C) 2022 MangosArentLiterature <mango@transmenace.dev>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>. */

package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// newUserAlertClient builds a client standing in a with the given UID and
// registers it in the global client list so UID lookups and broadcasts see it.
func newUserAlertClient(a *area.Area, uid int) (*Client, *captureConn) {
	conn := &captureConn{}
	c := &Client{conn: conn, uid: uid, ipid: "ip-useralert", area: a, char: -1, pair: ClientPairInfo{wanted_id: -1}}
	clients.AddClient(c)
	clients.RegisterUID(c)
	return c, conn
}

// TestUserAlertTargeted verifies a targeted popup reaches only the named UID.
func TestUserAlertTargeted(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	sender, senderConn := newUserAlertClient(a, 1)
	target, targetConn := newUserAlertClient(a, 2)
	bystander, bystanderConn := newUserAlertClient(a, 3)
	t.Cleanup(func() {
		clients.RemoveClient(sender)
		clients.RemoveClient(target)
		clients.RemoveClient(bystander)
	})

	cmdUserAlert(sender, []string{"2", "hello", "there"}, "")

	if got := targetConn.String(); !strings.Contains(got, "BB#hello there#%") {
		t.Errorf("target did not receive the popup; got %q", got)
	}
	if got := bystanderConn.String(); strings.Contains(got, "BB#") {
		t.Errorf("bystander received the popup: %q", got)
	}
	if got := senderConn.String(); strings.Contains(got, "BB#") {
		t.Errorf("sender received their own popup: %q", got)
	}
}

// TestUserAlertGlobal verifies the 'global' keyword reaches every client.
func TestUserAlertGlobal(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	sender, senderConn := newUserAlertClient(a, 1)
	other, otherConn := newUserAlertClient(a, 2)
	t.Cleanup(func() {
		clients.RemoveClient(sender)
		clients.RemoveClient(other)
	})

	cmdUserAlert(sender, []string{"global", "maintenance", "soon"}, "")

	for name, conn := range map[string]*captureConn{"sender": senderConn, "other": otherConn} {
		if got := conn.String(); !strings.Contains(got, "BB#maintenance soon#%") {
			t.Errorf("%s did not receive the global popup; got %q", name, got)
		}
	}
}

// TestUserAlertUnknownUID verifies a missing target reports a clear error.
func TestUserAlertUnknownUID(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	sender, senderConn := newUserAlertClient(a, 1)
	t.Cleanup(func() { clients.RemoveClient(sender) })

	cmdUserAlert(sender, []string{"99", "hi"}, "")

	if got := senderConn.String(); !strings.Contains(got, "No connected player has UID 99") {
		t.Errorf("expected not-found message, got %q", got)
	}
}

// TestUserAlertBareShowsFormattingHelp verifies running /useralert bare prints
// the formatting guide (like /roommotd does).
func TestUserAlertBareShowsFormattingHelp(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	sender, senderConn := newUserAlertClient(a, 1)
	t.Cleanup(func() { clients.RemoveClient(sender) })

	cmdUserAlert(sender, nil, "")

	if got := senderConn.String(); !strings.Contains(got, "Formatting:") {
		t.Errorf("bare /useralert did not print the formatting guide; got %q", got)
	}
}

// TestUserAlertNewlineFormatting verifies a literal \n in the message becomes a
// real newline in the popup.
func TestUserAlertNewlineFormatting(t *testing.T) {
	newTestClients(t)
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)

	sender, _ := newUserAlertClient(a, 1)
	target, targetConn := newUserAlertClient(a, 2)
	t.Cleanup(func() {
		clients.RemoveClient(sender)
		clients.RemoveClient(target)
	})

	cmdUserAlert(sender, []string{"2", "Line", "one\\nLine", "two"}, "")

	if got := targetConn.String(); !strings.Contains(got, "BB#Line one\nLine two#%") {
		t.Errorf("popup did not translate \\n to a newline; got %q", got)
	}
}
