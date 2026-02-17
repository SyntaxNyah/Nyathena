package athena

import (
	"fmt"
	"strings"
	"testing"
)

func TestSendChunkedPacket(t *testing.T) {
	tests := []struct {
		name           string
		header         string
		contents       []string
		expectChunks   bool
		maxChunkSize   int
	}{
		{
			name:         "Small character list fits in one packet",
			header:       "SC",
			contents:     makeTestCharacters(50),
			expectChunks: false,
		},
		{
			name:         "Large character list requires chunking",
			header:       "SC",
			contents:     makeTestCharacters(2600),
			expectChunks: true,
		},
		{
			name:         "Music list typically fits in one packet",
			header:       "SM",
			contents:     makeTestMusic(165),
			expectChunks: false,
		},
		{
			name:         "Very large music list requires chunking",
			header:       "SM",
			contents:     makeTestMusic(1000),
			expectChunks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test client with a mock writer
			mockClient := &mockWriteClient{packets: []string{}}
			
			// Call the chunking function
			sendChunkedPacket(mockClient, tt.header, tt.contents)
			
			// Verify chunking behavior
			if tt.expectChunks && len(mockClient.packets) <= 1 {
				t.Errorf("Expected multiple chunks but got %d packet(s)", len(mockClient.packets))
			}
			if !tt.expectChunks && len(mockClient.packets) > 1 {
				t.Errorf("Expected single packet but got %d chunks", len(mockClient.packets))
			}
			
			// Verify all packets are under the max chunk size
			for i, packet := range mockClient.packets {
				if len(packet) > maxChunkedPacketSize {
					t.Errorf("Chunk %d exceeds maxChunkedPacketSize (%d bytes): %d bytes", 
						i+1, maxChunkedPacketSize, len(packet))
				}
			}
			
			// Verify all content was sent
			sentItems := 0
			for _, packet := range mockClient.packets {
				// Parse packet: "HEADER#item1#item2#...#%"
				parts := strings.Split(strings.TrimSuffix(packet, "#%"), "#")
				if len(parts) > 1 {
					sentItems += len(parts) - 1 // -1 for header
				}
			}
			if sentItems != len(tt.contents) {
				t.Errorf("Expected %d items, but sent %d", len(tt.contents), sentItems)
			}
			
			// Log chunk information
			t.Logf("Sent %d item(s) in %d packet(s)", len(tt.contents), len(mockClient.packets))
			for i, packet := range mockClient.packets {
				t.Logf("  Packet %d: %d bytes (%.1f KB)", i+1, len(packet), float64(len(packet))/1024)
			}
		})
	}
}

// mockWriteClient is a test double for Client that records written packets
type mockWriteClient struct {
	packets []string
}

func (m *mockWriteClient) write(message string) {
	m.packets = append(m.packets, message)
}

func (m *mockWriteClient) SendPacket(header string, contents ...string) {
	packet := header + "#" + strings.Join(contents, "#") + "#%"
	m.write(packet)
}

// makeTestCharacters generates test character names
func makeTestCharacters(count int) []string {
	chars := make([]string, count)
	for i := 0; i < count; i++ {
		chars[i] = fmt.Sprintf("Character_%04d", i)
	}
	return chars
}

// makeTestMusic generates test music entries
func makeTestMusic(count int) []string {
	music := make([]string, count)
	music[0] = "Courtroom 1, Courtroom 2, Courtroom 3" // area names
	for i := 1; i < count; i++ {
		music[i] = fmt.Sprintf("Ace Attorney/Music/[AA] Track %03d.opus", i)
	}
	return music
}

func TestChunkingPreservesOrder(t *testing.T) {
	// Create a character list
	original := makeTestCharacters(2600)
	
	// Send through chunking
	mockClient := &mockWriteClient{packets: []string{}}
	sendChunkedPacket(mockClient, "SC", original)
	
	// Reconstruct from packets
	var reconstructed []string
	for _, packet := range mockClient.packets {
		parts := strings.Split(strings.TrimSuffix(packet, "#%"), "#")
		if len(parts) > 1 {
			reconstructed = append(reconstructed, parts[1:]...) // Skip header
		}
	}
	
	// Verify order is preserved
	if len(reconstructed) != len(original) {
		t.Fatalf("Length mismatch: got %d, want %d", len(reconstructed), len(original))
	}
	
	for i := range original {
		if reconstructed[i] != original[i] {
			t.Errorf("Item %d mismatch: got %q, want %q", i, reconstructed[i], original[i])
		}
	}
}
