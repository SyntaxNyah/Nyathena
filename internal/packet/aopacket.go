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

// Package packet implements AO2 network packets.
package packet

import (
	"fmt"
	"strings"
)

// Packet represents an AO2 network packet.
// AO2 network packets are comprised of a non-empty header, followed by a '#'-separated list of parameters, ending with a '%'.
type Packet struct {
	Header string
	Body   []string
}

// NewPacket returns a new Packet with the specified data, which should be a valid AO2 packet.
func NewPacket(data string) (*Packet, error) {
	// Split off the header at the first '#' without allocating a full slice for
	// the entire packet up front.  For packets with no body this avoids any
	// string-split allocation entirely.
	idx := strings.IndexByte(data, '#')
	var header, rest string
	if idx < 0 {
		header = data
	} else {
		header = data[:idx]
		rest = data[idx+1:]
	}
	if strings.TrimSpace(header) == "" {
		return nil, fmt.Errorf("packet header cannot be empty")
	}

	var body []string
	if rest != "" {
		body = strings.Split(rest, "#")
		// Remove the empty trailing entry produced by the final '#' delimiter.
		if len(body) > 1 && body[len(body)-1] == "" {
			body = body[:len(body)-1]
		}
	}
	return &Packet{Header: header, Body: body}, nil
}

// String returns a string representation of the Packet.
func (p Packet) String() string {
	return p.Header + "#" + strings.Join(p.Body, "#") + "#%"
}
