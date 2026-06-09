# AGENTS.md

## Project Mission

This project builds `go-file-arch`: a Go-native architecture checker for Go repositories.

The tool must eventually replace separate architecture-enforcement tools by combining:

1. Import/package dependency boundary checks.
2. File-content AST checks.
3. File naming convention checks.
4. YAML-driven project rules.
5. A reusable Go library plus a small CLI.
6. Pre-commit and CI-friendly execution.

The main target use case is a layered Go architecture with folders such as:

```text
cmd/
app/
interface/
adapter/
core/
library/
```

The tool should help prevent architectural drift, especially when multiple AI agents or developers are editing the same codebase.

## Non-Negotiable Goals

* Use Go, not Python.
* Use Go AST, not regex over source text.
* Use `golang.org/x/tools/go/analysis` as the analyzer foundation.
* Expose a reusable library package.
* Expose a CLI under `cmd/go-file-arch`.
* Load rules from `.go-file-arch.yml`.
* Work from pre-commit with `pass_filenames: false`.
* Produce clear diagnostics that tell the developer where to move code.
* Do not bypass hooks using `--no-verify`.
* Do not remove existing tools from a consuming repository until parity is proven.

## Intended Usage

The CLI should support:

```bash
go-file-arch -config .go-file-arch.yml ./...
```

During local development, it may also be run as:

```bash
go run ./cmd/go-file-arch -config .go-file-arch.yml ./...
```

Example pre-commit hook:

```yaml
repos:
  - repo: local
    hooks:
      - id: go-file-arch
        name: Go file architecture rules
        entry: go run ./cmd/go-file-arch -config .go-file-arch.yml ./...
        language: system
        pass_filenames: false
        types:
          - go
```

Do not make the hook inspect only changed filenames. Architecture rules need package and repository context.

## Desired Repository Structure

Prefer this structure:

```text
cmd/go-file-arch/
  main.go

filearch/
  analyzer.go
  config.go
  runner.go
  components.go
  dependency_rules.go
  content_rules.go
  filename_rules.go
  glob.go
  diagnostics.go

filearch/testdata/
  src/...

docs/
  config.md
  pre-commit.md
  architecture.md

.go-file-arch.yml
README.md
AGENTS.md
```

If this project starts inside another repository first, `internal/go_file_arch` is acceptable. If this is a standalone reusable library, prefer exported package name:

```go
package filearch
```

## Implementation Phases

### Phase 1: Content Rules

Implement YAML-driven AST declaration checks.

Supported declaration kinds:

```text
interface
struct
func
var
const
type
```

Rules should support:

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

Expected behavior:

* Match files using include/exclude globs.
* Inspect declarations using Go AST.
* Report denied declarations.
* If `allow.declarations` is non-empty, report declarations not in the allow list.
* Package and import declarations are allowed by default unless future config explicitly denies them.

### Phase 2: File Naming Rules

Add file naming convention checks.

Example:

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

Support these file name matchers first:

```text
equals
equalsAny
matches
prefix
suffix
```

Keep file naming rules high-signal. Do not over-enforce style-only preferences unless explicitly requested.

### Phase 3: Dependency Rules

Implement go-arch-lint-like dependency boundary checks.

Example config:

```yaml
components:
  app:
    in:
      - app/**

  cmd:
    in:
      - cmd/**

  interface_dto:
    in:
      - interface/**/dto/**

  interface:
    in:
      - interface/**

  adapter_entities:
    in:
      - adapter/**/entities/**

  adapter:
    in:
      - adapter/**

  core_model:
    in:
      - core/**/model/**

  core:
    in:
      - core/**

  library:
    in:
      - library/**

commonComponents:
  - library

dependencyRules:
  cmd:
    mayDependOn:
      - app

  app:
    mayDependOn:
      - adapter
      - adapter_entities
      - core
      - core_model
      - interface
      - interface_dto
      - library

  interface:
    mayDependOn:
      - core
      - core_model
      - interface
      - interface_dto

  interface_dto:
    mayDependOn:
      - core
      - core_model

  adapter:
    mayDependOn:
      - core
      - core_model
      - adapter
      - adapter_entities

  adapter_entities:
    mayDependOn:
      - core
      - core_model

  core:
    mayDependOn:
      - core
      - core_model

  core_model:
    mayDependOn:
      - core

  library:
    mayDependOn:
      - library
```

