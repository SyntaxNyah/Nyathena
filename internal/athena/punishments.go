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
	"regexp"
	"strings"
	"unicode"
)

// confusedSplitter splits on any sequence of non-letter, non-digit characters
var confusedSplitter = regexp.MustCompile(`[^\p{L}\p{N}]+`)

const maxTextLength = 2000

// safeSubstring safely extracts a substring with bounds checking
func safeSubstring(s string, start, length int) string {
	runes := []rune(s)
	if start >= len(runes) {
		return ""
	}
	end := start + length
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

// truncateText ensures text doesn't exceed maximum length
func truncateText(text string) string {
	if len(text) > maxTextLength {
		return safeSubstring(text, 0, maxTextLength)
	}
	return text
}

// applyBackward reverses character order
func applyBackward(text string) string {
	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// applyStutterstep doubles every word
func applyStutterstep(text string) string {
	words := strings.Fields(text)
	var result strings.Builder
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(word)
		result.WriteString(" ")
		result.WriteString(word)
	}
	return truncateText(result.String())
}

// applyElongate repeats vowels
func applyElongate(text string) string {
	var result strings.Builder
	vowels := "aeiouAEIOU"
	for _, r := range text {
		result.WriteRune(r)
		if strings.ContainsRune(vowels, r) {
			result.WriteRune(r)
			result.WriteRune(r)
		}
	}
	return truncateText(result.String())
}

// applyUppercase converts to uppercase
func applyUppercase(text string) string {
	return strings.ToUpper(text)
}

// applyLowercase converts to lowercase
func applyLowercase(text string) string {
	return strings.ToLower(text)
}

// applyRobotic replaces with [BEEP] [BOOP]
func applyRobotic(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "[BEEP]"
	}
	var result strings.Builder
	robotWords := []string{"[BEEP]", "[BOOP]", "[WHIRR]", "[BUZZ]"}
	for i := 0; i < len(words); i++ {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(robotWords[i%len(robotWords)])
	}
	return truncateText(result.String())
}

// applyAlternating creates alternating case
func applyAlternating(text string) string {
	runes := []rune(text)
	upper := true
	for i, r := range runes {
		if unicode.IsLetter(r) {
			if upper {
				runes[i] = unicode.ToUpper(r)
			} else {
				runes[i] = unicode.ToLower(r)
			}
			upper = !upper
		}
	}
	return string(runes)
}

// applyFancy converts to Unicode fancy characters (mathematical bold)
func applyFancy(text string) string {
	var result strings.Builder
	fancyMap := map[rune]rune{
		'a': '𝐚', 'b': '𝐛', 'c': '𝐜', 'd': '𝐝', 'e': '𝐞', 'f': '𝐟', 'g': '𝐠',
		'h': '𝐡', 'i': '𝐢', 'j': '𝐣', 'k': '𝐤', 'l': '𝐥', 'm': '𝐦', 'n': '𝐧',
		'o': '𝐨', 'p': '𝐩', 'q': '𝐪', 'r': '𝐫', 's': '𝐬', 't': '𝐭', 'u': '𝐮',
		'v': '𝐯', 'w': '𝐰', 'x': '𝐱', 'y': '𝐲', 'z': '𝐳',
		'A': '𝐀', 'B': '𝐁', 'C': '𝐂', 'D': '𝐃', 'E': '𝐄', 'F': '𝐅', 'G': '𝐆',
		'H': '𝐇', 'I': '𝐈', 'J': '𝐉', 'K': '𝐊', 'L': '𝐋', 'M': '𝐌', 'N': '𝐍',
		'O': '𝐎', 'P': '𝐏', 'Q': '𝐐', 'R': '𝐑', 'S': '𝐒', 'T': '𝐓', 'U': '𝐔',
		'V': '𝐕', 'W': '𝐖', 'X': '𝐗', 'Y': '𝐘', 'Z': '𝐙',
	}
	for _, r := range text {
		if fancy, ok := fancyMap[r]; ok {
			result.WriteRune(fancy)
		} else {
			result.WriteRune(r)
		}
	}
	return truncateText(result.String())
}

