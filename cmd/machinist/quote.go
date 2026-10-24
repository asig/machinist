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

import (
	"regexp"
	"strings"
)

var safeArgPattern = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func quote(s string) string {
	if s != "" && safeArgPattern.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
