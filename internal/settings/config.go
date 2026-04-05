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

// Package settings handles reading and writing to Athena's configuration files.
package settings

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// Stores the path to the config directory
var ConfigPath string

type Config struct {
	ServerConfig  `toml:"Server"`
	LogConfig     `toml:"Logging"`
	MSConfig      `toml:"MasterServer"`
	DiscordConfig `toml:"Discord"`
}

type ServerConfig struct {
	Addr                  string `toml:"addr"`
	Port                  int    `toml:"port"`
	AdvertiseHostname     string `toml:"advertise_hostname"`
	Name                  string `toml:"name"`
	Desc                  string `toml:"description"`
	MaxPlayers            int    `toml:"max_players"`
	MaxMsg                int    `toml:"max_message_length"`
	BanLen                string `toml:"default_ban_duration"`
	EnableWS              bool   `toml:"enable_webao"`
	WSPort                int    `toml:"webao_port"`
	EnableWSS             bool   `toml:"enable_webao_secure"`
	WSSPort               int    `toml:"webao_secure_port"`
	TLSCertPath           string `toml:"tls_cert_path"`
	TLSKeyPath            string `toml:"tls_key_path"`
	ReverseProxyMode      bool   `toml:"reverse_proxy_mode"`
	ReverseProxyHTTPPort  int    `toml:"reverse_proxy_http_port"`
	ReverseProxyHTTPSPort int    `toml:"reverse_proxy_https_port"`
	MCLimit               int    `toml:"multiclient_limit"`
	AssetURL              string `toml:"asset_url"`
	WebhookURL            string `toml:"webhook_url"`
	WebhookPingRoleID     string `toml:"webhook_ping_role_id"`
	PunishmentWebhookURL  string `toml:"punishment_webhook_url"`
	MaxDice               int    `toml:"max_dice"`
	MaxSide               int    `toml:"max_sides"`
	Motd                  string `toml:"motd"`
	MaxStatement          int    `toml:"max_testimony"`
	RateLimit             int    `toml:"message_rate_limit"`
	RateLimitWindow       int    `toml:"message_rate_limit_window"`
	ModcallCooldown       int    `toml:"modcall_cooldown"`
	ConnRateLimit              int    `toml:"connection_rate_limit"`
	ConnRateLimitWindow        int    `toml:"connection_rate_limit_window"`
	ConnFloodAutoban           bool   `toml:"conn_flood_autoban"`
	ConnFloodAutobanThreshold  int    `toml:"conn_flood_autoban_threshold"`
	PacketFloodAutoban         bool   `toml:"packet_flood_autoban"`
	RawPacketRateLimit         int    `toml:"raw_packet_rate_limit"`
	RawPacketRateLimitWindow   float64 `toml:"raw_packet_rate_limit_window"`
	OOCRateLimit          int    `toml:"ooc_rate_limit"`
	OOCRateLimitWindow    int    `toml:"ooc_rate_limit_window"`
	PingRateLimit             int    `toml:"ping_rate_limit"`
	PingRateLimitWindow       int    `toml:"ping_rate_limit_window"`
	NewIPIDOOCCooldown        int    `toml:"new_ipid_ooc_cooldown"`
	NewIPIDModcallCooldown    int    `toml:"new_ipid_modcall_cooldown"`
	GlobalNewIPRateLimit      int    `toml:"global_new_ip_rate_limit"`
	GlobalNewIPRateLimitWindow int   `toml:"global_new_ip_rate_limit_window"`
	IPRetentionDays           int    `toml:"ip_retention_days"`
	WebAOAllowedOrigin        string `toml:"webao_allowed_origin"`
	AutoModEnabled             bool   `toml:"automod_enabled"`
	AutoModWordlist            string `toml:"automod_wordlist"`
	AutoModAction              string `toml:"automod_action"`
	RandomSongCooldown         int    `toml:"random_song_cooldown"`
	BotBanPlaytimeThreshold    int    `toml:"botban_playtime_threshold"`
	IPHubAPIKey                string `toml:"iphub_api_key"`
	EnableCasino               bool     `toml:"enable_casino"`
	RegisterCaptcha            bool     `toml:"register_captcha"`
	EnableCommunityVote        bool     `toml:"enable_community_vote"`
	VoteThreshold              int      `toml:"vote_threshold"`
	VoteDuration               int      `toml:"vote_duration"`
	VoteActions                []string `toml:"vote_actions"`
	VoteMuteDuration           int      `toml:"vote_mute_duration"`
	TypingRacePhrases          []string `toml:"typing_race_phrases"`
	EnableNewspaper            bool     `toml:"enable_newspaper"`
	NewspaperInterval          string   `toml:"newspaper_interval"`
	NewspaperSections          []string `toml:"newspaper_sections"`
	// MaxConnectionGoroutines caps the number of concurrent connection-handling
	// goroutines.  When the pool is full, new connections wait until a slot
	// becomes available rather than spinning up an unbounded number of goroutines.
	// 0 (the default) disables the pool and preserves the original unbounded behaviour.
	MaxConnectionGoroutines int `toml:"max_connection_goroutines"`
}

type LogConfig struct {
	BufSize              int      `toml:"log_buffer_size"`
	LogLevel             string   `toml:"log_level"`
	LogDir               string   `toml:"log_directory"`
	LogMethods           []string `toml:"log_methods"`
	EnableAreaLogging    bool     `toml:"enable_area_logging"`
	EnableNetworkLogging bool     `toml:"enable_network_logging"`
}

