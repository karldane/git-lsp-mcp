package git

import (
	"context"
	"errors"
)

type RealGitClient struct{}

func NewRealGitClient() *RealGitClient {
	return &RealGitClient{}
}

func (g *RealGitClient) Clone(ctx context.Context, url, branch, dir string) error {
	return errors.New("git client requires integration build tag")
}

func (g *RealGitClient) RemoteURL(ctx context.Context, dir string) (string, error) {
	return "", errors.New("git client requires integration build tag")
}

func (g *RealGitClient) Fetch(ctx context.Context, dir string) error {
	return errors.New("git client requires integration build tag")
}

func (g *RealGitClient) ResetHard(ctx context.Context, dir, ref string) error {
	return errors.New("git client requires integration build tag")
}

func (g *RealGitClient) Log(ctx context.Context, dir, path string, limit int) ([]Commit, error) {
	return nil, errors.New("git client requires integration build tag")
}

func (g *RealGitClient) Blame(ctx context.Context, dir, path string) ([]BlameLine, error) {
	return nil, errors.New("git client requires integration build tag")
}
