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
	"flag"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

func cmdPos(client *Client, args []string, _ string) {
	if len(args) == 0 {
		client.SendServerMessage(fmt.Sprintf("Your current position is: %v\nAvailable positions: %v",
			client.Pos(), strings.Join(validPositions, ", ")))
		return
	}
	pos := strings.ToLower(args[0])
	for _, v := range validPositions {
		if pos == v {
			client.SetPos(pos)
			addToBuffer(client, "CMD", fmt.Sprintf("Changed position to %v.", pos), false)
			client.SendServerMessage(fmt.Sprintf("Position changed to: %v", pos))
			return
		}
	}
	client.SendServerMessage(fmt.Sprintf("Invalid position. Available positions: %v", strings.Join(validPositions, ", ")))
}

// Handles /pair

func cmdPair(client *Client, args []string, _ string) {
	if client.CharID() < 0 {
		client.SendServerMessage("You have not selected a character.")
		return
	}

	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}

	if target == client {
		client.SendServerMessage("You cannot pair with yourself.")
		return
	}

	if target.Area() != client.Area() {
		client.SendServerMessage("That player is not in your area.")
		return
	}

	if target.CharID() < 0 {
		client.SendServerMessage("That player has not selected a character.")
		return
	}

	client.SetPairWantedID(target.CharID())

	// Check if the target is already requesting to pair with us (mutual pairing).
	if target.PairWantedID() == client.CharID() {
		// Establish UID-tracked pair on both sides so it persists across area changes.
		client.SetForcePairUID(target.Uid())
		target.SetForcePairUID(client.Uid())
		client.SendServerMessage(fmt.Sprintf("Now pairing with %v.", target.OOCName()))
		target.SendServerMessage(fmt.Sprintf("%v accepted your pair request.", client.OOCName()))
	} else {
		client.SendServerMessage(fmt.Sprintf("Sent pair request to %v.", target.OOCName()))
		target.SendServerMessage(fmt.Sprintf("%v wants to pair with you. Type /pair %v to accept.", client.OOCName(), client.Uid()))
	}
}

// Handles /forcepair

func cmdForcePair(client *Client, args []string, _ string) {
	uid1, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	uid2, err := strconv.Atoi(args[1])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target1, err := getClientByUid(uid1)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", uid1))
		return
	}

	target2, err := getClientByUid(uid2)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", uid2))
		return
	}

	if target1 == target2 {
		client.SendServerMessage("Cannot force a player to pair with themselves.")
		return
	}

	if target1.CharID() < 0 {
		client.SendServerMessage(fmt.Sprintf("UID %v has not selected a character.", uid1))
		return
	}

	if target2.CharID() < 0 {
		client.SendServerMessage(fmt.Sprintf("UID %v has not selected a character.", uid2))
		return
	}

	target1.SetPairWantedID(target2.CharID())
	target2.SetPairWantedID(target1.CharID())
	target1.SetForcePairUID(target2.Uid())
	target2.SetForcePairUID(target1.Uid())

	target1.SendServerMessage(fmt.Sprintf("You have been force-paired with %v by %v.", target2.OOCName(), client.OOCName()))
	target2.SendServerMessage(fmt.Sprintf("You have been force-paired with %v by %v.", target1.OOCName(), client.OOCName()))
	client.SendServerMessage(fmt.Sprintf("Force-paired %v and %v.", target1.OOCName(), target2.OOCName()))
	addToBuffer(client, "CMD", fmt.Sprintf("Force-paired UID %v and UID %v.", uid1, uid2), false)
}

// Handles /unpair

