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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/webhook"
)

// packetBufPool is a pool of reusable byte buffers used by SendPacket to build
// outgoing AO2 packets without allocating a new buffer on every call.
// Each buffer is reset before use and returned to the pool after the write.
var packetBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

type MuteState int

const (
	Unmuted MuteState = iota
	ICMuted
	OOCMuted
	ICOOCMuted
	MusicMuted
	JudMuted
	ParrotMuted
)

type PunishmentType int

const (
	PunishmentNone PunishmentType = iota
	// Text Modification (13 types)
	PunishmentWhisper
	PunishmentBackward
	PunishmentStutterstep
	PunishmentElongate
	PunishmentUppercase
	PunishmentLowercase
	PunishmentRobotic
	PunishmentAlternating
	PunishmentFancy
	PunishmentUwu
	PunishmentPirate
	PunishmentShakespearean
	PunishmentCaveman
	// Visibility/Cosmetic (2 types)
	PunishmentEmoji
	PunishmentInvisible
	// Timing Effects (4 types)
	PunishmentSlowpoke
	PunishmentFastspammer
	PunishmentPause
	PunishmentLag
	// Social Chaos (3 types)
	PunishmentSubtitles
	PunishmentRoulette
	PunishmentSpotlight
	// Text Processing (7 types)
	PunishmentCensor
	PunishmentConfused
	PunishmentParanoid
	PunishmentDrunk
	PunishmentHiccup
	PunishmentWhistle
	PunishmentMumble
	// Complex Effects (3 types)
	PunishmentSpaghetti
	PunishmentTorment
	PunishmentRng
	PunishmentEssay
	// Advanced (2 types)
	PunishmentHaiku
	PunishmentAutospell
	// Animal Punishments (12 types)
	PunishmentMonkey
	PunishmentSnake
	PunishmentDog
	PunishmentCat
	PunishmentBird
	PunishmentCow
	PunishmentFrog
	PunishmentDuck
	PunishmentHorse
	PunishmentLion
	PunishmentZoo
	PunishmentBunny
	// Dere-type Punishments (10 types)
	PunishmentTsundere
	PunishmentYandere
	PunishmentKuudere
	PunishmentDandere
	PunishmentDeredere
	PunishmentHimedere
	PunishmentKamidere
	PunishmentUndere
	PunishmentBakadere
	PunishmentMayadere
	// Text Emoticon Punishment
	PunishmentEmoticon
	// Social Torment Punishments
	PunishmentLovebomb
	PunishmentDegrade
	// Chaos/Outburst Punishments
	PunishmentTourettes
	// Internet Slang Punishment
	PunishmentSlang
	// New Fun Punishment Commands
	PunishmentThesaurusOverload
	PunishmentValleyGirl
	PunishmentBabytalk
	PunishmentThirdPerson
	PunishmentUnreliableNarrator
	PunishmentUncannyValley
	// 51 Messages Punishment
	Punishment51
	// Philosophical / Literary Punishments
	PunishmentPhilosopher
	PunishmentPoet
	PunishmentUpsidedown
	PunishmentSarcasm
	PunishmentAcademic
	PunishmentRecipe
	// Quote Punishment
	PunishmentQuote
)

type PunishmentState struct {
	punishmentType PunishmentType
	expiresAt      time.Time
	reason         string
	lastMsgTime    time.Time
	msgDelay       time.Duration
	msgCount       int
	lastEffect     int
	targetUID      int // For PunishmentLovebomb: UID of the lovebomb target (-1 = random area target)
}

type ClientPairInfo struct {
	name      string
	emote     string
	flip      string
	offset    string
	wanted_id int
}

// emergencyBypassWindow is how long a moderator has to confirm an emergency
// entry into a locked area by attempting to join it again after receiving the
// warning message.
const emergencyBypassWindow = 30 * time.Second

type Client struct {
	pair                ClientPairInfo
	mu                  sync.Mutex
	conn                net.Conn
	joining             bool
	hdid                string
	uid                 int
	area                *area.Area
	char                int
	charIDStr           string // cached strconv.Itoa(char); updated on every SetCharID call
	ipid                string
	oocName             string
	lastmsg             string
	lastTextColor       string
	perms               uint64
	authenticated       bool
	mod_name            string
	pos                 string
	case_prefs          [5]bool
	muted               MuteState
	muteuntil           time.Time
	showname            string
	narrator            bool
	jailedUntil         time.Time
	lastRpsTime         time.Time
	punishments         []PunishmentState
	msgTimestamps       []time.Time  // Tracks message timestamps for rate limiting
	oocMsgTimestamps    []time.Time  // Tracks OOC message timestamps for OOC rate limiting
	rawPktCount         int          // Packet count in the current raw-rate-limit window
	rawPktWindowStart   time.Time    // Start time of the current raw-rate-limit window
	lastModcallTime     time.Time    // Tracks last modcall time for cooldown
	lastBarDrinkTime    time.Time    // Tracks last /bar buy time for cooldown
	lastRandomCharTime  time.Time    // Tracks last /randomchar time for cooldown
	lastRandomBgTime    time.Time    // Tracks last /randombg time for cooldown
	lastRandomSongTime  time.Time    // Tracks last /randomsong time for cooldown
	forcePairUID        int          // UID of the client this client is force-paired with (-1 if none)
	possessing          int          // UID of the client being possessed (-1 if not possessing anyone)
	possessedPos        string       // Position of the possessed target (saved at time of possession)
	forcedShowname      string       // Showname forced by a moderator ("" if none)
	forcedIniswapChar   string       // Character name forced for iniswap-style IC output ("" = none)
	forcedIniswapIDStr  string       // Pre-computed strconv.Itoa(charID) matching forcedIniswapChar ("" = none)
	connectedAt         time.Time    // Time the client joined the server (uid assigned); zero if not yet joined
	jailAreaID          int          // Area index where this client is jailed; -1 = no specific jail area
	emergencyBypassArea *area.Area   // Locked area the client most recently tried to enter as a mod; nil = no pending bypass
	emergencyBypassAt   time.Time    // Time of the first locked-area attempt; used with emergencyBypassArea to confirm an emergency override
	hidden              bool         // Whether the client is hidden from the player list and area counts
	charStuckUntil      time.Time    // Time when the character-stuck restriction expires; zero = not stuck
	charStuckCharID     int          // Character ID the client is locked to; -1 = not stuck
	dancing             bool         // Whether the client has dance mode active (flips sprite every message)
	danceFlipped        bool         // Current flip state for dance mode; toggles each IC message
	gambleHide          bool         // Whether the client has opted out of seeing gambling broadcast messages
	pendingRegUser      string       // Username from a pending /register that is awaiting captcha confirmation
	pendingRegPass      []byte       // bcrypt hash from a pending /register that is awaiting captcha confirmation
	pendingRegCaptcha   string       // Expected captcha token for the pending registration
	sessionChipsAwarded int64        // Chips already awarded mid-session (hourly ticker); subtracted at disconnect to avoid double-counting
	ignoredIPIDs        sync.Map     // Set of IPIDs permanently ignored by this client. Key: IPID string, Value: struct{}. Lock-free reads.
	lastPingNano        atomic.Int64 // Unix nanosecond timestamp of the last CH packet; 0 until seeded on join.
}

// NewClient returns a new client.
func NewClient(conn net.Conn, ipid string) *Client {
	return &Client{
		conn:            conn,
		uid:             -1,
		char:            -1,
		charIDStr:       "-1",
		pair:            ClientPairInfo{wanted_id: -1},
		ipid:            ipid,
		forcePairUID:    -1,
		possessing:      -1,
		jailAreaID:      -1,
		charStuckCharID: -1,
	}
}

