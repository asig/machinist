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
	"path/filepath"
	"sort"
	"strconv"

	"github.com/asig/machinist/internal/model"
	"github.com/pelletier/go-toml/v2"
)

var scalarKeys = []string{"machine", "cpu", "smp", "memory", "vga", "display"}

type layer struct {
	Name, Path string
	Parent     string
	Binary     *string
	Scalars    map[string]string
	Netdevs    map[string]model.Props
	Audiodevs  map[string]model.Props
	Devices    []model.Props
	Drives     []model.Props
	ExtraArgs  []string
	Unknown    []string
}

func loadLayer(root, name string) (*layer, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	path := filepath.Join(root, filepath.FromSlash(name)+".toml")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config root %q does not exist", root)
		}
		return nil, fmt.Errorf("check config root %q: %w", root, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load config %q from %q: %w", name, path, err)
	}

	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		var decodeErr *toml.DecodeError
		if errors.As(err, &decodeErr) {
			row, _ := decodeErr.Position()
			return nil, fmt.Errorf("config %q (%s): line %d: invalid TOML: %w", name, path, row, err)
		}
		return nil, fmt.Errorf("config %q (%s): invalid TOML: %w", name, path, err)
	}

	result := &layer{
		Name:      name,
		Path:      path,
		Scalars:   make(map[string]string),
		Netdevs:   make(map[string]model.Props),
		Audiodevs: make(map[string]model.Props),
	}
	if err := decodeLayerValues(result, raw); err != nil {
		return nil, err
	}
	return result, nil
}

func decodeLayerValues(result *layer, raw map[string]any) error {
	known := map[string]bool{
		"parent": true, "binary": true, "netdev": true, "audiodev": true,
		"device": true, "drive": true, "extra_args": true,
	}
	for _, key := range scalarKeys {
		known[key] = true
	}

	for key := range raw {
		if !known[key] {
			result.Unknown = append(result.Unknown, key)
		}
	}
	sort.Strings(result.Unknown)

	if value, ok := raw["parent"]; ok {
		parent, ok := value.(string)
		if !ok {
			return result.valueError("parent", "must be a string")
		}
		if err := ValidateName(parent); err != nil {
			return fmt.Errorf("config %q (%s): parent: %w", result.Name, result.Path, err)
		}
		result.Parent = parent
	}

	if value, ok := raw["binary"]; ok {
		binary, ok := value.(string)
		if !ok {
			return result.valueError("binary", "must be a string")
		}
		if binary == "" {
			return result.valueError("binary", "must not be empty")
		}
		result.Binary = &binary
	}

	for _, key := range scalarKeys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch value := value.(type) {
		case string:
			result.Scalars[key] = value
		case int64:
			result.Scalars[key] = strconv.FormatInt(value, 10)
		default:
			return result.valueError(key, "must be a string or integer")
		}
	}

	var err error
	if value, ok := raw["netdev"]; ok {
		result.Netdevs, err = decodeNamedProps(result, "netdev", "type", value)
		if err != nil {
			return err
		}
	}
	if value, ok := raw["audiodev"]; ok {
		result.Audiodevs, err = decodeNamedProps(result, "audiodev", "driver", value)
		if err != nil {
			return err
		}
	}
	if value, ok := raw["device"]; ok {
		result.Devices, err = decodePropsArray(result, "device", "driver", value)
		if err != nil {
			return err
		}
	}
	if value, ok := raw["drive"]; ok {
		result.Drives, err = decodePropsArray(result, "drive", "", value)
		if err != nil {
			return err
		}
	}
	if value, ok := raw["extra_args"]; ok {
		result.ExtraArgs, err = decodeStringArray(result, "extra_args", value)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeNamedProps(result *layer, section, required string, value any) (map[string]model.Props, error) {
	tables, ok := value.(map[string]any)
	if !ok {
		return nil, result.valueError(section, "must be a table keyed by id")
	}

	propsByID := make(map[string]model.Props, len(tables))
	for id, value := range tables {
		keyPath := section + "." + id
		props, err := decodeProps(result, keyPath, value)
		if err != nil {
			return nil, err
		}
		if err := requireStringProperty(result, keyPath, required, props); err != nil {
			return nil, err
		}
		propsByID[id] = props
	}
	return propsByID, nil
}

func decodePropsArray(result *layer, section, required string, value any) ([]model.Props, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, result.valueError(section, "must be an array of tables")
	}

	propsList := make([]model.Props, 0, len(items))
	for index, item := range items {
		keyPath := fmt.Sprintf("%s[%d]", section, index)
		props, err := decodeProps(result, keyPath, item)
		if err != nil {
			return nil, err
		}
		if required != "" {
			if err := requireStringProperty(result, keyPath, required, props); err != nil {
				return nil, err
			}
		}
		propsList = append(propsList, props)
	}
	return propsList, nil
}

func decodeProps(result *layer, keyPath string, value any) (model.Props, error) {
	table, ok := value.(map[string]any)
	if !ok {
		return nil, result.valueError(keyPath, "must be a table")
	}

	props := make(model.Props, len(table))
	for key, value := range table {
		switch value.(type) {
		case string, int64, bool:
			props[key] = value
		default:
			return nil, result.valueError(keyPath+"."+key, fmt.Sprintf("unsupported value type %T", value))
		}
	}
	return props, nil
}

func requireStringProperty(result *layer, keyPath, key string, props model.Props) error {
	value, ok := props[key]
	if !ok {
		return result.valueError(keyPath+"."+key, "is required")
	}
	if _, ok := value.(string); !ok {
		return result.valueError(keyPath+"."+key, "must be a string")
	}
	return nil
}

func decodeStringArray(result *layer, keyPath string, value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, result.valueError(keyPath, "must be an array of strings")
	}
	strings := make([]string, len(items))
	for i, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, result.valueError(fmt.Sprintf("%s[%d]", keyPath, i), "must be a string")
		}
		strings[i] = value
	}
	return strings, nil
}

func (l *layer) valueError(keyPath, message string) error {
	return fmt.Errorf("config %q (%s): %s: %s", l.Name, l.Path, keyPath, message)
}