func cmdUnpair(client *Client, _ []string, _ string) {
	if client.PairWantedID() == -1 && client.ForcePairUID() == -1 {
		client.SendServerMessage("You do not have an active pair request.")
		return
	}

	// If force-paired, clear force-pair state on both sides.
	if client.ForcePairUID() >= 0 {
		if partner, err := getClientByUid(client.ForcePairUID()); err == nil {
			partner.SetForcePairUID(-1)
			partner.SetPairWantedID(-1)
			partner.SendServerMessage(fmt.Sprintf("%v has cancelled the pair.", client.OOCName()))
		}
		client.SetForcePairUID(-1)
	}

	// Notify any client that was paired with us.
	for c := range clients.GetAllClients() {
		if c != client && c.PairWantedID() == client.CharID() {
			c.SendServerMessage(fmt.Sprintf("%v has cancelled the pair.", client.OOCName()))
		}
	}

	client.SetPairWantedID(-1)
	client.SendServerMessage("Pair cancelled.")
}

// Handles /forceunpair

func cmdForceUnpair(client *Client, args []string, _ string) {
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", uid))
		return
	}

	if target.PairWantedID() == -1 && target.ForcePairUID() == -1 {
		client.SendServerMessage("That player is not paired.")
		return
	}

	// Clear the partner's pair state.
	if target.ForcePairUID() >= 0 {
		if partner, err := getClientByUid(target.ForcePairUID()); err == nil {
			partner.SetForcePairUID(-1)
			partner.SetPairWantedID(-1)
			partner.SendServerMessage(fmt.Sprintf("You have been force-unpaired by %v.", client.OOCName()))
		}
		target.SetForcePairUID(-1)
	}

	// Notify any non-UID-tracked client that was paired with the target.
	for c := range clients.GetAllClients() {
		if c != target && c.PairWantedID() == target.CharID() {
			c.SetPairWantedID(-1)
			c.SendServerMessage(fmt.Sprintf("Your pair with %v was cancelled by %v.", target.OOCName(), client.OOCName()))
		}
	}

	target.SetPairWantedID(-1)
	target.SendServerMessage(fmt.Sprintf("You have been force-unpaired by %v.", client.OOCName()))
	client.SendServerMessage(fmt.Sprintf("Force-unpaired %v.", target.OOCName()))
	addToBuffer(client, "CMD", fmt.Sprintf("Force-unpaired UID %v.", uid), false)
}

// Handles /possess - one-time possession that mimics target's appearance for a single message

