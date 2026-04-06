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
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	discordbot "github.com/MangosArentLiterature/Athena/internal/discord/bot"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/ms"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/playercount"
	"github.com/MangosArentLiterature/Athena/internal/settings"
	"github.com/MangosArentLiterature/Athena/internal/uidmanager"
	"github.com/MangosArentLiterature/Athena/internal/webhook"
	"github.com/ecnepsnai/discord"
	"github.com/xhit/go-str2duration/v2"
	"nhooyr.io/websocket"
)

const (
	version         = "v1.0.2"
	secondsPerHour  = int64(3600) // seconds in one hour; used for playtime-to-chips conversions
)

// encodedServerName is the AO2-encoded form of config.Name, pre-computed once
// at startup so that every server message avoids a repeated strings.Replacer call.
// Set in NewServer immediately after config is wired up; never modified afterwards.
var encodedServerName string

// connPool is a semaphore channel that limits the number of concurrently active
// connection goroutines when config.MaxConnectionGoroutines > 0.
// A nil channel means the pool is disabled (unbounded behaviour).
var connPool chan struct{}

var (
	config                                 *settings.Config
	characters, music, backgrounds, parrot []string
	charactersByName                       map[string]int // O(1) lookup: lowercase name → character ID
	areas                                  []*area.Area
	areaNames                              string
	smPacket                               string // pre-built SM#<areas>#<music>#% packet; built once at startup
	scPacket                               string // pre-built SC#<char1>#<char2>#...#% packet; built once at startup
	siPacket                               string // pre-built SI#<charCount>#<evidCount>#<musicCount>#% packet; built once at startup
	bgListStr                              string // pre-built background list for /bglist; zero alloc per call
	areaIndexMap                           map[*area.Area]int // pre-computed index lookup for O(1) getAreaIndex
	roles                                  []permissions.Role
	uids                                   *uidmanager.UidManager
	players                                playercount.PlayerCount
	enableDiscord                          bool
	clients                                *ClientList = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}
	updatePlayers                                     = make(chan int)      // Updates the advertiser's player count.
	advertDone                                        = make(chan struct{}) // Signals the advertiser to stop.
	FatalError                                        = make(chan error)    // Signals that the server should stop after a fatal error.
	RestartRequest                                    = make(chan struct{}) // Signals that the server should restart.

	// connTracker tracks connection attempts per IP for connection-rate limiting.
	connTracker = struct {
		mu         sync.Mutex
		timestamps map[string][]time.Time // ipid -> connection attempt timestamps
		rejections map[string]int         // ipid -> consecutive rejection count
	}{
		timestamps: make(map[string][]time.Time),
		rejections: make(map[string]int),
	}

	// ipModcallTracker tracks the last modcall time per IP across connections.
	ipModcallTracker = struct {
		mu    sync.Mutex
		times map[string]time.Time // ipid -> last modcall time
	}{
		times: make(map[string]time.Time),
	}

	// ipOOCTracker tracks OOC message timestamps per IP across connections.
	ipOOCTracker = struct {
		mu         sync.Mutex
		timestamps map[string][]time.Time // ipid -> OOC message timestamps
	}{
		timestamps: make(map[string][]time.Time),
	}

	// ipPingTracker tracks ping (CH) packet timestamps per IP across connections.
	ipPingTracker = struct {
		mu         sync.Mutex
		timestamps map[string][]time.Time // ipid -> ping timestamps
	}{
		timestamps: make(map[string][]time.Time),
	}

	// ipFirstSeenTracker records the first time each IPID connected to this server session.
	// Entries are never deleted so that returning IPIDs are never treated as "new" again.
	ipFirstSeenTracker = struct {
		mu    sync.Mutex
		times map[string]time.Time // ipid -> first seen time
	}{
		times: make(map[string]time.Time),
	}

	// globalNewIPTracker records timestamps of new unique IPs connecting to the server.
	// Used for the global new-connection rate limiter: if too many distinct new IPs arrive
	// within the configured window, additional unknown IPs are rejected until the window clears.
	globalNewIPTracker = struct {
		mu         sync.Mutex
		timestamps []time.Time
	}{}

	// serverLockdown, when set to true, prevents all new (previously-unseen) IPIDs from
	// connecting. Known IPIDs (those in ipFirstSeenTracker) are still allowed through.
	serverLockdown atomic.Bool

	// areaLastOOCMsg stores the last OOC message body (raw, as received) sent in each area.
	// Used to prevent consecutive identical OOC messages from different clients in the same area.
	// Key: *area.Area, Value: string. sync.Map is zero-value ready; no initialisation required.
	areaLastOOCMsg sync.Map

	// tormentedIPIDs holds the set of IPIDs that should be periodically disconnected.
	// Populated at startup from the database and updated at runtime by automod.
	// RWMutex is used so that the frequent read path (isIPIDTormented, called per
	// connection) never blocks other concurrent readers.
	tormentedIPIDs = struct {
		mu  sync.RWMutex
		set map[string]struct{}
	}{
		set: make(map[string]struct{}),
	}

	// Tournament mode state
	tournamentActive       bool
	tournamentMutex        sync.Mutex
	tournamentStartTime    time.Time
	tournamentParticipants map[int]*TournamentParticipant // uid -> participant data

	// server is the package-level singleton created by InitServer.
	server *Server
)

// TournamentParticipant tracks a user's tournament performance
type TournamentParticipant struct {
	uid          int
	messageCount int
	joinedAt     time.Time
}

// Server owns the runtime state for an Athena server instance and provides
// a structured API over it. It is created by NewServer and stored as the
// active instance in the package-level server variable. Package-level
// functions (e.g. ListenTCP) delegate to the active server's methods.
//
// Dependency injection: pass a *settings.Config to NewServer; all other
// dependencies (areas, roles, uids, etc.) are wired up inside the constructor.
// The package-level globals are kept in sync so that existing helper functions
// and command handlers continue to operate correctly.
type Server struct {
	config                 *settings.Config
	characters             []string
	music                  []string
	backgrounds            []string
	parrot                 []string
	areas                  []*area.Area
	areaNames              string
	bgListStr              string
	areaIndexMap           map[*area.Area]int
	roles                  []permissions.Role
	uids                   *uidmanager.UidManager
	players                playercount.PlayerCount
	enableDiscord          bool
	clients                *ClientList
	updatePlayers          chan int
	advertDone             chan struct{}
	tournamentActive       bool
	tournamentMutex        sync.Mutex
	tournamentStartTime    time.Time
	tournamentParticipants map[int]*TournamentParticipant
}

