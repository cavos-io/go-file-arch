# go-file-arch

`go-file-arch` is a Go analyzer for architecture rules. It is intended to grow into a reusable checker for both dependency boundaries and AST-level file content boundaries.

For now, it complements `go-arch-lint`. Do not remove `go-arch-lint` until `go-file-arch` reaches dependency-rule parity.

Run it from the repository root:

```sh
go run ./cmd/go-file-arch -config .go-file-arch.yml ./...
```

The pre-commit hook runs the same command with `language: system` and `pass_filenames: false`.

## Config

The YAML format is versioned and has three main sections:

- `components`: named package/file areas.
- `dependencyRules`: component dependency policy. These rules are parsed and validated in the first milestone, but not enforced yet.
- `contentRules`: AST declaration rules enforced by the analyzer.
- `fileNameRules`: file name rules that may be triggered by file-level or declaration-level conditions.

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

`require.fileName.matches` means the file name must match at least one regex. `deny.fileName.matches` means the file name must not match any listed regex.

## Dependency Rules

`dependencyRules` are currently config-only. The loader validates that every rule key and every `mayDependOn` target names a configured component, so malformed policies fail early.

Future enforcement should map each checked file and each import target to configured components, then report imports whose target component is not in the source component's `mayDependOn` list or `commonComponents`.

## Limitations

- Dependency rules are parsed and validated, not enforced yet.
- Content rules inspect syntax only; they do not use type checking or semantic ownership rules.
- File name rules use Go regular expressions and match only the base file name, not the full path.
- Type declarations are classified by AST shape: interfaces are `interface`, structs are `struct`, and other type declarations are `type`.
- Rules report matching declarations independently, so one declaration may produce diagnostics from multiple rules.
