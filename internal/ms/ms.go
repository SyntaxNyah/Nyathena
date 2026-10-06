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

package ms

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
)

type Advertisement struct {
	IP      string `json:"ip,omitempty"`
	Port    int    `json:"port"`
	WSPort  int    `json:"ws_port,omitempty"`
	WSSPort int    `json:"wss_port,omitempty"`
	Players int    `json:"players"`
	Name    string `json:"name"`
	Desc    string `json:"description"`
}

// Advertise begins the server's advertising routine, posting to every
// configured master server endpoint.
func Advertise(msUrls []string, advert Advertisement, updatePlayers chan (int), done chan (struct{})) {
	postServers(msUrls, advert)
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ticker.C:
			postServers(msUrls, advert)
		case advert.Players = <-updatePlayers:
			postServers(msUrls, advert)
		case <-done:
			ticker.Stop()
			return
		}
	}
}

// postServers sends an advertisement to every master server endpoint.
func postServers(msUrls []string, advert Advertisement) {
	for _, msUrl := range msUrls {
		postServer(msUrl, advert)
	}
}

// postServer sends an advertisement to the master server.
func postServer(msUrl string, advert Advertisement) {
	data, err := json.Marshal(advert)
	if err != nil {
		logger.LogErrorf("Failed to post advertisement: %v", err)
		return
	}

	resp, err := http.Post(msUrl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		logger.LogErrorf("Failed to post advertisement: %v", err)
		return
	}
	resp.Body.Close()
}
