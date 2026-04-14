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
	"errors"
	"fmt"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// Handles /favourite <char name>
//
// Toggles a character in the player's wardrobe favourites list.
// If the character is not yet favourited it is added; if it is already
// favourited it is removed. Requires an active player account (/login).
func cmdFavourite(client *Client, args []string, _ string) {
	if !client.Authenticated() {
		client.SendServerMessage(
			"You need an account to use the Wardrobe feature.\n" +
				"Create one for free: /register <username> <password>")
		return
	}

	charName := strings.Join(args, " ")
	charID := getCharacterID(charName)
	if charID == -1 {
		client.SendServerMessage(fmt.Sprintf("Character \"%v\" was not found in the character list.", charName))
		return
	}
	// Use the canonical character name from the list so the stored name always
	// matches the exact casing in characters[].
	canonicalName := characters[charID]
	username := client.ModName()

	already, err := db.IsFavourite(username, canonicalName)
	if err != nil {
		client.SendServerMessage("Failed to check favourites. Please try again.")
		return
	}

	if already {
		if err := db.RemoveFavourite(username, canonicalName); err != nil {
			client.SendServerMessage("Failed to remove favourite. Please try again.")
			return
		}
		client.SendServerMessage(fmt.Sprintf("💔 Removed %v from your wardrobe favourites.", canonicalName))
		return
	}

	if err := db.AddFavourite(username, canonicalName); err != nil {
		if errors.Is(err, db.ErrFavouriteLimitReached) {
			client.SendServerMessage(fmt.Sprintf(
				"Your wardrobe is full! You can save up to %d favourites.\n"+
					"Use /favourite <char> on an existing favourite to remove it.", db.MaxFavourites))
		} else {
			client.SendServerMessage("Failed to add favourite. Please try again.")
		}
		return
	}
	client.SendServerMessage(fmt.Sprintf(
		"⭐ Added %v to your wardrobe favourites!\n"+
			"Use /wardrobe to view your list, or /wardrobe %v to swap to them.",
		canonicalName, canonicalName))
}

// Handles /wardrobe [char name]
//
// With no arguments: lists the player's saved favourite characters.
// With a character name: swaps to that character if it is in the favourites list.
// Requires an active player account (/login).
func cmdWardrobe(client *Client, args []string, _ string) {
	if !client.Authenticated() {
		client.SendServerMessage(
			"You need an account to use the Wardrobe feature.\n" +
				"Create one for free: /register <username> <password>")
		return
	}

	username := client.ModName()

	// ── Swap path ────────────────────────────────────────────────────────────
	// Resolve character and check membership with a single indexed DB lookup
	// instead of fetching the entire favourites list just to scan it.
	if len(args) > 0 {
		charName := strings.Join(args, " ")
		charID := getCharacterID(charName)
		if charID == -1 {
			client.SendServerMessage(fmt.Sprintf("Character \"%v\" was not found in the character list.", charName))
			return
		}
		canonicalName := characters[charID]

		isFav, err := db.IsFavourite(username, canonicalName)
		if err != nil {
			client.SendServerMessage("Failed to check wardrobe. Please try again.")
			return
		}
		if !isFav {
			client.SendServerMessage(fmt.Sprintf(
				"\"%v\" is not in your wardrobe.\n"+
					"Use /favourite %v to add them first, then /wardrobe %v to swap.",
				canonicalName, canonicalName, canonicalName))
			return
		}

		// Respect char-stuck punishment.
		if stuckID := client.charStuckID(); stuckID >= 0 && charID != stuckID {
			client.SendServerMessage(fmt.Sprintf(
				"You are character stuck as %v and cannot change characters.", characters[stuckID]))
			return
		}

		// Respect forced tung iniswap.
		if client.IsTunged() {
			client.SendServerMessage("You have been tunged and cannot change characters until the effect is removed.")
			return
		}

		if client.Area().IsTaken(charID) && client.CharID() != charID {
			client.SendServerMessage(fmt.Sprintf(
				"Character \"%v\" is already taken in this area.", canonicalName))
			return
		}

		client.ChangeCharacter(charID)
		client.SendServerMessage(fmt.Sprintf("👗 Swapped to %v from your wardrobe!", canonicalName))
		return
	}

	// ── List path ─────────────────────────────────────────────────────────────
	favourites, err := db.GetFavourites(username)
	if err != nil {
		client.SendServerMessage("Failed to load wardrobe. Please try again.")
		return
	}

	if len(favourites) == 0 {
		client.SendServerMessage(
			"👗 Your wardrobe is empty!\n\n" +
				"Add characters with /favourite <char name>.\n" +
				"Then use /wardrobe <char name> to swap to them instantly.")
		return
	}

	// Pre-size the builder. Estimates per segment:
	//   25 B  — header line "👗 Your Wardrobe (N/100):\n"
	//   35 B  — per entry "  NN. <char name>\n"  (avg ~12-char name + index + spacing)
	//   90 B  — two footer lines
	var sb strings.Builder
	sb.Grow(25 + len(favourites)*35 + 90)
	fmt.Fprintf(&sb, "👗 Your Wardrobe (%d/%d):\n", len(favourites), db.MaxFavourites)
	for i, name := range favourites {
		fmt.Fprintf(&sb, "  %2d. %v\n", i+1, name)
	}
	sb.WriteString("\nUse /wardrobe <char name> to swap to any character above.\n")
	sb.WriteString("Use /favourite <char name> to add or remove characters.")
	client.SendServerMessage(sb.String())
}

