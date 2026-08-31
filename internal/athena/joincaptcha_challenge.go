package athena

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// This file builds the challenges used by the join captcha (see joincaptcha.go).
//
// Design goal: a challenge must be trivial for a human and awkward for a
// throwaway script. The naive form -- "please type ABC123 to verify" -- fails
// that immediately: an attacker never has to understand the message, they just
// regex out whatever follows "please type" and echo it back. Randomising the
// token accomplishes nothing, because the token is right there in the prompt.
//
// So every generator here obeys one rule, enforced at generation time by
// newJoinChallenge below rather than merely intended:
//
//	THE ANSWER IS NEVER A CONTIGUOUS SUBSTRING OF THE PROMPT.
//
// The answer always has to be *derived* -- by adding two numbers written as
// words, reversing a token, picking non-adjacent characters out of one,
// counting things, continuing a sequence. There is nothing in the text to copy,
// so a scraper has to actually implement the transformation, and there are
// joinChallengeKinds of them phrased joinChallengePhrasings different ways.
//
// The kind pool is configurable (join_captcha_kinds) precisely so that if some
// group does invest in a solver for the current mix, the operator can rotate to
// a different subset from config without a rebuild, and their work is wasted.

// joinChallenge is a single generated captcha: the text shown to the player and
// the set of answers that satisfy it. Multiple answers exist so that a numeric
// challenge accepts both "12" and "twelve".
type joinChallenge struct {
	Kind    string   // generator id, e.g. "math" -- surfaced to staff, never to the player
	Prompt  string   // the question shown in OOC
	Hint    string   // randomly-phrased "reply with /verify ..." line
	Answers []string // accepted answers, already normalized; empty in plugin token mode
	// PluginToken is opaque state handed back to the captcha plugin on verify.
	// Non-empty only for a plugin challenge that withheld its answers, which is
	// the mode where the answer never exists inside this process.
	PluginToken string
}

// Every generator below draws from a *captchaRand rather than a package-level
// RNG. That source is either the keyed HMAC stream derived from the server's
// secret (the normal case -- see joincaptcha_keyed.go for why that matters) or
// crypto/rand when keying is switched off. math/rand is never used: its output
// is recoverable from a handful of observed values, and a captcha's randomness
// is adversarial input.

// captchaPick returns a random element of s. Returns the zero value for an
// empty slice so callers never have to guard the corpora.
func captchaPick[T any](r *captchaRand, s []T) T {
	var zero T
	if len(s) == 0 {
		return zero
	}
	return s[r.intn(len(s))]
}

