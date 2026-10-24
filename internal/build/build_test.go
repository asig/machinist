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

package build

import (
	"reflect"
	"strings"
	"testing"

	"github.com/asig/machinist/internal/model"
)

func TestCommandLineOrderAndDeterminism(t *testing.T) {
	machine := model.Machine{
		Binary:  "qemu",
		Machine: "pc",
		CPU:     "host",
		SMP:     "2",
		Memory:  "128M",
		VGA:     "qxl",
		Display: "gtk",
		Netdevs: map[string]model.Props{
			"znet": {"type": "tap", "ifname": "tap0"},
			"anet": {"type": "user", "hostfwd": "tcp::2222-:22"},
		},
		Audiodevs: map[string]model.Props{
			"zaudio": {"driver": "alsa"},
			"aaudio": {"driver": "pa"},
		},
		Devices: []model.Props{
			{"driver": "virtio-net", "id": "net,0", "netdev": "anet"},
			{"driver": "virtio-mouse"},
		},
		Drives: []model.Props{
			{"file": "disk2.qcow2", "if": "ide", "unit": int64(1), "bus": int64(0)},
			{"file": "disk1.qcow2", "if": "ide", "id": "boot", "bus": int64(0)},
		},
		ExtraArgs: []string{"-S", "-s"},
	}

	got, err := CommandLine(machine)
	if err != nil {
		t.Fatalf("CommandLine returned error: %v", err)
	}
	want := []string{
		"qemu",
		"-machine", "pc",
		"-cpu", "host",
		"-smp", "2",
		"-m", "128M",
		"-vga", "qxl",
		"-display", "gtk",
		"-netdev", "user,id=anet,hostfwd=tcp::2222-:22",
		"-netdev", "tap,id=znet,ifname=tap0",
		"-audiodev", "pa,id=aaudio",
		"-audiodev", "alsa,id=zaudio",
		"-device", "virtio-net,id=net,,0,netdev=anet",
		"-device", "virtio-mouse",
		"-drive", "bus=0,file=disk2.qcow2,if=ide,unit=1",
		"-drive", "id=boot,bus=0,file=disk1.qcow2,if=ide",
		"-S", "-s",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CommandLine = %#v, want %#v", got, want)
	}

	second, err := CommandLine(machine)
	if err != nil {
		t.Fatalf("second CommandLine returned error: %v", err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("second CommandLine = %#v, want identical result %#v", second, want)
	}
}

func TestCommandLineOnlyBinaryWhenOptionsUnset(t *testing.T) {
	got, err := CommandLine(model.Machine{Binary: "qemu"})
	if err != nil {
		t.Fatalf("CommandLine returned error: %v", err)
	}
	if want := []string{"qemu"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CommandLine = %#v, want %#v", got, want)
	}
}

func TestCommandLineRejectsEmptyBinary(t *testing.T) {
	_, err := CommandLine(model.Machine{})
	if err == nil || !strings.Contains(err.Error(), "binary is empty") {
		t.Fatalf("CommandLine error = %v, want empty-binary error", err)
	}
}

func TestCommandLineReportsPropertyErrorsWithSectionContext(t *testing.T) {
	tests := []struct {
		name    string
		machine model.Machine
		want    string
	}{
		{
			name:    "netdev",
			machine: model.Machine{Binary: "qemu", Netdevs: map[string]model.Props{"n": {"type": 1}}},
			want:    `netdev "n"`,
		},
		{
			name:    "audiodev",
			machine: model.Machine{Binary: "qemu", Audiodevs: map[string]model.Props{"a": {}}},
			want:    `audiodev "a"`,
		},
		{
			name:    "device",
			machine: model.Machine{Binary: "qemu", Devices: []model.Props{{"driver": "x", "bad": 1.5}}},
			want:    "device[0]",
		},
		{
			name:    "drive",
			machine: model.Machine{Binary: "qemu", Drives: []model.Props{{"bad": 1.5}}},
			want:    "drive[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CommandLine(tt.machine)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("CommandLine error = %v, want context %q", err, tt.want)
			}
		})
	}
}
