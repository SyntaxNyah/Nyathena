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

// /leash: a one-way claim layered on top of the normal UID-tracked pair.
//
// A Leash is shared by exactly two clients — a master and a dog. Both hold a
// pointer to the same Leash, so neither side can be left with a stale UID
// after the other disconnects, changes character, or is claimed by someone
// else. It is deliberately separate from PairGroup: a dog is not a group
// member, and a group cannot override the leash's pair injection.
//
// The dog follows the master through area changes, including punishment-safe
// areas. `/leash` also bypasses the punishment-safe check when it applies the
// /dog effect, because the leash is a claim rather than a one-shot punishment.

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

const (
	leashAbandonPopup = "Dogs don't abandon their master"
	leashClaimPopup   = "Your partner has been leashed"
)

// Leash is the master/dog relationship created by /leash.
type Leash struct {
	mu     sync.Mutex
	master *Client
	dog    *Client
}

// Master returns the client holding the leash, or nil if it was dissolved.
func (l *Leash) Master() *Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.master
}

// Dog returns the leashed client, or nil if it was dissolved.
func (l *Leash) Dog() *Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dog
}

// Members returns both ends of the leash. Either value may be nil after a
// dissolve. The pointers are copied so callers never hold the leash lock
// while sending messages or taking a client lock.
func (l *Leash) Members() (master, dog *Client) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.master, l.dog
}

// Other returns the client on the other end of the leash from c, or nil.
func (l *Leash) Other(c *Client) *Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch c {
	case l.master:
		return l.dog
	case l.dog:
		return l.master
	}
	return nil
}

// Dissolve clears both clients' leash pointers. It is idempotent: calling it
// after a dissolve is a no-op, so disconnect cleanup and command handlers can
// both call it safely.
func (l *Leash) Dissolve() {
	if l == nil {
		return
	}
	l.mu.Lock()
	master, dog := l.master, l.dog
	l.master, l.dog = nil, nil
	l.mu.Unlock()

	if master != nil {
		master.SetLeash(nil)
	}
	if dog != nil {
		dog.SetLeash(nil)
	}
}

// dissolveLeash clears a leash and the pair state it rode on. Use this when
// breaking a claim; use Leash.Dissolve directly only when the caller is about
// to replace both pair states itself.
func dissolveLeash(l *Leash) {
	if l == nil {
		return
	}
	master, dog := l.Members()
	l.Dissolve()
	if master != nil {
		master.SetForcePairUID(-1)
		master.SetPairWantedID(-1)
	}
	if dog != nil {
		dog.SetForcePairUID(-1)
		dog.SetPairWantedID(-1)
	}
}

// removeLeashForUnpunish breaks a leash because /unpunish is clearing the
// target's full punishment set. The leash itself is not a PunishmentType, so
// /unpunish -t <type> deliberately does not call this — removing /dog must not
// silently release the claim. Both ends are told, including a partner outside
// the issuer's area. Returns true when a leash was actually broken.
func removeLeashForUnpunish(target, issuer *Client) bool {
	if target == nil {
		return false
	}
	l := target.Leash()
	if l == nil {
		return false
	}
	master, dog := l.Members()
	dissolveLeash(l)
	msg := fmt.Sprintf("Your leash was removed by %v.", oocDisplayName(issuer))
	if master != nil {
		master.SendServerMessage(msg)
	}
	if dog != nil {
		dog.SendServerMessage(msg)
	}
	return true
}

// dragLeashedDog moves a master's dog into the area the master just joined.
// Called at the tail of Client.ChangeArea, after the master has fully joined,
// so the dog's own area change runs every normal join side effect (roster,
// music, background, player count). Punishment-safe areas are not consulted:
// a leash follows its master through shelters.
func (client *Client) dragLeashedDog() {
	l := client.Leash()
	if l == nil || l.Master() != client {
		return
	}
	dog := l.Dog()
	target := client.Area()
	if dog == nil || dog == client || target == nil || dog.Area() == target {
		return
	}
	if !dog.ChangeArea(target) {
		client.SendServerMessage(fmt.Sprintf("%v could not follow you into %v.", oocDisplayName(dog), target.Name()))
	}
}

