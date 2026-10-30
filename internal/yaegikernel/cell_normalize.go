package yaegikernel

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"strconv"
	"strings"
)

// cellSourceNormalization rewrites a model-authored cell so a persistent
// yaegi session can evaluate it. Cells are conceptually REPL fragments —
// variables, imports, and declarations persist across cells — but models
// routinely emit full `package main` programs instead. Two program shapes
// break the session if passed through verbatim:
//
//  1. Re-importing a package path. Yaegi compiles every cell as `_.go` in
//     the same package, so `import "choir"` in cell 2 collides with cell 1's
//     import (`choir/_.go redeclared in this block`). The model retries the
//     same program shape on every redelivery and the desk never commits —
//     the 2026-09-27 texture required-write starvation loop was this.
//  2. Declaring `func main`. Yaegi treats `main` as the cell's entrypoint
//     and re-executes it on every subsequent cell, so a stale `main` from an
//     earlier cell re-runs its side effects forever.
//
// Normalization accepts both shapes and reduces them to safe cells:
//
//   - full source files and declaration fragments (parsed via go/parser):
//     drop the package clause, drop import specs whose path a prior cell
//     already loaded, rename `func main` to a unique `__cell_main_<n>` and
//     append one explicit call (keeps `return` semantics, kills auto-rerun);
//   - import/statement mixes (imports followed by bare statements — the
//     shape models actually emit, e.g. `import "choir"` then a bare
//     `choir.Write(...)`) — hoist the import lines into an import
//     decl, lift top-level `x := e` to `var x = e`, wrap the remaining
//     statements in a synthetic `func main`, and let the decl path rename +
//     invoke it. Without this the cell compiles in yaegi file mode, which
//     rejects top-level statements outright;
//   - bare-statement fragments (can't parse as a file): scan-and-drop
//     duplicate top-level import lines, keep everything else verbatim so
//     expression cells still yield their value;
//   - anything unparseable passes through unchanged so yaegi's own
//     diagnostics remain authoritative.

// Session-level state: which package paths earlier cells installed.
// Only successful evals mark a path — a failed compile proves nothing ran.

func (s *Session) normalizeCellSource(src string) string {
	s.cellDeclares = nil
	src = guardInitOnlyForLoops(src)
	src = expandOneLineImportLists(src)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cell.go", src, parser.ParseComments)
	if err != nil {
		// Declaration fragment: prepend a synthetic package clause so the
		// parser accepts import/const/var/func decls. If that also fails the
		// source is a bare-statement fragment (or invalid) — handle below.
		fset = token.NewFileSet()
		file, err = parser.ParseFile(fset, "cell.go", "package p\n"+src, parser.ParseComments)
	}
	if err != nil {
		// Import/statement mix: models emit `import "x"` followed by bare
		// statements — unparseable as a file (import decls can't precede
		// statements) AND unparseable as a decl fragment (imports are decls,
		// statements aren't). Lift the contiguous leading import lines into
		// a decl, wrap the statement tail in `func main()`, and re-enter the
		// decl path, which renames and invokes main exactly once.
		// When every leading import is already installed (fmt, strings and
		// encoding/json always are), the cell is a statement fragment: drop
		// the imports and let yaegi run the statements in order, exactly
		// like a cell that never had them. Hoisting `x := e` into package
		// vars reorders it before the body and loses names the body rebinds
		// (K4, docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md).
		if rest, ok := s.dropInstalledLeadingImports(src); ok {
			return s.normalizeCellSource(rest)
		}
		var ok bool
		if file, fset, ok = s.parseWrappedStatementMix(src); !ok {
			return guardLeadingFuncStatement(s.degradeBoundDefines(s.dedupeFragmentImports(src)))
		}
	}

	var buf bytes.Buffer
	emitted := false
	emit := func(n any) {
		if emitted {
			buf.WriteByte('\n')
		}
		if err := format.Node(&buf, fset, n); err == nil {
			emitted = true
		}
	}
	// The package clause is dropped either way; only decls are emitted.

	// Pass 1: emit imports that are new to this session.
	seenThisCell := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		kept := &ast.GenDecl{Tok: token.IMPORT, Lparen: gen.Lparen}
		for _, spec := range gen.Specs {
			is := spec.(*ast.ImportSpec)
			path := strings.Trim(is.Path.Value, `"`)
			if s.importedPaths[path] || seenThisCell[path] {
				continue
			}
			seenThisCell[path] = true
			kept.Specs = append(kept.Specs, is)
		}
		if len(kept.Specs) > 0 {
			emit(kept)
		}
	}

	// Pass 2: emit non-import decls; rename func main and remember to call it.
	mainName := ""
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			continue
		}
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Recv == nil {
			s.cellSeq++
			mainName = fmt.Sprintf("__cell_main_%d", s.cellSeq)
			fn.Name = ast.NewIdent(mainName)
		}
		emit(decl)
	}
	if mainName != "" {
		// A bare `__cell_main_N()` statement would push yaegi out of file
		// mode ("expected declaration"); a blank var initializer is a decl,
		// runs exactly once at install, and never re-fires on later cells.
		buf.WriteString("\nvar _ = " + mainName + "()\n")
	}
	if !emitted && mainName == "" {
		// Everything deduped away (e.g. re-import-only cell): leave a no-op
		// expression so the cell is well-formed.
		return "1"
	}
	return buf.String()
}

