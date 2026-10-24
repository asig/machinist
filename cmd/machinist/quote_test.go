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

package main

import "testing"

func TestQuote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "qemu-system-x86_64", want: "qemu-system-x86_64"},
		{in: "-m", want: "-m"},
		{in: "path/with space", want: "'path/with space'"},
		{in: "", want: "''"},
		{in: "a'b", want: "'a'\\''b'"},
	}
	for _, tt := range tests {
		if got := quote(tt.in); got != tt.want {
			t.Errorf("quote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
