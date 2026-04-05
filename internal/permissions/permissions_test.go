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

package permissions

import (
	"math"
	"testing"
)

func TestGetPermissions_Empty(t *testing.T) {
	r := &Role{Name: "empty", Permissions: []string{}}
	got := r.GetPermissions()
	if got != 0 {
		t.Errorf("GetPermissions([]) = %d, want 0", got)
	}
}

func TestGetPermissions_None(t *testing.T) {
	r := &Role{Name: "none", Permissions: []string{"NONE"}}
	got := r.GetPermissions()
	if got != 0 {
		t.Errorf("GetPermissions([NONE]) = %d, want 0", got)
	}
}

func TestGetPermissions_SinglePerm(t *testing.T) {
	r := &Role{Name: "kicker", Permissions: []string{"KICK"}}
	got := r.GetPermissions()
	want := PermissionField["KICK"]
	if got != want {
		t.Errorf("GetPermissions([KICK]) = %d, want %d", got, want)
	}
}

func TestGetPermissions_MultiplePerm(t *testing.T) {
	r := &Role{Name: "mod", Permissions: []string{"KICK", "BAN", "MUTE"}}
	got := r.GetPermissions()
	want := PermissionField["KICK"] | PermissionField["BAN"] | PermissionField["MUTE"]
	if got != want {
		t.Errorf("GetPermissions([KICK,BAN,MUTE]) = %d, want %d", got, want)
	}
}

func TestGetPermissions_Admin(t *testing.T) {
	r := &Role{Name: "admin", Permissions: []string{"ADMIN"}}
	got := r.GetPermissions()
	if got != math.MaxUint64 {
		t.Errorf("GetPermissions([ADMIN]) = %d, want MaxUint64", got)
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name     string
		perm     uint64
		required uint64
		want     bool
	}{
		{"exact match", PermissionField["KICK"], PermissionField["KICK"], true},
		{"superset", PermissionField["KICK"] | PermissionField["BAN"], PermissionField["KICK"], true},
		{"missing perm", PermissionField["CM"], PermissionField["KICK"], false},
		{"no perms", 0, PermissionField["KICK"], false},
		{"admin has all", math.MaxUint64, PermissionField["BAN"], true},
		{"none required", PermissionField["KICK"], 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPermission(tt.perm, tt.required)
			if got != tt.want {
				t.Errorf("HasPermission(%d, %d) = %v, want %v", tt.perm, tt.required, got, tt.want)
			}
		})
	}
}

func TestIsModerator(t *testing.T) {
	tests := []struct {
		name string
		perm uint64
		want bool
	}{
		{"no perms", 0, false},
		{"cm only", PermissionField["CM"], false},
		{"kick only", PermissionField["KICK"], true},
		{"cm and kick", PermissionField["CM"] | PermissionField["KICK"], true},
		{"ban", PermissionField["BAN"], true},
		{"admin", math.MaxUint64, true},
		{"mute", PermissionField["MUTE"], true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsModerator(tt.perm)
			if got != tt.want {
				t.Errorf("IsModerator(%d) = %v, want %v", tt.perm, got, tt.want)
			}
		})
	}
}
