# go-file-arch

`go-file-arch` enforces repository layout, Go file placement, declaration
shape, and import boundaries from strict YAML. It can run alone or alongside
`go-arch-lint`.

The executable strict example is
[`filearch/testdata/generic-strict/.architecture.yaml`](filearch/testdata/generic-strict/.architecture.yaml).
The test suite loads that policy and checks both a conforming service and the
complete, sorted output from a nonconforming service.

## Install

```sh
GOPRIVATE=github.com/cavos-io/* \
  go install github.com/cavos-io/go-file-arch/cmd/go-file-arch@v0.1.0
```

## CLI

```sh
# Managed policy name
go-file-arch -c .architecture.yaml ./...

# Analyze a repository with a policy stored elsewhere
go-file-arch -c ../config-files/repos/service-manager/.architecture.yaml \
  -C ./my-service ./...

# Optional copied go-arch-lint check
go-file-arch -c .architecture.yaml -a .go-arch-lint.yml ./...
```

| Short | Long | Meaning |
|---|---|---|
| `-c` | `--config` | YAML policy path |
| `-a` | `--arch-lint-config` | optional `go-arch-lint` policy |
| `-C` | `--workdir` | repository to analyze; overrides YAML `workdir` |
| `-h` | `--help` | usage |

Without `-c`, the CLI discovers legacy `.go-file-arch.yml` or
`.go-file-arch.yaml` in the workdir. A managed `.architecture.yaml` must be
passed explicitly.

Exit codes are stable:

- `0`: valid policy and no findings.
- `1`: one or more architecture findings.
- `2`: invalid policy, malformed Go, package-loading failure, or tool failure.

The Go API returns `*filearch.ViolationError` only for architecture findings,
so callers can distinguish them with `errors.As`.

## Configuration contract

The only supported version is `version: 1`. Unknown YAML fields are rejected.
Every list rule requires a globally unique `id`, a nonempty `message`, and
supports only omitted severity or `severity: error`.

All paths use slash-normalized, workdir-relative globs. `**` crosses directory
boundaries. Exclusions take precedence over inclusions.

### Components and dependency rules

Components classify Go source files and internal import targets. Directory
patterns classify importable packages; file patterns cover root files such as
`app.go`.

File patterns always classify source files. For import-target package
ownership, only patterns whose basename is exactly `*.go` contribute their
directory; constrained names such as `*_test.go` and `app.go` are source-only.

```yaml
componentOptions:
  requireMatch: true
  requireSingleMatch: true

components:
  composition:
    files:
      include: [app.go]
  core:
    in: [core/*]
  core_model:
    in: [core/*/model]

dependencyRules:
  composition:
    mayDependOn: [composition, core]
  core:
    mayDependOn: [core, core_model]
```

`requireMatch` rejects every unclassified Go file.
`requireSingleMatch` rejects overlap. When legacy resolution needs one winner,
specificity is deterministic: longer literal prefix, more literal bytes, fewer
wildcards, longer pattern, then component name.

A dependency rule applies to internal imports whose source and target both map
to components. `commonComponents` are implicitly allowed from every source.

### Closed filesystem rules

`pathRules` inspect the repository inventory, including `.`. A rule may
inspect only direct children or every recursive descendant.

```yaml
pathRules:
  - id: root-layout
    directories:
      include: ['.']
    depth: direct
    require:
      files: [.architecture.yaml, app.go, go.mod]
      directories: [core]
    allow:
      files: [.architecture.yaml, app.go, go.mod, rogue.go]
      directories: [core, legacy]
    deny:
      directories: [legacy]
    message: root layout contract
```

Each `require` pattern must match. A nonempty `allow` block closes the
corresponding file or directory set. `deny` always wins. Supported depths are
`direct` and `recursive`.

`directoryRules` retain the concise per-directory contract:

```yaml
directoryRules:
  - id: domain-layout
    directories:
      include: [core/*]
      exclude: [core/*/model]
    require:
      files: [service.go]
      anyFiles: [model/*_model.go]
    message: domain layout contract
```

Every `files` pattern is required; at least one `anyFiles` pattern is
required.

`directoryNameRules` apply exact, set, regular-expression, prefix, or suffix
conditions to each selected directory basename:

```yaml
directoryNameRules:
  - id: module-directory-name
    directories:
      include: [modules/**]
      exclude: [modules/generated/**]
    require:
      directoryName:
        matches: ['^[a-z][a-z0-9]*(_[a-z0-9]+)*$']
    deny:
      directoryName:
        equalsAny: [legacygroup]
    message: module directories use snake_case
```

A configured `require` block is an allowlist and `deny` takes precedence.
Directory-name rules use the repository inventory, so repository-wide checks
require the CLI or `filearch.Run`. Unknown fields, invalid globs, invalid
regular expressions, empty conditions, and duplicate rule IDs fail strict
configuration loading.

### Reusable path templates

Templates capture path segments and reuse them in sibling paths or declaration
names.

```yaml
templates:
  domain_service:
    pattern: core/{domain}/service.go
    captures:
      domain: '^[a-z][a-z0-9_]*$'
```

