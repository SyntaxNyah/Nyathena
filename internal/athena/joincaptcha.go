package athena

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Join captcha: a new IPID must answer one question before it can speak.
//
// The problem this solves is a raid, not a rude player. A flood arrives from
// many IPIDs (and many HDIDs) at once, so nothing that keys off identity --
// bans, cooldowns, per-IP rate limits -- can get ahead of it: by the time an
// address is worth blocking it has been discarded. What every one of those
// connections does have in common is that none of them can answer a question.
//
// The gate therefore sits in front of *speech*, not in front of the connection.
// A joining player can look around, pick a character and read the room while
// unverified; they simply cannot broadcast anything until they answer. That
// keeps the AO2 handshake untouched (there is no captcha step in the protocol
// to hook) and keeps the failure mode gentle for a real person who is confused.
//
// Two properties matter more than the question itself, and both are deliberate:
//
//   - The answer is never printed in the question. See
//     joincaptcha_challenge.go -- a scraper that echoes back whatever follows
//     "type this" gets nowhere, because there is nothing to echo.
//
//   - Failing is invisible. After join_captcha_strikes failures the connection
//     is not kicked but quarantined: its messages still echo back to it and it
//     still receives the room, so the sender sees a working chat. A kick tells
//     an attacker exactly which of their attempts was detected and hands them a
//     clean signal to iterate against; the quarantine tells them nothing, which
//     is what makes a solver expensive to develop rather than merely expensive
//     to run. See quarantineClient below.

const (
	// joinCaptchaActionQuarantine silently shadow-quarantines a connection that
	// runs out of strikes. The default.
	joinCaptchaActionQuarantine = "quarantine"
	// joinCaptchaActionKick disconnects it instead, with a plain explanation.
	joinCaptchaActionKick = "kick"
)

// activeCaptchaQuarantine counts the connections currently shadow-quarantined.
// The IC and OOC delivery paths consult it before doing any per-client work, so
// a server that has never quarantined anyone pays exactly one atomic load per
// message -- the same gating pattern /truepossess and /forcedisplay use.
var activeCaptchaQuarantine atomic.Int64

// joinCaptchaVerified is the set of IPIDs that have solved a challenge, seeded
// from JOIN_CAPTCHA_VERIFIED at startup so a restart does not re-challenge the
// whole player base. Only consulted once per connection, at join.
var joinCaptchaVerified = struct {
	mu  sync.RWMutex
	set map[string]struct{}
}{set: make(map[string]struct{})}

// seedJoinCaptchaVerified loads persisted verifications into memory. Called
// from InitServer alongside the other IPID-keyed sets.
func seedJoinCaptchaVerified() {
	ipids, err := db.LoadJoinCaptchaVerified()
	if err != nil {
		logger.LogErrorf("Failed to load join-captcha verifications from database: %v", err)
		return
	}
	joinCaptchaVerified.mu.Lock()
	for _, ipid := range ipids {
		joinCaptchaVerified.set[ipid] = struct{}{}
	}
	joinCaptchaVerified.mu.Unlock()
	if len(ipids) > 0 {
		logger.LogInfof("Loaded %d join-captcha verification(s) from database.", len(ipids))
	}
}

// isJoinCaptchaVerified reports whether an IPID has already solved a challenge.
func isJoinCaptchaVerified(ipid string) bool {
	joinCaptchaVerified.mu.RLock()
	defer joinCaptchaVerified.mu.RUnlock()
	_, ok := joinCaptchaVerified.set[ipid]
	return ok
}

// markJoinCaptchaVerified records a solved challenge for an IPID, in memory and
// (when join_captcha_remember is on) in the database.
//
// With remember off, verification lasts only as long as the connection: every
// reconnect is challenged again. That is stricter, and an operator under
// sustained attack may want it, but it is not the default -- a player whose
// connection drops mid-scene should not have to re-solve a puzzle to get back
// into the room they were already in.
func markJoinCaptchaVerified(ipid, kind string) {
	if config == nil || !config.JoinCaptchaRemember {
		return
	}
	joinCaptchaVerified.mu.Lock()
	joinCaptchaVerified.set[ipid] = struct{}{}
	joinCaptchaVerified.mu.Unlock()

	go func() {
		if err := db.AddJoinCaptchaVerified(ipid, time.Now().Unix(), kind); err != nil {
			logger.LogErrorf("Failed to persist join-captcha verification for %v: %v", ipid, err)
		}
	}()
}