// leashBlocksAreaChange reports whether client is a leashed dog trying to move
// somewhere other than its master's current area. It sends the refusal popup
// when it blocks, so player-initiated movement paths can call it and return.
// Server-forced moves (kickarea, vote kicks, /move <uid>, Discord) deliberately
// do not use this: moderation must still be able to move a leashed player.
func (client *Client) leashBlocksAreaChange(dest *area.Area) bool {
	l := client.Leash()
	if l == nil || l.Dog() != client || dest == nil {
		return false
	}
	master := l.Master()
	if master == nil || master.Area() == dest {
		return false
	}
	client.SendPopup(leashAbandonPopup)
	return true
}

// refuseIfLeashed blocks pairing while a client is part of a leash. A dog gets
// the abandonment popup; a master gets a plain instruction to release first.
func (client *Client) refuseIfLeashed() bool {
	l := client.Leash()
	if l == nil {
		return false
	}
	if l.Dog() == client {
		client.SendPopup(leashAbandonPopup)
		return true
	}
	client.SendServerMessage("You are holding a leash — /unpair it before pairing with someone else.")
	return true
}

// leaveGroupForLeash removes a client from any multi-pair group before a leash
// is created. A group would otherwise keep injecting its roster into the same
// IC pair slots the leash is about to own.
func leaveGroupForLeash(c *Client, reason string) {
	if g := c.PairGroup(); g != nil {
		g.remove(c, reason)
	}
}

// releaseLeashForClaim dissolves any leash a target was already in and notifies
// the previous partner with the claim popup. newPartner is the other half of
// the new leash; it is skipped because it is about to be claimed itself.
func releaseLeashForClaim(c, newPartner *Client, notified map[*Client]bool) {
	l := c.Leash()
	if l == nil {
		return
	}
	other := l.Other(c)
	dissolveLeash(l)
	if other == nil || other == newPartner || notified[other] {
		return
	}
	other.SendPopup(leashClaimPopup)
	notified[other] = true
}

// claimPairLinks clears every pairing reference to target on other clients,
// including normal (CharID-only) pair requests, and pops the claim notice at
// each previous partner. spare is the other half of the new leash and is
// skipped. target's own pair fields are cleared at the end.
func claimPairLinks(target, spare *Client, notified map[*Client]bool) {
	targetUID := target.Uid()
	targetCharID := target.CharID()

	clients.ForEach(func(c *Client) {
		if c == target || c == spare {
			return
		}
		referenced := false
		if c.ForcePairUID() == targetUID {
			c.SetForcePairUID(-1)
			referenced = true
		}
		if targetCharID >= 0 && c.PairWantedID() == targetCharID {
			c.SetPairWantedID(-1)
			referenced = true
		}
		if !referenced {
			return
		}
		// If this peer was in a leash with the target, drop that stale claim
		// too. A leash with somebody else is none of this claim's business.
		if l := c.Leash(); l != nil {
			master, dog := l.Members()
			if master == target || dog == target {
				l.Dissolve()
			}
		}
		if !notified[c] {
			c.SendPopup(leashClaimPopup)
			notified[c] = true
		}
	})
	target.SetForcePairUID(-1)
	target.SetPairWantedID(-1)
}

// clearLeashOnDisconnect dissolves a departing client's leash and tells the
// survivor why. The dog leaving earns the master the abandonment popup; the
// master leaving frees the dog with a release popup.
func clearLeashOnDisconnect(client *Client) {
	l := client.Leash()
	if l == nil {
		return
	}
	master, dog := l.Members()
	l.Dissolve()
	if master != nil {
		master.SetForcePairUID(-1)
		master.SetPairWantedID(-1)
	}
	if dog != nil {
		dog.SetForcePairUID(-1)
		dog.SetPairWantedID(-1)
	}

	switch {
	case dog == client && master != nil:
		master.SendPopup(leashAbandonPopup)
	case master == client && dog != nil:
		dog.SendPopup("Your master disconnected — the leash has been released")
	}
}