// handleClient handles a client connection to the server.
func (client *Client) HandleClient() {
	defer client.clientCleanup()

	if client.CheckBanned(db.IPID) {
		return
	}

	// If this IPID has been tormented by automod, schedule a random disconnect.
	if isIPIDTormented(client.Ipid()) {
		go startTormentDisconnect(client)
	}

	// Load this client's persisted ignore list.
	if ignoredIPs, err := db.LoadIgnoredIPIDs(client.Ipid()); err != nil {
		logger.LogErrorf("Failed to load ignore list for %v: %v", client.Ipid(), err)
	} else {
		for _, ipid := range ignoredIPs {
			client.ignoredIPIDs.Store(ipid, struct{}{})
		}
	}

	if config.MCLimit != 0 && clients.CountByIPID(client.Ipid()) >= config.MCLimit {
		client.SendPacket("BD", "Too many connections from your IP. Please disconnect your other clients. If you have no other clients open, wait 1-2 minutes and try again.")
		client.conn.Close()
		return
	}

	clients.AddClient(client)

	go timeout(client)

	client.SendPacket("decryptor", "NOENCRYPT") // Relic of FantaCrypt. AO2 requires a server to send this to proceed with the handshake.
	input := bufio.NewScanner(client.conn)

	splitfn := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '%'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
	input.Split(splitfn) // Split input when a packet delimiter ('%') is found

	for input.Scan() {
		rawPacket := strings.TrimSpace(input.Text())
		if logger.EnableNetworkLogging {
			logger.WriteNetworkLog(client.ipid, client.Hdid(), "RECV", rawPacket)
		}
		// Raw packet rate limit: ban bots/flooders that send far more packets per second
		// than any legitimate client ever would. The ban is committed synchronously before
		// the connection closes so the flooder cannot immediately reconnect.
		if client.CheckRawPacketRateLimit() {
			client.SendServerMessage("You have been banned for packet flooding.")
			logger.LogInfof("Client (IPID:%v UID:%v) banned for raw packet flooding", client.Ipid(), client.Uid())
			logger.WriteAudit(fmt.Sprintf("%v | PACKET_FLOOD | IPID:%v | UID:%v | Auto-banned for packet flooding", time.Now().UTC().Format("15:04:05"), client.Ipid(), client.Uid()))
			autoBanPacketFlooder(client.Ipid())
			if enableDiscord {
				ipid, uid := client.Ipid(), client.Uid()
				go func() {
					if err := webhook.PostPacketFlood(ipid, uid); err != nil {
						logger.LogErrorf("while posting packet flood webhook: %v", err)
					}
				}()
			}
			client.conn.Close()
			return
		}
		packet, err := packet.NewPacket(rawPacket)
		if err != nil {
			continue // Discard invalid packets
		}
		v := PacketMap[packet.Header] // Check if this is a known packet.
		if v.Func != nil && len(packet.Body) >= v.Args {
			if v.MustJoin && client.Uid() == -1 {
				continue
			}
			v.Func(client, packet)
		}
	}
}

// write sends the given message to the client's network socket.
// Write errors are intentionally ignored: any underlying connection failure
// will surface on the next read in HandleClient, which closes the connection.
func (client *Client) write(message string) {
	client.mu.Lock()
	io.WriteString(client.conn, message) //nolint:errcheck
	if logger.EnableNetworkLogging {
		logger.WriteNetworkLog(client.ipid, client.hdid, "SEND", message)
	}
	client.mu.Unlock()
}

// SendPacket sends the client a packet with the given header and contents.
// A bytes.Buffer from packetBufPool is used to assemble the packet in a
// single allocation-free pass; the buffer is returned to the pool afterwards.
// Write errors are intentionally ignored: any underlying connection failure
// will surface on the next read in HandleClient, which closes the connection.
func (client *Client) SendPacket(header string, contents ...string) {
	b := packetBufPool.Get().(*bytes.Buffer)
	b.Reset()
	b.WriteString(header)
	for _, c := range contents {
		b.WriteByte('#')
		b.WriteString(c)
	}
	b.WriteString("#%")

	client.mu.Lock()
	client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	if logger.EnableNetworkLogging {
		// Logging paths need a string copy; keep them off the common fast path.
		msg := b.String()
		client.conn.Write(b.Bytes()) //nolint:errcheck
		logger.WriteNetworkLog(client.ipid, client.hdid, "SEND", msg)
	} else {
		b.WriteTo(client.conn) //nolint:errcheck
	}
	client.conn.SetWriteDeadline(time.Time{}) //nolint:errcheck
	client.mu.Unlock()

	packetBufPool.Put(b)
}

// clientLogSnapshot holds the subset of client fields read by addToBuffer.
// All fields are captured under a single mutex acquisition in logSnapshot,
// eliminating the 6+ separate lock/unlock cycles that calling individual
// getters would require.
type clientLogSnapshot struct {
	charName string
	ipid     string
	hdid     string
	showname string
	oocName  string
	area     *area.Area
}

// logSnapshot captures the client fields needed for area-log and buffer entries
// under a single mutex lock.
func (client *Client) logSnapshot() clientLogSnapshot {
	client.mu.Lock()
	var charName string
	if client.char == -1 {
		charName = "Spectator"
	} else if client.char >= 0 && client.char < len(characters) {
		charName = characters[client.char]
	}
	snap := clientLogSnapshot{
		charName: charName,
		ipid:     client.ipid,
		hdid:     client.hdid,
		showname: client.showname,
		oocName:  client.oocName,
		area:     client.area,
	}
	client.mu.Unlock()
	return snap
}

// clientCleanup cleans up a disconnected client.
func (client *Client) clientCleanup() {
	if client.Uid() != -1 {
		logger.LogInfof("Client (IPID:%v UID:%v) left the server", client.ipid, client.Uid())

		// Accumulate session playtime and award 1 chip per newly-completed hour.
		// AddPlaytimeReturning is a single atomic SQL operation, so concurrent
		// disconnects for the same IPID (multiclient) cannot race on the boundary.
		if connAt := client.ConnectedAt(); !connAt.IsZero() {
			sessionSecs := int64(time.Since(connAt).Seconds())
			if sessionSecs > 0 {
				ipid := client.Ipid()
				alreadyAwarded := client.SessionChipsAwarded()
				go func() {
					newPt, err := db.AddPlaytimeReturning(ipid, sessionSecs)
					if err != nil {
						logger.LogErrorf("Failed to add playtime for %v: %v", ipid, err)
						return
					}
					oldPt := newPt - sessionSecs
					chipsEarned := (newPt / secondsPerHour) - (oldPt / secondsPerHour) - alreadyAwarded
					if chipsEarned > 0 && config.EnableCasino {
						if err := db.EnsureChipBalance(ipid); err == nil {
							if _, err := db.AddChips(ipid, chipsEarned); err != nil {
								logger.LogErrorf("Failed to award playtime chips for %v: %v", ipid, err)
							}
						}
					}
				}()
			}
		}

		// Clear possession links if this client was possessing someone
		if client.Possessing() != -1 {
			client.SetPossessing(-1)
			client.SetPossessedPos("")
		}

		// Clear possession links if anyone was possessing this client
		uid := client.Uid()
		clients.ForEach(func(c *Client) {
			if c.Possessing() == uid {
				c.SetPossessing(-1)
				c.SetPossessedPos("")
			}
		})

		if client.Area().PlayerCount() <= 1 {
			client.Area().Reset()
			sendLockArup()
			sendStatusArup()
			sendCMArup()
		} else if client.Area().HasCM(client.Uid()) {
			client.Area().RemoveCM(client.Uid())
			sendCMArup()
		}
		for _, a := range areas {
			if a.Lock() != area.LockFree {
				a.RemoveInvited(client.Uid())
			}
		}
		uids.ReleaseUid(client.Uid())
		players.RemovePlayer()
		if config.Advertise {
			updatePlayers <- players.GetPlayerCount()
		}
		client.Area().RemoveChar(client.CharID())
		if !client.Hidden() {
			client.Area().RemoveVisiblePlayer()
		}
		writeToAll("PR", strconv.Itoa(client.Uid()), "1")
		sendPlayerArup()
	}
	handleCasinoDisconnect(client)
	handleMafiaDisconnect(client)
	client.conn.Close()
	clients.RemoveClient(client)
}

// SendServerMessage sends a server OOC message to the client.
func (client *Client) SendServerMessage(message string) {
	client.SendPacket("CT", encodedServerName, encode(message), "1")
}

