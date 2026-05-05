/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: /fromsoftware punishment.

   Loads a word list from config/fromsoft.txt at startup. While the
   punishment is active, every occurrence of a listed word as a substring of
   any token the player sends is replaced with one asterisk per letter — so
   "ho" in the list will censor "should" → "s**uld", "how" → "**w", etc.
   That's the joke: it's an overzealous censor. */

package athena

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// fromsoftWords holds the lowercased words loaded from config/fromsoft.txt.
// Stored as a set for O(1) lookup on every IC message. Populated once at
// startup by initFromSoftWords; never modified afterwards (read-only in the
// hot path).
var fromsoftWords map[string]struct{}

// initFromSoftWords reads config/fromsoft.txt and populates fromsoftWords.
// Each non-empty, non-comment (lines starting with '#') line is treated as
// one word. The file is optional: if it is absent the punishment becomes a
// no-op and a warning is logged so operators are aware.
func initFromSoftWords() {
	path := filepath.Join(settings.ConfigPath, "fromsoft.txt")
	f, err := os.Open(path)
	if err != nil {
		// Not a fatal error — the file is optional. Log at warning level so
		// operators notice when /fromsoftware is used but the list is missing.
		logger.LogWarningf("fromsoftware: fromsoft.txt not found at %q — /fromsoftware will be a no-op", path)
		fromsoftWords = map[string]struct{}{}
		return
	}
	defer f.Close()

	words := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		words[strings.ToLower(line)] = struct{}{}
	}
	fromsoftWords = words
	logger.LogInfof("fromsoftware: loaded %d word(s) from %q", len(words), path)
}

// applyFromSoftware replaces every occurrence of a fromsoftWords entry as a
// substring within each space-separated token with one asterisk per rune.
// Matching is case-insensitive. Longer entries are applied before shorter ones
// so that a two-letter entry cannot split a region already blanked by a longer
// entry. The replacement is intentionally substring-level — entries like "ho"
// will censor "should" → "s**uld", "how" → "**w", etc.
func applyFromSoftware(text string) string {
	if len(fromsoftWords) == 0 {
		return text
	}

	// Build a slice of bad words sorted longest-first so that a longer match
	// takes precedence over a shorter one that overlaps it.
	sorted := make([]string, 0, len(fromsoftWords))
	for w := range fromsoftWords {
		sorted = append(sorted, w)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	tokens := strings.Fields(text)
	for i, tok := range tokens {
		lower := strings.ToLower(tok)
		for _, bad := range sorted {
			stars := strings.Repeat("*", utf8.RuneCountInString(bad))
			// Replace all non-overlapping occurrences of bad within this token.
			for {
				idx := strings.Index(lower, bad)
				if idx < 0 {
					break
				}
				tok = tok[:idx] + stars + tok[idx+len(bad):]
				lower = lower[:idx] + stars + lower[idx+len(bad):]
			}
		}
		tokens[i] = tok
	}
	return truncateText(strings.Join(tokens, " "))
}