// guardLeadingFuncStatement keeps a statement fragment that begins with the
// func keyword — `func() { ... }()`, the immediately invoked closure models
// write to scope a cell — from becoming a package-level main. Yaegi's
// incremental parser sees a leading FUNC token, fails to parse the fragment
// as a declaration, and retries it as `package main; func main() { ... }`
// in file mode: an installed main that re-runs on every later cell, so the
// closure's staged intents replay into cells that staged nothing (F2,
// docs/archive/problems/prompt-bar-minesweeper-demo-2026-10-10.md). A leading empty
// statement makes the first token a semicolon, so yaegi takes its ordinary
// statement path and evaluates the fragment exactly once.
//
// The same holds for a leading type, var or const declaration followed by
// statements (only such cells reach here; a cell of declarations alone
// parses as a file): yaegi would parse in declaration mode and fail
// "expected declaration" at the first statement. In statement mode the
// declarations still bind at the top level and persist into later cells.
func guardLeadingFuncStatement(src string) string {
	fset := token.NewFileSet()
	file := fset.AddFile("cell.go", fset.Base(), len(src))
	var scan scanner.Scanner
	scan.Init(file, []byte(src), nil, 0)
	switch _, tok, _ := scan.Scan(); tok {
	case token.FUNC, token.TYPE, token.VAR, token.CONST:
		return ";\n" + src
	}
	return src
}

// guardInitOnlyForLoops rewrites every `for init; ; {` header to
// `for init; true; {`, the same Go program. Yaegi v0.16.1 wires the
// init-only shape (forStmt1) back to its init statement on every
// iteration (interp/cfg.go: body.tnext = n.start), so a body that advances
// the loop variable never ends; a constant-true condition takes forStmt3,
// which is wired correctly. On the paper such cells grew until the guest
// OOM killer took a worker and then the daemon
// (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
//
// It scans tokens rather than parsing, so it reaches every cell shape. A
// header ends at its first brace outside parentheses and brackets, as in
// Go's grammar; a header holding a func literal therefore never matches and
// is left alone. The insertion stays on the header's line, so compile
// errors keep their line numbers.
func guardInitOnlyForLoops(src string) string {
	if !strings.Contains(src, "for") || !strings.Contains(src, ";") {
		return src
	}
	fset := token.NewFileSet()
	file := fset.AddFile("cell.go", fset.Base(), len(src))
	var scan scanner.Scanner
	scan.Init(file, []byte(src), func(token.Position, string) {}, 0)
	var inserts []int
	inHeader, depth, clause := false, 0, 0
	var clauseTokens [3]int
	secondSemi := 0
	for {
		pos, tok, _ := scan.Scan()
		if tok == token.EOF {
			break
		}
		if !inHeader {
			if tok == token.FOR {
				inHeader, depth, clause, clauseTokens = true, 0, 0, [3]int{}
			}
			continue
		}
		switch {
		case tok == token.LBRACE && depth == 0:
			if clause == 2 && clauseTokens[0] > 0 && clauseTokens[1] == 0 && clauseTokens[2] == 0 {
				inserts = append(inserts, secondSemi)
			}
			inHeader = false
		case tok == token.SEMICOLON && depth == 0:
			clause++
			if clause == 2 {
				secondSemi = file.Offset(pos)
			}
			if clause > 2 {
				inHeader = false
			}
		default:
			switch tok {
			case token.LPAREN, token.LBRACK:
				depth++
			case token.RPAREN, token.RBRACK:
				if depth > 0 {
					depth--
				}
			}
			clauseTokens[clause]++
		}
	}
	for i := len(inserts) - 1; i >= 0; i-- {
		at := inserts[i]
		src = src[:at] + "true" + src[at:]
	}
	return src
}

