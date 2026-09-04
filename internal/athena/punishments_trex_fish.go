/* Athena - A server for Attorney Online 2 written in Go

   Nyathena fork additions: the /trex and /fish animal punishments.

   Both are mod-only (MUTE) and wired into the shared cmdPunishment plumbing
   (-d/-r/-h, comma UID lists, `global`, /stack, DB persistence, /unpunish -t)
   exactly like the older animal filters, and both use the same
   applyAnimalSounds word-for-word substitution those use — so a long message
   stays long and a short one stays short, and the shape of what the target
   typed survives even though not one word of it does.

   /trex — the target is a tyrannosaur. The pool is built around RAAASRFH,
   the specific roar this command was asked for, with the rest of the entries
   as variations on it plus the tiny-arms running gag.

   /fish — the target is a fish. Every word becomes a bubble: blub, blublub,
   and the blublublib that named the command. */

package athena

// trexSounds are the roars /trex substitutes for each word. RAAASRFH is the
// canonical one; the others keep a long message from reading as one word
// stamped out N times.
var trexSounds = []string{
	"RAAASRFH",
	"RAAASRFH!",
	"RAASRFH",
	"RAAAASRFH!!",
	"raaasrfh",
	"RAWR",
	"ROAAAR",
	"GRAAAH",
	"RRRAAAGH",
	"*stomp*",
	"*tiny arms flail*",
	"*bites*",
	"*THUD*",
	"SKREEEE",
	"HRAAAA",
}

// fishSounds are the bubbles /fish substitutes for each word.
var fishSounds = []string{
	"blub",
	"blublub",
	"blublublib",
	"blublublub",
	"BLUB",
	"blub blub",
	"bloop",
	"glub",
	"glublub",
	"*bubbles*",
	"*gasps*",
	"*swims away*",
	"blub?",
	"blub!",
	"...blub",
}

// applyTrex replaces text with tyrannosaur roars.
func applyTrex(text string) string {
	return applyAnimalSounds(text, "RAAASRFH", trexSounds)
}

// applyFish replaces text with fish noises.
func applyFish(text string) string {
	return applyAnimalSounds(text, "blublublib", fishSounds)
}