// applyUwu converts to UwU speak
func applyUwu(text string) string {
	text = strings.ReplaceAll(text, "r", "w")
	text = strings.ReplaceAll(text, "R", "W")
	text = strings.ReplaceAll(text, "l", "w")
	text = strings.ReplaceAll(text, "L", "W")
	text = strings.ReplaceAll(text, "no", "nyo")
	text = strings.ReplaceAll(text, "No", "Nyo")
	text = strings.ReplaceAll(text, "na", "nya")
	text = strings.ReplaceAll(text, "Na", "Nya")
	
	// Add random UwU expressions
	if rand.Float32() < 0.3 {
		suffixes := []string{" uwu", " owo", " >w<", " ^w^"}
		text += suffixes[rand.Intn(len(suffixes))]
	}
	return truncateText(text)
}

// applyPirate converts to pirate speech
func applyPirate(text string) string {
	replacements := map[string]string{
		"hello": "ahoy",
		"hi":    "ahoy",
		"yes":   "aye",
		"my":    "me",
		"you":   "ye",
		"your":  "yer",
		"are":   "be",
		"is":    "be",
	}
	
	lower := strings.ToLower(text)
	for old, new := range replacements {
		lower = strings.ReplaceAll(lower, old, new)
	}
	
	// Add pirate expressions
	if rand.Float32() < 0.3 {
		suffixes := []string{", arr!", ", matey!", ", ye scurvy dog!"}
		lower += suffixes[rand.Intn(len(suffixes))]
	}
	return truncateText(lower)
}

// applyShakespearean converts to Shakespearean English
func applyShakespearean(text string) string {
	replacements := map[string]string{
		"you":       "thou",
		"your":      "thy",
		"yours":     "thine",
		"are":       "art",
		"yes":       "aye",
		"no":        "nay",
		"have":      "hast",
		"has":       "hath",
		"do":        "dost",
		"does":      "doth",
		"will":      "shalt",
		"would":     "wouldst",
		"could":     "couldst",
		"should":    "shouldst",
		"was":       "wast",
		"were":      "wert",
		"never":     "ne'er",
		"ever":      "e'er",
		"over":      "o'er",
		"every":     "ev'ry",
		"before":    "ere",
		"hello":     "hail",
		"goodbye":   "farewell",
		"bye":       "fare thee well",
		"why":       "wherefore",
		"where":     "whence",
		"here":      "hither",
		"there":     "thither",
		"away":      "hence",
		"soon":      "anon",
		"please":    "prithee",
		"help":      "aid",
		"maybe":     "perchance",
		"perhaps":   "mayhaps",
		"truly":     "verily",
		"really":    "forsooth",
		"sure":      "forsooth",
		"totally":   "verily",
		"actually":  "in sooth",
		"okay":      "very well",
		"ok":        "very well",
		"again":     "once more",
		"stop":      "cease",
		"start":     "commence",
		"begin":     "commence",
		"die":       "perish",
		"dead":      "fallen",
		"kill":      "slay",
		"fight":     "battle",
		"bad":       "vile",
		"very":      "most",
		"awesome":   "most wondrous",
		"cool":      "most excellent",
		"amazing":   "wondrous",
		"beautiful": "beauteous",
		"happy":     "merry",
		"sad":       "woeful",
		"angry":     "wrathful",
		"tired":     "weary",
		"people":    "souls",
		"person":    "soul",
		"friend":    "companion",
		"enemy":     "foe",
		"home":      "dwelling",
		"house":     "abode",
		"money":     "coin",
		"world":     "realm",
		"morning":   "morn",
		"evening":   "eve",
		"night":     "eventide",
		"worry":     "fret",
		"hurry":     "make haste",
		"because":   "for",
		"although":  "albeit",
		"thing":     "matter",
		"things":    "matters",
		"speak":     "speakest",
		"say":       "sayest",
	}

	words := strings.Fields(text)
	for i, word := range words {
		// Strip trailing punctuation for lookup, restore it after
		punct := ""
		stripped := word
		if len(stripped) > 0 {
			last := rune(stripped[len(stripped)-1])
			if strings.ContainsRune(".,!?;:", last) {
				punct = string(last)
				stripped = stripped[:len(stripped)-1]
			}
		}
		lower := strings.ToLower(stripped)
		if replacement, ok := replacements[lower]; ok {
			// Preserve leading capital if original word was capitalised
			if len(stripped) > 0 && unicode.IsUpper([]rune(stripped)[0]) {
				r := []rune(replacement)
				r[0] = unicode.ToUpper(r[0])
				replacement = string(r)
			}
			words[i] = replacement + punct
		}
	}

	result := strings.Join(words, " ")

	prefixes := []string{
		"Hark! ",
		"Forsooth! ",
		"Zounds! ",
		"Prithee, ",
		"Methinks ",
		"By my troth! ",
		"O fie! ",
		"Marry! ",
		"'Tis said that ",
		"Good morrow! ",
	}
	if rand.Float32() < 0.4 {
		result = prefixes[rand.Intn(len(prefixes))] + result
	}

	suffixes := []string{
		", methinks.",
		", forsooth!",
		", I prithee.",
		", good soul.",
		", 'tis so!",
		", verily.",
		", upon mine honour.",
		", I dare say.",
	}
	if rand.Float32() < 0.3 {
		result = result + suffixes[rand.Intn(len(suffixes))]
	}

	return truncateText(result)
}

