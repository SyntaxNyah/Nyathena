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
	"strings"
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

// defaultChipBalance is the starting chip balance assigned to new players.
const defaultChipBalance = 100

// MaxChipBalance is the hard upper limit on any player's chip balance.
// AddChips will silently clamp the result to this value, preventing runaway
// inflation across all casino games.
const MaxChipBalance = 10_000_000

// Database version.
// This should be incremented whenever changes are made to the DB that require existing databases to upgrade.
const ver = 8

// Persistent punishment kind constants.
const (
	PunishKindMute      = 0 // Mute/parrot; VALUE holds the MuteState integer.
	PunishKindJail      = 1 // Jail; VALUE unused (0).
	PunishKindText      = 2 // Text/behaviour punishment; SUBTYPE holds the PunishmentType integer.
	PunishKindCharStuck = 3 // Char-stuck; VALUE holds the locked character ID.
)

// PersistentPunishment holds one row from the PUNISHMENTS table.
type PersistentPunishment struct {
	Kind    int
	Subtype int   // 0 for mute/jail; PunishmentType for text punishments.
	Value   int   // MuteState for mutes; 0 for others.
	Expires int64 // Unix timestamp; 0 = no expiry (permanent).
	Reason  string
}

// ChipEntry holds one row from the CHIPS leaderboard query.
type ChipEntry struct {
	Ipid    string
	Balance int64
}

