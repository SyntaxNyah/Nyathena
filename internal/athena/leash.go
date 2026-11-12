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
// A Leash is shared by one master and up to maxLeashDogs dogs. Every member
// holds a pointer to the same Leash, so no side can be left with a stale UID
// after a disconnect, character change, or claim by someone else. The
// master/first-dog pair fields stay UID-tracked for movement and moderation,
// while a leash-owned PairGroup mirrors the full pack into the multi-pair
// renderer so JSON clients draw every dog alongside the master.
//
// The PairGroup is only a render mirror: the Leash is the authority. A dog
// cannot escape by /leavegroup, /deny, /pair, /unpair, or /unforcegrouppair;
// full /unpunish and moderator overrides release the claim.
//
// Every dog follows the master through area changes, including punishment-safe
// areas. `/leash` also bypasses the punishment-safe check when it applies the
// /dog effect, because the leash is a claim rather than a one-shot punishment.
//
// A dog protected by an `/antipunishshield ... backfire` cannot be leashed: the
// /dog effect lands on the issuing moderator instead, so the claim follows the
// punishment and reverses. The shielded player holds the leash and the moderator
// becomes the dog.

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/MangosArentLiterature/Athena/internal/area"
)

const (
	leashAbandonPopup = "Dogs don't abandon their master"
	leashClaimPopup   = "Your partner has been leashed"
	maxLeashDogs      = 6
)

// Leash is the master/dogs relationship created by /leash.
type Leash struct {
	mu     sync.Mutex
	master *Client
	dogs   []*Client
	group  *PairGroup
}

// Master returns the client holding the leash, or nil if it was dissolved.
func (l *Leash) Master() *Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.master
}

// Dog returns the first leashed client, or nil if it was dissolved. It is kept
// for the single-dog pairing paths and tests; use Dogs for the full pack.
func (l *Leash) Dog() *Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.dogs) == 0 {
		return nil
	}
	return l.dogs[0]
}

// Dogs returns a copy of the leashed clients, in leash order (first dog is the
// one the master's legacy pair slot points at).
func (l *Leash) Dogs() []*Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]*Client(nil), l.dogs...)
}

// Members returns the master and the first dog. Either value may be nil after
// a dissolve. The pointers are copied so callers never hold the leash lock
// while sending messages or taking a client lock.
func (l *Leash) Members() (master, dog *Client) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.dogs) > 0 {
		dog = l.dogs[0]
	}
	return l.master, dog
}

// AllMembers returns the master followed by every dog, or nil after a
// dissolve. The slice is a copy; callers may message or mutate clients freely.
func (l *Leash) AllMembers() []*Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.master == nil {
		return nil
	}
	members := make([]*Client, 0, len(l.dogs)+1)
	members = append(members, l.master)
	members = append(members, l.dogs...)
	return members
}

// IsDog reports whether c is one of the leashed dogs.
func (l *Leash) IsDog(c *Client) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, dog := range l.dogs {
		if dog == c {
			return true
		}
	}
	return false
}

// Contains reports whether c is the master or one of the dogs.
func (l *Leash) Contains(c *Client) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.master == c {
		return true
	}
	for _, dog := range l.dogs {
		if dog == c {
			return true
		}
	}
	return false
}

// Other returns the client on the other end of the leash from c, or nil. For a
// master with several dogs it returns the first dog; multi-dog callers should
// use Dogs or AllMembers.
func (l *Leash) Other(c *Client) *Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	if c == l.master {
		if len(l.dogs) > 0 {
			return l.dogs[0]
		}
		return nil
	}
	for _, dog := range l.dogs {
		if dog == c {
			return l.master
		}
	}
	return nil
}

// Dissolve clears every client's leash pointer and tears down the render
// group. It is idempotent: calling it after a dissolve is a no-op, so
// disconnect cleanup and command handlers can both call it safely.
func (l *Leash) Dissolve() {
	if l == nil {
		return
	}
	l.mu.Lock()
	master := l.master
	dogs := append([]*Client(nil), l.dogs...)
	group := l.group
	l.master, l.dogs, l.group = nil, nil, nil
	l.mu.Unlock()

	if group != nil {
		group.dissolve("")
	}
	if master != nil {
		master.SetLeash(nil)
	}
	for _, dog := range dogs {
		dog.SetLeash(nil)
	}
}

