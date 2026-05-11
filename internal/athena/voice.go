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

// Server-relayed voice chat.
//
// Every audio frame travels client → Athena → other clients in the same
// area.  No peer-to-peer path exists, which means peers never learn each
// other's IPs — only Athena sees them, and Athena already sees them anyway
// for every other AO2 packet.  This replaces the previous WebRTC-signalling
// relay; TURN/STUN are not used and no ICE configuration is advertised.
//
// The server treats Opus frames as opaque base64 blobs and forwards them
// unmodified.  No decode/encode is performed, so CPU cost is roughly one
// memcpy per frame per receiver.  A future voice-effects step would slot
// into relayFrame, between the rate-limit check and the broadcast call.
//
// Protocol:
//   VS_CAPS#<enabled>#<ptt_only>#<max_peers>#<codec>#<sample_rate>#<frame_ms>#<max_frame_bytes>#%   (S→C, handshake)
//   VS_JOIN#<uid>#%                                       (C→S, then broadcast to area)
//   VS_LEAVE#<uid>#%                                      (C→S or server-generated)
//   VS_PEERS#<csv_uids>#%                                 (S→C, current voice peers in area)
//   VS_FRAME#<b64_opus>#%                                 (C→S, audio frame for the area)
//   VS_AUDIO#<from_uid>#<b64_opus>#%                      (S→C, relayed audio frame)
//   VS_SPEAK#<uid>#<on_off>#%                             (C→S→area; speaking indicator)
//
// Only joined clients may send VS_FRAME or VS_SPEAK.  Muted or jailed
// clients may not join voice.

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// Protocol-fixed audio parameters.  Clients must use these exact values; the
// server never decodes the payload but the parameters are advertised in
// VS_CAPS so client codecs are configured consistently across deployments.
const (
	voiceCodec      = "opus"
	voiceSampleRate = 48000
	voiceFrameMs    = 20
)

// LogVoiceConfig prints the decoded [Voice] config at startup so operators
// can see at a glance whether the section was loaded.
func LogVoiceConfig() {
	if config == nil {
		logger.LogInfo("voice: config is nil at startup (this is a bug)")
		return
	}
	logger.LogInfof("voice: enable_voice=%v ptt_only=%v max_peers_per_area=%d max_frame_bytes=%d default_area_voice_allowed=%v",
		config.EnableVoice,
		config.PTTOnly,
		config.MaxPeersPerArea,
		config.MaxFrameBytes,
		config.DefaultAreaVoiceAllowed,
	)
}

// voiceRooms tracks which UIDs are currently publishing voice in each area.
// Guarded by voiceMu.  Keyed by *area.Area pointer, matching the client.Area()
// identity semantics used elsewhere in the package.
var (
	voiceMu    sync.RWMutex
	voiceRooms = map[*area.Area]map[int]struct{}{}
)

// voiceEnabled reports whether the server-level voice feature is on.
func voiceEnabled() bool {
	return config != nil && config.EnableVoice
}

// maxFrameBytes returns the configured per-frame byte cap, or a sane default
// if the operator left it at zero.  4000 bytes comfortably fits a 20 ms Opus
// frame at any bitrate up to 510 kbps (the codec's hard ceiling).
func maxFrameBytes() int {
	if config == nil || config.MaxFrameBytes <= 0 {
		return 4000
	}
	return config.MaxFrameBytes
}

// sendVoiceCaps informs the client whether voice is available and, if so,
// the audio parameters they should encode against.  Safe to call with voice
// disabled; in that case the client is told the feature is off and should
// not render voice UI.
func sendVoiceCaps(client *Client) {
	if !voiceEnabled() {
		logger.LogInfof("voice: emitting VS_CAPS#0 to UID %d (voice disabled)", client.Uid())
		client.SendPacket("VS_CAPS", "0", "1", "0", voiceCodec, strconv.Itoa(voiceSampleRate), strconv.Itoa(voiceFrameMs), strconv.Itoa(maxFrameBytes()))
		return
	}
	ptt := "0"
	if config.PTTOnly {
		ptt = "1"
	}
	maxPeers := strconv.Itoa(config.MaxPeersPerArea)
	mfb := strconv.Itoa(maxFrameBytes())
	logger.LogInfof("voice: emitting VS_CAPS#1#%s#%s#%s#%d#%d#%s to UID %d", ptt, maxPeers, voiceCodec, voiceSampleRate, voiceFrameMs, mfb, client.Uid())
	client.SendPacket("VS_CAPS", "1", ptt, maxPeers, voiceCodec, strconv.Itoa(voiceSampleRate), strconv.Itoa(voiceFrameMs), mfb)
}