var (
	oneLineImportList = regexp.MustCompile(`^(\s*)import\s*\((.*)\)\s*;?\s*$`)
	importListSpec    = regexp.MustCompile(`(?:([\w\.]+)\s+)?"([^"]+)"`)
)

// expandOneLineImportLists rewrites `import ("a"; "b")` lines into the
// multi-line block the import handling below recognizes (K9,
// docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md). A line
// whose parentheses hold anything but import specs is left alone.
func expandOneLineImportLists(src string) string {
	if !strings.Contains(src, "import") {
		return src
	}
	lines := strings.Split(src, "\n")
	changed := false
	for i, line := range lines {
		m := oneLineImportList.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		specs := importListSpec.FindAllStringSubmatch(m[2], -1)
		if len(specs) == 0 || strings.TrimSpace(importListSpec.ReplaceAllString(strings.ReplaceAll(m[2], ";", " "), "")) != "" {
			continue
		}
		var b strings.Builder
		b.WriteString(m[1] + "import (\n")
		for _, spec := range specs {
			b.WriteString(m[1] + "\t")
			if spec[1] != "" {
				b.WriteString(spec[1] + " ")
			}
			b.WriteString(strconv.Quote(spec[2]) + "\n")
		}
		b.WriteString(m[1] + ")")
		lines[i] = b.String()
		changed = true
	}
	if !changed {
		return src
	}
	return strings.Join(lines, "\n")
}

// readsAny reports whether any of exprs mentions one of names.
func readsAny(exprs []ast.Expr, names []string) bool {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	found := false
	for _, e := range exprs {
		ast.Inspect(e, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && want[id.Name] {
				found = true
			}
			return !found
		})
	}
	return found
}

// degradeBoundDefines handles the statement-fragment cell. Top-level
// compound statements run inside func() { ... }() (see below). A top-level
// `x := e` whose names are all already bound becomes `x = e`: yaegi's
// statement mode would otherwise declare a fresh x, so `y := y * 2` reads the
// new, zero y. Only a right-hand side that reads one of the names is
// rewritten; any other redefinition stays a fresh declaration, whatever
// its type. The names a fragment's top-level `:=` introduces are recorded
// for the next cell. Source that needs no rewrite, or does not parse as
// statements, passes through verbatim so yaegi's diagnostics and the final
// expression's value stay authoritative.
func (s *Session) degradeBoundDefines(src string) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cell.go", "package p\nfunc f() {\n"+src+"\n}", parser.ParseComments)
	if err != nil || len(file.Decls) == 0 {
		return src
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return src
	}
	rewrote := false
	wrap := make([]bool, len(fn.Body.List))
	for i, st := range fn.Body.List {
		// yaegi's top-level statement mode mis-compiles definitions inside
		// compound statements (`b, err := json.Marshal(x)` in a range body
		// reports "undefined: b"); a function body compiles them correctly.
		// Names declared inside a compound statement are block-scoped, so
		// running it as func() { ... }() changes nothing else.
		switch inner := st.(type) {
		case *ast.LabeledStmt:
			switch inner.Stmt.(type) {
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				wrap[i], rewrote = true, true
			}
		case *ast.ForStmt, *ast.RangeStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.BlockStmt:
			wrap[i], rewrote = true, true
		}
		as, ok := st.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE {
			continue
		}
		allBound := true
		var names []string
		for _, e := range as.Lhs {
			id, isIdent := e.(*ast.Ident)
			if !isIdent {
				allBound = false
				break
			}
			if id.Name == "_" {
				continue
			}
			names = append(names, id.Name)
			if !s.declaredNames[id.Name] {
				allBound = false
			}
		}
		s.cellDeclares = append(s.cellDeclares, names...)
		// A bound helper redefined with a func literal is a new definition,
		// usually with a new signature: assigning it fails "cannot use type"
		// (K10). Self-reference, the case the rewrite exists for, cannot
		// occur in a func literal's own definition.
		if len(as.Rhs) == 1 {
			if _, isFunc := as.Rhs[0].(*ast.FuncLit); isFunc {
				allBound = false
			}
		}
		// Only a right-hand side that reads a bound name needs the rewrite.
		// Any other redefinition stays a fresh declaration: assigning it
		// fails "cannot use type", or kills the worker through reflect.Set,
		// when its type changed (cluster B, cross-model-cell-errors-2026-10-13).
		if allBound && len(names) > 0 && readsAny(as.Rhs, names) {
			as.Tok = token.ASSIGN
			rewrote = true
		}
	}
	if !rewrote {
		return src
	}
	var out strings.Builder
	for i, st := range fn.Body.List {
		if i > 0 {
			out.WriteByte('\n')
		}
		if wrap[i] {
			out.WriteString("func() {\n")
		}
		if err := format.Node(&out, fset, st); err != nil {
			return src
		}
		if wrap[i] {
			out.WriteString("\n}()")
		}
	}
	return out.String()
}