// applyCaveman converts to caveman grunts
func applyCaveman(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "UGH"
	}
	
	cavemanWords := []string{"UGH", "GRUNT", "OOG", "RAWR", "HMPH", "GRUG"}
	var result strings.Builder
	for i := 0; i < (len(words)+1)/2; i++ {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(cavemanWords[rand.Intn(len(cavemanWords))])
	}
	return truncateText(result.String())
}

// applyCensor replaces words with [CENSORED]
func applyCensor(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		if len(word) > 3 && rand.Float32() < 0.4 {
			words[i] = "[CENSORED]"
		}
	}
	return truncateText(strings.Join(words, " "))
}

// applyConfused reorders words randomly
func applyConfused(text string) string {
	// Split on any non-letter, non-digit characters to prevent bypass via dots, hyphens, etc.
	parts := confusedSplitter.Split(text, -1)
	var words []string
	for _, w := range parts {
		if w != "" {
			words = append(words, w)
		}
	}
	if len(words) <= 1 {
		return text
	}

	// Shuffle words
	for i := range words {
		j := rand.Intn(len(words))
		words[i], words[j] = words[j], words[i]
	}
	return truncateText(strings.Join(words, " "))
}

// applyParanoid adds paranoid text
func applyParanoid(text string) string {
	paranoidPhrases := []string{
		" (they're watching)",
		" (don't trust them)",
		" (they know)",
		" (THEY'RE LISTENING)",
		" (it's a conspiracy)",
	}
	phrase := paranoidPhrases[rand.Intn(len(paranoidPhrases))]
	return truncateText(text + phrase)
}

// applyDrunk slurs and repeats words
func applyDrunk(text string) string {
	words := strings.Fields(text)
	var result strings.Builder
	
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		
		// Randomly repeat words
		if rand.Float32() < 0.3 {
			result.WriteString(word)
			result.WriteString(" ")
		}
		
		// Slur by repeating letters
		runes := []rune(word)
		for j, r := range runes {
			result.WriteRune(r)
			if j > 0 && rand.Float32() < 0.2 {
				result.WriteRune(r)
			}
		}
	}
	
	// Add hiccups
	if rand.Float32() < 0.3 {
		result.WriteString(" *hic*")
	}
	return truncateText(result.String())
}

// applyHiccup interrupts words with "hic"
func applyHiccup(text string) string {
	words := strings.Fields(text)
	var result strings.Builder
	
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(word)
		
		if rand.Float32() < 0.4 {
			result.WriteString(" *hic*")
		}
	}
	return truncateText(result.String())
}

// applyWhistle replaces letters with whistles
func applyWhistle(text string) string {
	words := strings.Fields(text)
	whistles := []string{"♪", "♫", "~", "♬"}
	
	var result strings.Builder
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		for range word {
			result.WriteString(whistles[rand.Intn(len(whistles))])
		}
	}
	return truncateText(result.String())
}

// applyMumble obscures message
func applyMumble(text string) string {
	words := strings.Fields(text)
	var result strings.Builder
	
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		
		runes := []rune(word)
		for j, r := range runes {
			if j == 0 || j == len(runes)-1 {
				result.WriteRune(r)
			} else if unicode.IsLetter(r) {
				result.WriteRune('*')
			} else {
				result.WriteRune(r)
			}
		}
	}
	return truncateText(result.String())
}

// applySpaghetti combines multiple random effects
func applySpaghetti(text string) string {
	effects := []func(string) string{
		applyUppercase,
		applyBackward,
		applyElongate,
		applyConfused,
		applyDrunk,
	}
	
	// Apply 2-3 random effects
	numEffects := 2 + rand.Intn(2)
	for i := 0; i < numEffects; i++ {
		effect := effects[rand.Intn(len(effects))]
		text = effect(text)
	}
	return text
}

// applyRng applies random effect from pool
func applyRng(text string) string {
	effects := []func(string) string{
		applyBackward,
		applyUppercase,
		applyLowercase,
		applyUwu,
		applyPirate,
		applyRobotic,
		applyAlternating,
	}
	effect := effects[rand.Intn(len(effects))]
	return effect(text)
}