// KickForRateLimit kicks the client for exceeding the message (IC/OOC/music) rate limit.
// Message-based rate limits always result in a kick, not a ban. Only raw packet flooding
// (handled separately) results in an automatic ban.
func (client *Client) KickForRateLimit() {
	client.SendServerMessage("You have been kicked for spamming.")
	logger.LogInfof("Client (IPID:%v UID:%v) kicked for exceeding rate limit", client.Ipid(), client.Uid())
	client.conn.Close()
}

// CurrentCharacter returns the client's current character name.
func (client *Client) CurrentCharacter() string {
	if client.CharID() == -1 {
		return "Spectator"
	} else {
		return characters[client.CharID()]
	}
}

// timeout closes an unjoined client's connection after 1 minute.
// Once the client has joined, if ping_timeout is configured, it also disconnects
// the client whenever the time since its last CH packet exceeds that threshold.
func timeout(client *Client) {
	time.Sleep(1 * time.Minute)
	if client.Uid() == -1 {
		client.conn.Close()
		return
	}
	deadline := config.PingTimeout
	if deadline <= 0 {
		return
	}
	interval := time.Duration(deadline) * time.Second
	intervalNanos := interval.Nanoseconds()
	for {
		time.Sleep(interval)
		if client.Uid() == -1 {
			return
		}
		if nanos := client.lastPingNano.Load(); nanos != 0 && time.Now().UnixNano()-nanos > intervalNanos {
			logger.LogInfof("Client (IPID:%v UID:%v) timed out: no CH packet in %v", client.Ipid(), client.Uid(), interval)
			client.conn.Close()
			return
		}
	}
}

// Hdid returns the client's hdid.
func (client *Client) Hdid() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.hdid
}

// SetHdid sets the client's hdid.
func (client *Client) SetHdid(hdid string) {
	client.mu.Lock()
	client.hdid = hdid
	client.mu.Unlock()
}

// Uid returns the client's user ID.
func (client *Client) Uid() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.uid
}

// SetUid sets the client's user ID.
func (client *Client) SetUid(id int) {
	client.mu.Lock()
	client.uid = id
	client.mu.Unlock()
}

// Area returns the client's current area.
func (client *Client) Area() *area.Area {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.area
}

// SetArea sets the client's current area.
func (client *Client) SetArea(area *area.Area) {
	client.mu.Lock()
	client.area = area
	client.mu.Unlock()
}

// CharID returns the client's character ID.
func (client *Client) CharID() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.char
}

// SetCharID sets the client's character ID.
func (client *Client) SetCharID(id int) {
	client.mu.Lock()
	client.char = id
	client.charIDStr = strconv.Itoa(id)
	client.mu.Unlock()
}

// CharIDStr returns the client's character ID pre-converted to a string.
// This avoids a strconv.Itoa allocation on every IC/MC packet validation.
func (client *Client) CharIDStr() string {
	client.mu.Lock()
	s := client.charIDStr
	client.mu.Unlock()
	return s
}

// Ipid returns the client's ipid.
func (client *Client) Ipid() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.ipid
}

// OOCName returns the client's current OOC username.
func (client *Client) OOCName() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.oocName
}

// SetOocName sets the client's OOC username.
func (client *Client) SetOocName(name string) {
	client.mu.Lock()
	client.oocName = name
	client.mu.Unlock()
}

// LastMsg returns the client's last sent IC message.
func (client *Client) LastMsg() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.lastmsg
}

// SetLastMsg sets the client's last sent IC message.
func (client *Client) SetLastMsg(msg string) {
	client.mu.Lock()
	client.lastmsg = msg
	client.mu.Unlock()
}

// LastTextColor returns the client's last used text color.
func (client *Client) LastTextColor() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.lastTextColor
}

// SetLastTextColor sets the client's last used text color.
func (client *Client) SetLastTextColor(color string) {
	client.mu.Lock()
	client.lastTextColor = color
	client.mu.Unlock()
}

// Perms returns the client's current permissions.
func (client *Client) Perms() uint64 {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.perms
}

// SetPerms sets the client's permissionss.
func (client *Client) SetPerms(perms uint64) {
	client.mu.Lock()
	client.perms = perms
	client.mu.Unlock()
}

// Authenticated returns whether the client is logged in as a moderator.
func (client *Client) Authenticated() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.authenticated
}

// SetAuthenticated sets whether the client is logged in as a moderator.
func (client *Client) SetAuthenticated(auth bool) {
	client.mu.Lock()
	client.authenticated = auth
	client.mu.Unlock()
}

// ModName returns the client's moderator username.
func (client *Client) ModName() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.mod_name
}

// SetModName sets the client's moderator username.
func (client *Client) SetModName(name string) {
	client.mu.Lock()
	client.mod_name = name
	client.mu.Unlock()
}

// PendingReg returns the client's pending registration data.
// username and captcha are empty strings, hashedPass is nil when no registration is pending.
func (client *Client) PendingReg() (username, captcha string, hashedPass []byte) {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.pendingRegUser, client.pendingRegCaptcha, client.pendingRegPass
}

// SetPendingReg stores a pending registration awaiting captcha confirmation.
// Pass empty strings and nil to clear any pending registration.
func (client *Client) SetPendingReg(username, captcha string, hashedPass []byte) {
	client.mu.Lock()
	client.pendingRegUser = username
	client.pendingRegCaptcha = captcha
	client.pendingRegPass = hashedPass
	client.mu.Unlock()
}

// Pos returns the client's current position.
func (client *Client) Pos() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.pos
}

// SetPos sets the client's position.
func (client *Client) SetPos(pos string) {
	client.mu.Lock()
	client.pos = pos
	client.mu.Unlock()
}

// CasePrefs returns all client's case preferences.
func (client *Client) CasePrefs() [5]bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.case_prefs
}

// CasePref returns a client's role alert preference.
func (client *Client) AlertRole(index int) bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.case_prefs[index]
}

// SetCasePref sets a client's role alert preference.
func (client *Client) SetRoleAlert(index int, b bool) {
	client.mu.Lock()
	client.case_prefs[index] = b
	client.mu.Unlock()
}

// PairInfo returns a client's pairing info.
func (client *Client) PairInfo() ClientPairInfo {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.pair
}

// SetPairInfo updates a client's pairing info.
func (client *Client) SetPairInfo(name string, emote string, flip string, offset string) {
	client.mu.Lock()
	client.pair.name, client.pair.emote, client.pair.flip, client.pair.offset = name, emote, flip, offset
	client.mu.Unlock()
}

// PairWantedID returns the character the client wishes to pair with.
func (client *Client) PairWantedID() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.pair.wanted_id
}

// SetPairWantedID sets the character the client wishes to pair with.
func (client *Client) SetPairWantedID(id int) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.pair.wanted_id = id
}

// ForcePairUID returns the UID of the client this client is force-paired with, or -1 if none.
func (client *Client) ForcePairUID() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.forcePairUID
}

// SetForcePairUID sets the UID of the client this client is force-paired with.
func (client *Client) SetForcePairUID(uid int) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.forcePairUID = uid
}

// RemoveAuth logs a client out as moderator.
func (client *Client) RemoveAuth() {
	client.mu.Lock()
	client.authenticated, client.perms, client.mod_name = false, 0, ""
	client.mu.Unlock()
	client.SendServerMessage("Logged out as moderator.")
	client.SendPacket("AUTH", "-1")
}

// RemoveAccountAuth logs a client out of a player account (no moderator badge change needed).
func (client *Client) RemoveAccountAuth() {
	client.mu.Lock()
	username := client.mod_name
	client.authenticated, client.perms, client.mod_name = false, 0, ""
	client.mu.Unlock()
	client.SendServerMessage(fmt.Sprintf("Logged out of account '%v'.", username))
}

