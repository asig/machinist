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

import "github.com/asig/machinist/internal/model"

const defaultBinary = "qemu-system-x86_64"

func mergeLayers(layers []*layer) *Result {
	machine := model.Machine{
		Netdevs:   make(map[string]model.Props),
		Audiodevs: make(map[string]model.Props),
	}
	result := &Result{}

	for _, current := range layers {
		result.Chain = append(result.Chain, current.Name)

		if current.Binary != nil {
			machine.Binary = *current.Binary
		}
		for _, key := range scalarKeys {
			if value, ok := current.Scalars[key]; ok {
				switch key {
				case "machine":
					machine.Machine = value
				case "cpu":
					machine.CPU = value
				case "smp":
					machine.SMP = value
				case "memory":
					machine.Memory = value
				case "vga":
					machine.VGA = value
				case "display":
					machine.Display = value
				}
			}
		}
		for id, props := range current.Netdevs {
			machine.Netdevs[id] = props
		}
		for id, props := range current.Audiodevs {
			machine.Audiodevs[id] = props
		}
		if current.Devices != nil {
			machine.Devices = current.Devices
		}
		if current.Drives != nil {
			machine.Drives = current.Drives
		}
		if current.ExtraArgs != nil {
			machine.ExtraArgs = current.ExtraArgs
		}
		for _, key := range current.Unknown {
			result.Warnings = append(result.Warnings, current.Path+`: unknown key "`+key+`"`)
		}
	}

	if machine.Binary == "" {
		machine.Binary = defaultBinary
	}
	result.Machine = machine
	return result
}
