/* Athena - A server for Attorney Online 2 written in Go

   Tests for the uma character pool behind /horse, and for the /trex and
   /fish animal filters. */

package athena

import (
	"os"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/db"
	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// cmdPunishment persists through the db package, so the tests that drive a
// real command need a temp DB.
func setupUmaTestDB(t *testing.T) {
	t.Helper()
	tmp, err := os.CreateTemp("", "athena-uma-*.db")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmp.Close()
	db.DBPath = tmp.Name()
	if err := db.Open(); err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(tmp.Name())
	})
}

// umaTestRoster mixes marked and unmarked characters, including the three
// traps the marker convention has to get right: a same-franchise character
// carrying a DIFFERENT marker, a same-franchise character carrying none, and a
// name that merely contains the letters "uma" without the parenthesised marker.
var umaTestRoster = []string{
	"Phoenix Wright",        // 0 — not uma
	"jungle pocket (uma)",   // 1
	"gold ship (gbf)",       // 2 — different marker, deliberately excluded
	"tokai teio (uma_h)",    // 3
	"still in love",         // 4 — same franchise, unmarked, excluded
	"Gold Ship (UMA_H)",     // 5 — matching is case-insensitive
	"Uma Musume Fan Club",   // 6 — contains "uma" but no marker; excluded
	"agnes tachyon (uma_h)", // 7
	"hayakwawa tazuna",      // 8 — unmarked, excluded
	"special week (uma)",    // 9
}

func setUmaTestRoster(t *testing.T) {
	t.Helper()
	orig := getCharacters()
	t.Cleanup(func() { setCharacters(orig) })
	setCharacters(umaTestRoster)
}

