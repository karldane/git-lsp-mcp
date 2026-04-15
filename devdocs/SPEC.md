# SPEC_GIT_LSP_MCP.md — git-lsp-mcp

## Overview

`git-lsp-mcp` is a Go binary implementing the MCP (Model Context Protocol) server interface. It provides semantic source code analysis for AI agents by managing a git-backed workspace and an LSP (Language Server Protocol) subprocess. It is designed to run as a backend subprocess within `mcp-bridge`.

A single instance targets **one git repository at one branch**. Multiple repos are handled by configuring multiple `mcp-bridge` backends with distinct `tool_prefix` values (e.g. `github_backoffice_`, `github_driverportal_`).

It uses the [`mcp-framework`](https://github.com/karlDane/mcp-framework) for tool registration, MCP protocol handling, and safety self-reporting via `EnforcerProfile`.

---

## Architecture
mcp-bridge subprocess spawn
│
▼
git-lsp-mcp (this binary)
├── lifecycle.go — clone/verify/pull, LSP start, lockfile
├── lsp.go — LSP JSON-RPC client over stdio
├── tools/ — MCP ToolHandler implementations
└── main.go — framework.Server wiring + env config
│
├── child process: gopls / tsserver / pyright (LSP)
│ └── pointed at CACHE_DIR/BACKEND_ID/
│
└── shared pod cache: /cache/src/{backend-id}/ ← emptyDir volume

text

### Startup Sequence (on `initialize`)

1. Derive `workDir` = `$CACHE_DIR/$BACKEND_ID`
2. Acquire `workDir + ".lock"` (flock) — guards against pool_size > 1 races
3. **If `workDir` does not exist:** `git clone $REPO_URL --branch $BRANCH --depth 1 workDir`
4. **If `workDir` exists:**
   - Assert `git remote get-url origin` matches `$REPO_URL` — fail fast if not
   - `git fetch origin`
   - `git reset --hard origin/$BRANCH` — deterministic, no merge ambiguity, no dirty state
5. Release lockfile
6. Start LSP child process: `$LSP_BINARY --stdio` with `rootUri` = `workDir`
7. Send LSP `initialize` / `initialized` handshake; wait for response
8. Signal MCP `initialize` complete → bridge marks process warm

The bridge's existing warmup window (between subprocess spawn and marking warm) absorbs steps 1–7. No changes to mcp-bridge are required.

---

## Cache Model

The cloned repository lives in a **pod-scoped cache** (`emptyDir` volume), not in process-ephemeral storage.

- Survives subprocess restarts and idle GC cycles
- Wiped automatically on pod recycle — no manual cleanup needed
- Multiple processes sharing the same backend (pool_size > 1) share the same on-disk checkout via the lockfile protocol
- All access is **read-only** after sync; no tool writes to the workspace

---

## Configuration (env vars, injected by mcp-bridge)

| Variable      | Required | Description                                                       |
|---------------|----------|-------------------------------------------------------------------|
| `REPO_URL`    | Yes      | Full git clone URL (https or ssh)                                 |
| `BRANCH`      | No       | Branch to track. Default: `main`                                  |
| `GIT_TOKEN`   | No       | Personal access token injected into clone URL for HTTPS auth      |
| `SSH_KEY_PATH`| No       | Path to mounted SSH key for SSH clone URLs                        |
| `CACHE_DIR`   | Yes      | Base path for workspace cache. e.g. `/cache/src`                  |
| `BACKEND_ID`  | Yes      | Unique identifier matching mcp-bridge backend id. Used as subdir  |
| `LSP`         | Yes      | LSP to start: `gopls`, `tsserver`, `pyright`, `rust-analyzer`, `perl`, `perl-language-server` |
| `LSP_BINARY`  | No       | Override resolved binary path. Default: resolved from `$PATH`     |
| `SYNC_ON_CALL`| No       | If `true`, re-pull before every tool call. Default: `false`       |

---

## MCP Tools

All tools are read-only. All implement `framework.ToolHandler` and declare `EnforcerProfile`.

### `read_file`
Read the full content of a file by relative path.
Args: path (string, required) — relative to repo root
Return: file content as string

text

```go
func (t *ReadFileTool) GetEnforcerProfile() framework.EnforcerProfile {
    return framework.NewEnforcerProfile(
        framework.WithRisk(framework.RiskLow),
        framework.WithImpact(framework.ImpactRead),
        framework.WithResourceCost(2),
        framework.WithPII(false),
        framework.WithIdempotent(true),
        framework.WithApprovalReq(false),
    )
}
```

### `search`
Full-text search across the workspace using ripgrep or stdlib filepath.Walk fallback.
Args: query (string, required)
glob (string, optional) — file pattern filter, e.g. "*.ts"
max_results (int, optional) — default 50
Return: []{ path, line_number, line_content }

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 3, PII false, Idempotent true.

### `directory_tree`
Return the directory structure of the repo root or a subdirectory.
Args: path (string, optional) — default "/"
depth (int, optional) — default 3
Return: tree as indented string

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 1, PII false, Idempotent true.

### `definition`
Resolve the definition of a symbol at a given file position (LSP `textDocument/definition`).
Args: path (string, required) — file path relative to repo root
line (int, required) — 0-indexed
column (int, required) — 0-indexed
Return: { path, line, column, source_snippet }

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 4, PII false, Idempotent true.

### `references`
Find all references to a symbol (LSP `textDocument/references`).
Args: path (string, required)
line (int, required)
column (int, required)
Return: []{ path, line, column, snippet }

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 5, PII false, Idempotent true.

### `hover`
Get type information and documentation for a symbol (LSP `textDocument/hover`).
Args: path (string, required)
line (int, required)
column (int, required)
Return: hover text as string

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 2, PII false, Idempotent true.

### `diagnostics`
Get LSP diagnostics (errors, warnings) for a file (LSP `textDocument/diagnostic`).
Args: path (string, required)
Return: []{ severity, line, column, message, source }

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 3, PII false, Idempotent true.

### `git_log`
Recent commit history for a file or the whole repo.
Args: path (string, optional) — scope to a file
limit (int, optional) — default 20, max 100
Return: []{ hash, author, date, message }

text

Implemented via `git log` shell-out; does not require LSP.

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 2, PII false, Idempotent true.

### `git_blame`
Line-by-line authorship for a file.
Args: path (string, required)
Return: []{ line_number, hash, author, date, content }

text

EnforcerProfile: RiskLow, ImpactRead, ResourceCost 3, PII false, Idempotent true.

---

## Project Structure
git-lsp-mcp/
├── main.go — env config, framework.Server wiring, tool registration
├── lifecycle.go — clone/verify/pull, lockfile, LSP process start + handshake
├── lsp.go — LSP JSON-RPC client (stdio transport, request/response/notify)
├── tools/
│ ├── read_file.go
│ ├── search.go
│ ├── directory_tree.go
│ ├── definition.go
│ ├── references.go
│ ├── hover.go
│ ├── diagnostics.go
│ ├── git_log.go
│ └── git_blame.go
├── tools_test.go — integration tests (requires git + LSP binary in PATH)
├── lifecycle_test.go
├── lsp_test.go
├── Makefile
├── go.mod
├── go.sum
└── README.md

text

---

## Makefile

Follows the same conventions as `mcp-bridge` and other project Makefiles.

```makefile
BINARY     = git-lsp-mcp
GO         = go
GOFLAGS    = -ldflags="-s -w"

.PHONY: all build test lint fmt vet clean docker

all: build

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

## Run tests (requires git, and at least one LSP binary in PATH)
test:
	$(GO) test -v -race ./...

lint: fmt vet

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

## Build Linux amd64 binary (for container / mcp-bridge deployment)
build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY)-linux-amd64 .

## Build macOS arm64 binary (Apple Silicon dev)
build-darwin:
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BINARY)-darwin-arm64 .

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-darwin-arm64

## Build and push container image
## Usage: make docker IMAGE=your-registry/git-lsp-mcp:latest
docker:
	docker build -t $(IMAGE) .
	docker push $(IMAGE)
```

Note: unlike `mcp-bridge`, this project does **not** use CGo. No C compiler is required. `CGO_ENABLED=0` builds are fully supported.

---

## Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o git-lsp-mcp .

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    git \
    nodejs npm \
    && npm install -g typescript-language-server typescript \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/git-lsp-mcp /usr/local/bin/git-lsp-mcp

CMD ["git-lsp-mcp"]
```

Build the image with only the LSP runtimes your backends actually need.

---

## mcp-bridge Deployment Notes

### New dependencies on the mcp-bridge pod / image

| Dependency                 | Why                        | Notes                                          |
|----------------------------|----------------------------|------------------------------------------------|
| `git`                      | Clone, fetch, reset, log   | `apt install git`                              |
| `gopls`                    | Go LSP                     | `go install golang.org/x/tools/gopls@latest`   |
| `typescript-language-server` | TypeScript/JS LSP        | `npm install -g typescript-language-server typescript` |
| `pyright`                  | Python LSP                 | `pip install pyright` — only if needed         |
| `rust-analyzer`            | Rust LSP                   | Binary from GitHub releases — only if needed   |
| `perl`                     | Perl LSP                   | `apt install perl` + `cpan Perl::LanguageServer` — only if needed |
| Network egress             | git clone/pull             | Pod must reach git host; check NetworkPolicy   |

### K8s additions to mcp-bridge deployment

```yaml
# Add to spec.template.spec.volumes
volumes:
  - name: src-cache
    emptyDir: {}           # wiped on pod recycle; persists across subprocess restarts

# Add to container volumeMounts
volumeMounts:
  - name: src-cache
    mountPath: /cache/src
```

### config.yaml backend entries

```yaml
backends:
  - id: github_backoffice
    name: "Source: Backoffice"
    command: "git-lsp-mcp"
    tool_prefix: "github_backoffice_"
    pool_size: 1
    env:
      REPO_URL: "https://github.com/internal/backoffice.git"
      BRANCH: "main"
      GIT_TOKEN: "${GITHUB_TOKEN}"
      CACHE_DIR: "/cache/src"
      BACKEND_ID: "github_backoffice"
      LSP: "tsserver"

  - id: github_driverportal
    name: "Source: Driver Portal"
    command: "git-lsp-mcp"
    tool_prefix: "github_driverportal_"
    pool_size: 1
    env:
      REPO_URL: "https://github.com/internal/driverportal.git"
      BRANCH: "main"
      GIT_TOKEN: "${GITHUB_TOKEN}"
      CACHE_DIR: "/cache/src"
      BACKEND_ID: "github_driverportal"
      LSP: "tsserver"
```

### pool_size recommendation

Set `pool_size: 1` for source backends. LSP processes maintain in-memory indexes and
are memory-intensive. Multiple instances per backend provide no meaningful throughput
benefit and will significantly increase memory usage.

---

## Dependencies
github.com/karlDane/mcp-framework — MCP server, ToolHandler, EnforcerProfile
github.com/gofrs/flock — cross-platform file locking
github.com/mark3labs/mcp-go — MCP protocol types (transitive via framework)

text

No CGo. No C compiler. Pure Go + subprocess exec (git, LSP binary).

---

## Safety Profile Summary

All tools are `ImpactRead`, `RiskLow`, `Idempotent: true`, `ApprovalRequired: false`.
No tool performs any write operation on the repository or the LSP state.

CEL policy complement (add to mcp-bridge `policies.yaml`):

```yaml
- id: source-readonly-enforce
  description: "Belt-and-braces: deny any source tool not in known read set"
  match: "github_backoffice_*"
  condition: >
    tool.name in [
      "github_backoffice_read_file",
      "github_backoffice_search",
      "github_backoffice_directory_tree",
      "github_backoffice_definition",
      "github_backoffice_references",
      "github_backoffice_hover",
      "github_backoffice_diagnostics",
      "github_backoffice_git_log",
      "github_backoffice_git_blame"
    ]
  action: ALLOW
  on_fail: DENY
```
