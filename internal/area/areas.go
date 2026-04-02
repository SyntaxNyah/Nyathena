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

package area

import (
	"strings"
	"sync"
	"time"
)

type EvidenceMode int
type Status int
type Lock int
type TRState int

const (
	EviMods EvidenceMode = iota
	EviAny
	EviCMs
)
const (
	StatusIdle Status = iota
	StatusPlayers
	StatusCasing
	StatusRecess
	StatusRP
	StatusGaming
)
const (
	LockFree Lock = iota
	LockSpectatable
	LockLocked
)

const (
	TRIdle TRState = iota
	TRRecording
	TRPlayback
	TRUpdating
	TRInserting
)

type TestimonyRecorder struct {
	Testimony []string
	Index     int
	State     TRState
}

type Poll struct {
	ID        int64
	Question  string
	Options   []string
	CreatedAt time.Time
	ClosesAt  time.Time
	CreatedBy string
}

type CoinflipChallenge struct {
	PlayerName string
	Choice     string
	CreatedAt  time.Time
}

type Area struct {
	data              AreaData
	defaults          defaults
	mu                sync.Mutex
	taken             []bool
	players           int
	visiblePlayers    int
	defhp             int
	prohp             int
	evidence          []string
	buffer            []string
	cms               map[int]struct{}
	last_msg          int
	evi_mode          EvidenceMode
	status            Status
	lock              Lock
	invited           map[int]struct{}
	doc               string
	description       string
	tr                TestimonyRecorder
	activePoll        *Poll
	lastPollTime      time.Time
	pollVotes         map[int]int
	playerVotes       map[int]int
	activeCoinflip    *CoinflipChallenge
	lastCoinflipTime  time.Time
	spectateMode      bool
	spectateInvited   map[int]struct{}
	casinoEnabled     bool
	casinoMinBet      int
	casinoMaxBet      int
	casinoMaxTables   int
	casinoJackpot     bool
	casinoJackpotPool int64
}

type AreaData struct {
	Name              string `toml:"name"`
	Description       string `toml:"description"`
	Evi_mode          string `toml:"evidence_mode"`
	Allow_iniswap     bool   `toml:"allow_iniswap"`
	Force_noint       bool   `toml:"force_nointerrupt"`
	Bg                string `toml:"background"`
	Allow_cms         bool   `toml:"allow_cms"`
	Force_bglist      bool   `toml:"force_bglist"`
	Lock_bg           bool   `toml:"lock_bg"`
	Lock_music        bool   `toml:"lock_music"`
	Casino_enabled    bool   `toml:"casino_enabled"`
	Casino_min_bet    int    `toml:"casino_min_bet"`
	Casino_max_bet    int    `toml:"casino_max_bet"`
	Casino_max_tables int    `toml:"casino_max_tables"`
	Casino_jackpot    bool   `toml:"casino_slots_jackpot"`
}

type defaults struct {
	evi_mode          EvidenceMode
	allow_iniswap     bool
	force_noint       bool
	bg                string
	description       string
	allow_cms         bool
	force_bglist      bool
	lock_bg           bool
	lock_music        bool
	casino_enabled    bool
	casino_min_bet    int
	casino_max_bet    int
	casino_max_tables int
	casino_jackpot    bool
}

