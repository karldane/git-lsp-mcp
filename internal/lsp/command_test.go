package lsp

import (
	"testing"
)

func TestLSPCommand(t *testing.T) {
	tests := []struct {
		name        string
		lsp         string
		wantBinary  string
		wantArgsLen int
	}{
		{
			name:        "perl-language-server",
			lsp:         "perl-language-server",
			wantBinary:  "perl",
			wantArgsLen: 3,
		},
		{
			name:        "perl alias",
			lsp:         "perl",
			wantBinary:  "perl",
			wantArgsLen: 3,
		},
		{
			name:        "gopls",
			lsp:         "gopls",
			wantBinary:  "gopls",
			wantArgsLen: 0,
		},
		{
			name:        "tsserver",
			lsp:         "tsserver",
			wantBinary:  "typescript-language-server",
			wantArgsLen: 1,
		},
		{
			name:        "typescript-language-server",
			lsp:         "typescript-language-server",
			wantBinary:  "typescript-language-server",
			wantArgsLen: 1,
		},
		{
			name:        "pyright",
			lsp:         "pyright",
			wantBinary:  "pyright-langserver",
			wantArgsLen: 1,
		},
		{
			name:        "rust-analyzer",
			lsp:         "rust-analyzer",
			wantBinary:  "rust-analyzer",
			wantArgsLen: 0,
		},
		{
			name:        "unknown treated as raw binary",
			lsp:         "/custom/path/my-lsp",
			wantBinary:  "/custom/path/my-lsp",
			wantArgsLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command(tt.lsp)
			if cmd.Binary != tt.wantBinary {
				t.Errorf("Command(%q).Binary = %q, want %q", tt.lsp, cmd.Binary, tt.wantBinary)
			}
			if len(cmd.Args) != tt.wantArgsLen {
				t.Errorf("Command(%q).Args len = %d, want %d", tt.lsp, len(cmd.Args), tt.wantArgsLen)
			}
		})
	}
}
