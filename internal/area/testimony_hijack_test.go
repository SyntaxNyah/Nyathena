package area

import (
	"strings"
	"testing"
)

// tstStatement builds a stored testimony statement (a 30-field server->client
// MS body, exactly what the IC handler records) with the slots playback cares
// about filled in.
func tstStatement(charID, character, showname, message string) string {
	fields := make([]string, 30)
	fields[0] = "1"      // desk modifier
	fields[2] = character
	fields[3] = "normal" // emote
	fields[4] = message
	fields[5] = "wit"
	fields[8] = charID
	fields[14] = "1" // text colour
	fields[15] = showname
	return strings.Join(fields, "#")
}

// TestTestimonyPlaybackBelongsToNobody pins the core of the hijack fix: the
// recorded statement keeps the speaker's char id, but the playback form hands
// the statement to nobody (char id -1). A client that receives an MS carrying
// its own char id treats it as the echo of its own message and clears the
// player's input, which is exactly what happened to the witness being replayed.
func TestTestimonyPlaybackBelongsToNobody(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	rec := tstStatement("5", "phoenix", "Nick", "The butler did it.")
	a.TstAppend(rec)

	if got := strings.Split(a.CurrentTstStatement(), "#")[8]; got != "5" {
		t.Fatalf("recorded statement char id = %q, want it kept as %q", got, "5")
	}

	playback := strings.Split(a.CurrentTstPlaybackStatement(), "#")
	if playback[8] != "-1" {
		t.Errorf("playback char id = %q, want %q (belongs to nobody)", playback[8], "-1")
	}
	// Everything that makes the statement render as the right character must
	// survive the rewrite.
	if playback[2] != "phoenix" || playback[3] != "normal" || playback[4] != "The butler did it." {
		t.Errorf("playback lost statement content: character=%q emote=%q message=%q",
			playback[2], playback[3], playback[4])
	}
	if playback[15] != "Nick" {
		t.Errorf("playback showname = %q, want the recorded showname %q", playback[15], "Nick")
	}
	if len(playback) != 30 {
		t.Errorf("playback has %d fields, want 30", len(playback))
	}
}

// TestTestimonyPlaybackFillsShownameFromCharacter covers the second half of the
// Akashi behaviour: with no char id the clients show the showname field, so a
// statement recorded without one must fall back to the character folder rather
// than rendering a blank name box.
func TestTestimonyPlaybackFillsShownameFromCharacter(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.TstAppend(tstStatement("3", "edgeworth", "", "Objection!"))

	playback := strings.Split(a.CurrentTstPlaybackStatement(), "#")
	if playback[15] != "edgeworth" {
		t.Errorf("playback showname = %q, want the character folder %q", playback[15], "edgeworth")
	}
	// The stored statement must not be mutated by reading its playback form.
	if got := strings.Split(a.CurrentTstStatement(), "#")[15]; got != "" {
		t.Errorf("stored showname = %q, want it left empty", got)
	}
}

// TestTestimonyPlaybackLeavesMalformedStatementAlone guards the defensive path:
// a statement too short to hold the showname slot is replayed verbatim instead
// of panicking on an out-of-range index.
func TestTestimonyPlaybackLeavesMalformedStatementAlone(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.TstAppend("garbage#0#1")

	if got := a.CurrentTstPlaybackStatement(); got != "garbage#0#1" {
		t.Errorf("playback of a malformed statement = %q, want it unchanged", got)
	}
}

// TestTestimonyReplaceAllKeepsStatementsVerbatim pins the loader's contract:
// TstReplaceAll stores statements as given (no text-colour rewrite, unlike
// TstAppend) and rewinds to the title, so a loaded testimony plays back exactly
// as it was saved.
func TestTestimonyReplaceAllKeepsStatementsVerbatim(t *testing.T) {
	a := NewArea(AreaData{}, 50, 0, EviAny)
	a.TstAppend(tstStatement("9", "phoenix", "Nick", "old title"))

	loaded := []string{
		tstStatement("9", "phoenix", "Nick", "Loaded title"),
		tstStatement("4", "maya", "Maya", "Loaded statement"),
	}
	a.TstReplaceAll(loaded)

	got := a.TstStatements()
	if len(got) != len(loaded) {
		t.Fatalf("TstStatements() returned %d statements, want %d", len(got), len(loaded))
	}
	for i := range loaded {
		if got[i] != loaded[i] {
			t.Errorf("statement %d = %q, want %q", i, got[i], loaded[i])
		}
	}
	if a.CurrentTstIndex() != 0 {
		t.Errorf("index after TstReplaceAll = %d, want 0 (the title)", a.CurrentTstIndex())
	}

	// TstStatements hands out a copy: mutating it must not touch the recorder.
	got[0] = "mutated"
	if a.TstStatements()[0] == "mutated" {
		t.Error("TstStatements() exposed the recorder's own slice")
	}
}
