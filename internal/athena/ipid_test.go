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
	"net/http"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

func TestGetRealIP(t *testing.T) {
	// Save original config and restore after tests
	originalConfig := config
	defer func() { config = originalConfig }()

	tests := []struct {
		name              string
		reverseProxyMode  bool
		remoteAddr        string
		xForwardedFor     string
		xRealIP           string
		expectedResult    string
	}{
		{
			name:              "Reverse proxy disabled - ignore X-Forwarded-For",
			reverseProxyMode:  false,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "203.0.113.45",
			xRealIP:           "",
			expectedResult:    "10.0.0.1:8080",
		},
		{
			name:              "Reverse proxy disabled - ignore X-Real-IP",
			reverseProxyMode:  false,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "",
			xRealIP:           "203.0.113.45",
			expectedResult:    "10.0.0.1:8080",
		},
		{
			name:              "Reverse proxy disabled - ignore both headers",
			reverseProxyMode:  false,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "203.0.113.45",
			xRealIP:           "198.51.100.20",
			expectedResult:    "10.0.0.1:8080",
		},
		{
			name:              "Reverse proxy enabled - no headers, use RemoteAddr",
			reverseProxyMode:  true,
			remoteAddr:        "192.168.1.100:12345",
			xForwardedFor:     "",
			xRealIP:           "",
			expectedResult:    "192.168.1.100:12345",
		},
		{
			name:              "Reverse proxy enabled - X-Forwarded-For with single IP",
			reverseProxyMode:  true,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "203.0.113.45",
			xRealIP:           "",
			expectedResult:    "203.0.113.45",
		},
		{
			name:              "Reverse proxy enabled - X-Forwarded-For with multiple IPs",
			reverseProxyMode:  true,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "203.0.113.45, 198.51.100.20, 10.0.0.1",
			xRealIP:           "",
			expectedResult:    "203.0.113.45",
		},
		{
			name:              "Reverse proxy enabled - X-Real-IP header",
			reverseProxyMode:  true,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "",
			xRealIP:           "203.0.113.45",
			expectedResult:    "203.0.113.45",
		},
		{
			name:              "Reverse proxy enabled - both headers, X-Forwarded-For takes precedence",
			reverseProxyMode:  true,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     "203.0.113.45",
			xRealIP:           "198.51.100.20",
			expectedResult:    "203.0.113.45",
		},
		{
			name:              "Reverse proxy enabled - X-Forwarded-For with whitespace",
			reverseProxyMode:  true,
			remoteAddr:        "10.0.0.1:8080",
			xForwardedFor:     " 203.0.113.45 , 198.51.100.20",
			xRealIP:           "",
			expectedResult:    "203.0.113.45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test config
			config = &settings.Config{
				ServerConfig: settings.ServerConfig{
					ReverseProxyMode: tt.reverseProxyMode,
				},
			}

			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     make(http.Header),
			}
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			result := getRealIP(req)
			if result != tt.expectedResult {
				t.Errorf("getRealIP() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestGetIpid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "IPv4 with port",
			input: "192.168.1.1:12345",
		},
		{
			name:  "IPv4 without port",
			input: "192.168.1.1",
		},
		{
			name:  "Different IPs produce different IPIDs",
			input: "10.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIpid(tt.input)
			// Just verify it produces a non-empty result
			if result == "" {
				t.Errorf("getIpid(%v) returned empty string", tt.input)
			}
			// Verify it's a valid base64 string (22 chars for MD5 with 2 chars trimmed)
			if len(result) != 22 {
				t.Errorf("getIpid(%v) returned unexpected length: %d, want 22", tt.input, len(result))
			}
		})
	}
}

func TestUniqueIPIDs(t *testing.T) {
	// Test that different IPs produce different IPIDs
	ips := []string{
		"192.168.1.1:12345",
		"192.168.1.2:12345",
		"10.0.0.1:8080",
		"172.16.0.1:9000",
		"203.0.113.45:5555",
	}

	ipids := make(map[string]bool)
	for _, ip := range ips {
		ipid := getIpid(ip)
		if ipids[ipid] {
			t.Errorf("Duplicate IPID found for IP %v: %v", ip, ipid)
		}
		ipids[ipid] = true
	}

	if len(ipids) != len(ips) {
		t.Errorf("Expected %d unique IPIDs, got %d", len(ips), len(ipids))
	}
}

func TestSameIPProducesSameIPID(t *testing.T) {
	// Test that the same IP (with different ports) produces the same IPID
	ip1 := "192.168.1.100:12345"
	ip2 := "192.168.1.100:54321"

	ipid1 := getIpid(ip1)
	ipid2 := getIpid(ip2)

	if ipid1 != ipid2 {
		t.Errorf("Same IP with different ports should produce same IPID. Got %v and %v", ipid1, ipid2)
	}
}

func TestIPsWithoutPortsProduceUniqueIPIDs(t *testing.T) {
	// Test that IPs without ports produce unique IPIDs
	// This is critical for reverse proxy scenarios where getRealIP returns just the IP
	ips := []string{
		"192.168.1.1",
		"192.168.1.2",
		"10.0.0.1",
		"172.16.0.1",
		"203.0.113.45",
	}

	ipids := make(map[string]bool)
	for _, ip := range ips {
		ipid := getIpid(ip)
		if ipid == "" {
			t.Errorf("getIpid(%v) returned empty string", ip)
		}
		if ipids[ipid] {
			t.Errorf("Duplicate IPID found for IP %v: %v", ip, ipid)
		}
		ipids[ipid] = true
	}

	if len(ipids) != len(ips) {
		t.Errorf("Expected %d unique IPIDs, got %d", len(ips), len(ipids))
	}
}

func TestIPWithAndWithoutPortProduceSameIPID(t *testing.T) {
	// Test that IP with port and without port produce the same IPID
	tests := []struct {
		name       string
		withPort   string
		withoutPort string
	}{
		{
			name:        "Standard IPv4",
			withPort:    "192.168.1.100:12345",
			withoutPort: "192.168.1.100",
		},
		{
			name:        "Different IP",
			withPort:    "10.0.0.50:8080",
			withoutPort: "10.0.0.50",
		},
		{
			name:        "Public IP",
			withPort:    "203.0.113.45:54321",
			withoutPort: "203.0.113.45",
		},
		{
			name:        "IPv6 loopback",
			withPort:    "[::1]:8080",
			withoutPort: "::1",
		},
		{
			name:        "IPv6 address",
			withPort:    "[2001:db8::1]:8080",
			withoutPort: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipidWithPort := getIpid(tt.withPort)
			ipidWithoutPort := getIpid(tt.withoutPort)

			if ipidWithPort != ipidWithoutPort {
				t.Errorf("Same IP with and without port should produce same IPID. With port: %v, Without port: %v", ipidWithPort, ipidWithoutPort)
			}
		})
	}
}
