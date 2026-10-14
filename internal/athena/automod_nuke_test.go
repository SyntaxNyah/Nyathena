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

package athena

import (
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// TestAutoModNukeBanDurationPermanent pins that every spelling of "permanent"
// maps to a non-temporary ban (temporary == false). applyAutoModNuke then keeps
// the ban expiry at -1, the value IsBanned reads as "permanent" -- never 0, the
// Unix epoch, which IsBanned reads as "expired long ago".
func TestAutoModNukeBanDurationPermanent(t *testing.T) {
	prev := config
	defer func() { config = prev }()

	for _, raw := range []string{"", "permanent", "forever", "perma", "∞", "infinity", "infinite", "0"} {
		config = &settings.Config{ServerConfig: settings.ServerConfig{AutoModNukeBanDuration: raw}}
		d, temporary := autoModNukeBanDuration()
		if temporary || d != 0 {
			t.Errorf("AutoModNukeBanDuration(%q): got (%v, %v), want (0, false)", raw, d, temporary)
		}
	}
}

// TestAutoModNukeBanDurationTemporary pins that a real duration stays temporary
// and positive, so applyAutoModNuke stores a future expiry timestamp.
func TestAutoModNukeBanDurationTemporary(t *testing.T) {
	prev := config
	defer func() { config = prev }()

	config = &settings.Config{ServerConfig: settings.ServerConfig{AutoModNukeBanDuration: "30m"}}
	d, temporary := autoModNukeBanDuration()
	if !temporary || d != 30*time.Minute {
		t.Errorf(`AutoModNukeBanDuration("30m"): got (%v, %v), want (30m, true)`, d, temporary)
	}
}
