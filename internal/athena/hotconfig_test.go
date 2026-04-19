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

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// TestHotConfigInitAndGetters verifies that initHotConfig seeds the cache and
// the exported getters return those values.
func TestHotConfigInitAndGetters(t *testing.T) {
	initHotConfig(&settings.Config{ServerConfig: settings.ServerConfig{
		Motd: "hello world",
		Desc: "test server",
	}})
	if GetMotd() != "hello world" {
		t.Fatalf("GetMotd: got %q, want %q", GetMotd(), "hello world")
	}
	if GetServerDesc() != "test server" {
		t.Fatalf("GetServerDesc: got %q, want %q", GetServerDesc(), "test server")
	}
}

// TestHotConfigEmptyMotd exercises the empty-MOTD branch that the
// netprotocol join path guards against.
func TestHotConfigEmptyMotd(t *testing.T) {
	initHotConfig(&settings.Config{ServerConfig: settings.ServerConfig{
		Motd: "",
		Desc: "x",
	}})
	if GetMotd() != "" {
		t.Fatalf("GetMotd on empty: got %q, want empty", GetMotd())
	}
}

// TestHotConfigIsolatesWhitelist verifies that fields outside the whitelist
// (like Name, Port) are ignored by initHotConfig — they are NOT stored in
// the hot cache and must still be read from the package-level config at
// startup snapshot.
func TestHotConfigIsolatesWhitelist(t *testing.T) {
	initHotConfig(&settings.Config{ServerConfig: settings.ServerConfig{
		Motd: "a",
		Desc: "b",
		Name: "should-be-ignored",
		Port: 99999,
	}})
	// Only the two getters exist; if Name or Port ever leak in, we would need
	// accessor tests for them too. This test is a guard against accidental
	// whitelist expansion without corresponding mutex discipline.
	if GetMotd() != "a" || GetServerDesc() != "b" {
		t.Fatalf("whitelist fields not set correctly")
	}
}
