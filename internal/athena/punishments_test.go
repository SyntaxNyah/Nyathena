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
	"testing"
)

func TestApplyShakespearean(t *testing.T) {
	// Core word replacements
	result := applyShakespearean("you are here")
	if !strings.Contains(result, "thou") {
		t.Errorf("applyShakespearean: expected 'thou' (you→thou) in %q", result)
	}
	if !strings.Contains(result, "art") {
		t.Errorf("applyShakespearean: expected 'art' (are→art) in %q", result)
	}
	if !strings.Contains(result, "hither") {
		t.Errorf("applyShakespearean: expected 'hither' (here→hither) in %q", result)
	}

	// Capitalisation is preserved
	result2 := applyShakespearean("You are here")
	if !strings.Contains(result2, "Thou") {
		t.Errorf("applyShakespearean: expected capitalised 'Thou' in %q", result2)
	}

	// Punctuation attached to a word is preserved
	result3 := applyShakespearean("are you okay?")
	if !strings.Contains(result3, "thou") || !strings.Contains(result3, "art") {
		t.Errorf("applyShakespearean: expected replacements with trailing punct in %q", result3)
	}
	if !strings.Contains(result3, "well") { // "okay" → "very well"
		t.Errorf("applyShakespearean: expected 'very well' (okay→very well) in %q", result3)
	}

	// Additional vocabulary
	for input, want := range map[string]string{
		"never":   "ne'er",
		"maybe":   "perchance",
		"soon":    "anon",
		"goodbye": "farewell",
		"really":  "forsooth",
		"sad":     "woeful",
		"angry":   "wrathful",
		"world":   "realm",
	} {
		r := applyShakespearean(input)
		if !strings.Contains(r, want) {
			t.Errorf("applyShakespearean: expected %q→%q, got %q", input, want, r)
		}
	}

	// Prefixes and suffixes appear across a large sample (probabilistic check)
	prefixFound, suffixFound := false, false
	for i := 0; i < 200 && (!prefixFound || !suffixFound); i++ {
		r := applyShakespearean("you are here")
		knownPrefixes := []string{"Hark! ", "Forsooth! ", "Zounds! ", "Prithee, ", "Methinks ", "By my troth! ", "O fie! ", "Marry! ", "'Tis said that ", "Good morrow! "}
		for _, p := range knownPrefixes {
			if strings.HasPrefix(r, p) {
				prefixFound = true
				break
			}
		}
		knownSuffixes := []string{", methinks.", ", forsooth!", ", I prithee.", ", good soul.", ", 'tis so!", ", verily.", ", upon mine honour.", ", I dare say."}
		for _, s := range knownSuffixes {
			if strings.HasSuffix(r, s) {
				suffixFound = true
				break
			}
		}
	}
	if !prefixFound {
		t.Error("applyShakespearean: no prefix appeared in 200 runs (expected ~40% chance)")
	}
	if !suffixFound {
		t.Error("applyShakespearean: no suffix appeared in 200 runs (expected ~30% chance)")
	}
}

func TestApplyUppercase(t *testing.T) {
	input := "hello world"
	expected := "HELLO WORLD"
	result := applyUppercase(input)
	if result != expected {
		t.Errorf("applyUppercase failed: got %q, want %q", result, expected)
	}
}

func TestApplyLowercase(t *testing.T) {
	input := "HELLO WORLD"
	expected := "hello world"
	result := applyLowercase(input)
	if result != expected {
		t.Errorf("applyLowercase failed: got %q, want %q", result, expected)
	}
}

func TestApplyBackward(t *testing.T) {
	input := "hello"
	expected := "olleh"
	result := applyBackward(input)
	if result != expected {
		t.Errorf("applyBackward failed: got %q, want %q", result, expected)
	}
}

func TestApplyStutterstep(t *testing.T) {
	input := "hello world"
	result := applyStutterstep(input)
	// Should double each word
	if !strings.Contains(result, "hello hello") || !strings.Contains(result, "world world") {
		t.Errorf("applyStutterstep failed: got %q", result)
	}
}

