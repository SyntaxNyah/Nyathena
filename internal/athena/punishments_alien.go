/* Athena - A server for Attorney Online 2 written in Go

   Nyathena fork additions: the /alien text punishment.

   /alien is a mod-only (MUTE) filter wired into the shared cmdPunishment
   plumbing (-d/-r/-h, comma UID lists, `global`, /stack, DB persistence,
   /unpunish -t) exactly like the animal filters. It uses the same
   applyAnimalSounds word-for-word substitution those use, so the shape of
   what the target typed survives even though every word is replaced by an
   alien noise.

   The pool deliberately contains no "Mina" prefix or name — this command is
   meant to sound like a space alien, not like a character saying a word. */

package athena

// alienSounds are the noises /alien substitutes for each word. Several
// entries contain a space so a long message does not read as one noise
// stamped out N times.
var alienSounds = []string{
	"zeep",
	"zeep zeep",
	"zeeb",
	"zeeb zeeb",
	"zorp",
	"zorp zeep",
	"bleem",
	"bleem bleem",
	"beep beep",
	"blorp",
	"blorp blorp",
	"glorp",
	"glorp glorp",
	"bweep",
	"bwoop",
	"weep woop",
	"zreee",
	"zreee zreee",
	"keeoo",
	"keeoo keeoo",
	"quirp",
	"quirp quirp",
	"snorp",
	"snorp snorp",
	"fweep fweep",
	"wibble wobble",
	"zap zoop",
	"veeoo veeoo",
	"mrrp",
	"zlorp zlorp",
	"bleep bloop",
	"zoop",
	"kzzzt",
	"vreee",
	"zorbbb",
	"*antennae twitch*",
	"*radio static*",
}

// applyAlien replaces text with alien noises.
func applyAlien(text string) string {
	return applyAnimalSounds(text, "ZEEP ZEEP", alienSounds)
}
