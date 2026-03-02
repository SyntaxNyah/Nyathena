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

package athena

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// TestRateLimitDisabled tests that rate limiting can be disabled
func TestRateLimitDisabled(t *testing.T) {
	// Backup original config
	oldConfig := config
	defer func() { config = oldConfig }()

	// Set rate limit to 0 (disabled)
	config = &settings.Config{}
	config.RateLimit = 0

	client := &Client{
		msgTimestamps: []time.Time{},
	}

	// Should never be rate limited when disabled
	for i := 0; i < 1000; i++ {
		if client.CheckRateLimit() {
			t.Errorf("Client was rate limited when rate limiting is disabled")
			return
		}
	}
}

// TestRateLimitBasic tests basic rate limiting functionality
func TestRateLimitBasic(t *testing.T) {
	// Backup original config
	oldConfig := config
	defer func() { config = oldConfig }()

	// Set rate limit to 5 messages per 1 second
	config = &settings.Config{}
	config.RateLimit = 5
	config.RateLimitWindow = 1

	client := &Client{
		msgTimestamps: []time.Time{},
	}

	// Send 5 messages - should all succeed
	for i := 0; i < 5; i++ {
		if client.CheckRateLimit() {
			t.Errorf("Client was rate limited on message %d (limit is 5)", i+1)
			return
		}
	}

	// 6th message should trigger rate limit
	if !client.CheckRateLimit() {
		t.Errorf("Client was not rate limited after exceeding limit")
	}
}

// TestRateLimitWindowSliding tests that the sliding window works correctly
func TestRateLimitWindowSliding(t *testing.T) {
	// Backup original config
	oldConfig := config
	defer func() { config = oldConfig }()

	// Set rate limit to 3 messages per 2 seconds
	config = &settings.Config{}
	config.RateLimit = 3
	config.RateLimitWindow = 2

	client := &Client{
		msgTimestamps: []time.Time{},
	}

	// Send 3 messages quickly
	for i := 0; i < 3; i++ {
		if client.CheckRateLimit() {
			t.Errorf("Client was rate limited on message %d (limit is 3)", i+1)
			return
		}
	}

	// 4th message should trigger rate limit
	if !client.CheckRateLimit() {
		t.Errorf("Client was not rate limited after exceeding limit")
		return
	}

	// Wait for window to expire
	time.Sleep(time.Duration(config.RateLimitWindow)*time.Second + 100*time.Millisecond)

	// Should be able to send again after window expires
	if client.CheckRateLimit() {
		t.Errorf("Client was rate limited after window expired")
	}
}

