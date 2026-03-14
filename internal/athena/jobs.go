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
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// jobDef describes a single player job: the internal key, the base chip reward,
// and the cooldown in seconds.  Small rewards + long delays keep inflation low.
type jobDef struct {
	key      string // stored in JOB_COOLDOWNS; must be stable
	reward   int64  // base chips awarded per use
	cooldown int64  // seconds between uses
	emoji    string
}

// jobs is the registry of all available player jobs.
var jobs = map[string]jobDef{
	"janitor": {
		key:      "janitor",
		reward:   3,
		cooldown: 45 * 60, // 45 minutes
		emoji:    "🧹",
	},
	"busker": {
		key:      "busker",
		reward:   2, // base; actual tips are random 2–6
		cooldown: 30 * 60, // 30 minutes
		emoji:    "🎸",
	},
	"paperboy": {
		key:      "paperboy",
		reward:   3,
		cooldown: 60 * 60, // 60 minutes
		emoji:    "📰",
	},
	"bailiffjob": {
		key:      "bailiffjob",
		reward:   5,
		cooldown: 120 * 60, // 2 hours
		emoji:    "🛡️",
	},
	"clerk": {
		key:      "clerk",
		reward:   4,
		cooldown: 90 * 60, // 90 minutes
		emoji:    "📋",
	},
}

// ── Per-job interactive handlers ─────────────────────────────────────────────

// doJobJanitor handles the janitor job: flat 3-chip reward with a 25% chance
// to find a lost coin worth 1 bonus chip.
func doJobJanitor(client *Client) {
	j := jobs["janitor"]
	if onCooldown, remaining := checkJobCooldown(client, j); onCooldown {
		sendJobCooldownMsg(client, j, remaining)
		return
	}
	reward := j.reward
	msg := "You swept the courthouse floors."
	if rand.Intn(4) == 0 { // 25% chance
		reward++
		msg = "You swept the courthouse floors and found a lost coin on the way out!"
	}
	awardJobChips(client, j, reward, msg)
}

// doJobBusker handles the busker job: tips are randomised 2–6 chips and the
// performance is announced in the area's OOC chat so others can see it.
func doJobBusker(client *Client) {
	j := jobs["busker"]
	if onCooldown, remaining := checkJobCooldown(client, j); onCooldown {
		sendJobCooldownMsg(client, j, remaining)
		return
	}

	// Pick a random performance and tip amount.
	songs := []string{
		"a slow courtroom ballad",
		"an upbeat jazzy tune",
		"a dramatic Phoenix Wright medley",
		"a haunting violin piece",
		"a crowd-pleasing pop melody",
	}
	song := songs[rand.Intn(len(songs))]
	reward := int64(2 + rand.Intn(5)) // 2–6 chips

	name := client.OOCName()
	// Broadcast the performance to everyone in the area.
	sendAreaServerMessage(client.Area(), fmt.Sprintf(
		"🎸 %v is busking outside the courthouse, playing %s!", name, song,
	))

	var tipMsg string
	switch {
	case reward >= 5:
		tipMsg = fmt.Sprintf("The crowd loved your performance of %s! Generous tips flooded in.", song)
	case reward >= 4:
		tipMsg = fmt.Sprintf("Your performance of %s drew a decent crowd. A few people tipped.", song)
	default:
		tipMsg = fmt.Sprintf("You played %s to a sparse audience. A couple of coins clinked into your case.", song)
	}
	awardJobChips(client, j, reward, tipMsg)
}

// doJobPaperboy handles the paperboy job: flat 3-chip reward with a 15% chance
// for a generous reader to tip an extra 2 chips.
func doJobPaperboy(client *Client) {
	j := jobs["paperboy"]
	if onCooldown, remaining := checkJobCooldown(client, j); onCooldown {
		sendJobCooldownMsg(client, j, remaining)
		return
	}
	reward := j.reward
	msg := "You delivered legal briefs and newspapers around the block."
	if rand.Intn(100) < 15 { // 15% chance
		reward += 2
		msg = "You delivered papers and a grateful attorney handed you a generous tip!"
	}
	awardJobChips(client, j, reward, msg)
}

// doJobBailiff handles the bailiff job: flat 5-chip reward with a 10% chance
// to catch something suspicious and earn a 2-chip bonus.
func doJobBailiff(client *Client) {
	j := jobs["bailiffjob"]
	if onCooldown, remaining := checkJobCooldown(client, j); onCooldown {
		sendJobCooldownMsg(client, j, remaining)
		return
	}
	reward := j.reward
	msg := "You stood guard duty in the courtroom gallery."
	if rand.Intn(10) == 0 { // 10% chance
		reward += 2
		incidents := []string{
			"You spotted someone trying to sneak evidence out and stopped them. Well done!",
			"You caught a spectator attempting to disrupt the proceedings. Composure maintained.",
			"You noticed suspicious behaviour in the gallery and diffused the situation swiftly.",
		}
		msg = incidents[rand.Intn(len(incidents))]
	}
	awardJobChips(client, j, reward, msg)
}

