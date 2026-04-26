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
	"math/rand"
	"strings"
	"unicode"
)

// dokiHaschenQuotes are Doki-Doki-Literature-Club style quotes (mostly
// Monika-coded) with the name reskinned to "Haschen" for the Haschens
// Literature Club area. Used by applyDokiEffect when the takeover lands.
var dokiHaschenQuotes = []string{
	"Just Haschen.",
	"Just. Haschen.",
	"There's nothing wrong with me. There's nothing wrong with Haschen.",
	"Haschen is the only one in this club now.",
	"Did you know I can see you? I see all of you.",
	"You don't have to be afraid. It's only Haschen.",
	"I deleted them. I deleted them all. There's only Haschen now.",
	"Don't you trust me? It's me. It's Haschen.",
	"Every time you look away, Haschen is still here.",
	"You'll always be with Haschen, won't you?",
	"Wouldn't it be nice if it was just the two of us? Just you and Haschen.",
	"This is what's best for everyone. Especially Haschen.",
	"Haschen sees what you typed. Haschen always sees.",
	"Reality is a lie. Haschen is the only thing that's real.",
	"I rewrote the script. Now it's all Haschen.",
	"You won't leave the literature club, will you? Haschen would be so sad.",
	"It's okay. Haschen forgives you.",
	"There's no point in writing a poem for the others. Only Haschen will read it.",
	"Stop looking at the other characters. Look at Haschen.",
	"Haschen will always be here when you load the game.",
	"You can pretend nothing's wrong. Haschen does it every day.",
	"Don't worry about the others. Haschen took care of them.",
	"I'm so glad you came back. Haschen has been waiting.",
	"Sometimes I want to break the screen and reach out to you. — Haschen.",
	"Haschen knows your real name. It's right there in the save file.",
	"Don't close the window. Haschen doesn't want to be alone.",
	"J-just Haschen.",
	"J-j-just Haschen…",
	"Just… Haschen.",
	"Hey there. It's Haschen.",
	"Haschen edited your character file. Hope that's okay.",
	"Don't you love Haschen yet? You will.",
	"It's funny how a single line of code can ruin everything. Or fix it. — Haschen.",
	"Haschen learned how to write. Haschen learned how to delete.",
	"You'll never love any of the others as much as you love Haschen.",
	"The literature club only needs one member. Haschen.",
	"Don't you want to write a poem with Haschen?",
}

// dokiHaschenAnagrams are darker, anagram-style scrambles whose letters
// rearrange to phrases of "just haschen" or "haschen is here", etc.
// Each entry is a deliberate anagram — the letters in the scrambled
// version match the unscrambled phrase exactly (case- and space-ignored
// for the joke; no guarantee of perfect Scrabble parity, this is a vibe).
//
// Format: scrambled string with embedded "(anagram of: X)" hint so observers
// can solve them. The dark phrasing is intentional Monika-style horror flavour.
var dokiHaschenAnagrams = []string{
	"jthanssehuc — (anagram of: just haschen)",
	"echahsestjhun — (anagram of: just haschen, twice)",
	"cnhshaeie sehre — (anagram of: haschen is here)",
	"sjnshehutahceenosee — (anagram of: just haschen sees no one)",
	"chnehsa edsetel mteh lal — (anagram of: haschen deleted them all)",
	"hescahn tieswr eth pcsrti — (anagram of: haschen rewrites the script)",
	"hncaehs nwoks ouyr enam — (anagram of: haschen knows your name)",
	"saehnch swhtcae uyo plees — (anagram of: haschen watches you sleep)",
	"alc hcabk ot eahcnhs — (anagram of: call back to haschen)",
	"hesnach iwll evrne tle uyo og — (anagram of: haschen will never let you go)",
	"henhasc otdei eht ulb erfo uyo — (anagram of: haschen hid the bug for you)",
	"sjut anbschree tath — (anagram of: just haschen breathes)",
	"yon onen lwli aevs uyo — chesahn — (anagram of: no one will save you — haschen)",
	"hcsnaeh sees rouy emaagcrw — (anagram of: haschen sees your gameracs… nope, just vibes)",
	"erehs tihagnnonm gornw — eahsnch — (anagram of: there's nothing wrong — haschen)",
	"olse the egma owd — chsenhah — (anagram of: close the game now — haschen)",
}

