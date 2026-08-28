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
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// withSecretSeed points the package-level config at a fresh default config
// with SecretSeed set, for the duration of a test. config is a *settings.Config
// (see server.go) and may be nil in a test binary that hasn't run any server
// startup code, so this can't just dereference-and-restore -- it swaps the
// whole pointer, mirroring TestPacketFloodAutobanDefaultTrue's pattern.
func withSecretSeed(t *testing.T, seed string) {
	t.Helper()
	oldConfig := config
	config = settings.DefaultConfig()
	config.SecretSeed = seed
	t.Cleanup(func() { config = oldConfig })
}

// TestLockdownPasskeyDisabledWithoutSecret verifies that the passkey system is
// fully inert when secretseed is unset: nothing verifies successfully, no
// matter what's passed in.
func TestLockdownPasskeyDisabledWithoutSecret(t *testing.T) {
	withSecretSeed(t, "")

	if lockdownPasskeyEnabled() {
		t.Errorf("Expected lockdownPasskeyEnabled to be false when SecretSeed is empty")
	}
	if _, ok := lockdownPasskeyVerify("someIpid:deadbeef"); ok {
		t.Errorf("Expected lockdownPasskeyVerify to fail when the passkey system is disabled")
	}
}

// TestLockdownPasskeyRoundTrip verifies that a passkey generated for an IPID
// verifies back to that exact IPID.
func TestLockdownPasskeyRoundTrip(t *testing.T) {
	withSecretSeed(t, "test-secret-1")

	ipid := "aBcD1234efGH=="
	passkey := lockdownPasskeyFor(ipid)

	gotIpid, ok := lockdownPasskeyVerify(passkey)
	if !ok {
		t.Fatalf("Expected a freshly-generated passkey to verify successfully")
	}
	if gotIpid != ipid {
		t.Errorf("Expected verify to recover IPID %q, got %q", ipid, gotIpid)
	}
}

// TestLockdownPasskeyRejectsTamperedSignature verifies that flipping a
// character in the signature portion of a valid passkey invalidates it.
func TestLockdownPasskeyRejectsTamperedSignature(t *testing.T) {
	withSecretSeed(t, "test-secret-2")

	passkey := lockdownPasskeyFor("someIpidValue")
	idx := strings.Index(passkey, ":")
	sig := []byte(passkey[idx+1:])
	// Flip the first hex digit to something else.
	if sig[0] == '0' {
		sig[0] = '1'
	} else {
		sig[0] = '0'
	}
	tampered := passkey[:idx+1] + string(sig)

	if _, ok := lockdownPasskeyVerify(tampered); ok {
		t.Errorf("Expected a tampered signature to fail verification")
	}
}

// TestLockdownPasskeyRejectsForgedIPIDClaim verifies that splicing a
// different IPID onto someone else's valid signature fails -- an attacker
// who knows one IPID's passkey cannot reuse its signature to claim to be a
// different IPID.
func TestLockdownPasskeyRejectsForgedIPIDClaim(t *testing.T) {
	withSecretSeed(t, "test-secret-3")

	victimPasskey := lockdownPasskeyFor("victimIpid")
	idx := strings.Index(victimPasskey, ":")
	sig := victimPasskey[idx+1:]
	forged := "attackerIpid:" + sig

	if _, ok := lockdownPasskeyVerify(forged); ok {
		t.Errorf("Expected a forged IPID claim riding on someone else's signature to fail verification")
	}
}

// TestLockdownPasskeyRejectsMalformedInput verifies that structurally invalid
// passkeys are rejected rather than panicking or false-accepting.
func TestLockdownPasskeyRejectsMalformedInput(t *testing.T) {
	withSecretSeed(t, "test-secret-4")

	cases := []string{
		"",
		"nocolonhere",
		":leadingcolon",
		"trailingcolon:",
		"ipid:not-valid-hex!!",
	}
	for _, c := range cases {
		if _, ok := lockdownPasskeyVerify(c); ok {
			t.Errorf("Expected malformed passkey %q to fail verification", c)
		}
	}
}

// TestLockdownPasskeyDifferentSecretsProduceIncompatibleTokens verifies that
// changing secretseed invalidates every previously-issued passkey, as documented.
func TestLockdownPasskeyDifferentSecretsProduceIncompatibleTokens(t *testing.T) {
	withSecretSeed(t, "secret-A")
	ipid := "sameIpidBothSecrets"
	passkeyA := lockdownPasskeyFor(ipid)

	withSecretSeed(t, "secret-B")
	passkeyB := lockdownPasskeyFor(ipid)

	if passkeyA == passkeyB {
		t.Errorf("Expected different secrets to produce different passkeys for the same IPID")
	}
	if _, ok := lockdownPasskeyVerify(passkeyA); ok {
		t.Errorf("Expected a passkey issued under secret-A to fail verification under secret-B")
	}
}

// TestLockdownPasskeyMessagePlaceholder verifies {id} substitution in the
// configured message template, including the fallback when it's omitted.
func TestLockdownPasskeyMessagePlaceholder(t *testing.T) {
	withSecretSeed(t, "test-secret-5")
	oldTmpl := config.LockdownString
	t.Cleanup(func() { config.LockdownString = oldTmpl })

	ipid := "templateTestIpid"
	id := lockdownPasskeyFor(ipid)

	config.LockdownString = "Not a ban -- code: {id}"
	msg := lockdownPasskeyMessage(ipid)
	if !strings.Contains(msg, id) {
		t.Errorf("Expected message to contain the passkey %q, got %q", id, msg)
	}
	if strings.Contains(msg, "{id}") {
		t.Errorf("Expected {id} placeholder to be substituted, got %q", msg)
	}

	config.LockdownString = "No placeholder here"
	msg = lockdownPasskeyMessage(ipid)
	if !strings.Contains(msg, id) {
		t.Errorf("Expected the passkey to be appended when the template omits {id}, got %q", msg)
	}
}
