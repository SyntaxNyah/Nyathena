# Nyathena Voice Chat — Developer Guide

> A complete technical reference for aspiring developers explaining how voice chat works in Nyathena: the server-side AO2 relay written in Go.

---

## Table of Contents

1. [What Is Nyathena?](#what-is-nyathena)
2. [Architecture Overview](#architecture-overview)
3. [The AO2 Packet Protocol](#the-ao2-packet-protocol)
4. [Full Connection Handshake](#full-connection-handshake)
5. [Voice Chat Deep Dive](#voice-chat-deep-dive)
   - [Voice Capabilities Advertisement](#voice-capabilities-advertisement)
   - [Client Joins the Voice Room](#client-joins-the-voice-room)
   - [Relaying Audio Frames](#relaying-audio-frames)
   - [Speaking State Relay](#speaking-state-relay)
   - [Client Leaves the Voice Room](#client-leaves-the-voice-room)
6. [Voice Punishments (Moderation Hooks)](#voice-punishments-moderation-hooks)
7. [Rate Limiting & Safety Checks](#rate-limiting--safety-checks)
8. [Moderation Commands](#moderation-commands)
9. [Configuration Reference](#configuration-reference)
10. [Complete Packet Reference](#complete-packet-reference)
11. [Full Sequence Diagram](#full-sequence-diagram)
12. [Key Source Files](#key-source-files)
13. [Glossary](#glossary)

---

## What Is Nyathena?

Nyathena is a **Go-based server** for the Attorney Online 2 (AO2) roleplaying game. It handles everything the clients need: game state, character management, in-character chat, music playback — and **voice chat relaying**.

Key facts:
- It is **only a server** — clients (like LemmyAO) connect to it.
- It supports connections over **plain TCP**, **plain WebSocket (ws://)**, and **secure WebSocket (wss://)**.
- Voice chat is **server-relayed**: audio frames pass through Nyathena, which forwards them to all other clients in the same area's voice room.
- The server **never decodes audio**. It handles Opus frames as opaque base64 blobs.
- This is a fork of the upstream Athena project, extended with moderation, casino system, minigames, and enhanced voice features.

---

## Architecture Overview

```
Client A (LemmyAO)
    |
    |  WebSocket (ws:// or wss://)
    |
Nyathena Server
    |
    |  WebSocket (server → other clients in same area)
    |
Client B, Client C, Client D...
```

Nyathena maintains:
- A list of **clients** (each with a UID, HDID, IPID, and punishment flags)
- A list of **areas** (each with state like background, music, evidence, and a voice room)
- A **voice room per area** — just a set of UIDs of who is currently voice-chatting there

When a client sends a `VS_FRAME`, Nyathena looks up the area's voice room and forwards the frame to every other UID in that room.

### Why No WebRTC?

Server-relayed audio is a deliberate design:
1. **Privacy**: Clients never exchange IP addresses — the server is the only network endpoint.
2. **Moderation**: The server can inspect, gate, or modify frame flow (voice punishments).
3. **Simplicity**: No STUN/TURN infrastructure, no ICE negotiation, no SDP handshakes.

---

## The AO2 Packet Protocol

### Format

Every message follows this text format:

```
COMMAND#arg1#arg2#...#argN#%
```

- `COMMAND` — packet type identifier (e.g. `HI`, `VS_JOIN`, `VS_FRAME`)
- `#` — field separator
- `%` — packet terminator
- Fields are plain UTF-8 strings

### Examples

```
HI#abc123def#%
VS_CAPS#1#0#20#opus#48000#20#4000#%
VS_AUDIO#42#//8NBJpa9Aa...==#%
```

### Where This Is Implemented

**File**: `internal/packet/aopacket.go`

Packets are parsed by splitting the raw text on `#`, reading the first element as the command, and passing remaining elements to the registered handler.

**File**: `internal/athena/netprotocol.go:123`

```go
var PacketMap = map[string]func(*Client, *packet.Packet){
  "HI":       pktHi,
  "ID":       pktId,
  "RD":       pktReqDone,
  "VS_JOIN":  pktVSJoin,
  "VS_FRAME": pktVSFrame,
  "VS_LEAVE": pktVSLeave,
  "VS_SPEAK": pktVSSpeak,
  // ... 40+ more handlers
}
```

---

## Full Connection Handshake

When a client connects and completes the handshake, voice capabilities are announced twice. Here is every step.

### Step 1 — Network Connection

**File**: `internal/athena/server.go`

The server accepts connections on three listeners running as goroutines:

```go
go athena.ListenTCP()   // AO2 binary protocol on TCP (default port 27016)
go athena.ListenWS()    // Plain WebSocket (default port 27017)
go athena.ListenWSS()   // TLS WebSocket (port 443 when enabled)
```

When a connection is accepted, a `Client` struct is created and a goroutine is spawned to read packets from it.

---

### Step 2 — Client Sends HI

```
Client → Server:  HI#<hdid>#%
```

**File**: `internal/athena/netprotocol.go` (`pktHi`)

The server receives the Hardware Device ID (HDID). It MD5-hashes it to generate the player's **IPID** and stores both. The server then responds with its own identification.

---

### Step 3 — Client Sends ID

```
Client → Server:  ID#<software>#<version>#%
```

**File**: `internal/athena/netprotocol.go:170` (`pktId`)

This is one of the most important steps. When the server receives `ID`, it:

1. Checks if the server is full.
2. Sends `PN` (player count): `PN#<current>#<max>#%`
3. Sends `FL` (feature list): `FL#noencryption#yellowtext#...#%`
4. Optionally sends `ASS` (asset server URL) if configured.
5. **Sends `VS_CAPS` for the first time** — announcing voice capabilities to the client before the full handshake completes.

```go
func pktId(client *Client, _ *packet.Packet) {
    // ... player count check ...
    client.Send(&packet.PN{...})
    client.Send(&packet.FL{Features: [...]})
    if config.AssetURL != "" {
        client.Send(&packet.ASS{AssetURL: config.AssetURL})
    }
    sendVoiceCaps(client)  // First VS_CAPS
}
```

---

### Step 4 — Server Sends VS_CAPS (First Time)

```
Server → Client:  VS_CAPS#1#0#20#opus#48000#20#4000#%
```

See [Voice Capabilities Advertisement](#voice-capabilities-advertisement) for full details.

This is sent early because the client may want to know voice settings immediately. However, some clients (like webAO / LemmyAO) don't build their voice subsystem until after the handshake fully completes, so it is sent **again** later.

---

### Step 5 — Client Requests Character List

```
Client → Server:  askchaa#%
```

The server responds with character count and the list:

```
Server → Client:  SI#<char_count>#<evidence_count>#<music_count>#%
Server → Client:  SC#<char1>&<char2>&...#%
Server → Client:  SM#<music1>&<music2>&...#%
```

---

### Step 6 — Client Sends RD (Ready / Request Done)

```
Client → Server:  RD#%
```

**File**: `internal/athena/netprotocol.go:240` (`pktReqDone`)

This is the final handshake step. When the server receives `RD`, it:

1. Assigns the client a **UID** from the pool.
2. Registers the client in the UID index.
3. Moves the client into Area 0.
4. Sends `DONE#%` to signal handshake completion.
5. Sends `BN#<background>#%` (current area background).
6. **Sends `VS_CAPS` for the second time** — with UID now assigned.

```go
func pktReqDone(client *Client, _ *packet.Packet) {
    client.SetUid(uids.GetUid())
    clients.RegisterUID(client)
    client.JoinArea(areas[0])
    client.Send(&packet.DONE{})
    client.Send(&packet.BN{Background: areas[0].Background()})

    // Re-emit VS_CAPS after DONE.
    // pktId sends it once during the early ID phase, but some clients
    // (notably webAO) ignore packets that arrive before their voice
    // subsystem is built. Sending it again after DONE guarantees the
    // voice panel can render whether or not the client caught the first copy.
    sendVoiceCaps(client)
}
```

---

### Handshake Summary

```
Client                          Nyathena
  |                               |
  |-- TCP/WebSocket connect ----->|
  |-- HI#<hdid>#% -------------->|
  |<------------ [server logs HDID, generates IPID]
  |-- ID#<software>#<version>#% ->|
  |<------------ PN#<count>#<max>#%
  |<------------ FL#features#%
  |<------------ ASS#<url>#%      (if configured)
  |<------------ VS_CAPS#1#...#%  (FIRST voice capabilities)
  |-- askchaa#% ---------------->|
  |<------------ SI#<counts>#%
  |<------------ SC#<chars>#%
  |<------------ SM#<music>#%
  |-- RD#% --------------------->|
  |<------------ DONE#%
  |<------------ BN#<background>#%
  |<------------ VS_CAPS#1#...#%  (SECOND voice capabilities, after DONE)
  |
  [Handshake complete. Player is in Area 0.]
```

---

## Voice Chat Deep Dive

All voice chat logic lives in:
- `internal/athena/voice.go` — core relay, room management, packet handlers
- `internal/athena/voice_mod.go` — IPID bans/mutes, rate limiting, cooldowns
- `internal/athena/voice_punishments.go` — per-client audio effects
- `internal/athena/voice_commands.go` — moderator commands

---

### Voice Capabilities Advertisement

**File**: `internal/athena/voice.go:106`

```go
func sendVoiceCaps(client *Client) {
    if !voiceEnabled() {
        client.Send(&packet.VSCaps{
            Enabled: "0", PTT: "1", MaxPeers: "0",
            Codec: voiceCodec, SampleRate: voiceSampleRate,
            FrameMs: voiceFrameMs, MaxFrameBytes: maxFrameBytes(),
        })
        return
    }
    ptt := "0"
    if config.PTTOnly {
        ptt = "1"
    }
    maxPeers := strconv.Itoa(config.MaxPeersPerArea)
    client.Send(&packet.VSCaps{
        Enabled: "1", PTT: ptt, MaxPeers: maxPeers,
        Codec: voiceCodec, SampleRate: voiceSampleRate,
        FrameMs: voiceFrameMs, MaxFrameBytes: maxFrameBytes(),
    })
}
```

#### Hard-Coded Audio Parameters

```go
const (
    voiceCodec      = "opus"   // Always Opus
    voiceSampleRate = 48000    // Always 48 kHz
    voiceFrameMs    = 20       // Always 20ms frames
)
```

These match the Opus standard for voice. The client is expected to use exactly these values.

#### VS_CAPS Packet Format

```
VS_CAPS#<enabled>#<ptt>#<max_peers>#<codec>#<sample_rate>#<frame_ms>#<max_frame_bytes>#%
```

| Field | Type | Example | Meaning |
|-------|------|---------|---------|
| `enabled` | "0" or "1" | `1` | Voice relay active |
| `ptt` | "0" or "1" | `0` | Push-to-talk required? |
| `max_peers` | integer string | `20` | Max clients per area voice room |
| `codec` | string | `opus` | Audio codec |
| `sample_rate` | integer string | `48000` | Samples per second |
| `frame_ms` | integer string | `20` | Milliseconds per frame |
| `max_frame_bytes` | integer string | `4000` | Max encoded frame size in bytes |

---

### Client Joins the Voice Room

**File**: `internal/athena/voice.go:258` (`pktVSJoin`)

This is triggered when the server receives `VS_JOIN#%` from a client.

#### Full Handler Logic

```go
func pktVSJoin(client *Client, _ *packet.Packet) {
    // 1. Is voice even enabled on this server?
    if !voiceEnabled() {
        client.SendServerMessage("Voice chat is disabled on this server.")
        return
    }

    // 2. Is the client in a state where they can speak?
    if client.IsJailed() || !client.CanSpeakOOC() {
        client.SendServerMessage("You cannot use voice chat right now.")
        return
    }

    a := client.Area()
    if a == nil { return }

    // 3. Is voice allowed in this specific area?
    if !a.VoiceAllowed() {
        client.SendServerMessage("Voice chat is not permitted in this area.")
        return
    }

    // 4. Is this IPID voice-banned?
    if banned, remaining, reason := IsVoiceBanned(client.Ipid()); banned {
        // ... format and send ban message ...
        return
    }

    // 5. Is this IPID voice-muted?
    if muted, remaining, reason := IsVoiceMuted(client.Ipid()); muted {
        // ... format and send mute message ...
        return
    }

    // 6. Is this a brand new IPID that hasn't waited long enough?
    if wait := touchVoiceFirstSeen(client.Ipid()); wait > 0 {
        client.SendServerMessage(fmt.Sprintf("New users must wait %ds before using voice chat.", wait))
        return
    }

    uid := client.Uid()

    // 7. Retry: already in voice? Just resend peer list.
    if inVoiceRoom(a, uid) {
        peers := currentVoicePeers(a)
        others := filterOutSelf(peers, uid)
        client.Send(&packet.VSPeers{UIDs: others})
        return
    }

    // 8. Per-UID join rate limit
    if ok, retry := allowVoiceJoin(uid); !ok {
        client.SendServerMessage(fmt.Sprintf("Voice join rate limit: try again in %ds.", retry))
        return
    }

    // 9. Is the room full?
    if !addVoicePeer(a, uid, config.MaxPeersPerArea) {
        client.SendServerMessage("Voice chat is full in this area.")
        return
    }

    // === Success ===

    // Send this client the list of current peers (everyone except themselves)
    peers := currentVoicePeers(a)
    others := filterOutSelf(peers, uid)
    client.Send(&packet.VSPeers{UIDs: others})

    // Tell everyone else in the voice room that this client joined
    broadcastToAreaVoice(a, uid, &packet.VSJoinOut{UID: uid})
}
```

#### State After Success

- Client's UID is added to the area's `voiceRooms` set.
- Client receives: `VS_PEERS#<uid1>,<uid2>,...#%`
- All other voice room members receive: `VS_JOIN#<new_uid>#%`

#### Voice Room Data Structure

**File**: `internal/athena/voice.go:83`

```go
var (
    voiceMu    sync.RWMutex
    voiceRooms = map[*area.Area]map[int]struct{}{}
    // area pointer → set of UIDs in that area's voice room
)
```

This is protected by a `sync.RWMutex` for thread safety since multiple clients may join simultaneously.

---

### Relaying Audio Frames

**File**: `internal/athena/voice.go:349` (`pktVSFrame`)

This is triggered when the server receives `VS_FRAME#<base64_opus>#%` from a client. It fires roughly 50 times per second per speaking client.

```go
func pktVSFrame(client *Client, p *packet.Packet) {
    if !voiceEnabled() { return }

    vf, err := packet.ParseVSFrame(p.Body)
    if err != nil { return }

    a := client.Area()
    if a == nil { return }

    uid := client.Uid()

    // Must be in voice room to relay
    if !inVoiceRoom(a, uid) { return }

    // Drop frames that exceed the size cap
    if len(vf.Payload) > maxFrameBytes() { return }

    // Per-UID frame rate limit (default 50 frames/sec max)
    if ok, _ := allowVoiceFrame(uid); !ok { return }

    // Apply voice punishments (may modify or drop the frame)
    payload, relay := applyVoiceFramePunishments(client, uid, vf.Payload)
    if !relay { return }

    // Relay to all voice room peers except the sender
    broadcastToAreaVoice(a, uid, &packet.VSAudio{FromUID: uid, Payload: payload})
}
```

#### What `broadcastToAreaVoice` Does

**File**: `internal/athena/voice.go` (helper function)

```go
func broadcastToAreaVoice(a *area.Area, excludeUID int, pkt packet.AOPacket) {
    voiceMu.RLock()
    defer voiceMu.RUnlock()
    room, ok := voiceRooms[a]
    if !ok { return }
    for uid := range room {
        if uid == excludeUID { continue }  // Don't echo back to sender
        if c := clients.GetByUID(uid); c != nil {
            c.Send(pkt)
        }
    }
}
```

#### The Relayed Packet

```
Server → peers:  VS_AUDIO#<from_uid>#<base64_opus>#%
```

The `from_uid` is added by the server — clients send frames without identifying themselves, and the server stamps the sender's UID onto the outgoing packet. This prevents spoofing.

---

### Speaking State Relay

**File**: `internal/athena/voice.go:396` (`pktVSSpeak`)

When a client sends `VS_SPEAK#1#%` (started speaking) or `VS_SPEAK#0#%` (stopped), the server rebroadcasts it to all voice room peers with the sender's UID attached.

```go
func pktVSSpeak(client *Client, p *packet.Packet) {
    if !voiceEnabled() { return }

    vs, err := packet.ParseVSSpeak(p.Body)
    if err != nil { return }

    a := client.Area()
    if a == nil || !inVoiceRoom(a, client.Uid()) { return }

    state := "0"
    if vs.On { state = "1" }

    broadcastToAreaVoice(a, client.Uid(), &packet.VSSpeakOut{
        UID: client.Uid(),
        On:  state,
    })
}
```

```
Client → Server:  VS_SPEAK#1#%           (I started speaking)
Server → peers:   VS_SPEAK#<uid>#1#%     (UID is speaking)
```

This packet drives the speaking indicator UI in clients. No audio processing — just a state change notification.

---

### Client Leaves the Voice Room

**File**: `internal/athena/voice.go:341` (`pktVSLeave`)

Triggered when the server receives `VS_LEAVE#%` from a client.

```go
func pktVSLeave(client *Client, _ *packet.Packet) {
    if client.Area() == nil { return }
    leaveVoiceForClient(client)
}

func leaveVoiceForClient(client *Client) {
    if client == nil { return }
    a := client.Area()
    if a == nil { return }
    if !removeVoicePeer(a, client.Uid()) { return }  // Remove from room set
    broadcastToAreaVoice(a, client.Uid(), &packet.VSLeaveOut{UID: client.Uid()})
}
```

```
Client → Server:  VS_LEAVE#%
Server → peers:   VS_LEAVE#<uid>#%    (broadcast departure)
```

`leaveVoiceForClient` is also called automatically when:
- The client disconnects from the server.
- The client moves to a different area.
- A moderator uses `/vkick`.
- The area has voice disabled via `/voicearea off`.

---

## Voice Punishments (Moderation Hooks)

**File**: `internal/athena/voice_punishments.go:121`

Voice punishments are applied **per relayed frame** before broadcasting. The server never decodes the audio — punishments work by controlling frame **flow**, not audio content.

```go
func applyVoiceFramePunishments(
    client *Client,
    uid int,
    frame string,
) (payload string, relay bool) {

    set := client.activeVoicePunishments()
    if !set.any() {
        return frame, true  // No punishment active, relay unchanged
    }

    // voicemute: drop every single frame (client appears silent to room)
    if set.mute {
        return "", false
    }

    // voicestatic (~60% drop) and voicegarble (~88% drop)
    // Use the higher drop chance if both are active
    dropChance := 0.0
    if set.static && voiceStaticDropChance > dropChance {
        dropChance = voiceStaticDropChance  // 0.60
    }
    if set.garble && voiceGarbleDropChance > dropChance {
        dropChance = voiceGarbleDropChance  // 0.88
    }
    if dropChance > 0 && rand.Float64() < dropChance {
        return "", false  // Drop this frame
    }

    // voicecutout: gate on a ~650ms on/off cycle (walkie-talkie sound)
    if set.cutout && voiceCutoutMuted(uid) {
        return "", false
    }

    // voicestutter: replay previous frame ~45% of the time
    // This makes speech sound repetitive/stuck
    if set.stutter {
        out := frame
        if held, ok := getStutterFrame(uid); ok && rand.Float64() < voiceStutterChance {
            out = held  // Replace with last frame
        }
        setStutterFrame(uid, frame)
        return out, true
    }

    return frame, true
}
```

### Punishment Summary

| Punishment | Effect | How It Works |
|-----------|--------|-------------|
| `voicemute` | Complete silence | Every frame dropped |
| `voicestatic` | Heavy static | 60% of frames dropped randomly |
| `voicegarble` | Near-unintelligible | 88% of frames dropped randomly |
| `voicecutout` | Walkie-talkie stutter | 650ms on/off gate cycle |
| `voicestutter` | Stuck/repeating speech | 45% chance to replay previous frame |

These stack. If a client has both `voicestatic` and `voicecutout`, both checks apply.

Importantly, the server never decodes the audio to apply these effects. It only controls whether a frame gets forwarded, or which frame gets forwarded. This keeps the relay **codec-agnostic** and requires **no native audio libraries**.

---

## Rate Limiting & Safety Checks

**File**: `internal/athena/voice_mod.go`

### Per-UID Join Rate Limit

```go
func allowVoiceJoin(uid int) (bool, int) {
    now := time.Now()
    events := voiceJoinEvents[uid]
    // Trim events older than the window
    cutoff := now.Add(-time.Duration(config.JoinRateLimitWindow) * time.Second)
    fresh := events[:0]
    for _, t := range events {
        if t.After(cutoff) {
            fresh = append(fresh, t)
        }
    }
    if len(fresh) >= config.JoinRateLimit {
        // Too many joins — calculate wait time
        oldest := fresh[0]
        wait := int(config.JoinRateLimitWindow) - int(now.Sub(oldest).Seconds())
        return false, wait
    }
    voiceJoinEvents[uid] = append(fresh, now)
    return true, 0
}
```

Default config: 5 joins per 10-second window.

### Per-UID Frame Rate Limit

Same sliding-window approach for `VS_FRAME` packets. Default: 50 frames per second. This caps bandwidth and prevents frame flooding attacks.

### New IPID Cooldown

```go
func touchVoiceFirstSeen(ipid string) int {
    voiceModMu.Lock()
    defer voiceModMu.Unlock()
    t, ok := voiceFirstSeen[ipid]
    if !ok {
        voiceFirstSeen[ipid] = time.Now()
        return config.NewIPIDVoiceCooldown  // e.g. 10 seconds
    }
    elapsed := int(time.Since(t).Seconds())
    remaining := config.NewIPIDVoiceCooldown - elapsed
    if remaining > 0 {
        return remaining
    }
    return 0
}
```

New IP addresses must wait a configurable number of seconds (default: 10) before joining voice. This deters quick join-and-harass attacks.

### IPID Voice Bans and Mutes

**File**: `internal/athena/voice_mod.go:39`

```go
type voiceRestriction struct {
    expires time.Time  // zero = permanent
    reason  string
}

var (
    voiceMutes = map[string]voiceRestriction{}  // ipid → restriction
    voiceBans  = map[string]voiceRestriction{}  // ipid → restriction
)
```

- **Mute**: Client can still be in the voice room, but their frames are rejected at the server level. Other clients cannot hear them.
- **Ban**: Client cannot join the voice room at all.

Both are scoped to IPID (a hash of the client's IP address), so they persist across reconnects.

---

## Moderation Commands

**File**: `internal/athena/voice_commands.go`

### IPID-Scoped (Persistent)

| Command | Effect |
|---------|--------|
| `/vmute <ipid> [-d <seconds>] [-r <reason>]` | Mute IPID from voice; ejects if currently in voice |
| `/vunmute <ipid>` | Lift voice mute |
| `/vban <ipid> [-d <seconds>] [-r <reason>]` | Ban IPID from voice; ejects if currently in voice |
| `/vunban <ipid>` | Lift voice ban |

### Per-UID (Live, No Persistence)

| Command | Effect |
|---------|--------|
| `/vkick <uid>` | Eject client from voice room immediately |
| `/vkick -i <ipid>` | Eject all clients matching an IPID |

### Area Control

| Command | Effect |
|---------|--------|
| `/voicearea on` | Enable voice chat for current area |
| `/voicearea off` | Disable voice chat and eject all current voice room members |

### Informational

| Command | Available To | Effect |
|---------|-------------|--------|
| `/vlist` | All players | Show who is in the area's voice room |
| `/vbans` | Moderators | List all active voice mutes and bans |

---

## Configuration Reference

Voice configuration lives under `[Voice]` in `config.toml`:

```toml
[Voice]
# Master toggle. Set to false to disable voice chat entirely.
enable_voice = true

# If true, clients are told PTT mode is required (advisory only — client enforces it).
ptt_only = false

# Maximum number of simultaneous voice speakers per area.
max_peers_per_area = 20

# Maximum encoded frame size in bytes. Frames larger than this are dropped.
# 4000 is a safe default for 20ms Opus frames at 24 kbps.
max_frame_bytes = 4000

# Whether new areas start with voice chat allowed.
default_area_voice_allowed = true

# Join rate limiting: max joins per window.
join_rate_limit = 5
join_rate_limit_window = 10  # seconds

# Frame rate limiting: max frames per window.
frame_rate_limit = 50
frame_rate_limit_window = 1  # seconds

# How many seconds a brand-new IPID must wait before joining voice.
new_ipid_voice_cooldown = 10
```

---

## Complete Packet Reference

### Server → Client

| Packet | Format | When Sent |
|--------|--------|-----------|
| `VS_CAPS` | `VS_CAPS#<enabled>#<ptt>#<maxPeers>#<codec>#<sampleRate>#<frameMs>#<maxBytes>#%` | Twice: after `ID` and after `RD` |
| `VS_PEERS` | `VS_PEERS#<csv_uids>#%` | After client's `VS_JOIN` succeeds |
| `VS_JOIN` | `VS_JOIN#<uid>#%` | Broadcast to room when a peer joins |
| `VS_LEAVE` | `VS_LEAVE#<uid>#%` | Broadcast to room when a peer leaves |
| `VS_SPEAK` | `VS_SPEAK#<uid>#<0_or_1>#%` | Broadcast when a peer changes speak state |
| `VS_AUDIO` | `VS_AUDIO#<from_uid>#<base64_opus>#%` | Broadcast of relayed audio frame |

### Client → Server

| Packet | Format | Meaning |
|--------|--------|---------|
| `VS_JOIN` | `VS_JOIN#%` | Request to join voice room |
| `VS_LEAVE` | `VS_LEAVE#%` | Leave voice room |
| `VS_FRAME` | `VS_FRAME#<base64_opus>#%` | Send 20ms encoded audio frame |
| `VS_SPEAK` | `VS_SPEAK#<0_or_1>#%` | Speaking state changed |

### Handshake Packets

| Packet | Direction | Format | Meaning |
|--------|-----------|--------|---------|
| `HI` | C→S | `HI#<hdid>#%` | Client identification |
| `ID` | C→S | `ID#<software>#<version>#%` | Software identification |
| `PN` | S→C | `PN#<current>#<max>#%` | Player count |
| `FL` | S→C | `FL#feature1#feature2#...#%` | Feature list |
| `DONE` | S→C | `DONE#%` | Handshake complete |
| `RD` | C→S | `RD#%` | Client ready |

---

## Full Sequence Diagram

```
Client                          Nyathena                     Voice Room Peers
  |                               |                               |
  |=== CONNECTION HANDSHAKE ============================================
  |                               |                               |
  |-- HI#<hdid>#% -------------->|                               |
  |-- ID#<software>#<version>#% ->|                               |
  |<---------- PN#<count>#<max>#%|                               |
  |<---------- FL#features#%     |                               |
  |<---------- VS_CAPS#1#0#20#...| (FIRST capabilities send)     |
  |-- askchaa#% ----------------->|                               |
  |<---------- SI + SC + SM      |                               |
  |-- RD#% ---------------------->|                               |
  |<---------- DONE#%            |                               |
  |<---------- BN#<bg>#%         |                               |
  |<---------- VS_CAPS#1#0#20#...| (SECOND capabilities send)    |
  |                               |                               |
  |=== VOICE JOIN ==================================================
  |                               |                               |
  |-- VS_JOIN#% ---------------->|                               |
  |  [server checks: enabled? not jailed? area allows? not banned?
  |   not muted? new-IPID cooldown? join rate limit? room not full?]
  |<---------- VS_PEERS#42,55#%  | (existing room: UIDs 42 and 55)
  |                               |-- VS_JOIN#<myUID>#% -------->|
  |                               |                     [peers create decoder for me]
  |                               |                               |
  |=== SPEAKING =====================================================
  |                               |                               |
  |-- VS_SPEAK#1#% ------------->|                               |
  |                               |-- VS_SPEAK#<myUID>#1#% ----->| [show mic icon]
  |                               |                               |
  |-- VS_FRAME#//aAbB...==#% --->|                               |
  | [server: in room? size ok? rate ok? punishments?]             |
  |                               |-- VS_AUDIO#<myUID>#//aAbB#% >| [decode+play]
  |-- VS_FRAME#//cCdD...==#% --->|                               |
  |                               |-- VS_AUDIO#<myUID>#//cCdD#% >|
  |-- VS_SPEAK#0#% ------------->|                               |
  |                               |-- VS_SPEAK#<myUID>#0#% ----->| [hide mic icon]
  |                               |                               |
  |=== RECEIVING FROM PEER 42 =======================================================
  |                               |                               |
  |                               |<-- VS_FRAME#//xXyY...#%     |  (peer 42 sends)
  |                               | [relay to room except 42]     |
  |<---------- VS_AUDIO#42#//xXyY|                               |
  |  [decode + play peer 42]      |                               |
  |<---------- VS_SPEAK#42#1#%   |                               |
  |  [show mic icon for 42]       |                               |
  |                               |                               |
  |=== PUNISHMENT EXAMPLE (voicestatic) ================================
  |                               |                               |
  |-- VS_FRAME#//aAbB...==#% --->|                               |
  |                 [rand() < 0.60 → DROPPED]                     |
  |-- VS_FRAME#//cCdD...==#% --->|                               |
  |                 [rand() >= 0.60 → RELAYED]                    |
  |                               |-- VS_AUDIO#<myUID>#//cCdD#% >|
  |                               |                               |
  |=== VOICE LEAVE ==================================================
  |                               |                               |
  |-- VS_LEAVE#% --------------->|                               |
  | [server: remove from room, broadcast leave]                   |
  |                               |-- VS_LEAVE#<myUID>#% ------->| [remove decoder]
  |                               |                               |
```

---

## Key Source Files

| File | What It Does |
|------|-------------|
| `internal/athena/voice.go` | Core voice relay: room management, `pktVSJoin`, `pktVSFrame`, `pktVSLeave`, `pktVSSpeak`, `sendVoiceCaps` |
| `internal/athena/voice_mod.go` | IPID bans/mutes, per-UID rate limits, new-IPID cooldowns |
| `internal/athena/voice_punishments.go` | Per-frame punishment hooks: mute, static, garble, cutout, stutter |
| `internal/athena/voice_commands.go` | Moderator commands: `/vmute`, `/vban`, `/vkick`, `/voicearea`, `/vlist` |
| `internal/athena/netprotocol.go` | Handshake handlers (`pktHi`, `pktId`, `pktReqDone`) and the `PacketMap` dispatch table |
| `internal/athena/client.go` | Client struct, punishment flags, `activeVoicePunishments()` |
| `internal/packet/types.go` | `VSCaps`, `VSPeers`, `VSJoinOut`, `VSLeaveOut`, `VSAudio`, `VSSpeakOut` packet structs |
| `internal/athena/server.go` | Server init, `ListenTCP()`, `ListenWS()`, `ListenWSS()` goroutines |
| `internal/settings/config.go` | Config struct including the `Voice` section |

---

## Glossary

| Term | Definition |
|------|-----------|
| **AO2** | Attorney Online 2 — the game protocol Nyathena implements |
| **Area** | A virtual courtroom/room. Players, evidence, and music are per-area. |
| **Base64** | Encoding that converts binary bytes to text characters (A-Z, 0-9, +, /) |
| **Broadcast** | Sending a packet to all members of a voice room |
| **HDID** | Hardware Device ID — a browser fingerprint from the client |
| **IPID** | Internet Protocol ID — an MD5 hash of the client's IP address |
| **Jitter** | Network timing variation that can cause audio gaps |
| **Opus** | Audio codec used here (same as Discord and WebRTC) |
| **PCM** | Pulse-Code Modulation — raw uncompressed audio samples |
| **PTT** | Push-to-Talk — microphone only active while a key is held |
| **Relay** | Server forwards packets from one client to others (not peer-to-peer) |
| **RWMutex** | Go's read-write mutex, allows multiple readers or one writer at a time |
| **Sample Rate** | How many audio samples per second (48,000 = 48 kHz) |
| **UID** | Unique ID — integer assigned to each client after the handshake |
| **Voice Room** | The set of UIDs currently participating in voice chat in an area |
| **WebRTC** | Browser peer-to-peer technology (NOT used here) |
| **WebSocket** | Bi-directional TCP-based connection used for all client communication |
