/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: the /weeb punishment.

   Sprinkles anime romaji over everything the target says: common English
   words swap to their romaji equivalents, capitalized names grow honorifics
   (-chan/-kun/-senpai/...), messages gain interjections ("Nani?!",
   "Yare yare…") and sentence-final particles ("desu~", "dattebayo!").

   The romaji corpus below is intentionally enormous (350+ distinct entries
   across four tables) so the output stays fresh over long punishments. */

package athena

import (
	"math/rand"
	"strings"
	"unicode"
)

// weebWordTable maps lowercase English words to romaji replacements.
var weebWordTable = map[string]string{
	// Greetings & courtesy
	"hello": "konnichiwa", "hi": "yahho", "hey": "oi", "goodbye": "sayonara",
	"bye": "ja ne", "goodnight": "oyasumi", "morning": "ohayou",
	"thanks": "arigatou", "thank": "arigatou", "sorry": "gomen",
	"please": "onegai", "welcome": "youkoso", "congratulations": "omedetou",
	"congrats": "omedetou", "cheers": "kanpai", "farewell": "saraba",
	// Basic responses
	"yes": "hai", "no": "iie", "okay": "daijoubu", "ok": "daijoubu",
	"maybe": "tabun", "really": "hontou ni", "seriously": "maji de",
	"sure": "mochiron", "never": "zettai ni", "always": "itsumo",
	"understood": "wakatta", "right": "sou da",
	// Question words
	"what": "nani", "why": "doushite", "who": "dare", "where": "doko",
	"when": "itsu", "how": "dou",
	// Pronouns & people
	"i": "watashi", "me": "ore", "you": "omae", "he": "kare", "she": "kanojo",
	"everyone": "minna", "everybody": "minna", "friend": "tomodachi",
	"friends": "tomodachi", "enemy": "teki", "person": "hito",
	"teacher": "sensei", "student": "seito", "senior": "senpai",
	"junior": "kouhai", "brother": "onii-chan", "sister": "onee-chan",
	"mom": "okaa-san", "mother": "okaa-san", "dad": "otou-san",
	"father": "otou-san", "grandma": "obaa-chan", "grandpa": "ojii-chan",
	"master": "shishou", "boss": "taichou", "princess": "hime",
	"prince": "ouji", "king": "ousama", "queen": "joou", "hero": "eiyuu",
	"villain": "akuyaku", "ninja": "shinobi", "wizard": "mahoutsukai",
	// Adjectives & exclamations
	"cute": "kawaii", "cool": "kakkoii", "amazing": "sugoi",
	"awesome": "sugoi", "incredible": "sugoi", "scary": "kowai",
	"dangerous": "abunai", "beautiful": "kirei", "pretty": "kawaii",
	"weird": "hen", "stupid": "baka", "idiot": "baka", "fool": "aho",
	"dummy": "baka", "pervert": "hentai", "liar": "usotsuki",
	"strong": "tsuyoi", "weak": "yowai", "fast": "hayai", "slow": "osoi",
	"big": "ookii", "small": "chiisai", "good": "yoi", "bad": "warui",
	"delicious": "oishii", "tasty": "oishii", "fun": "tanoshii",
	"happy": "ureshii", "sad": "kanashii", "angry": "okotteru",
	"annoying": "urusai", "loud": "urusai", "quiet": "shizuka",
	"gross": "kimoi", "disgusting": "kimoi", "strange": "fushigi",
	"mysterious": "fushigi", "boring": "tsumaranai",
	"interesting": "omoshiroi", "impossible": "muri", "terrible": "hidoi",
	"painful": "itai", "dangerous!": "abunai", "best": "saikou",
	"worst": "saitei", "serious": "majime", "lazy": "mendokusai",
	// Nouns
	"truth": "hontou", "lie": "uso", "lies": "uso", "secret": "himitsu",
	"promise": "yakusoku", "fate": "unmei", "destiny": "sadame",
	"dream": "yume", "dreams": "yume", "world": "sekai", "sky": "sora",
	"moon": "tsuki", "sun": "taiyou", "star": "hoshi", "stars": "hoshi",
	"flower": "hana", "rain": "ame", "snow": "yuki", "fire": "hi",
	"wind": "kaze", "lightning": "kaminari", "ocean": "umi", "sea": "umi",
	"sword": "katana", "blade": "yaiba", "power": "chikara",
	"magic": "mahou", "spell": "jumon", "battle": "tatakai",
	"fight": "tatakai", "war": "sensou", "victory": "shouri",
	"defeat": "haiboku", "training": "shugyou", "technique": "waza",
	"heart": "kokoro", "soul": "tamashii", "tears": "namida",
	"smile": "egao", "feelings": "kimochi", "memories": "omoide",
	"money": "okane", "house": "ie", "home": "uchi", "school": "gakkou",
	"class": "kurasu", "club": "bukatsu", "work": "shigoto",
	"job": "shigoto", "game": "geemu", "food": "gohan", "rice": "gohan",
	"water": "mizu", "tea": "ocha", "lunch": "bentou", "snack": "oyatsu",
	"meat": "niku", "cat": "neko", "cats": "neko", "dog": "inu",
	"dogs": "inu", "fox": "kitsune", "rabbit": "usagi", "bird": "tori",
	"fish": "sakana", "demon": "oni", "ghost": "yuurei",
	"monster": "bakemono", "god": "kami", "death": "shi",
	"problem": "mondai", "trouble": "yabai",
	// Courtroom flavour (it IS an AO server)
	"justice": "seigi", "evidence": "shouko", "lawyer": "bengoshi",
	"judge": "saibanchou", "court": "houtei", "guilty": "yuuzai",
	"innocent": "muzai", "objection": "igiari", "witness": "shounin",
	"testimony": "shougen", "verdict": "hanketsu",
	// Verbs & actions
	"wait": "matte", "stop": "yamete", "go": "ike", "run": "nigero",
	"help": "tasukete", "look": "mite", "listen": "kike", "see": "miru",
	"hear": "kiku", "say": "iu", "speak": "hanasu", "talk": "hanasu",
	"understand": "wakaru", "know": "shitteru", "remember": "oboeteru",
	"forget": "wasureru", "believe": "shinjiru", "win": "katsu",
	"lose": "makeru", "die": "shinu", "kill": "korosu", "love": "daisuki",
	"like": "suki", "hate": "kirai", "eat": "taberu", "drink": "nomu",
	"sleep": "neru", "wake": "okiro", "transform": "henshin",
	"explode": "bakuhatsu", "surrender": "kousan",
	// Time & misc
	"now": "ima", "later": "ato de", "today": "kyou", "tomorrow": "ashita",
	"yesterday": "kinou", "forever": "eien ni", "together": "issho ni",
	"alone": "hitori", "finally": "tsui ni", "very": "totemo",
	"little": "chotto", "again": "mou ichido", "everything": "zenbu",
	"nothing": "nani mo",
}

