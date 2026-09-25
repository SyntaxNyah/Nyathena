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

// Console menu for the custom command builder. This is the only entry point
// for authoring custom commands: it is reachable from the server's stdin
// console ("customcmd") and never from an in-game command. It is deliberately
// beginner-first — numbered options, Enter-accepts-a-default, keyword
// shortcuts, and a guided wizard — but every command it produces is just a JSON
// file that can also be edited by hand.
package athena

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/logger"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// customTips is the rotating tip pool shown when the menu opens.
var customTips = []string{
	"You don't need to code — pick a number and press Enter. Press Enter on any [default] to accept it.",
	"One-liner:  customcmd quick mute uwu 10m  builds a mute command instantly.",
	"Test before you ship:  customcmd test <name>  dry-runs a command against a fake player.",
	"Every command is a JSON file in config/custom_commands/ — edit one by hand and type 'reload'.",
	"Hand one command to one account without changing their role:  grantcmd <user> <cmd>.",
	"Put {caller.name} in a message to say who ran the command.",
	"The 'run' action can call ANY of the ~250 built-in commands, including punishments and minigames.",
	"Use the 'random' action to pick a random punishment each time the command runs.",
	"Type a keyword instead of a number — e.g. 'mute' or 'punish someone' — and it jumps there.",
	"Duration formats: 30s, 10m, 1h30m. Empty means the default for that action.",
	"Scopes for mute: ic, ooc, both, music, jud.",
	"Targets: @self, @args, @global, @area, @all, or literal UIDs like 5,7.",
	"An action list runs top to bottom — order matters (mute then punish then message).",
	"'unpunish all' clears every active effect on the target(s).",
	"grant/revoke actions are console-level power — gate them behind reqPerms ADMIN.",
	"If a file fails validation, 'validate' names the exact line and field to fix.",
	"customcmd import mycmd.json  installs a file from anywhere on disk.",
	"Built-in names always win — you can't shadow /ban or /help with a custom command.",
	"Press '?' at the menu for the full command reference.",
}

var customTipIndex int

func nextCustomTip() string {
	if len(customTips) == 0 {
		return ""
	}
	t := customTips[customTipIndex%len(customTips)]
	customTipIndex++
	return t
}

// permissionChoices lists the permission keys in the order the wizard offers
// them, paired with a short friendly label.
var permissionChoices = []struct {
	key   string
	label string
}{
	{"NONE", "Everyone"},
	{"CM", "Area CMs"},
	{"DJ", "DJs"},
	{"MOD_SPEAK", "Staff who can speak as mod"},
	{"MUTE", "Staff who can mute"},
	{"KICK", "Staff who can kick"},
	{"BAN", "Staff who can ban"},
	{"MODIFY_AREA", "Staff who can modify areas"},
	{"MOVE_USERS", "Staff who can move players"},
	{"ADMIN", "Admins"},
}

// readLine prints prompt and reads one trimmed line from the console. ok is
// false on EOF (Ctrl-D / closed stdin).
func readLine(scanner *bufio.Scanner, prompt string) (string, bool) {
	fmt.Print(prompt)
	if !scanner.Scan() {
		return "", false
	}
	return strings.TrimSpace(scanner.Text()), true
}

// readYesNo reads a y/n answer, defaulting to def when the line is empty.
func readYesNo(scanner *bufio.Scanner, prompt string, def bool) bool {
	for {
		line, ok := readLine(scanner, prompt)
		if !ok {
			return def
		}
		switch strings.ToLower(line) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		case "":
			return def
		default:
			fmt.Println("  (please answer y or n)")
		}
	}
}

// readPermission prompts for who may use the command. It returns the permission
// key plus an optional account name: when the operator picks "custom account
// name", the returned permission is "NONE" and account is the username that may
// use the command (a hard gate checked in dispatchCustomCommand).
func readPermission(scanner *bufio.Scanner) (string, string) {
	fmt.Println("Who can use it? (pick a number, or type a permission name)")
	for i, p := range permissionChoices {
		fmt.Printf("  %d. %s (%s)\n", i+1, p.label, p.key)
	}
	fmt.Printf("  %d. Custom account name (only that account, no perms needed)\n", len(permissionChoices)+1)
	for {
		line, ok := readLine(scanner, "  > ")
		if !ok {
			return "NONE", ""
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return "NONE", ""
		}
		if n, err := strconv.Atoi(trimmed); err == nil {
			if n >= 1 && n <= len(permissionChoices) {
				return permissionChoices[n-1].key, ""
			}
			if n == len(permissionChoices)+1 {
				return readCustomAccount(scanner)
			}
		}
		lower := strings.ToLower(trimmed)
		if lower == "account" || lower == "custom" {
			return readCustomAccount(scanner)
		}
		for _, p := range permissionChoices {
			if strings.EqualFold(p.key, trimmed) {
				return p.key, ""
			}
		}
		fmt.Printf("  (unknown — pick 1-%d, or type a name like MUTE)\n", len(permissionChoices)+1)
	}
}

// readCustomAccount prompts for an account username for the "custom account
// name" permission option.
func readCustomAccount(scanner *bufio.Scanner) (string, string) {
	name, _ := readLine(scanner, "  account username > ")
	name = strings.TrimSpace(name)
	if name == "" {
		fmt.Println("  (no account given — everyone can use it)")
		return "NONE", ""
	}
	return "NONE", name
}

