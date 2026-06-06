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

package packet

import (
	"encoding/json"
	"reflect"
	"testing"
)

// decodeJSON is a tiny helper for asserting on JSON output without depending
// on key ordering or map iteration order.
func decodeJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json output is not valid JSON: %v\nraw=%s", err, b)
	}
	return got
}

func TestParseJSON_HI(t *testing.T) {
	pkt, err := ParseJSON(`{"$header":"HI","hdid":"abc123"}`)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if pkt.Header != "HI" {
		t.Fatalf("header = %q, want HI", pkt.Header)
	}
	if !reflect.DeepEqual(pkt.Body, []string{"abc123"}) {
		t.Fatalf("body = %#v, want [abc123]", pkt.Body)
	}

	// The standard ParseHI should accept the same body.
	hi, err := ParseHI(pkt.Body)
	if err != nil || hi.HDID != "abc123" {
		t.Fatalf("ParseHI = (%+v, %v)", hi, err)
	}
}

func TestParseJSON_CC_relicSlot(t *testing.T) {
	// CC's wire form is "CC#0#<char_id>#<char_pw>#%". The leading "0" is a
	// protocol relic; ParseCC indexes char_id at body[1]. Verify the JSON
	// path produces a body with that slot preserved (empty placeholder),
	// even when the JSON doesn't mention it.
	pkt, err := ParseJSON(`{"$header":"CC","char_id":7,"char_pw":""}`)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if got, want := pkt.Body, []string{"", "7", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %#v, want %#v", got, want)
	}
	cc, err := ParseCC(pkt.Body)
	if err != nil || cc.CharID != 7 {
		t.Fatalf("ParseCC = (%+v, %v)", cc, err)
	}
}

func TestParseJSON_NumberCoercion(t *testing.T) {
	// HP fields are documented as numbers; the JSON form should accept
	// numeric values and flatten them to strings for the body slice.
	pkt, err := ParseJSON(`{"$header":"HP","bar":2,"value":10}`)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	hp, err := ParseHP(pkt.Body)
	if err != nil {
		t.Fatalf("ParseHP: %v", err)
	}
	if hp.Bar != 2 || hp.Value != 10 {
		t.Fatalf("HP = %+v, want {Bar:2 Value:10}", hp)
	}
}

func TestParseJSON_BoolCoercion(t *testing.T) {
	// SETCASE's per-role fields arrive as "0"/"1" strings; the JSON form
	// should also accept native booleans.
	pkt, err := ParseJSON(`{"$header":"SETCASE","caselist":"","cm":"0","def":true,"pro":false,"judge":"0","jury":"0","steno":"0"}`)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	sc, err := ParseSETCASE(pkt.Body)
	if err != nil {
		t.Fatalf("ParseSETCASE: %v", err)
	}
	if sc.Def != "1" || sc.Pro != "0" {
		t.Fatalf("SETCASE booleans not coerced: %+v", sc)
	}
}

func TestParseJSON_MSClient(t *testing.T) {
	// MS-client has a 26-field body; verify a representative named field
	// lands in the right wire slot when round-tripped through ParseMSClient.
	pkt, err := ParseJSON(`{
		"$header":"MS","desk_modifier":"1","preanim":"-",
		"character":"Phoenix","emote":"normal","message":"Objection!",
		"side":"wit","sfx_name":"0","emote_modifier":1,"char_id":3,
		"sfx_delay":"0","shout_modifier":"2","evidence_id":0,"flip":"0",
		"realization":"0","text_color":2,"showname":"Mr. Wright",
		"paired_charid":"-1","offset":"0&0",
		"noninterrupting_preanim":"0","sfx_looping":"0","screenshake":"0",
		"frames_shake":"","frames_realization":"","frames_sfx":"",
		"additive":"0","effect":"","blips":"male"
	}`)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	ms := ParseMSClient(pkt.Body)
	if ms.Character != "Phoenix" || ms.Message != "Objection!" || ms.CharID != "3" || ms.Blips != "male" {
		t.Fatalf("ParseMSClient round-trip mismatch: %+v", ms)
	}
}