// NewServer initializes a new Server from the provided configuration, wiring
// up all dependencies (database, areas, roles, uid heap, advertiser, etc.).
// It also populates the package-level global variables so that existing helper
// functions and command handlers continue to operate correctly.
// Call InitServer for the legacy single-process entry point.
func NewServer(conf *settings.Config) (*Server, error) {
	db.Open()
	// Remove expired punishment rows left over from previous sessions.
	// A failure here is non-fatal: expired rows are harmless (GetPunishments filters
	// them at read-time), so we log and continue rather than aborting startup.
	if err := db.PurgeExpired(); err != nil {
		logger.LogErrorf("Failed to purge expired punishments: %v", err)
	}

	// Prune KNOWN_IPS rows with less than 1 hour of accumulated playtime.
	// IPs that have played for at least an hour are retained permanently.
	// This keeps bots and drive-by connections from polluting the known-IP list.
	const minPlaytimeSeconds int64 = 3600
	if n, err := db.PruneShortPlaytimeIPs(minPlaytimeSeconds); err != nil {
		logger.LogErrorf("Failed to prune low-playtime IPs: %v", err)
	} else if n > 0 {
		logger.LogInfof("Pruned %d IP(s) with less than 1 hour of playtime.", n)
	}

	// Sync chip balances for players who accumulated playtime before the casino
	// was introduced. Uses INSERT OR IGNORE so existing balances are never touched.
	if conf.EnableCasino {
		if n, err := db.SyncChipsForExistingPlaytime(); err != nil {
			logger.LogErrorf("Failed to sync chips for pre-casino playtime: %v", err)
		} else if n > 0 {
			logger.LogInfof("Synced chip balances for %d IP(s) with pre-casino playtime.", n)
		}
	}

	// Pre-populate the in-memory first-seen tracker with IPs that were seen in
	// previous server sessions.  This ensures returning players are never treated
	// as "new" by the global new-IP rate limiter after a restart.
	if knownIPs, err := db.LoadKnownIPs(); err != nil {
		logger.LogErrorf("Failed to load known IPs from database: %v", err)
	} else {
		// Use the zero time so that any per-IP cooldowns (OOC, modcall) are
		// considered long-expired for these returning players.
		epoch := time.Unix(0, 0)
		ipFirstSeenTracker.mu.Lock()
		for _, ipid := range knownIPs {
			if _, exists := ipFirstSeenTracker.times[ipid]; !exists {
				ipFirstSeenTracker.times[ipid] = epoch
			}
		}
		ipFirstSeenTracker.mu.Unlock()
		if len(knownIPs) > 0 {
			logger.LogInfof("Loaded %d known IPs from database.", len(knownIPs))
		}
	}

	// Pre-populate the in-memory tormented IPID set from the database.
	if tormentedIPs, err := db.LoadTormentedIPs(); err != nil {
		logger.LogErrorf("Failed to load tormented IPs from database: %v", err)
	} else {
		tormentedIPIDs.mu.Lock()
		for _, ipid := range tormentedIPs {
			tormentedIPIDs.set[ipid] = struct{}{}
		}
		tormentedIPIDs.mu.Unlock()
		if len(tormentedIPs) > 0 {
			logger.LogInfof("Loaded %d tormented IP(s) from database.", len(tormentedIPs))
		}
	}

	s := &Server{
		config:                 conf,
		clients:                &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int), hdidCounts: make(map[string]int)},
		uids:                   &uidmanager.UidManager{},
		updatePlayers:          updatePlayers,
		advertDone:             advertDone,
		tournamentParticipants: make(map[int]*TournamentParticipant),
	}

	s.uids.InitHeap(conf.MaxPlayers)

	// Load server data.
	var err error
	s.music, err = settings.LoadMusic()
	if err != nil {
		return nil, err
	}
	s.characters, err = settings.LoadFile("/characters.txt")
	if err != nil {
		return nil, err
	} else if len(s.characters) == 0 {
		return nil, fmt.Errorf("empty character list")
	}
	areaData, err := settings.LoadAreas()
	if err != nil {
		return nil, err
	}

	s.roles, err = settings.LoadRoles()
	if err != nil {
		return nil, err
	}

	s.backgrounds, err = settings.LoadFile("/backgrounds.txt")
	if err != nil {
		return nil, err
	} else if len(s.backgrounds) == 0 {
		return nil, fmt.Errorf("empty background list")
	}
	var bgBuilder strings.Builder
	bgBuilder.Grow(len("Available backgrounds:\n") + estimateJoinedLen(s.backgrounds))
	bgBuilder.WriteString("Available backgrounds:\n")
	for i, bg := range s.backgrounds {
		if i > 0 {
			bgBuilder.WriteByte('\n')
		}
		bgBuilder.WriteString(bg)
	}
	s.bgListStr = bgBuilder.String()

	// Build O(1) background lookup set for area validation.
	bgSet := make(map[string]struct{}, len(s.backgrounds))
	for _, bg := range s.backgrounds {
		bgSet[bg] = struct{}{}
	}

	s.parrot, err = settings.LoadFile("/parrot.txt")
	if err != nil {
		return nil, err
	} else if len(s.parrot) == 0 {
		return nil, fmt.Errorf("empty parrot list")
	}
	_, err = str2duration.ParseDuration(conf.BanLen)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default_ban_duration: %v", err.Error())
	}

	// Webhook setup: set server name once for all webhook functions.
	webhook.ServerName = conf.Name

	// Discord webhook.
	if conf.WebhookURL != "" {
		s.enableDiscord = true
		webhook.PingRoleID = conf.WebhookPingRoleID
		discord.WebhookURL = conf.WebhookURL
	}

	// Punishment webhook (ban/kick logging).
	if conf.PunishmentWebhookURL != "" {
		webhook.PunishmentWebhookURL = conf.PunishmentWebhookURL
	}

	// Load areas.
	s.areas = make([]*area.Area, 0, len(areaData))
	var areaNameBuilder strings.Builder
	for i, a := range areaData {
		if i > 0 {
			areaNameBuilder.WriteByte('#')
		}
		areaNameBuilder.WriteString(a.Name)
		var evi_mode area.EvidenceMode
		switch strings.ToLower(a.Evi_mode) {
		case "any":
			evi_mode = area.EviAny
		case "cms":
			evi_mode = area.EviCMs
		case "mods":
			evi_mode = area.EviMods
		default:
			logger.LogWarningf("Area %v has an invalid or undefined evidence mode, defaulting to 'cms'.", a.Name)
			evi_mode = area.EviCMs
		}
		if _, validBg := bgSet[a.Bg]; a.Bg == "" || !validBg {
			logger.LogWarningf("Area %v has an invalid or undefined background, defaulting to 'default'.", a.Name)
			a.Bg = "default"
		}
		s.areas = append(s.areas, area.NewArea(a, len(s.characters), conf.BufSize, evi_mode))
	}
	s.areaNames = areaNameBuilder.String()

	// Build O(1) area-index lookup map.
	s.areaIndexMap = make(map[*area.Area]int, len(s.areas))
	for i, a := range s.areas {
		s.areaIndexMap[a] = i
	}

	// Initialize area logging if enabled.
	logger.EnableAreaLogging = conf.EnableAreaLogging
	if logger.EnableAreaLogging {
		logger.LogInfo("Area logging is enabled. Creating area log directories...")
		for _, a := range s.areas {
			if err := logger.CreateAreaLogDirectory(a.Name()); err != nil {
				logger.LogErrorf("Failed to create area log directory for %v: %v", a.Name(), err)
			}
		}
	}

	// Initialize network logging if enabled.
	logger.EnableNetworkLogging = conf.EnableNetworkLogging
	if logger.EnableNetworkLogging {
		logger.LogInfo("Network logging is enabled. All packets will be logged to network.log.")
	}

	if conf.Advertise {
		advert := ms.Advertisement{
			Port:    conf.Port,
			Players: s.players.GetPlayerCount(),
			Name:    conf.Name,
			Desc:    conf.Desc}
		if conf.AdvertiseHostname != "" {
			advert.IP = conf.AdvertiseHostname
		}
		if conf.EnableWS {
			if conf.ReverseProxyMode {
				advert.WSPort = conf.ReverseProxyHTTPPort
			} else {
				advert.WSPort = conf.WSPort
			}
		}
		if conf.EnableWSS {
			if conf.ReverseProxyMode {
				advert.WSSPort = conf.ReverseProxyHTTPSPort
			} else {
				advert.WSSPort = conf.WSSPort
			}
		}
		go ms.Advertise(conf.MSAddr, advert, updatePlayers, advertDone)
	}

	// Propagate to package-level globals so that existing helper functions
	// and command handlers continue to work without modification.
	config = s.config
	encodedServerName = encode(s.config.Name) // cache once; config.Name never changes at runtime
	characters = s.characters
	charactersByName = make(map[string]int, len(s.characters))
	for i, name := range s.characters {
		charactersByName[strings.ToLower(name)] = i
	}
	music = s.music
	backgrounds = s.backgrounds
	parrot = s.parrot
	areas = s.areas
	areaNames = s.areaNames
	bgListStr = s.bgListStr
	areaIndexMap = s.areaIndexMap
	roles = s.roles
	uids = s.uids
	enableDiscord = s.enableDiscord
	tournamentParticipants = s.tournamentParticipants

	// Pre-build the SM packet (sent to every client on join) once at startup
	// so that pktReqAM performs a single write with no allocations.
	smPacket = buildSMPacket(s.areaNames, s.music)
	// Pre-build the SC packet (character list) and SI packet (counts) once at startup
	// so that pktReqChar and pktResCount perform a single write with no per-connection allocations.
	// The SC packet can be very large (thousands of characters), so caching it is a significant win.
	scPacket = buildSCPacket(s.characters)
	siPacket = buildSIPacket(len(s.characters), len(s.areas[0].Evidence()), len(s.music))

	initCommands()
	initAutoMod(conf)
	initCvote(conf)
	// Initialise the goroutine pool if a limit is configured.
	if conf.MaxConnectionGoroutines > 0 {
		connPool = make(chan struct{}, conf.MaxConnectionGoroutines)
	} else {
		connPool = nil
	}
	go startConnTrackerCleanup()
	go startIdleKicker()
	if conf.EnableCasino {
		go startHourlyChipAward()
		go startUnscrambleLoop()
	}
	if conf.EnableNewspaper {
		go startNewspaperLoop()
	}
	return s, nil
}

