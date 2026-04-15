//go:build !integration

package lsp

import (
	"context"
	"errors"
)

type StdioLSPClient struct{}

func NewStdioLSPClient() *StdioLSPClient {
	return &StdioLSPClient{}
}

func (c *StdioLSPClient) SetCommand(cmd LSPCommand) error {
	return nil
}

func (c *StdioLSPClient) Initialize(ctx context.Context, rootURI string) error {
	return errors.New("lsp client requires integration build tag")
}

func (c *StdioLSPClient) Definition(ctx context.Context, file string, line, col int) (*Location, error) {
	return nil, errors.New("lsp client requires integration build tag")
}

func (c *StdioLSPClient) References(ctx context.Context, file string, line, col int) ([]Location, error) {
	return nil, errors.New("lsp client requires integration build tag")
}

func (c *StdioLSPClient) Hover(ctx context.Context, file string, line, col int) (string, error) {
	return "", errors.New("lsp client requires integration build tag")
}

func (c *StdioLSPClient) Diagnostics(ctx context.Context, file string) ([]Diagnostic, error) {
	return nil, errors.New("lsp client requires integration build tag")
}

func (c *StdioLSPClient) Shutdown() error {
	return nil
}
