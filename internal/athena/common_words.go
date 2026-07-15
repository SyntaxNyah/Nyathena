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
	"bufio"
	_ "embed"
	"strings"
)

// commonWordsRaw embeds a list of the ~10,000 most frequent English words
// (github.com/first20hours/google-10000-english, swear-word-filtered
// upstream) used only to sanity-check banned-word/censored-name entries at
// load time — see collidesWithCommonWords.
//
//go:embed data/common_words.txt
var commonWordsRaw string

var commonWords []string

func init() {
	scanner := bufio.NewScanner(strings.NewReader(commonWordsRaw))
	for scanner.Scan() {
		w := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if w != "" {
			commonWords = append(commonWords, w)
		}
	}
}

// maxCommonWordCollisions caps how many colliding words collidesWithCommonWords
// reports, so a warning log line stays readable even for a wildly overbroad entry.
const maxCommonWordCollisions = 5

// collidesWithCommonWords reports common English words that contain
// normalized as a substring (and are not normalized itself). A hit means the
// entry would also match that everyday word inside ordinary chat — e.g. an
// entry that normalizes to "tron" collides with "electronic", "strong",
// "astronomy", ... A non-empty result means the entry is too broad to load
// safely, regardless of how it ended up that short (digit-drop, leetspeak
// substitution, or a deliberately short entry).
func collidesWithCommonWords(normalized string) []string {
	var hits []string
	for _, w := range commonWords {
		if w == normalized {
			continue // the entry equals a real word; not a collateral match
		}
		if strings.Contains(w, normalized) {
			hits = append(hits, w)
			if len(hits) >= maxCommonWordCollisions {
				break
			}
		}
	}
	return hits
}