// forgetJoinCaptchaVerified clears one IPID's verification so its next
// connection is challenged again. Returns whether anything was cleared.
func forgetJoinCaptchaVerified(ipid string) bool {
	joinCaptchaVerified.mu.Lock()
	_, existed := joinCaptchaVerified.set[ipid]
	delete(joinCaptchaVerified.set, ipid)
	joinCaptchaVerified.mu.Unlock()

	go func() {
		if err := db.RemoveJoinCaptchaVerified(ipid); err != nil && err.Error() != "sql: no rows in result set" {
			logger.LogErrorf("Failed to clear join-captcha verification for %v: %v", ipid, err)
		}
	}()
	return existed
}

// forgetAllJoinCaptchaVerified clears every verification. Returns the number of
// database rows removed.
func forgetAllJoinCaptchaVerified() (int64, error) {
	joinCaptchaVerified.mu.Lock()
	joinCaptchaVerified.set = make(map[string]struct{})
	joinCaptchaVerified.mu.Unlock()
	return db.ClearJoinCaptchaVerified()
}

// joinCaptchaEnabled reports whether the feature is switched on.
func joinCaptchaEnabled() bool {
	return config != nil && config.JoinCaptcha
}

// joinCaptchaStrikeLimit returns the configured number of failures allowed
// before the action fires, with a sane floor so a misconfigured 0 cannot
// quarantine someone on their first blocked message.
func joinCaptchaStrikeLimit() int32 {
	if config == nil || config.JoinCaptchaStrikes < 1 {
		return 3
	}
	return int32(config.JoinCaptchaStrikes)
}

// joinCaptchaAction returns the configured strike-out action.
func joinCaptchaAction() string {
	if config == nil {
		return joinCaptchaActionQuarantine
	}
	switch strings.ToLower(strings.TrimSpace(config.JoinCaptchaAction)) {
	case joinCaptchaActionKick:
		return joinCaptchaActionKick
	default:
		return joinCaptchaActionQuarantine
	}
}

// issueJoinCaptcha challenges a joining client, unless it is exempt. Called
// once from pktReqDone after the join handshake has completed, so the OOC
// prompt lands in a client that is ready to render it.
//
// Moderators are exempt: their permissions come from an account, which they can
// only reach through /login, and /login is reachable while unverified anyway --
// but exempting them outright avoids challenging staff who are already
// authenticated from a previous action on this connection.
func issueJoinCaptcha(client *Client) {
	if !joinCaptchaEnabled() {
		return
	}
	if permissions.IsModerator(client.Perms()) {
		return
	}
	if isJoinCaptchaVerified(client.Ipid()) {
		return
	}

	// A configured plugin owns challenge generation entirely; the built-in
	// generators are the fallback for when none is configured or it is down.
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
		"%s\n\n    %s\n\n%s\n\nYou can look around and pick a character while you work it out -- you just can't send messages yet.",
		captchaBanner, c.Prompt, c.Hint))
	client.sendCaptchaPopup(c)

	go client.joinCaptchaTimeoutWatch()
}

// captchaBanner heads the challenge message.
const captchaBanner = "🔐 Verification required before you can chat."

// pendingChallenge returns the client's outstanding challenge.
func (client *Client) pendingChallenge() joinChallenge {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.pendingJoinChallenge
}

// joinCaptchaTimeoutWatch enforces join_captcha_timeout: a connection that
// never answers eventually strikes out on its own, so an idle bot that simply
// sits there is quarantined (or dropped) rather than lingering forever.
//
// One goroutine per challenged connection, exiting on client.done or as soon as
// the challenge is resolved, so it cannot outlive the connection.
func (client *Client) joinCaptchaTimeoutWatch() {
	timeout := 180
	if config != nil && config.JoinCaptchaTimeout > 0 {
		timeout = config.JoinCaptchaTimeout
	} else if config != nil && config.JoinCaptchaTimeout <= 0 {
		return // 0 disables the timeout; the gate simply stays up.
	}
	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timer.Stop()
	select {
	case <-client.done:
		return
	case <-timer.C:
		if !client.awaitingCaptcha.Load() {
			return
		}
		client.failJoinCaptcha("timed out")
	}
}