// InitServer initializes the server and stores it as the package-level singleton.
// It is the legacy entry point used by the main package; callers that need to
// manage the server lifecycle directly should use NewServer instead.
func InitServer(conf *settings.Config) error {
	var err error
	server, err = NewServer(conf)
	return err
}

// StartDiscordBot starts the Discord bot if a token is configured.
// It should be called after InitServer.
func (s *Server) StartDiscordBot() {
	if s.config.BotToken == "" {
		return
	}
	cfg := discordbot.Config{
		Token:     s.config.BotToken,
		GuildID:   s.config.GuildID,
		ModRoleID: s.config.ModRoleID,
	}
	b, err := discordbot.New(cfg, NewServerAdapter())
	if err != nil {
		logger.LogErrorf("Failed to create Discord bot: %v", err)
		return
	}
	if err := b.Start(); err != nil {
		logger.LogErrorf("Failed to start Discord bot: %v", err)
		return
	}
	logger.LogInfo("Discord bot started.")
}

// StartDiscordBot starts the Discord bot on the active server instance.
// Kept for backward compatibility; delegates to server.StartDiscordBot.
func StartDiscordBot() { server.StartDiscordBot() }

// ListenTCP starts the server's TCP listener.
func (s *Server) ListenTCP() {
	listener, err := net.Listen("tcp", config.Addr+":"+strconv.Itoa(config.Port))
	if err != nil {
		FatalError <- err
		return
	}
	logger.LogDebug("TCP listener started.")
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.LogError(err.Error())
		}
		rawAddr := conn.RemoteAddr().String()
		ipid := getIpid(rawAddr)
		if reject, autoban := checkConnRateLimit(ipid); reject {
			logger.LogInfof("Connection from %v rejected (connection rate limit exceeded)", ipid)
			if autoban {
				go autobanFlooder(ipid, "connection flooding")
			}
			conn.Close()
			continue
		}
		if checkGlobalNewIPRateLimit(ipid) {
			logger.LogInfof("Connection from new IP %v rejected (global new IP rate limit exceeded)", ipid)
			conn.Close()
			continue
		}
		// The firewall check may block on a network round-trip to IPHub.
		// Dispatch everything after the fast in-memory checks into its own
		// goroutine so the accept loop is never stalled waiting for the API.
		go acceptTCPConnection(conn, extractIP(rawAddr), ipid)
	}
}

// acceptTCPConnection completes the setup for a single accepted TCP connection.
// It runs in its own goroutine so that an IPHub API call (when the firewall is
// active) never stalls the ListenTCP accept loop.
// HandleClient is called without `go` because this goroutine IS the connection
// goroutine — it blocks for the lifetime of the connection, which is the
// standard one-goroutine-per-connection pattern in Go.
func acceptTCPConnection(conn net.Conn, rawIP, ipid string) {
	// Acquire a pool slot when the goroutine pool is enabled.  This blocks
	// until a slot is free, bounding the total number of active connections
	// that are in the "setup + serve" phase at any moment.
	if connPool != nil {
		connPool <- struct{}{}
		defer func() { <-connPool }()
	}
	if checkFirewallForIP(rawIP, ipid) {
		logger.LogInfof("Connection from %v rejected (VPN/proxy detected by IPHub firewall)", ipid)
		conn.Close()
		return
	}
	recordIPFirstSeen(ipid)
	// Persist the IP and update its last-seen timestamp for all connections
	// (new and returning). The upsert keeps FIRST_SEEN intact for existing rows.
	go func() {
		if err := db.MarkIPKnown(ipid); err != nil {
			logger.LogErrorf("Failed to update known IP %s: %v", ipid, err)
		}
	}()
	if logger.DebugNetwork {
		logger.LogDebugf("Connection received from %v", ipid)
	}
	client := NewClient(conn, ipid)
	client.HandleClient()
}

// ListenTCP starts the TCP listener on the active server instance.
// Kept for backward compatibility; delegates to server.ListenTCP.
func ListenTCP() { server.ListenTCP() }

// ListenWS starts the server's websocket listener.
func (s *Server) ListenWS() {
	listener, err := net.Listen("tcp", config.Addr+":"+strconv.Itoa(config.WSPort))
	if err != nil {
		FatalError <- err
		return
	}
	logger.LogDebug("WS listener started.")
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleWS)
	srv := &http.Server{
		Handler: mux,
	}
	err = srv.Serve(listener)
	if err != http.ErrServerClosed {
		FatalError <- err
	}
}

// ListenWS starts the WebSocket listener on the active server instance.
// Kept for backward compatibility; delegates to server.ListenWS.
func ListenWS() { server.ListenWS() }

