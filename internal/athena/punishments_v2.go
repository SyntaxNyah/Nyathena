/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: wave-2 punishment text transforms.

   New text-effect punishments: /zalgo, /leetspeak, /smallcaps, /piglatin,
   /vaporwave, /lisp, /spoonerism, /keysmash, /politician, /clickbait,
   /markov, /alliteration and the escalating /cipher. The weeb transform
   lives in punishments_weeb.go because of its large romaji tables.

   Several of these transforms EXPAND the text (combining marks, fullwidth
   runes, binary digits). pktIC re-validates message length AFTER punishment
   transforms run, so an oversized result would get the whole message
   silently dropped — every expanding transform therefore clamps its output
   with fitICBudget so the punished player keeps talking. */

package athena

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// icBudget returns the server's maximum IC message length in bytes (decoded
// form, which is what pktIC validates). Falls back to the config default when
// the config isn't loaded (unit tests).
func icBudget() int {
	if config != nil && config.MaxMsg > 0 {
		return config.MaxMsg
	}
	return 256
}

// truncBytes cuts s to at most max bytes on a rune boundary.
func truncBytes(s string, max int) string {
	if len(s) <= max || max <= 0 {
		if max <= 0 {
			return ""
		}
		return s
	}
	var b strings.Builder
	b.Grow(max)
	for _, r := range s {
		if b.Len()+utf8.RuneLen(r) > max {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

// fitICBudget clamps a transformed message so it never exceeds the IC length
// limit that pktIC enforces post-transform.
func fitICBudget(s string) string {
	return truncBytes(s, icBudget())
}

// splitWordCore splits a whitespace-free token into leading punctuation, the
// letter/digit core, and trailing punctuation, so word-level transforms can
// rewrite the core while preserving quotes, commas, etc.
func splitWordCore(w string) (pre, core, post string) {
	runes := []rune(w)
	start := 0
	for start < len(runes) && !unicode.IsLetter(runes[start]) && !unicode.IsDigit(runes[start]) {
		start++
	}
	end := len(runes)
	for end > start && !unicode.IsLetter(runes[end-1]) && !unicode.IsDigit(runes[end-1]) {
		end--
	}
	return string(runes[:start]), string(runes[start:end]), string(runes[end:])
}

// firstNWords returns up to n leading words of text joined by spaces.
func firstNWords(text string, n int) string {
	words := strings.Fields(text)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}

// isPlainVowel reports whether r is one of aeiou (either case).
func isPlainVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// onsetLen returns the length (in runes) of a word's leading consonant
// cluster: the letters before the first vowel ('y' counts as a vowel after
// the first letter). Returns 0 when the word starts with a vowel, has no
// vowel at all, or is entirely consumed by the cluster.
func onsetLen(core string) int {
	runes := []rune(core)
	for i, r := range runes {
		if !unicode.IsLetter(r) {
			return 0
		}
		if isPlainVowel(r) || (i > 0 && unicode.ToLower(r) == 'y') {
			return i
		}
	}
	return 0
}

// capitalizeFirst uppercases the first rune and lowercases the rest.
func capitalizeFirst(s string) string {
	runes := []rune(strings.ToLower(s))
	if len(runes) == 0 {
		return s
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ── /zalgo ────────────────────────────────────────────────────────────────

// applyZalgo corrupts the text with combining marks, reusing the doki-area
// zalgoify engine at a readable-ish intensity. Marks are ~2 bytes each so the
// input is pre-trimmed to keep the output inside the IC budget.
func applyZalgo(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	t := truncBytes(text, icBudget()/5)
	return fitICBudget(dokiZalgoify(t, 2))
}

// ── /leetspeak ────────────────────────────────────────────────────────────

var leetCharTable = map[rune]rune{
	'a': '4', 'A': '4', 'e': '3', 'E': '3', 'i': '1', 'I': '1',
	'o': '0', 'O': '0', 's': '5', 'S': '5', 't': '7', 'T': '7',
	'b': '8', 'B': '8', 'g': '9', 'G': '9',
}

var leetWordTable = map[string]string{
	"hacker": "h4x0r", "hackers": "h4x0rz", "hack": "h4xx", "hacked": "pwn3d",
	"elite": "1337", "leet": "1337", "skills": "skillz", "skill": "skillz",
	"you": "u", "your": "ur", "you're": "ur", "are": "r", "and": "&",
	"dude": "d00d", "owned": "pwn3d", "own": "pwn", "owns": "pwnz",
	"cool": "k3wl", "the": "teh", "what": "wut", "noob": "n00b",
	"newbie": "n00b", "rocks": "roxx0rz", "sucks": "suxx0rz",
	"awesome": "1337", "win": "w1n", "lose": "l0se", "loser": "l0z3r",
	"yes": "y3s", "no": "n0", "more": "moar", "ever": "evar",
}

var leetSuffixes = []string{" lol", " kek", " >:3", " gg no re", " ROFLMAO", " pwnt", " /b/"}

func applyLeetspeak(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	for i, w := range words {
		pre, core, post := splitWordCore(w)
		if rep, ok := leetWordTable[strings.ToLower(core)]; ok {
			words[i] = pre + rep + post
			continue
		}
		words[i] = strings.Map(func(r rune) rune {
			if v, ok := leetCharTable[r]; ok {
				return v
			}
			return r
		}, w)
	}
	out := strings.Join(words, " ")
	if rand.Intn(3) == 0 {
		out += leetSuffixes[rand.Intn(len(leetSuffixes))]
	}
	return fitICBudget(out)
}

// ── /smallcaps ────────────────────────────────────────────────────────────

var smallcapsTable = map[rune]rune{
	'a': 'ᴀ', 'b': 'ʙ', 'c': 'ᴄ', 'd': 'ᴅ', 'e': 'ᴇ', 'f': 'ꜰ', 'g': 'ɢ',
	'h': 'ʜ', 'i': 'ɪ', 'j': 'ᴊ', 'k': 'ᴋ', 'l': 'ʟ', 'm': 'ᴍ', 'n': 'ɴ',
	'o': 'ᴏ', 'p': 'ᴘ', 'q': 'ǫ', 'r': 'ʀ', 't': 'ᴛ', 'u': 'ᴜ',
	'v': 'ᴠ', 'w': 'ᴡ', 'y': 'ʏ', 'z': 'ᴢ',
	// 's' and 'x' have no distinct small-cap glyph; they pass through.
}

func applySmallcaps(text string) string {
	out := strings.Map(func(r rune) rune {
		if v, ok := smallcapsTable[unicode.ToLower(r)]; ok {
			return v
		}
		return unicode.ToLower(r)
	}, text)
	return fitICBudget(out)
}

// ── /piglatin ─────────────────────────────────────────────────────────────

// pigLatinCore converts one word core: leading consonant cluster moves to the
// end + "ay"; vowel-start words get "yay". Capitalization stays on the first
// letter of the result.
func pigLatinCore(core string) string {
	runes := []rune(core)
	if len(runes) < 2 {
		return core
	}
	for _, r := range runes {
		if !unicode.IsLetter(r) {
			return core // mixed alphanumerics (UIDs, "2nd") stay readable
		}
	}
	wasCap := unicode.IsUpper(runes[0])
	if isPlainVowel(runes[0]) {
		return core + "yay"
	}
	on := onsetLen(core)
	if on <= 0 || on >= len(runes) {
		return core + "ay" // no vowel: just tack on "ay"
	}
	out := strings.ToLower(string(runes[on:])) + strings.ToLower(string(runes[:on])) + "ay"
	if wasCap {
		out = capitalizeFirst(out)
	}
	return out
}

func applyPiglatin(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	for i, w := range words {
		pre, core, post := splitWordCore(w)
		if core == "" {
			continue
		}
		words[i] = pre + pigLatinCore(core) + post
	}
	return fitICBudget(strings.Join(words, " "))
}

// ── /vaporwave ────────────────────────────────────────────────────────────

var vaporwaveSuffixes = []string{"　ａｅｓｔｈｅｔｉｃ", "　ｖｉｂｅｓ", "　彡", "　≋"}

func applyVaporwave(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 3)
	for _, r := range text {
		switch {
		case r == ' ':
			b.WriteRune('　')
		case r > 0x20 && r < 0x7F:
			b.WriteRune(r + 0xFEE0) // ASCII → fullwidth block
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if rand.Intn(4) == 0 {
		out += vaporwaveSuffixes[rand.Intn(len(vaporwaveSuffixes))]
	}
	return fitICBudget(out)
}

// ── /lisp ─────────────────────────────────────────────────────────────────

var lispSuffixes = []string{" ...thorry.", " Tho thorry about that.", ""}

func applyLisp(text string) string {
	runes := []rune(text)
	var b strings.Builder
	b.Grow(len(text) + len(text)/4)
	for i, r := range runes {
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch r {
		case 's':
			if unicode.ToLower(next) == 'h' {
				b.WriteRune(r) // "sh" survives a classic lisp
			} else {
				b.WriteString("th")
			}
		case 'S':
			if unicode.ToLower(next) == 'h' {
				b.WriteRune(r)
			} else {
				b.WriteString("Th")
			}
		case 'z':
			b.WriteString("th")
		case 'Z':
			b.WriteString("Th")
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if rand.Intn(5) == 0 {
		out += lispSuffixes[rand.Intn(len(lispSuffixes))]
	}
	return fitICBudget(out)
}

// ── /spoonerism ───────────────────────────────────────────────────────────

// applySpoonerism swaps the leading consonant clusters of adjacent eligible
// word pairs ("shake a tower" style). Words must both have a consonant onset
// and at least one letter after it; ineligible words shift the pairing window
// by one.
func applySpoonerism(text string) string {
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}
	i := 0
	for i < len(words)-1 {
		p1, c1, s1 := splitWordCore(words[i])
		p2, c2, s2 := splitWordCore(words[i+1])
		o1, o2 := onsetLen(c1), onsetLen(c2)
		r1, r2 := []rune(c1), []rune(c2)
		if o1 > 0 && o2 > 0 && len(r1) > o1 && len(r2) > o2 {
			cap1 := unicode.IsUpper(r1[0])
			cap2 := unicode.IsUpper(r2[0])
			n1 := strings.ToLower(string(r2[:o2])) + strings.ToLower(string(r1[o1:]))
			n2 := strings.ToLower(string(r1[:o1])) + strings.ToLower(string(r2[o2:]))
			if cap1 {
				n1 = capitalizeFirst(n1)
			}
			if cap2 {
				n2 = capitalizeFirst(n2)
			}
			words[i] = p1 + n1 + s1
			words[i+1] = p2 + n2 + s2
			i += 2
		} else {
			i++
		}
	}
	return fitICBudget(strings.Join(words, " "))
}

// ── /keysmash ─────────────────────────────────────────────────────────────

var keysmashRow = []rune("asdfghjkl;")

func keysmashBurst() string {
	n := 5 + rand.Intn(5)
	runes := make([]rune, n)
	for i := range runes {
		runes[i] = keysmashRow[rand.Intn(len(keysmashRow))]
	}
	return string(runes)
}

func applyKeysmash(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return keysmashBurst()
	}
	bursts := 1 + rand.Intn(2)
	for k := 0; k < bursts; k++ {
		pos := rand.Intn(len(words) + 1)
		words = append(words[:pos], append([]string{keysmashBurst()}, words[pos:]...)...)
	}
	if rand.Intn(4) == 0 {
		words = append(words, keysmashBurst())
	}
	return fitICBudget(strings.Join(words, " "))
}

// ── /politician ───────────────────────────────────────────────────────────

var politicianOpeners = []string{
	"Great question.",
	"I'm so glad you asked that.",
	"Let me be perfectly clear.",
	"Look —",
	"With all due respect,",
	"Let me just say this:",
	"I've been very clear about this from day one.",
	"Now, I won't be lectured on this topic.",
	"My position has not changed.",
	"The people of this courtroom deserve answers.",
}

var politicianPivots = []string{
	"but what the people REALLY want to know is",
	"but the real issue here is",
	"yet nobody is talking about",
	"but let's focus on what actually matters:",
	"though my opponent doesn't want you asking about",
	"but I think we can all agree the bigger concern is",
	"however, the question we should be asking is about",
}

var politicianClosers = []string{
	"and that's why you can count on me.",
	"and I have always been consistent on that.",
	"and that's a promise.",
	"and the facts speak for themselves.",
	"and I will not apologize for it.",
	"— next question.",
	"and frankly, the polls agree with me.",
	"and history will prove me right.",
}

// applyPolitician deflects: the original message survives only as a vaguely
// gestured-at "topic" the speaker refuses to address.
func applyPolitician(text string) string {
	topic := firstNWords(text, 4)
	if topic == "" {
		topic = "the economy"
	}
	out := fmt.Sprintf("%s %s the matter of \"%s…\", %s",
		politicianOpeners[rand.Intn(len(politicianOpeners))],
		politicianPivots[rand.Intn(len(politicianPivots))],
		topic,
		politicianClosers[rand.Intn(len(politicianClosers))])
	return fitICBudget(out)
}

// ── /clickbait ────────────────────────────────────────────────────────────

// Templates use {name}/{snippet}/{n}/{m} placeholders expanded with a
// strings.Replacer (avoids fmt verb-count pitfalls across templates).
var clickbaitTemplates = []string{
	`You WON'T BELIEVE What {name} Just Said: "{snippet}…" (Number {n} Will SHOCK You)`,
	`{name} Said "{snippet}" And The Whole Courtroom Went SILENT — What Happened Next Is INSANE`,
	`Top {n} Signs "{snippet}" Changes EVERYTHING (#{m} Made Me Cry)`,
	`BREAKING: Local Legend {name} Drops "{snippet}…" — Experts FURIOUS`,
	`They Tried To SILENCE {name} For Saying This: "{snippet}…"`,
	`Doctors HATE {name} After "{snippet}" — Learn Their One Weird Trick`,
	`"{snippet}" — {name}'s Shocking Confession Has Everyone Talking (You Won't Believe #{m})`,
	`{n} Reasons {name} Saying "{snippet}" Broke The Internet`,
}

// applyClickbaitWithName turns the message into a clickbait headline about
// the speaker. pktIC passes the speaker's display name; the nameless fallback
// is used by random pools.
func applyClickbaitWithName(text, name string) string {
	if strings.TrimSpace(name) == "" {
		name = "This Player"
	}
	snippet := firstNWords(text, 6)
	if snippet == "" {
		snippet = "…"
	}
	n := 3 + rand.Intn(7)
	m := 1 + rand.Intn(n)
	out := strings.NewReplacer(
		"{name}", name,
		"{snippet}", snippet,
		"{n}", strconv.Itoa(n),
		"{m}", strconv.Itoa(m),
	).Replace(clickbaitTemplates[rand.Intn(len(clickbaitTemplates))])
	return fitICBudget(out)
}

func applyClickbait(text string) string {
	return applyClickbaitWithName(text, "This Player")
}

// ── /alliteration ─────────────────────────────────────────────────────────

var alliterationLetters = []rune("bcdfgklmnprstw")

// applyAlliteration picks a consonant per message and rewrites every word's
// onset to it, so sentences awkwardly — but boldly, bravely, beautifully —
// alliterate.
func applyAlliteration(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	letter := alliterationLetters[rand.Intn(len(alliterationLetters))]
	for i, w := range words {
		pre, core, post := splitWordCore(w)
		runes := []rune(core)
		if len(runes) < 3 {
			continue
		}
		if unicode.ToLower(runes[0]) == letter {
			continue // already alliterates
		}
		wasCap := unicode.IsUpper(runes[0])
		on := onsetLen(core)
		rest := strings.ToLower(string(runes[on:])) // on==0 → whole word, letter prefixes it
		newCore := string(letter) + rest
		if rand.Intn(7) == 0 {
			newCore = string(letter) + "-" + newCore // b-banter stutter
		}
		if wasCap {
			newCore = capitalizeFirst(newCore)
		}
		words[i] = pre + newCore + post
	}
	return fitICBudget(strings.Join(words, " "))
}

// ── /cipher ───────────────────────────────────────────────────────────────

func rot13(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+13)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+13)%26
		}
		return r
	}, s)
}

// applyCipherTier renders one encryption layer. tier cycles 0→1→2:
// ROT13 → BINARY → BASE64, each message sinking one layer deeper.
func applyCipherTier(text string, tier int) string {
	switch tier % 3 {
	case 0:
		return "🔐⟦LAYER 1/3 · ROT13⟧ " + rot13(text)
	case 1:
		prefix := "🔐⟦LAYER 2/3 · BINARY⟧ "
		// Each input byte renders as 8 bits + separator; trim input to budget.
		maxIn := (icBudget() - len(prefix)) / 9
		if maxIn < 1 {
			maxIn = 1
		}
		in := truncBytes(text, maxIn)
		var b strings.Builder
		b.Grow(len(in) * 9)
		for i := 0; i < len(in); i++ {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%08b", in[i])
		}
		return prefix + b.String()
	default:
		return "🔐⟦LAYER 3/3 · BASE64⟧ " + base64.StdEncoding.EncodeToString([]byte(text))
	}
}

// applyCipher is the stateless fallback (random pools): random layer per message.
func applyCipher(text string) string {
	return fitICBudget(applyCipherTier(text, rand.Intn(3)))
}

// applyCipherWithState escalates one layer per message. Completing all three
// layers reports a failed decryption attempt and re-arms — the target never
// escapes the encryption.
func applyCipherWithState(text string, state *PunishmentState) string {
	tier := state.lastEffect
	state.lastEffect++
	out := applyCipherTier(text, tier)
	if tier%3 == 2 {
		out += fmt.Sprintf(" ⟦decryption attempt #%d failed — re-arming layers⟧", tier/3+1)
	}
	return fitICBudget(out)
}

// ── /markov ───────────────────────────────────────────────────────────────

// markovCorpusSize bounds how many recent area messages feed the chain.
const markovCorpusSize = 150

// applyMarkov replaces the message with order-1 markov babble generated from
// the area's recent IC history — it sounds like the room because it IS the
// room. Falls back to a word shuffle when the area has no usable history yet.
func applyMarkov(text string, a *area.Area) string {
	corpus := a.RecentICMessages(markovCorpusSize)
	chain := make(map[string][]string)
	var starters []string
	total := 0
	for _, m := range corpus {
		ws := strings.Fields(m)
		if len(ws) == 0 {
			continue
		}
		starters = append(starters, ws[0])
		for i := 0; i+1 < len(ws); i++ {
			key := strings.ToLower(ws[i])
			chain[key] = append(chain[key], ws[i+1])
		}
		total += len(ws)
	}
	if total < 12 || len(starters) == 0 {
		return applyTimewarp(text)
	}
	want := len(strings.Fields(text)) + rand.Intn(5)
	if want < 5 {
		want = 5
	}
	if want > 26 {
		want = 26
	}
	out := make([]string, 0, want)
	cur := starters[rand.Intn(len(starters))]
	out = append(out, cur)
	for len(out) < want {
		nexts := chain[strings.ToLower(cur)]
		if len(nexts) == 0 {
			cur = starters[rand.Intn(len(starters))]
		} else {
			cur = nexts[rand.Intn(len(nexts))]
		}
		out = append(out, cur)
	}
	return fitICBudget(strings.Join(out, " "))
}
