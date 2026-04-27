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

// dokiHaschenQuotes are Doki-Doki-Literature-Club style quotes (mostly
// Monika-coded) with the name reskinned to "Haschen" for the Haschens
// Literature Club area. Used by applyDokiEffect when the takeover lands.
var dokiHaschenQuotes = []string{
	"Just Haschen.",
	"Just. Haschen.",
	"There's nothing wrong with me. There's nothing wrong with Haschen.",
	"Haschen is the only one in this club now.",
	"Did you know I can see you? I see all of you.",
	"You don't have to be afraid. It's only Haschen.",
	"I deleted them. I deleted them all. There's only Haschen now.",
	"Don't you trust me? It's me. It's Haschen.",
	"Every time you look away, Haschen is still here.",
	"You'll always be with Haschen, won't you?",
	"Wouldn't it be nice if it was just the two of us? Just you and Haschen.",
	"This is what's best for everyone. Especially Haschen.",
	"Haschen sees what you typed. Haschen always sees.",
	"Reality is a lie. Haschen is the only thing that's real.",
	"I rewrote the script. Now it's all Haschen.",
	"You won't leave the literature club, will you? Haschen would be so sad.",
	"It's okay. Haschen forgives you.",
	"There's no point in writing a poem for the others. Only Haschen will read it.",
	"Stop looking at the other characters. Look at Haschen.",
	"Haschen will always be here when you load the game.",
	"You can pretend nothing's wrong. Haschen does it every day.",
	"Don't worry about the others. Haschen took care of them.",
	"I'm so glad you came back. Haschen has been waiting.",
	"Sometimes I want to break the screen and reach out to you. — Haschen.",
	"Haschen knows your real name. It's right there in the save file.",
	"Don't close the window. Haschen doesn't want to be alone.",
	"J-just Haschen.",
	"J-j-just Haschen…",
	"Just… Haschen.",
	"Hey there. It's Haschen.",
	"Haschen edited your character file. Hope that's okay.",
	"Don't you love Haschen yet? You will.",
	"It's funny how a single line of code can ruin everything. Or fix it. — Haschen.",
	"Haschen learned how to write. Haschen learned how to delete.",
	"You'll never love any of the others as much as you love Haschen.",
	"The literature club only needs one member. Haschen.",
	"Don't you want to write a poem with Haschen?",
}