// ListenWSS starts the server's secure websocket listener.
// If TLS certificate and key paths are provided, it serves with TLS (direct HTTPS).
// If not provided, it serves plain HTTP (useful when behind a reverse proxy like Cloudflare).
func (s *Server) ListenWSS() {
	listener, err := net.Listen("tcp", config.Addr+":"+strconv.Itoa(config.WSSPort))
	if err != nil {
		FatalError <- err
		return
	}
	logger.LogDebug("WSS listener started.")
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleWS)
	srv := &http.Server{
		Handler: mux,
	}

	// Use TLS if certificate and key paths are provided, otherwise serve plain HTTP
	// (useful when behind a reverse proxy that handles TLS termination)
	if config.TLSCertPath != "" && config.TLSKeyPath != "" {
		logger.LogDebugf("WSS using TLS with cert: %s", config.TLSCertPath)
		err = srv.ServeTLS(listener, config.TLSCertPath, config.TLSKeyPath)
	} else {
		logger.LogDebug("WSS using plain HTTP (expecting reverse proxy for TLS)")
		err = srv.Serve(listener)
	}

	if err != http.ErrServerClosed {
		FatalError <- err
	}
}

// ListenWSS starts the secure WebSocket listener on the active server instance.
// Kept for backward compatibility; delegates to server.ListenWSS.
func ListenWSS() { server.ListenWSS() }

// HandleWS handles a websocket connection.
func HandleWS(w http.ResponseWriter, r *http.Request) {
	rawIP := getRealIP(r)
	ipid := getIpid(rawIP)
	if reject, autoban := checkConnRateLimit(ipid); reject {
		logger.LogInfof("Connection from %v rejected (connection rate limit exceeded)", ipid)
		if autoban {
			go autobanFlooder(ipid, "connection flooding")
		}
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}
	// Check if the IP is banned before consuming a global new-IP rate limit slot.
	// Banned clients that repeatedly reconnect must not exhaust the limit and
	// block legitimate new users from joining.
	if banned, _, err := db.IsBanned(db.IPID, ipid); err != nil {
		logger.LogErrorf("Failed to check IP ban for %v: %v", ipid, err)
	} else if banned {
		c, wsErr := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{config.WebAOAllowedOrigin}})
		if wsErr != nil {
			logger.LogError(wsErr.Error())
			return
		}
		client := NewClient(websocket.NetConn(r.Context(), c, websocket.MessageText), ipid)
		client.CheckBanned(db.IPID)
		return
	}
	if checkGlobalNewIPRateLimit(ipid) {
		logger.LogInfof("Connection from new IP %v rejected (global new IP rate limit exceeded)", ipid)
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}
	if checkFirewallForIP(extractIP(rawIP), ipid) {
		logger.LogInfof("Connection from %v rejected (VPN/proxy detected by IPHub firewall)", ipid)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	recordIPFirstSeen(ipid)
	// Persist the IP and update its last-seen timestamp for all connections
	// (new and returning). The upsert keeps FIRST_SEEN intact for existing rows.
	go func(id string) {
		if err := db.MarkIPKnown(id); err != nil {
			logger.LogErrorf("Failed to update known IP %s: %v", id, err)
		}
	}(ipid)
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{config.WebAOAllowedOrigin}})
	if err != nil {
		logger.LogError(err.Error())
		return
	}
	if logger.DebugNetwork {
		logger.LogDebugf("Connection received from %v", ipid)
	}
	client := NewClient(websocket.NetConn(context.TODO(), c, websocket.MessageText), ipid)
	go client.HandleClient()
}

// CleanupServer closes all connections to the server and closes the database.
func (s *Server) CleanupServer() {
	clients.ForEach(func(client *Client) {
		client.conn.Close()
	})
	db.Close()
	logger.CloseLogFiles()
}

// CleanupServer closes all connections on the active server instance.
// Kept for backward compatibility; delegates to server.CleanupServer.
func CleanupServer() { server.CleanupServer() }

// RequestRestart signals the main process to restart the server.
func RequestRestart() {
	RestartRequest <- struct{}{}
}

// writeToAll sends a message to all connected clients.
func writeToAll(header string, contents ...string) {
	clients.ForEach(func(client *Client) {
		if client.Uid() != -1 {
			client.SendPacket(header, contents...)
		}
	})
}

// writeToArea sends a message to all clients in a given area.
func writeToArea(area *area.Area, header string, contents ...string) {
	clients.ForEach(func(client *Client) {
		if client.Area() == area {
			client.SendPacket(header, contents...)
		}
	})
}

// writeToAreaFrom sends a message to all clients in a given area, skipping
// any recipient that has permanently ignored the sender's IPID.
// If senderIsMod is true the ignore list is bypassed so moderator messages
// always reach every client in the area.
func writeToAreaFrom(senderIPID string, senderIsMod bool, area *area.Area, header string, contents ...string) {
	clients.ForEach(func(client *Client) {
		if client.Area() == area && (senderIsMod || !client.IgnoresIPID(senderIPID)) {
			client.SendPacket(header, contents...)
		}
	})
}

// writeToAllClients writes a packet to all connected clients
func writeToAllClients(header string, contents ...string) {
	clients.ForEach(func(client *Client) {
		client.SendPacket(header, contents...)
	})
}

// addToBuffer writes to an area buffer according to a client's action.
// All client fields are read in one logSnapshot call (one mutex acquisition)
// rather than via individual getters, which would each acquire and release the
// client lock separately.
func addToBuffer(client *Client, action string, message string, audit bool) {
	now := time.Now().UTC().Format("15:04:05")
	snap := client.logSnapshot()
	s := fmt.Sprintf("%v | %v | %v | %v | %v | %v",
		now, action, snap.charName, snap.ipid, snap.oocName, message)
	snap.area.UpdateBuffer(s)

	// Write to area-specific log file if area logging is enabled.
	if logger.EnableAreaLogging {
		logEntry := fmt.Sprintf("[%v] | %v | %v | %v | %v | %v | %v | %v",
			now, action, snap.charName, snap.ipid, snap.hdid, snap.showname, snap.oocName, message)
		logger.WriteAreaLog(snap.area.Name(), logEntry)
	}

	if audit {
		logger.WriteAudit(s)
	}
}

// getAreaIndex returns the index of a given area in the areas slice.
// All areas come from the global slice initialised at startup, so the map
// always contains every valid *Area pointer.  A missing key returns 0,
// which matches the historic fallback behaviour.
func getAreaIndex(a *area.Area) int {
	return areaIndexMap[a]
}

// sendPlayerListToClient sends PR and PU packets for all currently joined players to a new client.
func sendPlayerListToClient(newClient *Client) {
	clients.ForEach(func(c *Client) {
		if c.Uid() == -1 || c == newClient || c.Hidden() {
			return
		}
		uid := strconv.Itoa(c.Uid())
		newClient.SendPacket("PR", uid, "0")
		if c.OOCName() != "" {
			newClient.SendPacket("PU", uid, "0", c.OOCName())
		}
		newClient.SendPacket("PU", uid, "1", c.CurrentCharacter())
		newClient.SendPacket("PU", uid, "2", decode(c.Showname()))
		newClient.SendPacket("PU", uid, "3", strconv.Itoa(getAreaIndex(c.Area())))
	})
}