// runCustomCommandMenu is the interactive console menu. It is invoked from
// ListenInput's "customcmd" case and returns when the operator chooses exit.
func runCustomCommandMenu(scanner *bufio.Scanner) {
	fmt.Println()
	fmt.Println("┌─ Custom Command Builder ───────────────────────────────┐")
	fmt.Printf("│  💡 %s\n", nextCustomTip())
	fmt.Println("│                                                        │")
	fmt.Println("│  Quick ideas:                                          │")
	fmt.Println("│   · Make a \"/boop\" message command                     │")
	fmt.Println("│   · Punish someone with \"uwu\" for 20m                  │")
	fmt.Println("│   · Mute + whisper + notify in one command             │")
	fmt.Println("└────────────────────────────────────────────────────────┘")
	for {
		fmt.Println()
		fmt.Println("  1. Make a new command      6. Import a JSON file")
		fmt.Println("  2. List my commands        7. Reload from disk")
		fmt.Println("  3. Edit a command          8. Examples")
		fmt.Println("  4. Delete a command        9. Validate files")
		fmt.Println("  5. Test a command (dry-run)  0. Exit")
		line, ok := readLine(scanner, "  Pick a number > ")
		if !ok {
			return
		}
		switch strings.ToLower(line) {
		case "0", "exit", "quit", "q":
			return
		case "1", "new", "create", "make":
			wizardNew(scanner, "")
		case "2", "list", "ls":
			cmdCustomList()
		case "3", "edit":
			cmdCustomEdit(scanner)
		case "4", "delete", "remove", "rm":
			cmdCustomDelete(scanner)
		case "5", "test":
			cmdCustomTest(scanner)
		case "6", "import":
			cmdCustomImport(scanner)
		case "7", "reload":
			cmdCustomReload()
		case "8", "example", "examples":
			cmdCustomExamples()
		case "9", "validate":
			cmdCustomValidate()
		case "?", "help":
			printMenuHelp()
		default:
			fmt.Println("  (type a number 0-9, or a keyword like 'list', 'test', 'reload')")
		}
	}
}

func printMenuHelp() {
	fmt.Println("  menu commands:")
	fmt.Println("    1/new        guided wizard to build a command")
	fmt.Println("    2/list       list every installed custom command")
	fmt.Println("    3/edit       reopen a command in the wizard")
	fmt.Println("    4/delete     remove a command (deletes its JSON file)")
	fmt.Println("    5/test       dry-run a command against a fake player")
	fmt.Println("    6/import     install JSON from a file (or paste JSON)")
	fmt.Println("    7/reload     re-read all JSON files from disk")
	fmt.Println("    8/example    print example templates")
	fmt.Println("    9/validate   check every JSON file for errors")
	fmt.Println("    0/exit       return to the main console")
	fmt.Println("  Console shortcuts (no menu):")
	fmt.Println("    customcmd list | show <name> | test <name> [args] | reload | validate")
	fmt.Println("    customcmd import <file> | delete <name> | example [kind] | quick ...")
}

func cmdCustomList() {
	cmds := getCustomCommands()
	if len(cmds) == 0 {
		fmt.Println("  You have no custom commands yet. Pick 1 to make your first one!")
		return
	}
	names := make([]string, 0, len(cmds))
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Printf("  %d custom command(s):\n", len(names))
	for _, n := range names {
		c := cmds[n]
		fmt.Printf("    /%-20s [%s] %s\n", c.Name, c.Category, c.Desc)
	}
}

func cmdCustomShow(name string) {
	c, ok := getCustomCommand(name)
	if !ok {
		fmt.Printf("  no custom command named %q\n", name)
		return
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	fmt.Println(string(data))
}

func cmdCustomReload() {
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload failed: %v\n", err)
		return
	}
	fmt.Println("  reloaded.")
}

func cmdCustomValidate() {
	dir := customCommandsDirPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  no custom_commands directory yet — nothing to validate.")
			return
		}
		fmt.Printf("  validate failed: %v\n", err)
		return
	}
	bad := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		if _, err := parseCustomCommandFile(dir + string(os.PathSeparator) + e.Name()); err != nil {
			fmt.Printf("  ✗ %v\n", err)
			bad++
		}
	}
	if bad == 0 {
		fmt.Println("  all files valid.")
	} else {
		fmt.Printf("  %d file(s) have errors.\n", bad)
	}
}

func cmdCustomDelete(scanner *bufio.Scanner) {
	line, ok := readLine(scanner, "  command name to delete > ")
	if !ok || line == "" {
		return
	}
	if _, ok := getCustomCommand(line); !ok {
		fmt.Printf("  no custom command named %q\n", line)
		return
	}
	if !readYesNo(scanner, fmt.Sprintf("  delete /%s? (y/N) > ", strings.ToLower(line)), false) {
		return
	}
	if err := deleteCustomCommandFile(line); err != nil {
		fmt.Printf("  delete failed: %v\n", err)
		return
	}
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload after delete failed: %v\n", err)
		return
	}
	fmt.Printf("  deleted /%s.\n", strings.ToLower(line))
}

func cmdCustomEdit(scanner *bufio.Scanner) {
	line, ok := readLine(scanner, "  command name to edit > ")
	if !ok || line == "" {
		return
	}
	if _, ok := getCustomCommand(line); !ok {
		fmt.Printf("  no custom command named %q\n", line)
		return
	}
	wizardNew(scanner, line)
}

func cmdCustomTest(scanner *bufio.Scanner) {
	line, ok := readLine(scanner, "  command name to test > ")
	if !ok || line == "" {
		return
	}
	argsLine, _ := readLine(scanner, "  args (comma-separated UIDs, e.g. 5,7 — Enter for none) > ")
	runCustomTest(line, strings.Fields(argsLine))
}

