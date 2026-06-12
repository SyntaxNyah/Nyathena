/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: wave-2 punishment command handlers.

   Plain wrappers binding the wave-2 text-transform punishments to the
   shared cmdPunishment plumbing (-d/-r/-h, comma UID lists, `global`,
   stacking, DB persistence). Transforms live in punishments_v2.go and
   punishments_weeb.go; protocol/viewport handlers live in
   punishments_protocol.go; delivery and mechanic handlers live in
   punishments_lifo.go and punishments_mechanics.go. */

package athena

func cmdZalgo(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentZalgo)
}

func cmdLeetspeak(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLeetspeak)
}

func cmdSmallcaps(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSmallcaps)
}

func cmdPiglatin(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPiglatin)
}

func cmdVaporwave(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVaporwave)
}

func cmdLisp(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentLisp)
}

func cmdSpoonerism(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentSpoonerism)
}

func cmdKeysmash(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentKeysmash)
}

func cmdWeeb(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentWeeb)
}

func cmdPolitician(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentPolitician)
}

func cmdClickbait(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentClickbait)
}

func cmdMarkov(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentMarkov)
}

func cmdAlliteration(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentAlliteration)
}

func cmdCipher(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentCipher)
}
