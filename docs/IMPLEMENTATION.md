# Machinist: Implementation Plan

This document says how to build what `docs/SPEC.md` describes. The spec decides behavior, this file decides structure and order. If the two disagree, the spec wins and this file must be fixed.

## 1. Repository Layout

```
machinist/
  cmd/machinist/
    main.go              # main(): calls run(), os.Exit
    run.go               # run(args, stdout, stderr) int: flags, orchestration
    quote.go             # shell quoting for --dry-run
    golden_test.go       # end-to-end golden tests (config.Load + build.CommandLine)
    run_test.go          # CLI tests (flags, exit codes, dry-run output)
  internal/
    model/
      model.go           # shared types: Machine, Props
    config/
      name.go            # ValidateName
      layer.go           # decode one TOML file into a typed layer
      chain.go           # resolve parent chain, cycle and depth checks
      merge.go           # merge layers into model.Machine
      eval.go            # expand ${...} in a layer against self and the merged parent
      load.go            # Load(): public entry point
      *_test.go
    expr/
      expr.go            # expression parser and evaluator, stdlib only
      expr_test.go
    build/
      build.go           # CommandLine(): model.Machine to []string
      props.go           # key=value serialization
      *_test.go
  testdata/
    <case>/
      config/...         # config root for this case
      name               # config name to launch (one line)
      expected.args      # expected argument vector, one argument per line
      expected.err       # optional: substring of the expected error (instead of expected.args)
      expected.warn      # optional: expected warnings, one per line
  docs/
    SPEC.md
    IMPLEMENTATION.md
  .github/copilot-instructions.md
  scripts/check-headers.sh   # license header check for Go files, called by `make check`
  LICENSE                    # GPLv3 full text
  Makefile
  go.mod
```

Dependency direction (no cycles, nothing else allowed):

```
cmd/machinist --> internal/config --> internal/model
                                   --> internal/expr
              --> internal/build  --> internal/model
```

`internal/config` and `internal/build` never import each other. `internal/expr` and `internal/model` import nothing from the project.

## 2. Types

### 2.1 `internal/model`

```go
package model

// Props holds the key/value properties of one netdev, audiodev, device or drive.
// Values are string, int64 or bool. The config loader guarantees this.
type Props map[string]any

// Machine is the fully merged configuration. Empty scalar strings mean "not set".
type Machine struct {
    Binary  string // default applied by the loader: "qemu-system-x86_64"
    Machine string
    CPU     string
    SMP     string // integers are converted to their decimal string form
    Memory  string
    VGA     string
    Display string

    Netdevs   map[string]Props // keyed by id
    Audiodevs map[string]Props // keyed by id
    Devices   []Props          // file order
    Drives    []Props          // file order
    ExtraArgs []string
}
```

### 2.2 `internal/config`

```go
// Result is what Load returns.
type Result struct {
    Machine  model.Machine
    Chain    []string // config names, root ancestor first, requested config last
    BinarySource string // config that set the binary, or "default"
    Warnings []string // e.g. unknown keys; the caller decides if they are fatal
}

// Load resolves name below root, merges the parent chain and returns the result.
func Load(root, name string) (*Result, error)

// ValidateName applies the naming rules of spec section 3.8.
func ValidateName(name string) error
```

Internal types (not exported):

```go
// layer is one decoded TOML file. Nil/absent means "not set in this file".
type layer struct {
    Name, Path string
    Parent     string
    Binary     *string
    Scalars    map[string]string // machine, cpu, smp, memory, vga, display
    Netdevs    map[string]model.Props
    Audiodevs  map[string]model.Props
    Devices    []model.Props // nil = absent, non-nil empty = present and empty
    Drives     []model.Props // same rule
    ExtraArgs  []string      // same rule
    ExtraArgsExpr *string     // extra_args given as one expression string
    Raw        map[string]any // decoded TOML, exposed to expressions as "self"
    Unknown    []string      // unknown top-level keys, sorted
}
```

The nil versus empty distinction is what makes "an empty array in the child clears the parent list" work. Tests must cover it.

### 2.3 `internal/build`

```go
// CommandLine returns the complete command line: binary first, then all
// arguments in the order of spec section 3.6. It is a pure function.
func CommandLine(m model.Machine) ([]string, error)
```

Helpers in `props.go`:

```go
// formatProps serializes props as "lead,id=<id>,k=v,..." (spec 3.4).
// lead is the key whose value is emitted first without a key ("type", "driver", or "" for none).
// id is emitted second if non-empty.
func formatProps(lead, id string, p model.Props) (string, error)

func formatValue(v any) (string, error) // string (escape "," as ",,"), int64, bool (on/off); else error
```

