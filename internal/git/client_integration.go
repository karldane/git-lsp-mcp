//go:build integration
// +build integration

package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RealGitClient struct{}

func NewRealGitClient() *RealGitClient {
	return &RealGitClient{}
}

func (g *RealGitClient) Clone(ctx context.Context, url, branch, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--depth", "1", url, dir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func (g *RealGitClient) RemoteURL(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url: %w", err)
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("empty remote URL")
	}
	return url, nil
}

func (g *RealGitClient) Fetch(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

func (g *RealGitClient) ResetHard(ctx context.Context, dir, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", ref)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}
	return nil
}

func (g *RealGitClient) Log(ctx context.Context, dir, path string, limit int) ([]Commit, error) {
	args := []string{"-C", dir, "log", "--format=%H%n%an%n%at%n%s", "-n", fmt.Sprintf("%d", limit)}
	if path != "" {
		args = append(args, "--", path)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []Commit
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		hash := scanner.Text()
		if !scanner.Scan() {
			break
		}
		author := scanner.Text()
		if !scanner.Scan() {
			break
		}
		timestampStr := scanner.Text()
		if !scanner.Scan() {
			break
		}
		message := scanner.Text()

		timestamp, err := time.Parse("1136214245", timestampStr)
		if err != nil {
			timestamp = time.Time{}
		}

		commits = append(commits, Commit{
			Hash:       hash,
			Author:     author,
			AuthorTime: timestamp,
			Message:    message,
		})
	}

	return commits, nil
}

func (g *RealGitClient) Blame(ctx context.Context, dir, path string) ([]BlameLine, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "blame", "--line-porcelain", "--", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame: %w", err)
	}

	var lines []BlameLine
	var current BlameLine
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, ObjectPrefix) {
			if current.Hash != "" {
				lines = append(lines, current)
			}
			hash := strings.TrimPrefix(line, ObjectPrefix)
			current = BlameLine{
				Hash:    hash,
				LineNum: 0,
			}
			lineNum = 0
			continue
		}

		if strings.HasPrefix(line, AuthorPrefix) {
			current.Author = strings.TrimPrefix(line, AuthorPrefix)
			continue
		}

		if strings.HasPrefix(line, AuthorTimePrefix) {
			ts, _ := time.Parse("1136214245", strings.TrimPrefix(line, AuthorTimePrefix))
			current.Date = ts
			continue
		}

		if strings.HasPrefix(line, "\t") {
			current.Content = strings.TrimPrefix(line, "\t")
			current.LineNum = lineNum
			lines = append(lines, current)
			current = BlameLine{}
		}
		lineNum++
	}

	return lines, nil
}
