/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: Discord-side handlers for the IPHub firewall
   and server-wide lockdown. Mirrors in-game /firewall and /lockdown so
   moderators can flip these toggles from Discord during an incident. */

package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// handleFirewall handles /firewall on|off.
func (b *Bot) handleFirewall(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	state := optionString(i.ApplicationCommandData().Options, "state")
	on := state == "on"
	if err := b.server.SetFirewall(on); err != nil {
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed to set firewall: %v", err)))
		return
	}
	if on {
		respondEmbed(s, i, successEmbed("Firewall Enabled",
			"🔥 IPHub VPN/proxy screening is now ACTIVE. New IPs will be checked before being allowed to connect."))
	} else {
		respondEmbed(s, i, successEmbed("Firewall Disabled",
			"🔓 IPHub VPN/proxy screening is OFF. New connections are no longer screened."))
	}
}

// handleLockdown handles /lockdown on|off|whitelist_all.
func (b *Bot) handleLockdown(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.requireMod(s, i) {
		return
	}
	action := optionString(i.ApplicationCommandData().Options, "action")
	switch action {
	case "on":
		if err := b.server.SetLockdown(true); err != nil {
			respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed: %v", err)))
			return
		}
		respondEmbed(s, i, successEmbed("Lockdown Enabled",
			"🔒 Server lockdown is now ACTIVE. New IPIDs will be rejected; only previously-known players can join."))
	case "off":
		if err := b.server.SetLockdown(false); err != nil {
			respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed: %v", err)))
			return
		}
		respondEmbed(s, i, successEmbed("Lockdown Disabled",
			"🔓 Server lockdown is OFF. New connections are now allowed."))
	case "whitelist_all":
		count, err := b.server.WhitelistAllConnected()
		if err != nil {
			respondEmbed(s, i, errorEmbed(fmt.Sprintf("Failed: %v", err)))
			return
		}
		respondEmbed(s, i, successEmbed("Whitelisted",
			fmt.Sprintf("✅ Whitelisted **%d** currently-connected IPID(s). They will be allowed back in even while lockdown is active.", count)))
	default:
		respondEmbed(s, i, errorEmbed(fmt.Sprintf("Unknown action: %s", action)))
	}
}