// currentVoicePeers returns the set of UIDs currently in voice in the given
// area.  The caller receives a copy safe to iterate without holding voiceMu.
func currentVoicePeers(a *area.Area) []int {
	voiceMu.RLock()
	defer voiceMu.RUnlock()
	room, ok := voiceRooms[a]
	if !ok {
		return nil
	}
	out := make([]int, 0, len(room))
	for uid := range room {
		out = append(out, uid)
	}
	return out
}

// inVoiceRoom reports whether the given UID is currently joined to voice in a.
func inVoiceRoom(a *area.Area, uid int) bool {
	voiceMu.RLock()
	defer voiceMu.RUnlock()
	if room, ok := voiceRooms[a]; ok {
		_, exists := room[uid]
		return exists
	}
	return false
}

// voiceRoomSize returns the current number of peers in a's voice room.
func voiceRoomSize(a *area.Area) int {
	voiceMu.RLock()
	defer voiceMu.RUnlock()
	return len(voiceRooms[a])
}

// addVoicePeer inserts uid into a's voice room.  Returns false if the room
// is already full.
func addVoicePeer(a *area.Area, uid int, max int) bool {
	voiceMu.Lock()
	defer voiceMu.Unlock()
	room, ok := voiceRooms[a]
	if !ok {
		room = make(map[int]struct{})
		voiceRooms[a] = room
	}
	if max > 0 && len(room) >= max {
		if _, already := room[uid]; !already {
			return false
		}
	}
	room[uid] = struct{}{}
	return true
}

// removeVoicePeer removes uid from a's voice room.  Returns true if the UID
// was actually in the room.  Empty rooms are reaped from the map.
func removeVoicePeer(a *area.Area, uid int) bool {
	voiceMu.Lock()
	defer voiceMu.Unlock()
	room, ok := voiceRooms[a]
	if !ok {
		return false
	}
	if _, exists := room[uid]; !exists {
		return false
	}
	delete(room, uid)
	if len(room) == 0 {
		delete(voiceRooms, a)
	}
	return true
}

// leaveVoiceForClient removes the client from any voice room it's in and
// broadcasts VS_LEAVE to the affected area.  Safe to call on disconnect or
// area change whether or not the client was actually in voice.
func leaveVoiceForClient(client *Client) {
	if client == nil {
		return
	}
	a := client.Area()
	if a == nil {
		return
	}
	if !removeVoicePeer(a, client.Uid()) {
		return
	}
	uidStr := strconv.Itoa(client.Uid())
	writeToAreaVoice(a, client.Uid(), "VS_LEAVE", uidStr)
}

// writeToAreaVoice sends a packet to every client in a's voice room, optionally
// skipping the sender.
func writeToAreaVoice(a *area.Area, skipUID int, header string, contents ...string) {
	peers := currentVoicePeers(a)
	if len(peers) == 0 {
		return
	}
	for _, uid := range peers {
		if uid == skipUID {
			continue
		}
		c := clients.GetClientByUID(uid)
		if c != nil {
			c.SendPacket(header, contents...)
		}
	}
}

