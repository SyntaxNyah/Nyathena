package athena

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// ─── Configurable newspaper section keys ───────────────────────────────────

// NewspaperSectionKey names a single article section.
// These are the string values that appear in the config file.
const (
	NewspaperSectionChipLeaderboard  = "chip_leaderboard"
	NewspaperSectionPlaytimeTop      = "playtime_top"
	NewspaperSectionUnscrambleTop    = "unscramble_top"
	NewspaperSectionRecentBans       = "recent_bans"
	NewspaperSectionServerStats      = "server_stats"
	NewspaperSectionWeather          = "weather"
	NewspaperSectionHoroscope        = "horoscope"
	NewspaperSectionWordOfTheDay     = "word_of_the_day"
	NewspaperSectionCasino           = "casino"
	NewspaperSectionAreaHighlight    = "area_highlight"
	NewspaperSectionJobMarket        = "job_market"
	NewspaperSectionPunishmentReport = "punishment_report"
	NewspaperSectionTip              = "tip_of_the_day"
	NewspaperSectionClassifieds      = "classifieds"
	NewspaperSectionHolidayGreeting  = "holiday_greeting"
)

// defaultNewspaperSections lists the sections shown when the config doesn't
// specify any, in display order.
var defaultNewspaperSections = []string{
	NewspaperSectionServerStats,
	NewspaperSectionWeather,
	NewspaperSectionHoroscope,
	NewspaperSectionTip,
	NewspaperSectionChipLeaderboard,
	NewspaperSectionAreaHighlight,
}

// ─── Flavour data pools ────────────────────────────────────────────────────

var newspaperWeatherReports = []string{
	"☀️ CLEAR: Bright and sunny. Recommended attire: your finest casual courtroom ensemble.",
	"🌧️ RAINY: Light showers forecast. Bring an umbrella and your objection skills.",
	"⛈️ STORMY: Thunderstorms ahead. Stay indoors and practice cross-examination.",
	"🌨️ SNOWY: A dusting of snow expected. Hot tea and evidence review recommended.",
	"🌬️ WINDY: Strong gusts reported. Hold onto your documents tightly.",
	"🌫️ FOGGY: Visibility low. Perfect conditions for mysterious RP.",
	"🌤️ PARTLY CLOUDY: Mixed skies. Much like the current case status.",
	"🌡️ HEATWAVE: Temperatures soaring. Fans advised; also the ventilation kind.",
	"🌈 RAINBOW: Following yesterday's storm, a rainbow of justice shines.",
	"❄️ FROST: Cold snap overnight. Evidence may be frozen pending review.",
}

var newspaperHoroscopes = []struct {
	sign string
	msg  string
}{
	{"♈ Aries", "Bold moves today, but think before you object."},
	{"♉ Taurus", "Patience is your greatest virtue — wait for the contradiction."},
	{"♊ Gemini", "Two sides to every story; you understand both."},
	{"♋ Cancer", "Trust your gut. The evidence will follow your instincts."},
	{"♌ Leo", "Your dramatic flair is an asset in court today."},
	{"♍ Virgo", "Attention to detail will reveal the hidden truth."},
	{"♎ Libra", "Balance and fairness guide you toward the right verdict."},
	{"♏ Scorpio", "Secrets are your domain — the witness is hiding something."},
	{"♐ Sagittarius", "Big-picture thinking leads to unexpected breakthroughs."},
	{"♑ Capricorn", "Methodical preparation wins the day. Trust your notes."},
	{"♒ Aquarius", "An unconventional approach surprises everyone — including yourself."},
	{"♓ Pisces", "Intuition over logic today; feel the flow of the courtroom."},
}

var newspaperWordsOfTheDay = []struct {
	word string
	def  string
}{
	{"Veridical", "adj. Coinciding with reality; truthful. (e.g., a veridical account of events)"},
	{"Sophistry", "n. The use of clever but false arguments to deceive. A prosecutor's nightmare."},
	{"Probative", "adj. Having the quality of proving or demonstrating something; evidential."},
	{"Recusant", "n. One who refuses to submit to authority. See also: the defendant, frequently."},
	{"Exculpatory", "adj. Tending to clear from fault or guilt. The kind of evidence you want."},
	{"Inculpatory", "adj. Tending to establish fault or guilt. The kind of evidence you don't."},
	{"Voir Dire", "n. Preliminary examination of a witness to determine competence to testify."},
	{"Subornation", "n. The act of inducing another to commit perjury. Highly frowned upon."},
	{"Pellucid", "adj. Transparently clear; easily understood. Strive for this in testimony."},
	{"Obfuscate", "v. To render obscure or unclear. What bad witnesses do under pressure."},
	{"Jurisprudence", "n. The theory and philosophy of law. The foundation of this whole enterprise."},
	{"Sequester", "v. To isolate, especially jurors, to prevent outside influence."},
	{"Caveat", "n. A warning or qualification. Every verdict comes with one."},
	{"Moot", "adj. Subject to debate; having no practical significance. Like this newspaper."},
	{"Indemnity", "n. Security against or exemption from liability or legal responsibility."},
	{"Deposition", "n. The sworn testimony of a witness taken outside of court."},
	{"Culpable", "adj. Deserving blame or censure as being wrong or harmful."},
	{"Affidavit", "n. A written statement confirmed by oath, used as evidence in court."},
	{"Adjudicate", "v. To make a formal judgment on a disputed matter."},
	{"Tort", "n. A civil wrong that gives rise to a claim for damages. Not a cake, unfortunately."},
}