// restorePunishments loads any persistent punishments for this client from the database
// and applies them. Called once after the client successfully joins the server.
func (client *Client) restorePunishments() {
	punishments, err := db.GetPunishments(client.Ipid())
	if err != nil {
		logger.LogErrorf("Error loading punishments for %v: %v", client.Ipid(), err)
		return
	}
	if len(punishments) == 0 {
		return
	}
	for _, p := range punishments {
		var expiresAt time.Time
		if p.Expires != 0 {
			expiresAt = time.Unix(p.Expires, 0).UTC()
		}
		switch p.Kind {
		case db.PunishKindMute:
			m := MuteState(p.Value)
			client.SetMuted(m)
			client.SetUnmuteTime(expiresAt)
		case db.PunishKindJail:
			client.SetJailedUntil(expiresAt)
			client.SetJailAreaID(p.Value)
		case db.PunishKindCharStuck:
			client.SetCharStuck(p.Value, expiresAt)
		case db.PunishKindText:
			pType := PunishmentType(p.Subtype)
			var remaining time.Duration
			if p.Expires != 0 {
				remaining = time.Until(expiresAt)
			}
			client.AddPunishment(pType, remaining, p.Reason)
		}
	}
	client.SendServerMessage("Your active punishments have been restored.")
	// If the client is jailed to a specific area, force-move them there on reconnect.
	jailArea := client.JailAreaID()
	if jailArea >= 0 {
		if jailArea < len(areas) && areas[jailArea] != client.Area() {
			client.forceChangeArea(areas[jailArea])
			client.SendServerMessage(fmt.Sprintf("You have been returned to your jail area: %v.", areas[jailArea].Name()))
		} else if jailArea >= len(areas) {
			logger.LogErrorf("Jailed IPID %v has out-of-range jail area ID %d; skipping force-move.", client.Ipid(), jailArea)
		}
	}
}

// CheckBanned checks whether the client is currently banned.
// If banned, it sends the BD packet, closes the connection, and returns true.
// Callers must return immediately when CheckBanned returns true.
func (client *Client) CheckBanned(by db.BanLookup) bool {
	var banned bool
	var baninfo db.BanInfo
	var err error
	switch by {
	case db.IPID:
		banned, baninfo, err = db.IsBanned(by, client.Ipid())
		if err != nil {
			logger.LogErrorf("Error reading IP ban for %v: %v", client.Ipid(), err)
		}
	case db.HDID:
		banned, baninfo, err = db.IsBanned(by, client.Hdid())
		if err != nil {
			logger.LogErrorf("Error reading HDID ban for %v: %v", client.Ipid(), err)
		}
	}

	if banned {
		var duration string
		if baninfo.Duration == -1 {
			duration = "∞"
		} else {
			duration = time.Unix(baninfo.Duration, 0).UTC().Format("02 Jan 2006 15:04 MST")
		}
		client.SendPacket("BD", fmt.Sprintf("%v\nUntil: %v\nID: %v", baninfo.Reason, duration, baninfo.Id))
		client.conn.Close()
		return true
	}
	return false
}

// JoinArea adds a client to an area.
func (client *Client) JoinArea(area *area.Area) {
	client.SetArea(area)
	area.AddChar(client.CharID())
	if !client.Hidden() {
		area.AddVisiblePlayer()
	}
	def, pro := area.HP()
	client.SendPacket("LE", areas[0].Evidence()...)
	client.SendPacket("CharsCheck", area.Taken()...)
	client.SendPacket("HP", "1", strconv.Itoa(def))
	client.SendPacket("HP", "2", strconv.Itoa(pro))
	client.SendPacket("BN", area.Background())
	if desc := area.Description(); desc != "" {
		client.SendServerMessage("📍 " + desc)
	}
	sendPlayerArup()
}

// ChangeArea changes the client's current area.
func (client *Client) ChangeArea(a *area.Area) bool {
	// Check if client is jailed
	if time.Now().UTC().Before(client.JailedUntil()) && !client.JailedUntil().IsZero() {
		client.SendServerMessage("You are jailed in this area")
		return false
	}
	if a.Lock() == area.LockLocked &&
		!a.HasInvited(client.Uid()) &&
		!permissions.HasPermission(client.Perms(), permissions.PermissionField["BYPASS_LOCK"]) {
		// Moderators without BYPASS_LOCK can force entry for emergencies on a
		// second attempt within the window — the first attempt warns them.
		if permissions.IsModerator(client.Perms()) {
			client.mu.Lock()
			pendingArea := client.emergencyBypassArea
			pendingAt := client.emergencyBypassAt
			client.mu.Unlock()
			if pendingArea == a && time.Since(pendingAt) <= emergencyBypassWindow {
				client.mu.Lock()
				client.emergencyBypassArea = nil
				client.emergencyBypassAt = time.Time{}
				client.mu.Unlock()
				logger.LogInfof("Moderator %v (UID:%v IPID:%v) used emergency bypass to enter locked area %v",
					client.ModName(), client.Uid(), client.Ipid(), a.Name())
				addToBuffer(client, "MOD", fmt.Sprintf("Used emergency bypass to enter locked area %v.", a.Name()), true)
			} else {
				client.mu.Lock()
				client.emergencyBypassArea = a
				client.emergencyBypassAt = time.Now()
				client.mu.Unlock()
				client.SendServerMessage(fmt.Sprintf(
					"🔒 %v is locked. If this is an emergency, try again within %d seconds to force entry.",
					a.Name(), int(emergencyBypassWindow.Seconds())))
				return false
			}
		} else {
			return false
		}
	}
	if client.Area() != nil {
		addToBuffer(client, "AREA", "Left area.", false)
		if client.Area().PlayerCount() <= 1 {
			client.Area().Reset()
			sendLockArup()
			sendStatusArup()
			sendCMArup()
		} else if client.Area().HasCM(client.Uid()) {
			client.Area().RemoveCM(client.Uid())
			sendCMArup()
		}
		client.Area().RemoveChar(client.CharID())
		if !client.Hidden() {
			client.Area().RemoveVisiblePlayer()
		}
	}
	if a.IsTaken(client.CharID()) {
		client.SetCharID(-1)
	}
	client.JoinArea(a)
	writeToAll("PU", strconv.Itoa(client.Uid()), "3", strconv.Itoa(getAreaIndex(a)))
	if client.CharID() == -1 {
		client.SendPacket("DONE")
	} else {
		writeToArea(a, "CharsCheck", a.Taken()...)
	}
	addToBuffer(client, "AREA", "Joined area.", false)
	return true
}

// HasCMPermission returns whether the client has CM permissions in it's area.
func (client *Client) HasCMPermission() bool {
	return client.Area().HasCM(client.Uid()) || permissions.HasPermission(client.Perms(), permissions.PermissionField["CM"])
}

// CanSpeakIC returns whether the client can send IC messages.
func (client *Client) CanSpeakIC() bool {
	switch {
	case client.CharID() == -1:
		return false
	case client.Area().Lock() == area.LockSpectatable && !client.area.HasInvited(client.Uid()) &&
		!permissions.HasPermission(client.Perms(), permissions.PermissionField["BYPASS_LOCK"]):
		return false
	case client.Area().SpectateMode() && !client.Area().HasCM(client.Uid()) && !client.Area().HasSpectateInvited(client.Uid()) &&
		!permissions.HasPermission(client.Perms(), permissions.PermissionField["BYPASS_LOCK"]):
		return false
	case client.Muted() == ICMuted || client.Muted() == ICOOCMuted:
		return client.CheckUnmute()
	}
	return true
}

// CanSpeakOOC returns whether the client can send OOC messages.
func (client *Client) CanSpeakOOC() bool {
	if client.IsJailed() {
		return false
	}
	m := client.Muted()
	if m == OOCMuted || m == ICOOCMuted {
		return client.CheckUnmute()
	}
	return true
}

// IsJailed returns true if the client is currently serving an active jail sentence.
func (client *Client) IsJailed() bool {
	t := client.JailedUntil()
	return !t.IsZero() && time.Now().UTC().Before(t)
}

// CanChangeMusic returns whether the client can change the music.
func (client *Client) CanChangeMusic() bool {
	switch {
	case client.CharID() == -1:
		return false
	case client.Area().LockMusic() && !client.HasCMPermission():
		return false
	case client.Area().Lock() == area.LockSpectatable && !client.area.HasInvited(client.Uid()) &&
		!permissions.HasPermission(client.Perms(), permissions.PermissionField["BYPASS_LOCK"]):
		return false
	case client.Muted() == MusicMuted || client.Muted() == ICMuted || client.Muted() == ICOOCMuted:
		return client.CheckUnmute()
	}
	return true
}

