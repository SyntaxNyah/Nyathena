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
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"unicode"
)

// confusedSplitter splits on any sequence of non-letter, non-digit characters
var confusedSplitter = regexp.MustCompile(`[^\p{L}\p{N}]+`)

const maxTextLength = 2000

// Package-level tables — allocated once at program start, reused on every IC
// message. Moving these out of the hot-path function bodies eliminates the
// per-call heap allocation that would otherwise occur on every punished message.

var (
	fancyTable = map[rune]rune{
		'a': '𝐚', 'b': '𝐛', 'c': '𝐜', 'd': '𝐝', 'e': '𝐞', 'f': '𝐟', 'g': '𝐠',
		'h': '𝐡', 'i': '𝐢', 'j': '𝐣', 'k': '𝐤', 'l': '𝐥', 'm': '𝐦', 'n': '𝐧',
		'o': '𝐨', 'p': '𝐩', 'q': '𝐪', 'r': '𝐫', 's': '𝐬', 't': '𝐭', 'u': '𝐮',
		'v': '𝐯', 'w': '𝐰', 'x': '𝐱', 'y': '𝐲', 'z': '𝐳',
		'A': '𝐀', 'B': '𝐁', 'C': '𝐂', 'D': '𝐃', 'E': '𝐄', 'F': '𝐅', 'G': '𝐆',
		'H': '𝐇', 'I': '𝐈', 'J': '𝐉', 'K': '𝐊', 'L': '𝐋', 'M': '𝐌', 'N': '𝐍',
		'O': '𝐎', 'P': '𝐏', 'Q': '𝐐', 'R': '𝐑', 'S': '𝐒', 'T': '𝐓', 'U': '𝐔',
		'V': '𝐕', 'W': '𝐖', 'X': '𝐗', 'Y': '𝐘', 'Z': '𝐙',
	}

	uwuSuffixes    = []string{" uwu", " owo", " >w<", " ^w^"}
	pirateSuffixes = []string{", arr!", ", matey!", ", ye scurvy dog!"}
	pirateTable    = map[string]string{
		"hello": "ahoy",
		"hi":    "ahoy",
		"yes":   "aye",
		"my":    "me",
		"you":   "ye",
		"your":  "yer",
		"are":   "be",
		"is":    "be",
	}

	shakespeareanTable = map[string]string{
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
	shakespeareanPrefixes = []string{
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
	shakespeareanSuffixes = []string{
		", methinks.",
		", forsooth!",
		", I prithee.",
		", good soul.",
		", 'tis so!",
		", verily.",
		", upon mine honour.",
		", I dare say.",
	}

	cavemanWords    = []string{"UGH", "GRUNT", "OOG", "RAWR", "HMPH", "GRUG"}
	robotWords      = []string{"[BEEP]", "[BOOP]", "[WHIRR]", "[BUZZ]"}
	whistleSounds   = []string{"♪", "♫", "~", "♬"}
	paranoidPhrases = []string{
		" (they're watching)",
		" (don't trust them)",
		" (they know)",
		" (THEY'RE LISTENING)",
		" (it's a conspiracy)",
	}
	subtitleLines = []string{
		" [ominous music playing]",
		" [confusing noises]",
		" [awkward silence]",
		" [dramatic pause]",
		" [indistinct chatter]",
	}
	snakeSuffixes = []string{" *hisss*", " ssss...", " ~hisssss~"}
	monkeySounds  = []string{"ook", "eek", "ooh ooh", "ahh ahh", "oo oo", "ee ee", "*scratches head*", "*swings from tree*"}
	dogSounds     = []string{"woof", "arf", "grr", "bark!", "ruff", "yip", "*wags tail*", "bork"}
	catSounds     = []string{"meow", "purrr~", "mrrrow", "mew", "nya~", "*purrs*", "prrrr", "mrrr"}
	birdSounds    = []string{"tweet", "chirp", "squawk", "cheep", "coo coo", "*flaps wings*", "peep", "caw"}
	cowSounds     = []string{"moo", "mooo", "MOOO", "moooo", "*chews cud*", "muu", "MOO MOO"}
	frogSounds    = []string{"ribbit", "croak", "brrr-ribbit", "riiibbit", "*jumps*", "crrroak", "ribbit-ribbit"}
	duckSounds    = []string{"quack", "QUACK", "quack!", "quack quack", "*waddles*", "QUACK!", "QUACK QUACK"}
	horseSounds   = []string{"neigh", "whinny", "nicker", "NEIGH!", "*clip clop*", "hrrrr", "snort"}
	lionSounds    = []string{"ROAR", "grrr", "rawr", "GRRR", "*snarls*", "rrrroar", "RAWRR"}
	bunnySounds   = []string{"*thump*", "*thump thump*", "*nose twitch*", "*hops away*", "*binky!*", "*flops*", "*teeth chattering*", "*nudges*"}
	emojiTable    = []string{"😀", "😎", "🤡", "👻", "🎃", "🦄", "🐱", "🐶", "🎮", "⭐"}

	autospellTable = map[string]string{
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

	spaghettiEffects = []func(string) string{
		applyUppercase,
		applyBackward,
		applyElongate,
		applyConfused,
		applyDrunk,
	}
	rngEffects = []func(string) string{
		applyBackward,
		applyUppercase,
		applyLowercase,
		applyUwu,
		applyPirate,
		applyRobotic,
		applyAlternating,
	}
	tormentEffects = []func(string) string{
		applyUppercase,
		applyBackward,
		applyUwu,
		applyRobotic,
		applyConfused,
	}
	zooEffects = []func(string) string{
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

	// ── ThesaurusOverload ────────────────────────────────────────────────────
	thesaurusTable = map[string]string{
		"go":         "peregrinate",
		"going":      "peregrinating",
		"walk":       "ambulate",
		"say":        "proclaim",
		"says":       "proclaims",
		"said":       "proclaimed",
		"talk":       "discourse",
		"want":       "desire",
		"wanted":     "desired",
		"need":       "necessitate",
		"needs":      "necessitates",
		"see":        "behold",
		"saw":        "beheld",
		"look":       "observe",
		"get":        "procure",
		"make":       "fabricate",
		"think":      "contemplate",
		"thought":    "contemplated",
		"know":       "comprehend",
		"feel":       "discern",
		"find":       "ascertain",
		"give":       "bestow",
		"take":       "appropriate",
		"have":       "possess",
		"big":        "gargantuan",
		"large":      "voluminous",
		"small":      "diminutive",
		"good":       "splendid",
		"great":      "magnanimous",
		"bad":        "deplorable",
		"awful":      "egregious",
		"happy":      "ebullient",
		"sad":        "melancholic",
		"angry":      "incensed",
		"smart":      "perspicacious",
		"dumb":       "obtuse",
		"stupid":     "obtuse",
		"fast":       "expeditious",
		"slow":       "sluggardly",
		"friend":     "compatriot",
		"enemy":      "adversary",
		"help":       "ameliorate",
		"stop":       "desist",
		"start":      "commence",
		"now":        "forthwith",
		"later":      "subsequently",
		"here":       "hereupon",
		"there":      "thereupon",
		"yes":        "affirmative",
		"no":         "negatory",
		"okay":       "acquiescence granted",
		"ok":         "affirmative",
		"please":     "I beseech thee",
		"thanks":     "gratitude proffered",
		"sorry":      "I sincerely apologise",
		"sure":       "unequivocally",
		"very":       "exceedingly",
		"really":     "veritably",
		"just":       "merely",
		"thing":      "entity",
		"things":     "entities",
		"stuff":      "sundry materials",
		"use":        "employ",
		"do":         "execute",
		"did":        "executed",
		"done":       "concluded",
		"try":        "endeavour",
		"wrong":      "erroneous",
		"right":      "perspicacious",
		"old":        "antiquated",
		"new":        "contemporary",
		"time":       "temporal juncture",
		"food":       "sustenance",
		"work":       "endeavour productively",
		"win":        "triumph",
		"lose":       "suffer defeat",
		"answer":     "riposte",
		"question":   "inquiry",
		"problem":    "predicament",
		"idea":       "conjecture",
		"place":      "locale",
		"home":       "domicile",
		"people":     "individuals",
		"person":     "individual",
		"tell":       "elucidate",
		"about":      "pertaining to",
		"because":    "for the reason that",
		"also":       "furthermore",
		"actually":   "in point of fact",
		"literally":  "in a wholly un-metaphorical capacity",
		"basically":  "fundamentally",
		"totally":    "unequivocally",
		"wait":       "remain stationary momentarily",
		"show":       "demonstrate",
		"weird":      "idiosyncratic",
		"strange":    "anomalous",
		"normal":     "normative",
		"fine":       "satisfactory",
		"hard":       "arduous",
		"easy":       "facile",
		"cold":       "frigid",
		"funny":      "mirthfully provocative",
		"serious":    "solemn",
		"true":       "veritable",
		"false":      "erroneous",
		"ask":        "inquire",
		"asked":      "inquired",
		"choose":     "elect",
		"believe":    "hypothesise",
		"remember":   "recollect",
		"forget":     "fail to recollect",
		"important":  "of considerable import",
		"leave":      "vacate",
		"return":     "recommence one's presence",
		"check":      "verify",
		"move":       "relocate",
		"write":      "inscribe",
		"read":       "peruse",
		"send":       "transmit",
		"call":       "invoke",
		"decide":     "adjudicate",
		"understand": "apprehend",
		"agree":      "concur",
		"disagree":   "dissent",
		"mean":       "signify",
		"like":       "regard with favour",
		"hate":       "hold in contemptuous disfavour",
		"love":       "harbour deep affection for",
		"eat":        "partake of sustenance",
		"sleep":      "enter a state of somnolence",
		"play":       "engage in recreational pursuits",
	}
	thesaurusSuffixes = []string{
		" (i.e., as previously stated)",
		" (per se)",
		" [citation needed]",
		" (source: my own perspicacity)",
		" (ergo, QED)",
		" (QED)",
		" (cf. the above)",
		" (vide infra)",
		" (this is veritably the case)",
		" (one might say)",
	}

	// ── ValleyGirl ───────────────────────────────────────────────────────────
	valleygirlFillers = []string{
		"like, ",
		"literally ",
		"okay sooo ",
		"um, ",
		"I mean, ",
		"honestly? ",
		"no but like, ",
		"okay but ",
	}
	valleygirlSuffixes = []string{
		" I literally can't.",
		" like, seriously??",
		" omg.",
		", like, whatever.",
		" ugh, literally.",
		" no cap.",
		"?? okay??",
		" I can't even.",
	}

	// ── Babytalk ─────────────────────────────────────────────────────────────
	babytalkStageDirections = []string{
		" *tiny stomp*",
		" *pout*",
		" *wiggles*",
		" *reaches arms up*",
		" *blows raspberry*",
		" *tugs sleeve*",
		" *bottom lip wobbles*",
		" *sniffles*",
	}

	// ── ThirdPerson ──────────────────────────────────────────────────────────
	thirdPersonTemplates = []string{
		"%s says: \"%s\"",
		"%s declares: \"%s\"",
		"%s, with great conviction, states: \"%s\"",
		"%s announces to the room: \"%s\"",
		"%s, without hesitation, proclaims: \"%s\"",
		"According to %s: \"%s\"",
		"%s would like the room to know: \"%s\"",
		"Allegedly, %s says: \"%s\"",
	}
	thirdPersonMoodTags = []string{
		" [dramatically]",
		" [emphatically]",
		" [passionately]",
		" [mysteriously]",
		" [suspiciously]",
		" [unconvincingly]",
	}

	// ── UnreliableNarrator ───────────────────────────────────────────────────
	unreliableHedges = []string{
		"allegedly",
		"supposedly",
		"in theory",
		"or so I'm told",
		"I think",
		"maybe",
		"perhaps",
		"apparently",
		"if memory serves",
		"according to sources",
	}
	unreliableSuffixes = []string{
		" (…or so I recall.)",
		" (Source: trust me.)",
		" (I think.)",
		" (allegedly.)",
		" *narrator's note: this is unverified*",
		" (unconfirmed)",
		" (or did they?)",
		" (citation: vibes)",
		" (don't quote me on this)",
		" (I was there. Probably.)",
		" (in a dream, maybe?)",
		" (my lawyer says I can't confirm this)",
	}

	// ── UncannyValley ────────────────────────────────────────────────────────
	uncannyGlitchTags = []string{
		" [checksum mismatch]",
		" [signal distortion detected]",
		" [buffer overflow at 0x0]",
		" [connection unstable]",
		" [data corruption detected]",
		" [reality.exe has stopped responding]",
		" [unexpected EOF]",
		" [ERR: IDENTITY_UNDEFINED]",
		" [rendering artifact]",
		" [frame desync]",
		" [memory leak suspected]",
		" [this message may not be real]",
	}
	// uncannyVowelSwaps maps vowels to safe lookalike substitutions for showname glitching.
	uncannyVowelSwaps = map[rune][]rune{
		'a': {'α', 'ä', 'â'},
		'e': {'ε', 'ë', 'é'},
		'i': {'ι', 'ï', 'í'},
		'o': {'ο', 'ö', 'ô'},
		'u': {'υ', 'ü', 'ú'},
		'A': {'Α', 'Ä', 'Â'},
		'E': {'Ε', 'Ë', 'É'},
		'I': {'Ι', 'Ï', 'Í'},
		'O': {'Ο', 'Ö', 'Ô'},
		'U': {'Υ', 'Ü', 'Ú'},
	}

	// tourettesAllVariants bundles the four outburst categories so applyTourettes
	// can pick one with a single rand.Intn call instead of allocating a new slice.
	tourettesAllVariants = [][]string{
		tourettesSwearing,
		tourettesRandom,
		tourettesExclamations,
		tourettesAnimalSounds,
	}

	// slangWords maps individual words to internet-slang shorthands.
	// Applied after phrase substitution; keys are already lower-cased.
	slangWords = map[string]string{
		// Pronouns / verbs
		"you":      "u",
		"your":     "ur",
		"yourself": "urself",
		"are":      "r",
		"be":       "b",
		"see":      "c",
		"why":      "y",
		"for":      "4",
		"to":       "2",
		"too":      "2",
		"two":      "2",
		// Polite words
		"please": "pls",
		"thanks": "thx",
		"okay":   "k",
		"ok":     "k",
		// Common shortenings
		"because":   "bc",
		"though":    "tho",
		"about":     "abt",
		"something": "smth",
		"someone":   "sm1",
		"everyone":  "evry1",
		"anyone":    "ne1",
		"anywhere":  "nywhr",
		"somewhere": "smwhr",
		"nothing":   "nth",
		"without":   "w/o",
		"with":      "w/",
		// Time
		"tomorrow": "tmrw",
		"tonight":  "2nite",
		"today":    "2day",
		"later":    "l8r",
		"before":   "b4",
		"forever":  "4ever",
		"together": "2gether",
		"second":   "sec",
		// Adjectives / adverbs
		"really":     "rly",
		"seriously":  "srsly",
		"definitely": "def",
		"probably":   "prob",
		"already":    "alrdy",
		"anyway":     "neway",
		// Fun leet-style
		"great":  "gr8",
		"wait":   "w8",
		"late":   "l8",
		"mate":   "m8",
		"hate":   "h8",
		"good":   "gud",
		"love":   "luv",
		"night":  "nite",
		"people": "ppl",
		"what":   "wut",
		"this":   "dis",
		"that":   "dat",
		// Relationships
		"girlfriend": "gf",
		"boyfriend":  "bf",
		"brother":    "bro",
		"sister":     "sis",
		// Misc
		"message":     "msg",
		"picture":     "pic",
		"pictures":    "pics",
		"information": "info",
		"whatever":    "w/e",
	}
)

// slangPhraseReplacer performs all multi-word phrase substitutions in a single
// left-to-right O(n) pass (Aho-Corasick internally).  Entries are ordered
// longest-first so that longer phrases always win over their shorter prefixes
// (e.g. "see you later" matches before "see you").
var slangPhraseReplacer = strings.NewReplacer(
	// ── 25+ chars ────────────────────────────────────────────────────────────
	"rolling on the floor laughing", "rotfl",
	// ── 21 chars ─────────────────────────────────────────────────────────────
	"at the end of the day", "ateotd",
	"in case you missed it", "icymi",
	"if i recall correctly", "iirc",
	"too long did not read", "tldr",
	// ── 20 chars ─────────────────────────────────────────────────────────────
	"you know what i mean", "ykwim",
	"don't worry about it", "dwai",
	"in my humble opinion", "imho",
	"greatest of all time", "goat",
	"best friends forever", "bff",
	"too long didn't read", "tldr",
	// ── 19 chars ─────────────────────────────────────────────────────────────
	"fear of missing out", "fomo",
	"laughing my ass off", "lmao",
	"as soon as possible", "asap",
	// ── 18 chars ─────────────────────────────────────────────────────────────
	"good luck have fun", "glhf",
	"you only live once", "yolo",
	// ── 17 chars ─────────────────────────────────────────────────────────────
	"not safe for work", "nsfw",
	"talk to you later", "ttyl",
	"laughing out loud", "lol",
	// ── 16 chars ─────────────────────────────────────────────────────────────
	"talk to you soon", "ttys",
	"not going to lie", "ngl",
	"as far as i know", "afaik",
	"long story short", "lss",
	"laugh my ass off", "lmao",
	"what do you mean", "wdym",
	// ── 15 chars ─────────────────────────────────────────────────────────────
	"shaking my head", "smh",
	"to be continued", "tbc",
	// ── 14 chars ─────────────────────────────────────────────────────────────
	"laugh out loud", "lol",
	// ── 13 chars ─────────────────────────────────────────────────────────────
	"see you later", "cya", // must come before "see you"
	"i do not know", "idk",
	"i do not care", "idc",
	"be right back", "brb",
	"what the heck", "wth",
	"what the hell", "wth",
	"what the fuck", "wtf",
	"at the moment", "atm",
	"in my opinion", "imo",
	"shake my head", "smh",
	"not gonna lie", "ngl",
	// ── 12 chars ─────────────────────────────────────────────────────────────
	"just kidding", "jk",
	"good morning", "gm",
	"in real life", "irl",
	"i don't know", "idk",
	"i don't care", "idc",
	"to be honest", "tbh",
	"i know right", "ikr",
	// ── 11 chars ─────────────────────────────────────────────────────────────
	"let me know", "lmk",
	"for the win", "ftw",
	// ── 10 chars ─────────────────────────────────────────────────────────────
	"oh my gosh", "omg", // before "oh my god"
	"never mind", "nvm",
	"no problem", "np",
	"to be fair", "tbf",
	"good night", "gn",
	"by the way", "btw",
	"i love you", "ily",
	// ── 9 chars ──────────────────────────────────────────────────────────────
	"got to go", "gtg",
	"on my way", "omw",
	"hit me up", "hmu",
	"all right", "aight",
	"good luck", "gl",
	"right now", "rn",
	"oh my god", "omg",
	// ── 8 chars ──────────────────────────────────────────────────────────────
	"have fun", "hf",
	"for real", "fr",
	// ── 7 chars ──────────────────────────────────────────────────────────────
	"see you", "cya",
)

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
	for _, r := range text {
		if fancy, ok := fancyTable[r]; ok {
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
		text += uwuSuffixes[rand.Intn(len(uwuSuffixes))]
	}
	return truncateText(text)
}

// applyPirate converts to pirate speech
func applyPirate(text string) string {
	lower := strings.ToLower(text)
	for old, new := range pirateTable {
		lower = strings.ReplaceAll(lower, old, new)
	}

	// Add pirate expressions
	if rand.Float32() < 0.3 {
		lower += pirateSuffixes[rand.Intn(len(pirateSuffixes))]
	}
	return truncateText(lower)
}

// applyShakespearean converts to Shakespearean English
func applyShakespearean(text string) string {
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
		if replacement, ok := shakespeareanTable[lower]; ok {
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

	if rand.Float32() < 0.4 {
		result = shakespeareanPrefixes[rand.Intn(len(shakespeareanPrefixes))] + result
	}

	if rand.Float32() < 0.3 {
		result = result + shakespeareanSuffixes[rand.Intn(len(shakespeareanSuffixes))]
	}

	return truncateText(result)
}

// applyCaveman converts to caveman grunts
func applyCaveman(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "UGH"
	}

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

	var result strings.Builder
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		for range word {
			result.WriteString(whistleSounds[rand.Intn(len(whistleSounds))])
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
	// Apply 2-3 random effects
	numEffects := 2 + rand.Intn(2)
	for i := 0; i < numEffects; i++ {
		text = spaghettiEffects[rand.Intn(len(spaghettiEffects))](text)
	}
	return text
}

// applyRng applies random effect from pool
func applyRng(text string) string {
	return rngEffects[rand.Intn(len(rngEffects))](text)
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
	words := strings.Fields(text)
	for i, word := range words {
		lower := strings.ToLower(word)
		if replacement, ok := autospellTable[lower]; ok {
			words[i] = replacement
		}
	}
	return strings.Join(words, " ")
}

// applyTorment cycles through different effects based on message count
func applyTorment(text string, cycleIndex int) string {
	return tormentEffects[cycleIndex%len(tormentEffects)](text)
}

// applySubtitles adds confusing annotations
func applySubtitles(text string) string {
	return text + subtitleLines[rand.Intn(len(subtitleLines))]
}

// applySpotlight adds an announcement prefix
func applySpotlight(text string) string {
	return "📣 EVERYONE LOOK: " + text
}

// ── ThesaurusOverload ────────────────────────────────────────────────────────

// applyThesaurusOverload replaces common words with absurdly pompous synonyms.
func applyThesaurusOverload(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		// Strip trailing punctuation
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
		if replacement, ok := thesaurusTable[lower]; ok {
			if len(stripped) > 0 && unicode.IsUpper([]rune(stripped)[0]) {
				r := []rune(replacement)
				r[0] = unicode.ToUpper(r[0])
				replacement = string(r)
			}
			words[i] = replacement + punct
		}
	}
	result := strings.Join(words, " ")
	if rand.Float32() < 0.4 {
		result += thesaurusSuffixes[rand.Intn(len(thesaurusSuffixes))]
	}
	return truncateText(result)
}

// ── ValleyGirl ───────────────────────────────────────────────────────────────

// applyValleyGirl injects valley-girl filler words and stretches vowels.
func applyValleyGirl(text string) string {
	// Inject a filler at the start ~60% of the time
	if rand.Float32() < 0.6 {
		text = valleygirlFillers[rand.Intn(len(valleygirlFillers))] + text
	}
	// Stretch some vowels for drama
	for _, ch := range []string{"o", "e", "a"} {
		if rand.Float32() < 0.3 && strings.Contains(strings.ToLower(text), ch) {
			// Extend first occurrence of this vowel run
			idx := strings.Index(strings.ToLower(text), ch)
			if idx >= 0 {
				text = text[:idx+1] + ch + ch + text[idx+1:]
			}
		}
	}
	// Replace "no" with "nooo" and "yes" with "yesss"
	text = strings.ReplaceAll(text, " no ", " nooo ")
	text = strings.ReplaceAll(text, " yes ", " yesss ")
	// Append a dramatic suffix ~50% of the time
	if rand.Float32() < 0.5 {
		text += valleygirlSuffixes[rand.Intn(len(valleygirlSuffixes))]
	}
	return truncateText(text)
}

// ── Babytalk ─────────────────────────────────────────────────────────────────

// applyBabytalk converts text to toddler-style speech.
func applyBabytalk(text string) string {
	// Phonetic substitutions (order matters: longer first)
	replacements := []struct{ old, new string }{
		{"please", "pwease"},
		{"pretty", "pwetty"},
		{"flower", "fwower"},
		{"friend", "fwiend"},
		{"brother", "bwudder"},
		{"little", "widdle"},
		{"bottle", "baba"},
		{"water", "wawa"},
		{"together", "togedder"},
		{"hungry", "hungy"},
		{"sorry", "sowwy"},
		{"right", "wight"},
		{"light", "wight"},
		{"give", "gib"},
		{"very", "vewy"},
		{"okay", "otay"},
		{"str", "stw"},
		{"tr", "tw"},
		{"dr", "dw"},
		{"fl", "fw"},
	}
	lower := strings.ToLower(text)
	for _, r := range replacements {
		lower = strings.ReplaceAll(lower, r.old, r.new)
	}
	// Replace profanity with toddler equivalents
	lower = strings.ReplaceAll(lower, "damn", "dang")
	lower = strings.ReplaceAll(lower, "hell", "heck")
	lower = strings.ReplaceAll(lower, "crap", "poop")

	// Add a stage direction ~40% of the time
	if rand.Float32() < 0.4 {
		lower += babytalkStageDirections[rand.Intn(len(babytalkStageDirections))]
	}
	return truncateText(lower)
}

// ── ThirdPerson ──────────────────────────────────────────────────────────────

// applyThirdPersonWithName wraps text in third-person narration using the
// player's display name. Call applyThirdPerson for the no-showname fallback.
func applyThirdPersonWithName(text, showname string) string {
	if strings.TrimSpace(showname) == "" {
		showname = "Someone"
	}
	// Determine mood tag from punctuation / capitalisation
	moodTag := ""
	upperCount := 0
	for _, r := range text {
		if unicode.IsUpper(r) {
			upperCount++
		}
	}
	hasExclamation := strings.Contains(text, "!")
	hasQuestion := strings.Contains(text, "?")
	upperRatio := 0.0
	if len(text) > 0 {
		upperRatio = float64(upperCount) / float64(len([]rune(text)))
	}
	switch {
	case upperRatio > 0.6 && hasExclamation:
		moodTag = " [feral]"
	case upperRatio > 0.6:
		moodTag = " [unhinged]"
	case hasExclamation && hasQuestion:
		moodTag = " [confused and dramatic]"
	case hasExclamation:
		moodTag = " [dramatic]"
	case hasQuestion:
		moodTag = " [confused]"
	case rand.Float32() < 0.25:
		moodTag = thirdPersonMoodTags[rand.Intn(len(thirdPersonMoodTags))]
	}
	template := thirdPersonTemplates[rand.Intn(len(thirdPersonTemplates))]
	result := fmt.Sprintf(template, showname, text) + moodTag
	return truncateText(result)
}

// applyThirdPerson is the no-showname version used by the generic dispatcher.
func applyThirdPerson(text string) string {
	return applyThirdPersonWithName(text, "")
}

// ── UnreliableNarrator ───────────────────────────────────────────────────────

// applyUnreliableNarrator makes the speaker sound like an untrustworthy narrator.
func applyUnreliableNarrator(text string) string {
	// Insert a hedge word after the first word ~60% of the time
	words := strings.Fields(text)
	if len(words) >= 2 && rand.Float32() < 0.6 {
		hedge := unreliableHedges[rand.Intn(len(unreliableHedges))]
		words = append(words[:1], append([]string{hedge}, words[1:]...)...)
		text = strings.Join(words, " ")
	}
	// Append a suspicious suffix
	text += unreliableSuffixes[rand.Intn(len(unreliableSuffixes))]
	return truncateText(text)
}

// ── UncannyValley ────────────────────────────────────────────────────────────

// applyUncannyValley adds glitchy system-note suffixes to messages.
// The display-name mutation is handled in netprotocol.go.
func applyUncannyValley(text string) string {
	// Easter egg: if they claim to be fine, add an unsettling smiley
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "im fine") ||
		strings.Contains(lowerText, "i'm fine") ||
		strings.Contains(lowerText, "i am fine") {
		text = strings.NewReplacer(
			"im fine", "im fine :)",
			"i'm fine", "i'm fine :)",
			"I'm fine", "I'm fine :)",
			"I am fine", "I am fine :)",
			"i am fine", "i am fine :)",
		).Replace(text)
	}
	// ~60% chance to append a glitch tag
	if rand.Float32() < 0.6 {
		text += uncannyGlitchTags[rand.Intn(len(uncannyGlitchTags))]
	}
	return truncateText(text)
}

// MutateShowname applies a mild per-message display-name glitch for
// PunishmentUncannyValley. It makes safe mutations (no true impersonation).
func MutateShowname(name string) string {
	if len(name) == 0 {
		return name
	}
	runes := []rune(name)
	choice := rand.Intn(4)
	switch choice {
	case 0:
		// Replace one vowel with a lookalike homoglyph
		var vowelIndices []int
		for i, r := range runes {
			if _, ok := uncannyVowelSwaps[r]; ok {
				vowelIndices = append(vowelIndices, i)
			}
		}
		if len(vowelIndices) > 0 {
			idx := vowelIndices[rand.Intn(len(vowelIndices))]
			options := uncannyVowelSwaps[runes[idx]]
			runes[idx] = options[rand.Intn(len(options))]
			return string(runes)
		}
		// Fallback: add underscore suffix
		return string(runes) + "_"
	case 1:
		// Add underscore suffix
		return string(runes) + "_"
	case 2:
		// Swap two adjacent letters (skip first char to keep capital intact)
		if len(runes) >= 3 {
			idx := 1 + rand.Intn(len(runes)-2)
			runes[idx], runes[idx+1] = runes[idx+1], runes[idx]
		} else {
			return string(runes) + "."
		}
		return string(runes)
	default:
		// Duplicate a random character (only if within length budget)
		if len(string(runes)) < maxShownameLength-1 {
			idx := rand.Intn(len(runes))
			newRunes := make([]rune, len(runes)+1)
			copy(newRunes, runes[:idx+1])
			newRunes[idx+1] = runes[idx]
			copy(newRunes[idx+2:], runes[idx+1:])
			return string(newRunes)
		}
		return string(runes) + "."
	}
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
	case PunishmentDegrade:
		return applyDegrade(text)
	case PunishmentTourettes:
		return applyTourettes(text)
	case PunishmentSlang:
		return applySlang(text)
	case PunishmentThesaurusOverload:
		return applyThesaurusOverload(text)
	case PunishmentValleyGirl:
		return applyValleyGirl(text)
	case PunishmentBabytalk:
		return applyBabytalk(text)
	case PunishmentThirdPerson:
		return applyThirdPerson(text)
	case PunishmentUnreliableNarrator:
		return applyUnreliableNarrator(text)
	case PunishmentUncannyValley:
		return applyUncannyValley(text)
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
		text += snakeSuffixes[rand.Intn(len(snakeSuffixes))]
	}
	return truncateText(text)
}

// applyDog replaces text with dog sounds
func applyDog(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "WOOF!"
	}
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
	return zooEffects[rand.Intn(len(zooEffects))](text)
}

// applyBunny replaces text with bunny sounds
func applyBunny(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "*thump thump*"
	}
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
	return emojiTable[rand.Intn(len(emojiTable))]
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

// degradeMessages are first-person degrading statements used by the degrade punishment.
var degradeMessages = []string{
	"I am pathetic",
	"I am powerless",
	"I am a bottomfeeder",
	"I am worthless",
	"I am nothing",
	"I am weak and pathetic",
	"I am beneath everyone here",
	"I am truly hopeless",
	"I am an absolute failure",
	"I am beyond pathetic",
	"I am completely powerless",
	"I am a worthless waste of space",
}

// applyDegrade replaces the message with a random degrading first-person statement.
func applyDegrade(text string) string {
	return degradeMessages[rand.Intn(len(degradeMessages))]
}

// tourettesSwearing contains censored-style swear outbursts for the tourettes effect.
var tourettesSwearing = []string{
	"SHIT", "DAMN", "CRAP", "BALLS", "BLOODY HELL",
	"FRICK", "HELL", "BASTARD", "DAMNIT", "CRUD",
	"ASS", "BOLLOCKS", "BUGGER", "BLIMEY", "JACKASS",
}

// tourettesRandom contains random everyday objects blurted out.
var tourettesRandom = []string{
	"REFRIGERATOR", "BICYCLE", "PENGUIN", "TOASTER", "SPOON",
	"BANANA", "CEILING", "CARPET", "STAPLER", "WINDOW",
	"BROCCOLI", "SAUSAGE", "LAMPSHADE", "DASHBOARD", "PICKLE",
	"DOORKNOB", "SPATULA", "PINEAPPLE", "BISCUIT", "FLAMINGO",
	"OTTOMAN", "SOCKET", "CHEDDAR", "TRAMPOLINE", "KAZOO",
}

// tourettesExclamations contains nonsense exclamatory outbursts.
var tourettesExclamations = []string{
	"BLARGH!", "YOINK!", "ZOINKS!", "WHAMMY!", "BLORT!",
	"FNURGLE!", "GADZOOKS!", "CRIKEY!", "YIKES!", "ZING!",
	"KAPOW!", "BOING!", "ZOWEE!", "WHOOPSIE!", "BONKERS!",
	"SNORKEL!", "WIBBLE!", "KERFUFFLE!", "HULLABALOO!", "GAZPACHO!",
	"FLIBBERTIGIBBET!", "WUMBO!", "SKADOOSH!", "BAZINGA!", "HODOR!",
}

// tourettesAnimalSounds contains sudden animal noise outbursts.
var tourettesAnimalSounds = []string{
	"SQUAWK", "MEOW", "WOOF WOOF", "RIBBIT", "OINK",
	"MOO", "NEIGH", "ROAR", "HISSSSS", "BAWK BAWK",
	"BAAA", "COCK-A-DOODLE-DOO", "CHIRP CHIRP", "HOOOOT", "RUFF",
}

// applyTourettes inserts random outbursts in the middle of messages.
// It randomly selects from several variant categories (swearing, random objects,
// nonsense exclamations, and animal sounds) and injects them between words.
func applyTourettes(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var result strings.Builder
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(word)

		// ~35% chance of an outburst after each word
		if rand.Float32() < 0.35 {
			category := tourettesAllVariants[rand.Intn(len(tourettesAllVariants))]
			outburst := category[rand.Intn(len(category))]
			result.WriteString(" ")
			result.WriteString(outburst)
		}
	}
	return truncateText(result.String())
}

