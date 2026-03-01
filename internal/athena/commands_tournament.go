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
	"sort"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

func cmdTournament(client *Client, args []string, usage string) {
	if len(args) < 1 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}

	action := strings.ToLower(args[0])

	switch action {
	case "start":
		tournamentMutex.Lock()
		defer tournamentMutex.Unlock()

		if tournamentActive {
			client.SendServerMessage("A tournament is already active.")
			return
		}

		tournamentActive = true
		tournamentStartTime = time.Now().UTC()
		tournamentParticipants = make(map[int]*TournamentParticipant)

		client.SendServerMessage("Tournament started! Users can now join with /join-tournament")
		writeToAllClients("CT", "OOC", "🏆 TOURNAMENT STARTED! Join with /join-tournament to compete! Random punishments will be applied.")
		addToBuffer(client, "CMD", "Started punishment tournament", false)

	case "stop":
		tournamentMutex.Lock()
		defer tournamentMutex.Unlock()

		if !tournamentActive {
			client.SendServerMessage("No tournament is currently active.")
			return
		}

		// Determine winner
		var winner *TournamentParticipant
		var winnerClient *Client
		for uid, participant := range tournamentParticipants {
			if winner == nil || participant.messageCount > winner.messageCount {
				winner = participant
				winnerClient = clients.GetClientByUID(uid)
			}
		}

		tournamentActive = false

		if winner != nil && winnerClient != nil {
			duration := time.Since(tournamentStartTime).Round(time.Second)
			announcement := fmt.Sprintf("🏆 TOURNAMENT ENDED! Winner: UID %d with %d messages over %v! Congratulations!",
				winner.uid, winner.messageCount, duration)
			writeToAllClients("CT", "OOC", announcement)
			
			// Remove all punishments from winner (memory and DB).
			winnerClient.RemoveAllPunishments()
			if err := db.DeleteAllPunishments(winnerClient.Ipid()); err != nil {
				logger.LogErrorf("Failed to remove persistent punishments for tournament winner %v: %v", winnerClient.Ipid(), err)
			}
			winnerClient.SendServerMessage("Congratulations! Your tournament punishments have been removed.")
		} else {
			writeToAllClients("CT", "OOC", "🏆 TOURNAMENT ENDED! No participants.")
		}

		tournamentParticipants = make(map[int]*TournamentParticipant)
		addToBuffer(client, "CMD", "Stopped punishment tournament", false)

	case "status":
		tournamentMutex.Lock()
		defer tournamentMutex.Unlock()

		if !tournamentActive {
			client.SendServerMessage("No tournament is currently active.")
			return
		}

		duration := time.Since(tournamentStartTime).Round(time.Second)
		msg := fmt.Sprintf("🏆 TOURNAMENT STATUS (Running for %v)\n", duration)
		msg += fmt.Sprintf("Participants: %d\n\n", len(tournamentParticipants))

		// Build leaderboard sorted by message count
		type leaderEntry struct {
			uid      int
			msgCount int
			duration time.Duration
		}
		var leaderboard []leaderEntry
		for uid, participant := range tournamentParticipants {
			leaderboard = append(leaderboard, leaderEntry{
				uid:      uid,
				msgCount: participant.messageCount,
				duration: time.Since(participant.joinedAt).Round(time.Second),
			})
		}

		// Sort by message count (descending)
		sort.Slice(leaderboard, func(i, j int) bool {
			return leaderboard[i].msgCount > leaderboard[j].msgCount
		})

		msg += "LEADERBOARD:\n"
		for i, entry := range leaderboard {
			rank := i + 1
			msg += fmt.Sprintf("%d. UID %d - %d messages (%v in tournament)\n",
				rank, entry.uid, entry.msgCount, entry.duration)
		}

		client.SendServerMessage(msg)

	default:
		client.SendServerMessage("Invalid action. Use: start, stop, or status")
	}
}

// cmdJoinTournament allows users to join the active tournament
func cmdJoinTournament(client *Client, args []string, usage string) {
	tournamentMutex.Lock()
	defer tournamentMutex.Unlock()

	if !tournamentActive {
		client.SendServerMessage("No tournament is currently active.")
		return
	}

	uid := client.Uid()
	if _, exists := tournamentParticipants[uid]; exists {
		client.SendServerMessage("You are already in the tournament!")
		return
	}

	// Add participant
	tournamentParticipants[uid] = &TournamentParticipant{
		uid:          uid,
		messageCount: 0,
		joinedAt:     time.Now().UTC(),
	}

	// Apply 2-3 random punishments
	allPunishments := []PunishmentType{
		PunishmentBackward, PunishmentStutterstep, PunishmentElongate,
		PunishmentUppercase, PunishmentLowercase, PunishmentRobotic,
		PunishmentAlternating, PunishmentUwu, PunishmentPirate,
		PunishmentConfused, PunishmentDrunk, PunishmentHiccup,
	}

	numPunishments := 2 + rand.Intn(2) // 2 or 3 punishments
	selectedPunishments := []PunishmentType{}
	
	// Randomly select unique punishments
	shuffled := make([]PunishmentType, len(allPunishments))
	copy(shuffled, allPunishments)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	for i := 0; i < numPunishments && i < len(shuffled); i++ {
		pType := shuffled[i]
		selectedPunishments = append(selectedPunishments, pType)
		client.AddPunishment(pType, 0, "Tournament Mode") // No expiration
	}

	// Build punishment list for message
	punishmentNames := []string{}
	for _, pType := range selectedPunishments {
		punishmentNames = append(punishmentNames, pType.String())
	}

	client.SendServerMessage(fmt.Sprintf("🏆 Joined tournament! You've been given: %s", strings.Join(punishmentNames, ", ")))
	writeToAllClients("CT", "OOC", fmt.Sprintf("🏆 UID %d joined the tournament!", uid))
	addToBuffer(client, "TOURNAMENT", "Joined tournament", false)
}
