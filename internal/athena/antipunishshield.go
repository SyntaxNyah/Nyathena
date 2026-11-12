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

// /antipunishshield — an admin-only, restart-persistent protection from the
// punishment system.
//
// Two modes:
//
//	/antipunishshield <uid>            spoof (default)
//	/antipunishshield <uid> backfire   spoof + apply the punishment to the issuer
//
// A shielded player never actually receives a punishment. Instead the issuer is
// recorded as a "spoof" viewer of that player's messages: every IC/OOC line the
// player sends is transformed with the punishment the issuer thought they
// applied, but only for that issuer. Everyone else, including the player, sees
// the normal message. In backfire mode the punishment is also applied to the
// issuer for real.
//
// The shield is an in-memory registry keyed by the player's HDID (and IPID as a
// fallback), so it survives reconnects but not a server restart, exactly as
// requested. It does not touch /kick, /ban, or /mute: those are real moderation,
// not punishment-system effects.

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

const (
	antiPunishShieldOff int32 = iota
	antiPunishShieldSpoof
	antiPunishShieldBackfire
)

// antiPunishSpoof is one punishment an issuer thinks they landed on a shielded
// player. expiresAt is zero for a permanent (until-restart) spoof.
type antiPunishSpoof struct {
	pType     PunishmentType
	expiresAt time.Time
}

// antiPunishShieldStore survives reconnects for the lifetime of the process.
// The map values are the antiPunishShield* mode constants.
var antiPunishShieldStore = struct {
	sync.RWMutex
	byHDID map[string]int32
	byIPID map[string]int32
}{
	byHDID: make(map[string]int32),
	byIPID: make(map[string]int32),
}

// AntiPunishShieldMode returns antiPunishShieldOff/Spoof/Backfire.
func (client *Client) AntiPunishShieldMode() int32 {
	return client.antiPunishShield.Load()
}

// SetAntiPunishShield records the shield mode on this connection.
func (client *Client) SetAntiPunishShield(mode int32) {
	client.antiPunishShield.Store(mode)
}

// RecordAntiPunishSpoof remembers that issuer believes they punished this
// client. Re-applying the same type refreshes its expiry rather than stacking
// duplicates.
func (client *Client) RecordAntiPunishSpoof(issuer *Client, pTypes []PunishmentType, duration time.Duration) {
	if issuer == nil || len(pTypes) == 0 {
		return
	}
	var expiresAt time.Time
	if duration > 0 {
		expiresAt = time.Now().UTC().Add(duration)
	}

	client.mu.Lock()
	if client.antiPunishSpoofs == nil {
		client.antiPunishSpoofs = make(map[*Client][]antiPunishSpoof)
	}
	_, existed := client.antiPunishSpoofs[issuer]
	list := client.antiPunishSpoofs[issuer]
	for _, pType := range pTypes {
		replaced := false
		for i := range list {
			if list[i].pType == pType {
				list[i].expiresAt = expiresAt
				replaced = true
				break
			}
		}
		if !replaced {
			list = append(list, antiPunishSpoof{pType: pType, expiresAt: expiresAt})
		}
	}
	client.antiPunishSpoofs[issuer] = list
	client.mu.Unlock()

	if !existed {
		client.antiPunishSpoofCount.Add(1)
	}
}

// AntiPunishSpoofs returns a copy of the active spoof map, dropping expired
// entries and disconnected issuers. The atomic count is the no-spoofs fast
// path for the IC/OOC hot paths.
func (client *Client) AntiPunishSpoofs() map[*Client][]antiPunishSpoof {
	if client.antiPunishSpoofCount.Load() == 0 {
		return nil
	}
	now := time.Now().UTC()

	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.antiPunishSpoofs) == 0 {
		client.antiPunishSpoofCount.Store(0)
		return nil
	}

	out := make(map[*Client][]antiPunishSpoof, len(client.antiPunishSpoofs))
	for issuer, list := range client.antiPunishSpoofs {
		if issuer == nil || issuer.closed.Load() {
			delete(client.antiPunishSpoofs, issuer)
			continue
		}
		active := make([]antiPunishSpoof, 0, len(list))
		for _, sp := range list {
			if sp.expiresAt.IsZero() || now.Before(sp.expiresAt) {
				active = append(active, sp)
			}
		}
		if len(active) == 0 {
			delete(client.antiPunishSpoofs, issuer)
			continue
		}
		out[issuer] = active
	}
	if len(out) == 0 {
		out = nil
	}
	client.antiPunishSpoofCount.Store(int32(len(out)))
	return out
}

