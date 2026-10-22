package athena

// Nyathena fork addition: tests for the wire-format-aware FL shim. A client's
// own FL is not modelled by aolib (FL is server→client only), so it reaches
// handleUnknownHeader, where it may arrive as FantaCode or as JSON. Before this
// shim learned JSON, a JSON advert was parsed by aolib.NewPacket — a '#'-splitter
// that returns a nil Body without an error — and cleared the feature set.

import "testing"

// TestFLJSONAdvertLands verifies a JSON-mode client's capability advertisement
// registers the feature. This is the form AsyncAO and LemmyAO emit once the
// server advertises JSON in the decryptor greeting.
func TestFLJSONAdvertLands(t *testing.T) {
	c := &Client{}
	c.handleUnknownHeader("FL", []byte(`{"$header":"FL","features":["timeleap"]}`))

	if !c.SupportsFeature("timeleap") {
		t.Fatal("JSON FL advert did not register \"timeleap\"")
	}
}

// TestFLPositionalAdvertStillLands verifies the FantaCode form is unchanged.
// The trailing '%' is already stripped by the read loop (client.go), so this
// matches what production passes in.
func TestFLPositionalAdvertStillLands(t *testing.T) {
	c := &Client{}
	c.handleUnknownHeader("FL", []byte("FL#grouppair#"))

	if !c.SupportsFeature("grouppair") {
		t.Fatal("positional FL advert did not register \"grouppair\"")
	}
}

// TestFLAdvertMultipleFeatures verifies a single frame carrying several
// features registers all of them (AsyncAO packs every feature into one frame).
func TestFLAdvertMultipleFeatures(t *testing.T) {
	c := &Client{}
	c.handleUnknownHeader("FL", []byte("FL#grouppair#timeleap#"))

	for _, f := range []string{"grouppair", "timeleap"} {
		if !c.SupportsFeature(f) {
			t.Fatalf("positional FL advert did not register %q", f)
		}
	}
}

// TestFLJSONSkipsNonFLHeader verifies a JSON frame whose $header is not FL is
// not mistaken for an advert.
func TestFLJSONSkipsNonFLHeader(t *testing.T) {
	c := &Client{}
	c.handleUnknownHeader("FL", []byte(`{"$header":"MS","features":["timeleap"]}`))

	if c.SupportsFeature("timeleap") {
		t.Fatal("non-FL JSON body was treated as an FL advert")
	}
}
