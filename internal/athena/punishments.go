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

// applyWhisper makes text only visible to mods (returned as empty for now, handled elsewhere)
func applyWhisper(text string) string {
	return text // Visibility handling done in broadcast logic
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
		"you":   "thou",
		"your":  "thy",
		"yours": "thine",
		"are":   "art",
		"yes":   "aye",
		"no":    "nay",
	}
	
	words := strings.Fields(text)
	for i, word := range words {
		lower := strings.ToLower(word)
		if replacement, ok := replacements[lower]; ok {
			words[i] = replacement
		}
	}
	
	result := strings.Join(words, " ")
	if rand.Float32() < 0.2 {
		result = "Hark! " + result
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
	words := strings.Fields(text)
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

// applyHaiku adds a note about haiku format
func applyHaiku(text string) string {
	// This is a validation, not a transformation
	// The actual validation should happen in message handling
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

// applyCopycats applies user-specific alterations to text
// The alterations should be consistent per user but different from the original
func applyCopycats(text string, userID int) string {
	if text == "" {
		return text
	}
	
	// Use user ID to seed which letters to double
	// This ensures each user has consistent but different alterations
	runes := []rune(text)
	var result strings.Builder
	
	// Determine doubling pattern based on user ID
	// Use modulo to create a pattern for which characters to double
	doublePattern := (userID % 5) + 2 // Doubles characters at intervals of 2-6 positions
	doubleOffset := userID % doublePattern // Offset within the pattern
	
	for i, r := range runes {
		result.WriteRune(r)
		// Double certain letters based on user ID pattern
		// Check if this position matches the user's doubling offset
		// Skip position 0 to avoid doubling the first character (often capitalized)
		if i > 0 && i%doublePattern == doubleOffset && unicode.IsLetter(r) {
			result.WriteRune(r)
		}
	}
	
	return truncateText(result.String())
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
	case PunishmentWhisper:
		return applyWhisper(text)
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
	case PunishmentHaiku:
		return applyHaiku(text)
	case PunishmentAutospell:
		return applyAutospell(text)
	case PunishmentSubtitles:
		return applySubtitles(text)
	case PunishmentSpotlight:
		return applySpotlight(text)
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

// ApplyPunishmentToTextWithUserID applies a punishment effect that requires user ID
func ApplyPunishmentToTextWithUserID(text string, pType PunishmentType, userID int) string {
	switch pType {
	case PunishmentCopycats:
		return applyCopycats(text, userID)
	default:
		return ApplyPunishmentToText(text, pType)
	}
}

// GetRandomName generates a random silly name
func GetRandomName() string {
	adjectives := []string{"Silly", "Goofy", "Wacky", "Bonkers", "Zany", "Quirky", "Absurd"}
	nouns := []string{"Banana", "Potato", "Noodle", "Pickle", "Waffle", "Muffin", "Pancake"}
	return adjectives[rand.Intn(len(adjectives))] + " " + nouns[rand.Intn(len(nouns))]
}

// GetRandomEmoji returns a random emoji string
func GetRandomEmoji() string {
	emojis := []string{"😀", "😎", "🤡", "👻", "🎃", "🦄", "🐱", "🐶", "🎮", "⭐"}
	return emojis[rand.Intn(len(emojis))]
}