// setAntiPunishShield applies a mode now and records it by HDID/IPID so the
// same player gets it again after reconnecting.
func setAntiPunishShield(target *Client, mode int32) {
	target.SetAntiPunishShield(mode)
	antiPunishShieldStore.Lock()
	if hdid := target.Hdid(); hdid != "" {
		antiPunishShieldStore.byHDID[hdid] = mode
	}
	if ipid := target.Ipid(); ipid != "" {
		antiPunishShieldStore.byIPID[ipid] = mode
	}
	antiPunishShieldStore.Unlock()
}

// applyStoredAntiPunishShield is called during the join handshake. HDID is the
// primary key because it survives IP changes; IPID is the fallback for the
// (normally impossible) case where the HDID is unavailable.
func applyStoredAntiPunishShield(client *Client) {
	var mode int32
	antiPunishShieldStore.RLock()
	if hdid := client.Hdid(); hdid != "" {
		mode = antiPunishShieldStore.byHDID[hdid]
	}
	if mode == antiPunishShieldOff {
		if ipid := client.Ipid(); ipid != "" {
			mode = antiPunishShieldStore.byIPID[ipid]
		}
	}
	antiPunishShieldStore.RUnlock()

	if mode != antiPunishShieldOff {
		client.SetAntiPunishShield(mode)
	}
}

// applyAntiPunishShieldIfActive is the single gate every moderator punishment
// path calls before applying a real effect. It returns true when the target is
// shielded and the caller must skip the real punishment. It records the spoof
// for the issuer and, in backfire mode, applies the punishment to the issuer.
func applyAntiPunishShieldIfActive(issuer, target *Client, pTypes []PunishmentType, duration time.Duration, reason string) bool {
	mode := target.AntiPunishShieldMode()
	if mode == antiPunishShieldOff {
		return false
	}
	// A player punishing themselves is still protected, but recording a spoof
	// against themselves would exclude them from their own normal broadcast.
	if issuer == nil || issuer == target {
		return true
	}

	target.RecordAntiPunishSpoof(issuer, pTypes, duration)
	backfired := mode == antiPunishShieldBackfire
	if backfired {
		applyAntiPunishBackfire(issuer, pTypes, duration, reason)
	}
	sendAntiPunishShieldPopup(target, issuer, pTypes, backfired)
	return true
}

// sendAntiPunishShieldPopup is the sneaky part: the shielded player is told a
// moderator just tried to punish them, what it was, and whether it backfired.
// The moderator sees the spoof and thinks it worked, so the target acting
// normal is what gaslights them.
func sendAntiPunishShieldPopup(target, issuer *Client, pTypes []PunishmentType, backfired bool) {
	if target == nil || issuer == nil || len(pTypes) == 0 {
		return
	}
	names := make([]string, len(pTypes))
	for i, pType := range pTypes {
		names[i] = "'" + pType.String() + "'"
	}
	outcome := "It didn't stick"
	if backfired {
		outcome = "It backfired on them instead"
	}
	target.SendPopup(fmt.Sprintf(
		"🤫 shh! %v tried to punish you with %v. %v — ur punishment protected. Act normal.",
		oocDisplayName(issuer), strings.Join(names, ", "), outcome))
}

// applyAntiPunishBackfire puts the punishment on the moderator who tried to
// apply it. This deliberately bypasses the issuer's own shield: backfire is a
// consequence, not a fresh moderator punishment.
func applyAntiPunishBackfire(issuer *Client, pTypes []PunishmentType, duration time.Duration, reason string) {
	if issuer == nil || len(pTypes) == 0 {
		return
	}
	var expires int64
	if duration > 0 {
		expires = time.Now().UTC().Add(duration).Unix()
	}
	for _, pType := range pTypes {
		issuer.addPunishmentBy(pType, duration, reason, IssuerSystem, true)
		if err := db.UpsertTextPunishmentBy(issuer.Ipid(), int(pType), expires, reason, int(IssuerSystem)); err != nil {
			logger.LogErrorf("Failed to persist anti-punishment backfire for %v: %v", issuer.Ipid(), err)
		}
	}
	// Deliberately no announcement: the moderator only sees the normal success
	// summary, so the backfire lands silently. The shielded player is told via
	// the sneaky popup instead.
}

