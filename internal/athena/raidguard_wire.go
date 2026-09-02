// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Wiring between the raid-guard detection engine (raidguard.go,
// raidguard_corr.go) and the packet hot paths in netprotocol.go/server.go.
//
// This file owns no detection logic of its own -- every function here is a
// thin translation from "packet X arrived" into a call on *raidState, plus
// the shared evaluate-then-enforce tail the engine expects callers to run
// whenever a signal fires. Keeping that translation in one place means every
// call site in netprotocol.go stays a one- or two-line hook, and the
// moderator exemption (see raidGuardExempt below) only has to be enforced
// once rather than copy-pasted into six different packet handlers.
package athena

import (
	"time"

	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// raidGuardExempt reports whether client must never be observed or scored by
// the raid guard at all, full stop -- not merely "never acted on". client.raidGuard()
// (raidguard.go) already keeps a moderator from ever being *punished* via
// raidGuardTier's exemption, but that is an enforcement-time check; this one
// runs before a single signal is ever recorded, so a moderator's traffic
// never so much as touches a raidState or the cross-IPID correlation window.
// Every hook below checks this first, before client.raidGuard().
func raidGuardExempt(client *Client) bool {
	return client == nil || permissions.IsModerator(client.Perms())
}

// raidGuardEvaluate is the canonical "something fired" tail: snapshot the
// connection's current score, scale the verdict thresholds to how established
// the connection is (raidGuardTier), and enforce the result. Every hook below
// that gets a signal back from the engine ends here, so this scale-then-
// enforce sequence is written exactly once rather than once per call site.
func raidGuardEvaluate(client *Client, rs *raidState, trigger string) {
	if client == nil || rs == nil {
		return
	}
	score, _, _ := rs.snapshot()
	scale, _ := raidGuardTier(client)
	raidGuardEnforce(client, rs, verdictForTier(score, scale), trigger)
}

// raidGuardTimings returns how long ago this connection was accepted and how
// long ago it first picked a character, in the form raidState.observe wants:
// a negative duration when the corresponding event hasn't happened yet,
// rather than a misleadingly huge positive one computed off a zero time.Time.
func raidGuardTimings(client *Client, now time.Time) (sinceConnect, sinceCharPick time.Duration) {
	sinceConnect, sinceCharPick = -1, -1
	if accepted := client.AcceptedAt(); !accepted.IsZero() {
		sinceConnect = now.Sub(accepted)
	}
	if picked := client.CharPickedAt(); !picked.IsZero() {
		sinceCharPick = now.Sub(picked)
	}
	return sinceConnect, sinceCharPick
}

// raidGuardOnAskchaa records this connection's askchaa. Called from
// pktResCount, before that handler's own early returns, so the guard sees the
// packet whether or not the server acts on it.
//
// Two different things are watched here. Reaching askchaa pre-join is the
// normal event and is only recorded -- it's what arrives *before* it that is
// the anomaly (see raidGuardOnHandshakeStep). Reaching it *post*-join is itself
// the anomaly, and is evaluated immediately (see SigHandshakeReplay).
func raidGuardOnAskchaa(client *Client) {
	if raidGuardExempt(client) {
		return
	}
	rs := client.raidGuard()
	if rs == nil {
		return
	}
	// Recorded unconditionally, including for the post-join case below. Skipping
	// it there would leave sawAskchaa false, so the next RC/RM/RD would fire the
	// ordering signal too and one behaviour would be charged twice -- 45 + 50 for
	// a single replayed handshake. In production the connection has necessarily
	// sent askchaa already (RD needs the `joining` flag that only pktResCount
	// sets), so this is belt-and-braces rather than a live path; it costs one
	// mutex acquisition on a handshake packet and removes the ordering
	// dependency entirely.
	rs.noteAskchaa()

	// An askchaa arriving after RD already assigned this connection a UID is the
	// one form of this packet that is not normal: pktId stops answering ID with
	// PN once a UID exists, and every client implementation sends askchaa only in
	// reply to PN, so nothing the server said can have prompted it. See
	// SigHandshakeReplay for the per-client evidence.
	if client.Uid() != -1 && rs.noteAskchaaPostJoin() {
		raidGuardEvaluate(client, rs, "handshake replay")
	}
}

// raidGuardOnHandshakeStep records one RC/RM/RD arrival and evaluates the
// connection if it turns out to be the handshake-order anomaly -- see
// raidState.noteHandshakeStep, which only fires the signal when this
// connection's own askchaa has not been seen yet. A real client always asks
// for the counts (askchaa) before it asks for the character/music/area lists
// those counts describe, so this never fires on ordinary traffic.
func raidGuardOnHandshakeStep(client *Client) {
	if raidGuardExempt(client) {
		return
	}
	rs := client.raidGuard()
	if rs == nil {
		return
	}
	if rs.noteHandshakeStep() {
		raidGuardEvaluate(client, rs, "handshake order")
	}
}

// raidGuardOnCharPick records a CC (character pick) attempt. sinceAccept is
// measured from AcceptedAt, NOT ConnectedAt: ConnectedAt is only set once RD
// assigns a UID, which happens strictly before a client can even send CC, so
// it could never see the sub-second window the fast-charpick signal exists to
// catch. Only the connection's FIRST successful pick sets charPickedAt (every
// later re-pick still feeds noteCharPick's char-churn signal, just without
// moving the timestamp) -- see SetCharPickedAt's doc comment for why the
// check-then-set here is race-free.
func raidGuardOnCharPick(client *Client) {
	if raidGuardExempt(client) {
		return
	}
	rs := client.raidGuard()
	if rs == nil {
		return
	}
	now := time.Now()
	if client.CharPickedAt().IsZero() {
		client.SetCharPickedAt(now)
	}
	sinceAccept := time.Duration(-1)
	if accepted := client.AcceptedAt(); !accepted.IsZero() {
		sinceAccept = now.Sub(accepted)
	}
	if fired := rs.noteCharPick(sinceAccept, now); len(fired) > 0 {
		raidGuardEvaluate(client, rs, "character pick")
	}
}

// raidGuardOnIC folds one IC message into this connection's layer-1 evidence
// and, if the same text is also being said by enough other IPIDs right now,
// the server-wide layer-2 correlation window (raidguard_corr.go). Called from
// pktIC only after the rate-limit, join-captcha, automod/censor and torment
// gates have already let the message through, so the guard only ever scores a
// message that would otherwise have reached the room unmodified.
//
// The shout modifier is read off the packet here via ms.Shout() rather than
// passed in by the caller. It is the same parse pktIC validates against, and
// taking it as a parameter meant a wiring mistake at the call site -- passing a
// constant, or the wrong local -- would disable the guard's headline signal
// silently, in a way no test that exercises this function could ever catch.
// Deriving it from the packet removes the parameter and the failure mode with
// it; the cost is one Cut and one Atoi per scored message.
func raidGuardOnIC(client *Client, ms *packet.MSPacket, text string) {
	if raidGuardExempt(client) {
		return
	}
	rs := client.raidGuard()
	if rs == nil {
		return
	}
	// A malformed modifier cannot reach here (pktIC drops the packet first), and
	// treating an unparseable one as "no shout" is the safe direction anyway.
	objection, _ := ms.Shout()
	now := time.Now()
	sinceConnect, sinceCharPick := raidGuardTimings(client, now)
	fired, _ := rs.observe(Observation{
		IPID:          client.Ipid(),
		IsIC:          true,
		Text:          text,
		Showname:      decode(ms.Showname),
		Objection:     objection,
		SinceConnect:  sinceConnect,
		SinceCharPick: sinceCharPick,
		Now:           now,
	})
	fired = append(fired, raidGuardCorrelate(rs, client.Ipid(), text, now)...)
	if len(fired) > 0 {
		raidGuardEvaluate(client, rs, "IC message")
	}
}

// raidGuardCorrelate feeds one message to the server-wide layer and folds
// whatever it says back into this connection's evidence. Written once and shared
// by the IC and OOC hooks so the two channels can never drift apart on which
// half of the graded corroboration they record -- the fan-out being detected
// does not care which channel it arrived on, and neither should this.
func raidGuardCorrelate(rs *raidState, ipid, text string, now time.Time) (fired []SignalKind) {
	echoed, corroborated := raidObserveContent(ipid, text, now)
	if corroborated && rs.noteCorrelated() {
		fired = append(fired, SigDupeAcrossIPIDs)
	}
	// Recorded even when the strong signal already fired: they are separate
	// pieces of evidence with separate weights, and only the strong one is
	// allowed anywhere near the ban gate.
	if echoed && rs.noteEchoed() {
		fired = append(fired, SigEchoedAcrossIPIDs)
	}
	return fired
}

// raidGuardOnWordHit folds a word-list match into this connection's evidence.
// Called from pktIC/pktOOC by the lead's code.
//
// SeverityNuke is checked and skipped here, not merely left to fall through:
// a nuke-tier hit bans the connection outright via AutoMod's own deterministic
// path (word_severity.go) before this file would ever have a say, and scoring
// a connection that is already being banned is pointless. Everything this
// function DOES pass on to the engine -- SeveritySevere and
// SeverityDefault/SeverityWatch -- is a heuristic signal like any other in
// this file: scored, tiered, and never by itself enough to disconnect or ban
// anyone. See the SigSlurSevere/SigSlurFlagged doc comments in raidguard.go.
func raidGuardOnWordHit(client *Client, m WordListMatch) {
	if raidGuardExempt(client) {
		return
	}
	if !m.Matched || m.Entry.Severity == SeverityNuke {
		return
	}
	rs := client.raidGuard()
	if rs == nil {
		return
	}
	if fired := rs.noteWordHit(m.Entry.Severity); len(fired) > 0 {
		raidGuardEvaluate(client, rs, "flagged word")
	}
}

// raidGuardOnOOC mirrors raidGuardOnIC for OOC chat. Called from pktOOC only
// once a plain (non-command) message has cleared the automod/censor checks
// and is about to reach the room via the normal broadcast path -- a
// stealthmuted or captcha-restricted connection returns from pktOOC before
// reaching that point, so this naturally never observes those messages
// without needing its own separate skip.
func raidGuardOnOOC(client *Client, oocName, text string) {
	if raidGuardExempt(client) {
		return
	}
	rs := client.raidGuard()
	if rs == nil {
		return
	}
	now := time.Now()
	sinceConnect, sinceCharPick := raidGuardTimings(client, now)
	fired, _ := rs.observe(Observation{
		IPID:          client.Ipid(),
		IsIC:          false,
		Text:          text,
		OOCName:       oocName,
		SinceConnect:  sinceConnect,
		SinceCharPick: sinceCharPick,
		Now:           now,
	})
	fired = append(fired, raidGuardCorrelate(rs, client.Ipid(), text, now)...)
	if len(fired) > 0 {
		raidGuardEvaluate(client, rs, "OOC message")
	}
}
