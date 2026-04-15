//go:build integration
// +build integration

package lsp

import (
	"fmt"
	"os/exec"
)

func FindLSPBinary(name string) (string, error) {
	if name != "" {
		if path, err := findInPath(name); err == nil {
			return path, nil
		}
	}

	names := []string{"gopls", "typescript-language-server", "pyright", "rust-analyzer"}
	for _, n := range names {
		if path, err := findInPath(n); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no LSP binary found")
}

func findInPath(name string) (string, error) {
	cmd := exec.Command("which", name)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("not found: %s", name)
	}
	return name, nil
}
