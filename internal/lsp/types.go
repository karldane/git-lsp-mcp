package lsp

import "context"

type Location struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Snippet string `json:"source_snippet,omitempty"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

type LSPClient interface {
	SetCommand(cmd LSPCommand) error
	Initialize(ctx context.Context, rootURI string) error
	SendDidOpen(ctx context.Context, uri, content string) error
	Definition(ctx context.Context, file string, line, col int) (*Location, error)
	References(ctx context.Context, file string, line, col int) ([]Location, error)
	Hover(ctx context.Context, file string, line, col int) (string, error)
	Diagnostics(ctx context.Context, file string) ([]Diagnostic, error)
	Shutdown() error
}
