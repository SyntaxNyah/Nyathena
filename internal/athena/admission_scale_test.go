package athena

import (
	"testing"
	"time"
)

// Count limits should use the configured (strict) value only while the raid
// guard says the server is under attack. Normal operation may relax them by the
// configured percentage.
func TestAdmissionLimitRelaxesOnlyOutsideAttack(t *testing.T) {
	withRaidConfig(t)
	if config != nil {
		config.AdmissionNormalScale = 200
	}
	raidSystem.corr.attackUntil.Store(0)
	t.Cleanup(func() { raidSystem.corr.attackUntil.Store(0) })

	if got := admissionLimit(5); got != 10 {
		t.Fatalf("normal admission limit = %d, want 10", got)
	}

	raidSystem.corr.attackUntil.Store(time.Now().Add(time.Minute).UnixNano())
	if got := admissionLimit(5); got != 5 {
		t.Fatalf("under-attack admission limit = %d, want 5", got)
	}
}

func TestAdmissionLimitNeverTightens(t *testing.T) {
	withRaidConfig(t)
	if config != nil {
		config.AdmissionNormalScale = 50 // invalid; must be ignored
	}
	raidSystem.corr.attackUntil.Store(0)
	t.Cleanup(func() { raidSystem.corr.attackUntil.Store(0) })

	if got := admissionLimit(7); got != 7 {
		t.Fatalf("scale below 100 changed the limit: got %d, want 7", got)
	}
	if got := admissionLimit(0); got != 0 {
		t.Fatalf("disabled limit changed: got %d, want 0", got)
	}
}

func TestAdmissionCooldownUsesNormalOnlyOutsideAttack(t *testing.T) {
	withRaidConfig(t)
	raidSystem.corr.attackUntil.Store(0)
	t.Cleanup(func() { raidSystem.corr.attackUntil.Store(0) })

	if got := admissionCooldown(350, 30); got != 30 {
		t.Fatalf("normal cooldown = %d, want 30", got)
	}
	if got := admissionCooldown(350, 0); got != 350 {
		t.Fatalf("normal cooldown with 0 override = %d, want configured 350", got)
	}
	if got := admissionCooldown(20, 30); got != 20 {
		t.Fatalf("normal cooldown larger than configured = %d, want configured 20", got)
	}

	raidSystem.corr.attackUntil.Store(time.Now().Add(time.Minute).UnixNano())
	if got := admissionCooldown(350, 30); got != 350 {
		t.Fatalf("under-attack cooldown = %d, want 350", got)
	}
}
