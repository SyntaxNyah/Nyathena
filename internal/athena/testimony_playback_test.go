package athena

import (
	"encoding/json"
	"strings"
	"testing"

	aolib "github.com/AO-Underground/aolib/go/v2"

	"github.com/MangosArentLiterature/Athena/internal/area"
	"github.com/MangosArentLiterature/Athena/internal/packetutil"
)

// recordedStatement builds the stored form of one testimony statement: the
// 30-field server->client MS body the IC handler records through
// MSToClient.ServerString().
func recordedStatement(charID, character, showname, message string) string {
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

// TestTestimonyPlaybackPacketBelongsToNobody pins the wire form of a replayed
// statement: char id -1 on both the FantaCode and JSON wires, so no client
// mistakes the replay for the echo of its own message and clears the player's
// input, while the character, emote, message and showname still render the
// statement as the witness who recorded it.
func TestTestimonyPlaybackPacketBelongsToNobody(t *testing.T) {
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)
	a.TstAppend(recordedStatement("7", "lucifer", "Lou", "I saw nothing."))

	ms := ParseMSToClientString(a.CurrentTstPlaybackStatement())
	if ms.CharID != "-1" {
		t.Errorf("playback CharID = %q, want %q", ms.CharID, "-1")
	}
	if ms.Character != "lucifer" || ms.Emote != "normal" || ms.Message != "I saw nothing." {
		t.Errorf("playback lost statement content: character=%q emote=%q message=%q",
			ms.Character, ms.Emote, ms.Message)
	}

	if got := ms.Args()[8]; got != "-1" {
		t.Errorf("FantaCode char id field = %q, want %q", got, "-1")
	}

	raw, err := packetutil.Encode(ms, aolib.WireJSON)
	if err != nil {
		t.Fatalf("Encode(JSON) error: %v", err)
	}
	var got struct {
		CharID    int    `json:"char_id"`
		Character string `json:"character"`
		Message   string `json:"message"`
		Showname  string `json:"showname"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if got.CharID != -1 {
		t.Errorf("JSON char_id = %d, want -1", got.CharID)
	}
	if got.Character != "lucifer" || got.Message != "I saw nothing." || got.Showname != "Lou" {
		t.Errorf("JSON statement content = %+v, want character/message/showname preserved", got)
	}
}

// TestTestimonyPlaybackPacketFillsEmptyShowname: with char id -1 the clients
// take the name from the showname field, so an empty one must fall back to the
// character folder instead of rendering a blank name box.
func TestTestimonyPlaybackPacketFillsEmptyShowname(t *testing.T) {
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)
	a.TstAppend(recordedStatement("7", "lucifer", "", "I saw nothing."))

	ms := ParseMSToClientString(a.CurrentTstPlaybackStatement())
	if ms.Showname != "lucifer" {
		t.Errorf("playback Showname = %q, want the character folder %q", ms.Showname, "lucifer")
	}
}

// TestParseTestimonyNav pins the navigation gate: only a bare token steers the
// testimony. Searching for the token anywhere in the message (the old
// behaviour) swallowed perfectly ordinary IC lines -- "I <3 that" rewound the
// testimony and never reached the room.
func TestParseTestimonyNav(t *testing.T) {
	tests := []struct {
		msg     string
		wantOK  bool
		wantDir string
		wantID  string
	}{
		{"<", true, "<", ""},
		{">", true, ">", ""},
		{">12", true, ">", "12"},
		{"  >2  ", true, ">", "2"},
		{"<3", true, "<", "3"}, // accepted as <N like the old parser did
		{"I <3 that", false, "", ""},
		{"I don't know >", false, "", ""},
		{"> 2", false, "", ""},
		{"a>b", false, "", ""},
		{">>", false, "", ""},
		{"", false, "", ""},
		{"   ", false, "", ""},
	}
	for _, tc := range tests {
		dir, id, ok := parseTestimonyNav(tc.msg)
		if ok != tc.wantOK {
			t.Errorf("parseTestimonyNav(%q) ok = %v, want %v", tc.msg, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if dir != tc.wantDir || id != tc.wantID {
			t.Errorf("parseTestimonyNav(%q) = (%q, %q), want (%q, %q)", tc.msg, dir, id, tc.wantDir, tc.wantID)
		}
	}
}

// TestJumpTestimonyTarget pins which jump targets are real statements: 0 is the
// title and the end of the slice loops, so only 1..len-1 may be jumped to.
func TestJumpTestimonyTarget(t *testing.T) {
	a := area.NewArea(area.AreaData{}, 50, 0, area.EviAny)
	a.TstAppend(recordedStatement("1", "a", "", "title"))
	a.TstAppend(recordedStatement("1", "a", "", "one"))
	a.TstAppend(recordedStatement("1", "a", "", "two"))

	if id, ok := jumpTestimonyTarget(a, "2"); !ok || id != 2 {
		t.Errorf("jumpTestimonyTarget(2) = (%d, %v), want (2, true)", id, ok)
	}
	for _, bad := range []string{"0", "3", "-1", "abc", ""} {
		if _, ok := jumpTestimonyTarget(a, bad); ok {
			t.Errorf("jumpTestimonyTarget(%q) accepted an invalid target", bad)
		}
	}
}
