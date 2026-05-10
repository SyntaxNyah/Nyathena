package athena

import (
	"reflect"
	"testing"
)

// TestExtractHiddenFlag covers the bug where Go's flag.Parse stops at the first
// positional, so a trailing "-h" (e.g. /tsundere 7 -h) was previously ignored.
// The preprocessor must extract -h regardless of position and must not eat a
// "-h" used as the value of -r or -d.
func TestExtractHiddenFlag(t *testing.T) {
	tests := []struct {
		name       string
		in         []string
		wantArgs   []string
		wantHidden bool
	}{
		{"trailing -h after uid", []string{"7", "-h"}, []string{"7"}, true},
		{"trailing -h after global", []string{"global", "-h"}, []string{"global"}, true},
		{"leading -h", []string{"-h", "7"}, []string{"7"}, true},
		{"-h between flags and uid", []string{"-d", "10m", "-h", "7"}, []string{"-d", "10m", "7"}, true},
		{"-h with -d -r before uid", []string{"-d", "10m", "-r", "rude", "-h", "7"}, []string{"-d", "10m", "-r", "rude", "7"}, true},
		{"-h after duration and uid", []string{"-d", "10m", "7", "-h"}, []string{"-d", "10m", "7"}, true},
		{"no -h", []string{"7"}, []string{"7"}, false},
		{"-h=true", []string{"7", "-h=true"}, []string{"7"}, true},
		{"-h=false", []string{"7", "-h=false"}, []string{"7"}, false},
		{"--h long form", []string{"7", "--h"}, []string{"7"}, true},
		{"-h as reason value is preserved", []string{"-r", "-h", "7"}, []string{"-r", "-h", "7"}, false},
		{"-h as duration value is preserved", []string{"-d", "-h", "7"}, []string{"-d", "-h", "7"}, false},
		{"empty args", nil, []string{}, false},
		{"comma-separated uids with trailing -h", []string{"7,8,9", "-h"}, []string{"7,8,9"}, true},
		{"stack-style multi-arg trailing -h", []string{"uwu", "pirate", "7", "-h"}, []string{"uwu", "pirate", "7"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotArgs, gotHidden := extractHiddenFlag(tc.in)
			if gotHidden != tc.wantHidden {
				t.Errorf("hidden = %v, want %v", gotHidden, tc.wantHidden)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Errorf("args = %v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}
