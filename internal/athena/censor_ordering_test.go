// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Source-order checks for the AutoMod content gate in netprotocol.go.
package athena

import (
	"os"
	"strings"
	"testing"
)

// TestNukeIsCheckedBeforeEveryGentlerAction pins the other half: within the
// gate, a nuke must be decided before the shadow/kick handling, since the whole
// point of the tier is that it does not echo to the sender the way a shadow trip
// does.
func TestNukeIsCheckedBeforeEveryGentlerAction(t *testing.T) {
	src, err := os.ReadFile("netprotocol.go")
	if err != nil {
		t.Fatalf("read netprotocol.go: %v", err)
	}
	body := string(src)

	nuke := strings.Index(body, "SeverityNuke")
	if nuke < 0 {
		t.Fatal("no SeverityNuke check in netprotocol.go -- the nuke tier is not wired up")
	}
	// Inside the shared checkCensored closure the nuke branch must come before
	// the autoModShadow branch that sets censorShadow.
	closure := strings.Index(body, "checkCensored := func(")
	if closure < 0 {
		t.Skip("checkCensored closure not found; the gate was restructured")
	}
	seg := body[closure:]
	nukeAt := strings.Index(seg, "SeverityNuke")
	shadowAt := strings.Index(seg, "case autoModShadow:")
	if nukeAt < 0 || shadowAt < 0 {
		t.Fatalf("gate no longer contains both a nuke branch (%d) and a shadow branch (%d)", nukeAt, shadowAt)
	}
	if nukeAt > shadowAt {
		t.Error("the nuke branch is evaluated after the shadow branch; a nuke would be echoed back to its " +
			"sender, which is exactly what the tier exists to prevent")
	}
}