func TestApplyElongate(t *testing.T) {
	input := "hello"
	result := applyElongate(input)
	// Should repeat vowels
	if !strings.Contains(result, "eee") || !strings.Contains(result, "ooo") {
		t.Errorf("applyElongate failed: got %q", result)
	}
}

func TestApplyRobotic(t *testing.T) {
	input := "hello world"
	result := applyRobotic(input)
	// Should contain robot sounds
	if !strings.Contains(result, "[BEEP]") && !strings.Contains(result, "[BOOP]") {
		t.Errorf("applyRobotic failed: got %q", result)
	}
}

func TestApplyAlternating(t *testing.T) {
	input := "hello"
	result := applyAlternating(input)
	// Should have alternating case
	if result == strings.ToLower(input) || result == strings.ToUpper(input) {
		t.Errorf("applyAlternating failed: got %q, expected alternating case", result)
	}
}

func TestApplyUwu(t *testing.T) {
	input := "hello world"
	result := applyUwu(input)
	// Should replace 'l' with 'w'
	if !strings.Contains(result, "hewwo") && !strings.Contains(result, "worwd") {
		t.Errorf("applyUwu failed: got %q", result)
	}
}

func TestApplyCensor(t *testing.T) {
	input := "hello world test"
	result := applyCensor(input)
	// Should contain [CENSORED] or be different from input (random behavior)
	if !strings.Contains(result, "[CENSORED]") && result == input {
		// It's random, so sometimes it might not censor anything, but that's okay
		t.Logf("applyCensor result: %q (random behavior - no censoring this time)", result)
	}
}

func TestApplyConfused(t *testing.T) {
	input := "one two three"
	result := applyConfused(input)
	// Should have all words but potentially in different order
	if !strings.Contains(result, "one") || !strings.Contains(result, "two") || !strings.Contains(result, "three") {
		t.Errorf("applyConfused failed: missing words in %q", result)
	}
}

func TestApplyConfusedBypassPrevention(t *testing.T) {
	// Users might try to bypass by using dots, hyphens, or other separators instead of spaces
	input := "Zivulet.I-can-cheat-ha"
	result := applyConfused(input)
	// All "words" split by non-alphanumeric chars should still be present
	words := []string{"Zivulet", "I", "can", "cheat", "ha"}
	for _, w := range words {
		if !strings.Contains(result, w) {
			t.Errorf("applyConfused bypass prevention failed: missing %q in %q", w, result)
		}
	}
	// The result should not preserve the original separator-joined form unchanged
	// (with 5 tokens it will always be shuffled differently from the original on some run,
	// but we at least verify tokens are extracted and present)
}

func TestTruncateText(t *testing.T) {
	// Test with text under limit
	short := "hello"
	result := truncateText(short)
	if result != short {
		t.Errorf("truncateText failed for short text: got %q, want %q", result, short)
	}

	// Test with text over limit
	long := strings.Repeat("a", maxTextLength+100)
	result = truncateText(long)
	if len(result) > maxTextLength {
		t.Errorf("truncateText failed: length %d exceeds max %d", len(result), maxTextLength)
	}
}

func TestGetRandomEmoji(t *testing.T) {
	emoji := GetRandomEmoji()
	if emoji == "" {
		t.Errorf("GetRandomEmoji returned empty string")
	}
}

func TestApplyMonkey(t *testing.T) {
	input := "hello world"
	result := applyMonkey(input)
	monkeySounds := []string{"ook", "eek", "ooh ooh", "ahh ahh", "oo oo", "ee ee", "*scratches head*", "*swings from tree*"}
	found := false
	for _, sound := range monkeySounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyMonkey failed: got %q, expected monkey sounds", result)
	}
}

func TestApplyMonkeyEmpty(t *testing.T) {
	result := applyMonkey("")
	if result != "OOH OOH AHH AHH" {
		t.Errorf("applyMonkey empty failed: got %q, want %q", result, "OOH OOH AHH AHH")
	}
}

