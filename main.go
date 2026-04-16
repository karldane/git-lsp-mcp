package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/karldane/git-lsp-mcp/config"
	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/git-lsp-mcp/internal/lsp"
	"github.com/karldane/git-lsp-mcp/internal/mcp"
	"github.com/karldane/git-lsp-mcp/tools"
)

func init() {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if os.Getenv("DEBUG") == "true" {
		opts.Level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, opts)))
}

func main() {
	precacheMode := os.Getenv("MCP_PRECACHE") == "true"

	cfg := &config.Config{}
	cfg.Load()

	if err := cfg.Validate(); err != nil {
		if !precacheMode {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
	}

	gitClient := git.NewRealGitClient()
	lspClient := lsp.NewStdioLSPClient()
	lifecycle := NewLifecycle(gitClient, lspClient, cfg.CacheDir, cfg.BackendID)

	if precacheMode {
		slog.Info("precache mode: skipping lifecycle initialization")
	} else {
		repoURL := cfg.InjectToken()
		ctx := context.Background()
		if err := lifecycle.Initialize(ctx, repoURL, cfg.Branch, cfg.LSP, cfg.LSPBinary); err != nil {
			fmt.Fprintf(os.Stderr, "Lifecycle initialization failed: %v\n", err)
			os.Exit(1)
		}
	}

	workDir := cfg.WorkDir

	mcpServer := mcp.NewMCPServer(workDir)
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewReadFileTool(workDir)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewSearchTool(workDir)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewDirectoryTreeTool(workDir)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewDefinitionTool(workDir, lspClient)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewReferencesTool(workDir, lspClient)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewHoverTool(workDir, lspClient)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewDiagnosticsTool(workDir, lspClient)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewGitLogTool(workDir, gitClient)))
	mcpServer.RegisterTool(mcp.NewToolAdapter(tools.NewGitBlameTool(workDir, gitClient)))

	slog.Info("git-lsp-mcp server initialized",
		"backend_id", cfg.BackendID,
		"repo", cfg.RepoURL,
		"branch", cfg.Branch,
		"lsp", cfg.LSP,
		"workdir", workDir,
	)

	if err := mcpServer.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
