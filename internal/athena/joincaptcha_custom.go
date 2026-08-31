package athena

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// Operator-supplied captcha questions.
//
// Everything in joincaptcha_challenge.go is open source. Someone who reads this
// repository can enumerate all the built-in kinds and, with enough effort,
// write a solver for them. Rotating join_captcha_kinds raises the cost of that,
// but it does not change the fact that the built-in generators are public
// knowledge.
//
// The only structural fix is the same one the lockdown passkeys use for
// secretseed: keep the code public and the *data* private. Questions written by
// the operator into config/captcha_questions.txt are not in this repository and
// cannot be derived from it. An attacker has to obtain that file, and if they
// somehow do, the operator rewrites it in a minute.
//
// Set join_captcha_custom_only = true to serve nothing but these, and the
// public source reveals nothing at all about what a joining player is asked.
//
// File format -- one challenge per line:
//
//	question text | answer
//	question text | answer, alternative answer, another
//
// Lines that are empty or start with '#' are ignored. Answers are compared
// through normalizeCaptchaAnswer, so case, spaces and punctuation do not
// matter; listing alternatives handles "12" versus "twelve".

// customChallenges holds the loaded operator question set behind an atomic
// pointer, matching the other hot-reloadable lists (see livereload.go): the
// join path reads it without locking and /reload swaps it atomically.
var customChallenges atomic.Pointer[[]joinChallenge]

// getCustomChallenges returns the current operator question set, or nil.
func getCustomChallenges() []joinChallenge {
	if p := customChallenges.Load(); p != nil {
		return *p
	}
	return nil
}

// setCustomChallenges publishes a new operator question set.
func setCustomChallenges(c []joinChallenge) {
	customChallenges.Store(&c)
}

// defaultCaptchaQuestionsFile is the config-relative path used when
// join_captcha_questions is left blank.
const defaultCaptchaQuestionsFile = "/captcha_questions.txt"

// captchaQuestionsPath resolves the configured question file to a full path.
func captchaQuestionsPath() string {
	name := defaultCaptchaQuestionsFile
	if config != nil && strings.TrimSpace(config.JoinCaptchaQuestions) != "" {
		name = strings.TrimSpace(config.JoinCaptchaQuestions)
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
	}
	return settings.ConfigPath + name
}

// loadCustomChallenges parses the operator question file. A missing file is not
// an error -- the feature is optional, and its absence simply means the
// built-in generators are used on their own.
//
// Every entry is checked against the same invariant the built-in generators
// obey: the answer must not appear inside the question. An operator writing
// "Type the word cheese | cheese" would otherwise hand a scraper a free pass,
// so that entry is rejected at load time with a warning naming it, rather than
// silently weakening the gate. The check runs here, at load and on /reload,
// never on the join path.
func loadCustomChallenges(path string) ([]joinChallenge, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []joinChallenge
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		question, answerPart, ok := strings.Cut(line, "|")
		if !ok {
			logger.LogWarningf("%s line %d: no '|' separator, expected \"question | answer\" -- skipped: %q", path, lineNo, line)
			continue
		}
		question = strings.TrimSpace(question)
		if question == "" {
			logger.LogWarningf("%s line %d: empty question -- skipped", path, lineNo)
			continue
		}
		var answers []string
		for _, a := range strings.Split(answerPart, ",") {
			if na := normalizeCaptchaAnswer(a); na != "" {
				answers = append(answers, na)
			}
		}
		if len(answers) == 0 {
			logger.LogWarningf("%s line %d: no usable answer after '|' -- skipped: %q", path, lineNo, line)
			continue
		}
		c := joinChallenge{Kind: "custom", Prompt: question, Answers: answers}
		if challengeAnswerLeaks(c) {
			logger.LogWarningf("%s line %d: the answer appears inside the question itself, which makes it solvable by copying rather than reading -- skipped: %q",
				path, lineNo, line)
			continue
		}
		out = append(out, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// initCustomChallenges loads the operator question file at startup. Called from
// InitServer; failures are logged and leave the set empty rather than aborting
// startup, since the built-in generators still work on their own.
func initCustomChallenges() {
	path := captchaQuestionsPath()
	list, err := loadCustomChallenges(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.LogErrorf("Failed to load captcha questions from %v: %v", path, err)
		}
		setCustomChallenges(nil)
		return
	}
	setCustomChallenges(list)
	if len(list) > 0 {
		logger.LogInfof("Loaded %d custom captcha question(s) from %v.", len(list), path)
	}
}

// reloadCustomChallenges re-reads the question file for /reload. Returns an
// error only when the file exists but cannot be parsed, so a hot reload fails
// loudly on a broken file instead of quietly emptying the set. A missing file
// clears the set, matching "delete the file to turn the feature off".
func reloadCustomChallenges() error {
	path := captchaQuestionsPath()
	list, err := loadCustomChallenges(path)
	if err != nil {
		if os.IsNotExist(err) {
			setCustomChallenges(nil)
			return nil
		}
		return fmt.Errorf("captcha questions: %w", err)
	}
	setCustomChallenges(list)
	return nil
}

// pickJoinChallenge returns the next challenge to serve.
//
// With join_captcha_custom_only set, only operator questions are used and the
// public generators are never reached -- if the file is empty the server falls
// back to the built-ins anyway, because serving no captcha at all would silently
// disable the gate, which is worse than serving a guessable one.
//
// Otherwise operator questions are mixed into the built-in pool, weighted by
// how many there are: with a handful of custom questions most challenges are
// still generated, and an operator who writes a large set sees it dominate. The
// mix means an attacker cannot even rely on which source a given challenge came
// from.
func pickJoinChallenge(ipid string) joinChallenge {
	r := challengeRandFor(ipid)
	custom := getCustomChallenges()
	if config != nil && config.JoinCaptchaCustomOnly && len(custom) > 0 {
		return decorateChallenge(r, captchaPick(r, custom))
	}
	if len(custom) > 0 {
		// Weight custom questions at roughly len(custom) against a nominal
		// pool of built-in variety, capped so they can never crowd the
		// generators out entirely (a repeated custom question is guessable
		// in a way a generated one is not).
		weight := len(custom)
		if weight > 8 {
			weight = 8
		}
		if r.intn(weight+8) < weight {
			return decorateChallenge(r, captchaPick(r, custom))
		}
	}
	var kinds []string
	if config != nil {
		kinds = config.JoinCaptchaKinds
	}
	return newJoinChallenge(r, kinds)
}