### 2.4 `cmd/machinist`

```go
// run is main without os.Exit, so it can be tested.
func run(args []string, stdout, stderr io.Writer) int

// quote returns s shell-quoted if needed: unchanged for [A-Za-z0-9_@%+=:,./-]+,
// otherwise single-quoted with embedded ' written as '\''.
func quote(s string) string
```

Flags use the standard `flag` package. Register short and long names for the same variable. Flags must come before the config name (as in the synopsis). Wrong usage returns exit code 2.

## 3. Algorithms

### 3.1 Loading a layer (`layer.go`)

1. Read `<root>/<name>.toml`. Missing root: error naming the root. Missing file: error naming name and full path.
2. `toml.Unmarshal` into `map[string]any`. On `*toml.DecodeError` report file and line (`Position()`).
3. Walk the top-level keys:
   - `parent`: string
   - `binary`: non-empty string
   - `machine`, `cpu`, `memory`, `vga`, `display`, `smp`: string or int64, stored as string
   - `netdev`, `audiodev`: table of tables, key is the id
   - `device`, `drive`: array of tables
   - `extra_args`: array of strings, or one string that is an expression yielding a list
   - anything else: collected in `Unknown` (sorted)
4. Every props table is validated: values must be string, int64 or bool. Float, datetime, array and nested table are errors naming file and key path.
5. Leading key (`type` for netdev, `driver` for audiodev and device) must exist and be a string.

### 3.2 Chain resolution (`chain.go`)

Iterative, not recursive:

1. Start with the requested name. Validate it with `ValidateName`.
2. Load the layer, append it, validate its `parent` (if any) with `ValidateName`.
3. Keep a visited set keyed by normalized name. A repeat is a cycle: report the path (`a -> b -> a`).
4. Stop with an error when the chain has more than 16 layers.
5. A missing parent reports the child that referenced it.
6. Reverse the slice so the root ancestor comes first.

### 3.3 Merge (`merge.go`)

Start from a zero `model.Machine`. For each layer, root first:

- `Binary` and scalars: replace if set in the layer.
- `Netdevs` and `Audiodevs`: for each id in the layer, replace the whole entry.
- `Devices`, `Drives`, `ExtraArgs`: replace the whole list if the layer's slice is non-nil.

