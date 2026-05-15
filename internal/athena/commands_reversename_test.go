/* Athena - A server for Attorney Online 2 written in Go
   Nyathena fork additions: tests for /reversename and /unreversename. */

package athena

import "testing"

// TestReverseShownameRoundTrip checks that reversing then restoring a plain
// showname returns the exact original.
func TestReverseShownameRoundTrip(t *testing.T) {
	c := &Client{showname: "Phoenix"}

	got, ok := c.ReverseShowname()
	if !ok {
		t.Fatal("ReverseShowname returned false for a non-empty showname")
	}
	if got != "xineohP" {
		t.Errorf("reversed showname = %q, want %q", got, "xineohP")
	}
	if c.EffectiveShowname() != "xineohP" {
		t.Errorf("EffectiveShowname after reverse = %q, want %q", c.EffectiveShowname(), "xineohP")
	}

	restored, ok := c.RestoreShowname()
	if !ok {
		t.Fatal("RestoreShowname returned false after a reverse")
	}
	if restored != "Phoenix" {
		t.Errorf("restored showname = %q, want %q", restored, "Phoenix")
	}
	if c.EffectiveShowname() != "Phoenix" {
		t.Errorf("EffectiveShowname after restore = %q, want %q", c.EffectiveShowname(), "Phoenix")
	}
}

// TestReverseShownameOnForcedName verifies that a reverse stacked on top of a
// /forcename restores back to the forced name, not the underlying showname.
func TestReverseShownameOnForcedName(t *testing.T) {
	c := &Client{showname: "real", forcedShowname: "Bob"}

	got, ok := c.ReverseShowname()
	if !ok || got != "boB" {
		t.Fatalf("ReverseShowname on forced name = (%q,%v), want (\"boB\",true)", got, ok)
	}

	restored, ok := c.RestoreShowname()
	if !ok || restored != "Bob" {
		t.Fatalf("RestoreShowname = (%q,%v), want (\"Bob\",true)", restored, ok)
	}
	if c.EffectiveShowname() != "Bob" {
		t.Errorf("EffectiveShowname after restore = %q, want %q", c.EffectiveShowname(), "Bob")
	}
}

// TestReverseShownameDoubleReverseRefused ensures a second /reversename on an
// already-reversed client is rejected rather than flipping back to the original.
func TestReverseShownameDoubleReverseRefused(t *testing.T) {
	c := &Client{showname: "Maya"}

	if _, ok := c.ReverseShowname(); !ok {
		t.Fatal("first ReverseShowname should succeed")
	}
	if got, ok := c.ReverseShowname(); ok {
		t.Errorf("second ReverseShowname should be refused, got (%q,true)", got)
	}
	if c.EffectiveShowname() != "ayaM" {
		t.Errorf("EffectiveShowname = %q, want %q (no double reverse)", c.EffectiveShowname(), "ayaM")
	}
}

// TestRestoreShownameWithoutReverseRefused ensures /unreversename on a client
// whose name was never reversed is a no-op.
func TestRestoreShownameWithoutReverseRefused(t *testing.T) {
	c := &Client{showname: "Edgeworth"}

	if got, ok := c.RestoreShowname(); ok {
		t.Errorf("RestoreShowname on a non-reversed client should be refused, got (%q,true)", got)
	}
}

// TestReverseShownameEmptyRefused ensures a player who has never set a showname
// is skipped (there is nothing to flip).
func TestReverseShownameEmptyRefused(t *testing.T) {
	c := &Client{}

	if got, ok := c.ReverseShowname(); ok {
		t.Errorf("ReverseShowname with empty showname should be refused, got (%q,true)", got)
	}
}

// TestReverseShownamePreservesEncoding ensures AO2 escape sequences survive the
// flip — the stored showname must remain validly encoded.
func TestReverseShownamePreservesEncoding(t *testing.T) {
	// "a#b" is stored AO2-encoded as "a<num>b".
	c := &Client{showname: encode("a#b")}

	got, ok := c.ReverseShowname()
	if !ok || got != "b#a" {
		t.Fatalf("ReverseShowname = (%q,%v), want (\"b#a\",true)", got, ok)
	}
	if c.EffectiveShowname() != encode("b#a") {
		t.Errorf("stored forced showname = %q, want %q (AO2-encoded)", c.EffectiveShowname(), encode("b#a"))
	}

	restored, ok := c.RestoreShowname()
	if !ok || restored != "a#b" {
		t.Fatalf("RestoreShowname = (%q,%v), want (\"a#b\",true)", restored, ok)
	}
	if c.EffectiveShowname() != encode("a#b") {
		t.Errorf("EffectiveShowname after restore = %q, want %q", c.EffectiveShowname(), encode("a#b"))
	}
}

// TestReverseShownameUnicode ensures multi-byte runes are reversed as runes,
// not bytes, so accented characters survive intact.
func TestReverseShownameUnicode(t *testing.T) {
	c := &Client{showname: "Amélie"}

	got, ok := c.ReverseShowname()
	if !ok || got != "eilémA" {
		t.Fatalf("ReverseShowname = (%q,%v), want (\"eilémA\",true)", got, ok)
	}
}