// Handles VS_JOIN#<uid>#%
//
// The uid argument is accepted for protocol symmetry with VS_LEAVE but is
// always overridden with client.Uid() to prevent spoofing.
func pktVSJoin(client *Client, _ *packet.Packet) {
	if !voiceEnabled() {
		client.SendServerMessage("Voice chat is disabled on this server.")
		return
	}
	if client.IsJailed() || !client.CanSpeakOOC() {
		client.SendServerMessage("You cannot use voice chat right now.")
		return
	}
	a := client.Area()
	if a == nil {
		return
	}
	if !a.VoiceAllowed() {
		client.SendServerMessage("Voice chat is not permitted in this area.")
		return
	}
	if banned, remaining, reason := IsVoiceBanned(client.Ipid()); banned {
		msg := "You are banned from voice chat."
		if remaining > 0 {
			msg += fmt.Sprintf(" (%ds remaining)", remaining)
		}
		if reason != "" {
			msg += " Reason: " + reason
		}
		client.SendServerMessage(msg)
		return
	}
	if muted, remaining, reason := IsVoiceMuted(client.Ipid()); muted {
		msg := "You are muted from voice chat."
		if remaining > 0 {
			msg += fmt.Sprintf(" (%ds remaining)", remaining)
		}
		if reason != "" {
			msg += " Reason: " + reason
		}
		client.SendServerMessage(msg)
		return
	}
	if wait := touchVoiceFirstSeen(client.Ipid()); wait > 0 {
		client.SendServerMessage(fmt.Sprintf("New users must wait %ds before using voice chat.", wait))
		return
	}
	uid := client.Uid()
	if inVoiceRoom(a, uid) {
		// Client is retrying while already joined.  Re-send the current peer
		// list so they can re-render the panel without consuming a rate-limit
		// slot or re-adding themselves to the room.
		peers := currentVoicePeers(a)
		peerStrs := make([]string, 0, len(peers))
		for _, p := range peers {
			if p == uid {
				continue
			}
			peerStrs = append(peerStrs, strconv.Itoa(p))
		}
		client.SendPacket("VS_PEERS", strings.Join(peerStrs, ","))
		return
	}
	if ok, retry := allowVoiceJoin(uid); !ok {
		client.SendServerMessage(fmt.Sprintf("Voice join rate limit: try again in %ds.", retry))
		return
	}
	if !addVoicePeer(a, uid, config.MaxPeersPerArea) {
		client.SendServerMessage("Voice chat is full in this area.")
		return
	}
	peers := currentVoicePeers(a)
	peerStrs := make([]string, 0, len(peers))
	for _, p := range peers {
		if p == uid {
			continue
		}
		peerStrs = append(peerStrs, strconv.Itoa(p))
	}
	client.SendPacket("VS_PEERS", strings.Join(peerStrs, ","))
	writeToAreaVoice(a, uid, "VS_JOIN", strconv.Itoa(uid))
}

// Handles VS_LEAVE#%
func pktVSLeave(client *Client, _ *packet.Packet) {
	if client.Area() == nil {
		return
	}
	leaveVoiceForClient(client)
}

// Handles VS_FRAME#<b64_opus>#%
//
// The payload is treated as an opaque base64 string.  The server enforces a
// length cap and a per-UID frame rate limit, then broadcasts to every other
// joined peer in the same area as VS_AUDIO#<from_uid>#<b64_opus>#%.
//
// A future voice-effects step (e.g. pitch-shift punishments) would decode
// the Opus payload between the rate-limit check and the broadcast call.
// That would introduce CGO and a per-frame DSP cost, so it's intentionally
// out of scope here — the relay stays codec-agnostic.
func pktVSFrame(client *Client, p *packet.Packet) {
	if !voiceEnabled() || len(p.Body) < 1 {
		return
	}
	a := client.Area()
	if a == nil {
		return
	}
	uid := client.Uid()
	if !inVoiceRoom(a, uid) {
		return
	}
	if len(p.Body[0]) > maxFrameBytes() {
		// Oversized frames are dropped silently — a chatty client may be
		// trying to flood and we don't want to give it back-pressure
		// signal.  Logged at debug-only level for the same reason.
		return
	}
	if ok, _ := allowVoiceFrame(uid); !ok {
		return
	}
	writeToAreaVoice(a, uid, "VS_AUDIO", strconv.Itoa(uid), p.Body[0])
}

// Handles VS_SPEAK#<on_off>#%  (0 = stopped talking, 1 = started)
func pktVSSpeak(client *Client, p *packet.Packet) {
	if !voiceEnabled() || len(p.Body) < 1 {
		return
	}
	a := client.Area()
	if a == nil || !inVoiceRoom(a, client.Uid()) {
		return
	}
	state := "0"
	if strings.TrimSpace(p.Body[0]) == "1" {
		state = "1"
	}
	writeToAreaVoice(a, client.Uid(), "VS_SPEAK", strconv.Itoa(client.Uid()), state)
}
