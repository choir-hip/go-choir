package yaegikernel

import (
	"bytes"
	"context"
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

	return &Evaluator{
		allowlist: allowlist,
		symbols:   filteredSymbols,
	}
}

// CheckImports statically inspects Go source code and returns an error if any
// import is not permitted under the allowlist.
func (e *Evaluator) CheckImports(src string) error {
	fset := token.NewFileSet()
	// Parse as a full file or package
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