// syntheticTestWorld builds a deterministic world for the dry-run tester: a
// synthetic admin caller plus a synthetic player for each UID the operator
// passed as an argument. Config flags mirror the live server so the transcript
// reflects the current kill-switch / feature state.
func syntheticTestWorld(args []string) TestWorld {
	w := TestWorld{
		Config: TestConfig{
			EnableCasino:        config != nil && config.EnableCasino,
			EnableAccounts:      config != nil && config.EnableAccounts,
			EnableVoice:         config != nil && config.EnableVoice,
			PunishmentsDisabled: punishmentsGloballyDisabled.Load(),
		},
		Areas:     []TestArea{{Name: "Area 0"}},
		Players:   make(map[int]*TestPlayer),
		CallerUID: 0,
	}
	w.Players[0] = &TestPlayer{UID: 0, Name: "you", Perms: adminPermBits(), AreaName: "Area 0"}
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			var uid int
			if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &uid); err != nil || uid <= 0 {
				continue
			}
			if _, exists := w.Players[uid]; exists {
				continue
			}
			w.Players[uid] = &TestPlayer{UID: uid, Name: fmt.Sprintf("player %d", uid), AreaName: "Area 0"}
		}
	}
	return w
}

func adminPermBits() uint64 {
	return permissions.PermissionField["ADMIN"]
}

// customTestNow is a fixed clock used by the dry-run tester so its transcript
// is byte-for-byte reproducible, independent of wall time.
var customTestNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// runCustomTest dry-runs a custom command and prints the full input/output
// transcript. It resolves random branches deterministically (choice 0) so the
// output is reproducible.
func runCustomTest(name string, args []string) {
	custom, ok := getCustomCommand(name)
	if !ok {
		fmt.Printf("  no custom command named %q\n", name)
		return
	}
	world := syntheticTestWorld(args)
	custom = custom.resolved()
	custom.Actions = resolveRandomInActions(custom.Actions, func(int) int { return 0 })
	res := executeCustomCommandPure(world, custom, args, customTestNow)

	fmt.Println("  Input:")
	fmt.Printf("    config: punishments=%v casino=%v accounts=%v voice=%v\n",
		res.World.Config.PunishmentsDisabled, res.World.Config.EnableCasino,
		res.World.Config.EnableAccounts, res.World.Config.EnableVoice)
	fmt.Printf("    caller: UID %d %q\n", res.World.CallerUID, callerNameForTest(world))
	fmt.Printf("    players: %s\n", testPlayerList(world))
	fmt.Printf("    areas: %s\n", testAreaList(world))
	fmt.Printf("  Run: /%s %s\n", custom.Name, strings.Join(args, " "))

	switch res.Decision {
	case DecisionRejectedBadArgs:
		fmt.Printf("  Decision: rejected (not enough args; needs %d)\n", custom.MinArgs)
		return
	case DecisionRejectedValidation:
		fmt.Println("  Decision: rejected (validation)")
		return
	}

	fmt.Println("  Decision: accepted")
	if len(res.Effects) == 0 {
		fmt.Println("  Effects: (none)")
	} else {
		fmt.Println("  Effects:")
		for i, e := range res.Effects {
			fmt.Printf("    %d. %s\n", i+1, effectString(e))
		}
	}
	fmt.Println("  Resulting state:")
	for _, uid := range sortedPlayerUIDs(res.World) {
		p := res.World.Players[uid]
		fmt.Printf("    UID %d: %s\n", uid, playerStateString(p))
	}
}

func callerNameForTest(w TestWorld) string {
	if c, ok := w.Players[w.CallerUID]; ok {
		return c.Name
	}
	return "?"
}

func testPlayerList(w TestWorld) string {
	var parts []string
	for _, uid := range sortedPlayerUIDs(w) {
		p := w.Players[uid]
		parts = append(parts, fmt.Sprintf("%d (%s)", uid, p.Name))
	}
	return strings.Join(parts, ", ")
}

func testAreaList(w TestWorld) string {
	var parts []string
	for _, a := range w.Areas {
		parts = append(parts, a.Name)
	}
	return strings.Join(parts, ", ")
}

func sortedPlayerUIDs(w TestWorld) []int {
	uids := make([]int, 0, len(w.Players))
	for uid := range w.Players {
		uids = append(uids, uid)
	}
	sort.Ints(uids)
	return uids
}

func effectString(e Effect) string {
	targets := strings.Trim(strings.ReplaceAll(fmt.Sprint(e.To), " ", ","), "[]")
	switch e.Kind {
	case EffectMessage:
		return fmt.Sprintf("message → UID %s: %q", targets, e.Text)
	case EffectAnnounce:
		return fmt.Sprintf("announce: %q", e.Text)
	case EffectPunishApplied:
		return fmt.Sprintf("punish UID %s: %s for %v (%q)", targets, e.Punishment.String(), e.Duration, e.Reason)
	case EffectPunishRemoved:
		if e.Punishment == PunishmentNone {
			return fmt.Sprintf("unpunish UID %s: all", targets)
		}
		return fmt.Sprintf("unpunish UID %s: %s", targets, e.Punishment.String())
	case EffectKick:
		return fmt.Sprintf("kick UID %s (%q)", targets, e.Reason)
	case EffectBan:
		return fmt.Sprintf("ban UID %s for %v (%q)", targets, e.Duration, e.Reason)
	case EffectMute:
		return fmt.Sprintf("mute UID %s: %s for %v", targets, e.Scope.String(), e.Duration)
	case EffectUnmute:
		return fmt.Sprintf("unmute UID %s", targets)
	case EffectMove:
		return fmt.Sprintf("move UID %s → %s", targets, e.Area)
	case EffectRun:
		return fmt.Sprintf("run /%s %s", e.Command, strings.Join(e.Args, " "))
	case EffectGrant:
		return fmt.Sprintf("grant %q to account %q", e.Command, e.Account)
	case EffectRevoke:
		return fmt.Sprintf("revoke %q from account %q", e.Command, e.Account)
	case EffectWait:
		return fmt.Sprintf("wait %v", e.Duration)
	}
	return "?"
}