// NewArea returns a new area.
func NewArea(data AreaData, charlen int, bufsize int, evi_mode EvidenceMode) *Area {
	return &Area{
		data: data,
		defaults: defaults{
			evi_mode:          evi_mode,
			allow_iniswap:     data.Allow_iniswap,
			force_noint:       data.Force_noint,
			bg:                data.Bg,
			description:       data.Description,
			allow_cms:         data.Allow_cms,
			force_bglist:      data.Force_bglist,
			lock_bg:           data.Lock_bg,
			lock_music:        data.Lock_music,
			casino_enabled:    data.Casino_enabled,
			casino_min_bet:    data.Casino_min_bet,
			casino_max_bet:    data.Casino_max_bet,
			casino_max_tables: data.Casino_max_tables,
			casino_jackpot:    data.Casino_jackpot,
		},
		taken:           make([]bool, charlen),
		defhp:           10,
		prohp:           10,
		buffer:          make([]string, bufsize),
		last_msg:        -1,
		evi_mode:        evi_mode,
		description:     data.Description,
		cms:             make(map[int]struct{}),
		invited:         make(map[int]struct{}),
		spectateInvited: make(map[int]struct{}),
		casinoEnabled:   data.Casino_enabled,
		casinoMinBet:    data.Casino_min_bet,
		casinoMaxBet:    data.Casino_max_bet,
		casinoMaxTables: data.Casino_max_tables,
		casinoJackpot:   data.Casino_jackpot,
	}
}

// Name returns the area's name.
func (a *Area) Name() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Name
}

// Taken returns the area's taken list, where "-1" is taken and "0" is free
func (a *Area) Taken() []string {
	a.mu.Lock()
	takenList := make([]string, len(a.taken))
	for i, t := range a.taken {
		if t {
			takenList[i] = "-1"
		} else {
			takenList[i] = "0"
		}
	}
	a.mu.Unlock()
	return takenList
}

// AddChar adds a new player to the area.
func (a *Area) AddChar(char int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if char != -1 {
		if a.taken[char] {
			return false
		} else {
			a.taken[char] = true
		}
	}
	a.players++
	return true
}

// SwitchChar switches a player's character.
func (a *Area) SwitchChar(old int, new int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if new == -1 {
		if old != -1 {
			a.taken[old] = false
		}
		return true
	} else {
		if a.taken[new] {
			return false
		} else {
			a.taken[new] = true
			if old != -1 {
				a.taken[old] = false
			}
		}
		return true
	}
}

// RemoveChar removes a player from the area.
func (a *Area) RemoveChar(char int) {
	a.mu.Lock()
	if char != -1 {
		a.taken[char] = false
	}
	a.players--
	a.mu.Unlock()
}

// HP returns the values of the area's def and pro HP bars.
func (a *Area) HP() (int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.defhp, a.prohp
}

// SetHP sets either the def or pro HP to the specified value.
// The bar must be 1 for the defense HP, 2 for pro HP.
// The value must be between 0 and 10.
func (a *Area) SetHP(bar int, v int) bool {
	if v > 10 || v < 0 {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch bar {
	case 1:
		a.defhp = v
	case 2:
		a.prohp = v
	default:
		return false
	}
	return true
}

// PlayerCount returns the number of players in the area.
func (a *Area) PlayerCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.players
}

// VisiblePlayerCount returns the number of non-hidden players in the area.
func (a *Area) VisiblePlayerCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.visiblePlayers
}

// AddVisiblePlayer increments the visible player count by one.
func (a *Area) AddVisiblePlayer() {
	a.mu.Lock()
	a.visiblePlayers++
	a.mu.Unlock()
}

// RemoveVisiblePlayer decrements the visible player count by one.
func (a *Area) RemoveVisiblePlayer() {
	a.mu.Lock()
	a.visiblePlayers--
	a.mu.Unlock()
}

// Evidence returns a list of evidence in the area.
func (a *Area) Evidence() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.evidence
}

// AddEvidence adds a piece of evidence to the area.
func (a *Area) AddEvidence(evi string) {
	a.mu.Lock()
	a.evidence = append(a.evidence, evi)
	a.mu.Unlock()
}

// RemoveEvidence removes a piece of evidence to the area.
func (a *Area) RemoveEvidence(id int) {
	a.mu.Lock()
	if len(a.evidence) >= id {
		copy(a.evidence[id:], a.evidence[id+1:])
		a.evidence = a.evidence[:len(a.evidence)-1]
	}
	a.mu.Unlock()
}

