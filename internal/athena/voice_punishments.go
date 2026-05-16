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

// Voice-chat punishments — the moderator gag commands that act on the
// server-relayed voice chat, the audio counterpart to the ~90 text
// punishments.
//
// The voice relay forwards Opus frames as opaque base64 blobs and never
// decodes them, so a voice punishment cannot change how a voice *sounds*
// (no pitch-shift, no reverb — that would need an Opus codec and CGO, which
// the relay deliberately avoids).  What it can do is sabotage the punished
// speaker's frame *flow*: drop frames, gate them on a duty cycle, or replay
// a stale one.  Decoded by the listener, that lands as a believable "your
// mic is broken" effect while keeping the relay codec-agnostic and CGO-free.
//
//   /voicemute    — drop every frame; the speaker is silent to the room
//   /voicestatic  — drop ~60% of frames; choppy, breaking up
//   /voicegarble  — drop ~88% of frames; barely intelligible
//   /voicecutout  — gate frames on a ~650ms on/off cycle; walkie-talkie
//   /voicestutter — sometimes replay the previous frame; glitchy stutter
//
// They are ordinary PunishmentType values dispatched through the shared
// cmdPunishment handler, so they inherit -d/-r/-h, comma-separated UID
// lists, the "global" keyword, /stack, /unpunish (including issuer-tier
// self-removal protection), DB persistence and restore-on-reconnect with no
// extra plumbing.  The only voice-specific part is where the effect lands:
// applyVoiceFramePunishments, called from pktVSFrame between the per-frame
// rate-limit check and the broadcast — the hook the relay always documented.

import (
	"math/rand"
	"sync"
	"time"
)

// Tuning for the frame-flow effects.  Opus tolerates lost frames well (its
// decoder runs packet-loss concealment), so a dropped frame surfaces as an
// audible glitch rather than a hard decode error.
const (
	voiceStaticDropChance = 0.60 // /voicestatic — choppy
	voiceGarbleDropChance = 0.88 // /voicegarble — barely intelligible
	voiceStutterChance    = 0.45 // /voicestutter — chance a frame is replayed stale
	voiceCutoutWindowMs   = 650  // /voicecutout — on/off half-period, milliseconds
)

// voiceStutterHeld stores the most recent frame from each /voicestutter
// target so a later frame can be swapped for this stale copy.  Keyed by UID,
// guarded by voiceStutterMu, cleared on UID release (clearVoiceStutterState,
// wired into clearVoiceRateStateForUID) so it cannot grow unbounded.
var (
	voiceStutterMu   sync.Mutex
	voiceStutterHeld = map[int]string{}
)

// voicePunishmentSet is the set of voice-chat punishments active on one
// client at a given instant.
type voicePunishmentSet struct {
	mute    bool
	static  bool
	garble  bool
	cutout  bool
	stutter bool
}

func (s voicePunishmentSet) any() bool {
	return s.mute || s.static || s.garble || s.cutout || s.stutter
}

// activeVoicePunishments returns the voice-chat punishments currently in
// effect on the client.  It is expiry-aware but, unlike
// CheckExpiredAndGetPunishments, has no side effects — no DB writes, no
// "punishment expired" notice — so it is cheap enough for the per-frame
// voice hot path.  An expired punishment is simply skipped here so the
// effect stops exactly at expiry; the stale slice/DB row is pruned lazily
// the next time the player sends an IC message, matching how every other
// punishment expires.
func (client *Client) activeVoicePunishments() voicePunishmentSet {
	client.mu.Lock()
	defer client.mu.Unlock()
	var set voicePunishmentSet
	if len(client.punishments) == 0 {
		return set
	}
	now := time.Now().UTC()
	for i := range client.punishments {
		p := &client.punishments[i]
		if !p.expiresAt.IsZero() && now.After(p.expiresAt) {
			continue
		}
		switch p.punishmentType {
		case PunishmentVoiceMute:
			set.mute = true
		case PunishmentVoiceStatic:
			set.static = true
		case PunishmentVoiceGarble:
			set.garble = true
		case PunishmentVoiceCutout:
			set.cutout = true
		case PunishmentVoiceStutter:
			set.stutter = true
		}
	}
	return set
}

