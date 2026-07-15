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
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// zeroWidthReplacer strips invisible formatting characters that are commonly
// inserted between letters to defeat substring matching (e.g. a zero-width
// space typed between every letter of a slur so it renders unchanged but no
// longer contains the banned substring).
var zeroWidthReplacer = strings.NewReplacer(
	"\u00ad", "", // soft hyphen
	"\u200b", "", // zero width space
	"\u200c", "", // zero width non-joiner
	"\u200d", "", // zero width joiner
	"\u2060", "", // word joiner
	"\ufeff", "", // BOM / zero width no-break space
)

// homoglyphs maps common non-Latin lookalike letters (Cyrillic, Greek) to
// their Latin equivalents. Unlike stylized Unicode letters (mathematical,
// fullwidth, circled, ...), these do not have a Unicode compatibility
// decomposition to Latin: they are canonically distinct letters that just
// happen to render almost identically, so NFKD alone will not catch them.
// Keys are lowercase; normalizeForFilter case-folds before looking up.
var homoglyphs = map[rune]rune{
	// Cyrillic
	'\u0430': 'a', // CYRILLIC SMALL LETTER A
	'\u0435': 'e', // CYRILLIC SMALL LETTER IE
	'\u0456': 'i', // CYRILLIC SMALL LETTER BYELORUSSIAN-UKRAINIAN I
	'\u043e': 'o', // CYRILLIC SMALL LETTER O
	'\u0440': 'p', // CYRILLIC SMALL LETTER ER
	'\u0441': 'c', // CYRILLIC SMALL LETTER ES
	'\u0443': 'y', // CYRILLIC SMALL LETTER U
	'\u0445': 'x', // CYRILLIC SMALL LETTER HA
	'\u043a': 'k', // CYRILLIC SMALL LETTER KA
	'\u043c': 'm', // CYRILLIC SMALL LETTER EM
	'\u043d': 'h', // CYRILLIC SMALL LETTER EN
	'\u0442': 't', // CYRILLIC SMALL LETTER TE
	'\u0455': 's', // CYRILLIC SMALL LETTER DZE
	'\u0458': 'j', // CYRILLIC SMALL LETTER JE
	// Greek
	'\u03b1': 'a', // GREEK SMALL LETTER ALPHA
	'\u03b2': 'b', // GREEK SMALL LETTER BETA
	'\u03b5': 'e', // GREEK SMALL LETTER EPSILON
	'\u03b9': 'i', // GREEK SMALL LETTER IOTA
	'\u03ba': 'k', // GREEK SMALL LETTER KAPPA
	'\u03bf': 'o', // GREEK SMALL LETTER OMICRON
	'\u03c1': 'p', // GREEK SMALL LETTER RHO
	'\u03c4': 't', // GREEK SMALL LETTER TAU
	'\u03c5': 'u', // GREEK SMALL LETTER UPSILON
	'\u03bd': 'v', // GREEK SMALL LETTER NU
	'\u03c7': 'x', // GREEK SMALL LETTER CHI
	'\u03b3': 'y', // GREEK SMALL LETTER GAMMA
}

// normalizeForFilter prepares text for banned-word/censored-name matching so
// that it survives common filter-evasion tricks:
//   - stylized Unicode letters (mathematical bold/script/fraktur, fullwidth,
//     circled, superscript, ...) are folded back to plain Latin via NFKD
//     compatibility decomposition
//   - zero-width characters inserted mid-word are stripped
//   - combining marks (accents, "zalgo" corruption) are stripped
//   - common Cyrillic/Greek homoglyphs are folded to their Latin lookalike
//
// Both the banned-word/censored-name lists and the text being checked are
// run through this function, so matching stays symmetric.
func normalizeForFilter(s string) string {
	s = zeroWidthReplacer.Replace(s)
	s = norm.NFKD.String(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue // strip combining marks (accents, zalgo)
		}
		r = unicode.ToLower(r)
		if repl, ok := homoglyphs[r]; ok {
			r = repl
		}
		b.WriteRune(r)
	}
	return b.String()
}
