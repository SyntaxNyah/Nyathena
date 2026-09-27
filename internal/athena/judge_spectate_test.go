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
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestCanJudSpectateMode covers the /spectate half of the judge-action gate.
// /spectate restricts IC speech (CanSpeakIC), but CanJud used to gate only on
// the /lock -s invite list and a missing character, so a watch-only room still
// let every player holding a character spam HP / WT-CE / testimony-title
// packets. Judge actions must follow the same spectate rules as IC speech:
// blocked for ordinary players, allowed for CMs, spectate-invited players and
// BYPASS_LOCK holders.
func TestCanJudSpectateMode(t *testing.T) {
	tests := []struct {
		name      string
		charID    int
		spectate  bool
		cm        bool
		invited   bool
		bypass    bool
		wantAllow bool
	}{
		{"spectate off, has character", 0, false, false, false, false, true},
		{"spectate off, no character", -1, false, false, false, false, false},
		{"spectate on, ordinary player", 0, true, false, false, false, false},
		{"spectate on, CM", 0, true, true, false, false, true},
		{"spectate on, invited", 0, true, false, true, false, true},
		{"spectate on, BYPASS_LOCK", 0, true, false, false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := makeTestArea("Courtroom")
			a.SetSpectateMode(tt.spectate)
			if tt.cm {
				a.AddCM(1)
			}
			if tt.invited {
				a.AddSpectateInvited(1)
			}

			perms := permissions.PermissionField["NONE"]
			if tt.bypass {
				perms = permissions.PermissionField["BYPASS_LOCK"]
			}

			client := &Client{uid: 1, char: tt.charID, area: a, perms: perms}

			if got := client.CanJud(); got != tt.wantAllow {
				t.Errorf("CanJud() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}