// captchaShuffle returns a shuffled copy of s (Fisher-Yates).
func captchaShuffle[T any](r *captchaRand, s []T) []T {
	out := make([]T, len(s))
	copy(out, s)
	for i := len(out) - 1; i > 0; i-- {
		j := r.intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// normalizeCaptchaAnswer reduces a string to comparable form: lowercase, with
// every character that isn't a letter or digit removed. This is what makes the
// captcha forgiving of the ways a real person types an answer -- "A7K Q2M",
// "a7k-q2m" and "a7kq2m" all match, as do "/verify 12." and "12".
func normalizeCaptchaAnswer(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// captchaAlphabet is the character pool for generated tokens. Visually
// ambiguous characters are excluded on purpose -- 0/O, 1/l/I, 5/S, 2/Z -- so a
// player never fails because of the font their client renders OOC in.
const captchaAlphabet = "ABCDEFGHJKMNPQRTUVWXY346789"

// captchaToken returns a random token of n characters from captchaAlphabet.
func captchaToken(r *captchaRand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = captchaAlphabet[r.intn(len(captchaAlphabet))]
	}
	return string(b)
}

// presentTokenLoose renders a token in one of several shapes -- plain, dashed,
// spaced, or mixed case. Answers are compared through normalizeCaptchaAnswer,
// which strips separators and case, so every shape reads the same to a person
// but none of them is the same string to a scraper looking for a token of a
// fixed form. Safe only where the separators cannot change the meaning of the
// question (reversing, joining) -- see presentTokenTight for the rest.
func presentTokenLoose(r *captchaRand, tok string) string {
	switch r.intn(4) {
	case 0:
		return tok
	case 1:
		return strings.Join(strings.Split(tok, ""), "-")
	case 2:
		return strings.Join(strings.Split(tok, ""), " ")
	default:
		return mixCase(r, tok)
	}
}

// presentTokenTight renders a token without inserting separators, for
// questions that ask about character positions -- "the 3rd character" has to
// stay unambiguous, so only the casing varies.
func presentTokenTight(r *captchaRand, tok string) string {
	if r.intn(2) == 0 {
		return tok
	}
	return mixCase(r, tok)
}

// mixCase randomises the case of each letter in s.
func mixCase(r *captchaRand, s string) string {
	runes := []rune(strings.ToLower(s))
	for i, c := range runes {
		if unicode.IsLetter(c) && r.intn(2) == 0 {
			runes[i] = unicode.ToUpper(c)
		}
	}
	return string(runes)
}

// numberWords spells 0-20 so a challenge can state its operands in words. A
// prompt that contains no digits at all cannot be attacked by "grab the numbers
// and try the four operators on them".
var numberWords = []string{
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
	"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen",
	"eighteen", "nineteen", "twenty",
}

// spellNumber returns the English word for n when it is small enough to have a
// single-word name, else the empty string. Used to accept "twelve" alongside
// "12"; callers drop the empty result.
func spellNumber(n int) string {
	if n >= 0 && n < len(numberWords) {
		return numberWords[n]
	}
	switch n {
	case 30:
		return "thirty"
	case 40:
		return "forty"
	case 50:
		return "fifty"
	case 60:
		return "sixty"
	case 70:
		return "seventy"
	case 80:
		return "eighty"
	case 90:
		return "ninety"
	}
	return ""
}

// numericAnswers returns the accepted forms of a numeric answer: the digits,
// plus the English word when one exists.
func numericAnswers(n int) []string {
	out := []string{fmt.Sprintf("%d", n)}
	if w := spellNumber(n); w != "" {
		out = append(out, w)
	}
	return out
}

// ordinal renders 1 as "1st", 2 as "2nd" and so on, for challenges that ask for
// a position within a token.
func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// joinList renders items as "a, b and c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// --- corpora -----------------------------------------------------------------

// captchaWords are ordinary words used by the acronym, letter-count and
// word-length challenges. Kept common and unambiguously spelled so that a
// non-native speaker is not disadvantaged.
var captchaWords = []string{
	"apple", "river", "candle", "garden", "pencil", "window", "silver", "orange",
	"pocket", "ladder", "planet", "rocket", "button", "guitar", "hammer", "island",
	"jacket", "kitten", "lemon", "monkey", "napkin", "pillow", "rabbit", "saddle",
	"tunnel", "violet", "walnut", "yellow", "anchor", "basket", "carpet", "dolphin",
	"engine", "forest", "gravel", "harbour", "helmet", "jungle", "kettle", "lantern",
	"marble", "needle", "outfit", "parrot", "quartz", "ribbon", "summer", "temple",
	"velvet", "wagon", "zebra", "bottle", "cactus", "diamond", "eagle", "feather",
}

// natoAlphabet maps a spelling word to the letter it stands for. The player
// only has to take first letters, so knowing the real NATO alphabet is not
// required -- but a scraper still cannot find the answer in the text.
var natoAlphabet = map[string]byte{
	"alpha": 'A', "bravo": 'B', "charlie": 'C', "delta": 'D', "echo": 'E',
	"foxtrot": 'F', "golf": 'G', "hotel": 'H', "india": 'I', "juliet": 'J',
	"kilo": 'K', "lima": 'L', "mike": 'M', "november": 'N', "oscar": 'O',
	"papa": 'P', "quebec": 'Q', "romeo": 'R', "sierra": 'S', "tango": 'T',
	"uniform": 'U', "victor": 'V', "whiskey": 'W', "xray": 'X', "yankee": 'Y',
	"zulu": 'Z',
}

// natoWords is natoAlphabet's key set, held separately so picking is not
// subject to Go's randomised map iteration order (which is not a CSPRNG).
var natoWords = []string{
	"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
	"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
	"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey",
	"xray", "yankee", "zulu",
}

// captchaCategories backs the "how many of these are X" challenge. Counting
// category members needs actual comprehension of each word, which is the one
// thing a pattern-matching scraper cannot fake.
var captchaCategories = []struct {
	name    string
	members []string
	others  []string
}{
	{
		name:    "animals",
		members: []string{"dog", "cat", "horse", "rabbit", "eagle", "dolphin", "monkey", "zebra"},
		others:  []string{"table", "rock", "pencil", "window", "kettle", "jacket", "anchor", "carpet"},
	},
	{
		name:    "colours",
		members: []string{"red", "blue", "green", "yellow", "purple", "orange", "silver", "violet"},
		others:  []string{"ladder", "guitar", "planet", "summer", "hammer", "island", "pillow", "temple"},
	},
	{
		name:    "fruits",
		members: []string{"apple", "banana", "cherry", "lemon", "peach", "melon", "grape", "mango"},
		others:  []string{"rocket", "helmet", "marble", "tunnel", "napkin", "saddle", "engine", "basket"},
	},
	{
		name:    "body parts",
		members: []string{"hand", "elbow", "shoulder", "ankle", "finger", "knee", "wrist", "thumb"},
		others:  []string{"bottle", "forest", "wagon", "ribbon", "gravel", "button", "cactus", "lantern"},
	},
}

// --- generators --------------------------------------------------------------

// joinChallengeGen builds one challenge of a given kind.
type joinChallengeGen struct {
	kind string
	gen  func(r *captchaRand) joinChallenge
}

// joinChallengeGens is the full generator pool. Every entry produces an answer
// that must be computed rather than copied. The kind ids are the values
// accepted by the join_captcha_kinds config key.
var joinChallengeGens = []joinChallengeGen{
	{"math", genChallengeMath},
	{"reverse", genChallengeReverse},
	{"acronym", genChallengeAcronym},
	{"charpick", genChallengeCharPick},
	{"everyother", genChallengeEveryOther},
	{"countletter", genChallengeCountLetter},
	{"category", genChallengeCategory},
	{"sequence", genChallengeSequence},
	{"nato", genChallengeNato},
	{"concat", genChallengeConcat},
	{"wordlength", genChallengeWordLength},
	{"wordorder", genChallengeWordOrder},
	{"sumdigits", genChallengeSumDigits},
	{"alphabet", genChallengeAlphabet},
	{"oddposition", genChallengeOddPosition},
}

// joinChallengeKindList returns every valid kind id, for config validation and
// for the /verify staff-facing help text.
func joinChallengeKindList() []string {
	out := make([]string, 0, len(joinChallengeGens))
	for _, g := range joinChallengeGens {
		out = append(out, g.kind)
	}
	return out
}

// genChallengeMath: arithmetic whose operands are spelled out, so the prompt
// contains no digits to scrape and the result appears nowhere in it.
func genChallengeMath(r *captchaRand) joinChallenge {
	a := 2 + r.intn(9) // 2..10
	b := 2 + r.intn(8) // 2..9
	var result int
	var verb string
	switch r.intn(3) {
	case 0:
		result, verb = a+b, "plus"
	case 1:
		// Order the operands so the answer is never negative.
		if b > a {
			a, b = b, a
		}
		result, verb = a-b, "minus"
	default:
		// Keep the product small enough to stay mental arithmetic.
		b = 2 + r.intn(3) // 2..4
		result, verb = a*b, "times"
	}
	aw, bw := numberWords[a], numberWords[b]
	prompt := captchaPick(r, []string{
		fmt.Sprintf("What is %s %s %s?", aw, verb, bw),
		fmt.Sprintf("Work out %s %s %s.", aw, verb, bw),
		fmt.Sprintf("Take %s and %s it by %s. What do you get?", aw, verb, bw),
		fmt.Sprintf("A quick sum: %s %s %s equals what?", aw, verb, bw),
		fmt.Sprintf("%s %s %s -- what does that come to?", aw, verb, bw),
		fmt.Sprintf("Answer with a number: %s %s %s.", aw, verb, bw),
		fmt.Sprintf("Do the arithmetic -- %s %s %s is how much?", aw, verb, bw),
	})
	return joinChallenge{Kind: "math", Prompt: prompt, Answers: numericAnswers(result)}
}

// genChallengeReverse: the answer is the prompt's token read backwards, so it
// is present only in the wrong order.
func genChallengeReverse(r *captchaRand) joinChallenge {
	tok := captchaToken(r, 4+r.intn(3)) // 4..6
	runes := []rune(tok)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	shown := presentTokenLoose(r, tok)
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Type this backwards: %s", shown),
		fmt.Sprintf("Read %s from right to left and type what you get.", shown),
		fmt.Sprintf("Reverse the order of these characters: %s", shown),
		fmt.Sprintf("%s -- now type it in reverse.", shown),
		fmt.Sprintf("Take %s and write it out back to front.", shown),
		fmt.Sprintf("Last character first: retype %s in the opposite order.", shown),
		fmt.Sprintf("If you spelled %s backwards, what would it be?", shown),
	})
	return joinChallenge{Kind: "reverse", Prompt: prompt, Answers: []string{string(runes)}}
}