// TestRateLimitConcurrency tests rate limiting with concurrent access
func TestRateLimitConcurrency(t *testing.T) {
	// Backup original config
	oldConfig := config
	defer func() { config = oldConfig }()

	// Set rate limit to 10 messages per 1 second
	config = &settings.Config{}
	config.RateLimit = 10
	config.RateLimitWindow = 1

	client := &Client{
		msgTimestamps: []time.Time{},
	}

	// Simulate concurrent access
	done := make(chan bool, 20)
	var exceeded int32

	for i := 0; i < 20; i++ {
		go func() {
			if client.CheckRateLimit() {
				atomic.AddInt32(&exceeded, 1)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should have at least 10 messages to exceed limit
	exceededCount := atomic.LoadInt32(&exceeded)
	if exceededCount < 10 {
		t.Errorf("Expected at least 10 messages to exceed limit, got %d", exceededCount)
	}
}

// TestModcallCooldownDisabled tests that modcall cooldown can be disabled
func TestModcallCooldownDisabled(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ModcallCooldown = 0

	client := &Client{}

	// Should never be limited when cooldown is disabled
	for i := 0; i < 10; i++ {
		if limited, _ := client.CheckModcallCooldown(); limited {
			t.Errorf("Client was modcall-limited when cooldown is disabled")
			return
		}
		client.SetLastModcallTime()
	}
}

// TestModcallCooldownEnforced tests that the modcall cooldown is enforced
func TestModcallCooldownEnforced(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ModcallCooldown = 60 // 60 second cooldown

	client := &Client{}

	// First modcall should be allowed
	if limited, _ := client.CheckModcallCooldown(); limited {
		t.Errorf("First modcall was blocked unexpectedly")
		return
	}
	client.SetLastModcallTime()

	// Immediate second modcall should be blocked
	if limited, remaining := client.CheckModcallCooldown(); !limited {
		t.Errorf("Second modcall was not blocked within cooldown period")
	} else if remaining <= 0 || remaining > 60 {
		t.Errorf("Unexpected remaining seconds: %d", remaining)
	}
}

// TestModcallCooldownExpires tests that the cooldown expires correctly
func TestModcallCooldownExpires(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ModcallCooldown = 1 // 1 second cooldown

	client := &Client{}

	// First modcall
	if limited, _ := client.CheckModcallCooldown(); limited {
		t.Errorf("First modcall was blocked unexpectedly")
		return
	}
	client.SetLastModcallTime()

	// Should be blocked immediately
	if limited, _ := client.CheckModcallCooldown(); !limited {
		t.Errorf("Modcall was not blocked within cooldown period")
		return
	}

	// Wait for cooldown to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if limited, _ := client.CheckModcallCooldown(); limited {
		t.Errorf("Modcall was blocked after cooldown expired")
	}
}

// TestRateLimitMemoryEfficiency tests that old timestamps are cleaned up
func TestRateLimitMemoryEfficiency(t *testing.T) {
	// Backup original config
	oldConfig := config
	defer func() { config = oldConfig }()

	// Set rate limit to 5 messages per 1 second
	config = &settings.Config{}
	config.RateLimit = 5
	config.RateLimitWindow = 1

	client := &Client{
		msgTimestamps: []time.Time{},
	}

	// Fill up the rate limit
	for i := 0; i < 5; i++ {
		client.CheckRateLimit()
	}

	initialLen := len(client.msgTimestamps)

	// Wait for window to expire
	time.Sleep(time.Duration(config.RateLimitWindow)*time.Second + 100*time.Millisecond)

	// Add one more message - should clean up old timestamps
	client.CheckRateLimit()

	// Should have removed old timestamps
	if len(client.msgTimestamps) >= initialLen {
		t.Errorf("Old timestamps were not cleaned up. Initial: %d, Current: %d", initialLen, len(client.msgTimestamps))
	}
}

// TestConnRateLimitDisabled tests that connection rate limiting can be disabled.
func TestConnRateLimitDisabled(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ConnRateLimit = 0

	// Reset tracker for a clean test
	connTracker.mu.Lock()
	connTracker.timestamps = make(map[string][]time.Time)
	connTracker.mu.Unlock()

	ipid := "testipDisabled"
	for i := 0; i < 100; i++ {
		if checkConnRateLimit(ipid) {
			t.Errorf("Connection was rejected when connection rate limiting is disabled (attempt %d)", i+1)
			return
		}
	}
}

// TestConnRateLimitBasic tests that the connection rate limit is enforced.
func TestConnRateLimitBasic(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ConnRateLimit = 5
	config.ConnRateLimitWindow = 5

	// Reset tracker for a clean test
	connTracker.mu.Lock()
	connTracker.timestamps = make(map[string][]time.Time)
	connTracker.mu.Unlock()

	ipid := "testipBasic"

	// First 5 connections should be allowed
	for i := 0; i < 5; i++ {
		if checkConnRateLimit(ipid) {
			t.Errorf("Connection %d was rejected (limit is 5)", i+1)
			return
		}
	}

	// 6th connection should be rejected
	if !checkConnRateLimit(ipid) {
		t.Errorf("Connection was not rejected after exceeding the limit")
	}
}

// TestConnRateLimitWindowExpiry tests that the connection rate window resets after expiry.
func TestConnRateLimitWindowExpiry(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ConnRateLimit = 3
	config.ConnRateLimitWindow = 1

	// Reset tracker for a clean test
	connTracker.mu.Lock()
	connTracker.timestamps = make(map[string][]time.Time)
	connTracker.mu.Unlock()

	ipid := "testipExpiry"

	// Fill up the limit
	for i := 0; i < 3; i++ {
		if checkConnRateLimit(ipid) {
			t.Errorf("Connection %d was rejected prematurely", i+1)
			return
		}
	}

	// Should be rejected now
	if !checkConnRateLimit(ipid) {
		t.Errorf("Connection was not rejected after reaching the limit")
		return
	}

	// Wait for window to expire
	time.Sleep(time.Duration(config.ConnRateLimitWindow)*time.Second + 100*time.Millisecond)

	// Should be allowed again after window expiry
	if checkConnRateLimit(ipid) {
		t.Errorf("Connection was rejected after window expired")
	}
}

// TestConnRateLimitIsolation tests that different IPs are tracked independently.
func TestConnRateLimitIsolation(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ConnRateLimit = 2
	config.ConnRateLimitWindow = 10

	// Reset tracker for a clean test
	connTracker.mu.Lock()
	connTracker.timestamps = make(map[string][]time.Time)
	connTracker.mu.Unlock()

	ipid1 := "testipIso1"
	ipid2 := "testipIso2"

	// Fill up limit for ipid1
	checkConnRateLimit(ipid1)
	checkConnRateLimit(ipid1)
	if !checkConnRateLimit(ipid1) {
		t.Errorf("ipid1 was not rejected after exceeding the limit")
		return
	}

	// ipid2 should not be affected
	if checkConnRateLimit(ipid2) {
		t.Errorf("ipid2 was rejected even though it has not exceeded its limit")
	}
}

// TestOOCRateLimitDisabled tests that OOC rate limiting can be disabled.
func TestOOCRateLimitDisabled(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 0

	client := &Client{
		oocMsgTimestamps: []time.Time{},
	}

	// Should never be rate limited when disabled
	for i := 0; i < 100; i++ {
		if client.CheckOOCRateLimit() {
			t.Errorf("Client was OOC rate limited when OOC rate limiting is disabled (attempt %d)", i+1)
			return
		}
	}
}

// TestOOCRateLimitBasic tests that the OOC rate limit is enforced.
func TestOOCRateLimitBasic(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 4
	config.OOCRateLimitWindow = 1

	client := &Client{
		oocMsgTimestamps: []time.Time{},
	}

	// First 4 OOC messages should be allowed
	for i := 0; i < 4; i++ {
		if client.CheckOOCRateLimit() {
			t.Errorf("OOC message %d was blocked (limit is 4)", i+1)
			return
		}
	}

	// 5th OOC message should be blocked
	if !client.CheckOOCRateLimit() {
		t.Errorf("OOC message was not blocked after exceeding the limit")
	}
}

// TestOOCRateLimitWindowExpiry tests that the OOC rate window resets after expiry.
func TestOOCRateLimitWindowExpiry(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 4
	config.OOCRateLimitWindow = 1

	client := &Client{
		oocMsgTimestamps: []time.Time{},
	}

	// Fill up the limit
	for i := 0; i < 4; i++ {
		if client.CheckOOCRateLimit() {
			t.Errorf("OOC message %d was blocked prematurely", i+1)
			return
		}
	}

	// Should be blocked now
	if !client.CheckOOCRateLimit() {
		t.Errorf("OOC message was not blocked after reaching the limit")
		return
	}

	// Wait for window to expire
	time.Sleep(time.Duration(config.OOCRateLimitWindow)*time.Second + 100*time.Millisecond)

	// Should be allowed again
	if client.CheckOOCRateLimit() {
		t.Errorf("OOC message was blocked after window expired")
	}
}

// TestOOCRateLimitIndependentFromGeneral tests that the OOC rate limit is tracked
// independently from the general message rate limit.
func TestOOCRateLimitIndependentFromGeneral(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.RateLimit = 20
	config.RateLimitWindow = 10
	config.OOCRateLimit = 4
	config.OOCRateLimitWindow = 1

	client := &Client{
		msgTimestamps:    []time.Time{},
		oocMsgTimestamps: []time.Time{},
	}

	// Exhaust the OOC rate limit
	for i := 0; i < 4; i++ {
		client.CheckOOCRateLimit()
	}

	// OOC limit should be exceeded
	if !client.CheckOOCRateLimit() {
		t.Errorf("OOC rate limit was not enforced after 4 messages")
	}

	// General rate limit should not be affected by OOC messages
	if client.CheckRateLimit() {
		t.Errorf("General rate limit was incorrectly triggered by OOC messages")
	}
}

func TestCharSelectRateLimit(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.RateLimit = 3
	config.RateLimitWindow = 1

	client := &Client{
		msgTimestamps: []time.Time{},
	}

	// First 3 charselect-equivalent calls should be allowed.
	for i := 0; i < 3; i++ {
		if client.CheckRateLimit() {
			t.Errorf("charselect was rate limited on call %d (limit is 3)", i+1)
			return
		}
	}

	// 4th call should be rejected, just as pktChangeChar would reject it.
	if !client.CheckRateLimit() {
		t.Errorf("charselect was not rate limited after exceeding the limit")
	}
}

// TestIPModcallCooldownDisabled tests that the per-IP modcall cooldown can be disabled.
func TestIPModcallCooldownDisabled(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ModcallCooldown = 0

	ipModcallTracker.mu.Lock()
	ipModcallTracker.times = make(map[string]time.Time)
	ipModcallTracker.mu.Unlock()

	ipid := "testIPModcallDisabled"
	for i := 0; i < 10; i++ {
		if limited, _ := checkIPModcallCooldown(ipid); limited {
			t.Errorf("IP was modcall-limited when cooldown is disabled")
			return
		}
		setIPModcallTime(ipid)
	}
}

// TestIPModcallCooldownEnforced tests that the per-IP modcall cooldown is enforced across connections.
func TestIPModcallCooldownEnforced(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ModcallCooldown = 60

	ipModcallTracker.mu.Lock()
	ipModcallTracker.times = make(map[string]time.Time)
	ipModcallTracker.mu.Unlock()

	ipid := "testIPModcallEnforced"

	if limited, _ := checkIPModcallCooldown(ipid); limited {
		t.Errorf("First modcall was blocked unexpectedly")
		return
	}
	setIPModcallTime(ipid)

	if limited, remaining := checkIPModcallCooldown(ipid); !limited {
		t.Errorf("Second modcall was not blocked within cooldown period")
	} else if remaining <= 0 || remaining > 60 {
		t.Errorf("Unexpected remaining seconds: %d", remaining)
	}
}

// TestIPModcallCooldownPersistsAcrossConnections tests that per-IP cooldown is not reset
// when a new client is created for the same IP (simulating a new connection).
func TestIPModcallCooldownPersistsAcrossConnections(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.ModcallCooldown = 60

	ipModcallTracker.mu.Lock()
	ipModcallTracker.times = make(map[string]time.Time)
	ipModcallTracker.mu.Unlock()

	ipid := "testIPModcallPersists"

	// First modcall allowed
	if limited, _ := checkIPModcallCooldown(ipid); limited {
		t.Errorf("First modcall was blocked unexpectedly")
		return
	}
	setIPModcallTime(ipid)

	// Simulate a new connection (new client) with the same IPID – cooldown must still apply.
	if limited, _ := checkIPModcallCooldown(ipid); !limited {
		t.Errorf("Modcall cooldown did not persist across a simulated new connection")
	}
}

// TestIPOOCRateLimitDisabled tests that per-IP OOC rate limiting can be disabled.
func TestIPOOCRateLimitDisabled(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 0

	ipOOCTracker.mu.Lock()
	ipOOCTracker.timestamps = make(map[string][]time.Time)
	ipOOCTracker.mu.Unlock()

	ipid := "testIPOOCDisabled"
	for i := 0; i < 100; i++ {
		if checkIPOOCRateLimit(ipid) {
			t.Errorf("IP was OOC rate limited when OOC rate limiting is disabled (attempt %d)", i+1)
			return
		}
	}
}

// TestIPOOCRateLimitBasic tests that the per-IP OOC rate limit is enforced.
func TestIPOOCRateLimitBasic(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 4
	config.OOCRateLimitWindow = 1

	ipOOCTracker.mu.Lock()
	ipOOCTracker.timestamps = make(map[string][]time.Time)
	ipOOCTracker.mu.Unlock()

	ipid := "testIPOOCBasic"

	for i := 0; i < 4; i++ {
		if checkIPOOCRateLimit(ipid) {
			t.Errorf("OOC message %d was blocked (limit is 4)", i+1)
			return
		}
	}

	if !checkIPOOCRateLimit(ipid) {
		t.Errorf("OOC message was not blocked after exceeding the limit")
	}
}

// TestIPOOCRateLimitPersistsAcrossConnections tests that per-IP OOC rate limiting is not reset
// when a new client is created for the same IP (simulating a new connection).
func TestIPOOCRateLimitPersistsAcrossConnections(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 4
	config.OOCRateLimitWindow = 5

	ipOOCTracker.mu.Lock()
	ipOOCTracker.timestamps = make(map[string][]time.Time)
	ipOOCTracker.mu.Unlock()

	ipid := "testIPOOCPersists"

	// Exhaust limit
	for i := 0; i < 4; i++ {
		checkIPOOCRateLimit(ipid)
	}

	// Simulate a new connection (new client) – rate limit must still apply.
	if !checkIPOOCRateLimit(ipid) {
		t.Errorf("OOC rate limit did not persist across a simulated new connection")
	}
}

// TestIPOOCRateLimitWindowExpiry tests that the per-IP OOC rate window resets after expiry.
func TestIPOOCRateLimitWindowExpiry(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.OOCRateLimit = 4
	config.OOCRateLimitWindow = 1

	ipOOCTracker.mu.Lock()
	ipOOCTracker.timestamps = make(map[string][]time.Time)
	ipOOCTracker.mu.Unlock()

	ipid := "testIPOOCExpiry"

	for i := 0; i < 4; i++ {
		checkIPOOCRateLimit(ipid)
	}

	if !checkIPOOCRateLimit(ipid) {
		t.Errorf("OOC message was not blocked after reaching the limit")
		return
	}

	time.Sleep(time.Duration(config.OOCRateLimitWindow)*time.Second + 100*time.Millisecond)

	if checkIPOOCRateLimit(ipid) {
		t.Errorf("OOC message was blocked after window expired")
	}
}

// TestIPPingRateLimitDisabled tests that ping rate limiting can be disabled.
func TestIPPingRateLimitDisabled(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.PingRateLimit = 0

	ipPingTracker.mu.Lock()
	ipPingTracker.timestamps = make(map[string][]time.Time)
	ipPingTracker.mu.Unlock()

	ipid := "testIPPingDisabled"
	for i := 0; i < 100; i++ {
		if checkIPPingRateLimit(ipid) {
			t.Errorf("IP was ping rate limited when ping rate limiting is disabled (attempt %d)", i+1)
			return
		}
	}
}

// TestIPPingRateLimitBasic tests that the per-IP ping rate limit is enforced.
func TestIPPingRateLimitBasic(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.PingRateLimit = 10
	config.PingRateLimitWindow = 5

	ipPingTracker.mu.Lock()
	ipPingTracker.timestamps = make(map[string][]time.Time)
	ipPingTracker.mu.Unlock()

	ipid := "testIPPingBasic"

	for i := 0; i < 10; i++ {
		if checkIPPingRateLimit(ipid) {
			t.Errorf("Ping %d was blocked (limit is 10)", i+1)
			return
		}
	}

	if !checkIPPingRateLimit(ipid) {
		t.Errorf("Ping was not blocked after exceeding the limit")
	}
}

// TestIPPingRateLimitPersistsAcrossConnections tests that per-IP ping rate limiting is not reset
// when a new client is created for the same IP (simulating a new connection).
func TestIPPingRateLimitPersistsAcrossConnections(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.PingRateLimit = 10
	config.PingRateLimitWindow = 5

	ipPingTracker.mu.Lock()
	ipPingTracker.timestamps = make(map[string][]time.Time)
	ipPingTracker.mu.Unlock()

	ipid := "testIPPingPersists"

	// Exhaust limit
	for i := 0; i < 10; i++ {
		checkIPPingRateLimit(ipid)
	}

	// Simulate a new connection (new client) – rate limit must still apply.
	if !checkIPPingRateLimit(ipid) {
		t.Errorf("Ping rate limit did not persist across a simulated new connection")
	}
}

// TestIPPingRateLimitWindowExpiry tests that the per-IP ping rate window resets after expiry.
func TestIPPingRateLimitWindowExpiry(t *testing.T) {
	oldConfig := config
	defer func() { config = oldConfig }()

	config = &settings.Config{}
	config.PingRateLimit = 5
	config.PingRateLimitWindow = 1

	ipPingTracker.mu.Lock()
	ipPingTracker.timestamps = make(map[string][]time.Time)
	ipPingTracker.mu.Unlock()

	ipid := "testIPPingExpiry"

	for i := 0; i < 5; i++ {
		checkIPPingRateLimit(ipid)
	}

	if !checkIPPingRateLimit(ipid) {
		t.Errorf("Ping was not blocked after reaching the limit")
		return
	}

	time.Sleep(time.Duration(config.PingRateLimitWindow)*time.Second + 100*time.Millisecond)

	if checkIPPingRateLimit(ipid) {
		t.Errorf("Ping was blocked after window expired")
	}
}