func cmdPossess(client *Client, args []string, _ string) {
	// Get the target UID
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	// Get the target client
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}

	// Validate CharID is within bounds
	if target.CharID() < 0 || target.CharID() >= len(characters) {
		client.SendServerMessage("Target has an invalid character.")
		return
	}

	// Get the message to send
	msg := strings.Join(args[1:], " ")
	if msg == "" {
		client.SendServerMessage("Message cannot be empty.")
		return
	}

	// Encode the message
	encodedMsg := encode(msg)

	// Get the target's current emote from their pair info, or use "normal" as fallback
	targetEmote := target.PairInfo().emote
	if targetEmote == "" {
		targetEmote = "normal"
	}

	// Get the target's displayed character name (handles iniswap)
	// Use PairInfo().name if available (contains iniswapped character), otherwise use their actual character
	targetCharName := target.PairInfo().name
	if targetCharName == "" {
		// Defensive bounds check before accessing characters array
		if target.CharID() >= 0 && target.CharID() < len(characters) {
			targetCharName = characters[target.CharID()]
		} else {
			client.SendServerMessage("Target has an invalid character.")
			return
		}
	}

	// Get the character ID for the displayed character
	targetCharID := getCharacterID(targetCharName)
	if targetCharID == -1 {
		// If character name is not found, fall back to target's actual character
		targetCharID = target.CharID()
		// Defensive bounds check before accessing characters array
		if targetCharID >= 0 && targetCharID < len(characters) {
			targetCharName = characters[targetCharID]
		} else {
			client.SendServerMessage("Target has an invalid character.")
			return
		}
	}

	// Create the IC message packet args following the MS packet format
	// This is a ONE-TIME possession that copies the target's appearance completely
	icArgs := make([]string, 30)
	icArgs[0] = "chat"                        // desk_mod
	icArgs[1] = ""                            // pre-anim
	icArgs[2] = targetCharName                // character name (target's displayed character, including iniswap)
	icArgs[3] = targetEmote                   // emote (target's emote)
	icArgs[4] = encodedMsg                    // message (encoded)
	icArgs[5] = target.Pos()                  // position (target's position to spoof them)
	icArgs[6] = ""                            // sfx-name
	icArgs[7] = "0"                           // emote_mod
	icArgs[8] = strconv.Itoa(targetCharID)    // char_id (ID of target's displayed character)
	icArgs[9] = "0"                           // sfx-delay
	icArgs[10] = "0"                          // objection_mod
	icArgs[11] = "0"                          // evidence
	icArgs[12] = "0"                          // flipping
	icArgs[13] = "0"                          // realization
	// Use target's last text color, default to "0" (white) if none set
	targetTextColor := target.LastTextColor()
	if targetTextColor == "" {
		targetTextColor = "0"
	}
	icArgs[14] = targetTextColor              // text color (target's color)
	// Use target's showname, falling back to displayed character name
	showname := target.Showname()
	if strings.TrimSpace(showname) == "" {
		showname = targetCharName
	}
	icArgs[15] = showname                     // showname (target's showname)
	icArgs[16] = "-1"                         // pair_id
	icArgs[17] = ""                           // pair_charid (server pairing)
	icArgs[18] = ""                           // pair_emote (server pairing)
	icArgs[19] = ""                           // offset
	icArgs[20] = ""                           // pair_offset (server pairing)
	icArgs[21] = ""                           // pair_flip (server pairing)
	icArgs[22] = "0"                          // non-interrupting pre
	icArgs[23] = "0"                          // sfx-looping
	icArgs[24] = "0"                          // screenshake
	icArgs[25] = ""                           // frames_shake
	icArgs[26] = ""                           // frames_realization
	icArgs[27] = ""                           // frames_sfx
	icArgs[28] = "0"                          // additive
	icArgs[29] = ""                           // blank (reserved)

	// Send the IC message to the target's area
	writeToArea(target.Area(), "MS", icArgs...)

	// Log the possession (use original message for readability in logs)
	addToBuffer(client, "CMD", fmt.Sprintf("Possessed UID %v to say: \"%v\"", uid, msg), true)

	// Notify the admin
	client.SendServerMessage(fmt.Sprintf("Possessed UID %v for one message.", uid))
}

// Handles /unpossess

func cmdUnpossess(client *Client, args []string, _ string) {
	if client.Possessing() == -1 {
		client.SendServerMessage("You are not possessing anyone.")
		return
	}

	// Clear the possession link
	client.SetPossessing(-1)

	// Clear the saved possessed position
	client.SetPossessedPos("")

	// Log the action
	addToBuffer(client, "CMD", "Stopped possessing.", true)

	// Notify the admin
	client.SendServerMessage("Stopped possessing.")
}

// Handles /fullpossess - makes all admin's IC messages appear from target

func cmdFullPossess(client *Client, args []string, _ string) {
	// Get the target UID
	uid, err := strconv.Atoi(args[0])
	if err != nil {
		client.SendServerMessage("Invalid UID.")
		return
	}

	// Get the target client
	target, err := getClientByUid(uid)
	if err != nil {
		client.SendServerMessage("Client does not exist.")
		return
	}

	// Validate CharID is within bounds
	if target.CharID() < 0 || target.CharID() >= len(characters) {
		client.SendServerMessage("Target has an invalid character.")
		return
	}

	// Establish the persistent possession link
	client.SetPossessing(target.Uid())

	// Save the target's current position to spoof it
	client.SetPossessedPos(target.Pos())

	// Log the action
	addToBuffer(client, "CMD", fmt.Sprintf("Started full possession of UID %v.", uid), true)

	// Notify the admin
	client.SendServerMessage(fmt.Sprintf("Now fully possessing UID %v. All YOUR IC messages will appear as them. Use /unpossess to stop.", uid))
}

