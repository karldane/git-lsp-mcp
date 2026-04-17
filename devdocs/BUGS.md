# Backoffice Backend Testing Findings

## Overview
The backoffice backend provides 9 LSP-style tools for repository analysis: `backoffice_repo_lsp_search`, `backoffice_repo_lsp_definition`, `backoffice_repo_lsp_references`, `backoffice_repo_lsp_hover`, `backoffice_repo_lsp_diagnostics`, `backoffice_repo_lsp_git_blame`, `backoffice_repo_lsp_read_file`, `backoffice_repo_lsp_directory_tree`, `backoffice_repo_lsp_git_log`.

## Fixed Issues

### 1. Wrong stdio framing (FIXED)
**Severity: Critical**

`git-lsp-mcp` used LSP Content-Length header parsing instead of MCP's newline-delimited JSON. All tools failed with "No response received" errors.

**Fix**: Changed `internal/mcp/server.go` to use `ReadString('\n')` + newline-delimited JSON for both reading and writing.

### 2. slog writes to stdout (FIXED)
**Severity: High**

`log/slog` default handler writes to stdout, mixing log messages with JSON responses. The MCP client couldn't parse log lines as JSON.

**Fix**: Added `init()` in `main.go` to redirect slog to stderr:
```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
```

### 3. nil context panic (FIXED)
**Severity: High**

`handleToolsCall()` called `handler.Handle(nil, ...)` which passed nil context to tools. Tools using `exec.CommandContext(nil, ...)` panicked.

**Fix**: Changed to `handler.Handle(context.Background(), ...)`.

### 4. git_blame and git_log exit 128 (FIXED)
**Severity: High**

Both tools passed a file path (e.g. `/tmp/backoffice/README.md`) as the git `-C` argument instead of the repo root (`/tmp/backoffice`).

**Fix**: Pass `t.workDir` (repo root) instead of `fullPath` (file path) to `git.Blame()` and `git.Log()`.

### 5. MCP bridge path prefix (FIXED)
**Severity: Medium**

The MCP bridge passes paths with a leading `/` (e.g. `/README.md`). Git interprets `/README.md` as an absolute path outside the repo, causing exit 128.

**Fix**: Strip leading slashes with `strings.TrimLeft(path, "/")` in both `git_blame.go` and `git_log.go`.

### 6. git_blame hash parsing wrong (FIXED)
**Severity: High**

`ObjectPrefix = "author "` matched the `author <name>` line instead of the commit hash (first line). Hash, author name, and date were all wrong.

**Fix**: Use `strings.Fields(line)` to extract `fields[0]` as the commit hash. Added `AuthorNamePrefix = "author "` for the author name.

### 7. git_blame timestamp parsing wrong (FIXED)
**Severity: High**

Used `time.Parse("1136214245", ...)` (fixed-layout format) instead of `time.Unix()`. All timestamps were zero.

**Fix**: `strconv.ParseInt(tsStr, 10, 64)` + `time.Unix(sec, 0)`.

### 8. git_blame author-mail overwrites name (FIXED)
**Severity: Low**

`author-mail` parsing overwrote the `author` name field, leaving author empty.

**Fix**: Only set `current.Author` from `author-mail` if it's currently empty.

### 9. git_log timestamp parsing wrong (FIXED)
**Severity: Medium**

Same `time.Parse` bug as blame. All `git_log` dates showed as "0001-01-01T00:00:00Z".

**Fix**: Same as blame — use `strconv.ParseInt` + `time.Unix()`.

### 10. git_blame line numbers wrong (FIXED)

Severity: Medium

All blame entries had `line_number` = 1 because the line counter was local to each git blame block and reset on each new hash.

**Fix**: Use the line number from the blame header (`fields[1]`) instead of a local counter.

### 11. Concurrent requests race on LSP stream (FIXED)

 Severity: High

 When multiple tool calls were sent concurrently, the LSP stdin/stdout stream got corrupted. Writes interleaved, and `readResponse` read async server messages instead of actual responses.

 **Fix**: Added `sendMu sync.Mutex` to serialize the entire request-response cycle in `sendRequest`. Added `readResponseWithTimeout` wrapper for timeout support. Also redirected perlnavigator subprocess stderr to `/tmp/lsp-stderr.log` to prevent subprocess output from polluting stdout.

### 12. ripgrep `-e` flag for search queries (FIXED)

 Severity: Medium

 Search queries starting with `-` (e.g., `->map`) were interpreted as ripgrep flags rather than search patterns, causing "Found argument '->' which wasn't expected" errors.

 **Fix**: Changed `args = append(args, query)` to `args = append(args, "-e", query)` in `tools/search.go`. The `-e` flag explicitly marks the next argument as a pattern, even if it starts with `-`.

### 13. Unbounded stderr log growth (FIXED)

 Severity: Low

 LSP subprocess stderr was appended to `/tmp/lsp-stderr.log` using `O_APPEND`, causing the file to grow indefinitely over time.

 **Fix**: Changed `O_APPEND` to `O_TRUNC` in `internal/lsp/client_integration.go` so the log is cleared on each LSP initialization.

### 14. Process crash detection (FIXED)

 Severity: Medium

 When a backend process crashed during a request, the error message was misleading ("No response received" or "No warm processes available") making it hard to distinguish from actual infrastructure issues.

 **Fix** (mcp-bridge):
 - Added `IsAlive()` method to `ManagedProcess` that checks if the process is still running via `proc.Cmd.ProcessState`
 - Updated `TryAcquireWarm()` and `WaitForWarmWithMax()` to detect dead processes, log a warning, and trigger respawn
 - Updated routing handlers to return clearer error messages:
   - `Backend <name> unavailable: <reason>` when pool can't provide a process
   - `Backend <name> process crashed during request, please retry` when process dies mid-request

## Verified Working Tools

| Tool | Status |
|------|--------|
| `directory_tree` | ✅ Works |
| `read_file` | ✅ Works |
| `git_log` (no path) | ✅ Works |
| `git_log` (with path) | ✅ Works |
| `search` | ✅ Works (requires ripgrep installed) |
| `git_blame` | ✅ Works |
| `definition` | ✅ Works with perlnavigator |
| `hover` | ✅ Works with perlnavigator |

## Known Limitations

| Tool | Issue | Root Cause |
|------|-------|------------|
| `references` | ❌ Fails: `Unhandled method textDocument/references` | perlnavigator 0.8.20 doesn't implement `textDocument/references` |
| `diagnostics` | ❌ Fails: `Unknown perlmethod _rpcreq_diagnostic` | perlnavigator 0.8.20 doesn't implement `textDocument/diagnostic` (pull model) |
| `directory_tree` | ⚠️ Large output | No pagination; 7.5MB+ for depth=1 in backoffice monorepo |
| `search` | ⚠️ Dependency | Requires `ripgrep` (`rg`) installed on the system |

## LSP Server: perlnavigator 0.8.20

The backend is configured to use `perlnavigator` (installed at `/usr/bin/perlnavigator`).

**Implemented capabilities:**
- `textDocument/definition` ✅
- `textDocument/hover` ✅
- `textDocument/completion`
- `textDocument/documentSymbol`
- `workspace/workspaceSymbol`
- `textDocument/formatting`
- `textDocument/signatureHelp`

**Not implemented:**
- `textDocument/references` ❌
- `textDocument/diagnostic` (pull model) ❌

### Why `definition` and `hover` work

The workspace was empty because `textDocument/didOpen` was never sent. The LSP server needs to receive `didOpen` notifications to load file contents into its workspace. The fix was to read the file content and send `textDocument/didOpen` before each `definition`/`hover` call.
