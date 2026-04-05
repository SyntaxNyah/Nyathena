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
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Spot-check a selection of expected default values.
	if cfg.Port != 27016 {
		t.Errorf("Port = %d, want 27016", cfg.Port)
	}
	if cfg.Name != "Unnamed Server" {
		t.Errorf("Name = %q, want \"Unnamed Server\"", cfg.Name)
	}
	if cfg.MaxPlayers != 100 {
		t.Errorf("MaxPlayers = %d, want 100", cfg.MaxPlayers)
	}
	if cfg.MaxMsg != 256 {
		t.Errorf("MaxMsg = %d, want 256", cfg.MaxMsg)
	}
	if cfg.BanLen != "3d" {
		t.Errorf("BanLen = %q, want \"3d\"", cfg.BanLen)
	}
	if cfg.MCLimit != 16 {
		t.Errorf("MCLimit = %d, want 16", cfg.MCLimit)
	}
	if cfg.RateLimit != 20 {
		t.Errorf("RateLimit = %d, want 20", cfg.RateLimit)
	}
	if cfg.BufSize != 150 {
		t.Errorf("BufSize = %d, want 150", cfg.BufSize)
	}
	if cfg.MSAddr != "https://servers.aceattorneyonline.com/servers" {
		t.Errorf("MSAddr = %q, unexpected value", cfg.MSAddr)
	}
	if cfg.ConnFloodAutoban != true {
		t.Errorf("ConnFloodAutoban = false, want true")
	}
}

// TestLoadFileSuccess reads a temp file and verifies the contents are returned.
func TestLoadFileSuccess(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// ConfigPath is prepended as-is before the file argument, so set it to the
	// directory and pass the filename with a leading '/'.
	ConfigPath = dir
	lines, err := LoadFile("/test.txt")
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("LoadFile returned %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("LoadFile returned %v, unexpected content", lines)
	}
}

// TestLoadFileMissing verifies that LoadFile returns an error for a non-existent file.
func TestLoadFileMissing(t *testing.T) {
	ConfigPath = t.TempDir()
	_, err := LoadFile("/nonexistent.txt")
	if err == nil {
		t.Error("LoadFile of missing file returned nil error, want error")
	}
}

// TestLoadFileEmpty verifies that LoadFile returns an empty slice for an empty file.
func TestLoadFileEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.txt"), []byte(""), 0600); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}
	ConfigPath = dir
	lines, err := LoadFile("/empty.txt")
	if err != nil {
		t.Fatalf("LoadFile on empty file returned error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("LoadFile on empty file returned %d lines, want 0", len(lines))
	}
}

// TestLoadConfigOverridesDefaults verifies that a minimal TOML config file
// correctly overrides default values.
func TestLoadConfigOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	toml := `[Server]
name = "My Test Server"
port = 12345
max_players = 50
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0600); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	ConfigPath = dir
	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}
	if cfg.Name != "My Test Server" {
		t.Errorf("Name = %q, want \"My Test Server\"", cfg.Name)
	}
	if cfg.Port != 12345 {
		t.Errorf("Port = %d, want 12345", cfg.Port)
	}
	if cfg.MaxPlayers != 50 {
		t.Errorf("MaxPlayers = %d, want 50", cfg.MaxPlayers)
	}
	// Keys not in the file should retain their default values.
	if cfg.MCLimit != 16 {
		t.Errorf("MCLimit = %d, want default 16", cfg.MCLimit)
	}
}

// TestGetConfigMissingFile verifies that GetConfig returns an error when no
// config.toml is present.
func TestGetConfigMissingFile(t *testing.T) {
	ConfigPath = t.TempDir() // empty directory – no config.toml
	_, err := GetConfig()
	if err == nil {
		t.Error("GetConfig with no config.toml returned nil error, want error")
	}
}