var newspaperTipsOfTheDay = []string{
	"💡 Always present your evidence before making an accusation — logic first, drama second.",
	"💡 A calm witness is a dangerous witness. Watch for what they're NOT saying.",
	"💡 Cross-examination is an art: one question at a time, never ask what you don't know.",
	"💡 The truth doesn't contradict itself. Find the contradiction, find the lie.",
	"💡 Circumstantial evidence stacks. One coincidence is nothing; five is a case.",
	"💡 Always know your client's alibi before you walk into the courtroom.",
	"💡 The opposing attorney is not your enemy — the truth is the goal for everyone.",
	"💡 Keep your notes organised. A messy desk loses cases.",
	"💡 Listen more than you speak. The answer is usually in what's already been said.",
	"💡 Hearsay is inadmissible, but the story behind it often points to the real truth.",
	"💡 Never underestimate a quiet defendant. Still waters run deep.",
	"💡 If the timeline doesn't work, the alibi doesn't either.",
	"💡 Always be the most prepared person in the room.",
	"💡 A good objection isn't about interrupting — it's about protecting the record.",
	"💡 Winning on a technicality is still winning, but winning on the truth is better.",
	"💡 Rest is important. Even the sharpest mind dulls without sleep.",
	"💡 The gallery is watching. Composure under pressure commands respect.",
	"💡 Every piece of evidence tells half a story. Find the other half.",
	"💡 Doubt is not proof of innocence, but certainty is not proof of guilt.",
	"💡 The gavel falls eventually. Make sure it falls on the right side.",
}

var newspaperClassifieds = []string{
	"🔎 LOST: One crucial piece of evidence. Last seen during cross-examination. If found, please notify the defence.",
	"📋 FOR SALE: Slightly used objection card. Only cried on once. Asking 50 chips.",
	"🏠 ROOM FOR RENT: Cosy jail cell, recently vacated. Amenities include one bunk and existential dread.",
	"📢 WANTED: Reliable alibi. Must be verifiable and not involve 'I was asleep'. Contact defence, urgently.",
	"🎓 TUTORING: Learn the art of dramatic pointing. Guaranteed 'OBJECTION!' improvement. 3 chips/session.",
	"🐾 FOUND: One stray cat in the evidence room. Answers to 'Exhibit B'. Please claim promptly.",
	"⚖️ FREE TO GOOD HOME: One double contradiction. Barely used. May cause prosecution anxiety.",
	"🍕 CATERING: Phoenix Wright's Takeaway. Specialising in cold cases and hot takes. Now open.",
	"🔧 REPAIRS: Broken logic fixed overnight. No case too circular. Call Dr. Gavel.",
	"📰 NOTICE: The courtroom vending machine is still out of order. Management apologises for any testimony delays.",
}

var newspaperHolidayGreetings = []string{
	"🎉 Happy New Year from everyone at Nyathena! May your evidence be irrefutable and your objections fly true.",
	"🎃 Happy Halloween! The court is in session — even the ghosts must testify.",
	"🎄 Happy Holidays! May your Christmas be filled with acquittals and peace.",
	"💝 Happy Valentine's Day! Even the toughest prosecutor has a heart... somewhere.",
	"🐰 Happy Easter! The truth, like an egg, is always hidden — but findable.",
	"🦃 Happy Thanksgiving! We are grateful for every piece of exculpatory evidence.",
	"🌸 Happy Spring! New beginnings, fresh cases, and blooming alibis.",
	"☀️ Happy Summer! Court is briefly in recess. Stay cool and hydrated.",
	"🍂 Happy Autumn! The leaves fall, and so do bad testimonies.",
	"🎆 Happy Independence Day! Freedom is the verdict we work toward every day.",
}

// ─── Section builders ─────────────────────────────────────────────────────

