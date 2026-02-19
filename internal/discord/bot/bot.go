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

// Package bot implements a Discord bot for Nyathena AO2 server moderation.
package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// Bot holds the Discord bot state.
type Bot struct {
	session    *discordgo.Session
	guildID    string
	modRoleID  string
	server     ServerInterface
	commands   []*discordgo.ApplicationCommand
}

// Config holds the configuration for the Discord bot.
type Config struct {
	Token     string
	GuildID   string
	ModRoleID string
}

// New creates and returns a new Bot instance.
func New(cfg Config, srv ServerInterface) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord bot token is empty")
	}

	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	b := &Bot{
		session:   session,
		guildID:   cfg.GuildID,
		modRoleID: cfg.ModRoleID,
		server:    srv,
	}
	return b, nil
}

// Start opens the Discord session, registers slash commands, and begins listening for events.
func (b *Bot) Start() error {
	b.session.AddHandler(b.handleInteraction)

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord session: %w", err)
	}

	if err := b.registerCommands(); err != nil {
		_ = b.session.Close()
		return fmt.Errorf("failed to register discord commands: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the Discord bot, removing registered commands.
func (b *Bot) Stop() {
	for _, cmd := range b.commands {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, b.guildID, cmd.ID); err != nil {
			// Best-effort cleanup; log but do not block shutdown.
			_ = err
		}
	}
	_ = b.session.Close()
}

// handleInteraction dispatches incoming Discord interaction events to the appropriate handler.
func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	handler, ok := b.commandHandlers()[data.Name]
	if !ok {
		return
	}
	handler(s, i)
}
