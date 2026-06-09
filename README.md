# go-file-arch

`go-file-arch` is a Go analyzer for architecture rules. It is intended to grow into a reusable checker for both dependency boundaries and AST-level file content boundaries.

For now, it complements `go-arch-lint`. Do not remove `go-arch-lint` until `go-file-arch` reaches dependency-rule parity.

Run it from the repository root:

```sh
go run ./cmd/go-file-arch -config .go-file-arch.yml ./...
```

The pre-commit hook runs the same command with `language: system` and `pass_filenames: false`.

The root `.go-file-arch.yml` dogfoods this repository with `cmd/**` and `filearch/**` components. Layered application examples are shown below and in test fixtures.

## Config

The YAML format is versioned and has three main sections:

- `components`: named package/file areas.
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

Implemented parity behaviors include missing component path validation, workdir-relative component matching, component specificity, local component dependency checks, and `commonComponents`.

Remaining gaps include vendor rules, graph output, todo/legalization support, deep dependency-injection scanning, and full output comparison against `go-arch-lint` on a real consuming repository.
