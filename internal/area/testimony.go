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

package area

import (
	"fmt"
	"strings"
)

// setServerStringField rewrites one slot of a server-format MS wire body.
// Nyathena stores testimony statements as raw MS strings, so editing one is a
// plain slot swap rather than a full parse/re-encode.
func setServerStringField(s string, field int, value string) string {
	fields := strings.Split(s, "#")
	if len(fields) > field {
		fields[field] = value
	}
	return strings.Join(fields, "#")
}

// setTextColorInServerString rewrites the TextColor slot (field 14) of a
// server-format MS wire body.
func setTextColorInServerString(s, color string) string {
	return setServerStringField(s, 14, color)
}

// tstPlaybackString rewrites a stored statement into the form it is broadcast
// in during playback.
//
// A replayed statement belongs to nobody. Every client that receives an MS
// whose char id (field 8) matches its own treats the message as the echo of a
// message it just sent and resets its IC input: the AO2 desktop client calls
// reset_ui() (clearing the text box, objection/realization/screenshake state
// and the evidence spinner) and webAO calls resetICParams() (clearing the input
// box, flash/shake, preanim/sfx and the selected shout). Replaying with the
// recorded char id therefore wiped the input of whoever held that slot — almost
// always the witness who was still typing. -1 belongs to no one and every
// client accepts it, so the statement still animates as the right character
// without being mistaken for anyone's own message.
//
// With no usable char id the clients fall back to the showname field (15) for
// the name box, so an empty showname is filled from the character folder
// (field 2) to keep the name readable.
func tstPlaybackString(s string) string {
	fields := strings.Split(s, "#")
	if len(fields) <= 15 {
		// Not a well-formed statement: there is no char id/showname pair to
		// rewrite, so hand back exactly what was recorded.
		return s
	}
	fields[8] = "-1"
	if strings.TrimSpace(fields[15]) == "" {
		fields[15] = fields[2]
	}
	return strings.Join(fields, "#")
}

// TstState returns the testimony recorder's current state.
func (a *Area) TstState() TRState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tr.State
}

// SetTstState sets the testimony recorder's state.
func (a *Area) SetTstState(s TRState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tr.State = s
}

// CurrentTstStatement returns the testimony recorder's current statement.
func (a *Area) CurrentTstStatement() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.tr.Testimony) == 0 {
		return ""
	}
	if a.tr.Index < 0 || a.tr.Index >= len(a.tr.Testimony) {
		a.tr.Index = len(a.tr.Testimony) - 1
	}
	return a.tr.Testimony[a.tr.Index]
}

// CurrentTstPlaybackStatement returns the testimony recorder's current
// statement in the form it is broadcast in during playback: char id -1, and an
// empty showname filled from the character folder. See tstPlaybackString for
// why a replayed statement must not carry the recorded char id.
func (a *Area) CurrentTstPlaybackStatement() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.tr.Testimony) == 0 {
		return ""
	}
	if a.tr.Index < 0 || a.tr.Index >= len(a.tr.Testimony) {
		a.tr.Index = len(a.tr.Testimony) - 1
	}
	return tstPlaybackString(a.tr.Testimony[a.tr.Index])
}

// CurrentTstIndex returns the testimony recorder's current index.
func (a *Area) CurrentTstIndex() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tr.Index
}

// TstInsert inserts a new statement into the testimony.
func (a *Area) TstInsert(s string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tr.Index < 0 || a.tr.Index >= len(a.tr.Testimony) {
		return fmt.Errorf("index out of range")
	}
	if a.tr.Index != 0 {
		s = setTextColorInServerString(s, "1")
	}
	a.tr.Testimony = append(a.tr.Testimony, "")
	copy(a.tr.Testimony[a.tr.Index+2:], a.tr.Testimony[a.tr.Index+1:])
	a.tr.Testimony[a.tr.Index+1] = s
	return nil
}

// TstRemove removes the current statement from the testimony.
func (a *Area) TstRemove() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.tr.Testimony) < 2 {
		return fmt.Errorf("empty testimony")
	}
	if a.tr.Index < 0 || a.tr.Index >= len(a.tr.Testimony) {
		return fmt.Errorf("index out of range")
	}
	a.tr.Testimony = append(a.tr.Testimony[:a.tr.Index], a.tr.Testimony[a.tr.Index+1:]...)
	if a.tr.Index >= len(a.tr.Testimony) {
		a.tr.Index = len(a.tr.Testimony) - 1
	}
	return nil
}

// TstUpdate updates the testimony's current statement.
func (a *Area) TstUpdate(s string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tr.Index < 0 || a.tr.Index >= len(a.tr.Testimony) {
		return fmt.Errorf("index out of range")
	}
	if a.tr.Index != 0 {
		s = setTextColorInServerString(s, "1")
	}
	a.tr.Testimony[a.tr.Index] = s
	return nil
}

// TstAdvance advances the testimony forward by one statement.
func (a *Area) TstAdvance() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.tr.Testimony) == 0 {
		a.tr.Index = 0
		return
	}
	if a.tr.Index >= len(a.tr.Testimony)-1 {
		if len(a.tr.Testimony) > 1 {
			a.tr.Index = 1
		} else {
			// Only the title has been recorded so far (len == 0 or 1). Move
			// past it instead of wrapping back to 0, otherwise the next
			// statement sent during /testify recording is mistaken for the
			// title again and recorded twice.
			a.tr.Index = len(a.tr.Testimony)
		}
	} else {
		a.tr.Index++
	}
}

// TstRewind advances the testimony backward by one statement.
func (a *Area) TstRewind() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.tr.Testimony) == 0 {
		a.tr.Index = 0
		return
	}
	if a.tr.Index <= 1 || a.tr.Index >= len(a.tr.Testimony) {
		a.tr.Index = len(a.tr.Testimony) - 1
	} else {
		a.tr.Index--
	}
}

// TstAppend appends a new statement to the testimony.
func (a *Area) TstAppend(s string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tr.Index != 0 {
		s = setTextColorInServerString(s, "1")
	}
	a.tr.Testimony = append(a.tr.Testimony, s)
}

// TstClear clears the currently recorded testimony.
func (a *Area) TstClear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tr.Testimony = []string{}
	a.tr.Index = 0
}

// TstStatements returns a copy of the recorder's stored statements, title
// included. Used by /savetestimony, which needs the raw MS bodies rather than
// the message text /testimony lists.
func (a *Area) TstStatements() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.tr.Testimony))
	copy(out, a.tr.Testimony)
	return out
}

// TstReplaceAll replaces the whole testimony with statements and rewinds to the
// title. The statements are stored exactly as given -- unlike TstAppend this
// does not rewrite the statement text colour, so a loaded testimony keeps the
// colours it was recorded with.
func (a *Area) TstReplaceAll(statements []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tr.Testimony = append([]string{}, statements...)
	a.tr.Index = 0
}

// TstLen returns the length of the testimony.
func (a *Area) TstLen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.tr.Testimony)
}

// TstJump advanced the testimony to the given index.
func (a *Area) TstJump(i int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if i < 0 {
		i = 0
	}
	if i >= len(a.tr.Testimony) && len(a.tr.Testimony) > 0 {
		i = len(a.tr.Testimony) - 1
	}
	a.tr.Index = i
}