func TestIsUmaCharacter(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"jungle pocket (uma)", true},
		{"tokai teio (uma_h)", true},
		{"Gold Ship (UMA_H)", true},
		{"T.M. Opera O (Uma)", true},
		{"gold ship (gbf)", false},
		{"still in love", false},
		{"hayakwawa tazuna", false},
		{"Uma Musume Fan Club", false}, // "uma" without the parens is not a marker
		{"Phoenix Wright", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isUmaCharacter(c.name); got != c.want {
			t.Errorf("isUmaCharacter(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// The pool is discovered by marker, so appending a new uma character to
// characters.txt is all an operator has to do — no code change, no rebuild.
func TestUmaPoolIsDiscoveredByMarker(t *testing.T) {
	setUmaTestRoster(t)

	got := getUmaCharIDs()
	want := []int{1, 3, 5, 7, 9}
	if len(got) != len(want) {
		t.Fatalf("getUmaCharIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("getUmaCharIDs() = %v, want %v", got, want)
		}
	}

	// Appending a marked character publishes it into the pool.
	setCharacters(append(append([]string{}, umaTestRoster...), "haru urara (uma_h)"))
	if got := getUmaCharIDs(); len(got) != 6 || got[5] != len(umaTestRoster) {
		t.Errorf("after append getUmaCharIDs() = %v, want the new slot %d included", got, len(umaTestRoster))
	}
}

// A server whose characters.txt has no marked characters — the upstream
// default — must get a silent no-op, never an error or a wrong swap.
func TestUmaPoolEmptyOnServerWithoutUmaCharacters(t *testing.T) {
	orig := getCharacters()
	t.Cleanup(func() { setCharacters(orig) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})

	if got := getUmaCharIDs(); len(got) != 0 {
		t.Errorf("getUmaCharIDs() = %v, want empty", got)
	}
}

func setupUmaSwapClients(t *testing.T) (*area.Area, *Client, *Client) {
	t.Helper()
	setUmaTestRoster(t)

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	a := area.NewArea(area.AreaData{Name: "Paddock"}, len(getCharacters()), 10, area.EviAny)

	mod := &Client{conn: &testConn{}, uid: 1, ipid: "ip-mod", char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	mod.SetArea(a)
	a.AddChar(0)
	mod.perms = permissions.PermissionField["MUTE"]

	target := &Client{conn: &testConn{}, uid: 2, ipid: "ip-target", char: 4, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	target.SetArea(a)
	a.AddChar(4)

	for _, c := range []*Client{mod, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}
	return a, mod, target
}

func TestTurnIntoRandomUmaSwapsTheCharacter(t *testing.T) {
	_, _, target := setupUmaSwapClients(t)

	name, ok := turnIntoRandomUma(target)
	if !ok {
		t.Fatal("turnIntoRandomUma reported no swap on a roster full of free uma characters")
	}
	if !isUmaCharacter(name) {
		t.Errorf("turnIntoRandomUma swapped to %q, which carries no uma marker", name)
	}
	if !isUmaCharacter(getCharacters()[target.CharID()]) {
		t.Errorf("target is on slot %d (%q), which is not a uma character",
			target.CharID(), getCharacters()[target.CharID()])
	}
}

// The swap is a cosmetic extra. It must never override another moderator's
// deliberate character lock, and must never report a swap it didn't make.
func TestTurnIntoRandomUmaRespectsCharacterLocks(t *testing.T) {
	t.Run("tunged", func(t *testing.T) {
		_, _, target := setupUmaSwapClients(t)
		target.SetForcedIniswapChar("Phoenix Wright", "0")
		if _, ok := turnIntoRandomUma(target); ok {
			t.Error("turnIntoRandomUma swapped a client with a forced iniswap")
		}
		if target.CharID() != 4 {
			t.Errorf("target moved off char 4 to %d", target.CharID())
		}
	})

	t.Run("spectating", func(t *testing.T) {
		_, _, target := setupUmaSwapClients(t)
		target.SetCharID(-1)
		if _, ok := turnIntoRandomUma(target); ok {
			t.Error("turnIntoRandomUma swapped a spectator")
		}
	})

	t.Run("no free uma slot", func(t *testing.T) {
		a, _, target := setupUmaSwapClients(t)
		for _, id := range getUmaCharIDs() {
			a.AddChar(id)
		}
		if _, ok := turnIntoRandomUma(target); ok {
			t.Error("turnIntoRandomUma swapped although every uma slot was taken")
		}
		if target.CharID() != 4 {
			t.Errorf("target moved off char 4 to %d", target.CharID())
		}
	})
}

// /horse still lands its sound punishment on a server with no uma characters —
// the swap is additive, never a precondition.
func TestHorseAppliesPunishmentWithoutUmaCharacters(t *testing.T) {
	setupUmaTestDB(t)

	origClients := clients
	t.Cleanup(func() { clients = origClients })
	clients = &ClientList{list: make(map[*Client]struct{}), uidIndex: make(map[int]*Client), ipidCounts: make(map[string]int)}

	origChars := getCharacters()
	t.Cleanup(func() { setCharacters(origChars) })
	setCharacters([]string{"Phoenix Wright", "Miles Edgeworth"})

	a := area.NewArea(area.AreaData{Name: "Courtroom"}, len(getCharacters()), 10, area.EviAny)
	mod := &Client{conn: &testConn{}, uid: 1, ipid: "ip-mod", char: 0, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	mod.SetArea(a)
	mod.perms = permissions.PermissionField["MUTE"]
	target := &Client{conn: &testConn{}, uid: 2, ipid: "ip-target", char: 1, possessing: -1, pair: ClientPairInfo{wanted_id: -1}}
	target.SetArea(a)
	for _, c := range []*Client{mod, target} {
		clients.AddClient(c)
		clients.RegisterUID(c)
	}

	cmdHorse(mod, []string{"2"}, "usage")

	if !target.HasActivePunishment(PunishmentHorse) {
		t.Error("/horse did not apply the horse punishment when the server has no uma characters")
	}
	if target.CharID() != 1 {
		t.Errorf("/horse moved the target off char 1 to %d with no uma characters available", target.CharID())
	}
}

func TestHorseTurnsTargetIntoUma(t *testing.T) {
	setupUmaTestDB(t)
	_, mod, target := setupUmaSwapClients(t)

	cmdHorse(mod, []string{"2"}, "usage")

	if !target.HasActivePunishment(PunishmentHorse) {
		t.Fatal("/horse did not apply the horse punishment")
	}
	if !isUmaCharacter(getCharacters()[target.CharID()]) {
		t.Errorf("/horse left the target on %q, which is not a uma character",
			getCharacters()[target.CharID()])
	}
}

// ── /trex and /fish ─────────────────────────────────────────────────────────

func TestTrexAndFishReplaceEveryWord(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) string
		pool []string
		zero string
	}{
		{"trex", applyTrex, trexSounds, "RAAASRFH"},
		{"fish", applyFish, fishSounds, "blublublib"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.fn(""); got != c.zero {
				t.Errorf("%s(\"\") = %q, want %q", c.name, got, c.zero)
			}

			in := "objection your honour the witness is lying"
			wantWords := len(strings.Fields(in))
			pool := map[string]bool{}
			for _, s := range c.pool {
				pool[s] = true
			}
			for i := 0; i < 200; i++ {
				got := c.fn(in)
				if strings.Contains(strings.ToLower(got), "objection") ||
					strings.Contains(strings.ToLower(got), "witness") {
					t.Fatalf("%s leaked the original text: %q", c.name, got)
				}
				if n := len(strings.Fields(got)); n < wantWords {
					// Pool entries may themselves contain a space ("blub blub"),
					// so the count can exceed the input's but never fall short.
					t.Fatalf("%s(%q) = %q — %d words, want at least %d", c.name, in, got, n, wantWords)
				}
			}
		})
	}
}

func TestTrexRoarsTheRequestedRoar(t *testing.T) {
	found := false
	for _, s := range trexSounds {
		if s == "RAAASRFH" {
			found = true
		}
	}
	if !found {
		t.Error("trexSounds no longer contains RAAASRFH — the roar /trex was asked for")
	}
}

func TestFishBlubs(t *testing.T) {
	found := false
	for _, s := range fishSounds {
		if s == "blublublib" {
			found = true
		}
	}
	if !found {
		t.Error("fishSounds no longer contains blublublib — the noise /fish was asked for")
	}
}

// ── registration ────────────────────────────────────────────────────────────

// Every one of these is a moderator tool and must stay gated on MUTE, listed
// in /help, and resolvable by /stack and /unpunish -t.
func TestNewPunishmentCommandsAreModOnlyAndDocumented(t *testing.T) {
	initCommands()

	for _, name := range []string{"horse", "trex", "fish"} {
		t.Run(name, func(t *testing.T) {
			cmd, ok := Commands[name]
			if !ok {
				t.Fatalf("/%s is not registered", name)
			}
			if cmd.handler == nil {
				t.Fatalf("/%s has a nil handler", name)
			}
			if cmd.reqPerms != permissions.PermissionField["MUTE"] {
				t.Errorf("/%s reqPerms = %v, want MUTE — these are moderator tools", name, cmd.reqPerms)
			}
			if cmd.publicHelp {
				t.Errorf("/%s is flagged publicHelp; it should only show to staff", name)
			}
			if cmd.category != "punishment" {
				t.Errorf("/%s category = %q, want \"punishment\"", name, cmd.category)
			}
			if strings.TrimSpace(cmd.desc) == "" {
				t.Errorf("/%s has no desc, so /help would list it blank", name)
			}
			if !strings.Contains(cmd.usage, "/"+name) {
				t.Errorf("/%s usage = %q, which does not name the command", name, cmd.usage)
			}

			// /help punishment renders from these groups; a command missing
			// from all of them would silently vanish from the listing.
			listed := false
			for _, g := range punishmentHelpGroups {
				for _, c := range g.cmds {
					if c == name {
						listed = true
					}
				}
			}
			if !listed {
				t.Errorf("/%s is not in any punishmentHelpGroups block, so /help punishment omits it", name)
			}
		})
	}
}

// /stack <type> and /unpunish -t <type> both resolve through
// parsePunishmentType, so a new punishment that isn't parseable there is
// stackable and removable only by name in the mod's imagination.
func TestNewPunishmentTypesRoundTrip(t *testing.T) {
	for _, p := range []PunishmentType{PunishmentTrex, PunishmentFish} {
		name := p.String()
		if name == "none" {
			t.Errorf("punishment %d has no String() name", p)
			continue
		}
		if got := parsePunishmentType(name); got != p {
			t.Errorf("parsePunishmentType(%q) = %v, want %v", name, got, p)
		}
	}
}
