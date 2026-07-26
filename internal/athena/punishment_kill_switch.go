/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: console-only global punishment kill switch. */

package athena

import (
	"sync/atomic"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

// punishmentsGloballyDisabled is a server-wide kill switch for the punishment
// system, ranking above every in-game authority tier (moderator, shadow mod,
// even ADMIN). It can only be armed or disarmed from the server operator's
// own stdin console — see cli.go's "punishment enable|disable|status"
// command — never from an in-game command, slash command, or the Discord
// bridge. While armed, every punishment-system command refuses for everyone;
// only /ban, /kick, and /mute (real moderation enforcement, not the
// punishment system) keep working.
var punishmentsGloballyDisabled atomic.Bool

// SetPunishmentsGloballyDisabled arms or disarms the switch. Only ever called
// from the stdin CLI (ListenInput).
func SetPunishmentsGloballyDisabled(v bool) {
	punishmentsGloballyDisabled.Store(v)
}

// PunishmentsGloballyDisabled reports the switch's current state.
func PunishmentsGloballyDisabled() bool {
	return punishmentsGloballyDisabled.Load()
}

// punishmentsDisabledMessage is shown to whoever — moderator, shadow mod, or
// admin — tries to land a punishment-system effect while the switch is armed.
const punishmentsDisabledMessage = "🎲 Punishments are globally disabled right now. Whether the server owner flips the switch on in the console is the luck of the draw — some days it's on, some days it's off. Makes things more fun! Try again later, sorry shadow mods, admins, and trolls :("

// punishmentsSystemDisabled reports whether the console switch is armed and,
// if so, sends the caller the standard notice. Punishment-issuing command
// handlers call this once at their top and bail out on true.
func punishmentsSystemDisabled(client *Client) bool {
	if !punishmentsGloballyDisabled.Load() {
		return false
	}
	client.SendServerMessage(punishmentsDisabledMessage)
	return true
}

// punishmentSafeArea folds the console switch into an area's own
// punishment-safe flag, so every existing call site already gated on
// Area.PunishmentSafe() — directly, or via punishmentSafeBlocked — also
// honors the console override for free.
func punishmentSafeArea(a *area.Area) bool {
	return punishmentsGloballyDisabled.Load() || a.PunishmentSafe()
}
