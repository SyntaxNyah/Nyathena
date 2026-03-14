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
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// jobDef describes a single player job: the internal key, the chip reward, and
// the cooldown in seconds.  Small rewards + long delays keep inflation low.
type jobDef struct {
	key      string // stored in JOB_COOLDOWNS; must be stable
	reward   int64  // chips awarded per use
	cooldown int64  // seconds between uses
	emoji    string
	flavour  string // short flavour text shown on success
}

// jobs is the registry of all available player jobs.
var jobs = map[string]jobDef{
	"janitor": {
		key:      "janitor",
		reward:   3,
		cooldown: 45 * 60, // 45 minutes
		emoji:    "🧹",
		flavour:  "You swept the courthouse floors.",
	},
	"busker": {
		key:      "busker",
		reward:   4,
		cooldown: 40 * 60, // 40 minutes
		emoji:    "🎸",
		flavour:  "You played music outside the courthouse for tips.",
	},
	"paperboy": {
		key:      "paperboy",
		reward:   3,
		cooldown: 50 * 60, // 50 minutes
		emoji:    "📰",
		flavour:  "You delivered legal briefs and newspapers around the block.",
	},
	"bailiffjob": {
		key:      "bailiffjob",
		reward:   5,
		cooldown: 60 * 60, // 60 minutes
		emoji:    "🛡️",
		flavour:  "You stood guard duty in the courtroom gallery.",
	},
	"clerk": {
		key:      "clerk",
		reward:   4,
		cooldown: 55 * 60, // 55 minutes
		emoji:    "📋",
		flavour:  "You filed paperwork at the records desk.",
	},
}

// doJob performs the named job for the client, checking and updating the
// cooldown, awarding chips, and messaging the player.
func doJob(client *Client, jobName string) {
	j, ok := jobs[jobName]
	if !ok {
		client.SendServerMessage("Unknown job.")
		return
	}

	onCooldown, remaining, err := db.CheckAndSetJobCooldown(client.Ipid(), j.key, j.cooldown)
	if err != nil {
		logger.LogErrorf("jobs: CheckAndSetJobCooldown failed for ipid=%v job=%v: %v", client.Ipid(), j.key, err)
		client.SendServerMessage("Something went wrong. Please try again later.")
		return
	}
	if onCooldown {
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
		return
	}

	newBal, chipErr := db.AddChips(client.Ipid(), j.reward)
	if chipErr != nil {
		logger.LogErrorf("jobs: AddChips failed for ipid=%v job=%v: %v", client.Ipid(), j.key, chipErr)
		client.SendServerMessage("Something went wrong awarding chips. Please try again later.")
		return
	}

	client.SendServerMessage(fmt.Sprintf(
		"%s %s +%d chips | Balance: %d chips",
		j.emoji, j.flavour, j.reward, newBal,
	))
}

// ── Command handlers ─────────────────────────────────────────────────────────

func cmdJanitor(client *Client, _ []string, _ string) { doJob(client, "janitor") }
func cmdBusker(client *Client, _ []string, _ string)  { doJob(client, "busker") }
func cmdPaperboy(client *Client, _ []string, _ string) { doJob(client, "paperboy") }
func cmdBailiffJob(client *Client, _ []string, _ string) { doJob(client, "bailiffjob") }
func cmdClerk(client *Client, _ []string, _ string)   { doJob(client, "clerk") }

// cmdJobs lists all available jobs and their rewards/cooldowns.
func cmdJobs(client *Client, _ []string, _ string) {
	var sb strings.Builder
	sb.WriteString("\n💼 Available Jobs (small chip rewards, long cooldowns)\n")
	sb.WriteString(fmt.Sprintf("  %-12s  %-6s  %s\n", "Command", "Reward", "Cooldown"))
	sb.WriteString("  " + strings.Repeat("─", 38) + "\n")
	order := []string{"janitor", "busker", "paperboy", "bailiffjob", "clerk"}
	for _, name := range order {
		j := jobs[name]
		cooldownMins := j.cooldown / 60
		sb.WriteString(fmt.Sprintf("  %s /%-10s  +%-4d   %d min\n",
			j.emoji, name, j.reward, cooldownMins))
	}
	sb.WriteString("\nType any of the commands above to work and earn chips!")
	client.SendServerMessage(sb.String())
}