// applyEssay ensures minimum character count
func applyEssay(text string) string {
	if len(text) < 50 {
		return text + " [MESSAGE TOO SHORT - MINIMUM 50 CHARACTERS REQUIRED]"
	}
	return text
}

// applyAutospell intentionally misspells words
func applyAutospell(text string) string {
	replacements := map[string]string{
		"the":   "teh",
		"you":   "u",
		"your":  "ur",
		"there": "their",
		"their": "there",
		"to":    "too",
		"too":   "to",
		"its":   "it's",
		"it's":  "its",
	}
	
	words := strings.Fields(text)
	for i, word := range words {
		lower := strings.ToLower(word)
		if replacement, ok := replacements[lower]; ok {
			words[i] = replacement
		}
	}
	return strings.Join(words, " ")
}

// applyTorment cycles through different effects based on message count
func applyTorment(text string, cycleIndex int) string {
	effects := []func(string) string{
		applyUppercase,
		applyBackward,
		applyUwu,
		applyRobotic,
		applyConfused,
	}
	effect := effects[cycleIndex%len(effects)]
	return effect(text)
}

// applySubtitles adds confusing annotations
func applySubtitles(text string) string {
	subtitles := []string{
		" [ominous music playing]",
		" [confusing noises]",
		" [awkward silence]",
		" [dramatic pause]",
		" [indistinct chatter]",
	}
	return text + subtitles[rand.Intn(len(subtitles))]
}

// applySpotlight adds an announcement prefix
func applySpotlight(text string) string {
	return "📣 EVERYONE LOOK: " + text
}

// ApplyPunishmentToText applies a punishment effect to text
func ApplyPunishmentToText(text string, pType PunishmentType) string {
	switch pType {
	case PunishmentBackward:
		return applyBackward(text)
	case PunishmentStutterstep:
		return applyStutterstep(text)
	case PunishmentElongate:
		return applyElongate(text)
	case PunishmentUppercase:
		return applyUppercase(text)
	case PunishmentLowercase:
		return applyLowercase(text)
	case PunishmentRobotic:
		return applyRobotic(text)
	case PunishmentAlternating:
		return applyAlternating(text)
	case PunishmentFancy:
		return applyFancy(text)
	case PunishmentUwu:
		return applyUwu(text)
	case PunishmentPirate:
		return applyPirate(text)
	case PunishmentShakespearean:
		return applyShakespearean(text)
	case PunishmentCaveman:
		return applyCaveman(text)
	case PunishmentCensor:
		return applyCensor(text)
	case PunishmentConfused:
		return applyConfused(text)
	case PunishmentParanoid:
		return applyParanoid(text)
	case PunishmentDrunk:
		return applyDrunk(text)
	case PunishmentHiccup:
		return applyHiccup(text)
	case PunishmentWhistle:
		return applyWhistle(text)
	case PunishmentMumble:
		return applyMumble(text)
	case PunishmentSpaghetti:
		return applySpaghetti(text)
	case PunishmentRng:
		return applyRng(text)
	case PunishmentEssay:
		return applyEssay(text)
	case PunishmentAutospell:
		return applyAutospell(text)
	case PunishmentSubtitles:
		return applySubtitles(text)
	case PunishmentSpotlight:
		return applySpotlight(text)
	case PunishmentMonkey:
		return applyMonkey(text)
	case PunishmentSnake:
		return applySnake(text)
	case PunishmentDog:
		return applyDog(text)
	case PunishmentCat:
		return applyCat(text)
	case PunishmentBird:
		return applyBird(text)
	case PunishmentCow:
		return applyCow(text)
	case PunishmentFrog:
		return applyFrog(text)
	case PunishmentDuck:
		return applyDuck(text)
	case PunishmentHorse:
		return applyHorse(text)
	case PunishmentLion:
		return applyLion(text)
	case PunishmentZoo:
		return applyZoo(text)
	case PunishmentBunny:
		return applyBunny(text)
	case PunishmentTsundere:
		return applyTsundere(text)
	case PunishmentYandere:
		return applyYandere(text)
	case PunishmentKuudere:
		return applyKuudere(text)
	case PunishmentDandere:
		return applyDandere(text)
	case PunishmentDeredere:
		return applyDeredere(text)
	case PunishmentHimedere:
		return applyHimedere(text)
	case PunishmentKamidere:
		return applyKamidere(text)
	case PunishmentUndere:
		return applyUndere(text)
	case PunishmentBakadere:
		return applyBakadere(text)
	case PunishmentMayadere:
		return applyMayadere(text)
	case PunishmentEmoticon:
		return applyEmoticon(text)
	default:
		return text
	}
}

