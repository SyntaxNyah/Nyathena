/* Athena - A server for Attorney Online 2 written in Go

   Nyathena fork addition: the uma character pool shared by /horse and
   /umahorse.

   The pool is discovered from characters.txt by MARKER rather than by a
   hardcoded list of names. Any character whose name carries "(uma)" or
   "(uma_h)" is in the pool, so an operator who adds new uma characters to
   characters.txt gets them picked up on the next /reload without a server
   rebuild — which is the whole reason the marker convention exists.
   Characters from the same franchise that carry no marker (or a different
   one, e.g. "(gbf)") are deliberately NOT in the pool: the marker is the
   opt-in.

   Two consumers, deliberately different in kind:

     /horse    — a ONE-SHOT swap. The target is genuinely moved onto a free
                 uma slot (Client.ChangeCharacter), so the area's taken-char
                 table, /players and everyone's client all agree, and the
                 target may change away afterwards like any /charcurse.

     /umahorse — a PER-MESSAGE re-roll. Here a real character change is the
                 wrong tool: ChangeCharacter broadcasts a CharsCheck to the
                 whole area, which is ~9 KB per recipient on a large roster
                 (see the "CharsCheck Fan-Out" note in CLAUDE.md), so doing
                 one per IC message would be a serious regression. Instead
                 the outgoing IC packet's sprite fields are rewritten just
                 before broadcast — the same technique /forcedisplay and the
                 forced iniswap (/tung) already use — which costs one slice
                 index and two string assignments and claims no slot. */

package athena

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/packet"
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

// randomUmaCharSlot returns a random uma character slot for a sprite rewrite,
// or -1 if the pool is empty. Unlike randomFreeUmaChar it ignores the area's
// taken table: nothing is being claimed, the slot is only being rendered, so
// two punished players may well show as the same uma at once.
//
// exclude is skipped when the pool has an alternative — callers pass the
// packet's pair partner so a message can never render as a character paired
// with itself, which clients handle badly.
func randomUmaCharSlot(exclude int) int {
	ids := getUmaCharIDs()
	switch len(ids) {
	case 0:
		return -1
	case 1:
		return ids[0] // nothing to swap to; excluding it would mean no sprite
	}
	// Slots are distinct, so at most one entry can equal exclude; stepping to
	// the next index is therefore guaranteed to land on a different character
	// and always succeeds, unlike rejection sampling.
	i := rand.Intn(len(ids))
	if ids[i] == exclude {
		i = (i + 1) % len(ids)
	}
	return ids[i]
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

// applyUmaHorseSprite rewrites the outgoing IC packet's sprite to a random uma
// character when the speaker carries an active /umahorse punishment, so every
// message they send arrives as a different horse girl. punishments is the
// speaker's already-filtered active set from pktIC, so no extra lock is taken.
//
// Called after field validation (like every other packet rewrite) and before
// maybeApplyForceDisplay, which is documented as having the final word on the
// outgoing sprite.
//
// The emote is reset to "normal" and any preanimation is stripped: the
// speaker's own emote and preanim names come from their real character's ini
// and are not going to exist on a randomly-drawn uma, so pointing the viewport
// at them would ask the client to play animations that aren't there.
func applyUmaHorseSprite(ms *packet.MSPacket, punishments []PunishmentState) {
	active := false
	for i := range punishments {
		if punishments[i].punishmentType == PunishmentUmaHorse {
			active = true
			break
		}
	}
	if !active {
		return
	}

	// Never draw the pair partner's slot: a character paired with itself
	// renders badly on desktop AO2.
	exclude := -1
	if ms.OtherCharID != "" {
		// The pair field may carry an order suffix ("5^1"); the slot is the
		// part before it, same as the pair resolution earlier in pktIC.
		pidStr, _, _ := strings.Cut(ms.OtherCharID, "^")
		if n, err := strconv.Atoi(pidStr); err == nil {
			exclude = n
		}
	}

	id := randomUmaCharSlot(exclude)
	if id == -1 {
		return // no uma characters on this server; leave the sprite alone
	}
	chars := getCharacters()
	if id >= len(chars) {
		return
	}
	ms.Character = chars[id]
	ms.CharID = strconv.Itoa(id)
	ms.Emote = "normal"
	ms.PreAnim = "-"
	switch ms.EmoteModifier {
	case "1", "2":
		ms.EmoteModifier = "0"
	case "6":
		ms.EmoteModifier = "5"
	}
}
