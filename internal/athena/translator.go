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
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// translatorLanguages maps friendly language names (and aliases) to ISO-639-1
// codes understood by MyMemory / LibreTranslate style APIs.  Keys are
// lower-cased; callers should lower-case input before lookup.
// Raw two-letter ISO codes are also accepted directly, bypassing this map.
var translatorLanguages = map[string]string{
	"english":    "en",
	"french":     "fr",
	"francais":   "fr",
	"spanish":    "es",
	"espanol":    "es",
	"german":     "de",
	"deutsch":    "de",
	"italian":    "it",
	"portuguese": "pt",
	"japanese":   "ja",
	"korean":     "ko",
	"chinese":    "zh-CN",
	"mandarin":   "zh-CN",
	"russian":    "ru",
	"arabic":     "ar",
	"hindi":      "hi",
	"dutch":      "nl",
	"polish":     "pl",
	"turkish":    "tr",
	"swedish":    "sv",
	"norwegian":  "no",
	"danish":     "da",
	"finnish":    "fi",
	"greek":      "el",
	"hebrew":     "he",
	"thai":       "th",
	"vietnamese": "vi",
	"indonesian": "id",
	"malay":      "ms",
	"czech":      "cs",
	"hungarian":  "hu",
	"romanian":   "ro",
	"ukrainian":  "uk",
	"bulgarian":  "bg",
	"croatian":   "hr",
	"serbian":    "sr",
	"slovak":     "sk",
	"slovenian":  "sl",
	"latin":      "la",
	"welsh":      "cy",
	"irish":      "ga",
	"latvian":    "lv",
	"lithuanian": "lt",
	"estonian":   "et",
	"persian":    "fa",
	"farsi":      "fa",
	"urdu":       "ur",
	"bengali":    "bn",
	"tamil":      "ta",
	"swahili":    "sw",
	"afrikaans":  "af",
	"albanian":   "sq",
	"catalan":    "ca",
	"filipino":   "tl",
	"tagalog":    "tl",
	"icelandic":  "is",
	"macedonian": "mk",
	"mongolian":  "mn",
	"esperanto":  "eo",
}

// translatorRandomPool is the set of language codes picked from when the
// target language is "random".  Curated for variety without being so large
// that the per-word API hit becomes unbounded.
var translatorRandomPool = []string{
	"fr", "es", "de", "it", "pt", "ja", "ko", "zh-CN", "ru", "ar",
	"hi", "nl", "pl", "tr", "sv", "el", "he", "th", "vi", "id",
	"cs", "hu", "ro", "uk", "bg", "la", "fi", "no", "da", "eo",
}

// translatorCache is a bounded in-memory cache keyed on "<targetLang>\x1f<text>".
// It absorbs repeated IC messages (players often spam the same line) so a
// single translation is reused until the server restarts.
const translatorCacheMax = 4096

var translatorCache = struct {
	mu sync.Mutex
	m  map[string]string
}{m: make(map[string]string)}

// translatorHTTPClient is shared across all translator lookups so TCP
// connections to the translation endpoint are reused (HTTP keep-alive).
var translatorHTTPClient = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:       4,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: true,
	},
}

// translatorEnabled reports whether the /translator punishment is both
// switched on and fully configured.  Callers should fall back to the original
// text when this returns false.
func translatorEnabled() bool {
	return config != nil && config.EnableTranslator && config.TranslatorAPIKey != "" && config.TranslatorAPIURL != ""
}

// resolveLanguage converts a user-supplied language name/code to the ISO code
// used by the translation API.  Returns the empty string when the input is
// not recognised and not already a plausible 2–5 char code.
func resolveLanguage(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	if code, ok := translatorLanguages[name]; ok {
		return code
	}
	// Accept raw codes like "fr", "zh-CN" (validated lightly by length).
	if l := len(name); l >= 2 && l <= 6 {
		return name
	}
	return ""
}

// sourceLang returns the configured source language or "en" as a sane default.
func sourceLang() string {
	if config != nil && config.TranslatorSourceLang != "" {
		return config.TranslatorSourceLang
	}
	return "en"
}

