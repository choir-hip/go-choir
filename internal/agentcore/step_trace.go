package agentcore

import "github.com/yusefmosiah/go-choir/internal/steptrace"

// stepTrace is the runtime's step tracker: which named step of a long
// operation (a cell, a Complete's terminal saga, the executor's freeze and
// teardown) is running now and since when. It is read through
// /api/runtime/observability so a hung operation can be located without SSH
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
// failure 3). Created on first use; the capsule executor shares it.
func (rt *Runtime) stepTrace() *steptrace.Tracker {
	if rt == nil {
		return nil
	}
	rt.stepsOnce.Do(func() { rt.steps = steptrace.New() })
	return rt.steps
}

// traceCall runs f as a named step of scope and returns its results. The
// step ends even if f panics.
func traceCall[T any](rt *Runtime, scope, name string, f func() (T, error)) (value T, err error) {
	end := rt.stepTrace().Begin(scope, name)
	defer func() { end(err) }()
	return f()
}

// traceErr runs f as a named step of scope.
func traceErr(rt *Runtime, scope, name string, f func() error) (err error) {
	end := rt.stepTrace().Begin(scope, name)
	defer func() { end(err) }()
	return f()
}
