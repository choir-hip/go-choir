package yaegikernel

import (
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
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

// buildFilteredSymbols merges allowlisted stdlib symbols with extra custom
// symbols (e.g. choir client bindings) for the persistent Session.
//
// A symbol whose type mentions unsafe.Pointer anywhere (a parameter, a
// result, an exported field or method of a type it names) is dropped: yaegi
// converts *T and uintptr to and from unsafe.Pointer, so one such symbol
// (sync/atomic's LoadPointer, StorePointer, SwapPointer, CompareAndSwapPointer)
// brings unsafe into a cell without the unsafe package (review r1, S1).
func buildFilteredSymbols(allowlist *Allowlist, extraSymbols interp.Exports) interp.Exports {
	filteredSymbols := make(interp.Exports)
	for pkgKey, symbols := range stdlib.Symbols {
		importPath := cleanImportPathFromSymbolKey(pkgKey)
		if err := allowlist.IsAllowed(importPath); err != nil {
			continue
		}
		kept := make(map[string]reflect.Value, len(symbols))
		for name, v := range symbols {
			if v.IsValid() && typeMentionsUnsafe(v.Type(), map[reflect.Type]bool{}) {
				continue
			}
			kept[name] = v
		}
		filteredSymbols[pkgKey] = kept
	}

	// Merge extra custom symbols (e.g. choir client bindings)
	for k, v := range extraSymbols {
		filteredSymbols[k] = v
	}
	return filteredSymbols
}

// checkImports statically inspects Go source code and returns an error if any
// import is not permitted under the allowlist.
func checkImports(allowlist *Allowlist, src string) error {
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
		if err := allowlist.IsAllowed(path); err != nil {
			return err
		}
	}
	return nil
}

// typeMentionsUnsafe reports whether t reaches unsafe.Pointer through its
// elements, function signature, exported fields or exported methods.
func typeMentionsUnsafe(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.UnsafePointer:
		return true
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		if typeMentionsUnsafe(t.Elem(), seen) {
			return true
		}
	case reflect.Map:
		if typeMentionsUnsafe(t.Key(), seen) || typeMentionsUnsafe(t.Elem(), seen) {
			return true
		}
	case reflect.Func:
		for i := 0; i < t.NumIn(); i++ {
			if typeMentionsUnsafe(t.In(i), seen) {
				return true
			}
		}
		for i := 0; i < t.NumOut(); i++ {
			if typeMentionsUnsafe(t.Out(i), seen) {
				return true
			}
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if f := t.Field(i); f.IsExported() && typeMentionsUnsafe(f.Type, seen) {
				return true
			}
		}
	}
	for i := 0; i < t.NumMethod(); i++ {
		if typeMentionsUnsafe(t.Method(i).Type, seen) {
			return true
		}
	}
	return false
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
