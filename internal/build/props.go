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

package build

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/asig/machinist/internal/model"
)

func formatProps(lead, id string, props model.Props) (string, error) {
	parts := make([]string, 0, len(props)+1)
	if lead != "" {
		value, exists := props[lead]
		if !exists {
			return "", fmt.Errorf("required property %q is missing", lead)
		}
		leadingValue, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("property %q must be a string, got %T", lead, value)
		}
		parts = append(parts, escapeCommas(leadingValue))
	}

	if id != "" {
		parts = append(parts, "id="+escapeCommas(id))
	} else if value, exists := props["id"]; exists {
		formatted, err := formatValue(value)
		if err != nil {
			return "", fmt.Errorf("property %q: %w", "id", err)
		}
		parts = append(parts, "id="+formatted)
	}

	keys := make([]string, 0, len(props))
	for key := range props {
		if key != lead && key != "id" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := formatValue(props[key])
		if err != nil {
			return "", fmt.Errorf("property %q: %w", key, err)
		}
		parts = append(parts, key+"="+value)
	}

	return strings.Join(parts, ","), nil
}

func formatValue(value any) (string, error) {
	switch value := value.(type) {
	case string:
		return escapeCommas(value), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case bool:
		if value {
			return "on", nil
		}
		return "off", nil
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}

func escapeCommas(value string) string {
	return strings.ReplaceAll(value, ",", ",,")
}
