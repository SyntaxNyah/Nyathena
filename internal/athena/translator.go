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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// translatorDefaultCooldown is the fallback cooldown for /translate when the
// config value is zero or negative.
const translatorDefaultCooldown = 25 * time.Second

// translator targets DeepL's REST API.  The free tier
// (https://api-free.deepl.com/v2/translate, 500 000 chars/month) fits the
// "lenient free key" brief; pro users can point translator_api_url at
// https://api.deepl.com/v2/translate without code changes.
//
// API shape (summarised from DeepL docs):
//   POST  <endpoint>
//   Authorization: DeepL-Auth-Key <key>
//   Content-Type:  application/x-www-form-urlencoded
//   Body:  text=<word>&text=<word>&target_lang=<UPPERCASE>&source_lang=<UPPERCASE>
//   Reply: {"translations":[{"detected_source_language":"EN","text":"..."}]}
// Multiple `text` parameters may be sent in one request and are translated in
// the same order they arrive; we exploit this to batch random-mode per-word
// translations into one request per target language.

// translatorLanguages maps friendly English language names (and aliases) to
// DeepL target codes (UPPERCASE, ISO-639-1, with regional variants like
// EN-US / PT-BR supported directly).  Keys are lower-cased; callers should
// lower-case input before lookup.  Raw DeepL codes are also accepted and
// uppercased by resolveLanguage.
var translatorLanguages = map[string]string{
	// DeepL-supported families only.
	"arabic":     "AR",
	"bulgarian":  "BG",
	"chinese":    "ZH",
	"mandarin":   "ZH",
	"czech":      "CS",
	"danish":     "DA",
	"dutch":      "NL",
	"english":    "EN-US",
	"british":    "EN-GB",
	"american":   "EN-US",
	"estonian":   "ET",
	"finnish":    "FI",
	"french":     "FR",
	"francais":   "FR",
	"german":     "DE",
	"deutsch":    "DE",
	"greek":      "EL",
	"hebrew":     "HE",
	"hungarian":  "HU",
	"indonesian": "ID",
	"italian":   "IT",
	"japanese":   "JA",
	"korean":     "KO",
	"latvian":    "LV",
	"lithuanian": "LT",
	"norwegian":  "NB",
	"polish":     "PL",
	"portuguese": "PT-PT",
	"brazilian":  "PT-BR",
	"romanian":   "RO",
	"russian":    "RU",
	"slovak":     "SK",
	"slovenian":  "SL",
	"spanish":    "ES",
	"espanol":    "ES",
	"swedish":    "SV",
	"thai":       "TH",
	"turkish":    "TR",
	"ukrainian":  "UK",
	"vietnamese": "VI",
}

// translatorRandomPool is the set of language codes picked from when the
// target language is "random".  Curated for variety using only DeepL-
// supported codes so the free tier never rejects a request as unsupported.
var translatorRandomPool = []string{
	"FR", "ES", "DE", "IT", "PT-BR", "JA", "KO", "ZH", "RU", "AR",
	"NL", "PL", "TR", "SV", "EL", "HE", "TH", "VI", "ID", "CS",
	"HU", "RO", "UK", "BG", "FI", "NB", "DA", "SK", "SL", "LV",
}

// Latency budgets.  Every translator call honours a hard deadline so the
// cursed player's IC pipeline is never stalled for longer than this by the
// upstream API.  On timeout the original text is used — the message is never
// swallowed.
const (
	translatorPerReqTimeout = 1500 * time.Millisecond // single API request
	translatorOverallBudget = 2500 * time.Millisecond // whole applyTranslator call
	translatorRandomMaxPar  = 6                       // concurrent batch workers
	translatorRandomMaxWord = 24                      // words translated in random mode
)

// translatorCache is a bounded in-memory cache keyed on "<targetLang>\x1f<text>".
// It absorbs repeated IC messages (players often spam the same line) so a
// single translation is reused until the server restarts.
const translatorCacheMax = 4096

var translatorCache = struct {
	mu sync.Mutex
	m  map[string]string
}{m: make(map[string]string)}

// translatorHTTPClient is shared across all translator lookups so TCP
// connections to the DeepL endpoint are reused (HTTP keep-alive).  The
// Client.Timeout is a belt-and-braces cap on top of the per-request context
// deadline — either one firing aborts the request.
var translatorHTTPClient = &http.Client{
	Timeout: translatorPerReqTimeout,
	Transport: &http.Transport{
		MaxIdleConns:       8,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: true,
	},
}

// Simple circuit breaker: when the upstream API fails repeatedly we stop
// hitting it for a cooldown window.  Without this, every cursed IC message
// would pay the full timeout round-trip while the API is unreachable.
const (
	translatorFailThreshold = 4                // consecutive failures before opening
	translatorOpenDuration  = 60 * time.Second // how long the breaker stays open
)

var (
	translatorFailStreak atomic.Int32
	translatorOpenUntil  atomic.Int64 // UnixNano; 0 = closed
)