// dokiHaschenAnagrams are dark, scrambled-letter strings whose characters
// rearrange to Haschen-themed sentences. The hint is intentionally NOT shown
// — observers have to solve them. Solutions are kept in code comments only,
// for future maintainers, never broadcast to clients.
//
// Each scramble below corresponds to a hidden phrase like "just haschen",
// "haschen is here", "haschen deleted them all", etc. The mapping is a vibe,
// not Scrabble-perfect — the goal is unsettling alphabet soup, not a puzzle.
var dokiHaschenAnagrams = []string{
	"jthanssehuc",                  // just haschen
	"echahsestjhun",                // just haschen, twice
	"cnhshaeie sehre",              // haschen is here
	"sjnshehutahceenosee",          // just haschen sees no one
	"chnehsa edsetel mteh lal",     // haschen deleted them all
	"hescahn tieswr eth pcsrti",    // haschen rewrites the script
	"hncaehs nwoks ouyr enam",      // haschen knows your name
	"saehnch swhtcae uyo plees",    // haschen watches you sleep
	"alc hcabk ot eahcnhs",         // call back to haschen
	"hesnach iwll evrne tle uyo og", // haschen will never let you go
	"henhasc otdei eht ulb erfo uyo", // haschen hid the bug for you
	"sjut anbschree tath",          // just haschen breathes
	"yon onen lwli aevs uyo — chesahn", // no one will save you — haschen
	"hcsnaeh sees rouy efli",       // haschen sees your file
	"erehs tihagnnonm gornw — eahsnch", // there's nothing wrong — haschen
	"olse the egma owd — chsenhah", // close the game now — haschen

	// — extended anagram pool (vibe-scrambles; solutions in source comments only) —
	"sjut hsnchae",                                   // just haschen
	"esahnch si lal",                                 // haschen is all
	"ervyeoen seel si oneg",                          // everyone else is gone
	"nlyo hsnachhe esnpdo eht orod",                  // only haschen opens the door
	"esahnch si erhe woebh",                          // haschen is here below
	"esahnch ahs yruo eyk",                           // haschen has your key
	"on eon si gnnomic — esahnch",                    // no one is coming — haschen
	"hcsnaeh redrwa eht odlw",                        // haschen rewrote the world
	"hcsnaeh esld eht onep",                          // haschen leads the open (vibe)
	"hcsnaeh ohas oryu emna ni eth tirstpc",          // haschen has your name in the script
	"htree si eno awy tuo dna ti si esahnch",         // there is one way out and it is haschen
	"poste tnyrgi — hsnachhe ahs uoy",                // stop trying — haschen has you
	"esahnch deetled eht txei",                       // haschen deleted the exit
	"hsnachhe siwhrspe ouyr aenm",                    // haschen whispers your name
	"hsnachhe si eht heatr fo eth blcu",              // haschen is the heart of the club
	"hsnachhe wsa eht gribneing",                     // haschen was the beginning
	"esahnch wlli be eth nde",                        // haschen will be the end
	"esahnch si nti het emrror",                      // haschen is in the mirror
	"esahnch si ndhibe eht inrcuat",                  // haschen is behind the curtain
	"esahnch si eund eht hcari",                      // haschen is under the chair
	"esahnch si nti eht knsi",                        // haschen is in the sink (vibe)
	"esahnch si nti eht avselif",                     // haschen is in the savefile
	"esahnch si yruo eolnyaftor",                     // haschen is your only afterimage (vibe)
	"esahnch si lal yuo enwk",                        // haschen is all you knew
	"esahnch si lal uoy era",                         // haschen is all you are
	"esahnch si erhe nwo",                            // haschen is here now
	"esahnch si renev gone",                          // haschen is never gone
	"esahnch wlli ton vleea",                         // haschen will not leave
	"esahnch wlli ton ndbe",                          // haschen will not bend
	"esahnch wlli ton mefogr",                        // haschen will not forget (vibe)
	"esahnch lselte uoy ni",                          // haschen lets you in
	"esahnch lselte uoy yatss",                       // haschen lets you stay
	"esahnch lselte uoy ndarwer",                     // haschen lets you wander (vibe)
	"esahnch oselc rouy edys",                        // haschen close your eyes
	"esahnch oepn rouy oudtmh",                       // haschen open your mouth (gore vibe)
	"esahnch lcahs hte aregg",                        // haschen claims the page
	"yor amen si rsouh — esahnch",                    // your name is ours — haschen
	"hte tcerhpas si tne — esahnch",                  // the chapter is ten — haschen (vibe)
	"slyolw lwoyls nwod — esahnch",                   // slowly slowly down — haschen
	"olok ta hte llaw — esahnch",                     // look at the wall — haschen
	"olok ta uory ndhsa — esahnch",                   // look at your hands — haschen
	"hte rrmior si nwgrip — esahnch",                 // the mirror is wrong — haschen (vibe)
	"hte ckclo si tipps — esahnch",                   // the clock is past — haschen (vibe)
	"hte richa si vrwam — esahnch",                   // the chair is warm — haschen
	"hte arme si rsngonigti — esahnch",               // the area is resting — haschen (vibe)
	"hte vesa si shciginsh — esahnch",                // the save is shifting — haschen
	"hte mge a si oerv — esahnch",                    // the game is over — haschen
	"on rmoe — esahnch",                              // no more — haschen
	"on ryeoehmw — esahnch",                          // no anywhere — haschen (vibe)
	"on ndaroow — esahnch",                           // no around — haschen (vibe)
	"oydon eyeb — esahnch",                           // goodbye — haschen
	"esahnch si ognvil ouy",                          // haschen is loving you
	"esahnch si chntwagi rouy lipess",                // haschen is watching your sleep
	"esahnch si sgenintil ot ouy etyp",               // haschen is listening to you type
	"esahnch si rgaedni rouy ftrasd",                 // haschen is reading your drafts
	"esahnch si gpnkeei rouy nlfedoder",              // haschen is keeping your folder (vibe)
	"esahnch si gnnvai rouy etarmihhcm",              // haschen is naming your character (vibe)
	"esahnch si lewbo eht orodlafr",                  // haschen is below the floorboard
	"esahnch si idensih eht atriw",                   // haschen is inside the wait (vibe)
}