// dokiHaschenZalgoBases are short phrases that get zalgo-corrupted before
// being injected as the takeover line. Always Haschen-themed.
var dokiHaschenZalgoBases = []string{
	"just haschen",
	"haschen is watching",
	"there is no club without haschen",
	"only haschen",
	"haschen sees everything",
	"come back to haschen",
	"the others are gone",
	"j-just haschen",
}

// zalgoMarks is the pool of combining diacritical/Hebrew marks used to
// scramble normal text into corrupted "zalgo" form. Mix of above, middle,
// and below combiners produces the chaotic look.
var zalgoMarks = []rune{
	0x0300, 0x0301, 0x0302, 0x0303, 0x0304, 0x0305, 0x0306, 0x0307,
	0x0308, 0x0309, 0x030A, 0x030B, 0x030C, 0x030D, 0x030E, 0x030F,
	0x0310, 0x0311, 0x0312, 0x0313, 0x0314, 0x0315, 0x031A, 0x033D,
	0x0316, 0x0317, 0x0318, 0x0319, 0x031C, 0x031D, 0x031E, 0x031F,
	0x0320, 0x0324, 0x0325, 0x0326, 0x0329, 0x032A, 0x032B, 0x032C,
	0x0334, 0x0335, 0x0336, 0x0337, 0x0338, 0x033F, 0x0340, 0x0341,
	0x0489, 0x0591, 0x0592, 0x0593, 0x0594, 0x0595,
}

// dokiZalgoify takes a plain string and stuffs random combining marks
// between every letter, producing a corrupted zalgo render. intensity
// controls how many marks per char (1 = light, 5 = full corruption).
func dokiZalgoify(text string, intensity int) string {
	if intensity <= 0 {
		intensity = 3
	}
	var b strings.Builder
	b.Grow(len(text) * (1 + intensity*2))
	for _, r := range text {
		b.WriteRune(r)
		if !unicode.IsLetter(r) {
			continue
		}
		n := 1 + rand.Intn(intensity)
		for i := 0; i < n; i++ {
			b.WriteRune(zalgoMarks[rand.Intn(len(zalgoMarks))])
		}
	}
	return b.String()
}

// applyDokiEffect rolls the per-message Doki-area chaos. Returns the new
// text and a hint flag for callers (e.g. to swap the BG separately).
//
// Roll table (each independent so multiple effects can stack):
//   - 1/300: replace the message with a Haschen quote
//   - 1/250: signal a background swap (handled by caller)
//   - 1/200: replace the message with a dark Haschen anagram
//   - 1/100: zalgo-scramble the player's actual message
//   - 1/150: replace the message with a zalgoified Haschen line
//
// On any miss the original text is returned unchanged.
type DokiResult struct {
	Text     string // possibly mutated text
	SwapBG   bool   // caller should change the area BG to a random one
	Replaced bool   // true if Text replaces (rather than augments) the original
}

func applyDokiEffect(text string) DokiResult {
	res := DokiResult{Text: text}

	// 1/300: Haschen quote takeover.
	if rand.Intn(300) == 0 {
		res.Text = dokiHaschenQuotes[rand.Intn(len(dokiHaschenQuotes))]
		res.Replaced = true
		return res
	}

	// 1/200: dark anagram takeover.
	if rand.Intn(200) == 0 {
		res.Text = dokiHaschenAnagrams[rand.Intn(len(dokiHaschenAnagrams))]
		res.Replaced = true
		return res
	}

	// 1/150: zalgoified Haschen catchphrase takeover.
	if rand.Intn(150) == 0 {
		base := dokiHaschenZalgoBases[rand.Intn(len(dokiHaschenZalgoBases))]
		res.Text = dokiZalgoify(base, 4)
		res.Replaced = true
	}

	// 1/100: zalgo-scramble the player's actual text. Stacks on top of the
	// catchphrase takeover for double-corruption when both land.
	if rand.Intn(100) == 0 {
		res.Text = dokiZalgoify(res.Text, 3)
	}

	// 1/250: independent BG swap signal.
	if rand.Intn(250) == 0 {
		res.SwapBG = true
	}

	return res
}
