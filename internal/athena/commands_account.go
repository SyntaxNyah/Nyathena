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
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

var validUsernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

// Handles /register
//
// Any player can create a free account. Accounts do not grant any extra
// permissions — they exist purely to track in-game features such as
// Nyathena Chip balance, playtime, and future leaderboard standings.
// All existing moderator/admin accounts remain fully compatible.
func cmdRegister(client *Client, args []string, _ string) {
	if client.Authenticated() {
		client.SendServerMessage("You already have an account linked to this session. Use /logout first if you want to register a different account.")
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

	err := db.RegisterPlayer(username, []byte(password), client.Ipid())
	if err != nil {
		logger.LogErrorf("Register failed for %v (IPID %v): %v", username, client.Ipid(), err)
		client.SendServerMessage("Registration failed. Please try again.")
		return
	}

	// Auto-login the new account (permissions stay 0 — no extra powers).
	client.SetAuthenticated(true)
	client.SetModName(username)
	// No SetPerms call needed — perms remain 0 (NONE).

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
	addToBuffer(client, "CMD", fmt.Sprintf("Registered player account %v.", username), false)
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

	// Accept an optional leading "top" subcommand keyword.
	if len(remaining) > 0 && strings.ToLower(remaining[0]) == "top" {
		remaining = remaining[1:]
	}

	// Accept an optional count argument.
	if len(remaining) > 0 {
		if v, err := strconv.Atoi(remaining[0]); err == nil && v > 0 && v <= 50 {
			n = v
		} else if remaining[0] != "" {
			client.SendServerMessage(usage)
			return
		}
	}

	entries, err := db.GetTopPlaytimes(n)
	if err != nil || len(entries) == 0 {
		client.SendServerMessage("No playtime data available yet.")
		return
	}

	var sb strings.Builder
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