A file set selects a template with `templates: [domain_service]`. Expansions
support `{domain}`, `{domain|snake}`, `{domain|camel}`, and
`{domain|pascal}`. Unknown captures, transforms, duplicate captures, and
invalid constraints fail configuration loading.

### File contracts and structural declarations

```yaml
fileContractRules:
  - id: repository-context-contract
    files:
      templates: [domain_service]
    require:
      declarations:
        - kind: interface
          name: 'I{domain|pascal}Repository'
          methods:
            all:
              parameters:
                first: {name: ctx, type: context.Context}
          count: {equals: 1}
    message: repository methods start with context
```

Declaration selectors support:

| Field | Contract |
|---|---|
| `kind` | `interface`, `struct`, `func`, `var`, `const`, `type`, or `alias` |
| `name` | exact name |
| `nameMatches` / `nameNotMatches` | Go regular expressions |
| `exported` | declaration visibility |
| `receiver` | `present`, `pointer`, `type`, `typeMatches` |
| `parameters` / `returns` | `count`, `first`, `contains`, `all` |
| `embeds` | embedded struct or interface types |
| `fields` | struct field `count`, `contains`, or `all` |
| `methods` | interface method `count`, `contains`, or `all` |
| `count` | matching declarations: `equals`, `min`, `max` |
| `value` | literal constant `equals` or `matches` |
| `underlying` | declared type condition; valid for `kind: type` |
| `initialized` | initializer presence; valid for `kind: var` or `kind: const` |

`alias` matches declarations written as `type A = B`. Named types such as
`type A B` remain `type` declarations.

A type condition accepts a legacy scalar such as `error` or a mapping with
`name`, `type`, `typeMatches`, `typeNotMatches`, and `exportedType`. A method
condition uses `name`, `nameMatches`, `parameters`, and `returns`. Configured
fields use AND semantics; `contains` requires every listed condition. A struct
field condition supports `name`, `nameMatches`, `type`, `typeMatches`,
`exported`, `exportedType`, and `tagMatches`.

```yaml
deny:
  declarations:
    - kind: type
      underlying:
        typeMatches: ["^(string|bool|u?int(8|16|32|64)?)$"]
    - kind: var
      exported: false
      initialized: true
```

Omitted declaration count means at least one match.

Matching is syntactic. It uses rendered Go AST types and does not perform type
identity, assignability, call-graph, or data-flow analysis. Constant matching
supports literal initializers but does not evaluate expressions or `iota`.

`declarationGroupingRules` apply count-dependent layout without knowing a
domain convention:

```yaml
declarationGroupingRules:
  - id: selected-variable-grouping
    files:
      include: ["**/state.go"]
    declaration:
      kind: var
      nameMatches: ["^State[A-Z]"]
    separateWhenCount: {max: 2}
    singleGroupWhenCount: {min: 3}
    message: selected variables use deterministic grouping
```

When the separate threshold matches, selected declarations cannot use a
parenthesized group. When the single-group threshold matches, they must share
one group containing no unselected declarations. Threshold ranges cannot
overlap.

### Complete import rules

Import rules inspect standard-library, matched internal, unmatched internal,
and external imports. Condition entries are alternatives; a match in any
configured field matches the block. A nonempty `allow` block is an allowlist,
and `deny` takes precedence.

```yaml
importRules:
  - id: core-import-boundary
    files:
      templates: [domain_service]
    allow:
      components: [core_model]
      paths: [context]
    deny:
      categories: [external, unmatched-internal]
    message: core imports must be explicit
```

Conditions support exact `paths`, `pathPrefixes`, `pathMatches`,
`components`, and these categories:

- `standard`: first import-path segment has no dot.
- `internal`: module-relative path matches a component.
- `unmatched-internal`: module-relative path matches no component.
- `external`: non-module import whose first segment contains a dot.

The module path comes from the analyzed workdir's `go.mod`.

### Content and filename rules

`contentRules` allow or deny declaration kinds or names in selected files.
Package and import declarations remain allowed unless explicitly constrained.

`fileNameRules` apply exact, set, regular-expression, prefix, or suffix
conditions to the base filename. They can be gated by declaration kinds or type
name expressions. See the executable fixture and package tests for canonical
field combinations.

## Diagnostics and repository-wide behavior

Diagnostics are sorted by path, line, column, rule ID, then message. Valid
analysis reports all findings rather than stopping at the first one.

Directory, path, and missing-sibling checks require the CLI or
`filearch.Run`, which builds a repository-wide inventory. `NewAnalyzer`
still supports per-package declaration and import checks but cannot prove that
an absent repository path exists elsewhere.

Generated files and tests are analyzed unless a rule excludes them. There is no
built-in baseline, warning mode, changed-files-only mode, or service-specific
suppression mechanism.

Source rules can select handwritten or generated files independently:

```yaml
files:
  include: ["**/*.go"]
  generated: false
```

Omitting `generated` preserves the default of analyzing both. Detection uses
Go's `ast.IsGenerated`; a filename alone never grants an exemption. Components
and directory/path rules remain path-based and cannot use this selector.

## Development

```sh
go tool lefthook install
go test ./...
go tool lefthook run pre-commit
go tool lefthook run pre-push
```

Do not bypass the hooks with `--no-verify`.
