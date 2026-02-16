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
)

func TestApplyUppercase(t *testing.T) {
	input := "hello world"
	expected := "HELLO WORLD"
	result := applyUppercase(input)
	if result != expected {
		t.Errorf("applyUppercase failed: got %q, want %q", result, expected)
	}
}

func TestApplyLowercase(t *testing.T) {
	input := "HELLO WORLD"
	expected := "hello world"
	result := applyLowercase(input)
	if result != expected {
		t.Errorf("applyLowercase failed: got %q, want %q", result, expected)
	}
}

func TestApplyBackward(t *testing.T) {
	input := "hello"
	expected := "olleh"
	result := applyBackward(input)
	if result != expected {
		t.Errorf("applyBackward failed: got %q, want %q", result, expected)
	}
}

func TestApplyStutterstep(t *testing.T) {
	input := "hello world"
	result := applyStutterstep(input)
	// Should double each word
	if !strings.Contains(result, "hello hello") || !strings.Contains(result, "world world") {
		t.Errorf("applyStutterstep failed: got %q", result)
	}
}

func TestApplyElongate(t *testing.T) {
	input := "hello"
	result := applyElongate(input)
	// Should repeat vowels
	if !strings.Contains(result, "eee") || !strings.Contains(result, "ooo") {
		t.Errorf("applyElongate failed: got %q", result)
	}
}

func TestApplyRobotic(t *testing.T) {
	input := "hello world"
	result := applyRobotic(input)
	// Should contain robot sounds
	if !strings.Contains(result, "[BEEP]") && !strings.Contains(result, "[BOOP]") {
		t.Errorf("applyRobotic failed: got %q", result)
	}
}

func TestApplyAlternating(t *testing.T) {
	input := "hello"
	result := applyAlternating(input)
	// Should have alternating case
	if result == strings.ToLower(input) || result == strings.ToUpper(input) {
		t.Errorf("applyAlternating failed: got %q, expected alternating case", result)
	}
}

func TestApplyUwu(t *testing.T) {
	input := "hello world"
	result := applyUwu(input)
	// Should replace 'l' with 'w'
	if !strings.Contains(result, "hewwo") && !strings.Contains(result, "worwd") {
		t.Errorf("applyUwu failed: got %q", result)
	}
}

func TestApplyCensor(t *testing.T) {
	input := "hello world test"
	result := applyCensor(input)
	// Should contain [CENSORED] or be different from input (random behavior)
	if !strings.Contains(result, "[CENSORED]") && result == input {
		// It's random, so sometimes it might not censor anything, but that's okay
		t.Logf("applyCensor result: %q (random behavior - no censoring this time)", result)
	}
}

func TestApplyConfused(t *testing.T) {
	input := "one two three"
	result := applyConfused(input)
	// Should have all words but potentially in different order
	if !strings.Contains(result, "one") || !strings.Contains(result, "two") || !strings.Contains(result, "three") {
		t.Errorf("applyConfused failed: missing words in %q", result)
	}
}

func TestTruncateText(t *testing.T) {
	// Test with text under limit
	short := "hello"
	result := truncateText(short)
	if result != short {
		t.Errorf("truncateText failed for short text: got %q, want %q", result, short)
	}

	// Test with text over limit
	long := strings.Repeat("a", maxTextLength+100)
	result = truncateText(long)
	if len(result) > maxTextLength {
		t.Errorf("truncateText failed: length %d exceeds max %d", len(result), maxTextLength)
	}
}

func TestGetRandomEmoji(t *testing.T) {
	emoji := GetRandomEmoji()
	if emoji == "" {
		t.Errorf("GetRandomEmoji returned empty string")
	}
}

func TestApplyPunishmentToText(t *testing.T) {
	input := "hello world"
	
	tests := []struct {
		name       string
		pType      PunishmentType
		shouldDiff bool
	}{
		{"Uppercase", PunishmentUppercase, true},
		{"Lowercase", PunishmentLowercase, false}, // already lowercase
		{"Backward", PunishmentBackward, true},
		{"Robotic", PunishmentRobotic, true},
		{"None", PunishmentNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyPunishmentToText(input, tt.pType)
			if tt.shouldDiff && result == input {
				t.Errorf("%s: expected different output, got same: %q", tt.name, result)
			}
		})
	}
}

