//go:build integration
// +build integration

package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

type StdioLSPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *json.Decoder
	mu     sync.Mutex
	reqID  int
	ready  bool
	cmdCmd LSPCommand
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
	c.cmd.Stderr = os.Stderr

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
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
		"textDocument": map[string]interface{}{"uri": fileToURI(file)},
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
		"textDocument": map[string]interface{}{"uri": fileToURI(file)},
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
		"textDocument": map[string]interface{}{"uri": fileToURI(file)},
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
		"textDocument": map[string]interface{}{"uri": fileToURI(file)},
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

	var resp map[string]interface{}
	if err := c.stdout.Decode(&resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
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
