package athena

import "testing"

// CurrentCharacter must never panic when the roster is empty or a character ID
// is out of range. This matters because it can be called from packet handlers
// and background paths while a character list is being reloaded.
func TestCurrentCharacterOutOfRange(t *testing.T) {
	orig := getCharacters()
	setCharacters(nil)
	t.Cleanup(func() { setCharacters(orig) })

	c := &Client{char: 1}
	if got := c.CurrentCharacter(); got != "Spectator" {
		t.Fatalf("CurrentCharacter with empty roster = %q, want Spectator", got)
	}
	c.char = -1
	if got := c.CurrentCharacter(); got != "Spectator" {
		t.Fatalf("CurrentCharacter for spectator = %q, want Spectator", got)
	}
}
