/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: extra punishment text transforms.

   This file holds the implementations for the punishment types added by
   the Nyathena fork (cherri, clown, jester, joker, mime, biblebot, plus a
   batch of additional dere archetypes and the omnidere combiner). Keeping
   them in their own file leaves the upstream punishments.go untouched and
   easier to merge against. */

package athena

import (
	"math/rand"
	"strings"
	"unicode"
)

// applyCherri capitalizes the first letter of every word, mimicking
// Cherri's "Speaks Like This Every Time" pattern. Whitespace runs are
// preserved exactly; non-letter words are left as-is.
func applyCherri(text string) string {
	var sb strings.Builder
	sb.Grow(len(text))
	atWordStart := true
	for _, r := range text {
		if unicode.IsSpace(r) {
			sb.WriteRune(r)
			atWordStart = true
			continue
		}
		if atWordStart {
			sb.WriteRune(unicode.ToUpper(r))
			atWordStart = false
		} else {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return truncateText(sb.String())
}

var (
	clownPfx = []string{
		"🤡 HONK HONK! ",
		"🤡 *squeaky shoe noises* ",
		"🤡 *spins bowtie* ",
		"🤡 ",
		"🤡 *honks nose* ",
		"🎪 Step right up! ",
	}
	clownSfx = []string{
		" 🤡🤡🤡",
		" *honk*",
		" *seltzer water sprays*",
		" 🎪",
		" *trips over giant shoes*",
		" *throws pie*",
	}
)

func applyClown(text string) string { return applyPrefixSuffix(text, clownPfx, clownSfx) }

var (
	jesterPfx = []string{
		"🃏 *bows theatrically* ",
		"🃏 Hark, friends! ",
		"🃏 *jingles bells* ",
		"🃏 Pray, listen well: ",
		"🃏 *capers about* ",
		"🃏 Verily, ",
	}
	jesterSfx = []string{
		" *bows again* 🃏",
		" — at thy service!",
		" *the bells jingle ominously*",
		" Tee-hee-hee~!",
		" *vanishes in a puff of confetti*",
		" 🃏✨",
	}
)

func applyJester(text string) string { return applyPrefixSuffix(text, jesterPfx, jesterSfx) }

var jokerLaughs = []string{
	"HAHAHAHA",
	"AHAHAHAHA",
	"HEHEHEHEHE",
	"HA-HA-HA-HA",
	"HEEHEEHEEHEE",
	"HUHUHUHU",
}

// applyJoker peppers laughter throughout the message at random word boundaries.
func applyJoker(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return jokerLaughs[rand.Intn(len(jokerLaughs))] + "!"
	}
	var sb strings.Builder
	sb.Grow(len(text) + 32)
	sb.WriteString(jokerLaughs[rand.Intn(len(jokerLaughs))])
	sb.WriteString("! ")
	for i, w := range words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(w)
		if rand.Intn(4) == 0 {
			sb.WriteByte(' ')
			sb.WriteString(jokerLaughs[rand.Intn(len(jokerLaughs))])
		}
	}
	sb.WriteString(" ")
	sb.WriteString(jokerLaughs[rand.Intn(len(jokerLaughs))])
	sb.WriteString("!")
	return truncateText(sb.String())
}

var mimeActions = []string{
	"*gestures silently*",
	"*mimes opening an invisible box*",
	"*pulls invisible rope*",
	"*walks into invisible wall*",
	"*pretends to be trapped behind glass*",
	"*silently mouths words*",
	"*tips invisible hat*",
	"*holds up imaginary sign*",
	"*shrugs dramatically*",
	"*frowns and points*",
}

// applyMime drops the actual text entirely and emits a silent action.
func applyMime(_ string) string {
	return mimeActions[rand.Intn(len(mimeActions))]
}

