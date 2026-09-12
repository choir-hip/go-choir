package yaegikernel

import (
	"context"
	"errors"
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
// A Session is poisoned by any failure that may have executed (runtime error,
// panic, timeout, overflow): a cancelled in-flight evaluation can leave
// interpreter state inconsistent, so the session refuses further work and the
// broker respawns it. Proven non-executing rejections (host import preflight,
// compile-phase) preserve the session. Poisoning is the session-level
// equivalent of the one-shot worker's process-group SIGKILL. Callers must
// create a new Session after any unsafe-to-reuse error.
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
// imports, and definitions. Only failures that may have executed poison the
// session; proven non-executing rejections (host import preflight,
// compile-phase) return a preserving EvalError and keep the worker alive.
// Classification is structural (which phase failed), never string matching.
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
		return res, &EvalError{err: err, Reuse: ReusePreserve, Kind: DiagImportPreflight}
	}
	// Compile gate: a Compile error proves nothing executed (isolation matrix
	// 2026-09-09: failed Compile leaves the heap exactly intact), so the
	// session is preserved. A successful Compile may install symbols, so every
	// Execute-phase failure below poisons. Compile mirrors Eval's parse path
	// (compileSrc with inc=true), so accepted programs are identical.
	prog, err := s.interp.Compile(src)
	if err != nil {
		res.Duration = time.Since(start)
		return res, &EvalError{err: err, Reuse: ReusePreserve, Kind: DiagCompile}
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
	panicked := false
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				if e, ok := r.(error); ok {
					evalErr = e
				} else {
					evalErr = fmt.Errorf("yaegi panic: %v", r)
				}
			}
			close(done)
		}()
		val, evalErr = s.interp.Execute(prog)
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

	finish := func(err error, kind DiagnosticKind) (EvalResult, error) {
		res.Stdout = stdoutCap.String()
		res.Stderr = stderrCap.String()
		if err == nil {
			res.Value = val
		}
		res.Duration = time.Since(start)
		if err != nil {
			s.poisoned = err
			return res, &EvalError{err: err, Reuse: ReuseUnsafeToReuse, Kind: kind}
		}
		return res, nil
	}
	select {
	case <-outCtx.Done():
		select {
		case <-overflowed:
			return finish(fmt.Errorf("yaegi: evaluation output exceeded limit"), DiagOverflow)
		default:
		}
		return finish(fmt.Errorf("yaegi: evaluation timed out: %w", outCtx.Err()), DiagTimeout)
	case <-done:
		select {
		case <-overflowed:
			return finish(fmt.Errorf("yaegi: evaluation output exceeded limit"), DiagOverflow)
		default:
		}
		if evalErr == nil {
			return finish(nil, "")
		}
		return finish(evalErr, classifyExecuteError(evalErr, panicked))
	}
}

// classifyExecuteError names the diagnostic kind of an Execute-phase failure
// without inspecting message text: a recovered panic (in this goroutine or as
// a Yaegi Panic value) is panic, anything else that reached execution is
// runtime. Timeout and overflow are classified structurally at the call site.
func classifyExecuteError(evalErr error, panicked bool) DiagnosticKind {
	if panicked {
		return DiagPanic
	}
	var ipanic interp.Panic
	if errors.As(evalErr, &ipanic) {
		return DiagPanic
	}
	return DiagRuntime
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
