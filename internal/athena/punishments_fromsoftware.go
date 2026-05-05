/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: /fromsoftware punishment.

   Loads a word list from config/fromsoft.txt at startup. While the
   punishment is active every word a player sends that matches an entry in
   the list (whole-word, case-insensitive) is replaced with *** in the IC
   message visible to the area. */

package athena

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

// applyFromSoftware replaces each word that appears in the fromsoftWords list
// with ***. Matching is whole-word and case-insensitive; surrounding
// punctuation (e.g. commas, exclamation marks) is preserved on the token but
// stripped before the lookup so "bum!" also triggers a match for "bum".
func applyFromSoftware(text string) string {
	if len(fromsoftWords) == 0 {
		return text
	}

	words := strings.Fields(text)
	for i, w := range words {
		// Strip surrounding non-letter / non-digit runes to get the bare word.
		core := strings.ToLower(strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}))
		if _, ok := fromsoftWords[core]; !ok {
			continue
		}
		// Locate the core substring in the original token and overwrite it
		// with *** while keeping any flanking punctuation intact.
		lower := strings.ToLower(w)
		if idx := strings.Index(lower, core); idx >= 0 {
			words[i] = w[:idx] + "***" + w[idx+len(core):]
		}
	}
	return truncateText(strings.Join(words, " "))
}