// addDogs appends dogs, installs the shared pointer, refreshes the legacy pair
// fields, and rebuilds the multi-pair render group.
func (l *Leash) addDogs(dogs []*Client) {
	if l == nil || len(dogs) == 0 {
		return
	}
	l.mu.Lock()
	l.dogs = append(l.dogs, dogs...)
	l.mu.Unlock()
	for _, dog := range dogs {
		dog.SetLeash(l)
	}
	l.syncPairFields()
	l.rebuildGroup()
}

// removeDog detaches one dog. If it was the last dog the whole leash is
// dissolved; otherwise the group and the master's first-dog pair slot are
// refreshed. The removed dog's pair fields are left to the caller, which
// usually wants to install a different pair immediately.
func (l *Leash) removeDog(dog *Client) (master *Client, remaining []*Client, dissolved bool) {
	if l == nil {
		return nil, nil, false
	}
	l.mu.Lock()
	idx := -1
	for i, d := range l.dogs {
		if d == dog {
			idx = i
			break
		}
	}
	if idx < 0 {
		l.mu.Unlock()
		return nil, nil, false
	}
	l.dogs = append(l.dogs[:idx], l.dogs[idx+1:]...)
	master = l.master
	remaining = append([]*Client(nil), l.dogs...)
	if len(remaining) == 0 {
		group := l.group
		l.master, l.dogs, l.group = nil, nil, nil
		l.mu.Unlock()
		dog.SetLeash(nil)
		if master != nil {
			master.SetLeash(nil)
			master.SetForcePairUID(-1)
			master.SetPairWantedID(-1)
		}
		if group != nil {
			group.dissolve("")
		}
		return master, nil, true
	}
	l.mu.Unlock()

	dog.SetLeash(nil)
	l.syncPairFields()
	l.rebuildGroup()
	return master, remaining, false
}

// syncPairFields makes the master point at the first dog and every dog point
// at the master. The extra dogs ride in the PairGroup's additional_chars list;
// the standard pair slots can only name one partner.
func (l *Leash) syncPairFields() {
	master := l.Master()
	dogs := l.Dogs()
	if master == nil || len(dogs) == 0 {
		return
	}
	master.SetForcePairUID(dogs[0].Uid())
	master.SetPairWantedID(dogs[0].CharID())
	for _, dog := range dogs {
		dog.SetForcePairUID(master.Uid())
		dog.SetPairWantedID(master.CharID())
	}
}

// rebuildGroup replaces the render mirror with an accepted, leash-owned group
// containing the master and every current dog. The master is first so the
// standard pair slots anchor on the pack's owner.
func (l *Leash) rebuildGroup() {
	l.mu.Lock()
	master := l.master
	dogs := append([]*Client(nil), l.dogs...)
	old := l.group
	l.group = nil
	l.mu.Unlock()

	if old != nil {
		old.dissolve("")
	}
	if master == nil || len(dogs) == 0 {
		return
	}

	members := make([]*Client, 0, len(dogs)+1)
	members = append(members, master)
	members = append(members, dogs...)
	accepted := make(map[int]bool, len(members))
	for _, m := range members {
		accepted[m.Uid()] = true
	}
	group := &PairGroup{
		members:  members,
		accepted: accepted,
		groupID:  fmt.Sprintf("leash-%d", master.Uid()),
		forced:   true,
		leash:    true,
	}

	l.mu.Lock()
	// A concurrent dissolve/removal won the race; leave the group detached.
	if l.master != master || len(l.dogs) != len(dogs) {
		l.mu.Unlock()
		return
	}
	l.group = group
	l.mu.Unlock()

	for _, m := range members {
		m.SetPairGroup(group)
	}
	sendGroupState(group)
}

// dissolveLeash clears a leash and the pair state it rode on. Use this when
// breaking a claim; use Leash.Dissolve directly only when the caller is about
// to replace the pair states itself.
func dissolveLeash(l *Leash) {
	if l == nil {
		return
	}
	members := l.AllMembers()
	l.Dissolve()
	for _, m := range members {
		m.SetForcePairUID(-1)
		m.SetPairWantedID(-1)
	}
}

