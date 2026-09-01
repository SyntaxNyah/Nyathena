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

// Raid guard: behavioural raid detection.
//
// Every rate limit the server had before this file is keyed on one identity --
// an IPID, an HDID, a connection. That works against one abusive player and is
// structurally blind to the thing that actually happens during a raid, which is
// a fan-out: a real incident on this server put 65 distinct IPIDs on the box
// inside three seconds, and *every one of them individually stayed under
// message_rate_limit*. Measured against a clean baseline capture, the per-IPID
// speech rate of a raider (median 2.31 msg/s) is indistinguishable from that of
// an ordinary player mid-conversation (2.00 msg/s). The aggregate rate differed
// by 41x. There is no per-IPID threshold that separates those two populations,
// because on a per-IPID basis they are the same population.
//
// So this file scores *behaviour* rather than volume, in two layers:
//
//	Layer 1 (this file) -- what one connection did. Signals that describe how a
//	connection behaves rather than how fast it talks: did it pick a character in
//	under a second, is every one of its messages an objection shout, is it
//	wearing a different OOC name on each line.
//
//	Layer 2 (raidguard_corr.go) -- what the server as a whole is seeing. Content
//	repeated across many *different* IPIDs inside a few seconds, and new-IPID
//	arrival bursts. This is the layer that sees a fan-out, and it is the reason
//	the guard exists.
//
// Design rules, in priority order over detection power:
//
//  1. No single signal can ban. Scores accumulate and the ban threshold is set
//     above any one signal's weight, so a ban always means several independent
//     things were true at once.
//  2. Established players are exempt from every punitive action. "Established"
//     is db.GetPlaytime measured against lockdownPurgeEligible -- the same
//     predicate the lockdown purge, the repeat-offender autoban tiers and the
//     join captcha already use, so all four agree on the term.
//  3. A DB error fails OPEN. A hiccup reading playtime must never ban somebody.
//  4. Off by default, and the default maximum action is "silence", not "ban" --
//     see raidGuardMaxAction. An operator opts in to banning deliberately.
//  5. When disabled the hot path costs one atomic load.
package athena

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/xhit/go-str2duration/v2"
)

// raidGuardActive gates the whole feature on the hot path. Set once at startup
// from config and by /reload; a server that never enables the guard pays one
// atomic load per message and nothing else.
var raidGuardActive atomic.Bool

// SignalKind identifies one behavioural signal. Each fires at most once per
// connection, so a connection's score is bounded and a chatty session cannot
// inflate its way to a ban by repeating one behaviour.
type SignalKind uint16

const (
	SigHandshakeAnomaly SignalKind = iota
	SigDupeAcrossIPIDs
	SigEchoedAcrossIPIDs
	SigObjectionSpam
	SigOOCNameChurn
	SigShownameChurn
	SigFastCharPick
	SigCharChurn
	SigFastFirstSpeech
	SigShoutySpam
	SigGlobalSpam
	numRaidSignals
)

