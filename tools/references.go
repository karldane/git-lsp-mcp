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

type ReferencesTool struct {
	workDir string
	lsp     LSPClientWrapperProvider
}

type LSPClientWrapperProvider interface {
	SendDidOpen(ctx context.Context, uri, content string) error
	References(ctx context.Context, file string, line, col int) ([]lsp.Location, error)
}

func NewReferencesTool(workDir string, client lsp.LSPClient) *ReferencesTool {
	return &ReferencesTool{
		workDir: workDir,
		lsp:     &LSPClientWrapper{client: client},
	}
}

func NewReferencesToolWithProvider(workDir string, provider LSPClientWrapperProvider) *ReferencesTool {
	return &ReferencesTool{
		workDir: workDir,
		lsp:     provider,
	}
}

func (t *ReferencesTool) Name() string        { return "references" }
func (t *ReferencesTool) Description() string { return "Find all references to a symbol" }

func (t *ReferencesTool) Schema() mcp.ToolInputSchema {
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

func (t *ReferencesTool) Handle(ctx context.Context, args map[string]interface{}) (framework.ToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return framework.ToolResult{}, fmt.Errorf("path is required")
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
		return framework.ToolResult{}, fmt.Errorf("read file: %w", err)
	}

	if err := t.lsp.SendDidOpen(ctx, lsp.FileToURI(fullPath), string(content)); err != nil {
		return framework.ToolResult{}, fmt.Errorf("didOpen: %w", err)
	}

	locs, err := t.lsp.References(ctx, fullPath, line, col)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("references: %w", err)
	}

	result := map[string]interface{}{
		"path":   path,
		"line":   line,
		"column": col,
		"count":  len(locs),
	}

	if len(locs) > 0 {
		refs := make([]map[string]interface{}, len(locs))
		for i, loc := range locs {
			refs[i] = map[string]interface{}{
				"path":   loc.Path,
				"line":   loc.Line,
				"column": loc.Column,
			}
		}
		result["references"] = refs
	}

	data, err := json.Marshal(result)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("marshal: %w", err)
	}

	return framework.TextResult(string(data)), nil
}

func (t *ReferencesTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(5),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
