// SPDX-License-Identifier: AGPL-3.0-or-later
package athena

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

const (
	botDisplayName = "Kai-Bot-SG"
	botCooldown    = 5 * time.Minute
	botMaxSessions = 3
	botMaxRunes    = 250
	botPace        = 1300 * time.Millisecond
)

type botContent struct {
	Quotes      map[string]string
	Icebreakers []string
}
type botOptions struct {
	DailyQuote   bool
	Icebreaker   bool
	Color        aolib.TextColor
	IniCharacter string
	Emote        string
}

var botContentPtr atomic.Pointer[botContent]
var botOptionsPtr atomic.Pointer[botOptions]

// One server-wide concurrency and room cooldown budget for quote and icebreaker commands.
var botCommandLimiter = newBotLimiter()

func currentBotContent() *botContent {
	if x := botContentPtr.Load(); x != nil {
		return x
	}
	return &botContent{}
}
func currentBotOptions() botOptions {
	if x := botOptionsPtr.Load(); x != nil {
		return *x
	}
	return botOptions{}
}

func parseBotOptions(c *settings.Config) (botOptions, error) {
	if c == nil {
		return botOptions{}, errors.New("missing server configuration")
	}
	color := strings.ToLower(strings.TrimSpace(c.BotTextColor))
	if color == "" {
		color = "yellow"
	}
	switch color {
	case "white", "green", "red", "orange", "blue", "yellow", "pink", "cyan", "grey", "rainbow":
	default:
		return botOptions{}, fmt.Errorf("bot_text_color %q is invalid", color)
	}
	ini := strings.TrimSpace(c.BotIniCharacter)
	emote := strings.TrimSpace(c.BotEmote)
	if emote == "" {
		emote = "adult/normal"
	}
	if strings.ContainsAny(ini, "#\r\n") || strings.ContainsAny(emote, "#\r\n") {
		return botOptions{}, errors.New("bot character/emote names cannot contain packet delimiters")
	}
	return botOptions{DailyQuote: c.DailyQuote, Icebreaker: c.Icebreaker,
		Color: aolib.TextColor(color), IniCharacter: ini, Emote: emote}, nil
}

func publishBotOptions(c *settings.Config) (int, error) {
	next, err := parseBotOptions(c)
	if err != nil {
		return 0, err
	}
	old := botOptionsPtr.Load()
	if old != nil && *old == next {
		return 0, nil
	}
	botOptionsPtr.Store(&next)
	return 1, nil
}