Rules:

* Determine the component of the importing package or file.
* Determine the component of the imported local package.
* If target component is not allowed, report a diagnostic.
* External/vendor imports should be configurable later.
* If multiple components match a path, choose the most specific match.
* More specific globs must win over broad globs.
* For example, `interface/**/dto/**` must win over `interface/**`.

### Phase 4: Parity With go-arch-lint

Only after dependency rules work:

1. Compare output with existing go-arch-lint config in a real repository.
2. Fix false positives.
3. Fix false negatives.
4. Run both tools together temporarily.
5. Remove go-arch-lint only after confirmed parity.

Do not remove go-arch-lint from any consuming project in the first implementation.

## YAML Config Contract

The first full config should look like this:

```yaml
version: 1

components:
  app:
    in:
      - app/**

  cmd:
    in:
      - cmd/**

  interface_dto:
    in:
      - interface/**/dto/**

  interface:
    in:
      - interface/**

  adapter_entities:
    in:
      - adapter/**/entities/**

  adapter:
    in:
      - adapter/**

  core_model:
    in:
      - core/**/model/**

  core:
    in:
      - core/**

  library:
    in:
      - library/**

commonComponents:
  - library

dependencyRules:
  cmd:
    mayDependOn:
      - app

  app:
    mayDependOn:
      - adapter
      - adapter_entities
      - core
      - core_model
      - interface
      - interface_dto
      - library

  interface:
    mayDependOn:
      - core
      - core_model
      - interface
      - interface_dto

  interface_dto:
    mayDependOn:
      - core
      - core_model

  adapter:
    mayDependOn:
      - core
      - core_model
      - adapter
      - adapter_entities

  adapter_entities:
    mayDependOn:
      - core
      - core_model

  core:
    mayDependOn:
      - core
      - core_model

  core_model:
    mayDependOn:
      - core

  library:
    mayDependOn:
      - library

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

  - id: core-no-struct-outside-model
    files:
      include:
        - core/**/*.go
      exclude:
        - core/**/model/**/*.go
        - "**/*_test.go"
    deny:
      declarations:
        - struct
    message: "Structs in core should live under core/**/model/**."

fileNameRules:
  - id: ban-generic-file-names
    files:
      include:
        - "**/*.go"
      exclude:
        - "**/*_test.go"
    deny:
      fileName:
        equalsAny:
          - helper.go
          - helpers.go
          - common.go
          - misc.go
          - utils.go
    message: "Avoid generic file names. Use a domain-specific name."
```

Do not invent a different schema unless there is a strong technical reason. If a schema change is needed, document the reason.

## Diagnostics

Diagnostics must be actionable.

Bad:

```text
architecture violation
```

Good:

```text
core/user/repository.go:12:1: [core-repository-interface-only] core repository files may only define interfaces. Move implementations to adapter/**. detected declaration kind: struct
```

Every diagnostic should include:

```text
rule id
configured message
detected declaration kind or failed matcher
source location
```

When possible, point to the declaration position, not just the file start.

## Glob and Path Rules

Normalize all paths with:

```go
filepath.ToSlash(path)
```

Required glob support:

```text
**/*.go
core/**/repository.go
core/**/model/**/*.go
interface/**/dto/**
adapter/**/entities/**
```

Component matching must handle specificity.

Example:

```text
interface/**/dto/**  >  interface/**
adapter/**/entities/** > adapter/**
core/**/model/** > core/**
```

A practical first implementation may calculate specificity using:

```text
longer non-wildcard prefix
fewer wildcards
longer pattern length
```

Document the chosen approach.

## Analyzer Design

Use `analysis.Analyzer`.

The analyzer should:

1. Load config from `-config`.
2. Validate config.
3. Iterate over parsed files from the analysis pass.
4. Apply content rules.
5. Apply file naming rules.
6. Apply dependency rules when implemented.
7. Report diagnostics through `pass.Reportf` or `pass.Report`.

