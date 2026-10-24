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

package model

// Props contains the key/value properties of one QEMU object.
// Values are strings, int64 values, or booleans.
type Props map[string]any

// Machine is the fully merged QEMU configuration.
type Machine struct {
	// Binary selects the QEMU executable.
	Binary string

	// Machine sets the QEMU machine type.
	Machine string
	// CPU sets the QEMU CPU model.
	CPU string
	// SMP sets the number of CPUs as a decimal string.
	SMP string
	// Memory sets the amount of guest memory.
	Memory string
	// VGA sets the VGA adapter.
	VGA string
	// Display sets the display backend.
	Display string

	// Netdevs contains network backends keyed by id.
	Netdevs map[string]Props
	// Audiodevs contains audio backends keyed by id.
	Audiodevs map[string]Props
	// Devices contains device properties in configuration order.
	Devices []Props
	// Drives contains drive properties in configuration order.
	Drives []Props
	// ExtraArgs contains additional arguments in configuration order.
	ExtraArgs []string
}
