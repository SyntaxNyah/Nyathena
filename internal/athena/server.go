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

	// Tournament mode state
	tournamentActive       bool
	tournamentMutex        sync.Mutex
	tournamentStartTime    time.Time
	tournamentParticipants map[int]*TournamentParticipant // uid -> participant data
)

// TournamentParticipant tracks a user's tournament performance
type TournamentParticipant struct {
	uid          int
	messageCount int
	joinedAt     time.Time
}

// InitServer initalizes the server's database, uids, configs, and advertiser.
func InitServer(conf *settings.Config) error {
	db.Open()
	uids.InitHeap(conf.MaxPlayers)
	config = conf
	
	// Initialize tournament state
	tournamentParticipants = make(map[int]*TournamentParticipant)

	// Load server data.
	var err error
	music, err = settings.LoadMusic()
	if err != nil {
		return err
	}
	characters, err = settings.LoadFile("/characters.txt")
	if err != nil {
		return err
	} else if len(characters) == 0 {
		return fmt.Errorf("empty character list")
	}
	areaData, err := settings.LoadAreas()
	if err != nil {
		return err
	}

	roles, err = settings.LoadRoles()
	if err != nil {
		return err
	}

	backgrounds, err = settings.LoadFile("/backgrounds.txt")
	if err != nil {
		return err
	} else if len(backgrounds) == 0 {
		return fmt.Errorf("empty background list")
	}

	parrot, err = settings.LoadFile("/parrot.txt")
	if err != nil {
		return err
	} else if len(parrot) == 0 {
		return fmt.Errorf("empty parrot list")
	}
	_, err = str2duration.ParseDuration(conf.BanLen)
	if err != nil {
		return fmt.Errorf("failed to parse default_ban_duration: %v", err.Error())
	}

	// Discord webhook.
	if config.WebhookURL != "" {
		enableDiscord = true
		webhook.ServerName = config.Name
		webhook.PingRoleID = config.WebhookPingRoleID
		discord.WebhookURL = config.WebhookURL
	}

	// Load areas.
	areas = make([]*area.Area, 0, len(areaData))
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
		if a.Bg == "" || !sliceutil.ContainsString(backgrounds, a.Bg) {
			logger.LogWarningf("Area %v has an invalid or undefined background, defaulting to 'default'.", a.Name)
			a.Bg = "default"
		}
		areas = append(areas, area.NewArea(a, len(characters), conf.BufSize, evi_mode))
	}
	areaNames = areaNameBuilder.String()

	// Build O(1) area-index lookup map.
	areaIndexMap = make(map[*area.Area]int, len(areas))
	for i, a := range areas {
		areaIndexMap[a] = i
	}

	// Initialize area logging if enabled
	logger.EnableAreaLogging = conf.EnableAreaLogging
	if logger.EnableAreaLogging {
		logger.LogInfo("Area logging is enabled. Creating area log directories...")
		for _, a := range areas {
			if err := logger.CreateAreaLogDirectory(a.Name()); err != nil {
				logger.LogErrorf("Failed to create area log directory for %v: %v", a.Name(), err)
			}
		}
	}
	
	if config.Advertise {
		advert := ms.Advertisement{
			Port:    config.Port,
			Players: players.GetPlayerCount(),
			Name:    config.Name,
			Desc:    config.Desc}
		if config.AdvertiseHostname != "" {
			advert.IP = config.AdvertiseHostname
		}
		if config.EnableWS {
			if config.ReverseProxyMode {
				advert.WSPort = config.ReverseProxyHTTPPort
			} else {
				advert.WSPort = config.WSPort
			}
		}
		if config.EnableWSS {
			if config.ReverseProxyMode {
				advert.WSSPort = config.ReverseProxyHTTPSPort
			} else {
				advert.WSSPort = config.WSSPort
			}
		}
		go ms.Advertise(config.MSAddr, advert, updatePlayers, advertDone)
	}
	initCommands()
	return nil
}

// StartDiscordBot starts the Discord bot if a token is configured.
// It should be called after InitServer.
func StartDiscordBot() {
	if config.BotToken == "" {
		return
	}
	cfg := discordbot.Config{
		Token:     config.BotToken,
		GuildID:   config.GuildID,
		ModRoleID: config.ModRoleID,
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

// ListenTCP starts the server's TCP listener.
func ListenTCP() {
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
		if logger.DebugNetwork {
			logger.LogDebugf("Connection recieved from %v", ipid)
		}
		client := NewClient(conn, ipid)
		go client.HandleClient()
	}
}

// ListenWS starts the server's websocket listener.
func ListenWS() {
	listener, err := net.Listen("tcp", config.Addr+":"+strconv.Itoa(config.WSPort))
	if err != nil {
		FatalError <- err
		return
	}
	logger.LogDebug("WS listener started.")
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleWS)
	s := &http.Server{
		Handler: mux,
	}
	err = s.Serve(listener)
	if err != http.ErrServerClosed {
		FatalError <- err
	}
}

// ListenWSS starts the server's secure websocket listener.
// If TLS certificate and key paths are provided, it serves with TLS (direct HTTPS).
// If not provided, it serves plain HTTP (useful when behind a reverse proxy like Cloudflare).
func ListenWSS() {
	listener, err := net.Listen("tcp", config.Addr+":"+strconv.Itoa(config.WSSPort))
	if err != nil {
		FatalError <- err
		return
	}
	logger.LogDebug("WSS listener started.")
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleWS)
	s := &http.Server{
		Handler: mux,
	}
	
	// Use TLS if certificate and key paths are provided, otherwise serve plain HTTP
	// (useful when behind a reverse proxy that handles TLS termination)
	if config.TLSCertPath != "" && config.TLSKeyPath != "" {
		logger.LogDebugf("WSS using TLS with cert: %s", config.TLSCertPath)
		err = s.ServeTLS(listener, config.TLSCertPath, config.TLSKeyPath)
	} else {
		logger.LogDebug("WSS using plain HTTP (expecting reverse proxy for TLS)")
		err = s.Serve(listener)
	}
	
	if err != http.ErrServerClosed {
		FatalError <- err
	}
}

// HandleWS handles a websocket connection.
func HandleWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		logger.LogError(err.Error())
		return
	}
	ipid := getIpid(getRealIP(r))
	if logger.DebugNetwork {
		logger.LogDebugf("Connection recieved from %v", ipid)
	}
	client := NewClient(websocket.NetConn(context.TODO(), c, websocket.MessageText), ipid)
	go client.HandleClient()
}

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

// CleanupServer closes all connections to the server, and closes the server's database.
func CleanupServer() {
	for client := range clients.GetAllClients() {
		client.conn.Close()
	}
	db.Close()
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