// raidSignalWeight is the score each signal contributes. These are ordered by
// the separation each showed between a real raid capture and a clean baseline,
// discounted by how plausibly an ordinary player could trip it alone.
//
// The ban threshold (raid_guard_score_ban, default 160) is deliberately higher
// than any single weight here, and higher than any pair of them, so reaching it
// requires at least three independent signals.
var raidSignalWeight = [numRaidSignals]int{
	// A client that sent RC/RM/RD before its own askchaa is not following the
	// protocol any real client implements. Highest weight, but on its own it
	// still only raises an alert.
	//
	// Confirmed against the LemmyAO client source rather than assumed from the
	// capture. Each of those three sends lives inside the handler for the server
	// packet that answers the previous step: RC is sent from the SI handler
	// (client/fetchLists.ts applyServerCounts), RM from the character-list
	// handler (client/handleCharacterInfo.ts), RD from the SM handler
	// (client/addTrack.ts applyMusicListBatch), and askchaa itself from the PN
	// handler (client/handshake.ts applyServerInfo). SI only arrives in reply to
	// askchaa, so the chain askchaa -> SI -> RC -> SC -> RM -> SM -> RD is
	// causal, not conventional: there is no code path that emits any of the three
	// earlier, and no ordering a real client can be coaxed into that inverts it.
	SigHandshakeAnomaly: 50,
	// The same text from N distinct IPIDs inside a few seconds. The single
	// clearest fan-out signal, and one no per-IPID limiter can see.
	SigDupeAcrossIPIDs: 45,
	// The weaker half of the same evidence: somebody else is saying your line,
	// but not yet enough people to call it a fan-out. Added after the 2026-08-31
	// raid, where ten IPIDs spread ten different slurs over twenty-six seconds
	// and re-used lines two and three times but never four -- so the full
	// SigDupeAcrossIPIDs threshold was never met and layer 2 contributed nothing
	// while the raid ran. Replaying that capture, this fires on the fourth raid
	// message, seven seconds in.
	//
	// Weighted at 25 deliberately: below the watch threshold at every tier, so a
	// connection whose only distinction is that somebody echoed it is not even
	// alerted on, let alone acted on. It takes a second independent signal to
	// reach the captcha rung. It never satisfies the ban gate -- that stays on
	// SigDupeAcrossIPIDs alone (see raidBanAllowed), so two players quoting each
	// other cannot arm a ban between them.
	SigEchoedAcrossIPIDs: 25,
	// Sustained objection shouts. Bimodal in the capture: raiders who used it
	// used it on 100% of their messages, real players on none of theirs.
	//
	// Checked against the LemmyAO client source, because a client that latched
	// the shout on would manufacture exactly this pattern from an innocent
	// player. It does not: the shout is held in client state and sent with every
	// message (dom/onICEnter.ts), but resetICParams() clears it the moment the
	// client sees its own message echoed back (viewport/utils/handleICSpeaking.ts),
	// so ordinary play resets after every line.
	//
	// The residual case is a player whose messages are being dropped WITHOUT an
	// echo, who would keep re-sending the latched shout. Nyathena's own drop
	// paths (censor shadow-send, stealthmute, captcha restriction, and this
	// guard's own silence) all echo to the sender, so they reset it; only a
	// packet failing IC validation outright leaves it set. Reaching the signal
	// still needs three such messages at an 80% shout rate, and the tiers and
	// the three-signal disconnect floor sit underneath. Worth knowing about
	// rather than assuming away.
	SigObjectionSpam: 35,
	SigOOCNameChurn:  30,
	SigShownameChurn: 30,
	// Bounded by protocol physics rather than by observed timings. A real
	// client cannot pick a character before it has received the character list,
	// because it builds the picker from that list -- including LemmyAO's
	// ?char= share-link auto-pick, which fires on DONE and looks up the index in
	// client.chars. The fastest full handshake in the clean capture was 330ms,
	// so a threshold below that cannot be reached by any legitimate pick,
	// auto or manual. (1000ms, the obvious-looking value, was wrong: 6 of 19
	// real handshakes completed inside it, so an auto-picking player on a fast
	// connection would have tripped it.)
	SigFastCharPick: 25,
	SigCharChurn:    25,
	// Weakest timing signal: a returning player with a saved character can
	// legitimately be quick, so this is a nudge, not an accusation.
	SigFastFirstSpeech: 20,
	// Content-shape signals are the weakest and most culture-dependent: AO2 is
	// a courtroom game and dramatic capslock is ordinary there. Present so they
	// can tip a borderline case, never enough to matter on their own.
	SigShoutySpam: 10,
	SigGlobalSpam: 10,
}

// raidSignalName labels a signal for staff alerts and the audit log.
var raidSignalName = [numRaidSignals]string{
	SigHandshakeAnomaly:  "handshake out of order",
	SigDupeAcrossIPIDs:   "text repeated across many IPIDs",
	SigEchoedAcrossIPIDs: "text echoed by another IPID",
	SigObjectionSpam:     "every message an objection shout",
	SigOOCNameChurn:      "OOC name changing per message",
	SigShownameChurn:     "showname changing per message",
	SigFastCharPick:      "picked a character instantly",
	SigCharChurn:         "re-rolling characters rapidly",
	SigFastFirstSpeech:   "spoke instantly after joining",
	SigShoutySpam:        "shouted repetitive all-caps",
	SigGlobalSpam:        "global-channel spam while brand new",
}

// Verdict is the strongest action a score justifies. Ordered by severity; the
// engine only ever escalates, never downgrades a connection it already acted on.
type Verdict uint8