// Handles /rmusr

func cmdRoll(client *Client, args []string, _ string) {
	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	private := flags.Bool("p", false, "")
	flags.Parse(args)
	b, _ := regexp.MatchString("([[:digit:]])d([[:digit:]])", flags.Arg(0))
	if !b {
		client.SendServerMessage("Argument not recognized.")
		return
	}
	s := strings.Split(flags.Arg(0), "d")
	num, _ := strconv.Atoi(s[0])
	sides, _ := strconv.Atoi(s[1])
	if num <= 0 || num > config.MaxDice || sides <= 0 || sides > config.MaxSide {
		client.SendServerMessage("Invalid num/side.")
		return
	}
	var result []string
	for i := 0; i < num; i++ {
		result = append(result, fmt.Sprint(rand.Intn(sides)+1))
	}
	if *private {
		client.SendServerMessage(fmt.Sprintf("Results: %v.", strings.Join(result, ", ")))
	} else {
		sendAreaServerMessage(client.Area(), fmt.Sprintf("%v rolled %v. Results: %v.", client.OOCName(), flags.Arg(0), strings.Join(result, ", ")))
	}
	addToBuffer(client, "CMD", fmt.Sprintf("Rolled %v.", flags.Arg(0)), false)
}

// Handles /setrole

func cmdRps(client *Client, args []string, _ string) {
	// Check cooldown (30 seconds)
	if time.Now().UTC().Before(client.LastRpsTime().Add(30 * time.Second)) && !client.LastRpsTime().IsZero() {
		remaining := time.Until(client.LastRpsTime().Add(30 * time.Second))
		client.SendServerMessage(fmt.Sprintf("Please wait %v seconds before playing RPS again.", int(remaining.Seconds())+1))
		return
	}

	choice := strings.ToLower(args[0])
	if choice != "rock" && choice != "paper" && choice != "scissors" {
		client.SendServerMessage("Invalid choice. Use: rock, paper, or scissors.")
		return
	}

	// Update last RPS time
	client.SetLastRpsTime(time.Now().UTC())

	// Generate random server choice
	choices := []string{"rock", "paper", "scissors"}
	serverChoice := choices[rand.Intn(3)]

	// Determine winner
	var result string
	if choice == serverChoice {
		result = "It's a tie!"
	} else if (choice == "rock" && serverChoice == "scissors") ||
		(choice == "paper" && serverChoice == "rock") ||
		(choice == "scissors" && serverChoice == "paper") {
		result = fmt.Sprintf("%v wins!", client.OOCName())
	} else {
		result = "Server wins!"
	}

	// Broadcast to area
	message := fmt.Sprintf("%v played %v, Server played %v. %v", client.OOCName(), choice, serverChoice, result)
	sendAreaServerMessage(client.Area(), message)
	addToBuffer(client, "GAME", fmt.Sprintf("Played RPS: %v vs %v - %v", choice, serverChoice, result), false)
}

// Handles /coinflip

