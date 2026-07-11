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
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// captureConn (defined in voice_test.go) records everything the server writes
// to a client so these tests can assert on the exact MC/OOC bytes emitted.

// TestIsMusicURL verifies the scheme-prefix classification that routes a
// music-change name to the streaming-URL branch of pktAM.
func TestIsMusicURL(t *testing.T) {
	cases := map[string]bool{
		"https://host.com/stuff.mp3": true,
		"http://host.com/stuff.opus": true,
		"[aatnt] godot.opus":         false,
		"Songs":                      false,
		"Lobby":                      false,
		"ftp://host.com/stuff.mp3":   false,
		"":                           false,
		"https//missing-colon.com/x": false,
	}
	for in, want := range cases {
		if got := isMusicURL(in); got != want {
			t.Errorf("isMusicURL(%q) = %v, want %v", in, got, want)
		}
	}
}

// newMusicTestClient builds a struct-literal client wired to a capturing conn
// and placed alone in a fresh area, registered in the global client list. The
// nil sendCh routes SendPacket through the synchronous path so writes land on
// the capturing conn immediately.
func newMusicTestClient(t *testing.T) (*Client, *captureConn) {
	t.Helper()

	origClients := clients
	origConfig := config
	t.Cleanup(func() { clients = origClients; config = origConfig })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}
	// A zero Config disables rate limiting (RateLimit == 0), which is all
	// pktAM needs; a nil config would panic in CheckRateLimit.
	config = &settings.Config{}

	a := area.NewArea(area.AreaData{Name: "Lobby"}, 4, 50, area.EviAny)
	conn := &captureConn{}
	client := &Client{
		conn:       conn,
		uid:        1,
		char:       0,
		possessing: -1,
		ipid:       "ipid-a",
		hdid:       "hdid-a",
		pair:       ClientPairInfo{wanted_id: -1},
	}
	client.SetArea(a)
	clients.AddClient(client)
	return client, conn
}

// TestMCURLWhitelistedBroadcastsVerbatim is OmniTroid's request: when a client
// sends an MC packet whose song slot is a whitelisted http(s) URL, the server
// re-broadcasts that URL byte-for-byte — it must never mangle it.
func TestMCURLWhitelistedBroadcastsVerbatim(t *testing.T) {
	origCDNs := getCDNs()
	t.Cleanup(func() { setCDNs(origCDNs) })
	setCDNs([]string{"host.com"})
	client, conn := newMusicTestClient(t)

	const url = "https://host.com/stuff.mp3"
	pktAM(client, &packet.Packet{Header: "MC", Body: []string{url, "0"}})

	out := conn.String()
	wantMC := "MC#" + url + "#0#"
	if !strings.Contains(out, wantMC) {
		t.Fatalf("expected MC broadcast to contain %q (verbatim URL), got %q", wantMC, out)
	}
	if strings.Contains(out, "Illegal origin") {
		t.Fatalf("whitelisted URL should not be rejected, got %q", out)
	}
	if client.Area().CurrentSong() != url {
		t.Errorf("CurrentSong = %q, want %q", client.Area().CurrentSong(), url)
	}
}

// TestMCURLUnwhitelistedRejected verifies that an MC URL whose host is not in
// the CDN whitelist is rejected with an OOC "Illegal origin" notice and is
// never broadcast as a music change.
func TestMCURLUnwhitelistedRejected(t *testing.T) {
	origCDNs := getCDNs()
	t.Cleanup(func() { setCDNs(origCDNs) })
	setCDNs([]string{"trusted.example"})
	client, conn := newMusicTestClient(t)

	const url = "https://evil.com/stuff.mp3"
	pktAM(client, &packet.Packet{Header: "MC", Body: []string{url, "0"}})

	out := conn.String()
	if !strings.Contains(out, "Illegal origin") {
		t.Fatalf("expected an 'Illegal origin' OOC message, got %q", out)
	}
	if strings.Contains(out, "MC#"+url) {
		t.Fatalf("un-whitelisted URL must not be broadcast, got %q", out)
	}
	if client.Area().CurrentSong() == url {
		t.Errorf("CurrentSong should not be set to a rejected URL")
	}
}

// TestJoinAreaSyncsCurrentMusic verifies that a client joining an area where a
// track is already playing (initial connect into area 0, or an /area change
// into any other area) is sent an MC packet for that track. Without this, the
// joining client's player simply sits in silence — nothing ever tells it what
// to play — until someone changes the track again or the player thinks to
// run /getmusic by hand.
func TestJoinAreaSyncsCurrentMusic(t *testing.T) {
	newTestClients(t)
	origAreas := areas
	t.Cleanup(func() { areas = origAreas })

	a := area.NewArea(area.AreaData{Name: "Lobby"}, 4, 50, area.EviAny)
	areas = []*area.Area{a}
	a.SetCurrentSong("[aatnt] godot.opus")

	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	clients.AddClient(client)

	client.JoinArea(a)

	const wantMC = "MC#[aatnt] godot.opus#0#Server#1#0#0#%"
	if out := conn.String(); !strings.Contains(out, wantMC) {
		t.Fatalf("JoinArea did not sync current music: expected %q in output, got %q", wantMC, out)
	}
}

// TestJoinAreaNoCurrentMusicSendsNoMC verifies that joining an area where
// nothing has played yet does not emit a spurious MC packet.
func TestJoinAreaNoCurrentMusicSendsNoMC(t *testing.T) {
	newTestClients(t)
	origAreas := areas
	t.Cleanup(func() { areas = origAreas })

	a := area.NewArea(area.AreaData{Name: "Lobby"}, 4, 50, area.EviAny)
	areas = []*area.Area{a}

	conn := &captureConn{}
	client := &Client{conn: conn, uid: 1, char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	clients.AddClient(client)

	client.JoinArea(a)

	if out := conn.String(); strings.Contains(out, "MC#") {
		t.Fatalf("expected no MC packet when area has no current song, got %q", out)
	}
}

// TestMCURLWithQueryStringNotMangled is the stronger no-mangle guarantee: a URL
// carrying a query string arrives AO2-escape-encoded ('&' → "<and>") on the
// wire. The server must re-broadcast that wire form unchanged, so that when a
// recipient decodes it they recover the exact original URL.
func TestMCURLWithQueryStringNotMangled(t *testing.T) {
	origCDNs := getCDNs()
	t.Cleanup(func() { setCDNs(origCDNs) })
	setCDNs([]string{"host.com"})
	client, conn := newMusicTestClient(t)

	const originalURL = "https://host.com/stream?id=7&fmt=mp3"
	wireName := encode(originalURL) // what a client actually puts on the wire
	pktAM(client, &packet.Packet{Header: "MC", Body: []string{wireName, "0"}})

	out := conn.String()
	if !strings.Contains(out, "MC#"+wireName+"#0#") {
		t.Fatalf("expected verbatim wire form %q in broadcast, got %q", wireName, out)
	}
	// The broadcast wire form must decode back to the exact original URL.
	if got := decode(wireName); got != originalURL {
		t.Errorf("round-trip mangled the URL: decode(%q) = %q, want %q", wireName, got, originalURL)
	}
}
