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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
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
		discord.WebhookURL = config.WebhookURL
	}

	// Load areas.
	for _, a := range areaData {
		areaNames += a.Name + "#"
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
	areaNames = strings.TrimSuffix(areaNames, "#")
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

// setupHTTPMux creates an HTTP mux with WebSocket handler and optional static asset serving
func setupHTTPMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Register specific paths BEFORE the catch-all WebSocket handler
	// This ensures /base/ is handled by the file server, not the WebSocket handler
	if config.AssetPath != "" {
		// Validate that the asset directory exists
		if info, err := os.Stat(config.AssetPath); err != nil {
			logger.LogErrorf("Asset path configured but directory does not exist: %s (%v)", config.AssetPath, err)
		} else if !info.IsDir() {
			logger.LogErrorf("Asset path configured but is not a directory: %s", config.AssetPath)
		} else {
			logger.LogDebugf("Serving static assets from %s at /base/", config.AssetPath)
			fileServer := http.FileServer(http.Dir(config.AssetPath))
			mux.Handle("/base/", http.StripPrefix("/base/", fileServer))
		}
	}

	// Register WebSocket handler as catch-all LAST
	// This must be registered after specific paths like /base/
	mux.HandleFunc("/", HandleWS)

	return mux
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

	s := &http.Server{
		Handler: setupHTTPMux(),
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

	s := &http.Server{
		Handler: setupHTTPMux(),
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
	// Get the origin from the request for logging
	origin := r.Header.Get("Origin")
	remoteAddr := getRealIP(r)

	// Log the incoming WebSocket connection attempt
	if logger.DebugNetwork {
		logger.LogDebugf("WebSocket connection attempt from %s (Origin: %s, Path: %s)", remoteAddr, origin, r.URL.Path)
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: config.WebSocketOrigins,
	})
	if err != nil {
		logger.LogErrorf("WebSocket connection failed from %s (Origin: %s): %v", remoteAddr, origin, err)
		return
	}

	ipid := getIpid(remoteAddr)
	if logger.DebugNetwork {
		logger.LogDebugf("WebSocket connection accepted from %v (Origin: %s)", ipid, origin)
	}
	// Use MessageBinary instead of MessageText to avoid UTF-8 validation errors
	// The Attorney Online protocol may contain non-UTF-8 data, and strict UTF-8
	// validation in MessageText mode causes browsers to close connections with
	// code 1002 (Protocol Error). Binary mode allows the protocol to work with
	// any byte sequence while still transmitting text data.
	client := NewClient(websocket.NetConn(context.TODO(), c, websocket.MessageBinary), ipid)
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
	s := fmt.Sprintf("%v | %v | %v | %v | %v | %v",
		time.Now().UTC().Format("15:04:05"), action, client.CurrentCharacter(), client.Ipid(), client.OOCName(), message)
	client.Area().UpdateBuffer(s)
	if audit {
		logger.WriteAudit(s)
	}
}

// sendPlayerArup sends a player ARUP to all connected clients.
func sendPlayerArup() {
	plCounts := []string{"0"}
	for _, a := range areas {
		s := strconv.Itoa(a.PlayerCount())
		plCounts = append(plCounts, s)
	}
	writeToAll("ARUP", plCounts...)
}

// sendCMArup sends a CM ARUP to all connected clients.
func sendCMArup() {
	returnL := []string{"2"}
	for _, a := range areas {
		var cms []string
		var uids []int
		uids = append(uids, a.CMs()...)
		if len(uids) == 0 {
			returnL = append(returnL, "FREE")
			continue
		}
		for _, u := range uids {
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
	statuses := []string{"1"}
	for _, a := range areas {
		statuses = append(statuses, a.Status().String())
	}
	writeToAll("ARUP", statuses...)
}

// sendLockArup sends a lock ARUP to all connected clients.
func sendLockArup() {
	locks := []string{"3"}
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
	for c := range clients.GetAllClients() {
		if c.Uid() == uid {
			return c, nil
		}
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
func getParrotMsg() string {
	gen := rand.New(rand.NewSource(time.Now().Unix()))
	return parrot[gen.Intn(len(parrot))]
}
