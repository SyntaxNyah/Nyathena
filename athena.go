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

package main

import (
	"embed"
	"flag"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/MangosArentLiterature/Athena/internal/athena"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/packet"
	"github.com/MangosArentLiterature/Athena/internal/settings"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
)

// schemaFS embeds the AO2 packet JSON schemas so MS validation works from the
// compiled binary regardless of the working directory.
//
//go:embed schemas/MSRequest.schema.json schemas/MSBroadcast.schema.json
var schemaFS embed.FS

var (
	configFlag = flag.String("c", "config", "path to config directory")
	cliFlag    = flag.Bool("nocli", false, "disables listening for commands on stdin")
	tuiFlag    = flag.Bool("tui", false, "enables a read-only terminal dashboard; implies -nocli and suppresses stdout logging while active")
)

// loadMSSchemas compiles the embedded MS request/broadcast schemas and installs
// them as the active validators. A failure is non-fatal: validation simply
// stays disabled (the server runs exactly as before the schemas existed).
func loadMSSchemas() error {
	req, err := schemaFS.ReadFile("schemas/MSRequest.schema.json")
	if err != nil {
		return err
	}
	bcast, err := schemaFS.ReadFile("schemas/MSBroadcast.schema.json")
	if err != nil {
		return err
	}
	return packet.CompileMSSchemas(req, bcast)
}

func main() {
	flag.Parse()
	if *configFlag != "" {
		settings.ConfigPath = path.Clean(*configFlag)
	}
	config, err := settings.GetConfig()
	if err != nil {
		logger.LogFatalf("failed to read config: %v", err)
		os.Exit(1)
	}
	logger.LogPath = path.Clean(config.LogDir)
	if _, err := os.Stat(logger.LogPath); os.IsNotExist(err) {
		if err := os.Mkdir(logger.LogPath, 0755); err != nil {
			logger.LogErrorf("failed to make logdir: %v", err)
		}
	}

	switch config.LogLevel {
	case "info":
		logger.CurrentLevel = logger.Info
	case "warning":
		logger.CurrentLevel = logger.Warning
	case "error":
		logger.CurrentLevel = logger.Error
	case "fatal":
		logger.CurrentLevel = logger.Fatal
	}
	logger.LogStdOut = sliceutil.ContainsString(config.LogMethods, "stdout")
	logger.LogFile = sliceutil.ContainsString(config.LogMethods, "log_file")
	db.DBPath = settings.ConfigPath + "/athena.db"

	if err := loadMSSchemas(); err != nil {
		logger.LogWarningf("MS JSON-schema validation disabled: %v", err)
	} else {
		logger.LogInfo("Loaded MS JSON schemas (request/broadcast validation enabled).")
	}

	err = athena.InitServer(config)
	if err != nil {
		logger.LogFatalf("Failed to initalize server: %v", err)
		athena.CleanupServer()
		os.Exit(1)
	}
	logger.LogInfo("Started server.")
	go athena.ListenTCP()
	go athena.StartDiscordBot()

	// When both WS and WSS are enabled with the same port (common in reverse proxy setups),
	// only start one listener to avoid "address already in use" error
	if config.EnableWS && config.EnableWSS && config.WSPort == config.WSSPort {
		logger.LogInfof("WS and WSS using same port %d, starting single listener", config.WSPort)
		go athena.ListenWS()
	} else {
		if config.EnableWS {
			go athena.ListenWS()
		}
		if config.EnableWSS {
			go athena.ListenWSS()
		}
	}
	// The TUI owns stdout and is read-only, so when it's enabled we skip the
	// stdin CLI entirely. Operators who want both can run the TUI in one
	// terminal pane and a second server instance for interactive tasks, or
	// just launch without -tui.
	// Either the -tui CLI flag OR enable_tui=true in config.toml turns it on;
	// the flag is a one-off override, the config entry is the persistent
	// default.
	tuiEnabled := *tuiFlag || config.EnableTUI
	tuiStop := make(chan struct{})
	if tuiEnabled {
		go athena.StartTUI(tuiStop)
	} else if !*cliFlag {
		go athena.ListenInput()
	}

	// SIGHUP triggers a full hot-reload of characters.txt (append-only),
	// music.txt, cdns.txt, backgrounds.txt, parrot.txt, 8ball.txt,
	// banned_words.txt and the config.toml motd/desc fields. Anything that
	// affects listeners, rate limits, area state or cached packets still
	// requires a full restart.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			summary, err := athena.ReloadConfig()
			if err != nil {
				logger.LogErrorf("SIGHUP reload failed: %v", err)
				continue
			}
			logger.LogInfof("SIGHUP reload ok: %s", summary)
		}
	}()

	stop := make(chan (os.Signal), 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	restart := false
	select {
	case <-stop:
		break
	case err := <-athena.FatalError:
		logger.LogFatal(err.Error())
		break
	case <-athena.RestartRequest:
		restart = true
	}
	close(tuiStop)
	athena.CleanupServer()
	if restart {
		logger.LogInfo("Restarting server...")
		executable, err := os.Executable()
		if err != nil {
			logger.LogFatalf("Failed to get executable path for restart: %v", err)
			os.Exit(1)
		}
		if err := syscall.Exec(executable, os.Args, os.Environ()); err != nil {
			logger.LogFatalf("Failed to restart server: %v", err)
			os.Exit(1)
		}
	}
	logger.LogInfo("Stopping server.")
}
