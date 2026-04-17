package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type DirectoryTreeTool struct {
	workDir string
	fs      DirWalker
}

type DirWalker interface {
	Walk(root string, fn filepath.WalkFunc) error
	Stat(path string) (os.FileInfo, error)
}

type OsDirWalker struct{}

func (OsDirWalker) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

func (OsDirWalker) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func NewDirectoryTreeTool(workDir string) *DirectoryTreeTool {
	return &DirectoryTreeTool{workDir: workDir, fs: OsDirWalker{}}
}

func NewDirectoryTreeToolWithFS(workDir string, fs DirWalker) *DirectoryTreeTool {
	return &DirectoryTreeTool{workDir: workDir, fs: fs}
}

func (t *DirectoryTreeTool) Name() string { return "directory_tree" }
func (t *DirectoryTreeTool) Description() string {
	return "Return the directory structure of the repo root or a subdirectory"
}

func (t *DirectoryTreeTool) Schema() mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path relative to repo root (default: /)",
				"default":     "/",
			},
			"depth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum depth (default: 3)",
				"default":     3,
			},
		},
	}
}

func (t *DirectoryTreeTool) Handle(ctx context.Context, args map[string]interface{}) (framework.ToolResult, error) {
	path := "/"
	if v, ok := args["path"].(string); ok {
		path = v
	}

	depth := 3
	if v, ok := args["depth"].(float64); ok {
		depth = int(v)
		if depth < 0 {
			depth = 0
		}
	}

	fullPath := filepath.Join(t.workDir, path)
	info, err := t.fs.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return framework.ToolResult{}, fmt.Errorf("path does not exist: %s", path)
		}
		return framework.ToolResult{}, fmt.Errorf("stat: %w", err)
	}
	if !info.IsDir() {
		return framework.ToolResult{}, fmt.Errorf("path is not a directory: %s", path)
	}

	tree := buildTree(t.fs, fullPath, depth)

	data, err := json.Marshal(map[string]interface{}{
		"path":  path,
		"depth": depth,
		"tree":  tree,
	})
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("marshal: %w", err)
	}

	return framework.TextResult(string(data)), nil
}

func buildTree(fs DirWalker, root string, maxDepth int) []TreeNode {
	var nodes []TreeNode

	fs.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		parts := strings.Split(relPath, string(filepath.Separator))
		currentDepth := len(parts) - 1
		if currentDepth > maxDepth {
			return filepath.SkipDir
		}

		node := TreeNode{
			Name:     info.Name(),
			Path:     relPath,
			IsDir:    info.IsDir(),
			Children: nil,
		}

		if info.IsDir() {
			node.Children = buildTree(fs, path, maxDepth-currentDepth)
		}

		nodes = append(nodes, node)
		return nil
	})

	return nodes
}

type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Children []TreeNode `json:"children,omitempty"`
}

func (t *DirectoryTreeTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(1),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}
