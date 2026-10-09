// Command lockscope reports code that holds a mutex across blocking work
// (operational invariant O12, docs/operational-invariants-register-2026-10-08.md:
// "control-plane and store locks are never held across network calls, exec,
// boot waits or sweeps").
//
// It is syntactic (no type checking), so it sees only direct calls in the
// function that took the lock: a lock held across a helper that does the I/O
// is not reported. It is a ratchet, not a proof: `-check` fails only on
// findings that are not in the baseline, so existing debt is listed and new
// debt is refused.
//
// Ways it could be wrong, each pinned by a test:
//  1. A lock released by `defer mu.Unlock()` is held for the rest of the
//     function; missing that hides the commonest shape.
//  2. Calls inside a func literal or `go` statement run later, not under the
//     lock; reporting them is noise.
//  3. Lock, Unlock, then I/O is fine; Unlock must end the held span.
//  4. An Unlock inside an early-return branch does not release the lock for
//     the statements after the branch.
//  5. Baseline keys must not contain line numbers, or every edit above a
//     finding looks like a regression.
//  6. Test files are not product paths and are skipped.
//  7. sync.Cond.Wait under its lock is correct use, not a finding.
//
// Database statements are not reported: the store's engine and write
// mutexes exist to serialize those statements, so a statement under them is
// the protected work itself. O12's concern there (reads queueing behind
// whole-computer scans) needs a cost budget, not a lock-shape rule.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Finding is one blocking call made while a lock is held.
type Finding struct {
	File  string // repo-relative
	Line  int
	Func  string
	Lock  string // expression locked, e.g. "rt.mu"
	Class string // net, exec, sleep, chan, wait
	Call  string // normalized call text
}

// Key identifies a finding without its line number (failure mode 5).
func (f Finding) Key() string {
	return strings.Join([]string{f.File, f.Func, f.Lock, f.Class, f.Call}, " | ")
}

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.String("check", "", "baseline file: fail on findings not listed in it")
	write := flag.String("write", "", "write the current findings as a baseline to this file")
	flag.Parse()

	findings, err := Scan(*root, []string{"cmd", "internal"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "lockscope:", err)
		os.Exit(2)
	}
	if *write != "" {
		if err := writeBaseline(*write, findings); err != nil {
			fmt.Fprintln(os.Stderr, "lockscope:", err)
			os.Exit(2)
		}
		fmt.Printf("lockscope: wrote %d findings to %s\n", len(findings), *write)
		return
	}
	if *check == "" {
		for _, f := range findings {
			fmt.Printf("%s:%d %s holds %s across %s %s\n", f.File, f.Line, f.Func, f.Lock, f.Class, f.Call)
		}
		fmt.Printf("lockscope: %d findings\n", len(findings))
		return
	}
	baseline, err := readBaseline(*check)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lockscope:", err)
		os.Exit(2)
	}
	current := map[string]bool{}
	var fresh []Finding
	for _, f := range findings {
		current[f.Key()] = true
		if !baseline[f.Key()] {
			fresh = append(fresh, f)
		}
	}
	gone := 0
	for key := range baseline {
		if !current[key] {
			gone++
		}
	}
	fmt.Printf("lockscope: %d findings, %d in baseline, %d new, %d baseline entries no longer found\n",
		len(findings), len(baseline), len(fresh), gone)
	if gone > 0 {
		fmt.Printf("lockscope: shrink the baseline with: go run ./cmd/lockscope -write %s\n", *check)
	}
	for _, f := range fresh {
		fmt.Printf("NEW %s:%d %s holds %s across %s %s\n", f.File, f.Line, f.Func, f.Lock, f.Class, f.Call)
	}
	if len(fresh) > 0 {
		fmt.Println("lockscope: release the lock before blocking work (O12), or, if the hold is intended, say what the lock protects and what contends in the commit and add the key to the baseline")
		os.Exit(1)
	}
}