// ApplyPunishmentToTextWithState applies a punishment effect with state tracking
func ApplyPunishmentToTextWithState(text string, pType PunishmentType, state *PunishmentState) string {
	switch pType {
	case PunishmentTorment:
		// Cycle through effects based on message count
		result := applyTorment(text, state.lastEffect)
		state.lastEffect++
		return result
	default:
		return ApplyPunishmentToText(text, pType)
	}
}

// applyMonkey replaces text with monkey noises
func applyMonkey(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "OOH OOH AHH AHH"
	}
	monkeySounds := []string{"ook", "eek", "ooh ooh", "ahh ahh", "oo oo", "ee ee", "*scratches head*", "*swings from tree*"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(monkeySounds[rand.Intn(len(monkeySounds))])
	}
	return truncateText(result.String())
}

// applySnake replaces s sounds with extended hissing
func applySnake(text string) string {
	text = strings.ReplaceAll(text, "s", "sss")
	text = strings.ReplaceAll(text, "S", "SSS")
	if rand.Float32() < 0.5 {
		suffixes := []string{" *hisss*", " ssss...", " ~hisssss~"}
		text += suffixes[rand.Intn(len(suffixes))]
	}
	return truncateText(text)
}

// applyDog replaces text with dog sounds
func applyDog(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "WOOF!"
	}
	dogSounds := []string{"woof", "arf", "grr", "bark!", "ruff", "yip", "*wags tail*", "bork"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(dogSounds[rand.Intn(len(dogSounds))])
	}
	return truncateText(result.String())
}

// applyCat replaces text with cat sounds
func applyCat(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "meow~"
	}
	catSounds := []string{"meow", "purrr~", "mrrrow", "mew", "nya~", "*purrs*", "prrrr", "mrrr"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(catSounds[rand.Intn(len(catSounds))])
	}
	return truncateText(result.String())
}

// applyBird replaces text with bird sounds
func applyBird(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "tweet!"
	}
	birdSounds := []string{"tweet", "chirp", "squawk", "cheep", "coo coo", "*flaps wings*", "peep", "caw"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(birdSounds[rand.Intn(len(birdSounds))])
	}
	return truncateText(result.String())
}

// applyCow replaces text with cow sounds
func applyCow(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "MOO"
	}
	cowSounds := []string{"moo", "mooo", "MOOO", "moooo", "*chews cud*", "muu", "MOO MOO"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(cowSounds[rand.Intn(len(cowSounds))])
	}
	return truncateText(result.String())
}

// applyFrog replaces text with frog sounds
func applyFrog(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "ribbit!"
	}
	frogSounds := []string{"ribbit", "croak", "brrr-ribbit", "riiibbit", "*jumps*", "crrroak", "ribbit-ribbit"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(frogSounds[rand.Intn(len(frogSounds))])
	}
	return truncateText(result.String())
}

// applyDuck replaces text with duck sounds
func applyDuck(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "QUACK!"
	}
	duckSounds := []string{"quack", "QUACK", "quack!", "quack quack", "*waddles*", "QUACK!", "QUACK QUACK"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(duckSounds[rand.Intn(len(duckSounds))])
	}
	return truncateText(result.String())
}

// applyHorse replaces text with horse sounds
func applyHorse(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "NEIGH!"
	}
	horseSounds := []string{"neigh", "whinny", "nicker", "NEIGH!", "*clip clop*", "hrrrr", "snort"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(horseSounds[rand.Intn(len(horseSounds))])
	}
	return truncateText(result.String())
}

// applyLion replaces text with lion sounds
func applyLion(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "ROAR!"
	}
	lionSounds := []string{"ROAR", "grrr", "rawr", "GRRR", "*snarls*", "rrrroar", "RAWRR"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(lionSounds[rand.Intn(len(lionSounds))])
	}
	return truncateText(result.String())
}

// applyZoo applies a random animal punishment from the full zoo
func applyZoo(text string) string {
	animalEffects := []func(string) string{
		applyMonkey,
		applySnake,
		applyDog,
		applyCat,
		applyBird,
		applyCow,
		applyFrog,
		applyDuck,
		applyHorse,
		applyLion,
		applyBunny,
	}
	effect := animalEffects[rand.Intn(len(animalEffects))]
	return effect(text)
}