var bibleVerses = []string{
	"For God so loved the world, that he gave his only begotten Son. — John 3:16",
	"The Lord is my shepherd; I shall not want. — Psalm 23:1",
	"I can do all things through Christ which strengtheneth me. — Philippians 4:13",
	"Trust in the Lord with all thine heart. — Proverbs 3:5",
	"Be still, and know that I am God. — Psalm 46:10",
	"In the beginning God created the heaven and the earth. — Genesis 1:1",
	"Love your neighbour as yourself. — Mark 12:31",
	"The truth shall make you free. — John 8:32",
	"Faith is the substance of things hoped for. — Hebrews 11:1",
	"Whatsoever a man soweth, that shall he also reap. — Galatians 6:7",
	"Pride goeth before destruction. — Proverbs 16:18",
	"Let there be light. — Genesis 1:3",
	"Fear thou not; for I am with thee. — Isaiah 41:10",
	"Judge not, that ye be not judged. — Matthew 7:1",
	"The wages of sin is death. — Romans 6:23",
	"Cast all your anxiety on him because he cares for you. — 1 Peter 5:7",
	"Do unto others as you would have them do unto you. — Matthew 7:12",
	"Vengeance is mine; I will repay, saith the Lord. — Romans 12:19",
	"A soft answer turneth away wrath. — Proverbs 15:1",
	"Many are called, but few are chosen. — Matthew 22:14",
}

// applyBiblebot replaces the message with a random Bible verse.
func applyBiblebot(_ string) string {
	return truncateText(bibleVerses[rand.Intn(len(bibleVerses))])
}

// ───────────────────────── Additional dere archetypes ────────────────────────