// genChallengeAcronym: first letter of each listed word.
func genChallengeAcronym(r *captchaRand) joinChallenge {
	n := 3 + r.intn(2) // 3..4
	words := captchaShuffle(r, captchaWords)[:n]
	var sb strings.Builder
	for _, w := range words {
		sb.WriteByte(w[0])
	}
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Take the first letter of each of these words, in order: %s", joinList(words)),
		fmt.Sprintf("Spell out the initials of: %s", joinList(words)),
		fmt.Sprintf("What do the first letters of %s spell?", joinList(words)),
		fmt.Sprintf("%s -- type just the letter each one starts with.", joinList(words)),
		fmt.Sprintf("Give me the opening letter of every word here, in order: %s", joinList(words)),
		fmt.Sprintf("Reduce each of %s to its first letter and type the result.", joinList(words)),
	})
	return joinChallenge{Kind: "acronym", Prompt: prompt, Answers: []string{sb.String()}}
}

// genChallengeCharPick: characters at named positions. Positions are chosen
// non-adjacent so the answer cannot be a run of the token; newJoinChallenge
// re-rolls in the rare case it still collides.
func genChallengeCharPick(r *captchaRand) joinChallenge {
	tok := captchaToken(r, 6+r.intn(2)) // 6..7
	// Pick 3 ascending positions with a gap of at least 2 between them, which
	// guarantees the picked characters were never contiguous in the token.
	p1 := 1 + r.intn(2)      // 1..2
	p2 := p1 + 2 + r.intn(2) // +2..+3
	p3 := p2 + 2             // +2
	if p3 > len(tok) {
		p3 = len(tok)
	}
	if p2 >= p3 {
		p2 = p3 - 2
	}
	positions := []int{p1, p2, p3}
	var sb strings.Builder
	labels := make([]string, 0, len(positions))
	for _, p := range positions {
		sb.WriteByte(tok[p-1])
		labels = append(labels, ordinal(p))
	}
	shown := presentTokenTight(r, tok)
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Type the %s characters of %s, in that order.", joinList(labels), shown),
		fmt.Sprintf("From %s, take the %s characters and type them together.", shown, joinList(labels)),
		fmt.Sprintf("Look at %s. Type its %s characters, in order.", shown, joinList(labels)),
		fmt.Sprintf("Count along %s and pull out the %s characters.", shown, joinList(labels)),
		fmt.Sprintf("Only the %s characters of %s, please -- in order.", joinList(labels), shown),
		fmt.Sprintf("In %s, which characters sit at the %s positions? Type them together.", shown, joinList(labels)),
	})
	return joinChallenge{Kind: "charpick", Prompt: prompt, Answers: []string{sb.String()}}
}