// antiPunishSpoofText applies the spoofed punishment types to one message. The
// special-cased types need the speaker's name or area; everything else goes
// through the normal text transform.
func antiPunishSpoofText(target *Client, base string, spoofs []antiPunishSpoof) string {
	text := base
	for _, sp := range spoofs {
		switch sp.pType {
		case PunishmentLovebomb:
			text = applyLovebombMessage("")
		case PunishmentClickbait:
			text = applyClickbaitWithName(text, clientDisplayName(target))
		case PunishmentMarkov:
			text = applyMarkov(text, target.Area())
		case PunishmentThirdPerson:
			text = applyThirdPersonWithName(text, clientDisplayName(target))
		default:
			text = ApplyPunishmentToText(text, sp.pType)
		}
	}
	return text
}

// broadcastSpoofedIC sends the normal IC packet to everyone except the spoofed
// issuers, then sends each issuer their own transformed copy. Returns true when
// it handled the broadcast.
func broadcastSpoofedIC(sender *Client, ms *MSToClient) bool {
	spoofs := sender.AntiPunishSpoofs()
	if len(spoofs) == 0 {
		return false
	}

	senderIsMod := senderBypassesIgnore(sender.Perms())
	excluded := make(map[int]struct{}, len(spoofs))
	for issuer := range spoofs {
		excluded[issuer.Uid()] = struct{}{}
	}

	senderArea := sender.Area()
	clients.ForEach(func(c *Client) {
		if c.Area() != senderArea {
			return
		}
		if _, skip := excluded[c.Uid()]; skip {
			return
		}
		if !senderIsMod && c.IgnoresIPID(sender.Ipid()) {
			return
		}
		c.Send(ms)
	})

	for issuer, list := range spoofs {
		if issuer.Area() != senderArea {
			continue
		}
		spoofed := *ms
		spoofed.Message = encode(antiPunishSpoofText(sender, decode(ms.Message), list))
		issuer.Send(&spoofed)
	}
	return true
}

// broadcastSpoofedOOC is the OOC half of broadcastSpoofedIC.
func broadcastSpoofedOOC(sender *Client, displayName, msg string) bool {
	spoofs := sender.AntiPunishSpoofs()
	if len(spoofs) == 0 {
		return false
	}

	senderIsMod := senderBypassesIgnore(sender.Perms())
	excluded := make(map[int]struct{}, len(spoofs))
	for issuer := range spoofs {
		excluded[issuer.Uid()] = struct{}{}
	}

	senderArea := sender.Area()
	clients.ForEach(func(c *Client) {
		if c.Area() != senderArea {
			return
		}
		if _, skip := excluded[c.Uid()]; skip {
			return
		}
		if !senderIsMod && c.IgnoresIPID(sender.Ipid()) {
			return
		}
		c.Send(&aolib.CTToClient{Name: displayName, Message: msg, IsFromServer: false})
	})

	for issuer, list := range spoofs {
		if issuer.Area() != senderArea {
			continue
		}
		issuer.Send(&aolib.CTToClient{
			Name: displayName, Message: antiPunishSpoofText(sender, msg, list), IsFromServer: false,
		})
	}
	return true
}

// Handles /antipunishshield <uid> [backfire]. Admin-only via the registry.
func cmdAntiPunishShield(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	if len(args) > 2 {
		client.SendServerMessage("Too many arguments:\n" + usage)
		return
	}

	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", uid))
		return
	}

	mode := antiPunishShieldSpoof
	modeName := "spoof"
	if len(args) == 2 {
		switch strings.ToLower(args[1]) {
		case "backfire":
			mode = antiPunishShieldBackfire
			modeName = "backfire"
		case "spoof", "normal":
			// default mode, named explicitly
		default:
			client.SendServerMessage("Unknown mode. Use /antipunishshield <uid> [backfire].")
			return
		}
	}

	setAntiPunishShield(target, mode)
	client.SendServerMessage(fmt.Sprintf("🛡 Anti-punishment shield applied to %v (%v mode) until server restart.",
		oocDisplayName(target), modeName))
	addToBuffer(client, "CMD", fmt.Sprintf("Applied anti-punishment shield (%v) to UID %v.", modeName, uid), true)
}
