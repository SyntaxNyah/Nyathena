package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/BurntSushi/toml"
)

const testConfigDoc = "[Server]\nname = \"Nyathena\"\n"

// encodeUTF16 builds the bytes a Windows editor writes for s: optional BOM
// followed by UTF-16 code units.
func encodeUTF16(s string, bigEndian, bom bool) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units)*2+2)
	if bom {
		if bigEndian {
			out = append(out, 0xFE, 0xFF)
		} else {
			out = append(out, 0xFF, 0xFE)
		}
	}
	for _, u := range units {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

func TestNormalizeTOMLBytes(t *testing.T) {
	tests := []struct {
		name    string
		in      []byte
		want    string
		wantFix bool
	}{
		{name: "clean UTF-8 is untouched", in: []byte(testConfigDoc), want: testConfigDoc},
		{name: "UTF-8 BOM", in: append([]byte{0xEF, 0xBB, 0xBF}, testConfigDoc...), want: testConfigDoc, wantFix: true},
		{name: "UTF-16LE with BOM", in: encodeUTF16(testConfigDoc, false, true), want: testConfigDoc, wantFix: true},
		{name: "UTF-16BE with BOM", in: encodeUTF16(testConfigDoc, true, true), want: testConfigDoc, wantFix: true},
		{name: "UTF-16LE without BOM", in: encodeUTF16(testConfigDoc, false, false), want: testConfigDoc, wantFix: true},
		{name: "UTF-16BE without BOM", in: encodeUTF16(testConfigDoc, true, false), want: testConfigDoc, wantFix: true},
		{name: "stray U+FEFF", in: []byte("[Server]\n\xEF\xBB\xBFname = \"Nyathena\"\n"), want: testConfigDoc, wantFix: true},
		{name: "NUL bytes", in: []byte("[Server]\x00\nname = \"Nyathena\"\n"), want: testConfigDoc, wantFix: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, fixes := normalizeTOMLBytes(tt.in)
			if string(got) != tt.want {
				t.Fatalf("normalizeTOMLBytes() = %q, want %q", got, tt.want)
			}
			if tt.wantFix && len(fixes) == 0 {
				t.Fatal("expected at least one recorded fix")
			}
			if !tt.wantFix && len(fixes) != 0 {
				t.Fatalf("expected no fixes, got %v", fixes)
			}
			if _, err := toml.Decode(string(got), &struct{}{}); err != nil {
				t.Fatalf("repaired bytes still do not parse: %v", err)
			}
		})
	}
}

func TestNormalizeTOMLBytesPreservesUnicode(t *testing.T) {
	doc := "[Server]\nname = \"Café — Discord \U0001F600\"\n"
	for _, bigEndian := range []bool{false, true} {
		got, fixes := normalizeTOMLBytes(encodeUTF16(doc, bigEndian, true))
		if len(fixes) == 0 {
			t.Fatalf("bigEndian=%v: expected a UTF-16 decode", bigEndian)
		}
		if string(got) != doc {
			t.Errorf("bigEndian=%v: got %q, want %q", bigEndian, got, doc)
		}
	}
}

func TestNormalizeTOMLBytesLeavesUnfixableFileAlone(t *testing.T) {
	bad := []byte("[Server]\nname = \"\xff\xfe\xfd\"\n")
	got, fixes := normalizeTOMLBytes(bad)
	if len(fixes) != 0 {
		t.Fatalf("expected no fixes for invalid UTF-8, got %v", fixes)
	}
	if !bytes.Equal(got, bad) {
		t.Fatal("bytes that cannot be repaired mechanically must not be modified")
	}
}

func TestDecodeTOMLFileRepairsFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, testConfigDoc...), 0o600); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Server struct{ Name string }
	}
	if _, err := decodeTOMLFile(path, &doc); err != nil {
		t.Fatalf("decodeTOMLFile() error = %v", err)
	}
	if doc.Server.Name != "Nyathena" {
		t.Fatalf("Server.Name = %q, want Nyathena", doc.Server.Name)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(onDisk, utf8BOM) {
		t.Error("BOM still present on disk after decodeTOMLFile")
	}
	if string(onDisk) != testConfigDoc {
		t.Errorf("file on disk = %q, want %q", onDisk, testConfigDoc)
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) != 1 {
		t.Errorf("atomic write left a temporary file behind: %d entries", len(entries))
	}
}

func TestConfigLoadRepairsBOM(t *testing.T) {
	dir := t.TempDir()
	previous := ConfigPath
	ConfigPath = dir
	t.Cleanup(func() { ConfigPath = previous })

	path := filepath.Join(dir, "config.toml")
	doc := "[Server]\nname = \"Skrapegropen\"\nmax_players = 42\n"
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, doc...), 0o644); err != nil {
		t.Fatal(err)
	}

	conf := DefaultConfig()
	if err := conf.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if conf.Name != "Skrapegropen" {
		t.Errorf("Name = %q, want Skrapegropen", conf.Name)
	}
	if conf.MaxPlayers != 42 {
		t.Errorf("MaxPlayers = %d, want 42", conf.MaxPlayers)
	}
	if conf.Port != DefaultConfig().Port {
		t.Errorf("Port = %d, want the default %d", conf.Port, DefaultConfig().Port)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(onDisk, utf8BOM) {
		t.Error("BOM still on disk after Config.Load")
	}
}

func TestDecodeTOMLFileMissingFile(t *testing.T) {
	if _, err := decodeTOMLFile(filepath.Join(t.TempDir(), "missing.toml"), &struct{}{}); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
