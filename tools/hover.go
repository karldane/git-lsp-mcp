package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karldane/git-lsp-mcp/internal/lsp"
	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type HoverTool struct {
	workDir string
	lsp     LSPClientHoverProvider
}

type LSPClientHoverProvider interface {
	SendDidOpen(ctx context.Context, uri, content string) error
	Hover(ctx context.Context, file string, line, col int) (string, error)
}

func NewHoverTool(workDir string, client lsp.LSPClient) *HoverTool {
	return &HoverTool{
		workDir: workDir,
		lsp:     &LSPClientWrapper{client: client},
	}
}

func NewHoverToolWithProvider(workDir string, provider LSPClientHoverProvider) *HoverTool {
	return &HoverTool{
		workDir: workDir,
		lsp:     provider,
	}
}

func (t *HoverTool) Name() string { return "hover" }
func (t *HoverTool) Description() string {
	return "Get type information and documentation for a symbol"
}

func (t *HoverTool) Schema() mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to repo root",
			},
			"line": map[string]interface{}{
				"type":        "integer",
				"description": "Line number (0-indexed)",
			},
			"column": map[string]interface{}{
				"type":        "integer",
				"description": "Column number (0-indexed)",
			},
		},
		Required: []string{"path", "line", "column"},
	}
}

func (t *HoverTool) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	line := 0
	if v, ok := args["line"].(float64); ok {
		line = int(v)
	}

	col := 0
	if v, ok := args["column"].(float64); ok {
		col = int(v)
	}

	fullPath := filepath.Join(t.workDir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if err := t.lsp.SendDidOpen(ctx, lsp.FileToURI(fullPath), string(content)); err != nil {
		return "", fmt.Errorf("didOpen: %w", err)
	}

	hover, err := t.lsp.Hover(ctx, fullPath, line, col)
	if err != nil {
		return "", fmt.Errorf("hover: %w", err)
	}

	result := map[string]interface{}{
		"path":   path,
		"line":   line,
		"column": col,
		"hover":  hover,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	return string(data), nil
}

func (t *HoverTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(2),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