// joinCaptchaBlocked is the gate on every outbound speech path.
//
// It returns true when the caller must drop the message. The fast path is one
// atomic load: a server with the captcha off, or a player who has verified,
// never reaches the rest of this function.
//
// surface names what was blocked ("speak in character", "use commands") purely
// so the nudge can be specific about what the player just tried to do.
func (client *Client) joinCaptchaBlocked(surface string) bool {
	if !client.awaitingCaptcha.Load() {
		return false
	}
	strikes := client.captchaStrikes.Add(1)
	limit := joinCaptchaStrikeLimit()

	if strikes >= limit {
		client.failJoinCaptcha("used all attempts")
		return true
	}

	c := client.pendingChallenge()
	remaining := limit - strikes
	attempts := "attempts"
	if remaining == 1 {
		attempts = "attempt"
	}
	client.SendServerMessage(fmt.Sprintf(
		"🔐 You need to answer the verification question before you can %s.\n\n    %s\n\n%s\n\n(%d %s left.)",
		surface, c.Prompt, c.Hint, remaining, attempts))
	// Repeat the popup on a blocked attempt. Someone who tried to talk in
	// character has demonstrably not read the OOC tab, which is the one case
	// where a modal is worth the interruption. Bounded by the strike limit, so
	// this is a couple of dialogs at most before the outcome is decided.
	client.sendCaptchaPopup(c)
	return true
}

// sendCaptchaPopup shows the challenge in a client-side dialog via the AO2 BB
// packet, which desktop AO2 renders as a modal message box and WebAO as a
// notice. Sent alongside the OOC copy, never instead of it: the popup is
// unmissable but transient, and a player who dismisses it still needs the
// question sitting in the OOC log where they can re-read it.
func (client *Client) sendCaptchaPopup(c joinChallenge) {
	if config != nil && !config.JoinCaptchaPopup {
		return
	}
	client.Send(&packet.BB{Message: encode(fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\nType it in the OOC chat box (the one at the top), not in character.",
		captchaBanner, c.Prompt, c.Hint))})
}

// failJoinCaptcha applies the configured strike-out action.
//
// Under the default quarantine action nothing is sent to the player: the whole
// point is that they cannot tell the difference between passing and failing.
// The connection stops broadcasting, keeps receiving, and keeps seeing its own
// messages echoed back. Staff are told, and the area log keeps the text, so
// a real person caught by it is visible to a moderator and recoverable.
func (client *Client) failJoinCaptcha(why string) {
	if !client.awaitingCaptcha.Load() {
		return
	}
	if joinCaptchaAction() == joinCaptchaActionKick {
		client.awaitingCaptcha.Store(false)
		logger.LogInfof("Client (IPID:%v UID:%v) kicked by the join captcha (%v)", client.Ipid(), client.Uid(), why)
		alertJoinCaptchaStaff(client, "was kicked", why)
		client.SendSync(&packet.KK{Reason: "You didn't complete the verification question posted in OOC. " +
			"Reconnect and answer it with /verify <answer> to chat -- sorry for the hassle, it keeps the raids out."})
		client.markClosed()
		return
	}

	// Quarantine. awaitingCaptcha is cleared so the player stops being nudged
	// and their messages start "working"; captchaQuarantined takes over as the
	// flag the delivery paths consult. The pending challenge is deliberately
	// kept so a real player who works the answer out can still /verify and be
	// released without staff involvement.
	client.awaitingCaptcha.Store(false)
	if client.captchaQuarantined.CompareAndSwap(false, true) {
		activeCaptchaQuarantine.Add(1)
		logger.LogInfof("Client (IPID:%v UID:%v) shadow-quarantined by the join captcha (%v)", client.Ipid(), client.Uid(), why)
		logger.WriteAudit(fmt.Sprintf("%v | CAPTCHA_QUARANTINE | IPID:%v | UID:%v | %v",
			time.Now().UTC().Format("15:04:05"), client.Ipid(), client.Uid(), why))
		alertJoinCaptchaStaff(client, "was shadow-quarantined", why)
	}
}