// applySlang converts common words and phrases to internet-slang shorthands.
//
// Two-phase design for maximum efficiency:
//
//  1. slangPhraseReplacer.Replace performs ALL multi-word phrase substitutions
//     in a single O(n) left-to-right scan (Aho-Corasick automaton, built once
//     at package init).
//
//  2. A single word-level scan substitutes individual words from slangWords,
//     stripping and restoring one byte of trailing ASCII punctuation per token.
//     strings.Builder with a pre-grown buffer keeps allocations to the minimum.
//
// IPID persistence: the punishment is stored and restored by the generic
// cmdPunishment / restorePunishments machinery, so slang survives reconnects.
func applySlang(text string) string {
	// Phase 1: lower-case once; replace all phrases in one pass.
	s := slangPhraseReplacer.Replace(strings.ToLower(text))

	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}

	// Phase 2: word-level substitution.
	// Pre-grow to len(s): slang replacements shrink text, so this is a safe
	// upper bound that avoids any re-allocation inside the builder.
	var b strings.Builder
	b.Grow(len(s))

	for i, word := range words {
		if i > 0 {
			b.WriteByte(' ')
		}
		// Detect and strip a single trailing ASCII punctuation byte so that
		// "you," → "u," rather than leaving "you," unmatched.
		punct := byte(0)
		if n := len(word); n > 0 {
			c := word[n-1]
			if c == '.' || c == ',' || c == '!' || c == '?' || c == ';' || c == ':' {
				punct = c
				word = word[:n-1]
			}
		}
		if rep, ok := slangWords[word]; ok {
			b.WriteString(rep)
		} else {
			b.WriteString(word)
		}
		if punct != 0 {
			b.WriteByte(punct)
		}
	}
	return truncateText(b.String())
}