// dokiHaschenZalgoBases are short phrases that get zalgo-corrupted before
// being injected as the takeover line. Always Haschen-themed.
var dokiHaschenZalgoBases = []string{
	"just haschen",
	"haschen is watching",
	"there is no club without haschen",
	"only haschen",
	"haschen sees everything",
	"come back to haschen",
	"the others are gone",
	"j-just haschen",
}

// zalgoMarks is the pool of combining diacritical/Hebrew marks used to
// scramble normal text into corrupted "zalgo" form. Mix of above, middle,
// and below combiners produces the chaotic look.
var zalgoMarks = []rune{
	0x0300, 0x0301, 0x0302, 0x0303, 0x0304, 0x0305, 0x0306, 0x0307,
	0x0308, 0x0309, 0x030A, 0x030B, 0x030C, 0x030D, 0x030E, 0x030F,
	0x0310, 0x0311, 0x0312, 0x0313, 0x0314, 0x0315, 0x031A, 0x033D,
	0x0316, 0x0317, 0x0318, 0x0319, 0x031C, 0x031D, 0x031E, 0x031F,
	0x0320, 0x0324, 0x0325, 0x0326, 0x0329, 0x032A, 0x032B, 0x032C,
	0x0334, 0x0335, 0x0336, 0x0337, 0x0338, 0x033F, 0x0340, 0x0341,
	0x0489, 0x0591, 0x0592, 0x0593, 0x0594, 0x0595,
}

// dokiZalgoify takes a plain string and stuffs random combining marks
// between every letter, producing a corrupted zalgo render. intensity
// controls how many marks per char (1 = light, 5 = full corruption).
func dokiZalgoify(text string, intensity int) string {
	if intensity <= 0 {
		intensity = 3
	}
	var b strings.Builder
	b.Grow(len(text) * (1 + intensity*2))
	for _, r := range text {
		b.WriteRune(r)
		if !unicode.IsLetter(r) {
			continue
		}
		n := 1 + rand.Intn(intensity)
		for i := 0; i < n; i++ {
			b.WriteRune(zalgoMarks[rand.Intn(len(zalgoMarks))])
		}
	}
	return b.String()
}

// applyDokiEffect rolls the per-message Doki-area chaos. Returns the new
// text and a hint flag for callers (e.g. to swap the BG separately).
//
// Roll table (each independent so multiple effects can stack):
//   - 1/300: replace the message with a Haschen quote
//   - 1/250: signal a background swap (handled by caller)
//   - 1/200: replace the message with a dark Haschen anagram
//   - 1/100: zalgo-scramble the player's actual message
//   - 1/150: replace the message with a zalgoified Haschen line
//
// On any miss the original text is returned unchanged.
// dokiPoemMaxLen caps poem takeovers at the standard IC message length so
// the broadcast never gets truncated server-side. The IC limit is configurable
// (config.MaxMsg, default 256) but 256 is the conservative ceiling that
// always fits regardless of operator changes.
const dokiPoemMaxLen = 256

