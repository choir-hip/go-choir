package yaegikernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// EvalResult holds the outcome of a Yaegi code evaluation.
type EvalResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Value    reflect.Value `json:"-"`
	Duration time.Duration `json:"duration"`
}

// ReuseDisposition answers whether the live interpreter may serve the next
// cell after a failed evaluation (settlement-gate item 1). It is orthogonal
// to DiagnosticKind: disposition is reuse safety, kind is what happened.
// Only failures proven by the isolation matrix to occur before interpreter
// entry (host import preflight, compile-phase rejection) preserve.
type ReuseDisposition string

const (
	// ReusePreserve: nothing executed; the heap is exactly intact and the
	// worker stays alive. No string matching is involved: the phase decides.
	ReusePreserve ReuseDisposition = "preserve"
	// ReuseUnsafeToReuse: the cell may have partially executed (failed
	// execution demonstrably leaves partial mutation) or the worker may be
	// dead; poison and respawn from the durable snapshot.
	ReuseUnsafeToReuse ReuseDisposition = "unsafe_to_reuse"
)

// DiagnosticKind names the failure phase with the original message preserved
// verbatim. Compile-phase rejections (syntax, type, declaration, undefined
// symbol) share DiagCompile: splitting them further would require string
// matching Yaegi messages, which the settlement contract forbids.
type DiagnosticKind string

const (
	DiagImportPreflight DiagnosticKind = "import_preflight"
	DiagCompile         DiagnosticKind = "compile"
	DiagRuntime         DiagnosticKind = "runtime"
	DiagPanic           DiagnosticKind = "panic"
	DiagTimeout         DiagnosticKind = "timeout"
	DiagOverflow        DiagnosticKind = "overflow"
	DiagWorker          DiagnosticKind = "worker"
)

// EvalError is a typed evaluation failure: reuse disposition and diagnostic
// kind travel as fields, the original message verbatim. Callers MUST switch
// on Reuse/Kind via AsEvalError; matching on message text carries no
// classification.
type EvalError struct {
	err   error
	Reuse ReuseDisposition
	Kind  DiagnosticKind
}

func (e *EvalError) Error() string { return e.err.Error() }
func (e *EvalError) Unwrap() error { return e.err }

// AsEvalError recovers the typed failure, if the error carries one.
func AsEvalError(err error) (*EvalError, bool) {
	var ee *EvalError
	if errors.As(err, &ee) {
		return ee, true
	}
	return nil, false
}

// Evaluator evaluates model-authored Go source code in a constrained Yaegi interpreter.
type Evaluator struct {
	allowlist *Allowlist
	symbols   interp.Exports
}

// NewEvaluator creates a new Evaluator with the specified allowlist and optional exported symbols.
func NewEvaluator(allowlist *Allowlist, extraSymbols interp.Exports) *Evaluator {
	if allowlist == nil {
		allowlist = NewDefaultSafeAllowlist()
	}
	return &Evaluator{
		allowlist: allowlist,
		symbols:   buildFilteredSymbols(allowlist, extraSymbols),
	}
}

// buildFilteredSymbols merges allowlisted stdlib symbols with extra custom
// symbols (e.g. choir client bindings). Shared by one-shot Evaluator and the
// persistent Session so both enforce the identical symbol surface.
func buildFilteredSymbols(allowlist *Allowlist, extraSymbols interp.Exports) interp.Exports {
	filteredSymbols := make(interp.Exports)
	for pkgKey, symbols := range stdlib.Symbols {
		importPath := cleanImportPathFromSymbolKey(pkgKey)
		if err := allowlist.IsAllowed(importPath); err == nil {
			filteredSymbols[pkgKey] = symbols
		}
	}

	// Merge extra custom symbols (e.g. choir client bindings)
	for k, v := range extraSymbols {
		filteredSymbols[k] = v
	}
	return filteredSymbols
}

// CheckImports statically inspects Go source code and returns an error if any
// import is not permitted under the allowlist.
func (e *Evaluator) CheckImports(src string) error {
	src = CleanGoSource(src)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "src.go", src, parser.ImportsOnly)
	if err != nil {
		// If src is a snippet without 'package main', wrap it to check imports
		wrapped := "package main\n" + src
		node, err = parser.ParseFile(fset, "src.go", wrapped, parser.ImportsOnly)
		if err != nil {
			// Fail closed: an import block we cannot statically parse must never
			// reach the interpreter, or the allowlist is silently bypassed.
			return fmt.Errorf("yaegi: cannot statically resolve imports (fail closed): %w", err)
		}
	}

	for _, imp := range node.Imports {
		path := imp.Path.Value
		if err := e.allowlist.IsAllowed(path); err != nil {
			return err
		}
	}
	return nil
}