// breakerOpen reports whether the circuit breaker is currently open, meaning
// callers must skip the API and fall back to the original text.
func breakerOpen() bool {
	until := translatorOpenUntil.Load()
	if until == 0 {
		return false
	}
	if time.Now().UnixNano() >= until {
		// Half-open: clear the breaker and let the next call try again.
		translatorOpenUntil.Store(0)
		translatorFailStreak.Store(0)
		return false
	}
	return true
}

func recordTranslatorSuccess() {
	translatorFailStreak.Store(0)
}

func recordTranslatorFailure() {
	streak := translatorFailStreak.Add(1)
	if streak >= translatorFailThreshold {
		translatorOpenUntil.Store(time.Now().Add(translatorOpenDuration).UnixNano())
	}
}

// translatorEnabled reports whether the /translator punishment is both
// switched on and fully configured.  Callers should fall back to the original
// text when this returns false.
func translatorEnabled() bool {
	return config != nil && config.EnableTranslator && config.TranslatorAPIKey != "" && config.TranslatorAPIURL != ""
}

// resolveLanguage converts a user-supplied language name/code to the DeepL
// target code (UPPERCASE) used on the wire.  Returns the empty string when
// the input is not recognised and not already a plausible code.
func resolveLanguage(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	if code, ok := translatorLanguages[name]; ok {
		return code
	}
	// Accept raw DeepL codes like "fr", "en-us", "zh-hans" and uppercase them.
	if l := len(name); l >= 2 && l <= 7 {
		return strings.ToUpper(name)
	}
	return ""
}

// deepLResponse mirrors the DeepL JSON shape.  Only fields we use are decoded.
type deepLResponse struct {
	Translations []struct {
		DetectedSourceLanguage string `json:"detected_source_language"`
		Text                   string `json:"text"`
	} `json:"translations"`
}

// queryTranslatorBatch POSTs up to len(texts) strings to DeepL in a single
// request and returns their translations in the same order.  The texts slice
// must not be empty.
func queryTranslatorBatch(ctx context.Context, texts []string, targetLang string) ([]string, error) {
	endpoint := config.TranslatorAPIURL
	apiKey := config.TranslatorAPIKey

	form := url.Values{}
	for _, t := range texts {
		form.Add("text", t)
	}
	form.Set("target_lang", strings.ToUpper(targetLang))
	// Omit source_lang: DeepL auto-detects the input language.  Forcing a
	// source (e.g. EN) caused the curse to no-op whenever a cursed player
	// typed in a non-matching language (Spanish, Russian, etc.) — DeepL
	// would return the input unchanged, bypassing the punishment.

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "DeepL-Auth-Key "+apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := translatorHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepL returned status %d", resp.StatusCode)
	}

	var result deepLResponse
	// Cap response at 64 KiB — translated IC messages are tiny but batches
	// in random mode can carry up to ~24 short strings.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Translations) != len(texts) {
		return nil, fmt.Errorf("DeepL returned %d translations for %d texts", len(result.Translations), len(texts))
	}
	out := make([]string, len(texts))
	for i, tr := range result.Translations {
		out[i] = strings.TrimSpace(tr.Text)
		if out[i] == "" {
			out[i] = texts[i] // keep the original rather than emitting a blank
		}
	}
	return out, nil
}

// translateCachedBatch returns translations for each input text, consulting
// the in-memory cache first and batching any cache misses into a single
// upstream request.  On any API error (or breaker-open) missing entries
// fall back to the original text so the IC message is never blank.
func translateCachedBatch(ctx context.Context, texts []string, targetLang string) []string {
	out := make([]string, len(texts))
	var missIdx []int
	var missText []string

	translatorCache.mu.Lock()
	for i, t := range texts {
		key := targetLang + "\x1f" + t
		if v, ok := translatorCache.m[key]; ok {
			out[i] = v
		} else {
			missIdx = append(missIdx, i)
			missText = append(missText, t)
		}
	}
	translatorCache.mu.Unlock()

	if len(missText) == 0 {
		return out
	}

	// Fill remaining slots with the original text up front so any failure
	// path below leaves the message intact.
	for _, i := range missIdx {
		out[i] = texts[i]
	}

	if breakerOpen() {
		return out
	}

	translations, err := queryTranslatorBatch(ctx, missText, targetLang)
	if err != nil {
		recordTranslatorFailure()
		return out
	}
	recordTranslatorSuccess()

	translatorCache.mu.Lock()
	for n, i := range missIdx {
		tr := translations[n]
		out[i] = tr
		if len(translatorCache.m) >= translatorCacheMax {
			// Simple random eviction — good enough for best-effort cache.
			for k := range translatorCache.m {
				delete(translatorCache.m, k)
				break
			}
		}
		translatorCache.m[targetLang+"\x1f"+texts[i]] = tr
	}
	translatorCache.mu.Unlock()

	return out
}

