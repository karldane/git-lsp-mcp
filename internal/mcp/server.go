//go:build integration
// +build integration

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type ToolHandler interface {
	Name() string
	Description() string
	Schema() string
	Handle(ctx context.Context, args map[string]interface{}) (string, error)
	GetEnforcerProfile() *EnforcerProfile
}

type EnforcerProfile struct {
	RiskLevel    string
	ImpactScope  string
	ResourceCost int
	PIIExposure  bool
	Idempotent   bool
	ApprovalReq  bool
}

type ToolAdapter struct {
	tool interface {
		Name() string
		Description() string
		Handle(ctx context.Context, args map[string]interface{}) (string, error)
		GetEnforcerProfile() *framework.EnforcerProfile
		Schema() mcp.ToolInputSchema
	}
}

func NewToolAdapter(tool interface {
	Name() string
	Description() string
	Handle(ctx context.Context, args map[string]interface{}) (string, error)
	GetEnforcerProfile() *framework.EnforcerProfile
	Schema() mcp.ToolInputSchema
}) *ToolAdapter {
	return &ToolAdapter{tool: tool}
}

func (a *ToolAdapter) Name() string        { return a.tool.Name() }
func (a *ToolAdapter) Description() string { return a.tool.Description() }
func (a *ToolAdapter) Schema() string {
	schema := a.tool.Schema()
	data, _ := json.Marshal(schema)
	return string(data)
}
func (a *ToolAdapter) Handle(ctx context.Context, args map[string]interface{}) (framework.ToolResult, error) {
	return a.tool.Handle(ctx, args)
}
func (a *ToolAdapter) GetEnforcerProfile() *EnforcerProfile {
	profile := a.tool.GetEnforcerProfile()
	return &EnforcerProfile{
		RiskLevel:    string(profile.RiskLevel),
		ImpactScope:  string(profile.ImpactScope),
		ResourceCost: profile.ResourceCost,
		PIIExposure:  profile.PIIExposure,
		Idempotent:   profile.Idempotent,
		ApprovalReq:  profile.ApprovalReq,
	}
}

type MCPServer struct {
	handlers map[string]ToolHandler
	mu       sync.Mutex
	workDir  string
	reader   *bufio.Reader
}

func NewMCPServer(workDir string) *MCPServer {
	return &MCPServer{
		handlers: make(map[string]ToolHandler),
		workDir:  workDir,
		reader:   bufio.NewReaderSize(os.Stdin, 1024*1024),
	}
}

func (s *MCPServer) RegisterTool(handler ToolHandler) {
	s.handlers[handler.Name()] = handler
}

func (s *MCPServer) Start() error {
	slog.Info("MCP server starting via stdio")
	return s.serve()
}

func (s *MCPServer) serve() error {
	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			slog.Warn("read error", "error", err)
			continue
		}
		s.handleMessage(msg)
	}
}

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

type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (s *MCPServer) handleMessage(msg *JSONRPCMessage) {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg)
	case "notifications/initialized":
	case "tools/list":
		s.handleToolsList(msg)
	case "tools/call":
		s.handleToolsCall(msg)
	}
}

func (s *MCPServer) handleInitialize(msg *JSONRPCMessage) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "github.com/karldane/git-lsp-mcp",
				"version": "1.0.0",
			},
			"instructions": "Git LSP MCP Server",
		},
	}
	s.writeResponse(resp)
}

func (s *MCPServer) handleToolsList(msg *JSONRPCMessage) {
	mcpTools := make([]map[string]interface{}, 0, len(s.handlers))

	for name, handler := range s.handlers {
		profile := handler.GetEnforcerProfile()
		mcpTool := map[string]interface{}{
			"name":        name,
			"description": handler.Description(),
			"inputSchema": json.RawMessage(handler.Schema()),
			"annotations": map[string]interface{}{
				"title":          name,
				"readOnlyHint":   true,
				"idempotentHint": profile.Idempotent,
				"openWorldHint":  profile.PIIExposure,
			},
			"_meta": map[string]interface{}{
				"enforcer_profile": map[string]interface{}{
					"risk_level":    profile.RiskLevel,
					"impact_scope":  profile.ImpactScope,
					"resource_cost": profile.ResourceCost,
					"pii_exposure":  profile.PIIExposure,
					"idempotent":    profile.Idempotent,
					"approval_req":  profile.ApprovalReq,
				},
			},
		}
		mcpTools = append(mcpTools, mcpTool)
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result": map[string]interface{}{
			"tools": mcpTools,
		},
	}
	s.writeResponse(resp)
}

func (s *MCPServer) handleToolsCall(msg *JSONRPCMessage) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.writeError(msg.ID, -32602, "Invalid params")
		return
	}

	handler, ok := s.handlers[params.Name]
	if !ok {
		s.writeError(msg.ID, -32602, fmt.Sprintf("Tool not found: %s", params.Name))
		return
	}

	result, err := handler.Handle(context.Background(), params.Arguments)
	if err != nil {
		s.writeError(msg.ID, -32603, err.Error())
		return
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": result},
			},
		},
	}
	s.writeResponse(resp)
}

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

func (s *MCPServer) writeError(id json.RawMessage, code int, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	s.writeResponse(resp)
}
