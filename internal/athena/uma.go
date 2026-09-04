/* Athena - A server for Attorney Online 2 written in Go

   Nyathena fork addition: the uma character pool behind /horse.

   The pool is discovered from characters.txt by MARKER rather than by a
   hardcoded list of names. Any character whose name carries "(uma)" or
   "(uma_h)" is in the pool, so an operator who adds new uma characters to
   characters.txt gets them picked up on the next /reload without a server
   rebuild — which is the whole reason the marker convention exists.
   Characters from the same franchise that carry no marker (or a different
   one, e.g. "(gbf)") are deliberately NOT in the pool: the marker is the
   opt-in.

   /horse applies a ONE-SHOT swap: the target is genuinely moved onto a free
   uma slot (Client.ChangeCharacter), so the area's taken-char table,
   /players and everyone's client all agree, and the target may change away
   afterwards like any /charcurse. Nothing here touches the IC hot path. */

package athena

import (
	"math/rand"
	"strings"
)

// umaCharMarkers are the case-insensitive substrings that opt a character in
// characters.txt into the uma pool. Note that "(uma_h)" does not contain the
// literal "(uma)" — the closing paren makes them disjoint — so a name can
// never be counted by both markers.
var umaCharMarkers = []string{"(uma)", "(uma_h)"}

// isUmaCharacter reports whether a character name carries a uma marker.
func isUmaCharacter(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range umaCharMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// buildUmaCharIDs returns the slots of chars whose names carry a uma marker.
// Called from setCharacters; the result is published as an immutable snapshot.
func buildUmaCharIDs(chars []string) []int {
	var ids []int
	for i, name := range chars {
		if isUmaCharacter(name) {
			ids = append(ids, i)
		}
	}
	return ids
}

// getUmaCharIDs returns the current uma slots. Empty on a server whose
// characters.txt has no marked characters — which is the upstream default, and
// the reason every caller treats an empty pool as "do nothing" rather than as
// an error.
//
// Entries are bound-checked against the live character list before being
// returned, so a cache published against a longer list can never hand out an
// index that would panic the IC path. The linear-scan fallback mirrors
// getCharacterID's: it keeps the pool correct if the character list was ever
// published without its derived caches.
func getUmaCharIDs() []int {
	chars := getCharacters()
	cached := umaCharIDsPtr.Load()
	if cached == nil {
		return buildUmaCharIDs(chars)
	}
	ids := *cached
	// buildUmaCharIDs walks the roster in order, so the slots are ascending and
	// checking the last one bounds the whole slice in O(1).
	if len(ids) > 0 && ids[len(ids)-1] >= len(chars) {
		return buildUmaCharIDs(chars)
	}
	return ids
}

// randomFreeUmaChar returns a uma character slot that is free in the target's
// area, or -1 when the server has no uma characters or every one of them is
// already taken there. Mirrors getRandomFreeChar, narrowed to the uma pool.
func randomFreeUmaChar(target *Client) int {
	ids := getUmaCharIDs()
	if len(ids) == 0 {
		return -1
	}
	free := make([]int, 0, len(ids))
	for _, id := range ids {
		if !target.Area().IsTaken(id) {
			free = append(free, id)
		}
	}
	if len(free) == 0 {
		return -1
	}
	return free[rand.Intn(len(free))]
}

// turnIntoRandomUma moves the target onto a random free uma character and
// returns its name. It reports false — changing nothing — when the server has
// no free uma character, or when another moderator effect has already pinned
// the target's character (a /charstuck lock or a forced iniswap), so this
// cosmetic extra never quietly overrides a deliberate lock. Callers apply the
// punishment either way; the swap is a bonus, never a precondition.
func turnIntoRandomUma(target *Client) (string, bool) {
	if target.IsTunged() || target.charStuckID() >= 0 {
		return "", false
	}
	if target.CharID() == -1 {
		return "", false // spectating; there is no sprite to swap
	}
	id := randomFreeUmaChar(target)
	if id == -1 {
		return "", false
	}
	target.ChangeCharacter(id)
	// ChangeCharacter is a no-op if the area refused the slot, so confirm the
	// swap actually landed before telling anyone it did.
	if target.CharID() != id {
		return "", false
	}
	return getCharacters()[id], true
}
