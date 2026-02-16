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
	"time"
)

// TestStackingPunishments tests that multiple different punishment types can be applied
func TestStackingPunishments(t *testing.T) {
	client := &Client{
		punishments: []PunishmentState{},
	}

	// Apply multiple different punishments
	client.AddPunishment(PunishmentUppercase, 10*time.Minute, "Test reason 1")
	client.AddPunishment(PunishmentBackward, 10*time.Minute, "Test reason 2")
	client.AddPunishment(PunishmentUwu, 10*time.Minute, "Test reason 3")

	if len(client.punishments) != 3 {
		t.Errorf("Expected 3 stacked punishments, got %d", len(client.punishments))
	}

	// Verify each punishment type is present
	hasUppercase := false
	hasBackward := false
	hasUwu := false

	for _, p := range client.punishments {
		switch p.punishmentType {
		case PunishmentUppercase:
			hasUppercase = true
		case PunishmentBackward:
			hasBackward = true
		case PunishmentUwu:
			hasUwu = true
		}
	}

	if !hasUppercase || !hasBackward || !hasUwu {
		t.Errorf("Not all punishment types were properly stacked")
	}
}

// TestPunishmentReplacement tests that adding the same punishment type replaces the old one
func TestPunishmentReplacement(t *testing.T) {
	client := &Client{
		punishments: []PunishmentState{},
	}

	// Apply same punishment twice
	client.AddPunishment(PunishmentUppercase, 10*time.Minute, "First")
	client.AddPunishment(PunishmentUppercase, 20*time.Minute, "Second")

	if len(client.punishments) != 1 {
		t.Errorf("Expected 1 punishment (replacement), got %d", len(client.punishments))
	}

	if client.punishments[0].reason != "Second" {
		t.Errorf("Expected second punishment to replace first, got reason: %s", client.punishments[0].reason)
	}
}

// TestPunishmentTypeStringConversion tests the String() method
func TestPunishmentTypeStringConversion(t *testing.T) {
	tests := []struct {
		pType    PunishmentType
		expected string
	}{
		{PunishmentUppercase, "uppercase"},
		{PunishmentBackward, "backward"},
		{PunishmentUwu, "uwu"},
		{PunishmentNone, "none"},
	}

	for _, tt := range tests {
		result := tt.pType.String()
		if result != tt.expected {
			t.Errorf("PunishmentType.String() for %v: expected %s, got %s", tt.pType, tt.expected, result)
		}
	}
}

// TestParsePunishmentType tests the parsePunishmentType function
func TestParsePunishmentType(t *testing.T) {
	tests := []struct {
		input    string
		expected PunishmentType
	}{
		{"uppercase", PunishmentUppercase},
		{"UPPERCASE", PunishmentUppercase}, // Test case insensitivity
		{"backward", PunishmentBackward},
		{"uwu", PunishmentUwu},
		{"invalid", PunishmentNone},
	}

	for _, tt := range tests {
		result := parsePunishmentType(tt.input)
		if result != tt.expected {
			t.Errorf("parsePunishmentType(%s): expected %v, got %v", tt.input, tt.expected, result)
		}
	}
}

// TestTournamentParticipantCreation tests tournament participant initialization
func TestTournamentParticipantCreation(t *testing.T) {
	participant := &TournamentParticipant{
		uid:          123,
		messageCount: 0,
		joinedAt:     time.Now().UTC(),
	}

	if participant.uid != 123 {
		t.Errorf("Expected UID 123, got %d", participant.uid)
	}

	if participant.messageCount != 0 {
		t.Errorf("Expected message count 0, got %d", participant.messageCount)
	}
}

// TestApplyMultiplePunishments tests that multiple punishment effects are applied sequentially
func TestApplyMultiplePunishments(t *testing.T) {
	input := "hello world"

	// Apply uppercase first
	step1 := ApplyPunishmentToText(input, PunishmentUppercase)
	if step1 != "HELLO WORLD" {
		t.Errorf("Uppercase transformation failed: got %s", step1)
	}

	// Then apply backward to the uppercased result
	step2 := ApplyPunishmentToText(step1, PunishmentBackward)
	if step2 != "DLROW OLLEH" {
		t.Errorf("Sequential punishment application failed: got %s", step2)
	}
}
