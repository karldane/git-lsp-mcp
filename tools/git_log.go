package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type GitLogTool struct {
	workDir string
	git     GitClientProvider
}

type GitClientProvider interface {
	Log(ctx context.Context, dir, path string, limit int) ([]git.Commit, error)
}

type GitClientWrapper struct {
	client git.GitClient
}

func (w *GitClientWrapper) Log(ctx context.Context, dir, path string, limit int) ([]git.Commit, error) {
	return w.client.Log(ctx, dir, path, limit)
}

func (w *GitClientWrapper) Blame(ctx context.Context, dir, path string) ([]git.BlameLine, error) {
	return w.client.Blame(ctx, dir, path)
}

func NewGitLogTool(workDir string, client git.GitClient) *GitLogTool {
	return &GitLogTool{
		workDir: workDir,
		git:     &GitClientWrapper{client: client},
	}
}

func NewGitLogToolWithProvider(workDir string, provider GitClientProvider) *GitLogTool {
	return &GitLogTool{
		workDir: workDir,
		git:     provider,
	}
}

func (t *GitLogTool) Name() string { return "git_log" }
func (t *GitLogTool) Description() string {
	return "Recent commit history for a file or the whole repo"
}

func (t *GitLogTool) Schema() mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Scope to a specific file (optional)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of commits (default 20, max 100)",
				"default":     20,
			},
		},
	}
}

func (t *GitLogTool) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	var path string
	if v, ok := args["path"].(string); ok {
		path = v
	}

	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
	}

	var fullPath string
	if path != "" {
		fullPath = filepath.Join(t.workDir, path)
	} else {
		fullPath = t.workDir
	}

	commits, err := t.git.Log(ctx, fullPath, path, limit)
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}

	type CommitOutput struct {
		Hash    string `json:"hash"`
		Author  string `json:"author"`
		Date    string `json:"date"`
		Message string `json:"message"`
	}

	output := make([]CommitOutput, len(commits))
	for i, c := range commits {
		output[i] = CommitOutput{
			Hash:    c.Hash,
			Author:  c.Author,
			Date:    c.AuthorTime.Format("2006-01-02T15:04:05Z07:00"),
			Message: c.Message,
		}
	}

	result := map[string]interface{}{
		"path":    path,
		"limit":   limit,
		"count":   len(commits),
		"commits": output,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	return string(data), nil
}

func (t *GitLogTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(2),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
