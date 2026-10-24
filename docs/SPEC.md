# QEMU Config Launcher: Specification (v1, revised)

## 1. Overview

A small, statically linked command-line tool that launches QEMU using named configuration files.
Configurations are human-readable TOML files that support comments and single-parent inheritance with overrides.

## 2. Goals

- Named configurations stored as plain text files
- Native support for comments
- Hierarchical configuration (base + overrides)
- Simple, deterministic mapping from config values to QEMU command-line arguments
- Single statically linked binary (no external runtime dependencies)
- Minimal, easy-to-understand design

## 3. Configuration Format

- File format: TOML v1.0
- File extension: `.toml`
- Comments (`#`) supported
- UTF-8 encoding

### 3.1 Core Structure

```toml
# Optional parent configuration (name without extension, relative to the config root,
# may contain subdirectories, e.g. "common/base")
parent = "base"

# QEMU binary to execute (optional, default "qemu-system-x86_64")
binary = "qemu-system-i386"

# Scalar keys
machine = "pc,accel=kvm"   # -machine
cpu     = "host"           # -cpu
smp     = 2                # -smp
memory  = "128M"           # -m
vga     = "qxl"            # -vga
display = "gtk,zoom-to-fit=on"  # -display

# Named objects (tables keyed by id)
[netdev.mynet0]
type = "user"

[audiodev.audio0]
driver = "pa"

# Devices (ordered list, optional id)
[[device]]
driver = "ne2k_pci"
netdev = "mynet0"

[[device]]
driver = "sb16"
audiodev = "audio0"

[[device]]
driver = "virtio-mouse"

# Drives (ordered list)
[[drive]]
file = "disks/system/dos.qcow2"
if   = "ide"
bus  = 0
unit = 0

[[drive]]
file = "disks/data.qcow2"
if   = "ide"
bus  = 0
unit = 1

# Free-form extra arguments (appended last, passed verbatim)
extra_args = ["-monitor", "stdio"]
```

### 3.2 Structure Rules

- `[netdev.<id>]` and `[audiodev.<id>]` are tables keyed by id. They are never declared as arrays.
- `[[device]]` and `[[drive]]` are arrays of tables. Devices are never declared as `[device.<name>]`, so there is no table/array conflict.
- A `[[device]]` entry may contain an optional `id` key, which is emitted like any other property.
- `extra_args` is an array of strings.

### 3.3 Hierarchy Rules

- A configuration may declare at most one `parent`.
- The parent value is a config name as described in 3.8: a slash-separated path relative to the config root, without extension.
- Parent names are always resolved relative to the config root, never relative to the directory of the child config. A config in `dos/win311.toml` that wants `base.toml` in the root writes `parent = "base"`; for `dos/common.toml` it writes `parent = "dos/common"`.
- Merge semantics, applied from the root ancestor down to the requested config:
  - Scalar keys: the child value replaces the parent value.
  - Tables (`netdev`, `audiodev`): merged by id. A child entry with the same id replaces the parent entry as a whole (no per-property merge). Entries with other ids are kept.
  - Arrays (`device`, `drive`, `extra_args`): the child array replaces the parent array entirely. An empty array in the child clears the parent list.
  - A child cannot delete a scalar or table entry of its parent (see Non-Goals).
- The `parent` key itself is not inherited.
- Circular parent references are rejected with the full cycle path.
- Maximum chain depth: 16.

### 3.4 Value Serialization

Properties of `netdev`, `audiodev`, `device` and `drive` are serialized as comma-separated `key=value` pairs.

- Strings are emitted as is. A comma in a value is escaped as `,,` (QEMU rule).
- Integers are emitted in decimal.
- Booleans are emitted as `on` / `off`.
- Floats, datetimes and nested tables are errors.

Key order inside one argument is deterministic:

1. The leading key (see table below)
2. `id` (if present, or the table key for `netdev` / `audiodev`)
3. All remaining keys sorted alphabetically

| Section        | Flag        | Leading key | Example output                                    |
|----------------|-------------|-------------|---------------------------------------------------|
| `[netdev.X]`   | `-netdev`   | `type`      | `-netdev user,id=mynet0`                          |
| `[audiodev.X]` | `-audiodev` | `driver`    | `-audiodev pa,id=audio0`                          |
| `[[device]]`   | `-device`   | `driver`    | `-device ne2k_pci,netdev=mynet0`                  |
| `[[drive]]`    | `-drive`    | none        | `-drive bus=0,file=disks/data.qcow2,if=ide,unit=1`|

`type` or `driver` is required in its section; a missing leading key is an error.

### 3.5 Reserved Scalar Keys

| Key       | Flag        |
|-----------|-------------|
| `machine` | `-machine`  |
| `cpu`     | `-cpu`      |
| `smp`     | `-smp`      |
| `memory`  | `-m`        |
| `vga`     | `-vga`      |
| `display` | `-display`  |

Scalar values may be strings or integers; they are emitted via their string form.

### 3.5.1 QEMU Binary

- `binary` (string, optional) selects the QEMU executable. It is inherited and overridden like any other scalar key.
- Default if no config in the chain sets it: `qemu-system-x86_64`.
- A value without a path separator is looked up in `PATH`. A value containing `/` is used as given (absolute, or relative to the working directory, as in 3.7).
- `binary` does not produce a QEMU flag. It becomes the first element of the command line (`argv[0]` is the resolved executable path as usual).
- There is no command-line option to override the binary.

Unknown top-level keys: warning by default, fatal in strict mode.

### 3.6 Argument Order

The final command line is always built in this order:

0. The `binary` (3.5.1)
1. `-machine`, `-cpu`, `-smp`, `-m`, `-vga`, `-display` (only if set)
2. `-netdev` entries, sorted by id
3. `-audiodev` entries, sorted by id
4. `-device` entries, in file order
5. `-drive` entries, in file order
6. `extra_args`, verbatim, always last

### 3.7 Paths

Values such as `drive.file` are passed through unchanged. Relative paths are therefore relative to the working directory of the launcher, not to the config root.

### 3.8 Config Root and Config Names

- The config root is a directory, given with `-c` / `--config-dir`. Default: `config`, relative to the current working directory.
- Configs may be organized in arbitrary subdirectory hierarchies below the config root.
- A config name is a slash-separated path relative to the config root, without the `.toml` extension. Examples: `dos`, `dos/win311`, `common/base`. The file `<root>/dos/win311.toml` has the name `dos/win311`.
- The same naming rules apply to the command-line `<config-name>` and to the `parent` key.
- A name is rejected if it:
  - is empty or contains an empty segment (`a//b`, leading or trailing `/`)
  - is absolute
  - contains a `.` or `..` segment
  - contains a backslash or a NUL byte
  - ends in `.toml` (the extension is added by the tool, not given by the user)
- Validation is lexical, applied before any filesystem access. Symlinks are followed as usual and are not restricted.
- Only files with the `.toml` extension are configs. Directories are never configs themselves.
- Identity for cycle detection is the normalized config name, so each file has exactly one name.

## 4. Command-Line Interface

```
machinist [options] <config-name>

Options:
  -c, --config-dir <path>   Config root directory (default: config, relative to the current working directory)
  -n, --dry-run             Print the final QEMU command line and exit
  -v, --verbose             Print the resolved (merged) configuration and final command to stderr
      --strict              Treat unknown keys and warnings as fatal errors
  -h, --help                Show help
      --version             Show version
```

- `<config-name>` follows the naming rules in 3.8 (`dos` resolves to `<config-dir>/dos.toml`, `dos/win311` resolves to `<config-dir>/dos/win311.toml`).
- The tool resolves the inheritance chain, merges, determines the binary from the config, builds the argument vector and replaces itself with QEMU via `exec` (`syscall.Exec` on Linux), so signals and exit codes pass through directly.
- `--dry-run` prints one line to stdout, starting with the binary name as configured (not resolved against `PATH`), with each argument shell-quoted where needed. Nothing else is written to stdout.
- Warnings and verbose output go to stderr.

### 4.1 Exit Codes

| Code | Meaning                                          |
|------|--------------------------------------------------|
| 0    | Success (dry-run) or exec replaced the process   |
| 1    | Configuration error (missing file, parse, cycle, invalid value, strict violation) |
| 2    | Usage error (bad flags)                          |
| 3    | QEMU binary not found                            |

## 5. Binary Requirements

- Written in Go.
- Fully static binary (`CGO_ENABLED=0`).
- Exactly one third-party dependency: a TOML library (for example `github.com/pelletier/go-toml/v2`). Everything else from the standard library.
- Single executable, no external files needed by the tool itself.
- Primary target: Linux amd64. Cross-compilation must work.

## 6. Error Handling

- Missing config root directory: error naming the path.
- Missing config file: error naming the config name and the full path that was searched. For a missing parent, the error also names the child config that referenced it.
- Circular inheritance: error showing the cycle, for example `dos/win311 -> dos/common -> dos/win311`.
- Invalid config name (command line or `parent`): error naming the offending value and the violated rule.
- Invalid TOML: parse error with file name and line number.
- Missing `type` / `driver`, unsupported value types: error naming the config file and key path.
- Unknown keys: warning, or fatal with `--strict`.
- `binary` of the wrong type (not a string) or empty: error naming the config file and key.
- QEMU binary not found in `PATH` (or at the configured path): error naming the binary and the config that set it (or "default"), raised before execution, exit code 3. Not checked in `--dry-run`.

## 7. Non-Goals (v1)

- GUI or interactive configuration editor
- Automatic discovery of QEMU capabilities
- Deleting inherited keys or per-element list merging
- Support for every QEMU option out of the box (extensible via `extra_args`)
- Hot-reloading or daemon mode

## 8. Example Layout

```
config/
  base.toml              # common settings
  dos/
    common.toml          # parent = "base", settings shared by all DOS machines
    dos622.toml          # parent = "dos/common", memory = "64M", specific drives
    win311.toml          # parent = "dos/common", different memory/drives
  debug/
    dos622.toml          # parent = "dos/dos622", extra_args = ["-S", "-s"]
```

Usage:

```
machinist dos/win311
machinist -c ~/vms/config debug/dos622
```

Note: `debug/dos622.toml` replaces `extra_args` of `dos/dos622` completely. If the parent defines `extra_args`, the child must repeat them.

## 9. Testing Requirements

- Unit tests for config loading, merging, cycle detection and value serialization.
- Golden-file tests: each `testdata/<case>/` contains the config files, the config name to launch, and `expected.args` (one argument per line). Tests run the pure build function, no QEMU required.
- Mandatory cases: default binary, binary set in config, binary inherited from parent, binary overridden in child, invalid binary value, simple config, two-level inheritance, list replacement, empty list clearing a parent list, `netdev` replace by id, missing parent, cycle across subdirectories, parent in a different subdirectory than the child (resolved from the config root), invalid config names (`..`, absolute, empty segment, trailing `.toml`, backslash), default config root `config`, custom `--config-dir`, unknown key (warning and strict), `extra_args` ordering, comma escaping, boolean serialization.

## 10. Future Extensions (out of scope for v1)

- Explicit list merge syntax
- Environment variable expansion
- Multiple parent inheritance
- Command-line overrides (`--set memory=256M`)
- Key deletion in children