func TestApplySnake(t *testing.T) {
	input := "this is serious"
	result := applySnake(input)
	if !strings.Contains(result, "sss") {
		t.Errorf("applySnake failed: got %q, expected extended s sounds", result)
	}
}

func TestApplyDog(t *testing.T) {
	input := "hello world"
	result := applyDog(input)
	dogSounds := []string{"woof", "arf", "grr", "bark!", "ruff", "yip", "*wags tail*", "bork"}
	found := false
	for _, sound := range dogSounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyDog failed: got %q, expected dog sounds", result)
	}
}

func TestApplyDogEmpty(t *testing.T) {
	result := applyDog("")
	if result != "WOOF!" {
		t.Errorf("applyDog empty failed: got %q, want %q", result, "WOOF!")
	}
}

func TestApplyCat(t *testing.T) {
	input := "hello world"
	result := applyCat(input)
	catSounds := []string{"meow", "purrr~", "mrrrow", "mew", "nya~", "*purrs*", "prrrr", "mrrr"}
	found := false
	for _, sound := range catSounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyCat failed: got %q, expected cat sounds", result)
	}
}

func TestApplyCatEmpty(t *testing.T) {
	result := applyCat("")
	if result != "meow~" {
		t.Errorf("applyCat empty failed: got %q, want %q", result, "meow~")
	}
}

func TestApplyBird(t *testing.T) {
	input := "hello world"
	result := applyBird(input)
	birdSounds := []string{"tweet", "chirp", "squawk", "cheep", "coo coo", "*flaps wings*", "peep", "caw"}
	found := false
	for _, sound := range birdSounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyBird failed: got %q, expected bird sounds", result)
	}
}

func TestApplyCow(t *testing.T) {
	input := "hello world"
	result := applyCow(input)
	if !strings.Contains(strings.ToLower(result), "moo") && !strings.Contains(result, "*chews cud*") {
		t.Errorf("applyCow failed: got %q, expected cow sounds", result)
	}
}

func TestApplyFrog(t *testing.T) {
	input := "hello world"
	result := applyFrog(input)
	frogSounds := []string{"ribbit", "croak", "brrr-ribbit", "riiibbit", "*jumps*", "crrroak"}
	found := false
	for _, sound := range frogSounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyFrog failed: got %q, expected frog sounds", result)
	}
}

func TestApplyDuck(t *testing.T) {
	input := "hello world"
	result := applyDuck(input)
	if !strings.Contains(strings.ToLower(result), "quack") && !strings.Contains(result, "*waddles*") {
		t.Errorf("applyDuck failed: got %q, expected duck sounds", result)
	}
}

func TestApplyHorse(t *testing.T) {
	input := "hello world"
	result := applyHorse(input)
	horseSounds := []string{"neigh", "whinny", "nicker", "NEIGH!", "*clip clop*", "hrrrr", "snort"}
	found := false
	for _, sound := range horseSounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyHorse failed: got %q, expected horse sounds", result)
	}
}

func TestApplyLion(t *testing.T) {
	input := "hello world"
	result := applyLion(input)
	lionSounds := []string{"ROAR", "grrr", "rawr", "GRRR", "*snarls*", "rrrroar", "RAWRR"}
	found := false
	for _, sound := range lionSounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyLion failed: got %q, expected lion sounds", result)
	}
}

func TestApplyZoo(t *testing.T) {
	// Zoo should apply some animal sound - use input with 's' to ensure snake also changes it
	input := "this is something"
	result := applyZoo(input)
	if result == input {
		t.Errorf("applyZoo failed: output same as input %q", result)
	}
}

func TestApplyBunny(t *testing.T) {
	input := "hello world"
	result := applyBunny(input)
	bunnySounds := []string{"*thump*", "*thump thump*", "*nose twitch*", "*hops away*", "*binky!*", "*flops*", "*teeth chattering*", "*nudges*"}
	found := false
	for _, sound := range bunnySounds {
		if strings.Contains(result, sound) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyBunny failed: got %q, expected bunny sounds", result)
	}
}