// releaseJoinCaptcha clears both the pending challenge and any quarantine,
// records the verification and tells the player they are through. Shared by a
// correct /verify answer and a moderator's manual release.
func (client *Client) releaseJoinCaptcha(kind string, notify bool) {
	client.awaitingCaptcha.Store(false)
	if client.captchaQuarantined.CompareAndSwap(true, false) {
		activeCaptchaQuarantine.Add(-1)
	}
	client.captchaStrikes.Store(0)
	markJoinCaptchaVerified(client.Ipid(), kind)
	if notify {
		client.SendServerMessage("✅ Verified — you can chat now. Thanks for putting up with that; it keeps the bots out.")
	}
}

// tryJoinCaptchaAnswer checks a supplied answer, releasing the client on a
// match and burning a strike otherwise. Returns true when the answer was
// correct.
//
// A quarantined client is still allowed to answer: they are told nothing about
// their state either way, but a correct answer quietly restores them, so a
// real player who got there late is not silently muted for the session.
func (client *Client) tryJoinCaptchaAnswer(supplied string) bool {
	if !client.awaitingCaptcha.Load() && !client.captchaQuarantined.Load() {
		return false
	}
	c := client.pendingChallenge()

	var correct bool
	switch {
	case c.PluginToken != "":
		// Token mode: only the plugin knows the answer.
		ok, err := pluginVerify(client, c.PluginToken, supplied)
		if err != nil {
			// The plugin is unreachable, so this answer cannot be judged. It
			// may well have been right, and quarantining a player for a helper
			// process's outage is exactly the false positive to avoid -- so
			// nothing is counted against them and they are handed a fresh
			// question the server can judge on its own.
			logger.LogErrorf("Captcha plugin verify failed, reissuing a built-in question for IPID %v: %v", client.Ipid(), err)
			fresh := pickJoinChallenge(client.Ipid())
			client.mu.Lock()
			client.pendingJoinChallenge = fresh
			client.mu.Unlock()
			client.SendServerMessage(fmt.Sprintf(
				"Sorry — that one couldn't be checked just now. Here's a different question:\n\n    %s\n\n%s",
				fresh.Prompt, fresh.Hint))
			return false
		}
		correct = ok
	case len(c.Answers) > 0:
		correct = checkChallengeAnswer(c, supplied)
	}
	if !correct {
		return false
	}
	wasQuarantined := client.captchaQuarantined.Load()
	client.releaseJoinCaptcha(c.Kind, true)
	if wasQuarantined {
		logger.LogInfof("Client (IPID:%v UID:%v) answered correctly and was released from captcha quarantine", client.Ipid(), client.Uid())
	}
	return true
}

// alertJoinCaptchaStaff notifies online staff that the captcha acted on
// someone. A quarantine is invisible to its target by design, which makes it
// exactly the kind of state that must not also be invisible to moderators --
// otherwise a mis-tuned captcha silently mutes real players and nobody notices.
func alertJoinCaptchaStaff(client *Client, what, why string) {
	msg := fmt.Sprintf("[CAPTCHA] %v (UID: %v, IPID: %v) %v after failing verification (%v). Release with /joincaptcha verify %v.",
		client.CurrentCharacter(), client.Uid(), client.Ipid(), what, why, client.Uid())
	clients.ForEach(func(c *Client) {
		if permissions.HasPermission(c.Perms(), permissions.PermissionField["MOD_CHAT"]) {
			c.SendServerMessage(msg)
		}
	})
}

// joinCaptchaAllowedCommands are the only commands an unverified connection may
// run. Deliberately short: most commands broadcast something (/global, /pm,
// /roll, /a), and a gate a bot can talk through is not a gate. What is left is
// what a real, confused player needs -- answer the question, sign in to an
// existing account, or read the help.
var joinCaptchaAllowedCommands = map[string]struct{}{
	"verify": {},
	"login":  {},
	"help":   {},
	"about":  {},
}

// joinCaptchaCommandAllowed reports whether a command is usable while
// unverified.
func joinCaptchaCommandAllowed(command string) bool {
	_, ok := joinCaptchaAllowedCommands[strings.ToLower(command)]
	return ok
}

// --- shadow quarantine delivery ----------------------------------------------

