package provideriface

import "context"

// Caller names who a model call is for, so the gateway's line for the call
// carries the desk and the run (first design, F0). It travels in headers, is
// only logged by the gateway and never decides anything there.
type Caller struct {
	RunID string
	Desk  string
}

// DeskHost is the desk of a call made by host code that no desk's run owns
// (the event layer, intake labels, a host job).
const DeskHost = "host"

// The header names the gateway reads (internal/gateway); the same names the
// search attribution uses.
const (
	CallerHeaderRun  = "X-Choir-Run"
	CallerHeaderDesk = "X-Choir-Desk"
)

type callerKey struct{}

// WithCaller returns ctx naming the caller of the model calls made with it.
// A field left empty falls back to the run's execution context.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFrom returns the caller WithCaller put in ctx (zero when none).
func CallerFrom(ctx context.Context) Caller {
	c, _ := ctx.Value(callerKey{}).(Caller)
	return c
}