// applyBunny replaces text with bunny sounds
func applyBunny(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "*thump thump*"
	}
	bunnySounds := []string{"*thump*", "*thump thump*", "*nose twitch*", "*hops away*", "*binky!*", "*flops*", "*teeth chattering*", "*nudges*"}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(bunnySounds[rand.Intn(len(bunnySounds))])
	}
	return truncateText(result.String())
}

// GetRandomEmoji returns a random emoji string
func GetRandomEmoji() string {
	emojis := []string{"😀", "😎", "🤡", "👻", "🎃", "🦄", "🐱", "🐶", "🎮", "⭐"}
	return emojis[rand.Intn(len(emojis))]
}

// ── Dere-type punishments ────────────────────────────────────────────────────
// All phrase tables are package-level vars — allocated once at startup, never
// on the hot-path (every IC message). Each archetype has 8 entries so a
// single rand.Intn(8) selects uniformly without an extra len() call.

var (
	tsunderePfx = []string{
		"H-Hmph! It's not like I care, but fine, I'll say it: ",
		"D-Don't you DARE misunderstand, this is purely informational: ",
		"*crosses arms* B-baka! I only said this because I had to!! ",
		"*looks away sharply* ...Fine. FINE. If you must know: ",
		"*goes bright red* I-I am NOT flustered right now!! Anyway: ",
		"*stamps foot* W-whatever! Just listen already!! ",
		"You'd better not read into this, i-idiot!! ",
		"*huffs loudly* I was going to stay QUIET, but: ",
	}
	tsundereSfx = []string{
		" ...N-not that I even cared about saying it!!",
		" ...b-b-BAKA!!",
		" D-Don't you DARE get any weird ideas!!",
		" *turns away furiously* I-It's NOTHING, forget I said it!!",
		" *goes even redder* S-stop LOOKING at me like that!!",
		" ...Idiot. Utter and complete idiot.",
		" I-It's not like I WANTED you to hear that!!",
		" *mutters* ...why is my face so hot right now?!",
	}

	yanderePfx = []string{
		"My beloved~ ♥ Listen very, very carefully... ",
		"Hehehe~ nobody ELSE is allowed to hear this but you~ ",
		"I've been watching you for sooo long now, and ",
		"You're MINE and only mine, so you need to know: ",
		"*stabs diary lovingly* Okay, I am completely calm now... ",
		"Fufu~ ♥ I saved this secret just for you, my darling: ",
		"*clutches photo of you tightly* There's something I need to say: ",
		"*counts down from ten, still smiling* Right. So. Calmly. ",
	}
	yandereSfx = []string{
		" ...You won't leave me, right? You simply CAN'T. ♥",
		" If you betray me I will always find you~ ♥ Always. ♥",
		" Hehehehe~ ♥♥♥",
		" Remember: you belong to me and ONLY me, forever. ♥",
		" ...I love you so, SO much. It genuinely isn't normal. ♥",
		" I've already memorised your entire daily schedule, by the way. ♥",
		" ...The last person who said that to someone else... hehe~ ♥",
		" Our love is eternal~ just like the names I carved into this tree. ♥",
	}

	kuuderePfx = []string{
		"...",
		"*monotone voice* ",
		"*stares blankly* ",
		"*zero facial expression* ",
		"",
		"Mm. ",
		"[Processing] ",
		"*single blink* ",
	}
	kuudereSfx = []string{
		". Acknowledged.",
		". I understand.",
		". Noted.",
		". That is all.",
		". Affirmative.",
		". Data received. Nothing to add.",
		". I have no further comment.",
		". This interaction is complete.",
	}

	dandereSttrs = []string{"u-um... ", "a-ah... ", "s-sorry, ", "...uh... ", "e-err... ", "i-i mean... ", "w-wait... ", "oh, um, "}
	dandereSfx   = []string{
		"... s-sorry for talking so much...",
		"... if that's okay with you...",
		"... p-please ignore me...",
		"...",
		"... I probably shouldn't have said that...",
		"... sorry, just forget I said anything...",
		"... *stares at shoes*",
		"... y-you didn't hear that, right...?",
	}

	deredere_pfx = []string{
		"Kyaa~!! ♥♥♥ ",
		"Oh my gosh you are SO amazing!! ♥ ",
		"I love EVERYONE so much right now!! ♥ ",
		"*sparkles intensify* You are literally the best!! ♥ ",
		"EEE I'm so insanely happy~!! ♥ ",
		"AHHH this is too adorable I can't even~!! ♥ ",
		"You make my heart go DOKI DOKI!! ♥ ",
		"I want to hug the entire WORLD right now~!! ♥♥ ",
	}
	deredere_sfx = []string{
		" ♥♥♥ You are SO so wonderful!!!",
		" I love you ALL so incredibly much~!! ♥",
		" ~(*^▽^*)~ ♥♥♥",
		" ♥ This is literally THE BEST DAY EVER!!",
		" EEE so absolutely amazing~!! ♥",
		" *happy tears* ♥♥♥",
		" I could literally BURST with happiness~!! ♥",
		" YES YES YES ♥♥♥!!",
	}

	himederePfx = []string{
		"Hmph! As expected of one such as I, I decree: ",
		"Listen well, commoner. ",
		"You should be honoured that I, in my infinite grace, declare: ",
		"Bow before me as I bestow this announcement: ",
		"As your undisputed superior in every conceivable way, I shall inform you: ",
		"We, in our immeasurable benevolence, have deigned to speak: ",
		"*adjusts crown with one finger* The royal decree is thus: ",
		"It is beneath me to repeat myself, so attend carefully: ",
	}
	himedereSfx = []string{
		" Now kneel!",
		" Understand, peasant?",
		" You are welcome. Bow.",
		" That is my royal decree. There shall be no further discussion.",
		" Do not keep me waiting next time, commoner.",
		" Naturally, I expect your eternal gratitude.",
		" The royal audience is now dismissed.",
		" Hmph. You had best remember every word.",
	}

	kamiderePfx = []string{
		"KNEEL! The divine one speaks: ",
		"Silence, insignificant mortals. I, a being of transcendent intellect, proclaim: ",
		"You are wholly unworthy to receive these words, yet I bestow them: ",
		"This universe exists solely for my amusement. Hear me now: ",
		"As inscribed in the very fabric of the cosmos, I declare: ",
		"⚡ The heavens themselves tremble as I utter: ",
		"I have descended from on high to enlighten you insects: ",
		"Your feeble minds cannot possibly comprehend me, yet I speak: ",
	}
	kamidereSfx = []string{
		" It is so because I will it. ⚡",
		" Rejoice that I even acknowledged your existence.",
		" You may thank me at your earliest convenience.",
		" I am never wrong. I have never been wrong. I will never be wrong. ⚡",
		" Worship me accordingly.",
		" ⚡ Bow and despair at my absolute magnificence.",
		" This is not a suggestion.",
		" I shall expect a suitable shrine erected by morning.",
	}

	underePfx = []string{
		"Yes!! Absolutely!! ",
		"You are so right, and also: ",
		"That is completely valid!! And: ",
		"I agree one hundred percent! ",
		"OMG YES and: ",
		"Exactly what I was already thinking!! ",
		"You are SO correct and: ",
		"I could NOT agree more!! ",
	}
	undereSfx = []string{
		" ...and I agree with every single word of that too!!",
		" ...yes, yes, a thousand times YES!!",
		" ...you are so right about literally everything, always!!",
		" ...whatever you say is absolutely flawless!!",
		" ...I would never in a million years disagree with you~",
		" ...that is genuinely the best thing I have ever heard!!",
		" ...I was literally JUST thinking that exact thing!!",
		" ...you are basically always right about everything, forever.",
	}

	bakadereIntj = []string{"*trips*", "*bumps into wall*", "ehehe~", "*drops everything*", "*falls over*", "uuu~", "*knocks over cup*", "*gets tangled in own feet*"}
	bakadereEnd  = []string{
		" ...ehehe~",
		" *bumps into doorframe on the way out*",
		" Uuu, gomen gomen~",
		" *accidentally knocks something over*",
		" *falls off chair*",
		" hehe... o-oops?",
		" *trips over nothing at all*",
		" ehehe, I did it again didn't I~",
	}

	mayaderePfx = []string{
		"...The shadows specifically told me to tell you: ",
		"Kukuku~ How deliciously intriguing... ",
		"*materialises from thin air behind you* ",
		"How very curious that you would say such a thing: ",
		"I have foreseen this precise moment in detail... ",
		"*tilts head at an unnerving angle* Did you know: ",
		"The cards spoke of this very message. They said: ",
		"Fufu~ your fate was sealed the moment you thought to say: ",
	}
	mayadereSfx = []string{
		" ...Just as I calculated.",
		" ...How deliciously entertaining.",
		" *dissolves back into the shadows*",
		" ...The stars have already confirmed it.",
		" Kukuku~ most fascinating.",
		" ...I have known this would happen for a very long time.",
		" *smiles in a way that doesn't reach the eyes*",
		" ...Everything is proceeding precisely as I planned.",
	}
)