type MSConfig struct {
	Advertise bool   `toml:"advertise"`
	MSAddr    string `toml:"addr"`
}

type DiscordConfig struct {
	BotToken  string `toml:"bot_token"`
	GuildID   string `toml:"guild_id"`
	ModRoleID string `toml:"mod_role_id"`
}

// Returns a default configuration.
func defaultConfig() *Config {
	return DefaultConfig()
}

// DefaultConfig returns the default server configuration. Exported for testing.
func DefaultConfig() *Config {
	return &Config{
		ServerConfig{
			Addr:                  "",
			Port:                  27016,
			AdvertiseHostname:     "",
			Name:                  "Unnamed Server",
			Desc:                  "",
			MaxPlayers:            100,
			MaxMsg:                256,
			BanLen:                "3d",
			EnableWS:              false,
			WSPort:                27017,
			EnableWSS:             false,
			WSSPort:               443,
			TLSCertPath:           "",
			TLSKeyPath:            "",
			ReverseProxyMode:      false,
			ReverseProxyHTTPPort:  80,
			ReverseProxyHTTPSPort: 443,
			MCLimit:               16,
			MaxDice:               100,
			MaxSide:               100,
			MaxStatement:          10,
			RateLimit:             20,
			RateLimitWindow:       10,
			ModcallCooldown:       0,
			ConnRateLimit:              10,
			ConnRateLimitWindow:        10,
			ConnFloodAutoban:           true,
			ConnFloodAutobanThreshold:  6,
			PacketFloodAutoban:         true,
			RawPacketRateLimit:         20,
			RawPacketRateLimitWindow:   2,
			OOCRateLimit:          4,
			OOCRateLimitWindow:    1,
			PingRateLimit:             10,
			PingRateLimitWindow:       5,
			NewIPIDOOCCooldown:        10,
			NewIPIDModcallCooldown:    60,
			GlobalNewIPRateLimit:      5,
			GlobalNewIPRateLimitWindow: 10,
			IPRetentionDays:           0,
			WebAOAllowedOrigin:        "web.aceattorneyonline.com",
			AutoModEnabled:             false,
			AutoModWordlist:            "banned_words.txt",
			AutoModAction:              "ban",
			RandomSongCooldown:         10,
			BotBanPlaytimeThreshold:    120,
			EnableCasino:               false,
			RegisterCaptcha:            true,
			EnableCommunityVote:        false,
			VoteThreshold:              3,
			VoteDuration:               120,
			VoteActions:                []string{"kick"},
			VoteMuteDuration:           300,
		},
		LogConfig{
			BufSize:              150,
			LogLevel:             "info",
			LogDir:               "logs",
			LogMethods:           []string{"stdout"},
			EnableAreaLogging:    false,
			EnableNetworkLogging: false,
		},
		MSConfig{
			Advertise: false,
			MSAddr:    "https://servers.aceattorneyonline.com/servers",
		},
		DiscordConfig{
			BotToken:  "",
			GuildID:   "",
			ModRoleID: "",
		},
	}
}

// Load reads the server's main configuration file.
func (conf *Config) Load() error {
	_, err := toml.DecodeFile(ConfigPath+"/config.toml", conf)
	if err != nil {
		return err
	}
	return nil
}

// GetConfig returns the server's config options.
func GetConfig() (*Config, error) {
	conf := defaultConfig()
	err := conf.Load()

	if err != nil {
		return nil, err
	}

	return conf, nil
}

// LoadMusic reads the server's music file, returning it's contents.
func LoadMusic() ([]string, error) {
	var musicList []string
	f, err := os.Open(ConfigPath + "/music.txt")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	in := bufio.NewScanner(f)
	for in.Scan() {
		musicList = append(musicList, in.Text())
	}
	if len(musicList) == 0 {
		return nil, fmt.Errorf("empty musiclist")
	}
	if strings.ContainsRune(musicList[0], '.') {
		musicList = append([]string{"Songs"}, musicList...)
	}
	return musicList, nil
}

// LoadFile reads a server file, returning it's contents.
func LoadFile(file string) ([]string, error) {
	var l []string
	f, err := os.Open(ConfigPath + file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	in := bufio.NewScanner(f)
	for in.Scan() {
		l = append(l, in.Text())
	}
	return l, nil
}

// LoadAreas reads the server's area configuration file, returning it's contents.
func LoadAreas() ([]area.AreaData, error) {
	var conf struct {
		Area []area.AreaData
	}
	_, err := toml.DecodeFile(ConfigPath+"/areas.toml", &conf)
	if err != nil {
		return conf.Area, err
	}
	if len(conf.Area) == 0 {
		return conf.Area, fmt.Errorf("empty arealist")
	}
	return conf.Area, nil
}

// LoadAreas reads the server's role configuration file, returning it's contents.
func LoadRoles() ([]permissions.Role, error) {
	var conf struct {
		Role []permissions.Role
	}
	_, err := toml.DecodeFile(ConfigPath+"/roles.toml", &conf)
	if err != nil {
		return conf.Role, err
	}
	if len(conf.Role) == 0 {
		return conf.Role, fmt.Errorf("empty rolelist")
	}
	return conf.Role, nil
}
