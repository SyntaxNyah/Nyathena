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

// Voice chat relay.
//
// The server never handles media — it only forwards WebRTC signalling
// (SDP offers/answers, ICE candidates) between peers in the same area.
// Topology is full-mesh per area; peers negotiate directly.
//
// Protocol:
//   VC_CAPS#<enabled>#<ptt_only>#<max_peers>#<ice_json>#%   (S→C, on handshake)
//   VC_JOIN#<uid>#%                                         (C→S, then broadcast to area)
//   VC_LEAVE#<uid>#%                                        (C→S or server-generated)
//   VC_PEERS#<csv_uids>#%                                   (S→C, current voice peers in area)
//   VC_SIG#<from_uid>#<to_uid>#<b64_payload>#%              (C→S→target; opaque blob)
//   VC_SPEAK#<uid>#<on_off>#%                               (C→S→area; speaking indicator)
//
// Only joined clients may send voice packets.  Muted or jailed clients
// may not join voice or send signalling.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

// LogVoiceConfig prints the decoded [Voice] config at startup so operators
// can see at a glance whether the section was loaded.  Called once from
// NewServer, after `config` is wired to the package-level singleton.
func LogVoiceConfig() {
	if config == nil {
		logger.LogInfo("voice: config is nil at startup (this is a bug)")
		return
	}
	logger.LogInfof("voice: enable_voice=%v ptt_only=%v max_peers_per_area=%d stun_servers=%d turn_servers=%d default_area_voice_allowed=%v",
		config.EnableVoice,
		config.PTTOnly,
		config.MaxPeersPerArea,
		len(config.STUNServers),
		len(config.TURNServers),
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

// voiceICEConfigJSON returns the JSON blob advertised to clients describing
// the ICE servers they should use.  Clients pass this directly into
// RTCPeerConnection's configuration.
func voiceICEConfigJSON() string {
	if !voiceEnabled() {
		return "[]"
	}
	type iceServer struct {
		URLs       []string `json:"urls"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
	}
	var servers []iceServer
	for _, s := range config.STUNServers {
		if s = strings.TrimSpace(s); s != "" {
			servers = append(servers, iceServer{URLs: []string{s}})
		}
	}
	if len(config.TURNServers) > 0 {
		var urls []string
		for _, s := range config.TURNServers {
			if s = strings.TrimSpace(s); s != "" {
				urls = append(urls, s)
			}
		}
		if len(urls) > 0 {
			servers = append(servers, iceServer{
				URLs:       urls,
				Username:   config.TURNUsername,
				Credential: config.TURNCredential,
			})
		}
	}
	b, err := json.Marshal(servers)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// sendVoiceCaps informs the client whether voice is available and, if so,
// the ICE configuration and PTT policy.  Safe to call with voice disabled;
// in that case the client is told the feature is off and should not render
// voice UI.
func sendVoiceCaps(client *Client) {
	if !voiceEnabled() {
		logger.LogInfof("voice: emitting VC_CAPS#0#1#0#[]#%% to UID %d (voice disabled)", client.Uid())
		client.SendPacket("VC_CAPS", "0", "1", "0", "[]")
		return
	}
	ptt := "0"
	if config.PTTOnly {
		ptt = "1"
	}
	maxPeers := strconv.Itoa(config.MaxPeersPerArea)
	iceJSON := voiceICEConfigJSON()
	logger.LogInfof("voice: emitting VC_CAPS#1#%s#%s#%s#%% to UID %d", ptt, maxPeers, iceJSON, client.Uid())
	client.SendPacket("VC_CAPS", "1", ptt, maxPeers, iceJSON)
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
// broadcasts VC_LEAVE to the affected area.  Safe to call on disconnect or
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
	writeToArea(a, "VC_LEAVE", uidStr)
}

// writeToAreaVoice sends a packet to every client in a's voice room, optionally
// skipping the sender.
func writeToAreaVoice(a *area.Area, skipUID int, header string, contents ...string) {
	peers := currentVoicePeers(a)
	if len(peers) == 0 {
		return
	}
	skip := make(map[int]struct{}, 1)
	skip[skipUID] = struct{}{}
	for _, uid := range peers {
		if _, s := skip[uid]; s {
			continue
		}
		c := clients.GetClientByUID(uid)
		if c != nil {
			c.SendPacket(header, contents...)
		}
	}
}

// Handles VC_JOIN#<uid>#%
//
// The uid argument is accepted for protocol symmetry with VC_LEAVE but is
// always overridden with client.Uid() to prevent spoofing.
func pktVCJoin(client *Client, _ *packet.Packet) {
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
	if ok, retry := allowVoiceJoin(client.Uid()); !ok {
		client.SendServerMessage(fmt.Sprintf("Voice join rate limit: try again in %ds.", retry))
		return
	}
	uid := client.Uid()
	if inVoiceRoom(a, uid) {
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
	client.SendPacket("VC_PEERS", strings.Join(peerStrs, ","))
	writeToAreaVoice(a, uid, "VC_JOIN", strconv.Itoa(uid))
}

// Handles VC_LEAVE#%
func pktVCLeave(client *Client, _ *packet.Packet) {
	if client.Area() == nil {
		return
	}
	leaveVoiceForClient(client)
}

// Handles VC_SIG#<to_uid>#<payload>#%
//
// Signalling is relayed to the target peer only if both sender and target are
// in the same area's voice room.  The payload is an opaque base64 blob — the
// server does not parse it.
func pktVCSig(client *Client, p *packet.Packet) {
	if !voiceEnabled() {
		return
	}
	if len(p.Body) < 2 {
		return
	}
	a := client.Area()
	if a == nil {
		return
	}
	fromUID := client.Uid()
	if !inVoiceRoom(a, fromUID) {
		return
	}
	if ok, _ := allowVoiceSig(fromUID); !ok {
		// Silently drop — signalling bursts are noisy and a message per drop
		// would spam the sender.  A misbehaving client will see failed ICE.
		return
	}
	toUID, err := strconv.Atoi(strings.TrimSpace(p.Body[0]))
	if err != nil || toUID == fromUID {
		return
	}
	if !inVoiceRoom(a, toUID) {
		return
	}
	target := clients.GetClientByUID(toUID)
	if target == nil || target.Area() != a {
		return
	}
	target.SendPacket("VC_SIG", strconv.Itoa(fromUID), p.Body[1])
}

// Handles VC_SPEAK#<on_off>#%  (0 = stopped talking, 1 = started)
func pktVCSpeak(client *Client, p *packet.Packet) {
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
	writeToAreaVoice(a, client.Uid(), "VC_SPEAK", strconv.Itoa(client.Uid()), state)
}
