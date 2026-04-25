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
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"golang.org/x/crypto/bcrypt"
)

var validUsernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

// generateCaptcha returns a random 16-character hex string used as a registration captcha token.
// Uses a stack-allocated [8]byte to avoid a heap allocation.
func generateCaptcha() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// onRegistered completes account creation after the DB row is already written:
// auto-logs the client in, seeds chip balance (casino only), and sends the
// success message. The message is tailored to whichever feature set is live —
// full casino vs. accounts-only wardrobe / tag / playtime mode.
func onRegistered(client *Client, username string) {
	client.SetAuthenticated(true)
	client.SetModName(username)
	if config.EnableCasino {
		if err := db.EnsureChipBalance(client.Ipid()); err != nil {
			logger.LogErrorf("Failed to seed chip balance on register for %v: %v", username, err)
		}
		client.SendServerMessage(fmt.Sprintf(
			"✅ Account '%v' created and logged in!\n\n"+
				"📋 What your account tracks:\n"+
				"  • 💰 Nyathena Chips (casino balance)\n"+
				"  • ⏱ Playtime on this server\n"+
				"  • 🏆 Casino leaderboard standings\n\n"+
				"Use /account to view your profile.\n"+
				"Use /chips to check your balance.\n"+
				"Your account is linked to your connection — use /login <username> <password> to sign in on reconnect.",
			username))
	} else {
		client.SendServerMessage(fmt.Sprintf(
			"✅ Account '%v' created and logged in!\n\n"+
				"📋 What your account tracks (gambling is off on this server):\n"+
				"  • 👗 Wardrobe — save favourite characters with /favourite <char>\n"+
				"  • 🏷️ Default tag — any tag in /shop is free to equip with /settag <id>\n"+
				"  • ⏱ Playtime — accumulates across sessions, see /playtime top\n\n"+
				"Use /account to view your profile.\n"+
				"Your account is linked to your connection — use /login <username> <password> to sign in on reconnect.",
			username))
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Registered player account %v.", username), false)
}

// Handles /register
//
// Any player can create a free account. Accounts do not grant any extra
// permissions — they exist purely to track in-game features such as
// Nyathena Chip balance, playtime, and future leaderboard standings.
//
// When register_captcha is true (default) this is a two-step flow:
//  1. /register <username> <password> — validates inputs and issues a captcha.
//  2. /captcha <token>               — confirms the token and creates the account.
//
// When register_captcha is false the account is created immediately.
func cmdRegister(client *Client, args []string, _ string) {
	if client.Authenticated() {
		client.SendServerMessage("You already have an account linked to this session. Use /logout first if you want to register a different account.")
		return
	}

	// One account per IPID.
	if existing, err := db.GetUsernameByIPID(client.Ipid()); err == nil && existing != "" {
		client.SendServerMessage(fmt.Sprintf(
			"An account ('%v') is already registered on your connection.\nUse /login %v <password> to sign in.",
			existing, existing))
		return
	}

	username, password := args[0], args[1]

	if !validUsernameRe.MatchString(username) {
		client.SendServerMessage(fmt.Sprintf(
			"❌ '%v' isn't a valid username.\n"+
				"Usernames must be 3–20 characters and may only contain letters (A–Z, a–z), numbers (0–9), and underscores (_).\n"+
				"Examples that work: ena_m00ny, alice42, my_alt",
			username))
		return
	}
	if len(password) < 6 {
		client.SendServerMessage("Password must be at least 6 characters.")
		return
	}
	if db.UserExists(username) {
		// Distinguish "name claimed by a moderator account" from "name claimed
		// by another player" so users picking a name like "ena_m00ny" aren't
		// left wondering why a fresh-looking name is unavailable.
		if db.IsModUser(username) {
			client.SendServerMessage(fmt.Sprintf(
				"That username ('%v') is reserved by a staff account on this server. Please pick a different name for your player account.",
				username))
		} else {
			client.SendServerMessage(fmt.Sprintf(
				"That username ('%v') is already registered by another player. Please choose a different one.",
				username))
		}
		return
	}

	// When captcha is disabled, register immediately — no token generation,
	// no pending state, no extra allocations.
	if !config.RegisterCaptcha {
		if err := db.RegisterPlayer(username, []byte(password), client.Ipid()); err != nil {
			logger.LogErrorf("Register failed for %v (IPID %v): %v", username, client.Ipid(), err)
			client.SendServerMessage("Registration failed. Please try again.")
			return
		}
		onRegistered(client, username)
		return
	}

	// Captcha enabled: hash the password now so no plaintext is kept in pending state.
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		logger.LogErrorf("Failed to hash password for %v (IPID %v): %v", username, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}
	token, err := generateCaptcha()
	if err != nil {
		logger.LogErrorf("Failed to generate captcha for %v (IPID %v): %v", username, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}
	client.SetPendingReg(username, token, hashed)

	client.SendServerMessage(fmt.Sprintf(
		"🔐 One last step to create account '%v'!\n\n"+
			"To confirm you are human, please type the following command exactly:\n\n"+
			"  /captcha %v\n\n"+
			"This token expires when you disconnect.",
		username, token))
}

// Handles /captcha <token>
//
// Completes a pending registration that was started with /register.
// The player must supply the exact token that was issued during /register.
func cmdCaptcha(client *Client, args []string, _ string) {
	if !config.RegisterCaptcha {
		client.SendServerMessage("Registration captcha is not enabled on this server.")
		return
	}

	pendingUser, expectedToken, pendingHashedPass := client.PendingReg()
	if pendingUser == "" {
		client.SendServerMessage("You don't have a pending registration. Use /register <username> <password> to start one.")
		return
	}

	// Hex tokens are case-insensitive by definition; trimming surrounding
	// whitespace handles AO2 clients that auto-add a trailing space when copying.
	supplied := strings.ToLower(strings.TrimSpace(args[0]))
	// Constant-time comparison prevents timing-based token guessing.
	if subtle.ConstantTimeCompare([]byte(supplied), []byte(expectedToken)) != 1 {
		client.SetPendingReg("", "", nil)
		client.SendServerMessage("❌ Incorrect captcha token. Please use /register <username> <password> again to get a new token.")
		return
	}

	// Token correct — clear pending state before touching the DB.
	client.SetPendingReg("", "", nil)

	// Re-check conditions that may have changed while the captcha was pending.
	if client.Authenticated() {
		client.SendServerMessage("You are already logged in.")
		return
	}
	if existing, err := db.GetUsernameByIPID(client.Ipid()); err == nil && existing != "" {
		client.SendServerMessage(fmt.Sprintf(
			"An account ('%v') was registered on your connection in the meantime. Use /login %v <password> to sign in.",
			existing, existing))
		return
	}
	if db.UserExists(pendingUser) {
		client.SendServerMessage("That username was taken while you were completing the captcha. Please use /register <username> <password> with a different name.")
		return
	}

	// Password was already bcrypt-hashed at /register time.
	if err := db.RegisterPlayerHashed(pendingUser, pendingHashedPass, client.Ipid()); err != nil {
		logger.LogErrorf("Register failed for %v (IPID %v): %v", pendingUser, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}
	onRegistered(client, pendingUser)
}

// Handles /account
//
// Displays the current player's account information: username, chip balance,
// and playtime. Prompts unregistered players to create an account.
func cmdAccount(client *Client, _ []string, _ string) {
	var username string
	if client.Authenticated() {
		username = client.ModName()
	} else {
		// Try to find a linked account for this IPID even if not currently logged in.
		u, err := db.GetUsernameByIPID(client.Ipid())
		if err == nil && u != "" {
			client.SendServerMessage(fmt.Sprintf(
				"Your account '%v' is not currently active. Use /login %v <password> to sign in.", u, u))
			return
		}
		if config.EnableCasino {
			client.SendServerMessage(
				"You don't have an account yet.\n\n" +
					"💡 Accounts are free and let you track:\n" +
					"  • 💰 Nyathena Chips (casino currency)\n" +
					"  • ⏱ Playtime on this server\n" +
					"  • 🏆 Casino leaderboard standings\n\n" +
					"Create one now with: /register <username> <password>\n" +
					"(Username: 3–20 chars, letters/numbers/underscore; Password: 6+ chars)")
		} else {
			client.SendServerMessage(
				"You don't have an account yet.\n\n" +
					"💡 Accounts are free and let you track (gambling is off here):\n" +
					"  • 👗 Wardrobe favourites — /favourite <char>\n" +
					"  • 🏷️ Default cosmetic tag — /settag <tag_id> (every tag in /shop is free)\n" +
					"  • ⏱ Playtime on this server — /playtime top\n\n" +
					"Create one now with: /register <username> <password>\n" +
					"(Username: 3–20 chars, letters/numbers/underscore; Password: 6+ chars)")
		}
		return
	}

	chips, playtimeSec, err := db.GetAccountStats(client.Ipid())
	if err != nil {
		chips = 0
		playtimeSec = 0
	}
	// Add the current session's elapsed time so the display reflects live playtime.
	if connAt := client.ConnectedAt(); !connAt.IsZero() {
		playtimeSec += int64(time.Since(connAt).Seconds())
	}

	if config.EnableCasino {
		client.SendServerMessage(fmt.Sprintf(
			"\n👤 Account: %v\n"+
				"💰 Chips: %d\n"+
				"⏱ Playtime: %v",
			username, chips, formatPlaytime(playtimeSec)))
	} else {
		activeTag := db.GetActiveTag(client.Ipid())
		tagDisplay := "(none)"
		if t := formatTagDisplay(activeTag); t != "" {
			tagDisplay = t
		}
		client.SendServerMessage(fmt.Sprintf(
			"\n👤 Account: %v\n"+
				"🏷️ Active tag: %v\n"+
				"⏱ Playtime: %v",
			username, tagDisplay, formatPlaytime(playtimeSec)))
	}
}

// formatPlaytime converts seconds into a human-readable "Xh Ym" / "Ym" string.
func formatPlaytime(seconds int64) string {
	if seconds <= 0 {
		return "less than a minute"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// Handles /playtime [top [n]]
//
// Displays the global playtime leaderboard. Only registered players appear;
// anonymous connections are excluded. Results come from a single efficient
// INNER JOIN query — no extra resource cost.
func cmdPlaytimeTop(client *Client, args []string, usage string) {
	n := 10
	remaining := args

	// Accept an optional leading "top" subcommand keyword (case-insensitive, no allocation).
	if len(remaining) > 0 && strings.EqualFold(remaining[0], "top") {
		remaining = remaining[1:]
	}

	// Accept an optional count argument (1–50). Any other token is a usage error.
	if len(remaining) > 0 {
		if v, err := strconv.Atoi(remaining[0]); err == nil && v > 0 && v <= 50 {
			n = v
		} else {
			client.SendServerMessage(usage)
			return
		}
	}

	entries, err := db.GetTopPlaytimes(n)
	if err != nil || len(entries) == 0 {
		client.SendServerMessage("No playtime data available yet.")
		return
	}

	// Build a map of IPID → current-session seconds for all connected clients
	// so the leaderboard reflects live playtime, not just the last-flushed DB value.
	// Multiple clients may share the same IPID (multiclient), so use += to sum
	// all active sessions for the same IPID, matching clientCleanup behaviour.
	// Pre-size with player count to avoid rehashing.
	liveSecs := make(map[string]int64, players.GetPlayerCount())
	clients.ForEach(func(c *Client) {
		if connAt := c.ConnectedAt(); !connAt.IsZero() {
			if secs := int64(time.Since(connAt).Seconds()); secs > 0 {
				liveSecs[c.Ipid()] += secs
			}
		}
	})
	// Merge live session seconds into the DB entries and re-sort.
	if len(liveSecs) > 0 {
		for i := range entries {
			if s, ok := liveSecs[entries[i].Ipid]; ok {
				entries[i].Playtime += s
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Playtime > entries[j].Playtime
		})
	}

	// Pre-size the builder: header ~35 bytes + ~35 bytes per row.
	var sb strings.Builder
	sb.Grow(35 + len(entries)*35)
	sb.WriteString(fmt.Sprintf("\n⏱ Playtime Leaderboard (Top %d)\n", len(entries)))
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %2d. %-20v  %v\n", i+1, e.Username, formatPlaytime(e.Playtime)))
	}
	client.SendServerMessage(sb.String())
}

// cmdProfile implements /profile [uid]. With no argument, shows the caller's
// own profile card; with a UID, shows the profile card of the target online
// player. Visible to everyone — the card aggregates data that is already
// readable via /account, /playtime, /chips, /wardrobe.
func cmdProfile(client *Client, args []string, _ string) {
	target := client
	if len(args) > 0 {
		uid, err := strconv.Atoi(args[0])
		if err != nil {
			client.SendServerMessage("Invalid UID.")
			return
		}
		t, err := getClientByUid(uid)
		if err != nil {
			client.SendServerMessage("Client does not exist.")
			return
		}
		target = t
	}

	// Resolve the display name: showname -> character -> "(no character)".
	displayName := clientDisplayName(target)
	if strings.TrimSpace(displayName) == "" {
		displayName = "(no character selected)"
	}

	// Look up any linked account username for this IPID. This works whether or
	// not the target is currently authenticated, falling back to "(guest)".
	username := "(guest)"
	if u, err := db.GetUsernameByIPID(target.Ipid()); err == nil && u != "" {
		username = u
	}

	// Chips + playtime: pulled from the account-stats table; session time is
	// added on top for a live total, matching /account's behaviour.
	chips, playtimeSec, err := db.GetAccountStats(target.Ipid())
	if err != nil {
		chips = 0
		playtimeSec = 0
	}
	if connAt := target.ConnectedAt(); !connAt.IsZero() {
		playtimeSec += int64(time.Since(connAt).Seconds())
	}

	// Favourite characters (wardrobe) — only meaningful if linked to an account.
	var favs []string
	if username != "(guest)" {
		if f, err := db.GetFavourites(username); err == nil {
			favs = f
		}
	}
	favsDisplay := "(none)"
	if len(favs) > 0 {
		if len(favs) > 8 {
			favsDisplay = strings.Join(favs[:8], ", ") + fmt.Sprintf(", … (+%d more)", len(favs)-8)
		} else {
			favsDisplay = strings.Join(favs, ", ")
		}
	}

	// Active cosmetic tag (casino or non-casino modes both expose this).
	activeTag := db.GetActiveTag(target.Ipid())
	tagDisplay := "(none)"
	if t := formatTagDisplay(activeTag); t != "" {
		tagDisplay = t
	}

	// Active punishments count — don't leak details, just a count so players
	// can tell at a glance if the target is currently cursed with something.
	activePunishments := len(target.Punishments())

	areaName := "(unknown)"
	if a := target.Area(); a != nil {
		areaName = a.Name()
	}

	var sb strings.Builder
	sb.WriteString("\n👤 Profile\n")
	sb.WriteString(fmt.Sprintf("  Name:        %v\n", displayName))
	sb.WriteString(fmt.Sprintf("  UID:         %d\n", target.Uid()))
	sb.WriteString(fmt.Sprintf("  Account:     %v\n", username))
	sb.WriteString(fmt.Sprintf("  Area:        %v\n", areaName))
	sb.WriteString(fmt.Sprintf("  Playtime:    %v\n", formatPlaytime(playtimeSec)))
	if config != nil && config.EnableCasino {
		sb.WriteString(fmt.Sprintf("  Chips:       %d\n", chips))
	}
	sb.WriteString(fmt.Sprintf("  Active tag:  %v\n", tagDisplay))
	sb.WriteString(fmt.Sprintf("  Favourites:  %v\n", favsDisplay))
	if activePunishments > 0 {
		sb.WriteString(fmt.Sprintf("  Punishments: %d active\n", activePunishments))
	}
	client.SendServerMessage(sb.String())
}
