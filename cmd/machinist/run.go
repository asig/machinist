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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"

	"github.com/asig/machinist/internal/build"
	"github.com/asig/machinist/internal/config"
	"github.com/asig/machinist/internal/model"
)

var (
	lookPath = exec.LookPath
	statFile = os.Stat
	doExec   = syscall.Exec
	getEnv   = os.Environ
)

func run(args []string, stdout, stderr io.Writer) int {
	configDir, dryRun, verbose, strict, showHelp, showVersion, remaining, parseErr := parseFlags(args, stderr)
	if parseErr != nil {
		return 2
	}
	if showHelp {
		printUsage(stderr)
		return 0
	}
	if showVersion {
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	}

	if len(remaining) != 1 {
		fmt.Fprintln(stderr, "usage error: exactly one <config-name> is required")
		printUsage(stderr)
		return 2
	}
	name := remaining[0]

	result, err := config.Load(configDir, name)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, warning := range result.Warnings {
		fmt.Fprintln(stderr, warning)
	}
	if strict && len(result.Warnings) > 0 {
		return 1
	}

	commandLine, err := build.CommandLine(result.Machine)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if verbose {
		fmt.Fprintf(stderr, "chain: %s\n", strings.Join(result.Chain, " -> "))
		printResolved(stderr, result)
		fmt.Fprintf(stderr, "args: %s\n", strings.Join(commandLine[1:], " "))
	}

	if dryRun {
		quoted := make([]string, len(commandLine))
		for i, arg := range commandLine {
			quoted[i] = quote(arg)
		}
		fmt.Fprintln(stdout, strings.Join(quoted, " "))
		return 0
	}

	path, err := resolveBinary(commandLine[0])
	if err != nil {
		fmt.Fprintf(stderr, "%v (set by %s)\n", err, result.BinarySource)
		return 3
	}
	argv := append([]string{path}, commandLine[1:]...)
	if err := doExec(path, argv, getEnv()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func parseFlags(args []string, stderr io.Writer) (configDir string, dryRun, verbose, strict, showHelp, showVersion bool, remaining []string, err error) {
	configDir = "config"
	flagSet := flag.NewFlagSet("machinist", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	flagSet.StringVar(&configDir, "c", "config", "")
	flagSet.StringVar(&configDir, "config-dir", "config", "")
	flagSet.BoolVar(&dryRun, "n", false, "")
	flagSet.BoolVar(&dryRun, "dry-run", false, "")
	flagSet.BoolVar(&verbose, "v", false, "")
	flagSet.BoolVar(&verbose, "verbose", false, "")
	flagSet.BoolVar(&strict, "strict", false, "")
	flagSet.BoolVar(&showHelp, "h", false, "")
	flagSet.BoolVar(&showHelp, "help", false, "")
	flagSet.BoolVar(&showVersion, "version", false, "")

	if err := flagSet.Parse(args); err != nil {
		fmt.Fprintln(stderr, "usage error:", err)
		printUsage(stderr)
		return "", false, false, false, false, false, nil, err
	}
	return configDir, dryRun, verbose, strict, showHelp, showVersion, flagSet.Args(), nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: machinist [options] <config-name>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -c, --config-dir <path>   Config root directory (default: config)")
	fmt.Fprintln(w, "  -n, --dry-run             Print command line and exit")
	fmt.Fprintln(w, "  -v, --verbose             Print resolved details to stderr")
	fmt.Fprintln(w, "      --strict              Treat warnings as fatal errors")
	fmt.Fprintln(w, "  -h, --help                Show help")
	fmt.Fprintln(w, "      --version             Show version")
}

func resolveBinary(binary string) (string, error) {
	if strings.Contains(binary, "/") {
		info, err := statFile(binary)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("QEMU binary %q not found", binary)
			}
			return "", fmt.Errorf("check QEMU binary %q: %w", binary, err)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return "", fmt.Errorf("QEMU binary %q not found", binary)
		}
		return binary, nil
	}

	resolved, err := lookPath(binary)
	if err != nil {
		return "", fmt.Errorf("QEMU binary %q not found in PATH", binary)
	}
	return resolved, nil
}

// printResolved dumps the merged configuration in a fixed order.
func printResolved(w io.Writer, result *config.Result) {
	m := result.Machine
	fmt.Fprintf(w, "binary: %s (set by %s)\n", m.Binary, result.BinarySource)
	for _, kv := range [][2]string{{"machine", m.Machine}, {"cpu", m.CPU}, {"smp", m.SMP}, {"memory", m.Memory}, {"vga", m.VGA}, {"display", m.Display}} {
		if kv[1] != "" {
			fmt.Fprintf(w, "%s: %s\n", kv[0], kv[1])
		}
	}
	printProps(w, "netdev", sortedProps(m.Netdevs))
	printProps(w, "audiodev", sortedProps(m.Audiodevs))
	printProps(w, "device", m.Devices)
	printProps(w, "drive", m.Drives)
	for _, arg := range m.ExtraArgs {
		fmt.Fprintf(w, "extra_arg: %s\n", quote(arg))
	}
}

func sortedProps(entries map[string]model.Props) []model.Props {
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]model.Props, 0, len(ids))
	for _, id := range ids {
		props := model.Props{"id": id}
		for k, v := range entries[id] {
			props[k] = v
		}
		out = append(out, props)
	}
	return out
}

func printProps(w io.Writer, label string, list []model.Props) {
	for _, props := range list {
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + displayValue(props[k])
		}
		fmt.Fprintf(w, "%s: %s\n", label, strings.Join(parts, " "))
	}
}

// displayValue prints booleans as on/off, like the generated command line.
func displayValue(v any) string {
	if b, ok := v.(bool); ok {
		if b {
			return "on"
		}
		return "off"
	}
	return fmt.Sprint(v)
}
