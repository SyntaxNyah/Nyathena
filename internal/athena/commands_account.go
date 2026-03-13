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
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"golang.org/x/crypto/bcrypt"
)

var validUsernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

// generateCaptcha returns a random 16-character hex string used as a registration captcha token.
func generateCaptcha() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Handles /register
//
// Any player can create a free account. Accounts do not grant any extra
// permissions — they exist purely to track in-game features such as
// Nyathena Chip balance, playtime, and future leaderboard standings.
// All existing moderator/admin accounts remain fully compatible.
//
// Registration is a two-step process:
//  1. /register <username> <password> — validates the request and issues a captcha token.
//  2. /captcha <token>               — confirms the captcha and finalises account creation.
func cmdRegister(client *Client, args []string, _ string) {
	if client.Authenticated() {
		client.SendServerMessage("You already have an account linked to this session. Use /logout first if you want to register a different account.")
		return
	}

	// One account per IPID: block if this connection already has a linked account.
	existingUser, err := db.GetUsernameByIPID(client.Ipid())
	if err == nil && existingUser != "" {
		client.SendServerMessage(fmt.Sprintf(
			"An account ('%v') is already registered on your connection.\nUse /login %v <password> to sign in.",
			existingUser, existingUser))
		return
	}

	username := args[0]
	password := args[1]

	if !validUsernameRe.MatchString(username) {
		client.SendServerMessage("Username must be 3–20 characters and may only contain letters, numbers, and underscores.")
		return
	}
	if len(password) < 6 {
		client.SendServerMessage("Password must be at least 6 characters.")
		return
	}

	if db.UserExists(username) {
		client.SendServerMessage("That username is already taken. Please choose another.")
		return
	}

	// Generate a random captcha token and hold the registration details until confirmed.
	// Hash the password now so no plaintext password is kept in memory after this point.
	token, err := generateCaptcha()
	if err != nil {
		logger.LogErrorf("Failed to generate captcha for %v (IPID %v): %v", username, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		logger.LogErrorf("Failed to hash password for %v (IPID %v): %v", username, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}
	client.SetPendingReg(username, string(hashed), token)

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
func cmdCaptcha(client *Client, args []string, usage string) {
	pendingUser, pendingHashedPass, expectedToken := client.PendingReg()
	if pendingUser == "" {
		client.SendServerMessage("You don't have a pending registration. Use /register <username> <password> to start one.")
		return
	}

	// Use a constant-time comparison to prevent timing-based token guessing.
	if subtle.ConstantTimeCompare([]byte(args[0]), []byte(expectedToken)) != 1 {
		// Wrong token — clear the pending state so they have to restart.
		client.SetPendingReg("", "", "")
		client.SendServerMessage("❌ Incorrect captcha token. Please use /register <username> <password> again to get a new token.")
		return
	}

	// Captcha correct — clear pending state before creating the account.
	client.SetPendingReg("", "", "")

	// Re-check conditions that could have changed since /register was typed.
	if client.Authenticated() {
		client.SendServerMessage("You are already logged in.")
		return
	}
	existingUser, err := db.GetUsernameByIPID(client.Ipid())
	if err == nil && existingUser != "" {
		client.SendServerMessage(fmt.Sprintf(
			"An account ('%v') was registered on your connection in the meantime. Use /login %v <password> to sign in.",
			existingUser, existingUser))
		return
	}
	if db.UserExists(pendingUser) {
		client.SendServerMessage("That username was taken while you were completing the captcha. Please use /register <username> <password> with a different name.")
		return
	}

	// The password was already hashed at /register time; use the pre-hashed form.
	if err := db.RegisterPlayerHashed(pendingUser, []byte(pendingHashedPass), client.Ipid()); err != nil {
		logger.LogErrorf("Register failed for %v (IPID %v): %v", pendingUser, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}

	// Auto-login the new account (permissions stay 0 — no extra powers).
	client.SetAuthenticated(true)
	client.SetModName(pendingUser)

	// Guarantee a chip row exists (100 chips if not already seeded).
	if config.EnableCasino {
		if err := db.EnsureChipBalance(client.Ipid()); err != nil {
			logger.LogErrorf("Failed to seed chip balance on register for %v: %v", pendingUser, err)
		}
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
		pendingUser))
	addToBuffer(client, "CMD", fmt.Sprintf("Registered player account %v.", pendingUser), false)
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
		client.SendServerMessage(
			"You don't have an account yet.\n\n" +
				"💡 Accounts are free and let you track:\n" +
				"  • 💰 Nyathena Chips (casino currency)\n" +
				"  • ⏱ Playtime on this server\n" +
				"  • 🏆 Casino leaderboard standings\n\n" +
				"Create one now with: /register <username> <password>\n" +
				"(Username: 3–20 chars, letters/numbers/underscore; Password: 6+ chars)")
		return
	}

	bal, err := db.GetChipBalance(client.Ipid())
	if err != nil {
		bal = 0
	}
	playtimeSec, _ := db.GetPlaytime(client.Ipid())

	client.SendServerMessage(fmt.Sprintf(
		"\n👤 Account: %v\n"+
			"💰 Chips: %d\n"+
			"⏱ Playtime: %v",
		username, bal, formatPlaytime(playtimeSec)))
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
// Displays the global playtime leaderboard. Shows account names when players
// have linked accounts, falling back to their IPID otherwise.
// Results come from a single efficient LEFT JOIN query — no extra resources.
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

	// Pre-size the builder: header ~35 bytes + ~35 bytes per row.
	var sb strings.Builder
	sb.Grow(35 + len(entries)*35)
	sb.WriteString(fmt.Sprintf("\n⏱ Playtime Leaderboard (Top %d)\n", len(entries)))
	for i, e := range entries {
		displayName := e.Ipid
		if e.Username != "" {
			displayName = e.Username
		}
		sb.WriteString(fmt.Sprintf("  %2d. %-20v  %v\n", i+1, displayName, formatPlaytime(e.Playtime)))
	}
	client.SendServerMessage(sb.String())
}
