package athena

import (
	"strings"
	"sync"
	"time"

	aolib "github.com/AO-Underground/aolib/go/v2"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/plugin"
)

// Spooky-names plugin bridge.
//
// An optional external program that rewrites the display names the server
// forwards to clients -- OOC names, IC shownames and pair names, and the
// player-list name/showname/character entries. It speaks the same
// newline-delimited JSON protocol as the captcha plugin (the "rewrite" op),
// so it can be written in any language and cannot crash the server.
//
// The server caches the rewrite per distinct name, so the plugin is consulted
// once per name rather than once per message, and a down or slow plugin
// degrades to passing names through unchanged.

var (
	spookyPlugin     *plugin.Plugin
	spookyPluginOnce sync.Once

	spookyCacheMu sync.Mutex
	spookyCache   = make(map[string]string)
)

// spookyCacheCap bounds the per-name cache. Distinct display names are few
// (a server carries hundreds, not millions); past the cap the whole cache is
// dropped and rebuilt, which costs one plugin round trip per fresh name.
const spookyCacheCap = 4096

// initSpookyPlugin starts the configured spooky-names plugin. Called once
// from InitServer; a blank spooky_plugin leaves the feature inert.
func initSpookyPlugin() {
	spookyPluginOnce.Do(func() {
		if config == nil || strings.TrimSpace(config.SpookyPlugin) == "" {
			return
		}
		timeout := time.Duration(config.SpookyPluginTimeout) * time.Millisecond
		spookyPlugin = plugin.New("spooky-names", strings.TrimSpace(config.SpookyPlugin), timeout, logger.LogInfof)
		spookyPlugin.Start()
		logger.LogInfof("Spooky-names plugin configured: %v", config.SpookyPlugin)
	})
}

// spookyPluginActive reports whether a plugin is configured and up.
func spookyPluginActive() bool {
	return spookyPlugin != nil && spookyPlugin.Running()
}

// spookyRewriteReq asks the plugin to haunt one name.
type spookyRewriteReq struct {
	Name string `json:"name"`
}

// spookyRewriteResp is the haunted name.
type spookyRewriteResp struct {
	Name string `json:"name"`
}

// hauntName rewrites one display name through the spooky-names plugin.
// Results are cached per distinct name; a plugin error leaves the name
// unchanged (and the unchanged result is cached too, so a dead plugin costs
// one timeout per name rather than one per message).
func hauntName(name string) string {
	if name == "" || !spookyPluginActive() {
		return name
	}

	spookyCacheMu.Lock()
	if cached, ok := spookyCache[name]; ok {
		spookyCacheMu.Unlock()
		return cached
	}
	spookyCacheMu.Unlock()

	out := name
	var resp spookyRewriteResp
	if err := spookyPlugin.Call("rewrite", spookyRewriteReq{Name: name}, &resp); err != nil {
		logger.LogWarningf("spooky-names plugin rewrite failed, leaving the name unchanged: %v", err)
	} else if strings.TrimSpace(resp.Name) != "" {
		out = resp.Name
	}

	spookyCacheMu.Lock()
	if len(spookyCache) >= spookyCacheCap {
		spookyCache = make(map[string]string)
	}
	spookyCache[name] = out
	spookyCacheMu.Unlock()
	return out
}

// hauntOutgoing rewrites every display-name field on an outbound packet. The
// rewrite is idempotent (a name that already carries a spooky emoji is left
// alone), so the shared packet pointer the broadcast helpers fan out to every
// recipient stays consistent across recipients.
func hauntOutgoing(p aolib.Outgoing) {
	if !spookyPluginActive() {
		return
	}
	switch t := p.(type) {
	case *aolib.CTToClient:
		if t.IsFromServer {
			return
		}
		t.Name = hauntName(t.Name)
	case *aolib.MSToClient:
		t.Showname = hauntName(t.Showname)
		if t.PairedName != "" {
			t.PairedName = hauntName(t.PairedName)
		}
	case *aolib.PU:
		switch t.Type {
		case aolib.PlayerDataTypeOocName, aolib.PlayerDataTypeCharName, aolib.PlayerDataTypeShowname:
			t.Data = hauntName(t.Data)
		}
	}
}
