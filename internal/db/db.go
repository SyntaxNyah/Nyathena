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

package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type BanInfo struct {
	Id        int
	Ipid      string
	Hdid      string
	Time      int64
	Duration  int64
	Reason    string
	Moderator string
}

type BanLookup int

const (
	IPID BanLookup = iota
	HDID
	BANID
)

var DBPath string
var db *sql.DB

// Database version.
// This should be incremented whenever changes are made to the DB that require existing databases to upgrade.
const ver = 3

// Persistent punishment kind constants.
const (
	PunishKindMute = 0 // Mute/parrot; VALUE holds the MuteState integer.
	PunishKindJail = 1 // Jail; VALUE unused (0).
	PunishKindText = 2 // Text/behaviour punishment; SUBTYPE holds the PunishmentType integer.
)

// PersistentPunishment holds one row from the PUNISHMENTS table.
type PersistentPunishment struct {
	Kind    int
	Subtype int   // 0 for mute/jail; PunishmentType for text punishments.
	Value   int   // MuteState for mutes; 0 for others.
	Expires int64 // Unix timestamp; 0 = no expiry (permanent).
	Reason  string
}

// Opens the server's database connection.
func Open() error {
	var err error
	db, err = sql.Open("sqlite", DBPath)
	if err != nil {
		return err
	}
	// SQLite performs best with a single writer connection.
	db.SetMaxOpenConns(1)

	// Performance pragmas: WAL journal is faster for mixed read/write workloads;
	// synchronous=NORMAL is safe with WAL and reduces fsync overhead;
	// cache_size=-2000 allocates ~2 MB of page cache.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-2000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}

	var v int
	r := db.QueryRow("PRAGMA user_version")
	r.Scan(&v)
	if v < ver {
		err := upgradeDB(v)
		if err != nil {
			return err
		}
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS BANS(ID INTEGER PRIMARY KEY, IPID TEXT, HDID TEXT, TIME INTEGER, DURATION INTEGER, REASON TEXT, MODERATOR TEXT)")
	if err != nil {
		return err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS USERS(USERNAME TEXT PRIMARY KEY, PASSWORD TEXT, PERMISSIONS TEXT)")
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS PUNISHMENTS(
		IPID    TEXT    NOT NULL,
		KIND    INTEGER NOT NULL,
		SUBTYPE INTEGER NOT NULL DEFAULT 0,
		VALUE   INTEGER NOT NULL DEFAULT 0,
		EXPIRES INTEGER NOT NULL DEFAULT 0,
		REASON  TEXT    NOT NULL DEFAULT '',
		UNIQUE(IPID, KIND, SUBTYPE)
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS KNOWN_IPS(
		IPID       TEXT    PRIMARY KEY,
		FIRST_SEEN INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}
	return nil
}

// upgradeDB upgrades the server's database to the latest version.
func upgradeDB(v int) error {
	switch v {
	case 0:
		_, err := db.Exec("PRAGMA user_version = 1")
		if err != nil {
			return err
		}
		fallthrough
	case 1:
		_, err := db.Exec("PRAGMA user_version = 2")
		if err != nil {
			return err
		}
		fallthrough
	case 2:
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS KNOWN_IPS(
			IPID       TEXT    PRIMARY KEY,
			FIRST_SEEN INTEGER NOT NULL DEFAULT 0
		)`)
		if err != nil {
			return err
		}
		_, err = db.Exec("PRAGMA user_version = 3")
		if err != nil {
			return err
		}
	}
	return nil
}

// UserExists returns whether a user exists within the server's database.
func UserExists(username string) bool {
	result := db.QueryRow("SELECT USERNAME FROM USERS WHERE USERNAME = ?", username)
	if result.Scan() == sql.ErrNoRows {
		return false
	} else {
		return true
	}
}

// CreateUser adds a new user to the server's database.
func CreateUser(username string, password []byte, permissions uint64) error {
	hashed, err := bcrypt.GenerateFromPassword(password, 12)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO USERS VALUES(?, ?, ?)", username, hashed, strconv.FormatUint(permissions, 10))
	if err != nil {
		return err
	}
	return nil
}

// RemoveUser deletes a user from the server's database.
func RemoveUser(username string) error {
	_, err := db.Exec("DELETE FROM USERS WHERE USERNAME = ?", username)
	if err != nil {
		return err
	}
	return nil
}

// AuthenticateUser returns whether or not the user's credentials match those in the database, and that user's permissions.
func AuthenticateUser(username string, password []byte) (bool, uint64) {
	var rpass, rperms string
	result := db.QueryRow("SELECT PASSWORD, PERMISSIONS FROM USERS WHERE USERNAME = ?", username)
	result.Scan(&rpass, &rperms)
	err := bcrypt.CompareHashAndPassword([]byte(rpass), password)
	if err != nil {
		return false, 0
	}
	p, err := strconv.ParseUint(rperms, 10, 64)
	if err != nil {
		return false, 0
	}
	return true, p
}

// ChangePermissions updated the permissions of a user in the database.
func ChangePermissions(username string, permissions uint64) error {
	_, err := db.Exec("UPDATE USERS SET PERMISSIONS = ? WHERE USERNAME = ?", strconv.FormatUint(permissions, 10), username)
	if err != nil {
		return err
	}
	return nil
}

// AddBan adds a new ban to the database.
func AddBan(ipid string, hdid string, time int64, duration int64, reason string, moderator string) (int, error) {
	result, err := db.Exec("INSERT INTO BANS VALUES(NULL, ?, ?, ?, ?, ?, ?)", ipid, hdid, time, duration, reason, moderator)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// UnBan nullifies a ban in the database.
func UnBan(id int) error {
	_, err := db.Exec("UPDATE BANS SET DURATION = 0 WHERE ID = ?", id)
	if err != nil {
		return err
	}
	return nil
}

// GetBan returns a list of bans matching a given value.
func GetBan(by BanLookup, value any) ([]BanInfo, error) {
	var stmt *sql.Stmt
	var err error
	switch by {
	case BANID:
		stmt, err = db.Prepare("SELECT * FROM BANS WHERE ID = ?")
	case IPID:
		stmt, err = db.Prepare("SELECT * FROM BANS WHERE IPID = ? ORDER BY TIME DESC")
	}
	if err != nil {
		return []BanInfo{}, err
	}
	result, err := stmt.Query(value)
	if err != nil {
		return []BanInfo{}, err
	}
	stmt.Close()
	defer result.Close()
	var bans []BanInfo
	for result.Next() {
		var b BanInfo
		result.Scan(&b.Id, &b.Ipid, &b.Hdid, &b.Time, &b.Duration, &b.Reason, &b.Moderator)
		bans = append(bans, b)
	}
	return bans, nil
}

// GetRecentBans returns the 5 most recent bans.
func GetRecentBans() ([]BanInfo, error) {
	result, err := db.Query("SELECT * FROM BANS ORDER BY TIME DESC LIMIT 5")
	if err != nil {
		return []BanInfo{}, err
	}
	defer result.Close()
	var bans []BanInfo
	for result.Next() {
		var b BanInfo
		result.Scan(&b.Id, &b.Ipid, &b.Hdid, &b.Time, &b.Duration, &b.Reason, &b.Moderator)
		bans = append(bans, b)
	}
	return bans, nil
}

// IsBanned returns whether the given ipid/hdid is banned, and the info of the ban.
func IsBanned(by BanLookup, value string) (bool, BanInfo, error) {
	var stmt *sql.Stmt
	var err error
	switch by {
	case IPID:
		stmt, err = db.Prepare("SELECT ID, DURATION, REASON FROM BANS WHERE IPID = ?")
	case HDID:
		stmt, err = db.Prepare("SELECT ID, DURATION, REASON FROM BANS WHERE HDID = ?")
	}
	if err != nil {
		return false, BanInfo{}, err
	}
	result, err := stmt.Query(value)
	if err != nil {
		return false, BanInfo{}, err
	}
	stmt.Close()
	defer result.Close()
	for result.Next() {
		var (
			duration int64
			id       int
			reason   string
		)
		result.Scan(&id, &duration, &reason)
		if duration == -1 || time.Unix(duration, 0).UTC().After(time.Now().UTC()) {
			return true, BanInfo{Id: id, Duration: duration, Reason: reason}, nil
		}
	}
	return false, BanInfo{}, nil
}

// UpdateReason updates the reason of a ban.
func UpdateReason(id int, reason string) error {
	_, err := db.Exec("UPDATE BANS SET REASON = ? WHERE ID = ?", reason, id)
	if err != nil {
		return err
	}
	return nil
}

// UpdateDuration updates the duration of a ban.
func UpdateDuration(id int, duration int64) error {
	_, err := db.Exec("UPDATE BANS SET DURATION = ? WHERE ID = ?", duration, id)
	if err != nil {
		return err
	}
	return nil
}

// Closes the server's database connection.
func Close() {
	db.Close()
}

// UpsertMute stores (or replaces) the mute state for an IPID.
// muteType is the MuteState integer value. expires is a Unix timestamp (0 = permanent).
func UpsertMute(ipid string, muteType int, expires int64) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO PUNISHMENTS(IPID, KIND, SUBTYPE, VALUE, EXPIRES, REASON) VALUES(?, ?, 0, ?, ?, '')",
		ipid, PunishKindMute, muteType, expires)
	return err
}

// DeleteMute removes any stored mute for an IPID.
func DeleteMute(ipid string) error {
	_, err := db.Exec("DELETE FROM PUNISHMENTS WHERE IPID = ? AND KIND = ?", ipid, PunishKindMute)
	return err
}

// UpsertJail stores (or replaces) the jail state for an IPID.
// expires is a Unix timestamp of when the jail ends.
func UpsertJail(ipid string, expires int64, reason string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO PUNISHMENTS(IPID, KIND, SUBTYPE, VALUE, EXPIRES, REASON) VALUES(?, ?, 0, 0, ?, ?)",
		ipid, PunishKindJail, expires, reason)
	return err
}

// DeleteJail removes any stored jail for an IPID.
func DeleteJail(ipid string) error {
	_, err := db.Exec("DELETE FROM PUNISHMENTS WHERE IPID = ? AND KIND = ?", ipid, PunishKindJail)
	return err
}

// UpsertTextPunishment stores (or replaces) a text/behaviour punishment for an IPID.
// pType is the PunishmentType integer. expires is a Unix timestamp (0 = permanent).
func UpsertTextPunishment(ipid string, pType int, expires int64, reason string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO PUNISHMENTS(IPID, KIND, SUBTYPE, VALUE, EXPIRES, REASON) VALUES(?, ?, ?, 0, ?, ?)",
		ipid, PunishKindText, pType, expires, reason)
	return err
}

// DeleteTextPunishment removes a specific text punishment for an IPID.
func DeleteTextPunishment(ipid string, pType int) error {
	_, err := db.Exec("DELETE FROM PUNISHMENTS WHERE IPID = ? AND KIND = ? AND SUBTYPE = ?",
		ipid, PunishKindText, pType)
	return err
}

// DeleteAllPunishments removes ALL stored punishments (mute, jail, and text) for an IPID.
func DeleteAllPunishments(ipid string) error {
	_, err := db.Exec("DELETE FROM PUNISHMENTS WHERE IPID = ?", ipid)
	return err
}

// GetPunishments returns all currently active persistent punishments for an IPID.
// Expired entries are filtered in the query rather than deleted, keeping this a pure read.
// Call PurgeExpired periodically to reclaim space from old expired rows.
func GetPunishments(ipid string) ([]PersistentPunishment, error) {
	now := time.Now().Unix()
	rows, err := db.Query(
		"SELECT KIND, SUBTYPE, VALUE, EXPIRES, REASON FROM PUNISHMENTS WHERE IPID = ? AND (EXPIRES = 0 OR EXPIRES > ?)",
		ipid, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var punishments []PersistentPunishment
	for rows.Next() {
		var p PersistentPunishment
		if err := rows.Scan(&p.Kind, &p.Subtype, &p.Value, &p.Expires, &p.Reason); err != nil {
			continue
		}
		punishments = append(punishments, p)
	}
	return punishments, nil
}

// PurgeExpired deletes all expired punishment rows from the database.
// It is safe to call from a background goroutine.
func PurgeExpired() error {
	_, err := db.Exec("DELETE FROM PUNISHMENTS WHERE EXPIRES != 0 AND EXPIRES <= ?", time.Now().Unix())
	return err
}

// GetAllBans returns all bans from the database.
func GetAllBans() ([]BanInfo, error) {
	result, err := db.Query("SELECT * FROM BANS ORDER BY TIME DESC")
	if err != nil {
		return []BanInfo{}, err
	}
	defer result.Close()
	var bans []BanInfo
	for result.Next() {
		var b BanInfo
		if err := result.Scan(&b.Id, &b.Ipid, &b.Hdid, &b.Time, &b.Duration, &b.Reason, &b.Moderator); err != nil {
			continue
		}
		bans = append(bans, b)
	}
	return bans, nil
}

// MarkIPKnown records an IPID as known (i.e. it has connected to the server at least once).
// If the IPID is already present, this is a no-op (INSERT OR IGNORE).
func MarkIPKnown(ipid string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec("INSERT OR IGNORE INTO KNOWN_IPS(IPID, FIRST_SEEN) VALUES(?, ?)", ipid, time.Now().Unix())
	return err
}

// LoadKnownIPs returns every IPID that has previously been recorded by MarkIPKnown.
// It is called once at server startup to pre-populate the in-memory first-seen tracker.
func LoadKnownIPs() ([]string, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.Query("SELECT IPID FROM KNOWN_IPS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ipids []string
	for rows.Next() {
		var ipid string
		if err := rows.Scan(&ipid); err != nil {
			return ipids, fmt.Errorf("LoadKnownIPs scan: %w", err)
		}
		ipids = append(ipids, ipid)
	}
	return ipids, rows.Err()
}
