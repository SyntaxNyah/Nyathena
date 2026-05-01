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
	"unicode/utf8"
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

// babytalkReplacer applies all babytalk phonetic substitutions in a single O(n)
// pass. Longer patterns are listed before shorter ones that share a prefix (e.g.
// "together" before "tr", "flower" before "fl") so that the first match wins.
var babytalkReplacer = strings.NewReplacer(
	// 8 chars
	"together", "togedder",
	// 7 chars
	"brother", "bwudder",
	// 6 chars
	"bottle", "baba",
	"flower", "fwower",
	"friend", "fwiend",
	"hungry", "hungy",
	"little", "widdle",
	"please", "pwease",
	"pretty", "pwetty",
	// 5 chars
	"light", "wight",
	"right", "wight",
	"sorry", "sowwy",
	"water", "wawa",
	// 4 chars
	"crap", "poop",
	"damn", "dang",
	"give", "gib",
	"hell", "heck",
	"okay", "otay",
	"very", "vewy",
	// 3 chars (str before tr so "str" isn't partially consumed by "tr")
	"str", "stw",
	// 2 chars
	"dr", "dw",
	"fl", "fw",
	"tr", "tw",
)

// uncannyFineReplacer handles all capitalisation variants of "im/i'm/i am fine"
// so that applyUncannyValley doesn't allocate a Replacer on every hot-path call.
var uncannyFineReplacer = strings.NewReplacer(
	"I am fine", "I am fine :)",
	"i am fine", "i am fine :)",
	"I'm fine", "I'm fine :)",
	"i'm fine", "i'm fine :)",
	"Im fine", "Im fine :)",
	"im fine", "im fine :)",
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
			// Preserve leading capital if original word was capitalised.
			// utf8.DecodeRuneInString avoids allocating a full []rune slice.
			if firstRune, _ := utf8.DecodeRuneInString(stripped); unicode.IsUpper(firstRune) {
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
			if firstRune, _ := utf8.DecodeRuneInString(stripped); unicode.IsUpper(firstRune) {
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
	// Stretch some vowels for drama. Compute lower once and keep it in sync
	// with text so subsequent searches use accurate positions without
	// redundant ToLower calls.
	lower := strings.ToLower(text)
	for _, ch := range []string{"o", "e", "a"} {
		if rand.Float32() < 0.3 {
			if idx := strings.Index(lower, ch); idx >= 0 {
				text = text[:idx+1] + ch + ch + text[idx+1:]
				lower = lower[:idx+1] + ch + ch + lower[idx+1:]
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
	// babytalkReplacer performs all phonetic substitutions (including profanity
	// softening) in a single O(n) pass instead of 22 sequential ReplaceAll calls.
	lower := babytalkReplacer.Replace(strings.ToLower(text))
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
	// Determine mood tag from punctuation / capitalisation.
	// Count uppercase letters and total runes in a single pass to avoid the
	// extra O(n) allocation that len([]rune(text)) would cause.
	moodTag := ""
	upperCount := 0
	runeCount := 0
	for _, r := range text {
		runeCount++
		if unicode.IsUpper(r) {
			upperCount++
		}
	}
	hasExclamation := strings.Contains(text, "!")
	hasQuestion := strings.Contains(text, "?")
	upperRatio := 0.0
	if runeCount > 0 {
		upperRatio = float64(upperCount) / float64(runeCount)
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
	// Insert a hedge word after the first word ~60% of the time.
	// Use in-place grow-and-shift to avoid allocating two temporary slices.
	words := strings.Fields(text)
	if len(words) >= 2 && rand.Float32() < 0.6 {
		hedge := unreliableHedges[rand.Intn(len(unreliableHedges))]
		words = append(words, "")  // grow by one
		copy(words[2:], words[1:]) // shift [1:] one position right
		words[1] = hedge
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
	// Easter egg: if they claim to be fine, add an unsettling smiley.
	// uncannyFineReplacer is package-level so no allocation occurs here.
	// It returns the original string unchanged if none of the patterns match,
	// which is cheaper than pre-checking with ToLower + Contains.
	text = uncannyFineReplacer.Replace(text)
	// ~60% chance to append a glitch tag
	if rand.Float32() < 0.6 {
		text += uncannyGlitchTags[rand.Intn(len(uncannyGlitchTags))]
	}
	return truncateText(text)
}

// ── 51 ───────────────────────────────────────────────────────────────────────

var messages51 = []string{
	"I was in a bad mood, to say the least, as it was obvious that they lied to me and had seen someone else.",
	"I was ranting about her dishonesty, about how badly I was treated in the end, and demanding the truth.",
	"In the end, they blocked me and never spoke to me again.",
	"For context in that DM, I wrote 51 messages, as I'm not the kind of person who likes to write one long message like this.",
	"Two days after she blocked me, I realized I was rather rude with my statements and had gone too far.",
	"I really wanted to apologize, but there was no way for me to do so, so I just coped basically.",
	"Later on, I found a new server named Paradise of Despots, so I decided to join it out of curiosity.",
	"I found old friends from CoA there, and Twilight Sky was there too.",
	"I decided not to bring anything regarding us two and just said hello on that server to all the people I knew there, but I was immediately banned with the reason \"On the shitlist.\"",
	"When I asked the owner of the server, AKA Psyra, he basically said that Sky doesn't want to see me there for good.",
	"For me, it was devastating news; I just couldn't argue that at all, I was mentally exhausted and broken.",
	"Since that time, I decided to take a break from AO until December, as one of my friends suggested.",
	"That's what I've been doing since September, aka when I was banned.",
	"However, some friend of PoD notified me that Sky was spreading some information regarding our last interaction for the sake of mockery.",
	"That's when I was informed that I apparently wrote 51 messages, and I was both sad that I allowed myself to write so much and sad that Twilight decided to spread this in order to make fun of me behind my back.",
	"But even then, I decided to do nothing, as I was more focused on my life, and I was thinking that this would not do much harm to me.",
	"But I was wrong.",
	"To put it simply, Sky just decided to make fun of me based on the quantity of messages, even when I was mostly right about her snake behavior.",
	"In the end, it's just drama that should have stayed only between me and her.",
	"But I guess her tongue is too long.",
	"I am more annoyed about the fact that people that I used to call \"friends\" decided to take her side and join the childish behavior, not even giving a damn about my side of the story.",
	"I am even more annoyed that I appeared on the server only one time and never interacted with PoD at any point.",
	"And yet they are still targeting me for whatever reason.",
	"I'd like to not escalate this drama any further, as it's a pointless argument.",
}

// apply51 replaces the message with a random line from the 51-messages story.
func apply51(_ string) string {
	return messages51[rand.Intn(len(messages51))]
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
		// Add a period suffix (distinct from the underscore fallback in case 0)
		return string(runes) + "."
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
		// Duplicate a random character (only if within length budget).
		// len(runes) is the rune count — the correct comparison for maxShownameLength.
		if len(runes) < maxShownameLength-1 {
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
	case Punishment51:
		return apply51(text)
	case PunishmentPhilosopher:
		return applyPhilosopher(text)
	case PunishmentPoet:
		return applyPoet(text)
	case PunishmentUpsidedown:
		return applyUpsidedown(text)
	case PunishmentSarcasm:
		return applySarcasm(text)
	case PunishmentAcademic:
		return applyAcademic(text)
	case PunishmentRecipe:
		return applyRecipe(text)
	case PunishmentQuote:
		return applyQuote(text)
	case PunishmentTimewarp:
		return applyTimewarp(text)
	case PunishmentMorse:
		return applyMorse(text)
	case PunishmentRickroll:
		return applyRickroll(text)
	case PunishmentVowelhell:
		return applyVowelhell(text)
	case PunishmentChef:
		return applyChef(text)
	case PunishmentKaren:
		return applyKaren(text)
	case PunishmentPassiveAggressive:
		return applyPassiveAggressive(text)
	case PunishmentNervous:
		return applyNervous(text)
	case PunishmentDreamSequence:
		return applyDreamSequence(text)
	case PunishmentPickup:
		return applyPickup(text)
	case PunishmentBrainrot:
		return applyBrainrot(text)
	case PunishmentGordonRamsay:
		return applyGordonRamsay(text)
	case PunishmentCherri:
		return applyCherri(text)
	case PunishmentClown:
		return applyClown(text)
	case PunishmentJester:
		return applyJester(text)
	case PunishmentJoker:
		return applyJoker(text)
	case PunishmentMime:
		return applyMime(text)
	case PunishmentBiblebot:
		return applyBiblebot(text)
	case PunishmentSmugdere:
		return applySmugdere(text)
	case PunishmentDeretsun:
		return applyDeretsun(text)
	case PunishmentBokodere:
		return applyBokodere(text)
	case PunishmentThugdere:
		return applyThugdere(text)
	case PunishmentTeasedere:
		return applyTeasedere(text)
	case PunishmentDorodere:
		return applyDorodere(text)
	case PunishmentHinedere:
		return applyHinedere(text)
	case PunishmentHajidere:
		return applyHajidere(text)
	case PunishmentRindere:
		return applyRindere(text)
	case PunishmentUtsudere:
		return applyUtsudere(text)
	case PunishmentDarudere:
		return applyDarudere(text)
	case PunishmentButsudere:
		return applyButsudere(text)
	case PunishmentSDere:
		return applySDere(text)
	case PunishmentMDere:
		return applyMDere(text)
	case PunishmentTsuyodere:
		return applyTsuyodere(text)
	case PunishmentOmnidere:
		return applyOmnidere(text)
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

// applyAnimalSounds replaces each word in text with a random entry from sounds.
// If text is empty, defaultSound is returned instead.
// sounds must be a non-empty slice.
func applyAnimalSounds(text, defaultSound string, sounds []string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return defaultSound
	}
	var result strings.Builder
	for i := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(sounds[rand.Intn(len(sounds))])
	}
	return truncateText(result.String())
}

// applyMonkey replaces text with monkey noises
func applyMonkey(text string) string {
	return applyAnimalSounds(text, "OOH OOH AHH AHH", monkeySounds)
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
	return applyAnimalSounds(text, "WOOF!", dogSounds)
}

// applyCat replaces text with cat sounds
func applyCat(text string) string {
	return applyAnimalSounds(text, "meow~", catSounds)
}

// applyBird replaces text with bird sounds
func applyBird(text string) string {
	return applyAnimalSounds(text, "tweet!", birdSounds)
}

// applyCow replaces text with cow sounds
func applyCow(text string) string {
	return applyAnimalSounds(text, "MOO", cowSounds)
}

// applyFrog replaces text with frog sounds
func applyFrog(text string) string {
	return applyAnimalSounds(text, "ribbit!", frogSounds)
}

// applyDuck replaces text with duck sounds
func applyDuck(text string) string {
	return applyAnimalSounds(text, "QUACK!", duckSounds)
}

// applyHorse replaces text with horse sounds
func applyHorse(text string) string {
	return applyAnimalSounds(text, "NEIGH!", horseSounds)
}

// applyLion replaces text with lion sounds
func applyLion(text string) string {
	return applyAnimalSounds(text, "ROAR!", lionSounds)
}

// applyZoo applies a random animal punishment from the full zoo
func applyZoo(text string) string {
	return zooEffects[rand.Intn(len(zooEffects))](text)
}

// applyBunny replaces text with bunny sounds
func applyBunny(text string) string {
	return applyAnimalSounds(text, "*thump thump*", bunnySounds)
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

// applyPrefixSuffix wraps text with a random prefix from pfx and a random suffix from sfx.
// Both pfx and sfx must be non-empty slices.
func applyPrefixSuffix(text string, pfx, sfx []string) string {
	return pfx[rand.Intn(len(pfx))] + text + sfx[rand.Intn(len(sfx))]
}

// applyTsundere wraps text in classic tsundere denial and blush reactions.
func applyTsundere(text string) string {
	return applyPrefixSuffix(text, tsunderePfx, tsundereSfx)
}

// applyYandere wraps text in obsessive, unhinged yandere flavour.
func applyYandere(text string) string {
	return applyPrefixSuffix(text, yanderePfx, yandereSfx)
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
	return applyPrefixSuffix(text, kuuderePfx, kuudereSfx)
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
	return applyPrefixSuffix(text, deredere_pfx, deredere_sfx)
}

// applyHimedere makes the speaker act like imperious royalty.
func applyHimedere(text string) string {
	return applyPrefixSuffix(text, himederePfx, himedereSfx)
}

// applyKamidere makes the speaker act like a self-proclaimed god.
func applyKamidere(text string) string {
	return applyPrefixSuffix(text, kamiderePfx, kamidereSfx)
}

// applyUndere makes the speaker agree with absolutely everything.
func applyUndere(text string) string {
	return applyPrefixSuffix(text, underePfx, undereSfx)
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
	return applyPrefixSuffix(text, mayaderePfx, mayadereSfx)
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

// --- Philosophical / Literary Punishments ---

// philosopherQuestions is a curated list of deep philosophical questions
// that get appended to the punished player's IC messages.
var philosopherQuestions = []string{
	"But what IS reality, really?",
	"If a tree falls in the forest and no one is around to hear it, does it make a sound?",
	"Do we choose our fate, or does fate choose us?",
	"What is the true nature of consciousness?",
	"Is morality absolute or merely a social construct?",
	"If everything is relative, is that statement itself relative?",
	"Can one truly know anything, or is all knowledge an illusion?",
	"What came before the beginning of time?",
	"Does the self persist through change, or are we new people every moment?",
	"If I replace every plank of my ship, is it still the same ship?",
	"Why is there something rather than nothing?",
	"Is free will compatible with a deterministic universe?",
	"What separates right from wrong at the fundamental level?",
	"Is the examined life truly the only one worth living?",
	"Can language ever fully capture the truth of experience?",
	"What would you do if you had to live this moment forever?",
	"Are numbers discovered or invented?",
	"If your memories were replaced, would you still be you?",
	"Is justice the same as fairness?",
	"What is the value of a life well-lived versus a life well-enjoyed?",
	"Does the universe have meaning, or do we project meaning onto it?",
	"Can something be both true and unknowable?",
	"Is compassion an evolutionary accident or a moral imperative?",
	"What would it mean to live without regret?",
	"Is perfection even a coherent concept?",
}

// applyPhilosopher appends a random deep philosophical question to the text.
func applyPhilosopher(text string) string {
	q := philosopherQuestions[rand.Intn(len(philosopherQuestions))]
	return truncateText(text + " " + q)
}

// poeticSuffixes are lyrical flourishes appended after a poem-style message.
var poeticSuffixes = []string{
	"— as the moon weeps silver tears",
	"— sung by the nightingale at dusk",
	"— etched upon the wind's lament",
	"— whispered to the dying stars",
	"— carved in the bones of the earth",
	"— lost in the amber of memory",
	"— breathed by the lips of twilight",
	"— cast upon the trembling sea",
	"— woven through the veil of dreams",
	"— written in the ash of forgotten fires",
}

// poeticPrefixes are lyrical phrases prepended to create a poetic opening.
var poeticPrefixes = []string{
	"O, hark! ",
	"Lo, and behold — ",
	"Hear me, ye muses: ",
	"From the depths of my soul: ",
	"In iambic devotion: ",
	"Thus spake the bard: ",
	"By the light of verse: ",
	"Sing, O Muse, of ",
	"With quill and trembling hand: ",
	"In rhyme and meter: ",
}

// applyPoet wraps the text in poetic flourishes.
func applyPoet(text string) string {
	prefix := poeticPrefixes[rand.Intn(len(poeticPrefixes))]
	suffix := poeticSuffixes[rand.Intn(len(poeticSuffixes))]
	return truncateText(prefix + text + " " + suffix)
}

// flipRune returns the upside-down Unicode equivalent of r, or r unchanged if
// there is no mapping.  Implemented as a switch so the compiler can emit a
// static lookup rather than a hash-map query.
func flipRune(r rune) rune {
	switch r {
	// Lowercase letters
	case 'a':
		return 'ɐ'
	case 'b':
		return 'q'
	case 'c':
		return 'ɔ'
	case 'd':
		return 'p'
	case 'e':
		return 'ǝ'
	case 'f':
		return 'ɟ'
	case 'g':
		return 'ƃ'
	case 'h':
		return 'ɥ'
	case 'i':
		return 'ı'
	case 'j':
		return 'ɾ'
	case 'k':
		return 'ʞ'
	case 'l':
		return 'l'
	case 'm':
		return 'ɯ'
	case 'n':
		return 'u'
	case 'o':
		return 'o'
	case 'p':
		return 'd'
	case 'q':
		return 'b'
	case 'r':
		return 'ɹ'
	case 's':
		return 's'
	case 't':
		return 'ʇ'
	case 'u':
		return 'n'
	case 'v':
		return 'ʌ'
	case 'w':
		return 'ʍ'
	case 'x':
		return 'x'
	case 'y':
		return 'ʎ'
	case 'z':
		return 'z'
	// Uppercase letters
	case 'A':
		return '∀'
	case 'B':
		return 'B'
	case 'C':
		return 'Ɔ'
	case 'D':
		return 'D'
	case 'E':
		return 'Ǝ'
	case 'F':
		return 'Ⅎ'
	case 'G':
		return 'פ'
	case 'H':
		return 'H'
	case 'I':
		return 'I'
	case 'J':
		return 'ſ'
	case 'K':
		return 'K'
	case 'L':
		return '˥'
	case 'M':
		return 'W'
	case 'N':
		return 'N'
	case 'O':
		return 'O'
	case 'P':
		return 'Ԁ'
	case 'Q':
		return 'Q'
	case 'R':
		return 'R'
	case 'S':
		return 'S'
	case 'T':
		return '┴'
	case 'U':
		return '∩'
	case 'V':
		return 'Λ'
	case 'W':
		return 'M'
	case 'X':
		return 'X'
	case 'Y':
		return '⅄'
	case 'Z':
		return 'Z'
	// Digits
	case '0':
		return '0'
	case '1':
		return 'Ɩ'
	case '2':
		return 'ᄅ'
	case '3':
		return 'Ɛ'
	case '4':
		return 'ㄣ'
	case '5':
		return 'ϛ'
	case '6':
		return '9'
	case '7':
		return 'ㄥ'
	case '8':
		return '8'
	case '9':
		return '6'
	// Punctuation
	case '.':
		return '˙'
	case ',':
		return '\''
	case '?':
		return '¿'
	case '!':
		return '¡'
	case '"':
		return '„'
	case '&':
		return '⅋'
	case '_':
		return '‾'
	default:
		return r
	}
}

// applyUpsidedown flips the text upside-down by reversing it and replacing
// each character with its Unicode upside-down equivalent.
//
// Optimisation: the reverse and flip are combined into a single
// strings.Builder pass over the original rune slice, eliminating the
// need for a second iteration.  The map[rune]rune used previously is
// replaced by a switch so there is no hash computation per character.
func applyUpsidedown(text string) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	var b strings.Builder
	// Pre-grow using the original byte length as a reasonable lower-bound.
	// Many flipped characters encode to 2-3 UTF-8 bytes (e.g. '∀', 'Ǝ'),
	// so the true output may be larger, but starting from len(text) avoids
	// re-allocation for typical short messages.
	b.Grow(len(text))
	// Walk the rune slice backwards and emit the flipped character.
	for i := n - 1; i >= 0; i-- {
		b.WriteRune(flipRune(runes[i]))
	}
	return truncateText(b.String())
}

// sarcasmCommentaries are parenthetical sarcastic remarks appended to messages.
var sarcasmCommentaries = []string{
	"(wow, really?)",
	"(how original)",
	"(please, tell me more)",
	"(groundbreaking stuff)",
	"(I am absolutely shocked)",
	"(never seen that one before)",
	"(oh, do go on)",
	"(you don't say)",
	"(what a revelation)",
	"(thanks for that insight)",
	"(truly stunning analysis)",
	"(absolutely riveting)",
	"(I'll try to contain my excitement)",
	"(cool story, bro)",
	"(fascinating, truly)",
	"(I'm taking notes)",
	"(bold claim, no notes)",
	"(the wisdom, it overwhelms me)",
	"(a real Einstein, this one)",
	"(I had no idea until you told me)",
}

// applySarcasm appends a sarcastic parenthetical remark to the text.
func applySarcasm(text string) string {
	comment := sarcasmCommentaries[rand.Intn(len(sarcasmCommentaries))]
	return truncateText(text + " " + comment)
}

// academicPrefixes are overly formal academic-style openers.
var academicPrefixes = []string{
	"It is worth noting that ",
	"According to my analysis, ",
	"Empirical evidence suggests that ",
	"As postulated by contemporary scholarship, ",
	"Within the theoretical framework of discourse, ",
	"One must consider, in this context, that ",
	"Preliminary research indicates that ",
	"A critical examination reveals that ",
	"The prevailing academic consensus holds that ",
	"Notwithstanding the aforementioned, ",
}

// academicSuffixes are pompous academic-style closers.
var academicSuffixes = []string{
	" — as documented in peer-reviewed literature.",
	", which warrants further investigation.",
	", per the extant scholarly record.",
	" — a hypothesis requiring empirical validation.",
	", consistent with the theoretical underpinnings of the field.",
	" — subject to methodological caveats, of course.",
	", as outlined in the seminal corpus.",
	" — a nuanced perspective demanding interdisciplinary inquiry.",
	", insofar as the data are to be believed.",
	", within the broader socio-cultural paradigm.",
}

// applyAcademic wraps the text in overly formal academic language.
func applyAcademic(text string) string {
	prefix := academicPrefixes[rand.Intn(len(academicPrefixes))]
	suffix := academicSuffixes[rand.Intn(len(academicSuffixes))]
	return truncateText(prefix + text + suffix)
}

// recipeStepVerbs are culinary verbs used to format recipe-step messages.
var recipeStepVerbs = []string{
	"Combine", "Mix", "Fold in", "Whisk together", "Stir in",
	"Blend", "Layer", "Season with", "Garnish with", "Add",
	"Incorporate", "Marinate with", "Drizzle over", "Sprinkle",
	"Reduce", "Simmer", "Bring to a boil", "Bake at 350°F with",
}

// recipeEndings are amusing cooking-instruction suffixes.
var recipeEndings = []string{
	"Serve immediately.",
	"Let rest for 5 minutes before plating.",
	"Garnish with fresh confusion.",
	"Pairs well with a dry sense of humour.",
	"Best enjoyed piping hot.",
	"Cover and refrigerate for 24 hours.",
	"Yields approximately one awkward situation.",
	"Do not overmix.",
	"Season to taste.",
	"Sprinkle with regret before serving.",
	"Baste every 15 minutes for optimal results.",
	"Allow to cool before consuming.",
}

// applyRecipe reformats the text as a cooking recipe instruction step.
func applyRecipe(text string) string {
	verb := recipeStepVerbs[rand.Intn(len(recipeStepVerbs))]
	ending := recipeEndings[rand.Intn(len(recipeEndings))]
	var b strings.Builder
	b.Grow(len("Step 1: ") + len(verb) + len(" \"") + len(text) + len("\". ") + len(ending))
	b.WriteString("Step 1: ")
	b.WriteString(verb)
	b.WriteString(" \"")
	b.WriteString(text)
	b.WriteString("\". ")
	b.WriteString(ending)
	return truncateText(b.String())
}

// applyQuote wraps each word in quotation marks with a 20% chance per word.
func applyQuote(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		if rand.Float32() < 0.2 {
			words[i] = "\"" + word + "\""
		}
	}
	return truncateText(strings.Join(words, " "))
}

// areaRandomPunishments is the curated pool of STATELESS punishment types
// that the "punishment_area" feature picks from on every IC message. Each
// entry must be safe to call repeatedly without any PunishmentState —
// i.e. no torment (state cycling), no lovebomb (target UID), no
// third-person (needs showname context). Translator is handled separately
// in applyAreaRandomPunishment because it requires config plumbing.
var areaRandomPunishments = []PunishmentType{
	PunishmentBackward,
	PunishmentStutterstep,
	PunishmentElongate,
	PunishmentUppercase,
	PunishmentLowercase,
	PunishmentRobotic,
	PunishmentAlternating,
	PunishmentFancy,
	PunishmentUwu,
	PunishmentPirate,
	PunishmentShakespearean,
	PunishmentCaveman,
	PunishmentCensor,
	PunishmentConfused,
	PunishmentParanoid,
	PunishmentDrunk,
	PunishmentHiccup,
	PunishmentWhistle,
	PunishmentMumble,
	PunishmentSpaghetti,
	PunishmentRng,
	PunishmentEssay,
	PunishmentAutospell,
	PunishmentSubtitles,
	PunishmentSpotlight,
	PunishmentMonkey,
	PunishmentSnake,
	PunishmentDog,
	PunishmentCat,
	PunishmentBird,
	PunishmentCow,
	PunishmentFrog,
	PunishmentDuck,
	PunishmentHorse,
	PunishmentLion,
	PunishmentZoo,
	PunishmentBunny,
	PunishmentTsundere,
	PunishmentYandere,
	PunishmentKuudere,
	PunishmentDandere,
	PunishmentDeredere,
	PunishmentHimedere,
	PunishmentKamidere,
	PunishmentUndere,
	PunishmentBakadere,
	PunishmentMayadere,
	PunishmentEmoticon,
	PunishmentDegrade,
	PunishmentTourettes,
	PunishmentSlang,
	PunishmentThesaurusOverload,
	PunishmentValleyGirl,
	PunishmentBabytalk,
	PunishmentUnreliableNarrator,
	PunishmentUncannyValley,
	Punishment51,
	PunishmentPhilosopher,
	PunishmentPoet,
	PunishmentUpsidedown,
	PunishmentSarcasm,
	PunishmentAcademic,
	PunishmentRecipe,
	PunishmentQuote,
	PunishmentTimewarp,
	PunishmentMorse,
	PunishmentRickroll,
	PunishmentVowelhell,
	PunishmentChef,
	PunishmentKaren,
	PunishmentPassiveAggressive,
	PunishmentNervous,
	PunishmentDreamSequence,
	PunishmentGordonRamsay,
}

// pickAreaRandomPunishment returns a random punishment type from the pool.
// Exported as a var so the punishment-area tests can stub it if needed.
var pickAreaRandomPunishment = func() PunishmentType {
	return areaRandomPunishments[rand.Intn(len(areaRandomPunishments))]
}

// applyAreaRandomPunishmentText applies ONE random stateless punishment to
// text and returns the result. Translator is opt-in via includeTranslator —
// callers check translatorEnabled() before passing true.
//
// This is the text-only helper. The full IC-path hook (applyAreaRandomPunishment)
// also tweaks display name fields for emoji/uncanny — see netprotocol.go.
// Returned second value is the picked type, for logging/debug.
func applyAreaRandomPunishmentText(text string, includeTranslator bool) (string, PunishmentType) {
	// When translator is live, give it a real chance to be picked so the
	// area feels unpredictable rather than just "same list of filters".
	if includeTranslator && rand.Intn(len(areaRandomPunishments)+1) == 0 {
		return applyTranslator(text, "random"), PunishmentTranslator
	}
	pType := pickAreaRandomPunishment()
	return ApplyPunishmentToText(text, pType), pType
}

// applyTimewarp shuffles word order so time feels out of joint.
func applyTimewarp(text string) string {
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	return truncateText(strings.Join(words, " "))
}

// morseTable maps ASCII letters and digits to International Morse Code.
var morseTable = map[rune]string{
	'a': ".-", 'b': "-...", 'c': "-.-.", 'd': "-..", 'e': ".", 'f': "..-.",
	'g': "--.", 'h': "....", 'i': "..", 'j': ".---", 'k': "-.-", 'l': ".-..",
	'm': "--", 'n': "-.", 'o': "---", 'p': ".--.", 'q': "--.-", 'r': ".-.",
	's': "...", 't': "-", 'u': "..-", 'v': "...-", 'w': ".--", 'x': "-..-",
	'y': "-.--", 'z': "--..",
	'0': "-----", '1': ".----", '2': "..---", '3': "...--", '4': "....-",
	'5': ".....", '6': "-....", '7': "--...", '8': "---..", '9': "----.",
}

// rickrollLines are safe, generic meme stand-ins — a wink at the song without
// quoting copyrighted lyrics verbatim.
var rickrollLines = []string{
	"we're no strangers to this courtroom, are we?",
	"you know the rules, and so do i...",
	"a full commitment is what i'm thinkin' of",
	"never gonna give this case up",
	"never gonna let this witness down",
	"never gonna run around and desert the jury",
	"never gonna say goodbye to the defense",
	"never gonna tell a lie and hurt objection",
	"i just wanna tell you how the verdict's feeling",
	"gotta make you understand the evidence",
}

// applyRickroll replaces the message with a meme-styled placeholder line.
// Lyrics-adjacent only — no copyrighted content is reproduced.
func applyRickroll(_ string) string {
	return rickrollLines[rand.Intn(len(rickrollLines))]
}

// pickupLines is a deliberately enormous catalogue of the cheesiest, most
// dead-on-arrival pickup lines in existence. Each IC message from a punished
// player is replaced with one of these at random; the wider the range and
// worse the energy, the funnier the public humiliation.
var pickupLines = []string{
	// Classic dad-tier groaners
	"Are you a parking ticket? Because you've got 'fine' written all over you.",
	"Did it hurt when you fell from heaven? Because you landed face-first.",
	"Are you French? Because Eiffel for you.",
	"Is your name Google? Because you have everything I've been searching for.",
	"If you were a vegetable, you'd be a cute-cumber.",
	"Are you a magnet? Because I'm attracted to you and slightly concerned about my pacemaker.",
	"Do you have a Band-Aid? I just scraped my knee falling for you.",
	"If beauty were time, you'd be eternity. Sadly, I'm on a 30-second cooldown.",
	"Are you Wi-Fi? Because I'm feeling a connection — and also throttled.",
	"Was your dad a baker? Because you've got nice buns.",
	// Courtroom / AO2 themed
	"Objection — your honor, my heart is hearsay.",
	"You must be exhibit A, because you're the only evidence I need.",
	"If loving you is a crime, sustain me.",
	"Your honor, I plead guilty — to falling for you.",
	"I'd cross-examine you all night long, counsel.",
	"You're the prosecution and I'm the defense, and somehow we're both in love.",
	"Did the bailiff arrest you? Because you stole my heart.",
	"Are you a witness statement? Because I want to read you over and over.",
	"My client and I would like to enter a joint motion: dinner, Friday.",
	"Strike that from the record — but don't strike me from your heart.",
	// Science / nerd
	"Are you made of copper and tellurium? Because you're Cu-Te.",
	"You must be a 90-degree angle, because you're looking right.",
	"Are you a carbon sample? Because I want to date you.",
	"If you were a triangle, you'd be acute one.",
	"My love for you is like dividing by zero — undefined and probably a crash.",
	"You must be the speed of light, because time stops when I look at you.",
	"Are you the square root of -1? Because you can't be real.",
	"I'd never split from you — we're more stable than uranium-238.",
	"Are you a neutrino? Because you've passed right through my heart undetected.",
	"You're so hot, you'd violate the second law of thermodynamics.",
	// Food
	"If you were a fruit, you'd be a fine-apple.",
	"Are you a raisin? Because you're raisin' my standards.",
	"Do you like raisins? How do you feel about a date?",
	"Are you a microwave burrito? Because you've got me hot in the middle and confused on the outside.",
	"You must be a stack of pancakes, because I'm syrup-sly into you.",
	"Are you a vending machine? Because I'd put everything I had into you and still walk away with nothing.",
	"If you were a pizza topping, you'd be supreme.",
	"Are you cilantro? Because half the room hates you and I find you incredible.",
	// Surreal / cursed / self-defeating
	"Are you a loading screen? Because I've been staring at you for way too long.",
	"You must be a CAPTCHA, because I keep failing you.",
	"Are you a 404 error? Because I can't find you in my dating life.",
	"You're like a software update — inconvenient, mandatory, and somehow improving my life.",
	"Are you a tax form? Because you make me sweat and I don't fully understand you.",
	"If you were a shopping cart, I'd never return you.",
	"You must be a dropped call, because I can't stop thinking about reconnecting.",
	"Are you a spreadsheet? Because I want to fill you with values.",
	"You're like a Roomba — going in circles but somehow making my life better.",
	"Are you my browser history? Because I'd never let anyone else see you.",
	// Disastrously bad / aggressively cheesy
	"Hey, did you sit in a pile of sugar? Because you've got a pretty sweet... never mind.",
	"Are you a beaver? Because daaaaaam.",
	"If you were a chicken, you'd be impeccable.",
	"Hey baby, are you a thesaurus? Because you give meaning to my life. Synonym: also you.",
	"You must be tired, because you've been running through my CPU all day.",
	"Are you a fire alarm? Because you're loud, hot, and I don't know how to turn you off.",
	"Roses are red, violets are blue, I'm bad at poetry, marry me too.",
	"I'm not a photographer, but I can picture us getting blocked.",
	"Are you a parking lot? Because I'd circle you for forty minutes and still not figure you out.",
	"If kisses were snowflakes, I'd send you a mild drizzle and a vague apology.",
	// Ominous / unhinged
	"Are you a haunted lighthouse? Because something in me is drawn to you and probably shouldn't be.",
	"You must be a long Wikipedia article, because I've gone three hours deep and forgotten my original question.",
	"Are you the void? Because I keep yelling into you and getting nothing back.",
	"If you were a horror movie, I'd watch you alone, in the dark, against medical advice.",
	"You're like a museum exhibit — I'm not allowed to touch you and a guard is watching me.",
	"Are you an unread terms-of-service? Because I'm about to agree to anything you say.",
	"You must be an automated phone menu, because I keep pressing buttons hoping to reach a human.",
	"If you were a season, you'd be an unseasonably warm February — confusing and unsustainable.",
	"Are you a smoke detector at 3 AM? Because something in me is going off and I can't ignore it.",
	"You're the exact pickup line I'd be embarrassed to be caught using on you.",
}

// applyPickup replaces the message with a random cheesy pickup line.
// The original text is intentionally discarded — the punishment IS the
// substitution. Each delivery is a fresh, public, deeply preventable disaster.
func applyPickup(_ string) string {
	return pickupLines[rand.Intn(len(pickupLines))]
}

// karenPrefixes are opening escalations prepended to the original message.
var karenPrefixes = []string{
	"Excuse me, ",
	"Um, hello?? ",
	"Okay, I'm going to need you to listen very carefully. ",
	"This is ABSOLUTELY unacceptable. ",
	"I can't believe I even have to say this, but ",
	"I don't THINK so. ",
	"Do you even know who I am? ",
	"As a paying customer of this courtroom, ",
	"I was BORN in 1978, I have rights. ",
	"I did my own research. ",
}

// karenSuffixes are the entitled escalations appended afterwards.
var karenSuffixes = []string{
	" I want to speak to your manager.",
	" I am calling corporate.",
	" My husband is a lawyer, by the way.",
	" This is going on Yelp.",
	" I will be reviewing the CCTV.",
	" I want names. I want badge numbers.",
	" Refund. Compensation. Immediately.",
	" You just lost a five-star customer.",
	" I've been coming here for TWENTY YEARS.",
	" I will not be gaslit by a first-year clerk.",
}

// applyKaren wraps the message in escalating entitlement. Picks a random
// opener and a random closer so it varies every IC message.
func applyKaren(text string) string {
	prefix := karenPrefixes[rand.Intn(len(karenPrefixes))]
	suffix := karenSuffixes[rand.Intn(len(karenSuffixes))]
	// About 1 in 5 messages go full-tantrum with two suffixes.
	if rand.Float32() < 0.2 {
		suffix += " " + karenSuffixes[rand.Intn(len(karenSuffixes))]
	}
	return truncateText(prefix + text + suffix)
}

// passiveAggressiveOpeners prepend a cold, performatively-polite framing.
var passiveAggressiveOpeners = []string{
	"Per my last message, ",
	"As I said before, ",
	"Just to reiterate, ",
	"Not to be rude, but ",
	"With all due respect, ",
	"No offense, but ",
	"I guess I'll just say it: ",
	"Well, if you must know, ",
	"It's fine. Really. ",
	"Okay sure, whatever, ",
}

// passiveAggressiveClosers tack a deeply-not-fine sign-off onto the end.
var passiveAggressiveClosers = []string{
	" ...fine.",
	" ...whatever.",
	" ...I guess.",
	" ...if that's what you want.",
	" ...no worries, I'll just handle it myself.",
	" ...must be nice.",
	" ...cool cool cool.",
	" ...sure, that's totally fair.",
	" ...anyway.",
	" ...k.",
	" :)",
	" :)))",
}

// applyPassiveAggressive wraps a message with chilly politeness framings and
// emoticon smileys that absolutely do not mean the sender is happy.
func applyPassiveAggressive(text string) string {
	opener := passiveAggressiveOpeners[rand.Intn(len(passiveAggressiveOpeners))]
	closer := passiveAggressiveClosers[rand.Intn(len(passiveAggressiveClosers))]
	// 30% of the time, add a second closer to amp up the chill.
	if rand.Float32() < 0.3 {
		closer += passiveAggressiveClosers[rand.Intn(len(passiveAggressiveClosers))]
	}
	return truncateText(opener + text + closer)
}

// nervousFillers are little "uh / um / ah" tokens inserted between words.
var nervousFillers = []string{
	"um,", "uh,", "ah,", "er,", "uhh,", "umm,", "hmm,", "w-wait,", "s-so,", "i-i mean,",
	"sorry,", "s-sorry,", "n-no wait,", "that is to say,", "oh gosh,", "oh no,",
}

// nervousTails are jittery sentence endings occasionally appended.
var nervousTails = []string{
	" ...right?", " ...i think?", " ...i-if that's okay?", " ...sorry!",
	" ...oh god.", " ...please don't be mad.", " ...i'll shut up now.",
}

// stutterFirst inserts a stutter on the first consonant of a word.
func stutterFirst(word string) string {
	if len(word) == 0 {
		return word
	}
	runes := []rune(word)
	if !unicode.IsLetter(runes[0]) {
		return word
	}
	return string(runes[0]) + "-" + word
}

// applyNervous sprinkles fillers, stutters, and jittery tails so the speaker
// sounds one courtroom sneeze away from passing out.
func applyNervous(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "u-um..."
	}
	var out []string
	for i, w := range words {
		// 25% chance to insert a filler token before this word.
		if rand.Float32() < 0.25 {
			out = append(out, nervousFillers[rand.Intn(len(nervousFillers))])
		}
		// 35% chance to stutter the word itself.
		if rand.Float32() < 0.35 {
			out = append(out, stutterFirst(w))
		} else {
			out = append(out, w)
		}
		// Small chance to trail off mid-sentence.
		if i == len(words)/2 && rand.Float32() < 0.2 {
			out = append(out, "...")
		}
	}
	result := strings.Join(out, " ")
	if rand.Float32() < 0.6 {
		result += nervousTails[rand.Intn(len(nervousTails))]
	}
	return truncateText(result)
}

// dreamAdjectives, dreamNouns, and dreamVerbs feed the dream-rewrite function
// below. None of these contain anything more sinister than melting clocks.
var dreamAdjectives = []string{
	"floating", "shimmering", "dissolving", "whispering", "glowing", "endless",
	"impossible", "golden", "velvet", "rippling", "forgotten", "half-remembered",
}
var dreamNouns = []string{
	"the staircase", "a door that was never there", "my old teacher", "a courtroom made of water",
	"the moon", "a cat with my voice", "the version of me at seven years old",
	"a hallway that keeps going", "the choir", "someone I haven't met yet",
}
var dreamVerbs = []string{
	"is humming a lullaby",
	"is tearing up the evidence",
	"is on fire but laughing",
	"won't stop staring at me",
	"is begging me to wake up",
	"is holding my hand",
	"is turning into a bird",
	"is asking a question I can't hear",
}

// applyDreamSequence rewrites the message as a dreamlike gloss. Intensity
// ramps up based on message length — longer messages get more surreal.
func applyDreamSequence(text string) string {
	adj := dreamAdjectives[rand.Intn(len(dreamAdjectives))]
	noun := dreamNouns[rand.Intn(len(dreamNouns))]
	verb := dreamVerbs[rand.Intn(len(dreamVerbs))]

	// Short messages become pure dreamlogic. Longer messages keep a trace of
	// the original text, filtered through softly shimmering framing.
	if utf8.RuneCountInString(text) < 24 {
		return truncateText(fmt.Sprintf("...and then %s %s %s...", adj, noun, verb))
	}
	// Keep about half the words, wrapped in dreamy framing.
	words := strings.Fields(text)
	keep := len(words) / 2
	if keep < 3 {
		keep = len(words)
	}
	remembered := strings.Join(words[:keep], " ")
	return truncateText(fmt.Sprintf("i dreamt that %s... but %s %s %s.",
		remembered, adj, noun, verb))
}

// chefReplacements is the classic "Swedish Chef" word-level substitution set.
var chefReplacements = map[string]string{
	"the":   "zee",
	"The":   "Zee",
	"this":  "thees",
	"This":  "Thees",
	"that":  "thet",
	"with":  "veet",
	"very":  "fery",
	"are":   "iire",
	"for":   "fur",
	"our":   "oor",
	"ing":   "eeng",
	"and":   "und",
	"have":  "hefe",
	"is":    "ees",
	"to":    "tu",
	"of":    "uf",
	"on":    "un",
	"at":    "et",
	"a":     "e",
	"I":     "I",
	"my":    "mee",
	"you":   "yoo",
	"your":  "yoor",
	"don't": "doon't",
	"hello": "hullo",
}

// applyChef runs the Swedish-Chef filter: letter swaps + a trailing
// "bork bork bork!" for good measure.
func applyChef(text string) string {
	words := strings.Fields(text)
	for i, w := range words {
		if r, ok := chefReplacements[strings.ToLower(w)]; ok {
			// Preserve the first-letter capitalisation of the original.
			if len(w) > 0 && unicode.IsUpper([]rune(w)[0]) && len(r) > 0 {
				rr := []rune(r)
				rr[0] = unicode.ToUpper(rr[0])
				r = string(rr)
			}
			words[i] = r
			continue
		}
		// Letter-level vowel tweaks: o -> oo, e -> ee (softly).
		var b strings.Builder
		for _, c := range w {
			switch c {
			case 'o':
				b.WriteString("oo")
			case 'O':
				b.WriteString("Oo")
			case 'w':
				b.WriteRune('v')
			case 'W':
				b.WriteRune('V')
			default:
				b.WriteRune(c)
			}
		}
		words[i] = b.String()
	}
	result := strings.Join(words, " ") + " bork bork bork!"
	return truncateText(result)
}

// applyVowelhell replaces every consonant with a random vowel, keeping
// word/line structure intact so the result still looks like "text".
func applyVowelhell(text string) string {
	vowels := []rune{'a', 'e', 'i', 'o', 'u'}
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			isVowel := strings.ContainsRune("aeiouAEIOU", r)
			if isVowel {
				b.WriteRune(r)
			} else {
				v := vowels[rand.Intn(len(vowels))]
				if unicode.IsUpper(r) {
					v = unicode.ToUpper(v)
				}
				b.WriteRune(v)
			}
		default:
			b.WriteRune(r)
		}
	}
	return truncateText(b.String())
}

// applyMorse converts text to Morse code. Letters are separated by spaces,
// words by " / ". Unknown characters are dropped.
func applyMorse(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 4)
	firstWord := true
	for _, word := range strings.Fields(strings.ToLower(text)) {
		if !firstWord {
			b.WriteString(" / ")
		}
		firstWord = false
		firstLetter := true
		for _, r := range word {
			code, ok := morseTable[r]
			if !ok {
				continue
			}
			if !firstLetter {
				b.WriteByte(' ')
			}
			firstLetter = false
			b.WriteString(code)
		}
	}
	out := b.String()
	if out == "" {
		return "..."
	}
	return truncateText(out)
}

// ── Brainrot punishment ────────────────────────────────────────────────────

// brainrotItalian are complete Italian-brainrot entity names that replace the
// whole message (the "full-send" path).
var brainrotItalian = []string{
	"Bombardiro Crocodilo",
	"Tralalero Tralala",
	"Brr Brr Patapim",
	"Burbaloni Luliloli",
	"Tung Tung Tung Tung Sahur",
	"Lirili Larila",
	"Cappuccino Assassino",
	"Frigo Camelo",
	"Ballerina Cappuccina",
	"Chimpanzini Bananini",
	"Bobritto Bandito",
	"La Vaca Saturno Saturnita",
	"Trippi Troppi",
	"Glorbo Fruttodrillo",
	"Bombombini Gusini",
	"Crocodillo Brrrr",
	"Tridenti Rondini",
	"Giraffini Tozzolini",
	"Lirilì Larilà",
	"Panzerotti Luigini",
}

// brainrotSkibidi are skibidi-universe phrases substituted as prefixes.
var brainrotSkibidi = []string{
	"skibidi toilet",
	"skibidi sigma",
	"skibidi rizz",
	"skibidi ohio",
	"skibidi gyatt",
	"skibidi fanum tax",
	"sigma skibidi",
	"ohio skibidi",
}

// brainrotPrefixes are openers injected before the original text.
var brainrotPrefixes = []string{
	"SKIBIDI OHIO 💀",
	"no cap fr fr",
	"bro fell off 💀",
	"W rizz detected",
	"only in ohio:",
	"sigma grindset:",
	"real and based:",
	"understood the assignment:",
	"it's giving main character:",
	"GYATT 🗣️",
	"fanum tax incoming:",
	"chronically online take:",
	"delulu arc incoming:",
	"NPC behavior detected:",
	"ratio + L + bozo:",
	"ate and left no crumbs:",
	"not the brainrot 💀",
	"POV: you live in ohio:",
	"bro cooked fr:",
	"that's bussin no cap:",
}

// brainrotSuffixes are closers tacked onto the original text.
var brainrotSuffixes = []string{
	"💀💀💀",
	"no cap fr fr",
	"bro really said that 💀",
	"W rizz",
	"skibidi",
	"ohio moment",
	"sigma moment",
	"gyatt",
	"based and redpilled",
	"understood the assignment",
	"it's giving",
	"not the flop era 😭",
	"fanum tax",
	"real",
	"periodt",
	"slay bestie",
	"ate",
	"bussin fr",
	"lowkey highkey tho",
	"touch grass challenge: failed",
	"main character behavior",
	"Tralalero Tralala 🐬",
	"Bombardiro Crocodilo 🐊",
	"Tung Tung Tung 🥁",
	"Brr Brr Patapim 🦆",
	"Cappuccino Assassino ☕",
	"Chimpanzini Bananini 🍌",
	"Bobritto Bandito 🦫",
}

// brainrotInserts are mid-sentence injections.
var brainrotInserts = []string{
	"skibidi",
	"fr fr",
	"no cap",
	"sigma",
	"ohio",
	"rizz",
	"gyatt",
	"based",
	"bussin",
	"slay",
	"W",
	"L",
	"ratio",
	"delulu",
	"periodt",
	"real",
	"Bombardiro",
	"Tralalero",
	"Tung Tung",
	"Brr Brr",
	"Chimpanzini",
	"Cappuccino Assassino",
}

// brainrotWordMap replaces common words with brainrot equivalents.
var brainrotWordMap = map[string]string{
	"i":        "sigma me",
	"you":      "u (ratio)",
	"he":       "that NPC",
	"she":      "that NPC",
	"they":     "those NPCs",
	"we":       "the squad",
	"is":       "be like",
	"are":      "be like",
	"was":      "lowkey was",
	"think":    "fr think",
	"know":     "deadass know",
	"good":     "bussin",
	"bad":      "an L",
	"yes":      "W fr",
	"no":       "ratio",
	"cool":     "based",
	"okay":     "skibidi okay",
	"hello":    "skibidi hello",
	"hi":       "ayo",
	"bye":      "later npc",
	"because":  "cuz no cap",
	"but":      "but fr tho",
	"and":      "and ohio",
	"very":     "lowkey highkey",
	"really":   "fr fr",
	"actually": "deadass",
	"maybe":    "delulu maybe",
	"always":   "always no cap",
	"never":    "never (ratio)",
	"true":     "based and true",
	"false":    "mid and capped",
	"right":    "W",
	"wrong":    "L bozo",
	"great":    "goated fr",
	"terrible": "mid af",
}

// applyBrainrot corrupts a message with maximum skibidi sigma Italian brainrot.
// Three modes are chosen at random:
//  1. Full replace — swap the whole message for an Italian brainrot entity name.
//  2. Wrap — keep the original text but slam a brainrot prefix and suffix around it.
//  3. Inject — scatter brainrot keywords between words AND word-map common terms.
func applyBrainrot(text string) string {
	r := rand.Float32()

	switch {
	case r < 0.25:
		// Full Italian-brainrot replacement.
		entity := brainrotItalian[rand.Intn(len(brainrotItalian))]
		suffix := brainrotSuffixes[rand.Intn(len(brainrotSuffixes))]
		return truncateText(entity + " " + suffix)

	case r < 0.50:
		// Skibidi entity + original text + suffix.
		skib := brainrotSkibidi[rand.Intn(len(brainrotSkibidi))]
		suffix := brainrotSuffixes[rand.Intn(len(brainrotSuffixes))]
		return truncateText(strings.ToUpper(skib) + " " + text + " " + suffix)

	case r < 0.75:
		// Prefix + text + double suffix for maximum chaos.
		prefix := brainrotPrefixes[rand.Intn(len(brainrotPrefixes))]
		suffix1 := brainrotSuffixes[rand.Intn(len(brainrotSuffixes))]
		suffix2 := brainrotSuffixes[rand.Intn(len(brainrotSuffixes))]
		return truncateText(prefix + " " + text + " " + suffix1 + " " + suffix2)

	default:
		// Word-level injection: replace common words + randomly insert brainrot.
		words := strings.Fields(strings.ToLower(text))
		var b strings.Builder
		b.Grow(len(text) * 2)
		for i, w := range words {
			if i > 0 {
				b.WriteByte(' ')
			}
			// Strip trailing punctuation for lookup.
			punct := byte(0)
			base := w
			if n := len(base); n > 0 {
				c := base[n-1]
				if c == '.' || c == ',' || c == '!' || c == '?' || c == ';' || c == ':' {
					punct = c
					base = base[:n-1]
				}
			}
			if rep, ok := brainrotWordMap[base]; ok {
				b.WriteString(rep)
			} else {
				b.WriteString(base)
			}
			if punct != 0 {
				b.WriteByte(punct)
			}
			// ~30% chance to inject a random brainrot word after this token.
			if rand.Float32() < 0.30 {
				b.WriteByte(' ')
				b.WriteString(brainrotInserts[rand.Intn(len(brainrotInserts))])
			}
		}
		// Always cap with a suffix so it never just looks like a word-swap.
		suffix := brainrotSuffixes[rand.Intn(len(brainrotSuffixes))]
		b.WriteByte(' ')
		b.WriteString(suffix)
		return truncateText(b.String())
	}
}

// gordonRamsayQuotes is a roster of iconic Gordon Ramsay outbursts. Used by
// applyGordonRamsay to overwrite each IC line — the original text is discarded.
var gordonRamsayQuotes = []string{
	"This lamb is so undercooked it's still following its mother around!",
	"You donkey! What are you doing?!",
	"This risotto is so raw it's still growing!",
	"It's RAW! It's f***ing RAW!",
	"Hello? Hello?! Is anyone in there?!",
	"My gran could do better — and she's dead!",
	"Where's the lamb sauce?! Where IS the lamb sauce?!",
	"You absolute muppet, this is dreadful.",
	"This salmon is so undercooked it's still swimming!",
	"Get out. Get out of my kitchen!",
	"Shut it down! Shut the kitchen DOWN!",
	"This pasta is so overcooked it's gone back in time and become flour again.",
	"You call this food? I wouldn't feed this to my dog.",
	"Bland. Bland. BLAND. Wake up!",
	"Oh my god, that is disgusting.",
	"What is this? It's a wet, soggy mess!",
	"This is a disaster. An absolute disaster.",
	"Are you trying to poison the customers?",
	"You've ruined it. You've completely ruined it.",
	"This chicken is so dry, it's a fossil.",
	"Bin it. Bin the lot of it. Now.",
	"You're standing there like a stunned mullet, MOVE!",
	"Stop. Just stop. Stop everything.",
	"Tastes like the bottom of a birdcage.",
	"What were you thinking? Were you even thinking?!",
	"Come on, come on, come ON!",
	"Look at me. Look at me — focus!",
	"This is supposed to be a restaurant, not a hospital cafeteria.",
	"You numpty, this is bone dry.",
	"It's swimming in oil. SWIMMING.",
	"That is the worst plate of food I have ever seen.",
	"Wake up! WAKE UP!",
	"You've got the brains of a banana.",
	"Soggy bottom. Soggy bottom! It's a soggy bottom!",
	"Stop pratting about and cook!",
	"This kitchen is a joke. A bloody joke.",
	"Tasteless. Boring. Lazy. Next!",
	"Get the hairnet off your face — that's your eyebrows, you donkey.",
	"You're shaking. Why are you shaking? It's a saucepan, not a snake.",
	"My nan, with no teeth, could chew this faster.",
	"This dessert is colder than my ex's heart.",
	"You couldn't run a bath, let alone a kitchen.",
	"Pathetic. Truly, magnificently pathetic.",
	"It's so salty I just got high blood pressure looking at it.",
	"This sauce broke. Like your career is about to.",
	"Move with purpose! PURPOSE!",
	"Listen to me, listen, listen — you're killing me.",
	"This beef Wellington — what is going on inside it? An autopsy?",
	"Where is the seasoning? Where is the LOVE?",
	"Are you a chef or a hostage?",
	"This plate looks like a crime scene.",
	"You absolute disgrace of an apron.",
	"Get a grip. Get a GRIP.",
	"That's not seared, that's traumatised.",
	"Service! SERVICE!",
	"You've got one job. ONE.",
	"This is a kitchen, not a kindergarten — though right now I can't tell.",
	"It's overworked, it's overcooked, it's over.",
	"Even the flies wouldn't touch this.",
	"You're a chef? I'm a giraffe.",
	"This is undercooked, that is overcooked, and you — you're just cooked.",
	"Out. Out. OUT.",
}

// applyGordonRamsay replaces the IC text with a Gordon Ramsay tirade.
// Picks a quote at random from gordonRamsayQuotes; the original text is
// discarded (the punishment is meant to silence the speaker behind the meme).
func applyGordonRamsay(_ string) string {
	return truncateText(gordonRamsayQuotes[rand.Intn(len(gordonRamsayQuotes))])
}