// weebInterjections are standalone romaji exclamations occasionally
// prepended to the message.
var weebInterjections = []string{
	"Nani?!", "NANI?!", "Nan da to?!", "Sugoi!", "Sugee!", "Yabai!",
	"Yare yare…", "Yare yare daze…", "Ara ara~", "Oya oya…", "Uso!",
	"Uso da!", "Uso deshou?!", "Masaka!", "Maji de?!", "Maji ka…",
	"Eeeeh?!", "Ehhh?!", "Hee~", "Hou…", "Naruhodo.", "Naruhodo ne…",
	"Sou ka.", "Sou da ne.", "Sou desu ne.", "Sokka…", "Yosh!", "Yoshi!",
	"Ikuzo!", "Ikimasu!", "Itadakimasu!", "Gochisousama!", "Ohayou!",
	"Konnichiwa!", "Konbanwa!", "Oyasumi~", "Tadaima!", "Okaeri~",
	"Moshi moshi?", "Hai hai.", "Haaai~", "Iya iya iya.", "Iyada!",
	"Dame!", "Dame da!", "Dame desu!", "Zettai dame!", "Chotto matte!",
	"Chotto chotto!", "Matte yo!", "Mou ii!", "Mou~", "Hidoi!", "Hidoi yo!",
	"Kowai…", "Kawaii~!", "Kawaii desu ne~", "Kakkoii!", "Subarashii!",
	"Saikou!", "Saitei.", "Mendokusai…", "Urusai!", "Urusai urusai urusai!",
	"Baka!", "Baka ja nai no?", "Baka mitai.", "Aho ka.", "Hontou ni?",
	"Hontou da yo!", "Hontou ni mou…", "Wakatta.", "Wakatta wakatta.",
	"Wakaranai…", "Wakarimasen!", "Shiranai.", "Shiranai yo!",
	"Shouganai…", "Shikata ga nai.", "Sasuga!", "Sasuga senpai!",
	"Omoshiroi…", "Tsumaranai.", "Itai!", "Itai itai itai!", "Abunai!",
	"Nigero!", "Tasukete!", "Tasukete kudasai!", "Ganbatte!", "Ganbare!",
	"Ganbarimasu!", "Otsukare~", "Otsukaresama deshita.", "Arigatou!",
	"Arigatou gozaimasu!", "Domo.", "Domo arigatou.", "Gomen!",
	"Gomen nasai!", "Gomen ne~", "Sumimasen!", "Shitsurei shimasu.",
	"Onegai!", "Onegai shimasu!", "Daijoubu?", "Daijoubu da yo.",
	"Daijoubu desu!", "Yatta!", "Yatta ne!", "Banzai!", "Kanpai!",
	"Omedetou!", "Kuso!", "Chikushou!", "Shimatta!", "Are?", "Are are?",
	"Etto…", "Ano…", "Ano sa…", "Ano ne!", "Ne ne!", "Nee~", "Oi!",
	"Oi oi oi.", "Hora!", "Hora hora~", "Mite mite!", "Kore wa…",
	"Kore wa kore wa…", "Nandeyanen!", "Doushite…", "Naze da…",
	"Igiari!", "Matta!", "Kurae!", "Hissatsu!", "Henshin!",
	"Omae wa mou shindeiru.", "Keikaku doori…", "Mou ichido!", "Ima da!",
	"Osoi!", "Hayai!", "Hayaku hayaku!", "Tsuyoi…", "Yowai.", "Zannen!",
	"Zannen deshita~", "Fuhahaha!", "Ohohoho~", "Ufufu~", "Ehehe~",
	"Teehee~", "Nya~", "Nyaa!", "Uguu…", "Auu…", "Hawawa…", "Pyon!",
	"Desu wa!", "Da ze!", "Yokatta…", "Yokatta ne!", "Genki?",
	"Genki da yo!", "Hisashiburi!", "Hajimemashite.", "Yoroshiku!",
	"Yoroshiku onegai shimasu!", "Mata ne~", "Ja ne!", "Ja na.",
	"Sayonara~", "Saraba da.", "Bakana…", "Sonna…", "Sonna bakana!",
	"Guh…", "Kuh…", "Tch.", "Fun.", "Hmph.", "Nandato…", "Temee…",
	"Kisama…", "Onore…", "Yurusanai!", "Zettai yurusanai!",
	"Akiramenai!", "Makenai!", "Katsu zo!", "Ore no kachi da!",
	"Omae no make da.", "Mada mada dane.", "Daga kotowaru.",
	"Yume janai…", "Shinjirarenai!",
}

