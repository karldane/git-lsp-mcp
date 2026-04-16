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

type DefinitionTool struct {
	workDir string
	lsp     LSPClientProvider
}

type LSPClientProvider interface {
	SendDidOpen(ctx context.Context, uri, content string) error
	Definition(ctx context.Context, file string, line, col int) (*lsp.Location, error)
}

type LSPClientWrapper struct {
	client lsp.LSPClient
}

func (w *LSPClientWrapper) SendDidOpen(ctx context.Context, uri, content string) error {
	return w.client.SendDidOpen(ctx, uri, content)
}

func (w *LSPClientWrapper) Definition(ctx context.Context, file string, line, col int) (*lsp.Location, error) {
	return w.client.Definition(ctx, file, line, col)
}

func (w *LSPClientWrapper) References(ctx context.Context, file string, line, col int) ([]lsp.Location, error) {
	return w.client.References(ctx, file, line, col)
}

func (w *LSPClientWrapper) Hover(ctx context.Context, file string, line, col int) (string, error) {
	return w.client.Hover(ctx, file, line, col)
}

func (w *LSPClientWrapper) Diagnostics(ctx context.Context, file string) ([]lsp.Diagnostic, error) {
	return w.client.Diagnostics(ctx, file)
}

func NewDefinitionTool(workDir string, client lsp.LSPClient) *DefinitionTool {
	return &DefinitionTool{
		workDir: workDir,
		lsp:     &LSPClientWrapper{client: client},
	}
}

func NewDefinitionToolWithProvider(workDir string, provider LSPClientProvider) *DefinitionTool {
	return &DefinitionTool{
		workDir: workDir,
		lsp:     provider,
	}
}

func (t *DefinitionTool) Name() string { return "definition" }
func (t *DefinitionTool) Description() string {
	return "Resolve the definition of a symbol at a given file position"
}

func (t *DefinitionTool) Schema() mcp.ToolInputSchema {
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

func (t *DefinitionTool) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
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

	loc, err := t.lsp.Definition(ctx, fullPath, line, col)
	if err != nil {
		return "", fmt.Errorf("definition: %w", err)
	}

	result := map[string]interface{}{
		"path":   path,
		"line":   line,
		"column": col,
	}

	if loc != nil {
		result["definition"] = map[string]interface{}{
			"path":   loc.Path,
			"line":   loc.Line,
			"column": loc.Column,
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	return string(data), nil
}

func (t *DefinitionTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(4),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