// EditEvidence replaces a piece of evidence.
func (a *Area) EditEvidence(id int, evi string) {
	a.mu.Lock()
	if len(a.evidence) >= id {
		a.evidence[id] = evi
	}
	a.mu.Unlock()
}

// SwapEvidence swaps the indexes of two pieces of evidence.
func (a *Area) SwapEvidence(x int, y int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.evidence) < x+1 || len(a.evidence) < y+1 {
		return false
	}
	a.evidence[x], a.evidence[y] = a.evidence[y], a.evidence[x]
	return true
}

// UpdateBuffer adds a new line to the area's log buffer.
func (a *Area) UpdateBuffer(s string) {
	a.mu.Lock()
	a.buffer = append(a.buffer[1:], s)
	a.mu.Unlock()
}

// Buffer returns the area's log buffer.
func (a *Area) Buffer() []string {
	var returnList []string
	a.mu.Lock()
	for _, s := range a.buffer {
		if strings.TrimSpace(s) != "" {
			returnList = append(returnList, s)
		}
	}
	a.mu.Unlock()
	return returnList
}

// CMs returns a slice of UID values for all CMs in the area.
func (a *Area) CMs() []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]int, 0, len(a.cms))
	for uid := range a.cms {
		result = append(result, uid)
	}
	return result
}

// AddCM adds a new CM to the area.
func (a *Area) AddCM(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.cms[uid]; exists {
		return false
	}
	a.cms[uid] = struct{}{}
	return true
}

// RemoveCM removes a CM from the area.
func (a *Area) RemoveCM(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.cms[uid]; !exists {
		return false
	}
	delete(a.cms, uid)
	return true
}

// HasCM returns whether the given uid is a CM in the area.
func (a *Area) HasCM(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, exists := a.cms[uid]
	return exists
}

// EvidenceMode returns the area's evidence mode.
func (a *Area) EvidenceMode() EvidenceMode {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.evi_mode
}

// SetEvidenceMode sets the area's evidence mode.
func (a *Area) SetEvidenceMode(mode EvidenceMode) {
	a.mu.Lock()
	a.evi_mode = mode
	a.mu.Unlock()
}

// IniswapAllowed returns whether iniswapping is allowed in the area.
func (a *Area) IniswapAllowed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Allow_iniswap
}

// SetIniswapAllowed sets iniswapping in the area.
func (a *Area) SetIniswapAllowed(b bool) {
	a.mu.Lock()
	a.data.Allow_iniswap = b
	a.mu.Unlock()
}

// NoInterrupt returns whether preanims must not interrupt in the area.
func (a *Area) NoInterrupt() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Force_noint
}

// SetNoInterrupt sets non-interrupting preanims in the area.
func (a *Area) SetNoInterrupt(b bool) {
	a.mu.Lock()
	a.data.Force_noint = b
	a.mu.Unlock()
}

// LastSpeaker returns the character of the the last speaker.
func (a *Area) LastSpeaker() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last_msg
}

// SetLastSpeaker sets the area's last speaker.
func (a *Area) SetLastSpeaker(char int) {
	a.mu.Lock()
	a.last_msg = char
	a.mu.Unlock()
}

// Background returns the area's current background.
func (a *Area) Background() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Bg
}

// SetBackground sets the area's background.
func (a *Area) SetBackground(bg string) {
	a.mu.Lock()
	a.data.Bg = bg
	a.mu.Unlock()
}

// IsTaken returns whether the given character is taken in the area.
func (a *Area) IsTaken(char int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if char != -1 {
		return a.taken[char]
	} else {
		return false
	}
}

// CMsAllowed returns whether CMs are allowed in the area.
func (a *Area) CMsAllowed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Allow_cms
}

// SetCMsAllowed sets allowing CMs in the area.
func (a *Area) SetCMsAllowed(b bool) {
	a.mu.Lock()
	a.data.Allow_cms = b
	a.mu.Unlock()
}