// weebHonorifics are appended to capitalized mid-sentence words (names).
var weebHonorifics = []string{
	"chan", "kun", "san", "sama", "senpai", "sensei", "dono",
	"tan", "chin", "kyun", "han", "shi",
}

// weebParticles are sentence-final particles/copulas appended to messages.
var weebParticles = []string{
	"desu.", "desu~", "desu yo.", "desu ne~", "desu ka?", "da yo!",
	"da yo ne~", "da ze!", "da zo!", "dattebayo!", "nano.", "nano da!",
	"nanodesu.", "no da!", "wa yo!", "wa ne~", "kashira~", "janai ka.",
	"ja nai no?", "mitai na~", "toka?", "kamo.", "kamo ne~", "deshou?",
	"desho~", "yo ne?", "ne?", "ne~", "na no yo!", "nya~", "nyan!",
	"pyon!", "de gozaru.", "de gozaimasu.", "ssu.", "ssu yo!", "aru yo!",
	"zoi!", "keredo…", "demo ne…",
}

// weebRomajiEntryCount is the total number of distinct romaji strings across
// all four corpus tables; exported for the documentation claim ("200-300+
// romaji") and asserted by tests so the corpus can't silently shrink.
func weebRomajiEntryCount() int {
	return len(weebWordTable) + len(weebInterjections) + len(weebHonorifics) + len(weebParticles)
}

func firstRuneOf(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// applyWeeb is the /weeb transform: word swaps, honorifics on names,
// interjections and sentence-final particles. Sugoi, ne~?
func applyWeeb(text string) string {
	words := strings.Fields(text)
	for i, w := range words {
		pre, core, post := splitWordCore(w)
		if core == "" {
			continue
		}
		lower := strings.ToLower(core)
		if rep, ok := weebWordTable[lower]; ok {
			if unicode.IsUpper(firstRuneOf(core)) {
				rep = capitalizeFirst(rep)
			}
			words[i] = pre + rep + post
			continue
		}
		// Capitalized mid-sentence words read as names — gift them an honorific.
		if i > 0 && len([]rune(core)) > 2 && unicode.IsUpper(firstRuneOf(core)) && rand.Intn(5) < 2 {
			words[i] = pre + core + "-" + weebHonorifics[rand.Intn(len(weebHonorifics))] + post
		}
	}
	out := strings.Join(words, " ")
	if rand.Intn(2) == 0 {
		out = strings.TrimSpace(weebInterjections[rand.Intn(len(weebInterjections))] + " " + out)
	}
	if rand.Intn(5) < 3 {
		out = strings.TrimSpace(out + " " + weebParticles[rand.Intn(len(weebParticles))])
	}
	if out == "" {
		out = weebInterjections[rand.Intn(len(weebInterjections))]
	}
	return fitICBudget(out)
}