// dropInstalledLeadingImports returns src without its contiguous leading
// import lines when every path they name is already installed in the
// session and something follows them. Any new path, an aliased or
// unparseable spec, or an import block that does not close returns ok=false.
func (s *Session) dropInstalledLeadingImports(src string) (string, bool) {
	_, rest, paths, ok := leadingImportLines(src)
	if !ok {
		return "", false
	}
	for _, path := range paths {
		if !s.importedPaths[path] {
			return "", false
		}
	}
	return rest, true
}

// splitNewLeadingImports separates an import+statement cell whose imports
// are not all installed: head is the import lines alone, rest the statements.
// The session installs head first, then runs rest as an ordinary statement
// fragment. Wrapping the statements in a synthetic main and hoisting their
// definitions to package vars instead reorders them: yaegi ran the body
// before `var dump = func…` was set ("call of nil function", K9,
// docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md). A cell
// that parses as a Go file (imports plus declarations) is not split.
func (s *Session) splitNewLeadingImports(src string) (head, rest string, ok bool) {
	head, rest, paths, ok := leadingImportLines(expandOneLineImportLists(src))
	if !ok {
		return "", "", false
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "cell.go", "package p\n"+src, 0); err == nil {
		return "", "", false
	}
	for _, path := range paths {
		if !s.importedPaths[path] {
			return head, rest, true
		}
	}
	return "", "", false
}

// leadingImportLines splits src into its contiguous leading import lines
// (single `import "x"` lines and multi-line `import ( … )` blocks) and the
// non-empty rest. ok=false when there are no leading imports, nothing
// follows them, or a spec is aliased or unparseable.
func leadingImportLines(src string) (head, rest string, paths []string, ok bool) {
	lines := strings.Split(src, "\n")
	inBlock, sawImport := false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		var specs []string
		switch {
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case inBlock:
			if trimmed == "" {
				continue
			}
			specs = []string{trimmed}
		case trimmed == "":
			continue
		case trimmed == "import (" || trimmed == "import(":
			inBlock, sawImport = true, true
			continue
		case fragImportLine.MatchString(trimmed):
			specs = []string{strings.TrimSpace(strings.TrimPrefix(trimmed, "import"))}
			sawImport = true
		default:
			rest := strings.Join(lines[i:], "\n")
			if !sawImport || strings.TrimSpace(rest) == "" {
				return "", "", nil, false
			}
			return strings.Join(lines[:i], "\n"), rest, paths, true
		}
		for _, spec := range specs {
			path, err := strconv.Unquote(spec)
			if err != nil {
				return "", "", nil, false
			}
			paths = append(paths, path)
		}
	}
	return "", "", nil, false
}