const (
	VerdictClean Verdict = iota
	VerdictWatch
	VerdictChallenge
	VerdictSilence
	VerdictKick
	VerdictBan
)

func (v Verdict) String() string {
	switch v {
	case VerdictWatch:
		return "watch"
	case VerdictChallenge:
		return "captcha"
	case VerdictSilence:
		return "silence"
	case VerdictKick:
		return "kick"
	case VerdictBan:
		return "ban"
	default:
		return "clean"
	}
}

// parseRaidVerdict maps a raid_guard_max_action config string to a ceiling.
func parseRaidVerdict(s string) (Verdict, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "watch", "alert":
		return VerdictWatch, true
	case "captcha", "challenge", "verify":
		return VerdictChallenge, true
	case "silence", "mute", "quarantine":
		return VerdictSilence, true
	case "kick":
		return VerdictKick, true
	case "ban", "autoban":
		return VerdictBan, true
	default:
		return VerdictClean, false
	}
}

// Observation is one message's worth of evidence. It is a plain value with no
// pointer into live server state, so the scoring engine can be exercised in a
// test with a struct literal and no connection, no database and no area.
type Observation struct {
	IPID          string
	IsIC          bool
	Text          string        // decoded message body
	OOCName       string        // CT name field; empty for IC
	Showname      string        // MS showname field; empty for OOC
	Objection     int           // parsed ShoutModifier; 0 means no shout
	SinceConnect  time.Duration // since the connection was accepted
	SinceCharPick time.Duration // negative if no character has been picked
	Now           time.Time
}

// maxTrackedNames bounds the per-connection name sets. A connection cycling
// names is exactly what we are looking for, so we only need enough distinct
// values to prove churn -- never an unbounded history of them.
const maxTrackedNames = 32

// raidState is one connection's accumulated evidence. Lives on *Client and is
// self-synchronised, so callers on the IC/OOC hot path need not hold client.mu.
type raidState struct {
	mu sync.Mutex

	msgCount       int
	icCount        int
	objectionCount int

	oocNames  map[string]struct{}
	shownames map[string]struct{}

	charPicks    int
	lastCharPick time.Time

	sawAskchaa bool

	fired SignalKind // bitmask, not an index -- see markFired
	score int
	acted Verdict
}

func newRaidState() *raidState {
	return &raidState{
		oocNames:  make(map[string]struct{}, 4),
		shownames: make(map[string]struct{}, 4),
	}
}

// markFired records a signal and adds its weight, exactly once. Returns false
// if the signal had already fired, so callers can avoid re-alerting.
func (rs *raidState) markFired(k SignalKind) bool {
	bit := SignalKind(1) << k
	if rs.fired&bit != 0 {
		return false
	}
	rs.fired |= bit
	rs.score += raidSignalWeight[k]
	return true
}

func (rs *raidState) hasFired(k SignalKind) bool {
	return rs.fired&(SignalKind(1)<<k) != 0
}

// firedSignals lists the signals that have fired, for alerts and the audit log.
func (rs *raidState) firedSignals() []string {
	var out []string
	for k := SignalKind(0); k < numRaidSignals; k++ {
		if rs.hasFired(k) {
			out = append(out, raidSignalName[k])
		}
	}
	return out
}

// noteAskchaa records that this connection reached the askchaa handshake step.
// Anything arriving before it that should not have is a protocol violation.
func (rs *raidState) noteAskchaa() {
	rs.mu.Lock()
	rs.sawAskchaa = true
	rs.mu.Unlock()
}

// noteHandshakeStep records an RC/RM/RD. Arriving before this connection's own
// askchaa is the anomaly; a real client asks for the counts before it asks for
// the lists they describe.
func (rs *raidState) noteHandshakeStep() (fired bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.sawAskchaa {
		return false
	}
	return rs.markFired(SigHandshakeAnomaly)
}

