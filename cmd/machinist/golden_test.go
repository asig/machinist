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
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asig/machinist/internal/build"
	"github.com/asig/machinist/internal/config"
)

var updateGolden = flag.Bool("update", false, "rewrite golden expected.args files")

func TestGoldenConfigs(t *testing.T) {
	testdataRoot := filepath.Join("..", "..", "testdata")
	cases, err := os.ReadDir(testdataRoot)
	if err != nil {
		t.Fatalf("read testdata directory: %v", err)
	}
	for _, entry := range cases {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			runGoldenCase(t, filepath.Join(testdataRoot, entry.Name()))
		})
	}
}

func runGoldenCase(t *testing.T, caseDir string) {
	t.Helper()

	nameData, err := os.ReadFile(filepath.Join(caseDir, "name"))
	if err != nil {
		t.Fatalf("read config name: %v", err)
	}
	name := strings.TrimSuffix(string(nameData), "\n")

	configRoot := filepath.Join(caseDir, "config")
	result, err := config.Load(configRoot, name)
	if expectedErr, readErr := readOptional(filepath.Join(caseDir, "expected.err")); readErr != nil {
		t.Fatalf("read expected error: %v", readErr)
	} else if expectedErr != nil {
		if err == nil {
			t.Fatalf("Load returned nil error, want substring %q", strings.TrimSpace(*expectedErr))
		}
		if !strings.Contains(err.Error(), strings.TrimSpace(*expectedErr)) {
			t.Fatalf("Load error = %q, want substring %q", err, strings.TrimSpace(*expectedErr))
		}
		return
	}
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	expectedWarnings, err := readOptional(filepath.Join(caseDir, "expected.warn"))
	if err != nil {
		t.Fatalf("read expected warnings: %v", err)
	}
	if expectedWarnings == nil {
		if len(result.Warnings) != 0 {
			t.Fatalf("Warnings = %#v, want none", result.Warnings)
		}
	} else if got, want := result.Warnings, expandCaseDir(readLines(*expectedWarnings), caseDir); !equalLines(got, want) {
		t.Fatalf("Warnings = %#v, want %#v", got, want)
	}

	args, err := build.CommandLine(result.Machine)
	if err != nil {
		t.Fatalf("build.CommandLine returned error: %v", err)
	}
	argsPath := filepath.Join(caseDir, "expected.args")
	if *updateGolden {
		if err := os.WriteFile(argsPath, []byte(strings.Join(args, "\n")+"\n"), 0o600); err != nil {
			t.Fatalf("rewrite expected arguments: %v", err)
		}
	}
	expectedArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read expected arguments: %v", err)
	}
	if got, want := args, readLines(string(expectedArgs)); !equalLines(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func readOptional(path string) (*string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value := string(data)
	return &value, nil
}

func readLines(value string) []string {
	value = strings.TrimRight(value, "\n")
	if value == "" {
		return nil
	}
	lines := strings.Split(value, "\n")
	return lines
}

func equalLines(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func expandCaseDir(lines []string, caseDir string) []string {
	expanded := make([]string, len(lines))
	for i, line := range lines {
		expanded[i] = strings.ReplaceAll(line, "${CASE_DIR}", caseDir)
	}
	return expanded
}

func TestGoldenCasesHaveExpectedFiles(t *testing.T) {
	testdataRoot := filepath.Join("..", "..", "testdata")
	cases, err := os.ReadDir(testdataRoot)
	if err != nil {
		t.Fatalf("read testdata directory: %v", err)
	}
	for _, entry := range cases {
		if !entry.IsDir() {
			continue
		}
		caseDir := filepath.Join(testdataRoot, entry.Name())
		namePath := filepath.Join(caseDir, "name")
		if _, err := os.Stat(namePath); err != nil {
			t.Fatalf("%s missing: %v", namePath, err)
		}

		errPath := filepath.Join(caseDir, "expected.err")
		argsPath := filepath.Join(caseDir, "expected.args")
		_, errStatErr := os.Stat(errPath)
		_, errStatArgs := os.Stat(argsPath)
		hasErr := errStatErr == nil
		hasArgs := errStatArgs == nil
		if !hasErr && !hasArgs {
			t.Fatalf("%s must contain either expected.err or expected.args", caseDir)
		}
		if hasErr && hasArgs {
			t.Fatalf("%s must not contain both expected.err and expected.args", caseDir)
		}
		if !hasErr && !*updateGolden {
			if _, err := os.Stat(argsPath); err != nil {
				t.Fatalf("%s missing expected.args: %v", caseDir, err)
			}
		}
		if hasErr && *updateGolden {
			t.Logf("skipping %s update due to expected.err", caseDir)
		}
		if !hasErr && hasArgs {
			contents, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatalf("read %s: %v", argsPath, err)
			}
			if len(contents) > 0 && !strings.HasSuffix(string(contents), "\n") {
				t.Fatalf("%s must end with a newline", argsPath)
			}
		}
	}
}
