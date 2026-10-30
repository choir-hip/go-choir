package yaegikernel

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
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
	mu sync.Mutex
	// choirSurface is the desk's exported choir function set, used to name
	// what exists when a cell fails on the surface (surface.go).
	choirSurface map[string]bool
	interp       *interp.Interpreter
	allowlist    *Allowlist
	stdout       *switchWriter
	stderr       *switchWriter
	poisoned     error
	closed       bool
	// importedPaths tracks package paths earlier successful cells installed,
	// so normalization drops repeated import decls (yaegi compiles every
	// cell as _.go in the same package — a re-import is a hard redeclared
	// error). cellSeq numbers renamed func main decls (a declared main
	// re-executes on every later cell otherwise). declaredNames tracks
	// package-scope names earlier cells installed (hoisted vars, import
	// names) so a later cell's `x := e` reuses the existing binding as an
	// assignment instead of redeclaring it.
	importedPaths map[string]bool
	declaredNames map[string]bool
	cellDeclares  []string
	cellSeq       int
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

// NewSession creates a persistent interpreter over the allowlisted symbol
// surface. Symbols are loaded once; every cell shares
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
	s := &Session{interp: i, allowlist: allowlist, stdout: stdout, stderr: stderr, importedPaths: map[string]bool{}, declaredNames: map[string]bool{}}
	if choirExports, ok := extraSymbols["choir/choir"]; ok {
		s.choirSurface = make(map[string]bool, len(choirExports))
		for name := range choirExports {
			s.choirSurface[name] = true
		}
	}
	// Predeclare the packages desks reach for in nearly every cell. Without
	// them the demo traces of 2026-10-10 lost cells on "undefined: fmt",
	// "undefined: json" and grouped-import redeclarations; a model-authored
	// import of them is dropped by normalization like any repeated import.
	for _, path := range predeclaredCellPackages {
		if s.allowlist.IsAllowed(path) != nil {
			continue
		}
		if _, err := s.Eval(context.Background(), "import \""+path+"\""); err != nil {
			s.Close()
			return nil, fmt.Errorf("yaegi: predeclare %s: %w", path, err)
		}
	}
	// Predeclare the choir binding when the symbol surface provides it: the
	// desk's entire write path goes through choir.* verbs, and requiring the
	// model to `import "choir"` in its first cell burns one compile-rejected
	// iteration per activation (observed on staging 2026-09-28: the texture
	// desk's first two cells failed `undefined: choir` before it learned to
	// import). Eval through the session's own path so importedPaths records
	// the binding and later cells drop a repeated import exactly as for any
	if _, ok := extraSymbols["choir/choir"]; ok && s.allowlist.IsAllowed("choir") == nil {
		if _, err := s.Eval(context.Background(), "import \"choir\""); err != nil {
			s.Close()
			return nil, fmt.Errorf("yaegi: predeclare choir binding: %w", err)
		}
	}
	// yaegi v0.16.1 predates Go 1.21's min and max builtins; models reach
	// for them (s[:min(n, len(s))], cluster G of
	// docs/archive/reports/cross-model-cell-errors-2026-10-13.md).
	if _, err := s.Eval(context.Background(), minMaxPrelude); err != nil {
		s.Close()
		return nil, fmt.Errorf("yaegi: predeclare min and max: %w", err)
	}
	return s, nil
}

const minMaxPrelude = `min := func(x int, rest ...int) int {
	for _, v := range rest {
		if v < x {
			x = v
		}
	}
	return x
}
max := func(x int, rest ...int) int {
	for _, v := range rest {
		if v > x {
			x = v
		}
	}
	return x
}`

// aliasChoirImports rewrites an import of the choir package by a
// module-style path ("go-choir/choir", "github.com/.../choir") to the
// predeclared "choir": models write the repo's path for it, and the
// allowlist refused every such cell (cluster E of
// docs/archive/reports/cross-model-cell-errors-2026-10-13.md). Only a path whose
// last element is exactly "choir" is rewritten.
func aliasChoirImports(src string) string {
	prefix := ""
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, parser.ImportsOnly)
	if err != nil {
		prefix = "package main\n"
		fset = token.NewFileSet()
		if file, err = parser.ParseFile(fset, "src.go", prefix+src, parser.ImportsOnly); err != nil {
			return src
		}
	}
	out := src
	for i := len(file.Imports) - 1; i >= 0; i-- {
		lit := file.Imports[i].Path
		p, err := strconv.Unquote(lit.Value)
		if err != nil || p == "choir" || path.Base(p) != "choir" {
			continue
		}
		off := fset.Position(lit.Pos()).Offset - len(prefix)
		if off < 0 || off+len(lit.Value) > len(out) {
			continue
		}
		out = out[:off] + `"choir"` + out[off+len(lit.Value):]
	}
	return out
}

// predeclaredCellPackages are imported into every session at start.
var predeclaredCellPackages = []string{"fmt", "strings", "encoding/json"}

