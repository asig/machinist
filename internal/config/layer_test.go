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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadLayer(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "dos/win311", `
parent = "base"
binary = "qemu"
machine = "pc"
smp = 2
empty = "unrecognized"
extra_args = ["-S", "-s"]

[netdev.user]
type = "user"
queues = 4
enabled = true

[audiodev.audio]
driver = "pa"

[[device]]
driver = "virtio-net"
id = "net0"

[[drive]]
file = "disk.qcow2"
bus = 0
`)

	layer, err := loadLayer(root, "dos/win311")
	if err != nil {
		t.Fatalf("loadLayer returned error: %v", err)
	}
	if layer.Name != "dos/win311" {
		t.Errorf("Name = %q, want dos/win311", layer.Name)
	}
	if layer.Path != filepath.Join(root, "dos", "win311.toml") {
		t.Errorf("Path = %q, want config file path", layer.Path)
	}
	if layer.Parent != "base" {
		t.Errorf("Parent = %q, want base", layer.Parent)
	}
	if layer.Binary == nil || *layer.Binary != "qemu" {
		t.Errorf("Binary = %v, want qemu", layer.Binary)
	}
	if !reflect.DeepEqual(layer.Scalars, map[string]string{"machine": "pc", "smp": "2"}) {
		t.Errorf("Scalars = %#v", layer.Scalars)
	}
	if got := layer.Netdevs["user"]; got["type"] != "user" || got["queues"] != int64(4) || got["enabled"] != true {
		t.Errorf("Netdevs[user] = %#v", got)
	}
	if got := layer.Audiodevs["audio"]; got["driver"] != "pa" {
		t.Errorf("Audiodevs[audio] = %#v", got)
	}
	if len(layer.Devices) != 1 || layer.Devices[0]["driver"] != "virtio-net" {
		t.Errorf("Devices = %#v", layer.Devices)
	}
	if len(layer.Drives) != 1 || layer.Drives[0]["bus"] != int64(0) {
		t.Errorf("Drives = %#v", layer.Drives)
	}
	if !reflect.DeepEqual(layer.ExtraArgs, []string{"-S", "-s"}) {
		t.Errorf("ExtraArgs = %#v", layer.ExtraArgs)
	}
	if !reflect.DeepEqual(layer.Unknown, []string{"empty"}) {
		t.Errorf("Unknown = %#v, want sorted keys", layer.Unknown)
	}
}

func TestLoadLayerAllowsEmptyScalarAndEmptyLists(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "empty", `
memory = ""
device = []
drive = []
extra_args = []
`)

	layer, err := loadLayer(root, "empty")
	if err != nil {
		t.Fatalf("loadLayer returned error: %v", err)
	}
	if value, ok := layer.Scalars["memory"]; !ok || value != "" {
		t.Errorf("memory = %q, present = %t, want present empty value", value, ok)
	}
	if layer.Devices == nil || len(layer.Devices) != 0 {
		t.Errorf("Devices = %#v, want non-nil empty slice", layer.Devices)
	}
	if layer.Drives == nil || len(layer.Drives) != 0 {
		t.Errorf("Drives = %#v, want non-nil empty slice", layer.Drives)
	}
	if layer.ExtraArgs == nil || len(layer.ExtraArgs) != 0 {
		t.Errorf("ExtraArgs = %#v, want non-nil empty slice", layer.ExtraArgs)
	}
}

func TestLoadLayerReportsUnknownKeysSorted(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "unknown", "zebra = 1\nalpha = 2\nmiddle = 3\n")

	layer, err := loadLayer(root, "unknown")
	if err != nil {
		t.Fatalf("loadLayer returned error: %v", err)
	}
	if want := []string{"alpha", "middle", "zebra"}; !reflect.DeepEqual(layer.Unknown, want) {
		t.Errorf("Unknown = %#v, want %#v", layer.Unknown, want)
	}
}

func TestLoadLayerRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name, config, want string
	}{
		{"parent type", "parent = 1\n", "parent: must be a string"},
		{"empty parent", "parent = \"\"\n", "parent: invalid config name"},
		{"invalid parent", "parent = \"../base\"\n", "parent: invalid config name"},
		{"binary type", "binary = 1\n", "binary: must be a string"},
		{"empty binary", "binary = \"\"\n", "binary: must not be empty"},
		{"scalar bool", "cpu = true\n", "cpu: must be a string or integer"},
		{"scalar float", "smp = 2.5\n", "smp: must be a string or integer"},
		{"scalar array", "memory = [\"128M\"]\n", "memory: must be a string or integer"},
		{"netdev wrong shape", "netdev = []\n", "netdev: must be a table keyed by id"},
		{"netdev required", "[netdev.n]\nmtu = 1500\n", "netdev.n.type: is required"},
		{"netdev required type", "[netdev.n]\ntype = 1\n", "netdev.n.type: must be a string"},
		{"audiodev wrong shape", "audiodev = []\n", "audiodev: must be a table keyed by id"},
		{"audiodev required", "[audiodev.a]\nfoo = \"bar\"\n", "audiodev.a.driver: is required"},
		{"device wrong shape", "device = {}\n", "device: must be an array of tables"},
		{"device entry wrong shape", "device = [\"string\"]\n", "device[0]: must be a table"},
		{"device missing driver", "[[device]]\nid = \"x\"\n", "device[0].driver: is required"},
		{"device driver wrong type", "[[device]]\ndriver = false\n", "device[0].driver: must be a string"},
		{"drive wrong shape", "drive = {}\n", "drive: must be an array of tables"},
		{"drive entry wrong shape", "drive = [1]\n", "drive[0]: must be a table"},
		{"extra args wrong shape", "extra_args = \"-S\"\n", "extra_args: must be an array of strings"},
		{"extra args item wrong type", "extra_args = [1]\n", "extra_args[0]: must be a string"},
		{"property float", "[[drive]]\nfile = 1.5\n", "drive[0].file: unsupported value type"},
		{"property array", "[[drive]]\nfile = [\"disk\"]\n", "drive[0].file: unsupported value type"},
		{"property nested table", "[[drive]]\nfile.path = \"disk\"\n", "drive[0].file: unsupported value type"},
		{"property datetime", "[[drive]]\ncreated = 1979-05-27T07:32:00Z\n", "drive[0].created: unsupported value type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, "invalid", tt.config)
			_, err := loadLayer(root, "invalid")
			if err == nil {
				t.Fatalf("loadLayer returned nil error, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("loadLayer error = %q, want substring %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), filepath.Join(root, "invalid.toml")) {
				t.Errorf("error does not name file path: %v", err)
			}
		})
	}
}

func TestLoadLayerReportsParseLine(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "invalid", "machine = \"pc\"\n[broken\n")

	_, err := loadLayer(root, "invalid")
	if err == nil {
		t.Fatal("loadLayer returned nil error, want parse error")
	}
	if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "invalid.toml") {
		t.Fatalf("loadLayer error = %q, want file path and line 2", err)
	}
}

func TestLoadLayerReportsMissingRootAndFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-root")
	_, err := loadLayer(root, "dos")
	if err == nil || !strings.Contains(err.Error(), "config root") || !strings.Contains(err.Error(), root) {
		t.Fatalf("loadLayer missing root error = %v", err)
	}

	root = t.TempDir()
	_, err = loadLayer(root, "dos/win311")
	if err == nil || !strings.Contains(err.Error(), "dos/win311") ||
		!strings.Contains(err.Error(), filepath.Join(root, "dos", "win311.toml")) {
		t.Fatalf("loadLayer missing file error = %v", err)
	}
}

func writeConfig(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name)+".toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