func cmdCoinflip(client *Client, args []string, _ string) {
	choice := strings.ToLower(args[0])
	if choice != "heads" && choice != "tails" {
		client.SendServerMessage("Invalid choice. Use: heads or tails.")
		return
	}

	// Check if there's an active coinflip challenge in the area
	activeChallenge := client.Area().ActiveCoinflip()
	
	if activeChallenge == nil {
		// No active challenge - create a new one
		challenge := &area.CoinflipChallenge{
			PlayerName: client.OOCName(),
			Choice:     choice,
			CreatedAt:  time.Now().UTC(),
		}
		client.Area().SetActiveCoinflip(challenge)
		client.Area().SetLastCoinflipTime(time.Now().UTC())
		
		// Announce the challenge
		message := fmt.Sprintf("%v has chosen %v and is ready to coinflip! Type /coinflip %v to battle them!", 
			client.OOCName(), choice, oppositeChoice(choice))
		sendAreaServerMessage(client.Area(), message)
		addToBuffer(client, "GAME", fmt.Sprintf("Started coinflip challenge with %v", choice), false)
		
	} else {
		// There's an active challenge
		
		// Check if challenge has expired (30 seconds)
		if time.Now().UTC().After(activeChallenge.CreatedAt.Add(30 * time.Second)) {
			// Challenge expired, create new one
			challenge := &area.CoinflipChallenge{
				PlayerName: client.OOCName(),
				Choice:     choice,
				CreatedAt:  time.Now().UTC(),
			}
			client.Area().SetActiveCoinflip(challenge)
			client.Area().SetLastCoinflipTime(time.Now().UTC())
			
			message := fmt.Sprintf("Previous coinflip expired. %v has chosen %v and is ready to coinflip! Type /coinflip %v to battle them!", 
				client.OOCName(), choice, oppositeChoice(choice))
			sendAreaServerMessage(client.Area(), message)
			addToBuffer(client, "GAME", fmt.Sprintf("Started coinflip challenge with %v", choice), false)
			return
		}
		
		// Check if same player is trying to accept their own challenge
		if activeChallenge.PlayerName == client.OOCName() {
			client.SendServerMessage("You cannot accept your own coinflip challenge!")
			return
		}
		
		// Check if the choice is different from the challenger's choice
		if activeChallenge.Choice == choice {
			client.SendServerMessage(fmt.Sprintf("You must pick the opposite choice! The challenger picked %v, so you must pick %v.", 
				activeChallenge.Choice, oppositeChoice(activeChallenge.Choice)))
			return
		}
		
		// Battle time! Flip the coin
		coinResult := "heads"
		if rand.Intn(2) == 1 {
			coinResult = "tails"
		}
		
		// Determine winner
		var winner string
		if coinResult == activeChallenge.Choice {
			winner = activeChallenge.PlayerName
		} else {
			winner = client.OOCName()
		}
		
		// Announce result
		message := fmt.Sprintf("⚔️ COINFLIP BATTLE! %v (%v) vs %v (%v) - The coin landed on %v! 🎉 %v WINS! 🎉", 
			activeChallenge.PlayerName, activeChallenge.Choice,
			client.OOCName(), choice,
			coinResult, winner)
		sendAreaServerMessage(client.Area(), message)
		
		// Log for both players
		addToBuffer(client, "GAME", fmt.Sprintf("Coinflip battle: %v vs %v - Result: %v - Winner: %v", 
			activeChallenge.Choice, choice, coinResult, winner), false)
		
		// Clear the challenge
		client.Area().SetActiveCoinflip(nil)
	}
}

// oppositeChoice returns the opposite coinflip choice
func oppositeChoice(choice string) string {
	if choice == "heads" {
		return "tails"
	}
	return "heads"
}

// Handles /poll

