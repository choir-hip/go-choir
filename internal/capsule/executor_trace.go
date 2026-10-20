//go:build linux

package capsule

import "github.com/yusefmosiah/go-choir/internal/steptrace"

// SetStepTrace gives the executor the runtime's step tracker, so the freeze,
// digest and teardown steps that can stall a Complete are visible through
// /api/runtime/observability while they run
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
// failure 3). Without one the steps are untraced.
func (e *Executor) SetStepTrace(tracker *steptrace.Tracker) {
	e.trace.Store(tracker)
}

// beginStep starts a named step of scope (a capsule id) and returns its end.
func (e *Executor) beginStep(scope, name string) func(error) {
	return e.trace.Load().Begin(scope, name)
}

// traceStep runs f as a named step of scope. The step ends even if f panics.
func (e *Executor) traceStep(scope, name string, f func() error) (err error) {
	end := e.beginStep(scope, name)
	defer func() { end(err) }()
	return f()
}
