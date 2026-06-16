/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: the /medieval and /cheese punishments.

   Both are mod-only (MUTE) text punishments wired into the shared
   cmdPunishment plumbing (-d/-r/-h, comma UID lists, `global`, /stack, DB
   persistence, /unpunish -t) exactly like the other text effects.

   /medieval — rewrites the target's IC text into Olde-English / medieval
   speak: word-for-word swaps (you→thou, your→thy, is→be, yes→aye…), a
   prepended herald's cry ("Hark!", "Forsooth,", "Prithee,"…) and an
   appended courtly flourish ("…by my troth.", "…mine liege.", "…verily.").
   Because the prefix and suffix are each drawn at random and layered on top
   of the per-word swaps, a single short line already has hundreds of
   possible renderings — far more than 100 distinct combinations.

   /cheese — discards the target's text entirely and replaces it with one of
   100+ "statements about cheese" (the running joke that birthed this
   command: "So your text gets replaced with different statements about
   cheese?" "…sure."). It behaves like the other themed-quote replacers
   (/recipe, /rickroll, …): every message is a fresh cheese proclamation. */

package athena

import (
	"math/rand"
	"strings"
	"unicode"
)

// ── /medieval ───────────────────────────────────────────────────────────────

// medievalWordTable maps lowercase modern words to their medieval equivalents.
var medievalWordTable = map[string]string{
	// Pronouns & address
	"you": "thou", "your": "thy", "yours": "thine", "you're": "thou art",
	"yourself": "thyself", "u": "thee", "ya": "thee", "y'all": "all ye",
	"i": "I", "me": "mine own self", "my": "mine", "mine": "mine own",
	"we": "we", "us": "us", "he": "he", "she": "she", "they": "they",
	// Verbs / copula
	"are": "art", "is": "be", "am": "be", "was": "wert", "were": "wert",
	"have": "hath", "has": "hath", "do": "doth", "does": "doth",
	"will": "shall", "won't": "shall not", "can't": "cannot", "cannot": "cannot",
	"don't": "do not", "doesn't": "doth not", "isn't": "be not",
	"says": "sayeth", "say": "sayest", "said": "spake", "speak": "speaketh",
	"think": "ponder", "thinks": "pondereth", "know": "knoweth", "knows": "knoweth",
	"go": "goeth", "goes": "goeth", "come": "cometh", "comes": "cometh",
	"want": "desireth", "wants": "desireth", "need": "requireth",
	"see": "behold", "look": "lo", "give": "bequeath", "take": "seize",
	"fight": "do battle", "run": "flee", "help": "aid", "die": "perish",
	// Greetings / responses
	"hello": "well met", "hi": "hail", "hey": "hark", "yo": "hail",
	"goodbye": "fare thee well", "bye": "farewell", "thanks": "much obliged",
	"thank": "thank", "yes": "aye", "yeah": "aye", "yep": "verily", "ok": "so be it",
	"okay": "so be it", "no": "nay", "nope": "nay", "please": "prithee",
	"sorry": "forgive me", "welcome": "well come", "wow": "zounds",
	// Nouns / people
	"friend": "good sir", "friends": "good gentles", "man": "knave",
	"woman": "maiden", "guy": "fellow", "guys": "good gentles", "dude": "squire",
	"people": "the populace", "everyone": "all ye gathered", "king": "liege",
	"queen": "her majesty", "house": "manor", "home": "keep", "city": "burgh",
	"money": "gold coin", "food": "victuals", "drink": "ale", "beer": "mead",
	"sword": "blade", "war": "the great war", "battle": "skirmish",
	"job": "noble duty", "work": "labour", "boss": "lord", "police": "the guard",
	"prison": "the dungeon", "court": "the royal court", "lawyer": "barrister",
	"judge": "magistrate", "phone": "messenger bird", "car": "horse",
	"computer": "thinking-engine", "internet": "the great web of scrolls",
	// Adjectives
	"good": "most goodly", "great": "splendid", "bad": "most foul",
	"awesome": "wondrous", "cool": "right fashionable", "amazing": "marvellous",
	"stupid": "addle-pated", "crazy": "bedlam-mad", "angry": "wroth",
	"happy": "merry", "sad": "forlorn", "scared": "affrighted", "tired": "wearied",
	"big": "most great", "small": "wee", "fast": "swift", "slow": "slothful",
	"old": "ancient", "new": "newfangled", "beautiful": "comely", "ugly": "loathly",
	"true": "verily true", "false": "a falsehood", "very": "most",
	"really": "verily", "now": "anon", "soon": "ere long", "today": "this day",
	"tomorrow": "on the morrow", "always": "evermore", "never": "ne'er",
	"here": "hither", "there": "thither", "where": "whither", "because": "for",
	"about": "concerning", "before": "ere", "around": "about",
}

// medievalHeralds are heralds' cries occasionally prepended to a message.
var medievalHeralds = []string{
	"Hark!", "Hark, good gentles!", "Forsooth,", "Forsooth!", "Verily,",
	"Verily I say,", "Prithee,", "Prithee, hark,", "Anon,", "Hear ye, hear ye!",
	"Lo!", "Lo and behold,", "Zounds!", "Zounds and gadzooks!", "Egad!",
	"Gadzooks!", "By my troth,", "By the rood,", "Methinks", "Methinks that",
	"Mark me well,", "Attend me,", "I prithee,", "Good morrow!", "God save the King!",
	"Huzzah!", "Alas,", "Alack,", "Alack and alas,", "Hold!", "Stay thy hand!",
	"Avast,", "Hither, knave,", "By Saint George,", "Pray tell,", "Beshrew me,",
	"Fie!", "Fie upon it,",
}

// medievalFlourishes are courtly flourishes occasionally appended to a message.
var medievalFlourishes = []string{
	"by my troth.", "mine liege.", "good sir.", "I do declare.", "verily.",
	"forsooth.", "I dare say.", "m'lord.", "m'lady.", "as God is my witness.",
	"upon mine honour.", "I beseech thee.", "and no mistake.", "in sooth.",
	"by the saints.", "as the bards do sing.", "if it please the court.",
	"so help me.", "and that is the truth of it.", "by my faith.",
	"ere the morrow.", "as is right and proper.", "huzzah!", "amen.",
	"God willing.", "as my father taught me.", "thus it is written.",
	"and well thou knowest it.", "perchance.", "in days of yore.",
}

// medievalVariationCount is a conservative lower bound on the number of
// distinct renderings the transform can produce, asserted by tests so the
// documented "100+ combinations" claim can't silently regress. Even ignoring
// the per-word swaps, herald × flourish alone vastly exceeds 100.
func medievalVariationCount() int {
	return len(medievalHeralds) * len(medievalFlourishes)
}

// applyMedieval rewrites text into medieval speak: per-word swaps, a random
// herald prefix, and a random courtly flourish suffix.
func applyMedieval(text string) string {
	words := strings.Fields(text)
	for i, w := range words {
		pre, core, post := splitWordCore(w)
		if core == "" {
			continue
		}
		if rep, ok := medievalWordTable[strings.ToLower(core)]; ok {
			if unicode.IsUpper(firstRuneOf(core)) {
				rep = capitalizeFirst(rep)
			}
			words[i] = pre + rep + post
		}
	}
	out := strings.Join(words, " ")
	if rand.Intn(3) < 2 { // ~2/3 of messages gain a herald's cry
		out = strings.TrimSpace(medievalHeralds[rand.Intn(len(medievalHeralds))] + " " + out)
	}
	if rand.Intn(3) < 2 { // ~2/3 gain a courtly flourish
		out = strings.TrimRight(strings.TrimSpace(out), ".!?,") + ", " + medievalFlourishes[rand.Intn(len(medievalFlourishes))]
	}
	if strings.TrimSpace(out) == "" {
		out = medievalHeralds[rand.Intn(len(medievalHeralds))]
	}
	return fitICBudget(out)
}

// ── /cheese ──────────────────────────────────────────────────────────────────

// cheeseStatements is the pool of "statements about cheese" /cheese replaces
// every message with. 100+ distinct lines so a punished player rambling about
// cheese never repeats themselves for a long while. The "cheese is a sauce"
// running gag (the conversation that spawned this command) is honoured.
var cheeseStatements = []string{
	"Cheese is, technically, a sauce.",
	"Cheese is normally a sauce.",
	"Different statements about cheese being a sauce.",
	"I like cheese.",
	"Cheese is a sauce. I will not be taking questions.",
	"Did you know cheese is just aggressive milk?",
	"Cheddar is the most sold cheese in the world.",
	"There are over 1,800 distinct varieties of cheese.",
	"Mozzarella was traditionally made from water buffalo milk.",
	"Parmigiano-Reggiano is aged for at least 12 months.",
	"Cheese was being made over 7,000 years ago.",
	"Brie is known as 'the Queen of Cheeses'.",
	"The holes in Swiss cheese are called 'eyes'.",
	"A cheese without eyes is called 'blind'.",
	"Casu marzu is a Sardinian cheese that contains live maggots.",
	"Fear of cheese is called turophobia.",
	"A lover of cheese is called a turophile.",
	"Gorgonzola is one of the world's oldest blue cheeses.",
	"Feta must be made in Greece to be called feta.",
	"Roquefort is aged in the Combalou caves of southern France.",
	"Cheese is heavier than the argument you were about to make.",
	"Halloumi squeaks when you bite it. As do I, now.",
	"The Pilgrims brought cheese on the Mayflower.",
	"Pule, made from donkey milk, is the most expensive cheese in the world.",
	"Stilton can only be made in three English counties.",
	"Emmental is the cheese with the iconic big holes.",
	"Mascarpone is the cheese that holds tiramisu together.",
	"Ricotta means 'recooked' in Italian.",
	"Provolone comes in two kinds: dolce and piccante.",
	"Cottage cheese got its name from being made in cottages.",
	"Gouda is named after a city in the Netherlands.",
	"Edam famously comes coated in red wax.",
	"Camembert was supposedly invented in 1791 in Normandy.",
	"Manchego is made from the milk of Manchega sheep.",
	"Cheese curds squeak because of their tightly woven protein strands.",
	"Limburger cheese is famous for its powerful smell.",
	"Some say the moon is made of cheese. It is not. Probably.",
	"Cheese rolling down a hill is an actual competitive sport.",
	"At Cooper's Hill, people chase a wheel of cheese down a slope.",
	"The average person eats their body weight in cheese over a few years.",
	"Cream cheese was invented in America in 1872.",
	"Burrata is mozzarella with a creamy surprise inside.",
	"Paneer is a cheese that doesn't melt.",
	"Queso fresco crumbles instead of melting.",
	"Cheese can be made from cow, goat, sheep, buffalo, even camel milk.",
	"Velveeta is legally a 'pasteurized recipe cheese product'.",
	"American cheese is a 'processed cheese', not strictly a cheese.",
	"Rennet, an enzyme, is what curdles milk into cheese.",
	"Vegetarian cheeses use microbial rennet instead of animal rennet.",
	"The rind of some cheeses is perfectly edible.",
	"Wax-coated cheeses keep for a very long time.",
	"Aged cheese has less lactose than fresh cheese.",
	"Cheese contains tyramine, which is why some people get headaches.",
	"Blue cheese gets its veins from Penicillium mould.",
	"The mould in blue cheese is completely safe to eat.",
	"Wisconsin is nicknamed 'America's Dairyland' for its cheese.",
	"Green Bay fans are proudly called 'Cheeseheads'.",
	"A 'cheesemonger' is a person who sells cheese.",
	"Affineurs are the experts who age cheese to perfection.",
	"Cheese fondue comes from the French word for 'melted'.",
	"Raclette is both a cheese and the dish of melting it.",
	"Welsh rarebit is essentially fancy cheese on toast.",
	"Cacio e pepe is just cheese, pepper, and pasta water.",
	"Cheese was once used as currency in parts of history.",
	"Roman soldiers received cheese as part of their rations.",
	"There is a cheese aged in caves to develop its flavour.",
	"Cheese can be smoked over wood for extra flavour.",
	"Some cheeses are washed in brine, beer, or even wine.",
	"Tomme is a family of cheeses from the French Alps.",
	"Gruyère is the secret to a great French onion soup.",
	"Fontina is the melting cheese of choice in fonduta.",
	"Pecorino is made from sheep's milk — 'pecora' means sheep.",
	"Asiago can be eaten young and fresh or aged and sharp.",
	"Havarti is a buttery Danish cheese.",
	"Jarlsberg is a Norwegian cheese with a mild nutty taste.",
	"Wensleydale was famously beloved by a certain claymation inventor.",
	"Red Leicester gets its colour from annatto, a natural dye.",
	"Double Gloucester is the cheese they roll down the hill.",
	"Cheese strings are just mozzarella that learned a party trick.",
	"String cheese peels because of how its proteins align.",
	"A grilled cheese is a sandwich; nacho cheese is a sauce.",
	"Queso is literally just the Spanish word for cheese.",
	"Cheese sauce is what happens when cheese achieves its true sauce form.",
	"Nacho cheese proves, once again, that cheese is a sauce.",
	"Macaroni and cheese is pasta swimming in cheese sauce.",
	"A cheese pull is the most dramatic thing food can do.",
	"Stretchy cheese is the sign of a good melt.",
	"Some cheeses are aged for years before they're ready.",
	"The oldest cheese ever found was in an Egyptian tomb.",
	"Cheese has been found in 3,200-year-old tombs.",
	"Curds and whey are the two products of curdled milk.",
	"Little Miss Muffet was, in fact, eating cheese curds.",
	"Whey left over from cheese is used to make ricotta.",
	"A wheel of Parmesan can weigh up to 40 kilograms.",
	"Banks in Italy accept wheels of Parmesan as loan collateral.",
	"Yes — actual banks store cheese in vaults.",
	"A cheese vault is a real and glorious place.",
	"Cheese improves the flavour of nearly everything.",
	"The plural of 'cheese' is 'cheeses'. Behold my wisdom.",
	"Say 'cheese' and you smile — that is its true power.",
	"Cheese: it brings people together. Mostly to a fondue pot.",
	"A cheese platter is a complete personality.",
	"Goat cheese is tangier than cow cheese.",
	"Chèvre simply means goat cheese in French.",
	"Cheese is the dairy aisle's greatest achievement.",
	"Frankly, every meal could use more cheese.",
	"Cheese is proof that milk wanted to become something greater.",
	"In conclusion: cheese is a sauce. Thank you for coming.",
}

// cheeseLineCount exposes the pool size so tests can pin the documented
// "100+ statements" claim.
func cheeseLineCount() int { return len(cheeseStatements) }

// applyCheese discards the input and returns a random cheese statement.
func applyCheese(_ string) string {
	return fitICBudget(cheeseStatements[rand.Intn(len(cheeseStatements))])
}