func readBotLines(file string) ([]string, error) {
	path := filepath.Join(settings.ConfigPath, file)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if len(data) > 1024*1024 {
		return nil, fmt.Errorf("%s: exceeds 1 MiB", file)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: invalid UTF-8", file)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 64*1024)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "#") {
			return nil, fmt.Errorf("%s: '#' is not allowed in IC text", file)
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return lines, nil
}

func parseQuoteLines(lines []string) (map[string]string, error) {
	out := make(map[string]string, len(lines))
	for i, line := range lines {
		date, txt, ok := strings.Cut(line, "|")
		date, txt = strings.TrimSpace(date), strings.TrimSpace(txt)
		if !ok || len(date) != 5 || date[2] != '-' || txt == "" {
			return nil, fmt.Errorf("quote.txt: line %d should be MM-DD | quote", i+1)
		}
		month, erM := strconv.Atoi(date[:2])
		day, erD := strconv.Atoi(date[3:])
		if erM != nil || erD != nil {
			return nil, fmt.Errorf("quote.txt: invalid date %q", date)
		}
		// Use a leap year so a custom February 29 entry can be valid.
		t := time.Date(2024, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if int(t.Month()) != month || t.Day() != day {
			return nil, fmt.Errorf("quote.txt: invalid date %q", date)
		}
		if _, exists := out[date]; exists {
			return nil, fmt.Errorf("quote.txt: duplicate date %q", date)
		}
		if utf8.RuneCountInString(txt) > 3000 {
			return nil, fmt.Errorf("quote.txt: quote too long on %s", date)
		}
		out[date] = txt
	}
	return out, nil
}
func parseQuestionLines(lines []string) ([]string, error) {
	for _, line := range lines {
		if utf8.RuneCountInString(line) > 3000 {
			return nil, errors.New("icebreakers.txt: question exceeds 3000 characters")
		}
	}
	return append([]string(nil), lines...), nil
}
func loadBotContent() (*botContent, error) {
	qlines, err := readBotLines("quote.txt")
	if err != nil {
		return nil, err
	}
	quotes, err := parseQuoteLines(qlines)
	if err != nil {
		return nil, err
	}
	ilines, err := readBotLines("icebreakers.txt")
	if err != nil {
		return nil, err
	}
	questions, err := parseQuestionLines(ilines)
	if err != nil {
		return nil, err
	}
	return &botContent{Quotes: quotes, Icebreakers: questions}, nil
}
func initBotContent() {
	data, err := loadBotContent()
	if err != nil {
		logger.LogWarningf("bot: local data not loaded: %v", err)
		return
	}
	botContentPtr.Store(data)
}
func botContentDifferences(next *botContent) []string {
	old := currentBotContent()
	var changed []string
	if !equalQuoteMaps(old.Quotes, next.Quotes) {
		changed = append(changed, "quote.txt")
	}
	if !equalStrSlices(old.Icebreakers, next.Icebreakers) {
		changed = append(changed, "icebreakers.txt")
	}
	return changed
}
func equalQuoteMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
func botQuoteForDate(content *botContent, when time.Time) string {
	day := when.Format("01-02")
	if day == "02-29" {
		if s := content.Quotes[day]; s != "" {
			return s
		}
		day = "02-28"
	}
	return content.Quotes[day]
}

func botSprite(opts botOptions) string {
	if opts.IniCharacter != "" {
		return opts.IniCharacter
	}
	if chars := getCharacters(); len(chars) > 0 {
		return chars[0]
	}
	return "Phoenix"
}
func botSendIC(room *area.Area, txt, sprite, showname string, opts botOptions) {
	max := botMaxRunes
	if config != nil && config.MaxMsg > 0 && config.MaxMsg < max {
		max = config.MaxMsg
	}
	if max < 1 {
		return
	}
	parts := splitBotText(txt, max)
	for i, part := range parts {
		if i > 0 {
			time.Sleep(botPace)
		}
		broadcastToArea(room, &MSToClient{
			DeskModifier: aolib.DeskModifierShown, Character: encode(sprite), Emote: encode(opts.Emote),
			Message: encode(part), Side: aolib.SideWit, EmoteModifier: aolib.EmoteModifierNoPreanim,
			CharID: "-1", SfxDelay: "0", Evidence: "0", Realization: "0", TextColor: opts.Color,
			Showname: encode(showname), PairedCharID: "-1", NoninterruptingPreanim: "0",
			SfxLooping: "0", Screenshake: "0", Additive: "0",
		})
	}
}
func splitBotText(txt string, limit int) []string {
	if limit < 1 {
		return nil
	}
	var parts []string
	current := ""
	for _, word := range strings.Fields(txt) {
		r := []rune(word)
		if current != "" && len([]rune(current))+1+len(r) > limit {
			parts = append(parts, current)
			current = ""
		}
		for len(r) > limit {
			parts = append(parts, string(r[:limit]))
			r = r[limit:]
		}
		if len(r) == 0 {
			continue
		}
		if current == "" {
			current = string(r)
		} else {
			current += " " + string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

type botLimiter struct {
	mu     sync.Mutex
	busy   map[*area.Area]bool
	last   map[*area.Area]time.Time
	active int
}

func newBotLimiter() *botLimiter {
	return &botLimiter{busy: make(map[*area.Area]bool), last: make(map[*area.Area]time.Time)}
}
func (l *botLimiter) reserve(room *area.Area) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.busy[room] {
		return false, "Kai-Bot-SG is already speaking in this room."
	}
	if wait := botCooldown - time.Since(l.last[room]); !l.last[room].IsZero() && wait > 0 {
		return false, fmt.Sprintf("Please wait %v before requesting this command again here.", wait.Round(time.Second))
	}
	if l.active >= botMaxSessions {
		return false, "Kai-Bot-SG is busy in other rooms. Try again shortly."
	}
	l.busy[room] = true
	l.active++
	return true, ""
}
func (l *botLimiter) release(room *area.Area, success bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.busy[room] {
		return
	}
	delete(l.busy, room)
	l.active--
	if success {
		l.last[room] = time.Now()
	}
}

func botRandomIcebreaker(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[rand.Intn(len(list))]
}