Before a layer is merged, `evalLayer` (`eval.go`) expands its `${...}` expressions with `self` (the layer's raw values) and `parent` (the machine merged so far, scalars that are canonical decimal integers exposed as integers). Props values keep the type of a whole-string expression, `extra_args` items yielding lists are spliced, and `ExtraArgsExpr` must yield a list. Errors name layer, file and key path.

After the last layer: if `Binary` is empty, set `qemu-system-x86_64`. Collect warnings from `Unknown` as `<path>: unknown key "<key>"`.

### 3.3.1 Expressions (`internal/expr`)

`expr.Eval(template, env)` scans for `${`, parses with a hand-written recursive descent parser (grammar in the package doc, user-facing rules in spec 3.9) and returns `any` (int64, string, bool, `[]any`, `map[string]any`). A template that is exactly one placeholder returns the typed value, anything else a string. `$${` is the escape for a literal `${`. The package has no knowledge of config types.

### 3.4 Build (`build.go`)

Order exactly as spec 3.6:

1. `Binary`
2. `-machine`, `-cpu`, `-smp`, `-m`, `-vga`, `-display`, only if non-empty
3. `-netdev` for ids in sorted order, lead `type`, id from the key
4. `-audiodev` for ids in sorted order, lead `driver`, id from the key
5. `-device` in list order, lead `driver`, id from props `id` if present
6. `-drive` in list order, no lead, id from props `id` if present
7. `ExtraArgs` verbatim

Inside `formatProps`: emit the lead value first (value only, no `key=`), then `id=...`, then all remaining keys sorted alphabetically, excluding the lead key and `id`. Never range over a map to produce output without sorting the keys first.

### 3.5 CLI flow (`run.go`)

1. Parse flags. Errors and missing config name: usage message on stderr, return 2.
2. `config.Load(root, name)`. Error: print to stderr, return 1.
3. Print each warning to stderr. With `--strict`, return 1 if there were any.
4. `build.CommandLine`. Error: stderr, return 1.
5. `--verbose`: print chain and resolved settings to stderr. The format is free-form and not covered by golden tests.
6. `--dry-run`: print the quoted command line on one line to stdout, return 0. The binary is printed as configured, not resolved.
7. Resolve the binary: `exec.LookPath` if it has no `/`, otherwise check that the file exists and is executable. Not found: stderr, return 3.
8. `syscall.Exec(path, argv, os.Environ())` with `argv[0] = path` and the remaining arguments from the command line. If it returns, report the error and return 1.

## 4. Decisions Not Covered by the Spec

These are choices the implementation makes. If you agree, move them into `docs/SPEC.md`.

1. An empty string for a scalar (`memory = ""`) is an error.
2. An `id` key inside `[netdev.<id>]` or `[audiodev.<id>]` is an error, because the table key is the id.
3. Arrays as property values are errors, like nested tables.
4. Duplicate keys and other TOML syntax errors are reported by the parser as invalid TOML with file and line.
5. `--verbose` output format is not specified and may change.
6. `self` is not expanded: it exposes the raw values of the current file, so expressions never depend on evaluation order within one file.
7. Hyphens are not allowed in expression names, use `['net-0']` for such ids.

## 5. Tooling

`Makefile` targets:

- `build`: `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/machinist ./cmd/machinist`
- `test`: `go test ./...`
- `check`: `gofmt -l .` must print nothing, then `go vet ./...`, then `scripts/check-headers.sh` (section 8)
- `static`: build, then verify the binary is static (`ldd bin/machinist` reports "not a dynamic executable")
- `cross`: build for `linux/arm64` to make sure cross-compilation keeps working

Golden tests support `go test ./cmd/machinist -update` to rewrite `expected.args` from the current output. Review the diff before committing.

## 6. Build Order

One step is one commit. Every commit must pass `make check test`.

| Step | Content | Done when |
|------|---------|-----------|
| 1 | Scaffold: `LICENSE`, `scripts/check-headers.sh`, `go.mod`, directories, `Makefile`, `.gitignore`, `main.go` that prints the version. Go files with headers (section 8) | `make check build static` works |
| 2 | `internal/model` types, `config.ValidateName` | table-driven tests cover every rule in spec 3.8 |
| 3 | `config/layer.go`: decode and validate one TOML file | tests for every type error, unknown keys, parse error with line |
| 4 | `config/chain.go`: chain resolution | tests for cycle, depth limit, missing parent, parent in other subdirectory |
| 5 | `config/merge.go`, `config/load.go` | tests for scalar override, netdev replace by id, list replace, empty list clears, binary default and inheritance |
| 5a | `internal/expr`, `config/eval.go` | table tests for operators, precedence, types and every error; config tests for `self`/`parent`, splicing, type preservation |
| 6 | `build/props.go`: value formatting and property ordering | tests for comma escaping, bool, int, lead key, id, sorting, errors |
| 7 | `build/build.go`: command line assembly | unit tests with literal `model.Machine` values; `extra_args` is last |
| 8 | Golden test harness and cases for all mandatory cases in spec section 9 | `make test` green, every case has `expected.args` or `expected.err` |
| 9 | `cmd/machinist`: flags, `quote`, dry-run, verbose, strict | CLI tests for exit codes 0, 1, 2, stdout only contains the command line |
| 10 | exec path and exit code 3 | test with a missing binary; manual run with a real QEMU |
| 11 | Review pass: go through `docs/SPEC.md` section by section and check each rule has a test | list of gaps is empty or fixed |

## 7. Notes for the Agent

- Do not start a step before the previous one is committed.
- Do not change the types in section 2 without asking. Other steps depend on them.
- If a test needs behavior that is not in the spec or in section 4, stop and ask.
- Keep functions small. `chain.go` and `merge.go` should each stay under about 150 lines.

## 8. License Headers

Machinist is licensed under the GNU General Public License, version 3 or (at your option) any later version. The full text is in `LICENSE`. Every Go file carries the long-form header below as its first content. Other file types carry no header.

### 8.1 Go files

```go
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
```

Exactly one blank line between the header and `package`. Without it, the header would become the package doc comment.

### 8.2 Other file types

Only `*.go` files carry the header. Markdown, `Makefile`, shell scripts, `.gitignore`, `go.mod`, `go.sum`, TOML configs, everything below `docs/` and everything below `testdata/` have none. They are covered by the `LICENSE` of the repository.

### 8.3 Check

`scripts/check-headers.sh` lists all tracked and untracked, non-ignored `*.go` files (`git ls-files --cached --others --exclude-standard -- '*.go'`) and fails with a list of every file in which the first 30 lines do not contain the line `This file is part of Machinist.`. It is part of `make check`.

Header text is never edited per file. A year change or a new copyright holder is a repository-wide search and replace.