func playerStateString(p *TestPlayer) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("muted=%v", p.Muted.String()))
	if len(p.Punishments) == 0 {
		parts = append(parts, "punishments=[]")
	} else {
		var names []string
		for _, x := range p.Punishments {
			names = append(names, x.Type.String())
		}
		parts = append(parts, "punishments=["+strings.Join(names, ", ")+"]")
	}
	if p.AreaName != "" {
		parts = append(parts, "area="+p.AreaName)
	}
	return strings.Join(parts, " ")
}

func cmdCustomImport(scanner *bufio.Scanner) {
	line, ok := readLine(scanner, "  JSON file path (or 'paste' to type JSON) > ")
	if !ok || line == "" {
		return
	}
	if strings.EqualFold(line, "paste") {
		importPastedJSON(scanner)
		return
	}
	cmd, err := parseCustomCommandFile(line)
	if err != nil {
		fmt.Printf("  import failed: %v\n", err)
		return
	}
	path, err := writeCustomCommandFile(cmd)
	if err != nil {
		fmt.Printf("  import failed: %v\n", err)
		return
	}
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload after import failed: %v\n", err)
		return
	}
	fmt.Printf("  imported /%s → %s\n", cmd.Name, path)
}

func importPastedJSON(scanner *bufio.Scanner) {
	fmt.Println("  paste JSON, then a line containing only END:")
	var sb strings.Builder
	for {
		line, ok := readLine(scanner, "  > ")
		if !ok {
			return
		}
		if strings.TrimSpace(line) == "END" {
			break
		}
		sb.WriteString(line + "\n")
	}
	var cmd CustomCommand
	if err := json.Unmarshal([]byte(sb.String()), &cmd); err != nil {
		fmt.Printf("  invalid JSON: %v\n", err)
		return
	}
	cmd = cmd.resolved()
	if err := validateCustomCommand(cmd); err != nil {
		fmt.Printf("  invalid command: %v\n", err)
		return
	}
	path, err := writeCustomCommandFile(cmd)
	if err != nil {
		fmt.Printf("  save failed: %v\n", err)
		return
	}
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload failed: %v\n", err)
		return
	}
	fmt.Printf("  saved /%s → %s\n", cmd.Name, path)
}

func cmdCustomExamples() {
	fmt.Println("  example templates (print one with: customcmd example <name>, or 8 then a name):")
	for _, ex := range customCommandExamples() {
		fmt.Printf("    %-22s %s\n", ex.Name, ex.Desc)
	}
}

func printExample(kind string) {
	for _, ex := range customCommandExamples() {
		if strings.EqualFold(ex.Name, kind) {
			data, _ := json.MarshalIndent(ex, "", "  ")
			fmt.Println(string(data))
			return
		}
	}
	fmt.Printf("  no example named %q. Run 'customcmd example' to list them.\n", kind)
}

// customCommandExamples returns a ready-to-load set of example commands.
func customCommandExamples() []CustomCommand {
	return []CustomCommand{
		{
			Name: "boop", Desc: "Boop a player on the nose.", Category: "fun",
			ReqPerms: "MOD_SPEAK", Usage: "/boop <uid>", MinArgs: 1, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionMessage, Target: "@args", Text: "You were booped by {caller.name}!"},
			},
		},
		{
			Name: "greet", Desc: "Announce yourself to the whole server.", Category: "fun",
			ReqPerms: "NONE", Usage: "/greet", MinArgs: 0, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionAnnounce, Text: "{caller.name} says hello to everyone!"},
			},
		},
		{
			Name: "silence", Desc: "Mute + whisper + notify a player.", Category: "moderation",
			ReqPerms: "MUTE", Usage: "/silence <uid>", MinArgs: 1, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionMute, Target: "@args", Scope: "ic", Duration: "10m", Reason: "being too loud"},
				{Type: ActionPunish, Target: "@args", Effect: "whisper", Duration: "10m"},
				{Type: ActionMessage, Target: "@args", Text: "You have been silenced by {caller.name}."},
			},
		},
		{
			Name: "megapunish", Desc: "Stack several punishments via run.", Category: "punishment",
			ReqPerms: "ADMIN", Usage: "/megapunish <uid>", MinArgs: 1, PublicHelp: false,
			Actions: []CustomAction{
				{Type: ActionRun, Command: "backward", Args: []string{"{args.0}", "-d", "5m"}},
				{Type: ActionRun, Command: "uwu", Args: []string{"{args.0}", "-d", "5m"}},
				{Type: ActionRun, Command: "tsundere", Args: []string{"{args.0}", "-d", "5m"}},
			},
		},
		{
			Name: "randominsult", Desc: "Pick a random message.", Category: "fun",
			ReqPerms: "MOD_SPEAK", Usage: "/randominsult <uid>", MinArgs: 1, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionRandom, Choices: []CustomActionList{
					{Actions: []CustomAction{{Type: ActionMessage, Target: "@args", Text: "You are a silly goose."}}},
					{Actions: []CustomAction{{Type: ActionMessage, Target: "@args", Text: "You are a magnificent disaster."}}},
					{Actions: []CustomAction{{Type: ActionMessage, Target: "@args", Text: "You are a walking compiler error."}}},
				}},
			},
		},
		{
			Name: "rouletteparty", Desc: "Random punishment for the target.", Category: "punishment",
			ReqPerms: "MUTE", Usage: "/rouletteparty <uid>", MinArgs: 1, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionRandom, Choices: []CustomActionList{
					{Actions: []CustomAction{{Type: ActionPunish, Target: "@args", Effect: "uwu", Duration: "5m"}}},
					{Actions: []CustomAction{{Type: ActionPunish, Target: "@args", Effect: "backward", Duration: "5m"}}},
					{Actions: []CustomAction{{Type: ActionPunish, Target: "@args", Effect: "drunk", Duration: "5m"}}},
				}},
			},
		},
		{
			Name: "cleanup", Desc: "Lift every effect and unmute.", Category: "moderation",
			ReqPerms: "MUTE", Usage: "/cleanup <uid>", MinArgs: 1, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionUnpunish, Target: "@args", Effect: "all"},
				{Type: ActionUnmute, Target: "@args"},
				{Type: ActionMessage, Target: "@args", Text: "You have been cleaned up by {caller.name}."},
			},
		},
		{
			Name: "banwave", Desc: "Ban a player and announce it.", Category: "moderation",
			ReqPerms: "BAN", Usage: "/banwave <uid> <reason>", MinArgs: 2, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionBan, Target: "@args", Duration: "24h", Reason: "{args.1}"},
				{Type: ActionAnnounce, Text: "{caller.name} has banned {target.name}."},
			},
		},
		{
			Name: "templating-showcase", Desc: "Shows every placeholder.", Category: "fun",
			ReqPerms: "NONE", Usage: "/templating-showcase <uid>", MinArgs: 1, PublicHelp: true,
			Actions: []CustomAction{
				{Type: ActionMessage, Target: "@args", Text: "caller={caller.name} uid={caller.uid} target={target.name} uid={target.uid} args0={args.0}"},
			},
		},
	}
}

