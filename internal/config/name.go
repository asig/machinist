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
	"fmt"
	"strings"
)

// ValidateName checks that name follows the config naming rules.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid config name %q: name is empty", name)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("invalid config name %q: name contains a NUL byte", name)
	}
	if strings.ContainsRune(name, '\\') {
		return fmt.Errorf("invalid config name %q: backslashes are not allowed", name)
	}
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("invalid config name %q: name must be relative", name)
	}
	if strings.HasSuffix(name, ".toml") {
		return fmt.Errorf("invalid config name %q: do not include the .toml extension", name)
	}

	for _, segment := range strings.Split(name, "/") {
		switch segment {
		case "":
			return fmt.Errorf("invalid config name %q: name contains an empty segment", name)
		case ".", "..":
			return fmt.Errorf("invalid config name %q: dot segments are not allowed", name)
		}
	}

	return nil
}