// noteCharPick records a CC. Two signals come off it: picking a character
// implausibly soon after connecting, and re-rolling characters faster than a
// person can read a list of several thousand of them.
func (rs *raidState) noteCharPick(sinceConnect time.Duration, now time.Time) (fired []SignalKind) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.charPicks++
	prev := rs.lastCharPick
	rs.lastCharPick = now

	if fastMs := raidGuardInt(config.RaidGuardFastCharPickMs, 1000); rs.charPicks == 1 &&
		sinceConnect > 0 && sinceConnect < time.Duration(fastMs)*time.Millisecond {
		if rs.markFired(SigFastCharPick) {
			fired = append(fired, SigFastCharPick)
		}
	}
	if churnMs := raidGuardInt(config.RaidGuardCharChurnMs, 1000); !prev.IsZero() &&
		now.Sub(prev) < time.Duration(churnMs)*time.Millisecond {
		if rs.markFired(SigCharChurn) {
			fired = append(fired, SigCharChurn)
		}
	}
	return fired
}

// observe folds one message into the connection's evidence and returns the
// signals that newly fired plus the running score.
func (rs *raidState) observe(obs Observation) (fired []SignalKind, score int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.msgCount++
	if obs.IsIC {
		rs.icCount++
		if obs.Objection != 0 {
			rs.objectionCount++
		}
	}

	mark := func(k SignalKind) {
		if rs.markFired(k) {
			fired = append(fired, k)
		}
	}

	// Objection shouts. Requires a floor of messages so one dramatic entrance
	// cannot trip it, and a high sustained fraction so a real trial -- which
	// interleaves dialogue with shouts -- never reaches it.
	minMsgs := raidGuardInt(config.RaidGuardObjectionMinMsgs, 3)
	frac := config.RaidGuardObjectionFraction
	if frac <= 0 || frac > 1 {
		frac = 0.8
	}
	// Measured over IC messages only. The objection modifier is a field on the
	// IC packet and does not exist on OOC at all, so counting OOC in the
	// denominator lets a raider dilute their own shout rate below the threshold
	// just by spamming both channels -- which is exactly what the capture shows
	// them doing, and it hid several of them from this signal entirely.
	if rs.icCount >= minMsgs && float64(rs.objectionCount)/float64(rs.icCount) >= frac {
		mark(SigObjectionSpam)
	}

	// Name churn. A real client sends its OOC name on every packet, so the
	// count that matters is *distinct* values, not how often the field appears.
	churnMax := raidGuardInt(config.RaidGuardNameChurnMax, 3)
	if n := strings.TrimSpace(obs.OOCName); n != "" && len(rs.oocNames) < maxTrackedNames {
		rs.oocNames[n] = struct{}{}
	}
	if len(rs.oocNames) >= churnMax {
		mark(SigOOCNameChurn)
	}
	if n := strings.TrimSpace(obs.Showname); n != "" && len(rs.shownames) < maxTrackedNames {
		rs.shownames[n] = struct{}{}
	}
	if len(rs.shownames) >= churnMax {
		mark(SigShownameChurn)
	}

	// Spoke before a person could plausibly have read the room.
	fastMs := raidGuardInt(config.RaidGuardFastSpeechMs, 1500)
	if rs.msgCount == 1 && obs.SinceCharPick >= 0 &&
		obs.SinceCharPick < time.Duration(fastMs)*time.Millisecond {
		mark(SigFastFirstSpeech)
	}

	// Content shape. Weakest signals by design; see raidSignalWeight.
	if isShoutySpam(obs.Text) {
		mark(SigShoutySpam)
	}
	if !obs.IsIC && isGlobalCommand(obs.Text) && obs.SinceConnect > 0 &&
		obs.SinceConnect < 30*time.Second {
		mark(SigGlobalSpam)
	}

	return fired, rs.score
}

// noteCorrelated is called by the server-wide layer when this connection's text
// has also been seen from enough other IPIDs to count as coordinated.
func (rs *raidState) noteCorrelated() (fired bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.markFired(SigDupeAcrossIPIDs)
}

// noteEchoed is the weak half of noteCorrelated: somebody else is saying this
// connection's line, but not yet enough people for it to count as a fan-out.
func (rs *raidState) noteEchoed() (fired bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.markFired(SigEchoedAcrossIPIDs)
}

// snapshot returns the current score and fired-signal names.
func (rs *raidState) snapshot() (score int, signals []string, acted Verdict) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.score, rs.firedSignals(), rs.acted
}

// firedCount reports how many distinct signals have fired.
func (rs *raidState) firedCount() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	n := 0
	for k := SignalKind(0); k < numRaidSignals; k++ {
		if rs.hasFired(k) {
			n++
		}
	}
	return n
}