// broadcastToQuarantine delivers a quarantined client's packet to the sender
// and to any other quarantined client in the same area -- the "shadow copy" of
// the room. Nobody else ever receives it.
//
// Including the other quarantined clients is the part that makes this hold up
// under testing: an attacker who connects two clients to check whether their
// messages actually appear sees them appear, because both ends are inside the
// same shadow. A quarantine that echoed only to the sender would be detectable
// with two connections in about ten seconds.
func broadcastToQuarantine(sender *Client, a *area.Area, p packet.Outgoing) {
	header, args := p.Header(), p.Args()
	clients.ForEach(func(c *Client) {
		if c == sender || (c.Area() == a && c.captchaQuarantined.Load()) {
			c.SendPacket(header, args...)
		}
	})
}

// --- /verify ------------------------------------------------------------------

// cmdVerify handles /verify <answer>: the player's reply to the join captcha.
func cmdVerify(client *Client, args []string, _ string) {
	if !joinCaptchaEnabled() {
		client.SendServerMessage("Verification is not enabled on this server — you can chat freely.")
		return
	}
	if !client.awaitingCaptcha.Load() && !client.captchaQuarantined.Load() {
		client.SendServerMessage("You're already verified — nothing to answer.")
		return
	}
	answer := strings.Join(args, " ")
	if client.tryJoinCaptchaAnswer(answer) {
		return
	}

	// Wrong answer. A quarantined client must not learn anything from this, so
	// it gets the same reply a correct-looking-but-wrong answer would give a
	// verified player: nothing that distinguishes the two states.
	if client.captchaQuarantined.Load() {
		client.SendServerMessage("You're already verified — nothing to answer.")
		return
	}

	strikes := client.captchaStrikes.Add(1)
	limit := joinCaptchaStrikeLimit()
	if strikes >= limit {
		client.failJoinCaptcha("used all attempts")
		return
	}
	c := client.pendingChallenge()
	remaining := limit - strikes
	attempts := "attempts"
	if remaining == 1 {
		attempts = "attempt"
	}
	client.SendServerMessage(fmt.Sprintf(
		"❌ That's not the answer.\n\n    %s\n\n%s\n\n(%d %s left.)",
		c.Prompt, c.Hint, remaining, attempts))
}

// --- /joincaptcha (staff) ------------------------------------------------------

// cmdJoinCaptcha handles the moderator side of the feature.
//
//	/joincaptcha status              — who is currently gated or quarantined
//	/joincaptcha verify <uid>        — release someone (false positive)
//	/joincaptcha reset <uid|ipid>    — clear a verification, re-challenge them
//	/joincaptcha reset all           — clear every verification
func cmdJoinCaptcha(client *Client, args []string, usage string) {
	if len(args) == 0 {
		client.SendServerMessage(usage)
		return
	}
	switch strings.ToLower(args[0]) {
	case "status":
		joinCaptchaStatus(client)
	case "verify", "release":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /joincaptcha verify <uid>")
			return
		}
		joinCaptchaManualVerify(client, args[1])
	case "reset":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /joincaptcha reset <uid|ipid|all>")
			return
		}
		joinCaptchaReset(client, args[1])
	default:
		client.SendServerMessage(usage)
	}
}

// joinCaptchaStatus lists connections the captcha is currently acting on.
func joinCaptchaStatus(client *Client) {
	if !joinCaptchaEnabled() {
		client.SendServerMessage("The join captcha is disabled (join_captcha = false).")
		return
	}
	var pending, quarantined []string
	clients.ForEach(func(c *Client) {
		switch {
		case c.awaitingCaptcha.Load():
			pending = append(pending, fmt.Sprintf("  UID %v (%v, IPID %v) — %v strike(s), challenge [%v]",
				c.Uid(), c.CurrentCharacter(), c.Ipid(), c.captchaStrikes.Load(), c.pendingChallenge().Kind))
		case c.captchaQuarantined.Load():
			quarantined = append(quarantined, fmt.Sprintf("  UID %v (%v, IPID %v) in %v",
				c.Uid(), c.CurrentCharacter(), c.Ipid(), c.Area().Name()))
		}
	})
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔐 Join captcha: on, action '%v', %v strike(s) allowed.\n", joinCaptchaAction(), joinCaptchaStrikeLimit()))
	if n := len(getCustomChallenges()); n > 0 {
		mode := "mixed with the built-in generators"
		if config != nil && config.JoinCaptchaCustomOnly {
			mode = "used exclusively (join_captcha_custom_only)"
		}
		sb.WriteString(fmt.Sprintf("%v custom question(s) loaded, %v.\n", n, mode))
	}
	sb.WriteString("\nAwaiting an answer:\n")
	if len(pending) == 0 {
		sb.WriteString("  (nobody)\n")
	} else {
		sb.WriteString(strings.Join(pending, "\n") + "\n")
	}
	sb.WriteString("\nShadow-quarantined (they don't know):\n")
	if len(quarantined) == 0 {
		sb.WriteString("  (nobody)")
	} else {
		sb.WriteString(strings.Join(quarantined, "\n"))
	}
	client.SendServerMessage(sb.String())
}

