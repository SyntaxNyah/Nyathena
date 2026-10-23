package athena

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// withTestimonyConfigDir points settings.ConfigPath at a throwaway directory so
// save/load tests never touch the real config tree.
func withTestimonyConfigDir(t *testing.T) string {
	t.Helper()
	orig := settings.ConfigPath
	dir := t.TempDir()
	settings.ConfigPath = dir
	t.Cleanup(func() { settings.ConfigPath = orig })
	return dir
}

// fieldOf returns slot i of an MS body for readable assertions.
func fieldOf(t *testing.T, body string, i int) string {
	t.Helper()
	fields := strings.Split(body, "#")
	if i >= len(fields) {
		t.Fatalf("statement has %d fields, wanted index %d: %q", len(fields), i, body)
	}
	return fields[i]
}

// TestTestimonySaveLoadRoundTrip pins the on-disk contract: a testimony survives
// save -> load unchanged, except that the char id is cleared (a saved id names
// whoever holds that player slot today, not the witness who recorded it).
func TestTestimonySaveLoadRoundTrip(t *testing.T) {
	withTestimonyConfigDir(t)

	statements := []string{
		recordedStatement("4", "phoenix", "Nick", "~~<num>-- The <num> case --"),
		recordedStatement("4", "phoenix", "Nick", "He left at 10<percent> past."),
	}
	if err := saveTestimonyFile("trial-one", statements); err != nil {
		t.Fatalf("saveTestimonyFile: %v", err)
	}
	if !testimonyFileExists("trial-one") {
		t.Fatal("testimonyFileExists = false after saving")
	}

	loaded, err := loadTestimonyFile("trial-one", 11)
	if err != nil {
		t.Fatalf("loadTestimonyFile: %v", err)
	}
	if len(loaded) != len(statements) {
		t.Fatalf("loaded %d statements, want %d", len(loaded), len(statements))
	}
	for i := range statements {
		if got, want := fieldOf(t, loaded[i], 4), fieldOf(t, statements[i], 4); got != want {
			t.Errorf("statement %d message = %q, want %q", i, got, want)
		}
		if got := fieldOf(t, loaded[i], 2); got != "phoenix" {
			t.Errorf("statement %d character = %q, want %q", i, got, "phoenix")
		}
		if got := fieldOf(t, loaded[i], 8); got != "-1" {
			t.Errorf("statement %d char id = %q, want %q (a saved slot belongs to nobody)", i, got, "-1")
		}
	}
	// Both lines must still be exactly 30 fields: an unescaped separator would
	// have split one statement into two.
	if n := len(strings.Split(loaded[0], "#")); n != 30 {
		t.Errorf("loaded statement has %d fields, want 30", n)
	}
}

