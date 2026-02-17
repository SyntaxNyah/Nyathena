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