// genChallengeEveryOther: every second character of a token.
func genChallengeEveryOther(r *captchaRand) joinChallenge {
	tok := captchaToken(r, 6+2*r.intn(2)) // 6 or 8
	fromSecond := r.intn(2) == 0
	start := 0
	if fromSecond {
		start = 1
	}
	var sb strings.Builder
	for i := start; i < len(tok); i += 2 {
		sb.WriteByte(tok[i])
	}
	which := "first"
	if fromSecond {
		which = "second"
	}
	shown := presentTokenTight(r, tok)
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Type every second character of %s, starting with the %s one.", shown, which),
		fmt.Sprintf("Starting at the %s character of %s, take every other one and type the result.", which, shown),
		fmt.Sprintf("From %s, keep the %s character and every other one after it.", shown, which),
		fmt.Sprintf("Drop every other character of %s, keeping the %s one first.", shown, which),
		fmt.Sprintf("Read %s but skip alternate characters, beginning at the %s.", shown, which),
	})
	return joinChallenge{Kind: "everyother", Prompt: prompt, Answers: []string{sb.String()}}
}

// genChallengeCountLetter: how many times a letter occurs across some words.
func genChallengeCountLetter(r *captchaRand) joinChallenge {
	// Retry a few times for a letter that actually occurs, so the answer is
	// rarely the uninteresting "zero".
	var words []string
	var letter byte
	count := 0
	for attempt := 0; attempt < 12; attempt++ {
		words = captchaShuffle(r, captchaWords)[:2+r.intn(2)] // 2..3 words
		joined := strings.Join(words, "")
		letter = joined[r.intn(len(joined))]
		count = strings.Count(joined, string(letter))
		if count >= 1 {
			break
		}
	}
	up := strings.ToUpper(string(letter))
	prompt := captchaPick(r, []string{
		fmt.Sprintf("How many times does the letter %s appear in: %s?", up, joinList(words)),
		fmt.Sprintf("Count the %s's in %s.", up, joinList(words)),
		fmt.Sprintf("In %s, how many letter %s's are there in total?", joinList(words), up),
		fmt.Sprintf("%s -- how many %s's can you find?", joinList(words), up),
		fmt.Sprintf("Tally up every %s across %s.", up, joinList(words)),
		fmt.Sprintf("Give the number of %s's in %s.", up, joinList(words)),
	})
	return joinChallenge{Kind: "countletter", Prompt: prompt, Answers: numericAnswers(count)}
}

