package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type SearchTool struct {
	workDir string
	rg      Grepper
}

type Grepper interface {
	Search(ctx context.Context, dir, query, glob string, maxResults int) ([]SearchResult, error)
}

type GrepperImpl struct{}

func (GrepperImpl) Search(ctx context.Context, dir, query, glob string, maxResults int) ([]SearchResult, error) {
	args := []string{"--line-number", "--color=never"}
	if glob != "" {
		args = append(args, "--glob", glob)
	}
	args = append(args, "-e", query)
	args = append(args, dir)

	cmd := exec.CommandContext(ctx, "rg", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("rg search: %w", err)
	}

	var results []SearchResult
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i >= maxResults {
			break
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		results = append(results, SearchResult{
			Path:       parts[0],
			LineNumber: parts[1],
			Content:    parts[2],
		})
	}

	return results, nil
}

type GrepperFallback struct{}

func (GrepperFallback) Search(ctx context.Context, dir, query, glob string, maxResults int) ([]SearchResult, error) {
	var results []SearchResult

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if glob != "" && !matchesGlob(filepath.Base(path), glob) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, query) {
				relPath, _ := filepath.Rel(dir, path)
				results = append(results, SearchResult{
					Path:       relPath,
					LineNumber: fmt.Sprintf("%d", i+1),
					Content:    line,
				})
				if len(results) >= maxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, fmt.Errorf("walk: %w", err)
	}

	return results, nil
}

func matchesGlob(name, pattern string) bool {
	regex := strings.ReplaceAll(pattern, "*", ".*")
	regex = strings.ReplaceAll(regex, "?", ".")
	regex = "^" + regex + "$"
	matched, _ := regexp.MatchString(regex, name)
	return matched
}

type SearchResult struct {
	Path       string `json:"path"`
	LineNumber string `json:"line_number"`
	Content    string `json:"line_content"`
}

func NewSearchTool(workDir string) *SearchTool {
	return &SearchTool{workDir: workDir, rg: GrepperImpl{}}
}

func NewSearchToolWithGrepper(workDir string, rg Grepper) *SearchTool {
	return &SearchTool{workDir: workDir, rg: rg}
}

func (t *SearchTool) Name() string        { return "search" }
func (t *SearchTool) Description() string { return "Full-text search across the workspace" }

func (t *SearchTool) Schema() mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"glob": map[string]interface{}{
				"type":        "string",
				"description": "File pattern filter (e.g. *.ts)",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum results (default 50)",
				"default":     50,
			},
		},
		Required: []string{"query"},
	}
}

func (t *SearchTool) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	glob, _ := args["glob"].(string)
	maxResults := 50
	if v, ok := args["max_results"].(float64); ok {
		maxResults = int(v)
		if maxResults <= 0 {
			maxResults = 50
		}
	}

	results, err := t.rg.Search(ctx, t.workDir, query, glob, maxResults)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	data, err := json.Marshal(map[string]interface{}{
		"query":       query,
		"glob":        glob,
		"max_results": maxResults,
		"count":       len(results),
		"results":     results,
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	return string(data), nil
}

func (t *SearchTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
