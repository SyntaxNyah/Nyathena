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

package packet

import (
	"strings"
	"testing"
)

// TestMCToClientArgsDefaultsNumericFields guards the /play regression: an MC
// packet whose Effects (or Looping/Channel) field is left empty must never
// reach the wire as a bare "##", because AO2 clients parse those slots as
// numbers and silently drop the music change on a parse failure. Args() must
// substitute "0".
func TestMCToClientArgsDefaultsNumericFields(t *testing.T) {
	p := &MCToClient{
		Name: "https://file.garden/h.mp3", CharID: 1, Showname: "",
		Looping: "1", Channel: "0", Effects: "", // Effects left empty, as /play used to
	}
	args := p.Args()
	want := []string{"https://file.garden/h.mp3", "1", "", "1", "0", "0"}
	if len(args) != len(want) {
		t.Fatalf("Args() length = %d, want %d (%v)", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("Args()[%d] = %q, want %q", i, args[i], want[i])
		}
	}

	// The FantaCode wire form must end in "#0#%", not the broken "##%".
	wire := p.Header() + "#" + strings.Join(args, "#") + "#%"
	if strings.HasSuffix(wire, "##%") {
		t.Errorf("MC wire form ends in malformed '##%%': %q", wire)
	}
	if !strings.HasSuffix(wire, "#0#%") {
		t.Errorf("MC wire form should end in '#0#%%', got %q", wire)
	}
}

// TestMCToClientArgsPreservesURL is the no-mangle guarantee at the
// serialization layer: a streaming URL passes through Args() byte-for-byte.
func TestMCToClientArgsPreservesURL(t *testing.T) {
	const url = "https://host.com/stream.mp3"
	p := &MCToClient{Name: url, CharID: 2, Looping: "1", Channel: "0", Effects: "0"}
	if got := p.Args()[0]; got != url {
		t.Errorf("Args()[0] = %q, want verbatim URL %q", got, url)
	}
}