// dissolveLeashForForce breaks a leash because a moderator is force-pairing or
// force-grouping one of its members. This is an override, not a claim, so the
// other end gets a plain release notice rather than the "claimed" popup.
func dissolveLeashForForce(c *Client, newPartner *Client) {
	l := c.Leash()
	if l == nil {
		return
	}
	other := l.Other(c)
	dissolveLeash(l)
	if other != nil && other != newPartner {
		other.SendServerMessage(fmt.Sprintf("Your leash with %v was broken by a moderator.", oocDisplayName(c)))
	}
}

// Handles /leash — a moderator forces two players into a pair and turns the
// second into the first's dog.
func cmdLeash(client *Client, args []string, usage string) {
	if punishmentsSystemDisabled(client) {
		return
	}

	// Extract -h first, matching cmdPunishment: flag.Parse stops at the first
	// positional, so a trailing -h after the UIDs must not be lost.
	args, hidden := extractHiddenFlag(args)

	flags := flag.NewFlagSet("", 0)
	flags.SetOutput(io.Discard)
	reason := flags.String("r", "", "")
	durationStr := flags.String("d", "10m", "")
	flags.Parse(args)

	if len(flags.Args()) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	if len(flags.Args()) > 2 {
		client.SendServerMessage("Too many arguments:\n" + usage)
		return
	}

	masterUID, err := strconv.Atoi(flags.Arg(0))
	if err != nil {
		client.SendServerMessage("Invalid master UID.")
		return
	}
	dogUID, err := strconv.Atoi(flags.Arg(1))
	if err != nil {
		client.SendServerMessage("Invalid dog UID.")
		return
	}
	if masterUID == dogUID {
		client.SendServerMessage("A player cannot be their own master.")
		return
	}

	master, err := getClientByUid(masterUID)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", masterUID))
		return
	}
	dog, err := getClientByUid(dogUID)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", dogUID))
		return
	}
	if master == dog {
		client.SendServerMessage("A player cannot be their own master.")
		return
	}
	if master.CharID() < 0 {
		client.SendServerMessage(fmt.Sprintf("UID %v has not selected a character.", masterUID))
		return
	}
	if dog.CharID() < 0 {
		client.SendServerMessage(fmt.Sprintf("UID %v has not selected a character.", dogUID))
		return
	}

	// Apply the dog effect first, through the shared applicator with the
	// punishment-safe check bypassed. If the global kill switch is armed (or
	// the duration is invalid), nothing is punished and we must not create a
	// half-formed leash.
	dogArgs := []string{"-d", *durationStr, "-r", *reason, strconv.Itoa(dog.Uid())}
	if hidden {
		dogArgs = append(dogArgs, "-h")
	}
	if punished := cmdPunishmentIgnoringSafeArea(client, dogArgs, usage, PunishmentDog); len(punished) == 0 {
		return
	}

	// Claim both ends: break any existing leash, pair, or multi-pair group.
	notified := make(map[*Client]bool)
	releaseLeashForClaim(master, dog, notified)
	releaseLeashForClaim(dog, master, notified)
	leaveGroupForLeash(master, oocDisplayName(master)+" has been leashed — the pairing group has been updated.")
	leaveGroupForLeash(dog, oocDisplayName(dog)+" has been leashed — the pairing group has been updated.")
	claimPairLinks(master, dog, notified)
	claimPairLinks(dog, master, notified)

	master.SetPairWantedID(dog.CharID())
	dog.SetPairWantedID(master.CharID())
	master.SetForcePairUID(dog.Uid())
	dog.SetForcePairUID(master.Uid())

	l := &Leash{master: master, dog: dog}
	master.SetLeash(l)
	dog.SetLeash(l)

	if !hidden {
		master.SendServerMessage(fmt.Sprintf("You have leashed %v.", oocDisplayName(dog)))
		dog.SendServerMessage(fmt.Sprintf("You have been leashed by %v.", oocDisplayName(master)))
	}
	client.SendServerMessage(fmt.Sprintf("Leashed %v to %v.", oocDisplayName(dog), oocDisplayName(master)))
	addToBuffer(client, "CMD", fmt.Sprintf("Leashed UID %v to UID %v.", dog.Uid(), master.Uid()), false)
}
