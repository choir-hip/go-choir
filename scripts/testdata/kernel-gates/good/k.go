package k

import "context"

// Run is the only exported symbol.
func Run(ctx context.Context) error { return ctx.Err() }