// firedSignal reports whether a signal has fired, taking the lock. Used by the
// ban gate in raidGuardEnforce, which runs outside observe().
func (rs *raidState) firedSignal(k SignalKind) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.hasFired(k)
}

// escalate records that an action has been taken and reports whether it is a
// genuine escalation over what has already been done to this connection.
func (rs *raidState) escalate(v Verdict) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if v <= rs.acted {
		return false
	}
	rs.acted = v
	return true
}

// raidGuardScaleBase is the percentage meaning "no adjustment". Tier scales are
// percentages so they can move thresholds in both directions: below 100 makes
// the guard stricter, above 100 makes it more forgiving.
const raidGuardScaleBase = 100

// verdictForTier maps a score onto an action, with every threshold scaled by how
// established the connection is. scalePct comes from raidGuardTier: below 100
// for a connection with no history (which is what a raid is made of), 100 at
// baseline, and higher for a player with hours behind them, who must therefore
// look proportionally worse before the guard does anything at all.
//
// The comparison is cross-multiplied rather than dividing the threshold, so no
// tier boundary is lost to integer truncation.
func verdictForTier(score, scalePct int) Verdict {
	if scalePct <= 0 {
		scalePct = raidGuardScaleBase
	}
	at := func(v, def int) bool {
		return score*raidGuardScaleBase >= raidGuardInt(v, def)*scalePct
	}
	switch {
	case at(config.RaidGuardScoreBan, raidGuardDefaultScoreBan):
		return VerdictBan
	case at(config.RaidGuardScoreKick, 100):
		return VerdictKick
	case at(config.RaidGuardScoreSilence, 80):
		return VerdictSilence
	case at(config.RaidGuardScoreChallenge, 60):
		return VerdictChallenge
	case at(config.RaidGuardScoreWatch, 40):
		return VerdictWatch
	}
	return VerdictClean
}

// raidGuardDefaultScoreBan is the ban threshold used when no config is loaded.
// It matches settings.DefaultConfig and config_sample/config.toml, which both
// ship 160; the fallback here used to say 140 and so disagreed with the value
// every real server actually runs.
const raidGuardDefaultScoreBan = 160

// raidGuardInt returns a configured value, falling back to a default when the
// config is absent (tests) or the value is nonsensical.
func raidGuardInt(v, def int) int {
	if config == nil || v <= 0 {
		return def
	}
	return v
}

// raidGuardMaxAction is the ceiling an operator has opted into. Defaults to
// silence: the guard will quarantine a suspected raider out of the box, but
// will not ban one until somebody deliberately turns that on.
func raidGuardMaxAction() Verdict {
	if config == nil {
		return VerdictSilence
	}
	v, ok := parseRaidVerdict(config.RaidGuardMaxAction)
	if !ok {
		return VerdictSilence
	}
	return v
}

// raidGuardBanDuration is how long a raid-guard autoban lasts.
func raidGuardBanDuration() (time.Duration, bool) {
	if config == nil {
		return 30 * time.Minute, true
	}
	d, err := str2duration.ParseDuration(config.RaidGuardBanDuration)
	if err != nil || d <= 0 {
		return 30 * time.Minute, true
	}
	return d, true
}