// applyTsundere wraps text in classic tsundere denial and blush reactions.
func applyTsundere(text string) string {
	return tsunderePfx[rand.Intn(len(tsunderePfx))] + text + tsundereSfx[rand.Intn(len(tsundereSfx))]
}

// applyYandere wraps text in obsessive, unhinged yandere flavour.
func applyYandere(text string) string {
	return yanderePfx[rand.Intn(len(yanderePfx))] + text + yandereSfx[rand.Intn(len(yandereSfx))]
}

// applyKuudere flattens text into deadpan, emotionless kuudere delivery.
// A single strings.Map pass lowercases and strips emotional punctuation (!~?).
func applyKuudere(text string) string {
	text = strings.Map(func(r rune) rune {
		switch r {
		case '!', '~':
			return -1 // drop excitement markers
		case '?':
			return '.' // questions become flat statements
		}
		return unicode.ToLower(r)
	}, text)
	return kuuderePfx[rand.Intn(len(kuuderePfx))] + text + kuudereSfx[rand.Intn(len(kuudereSfx))]
}

// applyDandere makes text extremely shy: phrase stutters every few words,
// letter stutters on individual words, and a trailing hesitation suffix.
func applyDandere(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "...u-um..."
	}
	var sb strings.Builder
	sb.Grow(len(text)*2 + 48)
	for i, w := range words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if i%3 == 0 && i != 0 {
			sb.WriteString(dandereSttrs[rand.Intn(len(dandereSttrs))])
		}
		// Stutter first ASCII letter of the word for extra shyness.
		if len(w) > 0 && w[0] >= 'a' && w[0] <= 'z' && rand.Intn(3) == 0 {
			sb.WriteByte(w[0])
			sb.WriteByte('-')
		}
		sb.WriteString(w)
	}
	sb.WriteString(dandereSfx[rand.Intn(len(dandereSfx))])
	return truncateText(sb.String())
}