// lovebombTemplates are silly love-bomb message templates.
// %s is replaced with the target's display name.
var lovebombTemplates = []string{
	"I LOVE YOU %s!! ♥",
	"OH WOW %s, YOU ARE THE MOST AMAZING PERSON ALIVE!! 💕",
	"%s I WOULD LITERALLY FIGHT A BEAR FOR YOU!! 😍",
	"HAS ANYONE TOLD YOU TODAY THAT %s IS ABSOLUTELY STUNNING??",
	"I CANNOT STOP THINKING ABOUT %s!!!!! 💘",
	"NOTICE ME %s SENPAI!! 😳♥",
	"%s IS THE LIGHT OF MY LIFE AND I NEED EVERYONE TO KNOW THIS",
	"I WROTE A 40-PAGE ESSAY ABOUT WHY %s IS PERFECT",
	"EVERY SONG I HEAR REMINDS ME OF %s 🎵💕",
	"IF LOVING %s IS WRONG I DON'T WANT TO BE RIGHT",
	"%s I BAKED YOU 47 CAKES PLEASE LOOK AT ME",
	"I LOVE %s MORE THAN THE SUN LOVES THE SKY 🌞",
	"%s YOUR SMILE MAKES THE FLOWERS GROW AND THE BIRDS SING",
	"I WOULD CROSS THE GALAXY JUST TO WAVE HI TO %s 🌌",
	"MY LOVE FOR %s IS BIGGER THAN THE ENTIRE UNIVERSE",
	"GOOD MORNING AFTERNOON AND EVENING %s, I AM OBSESSED WITH YOU ♥",
	"I NAMED MY HOUSEPLANT AFTER %s BECAUSE THEY BRING ME JOY",
	"SCIENTISTS CANNOT EXPLAIN HOW MUCH I LOVE %s",
	"%s I WROTE YOUR NAME IN THE STARS, LITERALLY, I OWN A STAR",
	"MY HEART DOES A LITTLE DANCE EVERY TIME I SEE %s 💃",
	"I HAVE LOVED %s SINCE BEFORE TIME ITSELF BEGAN",
	"WHENEVER I CLOSE MY EYES I ONLY SEE %s, SEND HELP",
	"%s IF YOU WERE A VEGETABLE YOU WOULD BE A CUTECUMBER",
	"I COLLECT PHOTOS OF %s LIKE TRADING CARDS",
	"DEAR DIARY: %s LOOKED AT ME TODAY. MY LIFE IS COMPLETE",
	"I WOULD GIVE UP PIZZA FOREVER JUST TO HOLD %s'S HAND",
	"%s IS SO WONDERFUL THAT PUPPIES ARE JEALOUS",
	"ROSES ARE RED VIOLETS ARE BLUE %s IS PERFECT AND I LOVE THEM TOO",
	"I SET %s AS MY PHONE WALLPAPER, SCREENSAVER, AND LOCKSCREEN",
	"THEY SAY DIAMONDS ARE FOREVER BUT MY LOVE FOR %s IS EVEN MORE FOREVER",
}

// applyLovebombMessage returns a random lovebomb message for the given target display name.
// If targetShowname is empty it falls back to a nameless declaration.
func applyLovebombMessage(targetShowname string) string {
	if targetShowname == "" {
		return "I LOVE EVERYONE HERE SO MUCH!! ♥"
	}
	return fmt.Sprintf(lovebombTemplates[rand.Intn(len(lovebombTemplates))], targetShowname)
}
