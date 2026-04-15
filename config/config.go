package config

import (
	"context"
	"os"
	"strings"
	"sync"
)

type Config struct {
	RepoURL    string
	Branch     string
	GitToken   string
	SSHKeyPath string
	CacheDir   string
	BackendID  string
	LSP        string
	LSPBinary  string
	SyncOnCall bool
	WorkDir    string
}

func (c *Config) Load() *Config {
	initFlags.Do(func() {
		parseEnv(c)
	})

	c.WorkDir = c.CacheDir + "/" + c.BackendID

	return c
}

func parseEnv(c *Config) {
	c.Branch = "main"
	c.SyncOnCall = false

	if v := os.Getenv("REPO_URL"); v != "" {
		c.RepoURL = v
	}
	if v := os.Getenv("BRANCH"); v != "" {
		c.Branch = v
	}
	if v := os.Getenv("GIT_TOKEN"); v != "" {
		c.GitToken = v
	}
	if v := os.Getenv("SSH_KEY_PATH"); v != "" {
		c.SSHKeyPath = v
	}
	if v := os.Getenv("CACHE_DIR"); v != "" {
		c.CacheDir = v
	}
	if v := os.Getenv("BACKEND_ID"); v != "" {
		c.BackendID = v
	}
	if v := os.Getenv("LSP"); v != "" {
		c.LSP = v
	}
	if v := os.Getenv("LSP_BINARY"); v != "" {
		c.LSPBinary = v
	}
	if v := os.Getenv("SYNC_ON_CALL"); v == "true" {
		c.SyncOnCall = true
	}
}

func (c *Config) Validate() error {
	if c.RepoURL == "" {
		return ErrMissingRepoURL
	}
	if c.CacheDir == "" {
		return ErrMissingCacheDir
	}
	if c.BackendID == "" {
		return ErrMissingBackendID
	}
	if c.LSP == "" {
		return ErrMissingLSP
	}
	return nil
}

func (c *Config) InjectToken() string {
	if c.GitToken == "" {
		return c.RepoURL
	}

	if strings.HasPrefix(c.RepoURL, "https://") {
		protocolEnd := strings.Index(c.RepoURL, "://") + 3
		afterProtocol := c.RepoURL[protocolEnd:]
		slashIdx := strings.Index(afterProtocol, "/")
		if slashIdx == -1 {
			return c.RepoURL
		}
		atSlash := strings.Index(afterProtocol, "@")
		if atSlash != -1 && atSlash < slashIdx {
			return c.RepoURL
		}
		return c.RepoURL[:protocolEnd] + c.GitToken + "@" + afterProtocol
	}

	return c.RepoURL
}

type contextKey string

const configKey contextKey = "config"

func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey, cfg)
}

func FromContext(ctx context.Context) *Config {
	if cfg, ok := ctx.Value(configKey).(*Config); ok {
		return cfg
	}
	return &Config{}
}

var initFlags sync.Once

func ResetForTest() {
	initFlags = sync.Once{}
}

var (
	ErrMissingRepoURL   = &ConfigError{"REPO_URL is required"}
	ErrMissingCacheDir  = &ConfigError{"CACHE_DIR is required"}
	ErrMissingBackendID = &ConfigError{"BACKEND_ID is required"}
	ErrMissingLSP       = &ConfigError{"LSP is required"}
)

type ConfigError struct{ msg string }

func (e *ConfigError) Error() string { return e.msg }
