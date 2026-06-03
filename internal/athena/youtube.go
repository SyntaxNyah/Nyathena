/* Athena - A server for Attorney Online 2 written in Go

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version. */

package athena

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
)

var youtubeIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ytDownloadsInFlight dedupes concurrent /play requests for the same video.
// The value is a chan struct{} that is closed when the download completes
// (successfully or not). Waiters check the filesystem after the channel
// closes to decide whether to broadcast.
var ytDownloadsInFlight sync.Map

// youTubeEnabled reports whether /play <youtube-link> is configured.
// The expanded prefix must start with "http" and the destination must be set.
func youTubeEnabled() bool {
	if config == nil {
		return false
	}
	prefix := strings.TrimSpace(config.YouTubePlayPrefix)
	dest := strings.TrimSpace(config.YouTubeDownloadDestination)
	if prefix == "" || dest == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(expandAssetURLMacro(prefix)), "http")
}

// expandAssetURLMacro substitutes the literal "{ASSET_URL}" token with the
// configured asset_url so operators don't have to repeat the host.
func expandAssetURLMacro(s string) string {
	if config == nil {
		return s
	}
	return strings.ReplaceAll(s, "{ASSET_URL}", config.AssetURL)
}

// extractYouTubeID returns the 11-character video ID from any supported
// YouTube URL form, or "" if the URL is not a recognized YouTube link.
//
// Supported forms (case-insensitive host, www./m. prefixes tolerated):
//   - https://youtube.com/watch?v=<id>
//   - https://youtu.be/<id>
//   - https://youtube.com/shorts/<id>
//   - https://music.youtube.com/watch?v=<id>
func extractYouTubeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.HasPrefix(host, "www."):
		host = host[len("www."):]
	case strings.HasPrefix(host, "m."):
		host = host[len("m."):]
	}
	var id string
	switch host {
	case "youtu.be":
		if len(u.Path) > 1 {
			id = strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)[0]
		}
	case "youtube.com", "music.youtube.com":
		switch {
		case u.Path == "/watch":
			id = u.Query().Get("v")
		case strings.HasPrefix(u.Path, "/shorts/"):
			id = strings.SplitN(strings.TrimPrefix(u.Path, "/shorts/"), "/", 2)[0]
		}
	}
	if !youtubeIDRegex.MatchString(id) {
		return ""
	}
	return id
}

// youTubeDestDir resolves the configured file:// destination to a local path.
func youTubeDestDir() (string, error) {
	raw := strings.TrimSpace(config.YouTubeDownloadDestination)
	if raw == "" {
		return "", errors.New("youtube_download_destination is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid youtube_download_destination: %w", err)
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("only file:// destinations are supported (got %q)", u.Scheme)
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", fmt.Errorf("file:// destination must not specify a remote host")
	}
	if u.Path == "" {
		return "", errors.New("file:// destination has empty path")
	}
	return filepath.Clean(u.Path), nil
}

// youTubeCacheExtensions is the on-disk lookup order. New downloads always
// land as .opus; .mp3 is kept as a fallback so caches from older versions of
// the integration still serve.
var youTubeCacheExtensions = []string{".opus", ".mp3"}

// youTubeCacheLookup returns the extension (".opus" or ".mp3") of an existing
// cached file for the given video ID, or "" if neither is on disk. Preference
// order matches youTubeCacheExtensions.
func youTubeCacheLookup(destDir, id string) string {
	for _, ext := range youTubeCacheExtensions {
		if _, err := os.Stat(filepath.Join(destDir, id+ext)); err == nil {
			return ext
		}
	}
	return ""
}

func youTubePlayURL(id, ext string) string {
	prefix := expandAssetURLMacro(strings.TrimSpace(config.YouTubePlayPrefix))
	return prefix + id + ext
}

func youTubeMaxDurationSeconds() int {
	if config == nil || config.YouTubeMaxDurationSeconds <= 0 {
		return 600
	}
	return config.YouTubeMaxDurationSeconds
}

// ytDlpCookieArgs returns `--cookies <path>` if a cookies file is configured,
// otherwise an empty slice. Placed right after the subcommand-style flags so
// it is inert when unset.
func ytDlpCookieArgs() []string {
	if config == nil {
		return nil
	}
	path := strings.TrimSpace(config.YouTubeCookiesPath)
	if path == "" {
		return nil
	}
	return []string{"--cookies", path}
}

