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
	"fmt"
	"io"
	"strings"

	"github.com/asig/machinist/internal/build"
	"github.com/asig/machinist/internal/config"
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
		fmt.Fprintf(stderr, "binary: %s\n", result.Machine.Binary)
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