// broadcastPlayerJoin sends PR and PU packets to all clients when a new player joins.
// Hidden players are not broadcast.
func broadcastPlayerJoin(client *Client) {
	if client.Hidden() {
		return
	}
	uid := strconv.Itoa(client.Uid())
	writeToAll("PR", uid, "0")
	if client.OOCName() != "" {
		writeToAll("PU", uid, "0", client.OOCName())
	}
	writeToAll("PU", uid, "1", client.CurrentCharacter())
	writeToAll("PU", uid, "2", decode(client.Showname()))
	writeToAll("PU", uid, "3", strconv.Itoa(getAreaIndex(client.Area())))
}

// sendPlayerArup sends a player ARUP to all connected clients.
// Visible (non-hidden) player counts are read from each area's pre-maintained counter.
func sendPlayerArup() {
	plCounts := make([]string, 1, 1+len(areas))
	plCounts[0] = "0"
	for _, a := range areas {
		plCounts = append(plCounts, strconv.Itoa(a.VisiblePlayerCount()))
	}
	writeToAll("ARUP", plCounts...)
}

// sendCMArup sends a CM ARUP to all connected clients.
func sendCMArup() {
	returnL := make([]string, 1, 1+len(areas))
	returnL[0] = "2"
	for _, a := range areas {
		cmUIDs := a.CMs()
		if len(cmUIDs) == 0 {
			returnL = append(returnL, "FREE")
			continue
		}
		cms := make([]string, 0, len(cmUIDs))
		for _, u := range cmUIDs {
			c, err := getClientByUid(u)
			if err != nil {
				continue
			}
			cms = append(cms, fmt.Sprintf("%v (%v)", c.CurrentCharacter(), u))
		}
		returnL = append(returnL, strings.Join(cms, ", "))
	}
	writeToAll("ARUP", returnL...)
}

// sendStatusArup sends a status ARUP to all connected clients.
func sendStatusArup() {
	statuses := make([]string, 1, 1+len(areas))
	statuses[0] = "1"
	for _, a := range areas {
		statuses = append(statuses, a.Status().String())
	}
	writeToAll("ARUP", statuses...)
}

// sendLockArup sends a lock ARUP to all connected clients.
func sendLockArup() {
	locks := make([]string, 1, 1+len(areas))
	locks[0] = "3"
	for _, a := range areas {
		locks = append(locks, a.Lock().String())
	}
	writeToAll("ARUP", locks...)
}

// getRole returns the role with the corresponding name, or an error if the role does not exist.
func getRole(name string) (permissions.Role, error) {
	for _, role := range roles {
		if role.Name == name {
			return role, nil
		}
	}
	return permissions.Role{}, fmt.Errorf("role does not exist")
}

// getClientByUid returns the client with the given uid.
func getClientByUid(uid int) (*Client, error) {
	if c := clients.GetClientByUID(uid); c != nil {
		return c, nil
	}
	return nil, fmt.Errorf("client does not exist")
}

// getClientsByIpid returns all clients with the given ipid.
func getClientsByIpid(ipid string) []*Client {
	return clients.GetByIPID(ipid)
}

// sendAreaServerMessage sends a server OOC message to all clients in an area.
func sendAreaServerMessage(area *area.Area, message string) {
	writeToArea(area, "CT", encodedServerName, encode(message), "1")
}

// sendAreaGamblingMessage sends a gambling-result OOC message to all clients
// in an area who have not opted out of gambling broadcasts via /gamble hide.
func sendAreaGamblingMessage(a *area.Area, message string) {
	encoded := encode(message)
	clients.ForEach(func(client *Client) {
		if client.Area() == a && !client.GambleHide() {
			client.SendPacket("CT", encodedServerName, encoded, "1")
		}
	})
}

// sendGlobalServerMessage broadcasts a server OOC message to every joined client.
func sendGlobalServerMessage(message string) {
	writeToAll("CT", encodedServerName, encode(message), "1")
}

// getRealIP extracts the real client IP address from an HTTP request.
// When reverse_proxy_mode is enabled in the config, it checks X-Forwarded-For
// and X-Real-IP headers (for reverse proxy setups like nginx or Cloudflare).
// When reverse_proxy_mode is disabled, it always uses RemoteAddr directly.
//
// Security Note: Proxy headers (X-Forwarded-For, X-Real-IP) are only trusted when
// reverse_proxy_mode is explicitly enabled. This prevents IP spoofing when the server
// is directly exposed to the internet without a reverse proxy.
func getRealIP(r *http.Request) string {
	// Only trust proxy headers if reverse_proxy_mode is enabled in config
	if config.ReverseProxyMode {
		// Check X-Forwarded-For header first (may contain multiple IPs)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
			// The first IP is the original client
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// Check X-Real-IP header (single IP from reverse proxy)
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}

	// Use RemoteAddr if reverse_proxy_mode is disabled or no proxy headers are present
	return r.RemoteAddr
}

// Returns the IPID for a given IP address.
func getIpid(s string) string {
	// For privacy and ease of use, AO servers traditionally use a hashed version of a client's IP address to identify a client.
	// Athena uses the MD5 hash of the IP address, encoded in base64.
	ip := extractIP(s)
	hash := md5.Sum([]byte(ip))
	ipid := base64.StdEncoding.EncodeToString(hash[:])
	return ipid[:len(ipid)-2] // Removes the trailing padding.
}

// extractIP returns the plain IP address from a "host:port" string (or plain IP).
// It mirrors the extraction logic inside getIpid so callers can obtain the raw IP
// without re-parsing the same string.
func extractIP(s string) string {
	ip, _, err := net.SplitHostPort(s)
	if err != nil {
		return s
	}
	return ip
}

// getParrotMsg returns a random string from the server's parrot list.
// parrot is validated to be non-empty in InitServer, so no bounds check is required here.
func getParrotMsg() string {
	return parrot[rand.Intn(len(parrot))]
}

// checkConnRateLimit checks whether the given ipid has exceeded the connection rate limit.
// It records every connection attempt (including rejected ones) in the sliding window.
// Returns rejected=true if the connection should be rejected.
// Returns autoban=true when conn_flood_autoban is enabled and the rejection count hits
// the threshold exactly this call — the caller should ban the IP asynchronously.
// Both values are determined under a single lock acquisition.
func checkConnRateLimit(ipid string) (rejected, autoban bool) {
	if config.ConnRateLimit <= 0 {
		return false, false
	}

	connTracker.mu.Lock()
	defer connTracker.mu.Unlock()

	now := time.Now()
	window := time.Duration(config.ConnRateLimitWindow) * time.Second
	cutoff := now.Add(-window)

	// Prune timestamps outside the current window (zero-allocation pivot trim;
	// timestamps are always appended in order so expired entries are at the front).
	times := connTracker.timestamps[ipid]
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	if i == len(times) {
		times = nil
	} else if i > 0 {
		times = times[i:]
	}

	// Record this attempt regardless of outcome, so floods are always counted.
	times = append(times, now)
	connTracker.timestamps[ipid] = times

	if len(times) <= config.ConnRateLimit {
		return false, false
	}

	// Connection is rejected. Check whether the auto-ban threshold has been reached.
	// Use == (not >=) so the autoban goroutine is spawned exactly once per flood.
	if config.ConnFloodAutoban && config.ConnFloodAutobanThreshold > 0 {
		connTracker.rejections[ipid]++
		autoban = connTracker.rejections[ipid] == config.ConnFloodAutobanThreshold
	}
	return true, autoban
}

