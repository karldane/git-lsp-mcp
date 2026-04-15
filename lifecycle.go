package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/git-lsp-mcp/internal/lsp"

	"github.com/gofrs/flock"
)

type Lifecycle struct {
	Git       git.GitClient
	LSP       lsp.LSPClient
	Lock      Locker
	CacheDir  string
	BackendID string
}

type Locker interface {
	Lock() error
	Unlock() error
}

type FileLock struct {
	flock *flock.Flock
}

func NewFileLock(path string) *FileLock {
	return &FileLock{
		flock: flock.New(path),
	}
}

func (l *FileLock) Lock() error {
	return l.flock.Lock()
}

func (l *FileLock) Unlock() error {
	return l.flock.Unlock()
}

func NewLifecycle(gitClient git.GitClient, lspClient lsp.LSPClient, cacheDir, backendID string) *Lifecycle {
	return &Lifecycle{
		Git:       gitClient,
		LSP:       lspClient,
		Lock:      nil,
		CacheDir:  cacheDir,
		BackendID: backendID,
	}
}

func (l *Lifecycle) Initialize(ctx context.Context, repoURL, branch, lspName, lspBinary string) error {
	workDir := filepath.Join(l.CacheDir, l.BackendID)
	lockPath := workDir + ".lock"

	var lock Locker
	if l.Lock != nil {
		lock = l.Lock
	} else {
		lock = NewFileLock(lockPath)
	}

	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	l.Lock = lock

	defer func() {
		if l.Lock != nil {
			l.Lock.Unlock()
		}
	}()

	exists, err := l.dirExists(workDir)
	if err != nil {
		return fmt.Errorf("check workdir: %w", err)
	}

	if !exists {
		slog.Info("cloning repository", "url", repoURL, "branch", branch, "dir", workDir)
		if err := l.Git.Clone(ctx, repoURL, branch, workDir); err != nil {
			return fmt.Errorf("clone repo: %w", err)
		}
	} else {
		slog.Info("syncing existing repository", "dir", workDir)
		remoteURL, err := l.Git.RemoteURL(ctx, workDir)
		if err != nil {
			return fmt.Errorf("get remote url: %w", err)
		}
		if remoteURL != repoURL {
			return fmt.Errorf("remote mismatch: got %q, want %q", remoteURL, repoURL)
		}
		if err := l.Git.Fetch(ctx, workDir); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		if err := l.Git.ResetHard(ctx, workDir, "origin/"+branch); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	}

	l.Lock.Unlock()
	l.Lock = nil

	cmd := lsp.Command(lspName)
	if lspBinary != "" {
		cmd.Binary = lspBinary
	}

	slog.Info("starting LSP", "lsp", lspName, "binary", cmd.Binary, "args", cmd.Args)
	if err := l.LSP.SetCommand(cmd); err != nil {
		return fmt.Errorf("set lsp command: %w", err)
	}
	if err := l.LSP.Initialize(ctx, "file://"+workDir); err != nil {
		return fmt.Errorf("init lsp: %w", err)
	}

	return nil
}

func (l *Lifecycle) dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}
