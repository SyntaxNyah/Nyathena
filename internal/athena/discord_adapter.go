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
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/discord/bot"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// warnings is a simple in-memory warning store keyed by IPID.
var (
	warningsMu sync.RWMutex
	warnings   = make(map[string][]bot.WarnRecord)
)

// ServerAdapter implements bot.ServerInterface, bridging Discord bot commands to the AO2 server.
type ServerAdapter struct{}

// NewServerAdapter returns a new ServerAdapter.
func NewServerAdapter() *ServerAdapter {
	return &ServerAdapter{}
}

// findClientByArg finds a client by UID (numeric) or OOC name (string).
func findClientByArg(arg string) *Client {
	// Try numeric UID first.
	if uid, err := strconv.Atoi(arg); err == nil {
		c, err := getClientByUid(uid)
		if err == nil {
			return c
		}
	}
	// Fall back to OOC name search.
	arg = strings.ToLower(arg)
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 {
			continue
		}
		if strings.EqualFold(c.OOCName(), arg) || strings.EqualFold(c.CurrentCharacter(), arg) {
			return c
		}
	}
	return nil
}

// GetPlayers returns information about all connected players.
func (a *ServerAdapter) GetPlayers() []bot.PlayerInfo {
	var result []bot.PlayerInfo
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 {
			continue
		}
		areaName := ""
		if c.Area() != nil {
			areaName = c.Area().Name()
		}
		result = append(result, bot.PlayerInfo{
			UID:       c.Uid(),
			Character: c.CurrentCharacter(),
			OOCName:   c.OOCName(),
			Area:      areaName,
			IPID:      c.Ipid(),
		})
	}
	return result
}

// FindPlayer finds a player by UID or name.
func (a *ServerAdapter) FindPlayer(name string) *bot.PlayerInfo {
	c := findClientByArg(name)
	if c == nil {
		return nil
	}
	areaName := ""
	if c.Area() != nil {
		areaName = c.Area().Name()
	}
	return &bot.PlayerInfo{
		UID:       c.Uid(),
		Character: c.CurrentCharacter(),
		OOCName:   c.OOCName(),
		Area:      areaName,
		IPID:      c.Ipid(),
	}
}

// GetPlayerByUID returns a player by UID.
func (a *ServerAdapter) GetPlayerByUID(uid int) *bot.PlayerInfo {
	c, err := getClientByUid(uid)
	if err != nil {
		return nil
	}
	areaName := ""
	if c.Area() != nil {
		areaName = c.Area().Name()
	}
	return &bot.PlayerInfo{
		UID:       c.Uid(),
		Character: c.CurrentCharacter(),
		OOCName:   c.OOCName(),
		Area:      areaName,
		IPID:      c.Ipid(),
	}
}

// GetAreas returns information about all server areas.
func (a *ServerAdapter) GetAreas() []bot.AreaInfo {
	result := make([]bot.AreaInfo, len(areas))
	for i, ar := range areas {
		result[i] = bot.AreaInfo{
			Index:       i,
			Name:        ar.Name(),
			PlayerCount: ar.PlayerCount(),
			Status:      ar.Status().String(),
			Lock:        ar.Lock().String(),
		}
	}
	return result
}

// FindArea finds an area by name.
func (a *ServerAdapter) FindArea(name string) *bot.AreaInfo {
	name = strings.ToLower(name)
	for i, ar := range areas {
		if strings.EqualFold(ar.Name(), name) {
			return &bot.AreaInfo{
				Index:       i,
				Name:        ar.Name(),
				PlayerCount: ar.PlayerCount(),
				Status:      ar.Status().String(),
				Lock:        ar.Lock().String(),
			}
		}
	}
	return nil
}

// MutePlayer mutes a player by UID.
func (a *ServerAdapter) MutePlayer(uid int, duration time.Duration, reason string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	c.SetMuted(ICOOCMuted)
	if duration > 0 {
		c.SetUnmuteTime(time.Now().UTC().Add(duration))
	} else {
		c.SetUnmuteTime(time.Time{})
	}
	c.SendServerMessage(fmt.Sprintf("You have been muted. Reason: %s", reason))
	return nil
}

// UnmutePlayer unmutes a player by UID.
func (a *ServerAdapter) UnmutePlayer(uid int) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	c.SetMuted(Unmuted)
	c.SendServerMessage("You have been unmuted.")
	return nil
}

// KickPlayer kicks a player by UID.
func (a *ServerAdapter) KickPlayer(uid int, reason string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	c.SendServerMessage(fmt.Sprintf("You have been kicked. Reason: %s", reason))
	c.conn.Close()
	return nil
}

