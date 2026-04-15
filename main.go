package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/karldane/git-lsp-mcp/config"
	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/git-lsp-mcp/internal/lsp"
	"github.com/karldane/git-lsp-mcp/tools"

	"github.com/karldane/mcp-framework/framework"
)

func main() {
	cfg := &config.Config{}
	cfg.Load()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	repoURL := cfg.InjectToken()

	gitClient := git.NewRealGitClient()
	lspClient := lsp.NewStdioLSPClient()
	lifecycle := NewLifecycle(gitClient, lspClient, cfg.CacheDir, cfg.BackendID)

	ctx := context.Background()
	if err := lifecycle.Initialize(ctx, repoURL, cfg.Branch, cfg.LSP, cfg.LSPBinary); err != nil {
		fmt.Fprintf(os.Stderr, "Lifecycle initialization failed: %v\n", err)
		os.Exit(1)
	}

	serverConfig := &framework.Config{
		Name:    "github.com/karldane/git-lsp-mcp",
		Version: "1.0.0",
		Instructions: `Git LSP MCP Server

Provides semantic source code analysis for a git repository.
All tools are read-only and operate on the cloned workspace.

Required environment variables:
- REPO_URL: Full git clone URL
- BRANCH: Branch to track (default: main)
- CACHE_DIR: Base path for workspace cache
- BACKEND_ID: Unique identifier for this backend
- LSP: Language server (gopls, tsserver, pyright, rust-analyzer)`,
	}

	srv := framework.NewServerWithConfig(serverConfig)

	workDir := cfg.WorkDir
	srv.RegisterTool(tools.NewReadFileTool(workDir))
	srv.RegisterTool(tools.NewSearchTool(workDir))
	srv.RegisterTool(tools.NewDirectoryTreeTool(workDir))
	srv.RegisterTool(tools.NewDefinitionTool(workDir, lspClient))
	srv.RegisterTool(tools.NewReferencesTool(workDir, lspClient))
	srv.RegisterTool(tools.NewHoverTool(workDir, lspClient))
	srv.RegisterTool(tools.NewDiagnosticsTool(workDir, lspClient))
	srv.RegisterTool(tools.NewGitLogTool(workDir, gitClient))
	srv.RegisterTool(tools.NewGitBlameTool(workDir, gitClient))

	slog.Info("git-lsp-mcp server initialized",
		"backend_id", cfg.BackendID,
		"repo", cfg.RepoURL,
		"branch", cfg.Branch,
		"lsp", cfg.LSP,
		"workdir", workDir,
	)

	if err := srv.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