func cmdPoll(client *Client, args []string, usage string) {
	// Check if there's already an active poll
	if client.Area().ActivePoll() != nil {
		client.SendServerMessage("There is already an active poll in this area.")
		return
	}

	// Check cooldown (5 minutes)
	if time.Now().UTC().Before(client.Area().LastPollTime().Add(5 * time.Minute)) && !client.Area().LastPollTime().IsZero() {
		remaining := time.Until(client.Area().LastPollTime().Add(5 * time.Minute))
		client.SendServerMessage(fmt.Sprintf("Please wait %v before creating another poll in this area.", remaining.Round(time.Second)))
		return
	}

	// Parse poll format: question|option1|option2|...
	fullArg := strings.Join(args, " ")
	parts := strings.Split(fullArg, "|")
	
	if len(parts) < 3 {
		client.SendServerMessage("Not enough poll options. Format: " + usage)
		return
	}

	question := strings.TrimSpace(parts[0])
	options := make([]string, 0)
	for i := 1; i < len(parts); i++ {
		opt := strings.TrimSpace(parts[i])
		if opt != "" {
			options = append(options, opt)
		}
	}

	if len(options) < 2 {
		client.SendServerMessage("Poll must have at least 2 options.")
		return
	}

	// Create poll
	poll := &area.Poll{
		ID:        time.Now().UnixNano(),
		Question:  question,
		Options:   options,
		CreatedAt: time.Now().UTC(),
		ClosesAt:  time.Now().UTC().Add(2 * time.Minute),
		CreatedBy: client.OOCName(),
	}

	client.Area().SetActivePoll(poll)
	client.Area().SetLastPollTime(time.Now().UTC())
	client.Area().SetPollVotes(make(map[int]int))
	client.Area().SetPlayerVotes(make(map[int]int))

	// Broadcast poll to area
	pollMsg := fmt.Sprintf("=== POLL ===\n%v\n", question)
	for i, opt := range options {
		pollMsg += fmt.Sprintf("%v. %v\n", i+1, opt)
	}
	pollMsg += fmt.Sprintf("\nUse /vote <number> to vote. Poll closes in 2 minutes.")
	sendAreaServerMessage(client.Area(), pollMsg)
	addToBuffer(client, "CMD", fmt.Sprintf("Created poll: %v", question), false)

	// Schedule auto-close after 2 minutes
	go func(a *area.Area, pollID int64) {
		time.Sleep(2 * time.Minute)
		currentPoll := a.ActivePoll()
		if currentPoll != nil && currentPoll.ID == pollID {
			// Close poll
			resultMsg := fmt.Sprintf("=== POLL CLOSED ===\n%v\nResults:\n", currentPoll.Question)
			votes := a.PollVotes()
			for i, opt := range currentPoll.Options {
				count := 0
				if votes != nil {
					count = votes[i+1]
				}
				resultMsg += fmt.Sprintf("%v. %v - %v votes\n", i+1, opt, count)
			}
			sendAreaServerMessage(a, resultMsg)
			a.ClearPoll()
		}
	}(client.Area(), poll.ID)
}

// Handles /vote

func cmdVote(client *Client, args []string, usage string) {
	// Check if there's an active poll
	poll := client.Area().ActivePoll()
	if poll == nil {
		client.SendServerMessage("There is no active poll in this area.")
		return
	}

	// Check if poll has expired
	if time.Now().UTC().After(poll.ClosesAt) {
		client.SendServerMessage("This poll has expired.")
		client.Area().ClearPoll()
		return
	}

	// Check if player has already voted
	if client.Area().HasPlayerVoted(client.Uid()) {
		client.SendServerMessage("You have already voted in this poll.")
		return
	}

	// Parse vote option
	option, err := strconv.Atoi(args[0])
	if err != nil || option < 1 || option > len(poll.Options) {
		client.SendServerMessage(fmt.Sprintf("Invalid option. Choose a number between 1 and %v.", len(poll.Options)))
		return
	}

	// Record vote
	client.Area().AddPlayerVote(client.Uid(), option)
	client.SendServerMessage(fmt.Sprintf("You voted for: %v", poll.Options[option-1]))

	// Broadcast updated results to area
	resultMsg := fmt.Sprintf("=== POLL UPDATE ===\n%v\nCurrent Results:\n", poll.Question)
	votes := client.Area().PollVotes()
	for i, opt := range poll.Options {
		count := 0
		if votes != nil {
			count = votes[i+1]
		}
		resultMsg += fmt.Sprintf("%v. %v - %v votes\n", i+1, opt, count)
	}
	sendAreaServerMessage(client.Area(), resultMsg)
	addToBuffer(client, "VOTE", fmt.Sprintf("Voted for option %v in poll", option), false)
}

// cmdPunishment is a generic handler for punishment commands

func cmdWhisper(client *Client, args []string, usage string) {
	cmdPunishment(client, args, usage, PunishmentWhisper)
}


