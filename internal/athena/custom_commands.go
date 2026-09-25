// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Custom command builder: console-authored, runtime-defined slash commands.
//
// A custom command is a declarative JSON definition (one file per command,
// stored under config/custom_commands/) composed of an ordered list of
// "actions". Each action maps to a single server capability (send a message,
// apply a punishment, kick, mute, run an existing built-in, etc.). Operators
// author these from the server console via the "customcmd" menu (or by editing
// the JSON by hand) and they are installed into the live command registry
// without a restart.
//
// The definition files are the source of truth. They are parsed and validated
// here and published as an atomic snapshot (the same pattern as
// command_grants.go) that the dispatch path reads without locking.
package athena

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/settings"
	"github.com/MangosArentLiterature/Athena/internal/sliceutil"
)

// customCommandsDir is the directory (relative to settings.ConfigPath) that
// holds one JSON file per custom command.
const customCommandsDir = "custom_commands"

// CustomActionType enumerates the action verbs a custom command can contain.
type CustomActionType string

const (
	ActionMessage  CustomActionType = "message"
	ActionAnnounce CustomActionType = "announce"
	ActionPunish   CustomActionType = "punish"
	ActionUnpunish CustomActionType = "unpunish"
	ActionKick     CustomActionType = "kick"
	ActionBan      CustomActionType = "ban"
	ActionMute     CustomActionType = "mute"
	ActionUnmute   CustomActionType = "unmute"
	ActionMove     CustomActionType = "move"
	ActionRun      CustomActionType = "run"
	ActionRandom   CustomActionType = "random"
	ActionWait     CustomActionType = "wait"
	ActionGrant    CustomActionType = "grant"
	ActionRevoke   CustomActionType = "revoke"
)

// validActionTypes lists every action verb in the order the wizard presents
// them, so the menu and the validator share one source of truth.
var validActionTypes = []CustomActionType{
	ActionMessage, ActionAnnounce, ActionPunish, ActionUnpunish, ActionKick, ActionBan,
	ActionMute, ActionUnmute, ActionMove, ActionRun, ActionRandom, ActionWait, ActionGrant, ActionRevoke,
}

func isValidActionType(t CustomActionType) bool {
	for _, v := range validActionTypes {
		if v == t {
			return true
		}
	}
	return false
}

// CustomAction is one step of a custom command. Only the fields relevant to the
// action's Type are meaningful; the rest are ignored.
type CustomAction struct {
	Type     CustomActionType   `json:"type"`
	Target   string             `json:"target,omitempty"`
	Text     string             `json:"text,omitempty"`
	Effect   string             `json:"effect,omitempty"`
	Duration string             `json:"duration,omitempty"`
	Reason   string             `json:"reason,omitempty"`
	Scope    string             `json:"scope,omitempty"`
	Area     string             `json:"area,omitempty"`
	Command  string             `json:"command,omitempty"`
	Args     []string           `json:"args,omitempty"`
	Choices  []CustomActionList `json:"choices,omitempty"`
}

// CustomActionList is a named branch of a "random" action.
type CustomActionList struct {
	Actions []CustomAction `json:"actions"`
}

// CustomCommand is a fully-resolved custom command definition.
type CustomCommand struct {
	Name       string         `json:"name"`
	Desc       string         `json:"desc"`
	Category   string         `json:"category"`
	ReqPerms   string         `json:"reqPerms"`
	Usage      string         `json:"usage"`
	MinArgs    int            `json:"minArgs"`
	PublicHelp bool           `json:"publicHelp"`
	Actions    []CustomAction `json:"actions"`
}

// resolved returns a copy of the command with sane defaults filled in so that
// the rest of the pipeline never has to branch on zero values.
func (c CustomCommand) resolved() CustomCommand {
	if c.Category == "" {
		c.Category = "custom"
	}
	if c.ReqPerms == "" {
		c.ReqPerms = "NONE"
	}
	if c.Usage == "" {
		c.Usage = "/" + c.Name
	}
	return c
}

// reqPermBits resolves ReqPerms to a permission bitfield. An unknown key maps
// to NONE (0), which makes the command usable by everyone.
func (c CustomCommand) reqPermBits() uint64 {
	if v, ok := permissions.PermissionField[c.ReqPerms]; ok {
		return v
	}
	return 0
}

// customNameRegex matches a valid bare command name (no leading slash).
var customNameRegex = regexp.MustCompile(`^[a-z0-9_]+(-[a-z0-9_]+)*$`)

// customCommandsPtr holds the live snapshot of installed custom commands,
// keyed by lowercase name. Published atomically (same pattern as
// command_grants.go) so the dispatch path never takes a lock or hits disk.
var customCommandsPtr atomic.Pointer[map[string]CustomCommand]

func getCustomCommands() map[string]CustomCommand {
	if v := customCommandsPtr.Load(); v != nil {
		return *v
	}
	return nil
}

func setCustomCommands(m map[string]CustomCommand) { customCommandsPtr.Store(&m) }

