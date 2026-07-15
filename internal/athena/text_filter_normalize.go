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
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// charSubstitutions maps characters commonly used to evade word filters back
// to the Latin letter they are meant to impersonate:
//   - non-Latin homoglyphs (Cyrillic, Greek, Armenian, Cherokee) that render
//     almost identically to a Latin letter but are canonically distinct
//     codepoints, so NFKD compatibility decomposition alone will not fold
//     them. Verified against Unicode's confusables.txt (the same data UTS #39
//     security profiles are built from), not hand-guessed.
//   - leetspeak digit/symbol substitutions ("n1gg3r", "$h1t"). Only the
//     small set of unambiguous, high-confidence substitutions is included
//     (e.g. not "2", "6", "9", which collide too often with ordinary
//     numbers in chat).
//
// Keys are lowercase; normalizeForFilter case-folds before looking up, so a
// single lowercase entry also covers the uppercase source character.
var charSubstitutions = map[rune]rune{
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
	'\u0433': 'r', // CYRILLIC SMALL LETTER GHE
	'\u0448': 'w', // CYRILLIC SMALL LETTER SHA
	'\u044c': 'b', // CYRILLIC SMALL LETTER SOFT SIGN
	'\u0461': 'w', // CYRILLIC SMALL LETTER OMEGA
	'\u0475': 'v', // CYRILLIC SMALL LETTER IZHITSA
	'\u04af': 'y', // CYRILLIC SMALL LETTER STRAIGHT U
	'\u04bb': 'h', // CYRILLIC SMALL LETTER SHHA
	'\u04bd': 'e', // CYRILLIC SMALL LETTER ABKHASIAN CHE
	'\u04cf': 'l', // CYRILLIC SMALL LETTER PALOCHKA
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
	'\u03c3': 'o', // GREEK SMALL LETTER SIGMA
	'\u03ed': 'o', // COPTIC SMALL LETTER SHIMA
	'\u03f1': 'p', // GREEK RHO SYMBOL
	'\u03f2': 'c', // GREEK LUNATE SIGMA SYMBOL
	'\u03f3': 'j', // GREEK LETTER YOT
	'\u03f8': 'p', // GREEK SMALL LETTER SHO
	// Armenian
	'\u0561': 'w', // ARMENIAN SMALL LETTER AYB
	'\u0563': 'q', // ARMENIAN SMALL LETTER GIM
	'\u0566': 'q', // ARMENIAN SMALL LETTER ZA
	'\u0570': 'h', // ARMENIAN SMALL LETTER HO
	'\u0575': 'j', // ARMENIAN SMALL LETTER YI
	'\u0578': 'n', // ARMENIAN SMALL LETTER VO
	'\u057c': 'n', // ARMENIAN SMALL LETTER RA
	'\u057d': 'u', // ARMENIAN SMALL LETTER SEH
	'\u0581': 'g', // ARMENIAN SMALL LETTER CO
	'\u0582': 'i', // ARMENIAN SMALL LETTER YIWN
	'\u0584': 'f', // ARMENIAN SMALL LETTER KEH
	'\u0585': 'o', // ARMENIAN SMALL LETTER OH
	// Cherokee (syllabary letters based on Latin shapes)
	'\uab75': 'i', // CHEROKEE SMALL LETTER V
	'\uab81': 'r', // CHEROKEE SMALL LETTER HU
	'\uab83': 'w', // CHEROKEE SMALL LETTER LA
	'\uab92': 'h', // CHEROKEE SMALL LETTER NI
	'\uab93': 'z', // CHEROKEE SMALL LETTER NO
	'\uab9f': 'b', // CHEROKEE SMALL LETTER SI
	'\uaba4': 'w', // CHEROKEE SMALL LETTER TA
	'\uaba9': 'v', // CHEROKEE SMALL LETTER DO
	'\uabaa': 's', // CHEROKEE SMALL LETTER DU
	'\uabaf': 'c', // CHEROKEE SMALL LETTER TLI
	'\uabb7': 'd', // CHEROKEE SMALL LETTER TSU
	// Leetspeak digit/symbol substitutions
	'0': 'o', // digit zero
	'1': 'i', // digit one
	'3': 'e', // digit three
	'4': 'a', // digit four
	'5': 's', // digit five
	'7': 't', // digit seven
	'8': 'b', // digit eight
	'@': 'a', // at sign
	'$': 's', // dollar sign
	'!': 'i', // exclamation mark
}

// normalizeForFilter prepares text for banned-word/censored-name matching so
// that it survives common filter-evasion tricks:
//   - stylized Unicode letters (mathematical bold/script/fraktur, fullwidth,
//     circled, superscript, ...) are folded back to plain Latin via NFKD
//     compatibility decomposition
//   - combining marks (accents, "zalgo" corruption) are stripped
//   - non-Latin homoglyphs and leetspeak substitutions are folded to their
//     Latin letter via charSubstitutions
//   - anything left that is not a letter (whitespace, punctuation, zero-width
//     /format characters, unmapped digits and symbols) is dropped entirely,
//     which defeats spacing/punctuation insertion ("n i g g e r",
//     "n.i.g.g.e.r") the same way it defeats invisible zero-width
//     characters slipped between letters
//   - consecutive duplicate letters collapse to one, which defeats letter
//     stuffing ("niggggger") while staying symmetric with ordinary double
//     letters, since the same collapsing is applied to both the banned-word
//     list and the text being checked
//
// Dropping all non-letters and collapsing repeats means matching is no
// longer word-boundary-aware: this trades a wider false-positive surface
// (a banned word could in principle span text that used to be separated by
// punctuation, or a short entry could match inside more unrelated words than
// before) for closing off spacing/leetspeak/repetition evasion. Substring
// matching already carried this class of risk (e.g. "ass" inside "class");
// this extends it rather than introducing a new category. Keep banned-word
// entries as specific as practical to limit collateral matches.
//
// Both the banned-word/censored-name lists and the text being checked are
// run through this function, so matching stays symmetric.
func normalizeForFilter(s string) string {
	s = norm.NFKD.String(s)

	letters := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue // strip combining marks (accents, zalgo)
		}
		r = unicode.ToLower(r)
		if repl, ok := charSubstitutions[r]; ok {
			r = repl
		}
		if !unicode.IsLetter(r) {
			continue // drop separators: whitespace, punctuation, digits, symbols, zero-width/format chars
		}
		if n := len(letters); n > 0 && letters[n-1] == r {
			continue // collapse consecutive duplicate letters (letter-stuffing evasion)
		}
		letters = append(letters, r)
	}
	return string(letters)
}