// forgetIP removes an IPID from the in-memory first-seen tracker and from the
// KNOWN_IPS database table. This is called when an IP is banned so that, once the
// ban expires, the IP will be treated as new again (subject to cooldowns and rate limits).
func forgetIP(ipid string) {
	ipFirstSeenTracker.mu.Lock()
	delete(ipFirstSeenTracker.times, ipid)
	ipFirstSeenTracker.mu.Unlock()

	connTracker.mu.Lock()
	delete(connTracker.rejections, ipid)
	connTracker.mu.Unlock()

	go func() {
		if err := db.RemoveKnownIP(ipid); err != nil {
			logger.LogErrorf("Failed to remove banned IP %s from known IPs: %v", ipid, err)
		}
	}()
}

// resetKnownIPTracker clears the in-memory first-seen tracker entirely.
// Called after a full database purge so that all IPIDs are treated as new on
// their next connection.
func resetKnownIPTracker() {
	ipFirstSeenTracker.mu.Lock()
	ipFirstSeenTracker.times = make(map[string]time.Time)
	ipFirstSeenTracker.mu.Unlock()
}

// addTormentedIP adds an IPID to the in-memory torment set and persists it to the database.
func addTormentedIP(ipid string) {
	tormentedIPIDs.mu.Lock()
	tormentedIPIDs.set[ipid] = struct{}{}
	tormentedIPIDs.mu.Unlock()

	go func() {
		if err := db.AddTormentedIP(ipid); err != nil {
			logger.LogErrorf("Failed to persist tormented IP %s: %v", ipid, err)
		}
	}()
}

// removeTormentedIP removes an IPID from the in-memory torment set and from the database.
func removeTormentedIP(ipid string) {
	tormentedIPIDs.mu.Lock()
	delete(tormentedIPIDs.set, ipid)
	tormentedIPIDs.mu.Unlock()

	go func() {
		if err := db.RemoveTormentedIP(ipid); err != nil {
			logger.LogErrorf("Failed to remove tormented IP %s from database: %v", ipid, err)
		}
	}()
}

// isIPIDTormented returns true if the IPID is in the tormented set.
func isIPIDTormented(ipid string) bool {
	tormentedIPIDs.mu.RLock()
	_, ok := tormentedIPIDs.set[ipid]
	tormentedIPIDs.mu.RUnlock()
	return ok
}

// autobanFlooder bans an IP for the given flood reason using the configured default ban
// duration. reason should be a short description, e.g. "packet flooding".
// No-op if the IP is already banned or the ban cannot be recorded.
func autobanFlooder(ipid, reason string) {
	banned, _, err := db.IsBanned(db.IPID, ipid)
	if err != nil || banned {
		return
	}
	dur, err := str2duration.ParseDuration(config.BanLen)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	_, err = db.AddBan(ipid, "", now.Unix(), now.Add(dur).Unix(), "Automatic ban: "+reason, "Server")
	if err != nil {
		logger.LogErrorf("Failed to auto-ban %v (%s): %v", ipid, reason, err)
		return
	}
	forgetIP(ipid)
	logger.LogInfof("Auto-banned %v for %s", ipid, reason)
}

// autoBanPacketFlooder bans an IP that exceeded the raw packet rate limit.
// This is only triggered by raw packet flooding (sending hundreds of packets per second),
// which is characteristic of bots/DDoS tools. IC/OOC/music message rate limit violations
// result in a kick, not a ban (see KickForRateLimit).
// If the IP is already banned, no additional ban is added.
func autoBanPacketFlooder(ipid string) {
	autobanFlooder(ipid, "packet flooding")
}

// startConnTrackerCleanup periodically removes stale entries from the connection tracker
// to prevent unbounded memory growth from unique IPs that no longer connect.
// This goroutine runs for the lifetime of the server process; a graceful stop is not
// required because the process exits when the server stops.
func startConnTrackerCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		if config == nil || config.ConnRateLimitWindow <= 0 {
			continue
		}
		window := time.Duration(config.ConnRateLimitWindow) * time.Second
		cutoff := time.Now().Add(-window)
		connTracker.mu.Lock()
		for ipid, times := range connTracker.timestamps {
			i := 0
			for i < len(times) && !times[i].After(cutoff) {
				i++
			}
			if i == len(times) {
				delete(connTracker.timestamps, ipid)
				// Also clear rejection counts for IPs with no recent connection attempts,
				// so short-lived flooders that stop connecting don't accumulate forever.
				delete(connTracker.rejections, ipid)
			} else if i > 0 {
				connTracker.timestamps[ipid] = times[i:]
			}
		}
		connTracker.mu.Unlock()

		// Clean up stale modcall entries.
		if config.ModcallCooldown > 0 {
			cooldown := time.Duration(config.ModcallCooldown) * time.Second
			modcallCutoff := time.Now().Add(-cooldown)
			ipModcallTracker.mu.Lock()
			for ipid, t := range ipModcallTracker.times {
				if !t.After(modcallCutoff) {
					delete(ipModcallTracker.times, ipid)
				}
			}
			ipModcallTracker.mu.Unlock()
		}

		// Clean up stale OOC rate limit entries.
		if config.OOCRateLimitWindow > 0 {
			oocWindow := time.Duration(config.OOCRateLimitWindow) * time.Second
			oocCutoff := time.Now().Add(-oocWindow)
			ipOOCTracker.mu.Lock()
			for ipid, times := range ipOOCTracker.timestamps {
				i := 0
				for i < len(times) && !times[i].After(oocCutoff) {
					i++
				}
				if i == len(times) {
					delete(ipOOCTracker.timestamps, ipid)
				} else if i > 0 {
					ipOOCTracker.timestamps[ipid] = times[i:]
				}
			}
			ipOOCTracker.mu.Unlock()
		}

		// Clean up stale ping rate limit entries.
		if config.PingRateLimitWindow > 0 {
			pingWindow := time.Duration(config.PingRateLimitWindow) * time.Second
			pingCutoff := time.Now().Add(-pingWindow)
			ipPingTracker.mu.Lock()
			for ipid, times := range ipPingTracker.timestamps {
				i := 0
				for i < len(times) && !times[i].After(pingCutoff) {
					i++
				}
				if i == len(times) {
					delete(ipPingTracker.timestamps, ipid)
				} else if i > 0 {
					ipPingTracker.timestamps[ipid] = times[i:]
				}
			}
			ipPingTracker.mu.Unlock()
		}

		// Clean up stale global new-IP rate limit entries.
		if config.GlobalNewIPRateLimitWindow > 0 {
			globalWindow := time.Duration(config.GlobalNewIPRateLimitWindow) * time.Second
			globalCutoff := time.Now().Add(-globalWindow)
			globalNewIPTracker.mu.Lock()
			times := globalNewIPTracker.timestamps
			i := 0
			for i < len(times) && !times[i].After(globalCutoff) {
				i++
			}
			if i == len(times) {
				globalNewIPTracker.timestamps = nil
			} else if i > 0 {
				globalNewIPTracker.timestamps = times[i:]
			}
			globalNewIPTracker.mu.Unlock()
		}
	}
}