// genChallengeCategory: how many listed words belong to a category. Needs
// comprehension of the words, not their shape.
func genChallengeCategory(r *captchaRand) joinChallenge {
	cat := captchaPick(r, captchaCategories)
	nMembers := 2 + r.intn(3) // 2..4
	nOthers := 2 + r.intn(2)  // 2..3
	members := captchaShuffle(r, cat.members)[:nMembers]
	others := captchaShuffle(r, cat.others)[:nOthers]
	all := captchaShuffle(r, append(append([]string{}, members...), others...))
	prompt := captchaPick(r, []string{
		fmt.Sprintf("How many of these are %s: %s?", cat.name, joinList(all)),
		fmt.Sprintf("Count the %s in this list: %s.", cat.name, joinList(all)),
		fmt.Sprintf("Out of %s -- how many are %s?", joinList(all), cat.name),
		fmt.Sprintf("%s. How many of those words are %s?", joinList(all), cat.name),
		fmt.Sprintf("Ignore everything that isn't one of the %s, then count what's left: %s.", cat.name, joinList(all)),
		fmt.Sprintf("Number of %s in %s?", cat.name, joinList(all)),
	})
	return joinChallenge{Kind: "category", Prompt: prompt, Answers: numericAnswers(nMembers)}
}

// genChallengeSequence: continue an arithmetic run stated in words.
func genChallengeSequence(r *captchaRand) joinChallenge {
	step := 2 + r.intn(5) // 2..6
	// The four printed terms are numberWords[start + i*step] for i in 0..3, so
	// start must leave room for three whole steps inside the spelled-out range.
	// Without this bound a large step with a large start indexes past the end
	// of numberWords -- which is a panic on the connection's own goroutine, not
	// a bad question.
	maxStart := len(numberWords) - 1 - 3*step
	if maxStart > 5 {
		maxStart = 5
	}
	if maxStart < 1 {
		maxStart = 1
	}
	start := 1 + r.intn(maxStart) // 1..maxStart
	terms := make([]string, 4)
	for i := 0; i < 4; i++ {
		terms[i] = numberWords[start+i*step]
	}
	next := start + 4*step
	prompt := captchaPick(r, []string{
		fmt.Sprintf("What number comes next: %s, ...?", strings.Join(terms, ", ")),
		fmt.Sprintf("Continue the pattern -- %s, then what?", strings.Join(terms, ", ")),
		fmt.Sprintf("%s ... which number follows?", strings.Join(terms, ", ")),
		fmt.Sprintf("Carry this sequence one step further: %s.", strings.Join(terms, ", ")),
		fmt.Sprintf("The run goes %s. What is the next one?", strings.Join(terms, ", ")),
		fmt.Sprintf("Work out the step in %s and give the number after it.", strings.Join(terms, ", ")),
	})
	return joinChallenge{Kind: "sequence", Prompt: prompt, Answers: numericAnswers(next)}
}

// genChallengeNato: the letters spelled by NATO-alphabet words.
func genChallengeNato(r *captchaRand) joinChallenge {
	n := 3 + r.intn(2) // 3..4
	words := captchaShuffle(r, natoWords)[:n]
	var sb strings.Builder
	for _, w := range words {
		sb.WriteByte(natoAlphabet[w])
	}
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Each of these words stands for its own first letter. Type the letters they spell: %s", strings.Join(words, "-")),
		fmt.Sprintf("Spell out the letters these stand for: %s", strings.Join(words, " ")),
		fmt.Sprintf("%s -- type the letters, not the words.", strings.Join(words, ", ")),
		fmt.Sprintf("Radio alphabet: %s. What does it spell?", strings.Join(words, " ")),
		fmt.Sprintf("Turn %s into the letters they each begin with.", strings.Join(words, ", ")),
	})
	return joinChallenge{Kind: "nato", Prompt: prompt, Answers: []string{sb.String()}}
}