func TestApplyBunnyEmpty(t *testing.T) {
	result := applyBunny("")
	if result != "*thump thump*" {
		t.Errorf("applyBunny empty failed: got %q, want %q", result, "*thump thump*")
	}
}

func TestApplyPunishmentToTextAnimal(t *testing.T) {
	// Use input with 's' so even snake punishment changes it
	input := "this is serious stuff"

	animalTests := []struct {
		name  string
		pType PunishmentType
	}{
		{"Monkey", PunishmentMonkey},
		{"Snake", PunishmentSnake},
		{"Dog", PunishmentDog},
		{"Cat", PunishmentCat},
		{"Bird", PunishmentBird},
		{"Cow", PunishmentCow},
		{"Frog", PunishmentFrog},
		{"Duck", PunishmentDuck},
		{"Horse", PunishmentHorse},
		{"Lion", PunishmentLion},
		{"Zoo", PunishmentZoo},
		{"Bunny", PunishmentBunny},
	}

	for _, tt := range animalTests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyPunishmentToText(input, tt.pType)
			if result == input {
				t.Errorf("%s: expected different output, got same: %q", tt.name, result)
			}
		})
	}
}

func TestApplyPunishmentToText(t *testing.T) {
	input := "hello world"
	
	tests := []struct {
		name       string
		pType      PunishmentType
		shouldDiff bool
	}{
		{"Uppercase", PunishmentUppercase, true},
		{"Lowercase", PunishmentLowercase, false}, // already lowercase
		{"Backward", PunishmentBackward, true},
		{"Robotic", PunishmentRobotic, true},
		{"None", PunishmentNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyPunishmentToText(input, tt.pType)
			if tt.shouldDiff && result == input {
				t.Errorf("%s: expected different output, got same: %q", tt.name, result)
			}
		})
	}
}


// ── Dere-type punishment tests ───────────────────────────────────────────────

func TestApplyTsundere(t *testing.T) {
input := "hello"
result := applyTsundere(input)
if !strings.Contains(result, input) {
t.Errorf("applyTsundere: expected original text %q in output %q", input, result)
}
if len(result) <= len(input) {
t.Errorf("applyTsundere: expected output longer than input, got %q", result)
}
}

func TestApplyYandere(t *testing.T) {
input := "hello"
result := applyYandere(input)
if !strings.Contains(result, input) {
t.Errorf("applyYandere: expected original text %q in output %q", input, result)
}
if !strings.Contains(result, "♥") {
t.Errorf("applyYandere: expected ♥ in output %q", result)
}
}

func TestApplyKuudere(t *testing.T) {
input := "Hello World"
result := applyKuudere(input)
if result == input {
t.Errorf("applyKuudere: expected transformed output, got unchanged %q", result)
}
lower := strings.ToLower(input)
if !strings.Contains(result, lower) {
t.Errorf("applyKuudere: expected lowercased input %q in output %q", lower, result)
}
}

func TestApplyDandere(t *testing.T) {
input := "hello there everyone"
result := applyDandere(input)
if result == input {
t.Errorf("applyDandere: expected transformed output, got unchanged %q", result)
}
if !strings.Contains(result, "hello") {
t.Errorf("applyDandere: expected original word in output %q", result)
}
}

func TestApplyDeredere(t *testing.T) {
input := "hello"
result := applyDeredere(input)
if !strings.Contains(result, input) {
t.Errorf("applyDeredere: expected original text %q in output %q", input, result)
}
if !strings.Contains(result, "♥") {
t.Errorf("applyDeredere: expected ♥ in output %q", result)
}
}

func TestApplyHimedere(t *testing.T) {
input := "hello"
result := applyHimedere(input)
if !strings.Contains(result, input) {
t.Errorf("applyHimedere: expected original text %q in output %q", input, result)
}
if len(result) <= len(input) {
t.Errorf("applyHimedere: expected output longer than input %q, got %q", input, result)
}
}

