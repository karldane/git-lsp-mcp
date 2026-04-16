# SPEC: Fix Stdio Framing to Use Standard MCP Transport

## Context

`git-lsp-mcp` was built using LSP (Language Server Protocol) wire framing conventions for its stdio transport, rather than the standard MCP (Model Context Protocol) newline-delimited JSON transport. This caused the server to be incompatible with `mcp-bridge` end-to-end, as mcp-bridge sends and expects newline-delimited JSON messages.

### The MCP Spec Mandate

The MCP specification (as documented in the spec reference at `https://modelcontextprotocol.io/specification/`) states:

> All messages are sent as JSON objects delimited by a newline character (`\n`).

This means:
- Input: Read one line, parse it as JSON.
- Output: Marshal to JSON, append `\n`, write to stdout.

There is **no Content-Length header**. There is **no HTTP-like header block**. The newline IS the delimiter.

### What git-lsp-mcp Currently Does (Wrong)

`internal/mcp/server.go` implements LSP-style framing:

**`readMessage()` (lines 119-157):**
1. Reads header lines (e.g., `Content-Length: 142\r\n`) until a blank line.
2. Extracts the `Content-Length` integer.
3. Reads exactly that many bytes from stdin.
4. Returns the JSON body.
5. **Errors if Content-Length header is missing** (line 156).

**`writeResponse()` (lines 272-283):**
1. Marshals the JSON-RPC response.
2. Writes `Content-Length: <N>\r\n\r\n` headers to stdout.
3. Writes the raw JSON bytes.

This is textbook LSP (Language Server Protocol) framing. MCP does not use it.

## Root Cause

The codebase name (`git-lsp-mcp`) suggests the original author intended to model the wire protocol after LSP. However, MCP and LSP are distinct protocols with different transports. LSP uses Content-Length framing; MCP uses newline-delimited JSON.

## The Fix

### File: `internal/mcp/server.go`

#### 1. Simplify `readMessage()` (lines 119-157)

**Remove:** All header parsing, Content-Length extraction, byte-count reading.

**Replace with:** Read one line using `ReadString('\n')`, trim the newline, parse as JSON.

```go
func (s *MCPServer) readMessage() (*JSONRPCMessage, error) {
    line, err := s.reader.ReadString('\n')
    if err != nil {
        return nil, err
    }
    line = strings.TrimRight(line, "\r\n")
    if line == "" {
        return nil, fmt.Errorf("empty message")
    }
    var msg JSONRPCMessage
    if err := json.Unmarshal([]byte(line), &msg); err != nil {
        return nil, err
    }
    return &msg, nil
}
```

This is a pure deletion/replacement of the 38-line function body (lines 120-157) with ~15 lines.

**Import changes:** Remove `strconv` (no longer needed). `strings` is still needed for `TrimRight`.

#### 2. Simplify `writeResponse()` (lines 272-283)

**Remove:** `fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))` and the Content-Length header writes.

**Replace with:** Write the JSON bytes followed by `\n`.

```go
func (s *MCPServer) writeResponse(resp map[string]interface{}) {
    data, err := json.Marshal(resp)
    if err != nil {
        return
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    os.Stdout.Write(data)
    os.Stdout.Write([]byte{'\n'})
}
```

**Import changes:** `fmt` is still used in `writeError` (lines 285-294) so it stays. `strconv` is no longer needed anywhere.

#### 3. Verify no other files use `strconv`

Check that `strconv` is only used in `server.go` and only in the header-parsing code being removed. If so, remove the `"strconv"` import from `server.go`.

## Exact Edits

1. **`internal/mcp/server.go:119-157`** — Replace the entire `readMessage()` function body with the simplified version above.
2. **`internal/mcp/server.go:272-283`** — Replace `writeResponse()` with the simplified version above.
3. **`internal/mcp/server.go:6-16`** — Remove `"strconv"` from imports (if confirmed unused elsewhere).

## Verification

After applying the fix:

1. **Build**: `go build -tags integration -o git-lsp-mcp ./cmd/server`
2. **Unit test** (if exists): `go test -tags integration ./...`
3. **Manual test**: Pipe a valid JSON-RPC initialize request through stdin and verify a JSON response with a trailing newline comes back:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | go run -tags integration ./cmd/server
```

Expected output: JSON response ending with `\n`, no headers.

4. **End-to-end with mcp-bridge**: After mcp-bridge's stdio framing infrastructure is also reverted (see `SPEC_REVERT_STDIO_FRAMING.md` in the mcp-bridge devdocs), restart the mcp-bridge service and verify the `backoffice` backend scans successfully in ~1 second (vs. the 10-second timeout that currently occurs).

## Dependencies

None — this change has no external dependencies. It only modifies internal I/O logic.

## Impact

- Safe, isolated change: only affects stdio transport, not tool registration, not business logic.
- After the fix, the server will be compatible with any MCP-compliant client using newline-delimited JSON transport (which is the standard).
- The server name in the response (`github.com/karldane/git-lsp-mcp`) can be updated to a more accurate name in a separate change if desired.
