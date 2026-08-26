package yaegikernel

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
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

	var stdoutBuf, stderrBuf bytes.Buffer
	i := interp.New(interp.Options{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	})

	if err := i.Use(e.symbols); err != nil {
		res.Duration = time.Since(start)
		return res, fmt.Errorf("yaegi: load symbols: %w", err)
	}

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
		val, evalErr = i.EvalWithContext(ctx, src)
	}()

	select {
	case <-ctx.Done():
		res.Stdout = stdoutBuf.String()
		res.Stderr = stderrBuf.String()
		res.Duration = time.Since(start)
		return res, fmt.Errorf("yaegi: evaluation timed out: %w", ctx.Err())
	case <-done:
		res.Stdout = stdoutBuf.String()
		res.Stderr = stderrBuf.String()
		res.Value = val
		res.Duration = time.Since(start)
		return res, evalErr
	}
}

func cleanImportPathFromSymbolKey(key string) string {
	idx := strings.LastIndex(key, "/")
	if idx >= 0 {
		return key[:idx]
	}
	return key
}