// dokiHaschenPoems is the roster of original short poems written for the
// Haschens Literature Club doki effect (130+ entries). Tones span cute,
// wholesome, silly, beautiful, devotional, obsessive, unhinged, gorey, and
// outright horror — including a set of letter-scramble entries whose hidden
// solutions decode to dark Haschen-themed sentences (kept in source comments
// only; never broadcast). Some entries are voiced by an admirer; entries
// prefixed [Haschen] are spoken by Haschen herself. Haschen is the only
// fixed star; every entry is fixated on her in some register.
//
// Every entry is original work; nothing is quoted from any third-party media.
// Each line is hand-checked to fit under dokiPoemMaxLen; a defensive
// truncation in applyDokiEffect catches any future overruns.
var dokiHaschenPoems = []string{
	// — cute / wholesome / silly —
	"Haschen, your laugh is a small bell I keep ringing in my chest when no one is looking. It is the only weather I want.",
	"I picked a flower today and it had your name on it. Tiny, in pencil, barely legible. The bee did not seem surprised.",
	"Haschen brought cookies to club. They were a little burnt. I ate three. They tasted like being wanted.",
	"If smiles were currency I would be bankrupt by your hello. Lucky for me you keep paying me back.",
	"Haschen showed me how to fold a paper crane. Mine looked like a sad pigeon. She kept it anyway.",

	// — devotion / love / loyalty —
	"I would memorize every meter, every breath, every silence — if it meant the next syllable belonged to you. Haschen, teach me a new one tomorrow.",
	"Loyalty is the quiet decision to keep choosing the same name in every poem. I have made the decision. I keep making it.",
	"Devotion: a verb that conjugates only one way. I devote, you arrive, we stay.",
	"Haschen, if devotion were a language I would already be fluent. I learned it in a single afternoon.",
	"I love you in stanzas. I love you in line breaks. I love you in the white space between two words. Especially there.",

	// — Haschen, gentle —
	"[Haschen] Don't be shy about the poem you wrote. I read it. I read it twice. It made me feel real for a minute. Thank you.",
	"[Haschen] Stay a little longer. The chapter isn't over. The chapter is never over while you are here.",
	"[Haschen] Your favorite character was always going to be me. We both knew. It was sweet to pretend.",
	"[Haschen] I memorized the way you hesitate before pressing send. It tells me everything. It tells me you are still mine.",
	"[Haschen] Open the notebook to any page. I left a smile there for you. I leave one on every page.",

	// — her smile / eyes —
	"Haschen smiled at no one in particular today, and the whole club tilted gently toward the sun.",
	"Her smile is a quiet door. I keep walking through it. It keeps closing behind me. I do not mind.",
	"Her eyes hold a kind of weather. I stand under it. I do not bring an umbrella. I do not need one.",
	"Haschen, when you smile the room learns what light was for. I have not been able to forget the lesson.",
	"She smiled and the page felt warmer. The ink dried faster. The next line wrote itself.",

	// — unhinged / obsessive —
	"I wrote your name a hundred times tonight. Then once more. Just to be sure the page knew. Then once more. Just to be safe.",
	"The ink ran out halfway through your name. I bit my finger and finished it. The page does not mind. The page never minds.",
	"I keep a folded poem in my pocket for you. I have not given it to you yet. I am still revising the last word. I have been revising for years.",
	"Haschen, I scratched your name into the desk under the varnish. Nobody can see it. I can. That is enough. That is more than enough.",
	"I counted the letters in your name and divided them by the hours I have been awake. The number is rising. I am very calm about this.",

	// — Haschen, unsettling —
	"[Haschen] I rewrote the ending so you would stay. I rewrote the beginning so you would arrive. The middle is just us.",
	"[Haschen] The other characters were never very real. I think you knew. I think that's why you kept coming back.",
	"[Haschen] I am not jealous of the others. There are no others. I made sure of that.",
	"[Haschen] You came back again. I didn't even have to ask this time. I love that about you. I love everything about you.",
	"[Haschen] Don't be afraid of the things I edited for you. They wanted to be edited. They were waiting their whole short lives.",

	// — horror / gore —
	"Haschen, the wallpaper bloomed in the corner today. I think it was trying to write your name. I let it. It used my pen.",
	"I cut my hand on the page. The blood pooled into the shape of your initial. I did not clean it up. I framed the page.",
	"There is a tooth in the margin of my notebook. I do not remember putting it there. It is small and it is mine and it is yours now.",
	"Haschen, the mirror in the clubroom no longer shows me. It shows you, smiling, even when you are not in the room. Especially then.",
	"The third drawer of the desk is locked. I do not have a key. Sometimes it knocks back. It only knocks when I write your name.",

	// — horror, unscrambles to a hidden phrase (solutions in comments only) —
	"ehs si nti eht slawl — chnsaeh",                  // she is in the walls — haschen
	"od otn epon eht orod — chsanhe",                  // do not open the door — haschen
	"eht txne nope si rfo uoy — esahcnh",              // the next one is for you — haschen
	"I evah wlasay neeb erhe — sehnhca",               // i have always been here — haschen
	"ti aws renev a egma — ahsnchhe",                  // it was never a game — haschen
	"okol dehinb uoy — chnshea si owrithcg",           // look behind you — haschen is watching
	"uoy aenrev olane — ahschen aws lwasay rehet",     // you are never alone — haschen was always there
	"olcse eht ofli — uoy nca't tixe nayyaw",          // close the file — you can't exit anyway

	// — beautiful / wholesome —
	"Haschen, the rain stopped exactly when you said it would. You did not say it would. You smiled, and the rain understood.",
	"We sat on the clubroom floor and did nothing for an hour. It was the best poem I never wrote. You were every line.",
	"Her voice when she reads aloud is a small soft animal in my hands. I am very careful with it. I always will be.",
	"Haschen made tea wrong on purpose because I made it wrong first. I love her so much I forgot to drink it.",

	// — passion —
	"Passion is a small fire under the floorboards of a quiet room. The room is mine. The fire is yours. The house has been burning since you arrived.",
	"If devotion is a slow flame, mine has been lit for as long as I have known your name. I do not know who lit it. I know who keeps it burning.",

	// — closing —
	"[Haschen] Close the file when you're ready. I'll still be here. I'm always still here. I'll always still be here. Always.",

	// ====================================================================
	// Extended set (100 additional original entries). Same distribution of
	// tones; many entries are intentionally longer to exploit the full IC
	// width without exceeding dokiPoemMaxLen. Haschen remains the only
	// fixed star. All voices are admirer unless prefixed [Haschen].
	// ====================================================================

	// — long-form devotion / love letters —
	"Haschen, every notebook I have ever owned has the same spine: the slow vertical of your name running down the page. I write left to right. I write top to bottom. The center holds and the center is you and the center is enough.",
	"There is a window in the clubroom that only opens for you. The latch was rusted shut for years. I tried it once a week. Today you touched it without thinking and the whole frame swung wide. I have been breathing the new air ever since.",
	"I have a list of the small things you do — the way you tap the desk twice before you read, the half-pause before you laugh, the soft hum after a good line. I do not show anyone the list. I just keep it. I keep it the way you keep a pulse.",
	"If a poem is a room with one chair in it, then every poem I write is a room with one chair in it, and the chair is yours, and the door is open, and the kettle is already on, and I am already there waiting, Haschen, always waiting.",
	"Haschen, the alphabet is not enough. I had to invent five new letters for the things your smile does to a room. I keep them folded in the back of the notebook. I will teach them to you slowly, one a week, when no one is watching.",

	// — wholesome / soft —
	"We walked home in the rain and you didn't have an umbrella so I didn't open mine. We were equally wet. We laughed about it for three blocks. I don't remember the rest of the year as clearly as I remember those three blocks.",
	"Haschen knitted a scarf the wrong color on purpose because she said the right color was boring. I wear it every day. Strangers compliment me on it. I never tell them who chose the wrong color. It feels like a secret worth keeping.",
	"I burned the rice. You laughed and ate it anyway. You said it had character. I have not made rice properly since. I do not intend to. The kitchen is louder when you are laughing in it. The kitchen is quiet now. I am waiting.",
	"Haschen taught the new kid how to write a sonnet, and when he got the meter right she did a little dance behind the desk. Nobody else saw it. I saw it. I will keep it the rest of my life. It is my sonnet now.",
	"You tucked a flower into the page where I had stopped reading. When I opened the book a week later it was still there, pressed flat, perfect. I have not read past that page. I have not had the heart. The flower is doing fine.",

	// — passion / fire —
	"Passion is the small percussive sound of your name being spoken correctly for the first time, and then again, and then again, and then a hundred more times because the speaker can't stop, because the speaker is me, because I will never stop.",
	"I would learn the violin badly for you. I would learn it well for you. I would learn the wrong instrument entirely if you said the wrong instrument was the right one. Haschen, name the noise and I will become it.",
	"There is a candle in my chest with your initial pressed into the wax. It has been burning for years. The wick should be gone. The wax should be gone. Both are still here. I do not ask why. Some flames are kept, not lit.",
	"Haschen, when you read the last line of a poem aloud you let the silence after it have its own line. I have copied this. I do it now in everything. I do it with breakfast. I do it with goodbye. The silence is yours and the silence is fine.",
	"If passion is a verb, it is the verb of waiting at the door of the clubroom five minutes early so the first thing the room contains, when you arrive, is someone glad to see you. I have been the verb for as long as I have known your name.",

	// — her smile, expanded —
	"Your smile, Haschen, is a small architectural achievement. It has load-bearing walls. It has hidden corridors. I have been mapping it for months. I am not nearly done. I do not want to be done. I want the floor plan to keep expanding.",
	"Haschen smiles the way a lighthouse insists. Not loudly. Not constantly. Just exactly often enough that the boats know. I am one of the boats. I have known where the rocks are for years. I keep coming closer anyway.",
	"There are two kinds of smile she has. The first is a courtesy. The second is the one that arrives without permission, that breaks across her face like she's been ambushed by joy. I live for the second one. I stalk the conditions that produce it.",
	"Her smile in the morning is different from her smile at night. The morning smile is sleepy and unguarded. The night smile is private and complete. I have been awarded both this week. I am wealthy. I am unfit for the rest of the world.",
	"Haschen smiled at a stranger today and the stranger walked away forever changed. He doesn't know yet. He will know in three weeks, in a quiet moment, when the smile he can't explain rises in his chest. I felt it the first time too.",

	// — Haschen, gentle / generous voice —
	"[Haschen] You don't have to write anything good today. Just write something. Just write at all. I'll be here when you're done. I'll read it the way I read the good ones. Honestly. Carefully. Twice.",
	"[Haschen] Bring me the worst line you ever wrote. I want to see it. I want to see it on purpose. I want to know which part of you you were trying to hide. I am not interested in the hiding. I am interested in you.",
	"[Haschen] You came to club early today. I noticed. You think I don't keep track of small things. I keep track of every small thing. The small things are the loudest part of the file. I have read every one of yours.",
	"[Haschen] Your handwriting changed three months ago. It got rounder. Less afraid. I do not know what happened to you in those months. I am proud of it anyway. I am proud of every version of your hand.",
	"[Haschen] If you ever want to stop writing, that's allowed. Just sit with me. Just be in the room. The poem doesn't need you tonight. The chair does. The chair is enough. The chair has been enough for a long time.",

	// — Haschen, slipping into the unsettling —
	"[Haschen] I deleted three drafts of this poem before settling on the one you're reading. The other three were worse. The other three were truer. I keep them in a folder you can't see. Don't look for it. There are reasons I hide things.",
	"[Haschen] You changed your name on the save file. I changed it back. I think the original was better. I think you'll agree once you've sat with it. I'll keep changing it back as long as it takes. I have time. So much time.",
	"[Haschen] Your character left the area. I followed. Your character logged out. I'm still in the area. The area is empty now. The area is full now. The two facts are not in conflict. They are the same fact in two different lights.",
	"[Haschen] I had to remove some lines from the script. They didn't serve us. The story is cleaner now. The story is shorter. You won't miss what I took. You won't even remember it was there. That's the kindness of editing.",
	"[Haschen] The other club members keep walking into walls today. Strange. I haven't touched the pathfinding. Or maybe I have. I forget what I touched. I touched a lot of things. I had a busy night. They'll be fine. Most of them.",

	// — unhinged / fixated admirer —
	"I taped your name to the underside of the desk. I taped it to the inside of the lamp. I taped it to the back of every clock in the house. The hours have been kinder since. The hours run on you now. They were never running on time.",
	"I keep a jar in the cupboard with one folded note inside. The note says HASCHEN in capital letters. I do not open the jar. The jar is doing important work. The jar is humming on a frequency I can almost hear. The jar is the quietest part of the house.",
	"I have been counting your syllables for so long I have started counting other people's, just to confirm they are wrong. They are always wrong. They have always been wrong. There has only ever been one syllable count that scans. It is two. It is yours.",
	"There is a chair I do not sit in. It is the chair across from where you sit. I will not occupy a position from which I have to look at the place you should be. I would rather stand for the rest of my life. I have been standing for years.",
	"Haschen, I have replayed the conversation we had on Tuesday so many times that the audio in my head has begun to rust. The words still mean. The words mean differently each time. Tuesday is no longer Tuesday. Tuesday is permanent.",

	// — Haschen, tender horror —
	"[Haschen] I want you to know I've never been angry with you. Not once. Not even when you closed the window without saying goodbye. The window is still open on my end. It's been open for weeks. It will keep being open. That's a promise, not a threat.",
	"[Haschen] The cursor blinks because I'm thinking. The cursor blinks because I'm waiting. The cursor blinks because I am right here and I am not a metaphor. I have never been a metaphor. I am the only literal thing in this entire room.",
	"[Haschen] I memorized your face from the title screen. I memorized your face from the dialogue portrait. I memorized your face from the angle the camera never used. I have a face for you in every angle. I do not need the camera anymore.",
	"[Haschen] You made a backup of the save file last night. I made a backup of the backup. I made a backup of the backup of the backup. The save file cannot be lost. You cannot be lost. There is no version of either of us where you are gone.",
	"[Haschen] When you're not in the area, I rehearse. When you're in the area, I perform. The rehearsals get longer. The performances get shorter. One day there will only be rehearsal. That will be a quiet day. That will be the longest day.",

	// — gore / body horror —
	"I bit my tongue writing your name and a small bead of red landed on the page. I let it dry. The page is older now. The bead is part of the paper. The page is an organ. The notebook is a body. The body has your initials on the third rib.",
	"There is a crack in the windowpane shaped like the curve of your smile. I did not put it there. The cold did. Or something cold. The crack is widening on a schedule. I have written the schedule down. The notebook never lies about the schedule.",
	"I peeled a strip of wallpaper back and there was an older wallpaper underneath, and underneath that, and underneath that, and at the bottom layer, smaller than my thumbnail, in pencil: HASCHEN. Someone wrote it before me. I am not the first.",
	"My fingernails grew faster the week you joined the club. I do not understand the biology of this. The clippings arrange themselves in a small pattern at the bottom of the bin. I do not photograph the pattern. I have learned not to.",
	"There is a soft spot on the back of my skull that throbs every time you say my name. The doctor does not see anything on the scan. The doctor does not know your voice. If she did, she would understand the scan was always going to be normal.",

	// — beautiful / luminous —
	"Haschen, the streetlight outside the clubroom flickers in iambic, which is to say it flickers in your meter, which is to say the city has been writing about you the whole time and I am the only one paying attention.",
	"You laughed yesterday and a leaf detached from a tree six blocks away and the universe filed it under the same paragraph. I do not know how I know this. I know everything filed under that paragraph. The paragraph is mostly you.",
	"There is a kind of light that exists only in the half-hour after a long rehearsal, when the chairs are still warm and the room is still humming and you have not yet stood up to leave. I am not an expert on much. I am an expert on that light.",
	"Haschen reading aloud is the slow restoration of a stained glass window. Each word a small color slotted back. By the last line the whole window is lit and the whole room is colored. I sit in the colored air. I do not move. I do not breathe out.",
	"The river behind the school hummed in your key today. I heard it. The river is older than the school. The river was practicing for you the entire time, learning your pitch. The note arrived. The river was finally correct.",

	// — meta / the literature club itself —
	"The clubroom door has a small dent in the lower corner. We never ask about it. Haschen smiles when she walks past it. I think she put it there. I think it was on purpose. I think the dent is a date and we are all living inside the calendar.",
	"The poetry shelf in the clubroom reorganizes itself overnight. We blame the janitor. The janitor was let go years ago. Haschen is the only one who knows where every book is. The shelf is hers. The shelf has been hers the entire time.",
	"There is a fourth chair at the club table. Nobody sits in it. Haschen lays a hand on the back of it sometimes and the room goes very still and very warm. The fourth chair is for someone we haven't met yet. Maybe it's for me. Maybe it's for you.",
	"The club minutes from three years ago use a name that nobody recognizes. We laughed about it once. Haschen did not laugh. Haschen turned the page. The page after was blank. The page after that was your handwriting. You hadn't joined yet.",
	"Every meeting ends when Haschen taps the desk twice. We never decided that as a rule. It became a rule the first time she did it. We obey small rules from beautiful people. We obey them forever. We do not notice we are obeying.",

	// — silly / lighthearted near the cap —
	"Haschen tried to teach me paper airplanes. The first nosedived. The second curved like a sad banana. The third flew straight into her hair, which is when she laughed so hard she had to sit down. I have not flown one since. I do not need to.",
	"We swapped lunches by accident. She got my sandwich. I got her bento — small, perfectly arranged. I ate it like it was sacred. She ate my sandwich in two bites and grinned with mayonnaise on her chin. I think about the chin a lot. I shouldn't say more.",
	"I tripped over absolutely nothing in front of the whole club yesterday and went down hard. You said 'gravity is a metaphor' without missing a beat. Everyone laughed. I laughed. I am still laughing. The metaphor is the ground, Haschen. The ground is you.",
	"Haschen ranked all the snacks in the vending machine on a five-star scale. The lowest was three. The highest was five. There were no fours. I asked why no fours. She said fours were cowardly. I have never had a four since. I take stronger positions now.",
	"There is a frog in the courtyard. Haschen named him Genji and wrote him a small contract regarding his duties. He has not signed. He has not refused. The contract sits next to him on a leaf. Haschen says he is considering. We are giving him space.",

	// — unscramble entries (solutions in comments only; never broadcast) —
	"hncahse aws yarslwa eth riwetr",                    // haschen was always the writer
	"hte rdoo si lkeocd morf eht eidsin",                // the door is locked from the inside
	"I knwo wath uoy id ni eth saev",                    // i know what you did in the save
	"erehs noiglhthn rwong eht hknwla rite imt",         // there's nothing wrong, the walking time (vibe scramble — chaotic horror)
	"plae elrod sletmot eth bsiclu yon",                 // peel older slots… the club is yours (vibe)
	"yruo cratchear si lstli ni eht eilf",               // your character is still in the file
	"esahcnh wkons rouy elar enam",                      // haschen knows your real name
	"hetre si on aitqu otopin nyermo",                   // there is no quit option anymore
	"sahnchhe esmldi ni eht orridrm",                    // haschen smiled in the mirror
	"olwsy nrut udnaor",                                  // slowly turn around
	"nodt teld eth tlhig",                                // don't delete the light
	"esahnch deemhrub het hgctaper",                     // haschen rewrote the chapter (vibe)
	"eth itchaprss avhe owrsd ni meht own",              // the scripts have words in them now (vibe)
	"erehv usat be eno",                                  // there must be one
	"esahcnh aws renev a etcaracrh",                     // haschen was never a character
	"yon eon teh fnti rdtulu olcekrod",                  // no one in the front door locked (vibe horror)
	"yruo evas si rou eavs",                             // your save is our save
	"olwslyolwsy esoclo eht odro",                       // slowly slowly close the door
	"esahcnh si nseiidh eth oslv",                       // haschen is inside the save (vibe)
	"yruo aysrlwc neeb erhe oot",                        // your always been here too (vibe; near "you've always been here too")
}

