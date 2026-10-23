package athena

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/settings"
)

// Saved testimonies: the on-disk half of /savetestimony and /loadtestimony.
//
// A file holds one statement per line (title first), each line the statement's
// MS fields wire-escaped and joined with '#', exactly Akashi's
// Statement::toSavedLine/fromSavedLine. Escaping is what keeps a message that
// contains '#', '%', '$' or '&' from being split into extra fields when the
// line is read back; the recorder stores its statements in that same escaped
// form, so escaping on the way out and unescaping-then-re-escaping on the way
// in is stable across save -> load -> save.
const (
	// testimonyDirName is the directory, relative to settings.ConfigPath, that
	// saved testimonies live in. Config-adjacent like custom_commands/.
	testimonyDirName = "testimony"
	// testimonyFileExt is the extension of a saved testimony.
	testimonyFileExt = ".txt"
	// testimonyNameMaxLen caps a testimony name so it stays a sane filename.
	testimonyNameMaxLen = 64
	// testimonyMaxFileBytes caps how much of a saved file is read. The recorder
	// itself caps statements (max_testimony), this only stops a huge file from
	// being pulled into memory before that cap can be checked.
	testimonyMaxFileBytes = 256 << 10
	// testimonyMinFields is the smallest MS body that is a usable statement:
	// it must reach the showname slot (15) that playback rewrites.
	testimonyMinFields = 16
)

// errTestimonyNotFound reports a name with no saved file behind it.
var errTestimonyNotFound = errors.New("testimony name not found")

// testimonyNameRegex is the whole allowed filename alphabet: lower-case
// letters, digits, dashes and underscores. It cannot express "." or "..", a
// separator or a drive letter, so a name can never leave the testimony
// directory.
var testimonyNameRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

// testimonyDirPath returns the absolute directory holding saved testimonies.
func testimonyDirPath() string {
	return filepath.Join(settings.ConfigPath, testimonyDirName)
}

// sanitizeTestimonyName lower-cases and validates a user-supplied testimony
// name. The bool is false for anything that is not a plain name, which the
// caller reports back as an invalid name rather than touching the filesystem.
func sanitizeTestimonyName(raw string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if len(name) == 0 || len(name) > testimonyNameMaxLen || !testimonyNameRegex.MatchString(name) {
		return "", false
	}
	return name, true
}

// testimonyFilePath returns the path of a saved testimony. name must already
// have come out of sanitizeTestimonyName.
func testimonyFilePath(name string) string {
	return filepath.Join(testimonyDirPath(), name+testimonyFileExt)
}

// testimonyFileExists reports whether a testimony is already saved under name.
func testimonyFileExists(name string) bool {
	_, err := os.Stat(testimonyFilePath(name))
	return err == nil
}

// saveTestimonyFile writes statements to the testimony directory, one escaped
// statement per line. The caller is responsible for the name (sanitized) and
// for refusing to overwrite an existing testimony.
func saveTestimonyFile(name string, statements []string) error {
	dir := testimonyDirPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create the testimony directory: %w", err)
	}
	var b strings.Builder
	for _, s := range statements {
		fields := strings.Split(s, "#")
		for i, f := range fields {
			fields[i] = encode(f)
		}
		b.WriteString(strings.Join(fields, "#"))
		b.WriteString("\n")
	}
	if err := os.WriteFile(testimonyFilePath(name), []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("could not write the testimony file: %w", err)
	}
	return nil
}

// loadTestimonyFile reads a saved testimony and returns its statements in the
// form the recorder stores: fields re-escaped, char id cleared to -1 (the saved
// id belonged to a player slot in a long-gone session, and playback must never
// name whoever holds that slot now).
//
// maxStatements is a hard cap on the number of statements accepted; a file that
// exceeds it, or that holds a line which is not a statement, is rejected whole
// rather than partially loaded.
func loadTestimonyFile(name string, maxStatements int) ([]string, error) {
	path := testimonyFilePath(name)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errTestimonyNotFound
		}
		return nil, fmt.Errorf("could not read the testimony file: %w", err)
	}
	if info.Size() > testimonyMaxFileBytes {
		return nil, errors.New("testimony is too large to load")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read the testimony file: %w", err)
	}

	var statements []string
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "#")
		if len(fields) < testimonyMinFields {
			return nil, fmt.Errorf("line %d is not a statement", i+1)
		}
		for j, f := range fields {
			fields[j] = encode(decode(f))
		}
		fields[8] = "-1"
		if maxStatements > 0 && len(statements) >= maxStatements {
			return nil, errors.New("testimony is too large to load")
		}
		statements = append(statements, strings.Join(fields, "#"))
	}
	if len(statements) == 0 {
		return nil, errors.New("the testimony file is empty")
	}
	return statements, nil
}

// listSavedTestimonies returns the sorted names of every saved testimony. Files
// whose names would not pass sanitizeTestimonyName are left out: they cannot be
// loaded by name anyway, so offering them would only produce dead entries.
func listSavedTestimonies() []string {
	entries, err := os.ReadDir(testimonyDirPath())
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), testimonyFileExt) {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if _, ok := sanitizeTestimonyName(name); !ok {
			continue
		}
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	return names
}