// applyTranslator translates the whole message into targetLang, or (when
// targetLang is "random") translates each word into an independently-chosen
// random language.  Falls back to the original text on any error so a flaky
// API never swallows a player's message, and enforces a hard overall deadline
// so the IC pipeline is never stalled for longer than translatorOverallBudget.
func applyTranslator(text, targetLang string) string {
	if !translatorEnabled() {
		return text
	}
	if strings.TrimSpace(text) == "" {
		return text
	}

	ctx, cancel := context.WithTimeout(context.Background(), translatorOverallBudget)
	defer cancel()

	lang := strings.ToLower(strings.TrimSpace(targetLang))
	if lang == "random" {
		return applyTranslatorRandom(ctx, text)
	}

	code := resolveLanguage(lang)
	if code == "" {
		return text
	}
	translated := translateCachedBatch(ctx, []string{text}, code)
	if translated[0] == "" {
		logger.LogWarningf("translator: empty translation for lang %q", code)
		return text
	}
	return translated[0]
}

// applyTranslatorRandom translates each whitespace-delimited word into an
// independently-chosen random language.  Words are grouped by their assigned
// target language and each group is batched into a single DeepL POST, then
// the batches are executed concurrently under a shared deadline.  Any word
// whose call errors or exceeds the deadline keeps its original form so the
// IC message is never blank or truncated.
func applyTranslatorRandom(ctx context.Context, text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	capped := len(words) > translatorRandomMaxWord
	out := make([]string, len(words))
	copy(out, words)

	end := len(words)
	if capped {
		end = translatorRandomMaxWord
	}

	// Group eligible words by their randomly chosen target language so each
	// language's words can be fetched in a single batch request.
	groups := make(map[string][]int, 8)
	for i := 0; i < end; i++ {
		lang := translatorRandomPool[rand.Intn(len(translatorRandomPool))]
		groups[lang] = append(groups[lang], i)
	}

	sem := make(chan struct{}, translatorRandomMaxPar)
	var wg sync.WaitGroup
	var mu sync.Mutex // guards out[]

	for lang, idxs := range groups {
		lang := lang
		idxs := idxs
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			texts := make([]string, len(idxs))
			for n, i := range idxs {
				texts[n] = words[i]
			}
			translated := translateCachedBatch(ctx, texts, lang)
			mu.Lock()
			for n, i := range idxs {
				if translated[n] != "" {
					out[i] = translated[n]
				}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	return strings.Join(out, " ")
}

// cmdTranslate is the /translate command handler.  Any player may use it to
// translate a snippet of text via DeepL.  Requires enable_translator_punishment
// to be true and translator_api_key to be set — if the punishment system is
// configured, the same key and endpoint are reused so no extra configuration
// is needed.
//
// Usage: /translate <text> <language>
//
// The last whitespace-separated token is taken as the target language; all
// preceding tokens are the text to translate.  This matches the natural usage
// shown in the problem statement:
//
//	/translate はっはっはっ！！うっせえ、ボケ！！ english
func cmdTranslate(client *Client, args []string, usage string) {
	if !translatorEnabled() {
		client.SendServerMessage(
			"The translation feature is not available on this server.\n" +
				"(Requires enable_translator_punishment = true and translator_api_key set in config.toml.)")
		return
	}

	// args has at least 2 entries (minArgs = 2): text token(s) + language.
	// The registry joined them as individual whitespace tokens, so the last
	// element is always the target language and the rest is the text.
	lang := args[len(args)-1]
	text := strings.Join(args[:len(args)-1], " ")

	isRandom := strings.EqualFold(lang, "random")
	code := resolveLanguage(lang)
	if !isRandom && code == "" {
		client.SendServerMessage(fmt.Sprintf(
			"Unknown language %q.\n"+
				"  • English names — french, spanish, japanese, german, russian, arabic, ...\n"+
				"  • ISO codes     — fr, es, ja, de, ru, ar, zh-CN, ...\n"+
				"  • Keyword       — random  (each word translated into a different language)\n"+
				"Example: /translate Good morning! japanese",
			lang))
		return
	}

	// Enforce per-player cooldown.  Moderators bypass it entirely.
	cd := translatorDefaultCooldown
	if config != nil && config.TranslateCooldown > 0 {
		cd = time.Duration(config.TranslateCooldown) * time.Second
	}
	if client.Perms() == 0 {
		if ok, remaining := client.CheckAndUpdateTranslateCooldown(cd); !ok {
			secs := int(remaining.Seconds()) + 1
			unit := "seconds"
			if secs == 1 {
				unit = "second"
			}
			client.SendServerMessage(fmt.Sprintf(
				"Please wait %d %s before using /translate again.", secs, unit))
			return
		}
	}

	// Check if the API is reachable before burning the cooldown slot output.
	// breakerOpen is a fast local check — no network round-trip.
	if breakerOpen() {
		client.SendServerMessage("Translation unavailable right now. Please try again later.")
		return
	}

	translated := applyTranslator(text, lang)

	// Build a human-readable label for the output line.
	var displayLang string
	switch {
	case isRandom:
		displayLang = "random (multiple languages)"
	case code != "":
		displayLang = code
	default:
		displayLang = strings.ToUpper(lang)
	}
	client.SendServerMessage(fmt.Sprintf("[Translate → %s] %s", displayLang, translated))
}
