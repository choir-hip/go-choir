package yaegikernel

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
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
	declFrag := false
	if err != nil {
		// Declaration fragment: prepend a synthetic package clause so the
		// parser accepts import/const/var/func decls. If that also fails the
		// source is a bare-statement fragment (or invalid) — handle below.
		fset = token.NewFileSet()
		file, err = parser.ParseFile(fset, "cell.go", "package p\n"+src, parser.ParseComments)
		declFrag = true
	}
	if err != nil {
		return s.dedupeFragmentImports(src)
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
	_ = declFrag // package clause is dropped either way; only decls are emitted

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
