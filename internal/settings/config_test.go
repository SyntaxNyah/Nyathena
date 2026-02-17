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

package settings

import (
	"os"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDefaultWSMessageSizeLimit(t *testing.T) {
	conf := defaultConfig()

	// Default should be 1 MB (1048576 bytes)
	expected := int64(1048576)
	if conf.WSMessageSizeLimit != expected {
		t.Errorf("Expected default WSMessageSizeLimit to be %d, got %d", expected, conf.WSMessageSizeLimit)
	}
}

func TestConfigLoadWSMessageSizeLimit(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := tmpDir + "/config.toml"

	configContent := `
[Server]
name = "Test Server"
port = 27016
max_players = 100
websocket_message_size_limit = 2097152

[Logging]
log_level = "info"
log_directory = "logs"
log_methods = [ "stdout" ]

[MasterServer]
advertise = false
addr = "https://servers.aceattorneyonline.com/servers"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load the config
	var conf Config
	_, err = toml.DecodeFile(configFile, &conf)
	if err != nil {
		t.Fatalf("Failed to decode config: %v", err)
	}

	// Verify the value was loaded correctly
	expected := int64(2097152) // 2 MB
	if conf.WSMessageSizeLimit != expected {
		t.Errorf("Expected WSMessageSizeLimit to be %d, got %d", expected, conf.WSMessageSizeLimit)
	}
}

func TestConfigWithoutWSMessageSizeLimit(t *testing.T) {
	// Test that config without the field still loads with default
	tmpDir := t.TempDir()
	configFile := tmpDir + "/config.toml"

	configContent := `
[Server]
name = "Test Server"
port = 27016
max_players = 100

[Logging]
log_level = "info"
log_directory = "logs"
log_methods = [ "stdout" ]

[MasterServer]
advertise = false
addr = "https://servers.aceattorneyonline.com/servers"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load using the Load method which handles defaults
	ConfigPath = tmpDir
	conf := defaultConfig()
	err = conf.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Should default to 1 MB when not specified in config
	expected := int64(1048576)
	if conf.WSMessageSizeLimit != expected {
		t.Errorf("Expected WSMessageSizeLimit to default to %d when not in config, got %d", expected, conf.WSMessageSizeLimit)
	}
}