// compileCell compiles one normalized cell, converting a compiler panic into
// a value so the cell fails as a compile error instead of crashing the
// worker.
func (s *Session) compileCell(src string) (prog *interp.Program, err error, panicErr error) {
	defer func() {
		if r := recover(); r != nil {
			panicErr = fmt.Errorf("yaegi compile panic: %v", r)
		}
	}()
	prog, err = s.interp.Compile(src)
	return prog, err, nil
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
	if s.importedPaths["choir"] {
		src = aliasChoirImports(src)
	}
	if err := s.checkImports(src); err != nil {
		res.Duration = time.Since(start)
		return res, &EvalError{err: err, Reuse: ReusePreserve, Kind: DiagImportPreflight}
	}
	if head, rest, split := s.splitNewLeadingImports(src); split {
		// Install the cell's new imports on their own, then run the
		// statements as a plain fragment (K9). An import decl executes
		// nothing, so a failure here leaves the session as it was.
		prog, err, compilePanic := s.compileCell(s.normalizeCellSource(head))
		if compilePanic != nil {
			// Nothing executed: a compile error, as below.
			res.Duration = time.Since(start)
			return res, &EvalError{err: compilePanic, Reuse: ReusePreserve, Kind: DiagCompile}
		}
		if err != nil {
			res.Duration = time.Since(start)
			return res, &EvalError{err: withCompileErrorLine(err, head), Reuse: ReusePreserve, Kind: DiagCompile}
		}
		if _, err := s.interp.Execute(prog); err != nil {
			res.Duration = time.Since(start)
			s.poisoned = err
			return res, &EvalError{err: err, Reuse: ReuseUnsafeToReuse, Kind: DiagRuntime}
		}
		s.markCellImports(head)
		src = rest
	}
	// Normalize the program-shaped sources models actually emit into the
	// fragment shape this persistent session accepts: drop the package
	// clause, drop import decls for paths already loaded, and rename
	// func main to a unique callable so it can't auto-rerun on later cells.
	src = s.normalizeCellSource(src)
	// Compile gate: a Compile error proves nothing executed (isolation matrix
	// 2026-09-09: failed Compile leaves the heap exactly intact), so the
	// session is preserved. A successful Compile may install symbols, so every
	// Execute-phase failure below poisons. Compile mirrors Eval's parse path
	// (compileSrc with inc=true), so accepted programs are identical.
	prog, err, compilePanic := s.compileCell(src)
	if compilePanic != nil {
		// yaegi's compiler can panic (compDefineX on `b, _ := pkg.F()` with
		// pkg undefined; a top-level comma-ok or `x := v[i].F`). Nothing
		// executed, so it is a compile error and the session is preserved:
		// the same defect family already preserves as "constant definition
		// loop", and a panic leaves the same zero-valued ghosts an ordinary
		// failed compile does (cluster A,
		// docs/archive/reports/cross-model-cell-errors-2026-10-13.md).
		res.Duration = time.Since(start)
		return res, &EvalError{err: withCompileErrorLine(compilePanic, src), Reuse: ReusePreserve, Kind: DiagCompile}
	}
	if err != nil {
		res.Duration = time.Since(start)
		return res, &EvalError{err: withCompileErrorLine(err, src), Reuse: ReusePreserve, Kind: DiagCompile}
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
		case <-stderrOverflow:
		case <-done:
			return
		}
		// Record the overflow before cancelling: whoever observes the
		// cancellation must also see why, or an overflow reads as a
		// timeout.
		select {
		case overflowed <- struct{}{}:
		default:
		}
		outCancel()
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
			// The cell's import decls and hoisted vars are live on the
			// session now; record them so normalization drops repeats
			// and degrades re-`:=` to assignment on later cells.
			s.markCellImports(src)
			for _, n := range s.cellDeclares {
				s.declaredNames[n] = true
			}
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

// maxShownListedValue bounds each listed name's line in a cell result.
const maxShownListedValue = maxShownCellValue

// showListedNames shows, after a successful cell, each bare name the cell
// listed before its last statement, as "=> name: value". Reading a variable
// has no side effects, so evaluating the name again is safe; a name that
// does not resolve at session scope, a package, or a func is skipped.
func (s *Session) showListedNames(source string) (out string) {
	names := listedNames(source)
	if len(names) == 0 || s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.interp == nil || s.closed || s.poisoned != nil {
		return ""
	}
	var b strings.Builder
	for _, name := range names {
		func() {
			defer func() { _ = recover() }()
			value, err := s.interp.Eval(name)
			if err != nil {
				return
			}
			if text, ok := cellValueText(value, maxShownListedValue); ok {
				b.WriteString("=> " + name + ": " + text + "\n")
			}
		}()
	}
	return b.String()
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
	return checkImports(s.allowlist, src)
}

// ErrorHint names the desk's choir surface when a cell's failure is about
// it; empty for unrelated errors.
func (s *Session) ErrorHint(src, msg string) string {
	if s == nil {
		return ""
	}
	return cellErrorHint(s.choirSurface, src, msg)
}

var compileErrorPosition = regexp.MustCompile(`^(?:[^:\s]*\.go:)?(\d+):(\d+):`)

// withCompileErrorLine appends the source line a compile error points at.
// A bare "_.go:4:176: missing ','" left the model guessing; it blamed
// transport truncation for its own `"]` typo across three cells (K11,
// docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md). The
// line is the one the kernel compiled, which can differ from the cell text
// when imports or a statement mix were normalized.
func withCompileErrorLine(err error, src string) error {
	m := compileErrorPosition.FindStringSubmatch(err.Error())
	if m == nil {
		return err
	}
	lineNo, _ := strconv.Atoi(m[1])
	col, _ := strconv.Atoi(m[2])
	lines := strings.Split(src, "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return err
	}
	line := strings.TrimRight(lines[lineNo-1], " \t\r")
	const window = 160
	prefix := ""
	if col > window {
		cut := col - window/2
		line, col, prefix = line[cut:], col-cut, "…"
	}
	if len(line) > window {
		line = line[:window] + "…"
	}
	caret := ""
	if col >= 1 && col <= len(line)+1 {
		caret = "\n  | " + strings.Repeat(" ", len(prefix)+col-1) + "^"
	}
	return fmt.Errorf("%w\n  | %s%s%s", err, prefix, line, caret)
}