// TestTestimonySaveDoesNotDoubleEscape guards the escaping scheme: statements
// are already stored wire-escaped, so saving must leave <num>/<percent>/<dollar>
// exactly as they are.
func TestTestimonySaveDoesNotDoubleEscape(t *testing.T) {
	dir := withTestimonyConfigDir(t)
	stmt := recordedStatement("4", "phoenix", "Nick", "50<percent> off <num>1 <dollar>5")
	if err := saveTestimonyFile("escapes", []string{stmt}); err != nil {
		t.Fatalf("saveTestimonyFile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, testimonyDirName, "escapes"+testimonyFileExt))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if line := strings.TrimSuffix(string(data), "\n"); line != stmt {
		t.Errorf("saved line = %q, want it identical to the stored statement %q", line, stmt)
	}
}

// TestTestimonyLoadNormalisesHandEditedFields: a hand-edited file may contain a
// raw '&' (never escaped), which must not be able to break the field layout on
// load -- it is escaped back into <and> exactly as the wire form expects.
func TestTestimonyLoadNormalisesHandEditedFields(t *testing.T) {
	dir := withTestimonyConfigDir(t)
	if err := os.MkdirAll(filepath.Join(dir, testimonyDirName), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	line := recordedStatement("9", "maya", "Maya", "A&B")
	path := filepath.Join(dir, testimonyDirName, "handwritten"+testimonyFileExt)
	if err := os.WriteFile(path, []byte(line+"\n"), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	loaded, err := loadTestimonyFile("handwritten", 11)
	if err != nil {
		t.Fatalf("loadTestimonyFile: %v", err)
	}
	if got := fieldOf(t, loaded[0], 4); got != "A<and>B" {
		t.Errorf("loaded message = %q, want %q", got, "A<and>B")
	}
	if n := len(strings.Split(loaded[0], "#")); n != 30 {
		t.Errorf("loaded statement has %d fields, want 30", n)
	}
}

// TestSanitizeTestimonyName pins the filename alphabet: nothing that could
// escape the testimony directory may survive.
func TestSanitizeTestimonyName(t *testing.T) {
	valid := map[string]string{
		"trial-one":  "trial-one",
		"  Trial_1 ": "trial_1",
		"CASE42":     "case42",
	}
	for in, want := range valid {
		got, ok := sanitizeTestimonyName(in)
		if !ok || got != want {
			t.Errorf("sanitizeTestimonyName(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
	invalid := []string{
		"", "   ", "..", "../evil", "a/b", `a\b`, "C:evil", "name with space",
		"dot.name", strings.Repeat("a", testimonyNameMaxLen+1),
	}
	for _, in := range invalid {
		if got, ok := sanitizeTestimonyName(in); ok {
			t.Errorf("sanitizeTestimonyName(%q) = (%q, true), want it rejected", in, got)
		}
	}
}

// TestTestimonyLoadErrors makes sure every bad file is refused with a message
// the player can act on, rather than being partially loaded.
func TestTestimonyLoadErrors(t *testing.T) {
	dir := withTestimonyConfigDir(t)
	if err := os.MkdirAll(filepath.Join(dir, testimonyDirName), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, testimonyDirName, name+testimonyFileExt), []byte(body), 0644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	if _, err := loadTestimonyFile("missing", 11); err != errTestimonyNotFound {
		t.Errorf("load of a missing testimony = %v, want errTestimonyNotFound", err)
	}

	write("empty", "\n\n")
	if _, err := loadTestimonyFile("empty", 11); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("load of an empty testimony = %v, want an \"empty\" error", err)
	}

	write("garbage", recordedStatement("1", "a", "", "one")+"\nnot-a-statement\n")
	if _, err := loadTestimonyFile("garbage", 11); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("load of a malformed testimony = %v, want a \"line 2\" error", err)
	}

	write("toobig", recordedStatement("1", "a", "", "one")+"\n"+recordedStatement("1", "a", "", "two")+"\n")
	if _, err := loadTestimonyFile("toobig", 1); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("load past the statement cap = %v, want a \"too large\" error", err)
	}

	write("huge", strings.Repeat("x", testimonyMaxFileBytes+1))
	if _, err := loadTestimonyFile("huge", 11); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("load of an oversized file = %v, want a \"too large\" error", err)
	}
}

// TestListSavedTestimonies: only loadable names are offered, sorted.
func TestListSavedTestimonies(t *testing.T) {
	dir := withTestimonyConfigDir(t)
	if err := saveTestimonyFile("beta", []string{recordedStatement("1", "a", "", "t")}); err != nil {
		t.Fatalf("save beta: %v", err)
	}
	if err := saveTestimonyFile("alpha", []string{recordedStatement("1", "a", "", "t")}); err != nil {
		t.Fatalf("save alpha: %v", err)
	}
	// Neither of these can be loaded by name, so neither may be advertised.
	if err := os.WriteFile(filepath.Join(dir, testimonyDirName, "Bad Name.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing unusable name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, testimonyDirName, "notes.md"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing notes.md: %v", err)
	}

	got := listSavedTestimonies()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("listSavedTestimonies() = %v, want [alpha beta]", got)
	}
}

// TestTestimonyFileCommandsRegistered pins the two new commands to the
// testimony help category and the CM tier (grantable to a player through
// grantcmd, like the rest of the recorder).
func TestTestimonyFileCommandsRegistered(t *testing.T) {
	initCommands()
	for _, name := range []string{"savetestimony", "loadtestimony"} {
		cmd, ok := Commands[name]
		if !ok {
			t.Fatalf("/%s is not registered", name)
		}
		if cmd.reqPerms != permissions.PermissionField["CM"] {
			t.Errorf("/%s reqPerms = %v, want CM", name, cmd.reqPerms)
		}
		if cmd.category != "testimony" {
			t.Errorf("/%s category = %q, want \"testimony\"", name, cmd.category)
		}
	}
}
