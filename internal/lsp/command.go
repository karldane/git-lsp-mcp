package lsp

import (
	"strings"
)

type LSPCommand struct {
	Binary string
	Args   []string
}

func Command(lsp string) LSPCommand {
	switch strings.ToLower(lsp) {
	case "perl-language-server", "perl":
		return LSPCommand{
			Binary: "perl",
			Args: []string{
				"-MPerl::LanguageServer",
				"-e", "Perl::LanguageServer::run()",
			},
		}
	case "gopls":
		return LSPCommand{
			Binary: "gopls",
			Args:   []string{},
		}
	case "tsserver", "typescript-language-server", "typescript":
		return LSPCommand{
			Binary: "typescript-language-server",
			Args:   []string{"--stdio"},
		}
	case "pyright", "pyright-langserver":
		return LSPCommand{
			Binary: "pyright-langserver",
			Args:   []string{"--stdio"},
		}
	case "rust-analyzer":
		return LSPCommand{
			Binary: "rust-analyzer",
			Args:   []string{},
		}
	default:
		return LSPCommand{
			Binary: lsp,
			Args:   []string{},
		}
	}
}