// wizardNew is the guided, beginner-first command builder. If name is non-empty
// the existing command of that name is loaded as the starting point (edit mode).
func wizardNew(scanner *bufio.Scanner, name string) {
	cmd := CustomCommand{}
	if name != "" {
		if existing, ok := getCustomCommand(name); ok {
			cmd = existing
		}
	}

	if cmd.Name == "" {
		line, ok := readLine(scanner, "  What is the command called? > ")
		if !ok || line == "" {
			fmt.Println("  (cancelled)")
			return
		}
		cmd.Name = strings.TrimPrefix(strings.TrimSpace(line), "/")
	}
	if cmd.Desc == "" {
		line, _ := readLine(scanner, "  What does it do? > ")
		cmd.Desc = strings.TrimSpace(line)
	}
	if cmd.Category == "" {
		line, _ := readLine(scanner, "  Category [custom] > ")
		if strings.TrimSpace(line) != "" {
			cmd.Category = strings.TrimSpace(line)
		} else {
			cmd.Category = "custom"
		}
	}
	cmd.ReqPerms, cmd.Account = readPermission(scanner)
	if cmd.Usage == "" {
		cmd.Usage = "/" + cmd.Name
	}
	if cmd.MinArgs == 0 {
		line, _ := readLine(scanner, "  Minimum arguments [0] > ")
		if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			cmd.MinArgs = n
		}
	}

	fmt.Println("  Now add actions. Pick a number, or type a keyword like 'mute':")
	for {
		action, done := wizardPickAction(scanner)
		if done {
			break
		}
		cmd.Actions = append(cmd.Actions, action)
	}

	wizardSummary(cmd)
	if !readYesNo(scanner, "  Save this command? (y/N) > ", false) {
		return
	}
	path, err := writeCustomCommandFile(cmd)
	if err != nil {
		fmt.Printf("  save failed: %v\n", err)
		return
	}
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload failed: %v\n", err)
		return
	}
	fmt.Printf("  ✓ saved /%s → %s\n", cmd.Name, path)
	fmt.Printf("  Test it now? Use: customcmd test %s <uid>\n", cmd.Name)
	fmt.Printf("  Grant it to one account: grantcmd <username> %s\n", cmd.Name)
}

// wizardSummary prints a plain-English description of the finished command.
func wizardSummary(cmd CustomCommand) {
	fmt.Println("  ── That's it! Here's what /" + cmd.Name + " will do: ──")
	if cmd.Account != "" {
		fmt.Printf("    • account only: %s\n", cmd.Account)
	} else {
		fmt.Printf("    • permission: %s\n", cmd.ReqPerms)
	}
	for _, a := range cmd.Actions {
		fmt.Printf("    • %s\n", actionSummary(a))
	}
	fmt.Println("  ──────────────────────────────────────────────")
}

