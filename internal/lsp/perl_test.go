//go:build integration
// +build integration

package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPerlLanguageServer_Initialize(t *testing.T) {
	cmd := exec.Command("perl", "-MPerl::LanguageServer", "-e", "Perl::LanguageServer::run()")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start perl: %v", err)
	}
	defer cmd.Process.Kill()

	t.Logf("Perl LSP started with PID %d", cmd.Process.Pid)

	reqID := 1

	sendReq := func(method string, params map[string]interface{}) (map[string]interface{}, error) {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      reqID,
			"method":  method,
			"params":  params,
		}
		reqID++

		data, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}

		msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))
		t.Logf("Sending request: %s", msg)
		if _, err := stdin.Write([]byte(msg)); err != nil {
			return nil, fmt.Errorf("write: %w", err)
		}

		return readResponse(stdout, t)
	}

	sendNotif := func(method string, params map[string]interface{}) {
		notif := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  method,
			"params":  params,
		}
		data, _ := json.Marshal(notif)
		msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))
		t.Logf("Sending notification: %s", msg)
		stdin.Write([]byte(msg))
	}

	resp, err := sendReq("initialize", map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   "file:///tmp",
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition":  map[string]interface{}{},
				"references":  map[string]interface{}{},
				"hover":       map[string]interface{}{},
				"diagnostics": map[string]interface{}{},
			},
		},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	if errMsg, ok := resp["error"].(map[string]interface{}); ok {
		t.Fatalf("initialize returned error: %v", errMsg)
	}

	if resp["result"] == nil {
		t.Fatal("initialize returned no result")
	}

	t.Logf("Initialize result: %s", mustMarshal(resp["result"]))

	sendNotif("initialized", map[string]interface{}{})

	time.Sleep(500 * time.Millisecond)

	shutdownResp, err := sendReq("shutdown", map[string]interface{}{})
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	if shutdownResp["result"] == nil {
		t.Fatal("shutdown returned no result")
	}

	sendNotif("exit", map[string]interface{}{})

	t.Log("Perl LSP test completed successfully")
}

func readResponse(stdout io.ReadCloser, t *testing.T) (map[string]interface{}, error) {
	reader := bufio.NewReader(stdout)

	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimSuffix(line, "\r\n")
		if line == "" {
			break
		}
		t.Logf("Header: %s", line)
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			headers[strings.ToLower(parts[0])] = parts[1]
		}
	}

	contentLength := 0
	if len, ok := headers["content-length"]; ok {
		contentLength, _ = strconv.Atoi(len)
	}

	t.Logf("Content-Length: %d", contentLength)

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	t.Logf("Response body: %s", string(body))

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return resp, nil
}

func mustMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

type TestLSPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
	reqID  int
	logf   func(format string, args ...interface{})
}

func TestPerlLanguageServer_ClientLike(t *testing.T) {
	client := &TestLSPClient{}

	cmd := exec.Command("perl", "-MPerl::LanguageServer", "-e", "Perl::LanguageServer::run()")
	cmd.Stderr = os.Stderr

	var err error
	client.stdin, err = cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}

	client.stdout, err = cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start perl: %v", err)
	}
	client.cmd = cmd

	t.Logf("Started Perl LSP with PID %d", cmd.Process.Pid)

	client.logf = t.Logf

	client.mu.Lock()
	client.reqID = 0
	client.mu.Unlock()

	initializeParams := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   "file:///tmp",
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition":  map[string]interface{}{},
				"references":  map[string]interface{}{},
				"hover":       map[string]interface{}{},
				"diagnostics": map[string]interface{}{},
			},
		},
	}

	var resp map[string]interface{}
	if err := client.sendRequest("initialize", initializeParams, &resp); err != nil {
		cmd.Process.Kill()
		t.Fatalf("initialize failed: %v", err)
	}

	if errMsg, ok := resp["error"].(map[string]interface{}); ok {
		cmd.Process.Kill()
		t.Fatalf("initialize error: %v", errMsg)
	}

	t.Logf("Initialize result: %s", mustMarshal(resp["result"]))

	client.sendNotification("initialized", map[string]interface{}{})

	time.Sleep(500 * time.Millisecond)

	client.sendNotification("exit", map[string]interface{}{})
	cmd.Process.Wait()

	t.Log("Client-like test completed successfully")
}

func (c *TestLSPClient) sendRequest(method string, params, result interface{}) error {
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
		return fmt.Errorf("marshal: %w", err)
	}

	c.logf("Sending %s request id=%d", method, id)

	c.mu.Lock()
	c.stdin.Write([]byte("Content-Length: " + strconv.Itoa(len(data)) + "\r\n\r\n"))
	c.stdin.Write(data)
	c.mu.Unlock()

	resp, err := c.readResponse()
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

func (c *TestLSPClient) readResponse() (map[string]interface{}, error) {
	reader := bufio.NewReader(c.stdout)

	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		c.logf("Header line: %q", line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}

	contentLength := 0
	if len, ok := headers["content-length"]; ok {
		contentLength, _ = strconv.Atoi(len)
	}

	c.logf("Content-Length: %d", contentLength)

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	c.logf("Body: %s", string(body))

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return resp, nil
}

func (c *TestLSPClient) sendNotification(method string, params interface{}) {
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
