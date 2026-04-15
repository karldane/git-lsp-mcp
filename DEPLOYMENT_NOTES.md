# Deployment Notes — git-lsp-mcp + mcp-bridge

## New pod dependencies

The mcp-bridge container image must include the following when git-lsp-mcp backends
are configured:

| Dependency                  | Install                                               | Required for  |
|-----------------------------|-------------------------------------------------------|---------------|
| git                         | apt install git                                       | All backends  |
| gopls                       | go install golang.org/x/tools/gopls@latest            | Go repos      |
| typescript-language-server  | npm install -g typescript-language-server typescript  | TS/JS repos   |
| pyright                     | pip install pyright                                   | Python repos  |
| rust-analyzer               | GitHub releases binary                                | Rust repos    |
| libperl-languageserver-perl | apt install libperl-languageserver-perl (see below)   | Perl repos    |

Only include LSP runtimes needed by your configured backends.

---

## Perl LSP

Two options are available. Both are viable; choose based on your environment.

### Option A — Perl::LanguageServer (recommended)

Available as a Debian package — no CPAN, no compiler toolchain required. All
AnyEvent, Coro, and Moose dependencies are resolved as deb packages automatically.

| Debian release | Package version | Binary name             |
|----------------|-----------------|-------------------------|
| trixie (stable)| 2.6.2-1         | `perl-language-server`  |
| bookworm       | 2.5.0-1         | `perl-language-server`  |

Trixie (2.6.2) is recommended — it includes explicit support for Moose method
modifiers (`before`, `after`, `around`), which is important for modern Perl OO
codebases. If your backoffice system uses Moose, base your image on `debian:trixie-slim`.

In `config.yaml`, set `LSP: "perl-language-server"` for Perl backends.

**Supported LSP features relevant to backoffice analysis:**
- Goto Definition — follow a sub or module reference to its exact source
- Find References — trace all call sites across the codebase
- Workspace symbol search — index all subs and packages
- Syntax checking / diagnostics
- Call signatures
- Moose method modifiers (trixie / v2.6+ only)

### Option B — Perl Navigator (alternative)

A newer Node.js-based Perl LSP. Bundles all its own dependencies; no Perl module
installation required. Useful if Perl::LanguageServer proves problematic in your
environment (e.g. pinned to bookworm, or dependency conflicts).

```bash
npm install -g perl-navigator-server
```

Binary name: `perl-navigator`

In `config.yaml`, set `LSP: "perl-navigator"` for Perl backends.

Perl Navigator is generally considered easier to install but has less complete
support for large legacy Perl codebases compared to Perl::LanguageServer v2.6.

---

## Kubernetes additions

Add to mcp-bridge Deployment spec:

```yaml
# spec.template.spec.volumes
volumes:
  - name: src-cache
    emptyDir: {}           # wiped on pod recycle; persists across subprocess restarts

# container.volumeMounts
volumeMounts:
  - name: src-cache
    mountPath: /cache/src
```

---

## Dockerfile examples

### Perl backoffice (recommended base)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o git-lsp-mcp .

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    git \
    libperl-languageserver-perl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/git-lsp-mcp /usr/local/bin/git-lsp-mcp
CMD ["git-lsp-mcp"]
```

### TypeScript / JavaScript repos

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o git-lsp-mcp .

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y \
    ca-certificates git nodejs npm \
    && npm install -g typescript-language-server typescript \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/git-lsp-mcp /usr/local/bin/git-lsp-mcp
CMD ["git-lsp-mcp"]
```

### Go repos

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o git-lsp-mcp .

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y \
    ca-certificates git golang \
    && go install golang.org/x/tools/gopls@latest \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/git-lsp-mcp /usr/local/bin/git-lsp-mcp
CMD ["git-lsp-mcp"]
```

Build the image with only the LSP runtimes your backends need. Consider separate
per-language images if image size is a concern — a combined image including Perl,
Node, and Go runtimes will be substantially larger.

---

## config.yaml backend examples

```yaml
backends:
  - id: github_backoffice
    name: "Source: Backoffice (Perl)"
    command: "git-lsp-mcp"
    tool_prefix: "github_backoffice_"
    pool_size: 1
    env:
      REPO_URL: "https://github.com/internal/backoffice.git"
      BRANCH: "main"
      GIT_TOKEN: "${GITHUB_TOKEN}"
      CACHE_DIR: "/cache/src"
      BACKEND_ID: "github_backoffice"
      LSP: "perl-language-server"

  - id: github_driverportal
    name: "Source: Driver Portal (TypeScript)"
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

---

## Secret management

Inject GIT_TOKEN via Kubernetes Secret:

```yaml
env:
  - name: GITHUB_TOKEN
    valueFrom:
      secretKeyRef:
        name: mcp-bridge-secrets
        key: github-token
```

---

## pool_size recommendation

Always set `pool_size: 1` for source backends. LSP processes (gopls, tsserver,
perl-language-server) maintain in-memory indexes. Multiple instances per backend
inflate memory with no throughput gain.

---

## Network policy

The mcp-bridge pod must have egress to your git host (github.com or internal git).
Ensure NetworkPolicy rules permit this if your cluster uses network isolation.

---

## CEL policy (belt-and-braces allowlist)

Add to mcp-bridge `policies.yaml` for each source backend. Repeat per backend ID:

```yaml
- id: source-readonly-enforce-backoffice
  description: "Deny any source tool not in known read set"
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

