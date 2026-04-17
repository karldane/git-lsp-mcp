package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type GitBlameTool struct {
	workDir string
	git     GitClientBlameProvider
}

type GitClientBlameProvider interface {
	Blame(ctx context.Context, dir, path string) ([]git.BlameLine, error)
}

func NewGitBlameTool(workDir string, client git.GitClient) *GitBlameTool {
	return &GitBlameTool{
		workDir: workDir,
		git:     &GitClientWrapper{client: client},
	}
}

func NewGitBlameToolWithProvider(workDir string, provider GitClientBlameProvider) *GitBlameTool {
	return &GitBlameTool{
		workDir: workDir,
		git:     provider,
	}
}

func (t *GitBlameTool) Name() string        { return "git_blame" }
func (t *GitBlameTool) Description() string { return "Line-by-line authorship for a file" }

func (t *GitBlameTool) Schema() mcp.ToolInputSchema {
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

func (t *GitBlameTool) Handle(ctx context.Context, args map[string]interface{}) (framework.ToolResult, error) {
	path, _ := args["path"].(string)
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return framework.ToolResult{}, fmt.Errorf("path is required")
	}

	lines, err := t.git.Blame(ctx, t.workDir, path)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("git blame: %w", err)
	}

	type BlameOutput struct {
		LineNumber int    `json:"line_number"`
		Hash       string `json:"hash"`
		Author     string `json:"author"`
		Date       string `json:"date"`
		Content    string `json:"content"`
	}

	output := make([]BlameOutput, len(lines))
	for i, l := range lines {
		dateStr := ""
		if !l.Date.IsZero() {
			dateStr = l.Date.Format("2006-01-02T15:04:05Z07:00")
		}
		output[i] = BlameOutput{
			LineNumber: l.LineNum,
			Hash:       l.Hash,
			Author:     l.Author,
			Date:       dateStr,
			Content:    l.Content,
		}
	}

	result := map[string]interface{}{
		"path":  path,
		"count": len(lines),
		"blame": output,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("marshal: %w", err)
	}

	return framework.TextResult(string(data)), nil
}

func (t *GitBlameTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
