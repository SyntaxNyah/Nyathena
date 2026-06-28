package athena

import (
	"testing"

	"github.com/MangosArentLiterature/Athena/internal/permissions"
)

// TestSenderBypassesIgnore pins the contract behind the /ignore command and the
// IC/OOC ignore-list bypass: real moderators and admins always override a
// recipient's /ignore (their messages can't be silenced), while shadow mods do
// NOT — being un-ignorable is exactly what would out a shadow mod, so they must
// be ignorable just like any normal player.
func TestSenderBypassesIgnore(t *testing.T) {
	pf := permissions.PermissionField
	cases := []struct {
		name string
		perm uint64
		want bool
	}{
		{"plain player", 0, false},
		{"cm only", pf["CM"], false},
		{"dj only", pf["DJ"], false},
		{"cm+dj", pf["CM"] | pf["DJ"], false},
		{"regular mod (mute)", pf["MUTE"], true},
		{"regular mod (kick+ban)", pf["KICK"] | pf["BAN"], true},
		{"admin", pf["ADMIN"], true},
		{"shadow mod", pf["SHADOW"] | pf["MUTE"], false},
		{"shadow bit alone", pf["SHADOW"], false},
		{"shadow with full mod kit", pf["SHADOW"] | pf["KICK"] | pf["BAN"] | pf["MUTE"], false},
	}
	for _, tc := range cases {
		if got := senderBypassesIgnore(tc.perm); got != tc.want {
			t.Errorf("%s: senderBypassesIgnore(%#x) = %v, want %v", tc.name, tc.perm, got, tc.want)
		}
	}
}