// Status returns the area's current status.
func (a *Area) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// SetStatus sets the area's status.
func (a *Area) SetStatus(status Status) {
	a.mu.Lock()
	a.status = status
	a.mu.Unlock()
}

// Lock returns the area's lock type.
func (a *Area) Lock() Lock {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lock
}

// SetLock sets the area's lock.
func (a *Area) SetLock(lock Lock) {
	a.mu.Lock()
	a.lock = lock
	a.mu.Unlock()
}

// AddInvited adds a new UID to the area's invite list.
func (a *Area) AddInvited(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.invited[uid]; exists {
		return false
	}
	a.invited[uid] = struct{}{}
	return true
}

// RemoveInvited removes a UID from the area's invite list.
func (a *Area) RemoveInvited(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.invited[uid]; !exists {
		return false
	}
	delete(a.invited, uid)
	return true
}

// ClearInvited clears the area's invite list.
func (a *Area) ClearInvited() {
	a.mu.Lock()
	a.invited = make(map[int]struct{})
	a.mu.Unlock()
}

// Invited returns the area's invite list as a slice of UIDs.
func (a *Area) Invited() []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]int, 0, len(a.invited))
	for uid := range a.invited {
		result = append(result, uid)
	}
	return result
}

// HasInvited returns whether the given UID is on the area's invite list.
func (a *Area) HasInvited(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, exists := a.invited[uid]
	return exists
}

// Reset returns all area settings to their default values.
func (a *Area) Reset() {
	a.mu.Lock()
	a.evidence = []string{}
	a.invited = make(map[int]struct{})
	a.status = StatusIdle
	a.lock = LockFree
	a.cms = make(map[int]struct{})
	a.last_msg = -1
	a.defhp = 10
	a.prohp = 10
	a.evi_mode = a.defaults.evi_mode
	a.data.Allow_cms = a.defaults.allow_cms
	a.data.Allow_iniswap = a.defaults.allow_iniswap
	a.data.Force_noint = a.defaults.force_noint
	a.data.Bg = a.defaults.bg
	a.data.Force_bglist = a.defaults.force_bglist
	a.data.Lock_bg = a.defaults.lock_bg
	a.data.Lock_music = a.defaults.lock_music
	a.casinoEnabled = a.defaults.casino_enabled
	a.casinoMinBet = a.defaults.casino_min_bet
	a.casinoMaxBet = a.defaults.casino_max_bet
	a.casinoMaxTables = a.defaults.casino_max_tables
	a.casinoJackpot = a.defaults.casino_jackpot
	a.tr.Index = 0
	a.tr.State = TRIdle
	a.tr.Testimony = []string{}
	a.activePoll = nil
	a.pollVotes = nil
	a.playerVotes = nil
	a.spectateMode = false
	a.spectateInvited = make(map[int]struct{})
	a.mu.Unlock()
}

// SpectateMode returns whether spectate mode is enabled in the area.
func (a *Area) SpectateMode() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.spectateMode
}

// SetSpectateMode sets spectate mode in the area.
func (a *Area) SetSpectateMode(b bool) {
	a.mu.Lock()
	a.spectateMode = b
	if !b {
		a.spectateInvited = make(map[int]struct{})
	}
	a.mu.Unlock()
}

// AddSpectateInvited adds a UID to the spectate IC invite list.
func (a *Area) AddSpectateInvited(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.spectateInvited[uid]; exists {
		return false
	}
	a.spectateInvited[uid] = struct{}{}
	return true
}

// RemoveSpectateInvited removes a UID from the spectate IC invite list.
func (a *Area) RemoveSpectateInvited(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.spectateInvited[uid]; !exists {
		return false
	}
	delete(a.spectateInvited, uid)
	return true
}

// HasSpectateInvited returns whether the given UID is in the spectate IC invite list.
func (a *Area) HasSpectateInvited(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, exists := a.spectateInvited[uid]
	return exists
}

// ForceBGList returns whether the server BG list is enforced in the area.
func (a *Area) ForceBGList() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Force_bglist
}