// removeLeashForUnpunish breaks a leash because /unpunish is clearing the
// target's full punishment set. The leash itself is not a PunishmentType, so
// /unpunish -t <type> deliberately does not call this — removing /dog must not
// silently release the claim. Unpunishing one dog detaches just that dog;
// unpunishing the master releases the whole pack. Every affected member is
// told, including partners outside the issuer's area. Returns true when a
// leash was actually broken.
func removeLeashForUnpunish(target, issuer *Client) bool {
	if target == nil {
		return false
	}
	l := target.Leash()
	if l == nil {
		return false
	}
	msg := fmt.Sprintf("Your leash was removed by %v.", oocDisplayName(issuer))
	master := l.Master()
	if master == target {
		members := l.AllMembers()
		dissolveLeash(l)
		for _, m := range members {
			m.SendServerMessage(msg)
		}
		return true
	}

	master, remaining, dissolved := l.removeDog(target)
	if master == nil && !dissolved {
		return false
	}
	target.SetForcePairUID(-1)
	target.SetPairWantedID(-1)
	target.SendServerMessage(msg)
	if master != nil {
		master.SendServerMessage(fmt.Sprintf("%v's leash was removed by %v.",
			oocDisplayName(target), oocDisplayName(issuer)))
	}
	for _, dog := range remaining {
		dog.SendServerMessage(fmt.Sprintf("%v was released from the leash by %v.",
			oocDisplayName(target), oocDisplayName(issuer)))
	}
	return true
}

// dragLeashedDogs moves every dog into the area the master just joined. Called
// at the tail of Client.ChangeArea, after the master has fully joined, so each
// dog's own area change runs every normal join side effect (roster, music,
// background, player count). Punishment-safe areas are not consulted: a leash
// follows its master through shelters.
func (client *Client) dragLeashedDogs() {
	l := client.Leash()
	if l == nil || l.Master() != client {
		return
	}
	dogs := l.Dogs()
	target := client.Area()
	if len(dogs) == 0 || target == nil {
		return
	}
	for _, dog := range dogs {
		if dog == nil || dog == client || dog.Area() == target {
			continue
		}
		if !dog.ChangeArea(target) {
			client.SendServerMessage(fmt.Sprintf("%v could not follow you into %v.",
				oocDisplayName(dog), target.Name()))
		}
	}
}