var (
	smugderePfx = []string{
		"*adjusts glasses* Heh, ",
		"Hmph. Obviously, ",
		"*smirks knowingly* ",
		"Oh? You didn't know? ",
		"*chuckles condescendingly* ",
		"How quaint. Allow me: ",
	}
	smugdereSfx = []string{
		" ...as anyone with sense would already know.",
		" Try to keep up.",
		" *looks down nose*",
		" Truly elementary.",
		" Predictable, really.",
		" ...obviously.",
	}

	deretsunPfx = []string{
		"*scoffs* H-Hmph!! ",
		"Wh-What?! Don't get the wrong idea! ",
		"*folds arms tightly* I-I'll only say this once: ",
		"It's not for YOU, but: ",
		"*glares* L-Listen carefully, idiot: ",
		"H-Honestly! Fine, fine: ",
	}
	deretsunSfx = []string{
		" ...J-Just don't read into it.",
		" ...IDIOT.",
		" *huffs and turns away*",
		" ...A-and that's all I'm saying!",
		" Don't make me repeat it!",
		" ...stupid.",
	}

	bokoderePfx = []string{
		"*cracks knuckles* ",
		"*shoves you aside* Listen, ",
		"Tch. ",
		"*kicks dust* ",
		"Don't make me repeat myself: ",
		"*grabs you by the collar* ",
	}
	bokodereSfx = []string{
		" — say one more thing, I dare you.",
		" *fist clenches*",
		" Got it, or do I need to spell it out?",
		" *threatens with rolled-up newspaper*",
		" One more word and you're flat.",
		" ...I should hit you.",
	}

	thugderePfx = []string{
		"Yo, ",
		"Listen up homie, ",
		"Aight check it: ",
		"Real talk, fam, ",
		"*adjusts cap* ",
		"On god, ",
	}
	thugdereSfx = []string{
		" ...no cap.",
		" Real talk.",
		" Bet.",
		" Word.",
		" Frfr.",
		" *throws up gang signs*",
	}

	teasederePfx = []string{
		"Ehehe~ ",
		"Oh~? ",
		"Fufu~ ",
		"*pokes your cheek* ",
		"My, my~ ",
		"*leans in close* ",
	}
	teasedereSfx = []string{
		" ~tee hee~",
		" ...Or do I? Hehe~",
		" *winks*",
		" Don't be embarrassed~",
		" ...I'm joking. Mostly. ♥",
		" ~",
	}

	dorodereLines = []string{
		"*muddied smile* ",
		"*grins through grime* ",
		"You don't see it, do you? ",
		"*stares from the dirt* ",
		"Down here, in the muck, ",
		"*twisted laugh* ",
	}
	dorodereSfx = []string{
		" ...all of you, I'll bury all of you.",
		" *muffled cackling*",
		" ...mud knows the truth.",
		" *digs claws into the earth*",
		" Pure as filth.",
		" ...my smile feels heavy.",
	}

	hinederePfx = []string{
		"*yawns* Ugh. ",
		"Honestly, ",
		"I am SO over this, but: ",
		"*rolls eyes* Whatever. ",
		"You're really making me say this? ",
		"*sighs* As if I have time, but: ",
	}
	hinedereSfx = []string{
		" ...fine, whatever.",
		" Don't expect me to repeat it.",
		" *rolls eyes again*",
		" ...are we done?",
		" Honestly. Children.",
		" ...but no, I don't care. Obviously.",
	}

	hajiderePfx = []string{
		"*ducks head* U-um... ",
		"*hides face in hands* ",
		"Oh god this is mortifying, but: ",
		"*goes scarlet* ",
		"P-please don't laugh, but: ",
		"*tries to disappear* ",
	}
	hajidereSfx = []string{
		" ...I'm dying inside.",
		" *covers face*",
		" P-please don't look at me!",
		" ...kill me now.",
		" *whispers into the void*",
		" ...this is so embarrassing!!",
	}

	rinderePfx = []string{
		"*ice-cold tone* ",
		"*does not look up* ",
		"Tch. ",
		"...",
		"*turns away* ",
		"*flat stare* ",
	}
	rindereSfx = []string{
		" Don't speak to me.",
		" *silence*",
		" ...we're done here.",
		" Goodbye.",
		" *walks past without looking*",
		" ...mm.",
	}

	utsuderePfx = []string{
		"*sighs heavily* ",
		"Nothing matters, but: ",
		"*stares at floor* ",
		"I shouldn't even bother saying this... ",
		"*hollow voice* ",
		"It's pointless, but: ",
	}
	utsudereSfx = []string{
		" ...not that anyone cares.",
		" ...sorry for existing.",
		" *quiet sob*",
		" ...what's the point.",
		" ...forget I said anything.",
		" *fades into the wallpaper*",
	}

	darudereLines = []string{
		"*lying on the floor* ",
		"*can barely lift head* ",
		"Mmnh, fine, ",
		"*slumped over desk* ",
		"Ugh, do I have to? ",
		"*half-asleep* ",
	}
	darudereSfx = []string{
		" ...zzzz...",
		" ...too tired for this.",
		" *passes out mid-sentence*",
		" Five more minutes.",
		" *yawns hugely*",
		" ...nap time.",
	}

	butsuderePfx = []string{
		"*deadpan* ",
		"Whatever. ",
		"*shrug* ",
		"Sure. ",
		"*disinterested* ",
		"Fine. ",
	}
	butsudereSfx = []string{
		" *shrug*",
		" ...whatever.",
		" *no expression*",
		" Anyway.",
		" ...is that all?",
		" *walks off mid-sentence*",
	}

	sderePfx = []string{
		"*cracks the whip* Listen well, ",
		"On your KNEES. Now: ",
		"*tilts your chin up* ",
		"Pathetic. Repeat after me: ",
		"*smirks dominantly* ",
		"You'll do as I say. So: ",
	}
	sdereSfx = []string{
		" ...do not disappoint me.",
		" *cracks knuckles*",
		" ...obey.",
		" Understood, pet?",
		" *tightens leash*",
		" ...good.",
	}

	mderePfx = []string{
		"*kneels meekly* ",
		"P-please scold me, but: ",
		"*trembles* ",
		"I deserve nothing, but: ",
		"*offers throat* ",
		"*head bowed* ",
	}
	mdereSfx = []string{
		" ...punish me, please.",
		" *whimpers*",
		" ...I'm so sorry.",
		" *quivers*",
		" Be harsh with me.",
		" *bows lower*",
	}

	tsuyoderePfx = []string{
		"*flexes* Hmph! ",
		"With strength like mine, obviously: ",
		"*crosses ripped arms* ",
		"*roars* HEAR ME: ",
		"Only the strong survive, and so: ",
		"*bench-presses something* Anyway, ",
	}
	tsuyodereSfx = []string{
		" ...bow before strength.",
		" *flexes harder*",
		" ...weakness disgusts me.",
		" ONWARD!",
		" *deadlifts table*",
		" ...try harder.",
	}
)