// Eval executes the Go source code with timeout and output capture.
func (e *Evaluator) Eval(ctx context.Context, src string) (EvalResult, error) {
	start := time.Now()
	src = CleanGoSource(src)
	res := EvalResult{}
	// Static check first to fail fast on disallowed imports
	if err := e.CheckImports(src); err != nil {
		res.Duration = time.Since(start)
		return res, err
	}

	// Bounded, concurrency-safe output capture: a model-authored program that
	// prints indefinitely must be cut off at MaxOutputBytes (via a second worker
	// context cancel), or it can consume the capsule memory limit before the
	// broker's own post-eval cap is reached.
	stdoutCap := &cappedBuffer{max: maxEvalOutputBytes}
	stderrCap := &cappedBuffer{max: maxEvalOutputBytes}
	stdoutOverflow := make(chan struct{}, 1)
	stderrOverflow := make(chan struct{}, 1)
	wrapOverflow := func(ch chan struct{}, b *cappedBuffer) io.Writer {
		return &overflowWriter{c: b, notify: ch}
	}
	i := interp.New(interp.Options{
		Stdout: wrapOverflow(stdoutOverflow, stdoutCap),
		Stderr: wrapOverflow(stderrOverflow, stderrCap),
	})

	if err := i.Use(e.symbols); err != nil {
		res.Duration = time.Since(start)
		return res, fmt.Errorf("yaegi: load symbols: %w", err)
	}

	done := make(chan struct{})
	var val reflect.Value
	var evalErr error
	// outCtx derives from ctx so it inherits the caller's deadline but can also
	// be cancelled on output overflow. Declared before the goroutine uses it.
	outCtx, outCancel := context.WithCancel(ctx)
	defer outCancel()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				evalErr = fmt.Errorf("yaegi panic: %v", r)
			}
			close(done)
		}()
		val, evalErr = i.EvalWithContext(outCtx, src)
	}()

	// Kill the interpreter on output overflow by cancelling its own context.
	// The interpreter runs on outCtx (already declared above) so cancelling it
	// on overflow actually terminates EvalWithContext rather than leaving a
	// runaway goroutine. A watcher goroutine cancels outCtx the moment an
	// overflow fires, so the select below wakes on overflow rather than waiting
	// for the program to finish.
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

	select {
	case <-outCtx.Done():
		res.Stdout = stdoutCap.String()
		res.Stderr = stderrCap.String()
		res.Duration = time.Since(start)
		select {
		case <-overflowed:
			return res, fmt.Errorf("yaegi: evaluation output exceeded limit")
		default:
		}
		return res, fmt.Errorf("yaegi: evaluation timed out: %w", outCtx.Err())
	case <-done:
		res.Stdout = stdoutCap.String()
		res.Stderr = stderrCap.String()
		res.Value = val
		res.Duration = time.Since(start)
		select {
		case <-overflowed:
			return res, fmt.Errorf("yaegi: evaluation output exceeded limit")
		default:
			return res, evalErr
		}
	}
}

func cleanImportPathFromSymbolKey(key string) string {
	idx := strings.LastIndex(key, "/")
	if idx >= 0 {
		return key[:idx]
	}
	return key
}

// maxEvalOutputBytes bounds model-authored interpreter output so a runaway
// print loop cannot consume the capsule memory limit before the broker cap.
const maxEvalOutputBytes = 2 * 1024 * 1024 // 2 MiB

// cappedBuffer bounds total written bytes and records whether it overflowed.
type cappedBuffer struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	max  int
	full bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buf.Len()+len(p) > c.max {
		c.full = true
		remaining := c.max - c.buf.Len()
		if remaining > 0 {
			_, _ = c.buf.Write(p[:remaining])
		}
		return len(p), nil
	}
	_, _ = c.buf.Write(p)
	return len(p), nil
}

func (c *cappedBuffer) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// overflowWriter signals via notify when the underlying bounded buffer
// overflows, so Eval can cancel the interpreter promptly.
type overflowWriter struct {
	c      *cappedBuffer
	notify chan struct{}
}

func (w *overflowWriter) Write(p []byte) (int, error) {
	n, err := w.c.Write(p)
	if w.c.full {
		select {
		case w.notify <- struct{}{}:
		default:
		}
	}
	return n, err
}

// CleanGoSource strips markdown code fences (``` or ~~~) from model-authored Go source.
func CleanGoSource(s string) string {
	s = strings.TrimSpace(s)
	for _, fence := range []string{"```", "~~~"} {
		if strings.HasPrefix(s, fence) {
			if idx := strings.Index(s, "\n"); idx != -1 {
				s = s[idx+1:]
			} else {
				return ""
			}
			if idx := strings.LastIndex(s, fence); idx != -1 {
				s = s[:idx]
			}
			s = strings.TrimSpace(s)
			break
		}
	}
	return s
}
