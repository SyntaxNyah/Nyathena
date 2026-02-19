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

package bot

import "time"

// PlayerInfo holds information about a connected player.
type PlayerInfo struct {
	UID       int
	Character string
	OOCName   string
	Area      string
	IPID      string
}

// AreaInfo holds information about a server area.
type AreaInfo struct {
	Index       int
	Name        string
	PlayerCount int
	Status      string
	Lock        string
}

// BanRecord holds information about a ban entry.
type BanRecord struct {
	ID        int
	IPID      string
	HDID      string
	Reason    string
	Duration  int64
	Moderator string
	Time      int64
}

// WarnRecord holds information about a warning entry.
type WarnRecord struct {
	Reason    string
	Moderator string
	Time      int64
}

// ServerInterface defines the operations the Discord bot can perform on the AO2 server.
// This interface decouples the bot package from the athena package.
type ServerInterface interface {
	// Player queries
	GetPlayers() []PlayerInfo
	FindPlayer(name string) *PlayerInfo
	GetPlayerByUID(uid int) *PlayerInfo

	// Area queries
	GetAreas() []AreaInfo
	FindArea(name string) *AreaInfo

	// Moderation actions
	MutePlayer(uid int, duration time.Duration, reason string) error
	UnmutePlayer(uid int) error
	KickPlayer(uid int, reason string) error
	BanPlayer(ipid string, duration time.Duration, reason string, moderator string) error
	GagPlayer(uid int) error
	UngagPlayer(uid int) error
	WarnPlayer(uid int, reason string, moderator string) error
	GetWarnings(ipid string) []WarnRecord
	GetBanList() []BanRecord
	UnbanByID(id int) error

	// Punishment actions
	ApplyPunishment(uid int, punishmentName string, duration time.Duration) error
	RemovePunishment(uid int, punishmentName string) error

	// Communication
	SendPrivateMessage(uid int, message string) error
	SendAnnouncement(message string) error
	SendAnnouncementToPlayer(uid int, message string) error

	// Area control
	ForceMove(uid int, areaName string) error
	ClearArea(areaName string) error
	LockArea(areaName string) error
	UnlockArea(areaName string) error

	// Audit & Logs
	GetPlayerLogs(ipid string) []string
	GetAuditLog(filter string) []string

	// Server stats
	GetServerName() string
	GetPlayerCount() int
	GetMaxPlayers() int
}