// parseWrappedStatementMix rescues the import+statement shape models emit:
// contiguous leading `import "x"` / `import ( ... )` lines are lifted into a
// decl, top-level `name := expr` statements hoist to `var name = expr` when
// the name isn't already bound (so the binding persists for later cells
// exactly like a bare-statement cell) and degrade to `name = expr` in the
// body when it is, and the remaining statements wrap in `func main()`
// (which pass 2 renames to __cell_main_N and invokes once). Returns the
// parsed file+fileset, or ok=false when the shape doesn't fit (comments
// inside the import block, mid-file imports, or a statement tail that still
// won't parse) — the caller falls back to dedupeFragmentImports so yaegi's
// own diagnostics apply. Hoisted names land in s.cellDeclares so a
// successful eval can mark them bound for the next cell.
func (s *Session) parseWrappedStatementMix(src string) (*ast.File, *token.FileSet, bool) {
	s.cellDeclares = nil
	lines := strings.Split(src, "\n")
	var imports []string
	var rest []string
	inBlock := false
	sawBody := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case sawBody:
			rest = append(rest, line)
		case inBlock:
			if trimmed == ")" {
				inBlock = false
				continue
			}
			if trimmed == "" || fragImportSpecLine(trimmed) {
				imports = append(imports, trimmed)
				continue
			}
			return nil, nil, false
		case trimmed == "":
			continue // leading blank — ambiguous position, skip
		case trimmed == "import (" || trimmed == "import(":
			inBlock = true
		case fragImportLine.MatchString(trimmed):
			imports = append(imports, strings.TrimSpace(strings.TrimPrefix(trimmed, "import")))
		default:
			sawBody = true
			rest = append(rest, line)
		}
	}
	if inBlock || len(imports) == 0 || len(rest) == 0 {
		return nil, nil, false
	}
	// Names already bound at package scope this cell will see: every prior
	// declared/hoisted name plus this cell's import names (the package
	// identifier `fmt` in `import "fmt"` is a package-scope binding — a
	// `fmt := …` that hoisted to `var fmt` would collide).
	bound := map[string]bool{}
	for name := range s.declaredNames {
		bound[name] = true
	}
	for _, spec := range imports {
		if name := importSpecBoundName(spec); name != "" {
			bound[name] = true
		}
	}
	// Split the statement tail via AST: `x := e` on new names hoists to
	// `var x = e`; `x := e` on a bound name degrades to `x = e` in the body;
	// all other statements pass through in order.
	restSrc := strings.Join(rest, "\n")
	stmtSet := token.NewFileSet()
	stmtFile, stmtErr := parser.ParseFile(stmtSet, "stmts.go", "package p\nfunc f() {\n"+restSrc+"\n}", parser.ParseComments)
	if stmtErr != nil || len(stmtFile.Decls) == 0 {
		return nil, nil, false
	}
	fn, ok := stmtFile.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return nil, nil, false
	}
	var hoisted []string
	var body []string
	for _, st := range fn.Body.List {
		if as, ok := st.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
			allNew := true
			for _, e := range as.Lhs {
				id, isIdent := e.(*ast.Ident)
				if !isIdent || id.Name == "_" {
					continue
				}
				if bound[id.Name] {
					allNew = false
					break
				}
			}
			if allNew {
				var lhs, rhs strings.Builder
				for i, e := range as.Lhs {
					if i > 0 {
						lhs.WriteString(", ")
					}
					format.Node(&lhs, stmtSet, e)
				}
				for i, e := range as.Rhs {
					if i > 0 {
						rhs.WriteString(", ")
					}
					format.Node(&rhs, stmtSet, e)
				}
				hoisted = append(hoisted, "var "+lhs.String()+" = "+rhs.String())
				for _, e := range as.Lhs {
					if id, isIdent := e.(*ast.Ident); isIdent && id.Name != "_" {
						bound[id.Name] = true
					}
				}
				continue
			}
		}
		var out strings.Builder
		format.Node(&out, stmtSet, st)
		body = append(body, out.String())
	}
	var sb strings.Builder
	sb.WriteString("package p\nimport (\n")
	sb.WriteString(strings.Join(imports, "\n"))
	sb.WriteString("\n)\n")
	for _, h := range hoisted {
		sb.WriteString(h + "\n")
	}
	sb.WriteString("func main() {\n")
	sb.WriteString(strings.Join(body, "\n"))
	sb.WriteString("\n}\n")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cell.go", sb.String(), parser.ParseComments)
	if err != nil {
		return nil, nil, false
	}
	for _, st := range file.Decls {
		gd, isGen := st.(*ast.GenDecl)
		if !isGen || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			for _, n := range spec.(*ast.ValueSpec).Names {
				s.cellDeclares = append(s.cellDeclares, n.Name)
			}
		}
	}
	return file, fset, true
}