func TestBuildJSON_HI_unknownHeaderFallback(t *testing.T) {
	// HI isn't a server-direction packet — it should fall through to the
	// generic envelope so we don't silently drop it.
	out := BuildJSON("HI", []string{"abc"})
	got := decodeJSON(t, out)
	if got["$header"] != "HI" {
		t.Fatalf("$header = %v, want HI", got["$header"])
	}
	body, ok := got["body"].([]any)
	if !ok || len(body) != 1 || body[0] != "abc" {
		t.Fatalf("body = %#v, want [abc]", got["body"])
	}
}

func TestBuildJSON_ID_PlayerIDIsNumber(t *testing.T) {
	// Spec says player_id is a JSON number, not a string. Verify the schema
	// promotes it past the default "wire body is strings" treatment.
	pkt := &IDClient{PlayerNumber: 42, Software: "athena", Version: "1.0"}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	if n, ok := got["player_id"].(float64); !ok || n != 42 {
		t.Fatalf("player_id = %#v, want JSON number 42", got["player_id"])
	}
	if got["software"] != "athena" || got["version"] != "1.0" {
		t.Fatalf("ID strings wrong: %v", got)
	}
}

func TestBuildJSON_PN(t *testing.T) {
	pkt := &PN{PlayerCount: 4, MaxPlayers: 100, ServerDescription: "Welcome"}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	if got["$header"] != "PN" || got["player_count"] != float64(4) || got["max_players"] != float64(100) || got["server_description"] != "Welcome" {
		t.Fatalf("PN JSON wrong: %v", got)
	}
}

func TestBuildJSON_FL_StringArray(t *testing.T) {
	pkt := &FL{Features: []string{"yellowtext", "flipping", "noencryption"}}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	feats, ok := got["features"].([]any)
	if !ok || len(feats) != 3 || feats[0] != "yellowtext" {
		t.Fatalf("features array wrong: %v", got)
	}
	if _, present := got["body"]; present {
		t.Fatalf("FL should not emit a generic body; got: %v", got)
	}
}

func TestBuildJSON_ARUP_TypeAndTail(t *testing.T) {
	// ARUP combines one leading scalar (update_type) with a variable tail
	// (update_data). Verify both make it into the JSON object.
	pkt := &ARUP{Type: ARUPPlayerCounts, Data: []string{"4", "3", "7"}}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	if got["update_type"] != "0" {
		t.Fatalf("update_type = %v, want 0", got["update_type"])
	}
	data, ok := got["update_data"].([]any)
	if !ok || len(data) != 3 {
		t.Fatalf("update_data = %#v", got["update_data"])
	}
	if data[0] != "4" || data[1] != "3" || data[2] != "7" {
		t.Fatalf("update_data values wrong: %v", data)
	}
}

func TestBuildJSON_SC_ObjectArray(t *testing.T) {
	// SC stores each character as "name&desc&evi" in the wire body. The
	// JSON form should unfold each entry into an object.
	pkt := &SC{Entries: []string{"Phoenix&Defense attorney&", "Edgeworth&Prosecutor&"}}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	arr, ok := got["char_data"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("char_data array wrong: %v", got)
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("char_data[0] not an object: %#v", arr[0])
	}
	if first["name"] != "Phoenix" || first["desc"] != "Defense attorney" || first["evidence"] != "" {
		t.Fatalf("char_data[0] fields wrong: %v", first)
	}
}

func TestBuildJSON_SD_SplitOnStar(t *testing.T) {
	// SD's lone wire field is a '*'-joined position list. In JSON it
	// should be split back into an array.
	pkt := &SD{Sides: []string{"wit", "def", "pro"}}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	sides, ok := got["sides"].([]any)
	if !ok || len(sides) != 3 || sides[0] != "wit" || sides[2] != "pro" {
		t.Fatalf("sides array wrong: %v", got)
	}
}

