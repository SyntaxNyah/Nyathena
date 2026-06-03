package athena

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

func TestExtractYouTubeID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtube.com/watch?v=dQw4w9WgXcQ&list=ignored", "dQw4w9WgXcQ"},
		{"https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ?t=42", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"http://youtube.com/watch?v=ab_-CD12345", "ab_-CD12345"},
		{"https://example.com/watch?v=dQw4w9WgXcQ", ""},
		{"https://youtube.com/watch?v=tooShort", ""},
		{"https://youtube.com/playlist?list=PL123", ""},
		{"ftp://youtube.com/watch?v=dQw4w9WgXcQ", ""},
		{"not a url", ""},
		{"", ""},
		{"https://youtu.be/", ""},
	}
	for _, c := range cases {
		got := extractYouTubeID(c.in)
		if got != c.want {
			t.Errorf("extractYouTubeID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandAssetURLMacro(t *testing.T) {
	prev := config
	defer func() { config = prev }()
	config = &settings.Config{ServerConfig: settings.ServerConfig{AssetURL: "https://cdn.example.com/assets"}}

	if got := expandAssetURLMacro("{ASSET_URL}/yt/"); got != "https://cdn.example.com/assets/yt/" {
		t.Errorf("macro expansion = %q", got)
	}
	if got := expandAssetURLMacro("https://other.example/yt/"); got != "https://other.example/yt/" {
		t.Errorf("non-macro string changed: %q", got)
	}
}

func TestYouTubeEnabled(t *testing.T) {
	prev := config
	defer func() { config = prev }()

	config = &settings.Config{ServerConfig: settings.ServerConfig{
		YouTubePlayPrefix:          "",
		YouTubeDownloadDestination: "file:///tmp/yt",
	}}
	if youTubeEnabled() {
		t.Error("expected disabled when prefix empty")
	}

	config = &settings.Config{ServerConfig: settings.ServerConfig{
		YouTubePlayPrefix:          "https://cdn.example.com/yt/",
		YouTubeDownloadDestination: "",
	}}
	if youTubeEnabled() {
		t.Error("expected disabled when destination empty")
	}

	config = &settings.Config{ServerConfig: settings.ServerConfig{
		YouTubePlayPrefix:          "not-an-http-url",
		YouTubeDownloadDestination: "file:///tmp/yt",
	}}
	if youTubeEnabled() {
		t.Error("expected disabled when prefix doesn't start with http")
	}

	config = &settings.Config{ServerConfig: settings.ServerConfig{
		YouTubePlayPrefix:          "https://cdn.example.com/yt/",
		YouTubeDownloadDestination: "file:///tmp/yt",
	}}
	if !youTubeEnabled() {
		t.Error("expected enabled with valid prefix + destination")
	}

	// {ASSET_URL} macro must satisfy the http-prefix check after expansion.
	config = &settings.Config{ServerConfig: settings.ServerConfig{
		AssetURL:                   "https://cdn.example.com/assets",
		YouTubePlayPrefix:          "{ASSET_URL}/yt/",
		YouTubeDownloadDestination: "file:///tmp/yt",
	}}
	if !youTubeEnabled() {
		t.Error("expected enabled when {ASSET_URL} expands to https://")
	}
}

func TestYouTubeDestDir(t *testing.T) {
	prev := config
	defer func() { config = prev }()

	config = &settings.Config{ServerConfig: settings.ServerConfig{
		YouTubeDownloadDestination: "file:///var/lib/athena/yt",
	}}
	got, err := youTubeDestDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/var/lib/athena/yt" {
		t.Errorf("destDir = %q", got)
	}

	for _, bad := range []string{
		"",
		"http://example.com/yt",
		"file://remotehost/yt",
		"file://",
	} {
		config.YouTubeDownloadDestination = bad
		if _, err := youTubeDestDir(); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestYtDlpCookieArgs(t *testing.T) {
	prev := config
	defer func() { config = prev }()

	config = &settings.Config{ServerConfig: settings.ServerConfig{YouTubeCookiesPath: ""}}
	if got := ytDlpCookieArgs(); got != nil {
		t.Errorf("expected nil when unset, got %v", got)
	}

	config = &settings.Config{ServerConfig: settings.ServerConfig{YouTubeCookiesPath: "/etc/athena/yt-cookies.txt"}}
	got := ytDlpCookieArgs()
	if len(got) != 2 || got[0] != "--cookies" || got[1] != "/etc/athena/yt-cookies.txt" {
		t.Errorf("ytDlpCookieArgs() = %v", got)
	}
}

func TestYouTubePlayURL(t *testing.T) {
	prev := config
	defer func() { config = prev }()
	config = &settings.Config{ServerConfig: settings.ServerConfig{
		AssetURL:          "https://cdn.example.com/assets",
		YouTubePlayPrefix: "{ASSET_URL}/yt/",
	}}
	if got := youTubePlayURL("dQw4w9WgXcQ", ".opus"); got != "https://cdn.example.com/assets/yt/dQw4w9WgXcQ.opus" {
		t.Errorf("youTubePlayURL .opus = %q", got)
	}
	if got := youTubePlayURL("dQw4w9WgXcQ", ".mp3"); got != "https://cdn.example.com/assets/yt/dQw4w9WgXcQ.mp3" {
		t.Errorf("youTubePlayURL .mp3 = %q", got)
	}
}

func TestYouTubeCacheLookup(t *testing.T) {
	dir := t.TempDir()
	id := "dQw4w9WgXcQ"

	if got := youTubeCacheLookup(dir, id); got != "" {
		t.Errorf("empty dir: expected \"\", got %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, id+".mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := youTubeCacheLookup(dir, id); got != ".mp3" {
		t.Errorf("mp3 only: expected .mp3, got %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, id+".opus"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := youTubeCacheLookup(dir, id); got != ".opus" {
		t.Errorf("both present: expected .opus preferred, got %q", got)
	}
}
