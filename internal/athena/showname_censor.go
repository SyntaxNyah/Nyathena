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
	"path/filepath"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// censoredNamesFile lists shownames (or substrings of them) that nobody is
// allowed to speak under. It is independent of automod_enabled — moderators
// can curate a name list without turning on the wordlist-based AutoMod
// action — and is loaded unconditionally at startup, and reloadable via
// /reload like parrot.txt/backgrounds.txt.
const censoredNamesFile = "censored_names.txt"

// initShownameCensor loads censored_names.txt at startup. A missing file is
// not an error: matchCensoredName gates on an empty list, so the feature is
// simply inactive until the file exists and the server is started or reloaded.
func initShownameCensor() {
	path := filepath.Join(settings.ConfigPath, censoredNamesFile)
	names, err := loadWordListFile(path)
	if err != nil {
		return
	}
	setCensoredNames(names)
	logger.LogInfof("showname censor: loaded %d censored name(s) from %q", len(names), path)
}

// matchCensoredName performs a substring search of s (expected to already be
// normalizeForFilter'd) against every entry in censored_names.txt. Returns
// the matched entry and true on the first hit, or ("", false) if no match.
//
// An empty entry is skipped rather than matched: strings.Contains treats ""
// as a substring of everything, so a stray empty entry would match every
// showname unconditionally. See matchBannedWord for the same guard and why
// it's needed at this point of use even though loadWordListFile also filters
// empty entries out at load time.
func matchCensoredName(s string) (string, bool) {
	for _, name := range getCensoredNames() {
		if name == "" {
			continue
		}
		if strings.Contains(s, name) {
			return name, true
		}
	}
	return "", false
}

// checkCensoredShowname tests showname against censored_names.txt. On a
// match the speaker is shadow-muted (PunishmentStealthMute, persisted so it
// survives reconnect) and their IPID is added to the lag/torment list —
// mirroring what a moderator running /stealthmute and /lag by hand would do —
// every time they try to speak under that name. Returns true if a match
// fired, so the caller can also silence the very message that triggered it.
func checkCensoredShowname(client *Client, showname string) bool {
	if showname == "" || len(getCensoredNames()) == 0 {
		return false
	}
	matched, ok := matchCensoredName(normalizeForFilter(showname))
	if !ok {
		return false
	}

	if !client.HasActivePunishment(PunishmentStealthMute) {
		reason := fmt.Sprintf("Censored showname (matched %q)", matched)
		client.AddPunishmentBy(PunishmentStealthMute, 0, reason, IssuerSystem)
		if err := db.UpsertTextPunishmentBy(client.Ipid(), int(PunishmentStealthMute), 0, reason, int(IssuerSystem)); err != nil {
			logger.LogErrorf("showname censor: failed to persist stealthmute for %v: %v", client.Ipid(), err)
		}
	}
	if !isIPIDTormented(client.Ipid()) {
		addTormentedIP(client.Ipid())
		go startTormentDisconnect(client)
	}
	logger.LogInfof("showname censor: shadow-muted and lagged %v (uid %d) — matched name %q", client.Ipid(), client.Uid(), matched)
	return true
}