// genChallengeConcat: two halves stated separately, joined by the player. The
// full answer never appears contiguously in the prompt.
func genChallengeConcat(r *captchaRand) joinChallenge {
	a := captchaToken(r, 3)
	b := captchaToken(r, 3)
	sa, sb := presentTokenLoose(r, a), presentTokenLoose(r, b)
	prompt := captchaPick(r, []string{
		fmt.Sprintf("The first half is %s. The second half is %s. Type them joined together, with no space.", sa, sb),
		fmt.Sprintf("Glue %s onto the end of %s and type the result as one word.", sb, sa),
		fmt.Sprintf("Part one: %s. Part two: %s. Type both as a single block.", sa, sb),
		fmt.Sprintf("Put %s first and %s straight after it, as one run of characters.", sa, sb),
		fmt.Sprintf("Two pieces -- %s then %s. Type them as one.", sa, sb),
	})
	return joinChallenge{Kind: "concat", Prompt: prompt, Answers: []string{a + b}}
}

// genChallengeWordLength: how many letters a word has.
func genChallengeWordLength(r *captchaRand) joinChallenge {
	w := captchaPick(r, captchaWords)
	up := strings.ToUpper(w)
	prompt := captchaPick(r, []string{
		fmt.Sprintf("How many letters are in the word %s?", up),
		fmt.Sprintf("Count the letters in %s.", up),
		fmt.Sprintf("%s -- how many letters is that?", up),
		fmt.Sprintf("Give the length of the word %s.", up),
		fmt.Sprintf("How long is %s, in letters?", up),
	})
	return joinChallenge{Kind: "wordlength", Prompt: prompt, Answers: numericAnswers(len(w))}
}

// genChallengeWordOrder: the listed words typed back in reverse order. Distinct
// from genChallengeReverse, which reverses characters rather than words.
func genChallengeWordOrder(r *captchaRand) joinChallenge {
	n := 3 + r.intn(2) // 3..4
	words := captchaShuffle(r, captchaWords)[:n]
	rev := make([]string, 0, n)
	for i := len(words) - 1; i >= 0; i-- {
		rev = append(rev, words[i])
	}
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Type these words in the opposite order, with no spaces: %s", strings.Join(words, ", ")),
		fmt.Sprintf("Last word first -- retype %s in reverse order, joined together.", strings.Join(words, ", ")),
		fmt.Sprintf("Reverse the order of %s and type them as one run.", strings.Join(words, ", ")),
		fmt.Sprintf("Read %s back to front, word by word, and type the result.", strings.Join(words, ", ")),
	})
	return joinChallenge{Kind: "wordorder", Prompt: prompt, Answers: []string{strings.Join(rev, "")}}
}

// genChallengeSumDigits: add the digits of a spelled-out set. The digits are
// written as words so the prompt again carries no numerals to scrape.
func genChallengeSumDigits(r *captchaRand) joinChallenge {
	n := 3 + r.intn(2) // 3..4
	digits := make([]string, 0, n)
	total := 0
	for i := 0; i < n; i++ {
		d := 1 + r.intn(9) // 1..9
		total += d
		digits = append(digits, numberWords[d])
	}
	prompt := captchaPick(r, []string{
		fmt.Sprintf("Add these together: %s.", joinList(digits)),
		fmt.Sprintf("What do %s come to in total?", joinList(digits)),
		fmt.Sprintf("Sum up %s and give the number.", joinList(digits)),
		fmt.Sprintf("%s -- what is the total?", joinList(digits)),
		fmt.Sprintf("Add up every number here: %s.", joinList(digits)),
	})
	return joinChallenge{Kind: "sumdigits", Prompt: prompt, Answers: numericAnswers(total)}
}