// raidGuardTier decides how much benefit of the doubt a connection gets, read
// from the same KNOWN_IPS.PLAYTIME figure /playtime, the lockdown purge, the
// join captcha and the repeat-offender autoban all use -- so "an established
// player" means one thing everywhere on this server, and a regular already
// exempt from those is exempt from this too.
//
// Playtime is treated as just another signal, and the most trustworthy one the
// server has: somebody with thousands of hours here is not a raider, and a
// connection with no history at all is what every raid is made of. So the tiers
// run in both directions from baseline rather than only granting leniency.
//
// Four tiers, extending the shape rateLimitAutobanThresholdFor established:
//
//	Fully exempt -- moderators, and any IPID at or above raid_guard_min_playtime
//	(default 1200 = 20h). Never acted on, however it behaves; staff still get the
//	alert, so a regular whose account is misbehaving is visible without the
//	server automatically doing anything to them.
//
//	Lenient -- at or above raid_guard_lenient_playtime (default 120 = 2h).
//	Thresholds scaled by raid_guard_lenient_scale (default 200 = twice as much
//	evidence needed). Not immune, just materially harder to trip by accident.
//
//	Baseline -- at or above raid_guard_strict_playtime (default 15 minutes).
//	Thresholds as configured.
//
//	Strict -- below that: a brand-new connection with essentially no history.
//	Thresholds scaled by raid_guard_strict_scale (default 70, i.e. 30% less
//	evidence needed). This is the population a raid actually consists of, and
//	the only tier the guard leans into rather than away from.
//
// A DB error returns "not punishable". The guard must never act on somebody
// because a query hiccuped; this is the same fail-open rule the repeat-offender
// autoban uses, and deliberately the opposite of the join captcha's fail-closed
// one. Failing open here costs one raider message getting through. Failing
// closed would cost a real player, which is the outcome this whole file is
// arranged to avoid.
func raidGuardTier(client *Client) (scalePct int, punishable bool) {
	if client == nil {
		return 0, false
	}
	if permissions.IsModerator(client.Perms()) {
		return 0, false
	}
	secs, err := db.GetPlaytime(client.Ipid())
	if err != nil {
		return 0, false // fail open
	}
	// lockdownPurgeEligible(secs, t) is "below threshold t", so !eligible means
	// the connection has met it.
	if m := config.RaidGuardMinPlaytime; m > 0 && !lockdownPurgeEligible(secs, int64(m)*60) {
		return 0, false
	}
	if l := config.RaidGuardLenientPlaytime; l > 0 && !lockdownPurgeEligible(secs, int64(l)*60) {
		return raidGuardInt(config.RaidGuardLenientScale, 200), true
	}
	if st := config.RaidGuardStrictPlaytime; st > 0 && lockdownPurgeEligible(secs, int64(st)*60) {
		return raidGuardInt(config.RaidGuardStrictScale, 70), true
	}
	return raidGuardScaleBase, true
}

// isShoutySpam reports the combined all-caps shape: long, overwhelmingly
// uppercase, and repeating a word. Any one of those alone is ordinary in a
// courtroom game -- "OBJECTION!" is the point of AO2 -- so all three are
// required together, and even then the signal is weighted at 10.
func isShoutySpam(s string) bool {
	if len([]rune(s)) < 25 {
		return false
	}
	if capsRatio(s) < 0.8 {
		return false
	}
	return hasRepeatedWord(s)
}

// capsRatio is the fraction of cased letters that are uppercase. Returns 0 when
// there are no cased letters at all, so punctuation or a non-cased script
// (Japanese, say) never reads as shouting.
func capsRatio(s string) float64 {
	var upper, cased int
	for _, r := range s {
		if unicode.IsUpper(r) {
			upper++
			cased++
		} else if unicode.IsLower(r) {
			cased++
		}
	}
	if cased == 0 {
		return 0
	}
	return float64(upper) / float64(cased)
}

// hasRepeatedWord reports whether any word of 3+ characters appears twice.
func hasRepeatedWord(s string) bool {
	seen := make(map[string]struct{}, 8)
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if len(w) < 3 {
			continue
		}
		if _, dup := seen[w]; dup {
			return true
		}
		seen[w] = struct{}{}
	}
	return false
}

// isGlobalCommand reports whether an OOC line is a global broadcast. Global
// reaches every area at once, which is why a brand-new connection using it is
// worth a nudge -- it is the highest-reach thing a raider can do per packet.
func isGlobalCommand(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	return strings.HasPrefix(t, "/g ") || strings.HasPrefix(t, "/global ")
}

