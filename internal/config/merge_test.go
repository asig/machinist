/*
 * This file is part of Machinist.
 *
 * Copyright (C) 2026 Andreas Signer <asigner@gmail.com>
 *
 * Machinist is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License
 * as published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/asig/machinist/internal/model"
)

func TestLoadMergesScalarOverridesAndBinaryInheritance(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "base", `
binary = "qemu-base"
machine = "base-machine"
cpu = "base-cpu"
memory = "32M"
`)
	writeConfig(t, root, "child", `
parent = "base"
machine = "child-machine"
`)

	result, err := Load(root, "child")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := model.Machine{
		Binary:    "qemu-base",
		Machine:   "child-machine",
		CPU:       "base-cpu",
		Memory:    "32M",
		Netdevs:   map[string]model.Props{},
		Audiodevs: map[string]model.Props{},
	}
	if !reflect.DeepEqual(result.Machine, want) {
		t.Errorf("Machine = %#v, want %#v", result.Machine, want)
	}
	if wantChain := []string{"base", "child"}; !reflect.DeepEqual(result.Chain, wantChain) {
		t.Errorf("Chain = %#v, want %#v", result.Chain, wantChain)
	}
}

func TestLoadBinaryDefaultsAndChildOverride(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "default", "")
		result, err := Load(root, "default")
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if got := result.Machine.Binary; got != defaultBinary {
			t.Errorf("Binary = %q, want %q", got, defaultBinary)
		}
	})

	t.Run("child override", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "base", "binary = \"qemu-base\"\n")
		writeConfig(t, root, "child", "parent = \"base\"\nbinary = \"qemu-child\"\n")
		result, err := Load(root, "child")
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if got := result.Machine.Binary; got != "qemu-child" {
			t.Errorf("Binary = %q, want qemu-child", got)
		}
	})
}

func TestLoadReplacesNamedPropertiesByID(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "base", `
[netdev.shared]
type = "user"
hostfwd = "tcp::2222-:22"

[netdev.parent]
type = "tap"
`)
	writeConfig(t, root, "child", `
parent = "base"

[netdev.shared]
type = "socket"
connect = "host"
`)

	result, err := Load(root, "child")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	wantShared := model.Props{"type": "socket", "connect": "host"}
	if got := result.Machine.Netdevs["shared"]; !reflect.DeepEqual(got, wantShared) {
		t.Errorf("Netdevs[shared] = %#v, want %#v", got, wantShared)
	}
	wantParent := model.Props{"type": "tap"}
	if got := result.Machine.Netdevs["parent"]; !reflect.DeepEqual(got, wantParent) {
		t.Errorf("Netdevs[parent] = %#v, want %#v", got, wantParent)
	}
}

func TestLoadReplacesListsAndClearsWithEmptyLists(t *testing.T) {
	tests := []struct {
		name, parent, child string
		check               func(*testing.T, model.Machine)
	}{
		{
			name:   "device replacement",
			parent: "[[device]]\ndriver = \"parent\"\n",
			child:  "parent = \"base\"\n[[device]]\ndriver = \"child\"\n",
			check: func(t *testing.T, m model.Machine) {
				if len(m.Devices) != 1 || m.Devices[0]["driver"] != "child" {
					t.Errorf("Devices = %#v, want child entry", m.Devices)
				}
			},
		},
		{
			name:   "drive replacement",
			parent: "[[drive]]\nfile = \"parent.qcow2\"\n",
			child:  "parent = \"base\"\n[[drive]]\nfile = \"child.qcow2\"\n",
			check: func(t *testing.T, m model.Machine) {
				if len(m.Drives) != 1 || m.Drives[0]["file"] != "child.qcow2" {
					t.Errorf("Drives = %#v, want child entry", m.Drives)
				}
			},
		},
		{
			name:   "extra args replacement",
			parent: "extra_args = [\"-S\"]\n",
			child:  "parent = \"base\"\nextra_args = [\"-s\"]\n",
			check: func(t *testing.T, m model.Machine) {
				if !reflect.DeepEqual(m.ExtraArgs, []string{"-s"}) {
					t.Errorf("ExtraArgs = %#v, want [-s]", m.ExtraArgs)
				}
			},
		},
		{
			name:   "empty lists clear parent",
			parent: "extra_args = [\"-S\"]\n\n[[device]]\ndriver = \"parent\"\n\n[[drive]]\nfile = \"parent.qcow2\"\n",
			child:  "parent = \"base\"\ndevice = []\ndrive = []\nextra_args = []\n",
			check: func(t *testing.T, m model.Machine) {
				if m.Devices == nil || len(m.Devices) != 0 {
					t.Errorf("Devices = %#v, want non-nil empty slice", m.Devices)
				}
				if m.Drives == nil || len(m.Drives) != 0 {
					t.Errorf("Drives = %#v, want non-nil empty slice", m.Drives)
				}
				if m.ExtraArgs == nil || len(m.ExtraArgs) != 0 {
					t.Errorf("ExtraArgs = %#v, want non-nil empty slice", m.ExtraArgs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, "base", tt.parent)
			writeConfig(t, root, "child", tt.child)
			result, err := Load(root, "child")
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			tt.check(t, result.Machine)
		})
	}
}

func TestLoadReportsUnknownKeysWithConfigPaths(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "base", "z_unknown = 1\na_unknown = 2\n")
	writeConfig(t, root, "child", "parent = \"base\"\nchild_unknown = 3\n")

	result, err := Load(root, "child")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := []string{
		filepath.Join(root, "base.toml") + `: unknown key "a_unknown"`,
		filepath.Join(root, "base.toml") + `: unknown key "z_unknown"`,
		filepath.Join(root, "child.toml") + `: unknown key "child_unknown"`,
	}
	if !reflect.DeepEqual(result.Warnings, want) {
		t.Errorf("Warnings = %#v, want %#v", result.Warnings, want)
	}
}