// genChallengeAlphabet: the letter a fixed number of steps from a given one.
// Trivial for a person, and the answer letter is never printed.
func genChallengeAlphabet(r *captchaRand) joinChallenge {
	// Keep the start letter clear of the ends so the walk never wraps.
	start := byte('c') + byte(r.intn(18)) // c..t
	step := 1 + r.intn(3)                 // 1..3
	forward := r.intn(2) == 0
	var answer byte
	var dir string
	if forward {
		answer, dir = start+byte(step), "after"
	} else {
		answer, dir = start-byte(step), "before"
	}
	up := strings.ToUpper(string(start))
	stepWord := numberWords[step]
	var prompt string
	if step == 1 {
		prompt = captchaPick(r, []string{
			fmt.Sprintf("Which letter comes straight %s %s in the alphabet?", dir, up),
			fmt.Sprintf("Name the letter immediately %s %s.", dir, up),
			fmt.Sprintf("In the alphabet, what is the letter just %s %s?", dir, up),
		})
	} else {
		prompt = captchaPick(r, []string{
			fmt.Sprintf("Count %s letters %s %s in the alphabet. Which letter do you land on?", stepWord, dir, up),
			fmt.Sprintf("Starting at %s, move %s letters %s it in the alphabet. What letter is that?", up, stepWord, dir),
			fmt.Sprintf("Which letter sits %s places %s %s?", stepWord, dir, up),
		})
	}
	return joinChallenge{Kind: "alphabet", Prompt: prompt, Answers: []string{string(answer)}}
}

// genChallengeOddPosition: the position of the one word that does not belong.
// Answering needs comprehension of the words and a count, and the answer is a
// number that never appears in the list.
func genChallengeOddPosition(r *captchaRand) joinChallenge {
	cat := captchaPick(r, captchaCategories)
	n := 3 + r.intn(2) // 3..4 members
	items := captchaShuffle(r, cat.members)[:n]
	odd := captchaPick(r, cat.others)
	pos := 1 + r.intn(len(items)+1)
	// Splice the outsider in at pos (1-indexed).
	full := make([]string, 0, len(items)+1)
	full = append(full, items[:pos-1]...)
	full = append(full, odd)
	full = append(full, items[pos-1:]...)
	prompt := captchaPick(r, []string{
		fmt.Sprintf("One of these is not one of the %s. Counting from one, what position is it in? %s", cat.name, joinList(full)),
		fmt.Sprintf("%s -- all but one are %s. Give the position of the odd one out, counting from the left.", joinList(full), cat.name),
		fmt.Sprintf("Find the word that isn't one of the %s in %s, and answer with its position in the list.", cat.name, joinList(full)),
	})
	return joinChallenge{Kind: "oddposition", Prompt: prompt, Answers: numericAnswers(pos)}
}

// --- assembly ----------------------------------------------------------------

// captchaLeadIns frame the question itself. Varying the wrapper as well as the
// question means there is no stable anchor phrase ("please type", "verify with")
// for a scraper to cut on -- the useful text is in a different place, in
// different words, every time.
var captchaLeadIns = []string{
	"Quick check before you can talk:",
	"One question first:",
	"Prove you're a person:",
	"Answer this to start chatting:",
	"Human check --",
	"Before you can speak, solve this:",
	"A small puzzle for you:",
	"Just one thing:",
	"To unlock chat, answer this:",
	"Verification --",
	"Riddle me this:",
	"Here's your check:",
}

// captchaTailHints vary how the answer is asked for. Every variant names
// /verify, since that is what a real player actually needs to know.
var captchaTailHints = []string{
	"Reply with:  /verify <your answer>",
	"Answer it with:  /verify <answer>",
	"Send your answer as:  /verify <answer>",
	"Type  /verify <answer>  in OOC to reply.",
	"Put your answer after /verify, like:  /verify <answer>",
	"When you have it:  /verify <answer>",
}

// decorateChallenge wraps a generated question in a randomly chosen lead-in and
// answer hint. Applied before the leak check in newJoinChallenge so the framing
// text is covered by the same substring invariant as the question.
func decorateChallenge(r *captchaRand, c joinChallenge) joinChallenge {
	c.Prompt = captchaPick(r, captchaLeadIns) + " " + c.Prompt
	c.Hint = captchaPick(r, captchaTailHints)
	return c
}

// challengeAnswerLeaks reports whether any accepted answer appears verbatim
// inside the prompt once both are normalized. This is the invariant that makes
// the whole feature worth having: if it ever held false, the challenge would be
// solvable by a substring scrape and would be no better than "type ABC123".
func challengeAnswerLeaks(c joinChallenge) bool {
	np := normalizeCaptchaAnswer(c.Prompt)
	for _, a := range c.Answers {
		na := normalizeCaptchaAnswer(a)
		// An empty answer would match everything; treat it as a leak so the
		// generator is re-rolled rather than producing an unanswerable prompt.
		if na == "" || strings.Contains(np, na) {
			return true
		}
	}
	return false
}

