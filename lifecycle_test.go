package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/karldane/git-lsp-mcp/internal/git"
	"github.com/karldane/git-lsp-mcp/internal/lsp"
)

type fakeGitClient struct {
	cloneErr    error
	remoteErr   error
	fetchErr    error
	resetErr    error
	remoteURL   string
	cloneDir    string
	cloneCalled bool
}

func (f *fakeGitClient) Clone(ctx context.Context, url, branch, dir string) error {
	f.cloneCalled = true
	f.cloneDir = dir
	return f.cloneErr
}

func (f *fakeGitClient) RemoteURL(ctx context.Context, dir string) (string, error) {
	if f.remoteErr != nil {
		return "", f.remoteErr
	}
	return f.remoteURL, nil
}

func (f *fakeGitClient) Fetch(ctx context.Context, dir string) error {
	return f.fetchErr
}

func (f *fakeGitClient) ResetHard(ctx context.Context, dir, ref string) error {
	return f.resetErr
}

func (f *fakeGitClient) Log(ctx context.Context, dir, path string, limit int) ([]git.Commit, error) {
	return nil, nil
}

func (f *fakeGitClient) Blame(ctx context.Context, dir, path string) ([]git.BlameLine, error) {
	return nil, nil
}

type fakeLSPClient struct {
	initErr    error
	initCalled bool
	rootURI    string
}

func (f *fakeLSPClient) SetCommand(cmd lsp.LSPCommand) error {
	return nil
}

func (f *fakeLSPClient) Initialize(ctx context.Context, rootURI string) error {
	f.initCalled = true
	f.rootURI = rootURI
	return f.initErr
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

func (f *fakeLSPClient) Shutdown() error {
	return nil
}

type fakeLock struct {
	locked    bool
	lockErr   error
	unlockErr error
}

func (f *fakeLock) Lock() error {
	if f.lockErr != nil {
		return f.lockErr
	}
	f.locked = true
	return nil
}

func (f *fakeLock) Unlock() error {
	if f.unlockErr != nil {
		return f.unlockErr
	}
	f.locked = false
	return nil
}

func TestNewLifecycle(t *testing.T) {
	l := NewLifecycle(nil, nil, "/cache", "test-id")
	if l.CacheDir != "/cache" {
		t.Errorf("CacheDir = %q, want %q", l.CacheDir, "/cache")
	}
	if l.BackendID != "test-id" {
		t.Errorf("BackendID = %q, want %q", l.BackendID, "test-id")
	}
}

func TestFileLockLockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")
	lock := NewFileLock(lockPath)

	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	if !lock.flock.Locked() {
		t.Error("expected lock to be held")
	}

	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	if lock.flock.Locked() {
		t.Error("expected lock to be released")
	}
}

func TestDirExists(t *testing.T) {
	l := &Lifecycle{}
	tmpDir := t.TempDir()

	exists, err := l.dirExists(tmpDir)
	if err != nil {
		t.Fatalf("dirExists() error = %v", err)
	}
	if !exists {
		t.Error("expected directory to exist")
	}

	exists, err = l.dirExists(filepath.Join(tmpDir, "nonexistent"))
	if err != nil {
		t.Fatalf("dirExists() error = %v", err)
	}
	if exists {
		t.Error("expected directory to not exist")
	}
}

func TestLifecycleInitializeClone(t *testing.T) {
	tmpDir := t.TempDir()

	git := &fakeGitClient{}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if !git.cloneCalled {
		t.Error("expected Clone to be called")
	}
	if !lsp.initCalled {
		t.Error("expected LSP Initialize to be called")
	}
}

func TestLifecycleInitializeLockError(t *testing.T) {
	tmpDir := t.TempDir()

	git := &fakeGitClient{}
	lock := &fakeLock{lockErr: errors.New("lock failed")}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeSync(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteURL: "https://example.com/repo.git"}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if git.cloneCalled {
		t.Error("expected Clone NOT to be called (repo exists)")
	}
}

func TestLifecycleInitializeRemoteMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteURL: "https://different.com/repo.git"}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeCloneError(t *testing.T) {
	tmpDir := t.TempDir()

	git := &fakeGitClient{cloneErr: errors.New("clone failed")}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeSyncFetchError(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteURL: "https://example.com/repo.git", fetchErr: errors.New("fetch failed")}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeSyncResetError(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteURL: "https://example.com/repo.git", resetErr: errors.New("reset failed")}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeRemoteURLError(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteErr: errors.New("remote url error")}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeLSPError(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteURL: "https://example.com/repo.git"}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{initErr: errors.New("lsp init failed")}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLifecycleInitializeSyncSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	git := &fakeGitClient{remoteURL: "https://example.com/repo.git"}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "main", "gopls", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLifecycleInitializeWithCustomBranch(t *testing.T) {
	tmpDir := t.TempDir()

	git := &fakeGitClient{}
	lock := &fakeLock{}
	lsp := &fakeLSPClient{}

	l := &Lifecycle{
		Git:       git,
		LSP:       lsp,
		CacheDir:  tmpDir,
		BackendID: "repo",
		Lock:      lock,
	}

	ctx := context.Background()
	err := l.Initialize(ctx, "https://example.com/repo.git", "develop", "gopls", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !git.cloneCalled || git.cloneDir == "" {
		t.Error("expected clone to be called")
	}
}
