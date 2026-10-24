# Machinist

Machinist is a small QEMU launcher. It starts QEMU from named TOML config files that can inherit from a parent config, so common settings live in one place and each machine only states what differs.

- Plain-text configs with comments
- Single-parent inheritance with simple override rules
- Deterministic mapping from config to QEMU arguments
- One static binary (Linux amd64), no runtime dependencies

## Build

Requires Go 1.23 or later.

```
make build      # produces bin/machinist
make test       # unit and golden tests
make check      # gofmt, go vet, license header check
make static     # build and verify the binary is static
make cross      # cross-compile for linux/arm64
```

## Usage

```
machinist [options] <config-name>

Options:
  -c, --config-dir <path>   Config root directory (default: config)
  -n, --dry-run             Print the final QEMU command line and exit
  -v, --verbose             Print the resolved configuration to stderr
      --strict              Treat unknown keys and warnings as fatal errors
  -h, --help                Show help
      --version             Show version
```

`<config-name>` is a path relative to the config root, without the `.toml` extension. `dos/win311` loads `<config-dir>/dos/win311.toml`.

```
machinist dos622
machinist -c ~/vms/config dos/win311
machinist -n dos622
```

Without `--dry-run`, Machinist replaces itself with QEMU via `exec`, so signals and exit codes pass straight through.

### Exit codes

| Code | Meaning                                                |
|------|--------------------------------------------------------|
| 0    | Success (dry-run), or the process was replaced by QEMU |
| 1    | Configuration error                                    |
| 2    | Usage error                                            |
| 3    | QEMU binary not found                                  |

## Configuration

A config is a TOML file. See [config/dos622.toml](config/dos622.toml) for a complete example.

```toml
parent  = "base"               # optional, name relative to the config root
binary  = "qemu-system-x86_64" # optional, this is the default
machine = "pc,accel=kvm"       # -machine
cpu     = "host"               # -cpu
smp     = 2                    # -smp
memory  = "128M"               # -m
vga     = "qxl"                # -vga
display = "gtk"                # -display

extra_args = ["-monitor", "stdio"]  # appended last, verbatim

[netdev.mynet0]                # -netdev user,id=mynet0
type = "user"

[audiodev.audio0]              # -audiodev pa,id=audio0
driver = "pa"

[[device]]                     # -device ne2k_pci,netdev=mynet0
driver = "ne2k_pci"
netdev = "mynet0"

[[drive]]                      # -drive bus=0,file=disk.qcow2,if=ide,unit=0
file = "disk.qcow2"
if   = "ide"
bus  = 0
unit = 0
```

Top-level keys such as `extra_args` must appear before the first table, otherwise TOML assigns them to that table.

### Inheritance

A child config is merged on top of its parent chain (at most 16 levels):

- Scalar keys: the child replaces the parent value.
- `netdev` and `audiodev`: merged by id, a child entry replaces the parent entry as a whole.
- `device`, `drive`, `extra_args`: the child list replaces the parent list. An empty list clears it.
- A child cannot delete inherited scalars or table entries.

`parent` is always relative to the config root, never to the child's directory. Circular references are rejected.

### Output

Arguments are always emitted in this order: binary, `-machine`, `-cpu`, `-smp`, `-m`, `-vga`, `-display`, `-netdev` (sorted by id), `-audiodev` (sorted by id), `-device`, `-drive` (file order), then `extra_args`. Within one argument, the leading key (`type` or `driver`) comes first, then `id`, then the remaining keys alphabetically. Booleans become `on`/`off`, and commas in values are escaped as `,,`.

Relative paths such as `drive.file` are passed through unchanged, so they are relative to the working directory, not the config root.

## Documentation

- [docs/SPEC.md](docs/SPEC.md): behavior specification
- [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md): package structure and build order

## License

Copyright (C) 2026 Andreas Signer <asigner@gmail.com>

Machinist is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free
Software Foundation, either version 3 of the License, or (at your option)
any later version. See [LICENSE](LICENSE) for the full text.