// applyDeredere wraps text in over-the-top lovey-dovey sweetness.
func applyDeredere(text string) string {
	return deredere_pfx[rand.Intn(len(deredere_pfx))] + text + deredere_sfx[rand.Intn(len(deredere_sfx))]
}

// applyHimedere makes the speaker act like imperious royalty.
func applyHimedere(text string) string {
	return himederePfx[rand.Intn(len(himederePfx))] + text + himedereSfx[rand.Intn(len(himedereSfx))]
}

// applyKamidere makes the speaker act like a self-proclaimed god.
func applyKamidere(text string) string {
	return kamiderePfx[rand.Intn(len(kamiderePfx))] + text + kamidereSfx[rand.Intn(len(kamidereSfx))]
}

// applyUndere makes the speaker agree with absolutely everything.
func applyUndere(text string) string {
	return underePfx[rand.Intn(len(underePfx))] + text + undereSfx[rand.Intn(len(undereSfx))]
}

// applyBakadere inserts clumsy accident interjections into the message.
func applyBakadere(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "*trips* Ehehe~"
	}
	var sb strings.Builder
	sb.Grow(len(text)*2 + 32)
	for i, w := range words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if rand.Intn(4) == 0 {
			sb.WriteString(bakadereIntj[rand.Intn(len(bakadereIntj))])
			sb.WriteByte(' ')
		}
		sb.WriteString(w)
	}
	sb.WriteString(bakadereEnd[rand.Intn(len(bakadereEnd))])
	return truncateText(sb.String())
}

// applyMayadere gives text an eerie, enigmatic, mysterious quality.
func applyMayadere(text string) string {
	return mayaderePfx[rand.Intn(len(mayaderePfx))] + text + mayadereSfx[rand.Intn(len(mayadereSfx))]
}

var emoticons = []string{
	":)", ":D", ":P", ":(", ";)", ":3", ":O", "xD", ">:)", "o_o",
	"^_^", ":T", ">_<", "UwU", "OwO", "T_T", "x_x", "-_-", ":>", ":|",
	";D", "B)", ">.<", "c:", ":c", ":*", ":')", ":'(", "^.^", "o_O",
}

// applyEmoticon replaces the message with a random emoticon.
func applyEmoticon(text string) string {
	return emoticons[rand.Intn(len(emoticons))]
}
