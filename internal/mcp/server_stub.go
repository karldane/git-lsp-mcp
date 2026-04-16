//go:build !integration
// +build !integration

package mcp

import (
	"context"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPServer struct{}

func NewMCPServer(workDir string) *MCPServer {
	return &MCPServer{}
}

func (s *MCPServer) RegisterTool(handler ToolHandler) {}

func (s *MCPServer) Start() error { return nil }

type ToolHandler interface {
	Name() string
	Description() string
	Schema() string
	Handle(ctx context.Context, args map[string]interface{}) (string, error)
	GetEnforcerProfile() *EnforcerProfile
}

type EnforcerProfile struct {
	RiskLevel    string
	ImpactScope  string
	ResourceCost int
	PIIExposure  bool
	Idempotent   bool
	ApprovalReq  bool
}

type ToolAdapter struct {
	tool interface {
		Name() string
		Description() string
		Handle(ctx context.Context, args map[string]interface{}) (string, error)
		GetEnforcerProfile() *framework.EnforcerProfile
		Schema() mcp.ToolInputSchema
	}
}

func NewToolAdapter(tool interface {
	Name() string
	Description() string
	Handle(ctx context.Context, args map[string]interface{}) (string, error)
	GetEnforcerProfile() *framework.EnforcerProfile
	Schema() mcp.ToolInputSchema
}) *ToolAdapter {
	return &ToolAdapter{tool: tool}
}

func (a *ToolAdapter) Name() string        { return a.tool.Name() }
func (a *ToolAdapter) Description() string { return a.tool.Description() }
func (a *ToolAdapter) Schema() string      { return "{}" }
func (a *ToolAdapter) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	return a.tool.Handle(ctx, args)
}
func (a *ToolAdapter) GetEnforcerProfile() *EnforcerProfile {
	profile := a.tool.GetEnforcerProfile()
	return &EnforcerProfile{
		RiskLevel:    string(profile.RiskLevel),
		ImpactScope:  string(profile.ImpactScope),
		ResourceCost: profile.ResourceCost,
		PIIExposure:  profile.PIIExposure,
		Idempotent:   profile.Idempotent,
		ApprovalReq:  profile.ApprovalReq,
	}
}