// checkIPModcallCooldown checks if the given IPID is within the modcall cooldown period.
// Unlike the per-client check, this persists across connections, preventing bypass via reconnection.
// Returns true (and remaining seconds, rounded up) if the IPID must wait, false otherwise.
func checkIPModcallCooldown(ipid string) (bool, int) {
	if config.ModcallCooldown <= 0 {
		return false, 0
	}
	ipModcallTracker.mu.Lock()
	defer ipModcallTracker.mu.Unlock()
	last, exists := ipModcallTracker.times[ipid]
	if !exists {
		return false, 0
	}
	elapsed := time.Since(last)
	cooldown := time.Duration(config.ModcallCooldown) * time.Second
	if elapsed < cooldown {
		remaining := int(math.Ceil((cooldown - elapsed).Seconds()))
		return true, remaining
	}
	return false, 0
}

// setIPModcallTime records the current time as the last modcall time for the given IPID.
func setIPModcallTime(ipid string) {
	ipModcallTracker.mu.Lock()
	ipModcallTracker.times[ipid] = time.Now()
	ipModcallTracker.mu.Unlock()
}

// checkIPOOCRateLimit checks if the given IPID has exceeded the OOC message rate limit.
// Unlike the per-client check, this persists across connections, preventing bypass via reconnection.
// Returns true if the packet should be dropped, false if allowed.
func checkIPOOCRateLimit(ipid string) bool {
	if config.OOCRateLimit <= 0 {
		return false
	}
	ipOOCTracker.mu.Lock()
	defer ipOOCTracker.mu.Unlock()
	now := time.Now()
	window := time.Duration(config.OOCRateLimitWindow) * time.Second
	cutoff := now.Add(-window)
	times := ipOOCTracker.timestamps[ipid]
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	if i == len(times) {
		times = nil
	} else if i > 0 {
		times = times[i:]
	}
	if len(times) >= config.OOCRateLimit {
		ipOOCTracker.timestamps[ipid] = times
		return true
	}
	ipOOCTracker.timestamps[ipid] = append(times, now)
	return false
}

// checkIPPingRateLimit checks if the given IPID has exceeded the ping (CH) rate limit.
// This persists across connections, preventing bypass via reconnection.
// Returns true if the ping should be dropped, false if allowed.
func checkIPPingRateLimit(ipid string) bool {
	if config.PingRateLimit <= 0 {
		return false
	}
	ipPingTracker.mu.Lock()
	defer ipPingTracker.mu.Unlock()
	now := time.Now()
	window := time.Duration(config.PingRateLimitWindow) * time.Second
	cutoff := now.Add(-window)
	times := ipPingTracker.timestamps[ipid]
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	if i == len(times) {
		times = nil
	} else if i > 0 {
		times = times[i:]
	}
	if len(times) >= config.PingRateLimit {
		ipPingTracker.timestamps[ipid] = times
		return true
	}
	ipPingTracker.timestamps[ipid] = append(times, now)
	return false
}

// recordIPFirstSeen records the first time an IPID connects to this server session.
// If the IPID has already been seen this call is a no-op for the in-memory tracker.
// Entries are normally kept indefinitely so that returning players are never treated
// as new again; however, banned IPs are explicitly removed via forgetIP so that a
// reconnect after a ban expiry is subject to new-connection cooldowns.
// When a genuinely new IPID is recorded, a timestamp is also pushed to globalNewIPTracker
// for global new-connection rate limiting.
// NOTE: the caller is responsible for persisting the IP to the database (db.MarkIPKnown).
func recordIPFirstSeen(ipid string) {
	ipFirstSeenTracker.mu.Lock()
	if _, exists := ipFirstSeenTracker.times[ipid]; exists {
		ipFirstSeenTracker.mu.Unlock()
		return
	}
	ipFirstSeenTracker.times[ipid] = time.Now()
	ipFirstSeenTracker.mu.Unlock()

	// Inform the global new-IP rate limiter that a new unique IP has arrived.
	// globalNewIPTracker.mu is acquired only after ipFirstSeenTracker.mu is released
	// to keep lock-acquisition order consistent and avoid potential deadlocks.
	globalNewIPTracker.mu.Lock()
	globalNewIPTracker.timestamps = append(globalNewIPTracker.timestamps, time.Now())
	globalNewIPTracker.mu.Unlock()
}

// checkNewIPIDOOCCooldown checks whether a newly-seen IPID is still within the OOC chat cooldown.
// Returns true (and remaining seconds, rounded up) if the IPID must wait, false otherwise.
func checkNewIPIDOOCCooldown(ipid string) (bool, int) {
	if config.NewIPIDOOCCooldown <= 0 {
		return false, 0
	}
	ipFirstSeenTracker.mu.Lock()
	defer ipFirstSeenTracker.mu.Unlock()
	t, exists := ipFirstSeenTracker.times[ipid]
	if !exists {
		return false, 0
	}
	cooldown := time.Duration(config.NewIPIDOOCCooldown) * time.Second
	elapsed := time.Since(t)
	if elapsed < cooldown {
		remaining := int(math.Ceil((cooldown - elapsed).Seconds()))
		return true, remaining
	}
	return false, 0
}

// checkNewIPIDModcallCooldown checks whether a newly-seen IPID is still within the modcall cooldown.
// Returns true (and remaining seconds, rounded up) if the IPID must wait, false otherwise.
func checkNewIPIDModcallCooldown(ipid string) (bool, int) {
	if config.NewIPIDModcallCooldown <= 0 {
		return false, 0
	}
	ipFirstSeenTracker.mu.Lock()
	defer ipFirstSeenTracker.mu.Unlock()
	t, exists := ipFirstSeenTracker.times[ipid]
	if !exists {
		return false, 0
	}
	cooldown := time.Duration(config.NewIPIDModcallCooldown) * time.Second
	elapsed := time.Since(t)
	if elapsed < cooldown {
		remaining := int(math.Ceil((cooldown - elapsed).Seconds()))
		return true, remaining
	}
	return false, 0
}


// checkGlobalNewIPRateLimit checks whether too many new unique IPs have arrived within
// the configured time window. If so, connections from IPs that have never been seen before
// are rejected to protect the server against distributed floods using many unique IPs.
// Already-known IPs (those with an entry in ipFirstSeenTracker) are always permitted.
// Also enforces lockdown mode: while lockdown is active, all new (unseen) IPs are rejected.
// Returns true if the connection should be rejected.
func checkGlobalNewIPRateLimit(ipid string) bool {
	// Known IPs are always allowed through; only new, unseen IPs can trigger this limit.
	ipFirstSeenTracker.mu.Lock()
	_, known := ipFirstSeenTracker.times[ipid]
	ipFirstSeenTracker.mu.Unlock()
	if known {
		return false
	}

	// Lockdown mode: block all new IPIDs regardless of the configured rate limit.
	if serverLockdown.Load() {
		return true
	}

	if config.GlobalNewIPRateLimit <= 0 {
		return false
	}

	globalNewIPTracker.mu.Lock()
	defer globalNewIPTracker.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(config.GlobalNewIPRateLimitWindow) * time.Second)

	// Prune timestamps outside the current window (zero-allocation pivot trim;
	// timestamps are always appended in order so expired entries are at the front).
	times := globalNewIPTracker.timestamps
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	if i == len(times) {
		times = nil
	} else if i > 0 {
		times = times[i:]
	}
	globalNewIPTracker.timestamps = times

	return len(times) >= config.GlobalNewIPRateLimit
}