// buildNewspaperSection generates one article section and returns its text.
// Returns an empty string if the section cannot be generated (e.g. DB error).
func buildNewspaperSection(section string) string {
	switch section {
	case NewspaperSectionChipLeaderboard:
		return buildSectionChipLeaderboard()
	case NewspaperSectionPlaytimeTop:
		return buildSectionPlaytimeTop()
	case NewspaperSectionUnscrambleTop:
		return buildSectionUnscrambleTop()
	case NewspaperSectionRecentBans:
		return buildSectionRecentBans()
	case NewspaperSectionServerStats:
		return buildSectionServerStats()
	case NewspaperSectionWeather:
		return buildSectionWeather()
	case NewspaperSectionHoroscope:
		return buildSectionHoroscope()
	case NewspaperSectionWordOfTheDay:
		return buildSectionWordOfTheDay()
	case NewspaperSectionCasino:
		return buildSectionCasino()
	case NewspaperSectionAreaHighlight:
		return buildSectionAreaHighlight()
	case NewspaperSectionJobMarket:
		return buildSectionJobMarket()
	case NewspaperSectionPunishmentReport:
		return buildSectionPunishmentReport()
	case NewspaperSectionTip:
		return buildSectionTip()
	case NewspaperSectionClassifieds:
		return buildSectionClassifieds()
	case NewspaperSectionHolidayGreeting:
		return buildSectionHolidayGreeting()
	default:
		return ""
	}
}