// applyVoiceFramePunishments is the pktVSFrame voice-effects hook.  Given the
// sending client's active voice punishments it decides what happens to one
// inbound Opus frame: relay it (possibly swapped for a stale frame), or drop
// it.  It returns the payload to broadcast and whether to broadcast at all.
//
// Every effect here only ever drops or substitutes a frame — none duplicate
// it — so a voice punishment can never be turned into an amplifier that
// pushes peers past the frame rate limit.
func applyVoiceFramePunishments(client *Client, uid int, frame string) (payload string, relay bool) {
	set := client.activeVoicePunishments()
	if !set.any() {
		return frame, true
	}

	// Total mute outranks everything: not a single frame escapes.
	if set.mute {
		return "", false
	}

	// Random-drop family.  /voicestatic and /voicegarble each have a drop
	// chance; if both are stacked the harsher one wins.
	dropChance := 0.0
	if set.static && voiceStaticDropChance > dropChance {
		dropChance = voiceStaticDropChance
	}
	if set.garble && voiceGarbleDropChance > dropChance {
		dropChance = voiceGarbleDropChance
	}
	if dropChance > 0 && rand.Float64() < dropChance {
		return "", false
	}

	// /voicecutout — gate the frame on a walkie-talkie duty cycle.
	if set.cutout && voiceCutoutMuted(uid) {
		return "", false
	}

	// /voicestutter — sometimes relay the frame held from last time instead
	// of the current one.  The current frame is then remembered.  Exactly one
	// frame goes out per frame in, so the listener's jitter buffer never
	// drifts; the artefact is a ~20ms repeat-then-skip glitch.
	if set.stutter {
		out := frame
		if held, ok := getStutterFrame(uid); ok && rand.Float64() < voiceStutterChance {
			out = held
		}
		setStutterFrame(uid, frame)
		return out, true
	}

	return frame, true
}

// voiceCutoutMutedAt reports whether /voicecutout is in its "off" half-cycle
// for the given UID at the given wall-clock millisecond.  The phase is
// offset by UID so two cut-out players in the same room don't drop frames in
// lockstep.  Pure function — no clock read — so it is deterministically
// testable.
func voiceCutoutMutedAt(unixMs int64, uid int) bool {
	return ((unixMs/voiceCutoutWindowMs)+int64(uid))%2 == 1
}

// voiceCutoutMuted is voiceCutoutMutedAt evaluated against the current clock.
func voiceCutoutMuted(uid int) bool {
	return voiceCutoutMutedAt(time.Now().UnixMilli(), uid)
}

// getStutterFrame returns the frame held for a /voicestutter target, if any.
func getStutterFrame(uid int) (string, bool) {
	voiceStutterMu.Lock()
	defer voiceStutterMu.Unlock()
	f, ok := voiceStutterHeld[uid]
	return f, ok
}

// setStutterFrame records the most recent frame from a /voicestutter target.
func setStutterFrame(uid int, frame string) {
	voiceStutterMu.Lock()
	voiceStutterHeld[uid] = frame
	voiceStutterMu.Unlock()
}

// clearVoiceStutterState drops a UID's held stutter frame.  Called from
// clearVoiceRateStateForUID when the UID is released so the map cannot grow
// without bound across reconnects.
func clearVoiceStutterState(uid int) {
	voiceStutterMu.Lock()
	delete(voiceStutterHeld, uid)
	voiceStutterMu.Unlock()
}

// Command handlers.  Each is a thin wrapper over cmdPunishment, identical in
// shape to the text-punishment handlers (cmdTsundere et al.), so the voice
// punishments behave exactly like every other punishment command.

func cmdVoiceMute(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVoiceMute)
}

func cmdVoiceStatic(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVoiceStatic)
}

func cmdVoiceGarble(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVoiceGarble)
}

func cmdVoiceCutout(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVoiceCutout)
}

func cmdVoiceStutter(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentVoiceStutter)
}