// CanJud returns whether the client can use judge actions.
func (client *Client) CanJud() bool {
	switch {
	case client.CharID() == -1:
		return false
	case client.Area().Lock() == area.LockSpectatable && !client.area.HasInvited(client.Uid()) &&
		!permissions.HasPermission(client.Perms(), permissions.PermissionField["BYPASS_LOCK"]):
		return false
	case client.Muted() == JudMuted || client.Muted() == ICMuted || client.Muted() == ICOOCMuted:
		return client.CheckUnmute()
	}
	return true
}

// CheckUnmute checks the client's mute duration, unmuting them if nessecary, and returning whether the client is still muted.
func (client *Client) CheckUnmute() bool {
	if time.Now().UTC().After(client.UnmuteTime()) && !client.UnmuteTime().IsZero() {
		client.SendServerMessage("You have been unmuted.")
		client.SetMuted(Unmuted)
		go func(ipid string) {
			if err := db.DeleteMute(ipid); err != nil {
				logger.LogErrorf("Failed to remove expired mute from DB for %v: %v", ipid, err)
			}
		}(client.ipid)
		return true
	}
	return false
}

// IsParrot returns if the client has been parroted.
func (client *Client) IsParrot() bool {
	if client.Muted() == ParrotMuted {
		return !client.CheckUnmute()
	}
	return false
}

// IsNarrator returns whether the client is a narrator.
func (client *Client) IsNarrator() bool {
	return client.narrator
}

// ToggleNarrator sets whether a client is a narrator.
func (client *Client) ToggleNarrator() {
	client.mu.Lock()
	client.narrator = !client.narrator
	client.mu.Unlock()
	if client.narrator {
		client.SendServerMessage("You are now in narrator mode.")
	} else {
		client.SendServerMessage("You are no longer in narrator mode.")
	}
}

// ToggleDance toggles dance mode on or off and notifies the client.
func (client *Client) ToggleDance() {
	client.mu.Lock()
	client.dancing = !client.dancing
	if !client.dancing {
		client.danceFlipped = false
	}
	client.mu.Unlock()
	if client.dancing {
		client.SendServerMessage("Dance mode enabled. Your sprite will flip and unflip with every message.")
	} else {
		client.SendServerMessage("Dance mode disabled.")
	}
}

// CheckAndToggleDanceFlip atomically checks if dance mode is active and, if so,
// toggles the flip state in one lock acquisition. Returns the new flip value
// ("0" or "1") when dancing, or "" when dance mode is off.
func (client *Client) CheckAndToggleDanceFlip() string {
	client.mu.Lock()
	if !client.dancing {
		client.mu.Unlock()
		return ""
	}
	client.danceFlipped = !client.danceFlipped
	flipped := client.danceFlipped
	client.mu.Unlock()
	if flipped {
		return "1"
	}
	return "0"
}

// canAlterEvidence is a helper function that returns if a client can alter evidence in their current area.
func (client *Client) CanAlterEvidence() bool {
	if client.CharID() == -1 || !client.CanSpeakIC() {
		return false
	}
	switch client.Area().EvidenceMode() {
	case area.EviMods:
		if !permissions.HasPermission(client.Perms(), permissions.PermissionField["MOD_EVI"]) {
			return false
		}
	case area.EviCMs:
		if !client.HasCMPermission() {
			return false
		}
	}
	return true
}

// ChangeCharacter changes the client's character to the given character.
func (client *Client) ChangeCharacter(id int) {
	if client.Area().SwitchChar(client.CharID(), id) {
		client.SetCharID(id)
		// Do not reset showname here; it is set from IC messages so the
		// player's display name (e.g. "Adachi") persists across character
		// changes and is used correctly by possession commands.
		client.SendPacket("PV", "0", "CID", strconv.Itoa(id))
		writeToArea(client.Area(), "CharsCheck", client.Area().Taken()...)
		if client.Uid() != -1 {
			uid := strconv.Itoa(client.Uid())
			writeToAll("PU", uid, "1", client.CurrentCharacter())
			writeToAll("PU", uid, "2", decode(client.Showname()))
		}
	}
}

// Muted returns the client's mute state.
func (client *Client) Muted() MuteState {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.muted
}

// SetMuted sets the client's mute state.
func (client *Client) SetMuted(m MuteState) {
	client.mu.Lock()
	client.muted = m
	client.mu.Unlock()
}

// UnmuteTime returns the time when the client should be unmuted.
// If this the time is zero, the mute does not expire.
func (client *Client) UnmuteTime() time.Time {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.muteuntil
}

// SetUnmuteTime sets the time when the client should be unmuted.
func (client *Client) SetUnmuteTime(t time.Time) {
	client.mu.Lock()
	client.muteuntil = t
	client.mu.Unlock()
}

// Showname returns the client's showname.
func (client *Client) Showname() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.showname
}

// SetShowname sets the client's showname
func (client *Client) SetShowname(s string) {
	client.mu.Lock()
	client.showname = s
	client.mu.Unlock()
}

// ForcedShowname returns the showname forced by a moderator, or "" if none is set.
func (client *Client) ForcedShowname() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.forcedShowname
}

// SetForcedShowname sets the moderator-forced showname for the client.
func (client *Client) SetForcedShowname(s string) {
	client.mu.Lock()
	client.forcedShowname = s
	client.mu.Unlock()
}

// ForcedIniswapInfo returns the moderator-forced iniswap character name and its
// pre-computed char-ID string. Both values are read under a single mutex
// acquisition. Returns two empty strings when no forced iniswap is active.
func (client *Client) ForcedIniswapInfo() (charName, charIDStr string) {
	client.mu.Lock()
	charName, charIDStr = client.forcedIniswapChar, client.forcedIniswapIDStr
	client.mu.Unlock()
	return
}

// IsTunged returns true when the client has an active moderator-forced iniswap
// (i.e. a /tung effect). While true the client must not be allowed to change
// characters on their own — the lock is lifted by calling
// SetForcedIniswapChar("", "").
func (client *Client) IsTunged() bool {
	client.mu.Lock()
	active := client.forcedIniswapChar != ""
	client.mu.Unlock()
	return active
}

// SetForcedIniswapChar sets (or clears) the moderator-forced iniswap character.
// Pass charName="" and charIDStr="" to clear the effect.
// Both fields are written under a single mutex acquisition.
// Note: these are distinct from client.char / client.charIDStr (the real
// character slot). CharIDStr() always returns the real slot regardless of
// whether a forced iniswap is active.
func (client *Client) SetForcedIniswapChar(charName, charIDStr string) {
	client.mu.Lock()
	client.forcedIniswapChar, client.forcedIniswapIDStr = charName, charIDStr
	client.mu.Unlock()
}

// EffectiveShowname returns the moderator-forced showname if one is set,
// otherwise the client's own showname. Both fields are read under a single
// mutex acquisition to avoid two separate lock/unlock cycles in callers.
func (client *Client) EffectiveShowname() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.forcedShowname != "" {
		return client.forcedShowname
	}
	return client.showname
}

// UpdateShowname atomically updates the client's showname and returns true if
// the stored value actually changed. Callers use the boolean to decide whether
// to broadcast a PU packet, eliminating two extra lock/unlock pairs that a
// read-then-write-then-read pattern would require.
func (client *Client) UpdateShowname(s string) bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.showname == s {
		return false
	}
	client.showname = s
	return true
}

// JailedUntil returns the time when the client's jail expires.
func (client *Client) JailedUntil() time.Time {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.jailedUntil
}

// SetJailedUntil sets the time when the client's jail expires.
func (client *Client) SetJailedUntil(t time.Time) {
	client.mu.Lock()
	client.jailedUntil = t
	client.mu.Unlock()
}

// JailAreaID returns the area index where this client is jailed (-1 = no specific area).
func (client *Client) JailAreaID() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.jailAreaID
}

