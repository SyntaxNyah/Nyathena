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
	"os"
	"testing"
)

// loadSchemasForTest compiles the real schemas from the repository's top-level
// schemas/ folder and installs them, resetting the global validators when the
// test finishes so other tests in the package see the default (disabled) state.
func loadSchemasForTest(t *testing.T) {
	t.Helper()
	req, err := os.ReadFile("../../schemas/MSRequest.schema.json")
	if err != nil {
		t.Fatalf("reading MSRequest schema: %v", err)
	}
	bcast, err := os.ReadFile("../../schemas/MSBroadcast.schema.json")
	if err != nil {
		t.Fatalf("reading MSBroadcast schema: %v", err)
	}
	if err := CompileMSSchemas(req, bcast); err != nil {
		t.Fatalf("CompileMSSchemas: %v", err)
	}
	t.Cleanup(func() { msRequestSchema = nil; msBroadcastSchema = nil })
}

func TestMSSchemasCompile(t *testing.T) {
	loadSchemasForTest(t)
	if !MSSchemasLoaded() {
		t.Fatal("MSSchemasLoaded() = false after CompileMSSchemas")
	}
}

// TestValidateMSRequest_Accepts confirms a well-typed inbound MS object passes.
func TestValidateMSRequest_Accepts(t *testing.T) {
	loadSchemasForTest(t)
	valid := `{"$header":"MS","character":"Phoenix","emote":"(a)pointing","message":"Objection!","side":"wit","char_id":3}`
	if err := ValidateMSRequest([]byte(valid)); err != nil {
		t.Fatalf("valid MS request rejected: %v", err)
	}
	// And ParseJSON (which calls the validator) should accept it too.
	if _, err := ParseJSON(valid); err != nil {
		t.Fatalf("ParseJSON rejected a valid MS request: %v", err)
	}
}

// TestValidateMSRequest_Rejects covers the "type nonsense" cases the contract
// is meant to stop: a stringified number, an out-of-enum side, a missing
// required field, and an unexpected extra field.
func TestValidateMSRequest_Rejects(t *testing.T) {
	loadSchemasForTest(t)
	cases := map[string]string{
		"char_id as string":   `{"$header":"MS","character":"P","emote":"e","message":"m","side":"wit","char_id":"3"}`,
		"bad side enum":       `{"$header":"MS","character":"P","emote":"e","message":"m","side":"nope","char_id":3}`,
		"missing char_id":     `{"$header":"MS","character":"P","emote":"e","message":"m","side":"wit"}`,
		"additional property": `{"$header":"MS","character":"P","emote":"e","message":"m","side":"wit","char_id":3,"bogus":1}`,
		"offset as string":    `{"$header":"MS","character":"P","emote":"e","message":"m","side":"wit","char_id":3,"offset":"0&0"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateMSRequest([]byte(raw)); err == nil {
				t.Errorf("expected validation failure for %s, got nil", name)
			}
			// ParseJSON must surface the same rejection as an error (→ dropped).
			if _, err := ParseJSON(raw); err == nil {
				t.Errorf("ParseJSON should reject %s", name)
			}
		})
	}
}

// TestValidateMSRequest_NoOpWhenUnloaded verifies validation is fail-open until
// the schemas are compiled — a deployment that never loads them behaves exactly
// as before.
func TestValidateMSRequest_NoOpWhenUnloaded(t *testing.T) {
	if MSSchemasLoaded() {
		t.Skip("schemas already loaded by another test")
	}
	if err := ValidateMSRequest([]byte(`{"$header":"MS","char_id":"not a number"}`)); err != nil {
		t.Fatalf("validation should be a no-op when schemas are unloaded, got %v", err)
	}
}

// TestBuildJSON_MS_PassesBroadcastSchema is the round-trip guarantee: the JSON
// the server emits for a normal MS broadcast must validate against MSBroadcast,
// with numbers/booleans/objects rather than strings.
func TestBuildJSON_MS_PassesBroadcastSchema(t *testing.T) {
	loadSchemasForTest(t)

	ms := &MSPacket{
		DeskMod: "1", PreAnim: "-", Character: "Phoenix", Emote: "(a)pointing",
		Message: "Objection!", Side: "wit", SfxName: "0", EmoteModifier: "1",
		CharID: "3", SfxDelay: "0", ShoutModifier: "0", Evidence: "0",
		Flip: "0", Realization: "0", TextColor: "9", Showname: "Nick",
		OtherCharID: "-1", SelfOffset: "5&10",
		NonInterruptingPreAnim: "0", SfxLooping: "0", Screenshake: "0",
		FramesShake: "", FramesRealization: "", FramesSfx: "",
		Additive: "0", Effect: "",
	}
	out := BuildJSON(ms.Header(), ms.Args())
	if err := ValidateMSBroadcast(out); err != nil {
		t.Fatalf("server-built MS failed MSBroadcast validation: %v\njson=%s", err, out)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal built MS: %v", err)
	}
	if got["char_id"] != float64(3) {
		t.Errorf("char_id should be a JSON number 3, got %#v", got["char_id"])
	}
	if got["realization"] != false {
		t.Errorf("realization should be JSON boolean false, got %#v", got["realization"])
	}
	off, ok := got["offset"].(map[string]any)
	if !ok || off["x"] != float64(5) || off["y"] != float64(10) {
		t.Errorf("offset should be {x:5,y:10} object, got %#v", got["offset"])
	}
}

// TestBuildJSON_MS_OmitsBlips guards the decision to never emit the blips field
// in JSON mode (it is absent from MSBroadcast, whose additionalProperties is
// false, so emitting it would fail validation).
func TestBuildJSON_MS_OmitsBlips(t *testing.T) {
	loadSchemasForTest(t)
	ms := &MSPacket{
		Character: "P", Emote: "e", Message: "m", Side: "wit", CharID: "0",
		Blips: "sound", // set, but must not appear in the JSON output
	}
	out := BuildJSON(ms.Header(), ms.Args())
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := got["blips"]; present {
		t.Errorf("blips must not be emitted in JSON mode: %s", out)
	}
	if err := ValidateMSBroadcast(out); err != nil {
		t.Fatalf("MS with blips set should still validate (blips omitted): %v", err)
	}
}