// pathBase returns the package-scope identifier an import binds: `bar` for
// `foo/bar`, the trailing element for any path.
func pathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// importSpecBoundName returns the package-scope identifier an import spec
// binds: the explicit alias when present (`baz` for `baz "foo/bar"`), the
// path's trailing element otherwise, "" for `_`/`.` forms (which bind no
// usable package identifier here — dot-import collisions stay the
// interpreter's problem).
func importSpecBoundName(spec string) string {
	m := fragImportSpecRe.FindStringSubmatch(spec)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		if m[1] == "." || m[1] == "_" {
			return ""
		}
		return m[1]
	}
	return pathBase(m[2])
}

var fragImportSpecRe = regexp.MustCompile(`^(?:([\w\.]+)\s+)?"([^"]+)"$`)

// fragImportSpecLine reports whether a line inside an `import ( ... )` block
// is a bare spec (`"fmt"` or `alias "fmt"`), so the rescue pass can lift it.
func fragImportSpecLine(line string) bool {
	return regexp.MustCompile(`^(?:[\w\.]+\s+)?"[^"]+"$`).MatchString(line)
}

// dedupeFragmentImports strips top-level `import "path"` / `import (…)` lines
// whose path an earlier cell already loaded, from a fragment that couldn't be
// parsed as a file or decl list. Matching is deliberately conservative: only
// whole-line import statements at the start or after other imports are
// rewritten; anything ambiguous passes through so yaegi's diagnostics apply.
var fragImportLine = regexp.MustCompile(`^\s*import\s+(?:[\w\.]+\s+)?"([^"]+)"\s*$`)

func (s *Session) dedupeFragmentImports(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inBlock {
			if trimmed == ")" {
				inBlock = false
				out = append(out, line)
				continue
			}
			spec := fragImportLine.FindStringSubmatch("import " + trimmed)
			if spec != nil && s.importedPaths[spec[1]] {
				continue
			}
			out = append(out, line)
			continue
		}
		if trimmed == "import (" || trimmed == "import(" {
			inBlock = true
			out = append(out, line)
			continue
		}
		if m := fragImportLine.FindStringSubmatch(line); m != nil && s.importedPaths[m[1]] {
			continue
		}
		out = append(out, line)
	}
	joined := strings.Join(out, "\n")
	if strings.TrimSpace(joined) == "" {
		return "1"
	}
	return joined
}

// markCellImports records the import paths a successfully-evaluated cell
// carried, so subsequent cells drop the same decls. Called only after a
// successful eval — a compile-failed cell installed nothing.
func (s *Session) markCellImports(src string) {
	for _, path := range cellImportPaths(src) {
		s.importedPaths[path] = true
	}
}

// cellImportPaths extracts import paths from a raw cell source (any shape).
func cellImportPaths(src string) []string {
	fset := token.NewFileSet()
	var paths []string
	seen := map[string]bool{}
	collect := func(f *ast.File) {
		for _, is := range f.Imports {
			p := strings.Trim(is.Path.Value, `"`)
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	if f, err := parser.ParseFile(fset, "cell.go", src, parser.ImportsOnly); err == nil {
		collect(f)
		return paths
	}
	if f, err := parser.ParseFile(fset, "cell.go", "package p\n"+src, parser.ImportsOnly); err == nil {
		collect(f)
		return paths
	}
	// Statement fragment: scan lines.
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if inBlock {
			if t == ")" {
				inBlock = false
				continue
			}
			if m := fragImportLine.FindStringSubmatch("import " + t); m != nil && !seen[m[1]] {
				seen[m[1]] = true
				paths = append(paths, m[1])
			}
			continue
		}
		if t == "import (" || t == "import(" {
			inBlock = true
			continue
		}
		if m := fragImportLine.FindStringSubmatch(line); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			paths = append(paths, m[1])
		}
	}
	return paths
}