// normalizeChallenge normalizes a generated challenge's answers in place and
// drops any duplicates, so matching is a plain string comparison at check time.
func normalizeChallenge(c joinChallenge) joinChallenge {
	seen := make(map[string]struct{}, len(c.Answers))
	out := c.Answers[:0]
	for _, a := range c.Answers {
		na := normalizeCaptchaAnswer(a)
		if na == "" {
			continue
		}
		if _, dup := seen[na]; dup {
			continue
		}
		seen[na] = struct{}{}
		out = append(out, na)
	}
	c.Answers = out
	return c
}

// newJoinChallenge builds a challenge using a random kind from the enabled set.
//
// The invariant is enforced here, not assumed: a generator whose random draw
// happens to leak its own answer into the prompt (a palindromic token, a count
// that coincides with a digit in the text) is simply re-rolled. Kinds are tried
// in shuffled order so an unlucky kind cannot stall generation, and a final
// hard-coded fallback guarantees this function always returns something
// answerable even if every generator somehow failed.
//
// kinds selects the enabled generators by id; an empty or fully-unrecognised
// list means "use them all", which is what an operator who has not configured
// join_captcha_kinds gets.
func newJoinChallenge(r *captchaRand, kinds []string) joinChallenge {
	pool := enabledChallengeGens(kinds)
	for _, g := range captchaShuffle(r, pool) {
		for attempt := 0; attempt < 8; attempt++ {
			c, ok := safeGenerate(g, r)
			if !ok {
				break // this generator is broken; try the next kind
			}
			if len(c.Answers) > 0 && !challengeAnswerLeaks(c) {
				return c
			}
		}
	}
	// Unreachable in practice: genChallengeMath spells its operands as words,
	// so its prompt contains no digits for the numeric answer to collide with.
	// Kept so the caller can never be handed an empty challenge.
	a, b := 3, 4
	return normalizeChallenge(joinChallenge{
		Kind:    "math",
		Prompt:  fmt.Sprintf("What is %s plus %s?", numberWords[a], numberWords[b]),
		Answers: numericAnswers(a + b),
	})
}

// safeGenerate runs one generator, converting a panic into a failed attempt.
//
// These run on the connection's own goroutine during the join handshake, where
// an index slip in a generator would take the connection (or worse) down rather
// than merely producing a bad question. The generators are unit-tested against
// exactly that, but a captcha is not worth a crash, so the blast radius of a
// future mistake is capped here: a broken kind is skipped and another is used.
func safeGenerate(g joinChallengeGen, r *captchaRand) (c joinChallenge, ok bool) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.LogErrorf("captcha generator %q panicked and was skipped: %v", g.kind, rec)
			c, ok = joinChallenge{}, false
		}
	}()
	return decorateChallenge(r, normalizeChallenge(g.gen(r))), true
}

// enabledChallengeGens filters the generator pool by the configured kind ids.
// Unknown ids are ignored (they are reported once at startup by
// validateJoinCaptchaKinds), and a selection that matches nothing falls back to
// the full pool -- a typo in config must not silently leave every joining
// player with an unanswerable captcha.
func enabledChallengeGens(kinds []string) []joinChallengeGen {
	if len(kinds) == 0 {
		return joinChallengeGens
	}
	want := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		want[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	out := make([]joinChallengeGen, 0, len(joinChallengeGens))
	for _, g := range joinChallengeGens {
		if _, ok := want[g.kind]; ok {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		return joinChallengeGens
	}
	return out
}

// validateJoinCaptchaKinds returns the ids in kinds that name no known
// generator, so startup can warn about a typo instead of quietly ignoring it.
func validateJoinCaptchaKinds(kinds []string) []string {
	known := make(map[string]struct{}, len(joinChallengeGens))
	for _, g := range joinChallengeGens {
		known[g.kind] = struct{}{}
	}
	var bad []string
	for _, k := range kinds {
		if _, ok := known[strings.ToLower(strings.TrimSpace(k))]; !ok {
			bad = append(bad, k)
		}
	}
	return bad
}

// checkChallengeAnswer reports whether supplied satisfies the challenge.
func checkChallengeAnswer(c joinChallenge, supplied string) bool {
	got := normalizeCaptchaAnswer(supplied)
	if got == "" {
		return false
	}
	for _, a := range c.Answers {
		if got == a {
			return true
		}
	}
	return false
}