// SetJailAreaID sets the area index where this client is jailed.
func (client *Client) SetJailAreaID(id int) {
	client.mu.Lock()
	client.jailAreaID = id
	client.mu.Unlock()
}

// IsCharStuck returns true if the client is currently under a character-stuck restriction.
// Both fields are read under a single mutex lock to avoid double-locking.
func (client *Client) IsCharStuck() bool {
	client.mu.Lock()
	id := client.charStuckCharID
	t := client.charStuckUntil
	client.mu.Unlock()
	return id >= 0 && !t.IsZero() && time.Now().UTC().Before(t)
}

// charStuckID returns the locked character ID if the client is currently stuck, or -1 if not.
// Both fields are read under a single lock; this is the preferred hot-path check that avoids
// the need to call IsCharStuck and CharStuckCharID separately.
func (client *Client) charStuckID() int {
	client.mu.Lock()
	id := client.charStuckCharID
	t := client.charStuckUntil
	client.mu.Unlock()
	if id >= 0 && !t.IsZero() && time.Now().UTC().Before(t) {
		return id
	}
	return -1
}

// SetCharStuck atomically sets both the locked character ID and the expiry time in one lock.
func (client *Client) SetCharStuck(id int, until time.Time) {
	client.mu.Lock()
	client.charStuckCharID = id
	client.charStuckUntil = until
	client.mu.Unlock()
}

// ClearCharStuck atomically clears both char-stuck fields in one lock.
func (client *Client) ClearCharStuck() {
	client.mu.Lock()
	client.charStuckCharID = -1
	client.charStuckUntil = time.Time{}
	client.mu.Unlock()
}

// Hidden returns whether the client is hidden from the player list and area counts.
func (client *Client) Hidden() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.hidden
}

// SetHidden sets whether the client is hidden from the player list and area counts.
func (client *Client) SetHidden(h bool) {
	client.mu.Lock()
	client.hidden = h
	client.mu.Unlock()
}

// GambleHide returns whether the client has opted out of gambling broadcast messages.
func (client *Client) GambleHide() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.gambleHide
}

// SetGambleHide sets whether the client wants to suppress gambling broadcast messages.
func (client *Client) SetGambleHide(h bool) {
	client.mu.Lock()
	client.gambleHide = h
	client.mu.Unlock()
}

// forceChangeArea moves the client to the given area unconditionally, bypassing
// the jailed-player area-lock and area invitation checks that ChangeArea enforces.
// Used to place a jailed player into their designated cell (both at jail time and on reconnect).
func (client *Client) forceChangeArea(a *area.Area) {
	addToBuffer(client, "AREA", "Left area.", false)
	if client.Area().PlayerCount() <= 1 {
		client.Area().Reset()
		sendLockArup()
		sendStatusArup()
		sendCMArup()
	} else if client.Area().HasCM(client.Uid()) {
		client.Area().RemoveCM(client.Uid())
		sendCMArup()
	}
	client.Area().RemoveChar(client.CharID())
	if !client.Hidden() {
		client.Area().RemoveVisiblePlayer()
	}
	if a.IsTaken(client.CharID()) {
		client.SetCharID(-1)
	}
	client.JoinArea(a)
	writeToAll("PU", strconv.Itoa(client.Uid()), "3", strconv.Itoa(getAreaIndex(a)))
	if client.CharID() == -1 {
		client.SendPacket("DONE")
	} else {
		writeToArea(a, "CharsCheck", a.Taken()...)
	}
	addToBuffer(client, "AREA", "Joined area.", false)
}

// LastRpsTime returns the last time the client played RPS.
func (client *Client) LastRpsTime() time.Time {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.lastRpsTime
}

// SetLastRpsTime sets the last time the client played RPS.
func (client *Client) SetLastRpsTime(t time.Time) {
	client.mu.Lock()
	client.lastRpsTime = t
	client.mu.Unlock()
}

// CheckModcallCooldown checks if the client is within the modcall cooldown period.
// Returns true (and the remaining seconds, rounded up) if the client must wait, false otherwise.
// When the cooldown is disabled (0), always returns false.
func (client *Client) CheckModcallCooldown() (bool, int) {
	if config.ModcallCooldown <= 0 {
		return false, 0
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.lastModcallTime.IsZero() {
		return false, 0
	}
	elapsed := time.Since(client.lastModcallTime)
	cooldown := time.Duration(config.ModcallCooldown) * time.Second
	if elapsed < cooldown {
		remaining := int(math.Ceil((cooldown - elapsed).Seconds()))
		return true, remaining
	}
	return false, 0
}

// SetLastModcallTime records the current time as the client's last modcall time.
func (client *Client) SetLastModcallTime() {
	client.mu.Lock()
	client.lastModcallTime = time.Now()
	client.mu.Unlock()
}

const barDrinkCooldown = 20 * time.Second

// CheckBarDrinkCooldown checks if the client is within the bar drink cooldown period.
// Returns true (and the remaining seconds, rounded up) if the client must wait, false otherwise.
func (client *Client) CheckBarDrinkCooldown() (bool, int) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.lastBarDrinkTime.IsZero() {
		return false, 0
	}
	elapsed := time.Since(client.lastBarDrinkTime)
	if elapsed < barDrinkCooldown {
		remaining := int(math.Ceil((barDrinkCooldown - elapsed).Seconds()))
		return true, remaining
	}
	return false, 0
}

// SetLastBarDrinkTime records the current time as the client's last /bar buy time.
func (client *Client) SetLastBarDrinkTime() {
	client.mu.Lock()
	client.lastBarDrinkTime = time.Now()
	client.mu.Unlock()
}

// LastRandomCharTime returns the last time the client used /randomchar.
func (client *Client) LastRandomCharTime() time.Time {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.lastRandomCharTime
}

// SetLastRandomCharTime records the current time as the client's last /randomchar time.
func (client *Client) SetLastRandomCharTime(t time.Time) {
	client.mu.Lock()
	client.lastRandomCharTime = t
	client.mu.Unlock()
}

// CheckAndUpdateRandomBgCooldown atomically checks whether the /randombg cooldown
// has elapsed and, if so, records the current time as the new last-use timestamp.
// It returns (true, 0) when the command is allowed, or (false, remaining) when
// the client is still in cooldown.
func (client *Client) CheckAndUpdateRandomBgCooldown(cooldown time.Duration) (bool, time.Duration) {
	client.mu.Lock()
	defer client.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(client.lastRandomBgTime)
	if !client.lastRandomBgTime.IsZero() && elapsed < cooldown {
		return false, cooldown - elapsed
	}
	client.lastRandomBgTime = now
	return true, 0
}

// CheckAndUpdateRandomSongCooldown atomically checks whether the /randomsong cooldown
// has elapsed and, if so, records the current time as the new last-use timestamp.
// It returns (true, 0) when the command is allowed, or (false, remaining) when
// the client is still in cooldown.
func (client *Client) CheckAndUpdateRandomSongCooldown(cooldown time.Duration) (bool, time.Duration) {
	client.mu.Lock()
	defer client.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(client.lastRandomSongTime)
	if !client.lastRandomSongTime.IsZero() && elapsed < cooldown {
		return false, cooldown - elapsed
	}
	client.lastRandomSongTime = now
	return true, 0
}

// String returns the string representation of a mute state.
func (m MuteState) String() string {
	switch m {
	case ICMuted:
		return "IC"
	case OOCMuted:
		return "OOC"
	case ICOOCMuted:
		return "IC/OOC"
	case MusicMuted:
		return "from changing the music"
	case JudMuted:
		return "from judge controls"
	}
	return ""
}

// AddPunishment adds a punishment to the client.
func (client *Client) AddPunishment(pType PunishmentType, duration time.Duration, reason string) {
	client.mu.Lock()
	defer client.mu.Unlock()

	// Remove existing punishment of the same type (prevent duplicate same-type punishments)
	// Different punishment types can coexist and stack their effects
	for i := len(client.punishments) - 1; i >= 0; i-- {
		if client.punishments[i].punishmentType == pType {
			client.punishments = append(client.punishments[:i], client.punishments[i+1:]...)
			break
		}
	}

	expiresAt := time.Time{}
	if duration > 0 {
		expiresAt = time.Now().UTC().Add(duration)
	}

	client.punishments = append(client.punishments, PunishmentState{
		punishmentType: pType,
		expiresAt:      expiresAt,
		reason:         reason,
		lastMsgTime:    time.Time{},
		msgDelay:       0,
		msgCount:       0,
		lastEffect:     0,
		targetUID:      -1,
	})
}

