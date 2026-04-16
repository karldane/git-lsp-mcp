//go:build integration
// +build integration

package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StdioLSPClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *json.Decoder
	stdoutR io.ReadCloser
	mu      sync.Mutex
	sendMu  sync.Mutex
	reqID   int
	ready   bool
	cmdCmd  LSPCommand
}

func NewStdioLSPClient() *StdioLSPClient {
	return &StdioLSPClient{}
}

func (c *StdioLSPClient) SetCommand(cmd LSPCommand) error {
	c.cmdCmd = cmd
	return nil
}

func (c *StdioLSPClient) Initialize(ctx context.Context, rootURI string) error {
	var cmd LSPCommand
	if c.cmdCmd.Binary != "" {
		cmd = c.cmdCmd
	} else {
		binary, err := FindLSPBinary("")
		if err != nil {
			return fmt.Errorf("find lsp binary: %w", err)
		}
		cmd = LSPCommand{Binary: binary, Args: []string{"--stdio"}}
	}

	c.cmd = exec.Command(cmd.Binary, cmd.Args...)
	stderrFile, err := os.OpenFile("/tmp/lsp-stderr.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err == nil {
		c.cmd.Stderr = stderrFile
	} else {
		c.cmd.Stderr = os.Stderr
	}

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdoutR = stdout
	c.stdout = json.NewDecoder(stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start lsp: %w", err)
	}

	c.reqID = 0
	c.ready = false

	if err := c.sendInitialize(ctx, rootURI); err != nil {
		c.cmd.Process.Kill()
		return fmt.Errorf("send initialize: %w", err)
	}

	c.ready = true
	return nil
}

func (c *StdioLSPClient) SendDidOpen(ctx context.Context, uri, content string) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": "perl",
			"version":    1,
			"text":       content,
		},
	}
	c.sendNotification(ctx, "textDocument/didOpen", params)
	return nil
}

func (c *StdioLSPClient) sendInitialize(ctx context.Context, rootURI string) error {
	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition": map[string]interface{}{},
				"references": map[string]interface{}{},
				"hover":      map[string]interface{}{},
				"diagnostics": map[string]interface{}{
					"textDocument": map[string]interface{}{
						"dynamicRegistration": false,
					},
				},
			},
		},
	}

	var resp map[string]interface{}
	if err := c.sendRequest(ctx, "initialize", params, &resp); err != nil {
		return err
	}

	c.sendNotification(ctx, "initialized", map[string]interface{}{})
	return nil
}

func (c *StdioLSPClient) Definition(ctx context.Context, file string, line, col int) (*Location, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": FileToURI(file)},
		"position":     map[string]interface{}{"line": line, "character": col},
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.sendRequest(ctx, "textDocument/definition", params, &resp); err != nil {
		return nil, err
	}

	return ParseLocation(resp.Result)
}

func (c *StdioLSPClient) References(ctx context.Context, file string, line, col int) ([]Location, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": FileToURI(file)},
		"position":     map[string]interface{}{"line": line, "character": col},
		"context":      map[string]interface{}{"includeDeclaration": true},
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.sendRequest(ctx, "textDocument/references", params, &resp); err != nil {
		return nil, err
	}

	return ParseLocations(resp.Result)
}

func (c *StdioLSPClient) Hover(ctx context.Context, file string, line, col int) (string, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": FileToURI(file)},
		"position":     map[string]interface{}{"line": line, "character": col},
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.sendRequest(ctx, "textDocument/hover", params, &resp); err != nil {
		return "", err
	}

	return ParseHover(resp.Result)
}

func (c *StdioLSPClient) Diagnostics(ctx context.Context, file string) ([]Diagnostic, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": FileToURI(file)},
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.sendRequest(ctx, "textDocument/diagnostic", params, &resp); err != nil {
		return nil, err
	}

	return ParseDiagnostics(resp.Result)
}

func (c *StdioLSPClient) Shutdown() error {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	return nil
}

func (c *StdioLSPClient) sendRequest(ctx context.Context, method string, params, result interface{}) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.mu.Lock()
	c.reqID++
	id := c.reqID
	c.mu.Unlock()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	c.mu.Lock()
	c.stdin.Write([]byte("Content-Length: " + strconv.Itoa(len(data)) + "\r\n\r\n"))
	c.stdin.Write(data)
	c.mu.Unlock()

	resp, err := c.readResponseWithTimeout(ctx, 30*time.Second)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if errMsg, ok := resp["error"].(map[string]interface{}); ok {
		return fmt.Errorf("lsp error: %v", errMsg)
	}

	if resp["id"] == id {
		data, _ := json.Marshal(resp["result"])
		json.Unmarshal(data, result)
	}

	return nil
}

func (c *StdioLSPClient) readResponseWithTimeout(ctx context.Context, timeout time.Duration) (map[string]interface{}, error) {
	type result struct {
		resp map[string]interface{}
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := c.readResponse()
		done <- result{resp, err}
	}()

	select {
	case r := <-done:
		return r.resp, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for LSP response")
	}
}

func (c *StdioLSPClient) readResponse() (map[string]interface{}, error) {
	for attempts := 0; attempts < 20; attempts++ {
		reader := bufio.NewReader(c.stdoutR)

		headers := make(map[string]string)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("read header: %w", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
			}
		}

		contentLength := 0
		if len, ok := headers["content-length"]; ok && len != "" {
			contentLength, _ = strconv.Atoi(len)
		}

		var body []byte
		if contentLength > 0 {
			body = make([]byte, contentLength)
			_, err := io.ReadFull(reader, body)
			if err != nil {
				return nil, fmt.Errorf("read body: %w", err)
			}
		} else {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("read nldjson line: %w", err)
			}
			body = []byte(strings.TrimRight(line, "\r\n"))
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		if _, hasID := resp["id"]; hasID {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("too many async messages, no response")
}

func (c *StdioLSPClient) sendNotification(ctx context.Context, method string, params interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}

	data, _ := json.Marshal(notif)
	c.stdin.Write([]byte("Content-Length: " + strconv.Itoa(len(data)) + "\r\n\r\n"))
	c.stdin.Write(data)
}
