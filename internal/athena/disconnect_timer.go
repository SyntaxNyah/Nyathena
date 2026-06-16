/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: the self-service idle auto-disconnect timer.

   /dc and /dctime let any player opt into being automatically disconnected
   after a chosen stretch of inactivity — the same convenience WebAO's idle
   timeout provides, but user-controlled. It is:

     • opt-in     — OFF by default; nobody is ever disconnected unless they
                    personally enable it.
     • isolated   — the watcher goroutine only ever closes the connection of
                    the client that set it. It sends no packet to anyone else
                    and cannot affect another player.
     • forgiving  — sending an IC or OOC message resets the countdown, so the
                    timer fires only after genuine inactivity (AFK), exactly
                    like the WebAO behaviour it mirrors.

   With no minutes argument the timer defaults to a 1-hour countdown. */

package athena

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

const (
	// dcDefaultMinutes is the countdown used when /dc / /dctime is run without
	// an explicit number of minutes.
	dcDefaultMinutes = 60
	// dcMaxMinutes caps the idle window at 7 days so the deadline arithmetic
	// can never overflow and a typo can't park a connection open forever.
	dcMaxMinutes = 7 * 24 * 60
	// dcWatchInterval is how often the watcher re-checks idle time. The
	// disconnect therefore lands within this much of the configured deadline,
	// which is plenty precise for an AFK timer and keeps the goroutine cheap.
	dcWatchInterval = 10 * time.Second
)

// dcTouchActivity records that the client just did something (sent IC/OOC),
// resetting the idle countdown. Cheap atomic store; safe to call on the hot
// path. A no-op effect-wise when the timer is disabled.
func (client *Client) dcTouchActivity() {
	client.dcLastActivityNano.Store(time.Now().UnixNano())
}

// startDCWatcher lazily spawns the single per-connection watcher goroutine the
// first time the client enables the timer. The goroutine lives for the rest of
// the connection (exiting on client.done) and simply no-ops whenever the timer
// is disabled, so re-enabling never needs to respawn it and there is no
// start/stop race to manage.
func (client *Client) startDCWatcher() {
	if !client.dcWatcherStarted.CompareAndSwap(false, true) {
		return
	}
	go client.dcIdleWatcher()
}

// dcIdleWatcher disconnects the client once it has been idle for its configured
// /dc window. Only ever touches this one connection.
func (client *Client) dcIdleWatcher() {
	ticker := time.NewTicker(dcWatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-client.done:
			return
		case <-ticker.C:
			mins := client.dcIdleMinutes.Load()
			if mins <= 0 {
				continue // disabled — keep watching in case it's re-enabled
			}
			last := client.dcLastActivityNano.Load()
			if last == 0 {
				continue
			}
			idle := time.Now().UnixNano() - last
			if idle < int64(time.Duration(mins)*time.Minute) {
				continue
			}
			client.dcIdleMinutes.Store(0) // fire once
			client.SendServerMessage(fmt.Sprintf(
				"⏱ Auto-disconnecting: you were idle for %d minute(s) (your /dctime setting). Reconnect any time.", mins))
			logger.LogInfof("Client (IPID:%v UID:%v) auto-disconnected by its own /dctime idle timer (%d min)",
				client.Ipid(), client.Uid(), mins)
			client.markClosed()
			return
		}
	}
}

// cmdDC handles /dc and /dctime — set, inspect, or clear the personal idle
// auto-disconnect timer. Available to everyone; affects only the caller.
//
//	/dctime            → enable with the default 1-hour countdown
//	/dctime 30         → disconnect after 30 minutes of inactivity
//	/dctime off        → turn the timer off (also: 0, stop, cancel, disable)
//	/dctime status     → show the current setting
func cmdDC(client *Client, args []string, _ string) {
	// No argument: enable with the documented 1-hour default.
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		client.dcSetIdleMinutes(dcDefaultMinutes)
		client.SendServerMessage(fmt.Sprintf(
			"⏱ Idle auto-disconnect enabled: you'll be disconnected after %d minutes of inactivity. "+
				"Sending an IC or OOC message resets the countdown. Use /dctime off to cancel.", dcDefaultMinutes))
		return
	}

	arg := strings.ToLower(strings.TrimSpace(args[0]))
	switch arg {
	case "off", "stop", "cancel", "disable", "0", "none":
		if client.dcIdleMinutes.Load() == 0 {
			client.SendServerMessage("Your idle auto-disconnect timer is already off.")
			return
		}
		client.dcIdleMinutes.Store(0)
		client.SendServerMessage("⏱ Idle auto-disconnect timer cancelled.")
		return
	case "status", "show":
		if mins := client.dcIdleMinutes.Load(); mins > 0 {
			client.SendServerMessage(fmt.Sprintf(
				"⏱ Idle auto-disconnect is ON: %d minute(s) of inactivity. Use /dctime off to cancel.", mins))
		} else {
			client.SendServerMessage("Idle auto-disconnect is OFF. Use /dctime <minutes> (or just /dctime for 1 hour) to enable it.")
		}
		return
	}

	mins, err := strconv.Atoi(arg)
	if err != nil || mins <= 0 {
		client.SendServerMessage("Usage: /dctime <minutes> | off | status   (e.g. /dctime 30). Run /dctime with no number for a 1-hour timer.")
		return
	}
	if mins > dcMaxMinutes {
		mins = dcMaxMinutes
	}
	client.dcSetIdleMinutes(int64(mins))
	client.SendServerMessage(fmt.Sprintf(
		"⏱ Idle auto-disconnect enabled: you'll be disconnected after %d minute(s) of inactivity. "+
			"Sending an IC or OOC message resets the countdown. Use /dctime off to cancel.", mins))
}

// dcSetIdleMinutes enables the timer at the given window, reseeds the activity
// clock to "now" (so the user gets the full window), and ensures the watcher
// goroutine is running.
func (client *Client) dcSetIdleMinutes(mins int64) {
	client.dcTouchActivity()
	client.dcIdleMinutes.Store(mins)
	client.startDCWatcher()
}
