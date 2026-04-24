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

// Moderation commands for the voice-chat feature.
//
//   /vmute   <ipid>[,<ipid>...] [-d <seconds>] [-r <reason>]
//   /vunmute <ipid>[,<ipid>...]
//   /vkick   <uid>[,<uid>...]           (forces the client out of voice; no persistence)
//   /vkick   -i <ipid>[,<ipid>...]      (kicks every client for those IPIDs)
//   /vban    <ipid>[,<ipid>...] [-d <seconds>] [-r <reason>]
//   /vunban  <ipid>[,<ipid>...]
//   /vlist                              (lists voice participants in the current area)
//   /vbans                              (lists active voice mutes and bans)
//   /voicearea on|off                   (toggles voice for the current area; hot-ejects)

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// parseCsvIpids splits a comma-separated argument into IPIDs, dropping empties.
func parseCsvIpids(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cmdVmute installs an IPID-scoped voice mute.  Any live clients matching the
// IPID are ejected from their current voice room.
func cmdVmute(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.Int("d", 0, "")
	reason := flags.String("r", "", "")
	_ = flags.Parse(args)
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	ipids := parseCsvIpids(flags.Arg(0))
	if len(ipids) == 0 {
		client.SendServerMessage("No valid IPIDs provided.")
		return
	}
	dur := time.Duration(0)
	if *duration > 0 {
		dur = time.Duration(*duration) * time.Second
	}
	kicked := 0
	for _, ipid := range ipids {
		SetVoiceMute(ipid, dur, *reason)
		kicked += kickVoiceByIPID(ipid)
	}
	suffix := "permanently"
	if dur > 0 {
		suffix = fmt.Sprintf("for %ds", *duration)
	}
	client.SendServerMessage(fmt.Sprintf("Voice-muted %d IPID(s) %s (ejected %d live client(s)).", len(ipids), suffix, kicked))
	addToBuffer(client, "CMD", fmt.Sprintf("Voice-muted %v %s", ipids, suffix), false)
}

// cmdVunmute lifts a voice mute.
func cmdVunmute(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	ipids := parseCsvIpids(args[0])
	if len(ipids) == 0 {
		client.SendServerMessage("No valid IPIDs provided.")
		return
	}
	removed := 0
	for _, ipid := range ipids {
		if ClearVoiceMute(ipid) {
			removed++
		}
	}
	client.SendServerMessage(fmt.Sprintf("Lifted voice mute for %d IPID(s).", removed))
	addToBuffer(client, "CMD", fmt.Sprintf("Voice-unmuted %v", ipids), false)
}

// cmdVban installs an IPID-scoped voice ban.
func cmdVban(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	duration := flags.Int("d", 0, "")
	reason := flags.String("r", "", "")
	_ = flags.Parse(args)
	if len(flags.Args()) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	ipids := parseCsvIpids(flags.Arg(0))
	if len(ipids) == 0 {
		client.SendServerMessage("No valid IPIDs provided.")
		return
	}
	dur := time.Duration(0)
	if *duration > 0 {
		dur = time.Duration(*duration) * time.Second
	}
	kicked := 0
	for _, ipid := range ipids {
		SetVoiceBan(ipid, dur, *reason)
		kicked += kickVoiceByIPID(ipid)
	}
	suffix := "permanently"
	if dur > 0 {
		suffix = fmt.Sprintf("for %ds", *duration)
	}
	client.SendServerMessage(fmt.Sprintf("Voice-banned %d IPID(s) %s (ejected %d live client(s)).", len(ipids), suffix, kicked))
	addToBuffer(client, "CMD", fmt.Sprintf("Voice-banned %v %s", ipids, suffix), false)
}

// cmdVunban lifts a voice ban.
func cmdVunban(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	ipids := parseCsvIpids(args[0])
	if len(ipids) == 0 {
		client.SendServerMessage("No valid IPIDs provided.")
		return
	}
	removed := 0
	for _, ipid := range ipids {
		if ClearVoiceBan(ipid) {
			removed++
		}
	}
	client.SendServerMessage(fmt.Sprintf("Lifted voice ban for %d IPID(s).", removed))
	addToBuffer(client, "CMD", fmt.Sprintf("Voice-unbanned %v", ipids), false)
}

// cmdVkick ejects a live client (or set of clients) from voice without
// persistent punishment.  Accepts either a UID list as the positional
// argument or an `-i <ipid-list>` flag for IPID-based targeting.
func cmdVkick(client *Client, args []string, usage string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	ipidArg := flags.String("i", "", "")
	_ = flags.Parse(args)

	kicked := 0
	if *ipidArg != "" {
		for _, ipid := range parseCsvIpids(*ipidArg) {
			kicked += kickVoiceByIPID(ipid)
		}
	} else {
		if len(flags.Args()) == 0 {
			client.SendServerMessage("Not enough arguments:\n" + usage)
			return
		}
		for _, c := range getUidList(strings.Split(flags.Arg(0), ",")) {
			if c.Area() != nil && inVoiceRoom(c.Area(), c.Uid()) {
				leaveVoiceForClient(c)
				kicked++
			}
		}
	}
	client.SendServerMessage(fmt.Sprintf("Kicked %d client(s) from voice.", kicked))
	addToBuffer(client, "CMD", fmt.Sprintf("Voice-kicked %d client(s)", kicked), false)
}

// voiceClientRequirementHint is shown to mods/users who may wonder why the
// voice room looks empty or silent.  Vanilla AO2 desktop clients do not speak
// WebRTC — only WebAO builds that include the VC_* handlers can join voice.
const voiceClientRequirementHint = "ℹ️ Voice chat requires a WebAO client built with WebRTC support. " +
	"The classic AO2 desktop client cannot join voice — those players will appear connected for chat but " +
	"will not be able to speak or hear in voice rooms."

// cmdVlist reports everyone currently in the voice room for the caller's
// current area.  Public command — all users may invoke it.
func cmdVlist(client *Client, _ []string, _ string) {
	a := client.Area()
	if a == nil {
		return
	}
	peers := currentVoicePeers(a)
	if len(peers) == 0 {
		client.SendServerMessage("No voice participants in this area.\n\n" + voiceClientRequirementHint)
		return
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Voice participants in this area (%d):\n", len(peers)))
	for _, uid := range peers {
		c := clients.GetClientByUID(uid)
		if c == nil {
			fmt.Fprintf(&b, "  - [UID %d] (disconnected)\n", uid)
			continue
		}
		name := c.OOCName()
		if name == "" {
			name = "(no name)"
		}
		fmt.Fprintf(&b, "  - [UID %d] %s — %s\n", uid, name, c.CurrentCharacter())
	}
	client.SendServerMessage(b.String())
}

// cmdVbans lists the currently active voice mutes and voice bans.  Mod-only.
func cmdVbans(client *Client, _ []string, _ string) {
	mutes := ListVoiceMutes()
	bans := ListVoiceBans()
	if len(mutes) == 0 && len(bans) == 0 {
		client.SendServerMessage("No active voice mutes or bans.")
		return
	}
	var b strings.Builder
	now := time.Now().UTC()
	if len(bans) > 0 {
		fmt.Fprintf(&b, "Voice bans (%d):\n", len(bans))
		for ipid, r := range bans {
			b.WriteString(formatVoiceRestrictionLine(ipid, r, now))
		}
	}
	if len(mutes) > 0 {
		fmt.Fprintf(&b, "Voice mutes (%d):\n", len(mutes))
		for ipid, r := range mutes {
			b.WriteString(formatVoiceRestrictionLine(ipid, r, now))
		}
	}
	client.SendServerMessage(b.String())
}

func formatVoiceRestrictionLine(ipid string, r voiceRestriction, now time.Time) string {
	dur := "permanent"
	if !r.Until.IsZero() {
		dur = fmt.Sprintf("%ds left", int(r.Until.Sub(now).Seconds())+1)
	}
	reason := r.Reason
	if reason == "" {
		reason = "(no reason)"
	}
	return fmt.Sprintf("  - %s  [%s]  %s\n", ipid, dur, reason)
}

// cmdVoiceArea toggles the voice_allowed flag for the caller's current area.
// When toggled off, any currently-joined voice peers are ejected immediately.
func cmdVoiceArea(client *Client, args []string, usage string) {
	a := client.Area()
	if a == nil {
		return
	}
	if len(args) == 0 {
		state := "off"
		if a.VoiceAllowed() {
			state = "on"
		}
		client.SendServerMessage(fmt.Sprintf("Voice is currently %s in this area. Usage: %s", state, usage))
		return
	}
	switch strings.ToLower(args[0]) {
	case "on", "true", "1", "yes":
		a.SetVoiceAllowed(true)
		client.SendServerMessage("Voice chat enabled in this area.\n\n" + voiceClientRequirementHint)
		writeToArea(a, "CT", "[SERVER]", "Voice chat has been enabled in this area.", "1")
		addToBuffer(client, "CMD", "Enabled voice chat in area.", false)
	case "off", "false", "0", "no":
		a.SetVoiceAllowed(false)
		kickAllVoiceFromArea(a)
		client.SendServerMessage("Voice chat disabled in this area. All participants ejected.")
		writeToArea(a, "CT", "[SERVER]", "Voice chat has been disabled in this area.", "1")
		addToBuffer(client, "CMD", "Disabled voice chat in area.", false)
	default:
		client.SendServerMessage("Usage: " + usage)
	}
}
