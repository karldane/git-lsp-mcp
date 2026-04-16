package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type ReadFileTool struct {
	workDir string
	fs      FS
}

type FS interface {
	ReadFile(path string) ([]byte, error)
	Stat(path string) (os.FileInfo, error)
}

type OsFS struct{}

func (OsFS) ReadFile(path string) ([]byte, error)  { return os.ReadFile(path) }
func (OsFS) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }

func NewReadFileTool(workDir string) *ReadFileTool {
	return &ReadFileTool{workDir: workDir, fs: OsFS{}}
}

func NewReadFileToolWithFS(workDir string, fs FS) *ReadFileTool {
	return &ReadFileTool{workDir: workDir, fs: fs}
}

func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string {
	return "Read the full content of a file by relative path"
}

func (t *ReadFileTool) Schema() mcp.ToolInputSchema {
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

func (t *ReadFileTool) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(t.workDir, path)
	slog.Debug("read_file", "workDir", t.workDir, "path", path, "fullPath", fullPath)
	content, err := t.fs.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	result := map[string]interface{}{
		"path":    path,
		"content": string(content),
		"size":    len(content),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(data), nil
}

func (t *ReadFileTool) GetEnforcerProfile() *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(2),
		framework.WithPII(false),
		framework.WithIdempotent(true),
		framework.WithApprovalReq(false),
	)
}

type ReadFileToolTestFS struct {
	data map[string][]byte
	err  error
}

func NewReadFileToolTestFS() *ReadFileToolTestFS {
	return &ReadFileToolTestFS{data: make(map[string][]byte)}
}

func (f *ReadFileToolTestFS) AddFile(path string, content string) {
	f.data[path] = []byte(content)
}

func (f *ReadFileToolTestFS) ReadFile(path string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if content, ok := f.data[path]; ok {
		return content, nil
	}
	return nil, &fsError{path: path, op: "open"}
}

func (f *ReadFileToolTestFS) Stat(path string) (os.FileInfo, error) {
	return nil, nil
}

type fsError struct {
	path string
	op   string
}

func (e *fsError) Error() string { return fmt.Sprintf("%s %s: no such file", e.op, e.path) }

func ReadFileToolTest() {
	fs := NewReadFileToolTestFS()
	fs.AddFile("/test/file.txt", "hello world")
	_, _ = fs.ReadFile("/test/file.txt")
}