// estimateJoinedLen returns the total byte length of all strings in ss joined by newlines.
// Used to pre-size a strings.Builder, avoiding intermediate reallocations.
func estimateJoinedLen(ss []string) int {
	if len(ss) == 0 {
		return 0
	}
	n := len(ss) - 1 // separators
	for _, s := range ss {
		n += len(s)
	}
	return n
}

// buildSMPacket constructs the full SM#<areas>#<music>#% packet string that is
// sent verbatim to every client on join. Building it once at startup avoids a
// strings.Join allocation on every connection.
func buildSMPacket(areaNamesStr string, musicList []string) string {
	// SM# + areaNames + # + music[0] + # + ... + music[n-1] + #%
	// Add 8 bytes per entry as headroom for encoding expansion (worst case: '%' → "<percent>").
	size := 3 + len(areaNamesStr) + 1 + estimateJoinedLen(musicList) + len(musicList)*8 + 2
	var b strings.Builder
	b.Grow(size)
	b.WriteString("SM#")
	b.WriteString(areaNamesStr)
	for _, m := range musicList {
		b.WriteByte('#')
		encoder.WriteString(&b, m) //nolint:errcheck // strings.Builder.Write never returns an error
	}
	b.WriteString("#%")
	return b.String()
}

// buildSCPacket constructs the full SC#<char0>#<char1>#...#% packet string that
// is sent verbatim to every client during the join handshake (RC request).
// With thousands of character names this can be tens of kilobytes; building it
// once at startup eliminates the largest per-connection allocation.
func buildSCPacket(chars []string) string {
	// SC + # + char[0] + # + ... + char[n-1] + #%
	size := 2 + estimateJoinedLen(chars) + 2
	var b strings.Builder
	b.Grow(size)
	b.WriteString("SC")
	for _, c := range chars {
		b.WriteByte('#')
		b.WriteString(c)
	}
	b.WriteString("#%")
	return b.String()
}

// buildSIPacket constructs the SI#<charCount>#<evidCount>#<musicCount>#% packet
// string. The counts are fixed at startup so this never needs rebuilding.
func buildSIPacket(charCount, evidCount, musicCount int) string {
	return "SI#" + strconv.Itoa(charCount) + "#" + strconv.Itoa(evidCount) + "#" + strconv.Itoa(musicCount) + "#%"
}

// hourlyChipMsg is the notification sent when a player earns exactly 1 chip from the
// hourly ticker.  Defined as a constant to avoid a fmt.Sprintf allocation every tick.
const hourlyChipMsg = "💰 You earned 1 chip for being online! Balance updated."

// startHourlyChipAward runs in the background and awards 1 chip to every connected
// player for each hour of online time they have accumulated during the current session.
// This ensures players receive their hourly chip without needing to disconnect first.
// Awards are tracked per-client in sessionChipsAwarded so the disconnect handler does
// not double-count chips that have already been granted.
//
// EnsureChipBalance is intentionally not called here: chip rows are seeded at connect
// time (pktReqDone), so by the time a player has been online for a full hour the row
// is guaranteed to exist.
func startHourlyChipAward() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if config == nil || !config.EnableCasino {
			continue
		}
		// Capture now once so all per-client calculations use the same instant,
		// avoiding N repeated time.Now() syscalls inside the loop.
		now := time.Now()
		clients.ForEach(func(client *Client) {
			connAt := client.ConnectedAt()
			if connAt.IsZero() {
				return
			}
			sessionHours := int64(now.Sub(connAt).Seconds()) / secondsPerHour
			toAward := sessionHours - client.SessionChipsAwarded()
			if toAward <= 0 {
				return
			}
			ipid := client.Ipid()
			// Apply any passive income bonus passes the player has purchased.
			// chipsPerHour = 1 (base) + any bonuses from passive income upgrades.
			chipsPerHour := int64(1) + getPlayerHourlyBonus(ipid)
			chipsToAward := toAward * chipsPerHour
			if _, err := db.AddChips(ipid, chipsToAward); err != nil {
				logger.LogErrorf("startHourlyChipAward: AddChips failed for %v: %v", ipid, err)
				return
			}
			// Track hours credited (not chips) so the next tick knows where to resume.
			client.AddSessionChipsAwarded(toAward)
			var msg string
			if chipsToAward == 1 {
				msg = hourlyChipMsg
			} else if chipsPerHour > 1 {
				msg = fmt.Sprintf("💰 You earned %d chips for being online (%d/hr with income passes)! Balance updated.", chipsToAward, chipsPerHour)
			} else {
				msg = fmt.Sprintf("💰 You earned %d chips for being online! Balance updated.", chipsToAward)
			}
			client.SendServerMessage(msg)
		})
	}
}

// startIdleKicker runs in the background and disconnects fully-joined clients
// that have not sent any packet within the configured idle_kick_duration window.
// This defends against "ghost connection" floods where bots complete the AO2
// handshake and then hold slots open indefinitely without ever doing anything.
// Only clients with a UID (i.e. fully joined) are subject to idle kicks;
// unjoined clients are handled by the per-connection timeout() goroutine.
// Does nothing when IdleKickDuration is 0 (disabled).
func startIdleKicker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if config == nil || config.IdleKickDuration <= 0 {
			continue
		}
		cutoff := time.Now().Add(-time.Duration(config.IdleKickDuration) * time.Second)
		clients.ForEach(func(client *Client) {
			if client.Uid() == -1 {
				return // unjoined clients are handled by timeout()
			}
			client.mu.Lock()
			last := client.lastPacketTime
			client.mu.Unlock()
			if last.IsZero() || last.Before(cutoff) {
				client.SendServerMessage("You have been disconnected for being idle too long.")
				client.conn.Close()
			}
		})
	}
}

// checkAutoLockdown evaluates the server's current player count against
// AutoLockdownThreshold and engages or releases lockdown mode accordingly.
// It is called whenever a player joins or leaves. Does nothing when
// AutoLockdownThreshold is 0 (disabled) or MaxPlayers is 0.
func checkAutoLockdown() {
	if config == nil || config.AutoLockdownThreshold <= 0 || config.MaxPlayers <= 0 {
		return
	}
	// Use cross-multiplication to avoid integer division truncation:
	// playerCount * 100 >= threshold * maxPlayers  ⟺  pct >= threshold
	playerCount := players.GetPlayerCount()
	atOrAbove := playerCount*100 >= config.AutoLockdownThreshold*config.MaxPlayers
	if atOrAbove && !serverLockdown.Load() {
		serverLockdown.Store(true)
		writeToAll("CT", "OOC", "🔒 Server lockdown automatically engaged (capacity threshold reached). New unknown connections are restricted.", "1")
		logger.LogInfof("Auto-lockdown engaged (%d/%d players)", playerCount, config.MaxPlayers)
	} else if !atOrAbove && serverLockdown.Load() {
		serverLockdown.Store(false)
		writeToAll("CT", "OOC", "🔓 Server lockdown automatically lifted (capacity back within threshold).", "1")
		logger.LogInfof("Auto-lockdown lifted (%d/%d players)", playerCount, config.MaxPlayers)
	}
}