// RemovePunishment removes a punishment type from the client.
func (client *Client) RemovePunishment(pType PunishmentType) {
	client.mu.Lock()
	defer client.mu.Unlock()

	for i := len(client.punishments) - 1; i >= 0; i-- {
		if client.punishments[i].punishmentType == pType {
			client.punishments = append(client.punishments[:i], client.punishments[i+1:]...)
			return
		}
	}
}

// AddLovebombPunishment adds a lovebomb punishment with an optional specific target UID.
// Use targetUID=-1 for a random area target on each message.
func (client *Client) AddLovebombPunishment(targetUID int, duration time.Duration, reason string) {
	client.mu.Lock()
	defer client.mu.Unlock()

	// Remove existing lovebomb (same-type deduplication)
	for i := len(client.punishments) - 1; i >= 0; i-- {
		if client.punishments[i].punishmentType == PunishmentLovebomb {
			client.punishments = append(client.punishments[:i], client.punishments[i+1:]...)
			break
		}
	}

	expiresAt := time.Time{}
	if duration > 0 {
		expiresAt = time.Now().UTC().Add(duration)
	}

	client.punishments = append(client.punishments, PunishmentState{
		punishmentType: PunishmentLovebomb,
		expiresAt:      expiresAt,
		reason:         reason,
		targetUID:      targetUID,
	})
}

// RemoveAllPunishments removes all punishments from the client.
func (client *Client) RemoveAllPunishments() {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.punishments = []PunishmentState{}
}

// HasPunishment checks if the client has a specific punishment type.
func (client *Client) HasPunishment(pType PunishmentType) bool {
	client.mu.Lock()
	defer client.mu.Unlock()

	for _, p := range client.punishments {
		if p.punishmentType == pType {
			return true
		}
	}
	return false
}

// GetPunishment returns the punishment state for a specific type, or nil if not found.
// WARNING: The returned pointer should not be modified directly. Use UpdatePunishmentState instead.
// This method is provided for read-only access to punishment state.
func (client *Client) GetPunishment(pType PunishmentType) *PunishmentState {
	client.mu.Lock()
	defer client.mu.Unlock()

	for i := range client.punishments {
		if client.punishments[i].punishmentType == pType {
			return &client.punishments[i]
		}
	}
	return nil
}

// Punishments returns a snapshot copy of the client's active punishment list.
// The returned slice is a copy; modifying it has no effect on the client.
func (client *Client) Punishments() []PunishmentState {
	client.mu.Lock()
	defer client.mu.Unlock()
	snap := make([]PunishmentState, len(client.punishments))
	copy(snap, client.punishments)
	return snap
}

// HasAnyPunishment reports whether the client has at least one active
// punishment.  Unlike Punishments(), it never allocates a copy of the slice.
func (client *Client) HasAnyPunishment() bool {
	client.mu.Lock()
	n := len(client.punishments)
	client.mu.Unlock()
	return n > 0
}

// UpdatePunishmentState updates the state of a punishment (e.g., message counts).
func (client *Client) UpdatePunishmentState(pType PunishmentType, updateFunc func(*PunishmentState)) {
	client.mu.Lock()
	defer client.mu.Unlock()

	for i := range client.punishments {
		if client.punishments[i].punishmentType == pType {
			updateFunc(&client.punishments[i])
			return
		}
	}
}

// CheckExpiredPunishments removes all expired punishments and returns true if any were removed.
// Expired entries are also deleted from the database.
func (client *Client) CheckExpiredPunishments() bool {
	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now().UTC()
	removed := false

	for i := len(client.punishments) - 1; i >= 0; i-- {
		p := &client.punishments[i]
		if !p.expiresAt.IsZero() && now.After(p.expiresAt) {
			pType := p.punishmentType
			client.punishments = append(client.punishments[:i], client.punishments[i+1:]...)
			removed = true
			go func(ipid string, t PunishmentType) {
				if err := db.DeleteTextPunishment(ipid, int(t)); err != nil {
					logger.LogErrorf("Failed to remove expired punishment from DB for %v: %v", ipid, err)
				}
			}(client.ipid, pType)
		}
	}

	return removed
}

// GetActivePunishments returns a copy of all active punishments.
func (client *Client) GetActivePunishments() []PunishmentState {
	client.mu.Lock()
	defer client.mu.Unlock()

	// Clean up expired punishments first
	now := time.Now().UTC()
	active := make([]PunishmentState, 0, len(client.punishments))

	for _, p := range client.punishments {
		if p.expiresAt.IsZero() || now.Before(p.expiresAt) {
			active = append(active, p)
		}
	}

	return active
}

// CheckExpiredAndGetPunishments removes expired punishments and returns the remaining
// active ones under a single mutex acquisition.  wasExpired is true if at least one
// punishment was removed.  active is nil when there are no punishments.
//
// Use this instead of calling CheckExpiredPunishments() + GetActivePunishments()
// separately; it halves the number of mutex lock/unlock cycles and eliminates the
// redundant time.Now() call on every IC message.
func (client *Client) CheckExpiredAndGetPunishments() (wasExpired bool, active []PunishmentState) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if len(client.punishments) == 0 {
		return false, nil
	}

	now := time.Now().UTC()
	w := 0
	for i := range client.punishments {
		p := &client.punishments[i]
		if !p.expiresAt.IsZero() && now.After(p.expiresAt) {
			wasExpired = true
			pType := p.punishmentType
			ipid := client.ipid
			go func() {
				if err := db.DeleteTextPunishment(ipid, int(pType)); err != nil {
					logger.LogErrorf("Failed to remove expired punishment from DB for %v: %v", ipid, pType)
				}
			}()
		} else {
			client.punishments[w] = client.punishments[i]
			w++
		}
	}
	client.punishments = client.punishments[:w]

	if w == 0 {
		return wasExpired, nil
	}
	active = make([]PunishmentState, w)
	copy(active, client.punishments)
	return wasExpired, active
}