The CLI should use `singlechecker` for the first standalone version unless a custom runner is required for multi-package dependency resolution.

If a custom runner is needed later, document why.

## Library API Goal

Expose a reusable API.

Target shape:

```go
type Options struct {
	ConfigPath string
	Patterns  []string
	Workdir   string
}

func Run(ctx context.Context, opts Options) error
```

Also expose analyzer construction:

```go
func NewAnalyzer(opts AnalyzerOptions) *analysis.Analyzer
```

Keep the public API small until the tool stabilizes.

## Testing Requirements

Use normal unit tests for:

```text
config parsing
config validation
glob matching
component specificity
file name matcher behavior
```

Use analyzer tests for diagnostics.

Test fixtures should cover:

```text
valid core repository interface
invalid struct in core repository.go
invalid func in core repository.go
valid struct in core/**/model/**
invalid struct in core outside model
invalid DTO struct outside DTO location
generic filename violation
```

Use `// want` comments in analyzer fixtures where appropriate.

## Required Commands Before Commit

Run:

```bash
gofmt -w ./cmd ./filearch
go test ./...
go run ./cmd/go-file-arch -config .go-file-arch.yml ./...
```

If the project structure differs, adjust paths but run equivalent checks.

Do not skip tests.

Do not use:

```bash
git commit --no-verify
```

## Dependency Policy

Keep dependencies minimal.

Acceptable initial dependencies:

```text
golang.org/x/tools/go/analysis
golang.org/x/tools/go/analysis/singlechecker
golang.org/x/tools/go/analysis/analysistest
gopkg.in/yaml.v3
```

Before adding any other dependency, explain why the standard library is insufficient.

## Coding Style

* Prefer small focused files.
* Prefer plain structs over clever abstractions.
* Keep rule evaluation readable.
* Keep YAML decoding strict where practical.
* Validate config early.
* Fail loudly on malformed config.
* Avoid global mutable state except analyzer flags where needed.
* Do not hide errors.
* Do not use regex against raw Go source to detect declarations.
* Do not implement a full Semgrep clone.

## What Not To Build Yet

Do not build these in the first milestone:

```text
custom DSL
autofix
IDE integration
golangci-lint plugin
multi-language support
complex semantic type rules
full import graph visualizer
HTML reports
```

These may come later, after the core rule engine works.

## Future Ideas

Potential future built-in semantic rules:

```text
repository interface methods must accept context.Context
repository interface methods must not return adapter entities
adapter packages must not expose persistence entities to interface packages
core must not import database, HTTP, or framework packages
constructor functions must live in adapter or app, not core
```

These should be implemented as named built-in rules enabled by YAML, not as complex YAML-only logic.

Example future config:

```yaml
builtinRules:
  - id: repository-methods-require-context
    enabled: true
    files:
      include:
        - core/**/repository.go
```

## Agent Behavior

When working on this project:

1. Inspect existing files before editing.
2. Make small, reviewable changes.
3. Preserve the YAML schema unless explicitly changing it.
4. Add or update tests for every behavior change.
5. Run the required commands before reporting completion.
6. Report what was implemented and what remains.
7. Never claim parity with go-arch-lint until dependency rule comparison has been performed.
8. Never bypass pre-commit hooks.
9. Prefer partial, working milestones over large unfinished rewrites.
10. Keep diagnostics helpful for both humans and AI coding agents.

## Definition of Done for First Milestone

The first milestone is complete when:

```text
go-file-arch can load .go-file-arch.yml
contentRules are enforced
fileNameRules are enforced at least for simple file-name deny rules
tests pass
CLI runs successfully
pre-commit example is documented
dependencyRules are parsed and validated, even if not fully enforced yet
```

## Definition of Done for Replacement Milestone

The tool can replace go-arch-lint only when:

```text
dependencyRules are fully enforced
component matching supports specificity
local import resolution works reliably
excludes are honored
commonComponents are honored
output has been compared against go-arch-lint on a real repo
false positives and false negatives are documented
CI runs go-file-arch successfully
the team agrees to remove go-arch-lint
```
