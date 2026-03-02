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
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	discordbot "github.com/MangosArentLiterature/Athena/internal/discord/bot"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/ms"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/playercount"
	"github.com/MangosArentLiterature/Athena/internal/settings"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
	"github.com/MangosArentLiterature/Athena/internal/uidmanager"
	"github.com/MangosArentLiterature/Athena/internal/webhook"
	"github.com/ecnepsnai/discord"
	"github.com/xhit/go-str2duration/v2"
	"nhooyr.io/websocket"
)

const version = "v1.0.2"

var (
	config                                 *settings.Config
	characters, music, backgrounds, parrot []string
	areas                                  []*area.Area
	areaNames                              string
	areaIndexMap                           map[*area.Area]int // pre-computed index lookup for O(1) getAreaIndex
	roles                                  []permissions.Role
	uids                                   uidmanager.UidManager
	players                                playercount.PlayerCount
	enableDiscord                          bool
	clients                                ClientList = ClientList{list: make(map[*Client]struct{})}
	updatePlayers                                     = make(chan int)      // Updates the advertiser's player count.
	advertDone                                        = make(chan struct{}) // Signals the advertiser to stop.
	FatalError                                        = make(chan error)    // Signals that the server should stop after a fatal error.

	// connTracker tracks connection attempts per IP for connection-rate limiting.
	connTracker = struct {
		mu         sync.Mutex
		timestamps map[string][]time.Time // ipid -> connection attempt timestamps
	}{
		timestamps: make(map[string][]time.Time),
	}

	// ipModcallTracker tracks the last modcall time per IP across connections.
	ipModcallTracker = struct {
		mu    sync.Mutex
		times map[string]time.Time // ipid -> last modcall time
	}{
		times: make(map[string]time.Time),
	}

	// ipFirstJoinTracker records the first time each IPID completed joining the server.
	// This ensures the modcall join-wait cannot be bypassed by reconnecting.
	ipFirstJoinTracker = struct {
		mu    sync.Mutex
		times map[string]time.Time // ipid -> first join time
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
	areaIndexMap           map[*area.Area]int
	roles                  []permissions.Role
	uids                   uidmanager.UidManager
	players                playercount.PlayerCount
	enableDiscord          bool
	clients                ClientList
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

	s := &Server{
		config:                 conf,
		clients:                ClientList{list: make(map[*Client]struct{})},
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
		if a.Bg == "" || !sliceutil.ContainsString(s.backgrounds, a.Bg) {
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
	characters = s.characters
	music = s.music
	backgrounds = s.backgrounds
	parrot = s.parrot
	areas = s.areas
	areaNames = s.areaNames
	areaIndexMap = s.areaIndexMap
	roles = s.roles
	uids = s.uids
	enableDiscord = s.enableDiscord
	tournamentParticipants = s.tournamentParticipants

	initCommands()
	go startConnTrackerCleanup()
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
		ipid := getIpid(conn.RemoteAddr().String())
		if checkConnRateLimit(ipid) {
			logger.LogInfof("Connection from %v rejected (connection rate limit exceeded)", ipid)
			if config.ConnFloodAutoban {
				autoBanFlooder(ipid)
			}
			conn.Close()
			continue
		}
		if logger.DebugNetwork {
			logger.LogDebugf("Connection recieved from %v", ipid)
		}
		client := NewClient(conn, ipid)
		go client.HandleClient()
	}
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
	ipid := getIpid(getRealIP(r))
	if checkConnRateLimit(ipid) {
		logger.LogInfof("Connection from %v rejected (connection rate limit exceeded)", ipid)
		if config.ConnFloodAutoban {
			autoBanFlooder(ipid)
		}
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
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
	for client := range clients.GetAllClients() {
		client.conn.Close()
	}
	db.Close()
}

// CleanupServer closes all connections on the active server instance.
// Kept for backward compatibility; delegates to server.CleanupServer.
func CleanupServer() { server.CleanupServer() }

// writeToAll sends a message to all connected clients.
func writeToAll(header string, contents ...string) {
	for client := range clients.GetAllClients() {
		if client.Uid() == -1 {
			continue
		}
		client.SendPacket(header, contents...)
	}
}

// writeToArea sends a message to all clients in a given area.
func writeToArea(area *area.Area, header string, contents ...string) {
	for client := range clients.GetAllClients() {
		if client.Area() == area {
			client.SendPacket(header, contents...)
		}
	}
}

// writeToAllClients writes a packet to all connected clients
func writeToAllClients(header string, contents ...string) {
	for client := range clients.GetAllClients() {
		client.SendPacket(header, contents...)
	}
}

// addToBuffer writes to an area buffer according to a client's action.
func addToBuffer(client *Client, action string, message string, audit bool) {
	now := time.Now().UTC().Format("15:04:05")
	s := fmt.Sprintf("%v | %v | %v | %v | %v | %v",
		now, action, client.CurrentCharacter(), client.Ipid(), client.OOCName(), message)
	client.Area().UpdateBuffer(s)

	// Write to area-specific log file if area logging is enabled
	if logger.EnableAreaLogging {
		logEntry := fmt.Sprintf("[%v] | %v | %v | %v | %v | %v | %v | %v",
			now,
			action,
			client.CurrentCharacter(),
			client.Ipid(),
			client.Hdid(),
			client.Showname(),
			client.OOCName(),
			message)
		logger.WriteAreaLog(client.Area().Name(), logEntry)
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
	for c := range clients.GetAllClients() {
		if c.Uid() == -1 || c == newClient {
			continue
		}
		uid := strconv.Itoa(c.Uid())
		newClient.SendPacket("PR", uid, "0")
		if c.OOCName() != "" {
			newClient.SendPacket("PU", uid, "0", c.OOCName())
		}
		newClient.SendPacket("PU", uid, "1", c.CurrentCharacter())
		newClient.SendPacket("PU", uid, "2", decode(c.Showname()))
		newClient.SendPacket("PU", uid, "3", strconv.Itoa(getAreaIndex(c.Area())))
	}
}

// broadcastPlayerJoin sends PR and PU packets to all clients when a new player joins.
func broadcastPlayerJoin(client *Client) {
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
func sendPlayerArup() {
	plCounts := make([]string, 1, 1+len(areas))
	plCounts[0] = "0"
	for _, a := range areas {
		plCounts = append(plCounts, strconv.Itoa(a.PlayerCount()))
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
	var returnlist []*Client
	for c := range clients.GetAllClients() {
		if c.Ipid() == ipid {
			returnlist = append(returnlist, c)
		}
	}
	return returnlist
}

// sendAreaServerMessage sends a server OOC message to all clients in an area.
func sendAreaServerMessage(area *area.Area, message string) {
	writeToArea(area, "CT", encode(config.Name), encode(message), "1")
}

// sendGlobalServerMessage broadcasts a server OOC message to every joined client.
func sendGlobalServerMessage(message string) {
	writeToAll("CT", encode(config.Name), encode(message), "1")
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

	// Extract just the IP address, removing the port if present
	// Use net.SplitHostPort which correctly handles both IPv4 and IPv6 addresses
	ip, _, err := net.SplitHostPort(s)
	if err != nil {
		// If there's an error, the input doesn't have a port, so use it as-is
		ip = s
	}

	hash := md5.Sum([]byte(ip))
	ipid := base64.StdEncoding.EncodeToString(hash[:])
	return ipid[:len(ipid)-2] // Removes the trailing padding.
}

// getParrotMsg returns a random string from the server's parrot list.
// parrot is validated to be non-empty in InitServer, so no bounds check is required here.
func getParrotMsg() string {
	return parrot[rand.Intn(len(parrot))]
}

// checkConnRateLimit checks whether the given ipid has exceeded the connection rate limit.
// It records every connection attempt (including rejected ones) in the sliding window.
// Returns true if the connection should be rejected.
func checkConnRateLimit(ipid string) bool {
	if config.ConnRateLimit <= 0 {
		return false
	}

	connTracker.mu.Lock()
	defer connTracker.mu.Unlock()

	now := time.Now()
	window := time.Duration(config.ConnRateLimitWindow) * time.Second
	cutoff := now.Add(-window)

	// Prune timestamps outside the current window.
	times := connTracker.timestamps[ipid]
	valid := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Record this attempt regardless of outcome, so floods are always counted.
	valid = append(valid, now)
	connTracker.timestamps[ipid] = valid

	return len(valid) > config.ConnRateLimit
}

// autoBanFlooder adds a temporary ban for an IP that has exceeded the connection rate limit.
// If the IP is already banned, no additional ban is added.
func autoBanFlooder(ipid string) {
	banned, _, err := db.IsBanned(db.IPID, ipid)
	if err != nil || banned {
		return
	}
	dur, err := str2duration.ParseDuration(config.BanLen)
	if err != nil {
		return
	}
	expiry := time.Now().UTC().Add(dur).Unix()
	_, err = db.AddBan(ipid, "", time.Now().UTC().Unix(), expiry, "Automatic ban: connection flooding", "Server")
	if err != nil {
		logger.LogErrorf("Failed to auto-ban flooder %v: %v", ipid, err)
		return
	}
	logger.LogInfof("Auto-banned %v for connection flooding", ipid)
}

// autoBanSpammer adds a temporary ban for an IP that has exceeded the packet rate limit.
// If the IP is already banned, no additional ban is added.
func autoBanSpammer(ipid string) {
	banned, _, err := db.IsBanned(db.IPID, ipid)
	if err != nil || banned {
		return
	}
	dur, err := str2duration.ParseDuration(config.BanLen)
	if err != nil {
		return
	}
	expiry := time.Now().UTC().Add(dur).Unix()
	_, err = db.AddBan(ipid, "", time.Now().UTC().Unix(), expiry, "Automatic ban: packet flooding", "Server")
	if err != nil {
		logger.LogErrorf("Failed to auto-ban packet spammer %v: %v", ipid, err)
		return
	}
	logger.LogInfof("Auto-banned %v for packet flooding", ipid)
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
			valid := make([]time.Time, 0, len(times))
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(connTracker.timestamps, ipid)
			} else {
				connTracker.timestamps[ipid] = valid
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

		// Clean up first-join entries that are older than the modcall join-wait period.
		// Once 60 seconds have elapsed the entry serves no purpose and can be discarded.
		// !t.After(joinCutoff) is equivalent to t <= joinCutoff (before or equal).
		{
			const modcallJoinWait = 60 * time.Second
			joinCutoff := time.Now().Add(-modcallJoinWait)
			ipFirstJoinTracker.mu.Lock()
			for ipid, t := range ipFirstJoinTracker.times {
				if !t.After(joinCutoff) {
					delete(ipFirstJoinTracker.times, ipid)
				}
			}
			ipFirstJoinTracker.mu.Unlock()
		}

		// Clean up stale OOC rate limit entries.
		if config.OOCRateLimitWindow > 0 {
			oocWindow := time.Duration(config.OOCRateLimitWindow) * time.Second
			oocCutoff := time.Now().Add(-oocWindow)
			ipOOCTracker.mu.Lock()
			for ipid, times := range ipOOCTracker.timestamps {
				valid := make([]time.Time, 0, len(times))
				for _, t := range times {
					if t.After(oocCutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(ipOOCTracker.timestamps, ipid)
				} else {
					ipOOCTracker.timestamps[ipid] = valid
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
				valid := make([]time.Time, 0, len(times))
				for _, t := range times {
					if t.After(pingCutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(ipPingTracker.timestamps, ipid)
				} else {
					ipPingTracker.timestamps[ipid] = valid
				}
			}
			ipPingTracker.mu.Unlock()
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

// recordIPFirstJoin stores the first time an IPID completed joining the server.
// Subsequent calls for the same IPID are no-ops, preserving the original join time.
func recordIPFirstJoin(ipid string) {
	ipFirstJoinTracker.mu.Lock()
	defer ipFirstJoinTracker.mu.Unlock()
	if _, exists := ipFirstJoinTracker.times[ipid]; !exists {
		ipFirstJoinTracker.times[ipid] = time.Now()
	}
}

// checkIPJoinWait returns true (and the remaining seconds) if the IPID has not yet
// waited 60 seconds since first joining the server.  This persists across reconnections
// so that a client cannot bypass the wait by disconnecting and reconnecting.
func checkIPJoinWait(ipid string) (bool, int) {
	const modcallJoinWait = 60 * time.Second
	ipFirstJoinTracker.mu.Lock()
	defer ipFirstJoinTracker.mu.Unlock()
	first, exists := ipFirstJoinTracker.times[ipid]
	if !exists {
		return false, 0
	}
	elapsed := time.Since(first)
	if elapsed < modcallJoinWait {
		remaining := int(math.Ceil((modcallJoinWait - elapsed).Seconds()))
		return true, remaining
	}
	return false, 0
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
	valid := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= config.OOCRateLimit {
		ipOOCTracker.timestamps[ipid] = valid
		return true
	}
	valid = append(valid, now)
	ipOOCTracker.timestamps[ipid] = valid
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
	valid := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= config.PingRateLimit {
		ipPingTracker.timestamps[ipid] = valid
		return true
	}
	valid = append(valid, now)
	ipPingTracker.timestamps[ipid] = valid
	return false
}

