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
	"fmt"
	"sync"
	"testing"
	"time"
)

// BenchmarkWriteAreaLog measures the performance of single-threaded log writes
func BenchmarkWriteAreaLog(b *testing.B) {
	tempDir := b.TempDir()
	LogPath = tempDir
	EnableAreaLogging = true

	areaName := "TestArea"
	err := CreateAreaLogDirectory(areaName)
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WriteAreaLog(areaName, logEntry)
	}
}

// BenchmarkWriteAreaLogDisabled measures overhead when logging is disabled
func BenchmarkWriteAreaLogDisabled(b *testing.B) {
	EnableAreaLogging = false

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WriteAreaLog("TestArea", logEntry)
	}
}

// BenchmarkWriteAreaLogConcurrent measures concurrent write performance to same area
func BenchmarkWriteAreaLogConcurrent(b *testing.B) {
	tempDir := b.TempDir()
	LogPath = tempDir
	EnableAreaLogging = true

	areaName := "ConcurrentArea"
	err := CreateAreaLogDirectory(areaName)
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			WriteAreaLog(areaName, logEntry)
		}
	})
}

// BenchmarkWriteAreaLogMultipleAreas measures performance when writing to different areas
func BenchmarkWriteAreaLogMultipleAreas(b *testing.B) {
	tempDir := b.TempDir()
	LogPath = tempDir
	EnableAreaLogging = true

	numAreas := 10
	areas := make([]string, numAreas)
	for i := 0; i < numAreas; i++ {
		areas[i] = fmt.Sprintf("Area%d", i)
		err := CreateAreaLogDirectory(areas[i])
		if err != nil {
			b.Fatalf("Failed to create directory: %v", err)
		}
	}

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		areaIndex := 0
		for pb.Next() {
			WriteAreaLog(areas[areaIndex%numAreas], logEntry)
			areaIndex++
		}
	})
}

// BenchmarkSanitizeAreaName measures area name sanitization performance
func BenchmarkSanitizeAreaName(b *testing.B) {
	testNames := []string{
		"Normal Area",
		"Area/With/Slashes",
		"Area\\With\\Backslashes",
		"Area:With:Colons",
		"Area*With*Special*Characters*And*Long*Name",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeAreaName(testNames[i%len(testNames)])
	}
}

// TestWritePerformance is a regular test that validates performance characteristics
func TestWritePerformance(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableAreaLogging = true

	areaName := "PerformanceTest"
	err := CreateAreaLogDirectory(areaName)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	// Measure 100 sequential writes
	start := time.Now()
	for i := 0; i < 100; i++ {
		WriteAreaLog(areaName, logEntry)
	}
	duration := time.Since(start)

	avgTime := duration / 100
	t.Logf("Average write time: %v", avgTime)

	// Performance assertion: should be under 50ms per write (very conservative)
	// On SSDs this will be much faster (< 1ms), on HDDs up to 50ms is acceptable
	if avgTime > 50*time.Millisecond {
		t.Errorf("Write performance degraded: average %v per write (expected < 50ms)", avgTime)
	}
}

// TestConcurrentWritePerformance validates that concurrent writes don't block excessively
func TestConcurrentWritePerformance(t *testing.T) {
	tempDir := t.TempDir()
	LogPath = tempDir
	EnableAreaLogging = true

	numAreas := 5
	areas := make([]string, numAreas)
	for i := 0; i < numAreas; i++ {
		areas[i] = fmt.Sprintf("ConcurrentArea%d", i)
		err := CreateAreaLogDirectory(areas[i])
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	// Launch 10 goroutines writing to different areas
	numGoroutines := 10
	writesPerGoroutine := 20

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			areaIndex := goroutineID % numAreas
			for i := 0; i < writesPerGoroutine; i++ {
				WriteAreaLog(areas[areaIndex], fmt.Sprintf("%s (goroutine %d)", logEntry, goroutineID))
			}
		}(g)
	}

	wg.Wait()
	duration := time.Since(start)

	totalWrites := numGoroutines * writesPerGoroutine
	avgTime := duration / time.Duration(totalWrites)

	t.Logf("Concurrent writes: %d writes in %v", totalWrites, duration)
	t.Logf("Average time per write: %v", avgTime)
	t.Logf("Throughput: %.0f writes/second", float64(totalWrites)/duration.Seconds())

	// With 5 different areas, writes should be able to proceed in parallel
	// Total time should be much less than sequential time
	// Conservative check: should complete in under 5 seconds total
	if duration > 5*time.Second {
		t.Errorf("Concurrent write performance degraded: took %v for %d writes", duration, totalWrites)
	}
}

// TestDisabledLoggingOverhead validates zero overhead when logging is disabled
func TestDisabledLoggingOverhead(t *testing.T) {
	EnableAreaLogging = false

	logEntry := "[12:34:56] | IC | Phoenix Wright | ipid123 | hdid456 | Phoenix | TestUser | \"Test message\""

	// Measure time for 10000 calls with logging disabled
	start := time.Now()
	for i := 0; i < 10000; i++ {
		WriteAreaLog("TestArea", logEntry)
	}
	duration := time.Since(start)

	t.Logf("10000 disabled log calls took: %v", duration)
	t.Logf("Average per call: %v", duration/10000)

	// When disabled, should be extremely fast (just a boolean check and return)
	// Should take less than 1ms for 10000 calls (< 100ns per call)
	if duration > time.Millisecond {
		t.Errorf("Disabled logging has overhead: %v for 10000 calls", duration)
	}
}
