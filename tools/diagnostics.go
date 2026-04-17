package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/karldane/git-lsp-mcp/internal/lsp"
	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type DiagnosticsTool struct {
	workDir string
	lsp     LSPClientDiagnosticsProvider
}

type LSPClientDiagnosticsProvider interface {
	SendDidOpen(ctx context.Context, uri, content string) error
	Diagnostics(ctx context.Context, file string) ([]lsp.Diagnostic, error)
}

func NewDiagnosticsTool(workDir string, client lsp.LSPClient) *DiagnosticsTool {
	return &DiagnosticsTool{
		workDir: workDir,
		lsp:     &LSPClientWrapper{client: client},
	}
}

func NewDiagnosticsToolWithProvider(workDir string, provider LSPClientDiagnosticsProvider) *DiagnosticsTool {
	return &DiagnosticsTool{
		workDir: workDir,
		lsp:     provider,
	}
}

func (t *DiagnosticsTool) Name() string { return "diagnostics" }
func (t *DiagnosticsTool) Description() string {
	return "Get LSP diagnostics (errors, warnings) for a file"
}

func (t *DiagnosticsTool) Schema() mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to repo root",
			},
		},
		Required: []string{"path"},
	}
}

func (t *DiagnosticsTool) Handle(ctx context.Context, args map[string]interface{}) (framework.ToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return framework.ToolResult{}, fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(t.workDir, path)
	diags, err := t.lsp.Diagnostics(ctx, fullPath)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("diagnostics: %w", err)
	}

	result := map[string]interface{}{
		"path":  path,
		"count": len(diags),
	}

	if len(diags) > 0 {
		result["diagnostics"] = diags
	}

	data, err := json.Marshal(result)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("marshal: %w", err)
	}

	return framework.TextResult(string(data)), nil
}

func (t *DiagnosticsTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
