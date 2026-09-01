// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Severity tiers for the word lists.
//
// banned_words.txt used to be flat: every entry meant the same thing, and that
// one thing was whatever automod_action said. Two problems followed from it.
//
// First, an operator had no way to say "this word in particular is not a
// borderline call". A slur that should end a connection outright got the same
// treatment as a mild word somebody might say by accident, so the action had to
// be set for the worst case or the mildest, never both.
//
// Second, the word list could not feed the raid guard. "Said something vile" is
// a genuine behavioural signal -- the 2026-08-31 raid was ten connections whose
// entire contribution was slurs -- but a flat list has no notion of how bad an
// entry is, so there was nothing to score. A tier gives the guard a number.
//
// Tiers, weakest to strongest:
//
//	watch    delivered normally. Scores, and alerts staff. For words that are
//	         not punishable on their own but are worth knowing about, and that
//	         raise the score of a connection already behaving oddly.
//	default  what an unmarked entry has always meant: whatever automod_action
//	         says. Also scores, modestly.
//	severe   the configured action, plus a large raid-guard contribution -- big
//	         enough that one more independent signal reaches a punitive rung.
//	nuke     the message is destroyed before anybody sees it, including the
//	         sender, and the IPID is banned. No score, because scoring something
//	         already banned is pointless.
//
// The tier of a hit is decided by the WORST entry the message matches, never
// the first one found -- a line containing a mild word and a nuke word is a
// nuke.
package athena

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// WordSeverity is how bad one word-list entry is. Ordered, so a message's
// verdict is simply the maximum over the entries it matches.
type WordSeverity uint8

const (
	SeverityWatch WordSeverity = iota
	SeverityDefault
	SeveritySevere
	SeverityNuke
)

func (s WordSeverity) String() string {
	switch s {
	case SeverityWatch:
		return "watch"
	case SeveritySevere:
		return "severe"
	case SeverityNuke:
		return "nuke"
	default:
		return "default"
	}
}

// parseWordSeverity maps the tier written after an entry. An unrecognised tier
// is reported rather than guessed: silently downgrading "nuek" to default would
// leave an operator believing a word is banned when it is not.
func parseWordSeverity(s string) (WordSeverity, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "default", "normal", "ban", "action":
		return SeverityDefault, true
	case "watch", "flag", "score":
		return SeverityWatch, true
	case "severe", "high", "bad":
		return SeveritySevere, true
	case "nuke", "instaban", "instant", "kill":
		return SeverityNuke, true
	}
	return SeverityDefault, false
}

// MatchMode is how an entry's pattern is compared against a message.
//
// The two modes exist because evasion resistance and precision genuinely pull
// against each other, and which one an entry needs depends on the word.
//
// Substring matching runs on normalizeForFilter's output, which throws away
// every non-letter so that "n i g g e r", "n.i.g.g.e.r" and "n1gg3r" all
// collapse onto the same needle. That is what makes the filter hard to slip
// past -- and it is also why word boundaries do not exist by the time matching
// happens, so "rape" matches inside "therapeutic". loadWordListFile rejects
// entries like that outright, which means a word an operator obviously wants
// blocked can be silently unavailable to them.
//
// Word matching runs on a boundary-preserving normalization instead: the same
// case folding, confusable folding and leetspeak substitution, but separator
// runs become a single space rather than vanishing. "rape" then matches "rape"
// and "raping" but not "therapeutic". The cost is that spacing evasion works
// against it, which is the right trade only for entries that are too generic to
// match as a substring at all.
type MatchMode uint8

const (
	MatchSubstring MatchMode = iota
	MatchWord
)

func (m MatchMode) String() string {
	if m == MatchWord {
		return "word"
	}
	return "substring"
}

// parseMatchMode maps the match mode written after a tier.
func parseMatchMode(s string) (MatchMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "substring", "sub", "contains":
		return MatchSubstring, true
	case "word", "wholeword", "exact", "boundary":
		return MatchWord, true
	}
	return MatchSubstring, false
}

// WordEntry is one parsed word-list line.
type WordEntry struct {
	Raw      string // as the operator wrote it, for logs and staff alerts
	Pattern  string // normalized form actually compared against a message
	Severity WordSeverity
	Mode     MatchMode
}

func (e WordEntry) String() string {
	return fmt.Sprintf("%s (%s/%s)", e.Raw, e.Severity, e.Mode)
}

