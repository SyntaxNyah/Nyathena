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

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

func punishmentLockTestSetup(t *testing.T) {
	t.Helper()
	newTestClients(t)
}

// punishmentLockClient creates a registered client for the lock tests.
func punishmentLockClient(t *testing.T, uid int, name string) (*Client, *captureConn) {
	t.Helper()
	conn := &captureConn{}
	c := &Client{
		conn:     conn,
		uid:      uid,
		ipid:     fmt.Sprintf("ip-lock-%d", uid),
		oocName:  name,
		showname: name,
		char:     -1,
		area:     makeTestArea("Lock Test"),
	}
	clients.AddClient(c)
	clients.RegisterUID(c)
	t.Cleanup(func() { clients.RemoveClient(c) })
	return c, conn
}

func TestAddPunishmentLockedAndRemoveUnlocked(t *testing.T) {
	punishmentLockTestSetup(t)
	c, _ := punishmentLockClient(t, 100, "Locked")
	c.AddPunishment(PunishmentPirate, time.Hour, "plain")
	c.AddPunishmentLocked(PunishmentUwu, time.Hour, time.Hour, "Deathroll: rolled a 1")

	if until := c.PunishmentLockedUntil(PunishmentUwu); !until.After(time.Now()) {
		t.Fatalf("locked punishment has no future deadline: %v", until)
	}
	if until := c.PunishmentLockedUntil(PunishmentPirate); !until.IsZero() {
		t.Errorf("plain punishment reported a lock deadline: %v", until)
	}

	locked := c.RemoveAllUnlockedPunishments()
	if len(locked) != 1 || locked[0] != PunishmentUwu {
		t.Fatalf("RemoveAllUnlockedPunishments returned %v, want [uwu]", locked)
	}
	if c.HasPunishment(PunishmentPirate) {
		t.Error("unlocked punishment was not removed")
	}
	if !c.HasPunishment(PunishmentUwu) {
		t.Error("locked punishment was removed")
	}
}

func TestReapplyingSameTypePreservesLiveLock(t *testing.T) {
	punishmentLockTestSetup(t)
	c, _ := punishmentLockClient(t, 101, "Reapply")
	c.AddPunishmentLocked(PunishmentUwu, time.Hour, time.Hour, "Deathroll: rolled a 1")
	c.AddPunishment(PunishmentUwu, time.Minute, "re-applied")

	if until := c.PunishmentLockedUntil(PunishmentUwu); !until.After(time.Now()) {
		t.Errorf("a same-type re-application shed the live lock: %v", until)
	}
}

func TestExpiredLockCanBeRemoved(t *testing.T) {
	punishmentLockTestSetup(t)
	c, _ := punishmentLockClient(t, 102, "Expired")
	c.AddPunishmentLocked(PunishmentUwu, time.Hour, -time.Second, "expired lock")

	if locked := c.RemoveAllUnlockedPunishments(); len(locked) != 0 {
		t.Errorf("expired lock was kept: %v", locked)
	}
	if c.HasPunishment(PunishmentUwu) {
		t.Error("punishment with an expired lock was not removed")
	}
}

func TestMasoCannotRerollLockedEffect(t *testing.T) {
	punishmentLockTestSetup(t)
	c, conn := punishmentLockClient(t, 103, "Maso")
	c.AddPunishmentLocked(PunishmentUwu, time.Hour, time.Hour, "Deathroll: rolled a 1")
	c.mu.Lock()
	c.masoPunishment = PunishmentUwu
	c.mu.Unlock()

	cmdMaso(c, nil, "")

	if !c.HasPunishment(PunishmentUwu) {
		t.Error("/maso removed a locked Deathroll effect")
	}
	if !strings.Contains(conn.String(), "locked") {
		t.Errorf("/maso did not explain the lock; got %q", conn.String())
	}
}

func TestUnpunishSelfRespectsLockButOtherModCanClear(t *testing.T) {
	defer setupShadowDisconnectTestDB(t)()
	punishmentLockTestSetup(t)

	loser, loserConn := punishmentLockClient(t, 200, "Loser")
	mod, _ := punishmentLockClient(t, 201, "Mod")
	loser.perms = permissions.PermissionField["MUTE"]
	mod.perms = permissions.PermissionField["MUTE"]
	loser.AddPunishmentLocked(PunishmentUwu, time.Hour, time.Hour, "Deathroll: rolled a 1")

	// The loser cannot self-remove the locked effect.
	cmdUnpunish(loser, []string{fmt.Sprint(loser.Uid())}, "usage")
	if !loser.HasPunishment(PunishmentUwu) {
		t.Error("the loser self-removed their locked Deathroll effect")
	}
	if !strings.Contains(loserConn.String(), "locked") {
		t.Errorf("self-unpunish did not explain the lock; got %q", loserConn.String())
	}

	// Another moderator targeting the loser can still clear it.
	cmdUnpunish(mod, []string{fmt.Sprint(loser.Uid())}, "usage")
	if loser.HasPunishment(PunishmentUwu) {
		t.Error("another moderator could not clear the locked effect")
	}
}

func TestPotionOffCannotFlushLockedEffect(t *testing.T) {
	punishmentLockTestSetup(t)
	c, _ := punishmentLockClient(t, 202, "Potion")
	c.AddPunishmentLocked(PunishmentDrunk, time.Hour, time.Hour, "Deathroll: rolled a 1")

	clearAllPotions(c)

	if !c.HasPunishment(PunishmentDrunk) {
		t.Error("/potion off flushed a locked Deathroll effect")
	}
}