// PlaytimeEntry holds one row from the playtime leaderboard query.
type PlaytimeEntry struct {
	Ipid     string
	Username string // empty when no account is linked
	Playtime int64  // seconds
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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS USERS(USERNAME TEXT PRIMARY KEY, PASSWORD TEXT, PERMISSIONS TEXT, IPID TEXT NOT NULL DEFAULT '')")
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
		FIRST_SEEN INTEGER NOT NULL DEFAULT 0,
		LAST_SEEN  INTEGER NOT NULL DEFAULT 0,
		PLAYTIME   INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS TORMENTED_IPS(
		IPID TEXT PRIMARY KEY
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS CHIPS(
		IPID    TEXT    PRIMARY KEY,
		BALANCE INTEGER NOT NULL DEFAULT 100
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
		fallthrough
	case 3:
		_, err := db.Exec("ALTER TABLE KNOWN_IPS ADD COLUMN LAST_SEEN INTEGER NOT NULL DEFAULT 0")
		if err != nil {
			return err
		}
		// Initialise LAST_SEEN to FIRST_SEEN for existing rows so that currently
		// active players are not immediately pruned after upgrading.
		_, err = db.Exec("UPDATE KNOWN_IPS SET LAST_SEEN = FIRST_SEEN WHERE LAST_SEEN = 0")
		if err != nil {
			return err
		}
		_, err = db.Exec("PRAGMA user_version = 4")
		if err != nil {
			return err
		}
		fallthrough
	case 4:
		_, err := db.Exec("ALTER TABLE KNOWN_IPS ADD COLUMN PLAYTIME INTEGER NOT NULL DEFAULT 0")
		if err != nil {
			return err
		}
		_, err = db.Exec("PRAGMA user_version = 5")
		if err != nil {
			return err
		}
		fallthrough
	case 5:
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS TORMENTED_IPS(
			IPID TEXT PRIMARY KEY
		)`)
		if err != nil {
			return err
		}
		_, err = db.Exec("PRAGMA user_version = 6")
		if err != nil {
			return err
		}
		fallthrough
	case 6:
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS CHIPS(
			IPID    TEXT    PRIMARY KEY,
			BALANCE INTEGER NOT NULL DEFAULT 100
		)`)
		if err != nil {
			return err
		}
		_, err = db.Exec("PRAGMA user_version = 7")
		if err != nil {
			return err
		}
		fallthrough
	case 7:
		// Add IPID column to USERS so player accounts can be linked to their connection fingerprint.
		// Only alter the table when it already exists (i.e. we are upgrading an existing database).
		// Brand-new databases get the column from the CREATE TABLE statement in Open().
		var usersExists int
		db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='USERS'").Scan(&usersExists) //nolint:errcheck
		if usersExists > 0 {
			_, err := db.Exec("ALTER TABLE USERS ADD COLUMN IPID TEXT NOT NULL DEFAULT ''")
			if err != nil {
				return err
			}
		}
		_, err := db.Exec("PRAGMA user_version = 8")
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
// This creates a moderator/admin account with the given permissions.
// The IPID field is left empty and must be linked on first login via LinkIPIDToUser.
func CreateUser(username string, password []byte, permissions uint64) error {
	hashed, err := bcrypt.GenerateFromPassword(password, 12)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO USERS(USERNAME, PASSWORD, PERMISSIONS) VALUES(?, ?, ?)", username, hashed, strconv.FormatUint(permissions, 10))
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

// RegisterPlayer creates a player (non-moderator) account with zero permissions
// and records the player's IPID so it can be looked up later.
// Returns an error if the username is already taken.
func RegisterPlayer(username string, password []byte, ipid string) error {
	hashed, err := bcrypt.GenerateFromPassword(password, 12)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO USERS(USERNAME, PASSWORD, PERMISSIONS, IPID) VALUES(?, ?, '0', ?)", username, hashed, ipid)
	return err
}

// RegisterPlayerHashed is like RegisterPlayer but accepts an already-hashed
// bcrypt password. Use this when the password was hashed at an earlier step
// (e.g. before being stored in a pending-registration state) to avoid keeping
// a plaintext password in memory longer than necessary.
func RegisterPlayerHashed(username string, hashedPassword []byte, ipid string) error {
	_, err := db.Exec("INSERT INTO USERS(USERNAME, PASSWORD, PERMISSIONS, IPID) VALUES(?, ?, '0', ?)", username, hashedPassword, ipid)
	return err
}

// LinkIPIDToUser associates an IPID with a user account.
// Called on every successful login so the leaderboard can show account names.
// When the player's IPID has changed (e.g. their IP address changed), any
// playtime accumulated under the old IPID is merged into the new IPID so the
// leaderboard continues to display the correct total under the player's account name.
func LinkIPIDToUser(username, ipid string) error {
	if db == nil {
		return nil
	}

	// Retrieve the IPID currently stored for this account.
	var oldIPID string
	switch err := db.QueryRow("SELECT COALESCE(IPID, '') FROM USERS WHERE USERNAME = ?", username).Scan(&oldIPID); {
	case err == sql.ErrNoRows:
		// User not found — the UPDATE below will be a no-op.
	case err != nil:
		return err
	}

	// Update the USERS table with the new IPID.
	if _, err := db.Exec("UPDATE USERS SET IPID = ? WHERE USERNAME = ?", ipid, username); err != nil {
		return err
	}

	// Nothing more to do if the IPID hasn't changed or was previously unset.
	if oldIPID == "" || oldIPID == ipid {
		return nil
	}

	// Fetch the playtime accumulated under the old IPID.
	var oldPlaytime int64
	switch err := db.QueryRow("SELECT COALESCE(PLAYTIME, 0) FROM KNOWN_IPS WHERE IPID = ?", oldIPID).Scan(&oldPlaytime); {
	case err == sql.ErrNoRows:
		// No playtime recorded for old IPID — nothing to transfer.
		return nil
	case err != nil:
		return err
	}
	if oldPlaytime == 0 {
		return nil
	}

	// Merge old playtime into the new IPID. UPSERT ensures the row is created
	// if it does not yet exist in KNOWN_IPS (defensive; in practice MarkIPKnown
	// is called on every connection before /login can be issued).
	if _, err := db.Exec(`
		INSERT INTO KNOWN_IPS (IPID, FIRST_SEEN, LAST_SEEN, PLAYTIME)
		VALUES (?, 0, 0, ?)
		ON CONFLICT(IPID) DO UPDATE SET PLAYTIME = PLAYTIME + excluded.PLAYTIME`,
		ipid, oldPlaytime); err != nil {
		return err
	}
	_, err := db.Exec("UPDATE KNOWN_IPS SET PLAYTIME = 0 WHERE IPID = ?", oldIPID)
	return err
}

// GetUsernameByIPID returns the username whose account is linked to the given IPID.
// Returns ("", nil) when no account is associated with that IPID.
func GetUsernameByIPID(ipid string) (string, error) {
	if db == nil {
		return "", nil
	}
	row := db.QueryRow("SELECT USERNAME FROM USERS WHERE IPID = ?", ipid)
	var username string
	if err := row.Scan(&username); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return username, nil
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
// areaID is the area index the player is jailed in; use -1 for "current area" (no forced move on reconnect).
func UpsertJail(ipid string, expires int64, reason string, areaID int) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO PUNISHMENTS(IPID, KIND, SUBTYPE, VALUE, EXPIRES, REASON) VALUES(?, ?, 0, ?, ?, ?)",
		ipid, PunishKindJail, areaID, expires, reason)
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

// UpsertCharStuck stores (or replaces) a character-stuck record for an IPID.
// charID is the character the player is locked to. expires is a Unix timestamp (0 = permanent).
func UpsertCharStuck(ipid string, charID int, expires int64, reason string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO PUNISHMENTS(IPID, KIND, SUBTYPE, VALUE, EXPIRES, REASON) VALUES(?, ?, 0, ?, ?, ?)",
		ipid, PunishKindCharStuck, charID, expires, reason)
	return err
}

// DeleteCharStuck removes any stored char-stuck record for an IPID.
func DeleteCharStuck(ipid string) error {
	_, err := db.Exec("DELETE FROM PUNISHMENTS WHERE IPID = ? AND KIND = ?", ipid, PunishKindCharStuck)
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

// MarkIPKnown records an IPID as known and updates its last-seen timestamp.
// For new IPIDs both FIRST_SEEN and LAST_SEEN are set to now.
// For IPIDs already in the table FIRST_SEEN is preserved and only LAST_SEEN is updated.
func MarkIPKnown(ipid string) error {
	if db == nil {
		return nil
	}
	now := time.Now().Unix()
	_, err := db.Exec(
		`INSERT INTO KNOWN_IPS(IPID, FIRST_SEEN, LAST_SEEN) VALUES(?, ?, ?)
		 ON CONFLICT(IPID) DO UPDATE SET LAST_SEEN = excluded.LAST_SEEN`,
		ipid, now, now)
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

// RemoveKnownIP deletes an IPID from the KNOWN_IPS table.
// It is called when an IP is banned so that, once the ban expires, the IP is
// treated as new again (subject to new-connection cooldowns and rate limits).
func RemoveKnownIP(ipid string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec("DELETE FROM KNOWN_IPS WHERE IPID = ?", ipid)
	return err
}

// PruneInactiveIPs deletes all KNOWN_IPS rows whose LAST_SEEN timestamp is
// older than the provided Unix timestamp. It returns the number of rows removed.
// Call this at startup (after loading known IPs) to keep the table lean.
func PruneInactiveIPs(before int64) (int64, error) {
	if db == nil {
		return 0, nil
	}
	res, err := db.Exec("DELETE FROM KNOWN_IPS WHERE LAST_SEEN < ? AND LAST_SEEN != 0", before)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// AddPlaytime increments the PLAYTIME counter for an IPID by the given number of seconds.
// It is called when a client disconnects to accumulate their session duration.
func AddPlaytime(ipid string, seconds int64) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec("UPDATE KNOWN_IPS SET PLAYTIME = PLAYTIME + ? WHERE IPID = ?", seconds, ipid)
	return err
}

// AddPlaytimeReturning atomically increments the PLAYTIME counter for an IPID by
// the given number of seconds and returns the new accumulated total.
// Using a single RETURNING statement avoids the read-then-write race that arises
// when two sessions for the same IPID disconnect simultaneously.
// Returns (0, nil) when the database is not initialised.
func AddPlaytimeReturning(ipid string, seconds int64) (int64, error) {
	if db == nil {
		return 0, nil
	}
	row := db.QueryRow(
		"UPDATE KNOWN_IPS SET PLAYTIME = PLAYTIME + ? WHERE IPID = ? RETURNING PLAYTIME",
		seconds, ipid)
	var newTotal int64
	if err := row.Scan(&newTotal); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return newTotal, nil
}

// PruneShortPlaytimeIPs deletes all KNOWN_IPS rows whose accumulated PLAYTIME is
// less than minSeconds. IPs that have played for at least minSeconds are retained.
// It returns the number of rows removed.
func PruneShortPlaytimeIPs(minSeconds int64) (int64, error) {
	if db == nil {
		return 0, nil
	}
	res, err := db.Exec("DELETE FROM KNOWN_IPS WHERE PLAYTIME < ?", minSeconds)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetPlaytime returns the accumulated playtime in seconds for an IPID from the KNOWN_IPS table.
// Returns 0, nil if the IPID is not found.
func GetPlaytime(ipid string) (int64, error) {
	if db == nil {
		return 0, nil
	}
	row := db.QueryRow("SELECT PLAYTIME FROM KNOWN_IPS WHERE IPID = ?", ipid)
	var playtime int64
	if err := row.Scan(&playtime); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return playtime, nil
}

// GetAccountStats returns the chip balance and accumulated playtime for an IPID
// in a single query, avoiding two separate database round-trips.
// Returns (0, 0, nil) when the database is not initialised.
func GetAccountStats(ipid string) (chips, playtime int64, err error) {
	if db == nil {
		return 0, 0, nil
	}
	err = db.QueryRow(
		`SELECT COALESCE((SELECT BALANCE FROM CHIPS WHERE IPID = ?), 0),
		        COALESCE((SELECT PLAYTIME FROM KNOWN_IPS WHERE IPID = ?), 0)`,
		ipid, ipid).Scan(&chips, &playtime)
	return
}

// SyncChipsForExistingPlaytime creates CHIPS rows for all KNOWN_IPS entries
// that have accumulated PLAYTIME but do not yet have a CHIPS row. This is a
// one-time migration for players who were active before the casino update.
// The starting balance is the default (100) plus 1 chip per completed hour of
// pre-existing playtime. Existing CHIPS rows are never modified.
// Returns the number of new rows inserted.
func SyncChipsForExistingPlaytime() (int64, error) {
	if db == nil {
		return 0, nil
	}
	res, err := db.Exec(`
		INSERT OR IGNORE INTO CHIPS(IPID, BALANCE)
		SELECT IPID, ? + (PLAYTIME / 3600)
		FROM KNOWN_IPS
		WHERE PLAYTIME > 0`, defaultChipBalance)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PurgeKnownIPs deletes all rows from the KNOWN_IPS table.
// It returns the number of rows removed.
func PurgeKnownIPs() (int64, error) {
	if db == nil {
		return 0, nil
	}
	res, err := db.Exec("DELETE FROM KNOWN_IPS")
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// AddTormentedIP adds an IPID to the TORMENTED_IPS table.
// Tormented IPIDs experience random disconnects every 30–60 seconds instead of being banned.
func AddTormentedIP(ipid string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec("INSERT OR IGNORE INTO TORMENTED_IPS(IPID) VALUES(?)", ipid)
	return err
}

// RemoveTormentedIP deletes an IPID from the TORMENTED_IPS table.
func RemoveTormentedIP(ipid string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec("DELETE FROM TORMENTED_IPS WHERE IPID = ?", ipid)
	return err
}

// LoadTormentedIPs returns every IPID currently in the TORMENTED_IPS table.
// Called once at server startup to pre-populate the in-memory torment set.
func LoadTormentedIPs() ([]string, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.Query("SELECT IPID FROM TORMENTED_IPS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ipids []string
	for rows.Next() {
		var ipid string
		if err := rows.Scan(&ipid); err != nil {
			return ipids, fmt.Errorf("LoadTormentedIPs scan: %w", err)
		}
		ipids = append(ipids, ipid)
	}
	return ipids, rows.Err()
}

// EnsureChipBalance ensures an IPID exists in the CHIPS table, creating it with 100 chips if absent.
// It is safe to call on every connect; it is a no-op for known IPIDs.
func EnsureChipBalance(ipid string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec("INSERT OR IGNORE INTO CHIPS(IPID, BALANCE) VALUES(?, ?)", ipid, defaultChipBalance)
	return err
}

// GetChipBalance returns the current chip balance for an IPID.
// Returns 0, nil if the IPID is not found (EnsureChipBalance has not been called yet).
func GetChipBalance(ipid string) (int64, error) {
	if db == nil {
		return 0, nil
	}
	row := db.QueryRow("SELECT BALANCE FROM CHIPS WHERE IPID = ?", ipid)
	var balance int64
	if err := row.Scan(&balance); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return balance, nil
}

// AddChips adds the given amount to an IPID's chip balance and returns the new balance.
// The amount must be positive. EnsureChipBalance must be called first.
// The resulting balance is capped at MaxChipBalance to prevent runaway inflation.
func AddChips(ipid string, amount int64) (int64, error) {
	if db == nil {
		return 0, nil
	}
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	var newBalance int64
	err := db.QueryRow(
		"UPDATE CHIPS SET BALANCE = MIN(BALANCE + ?, ?) WHERE IPID = ? RETURNING BALANCE",
		amount, MaxChipBalance, ipid).Scan(&newBalance)
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

// SpendChips deducts the given amount from an IPID's chip balance, returning the new balance.
// Returns an error if the balance would go below zero.
func SpendChips(ipid string, amount int64) (int64, error) {
	if db == nil {
		return 0, nil
	}
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	var newBalance int64
	err := db.QueryRow(
		"UPDATE CHIPS SET BALANCE = BALANCE - ? WHERE IPID = ? AND BALANCE >= ? RETURNING BALANCE",
		amount, ipid, amount).Scan(&newBalance)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("insufficient chips")
	}
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

// GetChipBalancesByIPIDs returns a map of IPID → chip balance for all given IPIDs
// in a single query.  IPIDs with no CHIPS row are absent from the result map.
func GetChipBalancesByIPIDs(ipids []string) (map[string]int64, error) {
	if db == nil || len(ipids) == 0 {
		return map[string]int64{}, nil
	}
	placeholders := make([]string, len(ipids))
	args := make([]any, len(ipids))
	for i, id := range ipids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT IPID, BALANCE FROM CHIPS WHERE IPID IN ("+strings.Join(placeholders, ",")+")",
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int64, len(ipids))
	for rows.Next() {
		var ipid string
		var balance int64
		if err := rows.Scan(&ipid, &balance); err != nil {
			return m, err
		}
		m[ipid] = balance
	}
	return m, rows.Err()
}

// GetUsernamesByIPIDs returns a map of IPID → username for all given IPIDs that
// have a linked account. Unlisted IPIDs are simply absent from the result map.
// This batches N individual GetUsernameByIPID calls into a single query.
func GetUsernamesByIPIDs(ipids []string) (map[string]string, error) {
	if db == nil || len(ipids) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(ipids))
	args := make([]any, len(ipids))
	for i, id := range ipids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT IPID, USERNAME FROM USERS WHERE IPID IN ("+strings.Join(placeholders, ",")+")",
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string, len(ipids))
	for rows.Next() {
		var ipid, username string
		if err := rows.Scan(&ipid, &username); err != nil {
			return m, err
		}
		m[ipid] = username
	}
	return m, rows.Err()
}

// GetTopChipBalances returns the top n IPIDs by chip balance, ordered descending.
func GetTopChipBalances(n int) ([]ChipEntry, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.Query("SELECT IPID, BALANCE FROM CHIPS ORDER BY BALANCE DESC LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []ChipEntry
	for rows.Next() {
		var e ChipEntry
		if err := rows.Scan(&e.Ipid, &e.Balance); err != nil {
			return entries, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetTopPlaytimes returns the top n entries by accumulated playtime, ordered
// descending. Each entry includes the IPID and the linked account username
// (empty string when no account is associated). The query is a single
// LEFT JOIN so it is safe to call frequently without extra resource cost.
func GetTopPlaytimes(n int) ([]PlaytimeEntry, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT k.IPID, COALESCE(u.USERNAME, ''), k.PLAYTIME
		FROM KNOWN_IPS k
		LEFT JOIN USERS u ON u.IPID = k.IPID
		WHERE k.PLAYTIME > 0
		ORDER BY k.PLAYTIME DESC
		LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]PlaytimeEntry, 0, n)
	for rows.Next() {
		var e PlaytimeEntry
		if err := rows.Scan(&e.Ipid, &e.Username, &e.Playtime); err != nil {
			return entries, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