// getCustomCommand returns the custom command named name (case-insensitive), if
// one is installed.
func getCustomCommand(name string) (CustomCommand, bool) {
	cmd, ok := getCustomCommands()[strings.ToLower(name)]
	return cmd, ok
}

// customCommandExists reports whether name is an installed custom command.
func customCommandExists(name string) bool {
	_, ok := getCustomCommand(name)
	return ok
}

// customCommandsEnabled reports whether the custom command builder is turned on
// via enable_custom_commands. It is the single gate every entry point checks so
// the feature is entirely inert (no startup scan, no dispatch lookup, no console
// menu) when the flag is off. config is nil in tests until a test sets it.
func customCommandsEnabled() bool {
	return config != nil && config.EnableCustomCommands
}

// validateCustomCommand checks a command definition and returns a descriptive
// error for the first problem found.
func validateCustomCommand(cmd CustomCommand) error {
	cmd = cmd.resolved()
	name := strings.ToLower(strings.TrimSpace(cmd.Name))
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !customNameRegex.MatchString(name) {
		return fmt.Errorf("name %q is invalid: use lowercase letters, digits, and single dashes (e.g. my-cmd)", cmd.Name)
	}
	if _, exists := Commands[name]; exists {
		return fmt.Errorf("name %q collides with a built-in command", cmd.Name)
	}
	if len(cmd.Actions) == 0 {
		return fmt.Errorf("command %q has no actions", cmd.Name)
	}
	for i, a := range cmd.Actions {
		if err := validateCustomAction(name, a); err != nil {
			return fmt.Errorf("action %d: %w", i+1, err)
		}
	}
	return nil
}

// validateCustomAction validates a single action in the context of command
// name (used only for error messages).
func validateCustomAction(name string, a CustomAction) error {
	if !isValidActionType(a.Type) {
		return fmt.Errorf("unknown action type %q", a.Type)
	}
	switch a.Type {
	case ActionMessage, ActionAnnounce:
		if strings.TrimSpace(a.Text) == "" {
			return fmt.Errorf("%s action requires a non-empty \"text\"", a.Type)
		}
	case ActionPunish:
		if strings.TrimSpace(a.Effect) == "" {
			return fmt.Errorf("punish action requires an \"effect\" name")
		}
		if _, ok := punishmentTypeByName(a.Effect); !ok {
			return fmt.Errorf("unknown punishment effect %q", a.Effect)
		}
	case ActionUnpunish:
		if strings.TrimSpace(a.Effect) == "" {
			return fmt.Errorf("unpunish action requires an \"effect\" name (or \"all\")")
		}
	case ActionMute:
		if !validMuteScope(a.Scope) {
			return fmt.Errorf("mute action has invalid scope %q (use ic, ooc, both, music, jud)", a.Scope)
		}
	case ActionMove:
		if strings.TrimSpace(a.Area) == "" {
			return fmt.Errorf("move action requires an \"area\" name")
		}
	case ActionRun:
		if strings.TrimSpace(a.Command) == "" {
			return fmt.Errorf("run action requires a \"command\" name")
		}
	case ActionRandom:
		if len(a.Choices) < 2 {
			return fmt.Errorf("random action requires at least 2 choices")
		}
		for i, choice := range a.Choices {
			if len(choice.Actions) == 0 {
				return fmt.Errorf("random choice %d is empty", i+1)
			}
			for j, sub := range choice.Actions {
				if err := validateCustomAction(name, sub); err != nil {
					return fmt.Errorf("random choice %d action %d: %w", i+1, j+1, err)
				}
			}
		}
	case ActionGrant, ActionRevoke:
		if strings.TrimSpace(a.Command) == "" {
			return fmt.Errorf("%s action requires a \"command\" name", a.Type)
		}
	}
	return nil
}

// validMuteScope reports whether s names a valid mute scope.
func validMuteScope(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ic", "ooc", "both", "music", "jud", "":
		return true
	}
	return false
}

// parseMuteScope maps a scope string to a MuteState, defaulting to IC.
func parseMuteScope(s string) MuteState {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ooc":
		return OOCMuted
	case "both":
		return ICOOCMuted
	case "music":
		return MusicMuted
	case "jud":
		return JudMuted
	default:
		return ICMuted
	}
}

// ── Punishment name resolution ──────────────────────────────────────────────

// punishmentNameMap maps a lowercase punishment name to its PunishmentType.
// It is built exactly once at package init from PunishmentType.String(), so it
// is an immutable constant table: reading it is side-effect-free and
// deterministic (equivalent to reading a const), and concurrent reads need no
// lock. This keeps the pure executor free of any runtime global mutation.
var punishmentNameMap = buildPunishmentNameMap()

func buildPunishmentNameMap() map[string]PunishmentType {
	m := make(map[string]PunishmentType)
	for t := PunishmentType(PunishmentNone + 1); t <= PunishmentFish; t++ {
		s := strings.ToLower(strings.TrimSpace(t.String()))
		if s == "" || s == "none" {
			continue
		}
		m[s] = t
	}
	return m
}

