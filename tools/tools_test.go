package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/git-lsp-mcp/internal/lsp"
)

func TestReadFileTool(t *testing.T) {
	fs := &ReadFileToolTestFS{data: map[string][]byte{
		"/test.txt": []byte("hello world"),
	}}
	tool := NewReadFileToolWithFS("/test", fs)

	if tool.Name() != "read_file" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "read_file")
	}

	profile := tool.GetEnforcerProfile()
	if profile == nil {
		t.Fatal("GetEnforcerProfile() returned nil")
	}
	if profile.ImpactScope != "read" {
		t.Errorf("ImpactScope = %q, want %q", profile.ImpactScope, "read")
	}
}

func TestReadFileToolDescription(t *testing.T) {
	fs := &ReadFileToolTestFS{data: map[string][]byte{}}
	tool := NewReadFileToolWithFS("/test", fs)
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestReadFileToolSchema(t *testing.T) {
	fs := &ReadFileToolTestFS{data: map[string][]byte{}}
	tool := NewReadFileToolWithFS("/test", fs)
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestReadFileToolProductionConstructor(t *testing.T) {
	tool := NewReadFileTool("/test")
	if tool.Name() != "read_file" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "read_file")
	}
}

func TestReadFileToolHandle(t *testing.T) {
	cases := []struct {
		name       string
		args       map[string]interface{}
		wantErr    bool
		wantOutput bool
	}{
		{
			name:       "valid file",
			args:       map[string]interface{}{"path": "test.txt"},
			wantErr:    false,
			wantOutput: true,
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "empty path",
			args:    map[string]interface{}{"path": ""},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := &ReadFileToolTestFS{data: map[string][]byte{
				"/test/test.txt": []byte("hello world"),
			}}
			tool := NewReadFileToolWithFS("/test", fs)

			result, err := tool.Handle(context.Background(), tc.args)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if tc.wantOutput && len(result.Content) == 0 {
				t.Error("expected output, got empty")
			}
		})
	}
}

func TestSearchTool(t *testing.T) {
	fakeGrepper := &fakeGrepper{results: []SearchResult{
		{Path: "a.go", LineNumber: "1", Content: "func a()"},
	}}
	tool := NewSearchToolWithGrepper("/test", fakeGrepper)

	if tool.Name() != "search" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "search")
	}

	profile := tool.GetEnforcerProfile()
	if profile == nil {
		t.Fatal("GetEnforcerProfile() returned nil")
	}
}

