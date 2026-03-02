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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"syscall"

	"github.com/MangosArentLiterature/Athena/internal/athena"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/settings"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
)

var (
	configFlag   = flag.String("c", "config", "path to config directory")
	netDebugFlag = flag.Bool("netdebug", false, "log raw network traffic")
	cliFlag      = flag.Bool("nocli", false, "disables listening for commands on stdin")
)

func main() {
	// Catch any panic on the main goroutine and log it before exiting.
	defer func() {
		if r := recover(); r != nil {
			logger.WriteCrashLog(r, debug.Stack())
			os.Exit(2)
		}
	}()

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
	case "debug":
		logger.CurrentLevel = logger.Debug
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
	logger.DebugNetwork = *netDebugFlag
	logger.EnableNetworkLog = config.EnableNetworkLog
	db.DBPath = settings.ConfigPath + "/athena.db"

	err = athena.InitServer(config)
	if err != nil {
		logger.LogFatalf("Failed to initalize server: %v", err)
		athena.CleanupServer()
		os.Exit(1)
	}
	logger.LogInfo("Started server.")
	goWithCrashLog("ListenTCP", athena.ListenTCP)
	goWithCrashLog("StartDiscordBot", athena.StartDiscordBot)

	// When both WS and WSS are enabled with the same port (common in reverse proxy setups),
	// only start one listener to avoid "address already in use" error
	if config.EnableWS && config.EnableWSS && config.WSPort == config.WSSPort {
		logger.LogDebugf("WS and WSS using same port %d, starting single listener", config.WSPort)
		goWithCrashLog("ListenWS", athena.ListenWS)
	} else {
		if config.EnableWS {
			goWithCrashLog("ListenWS", athena.ListenWS)
		}
		if config.EnableWSS {
			goWithCrashLog("ListenWSS", athena.ListenWSS)
		}
	}
	if !*cliFlag {
		goWithCrashLog("ListenInput", athena.ListenInput)
	}
	stop := make(chan (os.Signal), 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		break
	case err := <-athena.FatalError:
		logger.LogFatal(err.Error())
		break
	}
	athena.CleanupServer()
	logger.LogInfo("Stopping server.")
}

// goWithCrashLog launches fn in a new goroutine.  If fn panics the crash is
// logged to crash-<ts>.log and network.log, then a fatal error is sent so the
// server shuts down cleanly.
func goWithCrashLog(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.WriteCrashLog(r, debug.Stack())
				select {
				case athena.FatalError <- fmt.Errorf("panic in %s: %v", name, r):
				default:
				}
			}
		}()
		fn()
	}()
}
