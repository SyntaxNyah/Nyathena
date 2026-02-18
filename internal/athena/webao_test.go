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

package athena

import (
	"reflect"
	"testing"
)

func TestGetAllowedOrigins(t *testing.T) {
	tests := []struct {
		name           string
		assetURL       string
		expectedResult []string
	}{
		{
			name:           "No custom asset URL - only default allowed",
			assetURL:       "",
			expectedResult: []string{"web.aceattorneyonline.com"},
		},
		{
			name:           "Custom asset URL with HTTPS",
			assetURL:       "https://custom.example.com/assets",
			expectedResult: []string{"web.aceattorneyonline.com", "custom.example.com"},
		},
		{
			name:           "Custom asset URL with HTTP",
			assetURL:       "http://custom.example.com/path/to/assets",
			expectedResult: []string{"web.aceattorneyonline.com", "custom.example.com"},
		},
		{
			name:           "Custom asset URL with port",
			assetURL:       "https://custom.example.com:8080/assets",
			expectedResult: []string{"web.aceattorneyonline.com", "custom.example.com:8080"},
		},
		{
			name:           "Custom asset URL without protocol - fallback to default only",
			assetURL:       "custom.example.com",
			expectedResult: []string{"web.aceattorneyonline.com"},
		},
		{
			name:           "Invalid URL - fallback to default only",
			assetURL:       "://invalid-url",
			expectedResult: []string{"web.aceattorneyonline.com"},
		},
		{
			name:           "URL with subdomain",
			assetURL:       "https://assets.cdn.example.com/webao",
			expectedResult: []string{"web.aceattorneyonline.com", "assets.cdn.example.com"},
		},
		{
			name:           "URL with path component (miku.pizza/base/)",
			assetURL:       "https://miku.pizza/base/",
			expectedResult: []string{"web.aceattorneyonline.com", "miku.pizza"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAllowedOrigins(tt.assetURL)
			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("getAllowedOrigins(%q) = %v, want %v", tt.assetURL, result, tt.expectedResult)
			}
		})
	}
}
