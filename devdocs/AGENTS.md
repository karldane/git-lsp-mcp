# AGENTS.md — git-lsp-mcp Developer Handbook

## Project Overview

`git-lsp-mcp` is a Go MCP server backend for `mcp-bridge`. It clones and syncs a
git repository to a pod-scoped cache, manages an LSP subprocess, and exposes
read-only MCP tools for semantic source code analysis.

See `SPEC_GIT_LSP_MCP.md` for full architecture. See `DEPLOYMENT_NOTES.md` for
Kubernetes and mcp-bridge integration details.

---

## Docs Map

| File                  | Purpose                                              |
|-----------------------|------------------------------------------------------|
| `SPEC_GIT_LSP_MCP.md` | Architecture, tools, lifecycle, config reference     |
| `DEPLOYMENT_NOTES.md` | K8s setup, Dockerfile, LSP options, CEL policies     |
| `README.md`           | Quick start, tool table, dependency list             |
| `AGENTS.md`           | This file — developer and agent working guidelines   |

---

## Development Philosophy

### Test-Driven Development

All code in this project is written test-first. The workflow is:

1. Write a failing test that specifies the behaviour
2. Write the minimum code to make it pass
3. Refactor — test still passes

No implementation code is written without a corresponding test written first.
PRs that add implementation without tests will not be merged.

### Coverage Requirement

**80% statement coverage is the minimum pass threshold** for CI.

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Coverage is measured across all packages. The 80% bar applies to the aggregate;
individual packages should aim higher where feasible.

#### Exemptions

Code that requires a live external server (real git remote, live LSP binary,
real filesystem with a cloned repo) may be placed behind an integration test
build tag and is exempt from the coverage threshold:

```go
//go:build integration
```

Run integration tests explicitly:

```bash
go test -tags integration -v ./...
```

**However**: the exemption is a last resort, not a first response. Before
reaching for it, ask whether the code can be refactored so that the logic
under test does not depend on the external server at all. In most cases it can.

---

## Abstraction Guidelines

The primary tool for keeping coverage high without live servers is **interface-driven
design**. External dependencies — git operations, LSP subprocess, filesystem — must
be hidden behind interfaces so tests can substitute fakes.

### Required interfaces

```go
// GitClient abstracts all git shell-outs
type GitClient interface {
    Clone(ctx context.Context, url, branch, dir string) error
    RemoteURL(ctx context.Context, dir string) (string, error)
    Fetch(ctx context.Context, dir string) error
    ResetHard(ctx context.Context, dir, ref string) error
    Log(ctx context.Context, dir, path string, limit int) ([]Commit, error)
    Blame(ctx context.Context, dir, path string) ([]BlameLine, error)
}

// LSPClient abstracts the LSP JSON-RPC session
type LSPClient interface {
    Initialize(ctx context.Context, rootURI string) error
    Definition(ctx context.Context, file string, line, col int) (*Location, error)
    References(ctx context.Context, file string, line, col int) ([]Location, error)
    Hover(ctx context.Context, file string, line, col int) (string, error)
    Diagnostics(ctx context.Context, file string) ([]Diagnostic, error)
}

// FS abstracts filesystem operations used by tools
type FS interface {
    ReadFile(path string) ([]byte, error)
    Walk(root string, fn filepath.WalkFunc) error
    Stat(path string) (os.FileInfo, error)
}
```

Production implementations (`RealGitClient`, `StdioLSPClient`, `OsFS`) live in
their respective files. Tests use fakes or mocks constructed inline — prefer
hand-written fakes over mocking libraries for simplicity and readability.

### Lifecycle abstraction

`lifecycle.go` coordinates clone/pull and LSP startup. Its dependencies
(`GitClient`, `LSPClient`, lock acquisition) are injected, making the lifecycle
logic fully testable without touching disk or spawning processes:

```go
type Lifecycle struct {
    Git      GitClient
    LSP      LSPClient
    Lock     Locker
    CacheDir string
    BackendID string
}
```

Tests construct a `Lifecycle` with fake implementations and assert state transitions
without any real git or LSP involvement.

---

## Go Best Practices

### General

- **Standard library first.** Reach for stdlib before adding a dependency.
  Justify every external import in the PR description.
