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

package settings

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/MangosArentLiterature/Athena/internal/logger"
)

// utf8BOM is U+FEFF encoded as UTF-8. BurntSushi's parser rejects a file that
// starts with one -- "expected '.' or '=', but got '\ufeff' instead" -- which is
// what an operator sees after saving config.toml from an editor that adds a
// byte-order mark, or from a tool that wrote it as UTF-16.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// decodeTOMLFile parses a TOML file exactly like toml.DecodeFile, after first
// repairing the byte-level damage that makes an otherwise valid file fail to
// load: a UTF-8 byte-order mark, UTF-16 text (PowerShell 5.1's ">" and
// "Out-File" write UTF-16 by default), stray U+FEFF markers and NUL bytes.
//
// The repair is written back to disk so the next boot is clean, but parsing
// always uses the cleaned copy in memory: a read-only config directory still
// starts the server instead of locking the operator out of it.
func decodeTOMLFile(path string, v any) (toml.MetaData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return toml.MetaData{}, err
	}

	clean, fixes := normalizeTOMLBytes(raw)
	if len(fixes) > 0 {
		reason := strings.Join(fixes, ", ")
		if err := writeFileAtomic(path, clean); err != nil {
			logger.LogErrorf("settings: %v could not be parsed as-is (%v) and could not be rewritten: %v; continuing with the repaired copy in memory", path, reason, err)
		} else {
			logger.LogWarningf("settings: %v could not be parsed as-is; repaired it in place (%v). Save it as UTF-8 without a BOM to avoid this", path, reason)
		}
	}

	return toml.Decode(string(clean), v)
}

// normalizeTOMLBytes returns raw as valid UTF-8 TOML where that is mechanically
// possible, together with a human-readable list of what had to be changed. A
// file that needs no repair is returned untouched with no fixes, so callers can
// skip rewriting it.
func normalizeTOMLBytes(raw []byte) ([]byte, []string) {
	original := raw
	clean := raw
	var fixes []string

	// UTF-16 first: its bytes are full of NULs, so every later step would only
	// mangle it further.
	switch {
	case len(clean) >= 2 && clean[0] == 0xFF && clean[1] == 0xFE:
		clean = decodeUTF16(clean[2:], false)
		fixes = append(fixes, "decoded UTF-16LE to UTF-8")
	case len(clean) >= 2 && clean[0] == 0xFE && clean[1] == 0xFF:
		clean = decodeUTF16(clean[2:], true)
		fixes = append(fixes, "decoded UTF-16BE to UTF-8")
	case bytes.IndexByte(clean, 0) >= 0:
		if bigEndian, ok := sniffUTF16Endian(clean); ok {
			clean = decodeUTF16(clean, bigEndian)
			fixes = append(fixes, "decoded BOM-less UTF-16 to UTF-8")
		}
	}

	if bytes.HasPrefix(clean, utf8BOM) {
		clean = clean[len(utf8BOM):]
		fixes = append(fixes, "removed the UTF-8 byte-order mark")
	}
	if n := bytes.Count(clean, utf8BOM); n > 0 {
		clean = bytes.ReplaceAll(clean, utf8BOM, nil)
		fixes = append(fixes, fmt.Sprintf("removed %d stray byte-order mark(s)", n))
	}
	if n := bytes.Count(clean, []byte{0}); n > 0 {
		clean = bytes.ReplaceAll(clean, []byte{0}, nil)
		fixes = append(fixes, fmt.Sprintf("removed %d NUL byte(s)", n))
	}

	// Anything still not UTF-8 needs a human (Latin-1, Shift-JIS, ...), not a
	// mechanical repair. Hand back the original bytes so the parser reports the
	// real problem instead of a misleading partial file.
	if len(fixes) == 0 || !utf8.Valid(clean) {
		return original, nil
	}
	return clean, fixes
}

// decodeUTF16 converts UTF-16 text to UTF-8. An odd trailing byte is dropped;
// lone surrogates become U+FFFD rather than an error, since the goal is to get
// the server booted and to preserve everything that is still readable.
func decodeUTF16(data []byte, bigEndian bool) []byte {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if bigEndian {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		} else {
			units = append(units, uint16(data[i+1])<<8|uint16(data[i]))
		}
	}
	return []byte(string(utf16.Decode(units)))
}

// sniffUTF16Endian guesses the byte order of BOM-less UTF-16 from the NUL lane:
// ASCII text stored as UTF-16LE has a zero high byte in every second position,
// UTF-16BE has one in every first position. Returns false when the bytes do not
// look like either, so a merely corrupt UTF-8 file is left alone.
func sniffUTF16Endian(b []byte) (bigEndian bool, ok bool) {
	if len(b) < 8 || len(b)%2 != 0 {
		return false, false
	}
	var pairs, evenZero, oddZero int
	for i := 0; i+1 < len(b); i += 2 {
		pairs++
		if b[i] == 0 {
			evenZero++
		}
		if b[i+1] == 0 {
			oddZero++
		}
	}
	switch {
	case oddZero*2 > pairs:
		return false, true
	case evenZero*2 > pairs:
		return true, true
	}
	return false, false
}

// writeFileAtomic replaces path with data without leaving a half-written config
// behind if the process dies mid-write, and keeps the file's permission bits
// (which matter on the shared hosts this runs on).
func writeFileAtomic(path string, data []byte) error {
	// Write through a symlinked config.toml to its target instead of replacing
	// the link with a regular file.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".toml-repair-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
