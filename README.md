# go-file-arch

`go-file-arch` is a Go analyzer for architecture rules. It is intended to grow into a reusable checker for both dependency boundaries and AST-level file content boundaries.

For now, it complements `go-arch-lint`. Do not remove `go-arch-lint` until `go-file-arch` reaches dependency-rule parity.

## Install

```sh
go install github.com/cavos-io/go-file-arch/cmd/go-file-arch@v0.0.1
```

Private repo, so set `GOPRIVATE=github.com/cavos-io/*` and a git token first (the `goget.sh` helper in consuming apps does this). Then the binary is on `$GOBIN`/`PATH`. To wire it into another Go app, add the install line to that app's `make install` target and drop a pre-commit hook (see below).

## Usage

```sh
# Explicit config (short or long flag)
go-file-arch -c .go-file-arch.yml ./...
go-file-arch --config .go-file-arch.yml ./...

# No -c: auto-discovers .go-file-arch.yml or .go-file-arch.yaml in the current directory
go-file-arch ./...

# Show help
go-file-arch -h
```

Flags:

| Short | Long | Meaning |
|-------|------|---------|
| `-c` | `--config` | path to the `go-file-arch` YAML config |
| `-a` | `--arch-lint-config` | optional `go-arch-lint` YAML config to run alongside |
| `-h` | `--help` | show usage |

When `-c` is omitted, the tool looks for `.go-file-arch.yml` then `.go-file-arch.yaml` in the current directory. If neither exists it exits with `config not found`. The `-arch-lint-config` file is **never** auto-discovered — it stays opt-in and only runs when passed explicitly.

Exit codes: `0` no violations, `1` violations found or an error occurred.

The pre-commit hook runs the same command with `language: system` and `pass_filenames: false`:

```yaml
repos:
  - repo: local
    hooks:
      - id: go-file-arch
        name: Go file architecture rules
        entry: go-file-arch -config .go-file-arch.yml ./...
        language: system
        pass_filenames: false
        stages: [pre-commit]
        types:
          - go
```

The root `.go-file-arch.yml` dogfoods this repository with `cmd/**` and `filearch/**` components. Layered application examples are shown below and in test fixtures.

To run copied `go-arch-lint` checks alongside `contentRules` and `fileNameRules`, pass a go-arch-lint config too:

```sh
go-file-arch -c .go-file-arch.yml -a .go-arch-lint.yml ./...
```

The library also exposes `CheckArchLint` for structured check results and `RunArchLintCLI` for copied go-arch-lint CLI commands such as `check`, `mapping`, `graph`, `schema`, `version`, and `selfInspect`.

## Config

The YAML format is versioned (`version: 1`) and has five sections:

- `components`: named package/file areas.
- `commonComponents`: components every other component may import without listing them.
- `dependencyRules`: component dependency policy for local imports.
- `contentRules`: AST declaration rules enforced by the analyzer.
- `fileNameRules`: file name rules that may be triggered by file-level or declaration-level conditions.

`workdir` is optional. When set, component, content, file-name, and dependency path matching are relative to that directory.

Example content rule:

```yaml
contentRules:
  - id: core-repository-interface-only
    files:
      include:
        - core/**/repository.go
      exclude:
        - "**/*_test.go"
    allow:
      declarations:
        - interface
    deny:
      declarations:
        - struct
        - func
        - var
        - const
    message: "core repository files may only define interfaces. Move implementations to adapter/**."
```

Supported declaration kinds are `interface`, `struct`, `func`, `var`, `const`, and `type`. Package and import declarations are allowed by default.

File matching uses slash-normalized glob patterns with `**` support, such as `**/*.go`, `core/**/repository.go`, and `core/**/model/**/*.go`.

## File Name Rules

`fileNameRules` run after path include/exclude matching. If `when` is omitted, the rule checks the file name and reports at file start. If `when.declarations` or `when.typeNameRegex` is present, the rule only applies when matching declarations exist and reports at the triggering declaration.

```yaml
fileNameRules:
  - id: dto-file-name
    files:
      include:
        - interface/**/*.go
      exclude:
        - "**/*_test.go"
    when:
      declarations:
        - struct
      typeNameRegex:
        - ".*DTO$"
        - ".*Dto$"
        - ".*Request$"
        - ".*Response$"
    require:
      fileName:
        matches:
          - ".*_dto\\.go$"
          - "dto\\.go$"
          - "request\\.go$"
          - "response\\.go$"
    message: "DTO/request/response structs must be placed in dto.go, *_dto.go, request.go, or response.go."
```

Supported file name matchers are `equals`, `equalsAny`, `matches`, `prefix`, and `suffix`. `require.fileName` means at least one configured matcher must match. `deny.fileName` means no configured matcher may match.

## Dependency Rules

`dependencyRules` are parsed, validated, and enforced for local imports that can be mapped to configured components. The loader validates that every rule key and every `mayDependOn` target names a configured component, and that each component `in` pattern matches at least one existing path under the config directory or `workdir`, so malformed policies fail early.

If multiple component globs match a path, `go-file-arch` chooses the most specific component using longer literal prefix, more literal pattern bytes, fewer wildcards, then longer pattern length. This makes patterns such as `interface/**/dto/**` win over `interface/**`.

## Limitations

- Dependency rules ignore external imports and local imports that do not match any configured component.
- Content rules inspect syntax only; they do not use type checking or semantic ownership rules.
- File name rules use Go regular expressions and match only the base file name, not the full path.
- Type declarations are classified by AST shape: interfaces are `interface`, structs are `struct`, and other type declarations are `type`.
- Rules report matching declarations independently, so one declaration may produce diagnostics from multiple rules.

## go-arch-lint Parity Status

`go-file-arch` vendors the go-arch-lint engine under `internal/archlint` with the original MIT license notice. The copied engine is available from the public `filearch` package and can be run alongside `contentRules` and `fileNameRules`.

Native dependency behavior still implements a smaller subset: missing component path validation, workdir-relative component matching, component specificity, local component dependency checks, and `commonComponents`. Full replacement still requires output comparison against `go-arch-lint` on a real consuming repository.