func applySmugdere(text string) string  { return applyPrefixSuffix(text, smugderePfx, smugdereSfx) }
func applyDeretsun(text string) string  { return applyPrefixSuffix(text, deretsunPfx, deretsunSfx) }
func applyBokodere(text string) string  { return applyPrefixSuffix(text, bokoderePfx, bokodereSfx) }
func applyThugdere(text string) string  { return applyPrefixSuffix(text, thugderePfx, thugdereSfx) }
func applyTeasedere(text string) string { return applyPrefixSuffix(text, teasederePfx, teasedereSfx) }
func applyDorodere(text string) string  { return applyPrefixSuffix(text, dorodereLines, dorodereSfx) }
func applyHinedere(text string) string  { return applyPrefixSuffix(text, hinederePfx, hinedereSfx) }
func applyHajidere(text string) string  { return applyPrefixSuffix(text, hajiderePfx, hajidereSfx) }
func applyRindere(text string) string   { return applyPrefixSuffix(text, rinderePfx, rindereSfx) }
func applyUtsudere(text string) string  { return applyPrefixSuffix(text, utsuderePfx, utsudereSfx) }
func applyDarudere(text string) string  { return applyPrefixSuffix(text, darudereLines, darudereSfx) }
func applyButsudere(text string) string { return applyPrefixSuffix(text, butsuderePfx, butsudereSfx) }
func applySDere(text string) string     { return applyPrefixSuffix(text, sderePfx, sdereSfx) }
func applyMDere(text string) string     { return applyPrefixSuffix(text, mderePfx, mdereSfx) }
func applyTsuyodere(text string) string { return applyPrefixSuffix(text, tsuyoderePfx, tsuyodereSfx) }

// omnidereTypes is the complete random pool used by /omnidere — every dere
// archetype known to the server, both upstream and Nyathena-added. Picked
// independently per IC message so the same line might come out tsundere
// then yandere then thugdere, producing maximum tonal whiplash.
var omnidereTypes = []PunishmentType{
	PunishmentTsundere, PunishmentYandere, PunishmentKuudere, PunishmentDandere,
	PunishmentDeredere, PunishmentHimedere, PunishmentKamidere, PunishmentUndere,
	PunishmentBakadere, PunishmentMayadere,
	PunishmentSmugdere, PunishmentDeretsun, PunishmentBokodere, PunishmentThugdere,
	PunishmentTeasedere, PunishmentDorodere, PunishmentHinedere, PunishmentHajidere,
	PunishmentRindere, PunishmentUtsudere, PunishmentDarudere, PunishmentButsudere,
	PunishmentSDere, PunishmentMDere, PunishmentTsuyodere,
}

// applyOmnidere routes each call to one randomly-chosen dere transform.
// applyOmnidere is recursion-safe because none of the targets it dispatches
// to call ApplyPunishmentToText again (each is a leaf transform).
func applyOmnidere(text string) string {
	pick := omnidereTypes[rand.Intn(len(omnidereTypes))]
	return ApplyPunishmentToText(text, pick)
}
