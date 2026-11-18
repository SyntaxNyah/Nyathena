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
	"strconv"
	"strings"
	"unicode"
)

type cmdParamList struct {
	list *[]string
}

func (v cmdParamList) String() string {
	if v.list != nil {
		return strings.Join(*v.list, ",")
	}
	return ""
}

func (v cmdParamList) Set(s string) error {
	x := strings.Split(s, ",")
	*v.list = x
	return nil
}

// getUidList returns a list of clients that have the given UID(s).
func getUidList(uids []string) []*Client {
	var l []*Client
	for _, s := range uids {
		uid, err := strconv.Atoi(s)
		if err != nil || uid == -1 {
			continue
		}
		c, err := getClientByUid(uid)
		if err != nil {
			continue
		}
		l = append(l, c)
	}
	return l
}

// getIpidList returns a list of clients that have the given IPID(s).
func getIpidList(ipids []string) []*Client {
	var l []*Client
	for _, s := range ipids {
		c := getClientsByIpid(s)
		l = append(l, c...)
	}
	return l
}

// getCharacterID returns the character ID for a given character name.
// Returns -1 if the character name is not found.
// Uses the O(1) name→ID index when available (production); falls back to a
// linear scan otherwise (tests that set the character list directly). The index
// is loaded once via the atomic accessor so a concurrent /reload swap is safe.
func getCharacterID(charName string) int {
	if idx := getCharacterIndex(); idx != nil {
		if id, ok := idx[strings.ToLower(charName)]; ok {
			return id
		}
		return -1
	}
	for i, name := range getCharacters() {
		if strings.EqualFold(name, charName) {
			return i
		}
	}
	return -1
}

// maxCharacterFolderLength caps the length of an unlisted character folder
// name accepted by moderator commands. Character folders are short; the cap
// keeps a malformed command from stuffing an oversized string into every IC
// packet that mentions the target.
const maxCharacterFolderLength = 64

// resolveCharacterName resolves a moderator-supplied character name against
// characters.txt. A listed name comes back with its canonical spelling and real
// slot ID. A name that is not listed is treated as a client-side character
// folder — an iniswap-only character that exists in the asset database but not
// in characters.txt — and comes back with ID -1, because the server has no slot
// for it. The bool is false for an empty name or one containing characters that
// would corrupt the '#'-delimited AO wire format.
func resolveCharacterName(raw string) (name string, charID int, ok bool) {
	name = strings.TrimSpace(raw)
	if name == "" {
		return "", -1, false
	}
	if id := getCharacterID(name); id >= 0 {
		return getCharacters()[id], id, true
	}
	if len(name) > maxCharacterFolderLength {
		return "", -1, false
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == '|' || r == '#' || r == '%' {
			return "", -1, false
		}
	}
	return name, -1, true
}

// clientDisplayName returns the client's effective showname, falling back to
// their character name when the showname is blank.
func clientDisplayName(c *Client) string {
	name := c.EffectiveShowname()
	if strings.TrimSpace(name) == "" && c.CharID() >= 0 && c.CharID() < len(getCharacters()) {
		name = getCharacters()[c.CharID()]
	}
	return name
}