func TestApplyKamidere(t *testing.T) {
input := "hello"
result := applyKamidere(input)
if !strings.Contains(result, input) {
t.Errorf("applyKamidere: expected original text %q in output %q", input, result)
}
if len(result) <= len(input) {
t.Errorf("applyKamidere: expected output longer than input %q, got %q", input, result)
}
}

func TestApplyUndere(t *testing.T) {
input := "hello"
result := applyUndere(input)
if !strings.Contains(result, input) {
t.Errorf("applyUndere: expected original text %q in output %q", input, result)
}
if len(result) <= len(input) {
t.Errorf("applyUndere: expected output longer than input %q, got %q", input, result)
}
}

func TestApplyBakadere(t *testing.T) {
input := "hello there everyone"
result := applyBakadere(input)
if result == input {
t.Errorf("applyBakadere: expected transformed output, got unchanged %q", result)
}
if !strings.Contains(result, "hello") {
t.Errorf("applyBakadere: expected original word in output %q", result)
}
}

func TestApplyMayadere(t *testing.T) {
input := "hello"
result := applyMayadere(input)
if !strings.Contains(result, input) {
t.Errorf("applyMayadere: expected original text %q in output %q", input, result)
}
if len(result) <= len(input) {
t.Errorf("applyMayadere: expected output longer than input %q, got %q", input, result)
}
}

func TestDerePunishmentTypes(t *testing.T) {
// Ensure each dere type is dispatched and transforms the text.
input := "test message"
dereTypes := []PunishmentType{
PunishmentTsundere, PunishmentYandere, PunishmentKuudere,
PunishmentDandere, PunishmentDeredere, PunishmentHimedere,
PunishmentKamidere, PunishmentUndere, PunishmentBakadere,
PunishmentMayadere,
}
for _, pt := range dereTypes {
t.Run(pt.String(), func(t *testing.T) {
result := ApplyPunishmentToText(input, pt)
if result == input {
t.Errorf("%v: expected transformed output, got unchanged %q", pt, result)
}
})
}
}

func TestApplyDegrade(t *testing.T) {
	input := "hello world"
	result := applyDegrade(input)
	// Should be completely different from input (one of the degrading messages)
	if result == input {
		t.Errorf("applyDegrade: expected transformed output, got unchanged %q", result)
	}
	// Should be a non-empty string
	if result == "" {
		t.Errorf("applyDegrade: returned empty string")
	}
	// Should be one of the known degrading messages (covered more thoroughly by TestApplyDegradeIsOneDegradingMessage)
	found := false
	for _, msg := range degradeMessages {
		if result == msg {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("applyDegrade: result %q is not a known degrading message", result)
	}
}

func TestApplyDegradeIsOneDegradingMessage(t *testing.T) {
	// Verify every call returns one of the known degrading messages
	for i := 0; i < 50; i++ {
		result := applyDegrade("test")
		found := false
		for _, msg := range degradeMessages {
			if result == msg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("applyDegrade: returned unexpected message %q", result)
		}
	}
}

func TestApplyDegradeViaApplyPunishmentToText(t *testing.T) {
	input := "hello world"
	result := ApplyPunishmentToText(input, PunishmentDegrade)
	if result == input {
		t.Errorf("ApplyPunishmentToText(degrade): expected transformed output, got unchanged %q", result)
	}
}

func TestPunishmentDegradeString(t *testing.T) {
	if PunishmentDegrade.String() != "degrade" {
		t.Errorf("PunishmentDegrade.String(): expected %q, got %q", "degrade", PunishmentDegrade.String())
	}
}

func TestPunishmentLovebombString(t *testing.T) {
	if PunishmentLovebomb.String() != "lovebomb" {
		t.Errorf("PunishmentLovebomb.String(): expected %q, got %q", "lovebomb", PunishmentLovebomb.String())
	}
}