// BanPlayer bans a player by IPID.
func (a *ServerAdapter) BanPlayer(ipid string, duration time.Duration, reason string, moderator string) error {
	var durUnix int64
	if duration <= 0 {
		durUnix = -1 // Permanent
	} else {
		durUnix = time.Now().UTC().Add(duration).Unix()
	}
	_, err := db.AddBan(ipid, "", time.Now().UTC().Unix(), durUnix, reason, moderator)
	if err != nil {
		return fmt.Errorf("failed to add ban: %w", err)
	}
	// Kick all clients with this IPID.
	for _, c := range getClientsByIpid(ipid) {
		c.SendServerMessage(fmt.Sprintf("You have been banned. Reason: %s", reason))
		c.conn.Close()
	}
	logger.WriteAudit(fmt.Sprintf("%v | BAN | IPID:%v | %v | By: %v", time.Now().UTC().Format("15:04:05"), ipid, reason, moderator))
	return nil
}

// GagPlayer mutes a player from IC chat.
func (a *ServerAdapter) GagPlayer(uid int) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	c.SetMuted(ICMuted)
	c.SetUnmuteTime(time.Time{})
	c.SendServerMessage("You have been gagged from IC chat.")
	return nil
}

// UngagPlayer removes IC mute from a player.
func (a *ServerAdapter) UngagPlayer(uid int) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	if c.Muted() == ICMuted {
		c.SetMuted(Unmuted)
	}
	c.SendServerMessage("Your gag has been removed.")
	return nil
}

// WarnPlayer issues a warning to a player (stored in memory keyed by IPID).
func (a *ServerAdapter) WarnPlayer(uid int, reason string, moderator string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	warningsMu.Lock()
	warnings[c.Ipid()] = append(warnings[c.Ipid()], bot.WarnRecord{
		Reason:    reason,
		Moderator: moderator,
		Time:      time.Now().UTC().Unix(),
	})
	warningsMu.Unlock()
	c.SendServerMessage(fmt.Sprintf("⚠️ Warning from moderator: %s", reason))
	logger.WriteAudit(fmt.Sprintf("%v | WARN | IPID:%v | %v | By: %v", time.Now().UTC().Format("15:04:05"), c.Ipid(), reason, moderator))
	return nil
}

// GetWarnings returns all warnings for a given IPID.
func (a *ServerAdapter) GetWarnings(ipid string) []bot.WarnRecord {
	warningsMu.RLock()
	defer warningsMu.RUnlock()
	return append([]bot.WarnRecord(nil), warnings[ipid]...)
}

// GetBanList returns all bans from the database.
func (a *ServerAdapter) GetBanList() []bot.BanRecord {
	bans, err := db.GetAllBans()
	if err != nil {
		return nil
	}
	result := make([]bot.BanRecord, len(bans))
	for i, b := range bans {
		result[i] = bot.BanRecord{
			ID:        b.Id,
			IPID:      b.Ipid,
			HDID:      b.Hdid,
			Reason:    b.Reason,
			Duration:  b.Duration,
			Moderator: b.Moderator,
			Time:      b.Time,
		}
	}
	return result
}

// UnbanByID removes a ban by its ID.
func (a *ServerAdapter) UnbanByID(id int) error {
	return db.UnBan(id)
}

// ApplyPunishment applies a named punishment to a player.
func (a *ServerAdapter) ApplyPunishment(uid int, punishmentName string, duration time.Duration) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	// Map punishment name to PunishmentType.
	pType := punishmentNameToType(punishmentName)
	if pType == PunishmentNone {
		return fmt.Errorf("unknown punishment: %s", punishmentName)
	}
	c.AddPunishment(pType, duration, "Applied by Discord moderator.")
	c.SendServerMessage(fmt.Sprintf("You have received the '%s' punishment.", punishmentName))
	return nil
}

// RemovePunishment removes a named punishment from a player.
func (a *ServerAdapter) RemovePunishment(uid int, punishmentName string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	pType := punishmentNameToType(punishmentName)
	if pType == PunishmentNone {
		return fmt.Errorf("unknown punishment: %s", punishmentName)
	}
	c.RemovePunishment(pType)
	return nil
}

// punishmentNameToType converts a string name to a PunishmentType.
func punishmentNameToType(name string) PunishmentType {
	switch strings.ToLower(name) {
	case "parrot":
		// The "parrot" Discord command applies a random text punishment effect.
		// The mute-based parrot behaviour (/parrot in-game) uses a separate MuteState
		// and is not directly accessible here. PunishmentRng is used as the closest
		// text-transformation equivalent for Discord-initiated parrot punishments.
		return PunishmentRng
	case "drunk":
		return PunishmentDrunk
	case "slowpoke":
		return PunishmentSlowpoke
	case "roulette":
		return PunishmentRoulette
	case "spotlight":
		return PunishmentSpotlight
	case "whisper":
		return PunishmentWhisper
	case "stutterstep":
		return PunishmentStutterstep
	case "backward":
		return PunishmentBackward
	}
	return PunishmentNone
}