// Scan parses non-test Go files under the given repo-relative dirs.
func Scan(root string, dirs []string) ([]Finding, error) {
	var out []Finding
	fset := token.NewFileSet()
	for _, dir := range dirs {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			out = append(out, scanFile(fset, filepath.ToSlash(rel), file)...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

func scanFile(fset *token.FileSet, rel string, file *ast.File) []Finding {
	var out []Finding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		s := &scanner{fset: fset, file: rel, fn: funcName(fn)}
		s.block(fn.Body.List, nil)
		out = append(out, s.found...)
	}
	return out
}

type scanner struct {
	fset  *token.FileSet
	file  string
	fn    string
	found []Finding
	seen  map[string]bool
}

// block walks statements in order. held is the set of lock expressions held
// on entry; changes inside a nested block do not leak out (failure mode 4).
func (s *scanner) block(stmts []ast.Stmt, held []string) {
	held = append([]string(nil), held...)
	for _, stmt := range stmts {
		if lock, op := lockOp(stmt); op != "" {
			switch op {
			case "lock":
				held = appendUnique(held, lock)
			case "unlock":
				held = remove(held, lock)
			}
			continue
		}
		// `defer mu.Unlock()` keeps the lock to the end (failure mode 1).
		if _, ok := stmt.(*ast.DeferStmt); ok {
			continue
		}
		s.stmt(stmt, held)
	}
}

func (s *scanner) stmt(stmt ast.Stmt, held []string) {
	switch st := stmt.(type) {
	case *ast.GoStmt:
		return // runs later, not under the lock (failure mode 2)
	case *ast.BlockStmt:
		s.block(st.List, held)
	case *ast.IfStmt:
		if st.Init != nil {
			s.stmt(st.Init, held)
		}
		s.expr(st.Cond, held)
		s.block(st.Body.List, held)
		if st.Else != nil {
			s.stmt(st.Else, held)
		}
	case *ast.ForStmt:
		if st.Init != nil {
			s.stmt(st.Init, held)
		}
		if st.Cond != nil {
			s.expr(st.Cond, held)
		}
		s.block(st.Body.List, held)
	case *ast.RangeStmt:
		s.expr(st.X, held)
		s.block(st.Body.List, held)
	case *ast.SwitchStmt:
		if st.Init != nil {
			s.stmt(st.Init, held)
		}
		if st.Tag != nil {
			s.expr(st.Tag, held)
		}
		for _, c := range st.Body.List {
			s.block(c.(*ast.CaseClause).Body, held)
		}
	case *ast.TypeSwitchStmt:
		for _, c := range st.Body.List {
			s.block(c.(*ast.CaseClause).Body, held)
		}
	case *ast.SelectStmt:
		hasDefault := false
		for _, c := range st.Body.List {
			if c.(*ast.CommClause).Comm == nil {
				hasDefault = true
			}
		}
		if !hasDefault && len(held) > 0 {
			s.report(st.Pos(), held, "chan", "select")
		}
		for _, c := range st.Body.List {
			s.block(c.(*ast.CommClause).Body, held)
		}
	case *ast.LabeledStmt:
		s.stmt(st.Stmt, held)
	default:
		s.node(stmt, held)
	}
}

func (s *scanner) expr(e ast.Expr, held []string) {
	if e != nil {
		s.node(e, held)
	}
}

func (s *scanner) node(n ast.Node, held []string) {
	if len(held) == 0 {
		return
	}
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false // failure mode 2
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				s.report(x.Pos(), held, "chan", "<-"+render(x.X))
			}
		case *ast.CallExpr:
			if class, text := blockingCall(x); class != "" {
				s.report(x.Pos(), held, class, text)
			}
		}
		return true
	})
}

func (s *scanner) report(pos token.Pos, held []string, class, call string) {
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	for _, lock := range held {
		f := Finding{File: s.file, Line: s.fset.Position(pos).Line, Func: s.fn, Lock: lock, Class: class, Call: call}
		if s.seen[f.Key()] {
			continue
		}
		s.seen[f.Key()] = true
		s.found = append(s.found, f)
	}
}

// lockOp recognizes `x.Lock()`, `x.RLock()`, `x.Unlock()`, `x.RUnlock()` as
// statements.
func lockOp(stmt ast.Stmt) (string, string) {
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return "", ""
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return "", ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	switch sel.Sel.Name {
	case "Lock", "RLock":
		return render(sel.X), "lock"
	case "Unlock", "RUnlock":
		return render(sel.X), "unlock"
	}
	return "", ""
}

var pkgBlocking = map[string]map[string]string{
	"http": {"Get": "net", "Post": "net", "Head": "net", "PostForm": "net"},
	"net":  {"Dial": "net", "DialTimeout": "net"},
	"time": {"Sleep": "sleep"},
}

var methodBlocking = map[string]string{
	"RoundTrip":      "net",
	"DialContext":    "net",
	"CombinedOutput": "exec",
	"Output":         "exec",
}

// blockingCall classifies a call that blocks on I/O, a process, a sleep or a
// wait. Receivers are unknown (no type checking), so method names are kept
// specific.
func blockingCall(call *ast.CallExpr) (string, string) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	name := sel.Sel.Name
	if pkg, ok := sel.X.(*ast.Ident); ok {
		if names, ok := pkgBlocking[pkg.Name]; ok {
			if class, ok := names[name]; ok {
				return class, pkg.Name + "." + name
			}
			return "", ""
		}
	}
	recv := render(sel.X)
	lower := strings.ToLower(recv)
	if class, ok := methodBlocking[name]; ok {
		if (name == "Output" || name == "CombinedOutput") && len(call.Args) != 0 {
			return "", ""
		}
		return class, recv + "." + name
	}
	switch name {
	case "Do":
		if strings.Contains(lower, "client") && len(call.Args) == 1 {
			return "net", recv + ".Do"
		}
	case "Run", "Wait":
		if strings.Contains(lower, "cond") {
			return "", "" // failure mode 7
		}
		if len(call.Args) == 0 && (strings.Contains(lower, "cmd") || isExecCommand(sel.X)) {
			return "exec", recv + "." + name
		}
		if name == "Wait" && len(call.Args) == 0 && (strings.Contains(lower, "wg") || strings.Contains(lower, "waitgroup")) {
			return "wait", recv + ".Wait"
		}
	}
	return "", ""
}

func isExecCommand(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "exec" && strings.HasPrefix(sel.Sel.Name, "Command")
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if idx, ok := t.(*ast.IndexExpr); ok {
		t = idx.X
	}
	return render(t) + "." + fn.Name.Name
}

func render(e ast.Node) string {
	var b strings.Builder
	_ = printer.Fprint(&b, token.NewFileSet(), e)
	text := b.String()
	if len(text) > 80 {
		text = text[:80] + "..."
	}
	return strings.Join(strings.Fields(text), " ")
}

func appendUnique(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}

func remove(list []string, v string) []string {
	out := list[:0:0]
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func writeBaseline(path string, findings []Finding) error {
	keys := map[string]bool{}
	for _, f := range findings {
		keys[f.Key()] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString("# lockscope baseline (O12): known locks held across blocking work.\n")
	b.WriteString("# Shrink it; never grow it without saying what the lock protects and what contends.\n")
	b.WriteString("# Regenerate: go run ./cmd/lockscope -write cmd/lockscope/baseline.txt\n")
	for _, k := range sorted {
		b.WriteString(k)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func readBaseline(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
	}
	return out, sc.Err()
}