// joinCaptchaManualVerify releases a connection by UID, for a false positive.
func joinCaptchaManualVerify(client *Client, uidArg string) {
	uid, err := strconv.Atoi(uidArg)
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("No client with that UID.")
		return
	}
	if !target.awaitingCaptcha.Load() && !target.captchaQuarantined.Load() {
		client.SendServerMessage(fmt.Sprintf("UID %v is not gated by the captcha.", uid))
		return
	}
	target.releaseJoinCaptcha("staff", true)
	client.SendServerMessage(fmt.Sprintf("Released UID %v (IPID %v) from the join captcha.", uid, target.Ipid()))
	logger.LogInfof("%v released UID %v (IPID %v) from the join captcha", client.ModName(), uid, target.Ipid())
	addToBuffer(client, "CMD", fmt.Sprintf("Released UID %v from the join captcha.", uid), true)
}

// joinCaptchaReset clears a stored verification so the target is challenged
// again on their next connection. Accepts a connected UID, a raw IPID (so an
// offline player can be reset), or "all".
func joinCaptchaReset(client *Client, target string) {
	if strings.EqualFold(target, "all") {
		n, err := forgetAllJoinCaptchaVerified()
		if err != nil {
			client.SendServerMessage("Failed to clear join-captcha verifications.")
			logger.LogErrorf("Failed to clear join-captcha verifications: %v", err)
			return
		}
		client.SendServerMessage(fmt.Sprintf("Cleared %v join-captcha verification(s). Everyone will be challenged again on their next connection.", n))
		logger.LogInfof("%v cleared all join-captcha verifications (%v row(s))", client.ModName(), n)
		addToBuffer(client, "CMD", "Cleared all join-captcha verifications.", true)
		return
	}

	// A numeric argument is a UID; anything else is a raw IPID, matching
	// /musicunban and /unpunish so offline players can be reset too.
	ipid := target
	if uid, err := strconv.Atoi(target); err == nil {
		c, err := getClientByUid(uid)
		if err != nil {
			client.SendServerMessage("No client with that UID.")
			return
		}
		ipid = c.Ipid()
	}
	if !forgetJoinCaptchaVerified(ipid) {
		client.SendServerMessage(fmt.Sprintf("IPID %v had no stored verification.", ipid))
		return
	}
	client.SendServerMessage(fmt.Sprintf("Cleared the join-captcha verification for IPID %v — they'll be challenged again on their next connection.", ipid))
	logger.LogInfof("%v cleared the join-captcha verification for IPID %v", client.ModName(), ipid)
	addToBuffer(client, "CMD", fmt.Sprintf("Cleared the join-captcha verification for IPID %v.", ipid), true)
}

// joinCaptchaOnLogin clears the captcha for a client that has just signed in to
// an account, including one that had already been quarantined. Nothing is said
// to a quarantined player about having been quarantined -- they are simply
// released, which from their side looks like nothing happened at all.
func joinCaptchaOnLogin(client *Client) {
	if !client.awaitingCaptcha.Load() && !client.captchaQuarantined.Load() {
		return
	}
	wasQuarantined := client.captchaQuarantined.Load()
	client.releaseJoinCaptcha("login", !wasQuarantined)
	if wasQuarantined {
		logger.LogInfof("Client (IPID:%v UID:%v) released from captcha quarantine by logging in", client.Ipid(), client.Uid())
	}
}