// SendPrivateMessage sends a server message to a specific player.
func (a *ServerAdapter) SendPrivateMessage(uid int, message string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	c.SendServerMessage("[Discord Mod] " + message)
	return nil
}

// SendAnnouncement sends a message to all connected players.
func (a *ServerAdapter) SendAnnouncement(message string) error {
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 {
			continue
		}
		c.SendServerMessage("[Announcement] " + message)
	}
	return nil
}

// SendAnnouncementToPlayer sends a message to a specific player.
func (a *ServerAdapter) SendAnnouncementToPlayer(uid int, message string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	c.SendServerMessage("[Announcement] " + message)
	return nil
}

// ForceMove moves a player to an area by name.
func (a *ServerAdapter) ForceMove(uid int, areaName string) error {
	c, err := getClientByUid(uid)
	if err != nil {
		return fmt.Errorf("player not found: UID %d", uid)
	}
	for _, ar := range areas {
		if strings.EqualFold(ar.Name(), areaName) {
			if !c.ChangeArea(ar) {
				return fmt.Errorf("could not move player to %s (area may be locked)", areaName)
			}
			c.SendServerMessage(fmt.Sprintf("You were moved to %s by a moderator.", ar.Name()))
			return nil
		}
	}
	return fmt.Errorf("area not found: %s", areaName)
}

// ClearArea moves all players out of a named area to area 0.
func (a *ServerAdapter) ClearArea(areaName string) error {
	var target *area.Area
	for _, ar := range areas {
		if strings.EqualFold(ar.Name(), areaName) {
			target = ar
			break
		}
	}
	if target == nil {
		return fmt.Errorf("area not found: %s", areaName)
	}
	if len(areas) == 0 {
		return fmt.Errorf("no areas configured")
	}
	lobby := areas[0]
	if target == lobby {
		return fmt.Errorf("cannot clear the default area")
	}
	for c := range clients.GetAllClients() {
		if c.Uid() != -1 && c.Area() == target {
			c.ChangeArea(lobby)
			c.SendServerMessage(fmt.Sprintf("You were moved out of %s by a moderator.", areaName))
		}
	}
	return nil
}

// LockArea locks a named area.
func (a *ServerAdapter) LockArea(areaName string) error {
	for _, ar := range areas {
		if strings.EqualFold(ar.Name(), areaName) {
			ar.SetLock(area.LockLocked)
			// Invite all current players.
			for c := range clients.GetAllClients() {
				if c.Uid() != -1 && c.Area() == ar {
					ar.AddInvited(c.Uid())
				}
			}
			sendAreaServerMessage(ar, fmt.Sprintf("%s was locked by a Discord moderator.", ar.Name()))
			sendLockArup()
			return nil
		}
	}
	return fmt.Errorf("area not found: %s", areaName)
}

// UnlockArea unlocks a named area.
func (a *ServerAdapter) UnlockArea(areaName string) error {
	for _, ar := range areas {
		if strings.EqualFold(ar.Name(), areaName) {
			if ar.Lock() == area.LockFree {
				return fmt.Errorf("area %s is not locked", areaName)
			}
			ar.SetLock(area.LockFree)
			ar.ClearInvited()
			sendAreaServerMessage(ar, fmt.Sprintf("%s was unlocked by a Discord moderator.", ar.Name()))
			sendLockArup()
			return nil
		}
	}
	return fmt.Errorf("area not found: %s", areaName)
}

// GetPlayerLogs returns the area buffer log entries for the area the player is currently in,
// filtered to lines containing the player's IPID.
func (a *ServerAdapter) GetPlayerLogs(ipid string) []string {
	var result []string
	for _, ar := range areas {
		for _, line := range ar.Buffer() {
			if strings.Contains(line, ipid) {
				result = append(result, line)
			}
		}
	}
	return result
}

// GetAuditLog returns the last N lines of the audit log, optionally filtered by a search string.
func (a *ServerAdapter) GetAuditLog(filter string) []string {
	auditPath := logger.LogPath + "/audit.log"
	f, err := os.Open(auditPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if filter == "" || strings.Contains(strings.ToLower(line), strings.ToLower(filter)) {
			lines = append(lines, line)
		}
	}

	// Return the last 50 matching lines.
	const maxLines = 50
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

// GetServerName returns the server's name.
func (a *ServerAdapter) GetServerName() string {
	return config.Name
}

// GetPlayerCount returns the current connected player count.
func (a *ServerAdapter) GetPlayerCount() int {
	return players.GetPlayerCount()
}

// GetMaxPlayers returns the server's max player count.
func (a *ServerAdapter) GetMaxPlayers() int {
	return config.MaxPlayers
}
