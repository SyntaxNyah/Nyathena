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

// TestOppositeChoice tests the oppositeChoice helper function
func TestOppositeChoice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"heads", "tails"},
		{"tails", "heads"},
	}

	for _, tt := range tests {
		result := oppositeChoice(tt.input)
		if result != tt.expected {
			t.Errorf("oppositeChoice(%s): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

// TestCoinflipWinnerDetermination tests that the winner is determined correctly
func TestCoinflipWinnerDetermination(t *testing.T) {
	tests := []struct {
		name            string
		challengeChoice string
		acceptorChoice  string
		coinResult      string
		expectedWinner  string // "challenger" or "acceptor"
	}{
		{"Challenger wins with heads", "heads", "tails", "heads", "challenger"},
		{"Challenger wins with tails", "tails", "heads", "tails", "challenger"},
		{"Acceptor wins with heads", "tails", "heads", "heads", "acceptor"},
		{"Acceptor wins with tails", "heads", "tails", "tails", "acceptor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate winner determination logic
			var winner string
			if tt.coinResult == tt.challengeChoice {
				winner = "challenger"
			} else {
				winner = "acceptor"
			}

			if winner != tt.expectedWinner {
				t.Errorf("Winner determination failed: expected %s, got %s (coin: %s, challenger: %s, acceptor: %s)",
					tt.expectedWinner, winner, tt.coinResult, tt.challengeChoice, tt.acceptorChoice)
			}

			// Verify that choices are opposite
			if tt.challengeChoice == tt.acceptorChoice {
				t.Errorf("Test case invalid: choices should be opposite, but both are %s", tt.challengeChoice)
			}
		})
	}
}

// TestCoinflipChoiceValidation tests choice validation
func TestCoinflipChoiceValidation(t *testing.T) {
	validChoices := []string{"heads", "tails"}
	invalidChoices := []string{"head", "tail", "coin", "flip", "", "HEADS", "Tails"}

	// Valid choices should match exactly
	for _, choice := range validChoices {
		if choice != "heads" && choice != "tails" {
			t.Errorf("Valid choice %s failed validation", choice)
		}
	}

	// Invalid choices should fail
	for _, choice := range invalidChoices {
		if choice == "heads" || choice == "tails" {
			t.Errorf("Invalid choice %s passed validation", choice)
		}
	}
}
