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
	"reflect"
	"strings"
	"testing"
)

func TestLoadEvaluatesExpressions(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "base", `
memory = "1024"
cpu = "host"
extra_args = ["-S"]

[[device]]
driver = "e1000"
`)
	writeConfig(t, root, "child", `
parent = "base"
memory = "${parent.memory * 2}"
cpu = "${parent.cpu}-v2"
smp = "${self.cores + 1}"
cores = 3
extra_args = "${parent.extra_args + ['-s', '-m', parent.memory]}"

[[device]]
driver = "${parent.device[0].driver}"
bus = "${1 + 1}"
rom = "${true}"
`)
	writeConfig(t, root, "grandchild", `
parent = "child"
extra_args = ["${parent.extra_args}", "-x${1 + 2}"]
`)

	result, err := Load(root, "child")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	m := result.Machine
	if m.Memory != "2048" || m.CPU != "host-v2" || m.SMP != "4" {
		t.Errorf("scalars = %q %q %q", m.Memory, m.CPU, m.SMP)
	}
	if want := []string{"-S", "-s", "-m", "1024"}; !reflect.DeepEqual(m.ExtraArgs, want) {
		t.Errorf("ExtraArgs = %#v, want %#v", m.ExtraArgs, want)
	}
	if got := m.Devices[0]; got["driver"] != "e1000" || got["bus"] != int64(2) || got["rom"] != true {
		t.Errorf("Devices[0] = %#v", got)
	}

	result, err = Load(root, "grandchild")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if want := []string{"-S", "-s", "-m", "1024", "-x3"}; !reflect.DeepEqual(result.Machine.ExtraArgs, want) {
		t.Errorf("ExtraArgs = %#v, want %#v", result.Machine.ExtraArgs, want)
	}
}

func TestLoadReportsExpressionErrors(t *testing.T) {
	tests := []struct{ name, config, want string }{
		{"unknown property", "memory = \"${parent.memory}\"\n", `memory: column 16: unknown property "memory"`},
		{"syntax", "cpu = \"${1 +}\"\n", "cpu: column 6: expected a name"},
		{"list in scalar", "cpu = \"${[1]}\"\n", "cpu: cannot use list as a string"},
		{"extra args not list", "extra_args = \"${1}\"\n", "extra_args: expression must yield a list"},
		{"lead key not string", "[[device]]\ndriver = \"${1}\"\n", "device[0].driver: must be a string"},
		{"empty binary", "binary = \"${''}\"\n", "binary: must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, "vm", tt.config)
			_, err := Load(root, "vm")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