// shellQuote produces a copy-pasteable representation of an argv slice for
// log lines. Not safe for actual shell execution — diagnostic only.
func shellQuote(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		if a == "" || strings.ContainsAny(a, " \t\"'\\$`*?#&|<>(){}[];") {
			b.WriteByte('\'')
			b.WriteString(strings.ReplaceAll(a, "'", `'\''`))
			b.WriteByte('\'')
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

// probeYouTubeDuration shells out to `yt-dlp --print duration --skip-download`
// and returns the video length in seconds (rounded). On failure the returned
// error embeds the full argv and the tail of yt-dlp's stderr so the log line
// can be reproduced by hand.
func probeYouTubeDuration(ctx context.Context, rawURL string) (int, error) {
	args := []string{
		"--print", "duration",
		"--skip-download",
		"--no-playlist",
	}
	args = append(args, ytDlpCookieArgs()...)
	args = append(args, "--", rawURL)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		cmdline := "yt-dlp " + shellQuote(args)
		if stderr != "" {
			return 0, fmt.Errorf("%w (cmd: %s) (stderr: %s)", err, cmdline, stderr)
		}
		return 0, fmt.Errorf("%w (cmd: %s)", err, cmdline)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, fmt.Errorf("yt-dlp returned empty duration (cmd: yt-dlp %s)", shellQuote(args))
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse duration %q: %w", s, err)
	}
	return int(f + 0.5), nil
}

// downloadYouTubeAudio invokes yt-dlp to extract Opus audio into
// destDir/<id>.opus. Opus is YouTube's native audio codec, so yt-dlp can
// passthrough the stream instead of transcoding when the source is already
// opus (most YouTube videos), keeping the download fast and lossless.
func downloadYouTubeAudio(ctx context.Context, rawURL, id, destDir string) error {
	if !youtubeIDRegex.MatchString(id) {
		return errors.New("invalid youtube video id")
	}
	outTmpl := filepath.Join(destDir, id+".%(ext)s")
	args := []string{
		"-x",
		"--audio-format", "opus",
		"--no-playlist",
		"-o", outTmpl,
	}
	args = append(args, ytDlpCookieArgs()...)
	args = append(args, "--", rawURL)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("yt-dlp: %v (cmd: yt-dlp %s) (output: %s)",
			err, shellQuote(args), strings.TrimSpace(string(out)))
	}
	return nil
}

// broadcastYouTubeReady fans the MC packet to the area the request originated
// in. Captured at request time so an area-hopping requester doesn't drag the
// song with them. ext is the on-disk extension (".opus" or ".mp3") of the
// cached file the area will fetch.
func broadcastYouTubeReady(targetArea *area.Area, id, ext string, charID int, showname string) {
	broadcastToArea(targetArea, &packet.MCToClient{
		Name: youTubePlayURL(id, ext), CharID: charID, Showname: showname,
		Looping: "1", Channel: "0", Effects: "0",
	})
}

// tryYouTubePlay handles the YouTube branch of /play. Returns true if the
// caller should stop (URL was a YouTube link, recognized or rejected here).
// Returns false if the URL isn't a YouTube link so cmdPlay can keep going.
func tryYouTubePlay(client *Client, rawURL string) bool {
	id := extractYouTubeID(rawURL)
	if id == "" {
		return false
	}
	if !youTubeEnabled() {
		client.SendServerMessage("YouTube /play is not configured on this server.")
		return true
	}
	destDir, err := youTubeDestDir()
	if err != nil {
		client.SendServerMessage("YouTube /play is misconfigured: " + err.Error())
		return true
	}
	targetArea := client.Area()
	charID := client.CharID()
	showname := client.Showname()

	// Already on disk — play immediately (preferring .opus over .mp3).
	if ext := youTubeCacheLookup(destDir, id); ext != "" {
		broadcastYouTubeReady(targetArea, id, ext, charID, showname)
		return true
	}

	// Try to claim the download slot. If another request beat us to it, wait
	// for that one to finish then broadcast in our area.
	newCh := make(chan struct{})
	if existing, loaded := ytDownloadsInFlight.LoadOrStore(id, newCh); loaded {
		client.SendServerMessage("That YouTube video is already being downloaded — it'll play here when it's ready.")
		go func() {
			<-existing.(chan struct{})
			if ext := youTubeCacheLookup(destDir, id); ext != "" {
				broadcastYouTubeReady(targetArea, id, ext, charID, showname)
			}
		}()
		return true
	}

	go runYouTubeDownload(client, rawURL, id, destDir, targetArea, charID, showname, newCh)
	return true
}

func runYouTubeDownload(client *Client, rawURL, id, destDir string, targetArea *area.Area, charID int, showname string, done chan struct{}) {
	defer func() {
		close(done)
		ytDownloadsInFlight.Delete(id)
	}()

	probeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	secs, err := probeYouTubeDuration(probeCtx, rawURL)
	cancel()
	if err != nil {
		logger.LogErrorf("youtube: probe failed for %s: %v", id, err)
		client.SendServerMessage("Could not query that YouTube link.")
		return
	}
	maxDur := youTubeMaxDurationSeconds()
	if secs > maxDur {
		client.SendServerMessage(fmt.Sprintf("That video is %ds long — the limit is %ds.", secs, maxDur))
		return
	}
	client.SendServerMessage(fmt.Sprintf("Processing YouTube audio (%ds)… it'll play here once ready.", secs))

	dlCtx, dlCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer dlCancel()
	if err := downloadYouTubeAudio(dlCtx, rawURL, id, destDir); err != nil {
		logger.LogErrorf("youtube: download failed for %s: %v", id, err)
		client.SendServerMessage("YouTube download failed.")
		return
	}
	ext := youTubeCacheLookup(destDir, id)
	if ext == "" {
		logger.LogErrorf("youtube: expected .opus/.mp3 file missing after download in %s for %s", destDir, id)
		client.SendServerMessage("YouTube download finished but the file is missing.")
		return
	}
	broadcastYouTubeReady(targetArea, id, ext, charID, showname)
}
