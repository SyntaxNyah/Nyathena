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

import "testing"

// TestFormatPlaytime verifies that formatPlaytime converts seconds to the
// expected human-readable string for all relevant cases.
func TestFormatPlaytime(t *testing.T) {
	cases := []struct {
		secs int64
		want string
	}{
		{-1, "less than a minute"},
		{0, "less than a minute"},
		{30, "0m"},                  // < 60s: minutes = 0
		{59, "0m"},                  // one second short of a minute
		{60, "1m"},                  // exactly one minute
		{90, "1m"},                  // 1m 30s → 1m (seconds truncated)
		{3600, "1h 0m"},             // exactly one hour
		{3661, "1h 1m"},             // 1h 1m 1s
		{7322, "2h 2m"},             // 2h 2m 2s
	}
	for _, tc := range cases {
		got := formatPlaytime(tc.secs)
		if got != tc.want {
			t.Errorf("formatPlaytime(%d) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}

// TestGenerateCaptcha verifies that generateCaptcha returns a non-empty string
// and that two successive calls produce different tokens.
func TestGenerateCaptcha(t *testing.T) {
	tok1, err := generateCaptcha()
	if err != nil {
		t.Fatalf("generateCaptcha returned unexpected error: %v", err)
	}
	if tok1 == "" {
		t.Fatal("generateCaptcha returned an empty string")
	}
	// 16 hex characters = 8 bytes.
	if len(tok1) != 16 {
		t.Errorf("expected token length 16, got %d", len(tok1))
	}

	tok2, err := generateCaptcha()
	if err != nil {
		t.Fatalf("second generateCaptcha returned unexpected error: %v", err)
	}
	if tok1 == tok2 {
		t.Error("two successive captcha tokens should not be identical")
	}
}

// TestClientPendingReg verifies that SetPendingReg stores values and PendingReg
// retrieves them, and that clearing works correctly.
func TestClientPendingReg(t *testing.T) {
	c := &Client{}

	// Initially all fields should be zero-valued.
	u, tok, hp := c.PendingReg()
	if u != "" || tok != "" || hp != nil {
		t.Errorf("expected empty pending reg, got (user=%q, tok=%q, pass=%v)", u, tok, hp)
	}

	// Set pending registration data.
	fakeHash := []byte("$2a$12$fakehashvalue")
	c.SetPendingReg("alice", "abc123def456abcd", fakeHash)
	u, tok, hp = c.PendingReg()
	if u != "alice" {
		t.Errorf("expected username 'alice', got %q", u)
	}
	if tok != "abc123def456abcd" {
		t.Errorf("expected token 'abc123def456abcd', got %q", tok)
	}
	if string(hp) != string(fakeHash) {
		t.Errorf("expected hash %q, got %q", fakeHash, hp)
	}

	// Clear pending registration.
	c.SetPendingReg("", "", nil)
	u, tok, hp = c.PendingReg()
	if u != "" || tok != "" || hp != nil {
		t.Errorf("expected cleared pending reg, got (user=%q, tok=%q, pass=%v)", u, tok, hp)
	}
}
