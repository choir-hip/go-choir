package toolregistry

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ExecutionContext contains the authoritative identity and run-derived values
// available to tools during one execution loop.
type ExecutionContext struct {
	RunID      string
	AgentID    string
	OwnerID    string
	Profile    string
	Role       string
	ChannelID  string
	SandboxID  string
	DesktopID  string
	OwnerEmail string
	WorkingDir string
	RunRecord  *types.RunRecord
}

type executionContextKey struct{}

// WithExecutionContext installs one authoritative tool execution context.
func WithExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	execution.RunID = strings.TrimSpace(execution.RunID)
	execution.AgentID = strings.TrimSpace(execution.AgentID)
	execution.OwnerID = strings.TrimSpace(execution.OwnerID)
	execution.Profile = strings.TrimSpace(execution.Profile)
	execution.Role = strings.TrimSpace(execution.Role)
	execution.ChannelID = strings.TrimSpace(execution.ChannelID)
	execution.SandboxID = strings.TrimSpace(execution.SandboxID)
	execution.DesktopID = strings.TrimSpace(execution.DesktopID)
	execution.OwnerEmail = strings.TrimSpace(execution.OwnerEmail)
	execution.WorkingDir = strings.TrimSpace(execution.WorkingDir)
	return context.WithValue(ctx, executionContextKey{}, execution)
}

// ExecutionContextFrom returns the installed tool execution context. A context
// without tool execution identity returns the zero value.
func ExecutionContextFrom(ctx context.Context) ExecutionContext {
	execution, _ := ctx.Value(executionContextKey{}).(ExecutionContext)
	return execution
}
