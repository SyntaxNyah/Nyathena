/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: /charshuffle and /uncharshuffle.

   Mirrors /nameshuffle but operates on character IDs. The shuffle uses
   Sattolo's algorithm to guarantee every player ends up on a different
   character (a derangement) without self-swaps.

   Atomicity: AO2's character system enforces "one client per character
   slot per area" via Area.taken[]. Naïvely calling ChangeCharacter() in
   a loop would fail on the very first swap because the destination slot
   is still occupied. We work around this by clearing every shuffled
   client's slot first, then taking each new slot — at which point the
   old occupant has already vacated. */

package athena

import (
	"fmt"
	"math/rand"
	"strconv"
)

// shuffledOrigCharID accessors

func (c *Client) ShuffledOrigCharID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shuffledOrigCharID
}

func (c *Client) SetShuffledOrigCharID(id int) {
	c.mu.Lock()
	c.shuffledOrigCharID = id
	c.mu.Unlock()
}

// cmdCharShuffle randomly permutes the character IDs of every player in the
// current area. Like /nameshuffle but for sprites: char1↔char2, char2↔char5,
// etc. Players already on character-select are skipped. The original IDs
// are remembered so /uncharshuffle can put everyone back exactly.
func cmdCharShuffle(client *Client, _ []string, _ string) {
	targetArea := client.Area()
	if targetArea.PlayerCount() < 2 {
		client.SendServerMessage("There are not enough players in this area to shuffle characters (need at least 2).")
		return
	}

	type entry struct {
		c       *Client
		origCID int
	}
	var participants []entry
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 || c.Area() != targetArea {
			return
		}
		// Skip charselect players — there's nothing to swap.
		if c.CharID() < 0 {
			return
		}
		participants = append(participants, entry{c: c, origCID: c.CharID()})
	})
	if len(participants) < 2 {
		client.SendServerMessage("There are not enough players with selected characters to shuffle (need at least 2).")
		return
	}

	// Build the destination slice and shuffle it in place via Sattolo so every
	// participant ends up on someone else's char.
	newIDs := make([]int, len(participants))
	for i, p := range participants {
		newIDs[i] = p.origCID
	}
	for i := len(newIDs) - 1; i > 0; i-- {
		j := rand.Intn(i)
		newIDs[i], newIDs[j] = newIDs[j], newIDs[i]
	}

	// Phase 1: free every participant's current character slot in the area.
	// Direct slot-table manipulation via SwitchChar(old, -1) is the safe way
	// to release without sending PV/PU broadcasts mid-shuffle.
	for _, p := range participants {
		targetArea.SwitchChar(p.origCID, -1)
		// Remember the pre-shuffle character so /uncharshuffle can restore it.
		// Only set if not already shuffled, so re-shuffling preserves the very
		// first original (a player who survives multiple shuffles still maps
		// back to their pre-shuffle character).
		if p.c.ShuffledOrigCharID() == -2 {
			p.c.SetShuffledOrigCharID(p.origCID)
		}
	}

	// Phase 2: claim the new slot and send the IC-side update for each
	// participant. Now every destination slot is free, so SwitchChar succeeds.
	for i, p := range participants {
		newID := newIDs[i]
		if !targetArea.SwitchChar(-1, newID) {
			// Should not happen — defensive fallback: pick any free slot.
			for cand := 0; cand < len(characters); cand++ {
				if !targetArea.IsTaken(cand) {
					if targetArea.SwitchChar(-1, cand) {
						newID = cand
						break
					}
				}
			}
		}
		p.c.SetCharID(newID)
		p.c.SendPacket("PV", "0", "CID", strconv.Itoa(newID))
		p.c.SendServerMessage("A moderator has shuffled characters in this area.")
	}
	// Single CharsCheck broadcast at the end is much cheaper than per-client.
	writeToArea(targetArea, "CharsCheck", targetArea.Taken()...)
	// Push PU updates so other players see the new char names too.
	for _, p := range participants {
		uid := strconv.Itoa(p.c.Uid())
		writeToAll("PU", uid, "1", p.c.CurrentCharacter())
	}

	client.SendServerMessage(fmt.Sprintf("Shuffled characters of %d players in the area.", len(participants)))
	addToBuffer(client, "CMD", fmt.Sprintf("Shuffled characters of %d players in area %v", len(participants), targetArea.Name()), true)
}

// cmdUnCharShuffle restores every participant's pre-shuffle character.
// Only acts on players whose ShuffledOrigCharID is set (-2 = untouched).
// Same two-phase logic as the shuffle: free first, then claim originals.
func cmdUnCharShuffle(client *Client, _ []string, _ string) {
	targetArea := client.Area()

	type entry struct {
		c       *Client
		origCID int
	}
	var participants []entry
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 || c.Area() != targetArea {
			return
		}
		orig := c.ShuffledOrigCharID()
		if orig == -2 {
			return // not shuffled
		}
		participants = append(participants, entry{c: c, origCID: orig})
	})
	if len(participants) == 0 {
		client.SendServerMessage("No players in this area have been character-shuffled.")
		return
	}

	// Free current slots.
	for _, p := range participants {
		if cur := p.c.CharID(); cur >= 0 {
			targetArea.SwitchChar(cur, -1)
		}
	}

	// Restore each player's original char (or charselect if it was -1).
	for _, p := range participants {
		if p.origCID < 0 {
			p.c.SetCharID(-1)
			p.c.SendPacket("PV", "0", "CID", "-1")
		} else if targetArea.SwitchChar(-1, p.origCID) {
			p.c.SetCharID(p.origCID)
			p.c.SendPacket("PV", "0", "CID", strconv.Itoa(p.origCID))
		} else {
			// Original slot is somehow taken (shouldn't happen unless someone
			// joined mid-shuffle). Fall back to charselect.
			p.c.SetCharID(-1)
			p.c.SendPacket("PV", "0", "CID", "-1")
		}
		p.c.SetShuffledOrigCharID(-2)
		p.c.SendServerMessage("A moderator has restored characters in this area.")
	}
	writeToArea(targetArea, "CharsCheck", targetArea.Taken()...)
	for _, p := range participants {
		uid := strconv.Itoa(p.c.Uid())
		writeToAll("PU", uid, "1", p.c.CurrentCharacter())
	}

	client.SendServerMessage(fmt.Sprintf("Restored characters of %d players in the area.", len(participants)))
	addToBuffer(client, "CMD", fmt.Sprintf("Restored characters of %d players in area %v", len(participants), targetArea.Name()), true)
}
