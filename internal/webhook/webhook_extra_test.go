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

package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPostJailEmptyURL verifies that PostJail returns nil immediately when
// PunishmentWebhookURL is empty (no HTTP call should be attempted).
func TestPostJailEmptyURL(t *testing.T) {
	PunishmentWebhookURL = ""
	if err := PostJail("ic", "show", "ooc", "ipid", "area", "1h", "reason", "mod", 1); err != nil {
		t.Errorf("PostJail with empty URL: got error %v, want nil", err)
	}
}

// TestPostBanEmptyURL verifies that PostBan returns nil when PunishmentWebhookURL is empty.
func TestPostBanEmptyURL(t *testing.T) {
	PunishmentWebhookURL = ""
	if err := PostBan("ic", "show", "ooc", "ipid", 1, 42, "1h", "reason", "mod"); err != nil {
		t.Errorf("PostBan with empty URL: got error %v, want nil", err)
	}
}

// TestPostBanNegativeUID verifies that a uid < 0 serialises as "N/A" (not a
// negative number string) – the embed must not contain "-1" as the UID value.
func TestPostBanNegativeUID(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf [4096]byte
		n, _ := r.Body.Read(buf[:])
		captured = make([]byte, n)
		copy(captured, buf[:n])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	PunishmentWebhookURL = srv.URL
	ServerName = "TestServer"
	if err := PostBan("ic", "show", "ooc", "ipid", -1, 1, "1h", "reason", "mod"); err != nil {
		t.Fatalf("PostBan unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("could not decode payload: %v", err)
	}
	embeds, _ := payload["embeds"].([]interface{})
	if len(embeds) == 0 {
		t.Fatal("no embeds in payload")
	}
	embed := embeds[0].(map[string]interface{})
	fields := embed["fields"].([]interface{})
	for _, f := range fields {
		field := f.(map[string]interface{})
		if field["name"] == "UID" {
			if field["value"] != "N/A" {
				t.Errorf("UID field value = %q, want \"N/A\" for uid=-1", field["value"])
			}
		}
	}
	PunishmentWebhookURL = ""
}

// TestPostKickEmptyURL verifies that PostKick returns nil when PunishmentWebhookURL is empty.
func TestPostKickEmptyURL(t *testing.T) {
	PunishmentWebhookURL = ""
	if err := PostKick("ic", "show", "ooc", "ipid", "reason", "mod", 1); err != nil {
		t.Errorf("PostKick with empty URL: got error %v, want nil", err)
	}
}

// TestPostUnbanEmptyURL verifies that PostUnban returns nil when PunishmentWebhookURL is empty.
func TestPostUnbanEmptyURL(t *testing.T) {
	PunishmentWebhookURL = ""
	if err := PostUnban(1, "ipid", "reason", "1h", "mod", "mod2"); err != nil {
		t.Errorf("PostUnban with empty URL: got error %v, want nil", err)
	}
}

// TestPostBotBanEmptyURL verifies that PostBotBan returns nil when PunishmentWebhookURL is empty.
func TestPostBotBanEmptyURL(t *testing.T) {
	PunishmentWebhookURL = ""
	if err := PostBotBan(3, "1.2.3.4,5.6.7.8", "mod"); err != nil {
		t.Errorf("PostBotBan with empty URL: got error %v, want nil", err)
	}
}

// TestPostPacketFloodEmptyURL verifies that PostPacketFlood returns nil when
// PunishmentWebhookURL is empty.
func TestPostPacketFloodEmptyURL(t *testing.T) {
	PunishmentWebhookURL = ""
	if err := PostPacketFlood("1.2.3.4", 5); err != nil {
		t.Errorf("PostPacketFlood with empty URL: got error %v, want nil", err)
	}
}

// TestPostPacketFloodNegativeUID verifies that uid=-1 is rendered as "N/A".
func TestPostPacketFloodNegativeUID(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf [4096]byte
		n, _ := r.Body.Read(buf[:])
		captured = make([]byte, n)
		copy(captured, buf[:n])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	PunishmentWebhookURL = srv.URL
	ServerName = "TestServer"
	if err := PostPacketFlood("1.2.3.4", -1); err != nil {
		t.Fatalf("PostPacketFlood unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	embeds, _ := payload["embeds"].([]interface{})
	if len(embeds) == 0 {
		t.Fatal("no embeds")
	}
	embed := embeds[0].(map[string]interface{})
	fields := embed["fields"].([]interface{})
	for _, f := range fields {
		field := f.(map[string]interface{})
		if field["name"] == "UID" && field["value"] != "N/A" {
			t.Errorf("UID field = %q, want \"N/A\" for uid=-1", field["value"])
		}
	}
	PunishmentWebhookURL = ""
}

// TestPostToURLHTTPError verifies that an HTTP error status is returned as an error.
func TestPostToURLHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	PunishmentWebhookURL = srv.URL
	ServerName = "TestServer"
	err := PostKick("ic", "show", "ooc", "ipid", "reason", "mod", 1)
	if err == nil {
		t.Error("expected error for HTTP 400, got nil")
	}
	PunishmentWebhookURL = ""
}