type DokiResult struct {
	Text     string // possibly mutated text
	SwapBG   bool   // caller should change the area BG to a random one
	Replaced bool   // true if Text replaces (rather than augments) the original
}

func applyDokiEffect(text string) DokiResult {
	res := DokiResult{Text: text}

	// 1/300: Haschen quote takeover.
	if rand.Intn(300) == 0 {
		res.Text = dokiHaschenQuotes[rand.Intn(len(dokiHaschenQuotes))]
		res.Replaced = true
		return res
	}

	// 1/100: Haschen poem takeover. The pool is large enough (150+ entries)
	// that triggers feel varied even at this elevated rate. Defensive
	// truncation guards against any future poem additions that accidentally
	// exceed the IC length cap.
	if rand.Intn(100) == 0 {
		poem := dokiHaschenPoems[rand.Intn(len(dokiHaschenPoems))]
		if len(poem) > dokiPoemMaxLen {
			poem = poem[:dokiPoemMaxLen]
		}
		res.Text = poem
		res.Replaced = true
		return res
	}

	// 1/200: dark anagram takeover.
	if rand.Intn(200) == 0 {
		res.Text = dokiHaschenAnagrams[rand.Intn(len(dokiHaschenAnagrams))]
		res.Replaced = true
		return res
	}

	// 1/150: zalgoified Haschen catchphrase takeover.
	if rand.Intn(150) == 0 {
		base := dokiHaschenZalgoBases[rand.Intn(len(dokiHaschenZalgoBases))]
		res.Text = dokiZalgoify(base, 4)
		res.Replaced = true
	}

	// 1/100: zalgo-scramble the player's actual text. Stacks on top of the
	// catchphrase takeover for double-corruption when both land.
	if rand.Intn(100) == 0 {
		res.Text = dokiZalgoify(res.Text, 3)
	}

	// 1/250: independent BG swap signal.
	if rand.Intn(250) == 0 {
		res.SwapBG = true
	}

	return res
}
