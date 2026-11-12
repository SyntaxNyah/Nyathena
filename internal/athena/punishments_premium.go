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

// AO Gold / Skrapegropen Premium paywall punishment.
//
// This is a joke monetisation effect: the punished player's messages are
// replaced with one of the branded "you need a premium account" notices. It
// deliberately does not implement accounts, payments, or a real premium tier —
// the whole feature is the fake paywall.
//
// The spelling is intentional except for "requires" in the OOPS line, which
// was a typo in the original copy and is fixed here.

import "math/rand"

// goldPaywallLines is the rotation of fake premium notices. Keep the brand
// capitalisation exactly as written: the inconsistency is part of the joke.
var goldPaywallLines = []string{
	"[You require AO gold/premium to see this message]",
	"This message requires SkrapeGOLD account to be seen.",
	"If you have not purchased a GOLD account, you may not view this message.",
	"You need AO Premium subscription to see this message.",
	"This message is only visible to users with SkrapeGolden account.",
	"OOPS! This message requires a Premium Skrapegropen Account.",
}

// goldPaywallLine picks one branding variant for a single message.
func goldPaywallLine() string {
	return goldPaywallLines[rand.Intn(len(goldPaywallLines))]
}

// applyGoldPaywall replaces an OOC message with a paywall notice when the
// speaker carries the punishment. IC messages go through
// ApplyPunishmentToText instead; this is the OOC half of the same effect.
func applyGoldPaywall(client *Client, message string) string {
	if client == nil || !client.HasActivePunishment(PunishmentGold) {
		return message
	}
	return goldPaywallLine()
}
