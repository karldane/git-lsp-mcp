# git-lsp-mcp

A Go MCP server backend for `mcp-bridge` that provides semantic source code analysis
by managing a git-backed workspace and an LSP (Language Server Protocol) subprocess.

## What it does

- Clones and syncs a target git repository to a pod-scoped cache on first use
- Starts and manages an LSP child process (gopls, tsserver, pyright, rust-analyzer, perl)
- Exposes read-only MCP tools for file reading, semantic search, symbol resolution,
  diagnostics, and git history
- Integrates with mcp-bridge's process pool, idle GC, and EnforcerProfile safety system

## Quick start

```bash
# Build
make build

# Run tests (requires git + LSP binary in PATH)
make test

# Build Linux binary for deployment
make build-linux
```

## Configuration

Set via environment variables injected by mcp-bridge config.yaml:

| Variable     | Required | Description                                   |
|--------------|----------|-----------------------------------------------|
| REPO_URL     | Yes      | Git clone URL (https or ssh)                  |
| BRANCH       | No       | Branch to track. Default: main                |
| GIT_TOKEN    | No       | PAT for HTTPS auth                            |
| CACHE_DIR    | Yes      | Pod-scoped cache base path e.g. /cache/src    |
| BACKEND_ID   | Yes      | Unique backend id, used as cache subdirectory |
| LSP          | Yes      | LSP binary name: gopls, tsserver, pyright, rust-analyzer, perl |
| LSP_BINARY   | No       | Override resolved binary path                 |
| SYNC_ON_CALL | No       | Re-pull before every tool call. Default: false|

## Available tools

| Tool           | Description                                       |
|----------------|---------------------------------------------------|
| read_file      | Read file content by relative path                |
| search         | Full-text search (ripgrep / stdlib fallback)      |
| directory_tree | Directory structure to configurable depth         |
| definition     | LSP textDocument/definition — follow a symbol     |
| references     | LSP textDocument/references — all usages          |
| hover          | LSP textDocument/hover — type info & docs         |
| diagnostics    | LSP textDocument/diagnostic — errors & warnings   |
| git_log        | Recent commit history for file or whole repo      |
| git_blame      | Line-by-line authorship for a file                |

## Dependencies

- github.com/karlDane/mcp-framework — MCP server, ToolHandler, EnforcerProfile
- github.com/gofrs/flock            — cross-platform file locking
- github.com/mark3labs/mcp-go       — MCP protocol types (transitive)

No CGo. No C compiler required.

## See also

- SPEC_GIT_LSP_MCP.md  — full architecture specification
- DEPLOYMENT_NOTES.md  — mcp-bridge integration and Kubernetes setup