func punishmentTypeByName(name string) (PunishmentType, bool) {
	t, ok := punishmentNameMap[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

// allPunishmentNames returns every punishment name sorted, for the wizard's
// completion hints and the example output.
func allPunishmentNames() []string {
	names := make([]string, 0, len(punishmentNameMap))
	for n := range punishmentNameMap {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ── Loading ──────────────────────────────────────────────────────────────────

// customCommandsDirPath returns the absolute directory holding definition files.
func customCommandsDirPath() string {
	return filepath.Join(settings.ConfigPath, customCommandsDir)
}

// parseCustomCommandFile reads and parses one JSON definition file.
func parseCustomCommandFile(path string) (CustomCommand, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CustomCommand{}, err
	}
	var cmd CustomCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return CustomCommand{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	cmd = cmd.resolved()
	if err := validateCustomCommand(cmd); err != nil {
		return CustomCommand{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return cmd, nil
}

// loadCustomCommands re-reads every definition file from disk and republishes
// the live snapshot. A missing directory is treated as "no custom commands",
// not an error, so operators don't have to pre-create it. Called once at
// startup and again on every "customcmd reload".
func loadCustomCommands() error {
	dir := customCommandsDirPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			setCustomCommands(map[string]CustomCommand{})
			return nil
		}
		return err
	}
	m := make(map[string]CustomCommand)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		cmd, err := parseCustomCommandFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		key := strings.ToLower(cmd.Name)
		if _, exists := m[key]; exists {
			return fmt.Errorf("duplicate custom command name %q", cmd.Name)
		}
		m[key] = cmd
	}
	setCustomCommands(m)
	logger.LogInfof("Loaded %d custom command(s).", len(m))
	return nil
}

// writeCustomCommandFile marshals cmd to its canonical file location and writes
// it, creating the directory if needed. Used by the wizard and by "customcmd
// import".
func writeCustomCommandFile(cmd CustomCommand) (string, error) {
	cmd = cmd.resolved()
	if err := validateCustomCommand(cmd); err != nil {
		return "", err
	}
	dir := customCommandsDirPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, strings.ToLower(cmd.Name)+".json")
	data, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// deleteCustomCommandFile removes the definition file for name.
func deleteCustomCommandFile(name string) error {
	path := filepath.Join(customCommandsDirPath(), strings.ToLower(name)+".json")
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

// ── Templating ───────────────────────────────────────────────────────────────

// renderContext carries the values available to the {placeholder} template
// language inside action strings.
type renderContext struct {
	args       []string
	callerUID  int
	callerName string
	targetUID  int
	targetName string
	reason     string
	duration   time.Duration
}

var argsTemplateRe = regexp.MustCompile(`\{args\.(\d+)\}`)

// renderTemplate expands {args.N}, {caller.uid}, {caller.name}, {target.uid},
// {target.name}, {reason} and {duration} placeholders in s.
func renderTemplate(s string, ctx renderContext) string {
	s = argsTemplateRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := argsTemplateRe.FindStringSubmatch(m)
		i, err := strconv.Atoi(sub[1])
		if err != nil || i < 0 || i >= len(ctx.args) {
			return ""
		}
		return ctx.args[i]
	})
	s = strings.ReplaceAll(s, "{caller.uid}", strconv.Itoa(ctx.callerUID))
	s = strings.ReplaceAll(s, "{caller.name}", ctx.callerName)
	s = strings.ReplaceAll(s, "{target.uid}", strconv.Itoa(ctx.targetUID))
	s = strings.ReplaceAll(s, "{target.name}", ctx.targetName)
	s = strings.ReplaceAll(s, "{reason}", ctx.reason)
	s = strings.ReplaceAll(s, "{duration}", ctx.duration.String())
	return s
}

// dispatchCustomCommand is the ParseCommand fallback for custom commands. It
// synthesizes a Command carrying the custom command's permission requirement so
// the exact same clientCanUseCommand chokepoint (role bits + CM/DJ + console
// grants) gates it as if it were built-in, then runs it against the live server.
func dispatchCustomCommand(client *Client, custom CustomCommand, args []string) {
	custom = custom.resolved()
	synth := Command{
		minArgs:    custom.MinArgs,
		usage:      custom.Usage,
		desc:       custom.Desc,
		reqPerms:   custom.reqPermBits(),
		category:   custom.Category,
		publicHelp: custom.PublicHelp,
	}
	if !clientCanUseCommand(client, custom.Name, synth) {
		client.SendServerMessage("You do not have permission to use that command.")
		return
	}
	if sliceutil.ContainsString(args, "-h") && !strings.Contains(custom.Usage, "[-h]") {
		client.SendServerMessage(custom.Usage)
		return
	}
	if len(args) < custom.MinArgs {
		client.SendServerMessage("Not enough arguments.\n" + custom.Usage)
		return
	}
	executeCustomCommandLive(client, custom, args)
}