// String returns the string representation of a punishment type.
func (p PunishmentType) String() string {
	switch p {
	case PunishmentWhisper:
		return "whisper"
	case PunishmentBackward:
		return "backward"
	case PunishmentStutterstep:
		return "stutterstep"
	case PunishmentElongate:
		return "elongate"
	case PunishmentUppercase:
		return "uppercase"
	case PunishmentLowercase:
		return "lowercase"
	case PunishmentRobotic:
		return "robotic"
	case PunishmentAlternating:
		return "alternating"
	case PunishmentFancy:
		return "fancy"
	case PunishmentUwu:
		return "uwu"
	case PunishmentPirate:
		return "pirate"
	case PunishmentShakespearean:
		return "shakespearean"
	case PunishmentCaveman:
		return "caveman"
	case PunishmentEmoji:
		return "emoji"
	case PunishmentInvisible:
		return "invisible"
	case PunishmentSlowpoke:
		return "slowpoke"
	case PunishmentFastspammer:
		return "fastspammer"
	case PunishmentPause:
		return "pause"
	case PunishmentLag:
		return "lag"
	case PunishmentSubtitles:
		return "subtitles"
	case PunishmentRoulette:
		return "roulette"
	case PunishmentSpotlight:
		return "spotlight"
	case PunishmentCensor:
		return "censor"
	case PunishmentConfused:
		return "confused"
	case PunishmentParanoid:
		return "paranoid"
	case PunishmentDrunk:
		return "drunk"
	case PunishmentHiccup:
		return "hiccup"
	case PunishmentWhistle:
		return "whistle"
	case PunishmentMumble:
		return "mumble"
	case PunishmentSpaghetti:
		return "spaghetti"
	case PunishmentTorment:
		return "torment"
	case PunishmentRng:
		return "rng"
	case PunishmentEssay:
		return "essay"
	case PunishmentHaiku:
		return "haiku"
	case PunishmentAutospell:
		return "autospell"
	case PunishmentMonkey:
		return "monkey"
	case PunishmentSnake:
		return "snake"
	case PunishmentDog:
		return "dog"
	case PunishmentCat:
		return "cat"
	case PunishmentBird:
		return "bird"
	case PunishmentCow:
		return "cow"
	case PunishmentFrog:
		return "frog"
	case PunishmentDuck:
		return "duck"
	case PunishmentHorse:
		return "horse"
	case PunishmentLion:
		return "lion"
	case PunishmentZoo:
		return "zoo"
	case PunishmentBunny:
		return "bunny"
	case PunishmentTsundere:
		return "tsundere"
	case PunishmentYandere:
		return "yandere"
	case PunishmentKuudere:
		return "kuudere"
	case PunishmentDandere:
		return "dandere"
	case PunishmentDeredere:
		return "deredere"
	case PunishmentHimedere:
		return "himedere"
	case PunishmentKamidere:
		return "kamidere"
	case PunishmentUndere:
		return "undere"
	case PunishmentBakadere:
		return "bakadere"
	case PunishmentMayadere:
		return "mayadere"
	case PunishmentEmoticon:
		return "emoticon"
	case PunishmentLovebomb:
		return "lovebomb"
	case PunishmentDegrade:
		return "degrade"
	case PunishmentTourettes:
		return "tourettes"
	case PunishmentSlang:
		return "slang"
	case PunishmentThesaurusOverload:
		return "thesaurusoverload"
	case PunishmentValleyGirl:
		return "valleygirl"
	case PunishmentBabytalk:
		return "babytalk"
	case PunishmentThirdPerson:
		return "thirdperson"
	case PunishmentUnreliableNarrator:
		return "unreliablenarrator"
	case PunishmentUncannyValley:
		return "uncannyvalley"
	case Punishment51:
		return "51"
	case PunishmentPhilosopher:
		return "philosopher"
	case PunishmentPoet:
		return "poet"
	case PunishmentUpsidedown:
		return "upsidedown"
	case PunishmentSarcasm:
		return "sarcasm"
	case PunishmentAcademic:
		return "academic"
	case PunishmentRecipe:
		return "recipe"
	case PunishmentQuote:
		return "quote"
	default:
		return "none"
	}
}

// CheckRateLimit checks if the client has exceeded the message rate limit.
// Returns true if the client should be kicked for spam, false otherwise.
// This is a lightweight implementation using a sliding window approach.
func (client *Client) CheckRateLimit() bool {
	// If rate limiting is disabled, always allow
	if config.RateLimit <= 0 {
		return false
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	window := rateLimitWindowDur
	if window == 0 {
		window = time.Duration(config.RateLimitWindow) * time.Second
	}

	// Remove timestamps outside the current window (sliding window)
	cutoff := now.Add(-window)

	// Find the first timestamp that is still within the window
	validIdx := -1
	for i, ts := range client.msgTimestamps {
		if ts.After(cutoff) {
			validIdx = i
			break
		}
	}

	// Clean up old timestamps
	if validIdx == -1 {
		// All timestamps are expired, release the underlying array for GC
		client.msgTimestamps = nil
	} else if validIdx > 0 {
		// Some timestamps are expired, remove them
		client.msgTimestamps = client.msgTimestamps[validIdx:]
	}

	// Check if rate limit is exceeded
	if len(client.msgTimestamps) >= config.RateLimit {
		return true
	}

	// Add current timestamp
	client.msgTimestamps = append(client.msgTimestamps, now)
	return false
}

// CheckOOCRateLimit checks if the client has exceeded the OOC message rate limit.
// Returns true if rate limit is exceeded and the OOC packet should be dropped, false if the packet is allowed.
// Uses a sliding window approach, mirroring CheckRateLimit.
func (client *Client) CheckOOCRateLimit() bool {
	if config.OOCRateLimit <= 0 {
		return false
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	window := oocRateLimitWindowDur
	if window == 0 {
		window = time.Duration(config.OOCRateLimitWindow) * time.Second
	}

	cutoff := now.Add(-window)

	validIdx := -1
	for i, ts := range client.oocMsgTimestamps {
		if ts.After(cutoff) {
			validIdx = i
			break
		}
	}

	if validIdx == -1 {
		client.oocMsgTimestamps = nil
	} else if validIdx > 0 {
		client.oocMsgTimestamps = client.oocMsgTimestamps[validIdx:]
	}

	if len(client.oocMsgTimestamps) >= config.OOCRateLimit {
		return true
	}

	client.oocMsgTimestamps = append(client.oocMsgTimestamps, now)
	return false
}

// CheckRawPacketRateLimit checks if the client has exceeded the raw packet rate limit.
// This counts every AO2 protocol packet regardless of type (PU, MS, CC, CH, etc.).
// Returns true if the client is flooding (and should be immediately banned), false otherwise.
//
// Uses a fixed-window counter: one int + one time.Time per client, zero heap allocations
// per call. This is called on every single incoming packet, so O(1) cost matters.
//
// Limit semantics: exactly RawPacketRateLimit packets are allowed per window;
// the (RawPacketRateLimit+1)th packet in the same window returns true.
func (client *Client) CheckRawPacketRateLimit() bool {
	if config.RawPacketRateLimit <= 0 {
		return false
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	window := rawPktRateLimitWindowDur
	if window == 0 {
		window = time.Duration(float64(time.Second) * config.RawPacketRateLimitWindow)
	}

	// Reset the counter when the window has expired or on the very first packet
	// (rawPktWindowStart is zero-valued for new clients).
	if client.rawPktWindowStart.IsZero() || now.Sub(client.rawPktWindowStart) >= window {
		client.rawPktWindowStart = now
		client.rawPktCount = 0
	}

	client.rawPktCount++
	return client.rawPktCount > config.RawPacketRateLimit
}

// Possessing returns the UID of the client being possessed, or -1 if not possessing anyone.
func (client *Client) Possessing() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.possessing
}

// SetPossessing sets the UID of the client being possessed.
func (client *Client) SetPossessing(uid int) {
	client.mu.Lock()
	client.possessing = uid
	client.mu.Unlock()
}

// PossessedPos returns the saved position of the possessed target.
func (client *Client) PossessedPos() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.possessedPos
}

// SetPossessedPos sets the saved position of the possessed target.
func (client *Client) SetPossessedPos(pos string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.possessedPos = pos
}

// ConnectedAt returns the time the client joined the server (was assigned a UID).
// Returns a zero Time if the client has not yet joined.
func (client *Client) ConnectedAt() time.Time {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.connectedAt
}

// SetConnectedAt records the time the client joined the server.
func (client *Client) SetConnectedAt(t time.Time) {
	client.mu.Lock()
	client.connectedAt = t
	client.mu.Unlock()
}

// SessionChipsAwarded returns the number of chips already awarded to this client
// by the hourly mid-session ticker during the current connection.
func (client *Client) SessionChipsAwarded() int64 {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.sessionChipsAwarded
}

// AddSessionChipsAwarded increments the mid-session chip award counter and
// returns the updated total.
func (client *Client) AddSessionChipsAwarded(n int64) int64 {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.sessionChipsAwarded += n
	return client.sessionChipsAwarded
}

// IgnoresIPID returns true if this client has permanently ignored the given IPID.
// Uses sync.Map.Load which is essentially lock-free for keys that have been stored
// at least once, keeping the hot per-message path free of mutex contention.
func (client *Client) IgnoresIPID(ipid string) bool {
	_, ok := client.ignoredIPIDs.Load(ipid)
	return ok
}

// AddIgnoredIPID adds an IPID to this client's in-memory permanent ignore set.
func (client *Client) AddIgnoredIPID(ipid string) {
	client.ignoredIPIDs.Store(ipid, struct{}{})
}

// RemoveIgnoredIPID removes an IPID from this client's in-memory permanent ignore set.
func (client *Client) RemoveIgnoredIPID(ipid string) {
	client.ignoredIPIDs.Delete(ipid)
}