// raidGuardEnforce applies a verdict, clamped to the operator's configured
// ceiling and skipped entirely for exempt clients. Alerts staff on every
// escalation so the guard can never act on a real player unnoticed.
func raidGuardEnforce(client *Client, rs *raidState, want Verdict, trigger string) {
	if client == nil || rs == nil || want == VerdictClean {
		return
	}
	// The one guarantee this system makes: a ban requires positive evidence that
	// this connection is part of a coordinated fan-out -- its own text corroborated
	// from several other IPIDs, while the server is concurrently seeing raid-shaped
	// traffic. Neither condition is reachable by one person acting alone, however
	// oddly they behave, so no accumulation of single-connection signals can ban a
	// lone player. A connection that earns a ban score without that corroboration is
	// quarantined instead, which a real player can undo themselves by answering the
	// pending captcha.
	if want == VerdictBan && !raidBanAllowed(rs.firedSignal(SigDupeAcrossIPIDs), raidGuardUnderAttack()) {
		want = VerdictSilence
	}

	// Disconnecting somebody is the first action they cannot undo themselves, so
	// it takes corroboration that no weighting can shortcut: at least
	// minSignalsToDisconnect distinct signals must have fired, independent of
	// the score they add up to.
	//
	// This exists because the signal weights are calibrated on two captures, and
	// one of them -- handshake ordering -- is an empirical observation (67% of
	// raid connections, 0% of baseline) that was never confirmed against real
	// client source. If some client somewhere does emit RC/RM/RD before its own
	// askchaa, that signal plus one unlucky content match would otherwise be
	// enough to kick a real player off a brand-new connection. Requiring a third
	// independent signal means a single mis-calibrated weight cannot cost anyone
	// their session; a quarantine, which they can lift themselves by answering
	// the pending captcha, is as far as two signals can go.
	want = clampDisconnect(want, rs.firedCount())

	// The challenge rung borrows the join captcha's machinery, so it cannot work
	// when the operator has turned that off. Degrade it to an alert rather than
	// silently handing out captchas on a server whose owner disabled them --
	// and specifically NOT to silence, which without a pending question the
	// player could answer is harsher than the rung above it, not gentler. If the
	// behaviour continues the score keeps climbing to kick or ban on its own.
	if want == VerdictChallenge && !joinCaptchaEnabled() {
		want = VerdictWatch
	}

	// Watch is only an alert, so it is safe for anyone; every punitive verdict is
	// clamped to the operator's configured ceiling and to the connection's
	// playtime tier. A fully-exempt player (moderator, or an established regular)
	// can still be alerted on, never acted on.
	if want > VerdictWatch {
		if ceiling := raidGuardMaxAction(); want > ceiling {
			want = ceiling
		}
		if _, punishable := raidGuardTier(client); !punishable {
			want = VerdictWatch
		}
	}
	if !rs.escalate(want) {
		return
	}

	score, signals, _ := rs.snapshot()
	reason := strings.Join(signals, ", ")

	switch want {
	case VerdictWatch:
		// Alert only.
	case VerdictChallenge:
		raidGuardChallenge(client)
	case VerdictSilence:
		raidGuardSilence(client)
	case VerdictKick:
		client.SendSync(&packet.KK{Reason: "Disconnected by the raid guard. If you are a real player caught by " +
			"this, reconnect and say so in OOC -- staff have been alerted and can clear it."})
		client.markClosed()
	case VerdictBan:
		if dur, ok := raidGuardBanDuration(); ok {
			autobanFlooderFor(client.Ipid(), "raid guard ("+reason+")", dur)
		}
		client.SendSync(&packet.KK{Reason: "Banned by the raid guard."})
		client.markClosed()
	}

	logger.LogInfof("Raid guard: %v action on IPID:%v UID:%v score=%d signals=[%v] trigger=%v",
		want, client.Ipid(), client.Uid(), score, reason, trigger)
	logger.WriteAudit(fmt.Sprintf("%v | RAID_GUARD | IPID:%v | UID:%v | action=%v | score=%d | signals=%v",
		time.Now().UTC().Format("15:04:05"), client.Ipid(), client.Uid(), want, score, reason))
	alertRaidGuard(client, want, score, signals)
}

