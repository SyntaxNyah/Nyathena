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

import "testing"

// newTestClients swaps in a fresh global client list for the duration of a test
// and restores the original on cleanup.
func newTestClients(t *testing.T) {
	t.Helper()
	orig := clients
	t.Cleanup(func() { clients = orig })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}
}
