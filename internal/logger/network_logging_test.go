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

package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- WriteNetworkLog tests ----

func TestWriteNetworkLogDisabled(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableNetworkLog = false

	WriteNetworkLog("hdid1", "ipid1", "RECV", "HI#abc#%")

	if _, err := os.Stat(filepath.Join(tempDir, "network.log")); !os.IsNotExist(err) {
		t.Error("network.log should not be created when EnableNetworkLog is false")
	}
}

func TestWriteNetworkLogRecv(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableNetworkLog = true
	defer func() { EnableNetworkLog = false }()

	WriteNetworkLog("hdid1", "ipid1", "RECV", "HI#abc#%")

	content, err := os.ReadFile(filepath.Join(tempDir, "network.log"))
	if err != nil {
		t.Fatalf("network.log was not created: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "RECV") {
		t.Errorf("Expected RECV in network.log, got:\n%s", s)
	}
	if !strings.Contains(s, "IPID:ipid1") {
		t.Errorf("Expected IPID:ipid1 in network.log, got:\n%s", s)
	}
	if !strings.Contains(s, "HDID:hdid1") {
		t.Errorf("Expected HDID:hdid1 in network.log, got:\n%s", s)
	}
	if !strings.Contains(s, "HI#abc#%") {
		t.Errorf("Expected packet content in network.log, got:\n%s", s)
	}
}

func TestWriteNetworkLogSend(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableNetworkLog = true
	defer func() { EnableNetworkLog = false }()

	WriteNetworkLog("hdid2", "ipid2", "SEND", "ID#0#Athena#v1.0.2#%")

	content, err := os.ReadFile(filepath.Join(tempDir, "network.log"))
	if err != nil {
		t.Fatalf("network.log was not created: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "SEND") {
		t.Errorf("Expected SEND in network.log, got:\n%s", s)
	}
	if !strings.Contains(s, "ID#0#Athena") {
		t.Errorf("Expected packet content in network.log, got:\n%s", s)
	}
}

func TestWriteNetworkLogAppendsMultipleEntries(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableNetworkLog = true
	defer func() { EnableNetworkLog = false }()

	WriteNetworkLog("hd1", "ip1", "RECV", "HI#abc#%")
	WriteNetworkLog("hd1", "ip1", "SEND", "ID#0#Athena#v1.0.2#%")
	WriteNetworkLog("hd1", "ip1", "RECV", "RD##%")

	content, _ := os.ReadFile(filepath.Join(tempDir, "network.log"))
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines in network.log, got %d:\n%s", len(lines), string(content))
	}
}

// ---- WriteCrashLog tests ----

func TestWriteCrashLogCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir

	panicVal := "something went wrong"
	fakeStack := []byte("goroutine 1 [running]:\nmain.main()\n\t/app/main.go:42")

	WriteCrashLog(panicVal, fakeStack)

	var crashFile string
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Could not read temp dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".log") {
			crashFile = filepath.Join(tempDir, e.Name())
			break
		}
	}
	if crashFile == "" {
		t.Fatal("No crash-*.log file was created")
	}

	content, err := os.ReadFile(crashFile)
	if err != nil {
		t.Fatalf("Could not read crash file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, panicVal) {
		t.Errorf("Crash file does not contain panic value %q:\n%s", panicVal, s)
	}
	if !strings.Contains(s, "goroutine 1") {
		t.Errorf("Crash file does not contain stack trace:\n%s", s)
	}
}

func TestWriteCrashLogContentsFormat(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir

	WriteCrashLog(42, []byte("stack trace here"))

	entries, _ := os.ReadDir(tempDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") {
			content, _ := os.ReadFile(filepath.Join(tempDir, e.Name()))
			if !strings.HasPrefix(string(content), "panic: 42") {
				t.Errorf("Crash file should start with 'panic: 42', got: %s", string(content))
			}
			return
		}
	}
	t.Error("No crash file found")
}

func TestWriteCrashLogAlsoWritesToNetworkLog(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableNetworkLog = true
	defer func() { EnableNetworkLog = false }()

	WriteCrashLog("network crash test", []byte("stack"))

	content, err := os.ReadFile(filepath.Join(tempDir, "network.log"))
	if err != nil {
		t.Fatalf("network.log was not created: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "CRASH") {
		t.Errorf("network.log should contain CRASH entry, got:\n%s", s)
	}
	if !strings.Contains(s, "network crash test") {
		t.Errorf("network.log should contain the panic value, got:\n%s", s)
	}
}
