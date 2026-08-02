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
	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// playAreaMusic broadcasts a music change to an area AND records it as the
// area's current track, so late joiners (JoinArea's music sync) and /getmusic
// resend the track that is actually playing.
//
// Every path that starts an area's BGM must go through here — the jukebox MC
// packet, a streaming URL MC packet, /play, /randomsong and the YouTube
// pipeline. Previously only the two MC-packet paths in pktAM recorded the
// song, so a custom track started with /play (or a Discord/YouTube URL) left
// the area's stored song pointing at the last *jukebox* entry: walking out of
// the area and back in resynced the joiner to that stale jukebox track instead
// of the custom one that was really playing.
//
// One-shot sound effects (the /sfxcurse MC fallback, Looping "0") deliberately
// do NOT go through here — they aren't the area's BGM and must not overwrite it.
func playAreaMusic(a *area.Area, p *packet.MCToClient) {
	a.SetCurrentSong(p.Name)
	broadcastToArea(a, p)
}