func TestSearchToolDescription(t *testing.T) {
	tool := NewSearchToolWithGrepper("/test", &fakeGrepper{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestSearchToolSchema(t *testing.T) {
	tool := NewSearchToolWithGrepper("/test", &fakeGrepper{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

type fakeGrepper struct {
	results []SearchResult
	err     error
}

func (f *fakeGrepper) Search(ctx context.Context, dir, query, glob string, maxResults int) ([]SearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func TestSearchToolHandle(t *testing.T) {
	cases := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid query",
			args: map[string]interface{}{"query": "test"},
		},
		{
			name:    "missing query",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "with glob",
			args: map[string]interface{}{"query": "test", "glob": "*.go"},
		},
		{
			name: "with max_results",
			args: map[string]interface{}{"query": "test", "max_results": float64(10)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeGrepper := &fakeGrepper{results: []SearchResult{}}
			tool := NewSearchToolWithGrepper("/test", fakeGrepper)

			_, err := tool.Handle(context.Background(), tc.args)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Handle() error = %v", err)
			}
		})
	}
}

func TestSearchToolHandleError(t *testing.T) {
	fakeGrepper := &fakeGrepper{err: errors.New("search error")}
	tool := NewSearchToolWithGrepper("/test", fakeGrepper)

	_, err := tool.Handle(context.Background(), map[string]interface{}{"query": "test"})
	if err == nil {
		t.Error("expected error from grepper")
	}
}

func TestMatchesGlob(t *testing.T) {
	cases := []struct {
		name    string
		name_   string
		pattern string
		want    bool
	}{
		{"exact match", "test.go", "test.go", true},
		{"wildcard extension", "test.go", "*.go", true},
		{"wildcard prefix", "test.go", "test.*", true},
		{"no match", "test.ts", "*.go", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesGlob(tc.name_, tc.pattern)
			if got != tc.want {
				t.Errorf("matchesGlob(%q, %q) = %v, want %v", tc.name_, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestDirectoryTreeTool(t *testing.T) {
	fakeWalker := &fakeDirWalker{}
	tool := NewDirectoryTreeToolWithFS("/test", fakeWalker)

	if tool.Name() != "directory_tree" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "directory_tree")
	}

	profile := tool.GetEnforcerProfile()
	if profile == nil {
		t.Fatal("GetEnforcerProfile() returned nil")
	}
}

func TestDirectoryTreeToolDescription(t *testing.T) {
	tool := NewDirectoryTreeToolWithFS("/test", &fakeDirWalker{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestDirectoryTreeToolSchema(t *testing.T) {
	tool := NewDirectoryTreeToolWithFS("/test", &fakeDirWalker{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestDirectoryTreeToolHandle(t *testing.T) {
	fs := &fakeDirWalkerWithFiles{
		files: map[string]os.FileInfo{
			"/test": fakeFileInfo{name: "test", isDir: true},
		},
	}
	tool := NewDirectoryTreeToolWithFS("/test", fs)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "/",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["path"] != "/" {
		t.Errorf("path = %v, want /", output["path"])
	}
}

func TestDirectoryTreeToolNotDir(t *testing.T) {
	fs := &fakeDirWalkerWithFiles{
		files: map[string]os.FileInfo{
			"/test": fakeFileInfo{name: "test.txt", isDir: false},
		},
	}
	tool := NewDirectoryTreeToolWithFS("/test", fs)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "/",
	})
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestDirectoryTreeToolNotExist(t *testing.T) {
	fs := &fakeDirWalkerWithFiles{
		files: map[string]os.FileInfo{},
	}
	tool := NewDirectoryTreeToolWithFS("/test", fs)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "/",
	})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestDirectoryTreeToolDepth(t *testing.T) {
	fs := &fakeDirWalkerWithFiles{
		files: map[string]os.FileInfo{
			"/test": fakeFileInfo{name: "test", isDir: true},
		},
	}
	tool := NewDirectoryTreeToolWithFS("/test", fs)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":  "/",
		"depth": float64(1),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["depth"].(float64) != 1 {
		t.Errorf("depth = %v, want 1", output["depth"])
	}
}

func TestDirectoryTreeToolNegativeDepth(t *testing.T) {
	fs := &fakeDirWalkerWithFiles{
		files: map[string]os.FileInfo{
			"/test": fakeFileInfo{name: "test", isDir: true},
		},
	}
	tool := NewDirectoryTreeToolWithFS("/test", fs)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":  "/",
		"depth": float64(-5),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["depth"].(float64) != 0 {
		t.Errorf("depth = %v, want 0", output["depth"])
	}
}

type fakeDirWalkerWithFiles struct {
	files map[string]os.FileInfo
}

func (f *fakeDirWalkerWithFiles) Walk(root string, fn filepath.WalkFunc) error {
	return nil
}

func (f *fakeDirWalkerWithFiles) Stat(path string) (os.FileInfo, error) {
	if info, ok := f.files[path]; ok {
		return info, nil
	}
	return nil, os.ErrNotExist
}

type fakeDirWalker struct {
	walkErr error
}

func (f *fakeDirWalker) Walk(root string, fn filepath.WalkFunc) error {
	return f.walkErr
}

func (f *fakeDirWalker) Stat(path string) (os.FileInfo, error) {
	return nil, nil
}

type fakeFileInfo struct {
	name  string
	isDir bool
	size  int64
	mode  os.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() interface{}   { return nil }

func TestDefinitionTool(t *testing.T) {
	provider := &fakeLSPProvider{}
	tool := NewDefinitionToolWithProvider("/test", provider)

	if tool.Name() != "definition" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "definition")
	}

	profile := tool.GetEnforcerProfile()
	if profile == nil {
		t.Fatal("GetEnforcerProfile() returned nil")
	}
	if profile.ResourceCost != 4 {
		t.Errorf("ResourceCost = %d, want %d", profile.ResourceCost, 4)
	}
}

func TestDefinitionToolDescription(t *testing.T) {
	tool := NewDefinitionToolWithProvider("/test", &fakeLSPProvider{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestDefinitionToolSchema(t *testing.T) {
	tool := NewDefinitionToolWithProvider("/test", &fakeLSPProvider{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestDefinitionToolProductionConstructor(t *testing.T) {
	client := &fakeLSPClient{}
	tool := NewDefinitionTool("/test", client)
	if tool.Name() != "definition" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "definition")
	}
}

func TestHoverToolProductionConstructor(t *testing.T) {
	client := &fakeLSPClient{}
	tool := NewHoverTool("/test", client)
	if tool.Name() != "hover" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "hover")
	}
}

func TestReferencesToolProductionConstructor(t *testing.T) {
	client := &fakeLSPClient{}
	tool := NewReferencesTool("/test", client)
	if tool.Name() != "references" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "references")
	}
}

func TestDiagnosticsToolProductionConstructor(t *testing.T) {
	client := &fakeLSPClient{}
	tool := NewDiagnosticsTool("/test", client)
	if tool.Name() != "diagnostics" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "diagnostics")
	}
}

func TestSearchToolProductionConstructor(t *testing.T) {
	tool := NewSearchTool("/test")
	if tool.Name() != "search" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "search")
	}
}

func TestDirectoryTreeToolProductionConstructor(t *testing.T) {
	tool := NewDirectoryTreeTool("/test")
	if tool.Name() != "directory_tree" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "directory_tree")
	}
}

func TestGitLogToolProductionConstructor(t *testing.T) {
	client := &fakeGitClient{}
	tool := NewGitLogTool("/test", client)
	if tool.Name() != "git_log" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "git_log")
	}
}

func TestGitBlameToolProductionConstructor(t *testing.T) {
	client := &fakeGitClient{}
	tool := NewGitBlameTool("/test", client)
	if tool.Name() != "git_blame" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "git_blame")
	}
}