func TestBuildJSON_PV_skipsRelic(t *testing.T) {
	// PV's wire form is "PV#<player_id>#CID#<char_id>#%". The "CID" slot
	// is a protocol relic and should NOT appear as a JSON field.
	pkt := &PV{PlayerID: 0, CharID: 5}
	out := BuildJSON(pkt.Header(), pkt.Args())
	got := decodeJSON(t, out)
	if got["player_id"] != "0" || got["char_id"] != "5" {
		t.Fatalf("PV fields wrong: %v", got)
	}
	for k := range got {
		if k == "_" || k == "CID" {
			t.Fatalf("PV JSON should not contain relic key %q", k)
		}
	}
}

func TestBuildJSON_MS_ServerDirection(t *testing.T) {
	// MS-server adds paired_name/paired_emote/paired_offset/paired_flip
	// relative to MS-client. Verify the outbound schema is the 30-field
	// shape, not the 26-field client shape.
	ms := &MSPacket{
		DeskMod: "1", PreAnim: "-", Character: "Phoenix", Emote: "normal",
		Message: "Hi", Side: "wit", SfxName: "0", EmoteModifier: "1",
		CharID: "3", SfxDelay: "0", ShoutModifier: "0", Evidence: "0",
		Flip: "0", Realization: "0", TextColor: "0", Showname: "",
		OtherCharID: "-1", OtherName: "Edgeworth", OtherEmote: "normal",
		SelfOffset: "0&0", OtherOffset: "0&0", OtherFlip: "0",
		NonInterruptingPreAnim: "0", SfxLooping: "0", Screenshake: "0",
		FramesShake: "", FramesRealization: "", FramesSfx: "",
		Additive: "0", Effect: "",
	}
	out := BuildJSON(ms.Header(), ms.Args())
	got := decodeJSON(t, out)
	if got["paired_name"] != "Edgeworth" {
		t.Fatalf("paired_name missing: %v", got)
	}
	// paired_flip is a numeric field, so it is now emitted as a JSON number
	// (decoded as float64), matching the MSBroadcast schema's integer type.
	if got["paired_emote"] != "normal" || got["paired_flip"] != float64(0) {
		t.Fatalf("paired_* wrong: %v", got)
	}
	// offset / paired_offset are emitted as {x,y} objects, not "0&0" strings.
	off, ok := got["offset"].(map[string]any)
	if !ok || off["x"] != float64(0) || off["y"] != float64(0) {
		t.Fatalf("offset should be an {x,y} object, got %v", got["offset"])
	}
}

// TestBuildJSON_MS_PairOrderSuffixStripped verifies the FantaCode-only
// "^order" pair-ordering suffix on the pair charid is stripped so JSON-mode
// paired_charid stays a bare number and survives MSBroadcast schema validation.
func TestBuildJSON_MS_PairOrderSuffixStripped(t *testing.T) {
	ms := &MSPacket{
		DeskMod: "1", PreAnim: "-", Character: "Phoenix", Emote: "normal",
		Message: "Hi", Side: "wit", SfxName: "0", EmoteModifier: "1",
		CharID: "3", SfxDelay: "0", ShoutModifier: "0", Evidence: "0",
		Flip: "0", Realization: "0", TextColor: "0", Showname: "",
		OtherCharID: "5^1", OtherName: "Edgeworth", OtherEmote: "normal",
		SelfOffset: "0&0", OtherOffset: "0&0", OtherFlip: "0",
		NonInterruptingPreAnim: "0", SfxLooping: "0", Screenshake: "0",
		FramesShake: "", FramesRealization: "", FramesSfx: "",
		Additive: "0", Effect: "",
	}
	out := BuildJSON(ms.Header(), ms.Args())
	got := decodeJSON(t, out)
	if got["paired_charid"] != float64(5) {
		t.Fatalf("paired_charid should strip the ^order suffix to numeric 5, got %v", got["paired_charid"])
	}
}

