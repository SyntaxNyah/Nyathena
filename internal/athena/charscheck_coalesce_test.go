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

import "testing"

// The first change is immediate, and a burst that follows within the debounce
// window collapses into one trailing CharsCheck per area. Before coalescing,
// every CC broadcast the full ~5,000-entry list to every area member; a 319-CC
// minute became thousands of full-list sends.
func TestRequestCharsCheckCoalescesBurst(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	cl, a := benchClients(3)
	clients = cl

	// The leading edge must be immediate for ordinary single changes and for
	// opening the character-select screen.
	requestCharsCheck(a)
	for c := range cl.list {
		select {
		case <-c.sendCh:
		default:
			t.Fatal("client received no immediate CharsCheck")
		}
	}

	for i := 0; i < 99; i++ {
		requestCharsCheck(a)
	}
	flushCharsCheck()

	for c := range cl.list {
		select {
		case <-c.sendCh:
		default:
			t.Fatal("client received no trailing CharsCheck")
		}
		select {
		case <-c.sendCh:
			t.Fatal("client received more than one trailing CharsCheck for one burst")
		default:
		}
	}
}

// Nil areas are legal (a client that has not joined yet); they must not panic
// or leave the flusher in a bad state.
func TestRequestCharsCheckNilArea(t *testing.T) {
	requestCharsCheck(nil)
	flushCharsCheck()
}

// Multiple areas in one burst must each get their own packet, not one shared
// broadcast to every client online.
func TestRequestCharsCheckScopesPerArea(t *testing.T) {
	prev := clients
	t.Cleanup(func() { clients = prev })
	first, a := benchClients(2)
	second, b := benchClients(2)
	for c := range second.list {
		first.list[c] = struct{}{}
	}
	clients = first

	requestCharsCheck(a)
	requestCharsCheck(b)
	flushCharsCheck()

	for c := range first.list {
		select {
		case <-c.sendCh:
		default:
			t.Fatal("client received no CharsCheck after multi-area flush")
		}
		select {
		case <-c.sendCh:
			t.Fatal("client received an unexpected duplicate CharsCheck")
		default:
		}
	}
}