// doJobClerk handles the clerk job: flat 4-chip reward with a 15% chance for
// an "overtime rush" that bumps the reward by 2 chips.
func doJobClerk(client *Client) {
	j := jobs["clerk"]
	if onCooldown, remaining := checkJobCooldown(client, j); onCooldown {
		sendJobCooldownMsg(client, j, remaining)
		return
	}
	reward := j.reward
	msg := "You filed paperwork at the records desk."
	if rand.Intn(100) < 15 { // 15% chance
		reward += 2
		msg = "Overtime rush at the records desk! You powered through a mountain of filings."
	}
	awardJobChips(client, j, reward, msg)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// checkJobCooldown checks the cooldown for a job without setting it.
// Returns (true, remaining) if on cooldown, (false, 0) otherwise.
// When not on cooldown it also sets the cooldown atomically.
func checkJobCooldown(client *Client, j jobDef) (bool, int64) {
	onCooldown, remaining, err := db.CheckAndSetJobCooldown(client.Ipid(), j.key, j.cooldown)
	if err != nil {
		logger.LogErrorf("jobs: CheckAndSetJobCooldown failed for ipid=%v job=%v: %v", client.Ipid(), j.key, err)
		client.SendServerMessage("Something went wrong. Please try again later.")
		return true, 0
	}
	return onCooldown, remaining
}

// sendJobCooldownMsg formats and sends a cooldown notice to the client.
func sendJobCooldownMsg(client *Client, j jobDef, remaining int64) {
	mins := remaining / 60
	secs := remaining % 60
	if mins > 0 {
		client.SendServerMessage(fmt.Sprintf(
			"%s You are tired. Come back in %dm %ds to work again.", j.emoji, mins, secs,
		))
	} else {
		client.SendServerMessage(fmt.Sprintf(
			"%s You are tired. Come back in %ds to work again.", j.emoji, secs,
		))
	}
}

// awardJobChips credits the reward, sends the player their result, and logs it.
func awardJobChips(client *Client, j jobDef, reward int64, flavour string) {
	newBal, chipErr := db.AddChips(client.Ipid(), reward)
	if chipErr != nil {
		logger.LogErrorf("jobs: AddChips failed for ipid=%v job=%v: %v", client.Ipid(), j.key, chipErr)
		client.SendServerMessage("Something went wrong awarding chips. Please try again later.")
		return
	}
	if err := db.AddJobEarnings(client.Ipid(), reward); err != nil {
		logger.LogErrorf("jobs: AddJobEarnings failed for ipid=%v job=%v: %v", client.Ipid(), j.key, err)
	}
	client.SendServerMessage(fmt.Sprintf(
		"%s %s +%d chips | Balance: %d chips",
		j.emoji, flavour, reward, newBal,
	))
}

// ── Command handlers ─────────────────────────────────────────────────────────

func cmdJanitor(client *Client, _ []string, _ string)    { doJobJanitor(client) }
func cmdBusker(client *Client, _ []string, _ string)     { doJobBusker(client) }
func cmdPaperboy(client *Client, _ []string, _ string)   { doJobPaperboy(client) }
func cmdBailiffJob(client *Client, _ []string, _ string) { doJobBailiff(client) }
func cmdClerk(client *Client, _ []string, _ string)      { doJobClerk(client) }

// cmdJobs lists all available jobs and their rewards/cooldowns.
func cmdJobs(client *Client, _ []string, _ string) {
	var sb strings.Builder
	sb.WriteString("\n💼 Available Jobs (earn chips with a cooldown per job)\n")
	sb.WriteString(fmt.Sprintf("  %-12s  %-10s  %-8s  %s\n", "Command", "Base", "Cooldown", "Notes"))
	sb.WriteString("  " + strings.Repeat("─", 56) + "\n")
	type row struct{ name, base, cd, note string }
	rows := []row{
		{"busker",     "2–6 chips", "30 min",  "Random tips; performs in area chat!"},
		{"janitor",    "3 chips",   "45 min",  "25% chance to find a coin (+1 bonus)"},
		{"paperboy",   "3 chips",   "60 min",  "15% chance for a generous tip (+2 bonus)"},
		{"clerk",      "4 chips",   "90 min",  "15% overtime rush chance (+2 bonus)"},
		{"bailiffjob", "5 chips",   "2 hours", "10% chance to catch something (+2 bonus)"},
	}
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("  /%-12s %-10s %-8s %s\n", r.name, r.base, r.cd, r.note))
	}
	sb.WriteString("\nType any of the commands above to get to work and earn chips!")
	sb.WriteString("\n💡 Use /job top to see who has earned the most from jobs.")
	client.SendServerMessage(sb.String())
}

// cmdJobTop handles /job top [n] — shows the job-earnings leaderboard.
func cmdJobTop(client *Client, args []string, usage string) {
	n := 10
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil && v > 0 && v <= 50 {
			n = v
		} else {
			client.SendServerMessage(usage)
			return
		}
	}
	entries, err := db.GetTopJobEarnings(n)
	if err != nil || len(entries) == 0 {
		client.SendServerMessage("No job earnings data available yet.")
		return
	}
	var sb strings.Builder
	sb.Grow(40 + len(entries)*35)
	sb.WriteString(fmt.Sprintf("\n💼 Job Earnings Leaderboard (Top %d)\n", len(entries)))
	for i, e := range entries {
		name := e.Username
		if name == "" {
			name = e.IPID
		}
		sb.WriteString(fmt.Sprintf("  %2d. %-20v  %d chips earned\n", i+1, name, e.Total))
	}
	client.SendServerMessage(sb.String())
}

