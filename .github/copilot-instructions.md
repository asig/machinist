# Copilot Instructions for Machinist

Machinist is a QEMU launcher written in Go. It starts QEMU from named, hierarchical TOML configs. The binary is called `machinist`.

## Source of truth

- `docs/SPEC.md` defines the behavior. Read the relevant section before implementing anything.
- `docs/IMPLEMENTATION.md` defines package structure, types and build order. Follow it.
- If the spec is ambiguous or contradicts itself, stop and ask. Do not guess and do not edit the spec on your own.
- Do not add flags, config keys or features that are not in the spec.

## Hard rules

- Language: Go, standard library only, plus exactly one third-party dependency: a TOML library (`github.com/pelletier/go-toml/v2`). Do not add other dependencies.
- The binary must be fully static: build with `CGO_ENABLED=0`. Do not use cgo or anything that requires it.
- Target: Linux amd64. Cross-compilation must keep working.
- Execution of QEMU uses `syscall.Exec`, not a child process.
- There is no command-line option for the QEMU binary. It comes from the `binary` config key (default `qemu-system-x86_64`).
- The config root defaults to `config` (relative to the working directory) and is set with `-c` / `--config-dir`.

## Architecture

- `cmd/machinist`: flag parsing, dry-run, verbose output, exec. No business logic.
- `internal/model`: shared types only (`Machine`, `Props`). Imports nothing from the project.
- `internal/config`: loading, config name validation, parent chain resolution, cycle detection, merge. Produces a fully merged `model.Machine`.
- `internal/build`: pure function from `model.Machine` to `[]string`. No file system access, no environment access, no imports of `internal/config`.
- Keep packages small and independent so each can be tested alone.

## Determinism

- The generated argument vector must be identical for identical input. Never iterate over a Go map to produce output. Sort keys explicitly as defined in the spec.
- Lists (`device`, `drive`, `extra_args`) keep file order. `netdev` and `audiodev` are sorted by id. `extra_args` always come last.

## Error handling

- Return errors with context (config name, file path, key path, line number for TOML errors). Wrap with `fmt.Errorf("...: %w", err)`.
- Use the exit codes from the spec. Warnings and verbose output go to stderr. Only the dry-run command line goes to stdout.
- Never panic on user input.

## Testing

- Every behavior in the spec needs a test. Run `go test ./...` before every commit and keep it green.
- Use golden-file tests: `testdata/<case>/` contains the config files, the config name to launch and `expected.args` (one argument per line). Test the pure build function, never start QEMU in tests.
- Cover all mandatory cases listed in section 9 of the spec.
- Run `gofmt` and `go vet ./...`. Code must be clean before committing.

## Style

- Idiomatic, simple Go. Prefer small functions and explicit types over abstraction. No interfaces without at least two implementations or a test need.
- Comments explain why, not what. Exported identifiers get doc comments.
- No em dashes in code, comments, docs or commit messages. Use commas, colons or hyphens.

## License

- Machinist is licensed under GPLv3 or later. The full text is in `LICENSE`.
- Every new Go file gets the long-form license header defined in section 8 of `docs/IMPLEMENTATION.md`, as the very first content of the file. Copy it exactly, including the project name "Machinist" and the copyright line.
- Go files use the `/* ... */` form, followed by one blank line before `package`.
- Only Go files carry a header. Do not add headers to any other file type (Markdown, `Makefile`, scripts, `go.mod`, TOML, `docs/`, `testdata/`).
- `make check` verifies the headers. Do not commit a file that fails it.

## Workflow

- Work in the order given in `docs/IMPLEMENTATION.md`.
- One logical step per commit, tests included in the same commit.
- Commit messages: short imperative subject line, optional body.