func TestDefinitionToolHandle(t *testing.T) {
	cases := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid",
			args: map[string]interface{}{"path": "a.go", "line": float64(1), "column": float64(2)},
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{"line": float64(1), "column": float64(2)},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeLSPProvider{}
			tool := NewDefinitionToolWithProvider("/test", provider)

			_, err := tool.Handle(context.Background(), tc.args)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestReferencesTool(t *testing.T) {
	provider := &fakeLSPProvider{}
	tool := NewReferencesToolWithProvider("/test", provider)

	if tool.Name() != "references" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "references")
	}

	profile := tool.GetEnforcerProfile()
	if profile.ResourceCost != 5 {
		t.Errorf("ResourceCost = %d, want %d", profile.ResourceCost, 5)
	}
}

func TestReferencesToolDescription(t *testing.T) {
	tool := NewReferencesToolWithProvider("/test", &fakeLSPProvider{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestReferencesToolSchema(t *testing.T) {
	tool := NewReferencesToolWithProvider("/test", &fakeLSPProvider{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestHoverTool(t *testing.T) {
	provider := &fakeLSPProvider{hoverResult: "type info"}
	tool := NewHoverToolWithProvider("/test", provider)

	if tool.Name() != "hover" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "hover")
	}

	profile := tool.GetEnforcerProfile()
	if profile.ResourceCost != 2 {
		t.Errorf("ResourceCost = %d, want %d", profile.ResourceCost, 2)
	}
}

func TestHoverToolDescription(t *testing.T) {
	tool := NewHoverToolWithProvider("/test", &fakeLSPProvider{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestHoverToolSchema(t *testing.T) {
	tool := NewHoverToolWithProvider("/test", &fakeLSPProvider{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestHoverToolHandle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	provider := &fakeLSPProvider{hoverResult: "type info"}
	tool := NewHoverToolWithProvider(dir, provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":   "a.go",
		"line":   float64(1),
		"column": float64(2),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["hover"] != "type info" {
		t.Errorf("hover = %v, want %q", output["hover"], "type info")
	}
}

func TestHoverToolMissingPath(t *testing.T) {
	provider := &fakeLSPProvider{}
	tool := NewHoverToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"line":   float64(1),
		"column": float64(2),
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestHoverToolLSPError(t *testing.T) {
	provider := &fakeLSPProvider{err: errors.New("lsp error")}
	tool := NewHoverToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":   "a.go",
		"line":   float64(1),
		"column": float64(2),
	})
	if err == nil {
		t.Error("expected error from LSP")
	}
}

func TestDiagnosticsTool(t *testing.T) {
	provider := &fakeLSPProvider{}
	tool := NewDiagnosticsToolWithProvider("/test", provider)

	if tool.Name() != "diagnostics" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "diagnostics")
	}

	profile := tool.GetEnforcerProfile()
	if profile.ResourceCost != 3 {
		t.Errorf("ResourceCost = %d, want %d", profile.ResourceCost, 3)
	}
}

func TestDiagnosticsToolDescription(t *testing.T) {
	tool := NewDiagnosticsToolWithProvider("/test", &fakeLSPProvider{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestDiagnosticsToolSchema(t *testing.T) {
	tool := NewDiagnosticsToolWithProvider("/test", &fakeLSPProvider{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestGitLogTool(t *testing.T) {
	provider := &fakeGitProvider{}
	tool := NewGitLogToolWithProvider("/test", provider)

	if tool.Name() != "git_log" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "git_log")
	}

	profile := tool.GetEnforcerProfile()
	if profile == nil {
		t.Fatal("GetEnforcerProfile() returned nil")
	}
}

func TestGitLogToolDescription(t *testing.T) {
	tool := NewGitLogToolWithProvider("/test", &fakeGitProvider{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestGitLogToolSchema(t *testing.T) {
	tool := NewGitLogToolWithProvider("/test", &fakeGitProvider{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

type fakeGitProvider struct {
	logResult   []git.Commit
	blameResult []git.BlameLine
	err         error
}

func (f *fakeGitProvider) Log(ctx context.Context, dir, path string, limit int) ([]git.Commit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.logResult, nil
}

func (f *fakeGitProvider) Blame(ctx context.Context, dir, path string) ([]git.BlameLine, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.blameResult, nil
}

type fakeGitClient struct{}

func (f *fakeGitClient) Clone(ctx context.Context, url, branch, dir string) error {
	return nil
}

func (f *fakeGitClient) RemoteURL(ctx context.Context, dir string) (string, error) {
	return "", nil
}

func (f *fakeGitClient) Fetch(ctx context.Context, dir string) error {
	return nil
}

func (f *fakeGitClient) ResetHard(ctx context.Context, dir, ref string) error {
	return nil
}

func (f *fakeGitClient) Log(ctx context.Context, dir, path string, limit int) ([]git.Commit, error) {
	return nil, nil
}

func (f *fakeGitClient) Blame(ctx context.Context, dir, path string) ([]git.BlameLine, error) {
	return nil, nil
}

type fakeLSPClient struct{}

func (f *fakeLSPClient) SetCommand(cmd lsp.LSPCommand) error {
	return nil
}

func (f *fakeLSPClient) Initialize(ctx context.Context, rootURI string) error {
	return nil
}

func (f *fakeLSPClient) Definition(ctx context.Context, file string, line, col int) (*lsp.Location, error) {
	return nil, nil
}

func (f *fakeLSPClient) References(ctx context.Context, file string, line, col int) ([]lsp.Location, error) {
	return nil, nil
}

func (f *fakeLSPClient) Hover(ctx context.Context, file string, line, col int) (string, error) {
	return "", nil
}

func (f *fakeLSPClient) Diagnostics(ctx context.Context, file string) ([]lsp.Diagnostic, error) {
	return nil, nil
}

func (f *fakeLSPClient) SendDidOpen(ctx context.Context, uri, content string) error {
	return nil
}

func (f *fakeLSPClient) Shutdown() error {
	return nil
}

type fakeLSPProvider struct {
	defResult   *lsp.Location
	refsResult  []lsp.Location
	hoverResult string
	diagResult  []lsp.Diagnostic
	err         error
}

func (f *fakeLSPProvider) Definition(ctx context.Context, file string, line, col int) (*lsp.Location, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.defResult, nil
}

func (f *fakeLSPProvider) References(ctx context.Context, file string, line, col int) ([]lsp.Location, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.refsResult, nil
}

func (f *fakeLSPProvider) Hover(ctx context.Context, file string, line, col int) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.hoverResult, nil
}

func (f *fakeLSPProvider) Diagnostics(ctx context.Context, file string) ([]lsp.Diagnostic, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.diagResult, nil
}

func (f *fakeLSPProvider) SendDidOpen(ctx context.Context, uri, content string) error {
	return nil
}

func TestGitLogToolHandle(t *testing.T) {
	provider := &fakeGitProvider{
		logResult: []git.Commit{
			{Hash: "abc123", Author: "Test", Message: "test commit"},
		},
	}
	tool := NewGitLogToolWithProvider("/test", provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", output["count"])
	}
}

func TestGitLogToolHandleError(t *testing.T) {
	provider := &fakeGitProvider{err: errors.New("git error")}
	tool := NewGitLogToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"limit": float64(10),
	})
	if err == nil {
		t.Error("expected error from git provider")
	}
}

func TestGitBlameTool(t *testing.T) {
	provider := &fakeGitProvider{}
	tool := NewGitBlameToolWithProvider("/test", provider)

	if tool.Name() != "git_blame" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "git_blame")
	}

	profile := tool.GetEnforcerProfile()
	if profile == nil {
		t.Fatal("GetEnforcerProfile() returned nil")
	}
}

func TestGitBlameToolDescription(t *testing.T) {
	tool := NewGitBlameToolWithProvider("/test", &fakeGitProvider{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestGitBlameToolSchema(t *testing.T) {
	tool := NewGitBlameToolWithProvider("/test", &fakeGitProvider{})
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, "object")
	}
}

func TestGitBlameToolHandle(t *testing.T) {
	provider := &fakeGitProvider{
		blameResult: []git.BlameLine{
			{LineNum: 1, Hash: "abc123", Author: "Test", Content: "line 1"},
		},
	}
	tool := NewGitBlameToolWithProvider("/test", provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "a.go",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", output["count"])
	}
}

func TestGitBlameToolHandleError(t *testing.T) {
	provider := &fakeGitProvider{err: errors.New("blame error")}
	tool := NewGitBlameToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "a.go",
	})
	if err == nil {
		t.Error("expected error from git provider")
	}
}

func TestGitBlameToolMissingPath(t *testing.T) {
	provider := &fakeGitProvider{}
	tool := NewGitBlameToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestDefinitionToolWithDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	provider := &fakeLSPProvider{}
	tool := NewDefinitionToolWithProvider(dir, provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "a.go",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["line"].(float64) != 0 {
		t.Errorf("line = %v, want 0", output["line"])
	}
	if output["column"].(float64) != 0 {
		t.Errorf("column = %v, want 0", output["column"])
	}
}

func TestDefinitionToolWithLocation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defPath := filepath.Join(dir, "a.go")
	provider := &fakeLSPProvider{
		defResult: &lsp.Location{Path: defPath, Line: 10, Column: 5},
	}
	tool := NewDefinitionToolWithProvider(dir, provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":   "a.go",
		"line":   float64(1),
		"column": float64(2),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	def, ok := output["definition"].(map[string]interface{})
	if !ok {
		t.Fatal("expected definition in output")
	}
	if def["path"] != defPath {
		t.Errorf("definition path = %v, want %v", def["path"], defPath)
	}
}

func TestDefinitionToolLSPError(t *testing.T) {
	provider := &fakeLSPProvider{err: errors.New("lsp error")}
	tool := NewDefinitionToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":   "a.go",
		"line":   float64(1),
		"column": float64(2),
	})
	if err == nil {
		t.Error("expected error from LSP")
	}
}

func TestReferencesToolWithDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	provider := &fakeLSPProvider{}
	tool := NewReferencesToolWithProvider(dir, provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "a.go",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["line"].(float64) != 0 {
		t.Errorf("line = %v, want 0", output["line"])
	}
}

func TestReferencesToolWithResults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	provider := &fakeLSPProvider{
		refsResult: []lsp.Location{
			{Path: filepath.Join(dir, "a.go"), Line: 10, Column: 5},
			{Path: filepath.Join(dir, "b.go"), Line: 20, Column: 10},
		},
	}
	tool := NewReferencesToolWithProvider(dir, provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":   "a.go",
		"line":   float64(1),
		"column": float64(2),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["count"].(float64) != 2 {
		t.Errorf("count = %v, want 2", output["count"])
	}
}

func TestReferencesToolLSPError(t *testing.T) {
	provider := &fakeLSPProvider{err: errors.New("lsp error")}
	tool := NewReferencesToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path":   "a.go",
		"line":   float64(1),
		"column": float64(2),
	})
	if err == nil {
		t.Error("expected error from LSP")
	}
}

func TestDiagnosticsToolHandle(t *testing.T) {
	provider := &fakeLSPProvider{
		diagResult: []lsp.Diagnostic{
			{Severity: "error", Line: 1, Column: 2, Message: "err", Source: "linter"},
		},
	}
	tool := NewDiagnosticsToolWithProvider("/test", provider)

	result, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "a.go",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &output); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if output["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", output["count"])
	}
}

func TestDiagnosticsToolMissingPath(t *testing.T) {
	provider := &fakeLSPProvider{}
	tool := NewDiagnosticsToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestDiagnosticsToolLSPError(t *testing.T) {
	provider := &fakeLSPProvider{err: errors.New("lsp error")}
	tool := NewDiagnosticsToolWithProvider("/test", provider)

	_, err := tool.Handle(context.Background(), map[string]interface{}{
		"path": "a.go",
	})
	if err == nil {
		t.Error("expected error from LSP")
	}
}

func TestLSPClientWrapperDefinition(t *testing.T) {
	client := &fakeLSPClient{}
	wrapper := &LSPClientWrapper{client: client}

	_, err := wrapper.Definition(context.Background(), "a.go", 1, 2)
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
}

func TestLSPClientWrapperReferences(t *testing.T) {
	client := &fakeLSPClient{}
	wrapper := &LSPClientWrapper{client: client}

	_, err := wrapper.References(context.Background(), "a.go", 1, 2)
	if err != nil {
		t.Fatalf("References() error = %v", err)
	}
}

func TestLSPClientWrapperHover(t *testing.T) {
	client := &fakeLSPClient{}
	wrapper := &LSPClientWrapper{client: client}

	_, err := wrapper.Hover(context.Background(), "a.go", 1, 2)
	if err != nil {
		t.Fatalf("Hover() error = %v", err)
	}
}

func TestLSPClientWrapperDiagnostics(t *testing.T) {
	client := &fakeLSPClient{}
	wrapper := &LSPClientWrapper{client: client}

	_, err := wrapper.Diagnostics(context.Background(), "a.go")
	if err != nil {
		t.Fatalf("Diagnostics() error = %v", err)
	}
}