// leashBlocksAreaChange reports whether client is a leashed dog trying to move
// somewhere other than its master's current area. It sends the refusal popup
// when it blocks, so player-initiated movement paths can call it and return.
// Server-forced moves (kickarea, vote kicks, /move <uid>, Discord) deliberately
// do not use this: moderation must still be able to move a leashed player.
func (client *Client) leashBlocksAreaChange(dest *area.Area) bool {
	l := client.Leash()
	if l == nil || !l.IsDog(client) || dest == nil {
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
	if l.IsDog(client) {
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

// releaseLeashForClaim detaches a target from any leash it was already in and
// notifies the previous pack with the claim popup. newPartner is the other half
// of the new leash; it is skipped because it is about to be claimed itself.
// Detaching a dog leaves the old master's remaining pack intact; detaching a
// master dissolves the whole old pack.
func releaseLeashForClaim(c, newPartner *Client, notified map[*Client]bool) {
	l := c.Leash()
	if l == nil {
		return
	}
	if l.Master() == c {
		members := l.AllMembers()
		dissolveLeash(l)
		for _, m := range members {
			if m == c || m == newPartner || notified[m] {
				continue
			}
			m.SendPopup(leashClaimPopup)
			notified[m] = true
		}
		return
	}

	master, remaining, dissolved := l.removeDog(c)
	if master == nil && !dissolved {
		return
	}
	c.SetForcePairUID(-1)
	c.SetPairWantedID(-1)
	affected := append([]*Client{master}, remaining...)
	for _, m := range affected {
		if m == nil || m == newPartner || notified[m] {
			continue
		}
		m.SendPopup(leashClaimPopup)
		notified[m] = true
	}
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
		if l := c.Leash(); l != nil && l.Contains(target) {
			if l.Master() == target {
				dissolveLeash(l)
			} else {
				l.removeDog(target)
			}
			target.SetForcePairUID(-1)
			target.SetPairWantedID(-1)
		}
		if !notified[c] {
			c.SendPopup(leashClaimPopup)
			notified[c] = true
		}
	})
	target.SetForcePairUID(-1)
	target.SetPairWantedID(-1)
}

// clearLeashOnDisconnect detaches a departing dog or dissolves the whole pack
// when the master leaves. A departing dog earns the master the abandonment
// popup; the master leaving frees every dog with a release popup.
func clearLeashOnDisconnect(client *Client) {
	l := client.Leash()
	if l == nil {
		return
	}
	if l.Master() == client {
		dogs := l.Dogs()
		l.Dissolve()
		client.SetForcePairUID(-1)
		client.SetPairWantedID(-1)
		for _, dog := range dogs {
			dog.SetForcePairUID(-1)
			dog.SetPairWantedID(-1)
			dog.SendPopup("Your master disconnected — the leash has been released")
		}
		return
	}

	master, remaining, dissolved := l.removeDog(client)
	client.SetForcePairUID(-1)
	client.SetPairWantedID(-1)
	if master != nil {
		master.SendPopup(leashAbandonPopup)
		if dissolved {
			master.SetForcePairUID(-1)
			master.SetPairWantedID(-1)
		}
	}
	for _, dog := range remaining {
		dog.SendServerMessage(fmt.Sprintf("%v disconnected — the leash has been updated.",
			oocDisplayName(client)))
	}
}

// dissolveLeashForForce breaks a leash because a moderator is force-pairing or
// force-grouping one of its members. This is an override, not a claim, so the
// rest of the pack gets a plain release notice rather than the "claimed" popup.
func dissolveLeashForForce(c *Client, newPartner *Client) {
	l := c.Leash()
	if l == nil {
		return
	}
	if l.Master() == c {
		members := l.AllMembers()
		dissolveLeash(l)
		for _, m := range members {
			if m == c || m == newPartner {
				continue
			}
			m.SendServerMessage(fmt.Sprintf("Your leash with %v was broken by a moderator.",
				oocDisplayName(c)))
		}
		return
	}

	master, remaining, dissolved := l.removeDog(c)
	if master == nil && !dissolved {
		return
	}
	c.SetForcePairUID(-1)
	c.SetPairWantedID(-1)
	if master != nil && master != newPartner {
		master.SendServerMessage(fmt.Sprintf("Your leash with %v was broken by a moderator.",
			oocDisplayName(c)))
	}
	for _, dog := range remaining {
		if dog == newPartner {
			continue
		}
		dog.SendServerMessage(fmt.Sprintf("%v was released from the leash by a moderator.",
			oocDisplayName(c)))
	}
}

// backfireShieldedDog returns the first requested dog whose /antipunishshield is
// in backfire mode, or nil. A backfire shield never lets the /dog effect land on
// the shielded player — it lands on the issuer — so the claim has to follow the
// punishment or the moderator would stay master of a dog that was never leashed.
func backfireShieldedDog(dogs []*Client) *Client {
	for _, dog := range dogs {
		if dog.AntiPunishShieldMode() == antiPunishShieldBackfire {
			return dog
		}
	}
	return nil
}

// Handles /leash — a moderator forces one master and up to maxLeashDogs dogs
// into a persistent pack. Every dog gets /dog, follows the master, and renders
// through the multi-pair group. Re-issuing the command with the same master
// adds more dogs until the cap is reached.
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

	positional := flags.Args()
	if len(positional) < 2 {
		client.SendServerMessage("Not enough arguments:\n" + usage)
		return
	}
	if len(positional) > 1+maxLeashDogs {
		client.SendServerMessage(fmt.Sprintf(
			"Too many arguments (a leash can hold at most %d dogs):\n%s", maxLeashDogs, usage))
		return
	}

	masterUID, err := strconv.Atoi(positional[0])
	if err != nil {
		client.SendServerMessage("Invalid master UID.")
		return
	}
	master, err := getClientByUid(masterUID)
	if err != nil {
		client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", masterUID))
		return
	}
	if master.CharID() < 0 {
		client.SendServerMessage(fmt.Sprintf("UID %v has not selected a character.", masterUID))
		return
	}

	requested := make([]*Client, 0, len(positional)-1)
	seen := make(map[int]bool, len(positional)-1)
	for _, arg := range positional[1:] {
		dogUID, err := strconv.Atoi(arg)
		if err != nil {
			client.SendServerMessage("Invalid dog UID.")
			return
		}
		if dogUID == masterUID {
			client.SendServerMessage("A player cannot be their own master.")
			return
		}
		if seen[dogUID] {
			client.SendServerMessage(fmt.Sprintf("Duplicate dog UID %v.", dogUID))
			return
		}
		seen[dogUID] = true
		dog, err := getClientByUid(dogUID)
		if err != nil {
			client.SendServerMessage(fmt.Sprintf("Client with UID %v does not exist.", dogUID))
			return
		}
		if dog == master {
			client.SendServerMessage("A player cannot be their own master.")
			return
		}
		if dog.CharID() < 0 {
			client.SendServerMessage(fmt.Sprintf("UID %v has not selected a character.", dogUID))
			return
		}
		requested = append(requested, dog)
	}

	// If the master is already a dog in somebody else's leash, this command
	// promotes them. The old claim is released below, after the punishment has
	// succeeded, so a failed command never breaks the old pack.
	existing := master.Leash()
	if existing != nil && existing.Master() != master {
		existing = nil
	}
	existingDogs := []*Client(nil)
	if existing != nil {
		existingDogs = existing.Dogs()
	}

	newDogs := make([]*Client, 0, len(requested))
	already := make([]string, 0)
	for _, dog := range requested {
		if existing != nil && existing.IsDog(dog) {
			already = append(already, oocDisplayName(dog))
			continue
		}
		newDogs = append(newDogs, dog)
	}
	if len(already) > 0 {
		client.SendServerMessage(fmt.Sprintf("Already on this leash: %v.", strings.Join(already, ", ")))
	}
	if len(newDogs) == 0 {
		client.SendServerMessage("No new dogs to add.")
		return
	}
	if len(existingDogs)+len(newDogs) > maxLeashDogs {
		client.SendServerMessage(fmt.Sprintf("A leash can hold at most %d dogs.", maxLeashDogs))
		return
	}

	// A backfire shield turns the claim around: the /dog effect already lands on
	// the issuer in that mode, so the leash follows the punishment and the
	// shielded player ends up holding it. A leash has a single master, so only a
	// one-on-one claim the issuer holds themselves can flip — bouncing a mixed
	// pack would strand the other dogs without the effect that makes them dogs.
	// Decided before any punishment is applied, so a refused command never
	// leaves a half-built pack with /dog already on it.
	bounce := backfireShieldedDog(newDogs)
	if bounce != nil {
		switch {
		case client != master:
			client.SendServerMessage(fmt.Sprintf(
				"Leash not created: %v's anti-punishment shield only bounces a leash you hold yourself.",
				oocDisplayName(bounce)))
			return
		case len(newDogs) > 1:
			client.SendServerMessage(fmt.Sprintf(
				"Leash not created: %v's anti-punishment shield only reverses a one-on-one leash.",
				oocDisplayName(bounce)))
			return
		}
		// Re-issuing the same command after a bounce would detach the moderator
		// from the leash the swap below is about to rebuild, so refuse it the
		// same way the normal path refuses a dog that is already on the leash.
		if l := bounce.Leash(); l != nil && l.Master() == bounce && l.IsDog(client) {
			client.SendServerMessage(fmt.Sprintf("You are already on %v's leash.", oocDisplayName(bounce)))
			return
		}
		// The backfire has to land before the claim can follow it, so a broken
		// -d aborts here exactly like it does for every other punishment
		// command instead of flipping the leash without its /dog effect.
		if _, ok := parsePunishmentDuration(client, *durationStr, true); !ok {
			return
		}
	}

	// Apply the dog effect first, through the shared applicator with the
	// punishment-safe check bypassed. One shared call keeps the whole pack
	// all-or-nothing: if the global kill switch is armed, the duration is
	// invalid, or an admin's anti-punishment shield spoofs any target, no
	// half-formed leash is created.
	dogUIDs := make([]string, len(newDogs))
	for i, dog := range newDogs {
		dogUIDs[i] = strconv.Itoa(dog.Uid())
	}
	dogArgs := []string{"-d", *durationStr, "-r", *reason, strings.Join(dogUIDs, ",")}
	if hidden {
		dogArgs = append(dogArgs, "-h")
	}
	punished := cmdPunishmentIgnoringSafeArea(client, dogArgs, usage, PunishmentDog)
	if bounce != nil {
		// The shielded dog never took the effect: the gate recorded the spoof
		// and the backfire put /dog on the moderator for real, silently. Swap
		// the ends before the pair links are built; everything below then
		// creates a normal leash with the shielded player as the master.
		master, newDogs = bounce, []*Client{client}
		existingDogs = nil
		if l := master.Leash(); l != nil && l.Master() == master {
			existingDogs = l.Dogs()
		}
	} else if len(punished) == 0 {
		return
	} else if len(punished) != len(newDogs) {
		client.SendServerMessage("Leash not created: at least one dog was shielded from the /dog punishment.")
		return
	}

	// Claim every member: break any existing leash, pair, or multi-pair group.
	// New members are pre-marked so the claim popups reach only the players
	// whose old partner was taken, not the pack being formed.
	notified := make(map[*Client]bool)
	notified[master] = true
	for _, dog := range newDogs {
		notified[dog] = true
	}

	if master.Leash() != nil && master.Leash().Master() != master {
		releaseLeashForClaim(master, nil, notified)
	}

	leash := master.Leash()
	if leash == nil {
		leaveGroupForLeash(master, oocDisplayName(master)+" has been leashed — the pairing group has been updated.")
		leash = &Leash{master: master}
		master.SetLeash(leash)
		claimPairLinks(master, newDogs[0], notified)
	}

	for _, dog := range newDogs {
		if dog.Leash() != nil {
			releaseLeashForClaim(dog, master, notified)
		}
		leaveGroupForLeash(dog, oocDisplayName(dog)+" has been leashed — the pairing group has been updated.")
		claimPairLinks(dog, master, notified)
	}
	leash.addDogs(newDogs)

	names := make([]string, len(newDogs))
	for i, dog := range newDogs {
		names[i] = oocDisplayName(dog)
	}
	joined := strings.Join(names, ", ")
	if !hidden {
		if len(existingDogs) == 0 {
			master.SendServerMessage(fmt.Sprintf("You have leashed %v.", joined))
		} else {
			master.SendServerMessage(fmt.Sprintf("You added %v to your leash.", joined))
		}
		for _, dog := range newDogs {
			dog.SendServerMessage(fmt.Sprintf("You have been leashed by %v.", oocDisplayName(master)))
		}
		for _, dog := range existingDogs {
			dog.SendServerMessage(fmt.Sprintf("Your master added %v to the leash.", joined))
		}
	}
	if bounce != nil {
		client.SendServerMessage(fmt.Sprintf(
			"🦴 %v's anti-punishment shield bounced the leash — you are the dog now.",
			oocDisplayName(master)))
		addToBuffer(client, "CMD", fmt.Sprintf(
			"Leash UID %v -> UID %v backfired; UID %v holds the leash.",
			client.Uid(), master.Uid(), master.Uid()), true)
	} else if len(existingDogs) == 0 {
		client.SendServerMessage(fmt.Sprintf("Leashed %v to %v.", joined, oocDisplayName(master)))
		addToBuffer(client, "CMD", fmt.Sprintf("Leashed UID %v to UID %v.",
			strings.Join(dogUIDs, ","), master.Uid()), false)
	} else {
		client.SendServerMessage(fmt.Sprintf("Added %v to %v's leash.", joined, oocDisplayName(master)))
		addToBuffer(client, "CMD", fmt.Sprintf("Added UID %v to UID %v's leash.",
			strings.Join(dogUIDs, ","), master.Uid()), false)
	}
}
