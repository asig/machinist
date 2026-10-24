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

	"github.com/asig/machinist/internal/model"
)

// CommandLine returns the complete QEMU command line for a machine.
func CommandLine(machine model.Machine) ([]string, error) {
	if machine.Binary == "" {
		return nil, fmt.Errorf("QEMU binary is empty")
	}

	args := []string{machine.Binary}
	appendScalar := func(flag, value string) {
		if value != "" {
			args = append(args, flag, value)
		}
	}
	appendScalar("-machine", machine.Machine)
	appendScalar("-cpu", machine.CPU)
	appendScalar("-smp", machine.SMP)
	appendScalar("-m", machine.Memory)
	appendScalar("-vga", machine.VGA)
	appendScalar("-display", machine.Display)

	netdevIDs := sortedIDs(machine.Netdevs)
	for _, id := range netdevIDs {
		formatted, err := formatProps("type", id, machine.Netdevs[id])
		if err != nil {
			return nil, fmt.Errorf("netdev %q: %w", id, err)
		}
		args = append(args, "-netdev", formatted)
	}

	audiodevIDs := sortedIDs(machine.Audiodevs)
	for _, id := range audiodevIDs {
		formatted, err := formatProps("driver", id, machine.Audiodevs[id])
		if err != nil {
			return nil, fmt.Errorf("audiodev %q: %w", id, err)
		}
		args = append(args, "-audiodev", formatted)
	}

	for index, props := range machine.Devices {
		formatted, err := formatProps("driver", "", props)
		if err != nil {
			return nil, fmt.Errorf("device[%d]: %w", index, err)
		}
		args = append(args, "-device", formatted)
	}

	for index, props := range machine.Drives {
		formatted, err := formatProps("", "", props)
		if err != nil {
			return nil, fmt.Errorf("drive[%d]: %w", index, err)
		}
		args = append(args, "-drive", formatted)
	}

	args = append(args, machine.ExtraArgs...)
	return args, nil
}

func sortedIDs(objects map[string]model.Props) []string {
	ids := make([]string, 0, len(objects))
	for id := range objects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
