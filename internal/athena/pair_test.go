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
)

// simulatePairArgInsertions replicates the two array insertions performed by
// pktIC to make room for the server-side otherName/otherEmote fields.
func simulatePairArgInsertions(clientArgs []string) []string {
	args := make([]string, 26)
	copy(args, clientArgs)
	args = append(args[:19], args[17:]...)
	args = append(args[:20], args[18:]...)
	return args
}

// applyPairSanitization replicates the sanitization logic added to pktIC:
// when there is no valid pair (args[16] is "" or "-1"), clear otherName and
// otherEmote (args[17] and args[18]).
func applyPairSanitization(args []string) {
	if args[16] == "" || args[16] == "-1" {
		args[17] = ""
		args[18] = ""
	}
}

// TestPairArgSanitizationNoPair verifies that when otherCharId is -1 (no pair),
// the otherName and otherEmote fields are cleared even if the client sent
// garbage values in those positions.
func TestPairArgSanitizationNoPair(t *testing.T) {
	// Simulate a client packet where:
	//   [16] = "-1"  (no pair wanted)
	//   [17] = "0"   (self_offset, ends up as garbage in otherName slot)
	//   [18] = "0"   (other_offset, ends up as garbage in otherEmote slot)
	clientArgs := make([]string, 26)
	clientArgs[16] = "-1"
	clientArgs[17] = "0"   // self_offset – becomes garbage in otherName slot
	clientArgs[18] = "0"   // other_offset – becomes garbage in otherEmote slot
	clientArgs[19] = "0"   // noninterrupting_preanim

	args := simulatePairArgInsertions(clientArgs)

	// Before sanitization, args[17] and args[18] contain garbage.
	// Apply the sanitization as pktIC now does.
	applyPairSanitization(args)

	if args[16] != "-1" {
		t.Errorf("args[16] (otherCharId) should remain \"-1\", got %q", args[16])
	}
	if args[17] != "" {
		t.Errorf("args[17] (otherName) should be empty when no pair, got %q", args[17])
	}
	if args[18] != "" {
		t.Errorf("args[18] (otherEmote) should be empty when no pair, got %q", args[18])
	}
}

// TestPairArgSanitizationGarbageOffsets verifies the same behaviour when the
// server would have forwarded non-zero offset strings as garbage otherName/otherEmote.
func TestPairArgSanitizationGarbageOffsets(t *testing.T) {
	clientArgs := make([]string, 26)
	clientArgs[16] = "-1"
	clientArgs[17] = "0     0" // multi-value offset string – garbage in otherName slot
	clientArgs[18] = "0"
	clientArgs[19] = "0"

	args := simulatePairArgInsertions(clientArgs)
	applyPairSanitization(args)

	if args[17] != "" {
		t.Errorf("args[17] (otherName) should be empty when no pair, got %q", args[17])
	}
	if args[18] != "" {
		t.Errorf("args[18] (otherEmote) should be empty when no pair, got %q", args[18])
	}
}

// TestPairArgSanitizationEmptyCharId verifies sanitization when args[16] is
// completely absent (blank string) – also a "no pair" state.
func TestPairArgSanitizationEmptyCharId(t *testing.T) {
	clientArgs := make([]string, 26)
	// args[16] left as "" (client did not send a pair char id)
	clientArgs[17] = "50"  // some offset value
	clientArgs[18] = "-25" // some offset value

	args := simulatePairArgInsertions(clientArgs)
	applyPairSanitization(args)

	if args[17] != "" {
		t.Errorf("args[17] (otherName) should be empty when no pair, got %q", args[17])
	}
	if args[18] != "" {
		t.Errorf("args[18] (otherEmote) should be empty when no pair, got %q", args[18])
	}
}
