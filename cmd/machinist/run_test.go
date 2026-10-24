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

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestRunUsageErrorsReturnCode2(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "bad flag", args: []string{"--bad"}},
		{name: "missing config name", args: []string{}},
		{name: "too many args", args: []string{"one", "two"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run code = %d, want 2", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "Usage: machinist") {
				t.Fatalf("stderr = %q, want usage text", stderr.String())
			}
		})
	}
}

func TestRunVersionAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("version code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("version output = %q, want %q", got, version)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("help stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage: machinist") {
		t.Fatalf("help stderr = %q, want usage", stderr.String())
	}
}

func TestRunDryRunPrintsCommandLineOnly(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "vm", `
binary = "qemu-system-i386"
machine = "pc,accel=kvm"
extra_args = ["-monitor", "stdio with space"]
`)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config-dir", root, "--dry-run", "vm"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "qemu-system-i386 -machine pc,accel=kvm -monitor 'stdio with space'" {
		t.Fatalf("dry-run stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("dry-run stderr = %q, want empty", stderr.String())
	}
}

func TestRunReturnsConfigError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--config-dir", t.TempDir(), "missing"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "missing.toml") {
		t.Fatalf("stderr = %q, want missing config path", stderr.String())
	}
}

func TestRunStrictTreatsWarningsAsErrors(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "vm", "unknown_key = 1\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-c", root, "--strict", "vm"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("strict code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("strict stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown key "unknown_key"`) {
		t.Fatalf("strict stderr = %q, want warning", stderr.String())
	}
}

func TestRunVerbosePrintsResolvedDetails(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "base", "cpu = \"host\"\n")
	writeConfigFile(t, root, "vm", "parent = \"base\"\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-c", root, "-n", "-v", "vm"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verbose dry-run code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout.String()) != "qemu-system-x86_64 -cpu host" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for _, expected := range []string{"chain: base -> vm", "binary: qemu-system-x86_64", "args: -cpu host"} {
		if !strings.Contains(stderr.String(), expected) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), expected)
		}
	}
}

func TestRunReturnsCode3WhenBinaryMissing(t *testing.T) {
	resetRuntimeHooks(t)
	root := t.TempDir()
	writeConfigFile(t, root, "vm", "binary = \"definitely-not-installed-qemu\"\n")

	lookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"-c", root, "vm"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("missing binary code = %d, want 3", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "QEMU binary") {
		t.Fatalf("stderr = %q, want missing binary message", stderr.String())
	}
}

func TestRunExecutesResolvedCommand(t *testing.T) {
	resetRuntimeHooks(t)
	root := t.TempDir()
	writeConfigFile(t, root, "vm", "cpu = \"host\"\n")

	lookPath = func(file string) (string, error) {
		if file != "qemu-system-x86_64" {
			t.Fatalf("lookPath file = %q, want default binary", file)
		}
		return "/usr/bin/qemu-system-x86_64", nil
	}

	var gotPath string
	var gotArgv []string
	var gotEnv []string
	doExec = func(path string, argv []string, envv []string) error {
		gotPath = path
		gotArgv = append([]string{}, argv...)
		gotEnv = append([]string{}, envv...)
		return syscall.EPERM
	}
	getEnv = func() []string {
		return []string{"A=B"}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"-c", root, "vm"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exec failure code = %d, want 1", code)
	}
	if gotPath != "/usr/bin/qemu-system-x86_64" {
		t.Fatalf("exec path = %q", gotPath)
	}
	wantArgv := []string{"/usr/bin/qemu-system-x86_64", "-cpu", "host"}
	if strings.Join(gotArgv, "\n") != strings.Join(wantArgv, "\n") {
		t.Fatalf("exec argv = %#v, want %#v", gotArgv, wantArgv)
	}
	if strings.Join(gotEnv, "\n") != "A=B" {
		t.Fatalf("exec env = %#v, want A=B", gotEnv)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "operation not permitted") {
		t.Fatalf("stderr = %q, want exec error", stderr.String())
	}
}

func resetRuntimeHooks(t *testing.T) {
	t.Helper()
	origLookPath := lookPath
	origStat := statFile
	origExec := doExec
	origEnv := getEnv
	t.Cleanup(func() {
		lookPath = origLookPath
		statFile = origStat
		doExec = origExec
		getEnv = origEnv
	})
}

func writeConfigFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name)+".toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
