/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork addition: punishment audit alerts.

   Whenever a moderator issues one of the punishment-system commands (dere
   archetypes, text effects, protocol/viewport curses, traps, voice curses,
   stacking, etc.) every ADMIN-permission holder gets a live OOC alert naming
   the issuing moderator -- mirroring the existing censor-trip alert pattern
   (censor_alerts.go). This gives admins default visibility into what
   regular and shadow moderators are doing with the punishment toolkit, even
   when a punishment was applied with -h (which only suppresses the
   *target*-facing notice, never the admin-facing one). Every trip is also
   written to the persistent audit log so the record survives a restart.

   Self-applied effects (/maso, /megamaso), the showname-punisher drip, and
   contagion catching a bystander never reach here because they call
   AddPunishment/AddPunishmentBy directly rather than through one of the
   instrumented command handlers -- they aren't a moderator issuing a fresh
   command, so CLAUDE.md's exemption for self-applied/system effects holds. */

package athena

import (
	"fmt"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

const punishmentAuditHint = "(Disable these alerts for yourself with /punishaudit off)"

// PunishmentAuditDisabled reports whether this client has muted punishment
// audit alerts for their current session.
func (c *Client) PunishmentAuditDisabled() bool {
	return c.punishAuditOff.Load()
}

// SetPunishmentAuditDisabled mutes or unmutes punishment audit alerts for
// this client's current session.
func (c *Client) SetPunishmentAuditDisabled(off bool) {
	c.punishAuditOff.Store(off)
}

// alertPunishmentIssued notifies every admin (minus those who muted it via
// /punishaudit off, and minus the issuer themselves if they are an admin)
// that issuer applied a punishment command.
//
// targetReport is the comma-separated UID list the caller already built for
// its own addToBuffer/summary line (may be empty for a trap-arming command
// like /silencebell that has no immediate target); targetCount is how many
// clients were hit right now.
func alertPunishmentIssued(issuer *Client, punishmentLabel, targetReport string, targetCount int, duration time.Duration, reason string, hidden bool) {
	if issuer == nil || !permissions.IsModerator(issuer.Perms()) {
		return
	}

	name := issuer.ModName()
	if name == "" {
		name = oocDisplayName(issuer)
	}
	if permissions.IsShadow(issuer.Perms()) {
		name += " (shadow)"
	}

	areaName := "unknown area"
	if a := issuer.Area(); a != nil {
		areaName = a.Name()
	}

	targets := targetReport
	if targets == "" {
		targets = "none yet"
	}

	var extra strings.Builder
	if duration > 0 {
		fmt.Fprintf(&extra, " for %v", duration)
	}
	if reason != "" {
		fmt.Fprintf(&extra, " -- reason: %s", reason)
	}
	if hidden {
		extra.WriteString(" [-h]")
	}

	msg := fmt.Sprintf("%s (UID %d, IPID %s) applied '%s' to %d client(s) [%s] in %s%s.\n%s",
		name, issuer.Uid(), issuer.Ipid(), punishmentLabel, targetCount, targets, areaName, extra.String(), punishmentAuditHint)

	logger.WriteAudit(fmt.Sprintf("PUNISH: %s (UID %d, IPID %s) applied '%s' to %d client(s) [%s] in %s%s",
		name, issuer.Uid(), issuer.Ipid(), punishmentLabel, targetCount, targets, areaName, extra.String()))

	out := &packet.CTToClient{Name: "[AUDIT]", Message: encode(msg), IsFromServer: "1"}
	issuerUID := issuer.Uid()
	clients.ForEach(func(c *Client) {
		if !permissions.IsAdmin(c.Perms()) || c.Uid() == issuerUID {
			return
		}
		if c.PunishmentAuditDisabled() {
			return
		}
		c.Send(out)
	})
}

// cmdPunishAudit handles /punishaudit <on|off>. With no argument it reports
// the caller's current setting. The toggle is per-session: alerts default
// back to on for every fresh connection.
func cmdPunishAudit(client *Client, args []string, usage string) {
	if len(args) == 0 {
		state := "ENABLED"
		if client.PunishmentAuditDisabled() {
			state = "DISABLED"
		}
		client.SendServerMessage(fmt.Sprintf("Punishment audit alerts are currently %s for you.\n%s", state, usage))
		return
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "on":
		client.SetPunishmentAuditDisabled(false)
		client.SendServerMessage("Punishment audit alerts are now ON for you.")
	case "off":
		client.SetPunishmentAuditDisabled(true)
		client.SendServerMessage("Punishment audit alerts are now OFF for you (this session only -- they reset to on when you reconnect).")
	default:
		client.SendServerMessage("Invalid argument:\n" + usage)
	}
}
