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
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ecnepsnai/discord"
)

var (
	ServerName  string
	ServerColor uint32 = 0x05b2f7
	
	webhookQueue chan webhookTask
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
)

type webhookTask struct {
	taskType string // "modcall" or "report"
	character string
	area     string
	reason   string
	filename string
	contents string
}

// Initialize starts the webhook worker goroutine.
// Must be called before using PostModcall or PostReport.
func Initialize() {
	ctx, cancel = context.WithCancel(context.Background())
	webhookQueue = make(chan webhookTask, 100) // Buffer to prevent blocking
	wg.Add(1)
	go webhookWorker()
}

// Shutdown gracefully stops the webhook worker and waits for pending tasks.
func Shutdown() {
	if cancel != nil {
		cancel()
	}
	wg.Wait()
}

// webhookWorker processes webhook tasks asynchronously.
func webhookWorker() {
	defer wg.Done()
	
	for {
		select {
		case <-ctx.Done():
			// Process remaining tasks before shutdown
			for task := range webhookQueue {
				processTask(task)
			}
			close(webhookQueue)
			return
		case task, ok := <-webhookQueue:
			if !ok {
				return
			}
			processTask(task)
		}
	}
}

// processTask handles individual webhook tasks.
// Errors are logged to stderr as webhook cannot import logger due to circular dependency.
func processTask(task webhookTask) {
	switch task.taskType {
	case "modcall":
		e := discord.Embed{
			Title:       fmt.Sprintf("%v sent a modcall in %v.", task.character, task.area),
			Description: task.reason,
			Color:       ServerColor,
		}
		p := discord.PostOptions{
			Username: ServerName,
			Embeds:   []discord.Embed{e},
		}
		err := discord.Post(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to post modcall webhook (character: %v, area: %v): %v\n", task.character, task.area, err)
		}
	case "report":
		c := strings.NewReader(task.contents)
		f := discord.FileOptions{
			FileName: task.filename,
			Reader:   c,
		}
		p := discord.PostOptions{
			Username: ServerName,
		}
		err := discord.UploadFile(p, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to upload report webhook (filename: %v): %v\n", task.filename, err)
		}
	}
}

// PostModcall queues a modcall to be sent to the discord webhook asynchronously.
func PostModcall(character string, area string, reason string) error {
	if webhookQueue == nil {
		return fmt.Errorf("webhook not initialized")
	}
	
	task := webhookTask{
		taskType:  "modcall",
		character: character,
		area:      area,
		reason:    reason,
	}
	
	select {
	case webhookQueue <- task:
		return nil
	default:
		return fmt.Errorf("webhook queue full, dropping modcall")
	}
}

// PostReport queues a report file to be sent to the discord webhook asynchronously.
func PostReport(name string, contents string) error {
	if webhookQueue == nil {
		return fmt.Errorf("webhook not initialized")
	}
	
	task := webhookTask{
		taskType: "report",
		filename: name,
		contents: contents,
	}
	
	select {
	case webhookQueue <- task:
		return nil
	default:
		return fmt.Errorf("webhook queue full, dropping report")
	}
}
