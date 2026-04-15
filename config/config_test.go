package config

import (
	"context"
	"os"
	"testing"
)

func TestConfigLoadDefaults(t *testing.T) {
	ResetForTest()
	os.Setenv("REPO_URL", "https://github.com/test/repo.git")
	os.Setenv("CACHE_DIR", "/cache")
	os.Setenv("BACKEND_ID", "test-backend")
	os.Setenv("LSP", "gopls")

	cfg := &Config{}
	cfg.Load()

	if cfg.RepoURL != "https://github.com/test/repo.git" {
		t.Errorf("RepoURL = %q, want %q", cfg.RepoURL, "https://github.com/test/repo.git")
	}
	if cfg.CacheDir != "/cache" {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, "/cache")
	}
	if cfg.BackendID != "test-backend" {
		t.Errorf("BackendID = %q, want %q", cfg.BackendID, "test-backend")
	}
	if cfg.LSP != "gopls" {
		t.Errorf("LSP = %q, want %q", cfg.LSP, "gopls")
	}
	if cfg.Branch != "main" {
		t.Errorf("Branch = %q, want %q", cfg.Branch, "main")
	}
	if cfg.SyncOnCall {
		t.Error("SyncOnCall should be false by default")
	}
}

func TestConfigLoadAllEnv(t *testing.T) {
	ResetForTest()
	os.Setenv("REPO_URL", "https://github.com/test/repo.git")
	os.Setenv("BRANCH", "develop")
	os.Setenv("GIT_TOKEN", "secret123")
	os.Setenv("SSH_KEY_PATH", "/path/to/key")
	os.Setenv("CACHE_DIR", "/cache")
	os.Setenv("BACKEND_ID", "test-backend")
	os.Setenv("LSP", "tsserver")
	os.Setenv("LSP_BINARY", "/usr/local/bin/tsserver")
	os.Setenv("SYNC_ON_CALL", "true")

	cfg := &Config{}
	cfg.Load()

	if cfg.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", cfg.Branch, "develop")
	}
	if cfg.GitToken != "secret123" {
		t.Errorf("GitToken = %q, want %q", cfg.GitToken, "secret123")
	}
	if cfg.SSHKeyPath != "/path/to/key" {
		t.Errorf("SSHKeyPath = %q, want %q", cfg.SSHKeyPath, "/path/to/key")
	}
	if cfg.LSPBinary != "/usr/local/bin/tsserver" {
		t.Errorf("LSPBinary = %q, want %q", cfg.LSPBinary, "/usr/local/bin/tsserver")
	}
	if !cfg.SyncOnCall {
		t.Error("SyncOnCall should be true")
	}
}

func TestConfigWorkDir(t *testing.T) {
	ResetForTest()
	os.Setenv("CACHE_DIR", "/cache")
	os.Setenv("BACKEND_ID", "my-backend")

	cfg := &Config{}
	cfg.Load()

	expected := "/cache/my-backend"
	if cfg.WorkDir != expected {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, expected)
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
		errType error
	}{
		{
			name:    "missing repo URL",
			cfg:     Config{CacheDir: "/cache", BackendID: "id", LSP: "gopls"},
			wantErr: true,
			errType: ErrMissingRepoURL,
		},
		{
			name:    "missing cache dir",
			cfg:     Config{RepoURL: "https://github.com/test", BackendID: "id", LSP: "gopls"},
			wantErr: true,
			errType: ErrMissingCacheDir,
		},
		{
			name:    "missing backend ID",
			cfg:     Config{RepoURL: "https://github.com/test", CacheDir: "/cache", LSP: "gopls"},
			wantErr: true,
			errType: ErrMissingBackendID,
		},
		{
			name:    "missing LSP",
			cfg:     Config{RepoURL: "https://github.com/test", CacheDir: "/cache", BackendID: "id"},
			wantErr: true,
			errType: ErrMissingLSP,
		},
		{
			name:    "valid config",
			cfg:     Config{RepoURL: "https://github.com/test", CacheDir: "/cache", BackendID: "id", LSP: "gopls"},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfigInjectToken(t *testing.T) {
	cases := []struct {
		name     string
		repoURL  string
		token    string
		expected string
	}{
		{
			name:     "no token",
			repoURL:  "https://github.com/test/repo.git",
			token:    "",
			expected: "https://github.com/test/repo.git",
		},
		{
			name:     "inject token",
			repoURL:  "https://github.com/test/repo.git",
			token:    "ghp_token123",
			expected: "https://ghp_token123@github.com/test/repo.git",
		},
		{
			name:     "already has token",
			repoURL:  "https://ghp_existing@github.com/test/repo.git",
			token:    "ghp_newtoken",
			expected: "https://ghp_existing@github.com/test/repo.git",
		},
		{
			name:     "ssh URL unchanged",
			repoURL:  "git@github.com:test/repo.git",
			token:    "ghp_token123",
			expected: "git@github.com:test/repo.git",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{RepoURL: tc.repoURL, GitToken: tc.token}
			got := cfg.InjectToken()
			if got != tc.expected {
				t.Errorf("InjectToken() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	err := ErrMissingRepoURL
	if err.Error() != "REPO_URL is required" {
		t.Errorf("Error() = %q, want %q", err.Error(), "REPO_URL is required")
	}
}

func TestWithConfigAndFromContext(t *testing.T) {
	cfg := &Config{
		RepoURL:   "https://github.com/test",
		CacheDir:  "/cache",
		BackendID: "test",
		LSP:       "gopls",
	}

	ctx := context.Background()
	ctx = WithConfig(ctx, cfg)

	got := FromContext(ctx)
	if got.RepoURL != cfg.RepoURL {
		t.Errorf("FromContext().RepoURL = %q, want %q", got.RepoURL, cfg.RepoURL)
	}
	if got.CacheDir != cfg.CacheDir {
		t.Errorf("FromContext().CacheDir = %q, want %q", got.CacheDir, cfg.CacheDir)
	}
}

func TestFromContextDefault(t *testing.T) {
	ctx := context.Background()
	got := FromContext(ctx)
	if got == nil {
		t.Error("FromContext() returned nil")
	}
	if got.RepoURL != "" {
		t.Errorf("FromContext().RepoURL = %q, want empty", got.RepoURL)
	}
}