// raidGuard returns this connection's raid-guard state, creating it on first
// use. Returns nil when the guard is disabled, so every caller on the hot path
// can bail on a nil check without allocating anything.
func (client *Client) raidGuard() *raidState {
	if client == nil || !raidGuardActive.Load() {
		return nil
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.raid == nil {
		client.raid = newRaidState()
	}
	return client.raid
}

// resetRaidGuard clears a connection's accumulated evidence. Used by
// /raidguard clear, the escape hatch for a connection the guard misjudged.
func (client *Client) resetRaidGuard() {
	if client == nil {
		return
	}
	client.mu.Lock()
	client.raid = nil
	client.mu.Unlock()
}

// minSignalsToDisconnect is how many distinct signals must fire before the guard
// will take an action the player cannot reverse on their own.
const minSignalsToDisconnect = 3

// clampDisconnect enforces minSignalsToDisconnect, factored out so the property
// can be asserted directly rather than inferred from enforcement behaviour.
func clampDisconnect(want Verdict, firedCount int) Verdict {
	if want >= VerdictKick && firedCount < minSignalsToDisconnect {
		return VerdictSilence
	}
	return want
}

// raidBanAllowed is the guard's central safety invariant, factored out so it can
// be asserted directly in a test rather than inferred from behaviour.
//
// A ban requires BOTH that this connection's own text was corroborated from
// several other IPIDs, AND that the server was concurrently seeing raid-shaped
// traffic. Neither is reachable by one person acting alone, so no accumulation
// of single-connection signals can ban a lone player however strangely they
// behave. Everything else the guard can do is recoverable -- a captcha the
// player answers themselves, a quarantine staff can lift -- but a ban is not,
// so it is the one action gated on evidence a real player cannot manufacture.
func raidBanAllowed(correlatedAcrossIPIDs, serverUnderAttack bool) bool {
	return correlatedAcrossIPIDs && serverUnderAttack
}

// raidGuardChallenge forces a join captcha onto a connection the guard finds
// suspicious, even one the captcha's own exemptions would have skipped.
//
// issueJoinCaptcha re-checks those exemptions itself and so cannot be reused
// here: the whole point is that this connection has *earned* a question through
// its behaviour, which is a different judgement from "we have never seen this
// IPID before". A client already awaiting or restricted by the captcha is left
// alone rather than being handed a fresh question.
func raidGuardChallenge(client *Client) {
	// Also checked by raidGuardEnforce, which degrades the verdict before
	// getting here; repeated so no future caller can hand out a captcha on a
	// server that has the feature switched off.
	if !joinCaptchaEnabled() {
		return
	}
	if client.awaitingCaptcha.Load() || client.captchaRestricted.Load() {
		return
	}
	c, ok := pluginChallengeFor(client)
	if !ok {
		c = pickJoinChallenge(client.Ipid())
	}
	client.mu.Lock()
	client.pendingJoinChallenge = c
	client.mu.Unlock()
	client.captchaStrikes.Store(0)
	client.awaitingCaptcha.Store(true)

	client.SendServerMessage(fmt.Sprintf(
		"%s\n\n    %s\n\n%s\n\nSomething about this connection looked automated, so we need one answer before you can chat.",
		captchaBanner, c.Prompt, c.Hint))
	client.sendCaptchaPopup(c)
	go client.joinCaptchaTimeoutWatch()
}

// raidGuardSilence quarantines a connection using the join captcha's own
// restricted-delivery path: messages echo back to the sender so their client
// looks normal, and reach nobody else. The pending challenge is left in place
// so a real player caught here can still answer it and free themselves without
// waiting for staff.
func raidGuardSilence(client *Client) {
	client.awaitingCaptcha.Store(false)
	if client.captchaRestricted.CompareAndSwap(false, true) {
		activeCaptchaRestricted.Add(1)
	}
}

// alertRaidGuard tells every staff member holding MOD_CHAT what the guard did
// and why, naming the signals so a false positive is obvious at a glance and
// can be reversed with /raidguard clear.
func alertRaidGuard(client *Client, v Verdict, score int, signals []string) {
	areaName := "unknown area"
	if a := client.Area(); a != nil {
		areaName = a.Name()
	}
	selfService := ""
	if v == VerdictSilence && !joinCaptchaEnabled() {
		// Normally a quarantined player frees themselves by answering the
		// pending captcha. With the captcha switched off there is no question to
		// answer, so staff are the only way out and need to know that.
		selfService = "\nThe join captcha is off, so they CANNOT free themselves — this needs staff."
	}
	msg := fmt.Sprintf("%s (UID %d, IPID %s) in %s — raid guard: %s (score %d).\nSignals: %s%s\n"+
		"If this is a real player, clear them with /raidguard clear %d.",
		oocDisplayName(client), client.Uid(), client.Ipid(), areaName, v, score,
		strings.Join(signals, "; "), selfService, client.Uid())
	out := &packet.CTToClient{Name: "[RAIDGUARD]", Message: encode(msg), IsFromServer: "1"}
	clients.ForEach(func(c *Client) {
		if !permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			return
		}
		c.Send(out)
	})
}