// minWordModeEntryLen is the shortest boundary-matched entry accepted.
//
// Lower than minNormalizedEntryLen (4) because the two floors are guarding
// against different things. The substring floor exists because a short needle
// matches inside unrelated words; a boundary-matched needle cannot, so the only
// risk left is that the entry IS a short ordinary word, which an operator can
// see for themselves. Three keeps genuinely tiny needles out without blocking a
// legitimate short slur.
const minWordModeEntryLen = 3

// normalizeForFilterBoundaries is normalizeForFilter with word boundaries kept.
//
// Identical rune-by-rune handling -- NFKD, combining marks stripped, confusables
// and leetspeak substituted, 3+ runs collapsed to 2 -- except that a run of
// non-letters emits one space instead of being dropped. The result is a
// space-separated sequence of normalized words that MatchWord can scan for
// whole tokens.
func normalizeForFilterBoundaries(s string) string {
	s = norm.NFKD.String(s)

	out := make([]rune, 0, len(s))
	streak := 0
	pendingSpace := false
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue // combining marks (accents, zalgo) never break a word
		}
		r = unicode.ToLower(r)
		if repl, ok := charSubstitutions[r]; ok {
			r = repl
		}
		if !unicode.IsLetter(r) {
			// A separator run becomes at most one boundary. Deferred rather
			// than emitted immediately so trailing separators never leave a
			// dangling space, and so a run of them counts once.
			if len(out) > 0 {
				pendingSpace = true
			}
			streak = 0
			continue
		}
		if pendingSpace {
			out = append(out, ' ')
			pendingSpace = false
			streak = 0
		}
		if n := len(out); n > 0 && out[n-1] == r {
			streak++
			if streak >= 2 {
				continue // same 3+-run collapse normalizeForFilter applies
			}
		} else {
			streak = 0
		}
		out = append(out, r)
	}
	return string(out)
}

// matchesWordBoundary reports whether pattern appears in the
// boundary-normalized text as a whole word or as the start of one.
//
// Prefix-of-a-word rather than strictly equal, which covers the inflections that
// keep the stem intact -- "rape" also catches "rapes" and "raped" -- but NOT the
// ones that change it: "raping" does not begin with "rape", and "trannies" does
// not begin with "tranny", so forms like those still need their own entry. This
// mode narrows what matches; it is not a stemmer, and an operator should list
// the forms they care about rather than assume one entry covers a whole word
// family.
//
// It deliberately does not match mid-token or as a suffix, since mid-token is
// where the collisions live: "therapeutic" contains "rape" only in the middle,
// and excluding that is the entire reason this mode exists.
func matchesWordBoundary(text, pattern string) bool {
	if pattern == "" {
		return false
	}
	for _, tok := range strings.Fields(text) {
		if strings.HasPrefix(tok, pattern) {
			return true
		}
	}
	return false
}

// WordListMatch is the outcome of checking one message against a word list.
type WordListMatch struct {
	Entry   WordEntry
	Matched bool
}

// matchWordEntries returns the WORST-severity entry the text matches.
//
// Worst rather than first, and this is the whole point of the type: a raider
// who pads a slur with mild words must not be handed the mild verdict because
// it happened to be checked earlier. Entries arrive grouped by descending
// severity (see buildWordEntries), so the scan returns as soon as a tier
// produces a hit -- the common case, a clean message, still costs one full pass
// but a nuke is found without examining anything gentler.
//
// Both normalizations are computed at most once per call and only when a mode
// that needs them is actually present, so a list with no word-mode entries
// never pays for the boundary pass.
func matchWordEntries(entries []WordEntry, text string) WordListMatch {
	if len(entries) == 0 || text == "" {
		return WordListMatch{}
	}
	var sub, bound string
	var subDone, boundDone bool

	best := WordListMatch{}
	for _, e := range entries {
		var hit bool
		switch e.Mode {
		case MatchWord:
			if !boundDone {
				bound, boundDone = normalizeForFilterBoundaries(text), true
			}
			hit = matchesWordBoundary(bound, e.Pattern)
		default:
			if !subDone {
				sub, subDone = normalizeForFilter(text), true
			}
			hit = strings.Contains(sub, e.Pattern)
		}
		if !hit {
			continue
		}
		if !best.Matched || e.Severity > best.Entry.Severity {
			best = WordListMatch{Entry: e, Matched: true}
		}
		if best.Entry.Severity == SeverityNuke {
			return best // nothing can outrank a nuke; stop looking
		}
	}
	return best
}
