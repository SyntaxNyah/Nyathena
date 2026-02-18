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
	"reflect"
	"testing"
)

func TestLoadMusic_PreservesOriginalCase(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "athena-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original ConfigPath and restore it after test
	originalConfigPath := ConfigPath
	defer func() { ConfigPath = originalConfigPath }()
	ConfigPath = tmpDir

	// Create test music.txt with mixed case paths
	musicContent := `Prelude
Ace Attorney/Prelude/[AA] Opening.opus
Ace Attorney/Prelude/[JFA] Prelude.opus
Trial
Ace Attorney/Trial/[AA] Trial.opus
Character Themes
Ace Attorney/Character Themes/[AA] Maya Fey - Turnabout Sisters 2001.opus
YTTD/[YTTD] Voice Of Healing.opus`

	musicPath := filepath.Join(tmpDir, "music.txt")
	err = os.WriteFile(musicPath, []byte(musicContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test music.txt: %v", err)
	}

	// Load music
	music, err := LoadMusic()
	if err != nil {
		t.Fatalf("LoadMusic() failed: %v", err)
	}

	// Expected result: Original case should be preserved
	expected := []string{
		"Prelude",
		"Ace Attorney/Prelude/[AA] Opening.opus",
		"Ace Attorney/Prelude/[JFA] Prelude.opus",
		"Trial",
		"Ace Attorney/Trial/[AA] Trial.opus",
		"Character Themes",
		"Ace Attorney/Character Themes/[AA] Maya Fey - Turnabout Sisters 2001.opus",
		"YTTD/[YTTD] Voice Of Healing.opus",
	}

	if !reflect.DeepEqual(music, expected) {
		t.Errorf("LoadMusic() result mismatch\nGot:  %v\nWant: %v", music, expected)
	}
}

func TestLoadMusic_KeepsCategoriesUnchanged(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "athena-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original ConfigPath and restore it after test
	originalConfigPath := ConfigPath
	defer func() { ConfigPath = originalConfigPath }()
	ConfigPath = tmpDir

	// Create test music.txt with only categories (no file extensions)
	musicContent := `Prelude
Questioning
Objection
Pursuit`

	musicPath := filepath.Join(tmpDir, "music.txt")
	err = os.WriteFile(musicPath, []byte(musicContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test music.txt: %v", err)
	}

	// Load music
	music, err := LoadMusic()
	if err != nil {
		t.Fatalf("LoadMusic() failed: %v", err)
	}

	// Expected result: categories should remain exactly as in file
	expected := []string{
		"Prelude",
		"Questioning",
		"Objection",
		"Pursuit",
	}

	if !reflect.DeepEqual(music, expected) {
		t.Errorf("LoadMusic() result mismatch\nGot:  %v\nWant: %v", music, expected)
	}
}
