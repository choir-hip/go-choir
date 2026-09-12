package yaegikernel

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
)

// Session is a persistent Yaegi interpreter that retains variables across
// sequential eval cells within one activation (Def 2 item 2). Eval calls are
// serialized: Yaegi interpreters are not safe for concurrent use, and
// serialization also gives each cell a clean output routing.
//
// A Session is poisoned by any failed eval (timeout, overflow, panic): a
// cancelled in-flight evaluation can leave interpreter state inconsistent,
// so the session refuses further work and the broker respawns it. Poisoning
// is the session-level equivalent of the one-shot worker's process-group
// SIGKILL. Callers must create a new Session after any error.
type Session struct {
	mu        sync.Mutex
	interp    *interp.Interpreter
	allowlist *Allowlist
	stdout    *switchWriter
	stderr    *switchWriter
	poisoned  error
	closed    bool
}

// switchWriter routes interpreter output to the current cell's buffer. The
// interpreter binds its writers once at creation; the Session swaps targets
// per Eval while holding the session mutex, so routing is always unambiguous.
type switchWriter struct {
	mu     sync.Mutex
	target io.Writer
}

func (w *switchWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.target == nil {
		return len(p), nil
	}
	return w.target.Write(p)
}

func (w *switchWriter) setTarget(target io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.target = target
}

// NewSession creates a persistent interpreter with the same allowlisted
// symbol surface as NewEvaluator. Symbols are loaded once; every cell shares
// them, so prebound modules (Def 2 item 3) are bound once per activation.
func NewSession(allowlist *Allowlist, extraSymbols interp.Exports) (*Session, error) {
	if allowlist == nil {
		allowlist = NewDefaultSafeAllowlist()
	}
	stdout := &switchWriter{}
	stderr := &switchWriter{}
	i := interp.New(interp.Options{Stdout: stdout, Stderr: stderr})
	if err := i.Use(buildFilteredSymbols(allowlist, extraSymbols)); err != nil {
		return nil, fmt.Errorf("yaegi: load session symbols: %w", err)
	}
	return &Session{interp: i, allowlist: allowlist, stdout: stdout, stderr: stderr}, nil
}

// Eval runs one cell on the persistent interpreter. Cells share variables,
// imports, and definitions; a cell that fails poisons the session.
func (s *Session) Eval(ctx context.Context, src string) (EvalResult, error) {
	start := time.Now()
	res := EvalResult{}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s == nil || s.interp == nil {
		return res, fmt.Errorf("yaegi: session unavailable")
	}
	if s.closed {
		return res, fmt.Errorf("yaegi: session closed")
	}
	if s.poisoned != nil {
		return res, fmt.Errorf("yaegi: session poisoned, respawn required: %w", s.poisoned)
	}
	if err := s.checkImports(src); err != nil {
		res.Duration = time.Since(start)
		return res, err
	}
	stdoutCap := &cappedBuffer{max: maxEvalOutputBytes}
	stderrCap := &cappedBuffer{max: maxEvalOutputBytes}
	stdoutOverflow := make(chan struct{}, 1)
	stderrOverflow := make(chan struct{}, 1)
	s.stdout.setTarget(&overflowWriter{c: stdoutCap, notify: stdoutOverflow})
	s.stderr.setTarget(&overflowWriter{c: stderrCap, notify: stderrOverflow})

	outCtx, outCancel := context.WithCancel(ctx)
	defer outCancel()
	done := make(chan struct{})
	var val reflect.Value
	var evalErr error
	go func() {
		defer func() {
			if r := recover(); r != nil {
				evalErr = fmt.Errorf("yaegi panic: %v", r)
			}
			close(done)
		}()
		val, evalErr = s.interp.EvalWithContext(outCtx, src)
	}()
	overflowed := make(chan struct{}, 1)
	go func() {
		select {
		case <-stdoutOverflow:
			outCancel()
		case <-stderrOverflow:
			outCancel()
		case <-done:
			return
		}
		select {
		case overflowed <- struct{}{}:
		default:
		}
	}()

	finish := func(err error) (EvalResult, error) {
		res.Stdout = stdoutCap.String()
		res.Stderr = stderrCap.String()
		res.Value = val
		res.Duration = time.Since(start)
		if err != nil {
			s.poisoned = err
		}
		return res, err
	}
	select {
	case <-outCtx.Done():
		select {
		case <-overflowed:
			return finish(fmt.Errorf("yaegi: evaluation output exceeded limit"))
		default:
		}
		return finish(fmt.Errorf("yaegi: evaluation timed out: %w", outCtx.Err()))
	case <-done:
		select {
		case <-overflowed:
			return finish(fmt.Errorf("yaegi: evaluation output exceeded limit"))
		default:
		}
		return finish(evalErr)
	}
}

// Close retires the session. In-flight Eval calls are unaffected beyond their
// own contexts; the broker SIGKILLs the worker process for hard termination.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.stdout.setTarget(nil)
	s.stderr.setTarget(nil)
}

func (s *Session) checkImports(src string) error {
	if s == nil || s.allowlist == nil {
		return fmt.Errorf("yaegi: session allowlist unavailable")
	}
	return (&Evaluator{allowlist: s.allowlist}).CheckImports(src)
}