// myMemoryResponse mirrors the MyMemory JSON shape.  Only fields we use are
// decoded; everything else is ignored.
type myMemoryResponse struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
	ResponseStatus json.Number `json:"responseStatus"`
}

// queryTranslator calls the configured translation endpoint and returns the
// translated string.  Assumes a MyMemory-compatible GET API:
//
//	<apiURL>?q=<text>&langpair=<src>|<tgt>&key=<apiKey>
//
// MyMemory is the reference implementation because its free tier accepts
// anonymous and keyed requests on the same endpoint, matching the user's
// "super free lenient API key" expectation.
func queryTranslator(text, targetLang string) (string, error) {
	endpoint := config.TranslatorAPIURL
	apiKey := config.TranslatorAPIKey
	src := sourceLang()

	params := url.Values{}
	params.Set("q", text)
	params.Set("langpair", src+"|"+targetLang)
	if apiKey != "" {
		params.Set("key", apiKey)
	}

	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	reqURL := endpoint + sep + params.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := translatorHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translator API returned status %d", resp.StatusCode)
	}

	var result myMemoryResponse
	// Cap response at 32 KiB — translated IC messages are tiny.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32*1024)).Decode(&result); err != nil {
		return "", err
	}
	translated := strings.TrimSpace(result.ResponseData.TranslatedText)
	if translated == "" {
		return "", fmt.Errorf("translator API returned empty translation")
	}
	return translated, nil
}

// translateCached wraps queryTranslator with an in-memory cache.  Identical
// (lang, text) pairs resolve instantly after the first hit.
func translateCached(text, targetLang string) (string, error) {
	key := targetLang + "\x1f" + text

	translatorCache.mu.Lock()
	if v, ok := translatorCache.m[key]; ok {
		translatorCache.mu.Unlock()
		return v, nil
	}
	translatorCache.mu.Unlock()

	translated, err := queryTranslator(text, targetLang)
	if err != nil {
		return "", err
	}

	translatorCache.mu.Lock()
	// Simple size cap: drop a random entry when full rather than implementing
	// LRU tracking.  Cache is purely best-effort.
	if len(translatorCache.m) >= translatorCacheMax {
		for k := range translatorCache.m {
			delete(translatorCache.m, k)
			break
		}
	}
	translatorCache.m[key] = translated
	translatorCache.mu.Unlock()

	return translated, nil
}

// applyTranslator translates the whole message into targetLang, or (when
// targetLang is "random") translates each word into an independently-chosen
// random language.  Falls back to the original text on any error so a flaky
// API never swallows a player's message.
func applyTranslator(text, targetLang string) string {
	if !translatorEnabled() {
		return text
	}
	if strings.TrimSpace(text) == "" {
		return text
	}

	lang := strings.ToLower(strings.TrimSpace(targetLang))
	if lang == "random" {
		return applyTranslatorRandom(text)
	}

	code := resolveLanguage(lang)
	if code == "" {
		return text
	}
	translated, err := translateCached(text, code)
	if err != nil {
		logger.LogWarningf("translator: %v", err)
		return text
	}
	return translated
}

// applyTranslatorRandom translates each whitespace-delimited word into an
// independently-chosen random language.  Cap at 24 words to avoid
// runaway API fan-out on pathological inputs (ellipsis-heavy monologues, etc.).
func applyTranslatorRandom(text string) string {
	const maxWords = 24
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	capped := len(words) > maxWords
	work := words
	if capped {
		work = words[:maxWords]
	}

	for i, w := range work {
		lang := translatorRandomPool[rand.Intn(len(translatorRandomPool))]
		translated, err := translateCached(w, lang)
		if err != nil {
			logger.LogWarningf("translator (random, word %q): %v", w, err)
			continue
		}
		work[i] = translated
	}

	if capped {
		// Re-attach the untouched tail so long messages aren't truncated.
		return strings.Join(append(work, words[maxWords:]...), " ")
	}
	return strings.Join(work, " ")
}