// SetForceBGList sets enforciing the server BG list in the area.
func (a *Area) SetForceBGList(b bool) {
	a.mu.Lock()
	a.data.Force_bglist = b
	a.mu.Unlock()
}

// LockBG returns whether the BG is locked in the area.
func (a *Area) LockBG() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Lock_bg
}

// SetLockBG sets locking the area's BG.
func (a *Area) SetLockBG(b bool) {
	a.mu.Lock()
	a.data.Lock_bg = b
	a.mu.Unlock()
}

// LockMusic returns whether music in the area is CM-only.
func (a *Area) LockMusic() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Lock_music
}

// SetLockMusic sets CM-only music in the area.
func (a *Area) SetLockMusic(b bool) {
	a.mu.Lock()
	a.data.Lock_music = b
	a.mu.Unlock()
}

// Doc returns the area's doc.
func (a *Area) Doc() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.doc
}

// SetDoc sets the area's doc.
func (a *Area) SetDoc(s string) {
	a.mu.Lock()
	a.doc = s
	a.mu.Unlock()
}

// Description returns the area's description shown to players on entry.
func (a *Area) Description() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.description
}

// SetDescription sets the area's entry description.
func (a *Area) SetDescription(s string) {
	a.mu.Lock()
	a.description = s
	a.mu.Unlock()
}

// HasTestimony returns whether the area has a recorded testimony.
func (a *Area) HasTestimony() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.tr.Testimony) > 2
}

// Testimony returns the area's recorded testimony.
func (a *Area) Testimony() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var rl []string
	for i, s := range a.tr.Testimony {
		if i == 0 {
			continue
		}
		rl = append(rl, strings.Split(s, "#")[4])
	}
	return rl
}

// String returns the string representation of the status.
func (status Status) String() string {
	switch status {
	case StatusIdle:
		return "IDLE"
	case StatusPlayers:
		return "LOOKING-FOR-PLAYERS"
	case StatusCasing:
		return "CASING"
	case StatusRecess:
		return "RECESS"
	case StatusRP:
		return "RP"
	case StatusGaming:
		return "GAMING"
	}
	return ""
}

// String returns the string representation of the lock.
func (lock Lock) String() string {
	switch lock {
	case LockFree:
		return "FREE"
	case LockSpectatable:
		return "SPECTATABLE"
	case LockLocked:
		return "LOCKED"
	}
	return ""
}

// String returns the string representation of the evimod.
func (evimod EvidenceMode) String() string {
	switch evimod {
	case EviAny:
		return "any"
	case EviCMs:
		return "cms"
	case EviMods:
		return "mods"
	}
	return ""
}

// String returns the string representation of the testimony recorder state.
func (tr TRState) String() string {
	switch tr {
	case TRIdle:
		return "idle"
	case TRRecording:
		return "recording"
	case TRPlayback:
		return "playback"
	case TRUpdating:
		return "updating"
	case TRInserting:
		return "inserting"
	}
	return ""
}

// ActivePoll returns the area's active poll.
func (a *Area) ActivePoll() *Poll {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activePoll
}

// SetActivePoll sets the area's active poll.
func (a *Area) SetActivePoll(p *Poll) {
	a.mu.Lock()
	a.activePoll = p
	a.mu.Unlock()
}

// LastPollTime returns the time of the last poll in the area.
func (a *Area) LastPollTime() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastPollTime
}

// SetLastPollTime sets the time of the last poll in the area.
func (a *Area) SetLastPollTime(t time.Time) {
	a.mu.Lock()
	a.lastPollTime = t
	a.mu.Unlock()
}

// PollVotes returns the poll votes map.
func (a *Area) PollVotes() map[int]int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pollVotes
}

// SetPollVotes sets the poll votes map.
func (a *Area) SetPollVotes(votes map[int]int) {
	a.mu.Lock()
	a.pollVotes = votes
	a.mu.Unlock()
}

