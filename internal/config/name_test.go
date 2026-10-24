/*
 * This file is part of Machinist.
 *
 * Copyright (C) 2026 Andreas Signer <asigner@gmail.com>
 *
 * Machinist is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Machinist is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Machinist.  If not, see <https://www.gnu.org/licenses/>.
 */

package config

import (
	"strconv"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name        string
		wantErrText string
	}{
		{name: "dos"},
		{name: "dos/win311"},
		{name: "common/base"},
		{name: "name with spaces"},
		{name: "", wantErrText: "name is empty"},
		{name: "/absolute", wantErrText: "must be relative"},
		{name: "a//b", wantErrText: "empty segment"},
		{name: "/a", wantErrText: "must be relative"},
		{name: "a/", wantErrText: "empty segment"},
		{name: ".", wantErrText: "dot segments"},
		{name: "..", wantErrText: "dot segments"},
		{name: "a/./b", wantErrText: "dot segments"},
		{name: "a/../b", wantErrText: "dot segments"},
		{name: `a\b`, wantErrText: "backslashes"},
		{name: "a\x00b", wantErrText: "NUL byte"},
		{name: "dos.toml", wantErrText: ".toml extension"},
		{name: "a/b.toml", wantErrText: ".toml extension"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("ValidateName(%q) returned error: %v", tt.name, err)
				}
				return
			}

			if err == nil {
				t.Fatalf("ValidateName(%q) returned nil, want error containing %q", tt.name, tt.wantErrText)
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("ValidateName(%q) error = %q, want substring %q", tt.name, err, tt.wantErrText)
			}
			if !strings.Contains(err.Error(), strconv.Quote(tt.name)) {
				t.Fatalf("ValidateName(%q) error does not name the offending value: %v", tt.name, err)
			}
		})
	}
}
