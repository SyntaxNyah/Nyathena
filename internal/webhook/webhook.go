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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ecnepsnai/discord"
)

var (
	ServerName           string
	ServerColor          uint32 = 0x05b2f7
	PingRoleID           string
	PunishmentWebhookURL string
)

// nonEmpty returns s if non-empty, otherwise "N/A".
// This prevents Discord webhook 400 errors caused by embed fields
// with empty values (the Discord API requires every field to have a value).
func nonEmpty(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// postToURL posts a discord message to the given webhook URL directly,
// bypassing the global discord.WebhookURL variable.
func postToURL(url string, content discord.PostOptions) error {
	if url == "" {
		return nil
	}
	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(content); err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// PostBan sends a ban notification embed to the punishment webhook.
func PostBan(icName, showname, oocName, ipid string, uid, banID int, duration, reason, moderator string) error {
	if PunishmentWebhookURL == "" {
		return nil
	}
	uidVal := fmt.Sprintf("%d", uid)
	if uid < 0 {
		uidVal = "N/A"
	}
	e := discord.Embed{
		Title: "🔨 Player Banned",
		Color: 0xe74c3c,
		Fields: []discord.Field{
			{Name: "IC Name", Value: nonEmpty(icName), Inline: true},
			{Name: "Showname", Value: nonEmpty(showname), Inline: true},
			{Name: "OOC Name", Value: nonEmpty(oocName), Inline: true},
			{Name: "IPID", Value: nonEmpty(ipid), Inline: true},
			{Name: "UID", Value: uidVal, Inline: true},
			{Name: "Ban ID", Value: fmt.Sprintf("%d", banID), Inline: true},
			{Name: "Duration", Value: nonEmpty(duration), Inline: true},
			{Name: "Reason", Value: nonEmpty(reason), Inline: false},
			{Name: "Moderator", Value: nonEmpty(moderator), Inline: true},
		},
	}
	p := discord.PostOptions{
		Username: ServerName,
		Embeds:   []discord.Embed{e},
	}
	return postToURL(PunishmentWebhookURL, p)
}

// PostKick sends a kick notification embed to the punishment webhook.
func PostKick(icName, showname, oocName, ipid, reason, moderator string, uid int) error {
	if PunishmentWebhookURL == "" {
		return nil
	}
	e := discord.Embed{
		Title: "👢 Player Kicked",
		Color: 0xe67e22,
		Fields: []discord.Field{
			{Name: "IC Name", Value: nonEmpty(icName), Inline: true},
			{Name: "Showname", Value: nonEmpty(showname), Inline: true},
			{Name: "OOC Name", Value: nonEmpty(oocName), Inline: true},
			{Name: "IPID", Value: nonEmpty(ipid), Inline: true},
			{Name: "UID", Value: fmt.Sprintf("%d", uid), Inline: true},
			{Name: "Reason", Value: nonEmpty(reason), Inline: false},
			{Name: "Moderator", Value: nonEmpty(moderator), Inline: true},
		},
	}
	p := discord.PostOptions{
		Username: ServerName,
		Embeds:   []discord.Embed{e},
	}
	return postToURL(PunishmentWebhookURL, p)
}

// PostUnban sends an unban notification embed to the punishment webhook.
// banID is the ID of the ban that was nullified.
// originalDuration should be either "Permanent" or a human-readable timestamp
// (e.g. "02 Jan 2006 15:04 MST") formatted by the caller from the stored ban record.
// The remaining string fields are taken directly from the stored ban record so
// the embed is informative even when the player is offline.
func PostUnban(banID int, ipid, originalReason, originalDuration, originalModerator, unbannedBy string) error {
	if PunishmentWebhookURL == "" {
		return nil
	}
	e := discord.Embed{
		Title: "✅ Ban Lifted",
		Color: 0x2ecc71,
		Fields: []discord.Field{
			{Name: "Ban ID", Value: fmt.Sprintf("%d", banID), Inline: true},
			{Name: "IPID", Value: nonEmpty(ipid), Inline: true},
			{Name: "Original Duration", Value: nonEmpty(originalDuration), Inline: true},
			{Name: "Original Reason", Value: nonEmpty(originalReason), Inline: false},
			{Name: "Originally Banned By", Value: nonEmpty(originalModerator), Inline: true},
			{Name: "Unbanned By", Value: nonEmpty(unbannedBy), Inline: true},
		},
	}
	p := discord.PostOptions{
		Username: ServerName,
		Embeds:   []discord.Embed{e},
	}
	return postToURL(PunishmentWebhookURL, p)
}

// PostModcall sends a modcall to the discord webhook.
func PostModcall(character string, area string, reason string) error {
	e := discord.Embed{
		Title:       fmt.Sprintf("%v sent a modcall in %v.", character, area),
		Description: reason,
		Color:       ServerColor,
	}
	content := ""
	if PingRoleID != "" {
		content = fmt.Sprintf("<@&%s>", PingRoleID)
	}
	p := discord.PostOptions{
		Username: ServerName,
		Content:  content,
		Embeds:   []discord.Embed{e},
	}
	err := discord.Post(p)
	return err
}

// PostReport sends a report file to the discord webhook.
func PostReport(name string, contents string) error {
	c := strings.NewReader(contents)
	f := discord.FileOptions{
		FileName: name,
		Reader:   c,
	}
	p := discord.PostOptions{
		Username: ServerName,
	}
	err := discord.UploadFile(p, f)
	return err
}