// PlayerVotes returns the player votes map.
func (a *Area) PlayerVotes() map[int]int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.playerVotes
}

// SetPlayerVotes sets the player votes map.
func (a *Area) SetPlayerVotes(votes map[int]int) {
	a.mu.Lock()
	a.playerVotes = votes
	a.mu.Unlock()
}

// ActiveCoinflip returns the area's active coinflip challenge.
func (a *Area) ActiveCoinflip() *CoinflipChallenge {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeCoinflip
}

// SetActiveCoinflip sets the area's active coinflip challenge.
func (a *Area) SetActiveCoinflip(c *CoinflipChallenge) {
	a.mu.Lock()
	a.activeCoinflip = c
	a.mu.Unlock()
}

// LastCoinflipTime returns the time of the last coinflip in the area.
func (a *Area) LastCoinflipTime() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastCoinflipTime
}

// SetLastCoinflipTime sets the time of the last coinflip in the area.
func (a *Area) SetLastCoinflipTime(t time.Time) {
	a.mu.Lock()
	a.lastCoinflipTime = t
	a.mu.Unlock()
}

// AddPlayerVote adds a player's vote to the poll.
func (a *Area) AddPlayerVote(uid int, option int) {
	a.mu.Lock()
	if a.playerVotes == nil {
		a.playerVotes = make(map[int]int)
	}
	if a.pollVotes == nil {
		a.pollVotes = make(map[int]int)
	}
	a.playerVotes[uid] = option
	a.pollVotes[option]++
	a.mu.Unlock()
}

// HasPlayerVoted checks if a player has already voted.
func (a *Area) HasPlayerVoted(uid int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playerVotes == nil {
		return false
	}
	_, voted := a.playerVotes[uid]
	return voted
}

// ClearPoll clears the active poll and related data.
func (a *Area) ClearPoll() {
	a.mu.Lock()
	a.activePoll = nil
	a.pollVotes = nil
	a.playerVotes = nil
	a.mu.Unlock()
}

// CasinoEnabled returns whether the casino is enabled for this area.
func (a *Area) CasinoEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.casinoEnabled
}

// SetCasinoEnabled enables or disables the casino for this area.
func (a *Area) SetCasinoEnabled(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoEnabled = v
}

// CasinoMinBet returns the minimum bet for this area's casino.
func (a *Area) CasinoMinBet() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.casinoMinBet
}

// SetCasinoMinBet sets the minimum bet for this area's casino.
func (a *Area) SetCasinoMinBet(v int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoMinBet = v
}

// CasinoMaxBet returns the maximum bet for this area's casino.
func (a *Area) CasinoMaxBet() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.casinoMaxBet
}

// SetCasinoMaxBet sets the maximum bet for this area's casino.
func (a *Area) SetCasinoMaxBet(v int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoMaxBet = v
}

// CasinoMaxTables returns the maximum active tables allowed in this area.
func (a *Area) CasinoMaxTables() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.casinoMaxTables
}

// SetCasinoMaxTables sets the maximum active tables for this area's casino.
func (a *Area) SetCasinoMaxTables(v int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoMaxTables = v
}

// CasinoJackpot returns whether the slots jackpot is enabled for this area.
func (a *Area) CasinoJackpot() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.casinoJackpot
}

// SetCasinoJackpot enables or disables the slots jackpot for this area.
func (a *Area) SetCasinoJackpot(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoJackpot = v
}

// CasinoJackpotPool returns the current jackpot pool size for this area's slots.
func (a *Area) CasinoJackpotPool() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.casinoJackpotPool
}

// AddCasinoJackpotPool adds amount to the jackpot pool.
func (a *Area) AddCasinoJackpotPool(amount int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoJackpotPool += amount
}

// ResetCasinoJackpotPool resets the jackpot pool to zero.
func (a *Area) ResetCasinoJackpotPool() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.casinoJackpotPool = 0
}