- **No `init()` functions.** Initialisation belongs in constructors or `main`.
- **No global state.** All dependencies injected via struct fields or function args.
- **Errors are values.** Wrap with `fmt.Errorf("context: %w", err)`. Never discard.
- **No `panic` in library code.** Only acceptable in `main` for unrecoverable
  startup failures.
- **Context everywhere.** All functions that do I/O, spawn processes, or block
  accept `context.Context` as their first argument.
- **Short variable names for short scopes.** `err`, `ctx`, `r`, `w` are fine
  within a function. Unexported struct fields may be short. Exported identifiers
  must be descriptive.

### Project-specific

- **No CGo.** This project is `CGO_ENABLED=0`. Do not introduce any dependency
  that requires CGo.
- **No `os/exec` outside `internal/git` and `internal/lsp`.** Shell-outs are
  confined to those packages. Tool handlers never shell out directly.
- **Tool handlers are pure functions of their inputs.** A tool handler receives
  args and calls interface methods. It does not reach into global state.
- **LSP communication is JSON-RPC over stdio.** Do not introduce an HTTP LSP
  transport. Keep the LSP client minimal — implement only the methods the tools
  actually use.
- **Log, don't print.** Use `log/slog` (stdlib, Go 1.21+) for structured logging.
  Never use `fmt.Println` for operational output.

### File size

Keep files under 400 lines. If a file is growing beyond this, it is doing too much
and should be split.

### Error messages

Error messages are lowercase, no trailing punctuation, and include enough context
to identify the callsite:

```go
// Good
fmt.Errorf("lifecycle: clone repo: %w", err)

// Bad
fmt.Errorf("Error cloning repository!")
```

---

## Test Guidelines

### Unit tests

- Live alongside the code they test (`foo_test.go` next to `foo.go`)
- Use `package foo` (white-box) for internal logic, `package foo_test` (black-box)
  for public API surface
- No sleeps, no real filesystem, no real network
- Table-driven tests for any function with multiple input/output cases:

```go
func TestGitClientRemoteURL(t *testing.T) {
    cases := []struct {
        name    string
        output  string
        wantURL string
        wantErr bool
    }{
        {"valid remote", "https://github.com/internal/app.git\n", "https://github.com/internal/app.git", false},
        {"empty output", "", "", true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### Integration tests (`//go:build integration`)

- Require: `git` in PATH, at least one LSP binary in PATH, network access
- Must be clearly documented at the top of the file with what they require
- Must clean up after themselves (`t.Cleanup` or `defer`)
- Must not assume a specific repo is available — use a local bare repo created
  in `t.TempDir()` where possible

### Running tests

```bash
# Unit tests only (CI default)
make test

# With coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration tests (requires live dependencies)
go test -tags integration -v ./...
```

---

## Git Protocol

- Branch from `main`; branch names: `feat/`, `fix/`, `chore/`, `docs/`
- Commits are imperative present tense: `"add LSP hover tool"` not `"added hover"`
- One logical change per commit
- PRs must pass `make test` (unit only) and achieve ≥ 80% coverage before review
- Squash merge to `main`

---

## Makefile Targets

```bash
make build          # build binary for current platform
make build-linux    # cross-compile linux/amd64
make build-darwin   # cross-compile darwin/arm64
make test           # unit tests with race detector
make lint           # fmt + vet
make clean          # remove built binaries
make docker         # build + push container image (requires IMAGE=...)
```

---

## Dependencies

| Module                          | Purpose                              | Justification                     |
|---------------------------------|--------------------------------------|-----------------------------------|
| `github.com/karlDane/mcp-framework` | MCP server, ToolHandler, EnforcerProfile | Project standard               |
| `github.com/gofrs/flock`        | Cross-platform file locking          | No stdlib equivalent              |
| `github.com/mark3labs/mcp-go`   | MCP protocol types                   | Transitive via mcp-framework      |

All other functionality uses the Go standard library. New dependencies require
explicit justification and lead developer approval.

---

## CI Checklist

Before marking a PR ready for review:

- [ ] `make lint` passes (fmt + vet clean)
- [ ] `make test` passes with no race conditions detected
- [ ] Coverage ≥ 80% (`go tool cover -func=coverage.out`)
- [ ] New tools have `GetEnforcerProfile()` implemented and tested
- [ ] No new `//go:build integration` exemptions without prior discussion
- [ ] `SPEC_GIT_LSP_MCP.md` updated if behaviour or config has changed
