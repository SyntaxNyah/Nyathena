package athena

import (
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

func assertShieldContains(t *testing.T, label, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("%s: got %q, want substring %q", label, got, want)
	}
}

func assertShieldNotContains(t *testing.T, label, got, bad string) {
	t.Helper()
	if strings.Contains(got, bad) {
		t.Errorf("%s: got %q, must not contain %q", label, got, bad)
	}
}

func TestShieldGatesCharCurse(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdCharCurse(w.mod, []string{"2", "Maya Fey"}, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Fatalf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got != -1 {
			t.Errorf("spoof mode moved the issuer to char %d", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Forced UID 2 to character Maya Fey.")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
		assertShieldNotContains(t, "target normal message", w.targetConn.String(), "forced you to play")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		cmdCharCurse(w.mod, []string{"2", "Maya Fey"}, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Fatalf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got != 2 {
			t.Errorf("backfire did not curse the issuer: char = %d, want 2", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Forced UID 2 to character Maya Fey.")
		assertShieldNotContains(t, "issuer", strings.ToLower(w.modConn.String()), "backfire")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesCharStuck(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdCharStuck(w.mod, []string{"-d", "5m", "2"}, "usage")

		if w.target.IsCharStuck() {
			t.Fatal("shielded target was char-stuck")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Stuck [2]")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		cmdCharStuck(w.mod, []string{"-d", "5m", "2", "Maya Fey"}, "usage")

		if w.target.IsCharStuck() {
			t.Fatal("shielded target was char-stuck")
		}
		if got := w.mod.CharID(); got != 2 {
			t.Errorf("backfire issuer char = %d, want 2", got)
		}
		if !w.mod.IsCharStuck() {
			t.Error("backfire did not char-stick the issuer")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Stuck [2]")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesCurseRandomChar(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdCurseRandomChar(w.mod, []string{"2"}, "usage")

		if w.target.curseRandomCharActive.Load() {
			t.Fatal("shielded target was random-char cursed")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Cursed UID 2")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		cmdCurseRandomChar(w.mod, []string{"2"}, "usage")

		if w.target.curseRandomCharActive.Load() {
			t.Fatal("shielded target was random-char cursed")
		}
		if !w.mod.curseRandomCharActive.Load() {
			t.Error("backfire did not curse the issuer")
		}
		w.mod.disarmCurseRandomChar()
		assertShieldContains(t, "issuer", w.modConn.String(), "Cursed UID 2")
		assertShieldNotContains(t, "issuer", strings.ToLower(w.modConn.String()), "backfire")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesForceRandomChar(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdForceRandomChar(w.mod, []string{"2"}, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Fatalf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got != -1 {
			t.Errorf("spoof mode moved the issuer to char %d", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Forced UID 2 to a random character.")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		cmdForceRandomChar(w.mod, []string{"2"}, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Fatalf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got == -1 || got == 0 {
			t.Fatalf("backfire did not random-char the issuer: char = %d", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Forced UID 2 to a random character.")
		assertShieldNotContains(t, "issuer", strings.ToLower(w.modConn.String()), "backfire")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesCharSelect(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.mod.perms |= permissions.PermissionField["CM"]
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdCharSelect(w.mod, []string{"2"}, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Fatalf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got != -1 {
			t.Errorf("spoof mode moved the issuer to char %d", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Moved 1 users to character select.")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.mod.perms |= permissions.PermissionField["CM"]
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)
		w.mod.ChangeCharacter(2)

		cmdCharSelect(w.mod, []string{"2"}, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Fatalf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got != -1 {
			t.Errorf("backfire did not move the issuer to character select: char = %d", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Moved 1 users to character select.")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesForcePos(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetPos("wit")
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdForcePos(w.mod, []string{"2", "def"}, "usage")

		if got := w.target.Pos(); got != "wit" {
			t.Errorf("shielded target pos = %q, want %q", got, "wit")
		}
		if got := w.mod.Pos(); got == "def" {
			t.Error("spoof mode moved the issuer")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Forced 1 player(s) to position: def.")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetPos("wit")
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		cmdForcePos(w.mod, []string{"2", "def"}, "usage")

		if got := w.target.Pos(); got != "wit" {
			t.Errorf("shielded target pos = %q, want %q", got, "wit")
		}
		if got := w.mod.Pos(); got != "def" {
			t.Errorf("backfire issuer pos = %q, want %q", got, "def")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Forced 1 player(s) to position: def.")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesReverseName(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetShowname("Phoenix")
		w.mod.SetShowname("Moderator")
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdReverseName(w.mod, []string{"2"}, "usage")

		if got := w.target.ForcedShowname(); got != "" {
			t.Errorf("shielded target forced showname = %q, want empty", got)
		}
		if got := w.mod.ForcedShowname(); got != "" {
			t.Errorf("spoof mode reversed the issuer: %q", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Reversed the showname of 1 player(s).")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetShowname("Phoenix")
		w.mod.SetShowname("Moderator")
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)

		cmdReverseName(w.mod, []string{"2"}, "usage")

		if got := w.target.ForcedShowname(); got != "" {
			t.Errorf("shielded target forced showname = %q, want empty", got)
		}
		if got := decode(w.mod.ForcedShowname()); got != "rotaredoM" {
			t.Errorf("backfire issuer showname = %q, want %q", got, "rotaredoM")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Reversed the showname of 1 player(s).")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
	})
}

func TestShieldGatesAreaAppearance(t *testing.T) {
	cases := []struct {
		name  string
		apply func(w *shieldTestWorld)
		want  string
	}{
		{
			name: "tung",
			apply: func(w *shieldTestWorld) {
				cmdTung(w.mod, []string{"global"}, "usage")
			},
			want: tungForcedCharacterName,
		},
		{
			name: "areainiswap",
			apply: func(w *shieldTestWorld) {
				cmdAreaIniswap(w.mod, []string{"Maya Fey"}, "usage")
			},
			want: "Maya Fey",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name+" spoof", func(t *testing.T) {
			w := newShieldGatedWorld(t)
			w.target.SetAntiPunishShield(antiPunishShieldSpoof)

			tc.apply(w)

			if name, _ := w.target.ForcedIniswapInfo(); name != "" {
				t.Errorf("shielded target forced iniswap = %q, want empty", name)
			}
			if name, _ := w.mod.ForcedIniswapInfo(); name != tc.want {
				t.Errorf("issuer forced iniswap = %q, want %q", name, tc.want)
			}
			if name, _ := w.other.ForcedIniswapInfo(); name != tc.want {
				t.Errorf("bystander forced iniswap = %q, want %q", name, tc.want)
			}
			assertShieldContains(t, "issuer", w.modConn.String(), "3 client(s)")
			assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
		})

		t.Run(tc.name+" backfire", func(t *testing.T) {
			w := newShieldGatedWorld(t)
			w.target.SetAntiPunishShield(antiPunishShieldBackfire)

			tc.apply(w)

			if name, _ := w.target.ForcedIniswapInfo(); name != "" {
				t.Errorf("shielded target forced iniswap = %q, want empty", name)
			}
			if name, _ := w.mod.ForcedIniswapInfo(); name != tc.want {
				t.Errorf("backfire issuer forced iniswap = %q, want %q", name, tc.want)
			}
			assertShieldContains(t, "issuer", w.modConn.String(), "3 client(s)")
			assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
		})
	}
}

func TestShieldSkipsMassShuffles(t *testing.T) {
	t.Run("nameshuffle", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetShowname("Target")
		w.other.SetShowname("Other")
		w.mod.SetShowname("Mod")
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdNameShuffle(w.mod, nil, "usage")

		if got := w.target.ForcedShowname(); got != "" {
			t.Errorf("shielded target showname = %q, want unchanged", got)
		}
		if got := w.mod.ForcedShowname(); got != "Other" {
			t.Errorf("issuer shuffled showname = %q, want %q", got, "Other")
		}
		if got := w.other.ForcedShowname(); got != "Mod" {
			t.Errorf("bystander shuffled showname = %q, want %q", got, "Mod")
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Shuffled shownames of 3 players")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("charshuffle", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.mod.ChangeCharacter(2)
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdCharShuffle(w.mod, nil, "usage")

		if got := w.target.CharID(); got != 0 {
			t.Errorf("shielded target char = %d, want 0", got)
		}
		if got := w.mod.CharID(); got != 1 {
			t.Errorf("issuer shuffled char = %d, want 1", got)
		}
		if got := w.other.CharID(); got != 2 {
			t.Errorf("bystander shuffled char = %d, want 2", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Shuffled characters of 3 players")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})
}

func TestShieldGatesForcePair(t *testing.T) {
	t.Run("spoof", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldSpoof)

		cmdForcePair(w.mod, []string{"2", "3"}, "usage")

		if got := w.target.ForcePairUID(); got != -1 {
			t.Errorf("shielded target force-pair uid = %d, want -1", got)
		}
		if got := w.other.ForcePairUID(); got != -1 {
			t.Errorf("bystander force-pair uid = %d, want -1", got)
		}
		if got := w.mod.ForcePairUID(); got != -1 {
			t.Errorf("spoof mode paired the issuer with uid %d", got)
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Force-paired")
		assertShieldContains(t, "target popup", w.targetConn.String(), "It didn't stick")
	})

	t.Run("backfire", func(t *testing.T) {
		w := newShieldGatedWorld(t)
		w.target.SetAntiPunishShield(antiPunishShieldBackfire)
		w.mod.ChangeCharacter(2)

		cmdForcePair(w.mod, []string{"2", "3"}, "usage")

		if got := w.target.ForcePairUID(); got != -1 {
			t.Errorf("shielded target force-pair uid = %d, want -1", got)
		}
		if got := w.other.ForcePairUID(); got != w.mod.Uid() {
			t.Errorf("bystander force-pair uid = %d, want issuer %d", got, w.mod.Uid())
		}
		if got := w.mod.ForcePairUID(); got != w.other.Uid() {
			t.Errorf("issuer force-pair uid = %d, want bystander %d", got, w.other.Uid())
		}
		assertShieldContains(t, "issuer", w.modConn.String(), "Force-paired")
		assertShieldContains(t, "target popup", w.targetConn.String(), "backfired on them instead")
		assertShieldContains(t, "bystander", w.otherConn.String(), "force-paired")
	})
}