func actionSummary(a CustomAction) string {
	switch a.Type {
	case ActionMessage:
		return fmt.Sprintf("send a message to %s: %q", a.Target, a.Text)
	case ActionAnnounce:
		return fmt.Sprintf("announce to everyone: %q", a.Text)
	case ActionPunish:
		return fmt.Sprintf("punish %s with %q for %s", a.Target, a.Effect, orDefault(a.Duration, "10m"))
	case ActionUnpunish:
		return fmt.Sprintf("unpunish %s (%s)", a.Target, orDefault(a.Effect, "all"))
	case ActionKick:
		return fmt.Sprintf("kick %s", a.Target)
	case ActionBan:
		return fmt.Sprintf("ban %s for %s", a.Target, orDefault(a.Duration, "24h"))
	case ActionMute:
		return fmt.Sprintf("mute %s (%s)", a.Target, orDefault(a.Scope, "ic"))
	case ActionUnmute:
		return fmt.Sprintf("unmute %s", a.Target)
	case ActionMove:
		return fmt.Sprintf("move %s to %q", a.Target, a.Area)
	case ActionRun:
		return fmt.Sprintf("run /%s %s", a.Command, strings.Join(a.Args, " "))
	case ActionRandom:
		return fmt.Sprintf("pick one of %d random options", len(a.Choices))
	case ActionText:
		var parts []string
		for _, r := range a.Replace {
			parts = append(parts, fmt.Sprintf("%q→%q", r.From, r.To))
		}
		s := "text-effect " + a.Target
		if len(parts) > 0 {
			s += " replace " + strings.Join(parts, ", ")
		}
		if len(a.Random) > 0 {
			s += fmt.Sprintf(" (sometimes → %q)", strings.Join(a.Random, "/"))
		}
		return s
	case ActionWait:
		return fmt.Sprintf("wait %s", orDefault(a.Duration, "0s"))
	case ActionGrant:
		return fmt.Sprintf("grant %q to account %q", a.Command, a.Target)
	case ActionRevoke:
		return fmt.Sprintf("revoke %q from account %q", a.Command, a.Target)
	}
	return string(a.Type)
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// targetOptions lists the target selectors the wizard offers.
var targetOptions = []struct {
	key   string
	label string
}{
	{"@args", "the player(s) typed in the command"},
	{"@self", "the person running it"},
	{"@area", "everyone in the area"},
	{"@global", "everyone in the area except staff"},
	{"@all", "everyone on the server"},
}

func readTarget(scanner *bufio.Scanner, def string) string {
	fmt.Println("  Who? (pick a number, or Enter for " + def + ")")
	for i, t := range targetOptions {
		fmt.Printf("    %d. %s (%s)\n", i+1, t.key, t.label)
	}
	line, _ := readLine(scanner, "    > ")
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(targetOptions) {
		return targetOptions[n-1].key
	}
	return line
}

// matchActionKeyword maps a free-typed string to an action type, so operators
// can type "mute someone" instead of a number. Returns false when unmatched.
func matchActionKeyword(s string) (CustomActionType, bool) {
	s = strings.ToLower(s)
	switch {
	case containsAny(s, "message", "say", "send", "msg", "tell", "pm"):
		return ActionMessage, true
	case containsAny(s, "announce", "broadcast", "shout", "yell"):
		return ActionAnnounce, true
	case containsAny(s, "text effect", "text transform", "find and replace", "replace ", "punctuation", "period"):
		return ActionText, true
	case containsAny(s, "unpunish", "remove punishment", "cleanse", "clear effect"):
		return ActionUnpunish, true
	case containsAny(s, "punish", "punishment", "effect"):
		return ActionPunish, true
	case containsAny(s, "unmute"):
		return ActionUnmute, true
	case containsAny(s, "kick"):
		return ActionKick, true
	case containsAny(s, "ban"):
		return ActionBan, true
	case containsAny(s, "mute", "silence"):
		return ActionMute, true
	case containsAny(s, "move", "teleport", "warp", "send them to"):
		return ActionMove, true
	case containsAny(s, "random", "chance", "roll", "spin"):
		return ActionRandom, true
	case containsAny(s, "wait", "delay", "pause"):
		return ActionWait, true
	case containsAny(s, "run", "existing", "built", "another command"):
		return ActionRun, true
	case containsAny(s, "grant"):
		return ActionGrant, true
	case containsAny(s, "revoke"):
		return ActionRevoke, true
	}
	return "", false
}

func containsAny(s string, keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// wizardPickAction prompts for the next action to add. It returns done=true when
// the operator is finished.
func wizardPickAction(scanner *bufio.Scanner) (CustomAction, bool) {
	fmt.Println("    Add an action (number or keyword):")
	fmt.Println("      1. message     send a message")
	fmt.Println("      2. announce    broadcast to everyone")
	fmt.Println("      3. punish      apply a punishment")
	fmt.Println("      4. unpunish    remove a punishment")
	fmt.Println("      5. kick        kick them")
	fmt.Println("      6. ban         ban them")
	fmt.Println("      7. mute        mute them")
	fmt.Println("      8. unmute      unmute them")
	fmt.Println("      9. move        move them to an area")
	fmt.Println("      10. run        run an existing command")
	fmt.Println("      11. random     pick one of several at random")
	fmt.Println("      12. text       change how their messages come out (find/replace, random words)")
	fmt.Println("      13. wait       pause before the next action")
	fmt.Println("      14. grant      grant a command to an account")
	fmt.Println("      15. revoke     revoke a command")
	fmt.Println("      16. done       finish and save")
	line, ok := readLine(scanner, "    > ")
	if !ok {
		return CustomAction{}, true
	}
	line = strings.TrimSpace(line)
	if line == "" || line == "16" || line == "done" || line == "save" || line == "finish" {
		return CustomAction{}, true
	}
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(validActionTypes) {
		return wizardFillAction(scanner, validActionTypes[n-1]), false
	}
	if t, ok := matchActionKeyword(line); ok {
		return wizardFillAction(scanner, t), false
	}
	fmt.Println("    (unknown — pick 1-16 or type a keyword like 'mute')")
	return wizardPickAction(scanner)
}

func wizardFillAction(scanner *bufio.Scanner, t CustomActionType) CustomAction {
	a := CustomAction{Type: t}
	switch t {
	case ActionMessage, ActionAnnounce:
		a.Text = readRequired(scanner, "    message text > ")
		if t == ActionMessage {
			a.Target = readTarget(scanner, "@args")
		}
	case ActionPunish:
		a.Effect = readPunishmentEffect(scanner)
		a.Duration = readWithDefault(scanner, "    for how long? [10m] > ", "10m")
		a.Reason = readOptional(scanner, "    why? (optional) > ")
		a.Target = readTarget(scanner, "@args")
	case ActionUnpunish:
		a.Effect = readWithDefault(scanner, "    which effect? (a name, or 'all') [all] > ", "all")
		a.Target = readTarget(scanner, "@args")
	case ActionKick:
		a.Reason = readOptional(scanner, "    reason? (optional) > ")
		a.Target = readTarget(scanner, "@args")
	case ActionBan:
		a.Duration = readWithDefault(scanner, "    for how long? [24h] > ", "24h")
		a.Reason = readOptional(scanner, "    reason? (optional) > ")
		a.Target = readTarget(scanner, "@args")
	case ActionMute:
		a.Scope = readScope(scanner)
		a.Duration = readWithDefault(scanner, "    for how long? (empty = permanent) [10m] > ", "10m")
		a.Reason = readOptional(scanner, "    reason? (optional) > ")
		a.Target = readTarget(scanner, "@args")
	case ActionUnmute:
		a.Target = readTarget(scanner, "@args")
	case ActionMove:
		a.Area = readRequired(scanner, "    to which area? > ")
		a.Target = readTarget(scanner, "@args")
	case ActionRun:
		a.Command = readRequired(scanner, "    which command to run? > ")
		if argsLine := readOptional(scanner, "    args? (space-separated; {args.0} = the player) > "); argsLine != "" {
			a.Args = strings.Fields(argsLine)
		}
	case ActionRandom:
		effects := readWithDefault(scanner, "    punishment effects (comma-separated, e.g. uwu,backward,drunk) > ", "")
		dur := readWithDefault(scanner, "    for how long? [5m] > ", "5m")
		target := readTarget(scanner, "@args")
		for _, eff := range strings.Split(effects, ",") {
			eff = strings.TrimSpace(eff)
			if eff == "" {
				continue
			}
			a.Choices = append(a.Choices, CustomActionList{Actions: []CustomAction{{Type: ActionPunish, Target: target, Effect: eff, Duration: dur}}})
		}
		if len(a.Choices) < 2 {
			fmt.Println("    (random needs at least 2 effects — it will be skipped; add more next time)")
		}
	case ActionText:
		a.Target = readTarget(scanner, "@args")
		a.Duration = readWithDefault(scanner, "    for how long? [10m] > ", "10m")
		a.Reason = readOptional(scanner, "    why? (optional) > ")
		for {
			from := readOptional(scanner, "    replace this (e.g. . ) — Enter to stop > ")
			if from == "" {
				break
			}
			to := readWithDefault(scanner, "    with what? > ", "")
			a.Replace = append(a.Replace, TextReplace{From: from, To: to})
		}
		pool := readOptional(scanner, "    sometimes replace the whole message with (comma-separated) > ")
		if pool != "" {
			for _, s := range strings.Split(pool, ",") {
				if s = strings.TrimSpace(s); s != "" {
					a.Random = append(a.Random, s)
				}
			}
		}
		if len(a.Random) > 0 {
			chance := readWithDefault(scanner, "    how often? (0-1, e.g. 0.25) [0.25] > ", "0.25")
			if f, err := strconv.ParseFloat(chance, 64); err == nil {
				a.Chance = f
			} else {
				a.Chance = 0.25
			}
		}
	case ActionWait:
		a.Duration = readWithDefault(scanner, "    how long to wait? [1s] > ", "1s")
	case ActionGrant, ActionRevoke:
		a.Target = readWithDefault(scanner, "    account username? ({args.0} = pass it) > ", "{args.0}")
		a.Command = readRequired(scanner, "    which command to grant/revoke? > ")
	}
	return a
}

func readRequired(scanner *bufio.Scanner, prompt string) string {
	for {
		line, ok := readLine(scanner, prompt)
		if !ok {
			return ""
		}
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
		fmt.Println("    (this is required)")
	}
}

func readOptional(scanner *bufio.Scanner, prompt string) string {
	line, _ := readLine(scanner, prompt)
	return strings.TrimSpace(line)
}

func readWithDefault(scanner *bufio.Scanner, prompt, def string) string {
	line, _ := readLine(scanner, prompt)
	if strings.TrimSpace(line) == "" {
		return def
	}
	return strings.TrimSpace(line)
}

func readPunishmentEffect(scanner *bufio.Scanner) string {
	names := allPunishmentNames()
	hint := ""
	if len(names) > 6 {
		hint = strings.Join(names[:6], ", ") + ", …"
	} else {
		hint = strings.Join(names, ", ")
	}
	fmt.Printf("    which effect? (e.g. %s)\n", hint)
	for {
		line, _ := readLine(scanner, "    > ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := punishmentTypeByName(line); ok {
			return strings.ToLower(line)
		}
		fmt.Printf("    (unknown effect %q — try one of: %s)\n", line, hint)
	}
}

func readScope(scanner *bufio.Scanner) string {
	fmt.Println("    mute scope: ic, ooc, both, music, jud [ic]")
	line, _ := readLine(scanner, "    > ")
	line = strings.TrimSpace(line)
	if line == "" {
		return "ic"
	}
	if validMuteScope(line) {
		return strings.ToLower(line)
	}
	fmt.Println("    (unknown scope — using ic)")
	return "ic"
}

// customcmdConsole is the non-interactive entry point invoked from ListenInput's
// "customcmd" case. With no subcommand it opens the interactive menu.
func customcmdConsole(scanner *bufio.Scanner, cmd []string) {
	if !customCommandsEnabled() {
		fmt.Println("  Custom commands are disabled. Set enable_custom_commands = true in config.toml to enable the builder.")
		return
	}
	if len(cmd) == 1 {
		// Suppress the streaming server log while the interactive menu is open
		// so prompts don't interleave with "Client timed out" and friends.
		logger.MuteConsole()
		defer logger.UnmuteConsole()
		runCustomCommandMenu(scanner)
		return
	}
	sub := strings.ToLower(cmd[1])
	rest := cmd[2:]
	switch sub {
	case "list", "ls":
		cmdCustomList()
	case "show":
		if len(rest) == 0 {
			fmt.Println("  usage: customcmd show <name>")
			return
		}
		cmdCustomShow(rest[0])
	case "test":
		if len(rest) > 0 {
			runCustomTest(rest[0], rest[1:])
		} else {
			cmdCustomTest(scanner)
		}
	case "reload":
		cmdCustomReload()
	case "validate":
		cmdCustomValidate()
	case "import":
		if len(rest) > 0 {
			if strings.EqualFold(rest[0], "paste") {
				importPastedJSON(scanner)
				return
			}
			imported, err := parseCustomCommandFile(rest[0])
			if err != nil {
				fmt.Printf("  import failed: %v\n", err)
				return
			}
			path, err := writeCustomCommandFile(imported)
			if err != nil {
				fmt.Printf("  import failed: %v\n", err)
				return
			}
			if err := loadCustomCommands(); err != nil {
				fmt.Printf("  reload failed: %v\n", err)
				return
			}
			fmt.Printf("  imported /%s → %s\n", imported.Name, path)
			return
		}
		cmdCustomImport(scanner)
	case "delete", "remove", "rm":
		if len(rest) > 0 {
			deleteCustomByName(rest[0])
		} else {
			cmdCustomDelete(scanner)
		}
	case "example", "examples":
		if len(rest) > 0 {
			printExample(rest[0])
		} else {
			cmdCustomExamples()
		}
	case "quick":
		quickCreate(scanner, rest)
	case "help", "?":
		printMenuHelp()
	default:
		fmt.Println("  unknown subcommand. Try: customcmd list|show|test|reload|validate|import|delete|example|quick")
	}
}

func deleteCustomByName(name string) {
	if _, ok := getCustomCommand(name); !ok {
		fmt.Printf("  no custom command named %q\n", name)
		return
	}
	if err := deleteCustomCommandFile(name); err != nil {
		fmt.Printf("  delete failed: %v\n", err)
		return
	}
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload after delete failed: %v\n", err)
		return
	}
	fmt.Printf("  deleted /%s.\n", strings.ToLower(name))
}

// quickCreate builds a command from a one-liner, e.g.
//
//	customcmd quick mute uwu 10m
//	customcmd quick boop message "boops on the nose"
//
// The first token is the action keyword unless it isn't one, in which case it
// is the command name and the second token is the action.
func quickCreate(scanner *bufio.Scanner, args []string) {
	_ = scanner
	if len(args) == 0 {
		fmt.Println("  usage: customcmd quick [name] <action> <params...>")
		fmt.Println("  e.g.   customcmd quick mute uwu 10m")
		fmt.Println("         customcmd quick boop message \"boops on the nose\"")
		return
	}
	name := ""
	actionWord := args[0]
	params := args[1:]
	if _, isAction := matchActionKeyword(actionWord); !isAction {
		if len(args) < 2 {
			fmt.Println("  usage: customcmd quick [name] <action> <params...>")
			return
		}
		name = actionWord
		actionWord = args[1]
		params = args[2:]
	}
	t, ok := matchActionKeyword(actionWord)
	if !ok {
		fmt.Printf("  unknown action %q. Use: message, announce, punish, unpunish, kick, ban, mute, unmute, move, run\n", actionWord)
		return
	}
	if name == "" {
		name = string(t)
	}

	cmd := CustomCommand{Name: name, Category: "custom", Usage: "/" + name, Desc: "Created with customcmd quick."}
	cmd.ReqPerms = quickPermFor(t)
	cmd.Actions = []CustomAction{quickAction(t, params)}
	if cmd.ReqPerms == "NONE" {
		cmd.MinArgs = 0
	} else {
		cmd.MinArgs = 1
	}

	path, err := writeCustomCommandFile(cmd)
	if err != nil {
		fmt.Printf("  quick failed: %v\n", err)
		return
	}
	if err := loadCustomCommands(); err != nil {
		fmt.Printf("  reload failed: %v\n", err)
		return
	}
	fmt.Printf("  ✓ saved /%s → %s\n", cmd.Name, path)
	fmt.Printf("  Test it: customcmd test %s 5\n", cmd.Name)
}

func quickPermFor(t CustomActionType) string {
	switch t {
	case ActionMessage, ActionAnnounce:
		return "NONE"
	case ActionRun, ActionGrant, ActionRevoke:
		return "ADMIN"
	default:
		return "MUTE"
	}
}

func quickAction(t CustomActionType, params []string) CustomAction {
	a := CustomAction{Type: t}
	switch t {
	case ActionMessage, ActionAnnounce:
		a.Text = strings.Join(params, " ")
		if a.Text == "" {
			a.Text = "hello!"
		}
		if t == ActionMessage {
			a.Target = "@args"
		}
	case ActionPunish:
		a.Target = "@args"
		if len(params) > 0 {
			a.Effect = params[0]
		}
		if len(params) > 1 {
			a.Duration = params[1]
		} else {
			a.Duration = "10m"
		}
	case ActionUnpunish:
		a.Target = "@args"
		if len(params) > 0 {
			a.Effect = params[0]
		} else {
			a.Effect = "all"
		}
	case ActionKick:
		a.Target = "@args"
		a.Reason = strings.Join(params, " ")
	case ActionBan:
		a.Target = "@args"
		if len(params) > 0 {
			a.Duration = params[0]
		} else {
			a.Duration = "24h"
		}
		a.Reason = strings.Join(params[1:], " ")
	case ActionMute:
		a.Target = "@args"
		if len(params) > 0 {
			a.Scope = params[0]
		} else {
			a.Scope = "ic"
		}
		if len(params) > 1 {
			a.Duration = params[1]
		} else {
			a.Duration = "10m"
		}
	case ActionUnmute:
		a.Target = "@args"
	case ActionMove:
		a.Target = "@args"
		if len(params) > 0 {
			a.Area = params[0]
		}
	case ActionRun:
		if len(params) > 0 {
			a.Command = params[0]
			a.Args = params[1:]
		}
	}
	return a
}