func TestParseJSON_RejectsMissingHeader(t *testing.T) {
	_, err := ParseJSON(`{"hdid":"abc"}`)
	if err == nil {
		t.Fatalf("ParseJSON should reject packet without header")
	}
}

func TestParseJSON_RejectsBlankHeader(t *testing.T) {
	_, err := ParseJSON(`{"$header":""}`)
	if err == nil {
		t.Fatalf("ParseJSON should reject empty header")
	}
}

func TestParseJSON_UnknownHeader(t *testing.T) {
	// Unknown headers should parse cleanly to a Packet with no body so the
	// dispatcher can skip them — same behaviour as NewPacket for unknown
	// FantaCode headers.
	pkt, err := ParseJSON(`{"$header":"XX","stuff":1}`)
	if err != nil {
		t.Fatalf("ParseJSON should tolerate unknown headers: %v", err)
	}
	if pkt.Header != "XX" || len(pkt.Body) != 0 {
		t.Fatalf("unknown header packet wrong: %+v", pkt)
	}
}

func TestParseJSON_MS_OffsetAsObject(t *testing.T) {
	// Real-world JSON clients send offset as {"x":..,"y":..} rather than
	// the documented "x&y" string. Verify both forms fold to the same
	// "0&0" wire body that ParseMSClient already understands.
	pkt, err := ParseJSON(`{"$header":"MS","desk_modifier":1,"preanim":"-","character":"Maya","emote":"normal","message":"hi","side":"wit","sfx_name":"0","emote_modifier":0,"char_id":37,"sfx_delay":0,"shout_modifier":0,"evidence_id":0,"flip":0,"realization":false,"text_color":0,"showname":"","paired_charid":-1,"offset":{"x":0,"y":0},"noninterrupting_preanim":false,"sfx_looping":false,"screenshake":false,"frames_shake":"-","frames_realization":"-","frames_sfx":"-","additive":false,"effect":"||"}`)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	ms := ParseMSClient(pkt.Body)
	if ms.SelfOffset != "0&0" {
		t.Fatalf("SelfOffset = %q, want \"0&0\"", ms.SelfOffset)
	}
	if ms.Character != "Maya" || ms.CharID != "37" || ms.Message != "hi" {
		t.Fatalf("MS round-trip mismatch: %+v", ms)
	}
}

func TestBuildJSON_VSCAPS_EnabledIsBoolean(t *testing.T) {
	// VS_CAPS.enabled must be a JSON boolean. Other fields stay strings
	// unless explicitly marked numeric.
	enabled := BuildJSON("VS_CAPS", []string{"1", "1", "8", "opus", "48000", "20", "1500"})
	got := decodeJSON(t, enabled)
	if v, ok := got["enabled"].(bool); !ok || v != true {
		t.Fatalf("enabled = %#v, want JSON true", got["enabled"])
	}
	disabled := BuildJSON("VS_CAPS", []string{"0", "0", "8", "opus", "48000", "20", "1500"})
	got = decodeJSON(t, disabled)
	if v, ok := got["enabled"].(bool); !ok || v != false {
		t.Fatalf("enabled = %#v, want JSON false", got["enabled"])
	}
}

func TestParseJSON_LegacyHeaderKeyFallback(t *testing.T) {
	// "$header" is canonical, but accept the plain "header" key too so we
	// don't break clients that drop the sigil.
	pkt, err := ParseJSON(`{"header":"HI","hdid":"xyz"}`)
	if err != nil {
		t.Fatalf("ParseJSON should accept legacy \"header\" key: %v", err)
	}
	if pkt.Header != "HI" {
		t.Fatalf("header = %q, want HI", pkt.Header)
	}
}

func TestDecryptor_AdvertisesJSON(t *testing.T) {
	// The first packet sent on every connection — verifies our capability
	// signal hasn't regressed back to NOENCRYPT.
	d := &Decryptor{}
	if got := d.Args(); len(got) != 1 || got[0] != "JSON" {
		t.Fatalf("Decryptor.Args = %v, want [JSON]", got)
	}
}
