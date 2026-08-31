package athena

import (
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/plugin"
)

// Captcha plugin bridge.
//
// Everything in this repository is AGPL and public, which means the built-in
// challenge generators are public too. They are written to survive that (the
// answer is never in the question, the kind mix is configurable, the keyed
// derivation is unpredictable without the server's secret) but a determined
// attacker who reads the source still starts with a map of what they are up
// against.
//
// A plugin removes that. Point captcha_plugin at your own program and the
// server stops generating challenges at all: it asks the plugin for a question,
// shows it, and passes answers back. What the question is, how it is built and
// how it is checked live in a binary that is not in this repository and that
// nobody but the operator has. The public code shows only that a captcha
// exists and where it attaches -- which was never the secret worth keeping.
//
// Two modes, chosen by the plugin per challenge:
//
//   - It returns the accepted answers with the question. The server then checks
//     answers locally, so a wrong guess costs no IPC. Simple, and fine when the
//     plugin's value is the *questions* rather than the checking.
//
//   - It returns no answers, only an opaque token. The server then calls back
//     with the token and the player's answer for every attempt, and the plugin
//     alone decides. In this mode the answer never exists inside the server
//     process at all -- a memory dump of the server tells an attacker nothing.
//
// If the plugin is not configured, or is down, the built-in generators are used
// instead. That is deliberate: a captcha that stops working because a helper
// crashed would open the gate wide during exactly the incident it exists for.

// captchaPlugin is the running plugin, nil when none is configured.
var (
	captchaPlugin     *plugin.Plugin
	captchaPluginOnce sync.Once
)

// captchaPluginChallengeReq is sent to the plugin to request a question.
// The IPID lets a plugin key its own per-address logic (rate limiting, a
// harder question for a suspicious range) without the server dictating any.
type captchaPluginChallengeReq struct {
	Ipid string `json:"ipid"`
	Uid  int    `json:"uid"`
	Area string `json:"area"`
}

// captchaPluginChallengeResp is the plugin's answer.
//
// Answers, when supplied, are normalized by the server exactly like a built-in
// challenge's, so a plugin does not have to worry about case or punctuation.
// Token is opaque to the server: whatever the plugin needs to recognise this
// challenge later, handed back verbatim on verify.
type captchaPluginChallengeResp struct {
	Prompt  string   `json:"prompt"`
	Hint    string   `json:"hint,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Token   string   `json:"token,omitempty"`
	Answers []string `json:"answers,omitempty"`
}

// captchaPluginVerifyReq asks the plugin to judge an answer.
type captchaPluginVerifyReq struct {
	Ipid   string `json:"ipid"`
	Uid    int    `json:"uid"`
	Token  string `json:"token"`
	Answer string `json:"answer"`
}

// captchaPluginVerifyResp is the plugin's verdict.
type captchaPluginVerifyResp struct {
	Ok bool `json:"ok"`
}

// initCaptchaPlugin starts the configured captcha plugin. Called once from
// InitServer; a blank captcha_plugin leaves the feature inert.
func initCaptchaPlugin() {
	captchaPluginOnce.Do(func() {
		if config == nil || strings.TrimSpace(config.CaptchaPlugin) == "" {
			return
		}
		timeout := time.Duration(config.CaptchaPluginTimeout) * time.Millisecond
		captchaPlugin = plugin.New("captcha", strings.TrimSpace(config.CaptchaPlugin), timeout, logger.LogInfof)
		captchaPlugin.Start()
		logger.LogInfof("Captcha plugin configured: %v", config.CaptchaPlugin)
	})
}

// stopCaptchaPlugin shuts the plugin down at server exit.
func stopCaptchaPlugin() {
	if captchaPlugin != nil {
		captchaPlugin.Stop()
	}
}

// captchaPluginActive reports whether a plugin is configured and up.
func captchaPluginActive() bool {
	return captchaPlugin != nil && captchaPlugin.Running()
}

// pluginChallengeFor asks the plugin for a challenge. Returns ok=false on any
// failure, so the caller falls back to the built-in generators.
func pluginChallengeFor(client *Client) (joinChallenge, bool) {
	if !captchaPluginActive() {
		return joinChallenge{}, false
	}
	areaName := ""
	if a := client.Area(); a != nil {
		areaName = a.Name()
	}
	var resp captchaPluginChallengeResp
	err := captchaPlugin.Call("challenge", captchaPluginChallengeReq{
		Ipid: client.Ipid(), Uid: client.Uid(), Area: areaName,
	}, &resp)
	if err != nil {
		logger.LogErrorf("Captcha plugin challenge failed, using a built-in question instead: %v", err)
		return joinChallenge{}, false
	}
	if strings.TrimSpace(resp.Prompt) == "" {
		logger.LogErrorf("Captcha plugin returned an empty prompt, using a built-in question instead")
		return joinChallenge{}, false
	}
	// A plugin that returns neither answers nor a token has given the server no
	// way to ever judge a reply -- the player could never pass. Reject it here
	// rather than issuing an unanswerable question.
	if len(resp.Answers) == 0 && strings.TrimSpace(resp.Token) == "" {
		logger.LogErrorf("Captcha plugin returned no answers and no token, so the question could never be passed; using a built-in one instead")
		return joinChallenge{}, false
	}

	kind := resp.Kind
	if kind == "" {
		kind = "plugin"
	}
	hint := resp.Hint
	if strings.TrimSpace(hint) == "" {
		hint = "Reply with:  /verify <your answer>"
	}
	c := joinChallenge{
		Kind:        kind,
		Prompt:      resp.Prompt,
		Hint:        hint,
		PluginToken: resp.Token,
	}
	for _, a := range resp.Answers {
		if na := normalizeCaptchaAnswer(a); na != "" {
			c.Answers = append(c.Answers, na)
		}
	}
	return c, true
}

// pluginVerify asks the plugin to judge an answer for a token-only challenge.
//
// A plugin failure here cannot be treated as "wrong": the player may well have
// answered correctly, and silently quarantining them for the helper's outage is
// the false-positive case this whole feature has to avoid. The caller reissues
// a fresh challenge instead -- see tryJoinCaptchaAnswer.
func pluginVerify(client *Client, token, answer string) (ok bool, err error) {
	if !captchaPluginActive() {
		return false, plugin.ErrUnavailable
	}
	var resp captchaPluginVerifyResp
	if err := captchaPlugin.Call("verify", captchaPluginVerifyReq{
		Ipid: client.Ipid(), Uid: client.Uid(), Token: token, Answer: answer,
	}, &resp); err != nil {
		return false, err
	}
	return resp.Ok, nil
}