func buildSectionChipLeaderboard() string {
	entries, err := db.GetTopChipBalances(5)
	if err != nil || len(entries) == 0 {
		return "💰 CHIP LEADERBOARD\nNo data available yet."
	}
	var sb strings.Builder
	sb.WriteString("💰 CHIP LEADERBOARD — Top Earners\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %d. %s — %d chips\n", i+1, e.Username, e.Balance))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildSectionPlaytimeTop() string {
	entries, err := db.GetTopPlaytimes(5)
	if err != nil || len(entries) == 0 {
		return "⏱️ PLAYTIME HALL OF FAME\nNo data available yet."
	}
	var sb strings.Builder
	sb.WriteString("⏱️ PLAYTIME HALL OF FAME — Most Dedicated Players\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %d. %s — %s\n", i+1, e.Username, formatPlaytime(e.Playtime)))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildSectionUnscrambleTop() string {
	entries, err := db.GetTopUnscrambleWins(5)
	if err != nil || len(entries) == 0 {
		return "🔤 UNSCRAMBLE CHAMPIONS\nNo data available yet."
	}
	var sb strings.Builder
	sb.WriteString("🔤 UNSCRAMBLE CHAMPIONS — Fastest Solvers\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %d. %s — %d win(s)\n", i+1, e.Username, e.Wins))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildSectionRecentBans() string {
	bans, err := db.GetRecentBans()
	if err != nil || len(bans) == 0 {
		return "⚖️ RECENT JUDGEMENTS\nThe courtroom has been quiet lately."
	}
	var sb strings.Builder
	sb.WriteString("⚖️ RECENT JUDGEMENTS — Justice Served\n")
	for _, b := range bans {
		reason := b.Reason
		if reason == "" {
			reason = "no reason given"
		}
		sb.WriteString(fmt.Sprintf("  • Banned by %s — %s\n", b.Moderator, reason))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildSectionServerStats() string {
	count := clients.Count()
	return fmt.Sprintf("📊 SERVER STATS\n  Currently online: %d player(s)\n  Areas available: %d", count, len(areas))
}

func buildSectionWeather() string {
	report := newspaperWeatherReports[rand.Intn(len(newspaperWeatherReports))]
	return "🗞️ WEATHER REPORT\n  " + report
}

func buildSectionHoroscope() string {
	h := newspaperHoroscopes[rand.Intn(len(newspaperHoroscopes))]
	return fmt.Sprintf("🔮 HOROSCOPE OF THE DAY\n  %s: %s", h.sign, h.msg)
}

func buildSectionWordOfTheDay() string {
	w := newspaperWordsOfTheDay[rand.Intn(len(newspaperWordsOfTheDay))]
	return fmt.Sprintf("📚 WORD OF THE DAY\n  %s\n  %s", w.word, w.def)
}

func buildSectionCasino() string {
	if config == nil || !config.EnableCasino {
		return ""
	}
	entries, err := db.GetTopChipBalances(3)
	if err != nil || len(entries) == 0 {
		return "🎰 CASINO REPORT\nNo casino activity to report."
	}
	var sb strings.Builder
	sb.WriteString("🎰 CASINO REPORT — High Rollers This Issue\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %d. %s — %d chips\n", i+1, e.Username, e.Balance))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildSectionAreaHighlight() string {
	// Pick the area with the most players, or a random one if empty.
	bestName := ""
	bestCount := 0
	for _, a := range areas {
		n := a.PlayerCount()
		if n > bestCount {
			bestCount = n
			bestName = a.Name()
		}
	}
	if bestName == "" || bestCount == 0 {
		if len(areas) > 0 {
			bestName = areas[rand.Intn(len(areas))].Name()
		} else {
			return ""
		}
		return fmt.Sprintf("📍 AREA SPOTLIGHT\n  Area \"%s\" is ready and waiting for roleplayers!", bestName)
	}
	return fmt.Sprintf("📍 AREA SPOTLIGHT\n  Most active area right now: \"%s\" with %d player(s). Come join the action!", bestName, bestCount)
}

func buildSectionJobMarket() string {
	entries, err := db.GetTopJobEarnings(3)
	if err != nil || len(entries) == 0 {
		return "💼 JOB MARKET REPORT\nNo job earnings data available yet."
	}
	var sb strings.Builder
	sb.WriteString("💼 JOB MARKET REPORT — Top Workers This Issue\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("  %d. %s — %d chips earned\n", i+1, e.Username, e.Total))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildSectionPunishmentReport() string {
	// Count currently punished clients.
	count := 0
	clients.ForEach(func(c *Client) {
		if len(c.Punishments()) > 0 {
			count++
		}
	})
	if count == 0 {
		return "🎭 PUNISHMENT REPORT\n  All players are behaving themselves today. Remarkable."
	}
	return fmt.Sprintf("🎭 PUNISHMENT REPORT\n  %d player(s) are currently under active punishment effects. Justice is ongoing.", count)
}

func buildSectionTip() string {
	tip := newspaperTipsOfTheDay[rand.Intn(len(newspaperTipsOfTheDay))]
	return "📌 TIP OF THE DAY\n  " + tip
}

func buildSectionClassifieds() string {
	ad := newspaperClassifieds[rand.Intn(len(newspaperClassifieds))]
	return "📋 CLASSIFIEDS\n  " + ad
}

func buildSectionHolidayGreeting() string {
	greeting := newspaperHolidayGreetings[rand.Intn(len(newspaperHolidayGreetings))]
	return "🎊 SPECIAL NOTICE\n  " + greeting
}

// ─── Issue generation ──────────────────────────────────────────────────────

// generateNewspaper builds and broadcasts one complete newspaper issue.
func generateNewspaper() {
	sections := defaultNewspaperSections
	if config != nil && len(config.NewspaperSections) > 0 {
		sections = config.NewspaperSections
	}

	now := time.Now()
	header := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📰  THE NYATHENA DAILY  —  %s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		now.Format("January 2, 2006"),
	)
	sendGlobalServerMessage(header)

	for _, section := range sections {
		text := buildNewspaperSection(section)
		if strings.TrimSpace(text) == "" {
			continue
		}
		sendGlobalServerMessage(text)
	}

	footer := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n" +
		"That's all for today's edition. Stay curious, stay just."
	sendGlobalServerMessage(footer)
}

// startNewspaperLoop runs in the background and broadcasts newspapers at the
// configured interval.  It should only be launched when EnableNewspaper is true.
func startNewspaperLoop() {
	intervalStr := "24h"
	if config != nil && config.NewspaperInterval != "" {
		intervalStr = config.NewspaperInterval
	}
	d, err := str2duration.ParseDuration(intervalStr)
	if err != nil || d <= 0 {
		logger.LogErrorf("newspaper: invalid interval %q, defaulting to 24h", intervalStr)
		d = 24 * time.Hour
	}

	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for range ticker.C {
		generateNewspaper()
	}
}

// ─── Admin command ────────────────────────────────────────────────────────

// cmdNewspaper allows moderators to trigger a newspaper issue immediately or
// configure which sections to show.
//
// Usage: /newspaper [now]
func cmdNewspaper(client *Client, args []string, usage string) {
	if len(args) > 0 && strings.ToLower(args[0]) == "now" {
		if config == nil || !config.EnableNewspaper {
			client.SendServerMessage("The newspaper system is not enabled (enable_newspaper = false in config).")
			return
		}
		client.SendServerMessage("Publishing newspaper issue now…")
		go generateNewspaper()
		return
	}
	// Show current configuration.
	sections := defaultNewspaperSections
	if config != nil && len(config.NewspaperSections) > 0 {
		sections = config.NewspaperSections
	}
	interval := "24h"
	if config != nil && config.NewspaperInterval != "" {
		interval = config.NewspaperInterval
	}
	enabled := config != nil && config.EnableNewspaper
	client.SendServerMessage(fmt.Sprintf(
		"📰 Newspaper config:\n  Enabled: %v\n  Interval: %s\n  Sections: %s\n\nAvailable sections:\n  "+
			"chip_leaderboard, playtime_top, unscramble_top, recent_bans, server_stats, weather, "+
			"horoscope, word_of_the_day, casino, area_highlight, job_market, punishment_report, "+
			"tip_of_the_day, classifieds, holiday_greeting",
		enabled, interval, strings.Join(sections, ", "),
	))
}
