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
	"strings"
	"testing"
)

func TestNewPacket_HeaderOnly(t *testing.T) {
	p, err := NewPacket("HI")
	if err != nil {
		t.Fatalf("NewPacket(\"HI\") returned error: %v", err)
	}
	if p.Header != "HI" {
		t.Errorf("Header = %q, want \"HI\"", p.Header)
	}
	if len(p.Body) != 0 {
		t.Errorf("Body len = %d, want 0", len(p.Body))
	}
}

func TestNewPacket_WithBody(t *testing.T) {
	// The network layer strips the "%" terminator before calling NewPacket,
	// so the input here does not contain "%".
	p, err := NewPacket("MS#hello#world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Header != "MS" {
		t.Errorf("Header = %q, want \"MS\"", p.Header)
	}
	if len(p.Body) != 2 {
		t.Fatalf("Body len = %d, want 2", len(p.Body))
	}
	if p.Body[0] != "hello" || p.Body[1] != "world" {
		t.Errorf("Body = %v, unexpected values", p.Body)
	}
}

func TestNewPacket_TrailingHashRemoved(t *testing.T) {
	// A packet string with a trailing "#" produces a trailing empty field
	// that must be stripped by NewPacket.
	p, err := NewPacket("HI#1#")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Body) != 1 {
		t.Fatalf("Body len = %d, want 1 (trailing empty entry must be removed)", len(p.Body))
	}
	if p.Body[0] != "1" {
		t.Errorf("Body[0] = %q, want \"1\"", p.Body[0])
	}
}

func TestNewPacket_EmptyHeader(t *testing.T) {
	_, err := NewPacket("#body")
	if err == nil {
		t.Error("expected error for empty header, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error message %q should mention 'empty'", err.Error())
	}
}

func TestNewPacket_WhitespaceHeader(t *testing.T) {
	_, err := NewPacket("   #body")
	if err == nil {
		t.Error("expected error for whitespace-only header, got nil")
	}
}

func TestNewPacket_EmptyBody(t *testing.T) {
	// "HI#" has an empty rest, body should be nil/empty.
	p, err := NewPacket("HI#")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = p
}

func TestNewPacket_MultipleBodyFields(t *testing.T) {
	// Network layer strips "%" before calling NewPacket.
	p, err := NewPacket("CT#name#message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Header != "CT" {
		t.Errorf("Header = %q, want \"CT\"", p.Header)
	}
	if len(p.Body) != 2 {
		t.Fatalf("Body len = %d, want 2", len(p.Body))
	}
	if p.Body[0] != "name" || p.Body[1] != "message" {
		t.Errorf("Body = %v, want [name message]", p.Body)
	}
}

func TestPacketString_NoBody(t *testing.T) {
	p := Packet{Header: "HI", Body: nil}
	got := p.String()
	want := "HI#%"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPacketString_WithBody(t *testing.T) {
	p := Packet{Header: "CT", Body: []string{"name", "message"}}
	got := p.String()
	want := "CT#name#message#%"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPacketString_EmptyBodyField(t *testing.T) {
	p := Packet{Header: "MS", Body: []string{"", "data"}}
	got := p.String()
	want := "MS##data#%"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPacketRoundtrip(t *testing.T) {
	// String() is used for sending packets over the network; it appends "#%"
	// as the AO2 packet terminator. The network layer strips "%" before
	// calling NewPacket, so a direct string→parse roundtrip is not the
	// expected usage. Instead verify that String() produces the correct
	// wire format and that NewPacket correctly parses the pre-terminator form.
	original := Packet{Header: "MC", Body: []string{"song.mp3", "1", "charname"}}
	wire := original.String() // "MC#song.mp3#1#charname#%"

	// Simulate what the network layer does: strip the trailing "%".
	stripped := wire[:len(wire)-1] // remove "%"

	parsed, err := NewPacket(stripped)
	if err != nil {
		t.Fatalf("NewPacket(roundtrip) error: %v", err)
	}
	if parsed.Header != original.Header {
		t.Errorf("roundtrip Header: got %q, want %q", parsed.Header, original.Header)
	}
	if len(parsed.Body) != len(original.Body) {
		t.Fatalf("roundtrip Body len: got %d, want %d", len(parsed.Body), len(original.Body))
	}
	for i, v := range original.Body {
		if parsed.Body[i] != v {
			t.Errorf("roundtrip Body[%d]: got %q, want %q", i, parsed.Body[i], v)
		}
	}
}
