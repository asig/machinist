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
	"errors"
	"fmt"
	"os"
	"strings"
)

const maxChainDepth = 16

func resolveChain(root, name string) ([]*layer, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	layers := make([]*layer, 0)
	positions := make(map[string]int)
	currentName := name
	childName := ""

	for {
		if position, exists := positions[currentName]; exists {
			cycle := make([]string, 0, len(layers)-position+1)
			for _, item := range layers[position:] {
				cycle = append(cycle, item.Name)
			}
			cycle = append(cycle, currentName)
			return nil, fmt.Errorf("circular parent chain: %s", strings.Join(cycle, " -> "))
		}
		if len(layers) >= maxChainDepth {
			return nil, fmt.Errorf("config %q: parent chain exceeds maximum depth of %d", name, maxChainDepth)
		}

		current, err := loadLayer(root, currentName)
		if err != nil {
			if childName != "" && errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("config %q references missing parent %q: %w", childName, currentName, err)
			}
			return nil, err
		}

		positions[currentName] = len(layers)
		layers = append(layers, current)
		if current.Parent == "" {
			break
		}
		childName = current.Name
		currentName = current.Parent
	}

	for left, right := 0, len(layers)-1; left < right; left, right = left+1, right-1 {
		layers[left], layers[right] = layers[right], layers[left]
	}
	return layers, nil
}
