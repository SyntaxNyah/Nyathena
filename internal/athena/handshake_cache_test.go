package athena

import (
	"strings"
	"testing"
)

// The SC packet is cached once per character-list version; a reload must
// rebuild both wire forms rather than serving the old roster.
func TestSCPacketCacheRebuildsOnCharacterChange(t *testing.T) {
	original := append([]string(nil), getCharacters()...)
	t.Cleanup(func() { setCharacters(original) })

	setCharacters([]string{"Test Character Alpha", "Test Character Beta"})
	fanta := getSCPacketBytes(false)
	if len(fanta) == 0 || !strings.Contains(string(fanta), "Test Character Alpha") {
		t.Fatalf("Fanta SC cache missing first character: %q", string(fanta))
	}
	jsonMode := getSCPacketBytes(true)
	if len(jsonMode) == 0 || !strings.Contains(string(jsonMode), "Test Character Alpha") {
		t.Fatalf("JSON SC cache missing first character: %q", string(jsonMode))
	}

	setCharacters([]string{"Test Character Gamma"})
	fanta = getSCPacketBytes(false)
	if strings.Contains(string(fanta), "Test Character Alpha") {
		t.Fatal("Fanta SC cache still contains the old roster after a reload")
	}
	if !strings.Contains(string(fanta), "Test Character Gamma") {
		t.Fatalf("Fanta SC cache missing the new roster: %q", string(fanta))
	}
}
