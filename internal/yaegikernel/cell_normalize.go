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
//     `choir.ApplyTexture(...)`) — hoist the import lines into an import
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
		var ok bool
		if file, fset, ok = s.parseWrappedStatementMix(src); !ok {
			return guardLeadingFuncStatement(s.dedupeFragmentImports(src))
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
// docs/problems/prompt-bar-minesweeper-demo-2026-10-10.md). A leading empty
// statement makes the first token a semicolon, so yaegi takes its ordinary
// statement path and evaluates the fragment exactly once.
func guardLeadingFuncStatement(src string) string {
	fset := token.NewFileSet()
	file := fset.AddFile("cell.go", fset.Base(), len(src))
	var scan scanner.Scanner
	scan.Init(file, []byte(src), nil, 0)
	if _, tok, _ := scan.Scan(); tok != token.FUNC {
		return src
	}
	return ";\n" + src
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
